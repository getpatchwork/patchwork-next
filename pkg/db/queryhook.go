// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import (
	"context"
	"time"

	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/log"
)

type slowQueryHook struct {
	threshold time.Duration
}

type startTimeKey struct{}

func NewSlowQueryHook(threshold time.Duration) bun.QueryHook {
	return &slowQueryHook{threshold: threshold}
}

func (h *slowQueryHook) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	return context.WithValue(ctx, startTimeKey{}, time.Now())
}

func (h *slowQueryHook) AfterQuery(ctx context.Context, event *bun.QueryEvent) {
	dur := time.Since(event.StartTime)
	if dur >= h.threshold {
		log.Warnf("slow query (%s): %s", dur.Round(time.Millisecond), event.Query)
	}
}
