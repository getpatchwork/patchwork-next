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
	"github.com/getpatchwork/patchwork/pkg/log"
)

func registerCoverRoutes(api huma.API, h *handler, prefix string, mw huma.Middlewares) {
	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: prefix + "/covers",
		OperationID: fmt.Sprintf("list-covers-v%s", prefix[5:]),
		Middlewares: mw,
	}, h.ListCovers)
	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: prefix + "/covers/{id}",
		OperationID: fmt.Sprintf("get-cover-v%s", prefix[5:]),
		Middlewares: mw,
	}, h.GetCover)
}

type ListCoversInput struct {
	PageParams
	SearchParams
	Project   string `query:"project" doc:"Project ID or linkname"`
	Submitter string `query:"submitter" doc:"Submitter ID or email"`
	Series    string `query:"series" doc:"Series ID"`
	Msgid     string `query:"msgid" doc:"Message ID"`
	Since     string `query:"since" doc:"Earliest date"`
	Before    string `query:"before" doc:"Latest date"`
}

type ListCoversOutput struct {
	Link string              `header:"Link" doc:"Pagination links"`
	Body []CoverListResponse `json:"body"`
}

func (h *handler) ListCovers(
	ctx context.Context, input *ListCoversInput,
) (*ListCoversOutput, error) {
	base := h.apiBase(ctx)

	idb := db.GetQueries(ctx).DB
	sq := idb.NewSelect().Model((*db.Cover)(nil))
	sq = applyCoverFilters(sq, input)

	total, err := sq.Count(ctx)
	if err != nil {
		log.Errorf("count covers: %v", err)
		return nil, huma.Error500InternalServerError("Internal error.")
	}

	perPage := input.PerPage
	if perPage > maxPerPage {
		perPage = maxPerPage
	}
	offset := (input.Page - 1) * perPage

	var covers []db.Cover
	if err := sq.Model(&covers).
		Relation("Submitter").Relation("Project").
		OrderExpr("cover.id DESC").Offset(offset).Limit(perPage).Scan(ctx); err != nil {
		log.Errorf("list covers: %v", err)
		return nil, huma.Error500InternalServerError("Internal error.")
	}
	loadCoverSeries(ctx, idb, covers)

	resp := &ListCoversOutput{
		Link: buildLinkHeader(input.Page, perPage, total),
		Body: make([]CoverListResponse, len(covers)),
	}
	for i := range covers {
		resp.Body[i] = coverToListResponse(&covers[i], base)
	}
	return resp, nil
}

type GetCoverInput struct {
	ID int `path:"id" doc:"Cover ID"`
}

type GetCoverOutput struct {
	Body CoverDetailResponse
}

func (h *handler) GetCover(
	ctx context.Context, input *GetCoverInput,
) (*GetCoverOutput, error) {
	idb := db.GetQueries(ctx).DB
	base := h.apiBase(ctx)

	var cover db.Cover
	if err := idb.NewSelect().Model(&cover).
		Relation("Submitter").Relation("Project").
		Where("cover.id = ?", input.ID).Scan(ctx); err != nil {
		return nil, huma.Error404NotFound("Not found.")
	}

	covers := []db.Cover{cover}
	loadCoverSeries(ctx, idb, covers)

	return &GetCoverOutput{
		Body: coverToDetailResponse(&covers[0], base),
	}, nil
}

func applyCoverFilters(q *bun.SelectQuery, input *ListCoversInput) *bun.SelectQuery {
	if input.Project != "" {
		if id, err := strconv.Atoi(input.Project); err == nil {
			q = q.Where("cover.project_id = ?", id)
		} else {
			q = q.Where("cover.project_id IN (SELECT id FROM project WHERE linkname = ?)", input.Project)
		}
	}
	if input.Submitter != "" {
		if id, err := strconv.Atoi(input.Submitter); err == nil {
			q = q.Where("cover.submitter_id = ?", id)
		} else {
			q = q.Where("cover.submitter_id IN (SELECT id FROM person WHERE email = ?)", input.Submitter)
		}
	}
	if input.Series != "" {
		if id, err := strconv.Atoi(input.Series); err == nil {
			q = q.Where("cover.id IN (SELECT cover_letter_id FROM series WHERE id = ?)", id)
		}
	}
	if input.Msgid != "" {
		q = q.Where("cover.msgid = ?", "<"+input.Msgid+">")
	}
	if input.Since != "" {
		q = q.Where("cover.date >= ?", input.Since)
	}
	if input.Before != "" {
		q = q.Where("cover.date < ?", input.Before)
	}
	if input.Q != "" {
		q = q.Where("cover.name LIKE ?", "%"+input.Q+"%")
	}
	return q
}

func coverToListResponse(c *db.Cover, base string) CoverListResponse {
	r := CoverListResponse{
		ID:       c.ID,
		URL:      fmt.Sprintf("%s/covers/%d/", base, c.ID),
		Msgid:    c.Msgid,
		Date:     c.Date,
		Name:     c.Name,
		Mbox:     fmt.Sprintf("%s/covers/%d/mbox/", base, c.ID),
		Comments: strp(fmt.Sprintf("%s/covers/%d/comments/", base, c.ID)),
		Series:   []SeriesEmbedded{},
	}
	if c.Project != nil {
		r.Project = projectToEmbedded(c.Project, base)
		if c.Project.WebURL != "" {
			r.WebURL = strp(fmt.Sprintf("%s/cover/%s/",
				c.Project.WebURL, c.Msgid))
		}
		archiveURL := listArchiveURL(c.Project, c.Msgid)
		if archiveURL != "" {
			r.ListArchiveURL = &archiveURL
		}
	}
	if c.Submitter != nil {
		r.Submitter = personToEmbedded(c.Submitter, base)
	}
	for _, s := range c.SeriesList {
		r.Series = append(r.Series, SeriesEmbedded{
			ID:   s.ID,
			URL:  fmt.Sprintf("%s/series/%d/", base, s.ID),
			Name: s.Name,
			Mbox: fmt.Sprintf("%s/series/%d/mbox/", base, s.ID),
		})
	}
	return r
}

func coverToDetailResponse(c *db.Cover, base string) CoverDetailResponse {
	r := CoverDetailResponse{
		CoverListResponse: coverToListResponse(c, base),
		Headers:           parseHeadersMap(c.Headers),
	}
	if c.Content != nil {
		r.Content = *c.Content
	}
	return r
}
