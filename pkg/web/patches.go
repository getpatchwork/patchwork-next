// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package web

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/db"
	"github.com/getpatchwork/patchwork/pkg/events"
)

func (h *webHandler) PatchList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := db.GetQueries(ctx)
	linkname := chi.URLParam(r, "linkname")

	project, err := q.GetProjectByLinkname(linkname)
	if err != nil {
		notFoundPage(w)
		return
	}

	params := r.URL.Query()
	page, _ := strconv.Atoi(params.Get("page"))
	if page < 1 {
		page = 1
	}
	perPage := 200

	sq := q.DB.NewSelect().Model((*db.Patch)(nil)).
		Column("id", "msgid", "date", "submitter_id", "project_id",
			"name", "state_id", "delegate_id", "archived", "series_id").
		Where("project_id = ?", project.ID)

	var filters []appliedFilter
	baseQuery := fmt.Sprintf("/project/%s/list/", linkname)

	sq, filters = applyWebFilters(q.Ctx, q.DB, sq, params, baseQuery)

	sort := params.Get("order")
	if sort == "" {
		sort = "-date"
	}
	sq = applySort(sq, sort)

	total, _ := sq.Count(q.Ctx)
	totalPages := (total + perPage - 1) / perPage
	if page > totalPages && totalPages > 0 {
		page = totalPages
	}

	var patches []db.Patch
	sq.Relation("Submitter").Relation("State").Relation("Delegate").
		Offset((page-1)*perPage).Limit(perPage).Scan(q.Ctx, &patches)

	populateWebPatchTags(q, patches)

	var tagAbbrevs []string
	q.DB.NewSelect().Model((*db.Tag)(nil)).Column("abbrev").
		Where("show_column = ?", true).OrderExpr("id ASC").
		Scan(q.Ctx, &tagAbbrevs)

	seriesNames := make(map[int]string)
	var seriesIDs []int
	for _, p := range patches {
		if p.SeriesID != nil {
			seriesIDs = append(seriesIDs, *p.SeriesID)
		}
	}
	if len(seriesIDs) > 0 {
		type nameRow struct {
			ID   int     `bun:"id"`
			Name *string `bun:"name"`
		}
		var rows []nameRow
		q.DB.NewSelect().Model((*db.Series)(nil)).
			Column("id", "name").
			Where("id IN ?", bun.Tuple(seriesIDs)).
			Scan(q.Ctx, &rows)
		for _, r := range rows {
			if r.Name != nil {
				seriesNames[r.ID] = *r.Name
			}
		}
	}

	qp := url.Values{}
	if sort != "-date" {
		qp.Set("order", sort)
	}
	for k, v := range params {
		if k != "page" && k != "order" {
			qp[k] = v
		}
	}
	bq := ""
	if len(qp) > 0 {
		bq = "?" + qp.Encode()
	}

	states, _ := q.ListStates()

	var bundles []db.Bundle
	if user := getWebUser(r); user != nil {
		bundles, _ = q.ListUserBundles(user.ID)
	}

	delegates, _ := q.ListProjectMaintainers(project.ID)

	data := patchListData{
		PC:          h.pageCtx(r),
		Project:     *project,
		Patches:     patches,
		Filters:     filters,
		Sort:        sort,
		Page:        page,
		PerPage:     perPage,
		Total:       total,
		TotalPages:  totalPages,
		BaseQuery:   bq,
		SeriesNames: seriesNames,
		TagAbbrevs:  tagAbbrevs,
		Bundles:     bundles,
		States:      states,
		Delegates:   delegates,
	}
	patchListPage(data).Render(ctx, w)
}

