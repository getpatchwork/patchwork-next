// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

func (q *Queries) GetCoverByID(id int) (*Cover, error) {
	var c Cover
	err := q.DB.NewSelect().Model(&c).
		Where("id = ?", id).
		Scan(q.Ctx)
	return &c, err
}

func (q *Queries) CreateCover(cover *Cover) error {
	return q.DB.NewInsert().Model(cover).
		On("CONFLICT (msgid, project_id) DO NOTHING").
		Returning("*").
		Scan(q.Ctx)
}

func (q *Queries) GetCoverByProjectAndMsgID(projectID int, msgid string) (*Cover, error) {
	var c Cover
	err := q.DB.NewSelect().Model(&c).
		Where("project_id = ?", projectID).
		Where("msgid = ?", msgid).
		Scan(q.Ctx)
	return &c, err
}

func (q *Queries) FindCoverByCommentMsgID(msgid string) (*Cover, error) {
	var c Cover
	err := q.DB.NewSelect().Model(&c).
		Join("JOIN cover_comment AS cc ON cc.cover_id = cover.id").
		Where("cc.msgid = ?", msgid).
		Limit(1).
		Scan(q.Ctx)
	return &c, err
}
