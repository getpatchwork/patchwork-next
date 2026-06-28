// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db_test

import (
	"context"
	"strings"
	"testing"

	"github.com/uptrace/bun"
	_ "modernc.org/sqlite"

	"github.com/getpatchwork/patchwork/pkg/config"
	"github.com/getpatchwork/patchwork/pkg/db"
	"github.com/getpatchwork/patchwork/pkg/db/migrations"
)

// TestSchemaDrift detects when Go models diverge from what the
// migrations produce. It creates two in-memory SQLite databases — one
// via migrations, one via CreateSchema (which reads live model structs)
// — and compares the resulting table definitions.
//
// If this test fails, either:
//   - a model was changed without a corresponding migration, or
//   - a migration alters the schema without updating the model
func TestSchemaDrift(t *testing.T) {
	ctx := context.Background()

	migratedDB := openMemoryDB(t)
	if err := migrations.RunMigrations(ctx, migratedDB); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	freshDB := openMemoryDB(t)
	if err := db.CreateSchema(ctx, freshDB); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	migratedTables := dumpSchema(t, migratedDB)
	freshTables := dumpSchema(t, freshDB)

	// Compare table-by-table
	allTables := make(map[string]bool)
	for name := range migratedTables {
		allTables[name] = true
	}
	for name := range freshTables {
		allTables[name] = true
	}

	// Skip migration tracking tables
	delete(allTables, "bun_migrations")
	delete(allTables, "bun_migration_locks")
	delete(allTables, "schema_migrations")

	for name := range allTables {
		m, mOK := migratedTables[name]
		f, fOK := freshTables[name]

		if !mOK {
			t.Errorf("table %q exists in CreateSchema but not in migrations", name)
			continue
		}
		if !fOK {
			t.Errorf("table %q exists in migrations but not in CreateSchema", name)
			continue
		}
		if m != f {
			t.Errorf("table %q schema mismatch:\n  migrations: %s\n  models:     %s",
				name, m, f)
		}
	}

	// Compare indexes
	migratedIdx := dumpIndexes(t, migratedDB)
	freshIdx := dumpIndexes(t, freshDB)

	if len(migratedIdx) != len(freshIdx) {
		t.Errorf("index count mismatch: migrations=%d, models=%d",
			len(migratedIdx), len(freshIdx))
	}

	allIdx := make(map[string]bool)
	for name := range migratedIdx {
		allIdx[name] = true
	}
	for name := range freshIdx {
		allIdx[name] = true
	}

	for name := range allIdx {
		m, mOK := migratedIdx[name]
		f, fOK := freshIdx[name]

		if !mOK {
			t.Errorf("index %q exists in CreateSchema but not in migrations", name)
			continue
		}
		if !fOK {
			t.Errorf("index %q exists in migrations but not in CreateSchema", name)
			continue
		}
		if m != f {
			t.Errorf("index %q mismatch:\n  migrations: %s\n  models:     %s",
				name, m, f)
		}
	}
}

func openMemoryDB(t *testing.T) *bun.DB {
	t.Helper()

	var cfg config.Config
	cfg.Database.URL = "sqlite://:memory:"
	database, err := db.Open(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func dumpSchema(t *testing.T, database *bun.DB) map[string]string {
	t.Helper()
	rows, err := database.QueryContext(context.Background(),
		`SELECT name, sql FROM sqlite_master WHERE type='table' AND sql IS NOT NULL ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	tables := make(map[string]string)
	for rows.Next() {
		var name, ddl string
		if err := rows.Scan(&name, &ddl); err != nil {
			t.Fatal(err)
		}
		tables[name] = normalizeDDL(ddl)
	}
	return tables
}

func dumpIndexes(t *testing.T, database *bun.DB) map[string]string {
	t.Helper()
	rows, err := database.QueryContext(context.Background(),
		`SELECT name, sql FROM sqlite_master WHERE type='index' AND sql IS NOT NULL ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	indexes := make(map[string]string)
	for rows.Next() {
		var name, ddl string
		if err := rows.Scan(&name, &ddl); err != nil {
			t.Fatal(err)
		}
		// Skip sqlite autoindex entries
		if strings.HasPrefix(name, "sqlite_") {
			continue
		}
		indexes[name] = normalizeDDL(ddl)
	}
	return indexes
}

// normalizeDDL strips whitespace variations so trivial formatting
// differences don't cause false positives.
func normalizeDDL(ddl string) string {
	return strings.Join(strings.Fields(ddl), " ")
}
