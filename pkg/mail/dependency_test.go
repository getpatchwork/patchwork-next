// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package mail

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/emersion/go-mbox"
)

func TestParseDependsOn(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	baseMsgID := "<base-patch@test>"
	parseEmail(t, ctx, database, sampleDiff,
		withMsgID(baseMsgID),
		withSubject("[PATCH 1/1] base patch"),
		withListID("test.example.com"))

	t.Run("garbage ref ignored", func(t *testing.T) {
		parseEmail(t, ctx, database, sampleDiff+"\nDepends-on: not-a-valid-ref\n",
			withSubject("[PATCH 1/1] dep patch"),
			withListID("test.example.com"))

		var depCount int
		database.NewSelect().TableExpr("series_dependencies").
			ColumnExpr("count(*)").
			Scan(context.Background(), &depCount)
		if depCount != 0 {
			t.Errorf("expected 0 dependencies, got %d", depCount)
		}
	})

	t.Run("msgid dependency", func(t *testing.T) {
		var baseSeries sql.NullInt32
		database.NewSelect().TableExpr("patch").
			Column("series_id").Where("msgid = ?", baseMsgID).
			Scan(context.Background(), &baseSeries)
		if !baseSeries.Valid {
			t.Fatalf("base patch has no series (series_id=%v)", baseSeries)
		}

		parseEmail(t, ctx, database, sampleDiff+"\nDepends-on: "+baseMsgID+"\n",
			withSubject("[PATCH 1/1] dependent patch"),
			withListID("test.example.com"))

		var depCount int
		database.NewSelect().TableExpr("series_dependencies").
			ColumnExpr("count(*)").
			Scan(context.Background(), &depCount)
		if depCount != 1 {
			t.Errorf("expected 1 dependency, got %d", depCount)
		}
	})
}

func TestDependencyByMsgIDOnCover(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	baseMsgID := "<dep-base@test>"
	parseEmail(t, ctx, database, sampleDiff,
		withMsgID(baseMsgID),
		withSubject("[PATCH 1/1] base"),
		withListID("test.example.com"))

	parseEmail(t, ctx, database, "cover body\n\nDepends-on: "+baseMsgID+"\n",
		withSubject("[PATCH 0/2] dependent series"),
		withMsgID("<dep-cover@test>"),
		withListID("test.example.com"))

	parseEmail(t, ctx, database, sampleDiff,
		withSubject("[PATCH 1/2] dependent patch 1"),
		withMsgID("<dep-patch-1@test>"),
		withInReplyTo("<dep-cover@test>"),
		withListID("test.example.com"))
	parseEmail(t, ctx, database, sampleDiff,
		withSubject("[PATCH 2/2] dependent patch 2"),
		withMsgID("<dep-patch-2@test>"),
		withInReplyTo("<dep-cover@test>"),
		withListID("test.example.com"))

	var depCount int
	database.NewSelect().TableExpr("series_dependencies").
		ColumnExpr("count(*)").
		Scan(context.Background(), &depCount)
	if depCount != 1 {
		t.Errorf("expected 1 dependency, got %d", depCount)
	}
}

func TestDependencyCircularPrevention(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	msgid := "<self-dep@test>"
	parseEmail(t, ctx, database, sampleDiff+"\nDepends-on: "+msgid+"\n",
		withMsgID(msgid),
		withSubject("[PATCH 1/1] self ref"),
		withListID("test.example.com"))

	assertDependencyCount(t, database, 0)
}

func TestDependencyCrossProjectPrevention(t *testing.T) {
	database, ctx, _, _ := testDB(t, "project-a.example.com")
	testProject(t, database, "project-b", "Project B", "project-b.example.com", "")

	baseMsgID := "<cross-base@test>"
	parseEmail(t, ctx, database, sampleDiff,
		withMsgID(baseMsgID),
		withSubject("[PATCH 1/1] base"),
		withListID("project-a.example.com"))

	parseEmail(t, ctx, database, sampleDiff+"\nDepends-on: "+baseMsgID+"\n",
		withSubject("[PATCH 1/1] dependent"),
		withListID("project-b.example.com"))

	assertDependencyCount(t, database, 0)
}

