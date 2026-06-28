// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import (
	"context"
	"log"
	"net/http"

	"github.com/uptrace/bun"
)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

// Middleware returns an HTTP middleware that creates a *Queries and
// stores it in the request context. GET/HEAD requests get a plain
// connection; all other methods get a transaction that is committed on
// success (2xx/3xx status) or rolled back otherwise.
func Middleware(database *bun.DB, bus EventBus) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			ctx = WithBus(ctx, bus)
			if r.Method == http.MethodGet || r.Method == http.MethodHead {
				q := &Queries{Ctx: ctx, DB: database, Events: bus}
				ctx = context.WithValue(ctx, queriesKey{}, q)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			q, err := Begin(ctx, database)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			ctx = context.WithValue(ctx, queriesKey{}, q)
			rw := &statusWriter{ResponseWriter: w}
			next.ServeHTTP(rw, r.WithContext(ctx))
			if rw.status >= 400 {
				if err := q.Rollback(); err != nil {
					log.Printf("rollback: %v", err)
				}
				return
			}
			if err := q.Commit(); err != nil {
				log.Printf("commit: %v", err)
			}
		})
	}
}

type queriesKey struct{}

// GetQueries retrieves the *Queries stored by the middleware. It panics
// if no Queries is found (meaning the middleware is not installed).
func GetQueries(ctx context.Context) *Queries {
	q, ok := ctx.Value(queriesKey{}).(*Queries)
	if !ok {
		panic("no Queries in context (middleware not installed?)")
	}
	return q
}
