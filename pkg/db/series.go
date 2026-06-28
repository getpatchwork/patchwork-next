// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import (
	"time"

	"github.com/uptrace/bun"
)

func (q *Queries) CreateSeries(s *Series) error {
	return q.DB.NewInsert().Model(s).
		Returning("*").
		Scan(q.Ctx)
}

func (q *Queries) GetSeriesByID(id int) (*Series, error) {
	var s Series
	err := q.DB.NewSelect().Model(&s).
		Where("id = ?", id).
		Scan(q.Ctx)
	return &s, err
}

func (q *Queries) FindSeriesByMsgID(msgid string) (*Series, error) {
	var s Series
	// check patches first
	err := q.DB.NewSelect().Model(&s).
		Where("id = (SELECT series_id FROM patch WHERE msgid = ? AND series_id IS NOT NULL LIMIT 1)", msgid).
		Scan(q.Ctx)
	if err == nil {
		return &s, nil
	}
	// check cover letters
	err = q.DB.NewSelect().Model(&s).
		Where("cover_letter_id = (SELECT id FROM cover WHERE msgid = ? LIMIT 1)", msgid).
		Scan(q.Ctx)
	return &s, err
}

func (q *Queries) FindSeriesByReference(projectID int, msgid string) (*Series, error) {
	var s Series
	err := q.DB.NewSelect().Model(&s).
		Join("JOIN series_reference AS sr ON sr.series_id = series.id").
		Where("sr.project_id = ?", projectID).
		Where("sr.msgid = ?", msgid).
		OrderBy("date", bun.OrderDesc).
		Limit(1).
		Scan(q.Ctx)
	return &s, err
}

func (q *Queries) FindSeriesByMarkers(
	projectID *int, submitterID int,
	version, total int,
	dateMin, dateMax time.Time, number *int,
) (*Series, error) {
	var s Series
	err := q.DB.NewSelect().Model(&s).
		Where("project_id = ?", projectID).
		Where("submitter_id = ?", submitterID).
		Where("version = ?", version).
		Where("total = ?", total).
		Where("date >= ?", dateMin).
		Where("date <= ?", dateMax).
		Where("NOT EXISTS (SELECT 1 FROM patch WHERE series_id = series.id AND number = ?)", number).
		OrderBy("date", bun.OrderDesc).
		Limit(1).
		Scan(q.Ctx)
	return &s, err
}

func (q *Queries) FindSeries(
	projectID int, submitterID int, refs []string, msgid string,
	number int, version, total int,
	date time.Time, delta time.Duration,
) (*Series, error) {
	var s Series

	slotCheck := func(query *bun.SelectQuery) *bun.SelectQuery {
		if number != 0 {
			query = query.Where(
				"NOT EXISTS (SELECT 1 FROM patch WHERE series_id = series.id AND number = ?)",
				number,
			)
		}
		return query
	}
	dateCheck := func(query *bun.SelectQuery) *bun.SelectQuery {
		if !date.IsZero() {
			query = query.
				Where("series.date >= ?", date.Add(-delta)).
				Where("series.date <= ?", date.Add(delta))
		}
		return query
	}

	// tier 1: match by message references
	if len(refs) > 0 {
		refQuery := q.DB.NewSelect().Model(&s).
			Join("JOIN series_reference AS sr ON sr.series_id = series.id").
			Where("sr.msgid IN ?", bun.Tuple(refs)).
			Where("series.project_id = ?", projectID).
			Where("series.version = ?", version).
			Where("series.total = ?", total)
		refQuery = dateCheck(slotCheck(refQuery))
		if msgid != "" {
			refQuery = refQuery.OrderExpr(
				"CASE WHEN sr.msgid = ? THEN 0 ELSE 1 END", msgid,
			)
		}
		refQuery = refQuery.OrderExpr("series.date DESC").Limit(1)
		if err := refQuery.Scan(q.Ctx); err == nil {
			return &s, nil
		}
	}

	// tier 2: match by markers (submitter, version, total, date)
	markerQuery := q.DB.NewSelect().Model(&s).
		Where("project_id = ?", projectID).
		Where("submitter_id = ?", submitterID).
		Where("version = ?", version).
		Where("total = ?", total)
	markerQuery = dateCheck(slotCheck(markerQuery))
	markerQuery = markerQuery.OrderExpr("date DESC").Limit(1)

	err := markerQuery.Scan(q.Ctx)
	return &s, err
}

func (q *Queries) FindPreviousSeriesByName(
	projectID *int, submitterID int, version int,
) ([]Series, error) {
	var series []Series
	err := q.DB.NewSelect().Model(&series).
		Where("project_id = ?", projectID).
		Where("submitter_id = ?", submitterID).
		Where("version = ?", version).
		OrderBy("date", bun.OrderDesc).
		Scan(q.Ctx)
	return series, err
}

func (q *Queries) CreateSeriesReference(projectID, seriesID int, msgid string) error {
	ref := &SeriesReference{
		ProjectID: projectID,
		SeriesID:  seriesID,
		Msgid:     msgid,
	}
	_, err := q.DB.NewInsert().Model(ref).
		On("CONFLICT DO NOTHING").
		Exec(q.Ctx)
	return err
}

func (q *Queries) UpdateSeriesCoverLetter(id int, coverLetterID *int) error {
	_, err := q.DB.NewUpdate().Model((*Series)(nil)).
		Set("cover_letter_id = ?", coverLetterID).
		Where("id = ?", id).
		Exec(q.Ctx)
	return err
}

func (q *Queries) UpdateSeriesName(id int, name *string) error {
	_, err := q.DB.NewUpdate().Model((*Series)(nil)).
		Set("name = ?", name).
		Where("id = ?", id).
		Exec(q.Ctx)
	return err
}

func (q *Queries) UpdateSeriesPreviousSeries(id int, previousSeriesID *int) error {
	_, err := q.DB.NewUpdate().Model((*Series)(nil)).
		Set("previous_series_id = ?", previousSeriesID).
		Where("id = ?", id).
		Exec(q.Ctx)
	return err
}

func (q *Queries) AddSeriesDependencies(fromSeriesID, toSeriesID int) error {
	_, err := q.DB.NewInsert().Model(&SeriesDependencies{
		FromSeriesID: fromSeriesID,
		ToSeriesID:   toSeriesID,
	}).On("CONFLICT DO NOTHING").
		ExcludeColumn("id").
		Exec(q.Ctx)
	return err
}