func (h *webHandler) PatchListAction(w http.ResponseWriter, r *http.Request) {
	if !requireLogin(w, r) {
		return
	}
	ctx := r.Context()
	q := db.GetQueries(ctx)
	user := getWebUser(r)
	linkname := chi.URLParam(r, "linkname")

	if !h.validateCSRF(r) {
		http.Redirect(w, r, "/project/"+linkname+"/list/", http.StatusFound)
		return
	}

	r.ParseForm()
	action := r.FormValue("action")
	patchIDs := r.Form["patch_id"]

	if len(patchIDs) == 0 {
		http.Redirect(w, r, "/project/"+linkname+"/list/", http.StatusFound)
		return
	}

	project, err := q.GetProjectByLinkname(linkname)
	if err != nil {
		notFoundPage(w)
		return
	}

	switch action {
	case "update":
		uq := q.DB.NewUpdate().Model((*db.Patch)(nil)).
			Where("id IN ?", bun.Tuple(patchIDs)).
			Where("project_id = ?", project.ID)
		changed := false
		if stateID, _ := strconv.ParseInt(r.FormValue("change_state"), 10, 32); stateID > 0 {
			uq = uq.Set("state_id = ?", stateID)
			changed = true
		}
		if del := r.FormValue("change_delegate"); del == "clear" {
			uq = uq.Set("delegate_id = NULL")
			changed = true
		} else if delegateID, _ := strconv.ParseInt(del, 10, 32); delegateID > 0 {
			uq = uq.Set("delegate_id = ?", delegateID)
			changed = true
		}
		switch r.FormValue("change_archive") {
		case "true":
			uq = uq.Set("archived = ?", true)
			changed = true
		case "false":
			uq = uq.Set("archived = ?", false)
			changed = true
		}
		if changed {
			if _, err := uq.Exec(q.Ctx); err != nil {
				break
			}
		}

	case "add-to-bundle":
		bundleID, _ := strconv.ParseInt(r.FormValue("bundle_id"), 10, 32)
		if bundleID == 0 {
			break
		}
		var bundle db.Bundle
		err := q.DB.NewSelect().Model(&bundle).
			Where("id = ?", bundleID).
			Where("owner_id = ?", user.ID).
			Scan(q.Ctx)
		if err != nil {
			break
		}
		var maxOrder int
		q.DB.NewSelect().Model((*db.BundlePatch)(nil)).
			ColumnExpr("COALESCE(MAX(?), -1)", bun.Ident("order")).
			Where("bundle_id = ?", bundle.ID).
			Scan(q.Ctx, &maxOrder)
		for i, idStr := range patchIDs {
			patchID, _ := strconv.ParseInt(idStr, 10, 32)
			if patchID == 0 {
				continue
			}
			bp := db.BundlePatch{
				BundleID: bundle.ID,
				PatchID:  int(patchID),
				Order:    maxOrder + int(i) + 1,
			}
			_, _ = q.DB.NewInsert().Model(&bp).
				On("CONFLICT DO NOTHING").
				ExcludeColumn("id").Exec(q.Ctx)
		}

	case "create-bundle":
		name := strings.TrimSpace(r.FormValue("new_bundle"))
		if name == "" {
			break
		}
		bundle := db.Bundle{
			OwnerID:   user.ID,
			ProjectID: project.ID,
			Name:      name,
		}
		if err := q.Insert(&bundle); err != nil {
			break
		}
		for i, idStr := range patchIDs {
			patchID, _ := strconv.ParseInt(idStr, 10, 32)
			if patchID == 0 {
				continue
			}
			bp := db.BundlePatch{
				BundleID: bundle.ID,
				PatchID:  int(patchID),
				Order:    int(i),
			}
			_, _ = q.DB.NewInsert().Model(&bp).ExcludeColumn("id").Exec(q.Ctx)
		}
	}

	http.Redirect(w, r, "/project/"+linkname+"/list/", http.StatusFound)
}

