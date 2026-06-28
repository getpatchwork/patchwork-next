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

	"github.com/getpatchwork/patchwork/pkg/db"
)

func registerProjectRoutes(api huma.API, h *handler, prefix string, mw huma.Middlewares) {
	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: prefix + "/projects",
		OperationID: fmt.Sprintf("list-projects-v%s", prefix[5:]),
		Middlewares: mw,
	}, h.ListProjects)
	huma.Register(api, huma.Operation{
		Method: http.MethodGet, Path: prefix + "/projects/{id}",
		OperationID: fmt.Sprintf("get-project-v%s", prefix[5:]),
		Middlewares: mw,
	}, h.GetProject)
	huma.Register(api, huma.Operation{
		Method: http.MethodPatch, Path: prefix + "/projects/{id}",
		OperationID: fmt.Sprintf("update-project-v%s", prefix[5:]),
		Middlewares: mw,
	}, h.UpdateProject)
	huma.Register(api, huma.Operation{
		Method: http.MethodPut, Path: prefix + "/projects/{id}",
		OperationID: fmt.Sprintf("put-project-v%s", prefix[5:]),
		Middlewares: mw,
	}, h.UpdateProject)
}

func projectToEmbedded(p *db.Project, base string) ProjectEmbedded {
	e := ProjectEmbedded{
		ID:              p.ID,
		URL:             fmt.Sprintf("%s/projects/%d/", base, p.ID),
		Name:            p.Name,
		LinkName:        p.Linkname,
		ListID:          p.Listid,
		ListEmail:       p.Listemail,
		WebURL:          p.WebURL,
		ScmURL:          p.ScmURL,
		WebScmURL:       p.WebScmURL,
		CommitURLFormat: strp(p.CommitURLFormat),
	}
	if p.ListArchiveURL != "" {
		e.ListArchiveURL = &p.ListArchiveURL
	}
	if p.ListArchiveURLFormat != "" {
		e.ListArchiveURLFormat = &p.ListArchiveURLFormat
	}
	return e
}

func projectToResponse(p *db.Project, base string) ProjectResponse {
	r := ProjectResponse{
		ID:               p.ID,
		URL:              fmt.Sprintf("%s/projects/%d/", base, p.ID),
		Name:             p.Name,
		LinkName:         p.Linkname,
		ListID:           p.Listid,
		ListEmail:        p.Listemail,
		WebURL:           p.WebURL,
		ScmURL:           p.ScmURL,
		WebScmURL:        p.WebScmURL,
		SubjectMatch:     strp(p.SubjectMatch),
		CommitURLFormat:  strp(p.CommitURLFormat),
		ShowDependencies: boolp(p.ShowDependencies),
		Maintainers:      make([]UserEmbedded, len(p.Maintainers)),
	}
	if p.ListArchiveURL != "" {
		r.ListArchiveURL = &p.ListArchiveURL
	}
	if p.ListArchiveURLFormat != "" {
		r.ListArchiveURLFormat = &p.ListArchiveURLFormat
	}
	for i := range p.Maintainers {
		r.Maintainers[i] = userToEmbedded(&p.Maintainers[i], base)
	}
	return r
}

type ListProjectsInput struct {
	PageParams
	SearchParams
}

type ListProjectsOutput struct {
	Link string            `header:"Link" doc:"Pagination links"`
	Body []ProjectResponse `json:"body"`
}

func (h *handler) ListProjects(
	ctx context.Context, input *ListProjectsInput,
) (*ListProjectsOutput, error) {
	base := h.apiBase(ctx)

	idb := db.GetQueries(ctx).DB
	sq := idb.NewSelect().Model((*db.Project)(nil))
	if input.Q != "" {
		sq = sq.Where("name LIKE ?", "%"+input.Q+"%")
	}

	total, _ := sq.Count(ctx)

	perPage := input.PerPage
	if perPage > maxPerPage {
		perPage = maxPerPage
	}
	offset := (input.Page - 1) * perPage

	var projects []db.Project
	sq.OrderExpr("id ASC").Offset(offset).Limit(perPage).Scan(ctx, &projects)
	loadProjectMaintainers(ctx, idb, projects)

	resp := &ListProjectsOutput{
		Link: buildLinkHeader(input.Page, perPage, total),
		Body: make([]ProjectResponse, len(projects)),
	}
	for i := range projects {
		resp.Body[i] = projectToResponse(&projects[i], base)
	}
	return resp, nil
}

