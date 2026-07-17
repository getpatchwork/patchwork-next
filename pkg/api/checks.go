// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/getpatchwork/patchwork/pkg/db"
	"github.com/getpatchwork/patchwork/pkg/log"
)

var checkStates = map[string]db.CheckState{
	"pending": db.CheckPending,
	"success": db.CheckSuccess,
	"warning": db.CheckWarning,
	"fail":    db.CheckFail,
}

var checkStateNames = map[db.CheckState]string{
	db.CheckPending: "pending",
	db.CheckSuccess: "success",
	db.CheckWarning: "warning",
	db.CheckFail:    "fail",
}

func registerCheckRoutes(api huma.API, h *handler, prefix string, mw huma.Middlewares) {
	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: prefix + "/patches/{patch_id}/checks",
		OperationID: fmt.Sprintf("list-checks-v%s", prefix[5:]),
		Middlewares: mw,
	}, h.ListChecks)
	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: prefix + "/patches/{patch_id}/checks/{check_id}",
		OperationID: fmt.Sprintf("get-check-v%s", prefix[5:]),
		Middlewares: mw,
	}, h.GetCheck)
	huma.Register(api, huma.Operation{
		Method: http.MethodPost, Path: prefix + "/patches/{patch_id}/checks",
		OperationID:   fmt.Sprintf("create-check-v%s", prefix[5:]),
		DefaultStatus: 201,
		Security:      authRequired,
		Middlewares:   mw,
	}, h.CreateCheck)
	huma.Register(api, huma.Operation{
		Method: http.MethodPatch, Path: prefix + "/patches/{patch_id}/checks/{check_id}",
		OperationID: fmt.Sprintf("update-check-v%s", prefix[5:]),
		Security:    authRequired,
		Middlewares: mw,
	}, h.UpdateCheck)
}

type ListChecksInput struct {
	PageParams
	PatchID int `path:"patch_id" doc:"Patch ID"`
}

type ListChecksOutput struct {
	Link string          `header:"Link" doc:"Pagination links"`
	Body []CheckResponse `json:"body"`
}

func (h *handler) ListChecks(
	ctx context.Context, input *ListChecksInput,
) (*ListChecksOutput, error) {
	idb := db.GetQueries(ctx).DB
	base := h.apiBase(ctx)

	total, err := idb.NewSelect().Model((*db.Check)(nil)).
		Where("patch_id = ?", input.PatchID).
		Count(ctx)
	if err != nil {
		log.Errorf("count checks: %v", err)
		return nil, huma.Error500InternalServerError("Internal error.")
	}

	perPage := input.PerPage
	if perPage > maxPerPage {
		perPage = maxPerPage
	}
	offset := (input.Page - 1) * perPage

	var checks []db.Check
	if err := idb.NewSelect().Model(&checks).
		Relation("User").
		Where(`ci_check.patch_id = ?`, input.PatchID).
		OrderExpr(`ci_check.id ASC`).Offset(offset).Limit(perPage).Scan(ctx); err != nil {
		log.Errorf("list checks: %v", err)
		return nil, huma.Error500InternalServerError("Internal error.")
	}
	setCheckURLs(base, input.PatchID, checks)

	resp := &ListChecksOutput{
		Link: buildLinkHeader(input.Page, perPage, total),
		Body: make([]CheckResponse, len(checks)),
	}
	for i := range checks {
		resp.Body[i] = checkToResponse(&checks[i], base)
	}
	return resp, nil
}

type GetCheckInput struct {
	PatchID int `path:"patch_id" doc:"Patch ID"`
	CheckID int `path:"check_id" doc:"Check ID"`
}

type GetCheckOutput struct {
	Body CheckResponse
}

func (h *handler) GetCheck(
	ctx context.Context, input *GetCheckInput,
) (*GetCheckOutput, error) {
	base := h.apiBase(ctx)

	var c db.Check
	err := db.GetQueries(ctx).DB.NewSelect().Model(&c).
		Relation("User").
		Where(`ci_check.id = ?`, input.CheckID).
		Where(`ci_check.patch_id = ?`, input.PatchID).
		Scan(ctx)
	if err != nil {
		return nil, huma.Error404NotFound("Not found.")
	}

	checks := []db.Check{c}
	setCheckURLs(base, input.PatchID, checks)

	return &GetCheckOutput{
		Body: checkToResponse(&checks[0], base),
	}, nil
}

type CreateCheckInput struct {
	PatchID int `path:"patch_id" doc:"Patch ID"`
	Body    CheckCreateBody
}

type CreateCheckOutput struct {
	Body CheckResponse
}

