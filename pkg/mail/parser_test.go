// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package mail

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/getpatchwork/patchwork/pkg/db"
)

var sampleDiff = "diff --git a/meep.text b/meep.text\n" +
	"index 3d75d48..a57f4dd 100644\n" +
	"--- a/meep.text\n" +
	"+++ b/meep.text\n" +
	"@@ -1,1 +1,2 @@\n" +
	" meep\n" +
	"+meep\n"

func TestEncodingParse(t *testing.T) {
	tests := []string{
		"mail/0012-invalid-header-char.mbox",
		"mail/0013-with-utf8-body.mbox",
		"mail/0014-with-unencoded-utf8-headers.mbox",
		"mail/0015-with-invalid-utf8-headers.mbox",
	}

	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			database, ctx, _, _ := testDB(t, "patchwork.ozlabs.org")
			result := parseMbox(t, ctx, database, name, "patchwork.ozlabs.org")
			assert.Equal(t, 1, result.patches)
		})
	}
}

func TestDuplicateMail(t *testing.T) {
	database, ctx, _, _ := testDB(t, "patchwork.ozlabs.org")

	result := parseMbox(t, ctx, database, "mail/0013-with-utf8-body.mbox", "patchwork.ozlabs.org")
	require.Equal(t, 1, result.patches, "first parse")

	result2 := parseMbox(t, ctx, database, "mail/0013-with-utf8-body.mbox", "patchwork.ozlabs.org")
	assert.Equal(t, 1, result2.patches, "second parse")
	assert.Equal(t, 1, result2.duplicates)
}

func TestWeirdMail(t *testing.T) {
	database, ctx, _, _ := testDB(t, "patchwork.ozlabs.org")

	fuzzFiles := []string{
		"fuzz/base64err.mbox",
		"fuzz/charset.mbox",
		"fuzz/codec-null.mbox",
		"fuzz/date.mbox",
		"fuzz/date-oserror.mbox",
		"fuzz/date-too-long.mbox",
		"fuzz/dateheader.mbox",
		"fuzz/email-len.mbox",
		"fuzz/msgid-len.mbox",
		"fuzz/msgid-len2.mbox",
		"fuzz/msgidheader.mbox",
		"fuzz/name-len.mbox",
		"fuzz/refshdr.mbox",
		"fuzz/unknown-encoding.mbox",
		"fuzz/value2.mbox",
		"fuzz/x-face.mbox",
		"fuzz/year-out-of-range.mbox",
	}

	for _, name := range fuzzFiles {
		t.Run(name, func(t *testing.T) {
			parseMbox(t, ctx, database, name)
		})
	}
}

func TestDuplicatePatchAndComment(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	patchMsgID := "<dup-patch@test>"

	err := parseEmail(t, ctx, database, sampleDiff,
		withMsgID(patchMsgID),
		withListID("test.example.com"))
	require.NoError(t, err)
	require.Equal(t, 1, countPatches(t, database))

	err = parseEmail(t, ctx, database, sampleDiff,
		withMsgID(patchMsgID),
		withListID("test.example.com"))
	var dupErr *DuplicateMailError
	assert.ErrorAs(t, err, &dupErr)
	assert.Equal(t, 1, countPatches(t, database), "expected still 1 patch")

	commentMsgID := "<dup-comment@test>"
	err = parseEmail(t, ctx, database, "nice patch\nAcked-by: Me <me@test>",
		withMsgID(commentMsgID),
		withSubject("Re: test"),
		withInReplyTo(patchMsgID),
		withListID("test.example.com"))
	require.NoError(t, err)
	require.Equal(t, 1, countPatchComments(t, database))

	err = parseEmail(t, ctx, database, "nice patch\nAcked-by: Me <me@test>",
		withMsgID(commentMsgID),
		withSubject("Re: test"),
		withInReplyTo(patchMsgID),
		withListID("test.example.com"))
	assert.ErrorAs(t, err, &dupErr, "expected DuplicateMailError for comment")
}

