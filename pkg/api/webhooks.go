// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/getpatchwork/patchwork/pkg/db"
)

func registerWebhookRoutes(api huma.API, h *handler, prefix string, mw huma.Middlewares) {
	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: prefix + "/projects/{projectID}/webhooks",
		OperationID: fmt.Sprintf("list-webhooks-v%s", prefix[5:]),
		Middlewares: mw,
	}, h.ListWebhooks)
	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: prefix + "/projects/{projectID}/webhooks/{webhookID}",
		OperationID: fmt.Sprintf("get-webhook-v%s", prefix[5:]),
		Middlewares: mw,
	}, h.GetWebhook)
	huma.Register(api, huma.Operation{
		Method: http.MethodPost, Path: prefix + "/projects/{projectID}/webhooks",
		OperationID:   fmt.Sprintf("create-webhook-v%s", prefix[5:]),
		DefaultStatus: 201,
		Middlewares:   mw,
	}, h.CreateWebhook)
	huma.Register(api, huma.Operation{
		Method: http.MethodPatch, Path: prefix + "/projects/{projectID}/webhooks/{webhookID}",
		OperationID: fmt.Sprintf("update-webhook-v%s", prefix[5:]),
		Middlewares: mw,
	}, h.UpdateWebhook)
	huma.Register(api, huma.Operation{
		Method: http.MethodDelete, Path: prefix + "/projects/{projectID}/webhooks/{webhookID}",
		OperationID:   fmt.Sprintf("delete-webhook-v%s", prefix[5:]),
		DefaultStatus: 204,
		Middlewares:   mw,
	}, h.DeleteWebhook)
}

var validEventCategories = map[string]bool{
	"cover-created":          true,
	"patch-created":          true,
	"patch-completed":        true,
	"patch-state-changed":    true,
	"patch-delegated":        true,
	"patch-relation-changed": true,
	"check-created":          true,
	"check-updated":          true,
	"series-created":         true,
	"series-completed":       true,
	"cover-comment-created":  true,
	"patch-comment-created":  true,
}

type ListWebhooksInput struct {
	PageParams
	ProjectID int `path:"projectID" doc:"Project ID"`
}

type ListWebhooksOutput struct {
	Link string            `header:"Link" doc:"Pagination links"`
	Body []WebhookResponse `json:"body"`
}

func (h *handler) ListWebhooks(
	ctx context.Context, input *ListWebhooksInput,
) (*ListWebhooksOutput, error) {
	if _, err := h.requireMaintainer(ctx, input.ProjectID); err != nil {
		return nil, err
	}

	base := h.apiBase(ctx)

	perPage := input.PerPage
	if perPage > maxPerPage {
		perPage = maxPerPage
	}
	offset := (input.Page - 1) * perPage

	sq := db.GetQueries(ctx).DB.NewSelect().Model((*db.Webhook)(nil)).
		Where("project_id = ?", input.ProjectID)

	total, _ := sq.Count(ctx)

	var hooks []db.Webhook
	sq.OrderExpr("id ASC").Offset(offset).Limit(perPage).Scan(ctx, &hooks)

	resp := &ListWebhooksOutput{
		Link: buildLinkHeader(input.Page, perPage, total),
		Body: make([]WebhookResponse, len(hooks)),
	}
	for i := range hooks {
		resp.Body[i] = webhookToResponse(&hooks[i], input.ProjectID, base)
	}
	return resp, nil
}

type GetWebhookInput struct {
	ProjectID int `path:"projectID" doc:"Project ID"`
	WebhookID int `path:"webhookID" doc:"Webhook ID"`
}

type GetWebhookOutput struct {
	Body WebhookResponse
}

func (h *handler) GetWebhook(
	ctx context.Context, input *GetWebhookInput,
) (*GetWebhookOutput, error) {
	if _, err := h.requireMaintainer(ctx, input.ProjectID); err != nil {
		return nil, err
	}

	q := db.GetQueries(ctx)
	base := h.apiBase(ctx)

	var hook db.Webhook
	if err := q.DB.NewSelect().Model(&hook).
		Where("id = ?", input.WebhookID).
		Where("project_id = ?", input.ProjectID).
		Scan(ctx); err != nil {
		return nil, huma.Error404NotFound("Not found.")
	}

	return &GetWebhookOutput{
		Body: webhookToResponse(&hook, input.ProjectID, base),
	}, nil
}

