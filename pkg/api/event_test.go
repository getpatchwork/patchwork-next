// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"fmt"
	"testing"
)

func TestEventActor(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "actor", "actor@test")
	patchID := s.insertPatch(t, projID, "<evact@test>", "p")
	s.exec(t, `INSERT INTO event (project_id, category, date, patch_id, actor_id)
		VALUES (?, 'patch-state-changed', datetime('now'), ?, ?)`,
		projID, patchID, userID)

	items := getList(t, s, "/api/1.4/events/")
	if len(items) != 1 {
		t.Fatalf("got %d", len(items))
	}
	assertNested(t, items[0], "actor", "id")
}

func TestEventActorNull(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<evan@test>", "p")
	s.exec(t, `INSERT INTO event (project_id, category, date, patch_id)
		VALUES (?, 'patch-created', datetime('now'), ?)`, projID, patchID)

	items := getList(t, s, "/api/1.4/events/")
	if items[0]["actor"] != nil {
		t.Errorf("actor should be null, got %v", items[0]["actor"])
	}
}

func TestEventCreate405(t *testing.T) {
	s := newTestServer(t)
	resp := s.authRequest(t, "POST", "/api/1.4/events/", "", map[string]string{"category": "x"})
	if resp.StatusCode != 405 {
		t.Errorf("status = %d, want 405", resp.StatusCode)
	}
}

func TestEventPayload(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<evp@test>", "event patch")
	s.exec(t, `INSERT INTO event (project_id, category, date, patch_id)
		VALUES (?, 'patch-created', datetime('now'), ?)`, projID, patchID)

	items := getList(t, s, "/api/1.4/events/")
	if len(items) != 1 {
		t.Fatalf("got %d", len(items))
	}
	assertField(t, items[0], "payload")
	payload, ok := items[0]["payload"].(map[string]any)
	if !ok {
		t.Fatal("payload is not an object")
	}
	patch, ok := payload["patch"].(map[string]any)
	if !ok {
		t.Fatal("payload.patch is not an object")
	}
	if patch["name"] != "event patch" {
		t.Errorf("payload.patch.name = %v", patch["name"])
	}
}

func TestEventsFilterActor(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "efa", "efa@test")
	patchID := s.insertPatch(t, projID, "<efa@test>", "p")
	s.exec(t, `INSERT INTO event (project_id, category, date, patch_id, actor_id)
		VALUES (?, 'patch-state-changed', datetime('now'), ?, ?)`,
		projID, patchID, userID)

	items := getList(t, s, fmt.Sprintf("/api/1.4/events/?actor=%d", userID))
	if len(items) != 1 {
		t.Errorf("got %d, want 1", len(items))
	}
	items = getList(t, s, "/api/1.4/events/?actor=99999")
	if len(items) != 0 {
		t.Errorf("got %d, want 0", len(items))
	}
}

func TestEventsFilterCategory(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<efc@test>", "p")
	s.exec(t, `
		INSERT INTO event (project_id, category, date, patch_id)
		VALUES (?, 'patch-created', datetime('now'), ?)
	`, projID, patchID)
	s.exec(t, `
		INSERT INTO event (project_id, category, date, series_id)
		VALUES (?, 'series-created', datetime('now'), NULL)
	`, projID)

	items := getList(t, s, "/api/1.4/events/?category=patch-created")
	if len(items) != 1 {
		t.Errorf("got %d, want 1", len(items))
	}
	items = getList(t, s, "/api/1.4/events/?category=nonexistent")
	if len(items) != 0 {
		t.Errorf("got %d, want 0", len(items))
	}
}

func TestEventsFilterPatch(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	p1 := s.insertPatch(t, projID, "<efpa1@test>", "p1")
	s.insertPatch(t, projID, "<efpa2@test>", "p2")
	s.exec(t, `INSERT INTO event (project_id, category, date, patch_id)
		VALUES (?, 'patch-created', datetime('now'), ?)`, projID, p1)

	items := getList(t, s, fmt.Sprintf("/api/1.4/events/?patch=%d", p1))
	if len(items) != 1 {
		t.Errorf("got %d, want 1", len(items))
	}
}

func TestEventsFilterProject(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<efp@test>", "p")
	s.exec(t, `INSERT INTO event (project_id, category, date, patch_id)
		VALUES (?, 'patch-created', datetime('now'), ?)`, projID, patchID)

	items := getList(t, s, fmt.Sprintf("/api/1.4/events/?project=%d", projID))
	if len(items) != 1 {
		t.Errorf("got %d, want 1", len(items))
	}
	items = getList(t, s, "/api/1.4/events/?project=99999")
	if len(items) != 0 {
		t.Errorf("got %d, want 0", len(items))
	}
}

func TestEventsFilterSeries(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	personID := s.insertPerson(t, "efs@test", "EFS")
	seriesID := s.insertSeries(t, projID, personID, "s")
	s.exec(t, `INSERT INTO event (project_id, category, date, series_id)
		VALUES (?, 'series-created', datetime('now'), ?)`, projID, seriesID)

	items := getList(t, s, fmt.Sprintf("/api/1.4/events/?series=%d", seriesID))
	if len(items) != 1 {
		t.Errorf("got %d, want 1", len(items))
	}
}

func TestEventsListEmpty(t *testing.T) {
	s := newTestServer(t)
	items := getList(t, s, "/api/1.4/events/")
	if len(items) != 0 {
		t.Errorf("got %d, want 0", len(items))
	}
}

func TestEventsOrderAscending(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	s.exec(t, `INSERT INTO event (project_id, category, date)
		VALUES (?, 'series-created', datetime('now', '-1 hour'))`, projID)
	s.exec(t, `INSERT INTO event (project_id, category, date)
		VALUES (?, 'patch-created', datetime('now'))`, projID)

	items := getList(t, s, "/api/1.4/events/?order=date")
	if len(items) != 2 {
		t.Fatalf("got %d, want 2", len(items))
	}
	if items[0]["category"] != "series-created" {
		t.Error("ascending order should show oldest first")
	}
}

func TestEventsOrderByDate(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	s.exec(t, `INSERT INTO event (project_id, category, date)
		VALUES (?, 'series-created', datetime('now', '-1 hour'))`, projID)
	s.exec(t, `INSERT INTO event (project_id, category, date)
		VALUES (?, 'patch-created', datetime('now'))`, projID)

	items := getList(t, s, "/api/1.4/events/")
	if len(items) != 2 {
		t.Fatalf("got %d, want 2", len(items))
	}
	if items[0]["category"] != "patch-created" {
		t.Error("default order should be newest first")
	}
}

func TestEventsWithData(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<ev@test>", "event patch")
	s.exec(t, `
		INSERT INTO event (project_id, category, date, patch_id)
		VALUES (?, 'patch-created', datetime('now'), ?)
	`, projID, patchID)

	items := getList(t, s, "/api/1.4/events/")
	if len(items) != 1 {
		t.Fatalf("got %d, want 1", len(items))
	}
	ev := items[0]
	assertField(t, ev, "id")
	assertField(t, ev, "category")
	assertField(t, ev, "date")
	assertNested(t, ev, "project", "id")
	if ev["category"] != "patch-created" {
		t.Errorf("category = %v", ev["category"])
	}
}
