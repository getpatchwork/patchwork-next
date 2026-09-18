// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package mail

import (
	"context"
	"crypto/md5"
	"encoding/base64"
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

func anonymizeMsgIDs(msgIDs []string, toListDomain string) []string {
	newMsgIDs := make([]string, 0)
	for _, msgID := range msgIDs {
		msgIDMd5 := md5.Sum([]byte(msgID))
		msgIDEncoded := base64.URLEncoding.EncodeToString(msgIDMd5[:])
		msgIDEncoded = fmt.Sprintf("%s@%s", msgIDEncoded, toListDomain)
		newMsgIDs = append(newMsgIDs, msgIDEncoded)
	}
	return newMsgIDs
}

func anonymizeAddress(address string, toListDomain string) string {
	md5 := md5.Sum([]byte(address))
	encoded := base64.URLEncoding.EncodeToString(md5[:])
	return fmt.Sprintf("%s@%s", encoded, toListDomain)
}

func anonymizeHeaders(listid string, header mail.Header, from mail.Address, msgid string) (mail.Header, *mail.Address, string) {
	newHeader := mail.Header{}

	inReplyToIDs, err := header.MsgIDList("In-Reply-To")
	if err != nil {
		log.Warnf("failed to parse in-reply-to: %v", err)
	} else if len(inReplyToIDs) != 0 {
		newHeader.SetMsgIDList("In-Reply-To", anonymizeMsgIDs(inReplyToIDs, listid))
	}

	referencesIDs, err := header.MsgIDList("References")
	if err != nil {
		log.Warnf("failed to parse references: %v", err)
	} else if len(referencesIDs) != 0 {
		newHeader.SetMsgIDList("References", anonymizeMsgIDs(referencesIDs, listid))
	}

	newHeader.Set("List-ID", fmt.Sprintf("<%s>", listid))

	from = *GetOriginalSender(&header, &from)
	fromEmail := anonymizeAddress(from.Address, listid)
	from = mail.Address{Name: from.Name, Address: fromEmail}

	newHeader.SetAddressList("From", []*mail.Address{&from})

	msgid = anonymizeAddress(msgid, listid)
	newHeader.SetMessageID(msgid)

	if state := header.Get("X-Patchwork-State"); state != "" {
		newHeader.Set("X-Patchwork-State", state)
	}
	if delegate := header.Get("X-Patchwork-Delegate"); delegate != "" {
		newHeader.Set("X-Patchwork-Delegate", delegate)
	}
	if hint := header.Get("X-Patchwork-Hint"); hint != "" {
		newHeader.Set("X-Patchwork-Hint", hint)
	}

	contentType, contentTypeParams, err := header.ContentType()
	if err != nil {
		log.Warnf("failed to parse content-type: %v", err)
	} else {
		newHeader.SetContentType(contentType, contentTypeParams)
	}

	contentDisposition, contentDispositionParams, err := header.ContentDisposition()
	if err == nil {
		newHeader.SetContentDisposition(contentDisposition, contentDispositionParams)
	}

	if mime := header.Get("MIME-Version"); mime != "" {
		newHeader.Set("MIME-Version", mime)
	}

	return newHeader, &from, msgid
}

func ParseMail(ctx context.Context, database *bun.DB, r io.Reader, anonymize bool, listid ...string) error {
	m, err := mail.CreateReader(r)
	if err != nil {
		return ParseErr("read message: %v", err)
	}

	// basic sanity checks

	header := m.Header

	if strings.EqualFold(m.Header.Get("X-Patchwork-Hint"), "ignore") {
		log.Debugf("ignoring email due to hint")
		return nil
	}
	subject, err := header.Subject()
	if err != nil {
		return ParseErr("subject: %v", err)
	}
	date, err := header.Date()
	if err != nil {
		log.Warnf("date: %v", err)
	}
	if date.IsZero() {
		date = time.Now()
	}
	msgid, err := header.MessageID()
	if err != nil {
		return ParseErr("message-id: %v", err)
	}
	from, err := mail.ParseAddress(header.Get("From"))
	if err != nil {
		return ParseErr("from: %v", err)
	}

	if anonymize {
		if len(listid) == 0 {
			return ParseErr("anonymize but no listid")
		}

		header, from, msgid = anonymizeHeaders(listid[0], header, *from, msgid)
		header.SetDate(date)
		header.SetSubject(subject)
	}

	queries, err := db.Begin(ctx, database)
	if err != nil {
		return err
	}
	defer queries.Rollback()

	p := parser{
		db:      queries,
		header:  &header,
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
	p.refs = FindReferences(&header)

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
	p.content.headers = FormatHeaders(&header)
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