func (h *webHandler) PatchDetailPage(w http.ResponseWriter, r *http.Request) {
	linkname := chi.URLParam(r, "linkname")
	rawMsgid, _ := url.PathUnescape(chi.URLParam(r, "msgid"))
	ctx := r.Context()
	q := db.GetQueries(ctx)
	msgid := "<" + rawMsgid + ">"

	project, err := q.GetProjectByLinkname(linkname)
	if err != nil {
		notFoundPage(w)
		return
	}

	var patch db.Patch
	err = q.DB.NewSelect().Model(&patch).
		Relation("Submitter").Relation("State").Relation("Delegate").
		Where("project_id = ?", project.ID).
		Where("msgid = ?", msgid).
		Scan(q.Ctx)
	if err != nil {
		notFoundPage(w)
		return
	}

	var series *db.Series
	if patch.SeriesID != nil {
		var s db.Series
		if q.DB.NewSelect().Model(&s).Where("id = ?", *patch.SeriesID).Scan(q.Ctx) == nil {
			series = &s
		}
	}

	comments, _ := q.ListPatchComments(patch.ID)

	var checks []db.Check
	q.DB.NewSelect().Model(&checks).
		Where("patch_id = ?", patch.ID).
		OrderExpr("date DESC").
		Scan(q.Ctx)

	var metadata map[string]string
	if series != nil {
		var rows []db.SeriesMetadata
		q.DB.NewSelect().Model(&rows).
			Where("series_id = ?", series.ID).
			Scan(q.Ctx)
		if len(rows) > 0 {
			metadata = make(map[string]string)
			for _, r := range rows {
				metadata[r.Key] = r.Value
			}
		}
	}

	var seriesPatches []seriesPatchRef
	var cover *db.Cover
	if series != nil {
		var sPatches []db.Patch
		q.DB.NewSelect().Model(&sPatches).
			Column("id", "msgid", "name").
			Where("series_id = ?", series.ID).
			OrderBy("number", bun.OrderAsc).
			Scan(q.Ctx)
		for _, sp := range sPatches {
			seriesPatches = append(seriesPatches, seriesPatchRef{
				Name:    sp.Name,
				URL:     patchURL(project.Linkname, sp.Msgid),
				Current: sp.ID == patch.ID,
			})
		}
		if series.CoverLetterID != nil {
			var c db.Cover
			if q.DB.NewSelect().Model(&c).
				Column("id", "msgid", "name").
				Where("id = ?", *series.CoverLetterID).
				Scan(q.Ctx) == nil {
				cover = &c
			}
		}
	}

	var states []db.State
	var delegates []db.User
	canEdit := false
	if user := getWebUser(r); user != nil {
		states, _ = q.ListStates()
		delegates, _ = q.ListProjectMaintainers(project.ID)
		canEdit = true
	}

	var related []db.PatchRef
	if patch.RelatedID != nil {
		var relPatches []db.Patch
		q.DB.NewSelect().Model(&relPatches).
			Column("id", "name").
			Where("related_id = ?", *patch.RelatedID).
			Where("id != ?", patch.ID).
			Scan(q.Ctx)
		for _, rp := range relPatches {
			related = append(related, db.PatchRef{ID: rp.ID, Name: rp.Name})
		}
	}

	var listArchiveURL string
	if project.ListArchiveURLFormat != "" {
		bare := strings.TrimPrefix(strings.TrimSuffix(patch.Msgid, ">"), "<")
		listArchiveURL = strings.ReplaceAll(
			project.ListArchiveURLFormat, "{}", url.PathEscape(bare),
		)
	}

	var prevSeries *db.Series
	var nextSeries []db.Series
	if series != nil {
		if series.PreviousSeriesID != nil {
			var ps db.Series
			if q.DB.NewSelect().Model(&ps).
				Where("id = ?", *series.PreviousSeriesID).
				Scan(q.Ctx) == nil {
				prevSeries = &ps
			}
		}
		q.DB.NewSelect().Model(&nextSeries).
			Column("id", "version").
			Where("previous_series_id = ?", series.ID).
			OrderExpr("version ASC").
			Scan(q.Ctx)
	}

	data := patchDetailData{
		PC:             h.pageCtx(r),
		Project:        *project,
		Patch:          patch,
		Comments:       comments,
		Checks:         checks,
		Series:         series,
		SeriesMetadata: metadata,
		SeriesPatches:  seriesPatches,
		Cover:          cover,
		States:         states,
		Delegates:      delegates,
		CanEdit:        canEdit,
		Related:        related,
		ListArchiveURL: listArchiveURL,
		PreviousSeries: prevSeries,
		NextSeries:     nextSeries,
	}
	patchDetailPage(data).Render(ctx, w)
}

