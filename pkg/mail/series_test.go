// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package mail

import (
	"context"
	"testing"
)

func TestBaseSeriesSinglePatch(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/base-single-patch.mbox", "test.example.com")
	if result.patches != 1 {
		t.Errorf("patches: got %d, want 1", result.patches)
	}
	if result.series != 1 {
		t.Errorf("series: got %d, want 1", result.series)
	}
	assertAllPatchesHaveSeries(t, database)
}

func TestBaseSeriesCoverLetter(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/base-cover-letter.mbox", "test.example.com")
	if result.patches != 2 {
		t.Errorf("patches: got %d, want 2", result.patches)
	}
	if result.covers != 1 {
		t.Errorf("covers: got %d, want 1", result.covers)
	}
	if result.series != 1 {
		t.Errorf("series: got %d, want 1", result.series)
	}
	assertAllPatchesInOneSeries(t, database)
	assertCoverLinkedToSeries(t, database)
}

func TestBaseSeriesNoCoverLetter(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/base-no-cover-letter.mbox", "test.example.com")
	if result.patches != 2 {
		t.Errorf("patches: got %d, want 2", result.patches)
	}
	if result.covers != 0 {
		t.Errorf("covers: got %d, want 0", result.covers)
	}
	if result.series != 1 {
		t.Errorf("series: got %d, want 1", result.series)
	}
	assertSerialized(t, database, []int{2})
}

func TestBaseSeriesDeepThreaded(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/base-deep-threaded.mbox", "test.example.com")
	if result.patches != 2 {
		t.Errorf("patches: got %d, want 2", result.patches)
	}
	if result.covers != 1 {
		t.Errorf("covers: got %d, want 1", result.covers)
	}
	if result.series != 1 {
		t.Errorf("series: got %d, want 1", result.series)
	}
}

func TestBaseSeriesOutOfOrder(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/base-out-of-order.mbox", "test.example.com")
	if result.patches != 2 {
		t.Errorf("patches: got %d, want 2", result.patches)
	}
	if result.covers != 1 {
		t.Errorf("covers: got %d, want 1", result.covers)
	}
	if result.series != 1 {
		t.Errorf("series: got %d, want 1", result.series)
	}
}

func TestBaseSeriesIncomplete(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/base-incomplete.mbox", "test.example.com")
	if result.patches != 1 {
		t.Errorf("patches: got %d, want 1", result.patches)
	}
	if result.covers != 1 {
		t.Errorf("covers: got %d, want 1", result.covers)
	}
	if result.series != 1 {
		t.Errorf("series: got %d, want 1", result.series)
	}
}

func TestBaseSeriesDifferentVersions(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/base-different-versions.mbox", "test.example.com")
	if result.covers != 1 {
		t.Errorf("covers: got %d, want 1", result.covers)
	}
	if result.patches != 4 {
		t.Errorf("patches: got %d, want 4", result.patches)
	}
}

func TestBaseSeriesNoReferences(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/base-no-references.mbox", "test.example.com")
	if result.patches != 2 {
		t.Errorf("patches: got %d, want 2", result.patches)
	}
}

func TestBaseSeriesNoReferencesNoCover(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/base-no-references-no-cover.mbox", "test.example.com")
	if result.covers != 1 {
		t.Errorf("covers: got %d, want 1", result.covers)
	}
	if result.patches != 2 {
		t.Errorf("patches: got %d, want 2", result.patches)
	}
}

func TestBaseSeriesExtraPatches(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/base-extra-patches.mbox", "test.example.com")
	if result.covers != 1 {
		t.Errorf("covers: got %d, want 1", result.covers)
	}
	if result.patches != 3 {
		t.Errorf("patches: got %d, want 3", result.patches)
	}
}

func TestBugsMultipleReferences(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/bugs-multiple-references.mbox", "test.example.com")
	if result.covers != 1 {
		t.Errorf("covers: got %d, want 1", result.covers)
	}
	if result.patches != 4 {
		t.Errorf("patches: got %d, want 4", result.patches)
	}
}

