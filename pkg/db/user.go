// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

func (q *Queries) GetUserByUsername(username string) (*User, error) {
	var u User
	err := q.DB.NewSelect().Model(&u).
		Where("username = ?", username).
		Where("is_active = ?", true).
		Scan(q.Ctx)
	return &u, err
}

func (q *Queries) GetUserByEmail(email string) (*User, error) {
	var u User
	err := q.DB.NewSelect().Model(&u).
		Where("lower(email) = lower(?)", email).
		Where("is_active = ?", true).
		Scan(q.Ctx)
	return &u, err
}

func (q *Queries) GetUserByToken(token string) (*User, error) {
	var u User
	err := q.DB.NewSelect().Model(&u).
		Where("id = (SELECT user_id FROM auth_token WHERE key = ?)", token).
		Scan(q.Ctx)
	return &u, err
}
