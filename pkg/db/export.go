// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/schema"
)

type exportTable struct {
	// dstName is the table name in the Go schema.
	dstName string
	// srcName is the table name in the Django schema.
	srcName string
	// optional tables may not exist in older Django versions.
	optional bool
}

// Order matters. Tables that reference others must be dumped *after*.
var exportTables = []exportTable{
	{dstName: "auth_user", srcName: "auth_user"},
	{dstName: "auth_token", srcName: "authtoken_token"},
	{dstName: "state", srcName: "patchwork_state"},
	{dstName: "tag", srcName: "patchwork_tag"},
	{dstName: "project", srcName: "patchwork_project"},
	{dstName: "project_maintainer", srcName: "patchwork_userprofile_maintainer_projects"},
	{dstName: "delegation_rule", srcName: "patchwork_delegationrule"},
	{dstName: "person", srcName: "patchwork_person"},
	{dstName: "patch_relation", srcName: "patchwork_patchrelation"},
	{dstName: "cover", srcName: "patchwork_cover"},
	{dstName: "series", srcName: "patchwork_series"},
	{dstName: "series_reference", srcName: "patchwork_seriesreference"},
	{dstName: "series_metadata", srcName: "patchwork_seriesmetadata", optional: true},
	{dstName: "series_dependencies", srcName: "patchwork_series_dependencies", optional: true},
	{dstName: "patch", srcName: "patchwork_patch"},
	{dstName: "patch_tag", srcName: "patchwork_patchtag"},
	{dstName: "patch_comment", srcName: "patchwork_patchcomment"},
	{dstName: "cover_comment", srcName: "patchwork_covercomment"},
	// Deduplicate checks: keep only the latest per (patch, context, user).
	// Django accumulates duplicate checks; we enforce uniqueness.
	{dstName: "ci_check", srcName: "patchwork_check"},
	{dstName: "bundle", srcName: "patchwork_bundle"},
	{dstName: "bundle_patch", srcName: "patchwork_bundlepatch"},
	{dstName: "event", srcName: "patchwork_event"},
	{dstName: "email_confirmation", srcName: "patchwork_emailconfirmation"},
	{dstName: "webhook", srcName: "patchwork_webhook", optional: true},
}

func resolveDialect(database *bun.DB, target string) schema.Dialect {
	switch target {
	case "postgres":
		return pgdialect.New()
	case "mysql":
		return mysqldialect.New()
	case "sqlite":
		return sqlitedialect.New()
	default:
		return database.Dialect()
	}
}

func appendName(b []byte, name string, d schema.Dialect) []byte {
	return dialect.AppendName(b, name, d.IdentQuote())
}

func appendColumns(b []byte, cols []string, d schema.Dialect) []byte {
	for i, c := range cols {
		if i > 0 {
			b = append(b, ", "...)
		}
		b = appendName(b, c, d)
	}
	return b
}

// Export reads data from a Django patchwork database and writes SQL
// statements to w, using the Go schema table and column names. The
// target dialect is auto-detected from the database connection unless
// overridden. PostgreSQL exports use the COPY protocol for speed; MySQL
// and SQLite exports use INSERT statements with proper identifier
// quoting. The output can be imported into a fresh database initialized
// with "pw db sync". All patchwork 3.x schema variations are supported.
func Export(ctx context.Context, database *bun.DB, w io.Writer, target string) error {
	if !hasDjangoMigrations(ctx, database) {
		return fmt.Errorf("not a Django database (django_migrations table not found)")
	}

	d := resolveDialect(database, target)

	fmt.Fprint(w, "BEGIN;\n")
	writeDisableConstraints(w, d)

	// Clear seeded data so the imported rows don't conflict with
	// defaults inserted by "pw db sync".
	fmt.Fprint(w, "DELETE FROM tag;\n")
	fmt.Fprint(w, "DELETE FROM state;\n")

	writeRows := func(ctx context.Context, database *bun.DB, w io.Writer, srcTable, dstTable string) error {
		return exportTableRows(ctx, database, w, srcTable, dstTable, d)
	}
	if d.Name() == dialect.PG {
		writeRows = func(ctx context.Context, database *bun.DB, w io.Writer, srcTable, dstTable string) error {
			return copyTableRows(ctx, database, w, srcTable, dstTable, d)
		}
	}

	for _, t := range exportTables {
		if t.optional && !tableExists(ctx, database, t.srcName) {
			continue
		}
		if err := writeRows(ctx, database, w, t.srcName, t.dstName); err != nil {
			return fmt.Errorf("export %s: %w", t.dstName, err)
		}
	}

	writeEnableConstraints(w, d)
	fmt.Fprint(w, "COMMIT;\n")
	return nil
}

