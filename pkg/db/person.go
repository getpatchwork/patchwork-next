// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import "strings"

func (q *Queries) GetOrCreatePerson(email, name string) (*Person, error) {
	p := &Person{Email: strings.ToLower(email)}
	if name != "" {
		p.Name = &name
	}
	err := q.DB.NewInsert().Model(p).
		On("CONFLICT (email) DO UPDATE").
		Set("name = EXCLUDED.name").
		Returning("*").
		Scan(q.Ctx)
	return p, err
}
