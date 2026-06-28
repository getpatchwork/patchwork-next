// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package mail

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/getpatchwork/patchwork/pkg/db"
	"github.com/getpatchwork/patchwork/pkg/log"
)

func (p *parser) allRefs() []string {
	refs := make([]string, 0, 1+len(p.refs))
	refs = append(refs, p.msgid)
	refs = append(refs, p.refs...)
	return refs
}

const seriesDelayInterval = 20 * time.Minute

func (p *parser) assignSeries() error {
	// use only In-Reply-To/References for finding existing series,
	// not the current message's own msgid (which isn't stored yet)
	s, err := p.db.FindSeries(
		p.project.ID,
		p.author.ID,
		p.refs,
		p.msgid,
		p.number,
		int(p.version),
		int(p.total),
		p.date,
		seriesDelayInterval,
	)
	// create new series
	if err != nil {
		s = &db.Series{
			ProjectID:   db.Ptr[int](p.project.ID),
			Date:        p.date,
			SubmitterID: p.author.ID,
			Version:     int(p.version),
			Total:       int(p.total),
		}
		if err := p.db.CreateSeries(s); err != nil {
			return err
		}
		log.Debugf("created new series id=%d", s.ID)
		p.series = s
		p.createSeriesCreatedEvent()
	} else {
		log.Debugf("found existing series id=%d", s.ID)
	}

	p.series = s

	// save all references (including own msgid) so later patches
	// find this series
	for _, ref := range p.allRefs() {
		_ = p.db.CreateSeriesReference(p.project.ID, p.series.ID, ref)
	}

	return p.assignPatchToSeries()
}

func (p *parser) assignPatchToSeries() error {
	if p.number <= 0 {
		return nil
	}

	p.patch.SeriesID = db.Ptr[int](p.series.ID)
	p.patch.Number = db.Ptr(p.number)

	err := p.db.UpdatePatchSeries(p.patch.ID, p.patch.SeriesID, p.patch.Number)
	if err != nil {
		return err
	}

	log.Debugf("assigned patch id=%d to series id=%d (n=%d)",
		p.patch.ID, p.series.ID, p.number)

	if p.series.Name == nil && p.number == 1 {
		p.series.Name = db.Ptr(p.patch.Name)
		_ = p.db.UpdateSeriesName(p.series.ID, p.series.Name)
		log.Debugf("series id=%d name set to %q", p.series.ID, p.patch.Name)
	}

	return nil
}

var (
	dependsOnMsgIDRe = regexp.MustCompile(`(?mi)^Depends-on: (<[^>]+>)\s*$`)
	dependsOnURLRe   = regexp.MustCompile(`(?mi)^Depends-on: (https?://[\w\d\-.\/=&@:%?_+()]+)\s*$`)
)

func (p *parser) parseDependencies(content string) {
	if p.series == nil {
		return
	}

	// dependencies by URL
	for _, m := range dependsOnURLRe.FindAllStringSubmatch(content, -1) {
		log.Debugf("dependency URL match: %s", m[1])
		if dep := p.findSeriesFromURL(m[1]); dep != nil {
			p.addDependency(dep.ID)
		}
	}

	// dependencies by message-id
	for _, m := range dependsOnMsgIDRe.FindAllStringSubmatch(content, -1) {
		log.Debugf("dependency msgid match: %s", m[1])
		if dep := p.findSeriesFromMsgID(m[1]); dep != nil {
			p.addDependency(dep.ID)
		}
	}
}

func (p *parser) addDependency(depSeriesID int) {
	if depSeriesID == p.series.ID {
		log.Debugf("dependency: skip self-reference series=%d", depSeriesID)
		return
	}
	dep, err := p.db.GetSeriesByID(depSeriesID)
	if err != nil {
		log.Debugf("dependency: series %d not found: %v", depSeriesID, err)
		return
	}
	// skip cross-project dependencies
	if dep.ProjectID == nil || *dep.ProjectID != p.project.ID {
		log.Debugf("dependency: skip cross-project series=%d (project %v vs %d)",
			depSeriesID, dep.ProjectID, p.project.ID)
		return
	}
	log.Debugf("dependency: series %d -> %d", p.series.ID, depSeriesID)
	_ = p.db.AddSeriesDependencies(p.series.ID, depSeriesID)
}

