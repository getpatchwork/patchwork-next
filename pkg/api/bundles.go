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

func registerBundleRoutes(api huma.API, h *handler, prefix string, mw huma.Middlewares) {
	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: prefix + "/bundles",
		OperationID: fmt.Sprintf("list-bundles-v%s", prefix[5:]),
		Middlewares: mw,
	}, h.ListBundles)
	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: prefix + "/bundles/{id}",
		OperationID: fmt.Sprintf("get-bundle-v%s", prefix[5:]),
		Middlewares: mw,
	}, h.GetBundle)
	huma.Register(api, huma.Operation{
		Method: http.MethodPost, Path: prefix + "/bundles",
		OperationID:   fmt.Sprintf("create-bundle-v%s", prefix[5:]),
		DefaultStatus: 201,
		Security:      authRequired,
		Middlewares:   mw,
	}, h.CreateBundle)
	huma.Register(api, huma.Operation{
		Method: http.MethodPatch, Path: prefix + "/bundles/{id}",
		OperationID: fmt.Sprintf("update-bundle-v%s", prefix[5:]),
		Security:    authRequired,
		Middlewares: mw,
	}, h.UpdateBundle)
	huma.Register(api, huma.Operation{
		Method: http.MethodPut, Path: prefix + "/bundles/{id}",
		OperationID: fmt.Sprintf("put-bundle-v%s", prefix[5:]),
		Security:    authRequired,
		Middlewares: mw,
	}, h.UpdateBundle)
	huma.Register(api, huma.Operation{
		Method: http.MethodDelete, Path: prefix + "/bundles/{id}",
		OperationID:   fmt.Sprintf("delete-bundle-v%s", prefix[5:]),
		DefaultStatus: 204,
		Security:      authRequired,
		Middlewares:   mw,
	}, h.DeleteBundle)
}

type ListBundlesInput struct {
	PageParams
	Project string `query:"project" doc:"Project ID or linkname"`
	Owner   string `query:"owner" doc:"Owner ID or username"`
	Public  string `query:"public" enum:"true,false," doc:"Public filter"`
}

type ListBundlesOutput struct {
	Link string           `header:"Link" doc:"Pagination links"`
	Body []BundleResponse `json:"body"`
}

func (h *handler) ListBundles(
	ctx context.Context, input *ListBundlesInput,
) (*ListBundlesOutput, error) {
	base := h.apiBase(ctx)

	idb := db.GetQueries(ctx).DB

	query := idb.NewSelect().Model((*db.Bundle)(nil))

	user := getAuthUser(ctx)
	if user != nil {
		query = query.Where("bundle.public = ? OR bundle.owner_id = ?", true, user.ID)
	} else {
		query = query.Where("bundle.public = ?", true)
	}

	query = applyBundleFilters(query, input)

	total, err := query.Count(ctx)
	if err != nil {
		log.Errorf("count bundles: %v", err)
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

	var bundles []db.Bundle
	if err := query.Model(&bundles).
		Relation("Owner").Relation("Project").
		OrderExpr("bundle.id ASC").Offset(offset).Limit(perPage).Scan(ctx); err != nil {
		log.Errorf("list bundles: %v", err)
		return nil, huma.Error500InternalServerError("Internal error.")
	}

	resp := &ListBundlesOutput{
		Link: buildLinkHeader(input.Page, perPage, total),
		Body: make([]BundleResponse, len(bundles)),
	}
	for i := range bundles {
		resp.Body[i] = bundleToResponse(&bundles[i], base)
	}
	return resp, nil
}

type GetBundleInput struct {
	ID int `path:"id" doc:"Bundle ID"`
}

type GetBundleOutput struct {
	Body BundleResponse
}

func (h *handler) GetBundle(
	ctx context.Context, input *GetBundleInput,
) (*GetBundleOutput, error) {
	base := h.apiBase(ctx)
	idb := db.GetQueries(ctx).DB

	var bundle db.Bundle
	if err := idb.NewSelect().Model(&bundle).
		Relation("Owner").Relation("Project").
		Where("bundle.id = ?", input.ID).Scan(ctx); err != nil {
		return nil, huma.Error404NotFound("Not found.")
	}

	bundles := []db.Bundle{bundle}
	loadBundlePatches(ctx, idb, bundles)

	return &GetBundleOutput{
		Body: bundleToResponse(&bundles[0], base),
	}, nil
}

type CreateBundleInput struct {
	Body BundleCreateUpdateBody `json:"body"`
}

func (h *handler) CreateBundle(
	ctx context.Context, input *CreateBundleInput,
) (*GetBundleOutput, error) {
	user, err := h.requireUser(ctx)
	if err != nil {
		return nil, err
	}

	body := &input.Body
	if body.Patches == nil || len(*body.Patches) == 0 {
		return nil, huma.Error400BadRequest("Bundles cannot be empty.")
	}
	if body.Name == nil || *body.Name == "" {
		return nil, huma.Error400BadRequest("A name is required.")
	}

	q := db.GetQueries(ctx)
	patchIDs := *body.Patches
	projectID, err := validateBundlePatches(ctx, q.DB, patchIDs)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}

	public := false
	if body.Public != nil {
		public = *body.Public
	}

	bundle := db.Bundle{
		OwnerID:   user.ID,
		ProjectID: projectID,
		Name:      *body.Name,
		Public:    public,
	}
	if err := q.Insert(&bundle); err != nil {
		return nil, huma.Error400BadRequest("Bundle creation failed.")
	}

	if err := insertBundlePatches(ctx, q.DB, bundle.ID, patchIDs); err != nil {
		return nil, huma.Error400BadRequest("Bundle creation failed.")
	}

	// Re-fetch with relations.
	if err := q.DB.NewSelect().Model(&bundle).
		Relation("Owner").Relation("Project").
		Where("bundle.id = ?", bundle.ID).Scan(ctx); err != nil {
		return nil, huma.Error400BadRequest("Bundle creation failed.")
	}

	bundles := []db.Bundle{bundle}
	loadBundlePatches(ctx, q.DB, bundles)

	base := h.apiBase(ctx)
	return &GetBundleOutput{
		Body: bundleToResponse(&bundles[0], base),
	}, nil
}

