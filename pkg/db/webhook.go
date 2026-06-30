// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import (
	"fmt"
	"strings"
)

var ValidEventCategories = map[string]bool{
	"cover-created":          true,
	"patch-created":          true,
	"patch-completed":        true,
	"patch-state-changed":    true,
	"patch-delegated":        true,
	"patch-relation-changed": true,
	"check-created":          true,
	"check-updated":          true,
	"series-created":         true,
	"series-completed":       true,
	"cover-comment-created":  true,
	"patch-comment-created":  true,
}

func ValidateEvents(events string) error {
	if events == "*" {
		return nil
	}
	for _, e := range strings.Split(events, ",") {
		if !ValidEventCategories[strings.TrimSpace(e)] {
			return fmt.Errorf("invalid event category: %q", strings.TrimSpace(e))
		}
	}
	return nil
}

func (q *Queries) GetActiveWebhooks(projectID int) ([]Webhook, error) {
	var hooks []Webhook
	err := q.DB.NewSelect().Model(&hooks).
		Where("project_id = ?", projectID).
		Where("active = ?", true).
		Scan(q.Ctx)
	return hooks, err
}

func (w *Webhook) MatchesEvent(category string) bool {
	if w.Events == "*" {
		return true
	}
	for _, e := range strings.Split(w.Events, ",") {
		if strings.TrimSpace(e) == category {
			return true
		}
	}
	return false
}
