// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package mail

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/emersion/go-message/mail"
	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/db"
	"github.com/getpatchwork/patchwork/pkg/log"
)

type parser struct {
	db *db.Queries

	header *mail.Header

	content struct {
		headers string
		diff    string
		comment string
	}

	project *db.Project
	author  *db.Person
	patch   *db.Patch
	series  *db.Series

	from     *mail.Address
	prefixes []string
	subject  string
	listid   string
	date     time.Time
	msgid    string
	pullURL  string
	number   int
	total    int
	version  int
	refs     []string
}

func ParseMail(ctx context.Context, database *bun.DB, r io.Reader, listid ...string) error {
	m, err := mail.CreateReader(r)
	if err != nil {
		return ParseErr("read message: %v", err)
	}

	// basic sanity checks

	if strings.EqualFold(m.Header.Get("X-Patchwork-Hint"), "ignore") {
		log.Debugf("ignoring email due to hint")
		return nil
	}
	subject, err := m.Header.Subject()
	if err != nil {
		return ParseErr("subject: %v", err)
	}
	date, err := m.Header.Date()
	if err != nil {
		log.Warnf("date: %v", err)
	}
	if date.IsZero() {
		date = time.Now()
	}
	msgid, err := m.Header.MessageID()
	if err != nil {
		return ParseErr("message-id: %v", err)
	}
	from, err := mail.ParseAddress(m.Header.Get("From"))
	if err != nil {
		return ParseErr("from: %v", err)
	}

	queries, err := db.Begin(ctx, database)
	if err != nil {
		return err
	}
	defer queries.Rollback()

	p := parser{
		db:      queries,
		header:  &m.Header,
		subject: subject,
		date:    date,
		msgid:   "<" + msgid + ">",
		from:    from,
	}
	if len(listid) > 0 && listid[0] != "" {
		p.listid = listid[0]
	}

	// parse metadata

	log.Debugf("parsing msgid=%s subject=%q", p.msgid, subject)

	if err = p.resolveProject(); err != nil {
		return fmt.Errorf("resolve project: %w", err)
	} else if p.project == nil {
		log.Warnf("no matching project found")
		return nil
	}
	log.Debugf("project=%s (id=%d)", p.project.Linkname, p.project.ID)

	p.subject, p.prefixes = CleanSubject(subject, []string{p.project.Linkname})
	isComment := IsComment(subject)
	p.parseSeriesMarker(isComment)

	p.version = ParseVersion(p.subject, p.prefixes)
	p.refs = FindReferences(&m.Header)

	log.Debugf("series marker: n=%d total=%d version=%d comment=%v refs=%v",
		p.number, p.total, p.version, isComment, p.refs)

	// parse content

	if isComment {
		p.content.comment = FindCommentContent(m)
	} else {
		p.content.diff, p.content.comment = FindPatchContent(m)
	}
	if p.content.diff == "" && p.content.comment == "" {
		log.Debugf("no diff or comment content, skipping")
		return nil
	}
	p.content.headers = FormatHeaders(&m.Header)
	p.pullURL = ParsePullRequest(p.content.comment)

	switch {
	case !isComment && (p.content.diff != "" || p.pullURL != ""):
		log.Debugf("dispatching as patch")
		err = p.handlePatch()
	case !isComment && p.number == 0 && p.total > 0:
		log.Debugf("dispatching as cover letter")
		err = p.handleCoverLetter()
	default:
		log.Debugf("dispatching as comment")
		err = p.handleComment()
	}
	if err != nil {
		return err
	}

	if err := queries.Commit(); err != nil {
		return err
	}

	return nil
}

func (p *parser) parseSeriesMarker(isComment bool) {
	p.number, p.total = ParseSeriesMarker(p.prefixes)
	if p.number == 0 && p.total == 0 && !isComment {
		p.number = 1
		p.total = 1
	}
}
