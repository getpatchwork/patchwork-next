// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	Register(up0002, down0002)
}

func up0002(ctx context.Context, tx bun.Tx) error {
	_, err := tx.NewCreateIndex().
		Table("patch").
		Index("idx_patch_project_id_archived_date_desc").
		ColumnExpr("project_id, archived, date DESC").
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return err
	}

	for _, idx := range []string{
		"idx_patch_date",
		"idx_ci_check_context",
		"idx_event_category",
		"idx_series_metadata_key",
		"idx_cover_date_project_id_submitter_id_name",
	} {
		_, err := tx.NewDropIndex().Index(idx).IfExists().Exec(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func down0002(ctx context.Context, tx bun.Tx) error {
	_, err := tx.NewDropIndex().
		Index("idx_patch_project_id_archived_date_desc").
		IfExists().
		Exec(ctx)
	if err != nil {
		return err
	}

	_, err = tx.NewCreateIndex().
		Table("patch").
		Index("idx_patch_date").
		Column("date").
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return err
	}
	_, err = tx.NewCreateIndex().
		Table("ci_check").
		Index("idx_ci_check_context").
		Column("context").
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return err
	}
	_, err = tx.NewCreateIndex().
		Table("event").
		Index("idx_event_category").
		Column("category").
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return err
	}
	_, err = tx.NewCreateIndex().
		Table("series_metadata").
		Index("idx_series_metadata_key").
		Column("key").
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return err
	}
	_, err = tx.NewCreateIndex().
		Table("cover").
		Index("idx_cover_date_project_id_submitter_id_name").
		Column("date", "project_id", "submitter_id", "name").
		IfNotExists().
		Exec(ctx)
	return err
}