func TestDuplicateCoverLetter(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	coverMsgID := "<dup-cover@test>"
	parseEmail(t, ctx, database, "cover body",
		withMsgID(coverMsgID),
		withSubject("[PATCH 0/1] test cover"),
		withListID("test.example.com"))

	err := parseEmail(t, ctx, database, "cover body",
		withMsgID(coverMsgID),
		withSubject("[PATCH 0/1] test cover"),
		withListID("test.example.com"))

	var dupErr *DuplicateMailError
	assert.ErrorAs(t, err, &dupErr, "expected DuplicateMailError for cover")
}

func TestInitialPatchState(t *testing.T) {
	database, ctx, _, proj := testDB(t, "test.example.com")
	makeMaintainer(t, database, "test-author@example.com", proj.ID)

	t.Run("default state", func(t *testing.T) {
		err := parseEmail(t, ctx, database, sampleDiff,
			withListID("test.example.com"))
		require.NoError(t, err)
		var stateID int
		database.NewSelect().TableExpr("patch").
			Column("state_id").Limit(1).
			Scan(context.Background(), &stateID)
		var ordering int
		database.NewSelect().TableExpr("state").
			Column("ordering").Where("id = ?", stateID).
			Scan(context.Background(), &ordering)
		assert.Equal(t, 0, ordering)
	})

	t.Run("explicit state", func(t *testing.T) {
		err := parseEmail(t, ctx, database, sampleDiff,
			withListID("test.example.com"),
			withHeader("X-Patchwork-State", "Accepted"))
		require.NoError(t, err)

		var stateName string
		database.NewSelect().TableExpr("patch AS p").
			Join("JOIN state AS s ON s.id = p.state_id").
			Column("s.name").
			OrderExpr("p.id DESC").Limit(1).
			Scan(context.Background(), &stateName)
		assert.Equal(t, "Accepted", stateName)
	})
}

func TestInitialPatchStateFull(t *testing.T) {
	database, ctx, _, proj := testDB(t, "test.example.com")
	makeMaintainer(t, database, "test-author@example.com", proj.ID)

	t.Run("implicit default state", func(t *testing.T) {
		parseEmail(t, ctx, database, sampleDiff, withListID("test.example.com"))
		var stateName string
		database.NewSelect().TableExpr("patch AS p").
			Join("JOIN state AS s ON s.id = p.state_id").
			Column("s.name").OrderExpr("p.id DESC").Limit(1).
			Scan(context.Background(), &stateName)
		assert.Equal(t, "New", stateName)
	})

	t.Run("explicit non-default state", func(t *testing.T) {
		parseEmail(t, ctx, database, sampleDiff,
			withListID("test.example.com"),
			withHeader("X-Patchwork-State", "RFC"))
		var stateName string
		database.NewSelect().TableExpr("patch AS p").
			Join("JOIN state AS s ON s.id = p.state_id").
			Column("s.name").OrderExpr("p.id DESC").Limit(1).
			Scan(context.Background(), &stateName)
		assert.Equal(t, "RFC", stateName)
	})

	t.Run("invalid state falls back to default", func(t *testing.T) {
		parseEmail(t, ctx, database, sampleDiff,
			withListID("test.example.com"),
			withHeader("X-Patchwork-State", "Nonexistent State"))
		var stateName string
		database.NewSelect().TableExpr("patch AS p").
			Join("JOIN state AS s ON s.id = p.state_id").
			Column("s.name").OrderExpr("p.id DESC").Limit(1).
			Scan(context.Background(), &stateName)
		assert.Equal(t, "New", stateName)
	})
}