func (p *parser) findSeriesFromMsgID(msgid string) *db.Series {
	s, err := p.db.FindSeriesByMsgID(msgid)
	if err == nil {
		return s
	}
	return nil
}

func (p *parser) findSeriesFromURL(rawURL string) *db.Series {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}

	path := strings.TrimSuffix(u.Path, "/")
	parts := strings.Split(path, "/")

	// series URL: /project/<linkname>/list/?series=<id>
	if strings.HasSuffix(path, "/list") || strings.HasSuffix(u.Path, "/list/") {
		if sid := u.Query().Get("series"); sid != "" {
			id, err := strconv.Atoi(sid)
			if err != nil {
				return nil
			}
			s, err := p.db.GetSeriesByID(int(id))
			if err == nil {
				return s
			}
		}
		return nil
	}

	// patch URL: /project/<linkname>/patch/<msgid>/
	for i, part := range parts {
		if part == "patch" && i+1 < len(parts) {
			msgid := "<" + strings.ReplaceAll(parts[i+1], "%2F", "/") + ">"
			return p.findSeriesFromMsgID(msgid)
		}
	}

	return nil
}

func (p *parser) linkPreviousSeries() {
	if p.series == nil || p.series.Version <= 1 {
		return
	}

	prevVersion := p.series.Version - 1
	log.Debugf("looking for previous series v%d for series id=%d v%d",
		prevVersion, p.series.ID, p.series.Version)

	// tier 1: find previous version via message references
	for _, ref := range p.refs {
		prev, err := p.db.FindSeriesByReference(p.project.ID, ref)
		if err != nil {
			continue
		}
		if prev.ID == p.series.ID {
			continue
		}
		if prev.SubmitterID != p.series.SubmitterID {
			continue
		}
		if prev.Version != prevVersion {
			continue
		}
		log.Debugf("previous series found by ref: id=%d v%d", prev.ID, prev.Version)
		_ = p.db.UpdateSeriesPreviousSeries(
			p.series.ID,
			db.Ptr[int](prev.ID),
		)
		if p.project.AutoSupersede {
			p.markAsSuperseded(prev)
		}
		return
	}

	// tier 2: name similarity matching
	candidates, err := p.db.FindPreviousSeriesByName(
		db.Ptr[int](p.project.ID),
		p.series.SubmitterID, prevVersion,
	)
	if err != nil || len(candidates) == 0 {
		log.Debugf("no previous series candidates found")
		return
	}

	if p.series.Name == nil {
		return
	}
	seriesName := stripPrefixes(*p.series.Name)
	if seriesName == "" {
		return
	}

	var best *db.Series
	bestScore := 0.0
	for i := range candidates {
		if candidates[i].Name == nil {
			continue
		}
		candName := stripPrefixes(*candidates[i].Name)
		if candName == "" {
			continue
		}
		score := nameSimilarity(seriesName, candName)
		log.Debugf("similarity %q vs %q: %.2f", seriesName, candName, score)
		if score >= 0.8 && score > bestScore {
			best = &candidates[i]
			bestScore = score
		}
	}

	if best != nil {
		log.Debugf("previous series found by name: id=%d (score=%.2f)",
			best.ID, bestScore)
		_ = p.db.UpdateSeriesPreviousSeries(
			p.series.ID,
			db.Ptr[int](best.ID),
		)
		if p.project.AutoSupersede {
			p.markAsSuperseded(best)
		}
	}
}

func (p *parser) markAsSuperseded(series *db.Series) {
	state, err := p.db.GetStateBySlug("superseded")
	if err != nil {
		log.Warnf("no superseded state found")
		return
	}
	_ = p.db.UpdatePatchesBySeriesToState(
		db.Ptr[int](series.ID),
		db.Ptr[int](state.ID),
	)
}