type UpdateBundleInput struct {
	ID   int                    `path:"id" doc:"Bundle ID"`
	Body BundleCreateUpdateBody `json:"body"`
}

func (h *handler) UpdateBundle(
	ctx context.Context, input *UpdateBundleInput,
) (*GetBundleOutput, error) {
	user, err := h.requireUser(ctx)
	if err != nil {
		return nil, err
	}

	idb := db.GetQueries(ctx).DB
	var bundle db.Bundle
	if err := idb.NewSelect().Model(&bundle).
		Where("id = ?", input.ID).Scan(ctx); err != nil {
		return nil, huma.Error404NotFound("Not found.")
	}

	if bundle.OwnerID != user.ID {
		return nil, ForbiddenErr
	}

	body := &input.Body

	if body.Patches != nil {
		if len(*body.Patches) == 0 {
			return nil, huma.Error400BadRequest("Bundles cannot be empty.")
		}
		projectID, err := validateBundlePatches(ctx, idb, *body.Patches)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		if _, err := idb.NewDelete().Model((*db.BundlePatch)(nil)).
			Where("bundle_id = ?", input.ID).Exec(ctx); err != nil {
			return nil, huma.Error400BadRequest("Update failed.")
		}
		if err := insertBundlePatches(ctx, idb, input.ID, *body.Patches); err != nil {
			return nil, huma.Error400BadRequest("Update failed.")
		}
		bundle.ProjectID = projectID
	}

	up := idb.NewUpdate().Model(&bundle).Where("id = ?", input.ID)
	if body.Name != nil {
		up = up.Set("name = ?", *body.Name)
	}
	if body.Public != nil {
		up = up.Set("public = ?", *body.Public)
	}
	if body.Patches != nil {
		up = up.Set("project_id = ?", bundle.ProjectID)
	}
	if _, err := up.Exec(ctx); err != nil {
		return nil, huma.Error400BadRequest("Update failed.")
	}

	if err := idb.NewSelect().Model(&bundle).
		Relation("Owner").Relation("Project").
		Where("bundle.id = ?", input.ID).Scan(ctx); err != nil {
		log.Errorf("get bundle: %v", err)
		return nil, huma.Error500InternalServerError("Internal error.")
	}

	bundles := []db.Bundle{bundle}
	loadBundlePatches(ctx, idb, bundles)

	base := h.apiBase(ctx)
	return &GetBundleOutput{
		Body: bundleToResponse(&bundles[0], base),
	}, nil
}

