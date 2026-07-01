// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/danielgtaylor/huma/v2"
	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/db"
)

func registerSeriesRoutes(api huma.API, h *handler, prefix string, mw huma.Middlewares) {
	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: prefix + "/series",
		OperationID: fmt.Sprintf("list-series-v%s", prefix[5:]),
		Middlewares: mw,
	}, h.ListSeries)
	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: prefix + "/series/{id}",
		OperationID: fmt.Sprintf("get-series-v%s", prefix[5:]),
		Middlewares: mw,
	}, h.LetSeries)
	huma.Register(api, huma.Operation{
		Method: http.MethodPatch, Path: prefix + "/series/{id}",
		OperationID: fmt.Sprintf("update-series-v%s", prefix[5:]),
		Security:    authRequired,
		Middlewares: mw,
	}, h.UpdateSeries)
}

type ListSeriesInput struct {
	PageParams
	SearchParams
	Project       string `query:"project" doc:"Project ID or linkname"`
	Submitter     int    `query:"submitter" doc:"Submitter ID"`
	Since         string `query:"since" doc:"Earliest date"`
	Before        string `query:"before" doc:"Latest date"`
	MetadataKey   string `query:"metadata_key" doc:"Metadata key filter"`
	MetadataValue string `query:"metadata_value" doc:"Metadata value filter"`
}

type ListSeriesOutput struct {
	Link string           `header:"Link" doc:"Pagination links"`
	Body []SeriesResponse `json:"body"`
}

func (h *handler) ListSeries(
	ctx context.Context, input *ListSeriesInput,
) (*ListSeriesOutput, error) {
	base := h.apiBase(ctx)

	idb := db.GetQueries(ctx).DB
	sq := idb.NewSelect().Model((*db.Series)(nil))
	sq = applySeriesFilters(sq, input)

	total, _ := sq.Count(ctx)

	perPage := input.PerPage
	if perPage > maxPerPage {
		perPage = maxPerPage
	}
	offset := (input.Page - 1) * perPage

	var series []db.Series
	sq.Model(&series).
		Relation("Submitter").Relation("Project").
		OrderExpr("series.id DESC").Offset(offset).Limit(perPage).Scan(ctx)

	loadSeriesDetail(ctx, idb, base, series)

	resp := &ListSeriesOutput{
		Link: buildLinkHeader(input.Page, perPage, total),
		Body: make([]SeriesResponse, len(series)),
	}
	for i := range series {
		resp.Body[i] = seriesToResponse(&series[i], base)
	}
	return resp, nil
}

type GetSeriesInput struct {
	ID int `path:"id" doc:"Series ID"`
}

type GetSeriesOutput struct {
	Body SeriesResponse
}

func (h *handler) LetSeries(
	ctx context.Context, input *GetSeriesInput,
) (*GetSeriesOutput, error) {
	base := h.apiBase(ctx)

	var s db.Series
	if err := db.GetQueries(ctx).DB.NewSelect().Model(&s).
		Relation("Submitter").Relation("Project").
		Where("series.id = ?", input.ID).Scan(ctx); err != nil {
		return nil, huma.Error404NotFound("Not found.")
	}

	series := []db.Series{s}
	loadSeriesDetail(ctx, db.GetQueries(ctx).DB, base, series)

	return &GetSeriesOutput{
		Body: seriesToResponse(&series[0], base),
	}, nil
}

type UpdateSeriesInput struct {
	ID   int              `path:"id" doc:"Series ID"`
	Body SeriesUpdateBody `json:"body"`
}

func (h *handler) UpdateSeries(
	ctx context.Context, input *UpdateSeriesInput,
) (*GetSeriesOutput, error) {
	user, err := h.requireUser(ctx)
	if err != nil {
		return nil, err
	}

	q := db.GetQueries(ctx)
	base := h.apiBase(ctx)

	var s db.Series
	if err := q.DB.NewSelect().Model(&s).
		Where("id = ?", input.ID).Scan(ctx); err != nil {
		return nil, huma.Error404NotFound("Not found.")
	}

	if s.ProjectID != nil && !q.IsMaintainer(user, *s.ProjectID) {
		return nil, ForbiddenErr
	}

	body := &input.Body

	if body.Version != nil {
		if _, err := q.DB.NewUpdate().Model(&s).
			Set("version = ?", *body.Version).
			Where("id = ?", input.ID).Exec(ctx); err != nil {
			return nil, huma.Error400BadRequest("Update failed.")
		}
		s.Version = *body.Version
	}

	if body.Metadata != nil {
		if _, err := q.DB.NewDelete().Model((*db.SeriesMetadata)(nil)).
			Where("series_id = ?", input.ID).Exec(ctx); err != nil {
			return nil, huma.Error400BadRequest("Update failed.")
		}
		for k, v := range *body.Metadata {
			if _, err := q.DB.NewInsert().Model(&db.SeriesMetadata{
				SeriesID: input.ID, Key: k, Value: v,
			}).Exec(ctx); err != nil {
				return nil, huma.Error400BadRequest("Update failed.")
			}
		}
	}

	// Re-fetch with relations after update.
	if err := q.DB.NewSelect().Model(&s).
		Relation("Submitter").Relation("Project").
		Where("series.id = ?", input.ID).Scan(ctx); err != nil {
		return nil, huma.Error404NotFound("Not found.")
	}

	series := []db.Series{s}
	loadSeriesDetail(ctx, db.GetQueries(ctx).DB, base, series)

	return &GetSeriesOutput{
		Body: seriesToResponse(&series[0], base),
	}, nil
}

