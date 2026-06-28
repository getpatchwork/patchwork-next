// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"fmt"
	"testing"
)

func TestWebhookCRUD(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "whmaint", "wh@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	// create
	resp := s.authRequest(t, "POST",
		fmt.Sprintf("/api/1.4/projects/%d/webhooks/", projID), token,
		map[string]string{
			"url":    "http://hook.example.com",
			"secret": "s3cret",
			"events": "*",
		})
	if resp.StatusCode != 201 {
		t.Fatalf("create: status = %d", resp.StatusCode)
	}
	var hook map[string]any
	decodeJSON(t, resp, &hook)
	if hook["url"] != "http://hook.example.com" {
		t.Errorf("url = %v", hook["url"])
	}
	if _, hasSecret := hook["secret"]; hasSecret {
		t.Error("secret should not be in response")
	}
	hookID := int(hook["id"].(float64))

	// list
	resp = s.authRequest(t, "GET",
		fmt.Sprintf("/api/1.4/projects/%d/webhooks/", projID), token, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("list: status = %d", resp.StatusCode)
	}
	var hooks []map[string]any
	decodeJSON(t, resp, &hooks)
	if len(hooks) != 1 {
		t.Errorf("list: got %d, want 1", len(hooks))
	}

	// detail
	resp = s.authRequest(t, "GET",
		fmt.Sprintf("/api/1.4/projects/%d/webhooks/%d", projID, hookID), token, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("detail: status = %d", resp.StatusCode)
	}

	// update
	resp = s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/projects/%d/webhooks/%d", projID, hookID), token,
		map[string]any{"active": false})
	if resp.StatusCode != 200 {
		t.Fatalf("update: status = %d", resp.StatusCode)
	}
	decodeJSON(t, resp, &hook)
	if hook["active"] != false {
		t.Errorf("active = %v, want false", hook["active"])
	}

	// delete
	resp = s.authRequest(t, "DELETE",
		fmt.Sprintf("/api/1.4/projects/%d/webhooks/%d", projID, hookID), token, nil)
	if resp.StatusCode != 204 {
		t.Fatalf("delete: status = %d", resp.StatusCode)
	}

	// list should be empty
	resp = s.authRequest(t, "GET",
		fmt.Sprintf("/api/1.4/projects/%d/webhooks/", projID), token, nil)
	decodeJSON(t, resp, &hooks)
	if len(hooks) != 0 {
		t.Errorf("after delete: got %d, want 0", len(hooks))
	}
}

func TestWebhookCreateInvalidEvents(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "whie", "whie@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	resp := s.authRequest(t, "POST",
		fmt.Sprintf("/api/1.4/projects/%d/webhooks/", projID), token,
		map[string]string{"url": "http://x", "events": "invalid-event"})
	if resp.StatusCode != 422 {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestWebhookCreateSpecificEvents(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "whse", "whse@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	resp := s.authRequest(t, "POST",
		fmt.Sprintf("/api/1.4/projects/%d/webhooks/", projID), token,
		map[string]string{
			"url":    "http://x",
			"secret": "",
			"events": "patch-created,series-completed",
		})
	if resp.StatusCode != 201 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var hook map[string]any
	decodeJSON(t, resp, &hook)
	if hook["events"] != "patch-created,series-completed" {
		t.Errorf("events = %v", hook["events"])
	}
}

func TestWebhookListAnonymous(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)

	resp := s.authRequest(t, "GET",
		fmt.Sprintf("/api/1.4/projects/%d/webhooks/", projID), "", nil)
	if resp.StatusCode != 401 {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestWebhookListNonMaintainer(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "whnm", "whnm@test")
	token := s.insertToken(t, userID)

	resp := s.authRequest(t, "GET",
		fmt.Sprintf("/api/1.4/projects/%d/webhooks/", projID), token, nil)
	if resp.StatusCode != 403 {
		t.Errorf("status = %d, want 403", resp.StatusCode)
	}
}

func TestWebhookSecretWriteOnly(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	userID := s.insertUser(t, "whsec", "whsec@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	resp := s.authRequest(t, "POST",
		fmt.Sprintf("/api/1.4/projects/%d/webhooks/", projID), token,
		map[string]string{"url": "http://x", "secret": "topsecret", "events": "*"})
	if resp.StatusCode != 201 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var hook map[string]any
	decodeJSON(t, resp, &hook)
	if _, has := hook["secret"]; has {
		t.Error("secret should not appear in response")
	}

	hookID := int(hook["id"].(float64))
	resp = s.authRequest(t, "GET",
		fmt.Sprintf("/api/1.4/projects/%d/webhooks/%d", projID, hookID), token, nil)
	decodeJSON(t, resp, &hook)
	if _, has := hook["secret"]; has {
		t.Error("secret should not appear in GET response")
	}
}
