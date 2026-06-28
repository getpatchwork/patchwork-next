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

func TestCommentUpdateNotAuthorized(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<cuna@test>", "p")
	commentID := s.insertComment(t, patchID, "<cuna-c@test>")

	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/patches/%d/comments/%d", patchID, commentID), "",
		map[string]bool{"addressed": true})
	if resp.StatusCode != 401 {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestCoverCommentDetailInvalidCover(t *testing.T) {
	s := newTestServer(t)
	resp := s.get(t, "/api/1.4/covers/99999/comments/1")
	if resp.StatusCode != 404 {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestCoverCommentList(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	coverID := s.insertCover(t, projID, "<ccl@test>", "c")
	personID := s.insertPerson(t, "ccr@test", "Reviewer")
	s.exec(t, `INSERT INTO cover_comment
		(msgid, date, headers, submitter_id, cover_id, content)
		VALUES ('<cc-comment@test>', datetime('now'), '', ?, ?, 'lgtm')`,
		personID, coverID)

	items := getList(t, s, fmt.Sprintf("/api/1.4/covers/%d/comments/", coverID))
	if len(items) != 1 {
		t.Fatalf("got %d, want 1", len(items))
	}
	assertField(t, items[0], "id")
	assertField(t, items[0], "content")
	assertNested(t, items[0], "submitter", "id")
}

func TestCoverCommentListInvalidCover(t *testing.T) {
	s := newTestServer(t)
	resp := s.get(t, "/api/1.4/covers/invalid/comments/")
	if resp.StatusCode != 422 {
		t.Errorf("status = %d, want 422", resp.StatusCode)
	}
}

func TestCoverCommentListNonExistent(t *testing.T) {
	s := newTestServer(t)
	items := getList(t, s, "/api/1.4/covers/99999/comments/")
	if len(items) != 0 {
		t.Errorf("got %d, want 0", len(items))
	}
}

func TestPatchCommentCreate405(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<pcc@test>", "p")

	resp := s.authRequest(t, "POST",
		fmt.Sprintf("/api/1.4/patches/%d/comments/", patchID), "",
		map[string]string{"content": "test"})
	if resp.StatusCode != 405 {
		t.Errorf("status = %d, want 405", resp.StatusCode)
	}
}

func TestPatchCommentDelete405(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<pcd@test>", "p")
	commentID := s.insertComment(t, patchID, "<pcd-c@test>")

	resp := s.authRequest(t, "DELETE",
		fmt.Sprintf("/api/1.4/patches/%d/comments/%d", patchID, commentID), "", nil)
	if resp.StatusCode != 405 {
		t.Errorf("status = %d, want 405", resp.StatusCode)
	}
}

func TestPatchCommentDetail(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<pcd@test>", "p")
	commentID := s.insertComment(t, patchID, "<cd@test>")

	c := getOne(t, s, fmt.Sprintf("/api/1.4/patches/%d/comments/%d", patchID, commentID))
	assertField(t, c, "content")
	assertNested(t, c, "submitter", "id")
}

func TestPatchCommentDetailInvalidPatch(t *testing.T) {
	s := newTestServer(t)
	resp := s.get(t, "/api/1.4/patches/99999/comments/1")
	if resp.StatusCode != 404 {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestPatchCommentList(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<pc@test>", "p")
	commentID := s.insertComment(t, patchID, "<comment@test>")

	items := getList(t, s, fmt.Sprintf("/api/1.4/patches/%d/comments/", patchID))
	if len(items) != 1 {
		t.Fatalf("got %d, want 1", len(items))
	}
	c := items[0]
	assertValue(t, c, "id", float64(commentID))
	assertValue(t, c, "msgid", "<comment@test>")
	assertContains(t, c, "url", fmt.Sprintf("/patches/%d/comments/%d/", patchID, commentID))
	assertField(t, c, "date")
	assertField(t, c, "addressed")
	assertNested(t, c, "submitter", "id")
	assertNested(t, c, "submitter", "email")
	if c["content"] != "looks good" {
		t.Errorf("content = %v, want 'looks good'", c["content"])
	}
}

func TestPatchCommentListEmpty(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<pce@test>", "p")

	items := getList(t, s, fmt.Sprintf("/api/1.4/patches/%d/comments/", patchID))
	if len(items) != 0 {
		t.Errorf("got %d, want 0", len(items))
	}
}

func TestPatchCommentListInvalidPatch(t *testing.T) {
	s := newTestServer(t)
	resp := s.get(t, "/api/1.4/patches/invalid/comments/")
	if resp.StatusCode != 422 {
		t.Errorf("status = %d, want 422", resp.StatusCode)
	}
}

func TestPatchCommentNonExistentPatch(t *testing.T) {
	s := newTestServer(t)
	items := getList(t, s, "/api/1.4/patches/99999/comments/")
	if len(items) != 0 {
		t.Errorf("got %d, want 0", len(items))
	}
}

func TestPatchCommentURLAndSubject(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<cus@test>", "p")
	personID := s.insertPerson(t, "cus@test", "CUS")
	s.exec(t, `INSERT INTO patch_comment
		(msgid, date, headers, submitter_id, patch_id, content)
		VALUES ('<cus-c@test>', datetime('now'), ?, ?, ?, 'ok')`,
		"Subject: Re: test\n", personID, patchID)

	items := getList(t, s, fmt.Sprintf("/api/1.4/patches/%d/comments/", patchID))
	if len(items) != 1 {
		t.Fatalf("got %d", len(items))
	}
	assertField(t, items[0], "url")
	if items[0]["subject"] != "Re: test" {
		t.Errorf("subject = %v, want 'Re: test'", items[0]["subject"])
	}
}

func TestUpdateCommentAddressed(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	patchID := s.insertPatch(t, projID, "<addr@test>", "p")
	commentID := s.insertComment(t, patchID, "<addr-c@test>")

	userID := s.insertUser(t, "m5", "m5@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/patches/%d/comments/%d", patchID, commentID),
		token, map[string]bool{"addressed": true})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var result map[string]any
	decodeJSON(t, resp, &result)
	if result["addressed"] != true {
		t.Errorf("addressed = %v, want true", result["addressed"])
	}
}

func TestUpdateCoverCommentAddressed(t *testing.T) {
	s := newTestServer(t)
	projID := s.insertProject(t)
	coverID := s.insertCover(t, projID, "<cca@test>", "c")
	personID := s.insertPerson(t, "cca@test", "CCA")
	var commentID int
	s.db.NewRaw(`INSERT INTO cover_comment
		(msgid, date, headers, submitter_id, cover_id, content)
		VALUES ('<cca-c@test>', datetime('now'), '', ?, ?, 'comment')
		RETURNING id`, personID, coverID).Scan(context.Background(), &commentID)

	userID := s.insertUser(t, "cca", "ccauser@test")
	token := s.insertToken(t, userID)
	s.makeMaintainer(t, userID, projID)

	resp := s.authRequest(t, "PATCH",
		fmt.Sprintf("/api/1.4/covers/%d/comments/%d", coverID, commentID),
		token, map[string]bool{"addressed": true})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var result map[string]any
	decodeJSON(t, resp, &result)
	if result["addressed"] != true {
		t.Errorf("addressed = %v, want true", result["addressed"])
	}
}
