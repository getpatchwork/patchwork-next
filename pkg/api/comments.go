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

	"github.com/getpatchwork/patchwork/pkg/db"
	"github.com/getpatchwork/patchwork/pkg/log"
)

func registerCommentRoutes(api huma.API, h *handler, prefix string, mw huma.Middlewares) {
	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: prefix + "/patches/{id}/comments",
		OperationID: fmt.Sprintf("list-patch-comments-v%s", prefix[5:]),
		Middlewares: mw,
	}, h.ListPatchComments)
	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: prefix + "/patches/{patch_id}/comments/{comment_id}",
		OperationID: fmt.Sprintf("get-patch-comment-v%s", prefix[5:]),
		Middlewares: mw,
	}, h.GetPatchComment)
	huma.Register(api, huma.Operation{
		Method: http.MethodPatch, Path: prefix + "/patches/{patch_id}/comments/{comment_id}",
		OperationID: fmt.Sprintf("update-patch-comment-v%s", prefix[5:]),
		Security:    authRequired,
		Middlewares: mw,
	}, h.UpdatePatchComment)
	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: prefix + "/covers/{id}/comments",
		OperationID: fmt.Sprintf("list-cover-comments-v%s", prefix[5:]),
		Middlewares: mw,
	}, h.ListCoverComments)
	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: prefix + "/covers/{cover_id}/comments/{comment_id}",
		OperationID: fmt.Sprintf("get-cover-comment-v%s", prefix[5:]),
		Middlewares: mw,
	}, h.GetCoverComment)
	huma.Register(api, huma.Operation{
		Method: http.MethodPatch, Path: prefix + "/covers/{cover_id}/comments/{comment_id}",
		OperationID: fmt.Sprintf("update-cover-comment-v%s", prefix[5:]),
		Security:    authRequired,
		Middlewares: mw,
	}, h.UpdateCoverComment)
}

// Patch comments

type ListPatchCommentsInput struct {
	PageParams
	ID int `path:"id" doc:"Patch ID"`
}

type ListPatchCommentsOutput struct {
	Link string            `header:"Link" doc:"Pagination links"`
	Body []CommentResponse `json:"body"`
}

