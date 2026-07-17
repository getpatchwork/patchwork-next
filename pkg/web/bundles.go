// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package web

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/db"
)

type bundleListData struct {
	PC      pageContext
	Bundles []db.Bundle
}

func (h *webHandler) BundleList(w http.ResponseWriter, r *http.Request) {
	if !requireLogin(w, r) {
		return
	}
	ctx := r.Context()
	q := db.GetQueries(ctx)
	user := getWebUser(r)
	pc := h.pageCtx(r)

	var bundles []db.Bundle
	q.DB.NewSelect().Model(&bundles).
		ColumnExpr("*, (SELECT count(*) FROM bundle_patch WHERE bundle_id = bundle.id) AS patch_count").
		Relation("Owner").Relation("Project").
		Where("owner_id = ?", user.ID).
		OrderExpr("name ASC").
		Scan(q.Ctx)

	bundleListPage(bundleListData{PC: pc, Bundles: bundles}).Render(ctx, w)
}

func (h *webHandler) ProjectBundleList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := db.GetQueries(ctx)
	linkname := chi.URLParam(r, "linkname")

	project, err := q.GetProjectByLinkname(linkname)
	if err != nil {
		notFoundPage(w)
		return
	}
	pc := h.projectPageCtx(r, project)

	sq := q.DB.NewSelect().Model((*db.Bundle)(nil)).
		ColumnExpr("*, (SELECT count(*) FROM bundle_patch WHERE bundle_id = bundle.id) AS patch_count").
		Relation("Owner").Relation("Project").
		Where("project_id = ?", project.ID).
		OrderExpr("name ASC")

	user := getWebUser(r)
	if user != nil {
		sq = sq.Where("(public = ? OR owner_id = ?)", true, user.ID)
	} else {
		sq = sq.Where("public = ?", true)
	}

	var bundles []db.Bundle
	if err := sq.Scan(q.Ctx, &bundles); err != nil {
		serverErrorPage(w, "list bundles", err)
		return
	}

	bundleListPage(bundleListData{PC: pc, Bundles: bundles}).Render(ctx, w)
}

type bundleDetailData struct {
	PC      pageContext
	Bundle  db.Bundle
	Patches []db.Patch
	IsOwner bool
}

func (h *webHandler) BundleDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := db.GetQueries(ctx)
	username := chi.URLParam(r, "username")
	bundlename := chi.URLParam(r, "bundlename")
	pc := h.pageCtx(r)
	user := getWebUser(r)

	var bundle db.Bundle
	err := q.DB.NewSelect().
		Model(&bundle).
		Join("JOIN auth_user AS u ON u.id = bundle.owner_id").
		Where("u.username = ?", username).
		Where("bundle.name = ?", bundlename).
		Scan(q.Ctx)
	if err != nil {
		notFoundPage(w)
		return
	}

	isOwner := user != nil && user.ID == bundle.OwnerID
	if !bundle.Public && !isOwner {
		notFoundPage(w)
		return
	}

	var patches []db.Patch
	err = q.DB.NewSelect().
		Model(&patches).
		Relation("Submitter").Relation("State").Relation("Delegate").
		Join("JOIN bundle_patch AS bp ON bp.patch_id = patch.id").
		Where("bp.bundle_id = ?", bundle.ID).
		OrderBy("bp.order", bun.OrderAsc).
		Scan(q.Ctx)
	if err != nil {
		serverErrorPage(w, "list bundle patches", err)
		return
	}

	populateWebPatchTags(q, patches)

	var owner db.User
	if q.DB.NewSelect().Model(&owner).Where("id = ?", bundle.OwnerID).Scan(q.Ctx) == nil {
		bundle.Owner = &owner
	}
	project, err := q.GetProjectByID(bundle.ProjectID)
	if err == nil {
		bundle.Project = project
	}

	data := bundleDetailData{
		PC:      pc,
		Bundle:  bundle,
		Patches: patches,
		IsOwner: isOwner,
	}
	bundleDetailPage(data).Render(ctx, w)
}

func (h *webHandler) BundleUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireLogin(w, r) {
		return
	}
	ctx := r.Context()
	q := db.GetQueries(ctx)
	user := getWebUser(r)
	username := chi.URLParam(r, "username")
	bundlename := chi.URLParam(r, "bundlename")

	if !h.validateCSRF(r) {
		http.Redirect(w, r, fmt.Sprintf("/bundle/%s/%s/", username, bundlename), http.StatusFound)
		return
	}

	var bundle db.Bundle
	err := q.DB.NewSelect().
		Model(&bundle).
		Join("JOIN auth_user AS u ON u.id = bundle.owner_id").
		Where("u.username = ?", username).
		Where("bundle.name = ?", bundlename).
		Scan(q.Ctx)
	if err != nil || bundle.OwnerID != user.ID {
		notFoundPage(w)
		return
	}

	r.ParseForm()
	action := r.FormValue("action")

	switch action {
	case "delete":
		if _, err := q.DB.NewDelete().Model((*db.BundlePatch)(nil)).
			Where("bundle_id = ?", bundle.ID).Exec(q.Ctx); err != nil {
			serverErrorPage(w, "delete bundle patches", err)
			return
		}
		if _, err := q.DB.NewDelete().Model((*db.Bundle)(nil)).
			Where("id = ?", bundle.ID).Exec(q.Ctx); err != nil {
			serverErrorPage(w, "delete bundle", err)
			return
		}
		http.Redirect(w, r, "/user/bundles/", http.StatusFound)

	case "update":
		newName := strings.TrimSpace(r.FormValue("name"))
		public := r.FormValue("public") == "on"
		if newName != "" {
			if _, err := q.DB.NewUpdate().Model(&bundle).
				Where("id = ?", bundle.ID).
				Set("name = ?", newName).
				Set("public = ?", public).
				Exec(q.Ctx); err != nil {
				serverErrorPage(w, "update bundle", err)
				return
			}
			http.Redirect(w, r, fmt.Sprintf("/bundle/%s/%s/", username, newName), http.StatusFound)
		} else {
			http.Redirect(w, r, fmt.Sprintf("/bundle/%s/%s/", username, bundlename), http.StatusFound)
		}

	case "remove-patches":
		for _, idStr := range r.Form["patch_id"] {
			patchID, err := strconv.ParseInt(idStr, 10, 32)
			if err != nil {
				continue
			}
			if _, err := q.DB.NewDelete().Model((*db.BundlePatch)(nil)).
				Where("bundle_id = ?", bundle.ID).
				Where("patch_id = ?", patchID).
				Exec(q.Ctx); err != nil {
				serverErrorPage(w, "remove patch from bundle", err)
				return
			}
		}
		http.Redirect(w, r, fmt.Sprintf("/bundle/%s/%s/", username, bundlename), http.StatusFound)

	default:
		http.Redirect(w, r, fmt.Sprintf("/bundle/%s/%s/", username, bundlename), http.StatusFound)
	}
}