func TestBugsMultipleContentTypes(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/bugs-multiple-content-types.mbox", "test.example.com")
	if result.patches != 1 {
		t.Errorf("patches: got %d, want 1", result.patches)
	}
	if result.patchComments != 1 {
		t.Errorf("patchComments: got %d, want 1", result.patchComments)
	}
}

func TestBugsNocover(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/bugs-nocover.mbox", "test.example.com")
	if result.patches != 4 {
		t.Errorf("patches: got %d, want 4", result.patches)
	}
}

func TestBugsNocoverNoversion(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/bugs-nocover-noversion.mbox", "test.example.com")
	if result.patches != 4 {
		t.Errorf("patches: got %d, want 4", result.patches)
	}
}

func TestBugsSpamming(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/bugs-spamming.mbox", "test.example.com")
	if result.patches != 3 {
		t.Errorf("patches: got %d, want 3", result.patches)
	}
}

func TestBugsUnnumbered(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/bugs-unnumbered.mbox", "test.example.com")
	if result.covers != 1 {
		t.Errorf("covers: got %d, want 1", result.covers)
	}
	if result.patches != 2 {
		t.Errorf("patches: got %d, want 2", result.patches)
	}
}

func TestBugsMixedVersions(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/bugs-mixed-versions.mbox", "test.example.com")
	if result.patches != 2 {
		t.Errorf("patches: got %d, want 2", result.patches)
	}
}

func TestRevisionBasic(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/revision-basic.mbox", "test.example.com")
	if result.patches != 4 {
		t.Errorf("patches: got %d, want 4", result.patches)
	}
	if result.covers != 2 {
		t.Errorf("covers: got %d, want 2", result.covers)
	}
	if result.series != 2 {
		t.Errorf("series: got %d, want 2", result.series)
	}
	assertSerialized(t, database, []int{2, 2})
}

func TestRevisionThreadedToSinglePatch(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/revision-threaded-to-single-patch.mbox", "test.example.com")
	if result.patches != 2 {
		t.Errorf("patches: got %d, want 2", result.patches)
	}
}

func TestRevisionThreadedToCover(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/revision-threaded-to-cover.mbox", "test.example.com")
	if result.covers != 2 {
		t.Errorf("covers: got %d, want 2", result.covers)
	}
	if result.patches != 4 {
		t.Errorf("patches: got %d, want 4", result.patches)
	}
	assertSerialized(t, database, []int{2, 2})
}

func TestRevisionThreadedToPatch(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/revision-threaded-to-patch.mbox", "test.example.com")
	if result.covers != 2 {
		t.Errorf("covers: got %d, want 2", result.covers)
	}
	if result.patches != 4 {
		t.Errorf("patches: got %d, want 4", result.patches)
	}
	assertSerialized(t, database, []int{2, 2})
}

func TestRevisionOutOfOrder(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/revision-out-of-order.mbox", "test.example.com")
	if result.covers != 2 {
		t.Errorf("covers: got %d, want 2", result.covers)
	}
	if result.patches != 4 {
		t.Errorf("patches: got %d, want 4", result.patches)
	}
	assertSerialized(t, database, []int{2, 2})
}

func TestRevisionNoCoverLetter(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/revision-no-cover-letter.mbox", "test.example.com")
	if result.covers != 1 {
		t.Errorf("covers: got %d, want 1", result.covers)
	}
	if result.patches != 4 {
		t.Errorf("patches: got %d, want 4", result.patches)
	}
	assertSerialized(t, database, []int{2, 2})
}

func TestRevisionUnlabeled(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/revision-unlabeled.mbox", "test.example.com")
	if result.covers != 2 {
		t.Errorf("covers: got %d, want 2", result.covers)
	}
	if result.patches != 4 {
		t.Errorf("patches: got %d, want 4", result.patches)
	}
	assertSerialized(t, database, []int{2, 2})
}

func TestRevisionUnlabeledNoreferences(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/revision-unlabeled-noreferences.mbox", "test.example.com")
	if result.patches != 4 {
		t.Errorf("patches: got %d, want 4", result.patches)
	}
	assertSerialized(t, database, []int{2, 2})
}