func writeDisableConstraints(w io.Writer, d schema.Dialect) {
	switch d.Name() {
	case dialect.MySQL:
		fmt.Fprint(w, "SET FOREIGN_KEY_CHECKS = 0;\n")
	case dialect.SQLite:
		fmt.Fprint(w, "PRAGMA foreign_keys = OFF;\n")
	}
}

func writeEnableConstraints(w io.Writer, d schema.Dialect) {
	switch d.Name() {
	case dialect.MySQL:
		fmt.Fprint(w, "SET FOREIGN_KEY_CHECKS = 1;\n")
	case dialect.SQLite:
		fmt.Fprint(w, "PRAGMA foreign_keys = ON;\n")
	}
}

func hasDjangoMigrations(ctx context.Context, database *bun.DB) bool {
	_, err := database.NewSelect().
		TableExpr("django_migrations").
		Limit(1).
		Exec(ctx)
	return err == nil
}

func tableExists(ctx context.Context, database *bun.DB, name string) bool {
	var n int
	_ = database.NewSelect().
		TableExpr("?", bun.Ident(name)).
		ColumnExpr("1").
		Limit(1).
		Scan(ctx, &n)
	return n == 1
}

const batchSize = 100

// dstColumns lists the expected columns for each destination table. When
// the source table has fewer columns (older Django schema), missing
// columns get default values.
var dstColumns = map[string][]string{
	"project": {
		"id", "linkname", "name", "listid", "listemail",
		"subject_match", "web_url", "scm_url", "webscm_url",
		"list_archive_url", "list_archive_url_format",
		"commit_url_format", "send_notifications", "use_tags",
		"show_dependencies", "auto_supersede",
	},
	"series": {
		"id", "project_id", "cover_letter_id", "previous_series_id",
		"name", "date", "submitter_id", "version", "total",
	},
	"patch_comment": {
		"id", "msgid", "date", "headers", "submitter_id",
		"content", "patch_id", "addressed",
	},
	"cover_comment": {
		"id", "msgid", "date", "headers", "submitter_id",
		"content", "cover_id", "addressed",
	},
}

// customQueries provide hand-written SELECT queries for tables where
// the Django schema differs structurally from the Go schema (column
// renames, joins, deduplication). Placeholders {column} are resolved
// at runtime: expanded to "e.column" if the column exists in the
// source table, or "NULL AS column" otherwise.
var customQueries = map[string]string{
	// Django auth_user has is_staff (dropped) and is_superuser (renamed
	// to is_admin). Profile columns are merged from patchwork_userprofile.
	"auth_user": `SELECT u.id, u.username, u.password,
		u.first_name, u.last_name, u.email,
		u.is_superuser AS is_admin, u.is_active,
		u.date_joined, u.last_login,
		COALESCE(p.send_email, false) AS send_email,
		COALESCE(p.items_per_page, 100) AS items_per_page,
		COALESCE(p.show_ids, false) AS show_ids
		FROM auth_user u
		LEFT JOIN patchwork_userprofile p ON p.user_id = u.id`,
	// Django project_maintainer goes through userprofile; resolve to
	// user_id directly.
	"patchwork_userprofile_maintainer_projects": `SELECT m.id,
		up.user_id, m.project_id
		FROM patchwork_userprofile_maintainer_projects m
		JOIN patchwork_userprofile up ON up.id = m.userprofile_id`,
	// Deduplicate checks: Django accumulates duplicates per
	// (patch, context, user). Keep only the most recent.
	"patchwork_check": `SELECT c.id, c.patch_id, c.user_id, c.date,
		c.state, c.target_url, c.context, c.description
		FROM patchwork_check c
		INNER JOIN (
			SELECT patch_id, context, user_id, MAX(date) AS max_date
			FROM patchwork_check
			GROUP BY patch_id, context, user_id
		) latest ON c.patch_id = latest.patch_id
			AND c.context = latest.context
			AND COALESCE(c.user_id, 0) = COALESCE(latest.user_id, 0)
			AND c.date = latest.max_date`,
	// Drop events that reference deduplicated checks.
	"patchwork_event": `SELECT e.id, e.project_id, e.category, e.date,
		e.actor_id, e.patch_id, e.series_id, e.cover_id,
		e.previous_state_id, e.current_state_id,
		e.previous_delegate_id, e.current_delegate_id,
		e.previous_relation_id, e.current_relation_id,
		e.created_check_id,
		{cover_comment_id},
		{patch_comment_id}
		FROM patchwork_event e
		WHERE e.created_check_id IS NULL
		   OR e.created_check_id IN (
			SELECT c.id FROM patchwork_check c
			INNER JOIN (
				SELECT patch_id, context, user_id, MAX(date) AS max_date
				FROM patchwork_check
				GROUP BY patch_id, context, user_id
			) latest ON c.patch_id = latest.patch_id
				AND c.context = latest.context
				AND COALESCE(c.user_id, 0) = COALESCE(latest.user_id, 0)
				AND c.date = latest.max_date
		)`,
}

