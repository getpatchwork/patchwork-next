// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import (
	"bufio"
	"os"
	"strings"

	"github.com/getpatchwork/patchwork/cmd/pw/pw"
	"github.com/getpatchwork/patchwork/pkg/log"
)

type ImportCmd struct{}

func (c *ImportCmd) Run(ctx *pw.Context) error {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)

	var stmt strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		if stmt.Len() > 0 {
			stmt.WriteByte('\n')
		}
		stmt.WriteString(line)
		if strings.HasSuffix(line, ";") {
			if _, err := ctx.DB.ExecContext(ctx, stmt.String()); err != nil {
				return err
			}
			stmt.Reset()
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if stmt.Len() > 0 {
		if _, err := ctx.DB.ExecContext(ctx, stmt.String()); err != nil {
			return err
		}
	}
	log.Noticef("import complete")
	return nil
}