func applySeriesFilters(q *bun.SelectQuery, input *ListSeriesInput) *bun.SelectQuery {
	if input.Project != "" {
		if id, err := strconv.Atoi(input.Project); err == nil {
			q = q.Where("series.project_id = ?", id)
		} else {
			q = q.Where("series.project_id IN (SELECT id FROM project WHERE linkname = ?)", input.Project)
		}
	}
	if input.Submitter != 0 {
		q = q.Where("series.submitter_id = ?", input.Submitter)
	}
	if input.Since != "" {
		q = q.Where("series.date >= ?", input.Since)
	}
	if input.Before != "" {
		q = q.Where("series.date < ?", input.Before)
	}
	if input.MetadataKey != "" {
		q = q.Where("series.id IN (SELECT series_id FROM series_metadata WHERE key = ?)", input.MetadataKey)
	}
	if input.MetadataValue != "" {
		q = q.Where("series.id IN (SELECT series_id FROM series_metadata WHERE value = ?)", input.MetadataValue)
	}
	if input.Q != "" {
		q = q.Where("series.name LIKE ?", "%"+input.Q+"%")
	}
	return q
}

func seriesToResponse(s *db.Series, base string) SeriesResponse {
	r := SeriesResponse{
		ID:            s.ID,
		URL:           fmt.Sprintf("%s/series/%d/", base, s.ID),
		Name:          s.Name,
		Date:          s.Date,
		Version:       s.Version,
		Total:         s.Total,
		ReceivedTotal: s.ReceivedTotal,
		ReceivedAll:   s.ReceivedAll,
		Mbox:          fmt.Sprintf("%s/series/%d/mbox/", base, s.ID),
		Patches:       make([]PatchEmbedded, len(s.Patches)),
		Dependencies:  s.Dependencies,
		Dependents:    s.Dependents,
	}
	if s.Project != nil {
		r.Project = projectToEmbedded(s.Project, base)
		if s.Project.WebURL != "" {
			r.WebURL = strp(fmt.Sprintf("%s/series/%d/",
				s.Project.WebURL, s.ID))
		}
	}
	if s.Submitter != nil {
		r.Submitter = personToEmbedded(s.Submitter, base)
	}
	if s.CoverLetter != nil {
		c := coverToEmbedded(s.CoverLetter, s.Project, base)
		r.CoverLetter = &c
	}
	for i := range s.Patches {
		r.Patches[i] = patchToEmbedded(&s.Patches[i], s.Project, base)
	}
	if r.Dependencies == nil {
		r.Dependencies = []string{}
	}
	if r.Dependents == nil {
		r.Dependents = []string{}
	}
	meta := s.Metadata
	if meta == nil {
		meta = map[string]string{}
	}
	r.Metadata = &meta
	return r
}

func patchToEmbedded(p *db.Patch, project *db.Project, base string) PatchEmbedded {
	e := PatchEmbedded{
		ID:    p.ID,
		URL:   fmt.Sprintf("%s/patches/%d/", base, p.ID),
		Msgid: p.Msgid,
		Date:  p.Date.Format("2006-01-02T15:04:05"),
		Name:  p.Name,
		Mbox:  fmt.Sprintf("%s/patches/%d/mbox/", base, p.ID),
	}
	if project != nil && project.WebURL != "" {
		e.WebURL = strp(fmt.Sprintf("%s/patch/%s/",
			project.WebURL, p.Msgid))
	}
	archiveURL := listArchiveURL(project, p.Msgid)
	if archiveURL != "" {
		e.ListArchiveURL = &archiveURL
	}
	return e
}

func coverToEmbedded(c *db.Cover, project *db.Project, base string) CoverEmbedded {
	e := CoverEmbedded{
		ID:    c.ID,
		URL:   fmt.Sprintf("%s/covers/%d/", base, c.ID),
		Msgid: c.Msgid,
		Date:  c.Date.Format("2006-01-02T15:04:05"),
		Name:  c.Name,
		Mbox:  fmt.Sprintf("%s/covers/%d/mbox/", base, c.ID),
	}
	if project != nil && project.WebURL != "" {
		e.WebURL = strp(fmt.Sprintf("%s/cover/%s/",
			project.WebURL, c.Msgid))
	}
	archiveURL := listArchiveURL(project, c.Msgid)
	if archiveURL != "" {
		e.ListArchiveURL = &archiveURL
	}
	return e
}
