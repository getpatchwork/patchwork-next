// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package migrations

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"time"

	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/db"
)

type MigrationFunc func(ctx context.Context, tx bun.Tx) error

type Migration struct {
	Num  int
	Name string
	Up   MigrationFunc
	Down MigrationFunc
}

var registered []Migration

var numRe = regexp.MustCompile(`^\d+`)

func Register(up, down MigrationFunc) {
	_, file, _, ok := runtime.Caller(1)
	if !ok {
		panic("migrations: cannot determine caller file")
	}
	name := filepath.Base(file)
	name = name[:len(name)-len(filepath.Ext(name))]

	m := numRe.FindString(name)
	if m == "" {
		panic(fmt.Sprintf("migrations: %q: no leading number", name))
	}
	num, err := strconv.Atoi(m)
	if err != nil {
		panic(fmt.Sprintf("migrations: %q: %v", name, err))
	}

	for _, existing := range registered {
		if existing.Num == num {
			panic(fmt.Sprintf(
				"migrations: duplicate number %d (%q and %q)",
				num, existing.Name, name,
			))
		}
	}

	registered = append(registered, Migration{
		Num: num, Name: name, Up: up, Down: down,
	})
}

type schemaMigration struct {
	bun.BaseModel `bun:"table:schema_migrations"`
	Num           int       `bun:"num,pk"`
	Name          string    `bun:"name,notnull"`
	AppliedAt     time.Time `bun:"applied_at,notnull"`
}

func ensureTable(ctx context.Context, database bun.IDB) error {
	_, err := database.NewCreateTable().
		Model((*schemaMigration)(nil)).
		IfNotExists().
		Exec(ctx)
	return err
}

func lastApplied(ctx context.Context, database *bun.DB) (int, error) {
	var num int
	err := database.NewSelect().Model((*schemaMigration)(nil)).
		ColumnExpr("COALESCE(MAX(num), 0)").
		Scan(ctx, &num)
	return num, err
}

func sorted() []Migration {
	out := make([]Migration, len(registered))
	copy(out, registered)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Num < out[j].Num
	})
	return out
}

func RunMigrations(ctx context.Context, database *bun.DB) error {
	if !tableExists(ctx, database, "schema_migrations") {
		return bootstrap(ctx, database)
	}

	if err := ensureTable(ctx, database); err != nil {
		return fmt.Errorf("ensure migration table: %w", err)
	}

	current, err := lastApplied(ctx, database)
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	for _, m := range sorted() {
		if m.Num <= current {
			continue
		}
		if m.Up == nil {
			continue
		}

		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("%s: begin tx: %w", m.Name, err)
		}

		if err := m.Up(ctx, tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("%s: %w", m.Name, err)
		}

		if _, err := tx.NewInsert().Model(&schemaMigration{
			Num:       m.Num,
			Name:      m.Name,
			AppliedAt: time.Now(),
		}).Exec(ctx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("%s: record migration: %w", m.Name, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("%s: commit: %w", m.Name, err)
		}
	}

	return nil
}

func Rollback(ctx context.Context, database *bun.DB) error {
	if err := ensureTable(ctx, database); err != nil {
		return fmt.Errorf("ensure migration table: %w", err)
	}

	current, err := lastApplied(ctx, database)
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if current == 0 {
		return fmt.Errorf("no migrations to roll back")
	}

	var last *Migration
	for _, m := range sorted() {
		if m.Num == current {
			last = &m
			break
		}
	}
	if last == nil {
		return fmt.Errorf("migration %d not found in registry", current)
	}
	if last.Down == nil {
		return fmt.Errorf("%s: no down migration", last.Name)
	}

	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("%s: begin tx: %w", last.Name, err)
	}

	if err := last.Down(ctx, tx); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("%s: %w", last.Name, err)
	}

	if _, err := tx.NewDelete().Model((*schemaMigration)(nil)).
		Where("num = ?", last.Num).Exec(ctx); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("%s: remove record: %w", last.Name, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%s: commit: %w", last.Name, err)
	}

	return nil
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

func bootstrap(ctx context.Context, database *bun.DB) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if err := db.CreateSchema(ctx, tx); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	if err := db.SeedDefaults(ctx, tx); err != nil {
		return fmt.Errorf("seed defaults: %w", err)
	}

	if err := ensureTable(ctx, tx); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}

	for _, m := range sorted() {
		if _, err := tx.NewInsert().Model(&schemaMigration{
			Num:       m.Num,
			Name:      m.Name,
			AppliedAt: time.Now(),
		}).Exec(ctx); err != nil {
			return fmt.Errorf("mark migration %s: %w", m.Name, err)
		}
	}

	return tx.Commit()
}

func CheckSchemaVersion(ctx context.Context, database *bun.DB) error {
	if err := ensureTable(ctx, database); err != nil {
		return fmt.Errorf("ensure migration table: %w", err)
	}

	current, err := lastApplied(ctx, database)
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	var pending int
	for _, m := range sorted() {
		if m.Num > current {
			pending++
		}
	}

	if pending > 0 {
		return fmt.Errorf(
			"database schema is out of date (%d pending), run 'pw db sync'",
			pending,
		)
	}

	return nil
}