func TestRevisedSeriesReplyNocover(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/bugs-nocover.mbox", "test.example.com")
	if result.patches != 4 {
		t.Errorf("patches: got %d, want 4", result.patches)
	}
	if result.series < 2 {
		t.Errorf("series: got %d, want >= 2", result.series)
	}
}

func TestRevisedSeriesReplyNocoverNoversion(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/bugs-nocover-noversion.mbox", "test.example.com")
	if result.patches != 4 {
		t.Errorf("patches: got %d, want 4", result.patches)
	}
}

func TestMercurialCoverLetter(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/mercurial-cover-letter.mbox", "test.example.com")
	if result.patches != 2 {
		t.Errorf("patches: got %d, want 2", result.patches)
	}
	if result.covers != 1 {
		t.Errorf("covers: got %d, want 1", result.covers)
	}
	if result.series != 1 {
		t.Errorf("series: got %d, want 1", result.series)
	}
}

func TestMercurialNoCoverLetter(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/mercurial-no-cover-letter.mbox", "test.example.com")
	if result.patches != 2 {
		t.Errorf("patches: got %d, want 2", result.patches)
	}
	if result.series != 1 {
		t.Errorf("series: got %d, want 1", result.series)
	}
}

func TestSeriesCorrelation(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	t.Run("new series", func(t *testing.T) {
		parseEmail(t, ctx, database, sampleDiff,
			withSubject("[PATCH 1/2] first patch"),
			withMsgID("<series-1-patch-1@test>"),
			withListID("test.example.com"))
		if countPatches(t, database) != 1 {
			t.Error("expected 1 patch")
		}

		var seriesCount int
		database.NewSelect().TableExpr("series").
			ColumnExpr("count(*)").
			Scan(context.Background(), &seriesCount)
		if seriesCount != 1 {
			t.Errorf("expected 1 series, got %d", seriesCount)
		}
	})

	t.Run("reply joins series", func(t *testing.T) {
		parseEmail(t, ctx, database, sampleDiff,
			withSubject("[PATCH 2/2] second patch"),
			withMsgID("<series-1-patch-2@test>"),
			withInReplyTo("<series-1-patch-1@test>"),
			withListID("test.example.com"))
		if countPatches(t, database) != 2 {
			t.Fatal("expected 2 patches")
		}

		var seriesCount int
		database.NewSelect().TableExpr("series").
			ColumnExpr("count(*)").
			Scan(context.Background(), &seriesCount)
		if seriesCount != 1 {
			t.Fatalf("expected still 1 series, got %d", seriesCount)
		}

		assertAllPatchesInOneSeries(t, database)
	})
}

func TestSeriesName(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	t.Run("cover letter sets name", func(t *testing.T) {
		result := parseMbox(t, ctx, database, "series/base-cover-letter.mbox", "test.example.com")
		if result.series != 1 {
			t.Fatal("expected 1 series")
		}

		var name string
		database.NewSelect().TableExpr("series").
			Column("name").Limit(1).
			Scan(context.Background(), &name)
		if name == "" {
			t.Error("series name should be set from cover letter")
		}
	})
}

func TestSeriesNameCoverLetter(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/base-cover-letter.mbox", "test.example.com")
	if result.series != 1 {
		t.Fatal("expected 1 series")
	}

	var name string
	database.NewSelect().TableExpr("series").
		Column("name").Limit(1).
		Scan(context.Background(), &name)
	if name == "" {
		t.Error("series name should be set from cover letter")
	}
	if name != "A sample series" {
		t.Errorf("series name = %q, want %q", name, "A sample series")
	}
}

func TestSeriesNameNoCoverLetter(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/base-no-cover-letter.mbox", "test.example.com")
	if result.series != 1 {
		t.Fatal("expected 1 series")
	}

	var seriesName, firstPatchName string
	database.NewSelect().TableExpr("series").
		Column("name").Limit(1).
		Scan(context.Background(), &seriesName)
	database.NewSelect().TableExpr("patch").
		Column("name").Where("number = 1").Limit(1).
		Scan(context.Background(), &firstPatchName)
	if seriesName == "" {
		t.Error("series name should be set from first patch")
	}
	if seriesName != firstPatchName {
		t.Errorf("series name = %q, want first patch name %q", seriesName, firstPatchName)
	}
}

