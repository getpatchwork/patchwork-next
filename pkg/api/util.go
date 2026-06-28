// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/emersion/go-message/mail"
	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/db"
)

func strp(s string) *string { return &s }
func boolp(b bool) *bool    { return &b }

func userToEmbedded(u *db.User, base string) UserEmbedded {
	return UserEmbedded{
		ID:        u.ID,
		URL:       fmt.Sprintf("%s/users/%d/", base, u.ID),
		Username:  u.Username,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Email:     u.Email,
	}
}

func personToEmbedded(p *db.Person, base string) PersonEmbedded {
	name := ""
	if p.Name != nil {
		name = *p.Name
	}
	return PersonEmbedded{
		ID:    p.ID,
		URL:   fmt.Sprintf("%s/people/%d/", base, p.ID),
		Name:  name,
		Email: p.Email,
	}
}

func loadProjectMaintainers(ctx context.Context, database bun.IDB, projects []db.Project) {
	if len(projects) == 0 {
		return
	}
	projectIDs := make([]int, len(projects))
	for i := range projects {
		projectIDs[i] = projects[i].ID
	}

	type maintainerRow struct {
		ProjectID int `bun:"project_id"`
		db.User
	}
	var rows []maintainerRow
	database.NewSelect().
		Model((*db.ProjectMaintainer)(nil)).
		ColumnExpr("project_maintainer.project_id, u.*").
		Join("JOIN auth_user AS u ON u.id = project_maintainer.user_id").
		Where("project_maintainer.project_id IN ?", bun.Tuple(projectIDs)).
		Scan(ctx, &rows)

	byProject := make(map[int][]db.User)
	for _, r := range rows {
		byProject[r.ProjectID] = append(byProject[r.ProjectID], r.User)
	}
	for i := range projects {
		projects[i].Maintainers = byProject[projects[i].ID]
		if projects[i].Maintainers == nil {
			projects[i].Maintainers = []db.User{}
		}
	}
}

