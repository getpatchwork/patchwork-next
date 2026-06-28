// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import (
	"github.com/getpatchwork/patchwork/cmd/pw/pw"
	"github.com/getpatchwork/patchwork/pkg/db/migrations"
	"github.com/getpatchwork/patchwork/pkg/log"
)

type SyncCmd struct{}

func (c *SyncCmd) Run(ctx *pw.Context) error {
	if err := migrations.RunMigrations(ctx, ctx.DB); err != nil {
		return err
	}
	log.Noticef("database schema is up to date")
	return nil
}