func buildQuery(ctx context.Context, database *bun.DB, srcTable, dstTable string) string {
	if q, ok := customQueries[srcTable]; ok {
		return resolveTemplateCols(ctx, database, srcTable, q)
	}

	expected, ok := dstColumns[dstTable]
	if !ok {
		return "SELECT * FROM " + srcTable
	}

	srcCols := probeColumns(ctx, database, srcTable)
	if len(srcCols) == 0 {
		return "SELECT * FROM " + srcTable
	}

	var selects []string
	for _, col := range expected {
		if srcCols[col] {
			selects = append(selects, col)
		} else {
			selects = append(selects, columnDefault(col)+" AS "+col)
		}
	}
	return "SELECT " + strings.Join(selects, ", ") + " FROM " + srcTable
}

// resolveTemplateCols replaces {column} placeholders in a query. If the
// column exists in the source table, it expands to "e.column". If not,
// it expands to "NULL AS column".
func resolveTemplateCols(ctx context.Context, database *bun.DB, srcTable string, query string) string {
	srcCols := probeColumns(ctx, database, srcTable)

	result := query
	for {
		start := strings.Index(result, "{")
		if start < 0 {
			break
		}
		end := strings.Index(result[start:], "}")
		if end < 0 {
			break
		}
		col := result[start+1 : start+end]
		var replacement string
		if srcCols[col] {
			replacement = "e." + col
		} else {
			replacement = "NULL AS " + col
		}
		result = result[:start] + replacement + result[start+end+1:]
	}
	return result
}

func probeColumns(ctx context.Context, database *bun.DB, table string) map[string]bool {
	rows, err := database.QueryContext(ctx, "SELECT * FROM "+table+" LIMIT 0")
	if err != nil {
		return nil
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil
	}
	m := make(map[string]bool, len(cols))
	for _, c := range cols {
		m[c] = true
	}
	return m
}

func columnDefault(col string) string {
	switch col {
	case "show_dependencies", "auto_supersede":
		return "false"
	default:
		return "NULL"
	}
}

func scanRows(ctx context.Context, database *bun.DB, srcTable, dstTable string) (*sql.Rows, []string, error) {
	query := buildQuery(ctx, database, srcTable, dstTable)

	rows, err := database.QueryContext(ctx, query)
	if err != nil {
		return nil, nil, err
	}

	cols, err := rows.Columns()
	if err != nil {
		rows.Close()
		return nil, nil, err
	}
	return rows, cols, nil
}

func exportTableRows(ctx context.Context, database *bun.DB, w io.Writer, srcTable, dstTable string, d schema.Dialect) error {
	rows, cols, err := scanRows(ctx, database, srcTable, dstTable)
	if err != nil {
		return err
	}
	defer rows.Close()

	if len(cols) == 0 {
		return nil
	}

	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}

	var header []byte
	header = append(header, "INSERT INTO "...)
	header = appendName(header, dstTable, d)
	header = append(header, " ("...)
	header = appendColumns(header, cols, d)
	header = append(header, ") VALUES\n"...)

	n := 0
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		if n%batchSize == 0 {
			if n > 0 {
				fmt.Fprint(w, ";\n")
			}
			_, _ = w.Write(header)
		} else {
			fmt.Fprint(w, ",\n")
		}
		fmt.Fprintf(w, "(%s)", appendValues(nil, vals, d))
		n++
	}
	if n > 0 {
		fmt.Fprint(w, ";\n")
	}
	return rows.Err()
}

