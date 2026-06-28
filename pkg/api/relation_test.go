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

func TestRelationCreateAnonymous(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	p1 := s.insertPatch(t, projID, "<rca1@test>", "p1")
	p2 := s.insertPatch(t, projID, "<rca2@test>", "p2")

	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/patches/%d", p1), "",
		map[string]any{"related": []int{p2}})
	if resp.StatusCode != 401 {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestRelationCreateThreePatch(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	p1 := s.insertPatch(t, projID, "<r3a@test>", "p1")
	p2 := s.insertPatch(t, projID, "<r3b@test>", "p2")
	p3 := s.insertPatch(t, projID, "<r3c@test>", "p3")
	userID := s.insertUser(t, "r3m", "r3m@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/patches/%d", p1), token,
		map[string]any{"related": []int{p2, p3}})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	// all three should share same relation
	for _, pid := range []int{p1, p2, p3} {
		patch := getOne(t, s, fmt.Sprintf("/api/1.4/patches/%d", pid))
		related, ok := patch["related"].([]any)
		if !ok {
			t.Errorf("patch %d: related field missing or not an array", pid)
			continue
		}
		if len(related) != 2 {
			t.Errorf("patch %d related: got %d, want 2", pid, len(related))
		}
	}
}

func TestRelationCreateUser(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	p1 := s.insertPatch(t, projID, "<rcu1@test>", "p1")
	p2 := s.insertPatch(t, projID, "<rcu2@test>", "p2")
	userID := s.insertUser(t, "reluser", "ru@test")
	token := s.insertToken(t, userID)

	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/patches/%d", p1), token,
		map[string]any{"related": []int{p2}})
	if resp.StatusCode != 403 {
		t.Errorf("status = %d, want 403", resp.StatusCode)
	}
}

func TestRelationCrossProjectForbidden(t *testing.T) {
	s := newTestServer(t)
	projA := s.insertProject(t)
	s.exec(t, `INSERT INTO project (
		linkname, name, listid, listemail, subject_match,
		web_url, scm_url, webscm_url,
		list_archive_url, list_archive_url_format, commit_url_format,
		send_notifications, use_tags, show_dependencies, auto_supersede
	) VALUES ('proj-b', 'Project B', 'b.example.com',
		'b@example.com', '', '', '', '', '', '', '',
		false, true, false, false)`)
	var projB int
	s.db.NewRaw(`SELECT id FROM project WHERE linkname = 'proj-b'`).
		Scan(context.Background(), &projB)

	pA := s.insertPatch(t, projA, "<cpa@test>", "a")
	personB := s.insertPerson(t, "cpb@test", "B")
	var stateID int
	s.db.NewRaw(`SELECT id FROM state WHERE ordering = 0`).
		Scan(context.Background(), &stateID)
	var pB int
	s.db.NewRaw(`INSERT INTO patch (
		msgid, date, headers, submitter_id, project_id, name, archived, state_id
	) VALUES ('<cpb@test>', datetime('now'), '', ?, ?, 'b', false, ?)
		RETURNING id`, personB, projB, stateID).Scan(context.Background(), &pB)

	userID := s.insertUser(t, "cpm", "cpm@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projA)

	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/patches/%d", pA), token,
		map[string]any{"related": []int{pB}})
	if resp.StatusCode != 403 {
		t.Errorf("status = %d, want 403", resp.StatusCode)
	}
}

func TestRelationDeleteAnonymous(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	p1 := s.insertPatch(t, projID, "<rda1@test>", "p1")
	p2 := s.insertPatch(t, projID, "<rda2@test>", "p2")
	userID := s.insertUser(t, "rdam", "rdam@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/patches/%d", p1), token,
		map[string]any{"related": []int{p2}})

	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/patches/%d", p1), "",
		map[string]any{"related": []int{}})
	if resp.StatusCode != 401 {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestRelationDeleteFromThree(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	p1 := s.insertPatch(t, projID, "<rdf1@test>", "p1")
	p2 := s.insertPatch(t, projID, "<rdf2@test>", "p2")
	p3 := s.insertPatch(t, projID, "<rdf3@test>", "p3")
	userID := s.insertUser(t, "rdfm", "rdfm@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/patches/%d", p1), token,
		map[string]any{"related": []int{p2, p3}})

	// remove p1 from the group
	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/patches/%d", p1), token,
		map[string]any{"related": []int{}})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	// p1 has no relations, p2 and p3 still related to each other
	patch1 := getOne(t, s, fmt.Sprintf("/api/1.4/patches/%d", p1))
	if rel, ok := patch1["related"].([]any); ok && len(rel) != 0 {
		t.Error("p1 should have no relations")
	}
	patch2 := getOne(t, s, fmt.Sprintf("/api/1.4/patches/%d", p2))
	rel2, ok := patch2["related"].([]any)
	if !ok {
		t.Error("p2 related field missing")
	} else if len(rel2) != 1 {
		t.Error("p2 should have 1 relation (p3)")
	}
}