func (h *webHandler) PatchUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireLogin(w, r) {
		return
	}
	ctx := r.Context()
	q := db.GetQueries(ctx)
	linkname := chi.URLParam(r, "linkname")
	rawMsgid, _ := url.PathUnescape(chi.URLParam(r, "msgid"))
	msgid := "<" + rawMsgid + ">"

	if !h.validateCSRF(r) {
		http.Redirect(w, r, patchURL(linkname, msgid), http.StatusFound)
		return
	}

	r.ParseForm()

	var patch db.Patch
	err := q.DB.NewSelect().Model(&patch).
		Where("project_id IN (SELECT id FROM project WHERE linkname = ?)", linkname).
		Where("msgid = ?", msgid).
		Scan(q.Ctx)
	if err != nil {
		notFoundPage(w)
		return
	}

	oldStateID := patch.StateID

	uq := q.DB.NewUpdate().Model(&patch).Where("id = ?", patch.ID)
	newStateID := int(0)
	if stateID, _ := strconv.ParseInt(r.FormValue("state"), 10, 64); stateID > 0 {
		newStateID = int(stateID)
		uq = uq.Set("state_id = ?", newStateID)
	}
	if del := r.FormValue("delegate"); del == "" {
		uq = uq.Set("delegate_id = NULL")
	} else if delegateID, _ := strconv.ParseInt(del, 10, 64); delegateID > 0 {
		uq = uq.Set("delegate_id = ?", delegateID)
	}
	uq = uq.Set("archived = ?", r.FormValue("archived") == "true")
	_, _ = uq.Exec(q.Ctx)

	if newStateID > 0 && (oldStateID == nil || *oldStateID != newStateID) {
		var p db.Patch
		if err := q.DB.NewSelect().Model(&p).
			Relation("Submitter").Relation("Project").Relation("State").
			Where("patch.id = ?", patch.ID).
			Scan(q.Ctx); err == nil {
			var oldState string
			if oldStateID != nil {
				var s db.State
				if err := q.DB.NewSelect().Model(&s).
					Where("id = ?", *oldStateID).
					Scan(q.Ctx); err == nil {
					oldState = s.Name
				}
			}
			actorID := 0
			if u := getWebUser(r); u != nil {
				actorID = u.ID
			}
			go events.PatchStateChanged(
				context.Background(), h.cfg, h.db,
				&p, actorID, oldState, p.State.Name,
			)
		}
	}

	http.Redirect(w, r, patchURL(linkname, msgid), http.StatusFound)
}

func (h *webHandler) CommentAddressed(w http.ResponseWriter, r *http.Request) {
	if !requireLogin(w, r) {
		return
	}
	ctx := r.Context()
	q := db.GetQueries(ctx)
	linkname := chi.URLParam(r, "linkname")
	rawMsgid, _ := url.PathUnescape(chi.URLParam(r, "msgid"))
	msgid := "<" + rawMsgid + ">"
	commentID, _ := strconv.ParseInt(chi.URLParam(r, "commentID"), 10, 32)

	if !h.validateCSRF(r) {
		http.Redirect(w, r, patchURL(linkname, msgid), http.StatusFound)
		return
	}

	addressed := r.FormValue("addressed") == "true"
	_, _ = q.DB.NewUpdate().Model((*db.PatchComment)(nil)).
		Set("addressed = ?", addressed).
		Where("id = ?", commentID).
		Exec(q.Ctx)

	http.Redirect(w, r, patchURL(linkname, msgid)+"#comment-"+strconv.FormatInt(int64(commentID), 10), http.StatusFound)
}

func (h *webHandler) PatchRawPage(w http.ResponseWriter, r *http.Request) {
	linkname := chi.URLParam(r, "linkname")
	rawMsgid, _ := url.PathUnescape(chi.URLParam(r, "msgid"))
	ctx := r.Context()
	q := db.GetQueries(ctx)
	msgid := "<" + rawMsgid + ">"

	var patch db.Patch
	err := q.DB.NewSelect().
		Model(&patch).
		Column("diff", "name").
		Join("JOIN project AS pr ON pr.id = patch.project_id").
		Where("pr.linkname = ?", linkname).
		Where("patch.msgid = ?", msgid).
		Scan(q.Ctx)
	if err != nil || patch.Diff == nil {
		notFoundPage(w)
		return
	}

	w.Header().Set("Content-Type", "text/x-patch; charset=utf-8")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%s.diff", sanitizeFilename(patch.Name)))
	_, _ = w.Write([]byte(*patch.Diff))
}

