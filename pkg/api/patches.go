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
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/db"
	"github.com/getpatchwork/patchwork/pkg/events"
)

func registerPatchRoutes(api huma.API, h *handler, prefix string, mw huma.Middlewares) {
	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: prefix + "/patches",
		OperationID: fmt.Sprintf("list-patches-v%s", prefix[5:]),
		Middlewares: mw,
	}, h.ListPatches)
	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: prefix + "/patches/{id}",
		OperationID: fmt.Sprintf("get-patch-v%s", prefix[5:]),
		Middlewares: mw,
	}, h.GetPatch)
	huma.Register(api, huma.Operation{
		Method: http.MethodPatch, Path: prefix + "/patches/{id}",
		OperationID: fmt.Sprintf("update-patch-v%s", prefix[5:]),
		Security:    authRequired,
		Middlewares: mw,
	}, h.UpdatePatch)
	huma.Register(api, huma.Operation{
		Method: http.MethodPut, Path: prefix + "/patches/{id}",
		OperationID: fmt.Sprintf("put-patch-v%s", prefix[5:]),
		Security:    authRequired,
		Middlewares: mw,
	}, h.UpdatePatch)
}

type ListPatchesInput struct {
	PageParams
	SearchParams
	Project   string `query:"project" doc:"Project ID or linkname"`
	Series    int    `query:"series" doc:"Series ID"`
	Submitter string `query:"submitter" doc:"Submitter ID or email"`
	Delegate  string `query:"delegate" doc:"Delegate ID or username"`
	State     string `query:"state" doc:"State slug"`
	Archived  string `query:"archived" enum:"true,false," doc:"Archived filter"`
	Hash      string `query:"hash" doc:"Patch hash"`
	Msgid     string `query:"msgid" doc:"Message ID"`
	Since     string `query:"since" doc:"Earliest date"`
	Before    string `query:"before" doc:"Latest date"`
}

type ListPatchesOutput struct {
	Link string              `header:"Link" doc:"Pagination links"`
	Body []PatchListResponse `json:"body"`
}

func (h *handler) ListPatches(
	ctx context.Context, input *ListPatchesInput,
) (*ListPatchesOutput, error) {
	base := h.apiBase(ctx)

	idb := db.GetQueries(ctx).DB
	sq := idb.NewSelect().Model((*db.Patch)(nil))
	sq = applyPatchFilters(sq, input)

	total, _ := sq.Count(ctx)

	perPage := input.PerPage
	if perPage > maxPerPage {
		perPage = maxPerPage
	}
	offset := (input.Page - 1) * perPage

	var patches []db.Patch
	sq.Model(&patches).
		Relation("Submitter").Relation("Project").Relation("State").Relation("Delegate").
		OrderExpr("patch.id DESC").Offset(offset).Limit(perPage).Scan(ctx)
	loadPatchTags(ctx, idb, patches)
	loadPatchSeries(ctx, idb, patches)
	loadCombinedCheck(ctx, idb, patches)
	loadPatchRelated(ctx, idb, patches)

	resp := &ListPatchesOutput{
		Link: buildLinkHeader(input.Page, perPage, total),
		Body: make([]PatchListResponse, len(patches)),
	}
	for i := range patches {
		resp.Body[i] = patchToListResponse(&patches[i], base)
	}
	return resp, nil
}

type GetPatchInput struct {
	ID int `path:"id" doc:"Patch ID"`
}

type GetPatchOutput struct {
	Body PatchDetailResponse
}

type UpdatePatchInput struct {
	ID   int             `path:"id" doc:"Patch ID"`
	Body PatchUpdateBody `json:"body"`
}

