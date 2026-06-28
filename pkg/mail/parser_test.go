// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package mail

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
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
			if result.patches != 1 {
				t.Errorf("expected exactly 1 patch, got %d", result.patches)
			}
		})
	}
}

func TestDuplicateMail(t *testing.T) {
	database, ctx, _, _ := testDB(t, "patchwork.ozlabs.org")

	result := parseMbox(t, ctx, database, "mail/0013-with-utf8-body.mbox", "patchwork.ozlabs.org")
	if result.patches != 1 {
		t.Fatalf("first parse: expected 1 patch, got %d", result.patches)
	}

	result2 := parseMbox(t, ctx, database, "mail/0013-with-utf8-body.mbox", "patchwork.ozlabs.org")
	if result2.patches != 1 {
		t.Errorf("second parse: expected still 1 patch, got %d", result2.patches)
	}
	if result2.duplicates != 1 {
		t.Errorf("expected 1 duplicate, got %d", result2.duplicates)
	}
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
	if err != nil {
		t.Fatal(err)
	}
	if countPatches(t, database) != 1 {
		t.Fatal("expected 1 patch")
	}

	err = parseEmail(t, ctx, database, sampleDiff,
		withMsgID(patchMsgID),
		withListID("test.example.com"))
	var dupErr *DuplicateMailError
	if !errors.As(err, &dupErr) {
		t.Errorf("expected DuplicateMailError, got %v", err)
	}
	if countPatches(t, database) != 1 {
		t.Error("expected still 1 patch")
	}

	commentMsgID := "<dup-comment@test>"
	err = parseEmail(t, ctx, database, "nice patch\nAcked-by: Me <me@test>",
		withMsgID(commentMsgID),
		withSubject("Re: test"),
		withInReplyTo(patchMsgID),
		withListID("test.example.com"))
	if err != nil {
		t.Fatal(err)
	}
	if countPatchComments(t, database) != 1 {
		t.Fatal("expected 1 comment")
	}

	err = parseEmail(t, ctx, database, "nice patch\nAcked-by: Me <me@test>",
		withMsgID(commentMsgID),
		withSubject("Re: test"),
		withInReplyTo(patchMsgID),
		withListID("test.example.com"))
	if !errors.As(err, &dupErr) {
		t.Errorf("expected DuplicateMailError for comment, got %v", err)
	}
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
	if !errors.As(err, &dupErr) {
		t.Errorf("expected DuplicateMailError for cover, got %v", err)
	}
}

func TestInitialPatchState(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	t.Run("default state", func(t *testing.T) {
		err := parseEmail(t, ctx, database, sampleDiff,
			withListID("test.example.com"))
		if err != nil {
			t.Fatal(err)
		}
		var stateID int
		database.NewSelect().TableExpr("patch").
			Column("state_id").Limit(1).
			Scan(context.Background(), &stateID)
		var ordering int
		database.NewSelect().TableExpr("state").
			Column("ordering").Where("id = ?", stateID).
			Scan(context.Background(), &ordering)
		if ordering != 0 {
			t.Errorf("expected default state (ordering=0), got ordering=%d", ordering)
		}
	})

	t.Run("explicit state", func(t *testing.T) {
		err := parseEmail(t, ctx, database, sampleDiff,
			withListID("test.example.com"),
			withHeader("X-Patchwork-State", "Accepted"))
		if err != nil {
			t.Fatal(err)
		}

		var stateName string
		database.NewSelect().TableExpr("patch AS p").
			Join("JOIN state AS s ON s.id = p.state_id").
			Column("s.name").
			OrderExpr("p.id DESC").Limit(1).
			Scan(context.Background(), &stateName)
		if stateName != "Accepted" {
			t.Errorf("state = %q, want Accepted", stateName)
		}
	})
}

func TestInitialPatchStateFull(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	t.Run("implicit default state", func(t *testing.T) {
		parseEmail(t, ctx, database, sampleDiff, withListID("test.example.com"))
		var stateName string
		database.NewSelect().TableExpr("patch AS p").
			Join("JOIN state AS s ON s.id = p.state_id").
			Column("s.name").OrderExpr("p.id DESC").Limit(1).
			Scan(context.Background(), &stateName)
		if stateName != "New" {
			t.Errorf("default state = %q, want New", stateName)
		}
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
		if stateName != "RFC" {
			t.Errorf("state = %q, want RFC", stateName)
		}
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
		if stateName != "New" {
			t.Errorf("state = %q, want New (fallback)", stateName)
		}
	})
}

