// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import (
	"context"

	"github.com/uptrace/bun"
)

func SeedDefaults(ctx context.Context, database bun.IDB) error {
	states := []State{
		{Name: "New", Slug: "new", Ordering: 0, ActionRequired: true},
		{Name: "Under Review", Slug: "under-review", Ordering: 1, ActionRequired: true},
		{Name: "Accepted", Slug: "accepted", Ordering: 2},
		{Name: "Rejected", Slug: "rejected", Ordering: 3},
		{Name: "RFC", Slug: "rfc", Ordering: 4},
		{Name: "Not Applicable", Slug: "not-applicable", Ordering: 5},
		{Name: "Changes Requested", Slug: "changes-requested", Ordering: 6},
		{Name: "Awaiting Upstream", Slug: "awaiting-upstream", Ordering: 7},
		{Name: "Superseded", Slug: "superseded", Ordering: 8},
		{Name: "Deferred", Slug: "deferred", Ordering: 9},
	}
	for i := range states {
		_, err := database.NewInsert().
			Model(&states[i]).
			On("CONFLICT DO NOTHING").
			Exec(ctx)
		if err != nil {
			return err
		}
	}

	tags := []Tag{
		{Name: "Acked-by", Pattern: `^Acked-by:`, Abbrev: "A", ShowColumn: true},
		{Name: "Reviewed-by", Pattern: `^Reviewed-by:`, Abbrev: "R", ShowColumn: true},
		{Name: "Tested-by", Pattern: `^Tested-by:`, Abbrev: "T", ShowColumn: true},
	}
	for i := range tags {
		_, err := database.NewInsert().
			Model(&tags[i]).
			On("CONFLICT DO NOTHING").
			Exec(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}