func (h *handler) UpdatePatch(
	ctx context.Context, input *UpdatePatchInput,
) (*GetPatchOutput, error) {
	user, err := h.requireUser(ctx)
	if err != nil {
		return nil, err
	}

	base := h.apiBase(ctx)
	q := db.GetQueries(ctx)

	var patch db.Patch
	if err := q.DB.NewSelect().Model(&patch).
		Relation("Submitter").Relation("Project").Relation("State").Relation("Delegate").
		Where("patch.id = ?", input.ID).Scan(ctx); err != nil {
		return nil, huma.Error404NotFound("Not found.")
	}

	if !q.IsMaintainer(user, patch.ProjectID) {
		return nil, ForbiddenErr
	}

	body := &input.Body
	uq := q.DB.NewUpdate().Model(&patch).Where("id = ?", input.ID)
	changed := false

	oldStateID := patch.StateID
	oldDelegateID := patch.DelegateID
	stateChanged := false
	delegateChanged := false

	if body.State != nil {
		var state db.State
		err := q.DB.NewSelect().Model(&state).
			Where("LOWER(name) = LOWER(?)", *body.State).
			Scan(ctx)
		if err != nil {
			return nil, huma.Error400BadRequest("Invalid state.")
		}
		if patch.StateID == nil || *patch.StateID != state.ID {
			stateChanged = true
		}
		uq = uq.Set("state_id = ?", state.ID)
		patch.StateID = &state.ID
		patch.State = &state
		changed = true
	}
	if body.Delegate != nil {
		if patch.DelegateID == nil || *patch.DelegateID != *body.Delegate {
			delegateChanged = true
		}
		uq = uq.Set("delegate_id = ?", *body.Delegate)
		patch.DelegateID = body.Delegate
		changed = true
	}
	if body.Archived != nil {
		uq = uq.Set("archived = ?", *body.Archived)
		patch.Archived = *body.Archived
		changed = true
	}
	if body.CommitRef != nil {
		uq = uq.Set("commit_ref = ?", *body.CommitRef)
		patch.CommitRef = body.CommitRef
		changed = true
	}
	if body.PullURL != nil {
		uq = uq.Set("pull_url = ?", *body.PullURL)
		patch.PullURL = body.PullURL
		changed = true
	}

	if changed {
		if _, err := uq.Exec(ctx); err != nil {
			return nil, huma.Error400BadRequest("Update failed.")
		}
	}

	if stateChanged {
		q.EnqueueEvent(db.Event{
			Category:        "patch-state-changed",
			ProjectID:       patch.ProjectID,
			PatchID:         &patch.ID,
			Date:            time.Now(),
			ActorID:         &user.ID,
			PreviousStateID: oldStateID,
			CurrentStateID:  patch.StateID,
		})
		if patch.State != nil {
			var oldStateName string
			if oldStateID != nil {
				var s db.State
				if q.DB.NewSelect().Model(&s).
					Where("id = ?", *oldStateID).
					Scan(ctx) == nil {
					oldStateName = s.Name
				}
			}
			events.PatchStateChanged(
				context.Background(), h.cfg, q.DB,
				&patch, user.ID, oldStateName, patch.State.Name,
			)
		}
	}
	if delegateChanged {
		q.EnqueueEvent(db.Event{
			Category:           "patch-delegated",
			ProjectID:          patch.ProjectID,
			PatchID:            &patch.ID,
			Date:               time.Now(),
			ActorID:            &user.ID,
			PreviousDelegateID: oldDelegateID,
			CurrentDelegateID:  patch.DelegateID,
		})
	}

	if body.Related != nil {
		if err := updateRelated(ctx, q.DB, user, &patch, *body.Related); err != nil {
			msg := err.Error()
			if msg == "forbidden" {
				return nil, huma.Error403Forbidden(msg)
			}
			if msg == "conflict" {
				return nil, huma.Error409Conflict(msg)
			}
			return nil, huma.Error400BadRequest(msg)
		}
	}

	// Re-fetch with relations to pick up any updated fields.
	if err := q.DB.NewSelect().Model(&patch).
		Relation("Submitter").Relation("Project").Relation("State").Relation("Delegate").
		Where("patch.id = ?", input.ID).Scan(ctx); err != nil {
		return nil, huma.Error404NotFound("Not found.")
	}

	patches := []db.Patch{patch}
	loadPatchTags(ctx, q.DB, patches)
	loadPatchSeries(ctx, q.DB, patches)
	loadCombinedCheck(ctx, q.DB, patches)
	loadPatchRelated(ctx, q.DB, patches)

	return &GetPatchOutput{
		Body: patchToDetailResponse(&patches[0], base),
	}, nil
}

func (h *handler) GetPatch(
	ctx context.Context, input *GetPatchInput,
) (*GetPatchOutput, error) {
	idb := db.GetQueries(ctx).DB
	base := h.apiBase(ctx)

	var patch db.Patch
	if err := idb.NewSelect().Model(&patch).
		Relation("Submitter").Relation("Project").Relation("State").Relation("Delegate").
		Where("patch.id = ?", input.ID).Scan(ctx); err != nil {
		return nil, huma.Error404NotFound("Not found.")
	}

	patches := []db.Patch{patch}
	loadPatchTags(ctx, idb, patches)
	loadPatchSeries(ctx, idb, patches)
	loadCombinedCheck(ctx, idb, patches)
	loadPatchRelated(ctx, idb, patches)

	return &GetPatchOutput{
		Body: patchToDetailResponse(&patches[0], base),
	}, nil
}

