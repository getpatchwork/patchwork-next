// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import (
	"strings"
)

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
