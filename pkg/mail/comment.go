// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package mail

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/getpatchwork/patchwork/pkg/db"
	"github.com/getpatchwork/patchwork/pkg/log"
)

func (p *parser) handleComment() error {
	for _, ref := range p.refs {
		patch, err := p.db.GetPatchByProjectAndMsgID(p.project.ID, ref)
		if err == nil {
			log.Debugf("comment on patch id=%d via direct ref %s", patch.ID, ref)
			return p.createPatchComment(patch)
		}

		patches, err := p.db.FindPatchByCommentMsgID(ref)
		if err == nil && len(patches) > 0 {
			log.Debugf("comment on patch id=%d via indirect ref %s", patches[0].ID, ref)
			return p.createPatchComment(&patches[0])
		}
	}

	for _, ref := range p.refs {
		cover, err := p.db.GetCoverByProjectAndMsgID(p.project.ID, ref)
		if err == nil {
			log.Debugf("comment on cover id=%d via direct ref %s", cover.ID, ref)
			return p.createCoverComment(cover)
		}

		cover, err = p.db.FindCoverByCommentMsgID(ref)
		if err == nil {
			log.Debugf("comment on cover id=%d via indirect ref %s", cover.ID, ref)
			return p.createCoverComment(cover)
		}
	}

	log.Debugf("no patch or cover found for comment refs=%v", p.refs)
	return nil
}

func (p *parser) createPatchComment(patch *db.Patch) error {
	author, err := p.getOrCreateAuthor()
	if err != nil {
		return err
	}

	var addressed *bool
	if p.header.Has("X-Patchwork-Action-Required") {
		addressed = db.Ptr(false)
	}

	comment := &db.PatchComment{
		PatchID:     patch.ID,
		Msgid:       p.msgid,
		Date:        p.date,
		Headers:     p.content.headers,
		SubmitterID: author.ID,
		Content:     db.Ptr(p.content.comment),
		Addressed:   addressed,
	}
	err = p.db.CreatePatchComment(comment)
	if errors.Is(err, sql.ErrNoRows) {
		return DuplicateMailErr(p.msgid)
	} else if err != nil {
		return fmt.Errorf("create patch comment: %w", err)
	}

	_ = p.db.RefreshTagCounts(patch)
	p.createPatchCommentCreatedEvent(comment, patch)

	log.Infof("patch comment saved patch=%d msgid=%s", patch.ID, p.msgid)
	return nil
}

func (p *parser) createCoverComment(cover *db.Cover) error {
	author, err := p.getOrCreateAuthor()
	if err != nil {
		return err
	}

	var addressed *bool
	if p.header.Has("X-Patchwork-Action-Required") {
		addressed = db.Ptr(false)
	}

	comment := &db.CoverComment{
		CoverID:     cover.ID,
		Msgid:       p.msgid,
		Date:        p.date,
		Headers:     p.content.headers,
		SubmitterID: author.ID,
		Content:     db.Ptr(p.content.comment),
		Addressed:   addressed,
	}
	err = p.db.CreateCoverComment(comment)
	if errors.Is(err, sql.ErrNoRows) {
		return DuplicateMailErr(p.msgid)
	} else if err != nil {
		return fmt.Errorf("create cover comment: %w", err)
	}
	p.createCoverCommentCreatedEvent(comment, cover)

	log.Infof("cover comment saved cover=%d msgid=%s", cover.ID, p.msgid)
	return nil
}
