// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package admin

type CLI struct {
	Project ProjectCmd `cmd:"" help:"Manage projects."`
	User    UserCmd    `cmd:"" help:"Manage users."`
}