type GetProjectInput struct {
	ID string `path:"id" doc:"Project ID or linkname"`
}

type GetProjectOutput struct {
	Body ProjectResponse
}

type UpdateProjectInput struct {
	ID   string            `path:"id" doc:"Project ID or linkname"`
	Body ProjectUpdateBody `json:"body"`
}

func (h *handler) UpdateProject(
	ctx context.Context, input *UpdateProjectInput,
) (*GetProjectOutput, error) {
	user, err := h.requireUser(ctx)
	if err != nil {
		return nil, err
	}

	q := db.GetQueries(ctx)
	base := h.apiBase(ctx)

	var project db.Project
	if id, err := strconv.ParseInt(input.ID, 10, 32); err == nil {
		err = q.DB.NewSelect().Model(&project).
			Where("id = ?", id).Scan(ctx)
		if err != nil {
			err = q.DB.NewSelect().Model(&project).
				Where("linkname = ?", input.ID).Scan(ctx)
		}
		if err != nil {
			return nil, huma.Error404NotFound("Not found.")
		}
	} else {
		if err := q.DB.NewSelect().Model(&project).
			Where("linkname = ?", input.ID).Scan(ctx); err != nil {
			return nil, huma.Error404NotFound("Not found.")
		}
	}

	if !q.IsMaintainer(user, project.ID) {
		return nil, ForbiddenErr
	}

	body := &input.Body
	uq := q.DB.NewUpdate().Model(&project).Where("id = ?", project.ID)
	if body.WebURL != nil {
		uq = uq.Set("web_url = ?", *body.WebURL)
	}
	if body.ScmURL != nil {
		uq = uq.Set("scm_url = ?", *body.ScmURL)
	}
	if body.WebScmURL != nil {
		uq = uq.Set("webscm_url = ?", *body.WebScmURL)
	}
	if body.ListArchiveURL != nil {
		uq = uq.Set("list_archive_url = ?", *body.ListArchiveURL)
	}
	if body.ListArchiveURLFormat != nil {
		uq = uq.Set("list_archive_url_format = ?", *body.ListArchiveURLFormat)
	}
	if body.CommitURLFormat != nil {
		uq = uq.Set("commit_url_format = ?", *body.CommitURLFormat)
	}
	if _, err := uq.Exec(ctx); err != nil {
		return nil, huma.Error400BadRequest("Update failed.")
	}

	q.DB.NewSelect().Model(&project).Where("id = ?", project.ID).Scan(ctx)
	projects := []db.Project{project}
	loadProjectMaintainers(ctx, q.DB, projects)

	return &GetProjectOutput{
		Body: projectToResponse(&projects[0], base),
	}, nil
}

func (h *handler) GetProject(
	ctx context.Context, input *GetProjectInput,
) (*GetProjectOutput, error) {
	q := db.GetQueries(ctx)
	base := h.apiBase(ctx)

	var project db.Project
	if id, err := strconv.ParseInt(input.ID, 10, 32); err == nil {
		err = q.DB.NewSelect().Model(&project).
			Where("id = ?", id).Scan(ctx)
		if err != nil {
			err = q.DB.NewSelect().Model(&project).
				Where("linkname = ?", input.ID).Scan(ctx)
		}
		if err != nil {
			return nil, huma.Error404NotFound("Not found.")
		}
	} else {
		if err := q.DB.NewSelect().Model(&project).
			Where("linkname = ?", input.ID).Scan(ctx); err != nil {
			return nil, huma.Error404NotFound("Not found.")
		}
	}

	projects := []db.Project{project}
	loadProjectMaintainers(ctx, h.db, projects)

	return &GetProjectOutput{
		Body: projectToResponse(&projects[0], base),
	}, nil
}
