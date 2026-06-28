// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/url"

	_ "github.com/go-sql-driver/mysql" // register mysql driver
	_ "github.com/jackc/pgx/v5/stdlib" // register pgx driver
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/schema"
	_ "modernc.org/sqlite" // register sqlite driver

	"github.com/getpatchwork/patchwork/pkg/config"
)

// Open connects to a database from a parsed URL. The scheme determines
// the driver and dialect:
//
//   - postgres:// postgresql:// pgx://  -> pgx + pgdialect
//   - mysql:// mariadb://               -> mysql + mysqldialect
//   - sqlite:// sqlite3://              -> sqlite + sqlitedialect
//
// The URL is rewritten to the format each driver expects.
func Open(cfg *config.Config) (*bun.DB, error) {
	var driver string
	var dsn string
	var dialect schema.Dialect

	u, err := url.Parse(cfg.Database.URL)
	if err != nil {
		return nil, err
	}

	switch u.Scheme {
	case "postgres", "postgresql", "pgx":
		// pgx accepts the standard postgres:// URL directly
		driver = "pgx"
		u.Scheme = "postgres"
		dsn = u.String()
		dialect = pgdialect.New()

	case "mysql", "mariadb":
		// go-sql-driver/mysql expects user:pass@tcp(host:port)/dbname
		host := u.Hostname()
		port := u.Port()
		if port == "" {
			port = "3306"
		}
		dbname := u.Path
		if len(dbname) > 0 && dbname[0] == '/' {
			dbname = dbname[1:]
		}
		userinfo := ""
		if u.User != nil {
			userinfo = u.User.String() + "@"
		}
		q := u.Query()
		q.Set("parseTime", "true")
		dsn = fmt.Sprintf("%stcp(%s:%s)/%s?%s",
			userinfo, host, port, dbname, q.Encode())
		driver = "mysql"
		dialect = mysqldialect.New()

	case "sqlite", "sqlite3":
		// modernc sqlite accepts file: URIs or plain paths.
		// url.Parse puts the path in Host for sqlite://foo.db
		// and in Path for sqlite:///tmp/foo.db.
		driver = "sqlite"
		path := u.Path
		if path == "" {
			path = u.Host
		}
		if path == ":memory:" {
			var buf [8]byte
			rand.Read(buf[:])
			name := hex.EncodeToString(buf[:])
			dsn = "file:" + name + "?mode=memory&cache=shared&_pragma=foreign_keys(1)"
		} else {
			q := u.Query()
			if !q.Has("_pragma") {
				q.Add("_pragma", "foreign_keys(1)")
				q.Add("_pragma", "journal_mode(WAL)")
				q.Add("_pragma", "busy_timeout(5000)")
			}
			dsn = fmt.Sprintf("file:%s?%s", path, q.Encode())
		}
		dialect = sqlitedialect.New()

	default:
		return nil, fmt.Errorf("unsupported database scheme %q", u.Scheme)
	}

	conn, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("sql.Open(%s): %w", driver, err)
	}

	return bun.NewDB(conn, dialect), nil
}
