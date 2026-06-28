// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import (
	"os"

	"github.com/getpatchwork/patchwork/cmd/pw/pw"
	"github.com/getpatchwork/patchwork/pkg/db"
)

type ExportCmd struct{}

func (c *ExportCmd) Run(ctx *pw.Context) error {
	return db.Export(ctx, ctx.DB, os.Stdout)
}
