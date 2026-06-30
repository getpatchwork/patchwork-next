// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/log"
)

// Schema is declared from struct tags:
//
//   bun.BaseModel `bun:"table:foo" unique:"col1,col2;col3,col4" index:"col1,col2"`
//
//   FieldA int `bun:"field_a,notnull" fk:"other_table.id,cascade" index:""`
//
// fk:"table.column"           - foreign key, no action
// fk:"table.column,cascade"   - foreign key ON DELETE CASCADE
// fk:"table.column,setnull"   - foreign key ON DELETE SET NULL
// index:"" on a field          - single column index
// index:"col1,col2" on BaseModel - composite index
// unique:"col1,col2" on BaseModel - composite unique index
// Multiple constraints separated by semicolons.

type tableFK struct {
	column    string
	refTable  string
	refColumn string
	action    string
}

func (k *tableFK) add(q *bun.CreateTableQuery) {
	q.ForeignKey("(?) REFERENCES ? (?) "+k.action,
		bun.Ident(k.column), bun.Ident(k.refTable), bun.Ident(k.refColumn))
}

type tableIndex struct {
	columns []string
	unique  bool
}

func (idx *tableIndex) create(model any, ctx context.Context, database bun.IDB) error {
	c := database.NewCreateIndex().Model(model).Column(idx.columns...)
	var tokens []string
	if idx.unique {
		c = c.Unique()
		tokens = append(tokens, "uidx")
	} else {
		tokens = append(tokens, "idx")
	}
	tokens = append(tokens, c.GetTableName())
	tokens = append(tokens, idx.columns...)
	name := strings.Join(tokens, "_")
	_, err := c.Index(name).Exec(ctx)
	return err
}

func parseFKTag(bunCol, tag string) tableFK {
	parts := strings.SplitN(tag, ",", 2)
	ref := strings.SplitN(parts[0], ".", 2)
	action := ""
	if len(parts) > 1 {
		switch parts[1] {
		case "cascade":
			action = "ON DELETE CASCADE"
		case "setnull":
			action = "ON DELETE SET NULL"
		}
	}
	return tableFK{
		column:    bunCol,
		refTable:  ref[0],
		refColumn: ref[1],
		action:    action,
	}
}

func bunColumnName(field reflect.StructField) string {
	tag := field.Tag.Get("bun")
	if tag == "" || tag == "-" {
		return ""
	}
	name := strings.SplitN(tag, ",", 2)[0]
	if name == "" {
		return ""
	}
	return name
}

func parseModelTags(model any) (fks []tableFK, indexes []tableIndex) {
	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// BaseModel field carries table-level unique/index tags
		if field.Anonymous && field.Type == reflect.TypeOf(bun.BaseModel{}) {
			if tag := field.Tag.Get("unique"); tag != "" {
				for _, entry := range strings.Split(tag, ";") {
					entry = strings.TrimSpace(entry)
					if entry != "" {
						indexes = append(indexes, tableIndex{
							columns: strings.Split(entry, ","),
							unique:  true,
						})
					}
				}
			}
			if tag := field.Tag.Get("index"); tag != "" {
				for _, entry := range strings.Split(tag, ";") {
					entry = strings.TrimSpace(entry)
					if entry != "" {
						indexes = append(indexes, tableIndex{
							columns: strings.Split(entry, ","),
							unique:  false,
						})
					}
				}
			}
			continue
		}

		col := bunColumnName(field)
		if col == "" {
			continue
		}

		if tag := field.Tag.Get("fk"); tag != "" {
			fks = append(fks, parseFKTag(col, tag))
		}

		if _, ok := field.Tag.Lookup("index"); ok {
			indexes = append(indexes, tableIndex{
				columns: []string{col},
				unique:  false,
			})
		}
	}

	return fks, indexes
}

func CreateSchema(ctx context.Context, database bun.IDB) error {
	tables := []any{
		(*User)(nil),
		(*AuthToken)(nil),
		(*Session)(nil),
		(*State)(nil),
		(*Tag)(nil),
		(*Project)(nil),
		(*ProjectMaintainer)(nil),
		(*DelegationRule)(nil),
		(*Person)(nil),
		(*PatchRelation)(nil),
		(*Cover)(nil),
		(*Series)(nil),
		(*SeriesReference)(nil),
		(*SeriesMetadata)(nil),
		(*SeriesDependencies)(nil),
		(*Patch)(nil),
		(*PatchTag)(nil),
		(*PatchComment)(nil),
		(*CoverComment)(nil),
		(*Check)(nil),
		(*Bundle)(nil),
		(*BundlePatch)(nil),
		(*Event)(nil),
		(*EmailConfirmation)(nil),
		(*Webhook)(nil),
	}
	return CreateSchemaFrom(ctx, database, tables)
}

func CreateSchemaFrom(ctx context.Context, database bun.IDB, schema []any) error {
	for _, model := range schema {
		fks, indexes := parseModelTags(model)

		q := database.NewCreateTable().Model(model)
		for _, fk := range fks {
			fk.add(q)
		}
		log.Noticef("creating table %q", q.GetTableName())
		if _, err := q.Exec(ctx); err != nil {
			return fmt.Errorf("create table %s: %w", q.GetTableName(), err)
		}

		// implicit index for each FK column
		for _, fk := range fks {
			idx := tableIndex{columns: []string{fk.column}}
			if err := idx.create(model, ctx, database); err != nil {
				return fmt.Errorf("create fk index: %w", err)
			}
		}

		for _, idx := range indexes {
			if err := idx.create(model, ctx, database); err != nil {
				return fmt.Errorf("create index: %w", err)
			}
		}
	}
	return nil
}
