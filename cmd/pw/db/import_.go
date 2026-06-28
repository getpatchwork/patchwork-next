// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import (
	"io"
	"os"

	"github.com/getpatchwork/patchwork/cmd/pw/pw"
	"github.com/getpatchwork/patchwork/pkg/log"
)

type ImportCmd struct{}

func (c *ImportCmd) Run(ctx *pw.Context) error {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	_, err = ctx.DB.ExecContext(ctx, string(data))
	if err != nil {
		return err
	}
	log.Noticef("import complete")
	return nil
}