func TestDependencyBySeriesURL(t *testing.T) {
	database, ctx, _, proj := testDB(t, "test.example.com")

	parseMbox(t, ctx, database, "series/dependency-base-patch.mbox", "test.example.com")

	var seriesID int
	database.NewSelect().TableExpr("series").
		Column("id").Limit(1).
		Scan(context.Background(), &seriesID)

	seriesURL := fmt.Sprintf("http://test/project/%s/list/?series=%d",
		proj.Linkname, seriesID)

	parseMboxTemplate(t, ctx, database, "series/dependency-one-cover.mbox.template",
		seriesURL, "test.example.com")

	assertDependencyCount(t, database, 1)
}

func TestDependencyByPatchURL(t *testing.T) {
	database, ctx, _, proj := testDB(t, "test.example.com")

	parseMbox(t, ctx, database, "series/dependency-base-patch.mbox", "test.example.com")

	var patchMsgID string
	database.NewSelect().TableExpr("patch").
		Column("msgid").OrderExpr("id").Limit(1).
		Scan(context.Background(), &patchMsgID)
	urlMsgID := strings.TrimPrefix(strings.TrimSuffix(patchMsgID, ">"), "<")

	patchURL := fmt.Sprintf("http://test/project/%s/patch/%s/",
		proj.Linkname, urlMsgID)

	parseMboxTemplate(t, ctx, database, "series/dependency-one-cover.mbox.template",
		patchURL, "test.example.com")

	assertDependencyCount(t, database, 1)
}

func TestDependencyByPatchMsgID(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	parseMbox(t, ctx, database, "series/dependency-base-patch.mbox", "test.example.com")

	var patchMsgID string
	database.NewSelect().TableExpr("patch").
		Column("msgid").OrderExpr("id").Limit(1).
		Scan(context.Background(), &patchMsgID)

	parseMboxTemplate(t, ctx, database, "series/dependency-one-first-patch.mbox.template",
		patchMsgID, "test.example.com")

	assertDependencyCount(t, database, 1)
}

func TestDependencyByCoverMsgID(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	parseMbox(t, ctx, database, "series/dependency-base-patch.mbox", "test.example.com")

	var coverMsgID string
	database.NewSelect().TableExpr("cover").
		Column("msgid").Limit(1).
		Scan(context.Background(), &coverMsgID)

	parseMboxTemplate(t, ctx, database, "series/dependency-one-first-patch.mbox.template",
		coverMsgID, "test.example.com")

	assertDependencyCount(t, database, 1)
}

func TestDependencyMulti(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	parseMbox(t, ctx, database, "series/dependency-base-patch.mbox", "test.example.com")

	var basePatchMsgID string
	database.NewSelect().TableExpr("patch").
		Column("msgid").OrderExpr("id").Limit(1).
		Scan(context.Background(), &basePatchMsgID)

	parseMboxTemplate(t, ctx, database, "series/dependency-one-first-patch.mbox.template",
		basePatchMsgID, "test.example.com")

	var series2PatchMsgID string
	database.NewSelect().TableExpr("patch").
		Column("msgid").OrderExpr("id DESC").Limit(1).
		Scan(context.Background(), &series2PatchMsgID)

	data, err := os.ReadFile("testdata/series/dependency-multi.mbox.template")
	if err != nil {
		t.Fatal(err)
	}
	expanded := strings.ReplaceAll(string(data), "{depends_token_1}", basePatchMsgID)
	expanded = strings.ReplaceAll(expanded, "{depends_token_2}", series2PatchMsgID)

	r := mbox.NewReader(strings.NewReader(expanded))
	for {
		msg, err := r.NextMessage()
		if err != nil {
			break
		}
		buf, _ := io.ReadAll(msg)
		ParseMail(ctx, database, bytes.NewReader(buf), "test.example.com")
	}

	var depCount int
	database.NewSelect().TableExpr("series_dependencies").
		ColumnExpr("count(*)").
		Scan(context.Background(), &depCount)
	if depCount < 3 {
		t.Errorf("expected at least 3 dependencies, got %d", depCount)
	}
}

