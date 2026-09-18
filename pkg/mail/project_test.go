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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		require.NoError(t, err)
		assert.Equal(t, 1, countPatches(t, database))
	})

	t.Run("keyword match", func(t *testing.T) {
		err := parseEmail(t, ctx, database, sampleDiff,
			withSubject("[PATCH keyword] subsystem"),
			withListID(listid))
		require.NoError(t, err)
		assert.Equal(t, 2, countPatches(t, database))
	})

	t.Run("default project", func(t *testing.T) {
		err := parseEmail(t, ctx, database, sampleDiff,
			withSubject("[PATCH unknown project]"),
			withListID(listid))
		require.NoError(t, err)
		assert.Equal(t, 3, countPatches(t, database))
	})

	t.Run("nonexistent project", func(t *testing.T) {
		err := parseEmail(t, ctx, database, sampleDiff,
			withSubject("[PATCH] test"),
			withListID("nonexistent.test.org"))
		require.NoError(t, err)
		assert.Equal(t, 3, countPatches(t, database), "should still be 3")
	})
}

func TestSubjectMatchListIDOverride(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")
	testProject(t, database, "keyword", "keyword", "test-subject-match.test.org", "keyword")

	data := createEmail(sampleDiff,
		withSubject("[PATCH keyword] test"),
		withListID("nonexistent.test.org"))
	err := ParseMail(ctx, database, bytes.NewReader(data), false,
		"test-subject-match.test.org")
	require.NoError(t, err)
	assert.Equal(t, 1, countPatches(t, database), "expected 1 patch with listid override")
}

func TestListIdHeader(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	t.Run("no list id", func(t *testing.T) {
		err := parseEmail(t, ctx, database, sampleDiff,
			withListID(""))
		require.NoError(t, err)
		assert.Equal(t, 0, countPatches(t, database))
	})

	t.Run("valid list id", func(t *testing.T) {
		err := parseEmail(t, ctx, database, sampleDiff,
			withListID("test.example.com"))
		require.NoError(t, err)
		assert.Equal(t, 1, countPatches(t, database))
	})
}

func TestListIdHeaderVariants(t *testing.T) {
	listid := "test.example.com"
	database, ctx, _, _ := testDB(t, listid)

	t.Run("blank list id", func(t *testing.T) {
		data := createEmail(sampleDiff, withListID(""))
		err := ParseMail(ctx, database, bytes.NewReader(data), false)
		require.NoError(t, err)
		assert.Equal(t, 0, countPatches(t, database), "expected 0 patches for blank list-id")
	})

	t.Run("substring list id", func(t *testing.T) {
		data := createEmail(sampleDiff, withListID("example.com"))
		err := ParseMail(ctx, database, bytes.NewReader(data), false)
		require.NoError(t, err)
		assert.Equal(t, 0, countPatches(t, database), "expected 0 patches for substring match")
	})

	t.Run("short list id", func(t *testing.T) {
		data := createEmail(sampleDiff)
		raw := strings.Replace(string(data), "List-Id: <test.example.com>", "List-Id: test.example.com", 1)
		err := ParseMail(ctx, database, strings.NewReader(raw), false)
		require.NoError(t, err)
		assert.Equal(t, 1, countPatches(t, database), "expected 1 patch for short list-id")
	})

	t.Run("long list id", func(t *testing.T) {
		data := createEmail(sampleDiff)
		raw := strings.Replace(string(data), "List-Id: <test.example.com>",
			"List-Id: Test text <test.example.com>", 1)
		err := ParseMail(ctx, database, strings.NewReader(raw), false)
		require.NoError(t, err)
		assert.Equal(t, 2, countPatches(t, database), "expected 2 patches for long list-id")
	})
}

func TestListIdWhitespace(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	data := createEmail(sampleDiff)
	raw := strings.Replace(string(data), "List-Id: <test.example.com>",
		"List-Id:  ", 1)
	ParseMail(ctx, database, strings.NewReader(raw), false)
	assert.Equal(t, 0, countPatches(t, database), "expected 0 patches for whitespace list-id")
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

	require.Equal(t, 2, countPatches(t, database))
	var countA, countB int
	database.NewSelect().TableExpr("patch").
		ColumnExpr("count(*)").
		Where("project_id = (SELECT id FROM project WHERE linkname = 'test-project')").
		Scan(context.Background(), &countA)
	database.NewSelect().TableExpr("patch").
		ColumnExpr("count(*)").
		Where("project_id = (SELECT id FROM project WHERE linkname = 'project-b')").
		Scan(context.Background(), &countB)
	assert.Equal(t, 1, countA, "project-a")
	assert.Equal(t, 1, countB, "project-b")
}
