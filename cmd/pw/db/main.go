// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

// CLI groups the database management subcommands.
type CLI struct {
	Sync SyncCmd `cmd:"" help:"Create or update the database schema."`
}
