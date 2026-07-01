// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"net/http"
	"strings"

	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/db"
)

func authMiddleware(database *bun.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			scheme, token, _ := strings.Cut(auth, " ")
			if strings.EqualFold(scheme, "bearer") ||
				strings.EqualFold(scheme, "token") {
				token = strings.TrimSpace(token)
				q := db.GetQueries(r.Context())
				user, err := q.GetUserByToken(token)
				if err == nil {
					r = r.WithContext(setAuthUser(r.Context(), user))
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
