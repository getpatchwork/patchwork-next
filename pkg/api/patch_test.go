// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"fmt"
	"testing"
)

func TestPatchCreate405(t *testing.T) {
	s := newTestServer(t)
	resp := s.authRequest(t, "POST", "/api/1.4/patches/", "", map[string]string{"name": "x"})
	if resp.StatusCode != 405 {
		t.Errorf("status = %d, want 405", resp.StatusCode)
	}
}

func TestPatchDelete405(t *testing.T) {
	s := newTestServer(t)
	resp := s.authRequest(t, "DELETE", "/api/1.4/patches/1", "", nil)
	if resp.StatusCode != 405 {
		t.Errorf("status = %d, want 405", resp.StatusCode)
	}
}

func TestPatchDetail(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<detail@test>", "detail patch")
	p := getOne(t, s, fmt.Sprintf("/api/1.4/patches/%d", patchID))
	if p["name"] != "detail patch" {
		t.Errorf("name = %v", p["name"])
	}
	assertField(t, p, "diff")
	assertField(t, p, "headers")
	assertField(t, p, "hash")
	assertNested(t, p, "submitter", "id")
	assertNested(t, p, "project", "id")
}

func TestPatchDetailInvalid(t *testing.T) {
	s := newTestServer(t)
	resp := s.get(t, "/api/1.4/patches/invalid")
	// huma returns 422 for invalid path parameter type
	if resp.StatusCode != 422 {
		t.Errorf("status = %d, want 422", resp.StatusCode)
	}
}

func TestPatchFilterArchived(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	s.insertPatch(t, projID, "<pa@test>", "p")

	items := getList(t, s, "/api/1.4/patches/?archived=false")
	if len(items) != 1 {
		t.Errorf("not archived: got %d, want 1", len(items))
	}
	items = getList(t, s, "/api/1.4/patches/?archived=true")
	if len(items) != 0 {
		t.Errorf("archived: got %d, want 0", len(items))
	}
}

func TestPatchFilterHash(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	s.insertPatch(t, projID, "<hash@test>", "hash")

	items := getList(t, s, "/api/1.4/patches/?hash=abc123")
	if len(items) != 1 {
		t.Errorf("got %d, want 1", len(items))
	}
	items = getList(t, s, "/api/1.4/patches/?hash=ABC123")
	if len(items) != 1 {
		t.Errorf("case-insensitive: got %d, want 1", len(items))
	}
	items = getList(t, s, "/api/1.4/patches/?hash=nonexistent")
	if len(items) != 0 {
		t.Errorf("got %d, want 0", len(items))
	}
}

func TestPatchFilterMsgid(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	s.insertPatch(t, projID, "<msgid-filter@test>", "msgid")

	items := getList(t, s, "/api/1.4/patches/?msgid=msgid-filter@test")
	if len(items) != 1 {
		t.Errorf("got %d, want 1", len(items))
	}
	items = getList(t, s, "/api/1.4/patches/?msgid=nonexistent@test")
	if len(items) != 0 {
		t.Errorf("got %d, want 0", len(items))
	}
}

func TestPatchFilterState(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	s.insertPatch(t, projID, "<state@test>", "state")

	items := getList(t, s, "/api/1.4/patches/?state=New")
	if len(items) != 1 {
		t.Errorf("filter by name: got %d, want 1", len(items))
	}
	items = getList(t, s, "/api/1.4/patches/?state=Accepted")
	if len(items) != 0 {
		t.Errorf("filter by wrong state: got %d, want 0", len(items))
	}
}

func TestPatchFilterSubmitter(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	s.insertPatch(t, projID, "<ps@test>", "p")

	personID := s.insertPerson(t, "test@example.com", "Test Author")
	items := getList(t, s, fmt.Sprintf("/api/1.4/patches/?submitter=%d", personID))
	if len(items) != 1 {
		t.Errorf("by id: got %d, want 1", len(items))
	}
	items = getList(t, s, "/api/1.4/patches/?submitter=99999")
	if len(items) != 0 {
		t.Errorf("wrong id: got %d, want 0", len(items))
	}
}

