// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package mail

import (
	"time"

	"github.com/getpatchwork/patchwork/pkg/db"
	"github.com/getpatchwork/patchwork/pkg/log"
)

const (
	eventCoverCreated        = "cover-created"
	eventPatchCreated        = "patch-created"
	eventPatchCompleted      = "patch-completed"
	eventSeriesCreated       = "series-created"
	eventSeriesCompleted     = "series-completed"
	eventCoverCommentCreated = "cover-comment-created"
	eventPatchCommentCreated = "patch-comment-created"
)

func (p *parser) enqueueEvent(e db.Event) {
	e.ProjectID = p.project.ID
	e.Date = time.Now()
	p.db.EnqueueEvent(e)
	log.Debugf("event queued: %s", e.Category)
}

func (p *parser) createPatchCreatedEvent() {
	p.enqueueEvent(db.Event{
		Category: eventPatchCreated,
		PatchID:  db.Ptr(p.patch.ID),
	})
}

func (p *parser) createCoverCreatedEvent(cover *db.Cover) {
	p.enqueueEvent(db.Event{
		Category: eventCoverCreated,
		CoverID:  db.Ptr(cover.ID),
	})
}

func (p *parser) createSeriesCreatedEvent() {
	p.enqueueEvent(db.Event{
		Category: eventSeriesCreated,
		SeriesID: db.Ptr(p.series.ID),
	})
}

func (p *parser) createPatchCommentCreatedEvent(comment *db.PatchComment, patch *db.Patch) {
	p.enqueueEvent(db.Event{
		Category:       eventPatchCommentCreated,
		PatchID:        db.Ptr(patch.ID),
		PatchCommentID: db.Ptr(comment.ID),
	})
}

func (p *parser) createCoverCommentCreatedEvent(comment *db.CoverComment, cover *db.Cover) {
	p.enqueueEvent(db.Event{
		Category:       eventCoverCommentCreated,
		CoverID:        db.Ptr(cover.ID),
		CoverCommentID: db.Ptr(comment.ID),
	})
}

func (p *parser) createPatchCompletedEvent() {
	if p.series == nil || p.patch == nil || p.patch.Number == nil {
		return
	}
	number := *p.patch.Number
	if number <= 0 {
		return
	}

	predCount, err := p.db.CountPredecessorPatches(p.series.ID, number)
	if err != nil || predCount != number-1 {
		return
	}

	p.enqueueEvent(db.Event{
		Category: eventPatchCompleted,
		PatchID:  db.Ptr(p.patch.ID),
		SeriesID: db.Ptr(p.series.ID),
	})

	successors, err := p.db.GetSuccessorPatches(p.series.ID, number)
	if err != nil {
		return
	}
	count := number + 1
	for _, s := range successors {
		if s.Number == nil || *s.Number != count {
			break
		}
		p.enqueueEvent(db.Event{
			Category: eventPatchCompleted,
			PatchID:  db.Ptr(s.ID),
			SeriesID: db.Ptr(p.series.ID),
		})
		count++
	}
}

func (p *parser) createSeriesCompletedEvent() {
	if p.series == nil || p.series.Total <= 0 {
		return
	}

	count, err := p.db.CountPatchesInSeries(p.series.ID)
	if err != nil {
		return
	}
	if count < p.series.Total {
		return
	}

	p.enqueueEvent(db.Event{
		Category: eventSeriesCompleted,
		SeriesID: db.Ptr(p.series.ID),
	})
}