func (h *handler) ListPatchComments(
	ctx context.Context, input *ListPatchCommentsInput,
) (*ListPatchCommentsOutput, error) {
	base := h.apiBase(ctx)

	q := db.GetQueries(ctx).DB.NewSelect().Model((*db.PatchComment)(nil)).
		Where("patch_comment.patch_id = ?", input.ID)

	total, err := q.Count(ctx)
	if err != nil {
		log.Errorf("count patch comments: %v", err)
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

	var comments []db.PatchComment
	if err := q.Model(&comments).Relation("Submitter").
		OrderExpr("patch_comment.id ASC").Offset(offset).Limit(perPage).Scan(ctx); err != nil {
		log.Errorf("list patch comments: %v", err)
		return nil, huma.Error500InternalServerError("Internal error.")
	}
	populateCommentURLs(base, input.ID, comments)

	resp := &ListPatchCommentsOutput{
		Link: buildLinkHeader(input.Page, perPage, total),
		Body: make([]CommentResponse, len(comments)),
	}
	for i := range comments {
		resp.Body[i] = patchCommentToResponse(&comments[i], base)
	}
	return resp, nil
}

type GetPatchCommentInput struct {
	PatchID   int `path:"patch_id" doc:"Patch ID"`
	CommentID int `path:"comment_id" doc:"Comment ID"`
}

type GetPatchCommentOutput struct {
	Body CommentResponse
}

func (h *handler) GetPatchComment(
	ctx context.Context, input *GetPatchCommentInput,
) (*GetPatchCommentOutput, error) {
	base := h.apiBase(ctx)

	var c db.PatchComment
	err := db.GetQueries(ctx).DB.NewSelect().Model(&c).
		Relation("Submitter").
		Where("patch_comment.id = ?", input.CommentID).
		Where("patch_comment.patch_id = ?", input.PatchID).
		Scan(ctx)
	if err != nil {
		return nil, huma.Error404NotFound("Not found.")
	}

	comments := []db.PatchComment{c}
	populateCommentURLs(base, input.PatchID, comments)

	return &GetPatchCommentOutput{
		Body: patchCommentToResponse(&comments[0], base),
	}, nil
}

type UpdatePatchCommentInput struct {
	PatchID   int `path:"patch_id" doc:"Patch ID"`
	CommentID int `path:"comment_id" doc:"Comment ID"`
	Body      CommentUpdateBody
}

type UpdatePatchCommentOutput struct {
	Body CommentResponse
}

func (h *handler) UpdatePatchComment(
	ctx context.Context, input *UpdatePatchCommentInput,
) (*UpdatePatchCommentOutput, error) {
	base := h.apiBase(ctx)

	user, err := h.requireUser(ctx)
	if err != nil {
		return nil, err
	}

	q := db.GetQueries(ctx)

	var c db.PatchComment
	err = q.DB.NewSelect().Model(&c).
		Relation("Submitter").
		Where("patch_comment.id = ?", input.CommentID).
		Where("patch_comment.patch_id = ?", input.PatchID).
		Scan(ctx)
	if err != nil {
		return nil, huma.Error404NotFound("Not found.")
	}

	var patch db.Patch
	if err := q.DB.NewSelect().Model(&patch).
		Where("id = ?", input.PatchID).
		Column("id", "project_id").Scan(ctx); err != nil {
		return nil, huma.Error404NotFound("Not found.")
	}

	if !q.IsMaintainer(user, patch.ProjectID) {
		return nil, ForbiddenErr
	}

	if input.Body.Addressed != nil {
		if _, err := q.DB.NewUpdate().Model((*db.PatchComment)(nil)).
			Set("addressed = ?", *input.Body.Addressed).
			Where("id = ?", input.CommentID).
			Exec(ctx); err != nil {
			return nil, huma.Error400BadRequest("Update failed.")
		}
		c.Addressed = input.Body.Addressed
	}

	comments := []db.PatchComment{c}
	populateCommentURLs(base, input.PatchID, comments)

	return &UpdatePatchCommentOutput{
		Body: patchCommentToResponse(&comments[0], base),
	}, nil
}

// Cover comments

type ListCoverCommentsInput struct {
	PageParams
	ID int `path:"id" doc:"Cover ID"`
}

type ListCoverCommentsOutput struct {
	Link string            `header:"Link" doc:"Pagination links"`
	Body []CommentResponse `json:"body"`
}

func (h *handler) ListCoverComments(
	ctx context.Context, input *ListCoverCommentsInput,
) (*ListCoverCommentsOutput, error) {
	base := h.apiBase(ctx)

	q := db.GetQueries(ctx).DB.NewSelect().Model((*db.CoverComment)(nil)).
		Where("cover_comment.cover_id = ?", input.ID)

	total, err := q.Count(ctx)
	if err != nil {
		log.Errorf("count cover comments: %v", err)
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

	var comments []db.CoverComment
	if err := q.Model(&comments).Relation("Submitter").
		OrderExpr("cover_comment.id ASC").Offset(offset).Limit(perPage).Scan(ctx); err != nil {
		log.Errorf("list cover comments: %v", err)
		return nil, huma.Error500InternalServerError("Internal error.")
	}
	populateCoverCommentURLs(base, input.ID, comments)

	resp := &ListCoverCommentsOutput{
		Link: buildLinkHeader(input.Page, perPage, total),
		Body: make([]CommentResponse, len(comments)),
	}
	for i := range comments {
		resp.Body[i] = coverCommentToResponse(&comments[i], base)
	}
	return resp, nil
}

type GetCoverCommentInput struct {
	CoverID   int `path:"cover_id" doc:"Cover ID"`
	CommentID int `path:"comment_id" doc:"Comment ID"`
}

type GetCoverCommentOutput struct {
	Body CommentResponse
}

func (h *handler) GetCoverComment(
	ctx context.Context, input *GetCoverCommentInput,
) (*GetCoverCommentOutput, error) {
	base := h.apiBase(ctx)

	var c db.CoverComment
	err := db.GetQueries(ctx).DB.NewSelect().Model(&c).
		Relation("Submitter").
		Where("cover_comment.id = ?", input.CommentID).
		Where("cover_comment.cover_id = ?", input.CoverID).
		Scan(ctx)
	if err != nil {
		return nil, huma.Error404NotFound("Not found.")
	}

	comments := []db.CoverComment{c}
	populateCoverCommentURLs(base, input.CoverID, comments)

	return &GetCoverCommentOutput{
		Body: coverCommentToResponse(&comments[0], base),
	}, nil
}

type UpdateCoverCommentInput struct {
	CoverID   int `path:"cover_id" doc:"Cover ID"`
	CommentID int `path:"comment_id" doc:"Comment ID"`
	Body      CommentUpdateBody
}

type UpdateCoverCommentOutput struct {
	Body CommentResponse
}

func (h *handler) UpdateCoverComment(
	ctx context.Context, input *UpdateCoverCommentInput,
) (*UpdateCoverCommentOutput, error) {
	base := h.apiBase(ctx)

	user, err := h.requireUser(ctx)
	if err != nil {
		return nil, err
	}

	q := db.GetQueries(ctx)

	var c db.CoverComment
	err = q.DB.NewSelect().Model(&c).
		Relation("Submitter").
		Where("cover_comment.id = ?", input.CommentID).
		Where("cover_comment.cover_id = ?", input.CoverID).
		Scan(ctx)
	if err != nil {
		return nil, huma.Error404NotFound("Not found.")
	}

	var cover db.Cover
	if err := q.DB.NewSelect().Model(&cover).
		Where("id = ?", input.CoverID).
		Column("id", "project_id").Scan(ctx); err != nil {
		return nil, huma.Error404NotFound("Not found.")
	}

	if !q.IsMaintainer(user, cover.ProjectID) {
		return nil, ForbiddenErr
	}

	if input.Body.Addressed != nil {
		if _, err := q.DB.NewUpdate().Model((*db.CoverComment)(nil)).
			Set("addressed = ?", *input.Body.Addressed).
			Where("id = ?", input.CommentID).
			Exec(ctx); err != nil {
			return nil, huma.Error400BadRequest("Update failed.")
		}
		c.Addressed = input.Body.Addressed
	}

	comments := []db.CoverComment{c}
	populateCoverCommentURLs(base, input.CoverID, comments)

	return &UpdateCoverCommentOutput{
		Body: coverCommentToResponse(&comments[0], base),
	}, nil
}

// Converters

func patchCommentToResponse(c *db.PatchComment, base string) CommentResponse {
	r := CommentResponse{
		ID:        c.ID,
		Msgid:     c.Msgid,
		Date:      c.Date,
		Subject:   c.Subject,
		Content:   "",
		Headers:   parseHeadersMap(c.Headers),
		Addressed: c.Addressed,
	}
	if c.URL != "" {
		r.URL = &c.URL
	}
	if c.Content != nil {
		r.Content = *c.Content
	}
	if c.Submitter != nil {
		r.Submitter = personToEmbedded(c.Submitter, base)
	}
	return r
}

func coverCommentToResponse(c *db.CoverComment, base string) CommentResponse {
	r := CommentResponse{
		ID:        c.ID,
		Msgid:     c.Msgid,
		Date:      c.Date,
		Subject:   c.Subject,
		Content:   "",
		Headers:   parseHeadersMap(c.Headers),
		Addressed: c.Addressed,
	}
	if c.URL != "" {
		r.URL = &c.URL
	}
	if c.Content != nil {
		r.Content = *c.Content
	}
	if c.Submitter != nil {
		r.Submitter = personToEmbedded(c.Submitter, base)
	}
	return r
}