func TestDelegateRequest(t *testing.T) {
	database, ctx, _, proj := testDB(t, "test.example.com")
	makeMaintainer(t, database, "test-author@example.com", proj.ID)

	database.NewRaw(`
		INSERT INTO auth_user (username, email, password, is_admin,
			is_active, date_joined, first_name, last_name,
			send_email, items_per_page, show_ids)
		VALUES ('testuser', 'delegate@example.com', '', false,
			true, datetime('now'), '', '',
			false, 100, false)
	`).Exec(context.Background())

	t.Run("valid delegate", func(t *testing.T) {
		parseEmail(t, ctx, database, sampleDiff,
			withListID("test.example.com"),
			withHeader("X-Patchwork-Delegate", "delegate@example.com"))
		var delegateID *int
		database.NewSelect().TableExpr("patch").
			Column("delegate_id").OrderExpr("id DESC").Limit(1).
			Scan(context.Background(), &delegateID)
		require.NotNil(t, delegateID, "expected delegate to be set")
		var email string
		database.NewRaw("SELECT email FROM auth_user WHERE id = ?",
			*delegateID).Scan(context.Background(), &email)
		assert.Equal(t, "delegate@example.com", email)
	})

	t.Run("no delegate", func(t *testing.T) {
		parseEmail(t, ctx, database, sampleDiff,
			withListID("test.example.com"))
		var delegateID *int
		database.NewSelect().TableExpr("patch").
			Column("delegate_id").OrderExpr("id DESC").Limit(1).
			Scan(context.Background(), &delegateID)
		assert.Nil(t, delegateID, "expected no delegate")
	})

	t.Run("invalid delegate", func(t *testing.T) {
		parseEmail(t, ctx, database, sampleDiff,
			withListID("test.example.com"),
			withHeader("X-Patchwork-Delegate", "nobody"))
		var delegateID *int
		database.NewSelect().TableExpr("patch").
			Column("delegate_id").OrderExpr("id DESC").Limit(1).
			Scan(context.Background(), &delegateID)
		assert.Nil(t, delegateID, "expected no delegate for invalid email")
	})
}

func TestParseTags(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	tagContent := "test comment\n\n" +
		"Tested-by: Test User <test@example.com>\n" +
		"Reviewed-by: Test User <test@example.com>\n"

	t.Run("tags on initial patch", func(t *testing.T) {
		patchMsgID := "<tag-patch@test>"
		parseEmail(t, ctx, database, tagContent+"\n"+sampleDiff,
			withMsgID(patchMsgID),
			withListID("test.example.com"))

		var testedCount, reviewedCount, ackedCount int
		database.NewRaw(`
			SELECT coalesce(sum(CASE WHEN t.name = 'Tested-by' THEN pt.count END), 0),
				coalesce(sum(CASE WHEN t.name = 'Reviewed-by' THEN pt.count END), 0),
				coalesce(sum(CASE WHEN t.name = 'Acked-by' THEN pt.count END), 0)
			FROM patch_tag pt
			JOIN tag t ON t.id = pt.tag_id
			JOIN patch p ON p.id = pt.patch_id
			WHERE p.msgid = ?
		`, patchMsgID).Scan(context.Background(), &testedCount, &reviewedCount, &ackedCount)

		assert.Equal(t, 1, testedCount, "Tested-by")
		assert.Equal(t, 1, reviewedCount, "Reviewed-by")
		assert.Equal(t, 0, ackedCount, "Acked-by")
	})

	t.Run("tags from comment update patch counts", func(t *testing.T) {
		patchMsgID := "<tag-patch2@test>"
		parseEmail(t, ctx, database, sampleDiff,
			withMsgID(patchMsgID),
			withListID("test.example.com"))

		parseEmail(t, ctx, database, tagContent,
			withSubject("Re: test"),
			withInReplyTo(patchMsgID),
			withListID("test.example.com"))

		var testedCount int
		database.NewRaw(`
			SELECT coalesce(sum(pt.count), 0)
			FROM patch_tag pt
			JOIN tag t ON t.id = pt.tag_id
			JOIN patch p ON p.id = pt.patch_id
			WHERE p.msgid = ? AND t.name = 'Tested-by'
		`, patchMsgID).Scan(context.Background(), &testedCount)

		assert.Equal(t, 1, testedCount, "Tested-by")
	})
}

