// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package web

import (
	"context"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/db"
)

var (
	postscriptRe = regexp.MustCompile(`(?m)^-{2,3} ?$`)
	responseRe   = regexp.MustCompile(`(?mi)^(Tested|Reviewed|Acked|Signed-off|Nacked|Reported)-by:.*$`)
)

func submissionToMbox(submission mboxSubmission) string {
	isPatch := submission.Diff != ""

	body := ""
	if submission.Content != "" {
		body = strings.TrimSpace(submission.Content) + "\n"
	}

	postscript := ""
	if loc := postscriptRe.FindStringIndex(body); loc != nil {
		postscript = body[loc[1]:]
		body = strings.TrimSpace(body[:loc[0]]) + "\n"
		postscript = strings.TrimRight(postscript, " \t\n")
	}

	// append tag lines from comments
	for _, content := range submission.CommentContents {
		for _, m := range responseRe.FindAllString(content, -1) {
			body += m + "\n"
		}
	}

	if postscript != "" {
		body += "---" + postscript + "\n"
	}

	if isPatch && submission.Diff != "" {
		body += "\n" + submission.Diff
	}

	// build headers
	var hdr strings.Builder

	fromLine := "From patchwork " + submission.Date.UTC().Format("Mon Jan  2 15:04:05 2006") + "\n"

	submitterName := submission.SubmitterEmail
	if submission.SubmitterName != "" {
		submitterName = submission.SubmitterName
	}
	xSubmitter := formatAddr(submitterName, submission.SubmitterEmail)

	origHeaders := parseHeaders(submission.Headers)

	hdr.WriteString(fromLine)

	dmarcReplaced := false
	for _, h := range origHeaders {
		key := h.key
		val := h.val

		if strings.EqualFold(key, "Content-Type") {
			if strings.Contains(val, "multipart/signed") {
				continue
			}
			continue
		}
		if strings.EqualFold(key, "Content-Transfer-Encoding") {
			continue
		}

		if strings.EqualFold(key, "From") {
			_, addr := parseFromHeader(val)
			if addr == submission.ListEmail {
				hdr.WriteString("X-Patchwork-Original-From: " + val + "\n")
				val = xSubmitter
				dmarcReplaced = true
			}
		}

		hdr.WriteString(key + ": " + val + "\n")
	}

	_ = dmarcReplaced

	hasDate := false
	for _, h := range origHeaders {
		if strings.EqualFold(h.key, "Date") {
			hasDate = true
			break
		}
	}
	if !hasDate {
		hdr.WriteString("Date: " + submission.Date.UTC().Format(time.RFC1123Z) + "\n")
	}

	hdr.WriteString("X-Patchwork-Submitter: " + xSubmitter + "\n")
	hdr.WriteString("X-Patchwork-Id: " + strconv.Itoa(int(submission.ID)) + "\n")
	if isPatch && submission.DelegateEmail != "" {
		hdr.WriteString("X-Patchwork-Delegate: " + submission.DelegateEmail + "\n")
	}

	hdr.WriteString("Content-Type: text/plain; charset=utf-8\n")
	hdr.WriteString("Content-Transfer-Encoding: 8bit\n")
	hdr.WriteString("\n")
	hdr.WriteString(body)

	return hdr.String()
}

type mboxSubmission struct {
	ID              int
	Date            time.Time
	Content         string
	Diff            string
	Headers         string
	SubmitterName   string
	SubmitterEmail  string
	DelegateEmail   string
	ListEmail       string
	CommentContents []string
}

type headerPair struct {
	key, val string
}

func parseHeaders(raw string) []headerPair {
	var headers []headerPair
	var currentKey, currentVal string

	for _, line := range strings.Split(raw, "\n") {
		if line == "" {
			continue
		}
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			currentVal += "\n" + line
		} else {
			if currentKey != "" {
				headers = append(headers, headerPair{currentKey, currentVal})
			}
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				currentKey = strings.TrimSpace(parts[0])
				currentVal = strings.TrimSpace(parts[1])
			} else {
				currentKey = ""
				currentVal = ""
			}
		}
	}
	if currentKey != "" {
		headers = append(headers, headerPair{currentKey, currentVal})
	}
	return headers
}

