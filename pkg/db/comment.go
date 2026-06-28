// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

func (q *Queries) GetPatchCommentByID(id int) (*PatchComment, error) {
	var c PatchComment
	err := q.DB.NewSelect().Model(&c).
		Where("id = ?", id).
		Scan(q.Ctx)
	return &c, err
}

func (q *Queries) GetCoverCommentByID(id int) (*CoverComment, error) {
	var c CoverComment
	err := q.DB.NewSelect().Model(&c).
		Where("id = ?", id).
		Scan(q.Ctx)
	return &c, err
}

func (q *Queries) ListPatchComments(patchID int) ([]PatchComment, error) {
	var comments []PatchComment
	err := q.DB.NewSelect().Model(&comments).
		Relation("Submitter").
		Where("patch_id = ?", patchID).
		OrderExpr("date ASC").
		Scan(q.Ctx)
	return comments, err
}

func (q *Queries) ListCoverComments(coverID int) ([]CoverComment, error) {
	var comments []CoverComment
	err := q.DB.NewSelect().Model(&comments).
		Relation("Submitter").
		Where("cover_id = ?", coverID).
		OrderExpr("date ASC").
		Scan(q.Ctx)
	return comments, err
}

func (q *Queries) CreatePatchComment(c *PatchComment) error {
	return q.DB.NewInsert().Model(c).
		On("CONFLICT (msgid, patch_id) DO NOTHING").
		Returning("*").
		Scan(q.Ctx)
}

func (q *Queries) CreateCoverComment(c *CoverComment) error {
	return q.DB.NewInsert().Model(c).
		On("CONFLICT (msgid, cover_id) DO NOTHING").
		Returning("*").
		Scan(q.Ctx)
}
