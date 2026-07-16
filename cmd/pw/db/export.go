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

type ExportCmd struct {
	Dialect string `help:"Target dialect. Detected from the database URL by default." enum:"auto,postgres,mysql,sqlite" default:"auto"`
}

func (c *ExportCmd) Run(ctx *pw.Context) error {
	return db.Export(ctx, ctx.DB, os.Stdout, c.Dialect)
}