func parseFromHeader(from string) (name, addr string) {
	a, err := mail.ParseAddress(from)
	if err != nil {
		// fallback: try to extract bare email
		from = strings.TrimSpace(from)
		if strings.Contains(from, "<") {
			parts := strings.SplitN(from, "<", 2)
			name = strings.TrimSpace(parts[0])
			addr = strings.Trim(parts[1], "> ")
		} else {
			addr = from
		}
		return
	}
	return a.Name, a.Address
}

func formatAddr(name, email string) string {
	if name == "" || name == email {
		return email
	}
	return fmt.Sprintf("%s <%s>", name, email)
}

func (h *webHandler) PatchMboxPage(w http.ResponseWriter, r *http.Request) {
	linkname := chi.URLParam(r, "linkname")
	rawMsgid, _ := url.PathUnescape(chi.URLParam(r, "msgid"))
	ctx := r.Context()
	q := db.GetQueries(ctx)
	msgid := "<" + rawMsgid + ">"

	var patch db.Patch
	err := q.DB.NewSelect().
		Model(&patch).
		Join("JOIN project AS pr ON pr.id = patch.project_id").
		Where("pr.linkname = ?", linkname).
		Where("patch.msgid = ?", msgid).
		Scan(ctx)
	if err != nil {
		notFoundPage(w)
		return
	}

	var project db.Project
	q.DB.NewSelect().Model(&project).Where("id = ?", patch.ProjectID).Scan(q.Ctx)

	seriesParam := r.URL.Query().Get("series")
	if seriesParam != "" {
		h.seriesPatchMbox(w, r, patch, project, seriesParam)
		return
	}

	h.servePatchMbox(w, patch, project)
}

func (h *webHandler) servePatchMbox(w http.ResponseWriter, patch db.Patch, project db.Project) {
	ctx := context.Background()
	sub := h.buildMboxSubmission(ctx, patch.ID, patch.Date,
		derefStr(patch.Content), derefStr(patch.Diff),
		patch.Headers, patch.SubmitterID, patch.DelegateID,
		project.Listemail, true)

	mbox := submissionToMbox(sub)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%s.patch", sanitizeFilename(patch.Name)))
	_, _ = w.Write([]byte(mbox))
}

func (h *webHandler) CoverMboxPage(w http.ResponseWriter, r *http.Request) {
	linkname := chi.URLParam(r, "linkname")
	rawMsgid, _ := url.PathUnescape(chi.URLParam(r, "msgid"))
	ctx := r.Context()
	q := db.GetQueries(ctx)
	msgid := "<" + rawMsgid + ">"

	var cover db.Cover
	err := q.DB.NewSelect().
		Model(&cover).
		Join("JOIN project AS pr ON pr.id = cover.project_id").
		Where("pr.linkname = ?", linkname).
		Where("cover.msgid = ?", msgid).
		Scan(ctx)
	if err != nil {
		notFoundPage(w)
		return
	}

	var project db.Project
	q.DB.NewSelect().Model(&project).Where("id = ?", cover.ProjectID).Scan(q.Ctx)

	h.serveCoverMbox(w, cover, project)
}

func (h *webHandler) serveCoverMbox(w http.ResponseWriter, cover db.Cover, project db.Project) {
	ctx := context.Background()
	sub := h.buildCoverMboxSubmission(ctx, cover, project)

	mbox := submissionToMbox(sub)

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%s.mbox", sanitizeFilename(cover.Name)))
	_, _ = w.Write([]byte(mbox))
}

