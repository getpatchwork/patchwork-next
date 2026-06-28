// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package events

import (
	"context"
	"fmt"
	"strings"

	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/config"
	"github.com/getpatchwork/patchwork/pkg/db"
	"github.com/getpatchwork/patchwork/pkg/log"
)

func PatchStateChanged(
	ctx context.Context,
	cfg *config.Config,
	database bun.IDB,
	patch *db.Patch,
	actorID int,
	oldState, newState string,
) {
	if patch.Project == nil || !patch.Project.SendNotifications {
		return
	}
	if patch.Submitter == nil || patch.Submitter.UserID == nil {
		return
	}
	if *patch.Submitter.UserID == actorID {
		return
	}
	rcpt := patch.Submitter.Email
	if rcpt == "" || rcpt == "<>" {
		return
	}

	var user db.User
	err := database.NewSelect().Model(&user).
		Where("id = ?", *patch.Submitter.UserID).
		Scan(ctx)
	if err != nil || !user.SendEmail {
		return
	}

	baseURL := strings.TrimRight(cfg.Http.BaseURL, "/")
	patchURL := fmt.Sprintf("%s/patch/%s/",
		baseURL, strings.TrimSuffix(strings.TrimPrefix(patch.Msgid, "<"), ">"))

	subject := fmt.Sprintf("[%s] Patch state changed: %s",
		patch.Project.Linkname, patch.Name)

	body := fmt.Sprintf(
		"Hello,\n\nThe following patch (submitted by you) has been updated in Patchwork:\n\n"+
			" * %s: %s\n"+
			"     - %s\n"+
			"     - for: %s\n"+
			"    was: %s\n"+
			"    now: %s\n\n"+
			"This email is a notification only - you do not need to respond.\n\n"+
			"Happy patchworking.\n\n"+
			"--\n\n"+
			"This is an automated mail sent by the Patchwork system at\n"+
			"%s.\n",
		patch.Project.Linkname, patch.Name,
		patchURL,
		patch.Project.Name,
		oldState, newState,
		baseURL,
	)

	headers := map[string]string{"Precedence": "bulk"}
	if err := SendEmail(&cfg.SMTP, rcpt, subject, body, headers); err != nil {
		log.Errorf("patch state notification: %v", err)
	}
}
