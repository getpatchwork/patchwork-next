// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/db"
	"github.com/getpatchwork/patchwork/pkg/log"
)

func registerPeopleRoutes(api huma.API, h *handler, prefix string, mw huma.Middlewares) {
	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: prefix + "/people",
		OperationID: fmt.Sprintf("list-people-v%s", prefix[5:]),
		Middlewares: mw,
	}, h.ListPeople)
	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: prefix + "/people/{id}",
		OperationID: fmt.Sprintf("get-person-v%s", prefix[5:]),
		Middlewares: mw,
	}, h.GetPerson)
}

type ListPeopleInput struct {
	PageParams
	SearchParams
}

type ListPeopleOutput struct {
	Link string           `header:"Link" doc:"Pagination links"`
	Body []PersonResponse `json:"body"`
}

func (h *handler) ListPeople(
	ctx context.Context, input *ListPeopleInput,
) (*ListPeopleOutput, error) {
	base := h.apiBase(ctx)

	sq := db.GetQueries(ctx).DB.NewSelect().Model((*db.Person)(nil))
	if input.Q != "" {
		sq = sq.WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("person.name LIKE ?", "%"+input.Q+"%").
				WhereOr("person.email LIKE ?", "%"+input.Q+"%")
		})
	}

	total, err := sq.Count(ctx)
	if err != nil {
		log.Errorf("count people: %v", err)
		return nil, huma.Error500InternalServerError("Internal error.")
	}

	perPage := input.PerPage
	if perPage < 1 {
		perPage = h.cfg.Http.ApiPageSize
	}
	if perPage > h.cfg.Http.ApiPageMax {
		perPage = h.cfg.Http.ApiPageMax
	}
	offset := (input.Page - 1) * perPage

	var people []db.Person
	if err := sq.Model(&people).Relation("User").
		OrderExpr("person.id ASC").Offset(offset).Limit(perPage).Scan(ctx); err != nil {
		log.Errorf("list people: %v", err)
		return nil, huma.Error500InternalServerError("Internal error.")
	}

	resp := &ListPeopleOutput{
		Link: buildLinkHeader(input.Page, perPage, total),
		Body: make([]PersonResponse, len(people)),
	}
	for i := range people {
		resp.Body[i] = personToResponse(&people[i], base)
	}
	return resp, nil
}

type GetPersonInput struct {
	ID int `path:"id" doc:"Person ID"`
}

type GetPersonOutput struct {
	Body PersonResponse
}

func (h *handler) GetPerson(
	ctx context.Context, input *GetPersonInput,
) (*GetPersonOutput, error) {
	q := db.GetQueries(ctx)
	base := h.apiBase(ctx)

	var person db.Person
	if err := q.DB.NewSelect().Model(&person).
		Relation("User").
		Where("person.id = ?", input.ID).Scan(ctx); err != nil {
		return nil, huma.Error404NotFound("Not found.")
	}

	return &GetPersonOutput{
		Body: personToResponse(&person, base),
	}, nil
}

func personToResponse(p *db.Person, base string) PersonResponse {
	name := ""
	if p.Name != nil {
		name = *p.Name
	}
	r := PersonResponse{
		ID:    p.ID,
		URL:   fmt.Sprintf("%s/people/%d/", base, p.ID),
		Name:  name,
		Email: p.Email,
	}
	if p.User != nil {
		u := userToEmbedded(p.User, base)
		r.User = &u
	}
	return r
}