func TestSeriesNameOutOfOrder(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/base-out-of-order.mbox", "test.example.com")
	if result.series != 1 {
		t.Fatal("expected 1 series")
	}

	var name string
	database.NewSelect().TableExpr("series").
		Column("name").Limit(1).
		Scan(context.Background(), &name)
	if name == "" {
		t.Error("series name should be set")
	}
}

func TestSeriesNameCustom(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/base-cover-letter.mbox", "test.example.com")
	if result.series != 1 {
		t.Fatal("expected 1 series")
	}

	database.NewRaw("UPDATE series SET name = 'Custom Name'").
		Exec(context.Background())

	var name string
	database.NewSelect().TableExpr("series").
		Column("name").Limit(1).
		Scan(context.Background(), &name)
	if name != "Custom Name" {
		t.Errorf("series name = %q, want Custom Name", name)
	}
}

func TestSeriesTotalComplete(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/base-cover-letter.mbox", "test.example.com")
	if result.patches != 2 {
		t.Errorf("patches: got %d, want 2", result.patches)
	}

	var total, patchCount int
	database.NewSelect().TableExpr("series").
		Column("total").Limit(1).
		Scan(context.Background(), &total)
	database.NewSelect().TableExpr("patch").
		ColumnExpr("count(*)").Where("series_id IS NOT NULL").
		Scan(context.Background(), &patchCount)
	if patchCount < total {
		t.Errorf("series not complete: %d/%d patches", patchCount, total)
	}
}

func TestSeriesReceivedAll(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	t.Run("complete", func(t *testing.T) {
		result := parseMbox(t, ctx, database, "series/base-cover-letter.mbox", "test.example.com")
		if result.series != 1 {
			t.Fatal("expected 1 series")
		}

		var total int
		var patchCount int
		database.NewSelect().TableExpr("series").
			Column("total").Limit(1).
			Scan(context.Background(), &total)
		database.NewSelect().TableExpr("patch").
			ColumnExpr("count(*)").Where("series_id IS NOT NULL").
			Scan(context.Background(), &patchCount)
		if int(patchCount) < total {
			t.Errorf("series not complete: %d/%d", patchCount, total)
		}
	})

	t.Run("incomplete", func(t *testing.T) {
		db2, ctx2, _, _ := testDB(t, "test.example.com")
		result := parseMbox(t, ctx2, db2, "series/base-incomplete.mbox", "test.example.com")
		if result.series != 1 {
			t.Fatal("expected 1 series")
		}

		var total int
		var patchCount int
		db2.NewSelect().TableExpr("series").
			Column("total").Limit(1).
			Scan(context.Background(), &total)
		db2.NewSelect().TableExpr("patch").
			ColumnExpr("count(*)").Where("series_id IS NOT NULL").
			Scan(context.Background(), &patchCount)
		if int(patchCount) >= total {
			t.Errorf("series should be incomplete: %d/%d", patchCount, total)
		}
	})
}

func TestNestedSeries(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	parseEmail(t, ctx, database, "cover body",
		withMsgID("<nested-v1-cover@test>"),
		withSubject("[PATCH 0/2] first series"),
		withListID("test.example.com"))
	parseEmail(t, ctx, database, sampleDiff,
		withMsgID("<nested-v1-p1@test>"),
		withSubject("[PATCH 1/2] first patch"),
		withInReplyTo("<nested-v1-cover@test>"),
		withListID("test.example.com"))
	parseEmail(t, ctx, database, sampleDiff,
		withMsgID("<nested-v1-p2@test>"),
		withSubject("[PATCH 2/2] second patch"),
		withInReplyTo("<nested-v1-cover@test>"),
		withListID("test.example.com"))

	parseEmail(t, ctx, database, "v2 cover body",
		withMsgID("<nested-v2-cover@test>"),
		withSubject("[PATCH v2 0/2] first series"),
		withInReplyTo("<nested-v1-cover@test>"),
		withListID("test.example.com"))
	parseEmail(t, ctx, database, sampleDiff,
		withMsgID("<nested-v2-p1@test>"),
		withSubject("[PATCH v2 1/2] first patch"),
		withInReplyTo("<nested-v2-cover@test>"),
		withListID("test.example.com"))

	var seriesCount int
	database.NewSelect().TableExpr("series").
		ColumnExpr("count(*)").
		Scan(context.Background(), &seriesCount)
	if seriesCount < 2 {
		t.Errorf("expected at least 2 series, got %d", seriesCount)
	}
}

