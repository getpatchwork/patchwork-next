// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package mail

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestSenderCorrelation(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	t.Run("existing sender same format", func(t *testing.T) {
		parseEmail(t, ctx, database, sampleDiff,
			withFrom("Existing Sender <existing@example.com>"),
			withListID("test.example.com"))
		parseEmail(t, ctx, database, sampleDiff,
			withFrom("Existing Sender <existing@example.com>"),
			withListID("test.example.com"))

		var count int
		database.NewSelect().TableExpr("person").
			ColumnExpr("count(*)").
			Where("email = ?", "existing@example.com").
			Scan(context.Background(), &count)
		if count != 1 {
			t.Errorf("expected 1 person, got %d", count)
		}
	})

	t.Run("existing sender different case", func(t *testing.T) {
		parseEmail(t, ctx, database, sampleDiff,
			withFrom("EXISTING SENDER <EXISTING@EXAMPLE.COM>"),
			withListID("test.example.com"))

		var count int
		database.NewSelect().TableExpr("person").
			ColumnExpr("count(*)").
			Where("lower(email) = ?", "existing@example.com").
			Scan(context.Background(), &count)
		if count != 1 {
			t.Errorf("expected 1 person (case-insensitive), got %d", count)
		}
	})
}

func TestSenderEncoding(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	t.Run("ascii", func(t *testing.T) {
		err := parseEmail(t, ctx, database, sampleDiff,
			withFrom("Test Author <test@example.com>"),
			withListID("test.example.com"))
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("utf8 base64", func(t *testing.T) {
		err := parseEmail(t, ctx, database, sampleDiff,
			withFrom("=?utf-8?b?w4nDqQ==?= <encoded@example.com>"),
			withListID("test.example.com"))
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid email", func(t *testing.T) {
		err := parseEmail(t, ctx, database, sampleDiff,
			withFrom("invalid-email"),
			withListID("test.example.com"))
		var pe *ParseError
		if !errors.As(err, &pe) {
			t.Errorf("expected ParseError, got %v", err)
		}
	})
}

func TestSenderEncodingEmpty(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	data := "Subject: test\r\nMessage-ID: <empty-from@test>\r\n" +
		"List-Id: <test.example.com>\r\n\r\n" + sampleDiff
	err := ParseMail(ctx, database,
		strings.NewReader(data), "test.example.com")
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Errorf("expected ParseError for empty From, got %v", err)
	}
}

func TestSenderEncodingQP(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	err := parseEmail(t, ctx, database, sampleDiff,
		withFrom("=?utf-8?q?=C3=89ric?= <eric@example.com>"),
		withListID("test.example.com"))
	if err != nil {
		t.Fatal(err)
	}
	var name string
	database.NewSelect().TableExpr("person").
		Column("name").Where("email = ?", "eric@example.com").
		Scan(context.Background(), &name)
	if name == "" {
		t.Error("expected person name to be decoded")
	}
}

func TestSenderEncodingQPSplit(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	err := parseEmail(t, ctx, database, sampleDiff,
		withFrom("=?utf-8?q?Test?= =?utf-8?q?_User?= <test-split@example.com>"),
		withListID("test.example.com"))
	if err != nil {
		t.Fatal(err)
	}

	var name string
	database.NewSelect().TableExpr("person").
		Column("name").Where("email = ?", "test-split@example.com").
		Scan(context.Background(), &name)
	if name == "" {
		t.Error("expected person name from split QP encoding")
	}
}

func TestSenderDifferentFormat(t *testing.T) {
	database, ctx, _, _ := testDB(t, "test.example.com")

	parseEmail(t, ctx, database, sampleDiff,
		withFrom("Existing Sender <existing@example.com>"),
		withListID("test.example.com"))
	parseEmail(t, ctx, database, sampleDiff,
		withFrom("existing@example.com"),
		withListID("test.example.com"))

	var count int
	database.NewSelect().TableExpr("person").
		ColumnExpr("count(*)").
		Where("email = ?", "existing@example.com").
		Scan(context.Background(), &count)
	if count != 1 {
		t.Errorf("expected 1 person for same email different format, got %d", count)
	}
}

func TestSenderDMARCMunging(t *testing.T) {
	database, ctx, _, proj := testDB(t, "test.example.com")

	parseEmail(t, ctx, database, sampleDiff,
		withFrom("Existing Sender <existing@example.com>"),
		withListID("test.example.com"))

	t.Run("mailman via Reply-To", func(t *testing.T) {
		munged := fmt.Sprintf("Existing Sender via List <%s>", proj.Listemail)
		data := createEmail(sampleDiff,
			withFrom(munged),
			withListID("test.example.com"))
		raw := string(data)
		raw = strings.Replace(raw, "\r\n\r\n",
			"\r\nReply-To: Existing Sender <existing@example.com>\r\n\r\n", 1)
		ParseMail(ctx, database, strings.NewReader(raw),
			"test.example.com")

		var count int
		database.NewSelect().TableExpr("person").
			ColumnExpr("count(*)").
			Where("email = ?", "existing@example.com").
			Scan(context.Background(), &count)
		if count != 1 {
			t.Errorf("expected 1 person for existing@example.com, got %d", count)
		}
		var ids []int
		database.NewRaw(
			"SELECT DISTINCT submitter_id FROM patch",
		).Scan(context.Background(), &ids)
		if len(ids) != 1 {
			t.Errorf("expected all patches to have same submitter, got %d distinct", len(ids))
		}
	})

	t.Run("google X-Original-From", func(t *testing.T) {
		munged := fmt.Sprintf("'Existing Sender' via List <%s>", proj.Listemail)
		data := createEmail(sampleDiff,
			withFrom(munged),
			withListID("test.example.com"),
			withHeader("X-Original-From", "Existing Sender <existing@example.com>"))
		ParseMail(ctx, database, bytes.NewReader(data),
			"test.example.com")

		var count int
		database.NewSelect().TableExpr("person").
			ColumnExpr("count(*)").
			Where("email = ?", "existing@example.com").
			Scan(context.Background(), &count)
		if count != 1 {
			t.Errorf("expected 1 person for existing@example.com, got %d", count)
		}
	})
}

func TestSenderWeirdDMARCMunging(t *testing.T) {
	database, ctx, _, proj := testDB(t, "test.example.com")

	parseEmail(t, ctx, database, sampleDiff,
		withFrom("Existing Sender <existing@example.com>"),
		withListID("test.example.com"))

	munged := fmt.Sprintf("'Existing Sender' via <%s>", proj.Listemail)
	data := createEmail(sampleDiff,
		withFrom(munged),
		withListID("test.example.com"),
		withHeader("X-Original-From", "Existing Sender <existing@example.com>"))
	ParseMail(ctx, database, bytes.NewReader(data),
		"test.example.com")

	var count int
	database.NewSelect().TableExpr("person").
		ColumnExpr("count(*)").
		Where("email = ?", "existing@example.com").
		Scan(context.Background(), &count)
	if count != 1 {
		t.Errorf("expected 1 person after weird DMARC unmangling, got %d", count)
	}
}