func dedup(ids []int) []int {
	seen := make(map[int]bool, len(ids))
	result := make([]int, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
}

func loadPatchTags(ctx context.Context, database bun.IDB, patches []db.Patch) {
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
	var rows []tagRow
	database.NewSelect().
		Model((*db.PatchTag)(nil)).
		ColumnExpr("patch_tag.patch_id, tag.abbrev, patch_tag.count").
		Join("JOIN tag ON tag.id = patch_tag.tag_id").
		Where("patch_tag.patch_id IN ?", bun.Tuple(dedup(ids))).
		Scan(ctx, &rows)

	tagMap := make(map[int]map[string]int)
	for _, r := range rows {
		if tagMap[r.PatchID] == nil {
			tagMap[r.PatchID] = make(map[string]int)
		}
		tagMap[r.PatchID][r.Abbrev] = int(r.Count)
	}

	for i := range patches {
		patches[i].Tags = tagMap[patches[i].ID]
		if patches[i].Tags == nil {
			patches[i].Tags = map[string]int{}
		}
	}
}

func loadPatchSeries(ctx context.Context, database bun.IDB, patches []db.Patch) {
	var seriesIDs []int
	for i := range patches {
		if patches[i].SeriesID != nil {
			seriesIDs = append(seriesIDs, *patches[i].SeriesID)
		}
	}
	seriesMap := make(map[int]*db.Series)
	if len(seriesIDs) > 0 {
		var series []db.Series
		database.NewSelect().Model(&series).
			Where("id IN ?", bun.Tuple(dedup(seriesIDs))).
			Scan(ctx)
		for i := range series {
			seriesMap[series[i].ID] = &series[i]
		}
	}
	for i := range patches {
		if patches[i].SeriesID != nil {
			if s, ok := seriesMap[*patches[i].SeriesID]; ok {
				patches[i].SeriesList = []db.SeriesRef{
					{ID: s.ID, Name: s.Name},
				}
			}
		}
		if patches[i].SeriesList == nil {
			patches[i].SeriesList = []db.SeriesRef{}
		}
	}
}

func loadPatchRelated(ctx context.Context, database bun.IDB, patches []db.Patch) {
	var relatedIDs []int
	for i := range patches {
		if patches[i].RelatedID != nil {
			relatedIDs = append(relatedIDs, *patches[i].RelatedID)
		}
	}
	relatedMap := make(map[int][]db.PatchRef)
	if len(relatedIDs) > 0 {
		var related []db.Patch
		database.NewSelect().Model(&related).
			Column("id", "name", "related_id").
			Where("related_id IN ?", bun.Tuple(dedup(relatedIDs))).
			Scan(ctx)
		for _, r := range related {
			if r.RelatedID != nil {
				relatedMap[*r.RelatedID] = append(relatedMap[*r.RelatedID],
					db.PatchRef{ID: r.ID, Name: r.Name})
			}
		}
	}
	for i := range patches {
		if patches[i].RelatedID != nil {
			for _, ref := range relatedMap[*patches[i].RelatedID] {
				if ref.ID != patches[i].ID {
					patches[i].Related = append(patches[i].Related, ref)
				}
			}
		}
		if patches[i].Related == nil {
			patches[i].Related = []db.PatchRef{}
		}
	}
}

func loadCombinedCheck(ctx context.Context, database bun.IDB, patches []db.Patch) {
	if len(patches) == 0 {
		return
	}
	ids := make([]int, len(patches))
	for i := range patches {
		ids[i] = patches[i].ID
	}

	type checkRow struct {
		PatchID int `bun:"patch_id"`
		State   int `bun:"state"`
	}
	var rows []checkRow
	database.NewSelect().
		Model((*db.Check)(nil)).
		Column("patch_id", "state").
		Distinct().
		Where("patch_id IN ?", bun.Tuple(dedup(ids))).
		Scan(ctx, &rows)

	statesByPatch := make(map[int][]int)
	for _, r := range rows {
		statesByPatch[r.PatchID] = append(statesByPatch[r.PatchID], r.State)
	}

	for i := range patches {
		states := statesByPatch[patches[i].ID]
		if len(states) == 0 {
			continue
		}
		combined := "success"
		for _, s := range states {
			switch db.CheckState(s) {
			case db.CheckFail:
				combined = "fail"
			case db.CheckWarning:
				if combined != "fail" {
					combined = "warning"
				}
			case db.CheckPending:
				if combined != "fail" && combined != "warning" {
					combined = "pending"
				}
			}
		}
		patches[i].CombinedCheck = &combined
	}
}

func loadCoverSeries(ctx context.Context, database bun.IDB, covers []db.Cover) {
	if len(covers) == 0 {
		return
	}
	coverIDs := make([]int, len(covers))
	for i := range covers {
		coverIDs[i] = covers[i].ID
	}

	var series []db.Series
	database.NewSelect().Model(&series).
		Where("cover_letter_id IN ?", bun.Tuple(dedup(coverIDs))).
		Scan(ctx)

	byCover := make(map[int]*db.Series)
	for i := range series {
		if series[i].CoverLetterID != nil {
			byCover[*series[i].CoverLetterID] = &series[i]
		}
	}

	for i := range covers {
		if s, ok := byCover[covers[i].ID]; ok {
			covers[i].SeriesList = []db.SeriesRef{
				{ID: s.ID, Name: s.Name},
			}
		}
		if covers[i].SeriesList == nil {
			covers[i].SeriesList = []db.SeriesRef{}
		}
	}
}

func loadSeriesDetail(ctx context.Context, database bun.IDB, base string, series []db.Series) {
	for i := range series {
		s := &series[i]

		count, _ := database.NewSelect().Model((*db.Patch)(nil)).
			Where("series_id = ?", s.ID).
			Count(ctx)
		s.ReceivedTotal = count
		s.ReceivedAll = count >= int(s.Total)

		if s.CoverLetterID != nil {
			var cover db.Cover
			if err := database.NewSelect().Model(&cover).
				Where("id = ?", *s.CoverLetterID).
				Scan(ctx); err == nil {
				s.CoverLetter = &cover
			}
		}

		var patches []db.Patch
		database.NewSelect().Model(&patches).
			Where("series_id = ?", s.ID).
			OrderExpr("number ASC").
			Scan(ctx)
		if patches == nil {
			patches = []db.Patch{}
		}
		s.Patches = patches

		var meta []db.SeriesMetadata
		database.NewSelect().Model(&meta).
			Where("series_id = ?", s.ID).
			Scan(ctx)
		s.Metadata = make(map[string]string, len(meta))
		for _, m := range meta {
			s.Metadata[m.Key] = m.Value
		}

		var depIDs []int
		database.NewSelect().
			Model((*db.SeriesDependencies)(nil)).
			Column("to_series_id").
			Where("from_series_id = ?", s.ID).
			Scan(ctx, &depIDs)
		s.Dependencies = make([]string, len(depIDs))
		for j, id := range depIDs {
			s.Dependencies[j] = fmt.Sprintf("%s/series/%d/", base, id)
		}

		var revIDs []int
		database.NewSelect().
			Model((*db.SeriesDependencies)(nil)).
			Column("from_series_id").
			Where("to_series_id = ?", s.ID).
			Scan(ctx, &revIDs)
		s.Dependents = make([]string, len(revIDs))
		for j, id := range revIDs {
			s.Dependents[j] = fmt.Sprintf("%s/series/%d/", base, id)
		}

		if s.PreviousSeriesID != nil {
			u := fmt.Sprintf("%s/series/%d/", base, *s.PreviousSeriesID)
			s.PreviousSeries = &u
		}

		var nextIDs []int
		database.NewSelect().
			Model((*db.Series)(nil)).
			Column("id").
			Where("previous_series_id = ?", s.ID).
			Scan(ctx, &nextIDs)
		s.NextSeries = make([]string, len(nextIDs))
		for j, id := range nextIDs {
			s.NextSeries[j] = fmt.Sprintf("%s/series/%d/", base, id)
		}
	}
}

func setCheckURLs(base string, patchID int, checks []db.Check) {
	for i := range checks {
		checks[i].URL = fmt.Sprintf("%s/patches/%d/checks/%d/", base, patchID, checks[i].ID)
	}
}

func populateCommentURLs(base string, patchID int, comments []db.PatchComment) {
	for i := range comments {
		comments[i].URL = fmt.Sprintf("%s/patches/%d/comments/%d/",
			base, patchID, comments[i].ID)
		comments[i].Subject = parseSubjectFromHeaders(comments[i].Headers)
	}
}

func populateCoverCommentURLs(base string, coverID int, comments []db.CoverComment) {
	for i := range comments {
		comments[i].URL = fmt.Sprintf("%s/covers/%d/comments/%d/",
			base, coverID, comments[i].ID)
		comments[i].Subject = parseSubjectFromHeaders(comments[i].Headers)
	}
}

func parseHeadersMap(raw string) map[string]string {
	m := make(map[string]string)
	var currentKey, currentVal string
	for _, line := range strings.Split(raw, "\n") {
		if line == "" {
			continue
		}
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			currentVal += "\n" + line
		} else {
			if currentKey != "" {
				m[currentKey] = currentVal
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
		m[currentKey] = currentVal
	}
	return m
}

func parseSubjectFromHeaders(headers string) string {
	if headers == "" {
		return ""
	}
	raw := strings.ReplaceAll(headers, "\n", "\r\n")
	if !strings.HasSuffix(raw, "\r\n\r\n") {
		raw += "\r\n"
	}
	m, err := mail.CreateReader(strings.NewReader(raw))
	if err != nil {
		return ""
	}
	subject, _ := m.Header.Subject()
	return subject
}

func listArchiveURL(project *db.Project, msgid string) string {
	if project == nil || project.ListArchiveURLFormat == "" {
		return ""
	}
	bare := strings.TrimPrefix(strings.TrimSuffix(msgid, ">"), "<")
	return strings.ReplaceAll(project.ListArchiveURLFormat, "{}", url.PathEscape(bare))
}

func loadBundlePatches(ctx context.Context, database bun.IDB, bundles []db.Bundle) {
	for i := range bundles {
		var patches []db.Patch
		database.NewSelect().
			Model(&patches).
			Join("JOIN bundle_patch AS bp ON bp.patch_id = patch.id").
			Where("bp.bundle_id = ?", bundles[i].ID).
			Scan(ctx)
		if patches == nil {
			patches = []db.Patch{}
		}
		bundles[i].BundlePatches = patches
	}
}

func updateRelated(
	ctx context.Context, database bun.IDB,
	user *db.User, patch *db.Patch, relatedIDs []int,
) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if len(relatedIDs) == 0 {
		if patch.RelatedID != nil {
			oldRelID := *patch.RelatedID
			if _, err := tx.NewUpdate().Model((*db.Patch)(nil)).
				Set("related_id = NULL").
				Where("id = ?", patch.ID).
				Exec(ctx); err != nil {
				return err
			}
			patch.RelatedID = nil

			remaining, _ := tx.NewSelect().Model((*db.Patch)(nil)).
				Where("related_id = ?", oldRelID).
				Count(ctx)
			if remaining < 2 {
				if _, err := tx.NewUpdate().Model((*db.Patch)(nil)).
					Set("related_id = NULL").
					Where("related_id = ?", oldRelID).
					Exec(ctx); err != nil {
					return err
				}
				if _, err := tx.NewDelete().Model((*db.PatchRelation)(nil)).
					Where("id = ?", oldRelID).
					Exec(ctx); err != nil {
					return err
				}
			}
		}
		return tx.Commit()
	}

	for _, pid := range relatedIDs {
		var p db.Patch
		if err := tx.NewSelect().Model(&p).
			Where("id = ?", pid).Column("id", "project_id", "related_id").
			Scan(ctx); err != nil {
			return fmt.Errorf("patch %d not found", pid)
		}
		if !db.GetQueries(ctx).IsMaintainer(user, p.ProjectID) {
			return fmt.Errorf("forbidden")
		}
		if p.RelatedID != nil && patch.RelatedID != nil && *p.RelatedID != *patch.RelatedID {
			return fmt.Errorf("conflict")
		}
		if p.RelatedID != nil && patch.RelatedID == nil {
			if _, err := tx.NewUpdate().Model((*db.Patch)(nil)).
				Set("related_id = ?", *p.RelatedID).
				Where("id = ?", patch.ID).
				Exec(ctx); err != nil {
				return err
			}
			patch.RelatedID = p.RelatedID
		}
	}

	if patch.RelatedID == nil {
		var relID int
		if err := tx.QueryRowContext(
			ctx,
			"INSERT INTO patch_relation DEFAULT VALUES RETURNING id",
		).Scan(&relID); err != nil {
			return err
		}
		patch.RelatedID = &relID
		if _, err := tx.NewUpdate().Model((*db.Patch)(nil)).
			Set("related_id = ?", relID).
			Where("id = ?", patch.ID).
			Exec(ctx); err != nil {
			return err
		}
	}

	for _, pid := range relatedIDs {
		if _, err := tx.NewUpdate().Model((*db.Patch)(nil)).
			Set("related_id = ?", *patch.RelatedID).
			Where("id = ?", pid).
			Exec(ctx); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func buildLinkHeader(page, perPage, total int) string {
	lastPage := (total + perPage - 1) / perPage
	if lastPage < 1 {
		lastPage = 1
	}
	link := fmt.Sprintf("</?page=1>; rel=\"first\", </?page=%d>; rel=\"last\"", lastPage)
	if page > 1 {
		link += fmt.Sprintf(", </?page=%d>; rel=\"prev\"", page-1)
	}
	if page < lastPage {
		link += fmt.Sprintf(", </?page=%d>; rel=\"next\"", page+1)
	}
	return link
}