func TestDelegateRequest(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

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
		if delegateID == nil {
			t.Fatal("expected delegate to be set")
		}
		var email string
		database.NewRaw("SELECT email FROM auth_user WHERE id = ?",
			*delegateID).Scan(context.Background(), &email)
		if email != "delegate@example.com" {
			t.Errorf("delegate email = %q, want delegate@example.com", email)
		}
	})

	t.Run("no delegate", func(t *testing.T) {
		parseEmail(t, ctx, database, sampleDiff,
			withListID("test.example.com"))
		var delegateID *int
		database.NewSelect().TableExpr("patch").
			Column("delegate_id").OrderExpr("id DESC").Limit(1).
			Scan(context.Background(), &delegateID)
		if delegateID != nil {
			t.Error("expected no delegate")
		}
	})

	t.Run("invalid delegate", func(t *testing.T) {
		parseEmail(t, ctx, database, sampleDiff,
			withListID("test.example.com"),
			withHeader("X-Patchwork-Delegate", "nobody"))
		var delegateID *int
		database.NewSelect().TableExpr("patch").
			Column("delegate_id").OrderExpr("id DESC").Limit(1).
			Scan(context.Background(), &delegateID)
		if delegateID != nil {
			t.Error("expected no delegate for invalid email")
		}
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

		if testedCount != 1 {
			t.Errorf("Tested-by count = %d, want 1", testedCount)
		}
		if reviewedCount != 1 {
			t.Errorf("Reviewed-by count = %d, want 1", reviewedCount)
		}
		if ackedCount != 0 {
			t.Errorf("Acked-by count = %d, want 0", ackedCount)
		}
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

		if testedCount != 1 {
			t.Errorf("Tested-by count = %d, want 1", testedCount)
		}
	})
}

func TestInlinePatchVariants(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	t.Run("signature stripped", func(t *testing.T) {
		body := "Test comment\nmore comment\n-- \nsig\n" + sampleDiff
		err := parseEmail(t, ctx, database, body, withListID("test.example.com"))
		if err != nil {
			t.Fatal(err)
		}
		if countPatches(t, database) != 1 {
			t.Error("expected 1 patch")
		}
	})

	t.Run("update comment preserved", func(t *testing.T) {
		body := "Test comment\n---\nUpdate: test update\n" + sampleDiff
		err := parseEmail(t, ctx, database, body, withListID("test.example.com"))
		if err != nil {
			t.Fatal(err)
		}
		if countPatches(t, database) != 2 {
			t.Error("expected 2 patches")
		}
	})

	t.Run("list footer stripped", func(t *testing.T) {
		body := "Test comment\n" + sampleDiff + "\n_______________________________________________\nfooter\n"
		err := parseEmail(t, ctx, database, body, withListID("test.example.com"))
		if err != nil {
			t.Fatal(err)
		}
		if countPatches(t, database) != 3 {
			t.Error("expected 3 patches")
		}
	})

	t.Run("diff word in comment", func(t *testing.T) {
		body := "This is a comment with the word differently in it\n" + sampleDiff
		err := parseEmail(t, ctx, database, body, withListID("test.example.com"))
		if err != nil {
			t.Fatal(err)
		}
		if countPatches(t, database) != 4 {
			t.Error("expected 4 patches")
		}
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
				strings.NewReader(data), "test.example.com")
			if err != nil {
				t.Fatal(err)
			}
		})
	}
	if countPatches(t, database) != 2 {
		t.Errorf("expected 2 patches from attachments, got %d",
			countPatches(t, database))
	}
}

func TestSubjectEncoding(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	t.Run("ascii", func(t *testing.T) {
		parseEmail(t, ctx, database, sampleDiff,
			withSubject("[PATCH] ascii subject test"),
			withListID("test.example.com"))
		if countPatches(t, database) != 1 {
			t.Error("expected 1 patch")
		}
	})

	t.Run("utf8 quoted-printable", func(t *testing.T) {
		parseEmail(t, ctx, database, sampleDiff,
			withSubject("=?utf-8?q?[PATCH]_=C3=A9_encoded?="),
			withListID("test.example.com"))
		if countPatches(t, database) != 2 {
			t.Error("expected 2 patches")
		}
	})
}

func TestSubjectEncodingMultipleWords(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	parseEmail(t, ctx, database, sampleDiff,
		withSubject("=?utf-8?q?[PATCH]_first?= =?utf-8?q?_second?="),
		withListID("test.example.com"))
	if countPatches(t, database) != 1 {
		t.Error("expected 1 patch for multi-word encoded subject")
	}
	var name string
	database.NewSelect().TableExpr("patch").
		Column("name").Limit(1).
		Scan(context.Background(), &name)
	if name == "" {
		t.Error("patch name should not be empty")
	}
}

func TestFindMessageID(t *testing.T) {
	t.Run("missing header", func(t *testing.T) {
		data := "From: test@example.com\r\nSubject: test\r\n\r\nbody\r\n"
		database, ctx, _, _ := testDB(t, "test.example.com")
		_ = ParseMail(ctx, database,
			strings.NewReader(data), "test.example.com")
	})

	t.Run("header with comments", func(t *testing.T) {
		database, ctx, _, _ := testDB(t, "test.example.com")
		parseEmail(t, ctx, database, sampleDiff,
			withMsgID("<test-id@example.com> (comment)"),
			withListID("test.example.com"))
		if countPatches(t, database) != 1 {
			t.Error("expected 1 patch despite comment in Message-ID")
		}
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
		strings.NewReader(data), "test.example.com")
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
