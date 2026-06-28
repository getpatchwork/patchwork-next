// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"context"
	"fmt"
	"testing"
)

func TestCheckCreateInvalidPatch(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "cip", "cip@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	resp := s.authRequest(t, "POST",
		"/api/1.4/patches/99999/checks/", token,
		map[string]string{"state": "success", "context": "ci"})
	if resp.StatusCode != 404 {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestCheckCreateMissingState(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<cms@test>", "p")
	userID := s.insertUser(t, "cms", "cms@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	resp := s.authRequest(t, "POST",
		fmt.Sprintf("/api/1.4/patches/%d/checks/", patchID), token,
		map[string]string{"context": "ci"})
	if resp.StatusCode != 422 {
		t.Errorf("status = %d, want 422", resp.StatusCode)
	}
}

func TestCheckCreateNonMaintainer(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<cnm@test>", "p")
	userID := s.insertUser(t, "cnm", "cnm@test")
	token := s.insertToken(t, userID)

	resp := s.authRequest(t, "POST",
		fmt.Sprintf("/api/1.4/patches/%d/checks/", patchID), token,
		map[string]string{"state": "success", "context": "ci"})
	if resp.StatusCode != 403 {
		t.Errorf("status = %d, want 403", resp.StatusCode)
	}
}

func TestCheckDetail(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<cdet@test>", "p")
	userID := s.insertUser(t, "ci2", "ci2@test")
	var checkID int
	s.db.NewRaw(`INSERT INTO ci_check
		(patch_id, user_id, date, state, target_url, context, description)
		VALUES (?, ?, datetime('now'), 1, 'http://ci/2', 'ci', 'ok')
		RETURNING id`, patchID, userID).Scan(context.Background(), &checkID)

	c := getOne(t, s, fmt.Sprintf("/api/1.4/patches/%d/checks/%d", patchID, checkID))
	assertField(t, c, "id")
	assertField(t, c, "context")
}

func TestCheckInvalidPatch(t *testing.T) {
	s := newTestServer(t)
	resp := s.get(t, "/api/1.4/patches/99999/checks/")
	var items []map[string]any
	decodeJSON(t, resp, &items)
	if len(items) != 0 {
		t.Errorf("got %d, want 0", len(items))
	}
}

func TestCheckList(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<cl@test>", "p")
	userID := s.insertUser(t, "cibot", "ci@test")
	s.exec(t, `INSERT INTO ci_check
		(patch_id, user_id, date, state, target_url, context, description)
		VALUES (?, ?, datetime('now'), 1, 'http://ci/1', 'ci/build', 'Build passed')`,
		patchID, userID)

	items := getList(t, s, fmt.Sprintf("/api/1.4/patches/%d/checks/", patchID))
	if len(items) != 1 {
		t.Fatalf("got %d, want 1", len(items))
	}
	c := items[0]
	assertValue(t, c, "state", "success")
	assertValue(t, c, "target_url", "http://ci/1")
	assertValue(t, c, "context", "ci/build")
	assertValue(t, c, "description", "Build passed")
	assertField(t, c, "id")
	assertField(t, c, "date")
	assertField(t, c, "url")
	assertNested(t, c, "user", "id")
}

func TestCheckListEmpty(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<chk@test>", "p")

	items := getList(t, s, fmt.Sprintf("/api/1.4/patches/%d/checks/", patchID))
	if len(items) != 0 {
		t.Errorf("got %d, want 0", len(items))
	}
}

func TestCheckStateString(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<css@test>", "p")
	userID := s.insertUser(t, "css", "css@test")
	s.exec(t, `INSERT INTO ci_check
		(patch_id, user_id, date, state, target_url, context, description)
		VALUES (?, ?, datetime('now'), 1, '', 'ci', '')`, patchID, userID)

	items := getList(t, s, fmt.Sprintf("/api/1.4/patches/%d/checks/", patchID))
	if len(items) != 1 {
		t.Fatalf("got %d, want 1", len(items))
	}
	if items[0]["state"] != "success" {
		t.Errorf("state = %v, want 'success'", items[0]["state"])
	}
}

func TestCheckDelete405(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<cud@test>", "p")

	resp := s.authRequest(t, "DELETE",
		fmt.Sprintf("/api/1.4/patches/%d/checks/1", patchID), "", nil)
	if resp.StatusCode != 405 {
		t.Errorf("DELETE status = %d, want 405", resp.StatusCode)
	}
}

func TestCreateCheckAnonymous(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<chk-anon@test>", "p")

	resp := s.authRequest(t, "POST",
		fmt.Sprintf("/api/1.4/patches/%d/checks/", patchID), "",
		map[string]string{"state": "success", "context": "ci"})
	if resp.StatusCode != 401 {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestCreateCheckInvalidState(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<chk-bad@test>", "p")
	userID := s.insertUser(t, "ci2", "ci2@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	resp := s.authRequest(t, "POST",
		fmt.Sprintf("/api/1.4/patches/%d/checks/", patchID), token,
		map[string]string{"state": "invalid"})
	if resp.StatusCode != 422 {
		t.Errorf("status = %d, want 422", resp.StatusCode)
	}
}

func TestCreateCheckMaintainer(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<chk-ok@test>", "p")
	userID := s.insertUser(t, "ci", "ci@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	resp := s.authRequest(t, "POST",
		fmt.Sprintf("/api/1.4/patches/%d/checks/", patchID), token,
		map[string]string{
			"state":       "success",
			"target_url":  "http://ci.example.com/1",
			"context":     "ci/build",
			"description": "Build passed",
		})
	if resp.StatusCode != 201 {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}

	var result map[string]any
	decodeJSON(t, resp, &result)
	assertField(t, result, "id")
	assertField(t, result, "date")
	if result["context"] != "ci/build" {
		t.Errorf("context = %v", result["context"])
	}
}

func TestPatchCombinedCheck(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<combo@test>", "p")
	userID := s.insertUser(t, "cc", "cc@test")
	s.exec(t, `INSERT INTO ci_check
		(patch_id, user_id, date, state, target_url, context, description)
		VALUES (?, ?, datetime('now'), 1, '', 'ci/build', '')`, patchID, userID)
	s.exec(t, `INSERT INTO ci_check
		(patch_id, user_id, date, state, target_url, context, description)
		VALUES (?, ?, datetime('now'), 2, '', 'ci/lint', '')`, patchID, userID)

	p := getOne(t, s, fmt.Sprintf("/api/1.4/patches/%d", patchID))
	if p["check"] != "warning" {
		t.Errorf("check = %v, want warning", p["check"])
	}
}
