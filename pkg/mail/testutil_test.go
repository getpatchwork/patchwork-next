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
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-mbox"
	"github.com/emersion/go-message/mail"
	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/config"
	"github.com/getpatchwork/patchwork/pkg/db"
	"github.com/getpatchwork/patchwork/pkg/db/migrations"
	"github.com/getpatchwork/patchwork/pkg/events"
)

// testDB returns a fresh in-memory database with schema bootstrapped.
// Each test gets its own isolated database, event bus, and context.
func testDB(t *testing.T, listid string) (*bun.DB, context.Context, *events.Bus, *db.Project) {
	t.Helper()

	cfg := &config.Config{}
	cfg.Database.URL = "sqlite://:memory:"
	database, err := db.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if err := migrations.RunMigrations(context.Background(), database); err != nil {
		t.Fatal(err)
	}

	bus := events.Start(context.Background(), database)
	ctx := db.WithBus(context.Background(), bus)
	t.Cleanup(func() {
		bus.Shutdown()
		database.Close()
	})

	proj := db.Project{
		Linkname:  "test-project",
		Name:      "Test Project",
		Listid:    listid,
		Listemail: "test@" + listid,
		UseTags:   true,
	}
	err = database.NewInsert().Model(&proj).
		Returning("*").
		Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	return database, ctx, bus, &proj
}

// testProject inserts a project with a custom linkname, name, and
// optional subject_match.
func testProject(t *testing.T, database *bun.DB, linkname, name, listid, subjectMatch string) *db.Project {
	t.Helper()
	ctx := context.Background()
	var id int
	err := database.NewRaw(`
		INSERT INTO project (
			linkname, name, listid, listemail, subject_match,
			web_url, scm_url, webscm_url,
			list_archive_url, list_archive_url_format, commit_url_format,
			send_notifications, use_tags, show_dependencies, auto_supersede
		) VALUES (?, ?, ?, ?, ?, '', '', '', '', '', '', false, true, false, false)
		RETURNING id
	`, linkname, name, listid, "test@"+listid, subjectMatch).Scan(ctx, &id)
	if err != nil {
		t.Fatal(err)
	}
	return &db.Project{
		ID: id, Linkname: linkname, Name: name,
		Listid: listid, Listemail: "test@" + listid,
		SubjectMatch: subjectMatch, UseTags: true,
	}
}

// TestDBSetup verifies the test database infrastructure works.
func TestDBSetup(t *testing.T) {
	database, _, _, proj := testDB(t, "test.example.com")

	var count int
	err := database.NewSelect().
		TableExpr("state").
		ColumnExpr("count(*)").
		Scan(t.Context(), &count)
	if err != nil {
		t.Fatal(err)
	}
	if count < 5 {
		t.Errorf("expected at least 5 states, got %d", count)
	}

	err = database.NewSelect().
		TableExpr("tag").
		ColumnExpr("count(*)").
		Scan(t.Context(), &count)
	if err != nil {
		t.Fatal(err)
	}
	if count < 3 {
		t.Errorf("expected at least 3 tags, got %d", count)
	}

	if proj.ID == 0 {
		t.Error("expected project ID to be set")
	}
}

// parseMbox reads an mbox file from testdata/ and calls ParseMail for
// each message. If listid is non-empty, a List-Id header is injected
// into each message. Returns counts of objects created.
func parseMbox(t *testing.T, ctx context.Context, database *bun.DB, filename string, listid ...string) parseResult {
	t.Helper()

	data, err := os.ReadFile("testdata/" + filename)
	if err != nil {
		t.Fatal(err)
	}

	r := mbox.NewReader(bytes.NewReader(data))
	var result parseResult

	for {
		msg, err := r.NextMessage()
		if err != nil {
			break
		}
		buf, err := io.ReadAll(msg)
		if err != nil {
			t.Fatal(err)
		}
		var lid string
		if len(listid) > 0 {
			lid = listid[0]
		}
		err = ParseMail(ctx, database, bytes.NewReader(buf), lid)
		if err != nil {
			var dup *DuplicateMailError
			if errors.As(err, &dup) {
				result.duplicates++
				continue
			}
			var pe *ParseError
			if errors.As(err, &pe) {
				result.parseErrors++
				continue
			}
			t.Fatalf("ParseMail: %v", err)
		}
		result.messages++
	}

	database.NewSelect().Model((*db.Patch)(nil)).
		ColumnExpr("count(*)").Scan(ctx, &result.patches)
	database.NewSelect().Model((*db.Cover)(nil)).
		ColumnExpr("count(*)").Scan(ctx, &result.covers)
	database.NewSelect().Model((*db.Series)(nil)).
		ColumnExpr("count(*)").Scan(ctx, &result.series)
	database.NewSelect().Model((*db.PatchComment)(nil)).
		ColumnExpr("count(*)").Scan(ctx, &result.patchComments)
	database.NewSelect().Model((*db.CoverComment)(nil)).
		ColumnExpr("count(*)").Scan(ctx, &result.coverComments)

	return result
}

