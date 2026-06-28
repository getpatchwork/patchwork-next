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

func (p *parser) handlePatch() error {
	author, err := p.getOrCreateAuthor()
	if err != nil {
		return err
	}
	p.author = author

	state, err := p.findState()
	if err != nil {
		return fmt.Errorf("find state: %w", err)
	}

	p.patch = &db.Patch{
		ProjectID:   p.project.ID,
		Msgid:       p.msgid,
		Name:        p.subject,
		Date:        p.date,
		Headers:     p.content.headers,
		SubmitterID: author.ID,
		Content:     db.Ptr(p.content.comment),
		Diff:        db.Ptr(p.content.diff),
		PullURL:     db.Ptr(p.pullURL),
		DelegateID:  p.findDelegate(),
		StateID:     db.Ptr[int](state.ID),
	}
	if p.content.diff != "" {
		p.patch.Hash = db.Ptr(HashDiff(p.content.diff))
	}

	err = p.db.CreatePatch(p.patch)
	if errors.Is(err, sql.ErrNoRows) {
		return DuplicateMailErr(p.msgid)
	} else if err != nil {
		return fmt.Errorf("create patch: %w", err)
	}

	_ = p.db.RefreshTagCounts(p.patch)

	log.Infof("patch saved id=%d msgid=%s", p.patch.ID, p.msgid)
	p.createPatchCreatedEvent()

	if err := p.assignSeries(); err != nil {
		log.Warnf("failed to assign series: %v", err)
		return nil
	}

	p.createPatchCompletedEvent()
	p.createSeriesCompletedEvent()

	p.parseDependencies(p.content.comment)

	if p.version > 1 {
		p.linkPreviousSeries()
	}

	return nil
}