func (h *webHandler) PatchRedirect(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := db.GetQueries(ctx)
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		notFoundPage(w)
		return
	}

	var patch struct {
		Msgid     string
		ProjectID int
	}
	err = q.DB.NewSelect().Model((*db.Patch)(nil)).
		Column("msgid", "project_id").
		Where("id = ?", id).
		Scan(q.Ctx, &patch)
	if err != nil {
		notFoundPage(w)
		return
	}

	project, err := q.GetProjectByID(patch.ProjectID)
	if err != nil {
		notFoundPage(w)
		return
	}

	http.Redirect(w, r, patchURL(project.Linkname, patch.Msgid), http.StatusMovedPermanently)
}

func (h *webHandler) PatchMboxByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := db.GetQueries(ctx)
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		notFoundPage(w)
		return
	}

	patch, err := q.GetPatchByID(int(id))
	if err != nil {
		notFoundPage(w)
		return
	}

	project, err := q.GetProjectByID(patch.ProjectID)
	if err != nil {
		notFoundPage(w)
		return
	}

	h.servePatchMbox(w, *patch, *project)
}

func (h *webHandler) PatchRawByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := db.GetQueries(ctx)
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 32)
	if err != nil {
		notFoundPage(w)
		return
	}

	var patch db.Patch
	err = q.DB.NewSelect().Model(&patch).
		Column("diff", "name").
		Where("id = ?", id).Scan(q.Ctx)
	if err != nil || patch.Diff == nil {
		notFoundPage(w)
		return
	}

	w.Header().Set("Content-Type", "text/x-patch; charset=utf-8")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=%s.diff", sanitizeFilename(patch.Name)))
	_, _ = w.Write([]byte(*patch.Diff))
}

func applySort(sq *bun.SelectQuery, sort string) *bun.SelectQuery {
	desc := false
	field := sort
	if strings.HasPrefix(sort, "-") {
		desc = true
		field = sort[1:]
	}

	colMap := map[string]string{
		"date":      "date",
		"name":      "name",
		"submitter": "submitter_id",
		"delegate":  "delegate_id",
		"state":     "state_id",
	}

	col, ok := colMap[field]
	if !ok {
		col = "date"
		desc = true
	}

	if desc {
		return sq.OrderExpr(col + " DESC")
	}
	return sq.OrderExpr(col + " ASC")
}

func populateWebPatchTags(q *db.Queries, patches []db.Patch) {
	if len(patches) == 0 {
		return
	}

	ids := make([]int, len(patches))
	for i := range patches {
		ids[i] = patches[i].ID
	}

	type tagRow struct {
		PatchID int    `bun:"patch_id"`
		Abbrev  string `bun:"abbrev"`
		Count   int    `bun:"count"`
	}
	var tagRows []tagRow
	q.DB.NewSelect().
		Model((*db.PatchTag)(nil)).
		ColumnExpr("patch_tag.patch_id, t.abbrev, patch_tag.count").
		Join("JOIN tag AS t ON t.id = patch_tag.tag_id").
		Where("patch_tag.patch_id IN ?", bun.Tuple(ids)).
		Scan(q.Ctx, &tagRows)

	tagMap := make(map[int]map[string]int)
	for _, r := range tagRows {
		if tagMap[r.PatchID] == nil {
			tagMap[r.PatchID] = make(map[string]int)
		}
		tagMap[r.PatchID][r.Abbrev] = r.Count
	}

	type checkRow struct {
		PatchID int `bun:"patch_id"`
		State   int `bun:"state"`
		Count   int `bun:"count"`
	}
	var checkRows []checkRow
	q.DB.NewSelect().
		Model((*db.Check)(nil)).
		Column("patch_id", "state").
		ColumnExpr("count(*) AS count").
		Where("patch_id IN ?", bun.Tuple(ids)).
		GroupExpr("patch_id, state").
		Scan(q.Ctx, &checkRows)

	checkMap := make(map[int][4]int)
	for _, r := range checkRows {
		c := checkMap[r.PatchID]
		if r.State >= 0 && r.State < 4 {
			c[r.State] = r.Count
		}
		checkMap[r.PatchID] = c
	}

	for i := range patches {
		patches[i].Tags = tagMap[patches[i].ID]
		if patches[i].Tags == nil {
			patches[i].Tags = map[string]int{}
		}
		patches[i].CheckCounts = checkMap[patches[i].ID]
	}
}
