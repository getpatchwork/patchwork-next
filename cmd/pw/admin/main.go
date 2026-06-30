// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package admin

type CLI struct {
	Project      ProjectCmd      `cmd:"" help:"Manage projects."`
	User         UserCmd         `cmd:"" help:"Manage users."`
	Tag          TagCmd          `cmd:"" help:"Manage tags."`
	State        StateCmd        `cmd:"" help:"Manage states."`
	Maintainer   MaintainerCmd   `cmd:"" help:"Manage project maintainers."`
	DelegateRule DelegateRuleCmd `cmd:"" name:"delegate-rule" help:"Manage delegation rules."`
	Webhook      WebhookCmd      `cmd:"" help:"Manage webhooks."`
	Gc           GcCmd           `cmd:"" help:"Garbage collect stale data."`
}