func appendValues(b []byte, vals []any, d schema.Dialect) []byte {
	for i, v := range vals {
		if i > 0 {
			b = append(b, ", "...)
		}
		b = appendValue(b, v, d)
	}
	return b
}

func appendValue(b []byte, v any, d schema.Dialect) []byte {
	if v == nil {
		return dialect.AppendNull(b)
	}
	switch val := v.(type) {
	case bool:
		return d.AppendBool(b, val)
	case int64:
		return strconv.AppendInt(b, val, 10)
	case float64:
		return dialect.AppendFloat64(b, val)
	case time.Time:
		return d.AppendTime(b, val)
	case []byte:
		return d.AppendString(b, string(val))
	case string:
		return d.AppendString(b, val)
	case sql.NullString:
		if !val.Valid {
			return dialect.AppendNull(b)
		}
		return d.AppendString(b, val.String)
	case sql.NullInt64:
		if !val.Valid {
			return dialect.AppendNull(b)
		}
		return strconv.AppendInt(b, val.Int64, 10)
	case sql.NullFloat64:
		if !val.Valid {
			return dialect.AppendNull(b)
		}
		return dialect.AppendFloat64(b, val.Float64)
	case sql.NullBool:
		if !val.Valid {
			return dialect.AppendNull(b)
		}
		return d.AppendBool(b, val.Bool)
	case sql.NullTime:
		if !val.Valid {
			return dialect.AppendNull(b)
		}
		return d.AppendTime(b, val.Time)
	default:
		return d.AppendString(b, fmt.Sprintf("%v", val))
	}
}

func copyTableRows(ctx context.Context, database *bun.DB, w io.Writer, srcTable, dstTable string, d schema.Dialect) error {
	rows, cols, err := scanRows(ctx, database, srcTable, dstTable)
	if err != nil {
		return err
	}
	defer rows.Close()

	if len(cols) == 0 {
		return nil
	}

	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}

	var header []byte
	header = append(header, "COPY "...)
	header = appendName(header, dstTable, d)
	header = append(header, " ("...)
	header = appendColumns(header, cols, d)
	header = append(header, ") FROM stdin;\n"...)
	_, _ = w.Write(header)

	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		fmt.Fprintln(w, formatCopyRow(vals))
	}
	fmt.Fprint(w, "\\.\n")
	return rows.Err()
}

func formatCopyRow(vals []any) string {
	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = formatCopyValue(v)
	}
	return strings.Join(parts, "\t")
}

func formatCopyValue(v any) string {
	if v == nil {
		return "\\N"
	}
	switch val := v.(type) {
	case bool:
		if val {
			return "t"
		}
		return "f"
	case int64:
		return strconv.FormatInt(val, 10)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case time.Time:
		return val.Format("2006-01-02 15:04:05.999999-07:00")
	case []byte:
		return escapeCopyString(string(val))
	case string:
		return escapeCopyString(val)
	case sql.NullString:
		if !val.Valid {
			return "\\N"
		}
		return escapeCopyString(val.String)
	case sql.NullInt64:
		if !val.Valid {
			return "\\N"
		}
		return strconv.FormatInt(val.Int64, 10)
	case sql.NullFloat64:
		if !val.Valid {
			return "\\N"
		}
		return strconv.FormatFloat(val.Float64, 'f', -1, 64)
	case sql.NullBool:
		if !val.Valid {
			return "\\N"
		}
		if val.Bool {
			return "t"
		}
		return "f"
	case sql.NullTime:
		if !val.Valid {
			return "\\N"
		}
		return val.Time.Format("2006-01-02 15:04:05.999999-07:00")
	default:
		return escapeCopyString(fmt.Sprintf("%v", val))
	}
}

func escapeCopyString(s string) string {
	r := strings.NewReplacer(
		"\\", "\\\\",
		"\t", "\\t",
		"\n", "\\n",
		"\r", "\\r",
	)
	return r.Replace(s)
}