func TestPreviousSeriesLinkage(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/revision-basic.mbox", "test.example.com")
	if result.series != 2 {
		t.Fatalf("expected 2 series, got %d", result.series)
	}

	var series []struct {
		ID               int
		Version          int
		PreviousSeriesID *int
	}
	database.NewSelect().TableExpr("series").
		Column("id", "version", "previous_series_id").
		OrderExpr("version").
		Scan(context.Background(), &series)

	if len(series) != 2 {
		t.Fatalf("expected 2 series rows, got %d", len(series))
	}

	v1 := series[0]
	v2 := series[1]

	if v1.Version != 1 {
		t.Errorf("first series version = %d, want 1", v1.Version)
	}
	if v2.Version != 2 {
		t.Errorf("second series version = %d, want 2", v2.Version)
	}

	if v2.PreviousSeriesID == nil {
		t.Error("v2 series should have previous_series_id set")
	} else if *v2.PreviousSeriesID != v1.ID {
		t.Errorf("v2 previous_series_id = %d, want %d", *v2.PreviousSeriesID, v1.ID)
	}

	if v1.PreviousSeriesID != nil {
		t.Error("v1 series should not have previous_series_id")
	}
}

func TestPreviousSeriesThreaded(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	result := parseMbox(t, ctx, database, "series/revision-threaded-to-cover.mbox", "test.example.com")
	if result.series != 2 {
		t.Fatalf("expected 2 series, got %d", result.series)
	}

	var count int
	database.NewSelect().TableExpr("series").
		ColumnExpr("count(*)").
		Where("previous_series_id IS NOT NULL").
		Scan(context.Background(), &count)
	if count != 1 {
		t.Errorf("expected 1 series with previous_series_id, got %d", count)
	}
}

func TestPreviousSeriesNameSimilarity(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	parseEmail(t, ctx, database, sampleDiff,
		withSubject("[PATCH] fix: improve error handling"),
		withMsgID("<v1-patch@test>"),
		withListID("test.example.com"))

	parseEmail(t, ctx, database, sampleDiff,
		withSubject("[PATCH v2] fix: improve error handling"),
		withMsgID("<v2-patch@test>"),
		withListID("test.example.com"))

	var count int
	database.NewSelect().TableExpr("series").
		ColumnExpr("count(*)").
		Where("previous_series_id IS NOT NULL").
		Scan(context.Background(), &count)
	if count != 1 {
		t.Errorf("expected 1 series with previous_series_id (name similarity), got %d", count)
	}
}

func TestCoverNamePriority(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	parseEmail(t, ctx, database, sampleDiff,
		withSubject("[PATCH 1/2] the patch name"),
		withMsgID("<covername-1@test>"),
		withListID("test.example.com"))

	patchName := "[1/2] the patch name"
	name := getSeriesName(t, database)
	if name != patchName {
		t.Errorf("series name after patch = %q, want %q", name, patchName)
	}

	parseEmail(t, ctx, database, "cover body\n",
		withSubject("[PATCH 0/2] the cover name"),
		withMsgID("<covername-0@test>"),
		withInReplyTo("<covername-1@test>"),
		withListID("test.example.com"))

	name = getSeriesName(t, database)
	if name != "the cover name" {
		t.Errorf("series name after cover = %q, want %q", name, "the cover name")
	}
}