type DeleteBundleInput struct {
	ID int `path:"id" doc:"Bundle ID"`
}

type DeleteBundleOutput struct{}

func (h *handler) DeleteBundle(
	ctx context.Context, input *DeleteBundleInput,
) (*DeleteBundleOutput, error) {
	user, err := h.requireUser(ctx)
	if err != nil {
		return nil, err
	}

	idb := db.GetQueries(ctx).DB

	var bundle db.Bundle
	if err := idb.NewSelect().Model(&bundle).
		Where("id = ?", input.ID).Scan(ctx); err != nil {
		return nil, huma.Error404NotFound("Not found.")
	}

	if bundle.OwnerID != user.ID {
		return nil, ForbiddenErr
	}

	if _, err := idb.NewDelete().Model((*db.BundlePatch)(nil)).
		Where("bundle_id = ?", input.ID).Exec(ctx); err != nil {
		return nil, huma.Error400BadRequest("Delete failed.")
	}
	if _, err := idb.NewDelete().Model((*db.Bundle)(nil)).
		Where("id = ?", input.ID).Exec(ctx); err != nil {
		return nil, huma.Error400BadRequest("Delete failed.")
	}

	return nil, nil
}

func applyBundleFilters(q *bun.SelectQuery, input *ListBundlesInput) *bun.SelectQuery {
	if input.Project != "" {
		if id, err := strconv.Atoi(input.Project); err == nil {
			q = q.Where("bundle.project_id = ?", id)
		} else {
			q = q.Where("bundle.project_id IN (SELECT id FROM project WHERE linkname = ?)", input.Project)
		}
	}
	if input.Owner != "" {
		if id, err := strconv.Atoi(input.Owner); err == nil {
			q = q.Where("bundle.owner_id = ?", id)
		} else {
			q = q.Where("bundle.owner_id IN (SELECT id FROM user WHERE username = ?)", input.Owner)
		}
	}
	if input.Public != "" {
		q = q.Where("bundle.public = ?", input.Public == "true")
	}
	return q
}

func validateBundlePatches(ctx context.Context, idb bun.IDB, patchIDs []int) (int, error) {
	var projectIDs []int
	idb.NewSelect().Model((*db.Patch)(nil)).
		Column("project_id").
		Where("id IN ?", bun.Tuple(patchIDs)).
		Scan(ctx, &projectIDs)

	if len(projectIDs) == 0 {
		return 0, fmt.Errorf("invalid patch IDs")
	}

	projectID := projectIDs[0]
	for _, pid := range projectIDs[1:] {
		if pid != projectID {
			return 0, fmt.Errorf("bundle patches must belong to the same project")
		}
	}

	return projectID, nil
}

func insertBundlePatches(ctx context.Context, idb bun.IDB, bundleID int, patchIDs []int) error {
	for i, pid := range patchIDs {
		bp := db.BundlePatch{
			BundleID: bundleID,
			PatchID:  pid,
			Order:    int(i),
		}
		if _, err := idb.NewInsert().Model(&bp).Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}

func bundleToResponse(b *db.Bundle, base string) BundleResponse {
	r := BundleResponse{
		ID:      b.ID,
		URL:     fmt.Sprintf("%s/bundles/%d/", base, b.ID),
		Name:    b.Name,
		Public:  b.Public,
		Mbox:    fmt.Sprintf("%s/bundles/%d/mbox/", base, b.ID),
		Patches: make([]PatchEmbedded, len(b.BundlePatches)),
	}
	if b.Project != nil {
		r.Project = projectToEmbedded(b.Project, base)
		if b.Project.WebURL != "" {
			r.WebURL = strp(fmt.Sprintf("%s/bundle/%s/%s/",
				b.Project.WebURL, b.Owner.Username, b.Name))
		}
	}
	if b.Owner != nil {
		u := userToEmbedded(b.Owner, base)
		r.Owner = &u
	}
	for i := range b.BundlePatches {
		r.Patches[i] = patchToEmbedded(&b.BundlePatches[i], b.Project, base)
	}
	return r
}
