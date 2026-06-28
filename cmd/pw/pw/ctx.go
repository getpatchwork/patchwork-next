// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package pw

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/config"
)

type Context struct {
	context.Context

	Config *config.Config
	DB     *bun.DB
}

func (c *Context) Value(key any) any {
	if key, ok := key.(string); ok {
		switch key {
		case "cfg", "config":
			return c.Config
		case "db", "database":
			return c.DB
		}
	}
	return nil
}
