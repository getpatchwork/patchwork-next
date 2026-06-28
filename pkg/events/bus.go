// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package events

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/db"
	"github.com/getpatchwork/patchwork/pkg/log"
)

type webhookItem struct {
	event *db.Event
	hooks []db.Webhook
}

// Bus processes events asynchronously via a background goroutine.
type Bus struct {
	ctx      context.Context
	q        *db.Queries
	webhooks chan webhookItem
	mu       sync.Mutex
	wg       sync.WaitGroup
}

// Start creates and starts an event bus with a buffered channel.
func Start(ctx context.Context, database *bun.DB) *Bus {
	b := &Bus{
		ctx:      ctx,
		q:        db.New(ctx, database),
		webhooks: make(chan webhookItem, 512),
	}
	for n := range 4 {
		b.wg.Add(1)
		go b.worker(n + 1)
	}
	return b
}

// Shutdown closes the channel and waits for the worker to drain all
// pending events.
func (b *Bus) Shutdown() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.webhooks != nil {
		close(b.webhooks)
		b.wg.Wait()
		b.webhooks = nil
	}
}

// Enqueue sends an event to the background worker.
func (b *Bus) Enqueue(e *db.Event) {
	if err := b.q.Insert(e); err != nil {
		log.Errorf("event %s: create: %v", e.Category, err)
		return
	}
	log.Debugf("event created: %s (id=%d)", e.Category, e.ID)

	hooks, err := b.q.GetActiveWebhooks(e.ProjectID)
	if err == nil && len(hooks) > 0 {
		b.webhooks <- webhookItem{event: e, hooks: hooks}
	}
}

func (b *Bus) worker(id int) {
	defer b.wg.Done()

	log.Debugf("starting webhook worker#%d", id)

	for item := range b.webhooks {
		e := item.event

		var project db.Project
		if err := b.q.DB.NewSelect().Model(&project).
			Where("id = ?", e.ProjectID).
			Scan(b.ctx); err != nil {
			log.Warnf("webhook: load project %d: %v", e.ProjectID, err)
			return
		}

		payload, err := serializeEvent(b.q, e, &project)
		if err != nil {
			log.Warnf("webhook: serialize %s: %v", e.Category, err)
			return
		}

		for i := range item.hooks {
			w := &item.hooks[i]
			if !w.MatchesEvent(e.Category) {
				continue
			}
			postWebhook(b.ctx, w, e.Category, fmt.Sprintf("%d", e.ID), payload)
		}
	}

	log.Debugf("shutting down webhook worker#%d", id)
}

const webhookTimeout = 10 * time.Second

func postWebhook(
	ctx context.Context,
	w *db.Webhook,
	category, deliveryID string,
	payload []byte,
) {
	ctx, cancel := context.WithTimeout(ctx, webhookTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, w.URL, bytes.NewReader(payload),
	)
	if err != nil {
		log.Warnf("webhook %s: %v", w.URL, err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Patchwork-Event", category)
	req.Header.Set("X-Patchwork-Delivery", deliveryID)

	if w.Secret != "" {
		mac := hmac.New(sha256.New, []byte(w.Secret))
		mac.Write(payload)
		sig := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Patchwork-Signature", "sha256="+sig)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Warnf("webhook %s: %v", w.URL, err)
		return
	}
	resp.Body.Close()
}

func serializeEvent(q *db.Queries, e *db.Event, project *db.Project) ([]byte, error) {
	payload, err := buildPayload(q, e)
	if err != nil {
		return nil, err
	}
	e.Project = project
	e.Payload = payload
	return json.Marshal(e)
}

func buildPayload(q *db.Queries, e *db.Event) (map[string]any, error) {
	m := map[string]any{}

	switch e.Category {
	case "cover-created":
		c, err := q.GetCoverByID(*e.CoverID)
		if err != nil {
			return nil, fmt.Errorf("cover %d: %w", *e.CoverID, err)
		}
		m["cover"] = c

	case "patch-created", "patch-completed":
		p, err := q.GetPatchByID(*e.PatchID)
		if err != nil {
			return nil, fmt.Errorf("patch %d: %w", *e.PatchID, err)
		}
		m["patch"] = p
		if e.SeriesID != nil {
			s, err := q.GetSeriesByID(*e.SeriesID)
			if err == nil {
				m["series"] = s
			}
		}

	case "series-created", "series-completed":
		s, err := q.GetSeriesByID(*e.SeriesID)
		if err != nil {
			return nil, fmt.Errorf("series %d: %w", *e.SeriesID, err)
		}
		m["series"] = s

	case "patch-comment-created":
		if e.PatchCommentID != nil {
			if pc, err := q.GetPatchCommentByID(*e.PatchCommentID); err == nil {
				m["comment"] = pc
			}
		}
		if e.PatchID != nil {
			if p, err := q.GetPatchByID(*e.PatchID); err == nil {
				m["patch"] = p
			}
		}

	case "cover-comment-created":
		if e.CoverCommentID != nil {
			if cc, err := q.GetCoverCommentByID(*e.CoverCommentID); err == nil {
				m["comment"] = cc
			}
		}
		if e.CoverID != nil {
			if c, err := q.GetCoverByID(*e.CoverID); err == nil {
				m["cover"] = c
			}
		}

	case "check-created", "check-updated":
		if e.PatchID != nil {
			if p, err := q.GetPatchByID(*e.PatchID); err == nil {
				m["patch"] = p
			}
		}

	case "patch-state-changed", "patch-delegated":
		if e.PatchID != nil {
			if p, err := q.GetPatchByID(*e.PatchID); err == nil {
				m["patch"] = p
			}
		}
	}

	return m, nil
}
