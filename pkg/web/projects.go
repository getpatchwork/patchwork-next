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

func (h *webHandler) ProjectList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := db.GetQueries(ctx)

	projects, err := q.ListProjects()
	if err != nil {
		serverErrorPage(w, "list projects", err)
		return
	}
	if len(projects) == 1 {
		http.Redirect(w, r,
			"/project/"+projects[0].Linkname+"/list/",
			http.StatusFound)
		return
	}
	projectListPage(h.pageCtx(r), projects).Render(ctx, w)
}

func (h *webHandler) ProjectDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := db.GetQueries(ctx)
	linkname := chi.URLParam(r, "linkname")

	project, err := q.GetProjectByLinkname(linkname)
	if err != nil {
		notFoundPage(w)
		return
	}

	maintainers, err := q.ListProjectMaintainers(project.ID)
	if err != nil {
		serverErrorPage(w, "list maintainers", err)
		return
	}

	nPatches, err := q.DB.NewSelect().Model((*db.Patch)(nil)).
		Where("project_id = ?", project.ID).
		Where("archived = ?", false).
		Count(q.Ctx)
	if err != nil {
		serverErrorPage(w, "count patches", err)
		return
	}

	projectDetailPage(h.projectPageCtx(r, project), *project, maintainers, nPatches).Render(ctx, w)
}
