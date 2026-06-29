// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package admin

import (
	"fmt"

	"github.com/getpatchwork/patchwork/cmd/pw/pw"
	"github.com/getpatchwork/patchwork/pkg/db"
)

type GcCmd struct{}

func (c *GcCmd) Run(ctx *pw.Context) error {
	q := db.New(ctx, ctx.DB)

	n, err := q.CleanExpiredSessions()
	if err != nil {
		return fmt.Errorf("sessions: %w", err)
	}
	if n > 0 {
		fmt.Printf("deleted %d expired session(s)\n", n)
	}

	n, err = q.CleanExpiredConfirmations()
	if err != nil {
		return fmt.Errorf("confirmations: %w", err)
	}
	if n > 0 {
		fmt.Printf("deleted %d expired confirmation(s)\n", n)
	}

	n, err = q.CleanInactiveUsers()
	if err != nil {
		return fmt.Errorf("inactive users: %w", err)
	}
	if n > 0 {
		fmt.Printf("deleted %d inactive user(s)\n", n)
	}

	return nil
}
