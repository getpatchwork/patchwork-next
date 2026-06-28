// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import (
	"regexp"
)

// RefreshTagCounts recalculates tag counts for a patch by scanning
// the patch content and all its comments for tag patterns.
func (q *Queries) RefreshTagCounts(patch *Patch) error {
	// load tags
	var tags []Tag
	err := q.DB.NewSelect().Model(&tags).Scan(q.Ctx)
	if err != nil {
		return err
	}

	// collect all text to scan: patch content + all comment contents
	var contents []string
	if patch.Content != nil && *patch.Content != "" {
		contents = append(contents, *patch.Content)
	}

	var comments []PatchComment
	q.DB.NewSelect().Model(&comments).
		Where("patch_id = ?", patch.ID).
		Scan(q.Ctx)
	for _, c := range comments {
		if c.Content != nil && *c.Content != "" {
			contents = append(contents, *c.Content)
		}
	}

	// count matches for each tag across all content
	for _, tag := range tags {
		re, err := regexp.Compile("(?mi)" + tag.Pattern)
		if err != nil {
			continue
		}
		count := 0
		for _, content := range contents {
			count += len(re.FindAllString(content, -1))
		}

		if count == 0 {
			if _, err := q.DB.NewDelete().Model((*PatchTag)(nil)).
				Where("patch_id = ?", patch.ID).
				Where("tag_id = ?", tag.ID).
				Exec(q.Ctx); err != nil {
				return err
			}
		} else {
			pt := &PatchTag{
				PatchID: patch.ID,
				TagID:   tag.ID,
				Count:   int(count),
			}
			_, err := q.DB.NewInsert().Model(pt).
				On("CONFLICT (patch_id, tag_id) DO UPDATE").
				Set("count = EXCLUDED.count").
				Exec(q.Ctx)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