type parseResult struct {
	messages      int
	duplicates    int
	parseErrors   int
	patches       int
	covers        int
	series        int
	patchComments int
	coverComments int
}

// makeHeader builds a go-message mail.Header from raw header key-value
// pairs. Useful for testing header parsing functions.
func makeHeader(t *testing.T, headers map[string]string) *mail.Header {
	t.Helper()
	var buf bytes.Buffer
	for k, v := range headers {
		fmt.Fprintf(&buf, "%s: %s\r\n", k, v)
	}
	buf.WriteString("\r\n")
	m, err := mail.CreateReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	return &m.Header
}

// countPatches returns the number of patches in the database.
func countPatches(t *testing.T, database *bun.DB) int {
	t.Helper()
	var n int
	database.NewSelect().TableExpr("patch").
		ColumnExpr("count(*)").Scan(context.Background(), &n)
	return n
}

// countCovers returns the number of cover letters in the database.
func countCovers(t *testing.T, database *bun.DB) int {
	t.Helper()
	var n int
	database.NewSelect().TableExpr("cover").
		ColumnExpr("count(*)").Scan(context.Background(), &n)
	return n
}

// countPatchComments returns the number of patch comments in the database.
func countPatchComments(t *testing.T, database *bun.DB) int {
	t.Helper()
	var n int
	database.NewSelect().TableExpr("patch_comment").
		ColumnExpr("count(*)").Scan(context.Background(), &n)
	return n
}

// assertAllPatchesHaveSeries verifies every patch has a series_id set.
func assertAllPatchesHaveSeries(t *testing.T, database *bun.DB) {
	t.Helper()
	var orphans int
	database.NewSelect().TableExpr("patch").
		ColumnExpr("count(*)").Where("series_id IS NULL").
		Scan(context.Background(), &orphans)
	if orphans > 0 {
		t.Errorf("%d patches have no series assignment", orphans)
	}
}

// assertAllPatchesInOneSeries verifies all patches belong to the same series.
func assertAllPatchesInOneSeries(t *testing.T, database *bun.DB) {
	t.Helper()
	var distinctSeries int
	database.NewRaw(
		"SELECT count(DISTINCT series_id) FROM patch WHERE series_id IS NOT NULL",
	).Scan(context.Background(), &distinctSeries)
	if distinctSeries != 1 {
		t.Errorf("patches belong to %d different series, want 1", distinctSeries)
	}
}

// assertCoverLinkedToSeries verifies the cover letter is linked to a series.
func assertCoverLinkedToSeries(t *testing.T, database *bun.DB) {
	t.Helper()
	var count int
	database.NewRaw(`
		SELECT count(*) FROM series
		WHERE cover_letter_id IS NOT NULL
	`).Scan(context.Background(), &count)
	if count == 0 {
		t.Error("no series has a cover letter linked")
	}
}

// assertPatchesInCorrectProject verifies all patches belong to the given project.
func assertPatchesInCorrectProject(t *testing.T, database *bun.DB, projectID int) {
	t.Helper()
	var wrong int
	database.NewSelect().TableExpr("patch").
		ColumnExpr("count(*)").Where("project_id != ?", projectID).
		Scan(context.Background(), &wrong)
	if wrong > 0 {
		t.Errorf("%d patches in wrong project", wrong)
	}
}