func applyPatchFilters(q *bun.SelectQuery, input *ListPatchesInput) *bun.SelectQuery {
	if input.Project != "" {
		if id, err := strconv.Atoi(input.Project); err == nil {
			q = q.Where("patch.project_id = ?", id)
		} else {
			q = q.Where("patch.project_id IN (SELECT id FROM project WHERE linkname = ?)", input.Project)
		}
	}
	if input.Series != 0 {
		q = q.Where("patch.series_id = ?", input.Series)
	}
	if input.Submitter != "" {
		if id, err := strconv.Atoi(input.Submitter); err == nil {
			q = q.Where("patch.submitter_id = ?", id)
		} else {
			q = q.Where("patch.submitter_id IN (SELECT id FROM person WHERE email = ?)", input.Submitter)
		}
	}
	if input.Delegate != "" {
		if id, err := strconv.Atoi(input.Delegate); err == nil {
			q = q.Where("patch.delegate_id = ?", id)
		} else {
			q = q.Where("patch.delegate_id IN (SELECT id FROM auth_user WHERE username = ?)", input.Delegate)
		}
	}
	if input.State != "" {
		if id, err := strconv.Atoi(input.State); err == nil {
			q = q.Where("patch.state_id = ?", id)
		} else {
			q = q.Where("patch.state_id IN (SELECT id FROM state WHERE LOWER(name) = LOWER(?))", input.State)
		}
	}
	if input.Archived != "" {
		q = q.Where("patch.archived = ?", input.Archived == "true")
	}
	if input.Hash != "" {
		q = q.Where("LOWER(patch.hash) = LOWER(?)", input.Hash)
	}
	if input.Msgid != "" {
		q = q.Where("patch.msgid = ?", "<"+input.Msgid+">")
	}
	if input.Since != "" {
		q = q.Where("patch.date >= ?", input.Since)
	}
	if input.Before != "" {
		q = q.Where("patch.date < ?", input.Before)
	}
	if input.Q != "" {
		q = q.Where("patch.name LIKE ?", "%"+input.Q+"%")
	}
	return q
}

func patchToListResponse(p *db.Patch, base string) PatchListResponse {
	r := PatchListResponse{
		ID:        p.ID,
		URL:       fmt.Sprintf("%s/patches/%d/", base, p.ID),
		Msgid:     p.Msgid,
		Date:      p.Date,
		Name:      p.Name,
		CommitRef: p.CommitRef,
		PullURL:   p.PullURL,
		Archived:  p.Archived,
		Mbox:      fmt.Sprintf("%s/patches/%d/mbox/", base, p.ID),
		Comments:  strp(fmt.Sprintf("%s/patches/%d/comments/", base, p.ID)),
		Checks:    fmt.Sprintf("%s/patches/%d/checks/", base, p.ID),
		Check:     "pending",
		Tags:      p.Tags,
		Series:    []SeriesEmbedded{},
		Related:   []PatchEmbedded{},
	}
	if p.Hash != nil {
		r.Hash = *p.Hash
	}
	if p.State != nil {
		r.State = p.State.Name
	}
	if p.Project != nil {
		r.Project = projectToEmbedded(p.Project, base)
		if p.Project.WebURL != "" {
			r.WebURL = strp(fmt.Sprintf("%s/patch/%s/",
				p.Project.WebURL, p.Msgid))
		}
	}
	if p.Submitter != nil {
		r.Submitter = personToEmbedded(p.Submitter, base)
	}
	if p.Delegate != nil {
		d := userToEmbedded(p.Delegate, base)
		r.Delegate = &d
	}
	if p.CombinedCheck != nil {
		r.Check = *p.CombinedCheck
	}
	if p.Tags == nil {
		r.Tags = map[string]int{}
	}
	for _, ref := range p.Related {
		r.Related = append(r.Related, PatchEmbedded{
			ID:   ref.ID,
			URL:  fmt.Sprintf("%s/patches/%d/", base, ref.ID),
			Name: ref.Name,
		})
	}
	for _, ref := range p.SeriesList {
		r.Series = append(r.Series, SeriesEmbedded{
			ID:   ref.ID,
			URL:  fmt.Sprintf("%s/series/%d/", base, ref.ID),
			Name: ref.Name,
			Mbox: fmt.Sprintf("%s/series/%d/mbox/", base, ref.ID),
		})
	}
	if r.Related == nil {
		r.Related = []PatchEmbedded{}
	}
	if r.Series == nil {
		r.Series = []SeriesEmbedded{}
	}
	return r
}

func patchToDetailResponse(p *db.Patch, base string) PatchDetailResponse {
	r := PatchDetailResponse{
		PatchListResponse: patchToListResponse(p, base),
	}
	if p.Content != nil {
		r.Content = *p.Content
	}
	if p.Diff != nil {
		r.Diff = *p.Diff
	}
	return r
}
