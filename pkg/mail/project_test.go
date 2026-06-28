// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package mail

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestSubjectMatch(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")
	listid := "test-subject-match.test.org"
	testProject(t, database, "project-x", "PROJECT X", listid, `.*PROJECT[\s]?X.*`)
	testProject(t, database, "default", "Default", listid, "")
	testProject(t, database, "keyword", "keyword", listid, "keyword")

	t.Run("regex match", func(t *testing.T) {
		err := parseEmail(t, ctx, database, sampleDiff,
			withSubject("[PATCH PROJECT X subsystem] test"),
			withListID(listid))
		if err != nil {
			t.Fatal(err)
		}
		if countPatches(t, database) != 1 {
			t.Error("expected 1 patch")
		}
	})

	t.Run("keyword match", func(t *testing.T) {
		err := parseEmail(t, ctx, database, sampleDiff,
			withSubject("[PATCH keyword] subsystem"),
			withListID(listid))
		if err != nil {
			t.Fatal(err)
		}
		if countPatches(t, database) != 2 {
			t.Error("expected 2 patches")
		}
	})

	t.Run("default project", func(t *testing.T) {
		err := parseEmail(t, ctx, database, sampleDiff,
			withSubject("[PATCH unknown project]"),
			withListID(listid))
		if err != nil {
			t.Fatal(err)
		}
		if countPatches(t, database) != 3 {
			t.Error("expected 3 patches")
		}
	})

	t.Run("nonexistent project", func(t *testing.T) {
		err := parseEmail(t, ctx, database, sampleDiff,
			withSubject("[PATCH] test"),
			withListID("nonexistent.test.org"))
		if err != nil {
			t.Fatal(err)
		}
		if countPatches(t, database) != 3 {
			t.Error("should still be 3")
		}
	})
}

func TestSubjectMatchListIDOverride(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")
	testProject(t, database, "keyword", "keyword", "test-subject-match.test.org", "keyword")

	data := createEmail(sampleDiff,
		withSubject("[PATCH keyword] test"),
		withListID("nonexistent.test.org"))
	err := ParseMail(ctx, database, bytes.NewReader(data),
		"test-subject-match.test.org")
	if err != nil {
		t.Fatal(err)
	}
	if countPatches(t, database) != 1 {
		t.Error("expected 1 patch with listid override")
	}
}

func TestListIdHeader(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	t.Run("no list id", func(t *testing.T) {
		err := parseEmail(t, ctx, database, sampleDiff,
			withListID(""))
		if err != nil {
			t.Fatal(err)
		}
		if countPatches(t, database) != 0 {
			t.Error("expected 0 patches")
		}
	})

	t.Run("valid list id", func(t *testing.T) {
		err := parseEmail(t, ctx, database, sampleDiff,
			withListID("test.example.com"))
		if err != nil {
			t.Fatal(err)
		}
		if countPatches(t, database) != 1 {
			t.Error("expected 1 patch")
		}
	})
}

func TestListIdHeaderVariants(t *testing.T) {
	listid := "test.example.com"
	database, ctx, _, _ := testDB(t, listid)

	t.Run("blank list id", func(t *testing.T) {
		data := createEmail(sampleDiff, withListID(""))
		err := ParseMail(ctx, database, bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if countPatches(t, database) != 0 {
			t.Error("expected 0 patches for blank list-id")
		}
	})

	t.Run("substring list id", func(t *testing.T) {
		data := createEmail(sampleDiff, withListID("example.com"))
		err := ParseMail(ctx, database, bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if countPatches(t, database) != 0 {
			t.Error("expected 0 patches for substring match")
		}
	})

	t.Run("short list id", func(t *testing.T) {
		data := createEmail(sampleDiff)
		raw := strings.Replace(string(data), "List-Id: <test.example.com>", "List-Id: test.example.com", 1)
		err := ParseMail(ctx, database, strings.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		if countPatches(t, database) != 1 {
			t.Error("expected 1 patch for short list-id")
		}
	})

	t.Run("long list id", func(t *testing.T) {
		data := createEmail(sampleDiff)
		raw := strings.Replace(string(data), "List-Id: <test.example.com>",
			"List-Id: Test text <test.example.com>", 1)
		err := ParseMail(ctx, database, strings.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		if countPatches(t, database) != 2 {
			t.Error("expected 2 patches for long list-id")
		}
	})
}

func TestListIdWhitespace(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	data := createEmail(sampleDiff)
	raw := strings.Replace(string(data), "List-Id: <test.example.com>",
		"List-Id:  ", 1)
	ParseMail(ctx, database, strings.NewReader(raw))
	if countPatches(t, database) != 0 {
		t.Error("expected 0 patches for whitespace list-id")
	}
}

func TestMultipleProjects(t *testing.T) {
	database, ctx, _, _ := testDB(t, "project-a.example.com")
	testProject(t, database, "project-b", "Project B", "project-b.example.com", "")

	parseEmail(t, ctx, database, sampleDiff,
		withSubject("[PATCH] test"),
		withListID("project-a.example.com"))
	parseEmail(t, ctx, database, sampleDiff,
		withSubject("[PATCH] test"),
		withListID("project-b.example.com"))

	if countPatches(t, database) != 2 {
		t.Fatalf("expected 2 patches (one per project), got %d",
			countPatches(t, database))
	}
	var countA, countB int
	database.NewSelect().TableExpr("patch").
		ColumnExpr("count(*)").
		Where("project_id = (SELECT id FROM project WHERE linkname = 'test-project')").
		Scan(context.Background(), &countA)
	database.NewSelect().TableExpr("patch").
		ColumnExpr("count(*)").
		Where("project_id = (SELECT id FROM project WHERE linkname = 'project-b')").
		Scan(context.Background(), &countB)
	if countA != 1 {
		t.Errorf("project-a: got %d patches, want 1", countA)
	}
	if countB != 1 {
		t.Errorf("project-b: got %d patches, want 1", countB)
	}
}
