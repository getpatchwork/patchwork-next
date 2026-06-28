// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"fmt"
	"testing"
)

func TestCheckFilterUser(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<cfu@test>", "p")
	userID := s.insertUser(t, "cifu", "cifu@test")
	s.exec(t, `INSERT INTO ci_check
		(patch_id, user_id, date, state, target_url, context, description)
		VALUES (?, ?, datetime('now'), 1, '', 'ci', '')`, patchID, userID)

	items := getList(t, s, fmt.Sprintf("/api/1.4/patches/%d/checks/", patchID))
	if len(items) != 1 {
		t.Fatalf("got %d, want 1", len(items))
	}
}

func TestCheckUserPopulated(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<cup@test>", "p")
	userID := s.insertUser(t, "checkuser", "cu@test")
	s.exec(t, `INSERT INTO ci_check
		(patch_id, user_id, date, state, target_url, context, description)
		VALUES (?, ?, datetime('now'), 0, '', 'ci', '')`, patchID, userID)

	items := getList(t, s, fmt.Sprintf("/api/1.4/patches/%d/checks/", patchID))
	assertNested(t, items[0], "user", "id")
	assertNested(t, items[0], "user", "username")
	assertField(t, items[0], "url")
}

func TestUserCreate405(t *testing.T) {
	s := newTestServer(t)
	resp := s.authRequest(t, "POST", "/api/1.4/users/", "", map[string]string{"username": "x"})
	if resp.StatusCode != 405 {
		t.Errorf("status = %d, want 405", resp.StatusCode)
	}
}

func TestUserDeleteCreate405(t *testing.T) {
	s := newTestServer(t)
	userID := s.insertUser(t, "ud", "ud@test")
	token := s.insertToken(t, userID)

	resp := s.authRequest(t, "DELETE",
		fmt.Sprintf("/api/1.4/users/%d", userID), token, nil)
	if resp.StatusCode != 405 {
		t.Errorf("DELETE status = %d, want 405", resp.StatusCode)
	}
}

func TestUserDetail(t *testing.T) {
	s := newTestServer(t)
	userID := s.insertUser(t, "detailer", "detail@test")
	token := s.insertToken(t, userID)

	resp := s.authRequest(t, "GET",
		fmt.Sprintf("/api/1.4/users/%d", userID), token, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var user map[string]any
	decodeJSON(t, resp, &user)
	if user["username"] != "detailer" {
		t.Errorf("username = %v", user["username"])
	}
}

func TestUserDetailAuthenticated(t *testing.T) {
	s := newTestServer(t)
	u1 := s.insertUser(t, "viewer", "viewer@test")
	u2 := s.insertUser(t, "target", "target@test")
	token := s.insertToken(t, u1)

	resp := s.authRequest(t, "GET",
		fmt.Sprintf("/api/1.4/users/%d", u2), token, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var user map[string]any
	decodeJSON(t, resp, &user)
	if user["username"] != "target" {
		t.Errorf("username = %v", user["username"])
	}
}

func TestUserDetailSelf(t *testing.T) {
	s := newTestServer(t)
	userID := s.insertUser(t, "self", "self@test")
	token := s.insertToken(t, userID)

	resp := s.authRequest(t, "GET",
		fmt.Sprintf("/api/1.4/users/%d", userID), token, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var user map[string]any
	decodeJSON(t, resp, &user)
	if user["username"] != "self" {
		t.Errorf("username = %v", user["username"])
	}
}

func TestUserListAnonymous(t *testing.T) {
	s := newTestServer(t)
	resp := s.authRequest(t, "GET", "/api/1.4/users/", "", nil)
	if resp.StatusCode != 401 {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestUserListAuthenticated(t *testing.T) {
	s := newTestServer(t)
	userID := s.insertUser(t, "lister", "lister@test")
	token := s.insertToken(t, userID)

	resp := s.authRequest(t, "GET", "/api/1.4/users/", token, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var users []map[string]any
	decodeJSON(t, resp, &users)
	if len(users) == 0 {
		t.Error("expected at least 1 user")
	}
	assertField(t, users[0], "id")
	assertField(t, users[0], "username")
	assertField(t, users[0], "url")
}

func TestUserNotFound(t *testing.T) {
	s := newTestServer(t)
	userID := s.insertUser(t, "nf", "nf@test")
	token := s.insertToken(t, userID)

	resp := s.authRequest(t, "GET", "/api/1.4/users/99999", token, nil)
	if resp.StatusCode != 404 {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestUserUpdateOther(t *testing.T) {
	s := newTestServer(t)
	u1 := s.insertUser(t, "u1", "u1@test")
	u2 := s.insertUser(t, "u2", "u2@test")
	token := s.insertToken(t, u1)

	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/users/%d", u2), token,
		map[string]string{"first_name": "Hacked"})
	if resp.StatusCode != 403 {
		t.Errorf("status = %d, want 403", resp.StatusCode)
	}
}

func TestUserUpdateSelf(t *testing.T) {
	s := newTestServer(t)
	userID := s.insertUser(t, "updater", "up@test")
	token := s.insertToken(t, userID)

	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/users/%d", userID), token,
		map[string]string{"first_name": "Updated"})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var user map[string]any
	decodeJSON(t, resp, &user)
	if user["first_name"] != "Updated" {
		t.Errorf("first_name = %v", user["first_name"])
	}
}