// assertDelegateIs verifies the most recent patch has the given delegate.
func assertDelegateIs(t *testing.T, database *bun.DB, wantUserID int) {
	t.Helper()
	var delegateID *int
	database.NewSelect().TableExpr("patch").
		Column("delegate_id").OrderExpr("id DESC").Limit(1).
		Scan(context.Background(), &delegateID)
	if delegateID == nil || *delegateID != wantUserID {
		t.Errorf("delegate_id = %v, want %d", delegateID, wantUserID)
	}
}

func assertPatchContent(t *testing.T, database *bun.DB, msgid string, wantDiff, wantComment bool) {
	t.Helper()
	var diff, content *string
	database.NewSelect().TableExpr("patch").
		Column("diff", "content").Where("msgid = ?", msgid).
		Scan(context.Background(), &diff, &content)
	if wantDiff && diff == nil {
		t.Error("expected diff to be set")
	}
	if !wantDiff && diff != nil {
		t.Error("expected no diff")
	}
	if wantComment && content == nil {
		t.Error("expected content to be set")
	}
}

// parseMboxTemplate reads an mbox template, substitutes {depends_token}
// with the given value, and parses all messages.
func parseMboxTemplate(t *testing.T, ctx context.Context, database *bun.DB, filename, dependsToken, listid string) parseResult {
	t.Helper()
	data, err := os.ReadFile("testdata/" + filename)
	if err != nil {
		t.Fatal(err)
	}
	expanded := strings.ReplaceAll(string(data), "{depends_token}", dependsToken)

	r := mbox.NewReader(strings.NewReader(expanded))
	var result parseResult

	for {
		msg, err := r.NextMessage()
		if err != nil {
			break
		}
		buf, err := io.ReadAll(msg)
		if err != nil {
			t.Fatal(err)
		}
		err = ParseMail(ctx, database, bytes.NewReader(buf), listid)
		if err != nil {
			var dup *DuplicateMailError
			if errors.As(err, &dup) {
				result.duplicates++
				continue
			}
			var pe *ParseError
			if errors.As(err, &pe) {
				result.parseErrors++
				continue
			}
			t.Fatalf("ParseMail: %v", err)
		}
		result.messages++
	}

	database.NewSelect().Model((*db.Patch)(nil)).
		ColumnExpr("count(*)").Scan(ctx, &result.patches)
	database.NewSelect().Model((*db.Cover)(nil)).
		ColumnExpr("count(*)").Scan(ctx, &result.covers)
	database.NewSelect().Model((*db.Series)(nil)).
		ColumnExpr("count(*)").Scan(ctx, &result.series)

	return result
}

// assertDependencyCount checks the number of series dependencies.
func assertDependencyCount(t *testing.T, database *bun.DB, want int) {
	t.Helper()
	var count int
	database.NewSelect().TableExpr("series_dependencies").
		ColumnExpr("count(*)").
		Scan(context.Background(), &count)
	if count != want {
		t.Errorf("dependency count = %d, want %d", count, want)
	}
}

// assertSerialized verifies patch-to-series assignment matches expected
// counts. counts is a list of expected patches per series, ordered by
// series date. For example, [2, 2] means 2 series with 2 patches each.
func assertSerialized(t *testing.T, database *bun.DB, patchCounts []int) {
	t.Helper()
	ctx := context.Background()

	var seriesCount int
	database.NewSelect().TableExpr("series").
		ColumnExpr("count(*)").Scan(ctx, &seriesCount)
	if seriesCount != len(patchCounts) {
		t.Errorf("series count = %d, want %d", seriesCount, len(patchCounts))
		return
	}

	var seriesIDs []int
	database.NewRaw(
		"SELECT id FROM series ORDER BY date",
	).Scan(ctx, &seriesIDs)

	for i, wantCount := range patchCounts {
		var gotCount int
		database.NewSelect().TableExpr("patch").
			ColumnExpr("count(*)").Where("series_id = ?", seriesIDs[i]).
			Scan(ctx, &gotCount)
		if gotCount != wantCount {
			t.Errorf("series[%d] (id=%d): %d patches, want %d",
				i, seriesIDs[i], gotCount, wantCount)
		}
	}
}

