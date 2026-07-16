// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import "time"

func (q *Queries) CleanOldEvents(cutoff time.Time) (int64, error) {
	res, err := q.DB.NewDelete().Model((*Event)(nil)).
		Where("date < ?", cutoff).Exec(q.Ctx)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}
