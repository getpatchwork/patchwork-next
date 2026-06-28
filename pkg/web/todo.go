// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package web

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/getpatchwork/patchwork/pkg/db"
)

type todoProject struct {
	Project  db.Project
	NPatches int
}

func (h *webHandler) TodoLists(w http.ResponseWriter, r *http.Request) {
	if !requireLogin(w, r) {
		return
	}
	ctx := r.Context()
	q := db.GetQueries(ctx)
	user := getWebUser(r)
	pc := h.pageCtx(r)

	projects, _ := q.ListProjects()

	var todos []todoProject
	for _, p := range projects {
		count, _ := q.DB.NewSelect().Model((*db.Patch)(nil)).
			Where("project_id = ?", p.ID).
			Where("archived = ?", false).
			Where("delegate_id = ?", user.ID).
			Where("state_id IN (SELECT id FROM state WHERE action_required = ?)", true).
			Count(q.Ctx)
		if count > 0 {
			todos = append(todos, todoProject{Project: p, NPatches: count})
		}
	}

	if len(todos) == 1 {
		http.Redirect(w, r,
			"/user/todo/"+todos[0].Project.Linkname+"/",
			http.StatusFound)
		return
	}

	todoListsPage(pc, todos).Render(ctx, w)
}

func (h *webHandler) todoList(w http.ResponseWriter, r *http.Request) {
	if !requireLogin(w, r) {
		return
	}
	ctx := r.Context()
	q := db.GetQueries(ctx)
	user := getWebUser(r)
	pc := h.pageCtx(r)
	linkname := chi.URLParam(r, "linkname")

	project, err := q.GetProjectByLinkname(linkname)
	if err != nil {
		notFoundPage(w)
		return
	}

	var patches []db.Patch
	q.DB.NewSelect().Model(&patches).
		Relation("Submitter").Relation("State").Relation("Delegate").
		Where("project_id = ?", project.ID).
		Where("archived = ?", false).
		Where("delegate_id = ?", user.ID).
		Where("state_id IN (SELECT id FROM state WHERE action_required = ?)", true).
		OrderExpr("date DESC").
		Scan(q.Ctx)

	populateWebPatchTags(q, patches)

	todoListPage(pc, *project, patches).Render(ctx, w)
}