// createEmail builds a raw RFC 822 message from the given parameters.
func createEmail(body string, opts ...emailOpt) []byte {
	o := emailOpts{
		subject: "Test subject",
		from:    "Test Author <test-author@example.com>",
		listid:  "test.example.com",
		msgid:   fmt.Sprintf("<%d@test>", time.Now().UnixNano()),
	}
	for _, fn := range opts {
		fn(&o)
	}
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "From: %s\r\n", o.from)
	fmt.Fprintf(&buf, "Subject: %s\r\n", o.subject)
	fmt.Fprintf(&buf, "Message-ID: %s\r\n", o.msgid)
	fmt.Fprintf(&buf, "List-Id: <%s>\r\n", o.listid)
	if o.inReplyTo != "" {
		fmt.Fprintf(&buf, "In-Reply-To: %s\r\n", o.inReplyTo)
	}
	if o.references != "" {
		fmt.Fprintf(&buf, "References: %s\r\n", o.references)
	}
	for k, v := range o.headers {
		fmt.Fprintf(&buf, "%s: %s\r\n", k, v)
	}
	fmt.Fprintf(&buf, "Content-Type: text/plain; charset=us-ascii\r\n")
	fmt.Fprintf(&buf, "\r\n%s\r\n", body)
	return buf.Bytes()
}

type emailOpts struct {
	subject    string
	from       string
	listid     string
	msgid      string
	inReplyTo  string
	references string
	headers    map[string]string
}

type emailOpt func(*emailOpts)

func withSubject(s string) emailOpt    { return func(o *emailOpts) { o.subject = s } }
func withFrom(s string) emailOpt       { return func(o *emailOpts) { o.from = s } }
func withListID(s string) emailOpt     { return func(o *emailOpts) { o.listid = s } }
func withMsgID(s string) emailOpt      { return func(o *emailOpts) { o.msgid = s } }
func withInReplyTo(s string) emailOpt  { return func(o *emailOpts) { o.inReplyTo = s } }
func withReferences(s string) emailOpt { return func(o *emailOpts) { o.references = s } }
func withHeader(k, v string) emailOpt {
	return func(o *emailOpts) {
		if o.headers == nil {
			o.headers = map[string]string{}
		}
		o.headers[k] = v
	}
}

// parseEmail creates and parses a single programmatic email.
func parseEmail(t *testing.T, ctx context.Context, database *bun.DB, body string, opts ...emailOpt) error {
	t.Helper()
	data := createEmail(body, opts...)
	return ParseMail(ctx, database, bytes.NewReader(data))
}

// parseEml reads a single .eml or .mbox file and calls ParseMail.
func parseEml(t *testing.T, ctx context.Context, database *bun.DB, filename string) error {
	t.Helper()
	data, err := os.ReadFile("testdata/" + filename)
	if err != nil {
		t.Fatal(err)
	}
	return ParseMail(ctx, database, bytes.NewReader(data))
}

// countEvents drains pending events by shutting down the bus, then
// returns the count of events with the given category.
func countEvents(t *testing.T, database *bun.DB, bus *events.Bus, category string) int {
	t.Helper()
	bus.Shutdown()
	var n int
	database.NewSelect().Model((*db.Event)(nil)).
		ColumnExpr("count(*)").Where("category = ?", category).
		Scan(context.Background(), &n)
	return n
}

// countAllEvents returns the total number of events.
func countAllEvents(t *testing.T, database *bun.DB) int {
	t.Helper()
	var n int
	database.NewSelect().Model((*db.Event)(nil)).
		ColumnExpr("count(*)").
		Scan(context.Background(), &n)
	return n
}

// getSeriesName returns the name of the first series.
func getSeriesName(t *testing.T, database *bun.DB) string {
	t.Helper()
	var name *string
	database.NewSelect().TableExpr("series").
		Column("name").OrderExpr("id ASC").Limit(1).
		Scan(context.Background(), &name)
	if name == nil {
		return ""
	}
	return *name
}
