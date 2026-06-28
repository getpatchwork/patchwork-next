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
	"strings"
	"time"

	"github.com/uptrace/bun"
)

// exportTable defines a table to export and an optional source name to read
// from when the database uses Django naming conventions.
type exportTable struct {
	name       string
	djangoName string
}

// Order matters. Tables that references others must be dumped *after*.
var exportTables = []exportTable{
	{name: "auth_user", djangoName: "auth_user"},
	{name: "auth_token", djangoName: "authtoken_token"},
	{name: "state", djangoName: "patchwork_state"},
	{name: "tag", djangoName: "patchwork_tag"},
	{name: "project", djangoName: "patchwork_project"},
	{name: "project_maintainer", djangoName: "patchwork_userprofile_maintainer_projects"},
	{name: "delegation_rule", djangoName: "patchwork_delegationrule"},
	{name: "person", djangoName: "patchwork_person"},
	{name: "patch_relation", djangoName: "patchwork_patchrelation"},
	{name: "series", djangoName: "patchwork_series"},
	{name: "series_reference", djangoName: "patchwork_seriesreference"},
	{name: "series_metadata", djangoName: "patchwork_seriesmetadata"},
	{name: "series_dependencies", djangoName: "patchwork_series_dependencies"},
	{name: "cover", djangoName: "patchwork_cover"},
	{name: "patch", djangoName: "patchwork_patch"},
	{name: "patch_tag", djangoName: "patchwork_patchtag"},
	{name: "patch_comment", djangoName: "patchwork_patchcomment"},
	{name: "cover_comment", djangoName: "patchwork_covercomment"},
	// Deduplicate checks: keep only the latest per (patch, context, user).
	// Django accumulates duplicate checks; we enforce uniqueness.
	{name: "ci_check", djangoName: "patchwork_check"},
	{name: "bundle", djangoName: "patchwork_bundle"},
	{name: "bundle_patch", djangoName: "patchwork_bundlepatch"},
	{name: "event", djangoName: "patchwork_event"},
	{name: "email_confirmation", djangoName: "patchwork_emailconfirmation"},
	{name: "webhook", djangoName: "patchwork_webhook"},
}

// Export writes all table data as SQL INSERT statements to w. Tables are
// exported in foreign key dependency order so the output can be imported into
// a fresh database.
func Export(ctx context.Context, database *bun.DB, w io.Writer) error {
	isDjango := isDjangoDatabase(ctx, database)

	fmt.Fprint(w, "BEGIN;\n")

	for _, t := range exportTables {
		srcTable := t.name
		if isDjango {
			if t.djangoName == "" {
				continue
			}
			srcTable = t.djangoName
		}
		if err := exportTableRows(ctx, database, w, srcTable, t.name, isDjango); err != nil {
			return fmt.Errorf("export %s: %w", t.name, err)
		}
	}

	fmt.Fprint(w, "COMMIT;\n")
	return nil
}

// isDjangoDatabase returns true if the database contains a django_migrations
// table, indicating it was created by Django.
func isDjangoDatabase(ctx context.Context, database *bun.DB) bool {
	_, err := database.NewSelect().
		TableExpr("django_migrations").
		Limit(1).
		Exec(ctx)
	return err == nil
}

const batchSize = 100

// columnTransforms maps (srcTable:django) to custom SELECT queries used when
// column names or structure differ between Django and Go schemas.
var columnTransforms = map[string]string{
	// Django auth_user has is_staff (dropped) and is_superuser (renamed
	// to is_admin). Profile columns are merged from patchwork_userprofile.
	"auth_user:django": `SELECT u.id, u.username, u.password,
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
	"patchwork_userprofile_maintainer_projects:django": `SELECT m.id,
		up.user_id, m.project_id
		FROM patchwork_userprofile_maintainer_projects m
		JOIN patchwork_userprofile up ON up.id = m.userprofile_id`,
	// Deduplicate checks: Django accumulates duplicates per
	// (patch, context, user). Keep only the most recent.
	"patchwork_check:django": `SELECT c.id, c.patch_id, c.user_id, c.date,
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
	"patchwork_event:django": `SELECT e.id, e.project_id, e.category, e.date,
		e.actor_id, e.patch_id, e.series_id, e.cover_id,
		e.previous_state_id, e.current_state_id,
		e.previous_delegate_id, e.current_delegate_id,
		e.previous_relation_id, e.current_relation_id,
		e.created_check_id, e.cover_comment_id, e.patch_comment_id
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

func exportTableRows(ctx context.Context, database *bun.DB, w io.Writer, srcTable, dstTable string, isDjango bool) error {
	query := "SELECT * FROM " + srcTable
	if isDjango {
		if q, ok := columnTransforms[srcTable+":django"]; ok {
			query = q
		}
	}

	rows, err := database.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	if len(cols) == 0 {
		return nil
	}

	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}

	header := fmt.Sprintf("INSERT INTO %q (%s) VALUES\n",
		dstTable, strings.Join(cols, ", "))
	n := 0
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		if n%batchSize == 0 {
			if n > 0 {
				fmt.Fprint(w, ";\n")
			}
			fmt.Fprint(w, header)
		} else {
			fmt.Fprint(w, ",\n")
		}
		fmt.Fprintf(w, "(%s)", formatValues(vals))
		n++
	}
	if n > 0 {
		fmt.Fprint(w, ";\n")
	}
	return rows.Err()
}

func formatValues(vals []any) string {
	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = formatValue(v)
	}
	return strings.Join(parts, ", ")
}

func formatValue(v any) string {
	if v == nil {
		return "NULL"
	}
	switch val := v.(type) {
	case bool:
		if val {
			return "1"
		}
		return "0"
	case int64:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%g", val)
	case time.Time:
		return "'" + val.Format("2006-01-02T15:04:05-07:00") + "'"
	case []byte:
		return "'" + escapeSQLString(string(val)) + "'"
	case string:
		return "'" + escapeSQLString(val) + "'"
	case sql.NullString:
		if !val.Valid {
			return "NULL"
		}
		return "'" + escapeSQLString(val.String) + "'"
	case sql.NullInt64:
		if !val.Valid {
			return "NULL"
		}
		return fmt.Sprintf("%d", val.Int64)
	case sql.NullFloat64:
		if !val.Valid {
			return "NULL"
		}
		return fmt.Sprintf("%g", val.Float64)
	case sql.NullBool:
		if !val.Valid {
			return "NULL"
		}
		if val.Bool {
			return "1"
		}
		return "0"
	case sql.NullTime:
		if !val.Valid {
			return "NULL"
		}
		return "'" + val.Time.Format("2006-01-02T15:04:05-07:00") + "'"
	default:
		return "'" + escapeSQLString(fmt.Sprintf("%v", val)) + "'"
	}
}

func escapeSQLString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
