// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

func (q *Queries) GetProjectByLinkname(linkname string) (*Project, error) {
	var p Project
	err := q.DB.NewSelect().Model(&p).
		Where("linkname = ?", linkname).
		Scan(q.Ctx)
	return &p, err
}

func (q *Queries) GetProjectByID(id int) (*Project, error) {
	var p Project
	err := q.DB.NewSelect().Model(&p).
		Where("id = ?", id).
		Scan(q.Ctx)
	return &p, err
}

func (q *Queries) ListProjects() ([]Project, error) {
	var projects []Project
	err := q.DB.NewSelect().Model(&projects).
		OrderExpr("name ASC").
		Scan(q.Ctx)
	return projects, err
}

func (q *Queries) ListStates() ([]State, error) {
	var states []State
	err := q.DB.NewSelect().Model(&states).
		OrderExpr("ordering ASC").
		Scan(q.Ctx)
	return states, err
}

func (q *Queries) ListProjectMaintainers(projectID int) ([]User, error) {
	var users []User
	err := q.DB.NewSelect().Model(&users).
		Join("JOIN project_maintainer AS pm ON pm.user_id = auth_user.id").
		Where("pm.project_id = ?", projectID).
		OrderExpr("auth_user.username ASC").
		Scan(q.Ctx)
	return users, err
}

func (q *Queries) IsMaintainer(user *User, projectID int) bool {
	if user.IsAdmin {
		return true
	}
	count, _ := q.DB.NewSelect().
		Model((*ProjectMaintainer)(nil)).
		Where("user_id = ?", user.ID).
		Where("project_id = ?", projectID).
		Count(q.Ctx)
	return count > 0
}

func (q *Queries) GetProjectByListID(listid string) ([]Project, error) {
	var projects []Project
	err := q.DB.NewSelect().Model(&projects).
		Where("listid = ?", listid).
		Scan(q.Ctx)
	return projects, err
}

func (q *Queries) GetDefaultState() (*State, error) {
	var s State
	err := q.DB.NewSelect().Model(&s).
		Where("ordering = 0").
		Scan(q.Ctx)
	return &s, err
}

func (q *Queries) GetStateByName(name string) (*State, error) {
	var s State
	err := q.DB.NewSelect().Model(&s).
		Where("lower(name) = lower(?)", name).
		Scan(q.Ctx)
	return &s, err
}

func (q *Queries) GetStateBySlug(slug string) (*State, error) {
	var s State
	err := q.DB.NewSelect().Model(&s).
		Where("slug = ?", slug).
		Scan(q.Ctx)
	return &s, err
}

func (q *Queries) ListDelegationRulesByProject(projectID int) ([]DelegationRule, error) {
	var rules []DelegationRule
	err := q.DB.NewSelect().Model(&rules).
		Where("project_id = ?", projectID).
		OrderExpr("priority DESC, path").
		Scan(q.Ctx)
	return rules, err
}

func (q *Queries) GetUserIDByEmail(email string) (int, error) {
	var id int
	err := q.DB.NewSelect().Model((*User)(nil)).Column("id").
		Where("lower(email) = lower(?)", email).
		Where("is_active = ?", true).
		Scan(q.Ctx, &id)
	return id, err
}
