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
	"github.com/getpatchwork/patchwork/pkg/log"
)

func strp(s string) *string { return &s }
func boolp(b bool) *bool    { return &b }

func userToEmbedded(u *db.User, base string) UserEmbedded {
	return UserEmbedded{
		ID:        u.ID,
		URL:       fmt.Sprintf("%s/users/%d", base, u.ID),
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
		URL:   fmt.Sprintf("%s/people/%d", base, p.ID),
		Name:  name,
		Email: p.Email,
	}
}

func loadSeriesDetail(ctx context.Context, q *db.Queries, base string, series []db.Series) {
	for i := range series {
		s := &series[i]

		count, err := q.Select((*db.Patch)(nil)).
			Where("series_id = ?", s.ID).
			Count(ctx)
		if err != nil {
			log.Errorf("count series patches: %v", err)
		}
		s.ReceivedTotal = int(count)
		s.ReceivedAll = count >= int64(s.Total)

		if s.CoverLetterID != nil {
			var cover db.Cover
			if err := q.Select(&cover).
				Where("id = ?", *s.CoverLetterID).
				Scan(ctx); err == nil {
				s.CoverLetter = &cover
			}
		}

		patches, err := q.ListSeriesPatches(s.ID)
		if err != nil {
			log.Errorf("load series patches: %v", err)
		}
		if patches == nil {
			patches = []db.Patch{}
		}
		s.Patches = patches

		s.Metadata, err = q.GetSeriesMetadata(s.ID)
		if err != nil {
			log.Errorf("load series metadata: %v", err)
		}

		var depIDs []int
		if err := q.Select((*db.SeriesDependencies)(nil)).
			Column("to_series_id").
			Where("from_series_id = ?", s.ID).
			Scan(ctx, &depIDs); err != nil {
			log.Errorf("load series dependencies: %v", err)
		}
		s.Dependencies = make([]string, len(depIDs))
		for j, id := range depIDs {
			s.Dependencies[j] = fmt.Sprintf("%s/series/%d", base, id)
		}

		var revIDs []int
		if err := q.Select((*db.SeriesDependencies)(nil)).
			Column("from_series_id").
			Where("to_series_id = ?", s.ID).
			Scan(ctx, &revIDs); err != nil {
			log.Errorf("load series dependents: %v", err)
		}
		s.Dependents = make([]string, len(revIDs))
		for j, id := range revIDs {
			s.Dependents[j] = fmt.Sprintf("%s/series/%d", base, id)
		}

		if s.PreviousSeriesID != nil {
			u := fmt.Sprintf("%s/series/%d", base, *s.PreviousSeriesID)
			s.PreviousSeries = &u
		}

		nextSeriesList, err := q.ListNextSeries(s.ID)
		if err != nil {
			log.Errorf("load next series: %v", err)
		}
		s.NextSeries = make([]string, len(nextSeriesList))
		for j, ns := range nextSeriesList {
			s.NextSeries[j] = fmt.Sprintf("%s/series/%d", base, ns.ID)
		}
	}
}

func setCheckURLs(base string, patchID int, checks []db.Check) {
	for i := range checks {
		checks[i].URL = fmt.Sprintf("%s/patches/%d/checks/%d", base, patchID, checks[i].ID)
	}
}

func populateCommentURLs(base string, patchID int, comments []db.PatchComment) {
	for i := range comments {
		comments[i].URL = fmt.Sprintf("%s/patches/%d/comments/%d",
			base, patchID, comments[i].ID)
		comments[i].Subject = parseSubjectFromHeaders(comments[i].Headers)
	}
}

func populateCoverCommentURLs(base string, coverID int, comments []db.CoverComment) {
	for i := range comments {
		comments[i].URL = fmt.Sprintf("%s/covers/%d/comments/%d",
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

			remaining, err := tx.NewSelect().Model((*db.Patch)(nil)).
				Where("related_id = ?", oldRelID).
				Count(ctx)
			if err != nil {
				log.Errorf("count remaining related: %v", err)
			}
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

func buildLinkHeader(page int, perPage, total int64) string {
	lastPage := (total + perPage - 1) / perPage
	if lastPage < 1 {
		lastPage = 1
	}
	link := fmt.Sprintf("</?page=1>; rel=\"first\", </?page=%d>; rel=\"last\"", lastPage)
	if page > 1 {
		link += fmt.Sprintf(", </?page=%d>; rel=\"prev\"", page-1)
	}
	if int64(page) < lastPage {
		link += fmt.Sprintf(", </?page=%d>; rel=\"next\"", page+1)
	}
	return link
}