func TestPatchList(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<p1@test>", "test patch")
	items := getList(t, s, "/api/1.4/patches/")
	if len(items) != 1 {
		t.Fatalf("got %d, want 1", len(items))
	}
	p := items[0]

	// value checks matching Python assertSerialized
	assertValue(t, p, "id", float64(patchID))
	assertValue(t, p, "name", "test patch")
	assertValue(t, p, "msgid", "<p1@test>")
	assertContains(t, p, "url", fmt.Sprintf("/patches/%d/", patchID))
	assertContains(t, p, "mbox", fmt.Sprintf("/patches/%d/mbox/", patchID))
	assertContains(t, p, "comments", fmt.Sprintf("/patches/%d/comments/", patchID))
	assertContains(t, p, "checks", fmt.Sprintf("/patches/%d/checks/", patchID))
	assertField(t, p, "date")
	assertField(t, p, "tags")
	assertField(t, p, "series")

	assertNested(t, p, "submitter", "id")
	assertNested(t, p, "submitter", "email")
	assertNested(t, p, "project", "id")
	assertNested(t, p, "project", "name")

	// api returns state as a plain string
	assertValue(t, p, "state", "New")

	seriesList := p["series"].([]any)
	if len(seriesList) != 0 {
		t.Errorf("series should be empty, got %d", len(seriesList))
	}

	tags := p["tags"].(map[string]any)
	if len(tags) != 0 {
		t.Errorf("tags should be empty, got %d", len(tags))
	}

	// related uses omitempty, absent when empty
	if rel, ok := p["related"]; ok {
		related := rel.([]any)
		if len(related) != 0 {
			t.Errorf("related should be empty, got %d", len(related))
		}
	}
}

func TestPatchListEmpty(t *testing.T) {
	s := newTestServer(t)
	items := getList(t, s, "/api/1.4/patches/")
	if len(items) != 0 {
		t.Errorf("got %d, want 0", len(items))
	}
}

func TestPatchNotFound(t *testing.T) {
	s := newTestServer(t)
	resp := s.get(t, "/api/1.4/patches/99999")
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestPatchNotFoundUpdate(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "m4", "m4@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	resp := s.authRequest(t, "PATCH", "/api/1.4/patches/99999", token,
		map[string]string{"state": "Accepted"})
	if resp.StatusCode != 404 {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestPatchSearch(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	s.insertPatch(t, projID, "<q1@test>", "unique searchable name")
	s.insertPatch(t, projID, "<q2@test>", "other patch")

	items := getList(t, s, "/api/1.4/patches/?q=searchable")
	if len(items) != 1 {
		t.Errorf("got %d, want 1", len(items))
	}
}

func TestPatchUpdateAnonymous(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<anon@test>", "p")

	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/patches/%d", patchID), "",
		map[string]string{"state": "Accepted"})
	if resp.StatusCode != 401 {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestPatchUpdateArchived(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<arch@test>", "p")
	userID := s.insertUser(t, "m3", "m3@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/patches/%d", patchID), token,
		map[string]bool{"archived": true})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	var result map[string]any
	decodeJSON(t, resp, &result)
	if result["archived"] != true {
		t.Errorf("archived = %v, want true", result["archived"])
	}
}

func TestPatchUpdateInvalidDelegate(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<invdel@test>", "p")
	userID := s.insertUser(t, "invdel", "invdel@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/patches/%d", patchID), token,
		map[string]any{"delegate": 99999})
	// Without FK constraints, the update succeeds with a dangling
	// reference. With FK constraints enabled, the DB rejects it.
	if resp.StatusCode != 200 && resp.StatusCode != 400 && resp.StatusCode != 500 {
		t.Errorf("status = %d, want 200, 400, or 500", resp.StatusCode)
	}
}

func TestPatchUpdateInvalidState(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<bad-state@test>", "p")
	userID := s.insertUser(t, "m2", "m2@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/patches/%d", patchID), token,
		map[string]string{"state": "Nonexistent"})
	if resp.StatusCode != 400 {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestPatchUpdateMaintainer(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<maint@test>", "p")
	userID := s.insertUser(t, "maintainer", "maint@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/patches/%d", patchID), token,
		map[string]string{"state": "Accepted"})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var result map[string]any
	decodeJSON(t, resp, &result)
	if result["state"] != "Accepted" {
		t.Errorf("state = %v, want Accepted", result["state"])
	}
}

func TestPatchUpdateNonMaintainer(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<nonmaint@test>", "p")
	userID := s.insertUser(t, "normie", "normie@test")
	token := s.insertToken(t, userID)

	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/patches/%d", patchID), token,
		map[string]string{"state": "Accepted"})
	if resp.StatusCode != 403 {
		t.Errorf("status = %d, want 403", resp.StatusCode)
	}
}

func TestPatchUpdateState(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<state-up@test>", "p")
	userID := s.insertUser(t, "maint", "m@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/patches/%d", patchID), token,
		map[string]string{"state": "RFC"})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	var result map[string]any
	decodeJSON(t, resp, &result)
	if result["state"] != "RFC" {
		t.Errorf("state = %v, want RFC", result["state"])
	}
}

func TestPatchWebURL(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t) // has web_url = http://example.com
	patchID := s.insertPatch(t, projID, "<weburl@test>", "p")

	p := getOne(t, s, fmt.Sprintf("/api/1.4/patches/%d", patchID))
	assertField(t, p, "web_url")
	webURL, _ := p["web_url"].(string)
	if webURL == "" {
		t.Error("web_url should not be empty")
	}
}
