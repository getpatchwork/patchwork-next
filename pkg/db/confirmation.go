// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import (
	"crypto/rand"
	"crypto/sha1"
	"fmt"
	"math/big"
	"time"
)

const confirmationValidityDays = 7

func (c *EmailConfirmation) IsValid() bool {
	return c.Active && time.Since(c.Date) < confirmationValidityDays*24*time.Hour
}

func (q *Queries) CreateEmailConfirmation(confType, email string, userID *int) (*EmailConfirmation, error) {
	n, _ := rand.Int(rand.Reader, big.NewInt(1<<31))
	raw := fmt.Sprintf("%v%s%d", userID, email, n.Int64())
	key := fmt.Sprintf("%x", sha1.Sum([]byte(raw)))

	conf := &EmailConfirmation{
		Type:   confType,
		Email:  email,
		UserID: userID,
		Key:    key,
		Date:   time.Now(),
		Active: true,
	}
	err := q.DB.NewInsert().Model(conf).Scan(q.Ctx)
	if err != nil {
		return nil, err
	}
	return conf, nil
}

func (q *Queries) CleanExpiredConfirmations() (int64, error) {
	cutoff := time.Now().Add(-confirmationValidityDays * 24 * time.Hour)
	res, err := q.DB.NewDelete().Model((*EmailConfirmation)(nil)).
		Where("date < ? OR active = ?", cutoff, false).
		Exec(q.Ctx)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (q *Queries) CleanInactiveUsers() (int64, error) {
	res, err := q.DB.NewDelete().Model((*User)(nil)).
		Where("is_active = ?", false).
		Where(
			"id NOT IN ?",
			q.DB.NewSelect().Model((*EmailConfirmation)(nil)).
				Column("user_id").
				Where("user_id IS NOT NULL"),
		).
		Exec(q.Ctx)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}
