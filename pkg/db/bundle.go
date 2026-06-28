// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

func (q *Queries) ListUserBundles(userID int) ([]Bundle, error) {
	var bundles []Bundle
	err := q.DB.NewSelect().Model(&bundles).
		Where("owner_id = ?", userID).
		OrderExpr("name ASC").
		Scan(q.Ctx)
	return bundles, err
}