func (h *handler) CreateCheck(
	ctx context.Context, input *CreateCheckInput,
) (*CreateCheckOutput, error) {
	q := db.GetQueries(ctx)
	base := h.apiBase(ctx)

	user, err := h.requireUser(ctx)
	if err != nil {
		return nil, err
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

	stateVal, ok := checkStates[input.Body.State]
	if !ok {
		return nil, huma.Error400BadRequest("Invalid state.")
	}

	targetURL := ""
	if input.Body.TargetURL != nil {
		targetURL = *input.Body.TargetURL
	}
	description := ""
	if input.Body.Description != nil {
		description = *input.Body.Description
	}

	// upsert: if a check with the same (patch, context, user) exists,
	// update it instead of creating a duplicate
	var check db.Check
	var eventCategory string
	err = q.DB.NewSelect().Model(&check).
		Where("patch_id = ?", input.PatchID).
		Where("context = ?", input.Body.Context).
		Where("user_id = ?", user.ID).
		Scan(ctx)
	if err == nil {
		eventCategory = "check-updated"
		check.State = stateVal
		check.Date = time.Now()
		check.TargetURL = targetURL
		check.Description = description
		_, err = q.DB.NewUpdate().Model(&check).
			Where("id = ?", check.ID).
			Set("state = ?", check.State).
			Set("date = ?", check.Date).
			Set("target_url = ?", check.TargetURL).
			Set("description = ?", check.Description).
			Exec(ctx)
		if err != nil {
			return nil, huma.Error400BadRequest("Update failed.")
		}
	} else {
		eventCategory = "check-created"
		check = db.Check{
			PatchID:     input.PatchID,
			UserID:      &user.ID,
			Date:        time.Now(),
			State:       stateVal,
			TargetURL:   targetURL,
			Context:     input.Body.Context,
			Description: description,
		}
		if err := q.Insert(&check); err != nil {
			return nil, huma.Error400BadRequest("Create failed.")
		}
	}

	q.EnqueueEvent(db.Event{
		Category:       eventCategory,
		ProjectID:      patch.ProjectID,
		PatchID:        &input.PatchID,
		CreatedCheckID: &check.ID,
		Date:           check.Date,
		ActorID:        &user.ID,
	})

	check.User = user
	checks := []db.Check{check}
	setCheckURLs(base, input.PatchID, checks)

	return &CreateCheckOutput{
		Body: checkToResponse(&checks[0], base),
	}, nil
}

type UpdateCheckInput struct {
	PatchID int `path:"patch_id" doc:"Patch ID"`
	CheckID int `path:"check_id" doc:"Check ID"`
	Body    CheckCreateBody
}

func (h *handler) UpdateCheck(
	ctx context.Context, input *UpdateCheckInput,
) (*GetCheckOutput, error) {
	q := db.GetQueries(ctx)
	base := h.apiBase(ctx)

	user, err := h.requireUser(ctx)
	if err != nil {
		return nil, err
	}

	var check db.Check
	if err := q.DB.NewSelect().Model(&check).
		Relation("User").
		Where("ci_check.id = ?", input.CheckID).
		Where("ci_check.patch_id = ?", input.PatchID).
		Scan(ctx); err != nil {
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

	stateVal, ok := checkStates[input.Body.State]
	if !ok {
		return nil, huma.Error400BadRequest("Invalid state.")
	}

	uq := q.DB.NewUpdate().Model(&check).Where("id = ?", check.ID).
		Set("state = ?", stateVal).
		Set("date = ?", time.Now())
	if input.Body.TargetURL != nil {
		uq = uq.Set("target_url = ?", *input.Body.TargetURL)
	}
	if input.Body.Description != nil {
		uq = uq.Set("description = ?", *input.Body.Description)
	}
	if _, err := uq.Exec(ctx); err != nil {
		return nil, huma.Error400BadRequest("Update failed.")
	}

	q.EnqueueEvent(db.Event{
		Category:       "check-updated",
		ProjectID:      patch.ProjectID,
		PatchID:        &input.PatchID,
		CreatedCheckID: &check.ID,
		Date:           time.Now(),
		ActorID:        &user.ID,
	})

	// re-fetch
	if err := q.DB.NewSelect().Model(&check).Relation("User").
		Where("ci_check.id = ?", check.ID).Scan(ctx); err != nil {
		log.Errorf("get check: %v", err)
		return nil, huma.Error500InternalServerError("Internal error.")
	}

	checks := []db.Check{check}
	setCheckURLs(base, input.PatchID, checks)

	return &GetCheckOutput{
		Body: checkToResponse(&checks[0], base),
	}, nil
}

func checkToResponse(c *db.Check, base string) CheckResponse {
	r := CheckResponse{
		ID:      c.ID,
		URL:     c.URL,
		Date:    c.Date,
		State:   checkStateNames[c.State],
		Context: c.Context,
	}
	if c.TargetURL != "" {
		r.TargetURL = &c.TargetURL
	}
	if c.Description != "" {
		r.Description = &c.Description
	}
	if c.User != nil {
		u := userToEmbedded(c.User, base)
		r.User = &u
	}
	return r
}
