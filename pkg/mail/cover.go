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

func (p *parser) handleCoverLetter() error {
	author, err := p.getOrCreateAuthor()
	if err != nil {
		return err
	}
	p.author = author

	cover := &db.Cover{
		ProjectID:   p.project.ID,
		Msgid:       p.msgid,
		Name:        p.subject,
		Date:        p.date,
		Headers:     p.content.headers,
		SubmitterID: author.ID,
		Content:     db.Ptr(p.content.comment),
	}
	err = p.db.CreateCover(cover)
	if errors.Is(err, sql.ErrNoRows) || cover.ID == 0 {
		return DuplicateMailErr(p.msgid)
	} else if err != nil {
		return fmt.Errorf("create cover: %w", err)
	}
	p.createCoverCreatedEvent(cover)

	// find existing series via reference, or create a new one
	s, err := p.db.FindSeries(
		p.project.ID,
		p.author.ID,
		[]string{p.msgid},
		p.msgid,
		0,
		int(p.version),
		int(p.total),
		p.date,
		seriesDelayInterval,
	)
	if err != nil {
		s = &db.Series{
			ProjectID:   db.Ptr[int](p.project.ID),
			Date:        p.date,
			SubmitterID: author.ID,
			Version:     int(p.version),
			Total:       int(p.total),
		}
		err = p.db.CreateSeries(s)
		if err != nil {
			return fmt.Errorf("create series: %w", err)
		}
		_ = p.db.CreateSeriesReference(p.project.ID, s.ID, p.msgid)
		p.series = s
		p.createSeriesCreatedEvent()
		log.Debugf("created new series id=%d for cover", s.ID)
	} else {
		log.Debugf("found existing series id=%d for cover", s.ID)
	}

	p.series = s

	_ = p.db.UpdateSeriesCoverLetter(
		s.ID, db.Ptr[int](cover.ID),
	)

	coverName := stripPrefixes(p.subject)
	if s.Name == nil {
		_ = p.db.UpdateSeriesName(s.ID, db.Ptr(coverName))
	} else {
		first, err := p.db.GetPatchBySeriesAndNumber(s.ID, 1)
		if err == nil && *s.Name == first.Name {
			_ = p.db.UpdateSeriesName(s.ID, db.Ptr(coverName))
		}
	}

	p.parseDependencies(p.content.comment)

	if p.version > 1 {
		p.linkPreviousSeries()
	}

	log.Infof("cover letter saved id=%d msgid=%s", cover.ID, p.msgid)

	return nil
}
