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

func registerUserRoutes(api huma.API, h *handler, prefix string, mw huma.Middlewares) {
	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: prefix + "/users",
		OperationID: fmt.Sprintf("list-users-v%s", prefix[5:]),
		Security:    authRequired,
		Middlewares: mw,
	}, h.ListUsers)
	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: prefix + "/users/{id}",
		OperationID: fmt.Sprintf("get-user-v%s", prefix[5:]),
		Security:    authRequired,
		Middlewares: mw,
	}, h.GetUserDetail)
	huma.Register(api, huma.Operation{
		Method: http.MethodPatch, Path: prefix + "/users/{id}",
		OperationID: fmt.Sprintf("update-user-v%s", prefix[5:]),
		Security:    authRequired,
		Middlewares: mw,
	}, h.UpdateUser)
	huma.Register(api, huma.Operation{
		Method: http.MethodPut, Path: prefix + "/users/{id}",
		OperationID: fmt.Sprintf("put-user-v%s", prefix[5:]),
		Security:    authRequired,
		Middlewares: mw,
	}, h.UpdateUser)
}

type ListUsersInput struct {
	PageParams
	SearchParams
}

type ListUsersOutput struct {
	Link string         `header:"Link" doc:"Pagination links"`
	Body []UserResponse `json:"body"`
}

func (h *handler) ListUsers(
	ctx context.Context, input *ListUsersInput,
) (*ListUsersOutput, error) {
	if _, err := h.requireUser(ctx); err != nil {
		return nil, err
	}

	base := h.apiBase(ctx)

	sq := db.GetQueries(ctx).DB.NewSelect().Model((*db.User)(nil))
	if input.Q != "" {
		sq = sq.WhereGroup(" AND ", func(sq *bun.SelectQuery) *bun.SelectQuery {
			return sq.Where("username LIKE ?", "%"+input.Q+"%").
				WhereOr("first_name LIKE ?", "%"+input.Q+"%").
				WhereOr("last_name LIKE ?", "%"+input.Q+"%").
				WhereOr("email LIKE ?", "%"+input.Q+"%")
		})
	}

	total, err := sq.Count(ctx)
	if err != nil {
		log.Errorf("count users: %v", err)
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

	var users []db.User
	if err := sq.OrderExpr("id ASC").Offset(offset).Limit(perPage).Scan(ctx, &users); err != nil {
		log.Errorf("list users: %v", err)
		return nil, huma.Error500InternalServerError("Internal error.")
	}

	resp := &ListUsersOutput{
		Link: buildLinkHeader(input.Page, perPage, total),
		Body: make([]UserResponse, len(users)),
	}
	for i := range users {
		resp.Body[i] = userToResponse(&users[i], base)
	}
	return resp, nil
}

type GetUserInput struct {
	ID int `path:"id" doc:"User ID"`
}

type GetUserOutput struct {
	Body UserDetailResponse
}

func (h *handler) GetUserDetail(
	ctx context.Context, input *GetUserInput,
) (*GetUserOutput, error) {
	q := db.GetQueries(ctx)
	caller, err := h.requireUser(ctx)
	if err != nil {
		return nil, err
	}

	base := h.apiBase(ctx)

	var user db.User
	if err := q.DB.NewSelect().Model(&user).
		Where("id = ?", input.ID).Scan(ctx); err != nil {
		return nil, huma.Error404NotFound("Not found.")
	}

	resp := &GetUserOutput{
		Body: userToDetailResponse(&user, base),
	}

	if caller.ID == user.ID {
		resp.Body.Settings = &UserSettings{
			SendEmail:    user.SendEmail,
			ItemsPerPage: user.ItemsPerPage,
			ShowIds:      user.ShowIds,
		}
	}

	return resp, nil
}

type UpdateUserInput struct {
	ID   int            `path:"id" doc:"User ID"`
	Body UserUpdateBody `json:"body"`
}

func (h *handler) UpdateUser(
	ctx context.Context, input *UpdateUserInput,
) (*GetUserOutput, error) {
	q := db.GetQueries(ctx)
	caller, err := h.requireUser(ctx)
	if caller == nil {
		return nil, err
	}
	if caller.ID != input.ID {
		return nil, ForbiddenErr
	}

	var user db.User
	if err := q.DB.NewSelect().Model(&user).
		Where("id = ?", input.ID).Scan(ctx); err != nil {
		return nil, huma.Error404NotFound("Not found.")
	}

	body := &input.Body
	uq := q.DB.NewUpdate().Model(&user).Where("id = ?", input.ID)
	if body.FirstName != nil {
		uq = uq.Set("first_name = ?", *body.FirstName)
		user.FirstName = *body.FirstName
	}
	if body.LastName != nil {
		uq = uq.Set("last_name = ?", *body.LastName)
		user.LastName = *body.LastName
	}
	if _, err := uq.Exec(ctx); err != nil {
		return nil, huma.Error400BadRequest("Update failed.")
	}

	base := h.apiBase(ctx)

	return &GetUserOutput{
		Body: userToDetailResponse(&user, base),
	}, nil
}

func userToResponse(u *db.User, base string) UserResponse {
	return UserResponse{
		ID:        u.ID,
		URL:       fmt.Sprintf("%s/users/%d/", base, u.ID),
		Username:  u.Username,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Email:     u.Email,
	}
}

func userToDetailResponse(u *db.User, base string) UserDetailResponse {
	return UserDetailResponse{
		UserResponse: userToResponse(u, base),
	}
}