func TestInlinePatchVariants(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	t.Run("signature stripped", func(t *testing.T) {
		body := "Test comment\nmore comment\n-- \nsig\n" + sampleDiff
		err := parseEmail(t, ctx, database, body, withListID("test.example.com"))
		require.NoError(t, err)
		assert.Equal(t, 1, countPatches(t, database))
	})

	t.Run("update comment preserved", func(t *testing.T) {
		body := "Test comment\n---\nUpdate: test update\n" + sampleDiff
		err := parseEmail(t, ctx, database, body, withListID("test.example.com"))
		require.NoError(t, err)
		assert.Equal(t, 2, countPatches(t, database))
	})

	t.Run("list footer stripped", func(t *testing.T) {
		body := "Test comment\n" + sampleDiff + "\n_______________________________________________\nfooter\n"
		err := parseEmail(t, ctx, database, body, withListID("test.example.com"))
		require.NoError(t, err)
		assert.Equal(t, 3, countPatches(t, database))
	})

	t.Run("diff word in comment", func(t *testing.T) {
		body := "This is a comment with the word differently in it\n" + sampleDiff
		err := parseEmail(t, ctx, database, body, withListID("test.example.com"))
		require.NoError(t, err)
		assert.Equal(t, 4, countPatches(t, database))
	})
}

func TestAttachmentPatch(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	for _, subtype := range []string{"x-patch", "x-diff"} {
		t.Run(subtype, func(t *testing.T) {
			boundary := "----=_test_boundary"
			body := fmt.Sprintf(
				"--%s\r\n"+
					"Content-Type: text/plain; charset=us-ascii\r\n\r\n"+
					"Test for attached patch\r\n"+
					"--%s\r\n"+
					"Content-Type: text/%s; charset=us-ascii\r\n\r\n"+
					"%s\r\n"+
					"--%s--\r\n",
				boundary, boundary, subtype, sampleDiff, boundary,
			)

			data := fmt.Sprintf(
				"From: test@example.com\r\n"+
					"Subject: [PATCH] test %s\r\n"+
					"Message-ID: <%s-attach@test>\r\n"+
					"List-Id: <test.example.com>\r\n"+
					"Content-Type: multipart/mixed; boundary=\"%s\"\r\n"+
					"\r\n%s",
				subtype, subtype, boundary, body,
			)

			err := ParseMail(ctx, database,
				strings.NewReader(data), false, "test.example.com")
			require.NoError(t, err)
		})
	}
	assert.Equal(t, 2, countPatches(t, database))
}

func TestSubjectEncoding(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	t.Run("ascii", func(t *testing.T) {
		parseEmail(t, ctx, database, sampleDiff,
			withSubject("[PATCH] ascii subject test"),
			withListID("test.example.com"))
		assert.Equal(t, 1, countPatches(t, database))
	})

	t.Run("utf8 quoted-printable", func(t *testing.T) {
		parseEmail(t, ctx, database, sampleDiff,
			withSubject("=?utf-8?q?[PATCH]_=C3=A9_encoded?="),
			withListID("test.example.com"))
		assert.Equal(t, 2, countPatches(t, database))
	})
}

func TestSubjectEncodingMultipleWords(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	parseEmail(t, ctx, database, sampleDiff,
		withSubject("=?utf-8?q?[PATCH]_first?= =?utf-8?q?_second?="),
		withListID("test.example.com"))
	assert.Equal(t, 1, countPatches(t, database), "expected 1 patch for multi-word encoded subject")
	var name string
	database.NewSelect().TableExpr("patch").
		Column("name").Limit(1).
		Scan(context.Background(), &name)
	assert.NotEmpty(t, name, "patch name should not be empty")
}

func TestFindMessageID(t *testing.T) {
	t.Run("missing header", func(t *testing.T) {
		data := "From: test@example.com\r\nSubject: test\r\n\r\nbody\r\n"
		database, ctx, _, _ := testDB(t, "test.example.com")
		_ = ParseMail(ctx, database,
			strings.NewReader(data), false, "test.example.com")
	})

	t.Run("header with comments", func(t *testing.T) {
		database, ctx, _, _ := testDB(t, "test.example.com")
		parseEmail(t, ctx, database, sampleDiff,
			withMsgID("<test-id@example.com> (comment)"),
			withListID("test.example.com"))
		assert.Equal(t, 1, countPatches(t, database), "expected 1 patch despite comment in Message-ID")
	})
}