type CreateWebhookInput struct {
	ProjectID int               `path:"projectID" doc:"Project ID"`
	Body      WebhookCreateBody `json:"body"`
}

func (h *handler) CreateWebhook(
	ctx context.Context, input *CreateWebhookInput,
) (*GetWebhookOutput, error) {
	user, err := h.requireMaintainer(ctx, input.ProjectID)
	if err != nil {
		return nil, err
	}

	body := &input.Body
	events := body.Events
	if events == "" {
		events = "*"
	}
	if events != "*" {
		for _, e := range strings.Split(events, ",") {
			if !validEventCategories[strings.TrimSpace(e)] {
				return nil, huma.Error400BadRequest("Invalid event category.")
			}
		}
	}

	active := true
	if body.Active != nil {
		active = *body.Active
	}

	hook := db.Webhook{
		ProjectID: input.ProjectID,
		URL:       body.URL,
		Secret:    body.Secret,
		Events:    events,
		Active:    active,
		CreatorID: user.ID,
		Created:   time.Now(),
	}
	if err := db.New(ctx, h.db).Insert(&hook); err != nil {
		return nil, huma.Error400BadRequest("Webhook creation failed.")
	}

	base := h.apiBase(ctx)
	return &GetWebhookOutput{
		Body: webhookToResponse(&hook, input.ProjectID, base),
	}, nil
}

type UpdateWebhookInput struct {
	ProjectID int               `path:"projectID" doc:"Project ID"`
	WebhookID int               `path:"webhookID" doc:"Webhook ID"`
	Body      WebhookUpdateBody `json:"body"`
}

func (h *handler) UpdateWebhook(
	ctx context.Context, input *UpdateWebhookInput,
) (*GetWebhookOutput, error) {
	if _, err := h.requireMaintainer(ctx, input.ProjectID); err != nil {
		return nil, err
	}

	q := db.GetQueries(ctx)
	var hook db.Webhook
	if err := q.DB.NewSelect().Model(&hook).
		Where("id = ?", input.WebhookID).
		Where("project_id = ?", input.ProjectID).
		Scan(ctx); err != nil {
		return nil, huma.Error404NotFound("Not found.")
	}

	body := &input.Body
	uq := q.DB.NewUpdate().Model(&hook).Where("id = ?", input.WebhookID)
	if body.URL != nil {
		uq = uq.Set("url = ?", *body.URL)
	}
	if body.Secret != nil {
		uq = uq.Set("secret = ?", *body.Secret)
	}
	if body.Events != nil {
		uq = uq.Set("events = ?", *body.Events)
	}
	if body.Active != nil {
		uq = uq.Set("active = ?", *body.Active)
	}
	if _, err := uq.Exec(ctx); err != nil {
		return nil, huma.Error400BadRequest("Update failed.")
	}

	q.DB.NewSelect().Model(&hook).Where("id = ?", input.WebhookID).Scan(ctx)

	base := h.apiBase(ctx)
	return &GetWebhookOutput{
		Body: webhookToResponse(&hook, input.ProjectID, base),
	}, nil
}

type DeleteWebhookInput struct {
	ProjectID int `path:"projectID" doc:"Project ID"`
	WebhookID int `path:"webhookID" doc:"Webhook ID"`
}

type DeleteWebhookOutput struct{}

func (h *handler) DeleteWebhook(
	ctx context.Context, input *DeleteWebhookInput,
) (*DeleteWebhookOutput, error) {
	if _, err := h.requireMaintainer(ctx, input.ProjectID); err != nil {
		return nil, err
	}

	q := db.GetQueries(ctx)
	if _, err := q.DB.NewDelete().Model((*db.Webhook)(nil)).
		Where("id = ?", input.WebhookID).
		Where("project_id = ?", input.ProjectID).
		Exec(ctx); err != nil {
		return nil, huma.Error400BadRequest("Delete failed.")
	}

	return nil, nil
}

func webhookToResponse(w *db.Webhook, _ int, _ string) WebhookResponse {
	return WebhookResponse{
		ID:      w.ID,
		URL:     w.URL,
		Events:  w.Events,
		Active:  w.Active,
		Created: w.Created,
	}
}
