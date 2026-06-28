// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

func (q *Queries) GetPatchByID(id int) (*Patch, error) {
	var p Patch
	err := q.DB.NewSelect().Model(&p).
		Where("id = ?", id).
		Scan(q.Ctx)
	return &p, err
}

func (q *Queries) CreatePatch(patch *Patch) error {
	return q.DB.NewInsert().Model(patch).
		On("CONFLICT (msgid, project_id) DO NOTHING").
		Returning("*").
		Scan(q.Ctx)
}

func (q *Queries) GetPatchByProjectAndMsgID(projectID int, msgid string) (*Patch, error) {
	var p Patch
	err := q.DB.NewSelect().Model(&p).
		Where("project_id = ?", projectID).
		Where("msgid = ?", msgid).
		Scan(q.Ctx)
	return &p, err
}

func (q *Queries) GetPatchByMsgID(msgid string) ([]Patch, error) {
	var patches []Patch
	err := q.DB.NewSelect().Model(&patches).
		Where("msgid = ?", msgid).
		Scan(q.Ctx)
	return patches, err
}

func (q *Queries) FindPatchByCommentMsgID(msgid string) ([]Patch, error) {
	var patches []Patch
	err := q.DB.NewSelect().Model(&patches).
		Join("JOIN patch_comment AS pc ON pc.patch_id = patch.id").
		Where("pc.msgid = ?", msgid).
		Scan(q.Ctx)
	return patches, err
}

func (q *Queries) UpdatePatchSeries(id int, seriesID *int, number *int) error {
	_, err := q.DB.NewUpdate().Model((*Patch)(nil)).
		Set("series_id = ?", seriesID).
		Set("number = ?", number).
		Where("id = ?", id).
		Exec(q.Ctx)
	return err
}

func (q *Queries) GetPatchBySeriesAndNumber(seriesID int, number int) (*Patch, error) {
	var p Patch
	err := q.DB.NewSelect().Model(&p).
		Where("series_id = ?", seriesID).
		Where("number = ?", number).
		Scan(q.Ctx)
	return &p, err
}

func (q *Queries) CountPredecessorPatches(seriesID int, number int) (int, error) {
	return q.DB.NewSelect().Model((*Patch)(nil)).
		Where("series_id = ?", seriesID).
		Where("number < ?", number).
		Count(q.Ctx)
}

func (q *Queries) GetSuccessorPatches(seriesID int, number int) ([]Patch, error) {
	var patches []Patch
	err := q.DB.NewSelect().Model(&patches).
		Where("series_id = ?", seriesID).
		Where("number > ?", number).
		OrderExpr("number ASC").
		Scan(q.Ctx)
	return patches, err
}

func (q *Queries) UpdatePatchesBySeriesToState(seriesID, stateID *int) error {
	_, err := q.DB.NewUpdate().Model((*Patch)(nil)).
		Set("state_id = ?", stateID).
		Where("series_id = ?", seriesID).
		Exec(q.Ctx)
	return err
}

func (q *Queries) CountPatchesInSeries(seriesID int) (int, error) {
	return q.DB.NewSelect().Model((*Patch)(nil)).
		Where("series_id = ?", seriesID).
		Count(q.Ctx)
}