func TestFindMessageIDInvalidFallback(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	data := fmt.Sprintf(
		"From: test@example.com\r\n"+
			"Subject: [PATCH] test\r\n"+
			"Message-ID: bad-msgid-no-brackets@example.com\r\n"+
			"List-Id: <test.example.com>\r\n"+
			"\r\n%s", sampleDiff,
	)
	_ = ParseMail(ctx, database,
		strings.NewReader(data), false, "test.example.com")
}

func TestFindReferencesInvalidFallback(t *testing.T) {
	h := makeHeader(t, map[string]string{
		"From":        "test@example.com",
		"Subject":     "test",
		"Message-ID":  "<test@example.com>",
		"In-Reply-To": "5899d592-8c87-47d9-92b6-d34260ce1aa4@radware.com>",
	})
	refs := FindReferences(h)
	_ = refs
}

func TestAnonymize(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	orgProj := db.Project{
		Linkname:  "org-test-project",
		Name:      "Original Test Project",
		Listid:    "prod.example.com",
		Listemail: "test@" + "prod.example.com",
		UseTags:   true,
	}
	err := database.NewInsert().Model(&orgProj).
		Returning("*").
		Scan(context.Background())
	require.NoError(t, err)

	data := fmt.Sprintf(
		"From: name <name@prod.example.com>\r\n"+
			"Subject: [PATCH] test\r\n"+
			"Message-ID: <msgid@prod.example.com>\r\n"+
			"In-Reply-To: <msgid2@prod.example.com>\r\n"+
			"References: <msgid3@prod.example.com>\r\n"+
			"List-Id: <prod.example.com>\r\n"+
			"Mime-Version: 1.0\r\n"+
			"Content-Type: text/plain\r\n"+
			"Date: Sat, 25 Oct 2025 16:08:59 +0300\r\n"+
			"DKIM-Signature: invalid\r\n"+
			"Received: invalid\r\n"+
			"X-Unknown-Header: some data\r\n"+
			"\r\n%s", sampleDiff,
	)
	_ = ParseMail(ctx, database,
		strings.NewReader(data), true, "test.example.com")

	var patch db.Patch
	database.NewSelect().TableExpr("patch").
		Limit(1).
		Scan(context.Background(), &patch)

	assert.Equal(t, "<YX-dQdlCn9WDRoa7DK8SUA==@test.example.com>", patch.Msgid)
	assert.Equal(t, sampleDiff, *patch.Diff)
	assert.Equal(t, "test", patch.Name)
	assert.NotEqual(t, orgProj.ID, patch.ProjectID)

	for line := range strings.Lines(patch.Headers) {
		split := strings.SplitN(line, ":", 2)
		name := split[0]
		value := strings.TrimSpace(split[1])

		switch name {
		case "From":
			assert.Equal(t, "\"name\" <jMgxeXuue9pjhfGwVtJ12g==@test.example.com>", value)
		case "Message-Id":
			assert.Equal(t, "<YX-dQdlCn9WDRoa7DK8SUA==@test.example.com>", patch.Msgid)
		case "In-Reply-To":
			assert.Equal(t, "<b1fDLNDfC2hpylSip1gjLg==@test.example.com>", value)
		case "References":
			assert.Equal(t, "<XT0eAL1xLL3UwE2Vw0eidA==@test.example.com>", value)
		case "List-Id":
			assert.Equal(t, "<test.example.com>", value)
		case "Mime-Version":
			assert.Equal(t, "1.0", value)
		case "Content-Type":
			assert.Equal(t, "text/plain", value)
		case "Date":
			assert.Equal(t, "Sat, 25 Oct 2025 16:08:59 +0300", value)
		case "Subject":
			assert.Equal(t, "[PATCH] test", value)
		default:
			assert.Fail(t, "invalid header in headers %s", name)
		}
	}

	var submitter db.Person
	database.NewSelect().TableExpr("person").
		Limit(1).
		Scan(context.Background(), &submitter)

	assert.Equal(t, submitter.ID, patch.SubmitterID)
	assert.Equal(t, "name", *submitter.Name)
	assert.Equal(t, "jmgxexuue9pjhfgwvtj12g==@test.example.com", submitter.Email)
}
