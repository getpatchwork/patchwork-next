// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"strings"

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
		// Go 1.26.0's url.Parse rejects the ":memory:" authority of
		// "sqlite://:memory:" as an invalid port. Forge the URL by
		// hand for sqlite DSNs so it works regardless of the Go
		// version.
		u, err = parseSqliteURL(cfg.Database.URL, err)
		if err != nil {
			return nil, err
		}
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
			if _, err := rand.Read(buf[:]); err != nil {
				return nil, err
			}
			name := hex.EncodeToString(buf[:])
			dsn = "file:" + name + "?mode=memory&cache=shared&_pragma=foreign_keys(1)"
		} else {
			// The database holds password hashes, API tokens and
			// sessions. Make sure the file is not world-readable by
			// creating it (or tightening it) with owner-only perms
			// before the driver opens it.
			if err := restrictDBFile(path); err != nil {
				return nil, err
			}
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

// parseSqliteURL rebuilds a URL for sqlite DSNs that url.Parse rejects,
// mirroring how a lenient url.Parse fills the fields. It only handles the
// sqlite and sqlite3 schemes; other URLs keep the original parse error.
func parseSqliteURL(rawURL string, parseErr error) (*url.URL, error) {
	scheme, rest, ok := strings.Cut(rawURL, "://")
	if !ok || (scheme != "sqlite" && scheme != "sqlite3") {
		return nil, parseErr
	}
	u := &url.URL{Scheme: scheme}
	if path, query, ok := strings.Cut(rest, "?"); ok {
		rest = path
		u.RawQuery = query
	}
	// url.Parse puts a relative path in Host and an absolute one in Path.
	if strings.HasPrefix(rest, "/") {
		u.Path = rest
	} else {
		u.Host = rest
	}
	return u, nil
}

// restrictDBFile ensures the sqlite database file exists with
// owner-only (0600) permissions before the driver opens it. Existing
// files created with a laxer umask are tightened as well. The companion
// -wal and -shm files sqlite may create inherit these permissions.
func restrictDBFile(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	_ = f.Close()
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("chmod %s: %w", path, err)
	}
	return nil
}

// EventBus is the interface satisfied by *events.Bus. Defined here to
// avoid a circular import between pkg/db and pkg/events.
type EventBus interface {
	Enqueue(*Event)
}

type busCtxKey struct{}

// WithBus stores an EventBus in the context.
func WithBus(ctx context.Context, bus EventBus) context.Context {
	return context.WithValue(ctx, busCtxKey{}, bus)
}

func GetBus(ctx context.Context) EventBus {
	bus, _ := ctx.Value(busCtxKey{}).(EventBus)
	return bus
}

// ProcessEvent is a fallback handler for synchronous event processing.
// It is called by Commit when no EventBus is available (e.g. in CLI
// commands that don't start the full async event bus). Set by
// pkg/events.init() to provide the full pipeline (persist + webhooks).
var ProcessEvent func(ctx context.Context, database *bun.DB, e *Event)

// Queries provides typed database access methods. It wraps a bun.IDB
// (either *bun.DB or bun.Tx) and the context for query execution.
type Queries struct {
	Ctx           context.Context
	DB            bun.IDB
	Events        EventBus
	database      *bun.DB
	pendingEvents []Event
}

// EnqueueEvent buffers an event to be sent to the bus after the
// transaction commits successfully. If no transaction is active, the
// event is sent immediately.
func (q *Queries) EnqueueEvent(e Event) {
	if _, ok := q.DB.(bun.Tx); ok {
		q.pendingEvents = append(q.pendingEvents, e)
		return
	}
	if q.Events != nil {
		q.Events.Enqueue(&e)
	} else if ProcessEvent != nil && q.database != nil {
		ProcessEvent(q.Ctx, q.database, &e)
	}
}

// New creates a Queries handle without a transaction.
func New(ctx context.Context, database bun.IDB) *Queries {
	return &Queries{Ctx: ctx, DB: database}
}

// Begin starts a transaction and returns a Queries handle. If an
// EventBus is stored in the context (via WithBus), it is propagated.
func Begin(ctx context.Context, database *bun.DB) (*Queries, error) {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &Queries{
		Ctx:      ctx,
		DB:       tx,
		Events:   GetBus(ctx),
		database: database,
	}, nil
}

func (q *Queries) Commit() error {
	if tx, ok := q.DB.(bun.Tx); ok {
		if err := tx.Commit(); err != nil {
			q.pendingEvents = nil
			return err
		}
	}
	for _, e := range q.pendingEvents {
		if q.Events != nil {
			q.Events.Enqueue(&e)
		} else if ProcessEvent != nil && q.database != nil {
			ProcessEvent(q.Ctx, q.database, &e)
		}
	}
	q.pendingEvents = nil
	return nil
}

func (q *Queries) Rollback() error {
	q.pendingEvents = nil
	if tx, ok := q.DB.(bun.Tx); ok {
		return tx.Rollback()
	}
	return nil
}

func (q *Queries) Insert(model any) error {
	query := q.DB.NewInsert().Model(model)

	typ := reflect.TypeOf(model)
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() == reflect.Struct {
		for i := range typ.NumField() {
			tag := typ.Field(i).Tag.Get("bun")
			if strings.Contains(tag, "autoincrement") {
				col, _, _ := strings.Cut(tag, ",")
				query = query.ExcludeColumn(col)
			}
		}
	}

	return query.Returning("*").Scan(q.Ctx)
}

func (q *Queries) Select(model any) *bun.SelectQuery {
	return q.DB.NewSelect().Model(model)
}

func (q *Queries) Update(model any) *bun.UpdateQuery {
	return q.DB.NewUpdate().Model(model)
}

func (q *Queries) Delete(model any) *bun.DeleteQuery {
	return q.DB.NewDelete().Model(model)
}

// IsUniqueViolation reports whether err is a unique constraint violation.
// The check is done on the driver message because each backend reports it
// differently (sqlite: "UNIQUE constraint failed", postgres: "duplicate
// key value violates unique constraint", mysql: "Duplicate entry").
func IsUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "duplicate entry")
}
