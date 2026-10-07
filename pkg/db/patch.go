// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import "github.com/uptrace/bun"

func (q *Queries) GetPatchByID(id int) (*Patch, error) {
	var p Patch
	err := q.Select(&p).
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
	err := q.Select(&p).
		Where("project_id = ?", projectID).
		Where("msgid = ?", msgid).
		Scan(q.Ctx)
	return &p, err
}

func (q *Queries) GetPatchByMsgID(msgid string) ([]Patch, error) {
	var patches []Patch
	err := q.Select(&patches).
		Where("msgid = ?", msgid).
		Scan(q.Ctx)
	return patches, err
}

func (q *Queries) FindPatchByCommentMsgID(msgid string) ([]Patch, error) {
	var patches []Patch
	err := q.Select(&patches).
		Join("JOIN patch_comment AS pc ON pc.patch_id = patch.id").
		Where("pc.msgid = ?", msgid).
		Scan(q.Ctx)
	return patches, err
}

func (q *Queries) UpdatePatchSeries(id int, seriesID *int, number *int) error {
	_, err := q.Update((*Patch)(nil)).
		Set("series_id = ?", seriesID).
		Set("number = ?", number).
		Where("id = ?", id).
		Exec(q.Ctx)
	return err
}

func (q *Queries) GetPatchBySeriesAndNumber(seriesID int, number int) (*Patch, error) {
	var p Patch
	err := q.Select(&p).
		Where("series_id = ?", seriesID).
		Where("number = ?", number).
		Scan(q.Ctx)
	return &p, err
}

func (q *Queries) CountPredecessorPatches(seriesID int, number int) (int64, error) {
	return q.Select((*Patch)(nil)).
		Where("series_id = ?", seriesID).
		Where("number < ?", number).
		Count(q.Ctx)
}

func (q *Queries) GetSuccessorPatches(seriesID int, number int) ([]Patch, error) {
	var patches []Patch
	err := q.Select(&patches).
		Where("series_id = ?", seriesID).
		Where("number > ?", number).
		OrderExpr("number ASC").
		Scan(q.Ctx)
	return patches, err
}

func (q *Queries) UpdatePatchesBySeriesToState(seriesID, stateID *int) error {
	_, err := q.Update((*Patch)(nil)).
		Set("state_id = ?", stateID).
		Where("series_id = ?", seriesID).
		Exec(q.Ctx)
	return err
}

func (q *Queries) CountPatchesInSeries(seriesID int) (int64, error) {
	return q.Select((*Patch)(nil)).
		Where("series_id = ?", seriesID).
		Count(q.Ctx)
}

func (q *Queries) LoadPatchRelated(patches []Patch) error {
	var relatedIDs []int
	for i := range patches {
		if patches[i].RelatedID != nil {
			relatedIDs = append(relatedIDs, *patches[i].RelatedID)
		}
	}
	byRelID := make(map[int][]PatchRef)
	if len(relatedIDs) > 0 {
		var related []Patch
		if err := q.Select(&related).
			Column("id", "name", "related_id").
			Where("related_id IN ?", bun.Tuple(relatedIDs)).
			Scan(q.Ctx); err != nil {
			return err
		}
		for _, r := range related {
			if r.RelatedID != nil {
				byRelID[*r.RelatedID] = append(
					byRelID[*r.RelatedID],
					PatchRef{ID: r.ID, Name: r.Name},
				)
			}
		}
	}
	for i := range patches {
		if patches[i].RelatedID != nil {
			for _, ref := range byRelID[*patches[i].RelatedID] {
				if ref.ID != patches[i].ID {
					patches[i].Related = append(patches[i].Related, ref)
				}
			}
		}
		if patches[i].Related == nil {
			patches[i].Related = []PatchRef{}
		}
	}
	return nil
}