func TestRelationDeleteMaintainer(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	p1 := s.insertPatch(t, projID, "<rd1@test>", "p1")
	p2 := s.insertPatch(t, projID, "<rd2@test>", "p2")
	userID := s.insertUser(t, "rdm", "rdm@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	// create
	s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/patches/%d", p1), token,
		map[string]any{"related": []int{p2}})

	// delete
	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/patches/%d", p1), token,
		map[string]any{"related": []int{}})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	patch := getOne(t, s, fmt.Sprintf("/api/1.4/patches/%d", p1))
	if rel, ok := patch["related"].([]any); ok && len(rel) != 0 {
		t.Errorf("related after delete: got %d, want 0", len(rel))
	}
}

func TestRelationExtendThroughNew(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	p1 := s.insertPatch(t, projID, "<ren1@test>", "p1")
	p2 := s.insertPatch(t, projID, "<ren2@test>", "p2")
	p3 := s.insertPatch(t, projID, "<ren3@test>", "new")
	userID := s.insertUser(t, "renm", "renm@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	// create relation between p1 and p2
	s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/patches/%d", p1), token,
		map[string]any{"related": []int{p2}})

	// extend by adding p3 via p1
	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/patches/%d", p3), token,
		map[string]any{"related": []int{p1}})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	// all three in same group
	patch3 := getOne(t, s, fmt.Sprintf("/api/1.4/patches/%d", p3))
	if rel, ok := patch3["related"].([]any); !ok {
		t.Error("p3 related field missing")
	} else if len(rel) != 2 {
		t.Errorf("p3 related: got %d, want 2", len(rel))
	}
}

func TestRelationForbidMoveBetweenRelations(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	p1 := s.insertPatch(t, projID, "<rfm1@test>", "a1")
	p2 := s.insertPatch(t, projID, "<rfm2@test>", "a2")
	p3 := s.insertPatch(t, projID, "<rfm3@test>", "b1")
	p4 := s.insertPatch(t, projID, "<rfm4@test>", "b2")
	userID := s.insertUser(t, "rfmm", "rfmm@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	// create two separate relations
	s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/patches/%d", p1), token,
		map[string]any{"related": []int{p2}})
	s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/patches/%d", p3), token,
		map[string]any{"related": []int{p4}})

	// try to merge -- should conflict
	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/patches/%d", p1), token,
		map[string]any{"related": []int{p3}})
	if resp.StatusCode != 409 {
		t.Errorf("status = %d, want 409", resp.StatusCode)
	}
}

func TestRelationListTwoPatch(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	p1 := s.insertPatch(t, projID, "<rel1@test>", "p1")
	p2 := s.insertPatch(t, projID, "<rel2@test>", "p2")
	userID := s.insertUser(t, "relmaint", "rm@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	// create relation
	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/patches/%d", p1), token,
		map[string]any{"related": []int{p2}})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	// check p1 sees p2
	patch1 := getOne(t, s, fmt.Sprintf("/api/1.4/patches/%d", p1))
	rel1, ok := patch1["related"].([]any)
	if !ok {
		t.Fatal("p1 related field missing")
	}
	if len(rel1) != 1 {
		t.Fatalf("p1 related: got %d, want 1", len(rel1))
	}
	relPatch := rel1[0].(map[string]any)
	if int(relPatch["id"].(float64)) != int(p2) {
		t.Errorf("p1 related[0].id = %v, want %d", relPatch["id"], p2)
	}

	// check p2 sees p1
	patch2 := getOne(t, s, fmt.Sprintf("/api/1.4/patches/%d", p2))
	rel2, ok := patch2["related"].([]any)
	if !ok {
		t.Fatal("p2 related field missing")
	}
	if len(rel2) != 1 {
		t.Fatalf("p2 related: got %d, want 1", len(rel2))
	}
}

func TestRelationNoRelation(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<norel@test>", "p")

	p := getOne(t, s, fmt.Sprintf("/api/1.4/patches/%d", patchID))
	related, ok := p["related"].([]any)
	if !ok {
		t.Fatal("related should be an array")
	}
	if len(related) != 0 {
		t.Errorf("related should be empty, got %d", len(related))
	}
}
