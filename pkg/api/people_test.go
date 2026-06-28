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

func TestPeopleList(t *testing.T) {
	s := newTestServer(t)
	s.insertPerson(t, "person@test.com", "Test Person")
	items := getList(t, s, "/api/1.4/people/")
	if len(items) != 1 {
		t.Fatalf("got %d, want 1", len(items))
	}
	p := items[0]
	assertField(t, p, "id")
	assertField(t, p, "name")
	assertField(t, p, "email")
	if p["email"] != "person@test.com" {
		t.Errorf("email = %v", p["email"])
	}
}

func TestPeopleListEmpty(t *testing.T) {
	s := newTestServer(t)
	items := getList(t, s, "/api/1.4/people/")
	if len(items) != 0 {
		t.Errorf("got %d, want 0", len(items))
	}
}

func TestPeopleSearch(t *testing.T) {
	s := newTestServer(t)
	s.insertPerson(t, "alice@test", "Alice")
	s.insertPerson(t, "bob@test", "Bob")

	items := getList(t, s, "/api/1.4/people/?q=Alice")
	if len(items) != 1 {
		t.Errorf("got %d, want 1", len(items))
	}
}

func TestPersonCreate405(t *testing.T) {
	s := newTestServer(t)
	resp := s.authRequest(t, "POST", "/api/1.4/people/", "", map[string]string{"name": "x"})
	if resp.StatusCode != 405 {
		t.Errorf("status = %d, want 405", resp.StatusCode)
	}
}

func TestPersonDetail(t *testing.T) {
	s := newTestServer(t)
	id := s.insertPerson(t, "detail@test", "Detail Person")
	p := getOne(t, s, fmt.Sprintf("/api/1.4/people/%d", id))
	if p["name"] != "Detail Person" {
		t.Errorf("name = %v", p["name"])
	}
}

func TestPersonDetailAnonymous(t *testing.T) {
	s := newTestServer(t)
	s.insertPerson(t, "anon@test", "Anon")
	resp := s.get(t, "/api/1.4/people/")
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestPersonDetailInvalid(t *testing.T) {
	s := newTestServer(t)
	resp := s.get(t, "/api/1.4/people/invalid")
	if resp.StatusCode != 422 {
		t.Errorf("status = %d, want 422", resp.StatusCode)
	}
}

func TestPersonDetailLinked(t *testing.T) {
	s := newTestServer(t)
	userID := s.insertUser(t, "linked", "linked@test")
	s.exec(t, `INSERT INTO person (email, name, user_id)
		VALUES ('linked@test', 'Linked Person', ?)`, userID)
	var personID int
	s.db.NewRaw(`SELECT id FROM person WHERE email = 'linked@test'`).
		Scan(context.Background(), &personID)

	p := getOne(t, s, fmt.Sprintf("/api/1.4/people/%d", personID))
	if p["email"] != "linked@test" {
		t.Errorf("email = %v", p["email"])
	}
}

func TestPersonNotFound(t *testing.T) {
	s := newTestServer(t)
	resp := s.get(t, "/api/1.4/people/99999")
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestPersonUserLinked(t *testing.T) {
	s := newTestServer(t)
	userID := s.insertUser(t, "linked2", "linked2@test")
	s.exec(t, `INSERT INTO person (email, name, user_id)
		VALUES ('linked2@test', 'Linked', ?)`, userID)
	var personID int
	s.db.NewRaw(`SELECT id FROM person WHERE email = 'linked2@test'`).
		Scan(context.Background(), &personID)

	p := getOne(t, s, fmt.Sprintf("/api/1.4/people/%d", personID))
	assertNested(t, p, "user", "id")
	assertNested(t, p, "user", "username")
}