func (h *webHandler) SeriesMbox(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := db.GetQueries(ctx)
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		notFoundPage(w)
		return
	}

	var series db.Series
	err = q.DB.NewSelect().Model(&series).Where("id = ?", id).Scan(q.Ctx)
	if err != nil {
		notFoundPage(w)
		return
	}

	var project db.Project
	if series.ProjectID != nil {
		q.DB.NewSelect().Model(&project).Where("id = ?", *series.ProjectID).Scan(q.Ctx)
	}

	var patches []db.Patch
	q.DB.NewSelect().Model(&patches).
		Where("series_id = ?", series.ID).
		OrderBy("number", bun.OrderAsc).
		Scan(ctx)

	var parts []string
	for _, p := range patches {
		sub := h.buildMboxSubmission(ctx, p.ID, p.Date,
			derefStr(p.Content), derefStr(p.Diff),
			p.Headers, p.SubmitterID, p.DelegateID,
			project.Listemail, true)
		parts = append(parts, submissionToMbox(sub))
	}

	name := "series"
	if series.Name != nil {
		name = *series.Name
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%s.patch", sanitizeFilename(name)))
	_, _ = w.Write([]byte(strings.Join(parts, "\n")))
}

func (h *webHandler) seriesPatchMbox(w http.ResponseWriter, r *http.Request, patch db.Patch, project db.Project, seriesParam string) {
	ctx := r.Context()
	q := db.GetQueries(ctx)

	if patch.SeriesID == nil {
		notFoundPage(w)
		return
	}

	if seriesParam != "*" {
		sid, err := strconv.ParseInt(seriesParam, 10, 32)
		if err != nil || int(sid) != *patch.SeriesID {
			notFoundPage(w)
			return
		}
	}

	var deps []db.Patch
	if patch.Number != nil {
		q.DB.NewSelect().Model(&deps).
			Where("series_id = ?", *patch.SeriesID).
			Where("number < ?", *patch.Number).
			OrderBy("number", bun.OrderAsc).
			Scan(ctx)
	}

	var parts []string
	for _, dep := range deps {
		sub := h.buildMboxSubmission(ctx, dep.ID, dep.Date,
			derefStr(dep.Content), derefStr(dep.Diff),
			dep.Headers, dep.SubmitterID, dep.DelegateID,
			project.Listemail, true)
		parts = append(parts, submissionToMbox(sub))
	}

	sub := h.buildMboxSubmission(ctx, patch.ID, patch.Date,
		derefStr(patch.Content), derefStr(patch.Diff),
		patch.Headers, patch.SubmitterID, patch.DelegateID,
		project.Listemail, true)
	parts = append(parts, submissionToMbox(sub))

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%s.patch", sanitizeFilename(patch.Name)))
	_, _ = w.Write([]byte(strings.Join(parts, "\n")))
}

func (h *webHandler) BundleMbox(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := db.GetQueries(ctx)
	username := chi.URLParam(r, "username")
	bundlename := chi.URLParam(r, "bundlename")

	var bundle db.Bundle
	err := q.DB.NewSelect().
		Model(&bundle).
		Join("JOIN auth_user AS u ON u.id = bundle.owner_id").
		Where("u.username = ?", username).
		Where("bundle.name = ?", bundlename).
		Scan(ctx)
	if err != nil {
		notFoundPage(w)
		return
	}

	if !bundle.Public {
		notFoundPage(w)
		return
	}

	var project db.Project
	q.DB.NewSelect().Model(&project).Where("id = ?", bundle.ProjectID).Scan(q.Ctx)

	var patches []db.Patch
	q.DB.NewSelect().
		Model(&patches).
		Join("JOIN bundle_patch AS bp ON bp.patch_id = patch.id").
		Where("bp.bundle_id = ?", bundle.ID).
		OrderBy("bp.order", bun.OrderAsc).
		Scan(ctx)

	var parts []string
	for _, p := range patches {
		sub := h.buildMboxSubmission(ctx, p.ID, p.Date,
			derefStr(p.Content), derefStr(p.Diff),
			p.Headers, p.SubmitterID, p.DelegateID,
			project.Listemail, true)
		parts = append(parts, submissionToMbox(sub))
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=bundle-%d-%s.mbox",
			bundle.ID, sanitizeFilename(bundle.Name)))
	_, _ = w.Write([]byte(strings.Join(parts, "\n")))
}