func TestDependencyByPatch2MsgID(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	parseMbox(t, ctx, database, "series/dependency-base-patch.mbox", "test.example.com")

	var patch2MsgID string
	database.NewRaw(
		"SELECT msgid FROM patch ORDER BY id LIMIT 1 OFFSET 1",
	).Scan(context.Background(), &patch2MsgID)
	if patch2MsgID == "" {
		t.Fatal("no second patch found in base series")
	}

	parseMboxTemplate(t, ctx, database, "series/dependency-one-first-patch.mbox.template",
		patch2MsgID, "test.example.com")

	assertDependencyCount(t, database, 1)
}

func TestDependencyMulti2(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	parseMbox(t, ctx, database, "series/dependency-base-patch.mbox", "test.example.com")

	var basePatchMsgID string
	database.NewRaw(
		"SELECT msgid FROM patch ORDER BY id LIMIT 1",
	).Scan(context.Background(), &basePatchMsgID)

	parseMboxTemplate(t, ctx, database, "series/dependency-one-first-patch.mbox.template",
		basePatchMsgID, "test.example.com")

	var series2PatchMsgID string
	database.NewRaw(
		"SELECT msgid FROM patch ORDER BY id DESC LIMIT 1",
	).Scan(context.Background(), &series2PatchMsgID)

	data, _ := os.ReadFile("testdata/series/dependency-multi-2.mbox.template")
	expanded := strings.ReplaceAll(string(data), "{depends_token_1}", basePatchMsgID)
	expanded = strings.ReplaceAll(expanded, "{depends_token_2}", series2PatchMsgID)

	r := mbox.NewReader(strings.NewReader(expanded))
	for {
		msg, err := r.NextMessage()
		if err != nil {
			break
		}
		buf, _ := io.ReadAll(msg)
		ParseMail(ctx, database, bytes.NewReader(buf), "test.example.com")
	}

	var depCount int
	database.NewSelect().TableExpr("series_dependencies").
		ColumnExpr("count(*)").Scan(context.Background(), &depCount)
	if depCount < 3 {
		t.Errorf("expected at least 3 dependencies, got %d", depCount)
	}
}

func TestParseDependsOnGarbageRef2(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	parseEmail(t, ctx, database, sampleDiff+"\nDepends-on: some random text\n",
		withSubject("[PATCH 1/1] test"),
		withListID("test.example.com"))

	assertDependencyCount(t, database, 0)
}

func TestParseDependsOnGarbageURL(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	parseEmail(t, ctx, database, sampleDiff+"\nDepends-on: http://not-a-patchwork-url.com/garbage\n",
		withSubject("[PATCH 1/1] test"),
		withListID("test.example.com"))

	assertDependencyCount(t, database, 0)
}

func TestParseDependsOnURL(t *testing.T) {
	database, ctx, _, proj := testDB(t, "test.example.com")

	baseMsgID := "<url-dep-base@test>"
	parseEmail(t, ctx, database, sampleDiff,
		withMsgID(baseMsgID),
		withSubject("[PATCH 1/1] base"),
		withListID("test.example.com"))

	var seriesID int
	database.NewRaw("SELECT id FROM series LIMIT 1").
		Scan(context.Background(), &seriesID)

	seriesURL := fmt.Sprintf("http://pw.example.com/project/%s/list/?series=%d",
		proj.Linkname, seriesID)

	parseEmail(t, ctx, database, sampleDiff+"\nDepends-on: "+seriesURL+"\n",
		withSubject("[PATCH 1/1] dependent"),
		withListID("test.example.com"))

	assertDependencyCount(t, database, 1)
}
