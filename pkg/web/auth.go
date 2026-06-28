// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package web

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/getpatchwork/patchwork/pkg/db"
	"github.com/getpatchwork/patchwork/pkg/events"
	"github.com/getpatchwork/patchwork/pkg/log"
)

var usernameRe = regexp.MustCompile(`^\w+$`)

func (h *webHandler) RegisterPage(w http.ResponseWriter, r *http.Request) {
	_ = registerPage(h.pageCtx(r), nil).Render(r.Context(), w)
}

func (h *webHandler) RegisterSubmit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := db.GetQueries(ctx)
	pc := h.pageCtx(r)
	err := r.ParseForm()

	if err != nil || !h.validateCSRF(r) {
		_ = registerPage(pc, []string{"Invalid request. Please try again."}).Render(ctx, w)
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	firstName := strings.TrimSpace(r.FormValue("first_name"))
	lastName := strings.TrimSpace(r.FormValue("last_name"))

	var errors []string
	if username == "" || !usernameRe.MatchString(username) {
		errors = append(errors, "Username must contain only letters, digits, and underscores.")
	}
	if email == "" || !strings.Contains(email, "@") {
		errors = append(errors, "Enter a valid email address.")
	}
	if password == "" {
		errors = append(errors, "Password is required.")
	}
	if len(username) > 30 {
		errors = append(errors, "Username must be 30 characters or fewer.")
	}

	if len(errors) == 0 {
		var count int
		count, _ = q.DB.NewSelect().Model((*db.User)(nil)).
			Where("LOWER(username) = LOWER(?)", username).
			Count(q.Ctx)
		if count > 0 {
			errors = append(errors, "A user with that username already exists.")
		}
	}
	if len(errors) == 0 {
		var count int
		count, _ = q.DB.NewSelect().Model((*db.User)(nil)).
			Where("LOWER(email) = LOWER(?)", email).
			Count(q.Ctx)
		if count > 0 {
			errors = append(errors, "A user with that email already exists.")
		}
	}

	if len(errors) > 0 {
		w.WriteHeader(http.StatusBadRequest)
		_ = registerPage(pc, errors).Render(ctx, w)
		return
	}

	user := db.User{
		Username:   username,
		Email:      email,
		Password:   db.HashPassword(password),
		FirstName:  firstName,
		LastName:   lastName,
		IsActive:   false,
		DateJoined: time.Now(),
	}
	err = q.Insert(&user)
	if err != nil {
		log.Errorf("register: insert user: %s", err)
		_ = registerPage(pc, []string{"Registration failed. Please try again."}).Render(ctx, w)
		return
	}

	conf, err := q.CreateEmailConfirmation("registration", email, &user.ID)
	if err != nil {
		log.Errorf("register: create confirmation: %s", err)
		_ = registerPage(pc, []string{"Registration failed. Please try again."}).Render(ctx, w)
		return
	}

	baseURL := h.cfg.Http.BaseURL
	if baseURL == "" {
		baseURL = "http://" + r.Host
	}
	link := fmt.Sprintf("%s/confirm/%s/", baseURL, conf.Key)
	body := fmt.Sprintf(
		"Please click the following link to confirm your registration:\n\n%s\n\nThis link will expire in 7 days.\n",
		link,
	)

	go func() {
		err = events.SendEmail(&h.cfg.SMTP, email, "Patchwork registration confirmation", body, nil)
		if err != nil {
			log.Errorf("register: send email: %s", err)
		}
	}()

	_ = registerConfirmSentPage(pc, email).Render(ctx, w)
}

func (h *webHandler) ConfirmHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := db.GetQueries(ctx)
	pc := h.pageCtx(r)
	key := chi.URLParam(r, "key")

	var conf db.EmailConfirmation
	err := q.DB.NewSelect().Model(&conf).
		Where("key = ?", key).
		Scan(q.Ctx)
	if err != nil {
		confirmResultPage(pc, "Confirmation not found.").Render(ctx, w)
		return
	}

	if !conf.IsValid() {
		msg := "This confirmation has expired."
		if !conf.Active {
			msg = "This confirmation has already been used."
		}
		confirmResultPage(pc, msg).Render(ctx, w)
		return
	}

	switch conf.Type {
	case "registration":
		h.confirmRegistration(w, r, q, &conf, pc)
	case "userperson":
		h.confirmLink(w, r, q, &conf, pc)
	default:
		confirmResultPage(pc, "Unknown confirmation type.").Render(ctx, w)
	}
}

func (h *webHandler) confirmRegistration(w http.ResponseWriter, r *http.Request, q *db.Queries, conf *db.EmailConfirmation, pc pageContext) {
	ctx := r.Context()

	if conf.UserID == nil {
		confirmResultPage(pc, "Invalid confirmation.").Render(ctx, w)
		return
	}

	_, err := q.DB.NewUpdate().Model((*db.User)(nil)).
		Where("id = ?", *conf.UserID).
		Set("is_active = ?", true).
		Exec(q.Ctx)
	if err != nil {
		log.Errorf("confirm registration: activate user: %s", err)
		confirmResultPage(pc, "Confirmation failed.").Render(ctx, w)
		return
	}

	var person db.Person
	err = q.DB.NewSelect().Model(&person).
		Where("LOWER(email) = LOWER(?)", conf.Email).
		Scan(q.Ctx)
	if err != nil {
		person = db.Person{Email: conf.Email}
		if _, err := q.DB.NewInsert().Model(&person).Exec(q.Ctx); err != nil {
			confirmResultPage(pc, "Confirmation failed.").Render(ctx, w)
			return
		}
	}
	if _, err := q.DB.NewUpdate().Model(&person).
		Where("id = ?", person.ID).
		Set("user_id = ?", *conf.UserID).
		Exec(q.Ctx); err != nil {
		confirmResultPage(pc, "Confirmation failed.").Render(ctx, w)
		return
	}

	_, _ = q.DB.NewUpdate().Model(conf).
		Where("id = ?", conf.ID).
		Set("active = ?", false).
		Exec(q.Ctx)

	confirmResultPage(pc, "Your registration has been confirmed. You can now log in.").Render(ctx, w)
}

func (h *webHandler) confirmLink(w http.ResponseWriter, r *http.Request, q *db.Queries, conf *db.EmailConfirmation, pc pageContext) {
	ctx := r.Context()

	if conf.UserID == nil {
		confirmResultPage(pc, "Invalid confirmation.").Render(ctx, w)
		return
	}

	var person db.Person
	err := q.DB.NewSelect().Model(&person).
		Where("LOWER(email) = LOWER(?)", conf.Email).
		Scan(q.Ctx)
	if err != nil {
		person = db.Person{Email: conf.Email}
		if _, err := q.DB.NewInsert().Model(&person).Exec(q.Ctx); err != nil {
			confirmResultPage(pc, "Confirmation failed.").Render(ctx, w)
			return
		}
	}
	if _, err := q.DB.NewUpdate().Model(&person).
		Where("id = ?", person.ID).
		Set("user_id = ?", *conf.UserID).
		Exec(q.Ctx); err != nil {
		confirmResultPage(pc, "Confirmation failed.").Render(ctx, w)
		return
	}

	_, _ = q.DB.NewUpdate().Model(conf).
		Where("id = ?", conf.ID).
		Set("active = ?", false).
		Exec(q.Ctx)

	confirmResultPage(pc, "Your email address has been linked to your account.").Render(ctx, w)
}