func (h *webHandler) CommentRedirect(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := db.GetQueries(ctx)
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		notFoundPage(w)
		return
	}

	// try patch comment first
	var pc struct {
		PatchID int
	}
	err = q.DB.NewSelect().Model((*db.PatchComment)(nil)).Column("patch_id").
		Where("id = ?", id).
		Scan(ctx, &pc)
	if err == nil {
		var patch struct {
			Msgid     string
			ProjectID int
		}
		q.DB.NewSelect().Model((*db.Patch)(nil)).Column("msgid", "project_id").
			Where("id = ?", pc.PatchID).
			Scan(ctx, &patch)
		var linkname string
		q.DB.NewSelect().Model((*db.Project)(nil)).Column("linkname").
			Where("id = ?", patch.ProjectID).
			Scan(ctx, &linkname)
		http.Redirect(w, r,
			patchURL(linkname, patch.Msgid)+fmt.Sprintf("#comment-%d", id),
			http.StatusMovedPermanently)
		return
	}

	// try cover comment
	var cc struct {
		CoverID int
	}
	err = q.DB.NewSelect().Model((*db.CoverComment)(nil)).Column("cover_id").
		Where("id = ?", id).
		Scan(ctx, &cc)
	if err == nil {
		var cover struct {
			Msgid     string
			ProjectID int
		}
		q.DB.NewSelect().Model((*db.Cover)(nil)).Column("msgid", "project_id").
			Where("id = ?", cc.CoverID).
			Scan(ctx, &cover)
		var linkname string
		q.DB.NewSelect().Model((*db.Project)(nil)).Column("linkname").
			Where("id = ?", cover.ProjectID).
			Scan(ctx, &linkname)
		http.Redirect(w, r,
			coverURL(linkname, cover.Msgid)+fmt.Sprintf("#comment-%d", id),
			http.StatusMovedPermanently)
		return
	}

	notFoundPage(w)
}

func (h *webHandler) buildMboxSubmission(
	ctx context.Context, patchID int, date time.Time,
	content, diff, headers string, submitterID int, delegateID *int,
	listEmail string, isPatch bool,
) mboxSubmission {
	q := db.New(ctx, h.db)
	sub := mboxSubmission{
		ID:        patchID,
		Date:      date,
		Content:   content,
		Diff:      diff,
		Headers:   headers,
		ListEmail: listEmail,
	}

	var submitter db.Person
	if q.DB.NewSelect().Model(&submitter).Where("id = ?", submitterID).Scan(q.Ctx) == nil {
		sub.SubmitterEmail = submitter.Email
		if submitter.Name != nil {
			sub.SubmitterName = *submitter.Name
		}
	}

	if isPatch && delegateID != nil {
		var delegate db.User
		if q.DB.NewSelect().Model(&delegate).Where("id = ?", *delegateID).Scan(q.Ctx) == nil {
			sub.DelegateEmail = delegate.Email
		}
	}

	// load comment contents for tag extraction
	if isPatch {
		var contents []string
		q.DB.NewSelect().Model((*db.PatchComment)(nil)).Column("content").
			Where("patch_id = ?", patchID).OrderExpr("date ASC").
			Scan(ctx, &contents)
		sub.CommentContents = contents
	} else {
		var contents []string
		q.DB.NewSelect().Model((*db.CoverComment)(nil)).Column("content").
			Where("cover_id = ?", patchID).OrderExpr("date ASC").
			Scan(ctx, &contents)
		sub.CommentContents = contents
	}

	return sub
}

func (h *webHandler) buildCoverMboxSubmission(ctx context.Context, cover db.Cover, project db.Project) mboxSubmission {
	return h.buildMboxSubmission(ctx, cover.ID, cover.Date,
		derefStr(cover.Content), "",
		cover.Headers, cover.SubmitterID, nil,
		project.Listemail, false)
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
