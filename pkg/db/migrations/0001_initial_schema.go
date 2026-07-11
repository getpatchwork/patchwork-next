// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

// FROZEN MIGRATION — do not modify. Changes go in a new migration file.

package migrations

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/db"
)

func init() {
	Register(up0001, nil)
}

type user0001 struct {
	bun.BaseModel `bun:"table:auth_user"`
	ID            int        `bun:"id,pk,autoincrement"`
	Username      string     `bun:"username,notnull"`
	Password      string     `bun:"password,notnull"`
	FirstName     string     `bun:"first_name,notnull"`
	LastName      string     `bun:"last_name,notnull"`
	Email         string     `bun:"email,notnull"`
	IsAdmin       bool       `bun:"is_admin,notnull"`
	IsActive      bool       `bun:"is_active,notnull"`
	DateJoined    time.Time  `bun:"date_joined,notnull"`
	LastLogin     *time.Time `bun:"last_login"`
	SendEmail     bool       `bun:"send_email,notnull"`
	ItemsPerPage  int        `bun:"items_per_page,notnull"`
	ShowIds       bool       `bun:"show_ids,notnull"`
}

type authToken0001 struct {
	bun.BaseModel `bun:"table:auth_token"`
	Key           string    `bun:"key,pk"`
	Created       time.Time `bun:"created,notnull"`
	UserID        int       `bun:"user_id,notnull,unique" fk:"auth_user.id,cascade"`
}

type session0001 struct {
	bun.BaseModel `bun:"table:auth_session"`
	SessionKey    string    `bun:"session_key,pk"`
	UserID        int       `bun:"user_id,notnull" fk:"auth_user.id,cascade"`
	ExpireDate    time.Time `bun:"expire_date,notnull"`
}

type state0001 struct {
	bun.BaseModel  `bun:"table:state"`
	ID             int    `bun:"id,pk,autoincrement"`
	Name           string `bun:"name,notnull,unique"`
	Slug           string `bun:"slug,notnull,unique"`
	Ordering       int    `bun:"ordering,notnull,unique"`
	ActionRequired bool   `bun:"action_required,notnull"`
}

type tag0001 struct {
	bun.BaseModel `bun:"table:tag"`
	ID            int    `bun:"id,pk,autoincrement"`
	Name          string `bun:"name,notnull"`
	Pattern       string `bun:"pattern,notnull"`
	Abbrev        string `bun:"abbrev,notnull,unique"`
	ShowColumn    bool   `bun:"show_column,notnull"`
}

type project0001 struct {
	bun.BaseModel        `bun:"table:project" unique:"listid,subject_match"`
	ID                   int    `bun:"id,pk,autoincrement"`
	Linkname             string `bun:"linkname,notnull,unique"`
	Name                 string `bun:"name,notnull,unique"`
	Listid               string `bun:"listid,notnull"`
	Listemail            string `bun:"listemail,notnull"`
	SubjectMatch         string `bun:"subject_match,notnull"`
	WebURL               string `bun:"web_url,notnull"`
	ScmURL               string `bun:"scm_url,notnull"`
	WebScmURL            string `bun:"webscm_url,notnull"`
	ListArchiveURL       string `bun:"list_archive_url,notnull"`
	ListArchiveURLFormat string `bun:"list_archive_url_format,notnull"`
	CommitURLFormat      string `bun:"commit_url_format,notnull"`
	SendNotifications    bool   `bun:"send_notifications,notnull"`
	UseTags              bool   `bun:"use_tags,notnull"`
	ShowDependencies     bool   `bun:"show_dependencies,notnull"`
	AutoSupersede        bool   `bun:"auto_supersede,notnull"`
	NavHTML              string `bun:"nav_html"`
}

type projectMaintainer0001 struct {
	bun.BaseModel `bun:"table:project_maintainer" unique:"user_id,project_id"`
	ID            int `bun:"id,pk,autoincrement"`
	UserID        int `bun:"user_id,notnull" fk:"auth_user.id,cascade"`
	ProjectID     int `bun:"project_id,notnull" fk:"project.id,cascade"`
}

type delegationRule0001 struct {
	bun.BaseModel `bun:"table:delegation_rule" unique:"path,project_id"`
	ID            int    `bun:"id,pk,autoincrement"`
	Path          string `bun:"path,notnull"`
	Priority      int    `bun:"priority,notnull"`
	ProjectID     int    `bun:"project_id,notnull" fk:"project.id,cascade"`
	UserID        int    `bun:"user_id,notnull" fk:"auth_user.id,cascade"`
}

type person0001 struct {
	bun.BaseModel `bun:"table:person"`
	ID            int     `bun:"id,pk,autoincrement"`
	Email         string  `bun:"email,notnull,unique"`
	Name          *string `bun:"name"`
	UserID        *int    `bun:"user_id" fk:"auth_user.id,setnull"`
}

type patchRelation0001 struct {
	bun.BaseModel `bun:"table:patch_relation"`
	ID            int `bun:"id,pk,autoincrement"`
}

type series0001 struct {
	bun.BaseModel    `bun:"table:series"`
	ID               int       `bun:"id,pk,autoincrement"`
	ProjectID        *int      `bun:"project_id" fk:"project.id,cascade"`
	CoverLetterID    *int      `bun:"cover_letter_id" fk:"cover.id,setnull"`
	PreviousSeriesID *int      `bun:"previous_series_id" fk:"series.id,setnull"`
	Name             *string   `bun:"name"`
	Date             time.Time `bun:"date,notnull"`
	SubmitterID      int       `bun:"submitter_id,notnull" fk:"person.id"`
	Version          int       `bun:"version,notnull"`
	Total            int       `bun:"total,notnull"`
}

type seriesReference0001 struct {
	bun.BaseModel `bun:"table:series_reference" unique:"project_id,msgid"`
	ID            int    `bun:"id,pk,autoincrement"`
	Msgid         string `bun:"msgid,notnull"`
	ProjectID     int    `bun:"project_id,notnull" fk:"project.id,cascade"`
	SeriesID      int    `bun:"series_id,notnull" fk:"series.id,cascade"`
}

type seriesMetadata0001 struct {
	bun.BaseModel `bun:"table:series_metadata" unique:"series_id,key"`
	ID            int    `bun:"id,pk,autoincrement"`
	SeriesID      int    `bun:"series_id,notnull" fk:"series.id,cascade"`
	Key           string `bun:"key,notnull" index:""`
	Value         string `bun:"value,notnull"`
}

type seriesDependencies0001 struct {
	bun.BaseModel `bun:"table:series_dependencies" unique:"from_series_id,to_series_id"`
	ID            int `bun:"id,pk,autoincrement"`
	FromSeriesID  int `bun:"from_series_id,notnull" fk:"series.id,cascade"`
	ToSeriesID    int `bun:"to_series_id,notnull" fk:"series.id,cascade"`
}

type cover0001 struct {
	bun.BaseModel `bun:"table:cover" unique:"msgid,project_id" index:"date,project_id,submitter_id,name"`
	ID            int       `bun:"id,pk,autoincrement"`
	Msgid         string    `bun:"msgid,notnull"`
	Date          time.Time `bun:"date,notnull"`
	Headers       string    `bun:"headers,notnull"`
	SubmitterID   int       `bun:"submitter_id,notnull" fk:"person.id"`
	Content       *string   `bun:"content"`
	ProjectID     int       `bun:"project_id,notnull" fk:"project.id,cascade"`
	Name          string    `bun:"name,notnull"`
}

type patch0001 struct {
	bun.BaseModel `bun:"table:patch" unique:"msgid,project_id;series_id,number" index:"archived,state_id,delegate_id,date,project_id,submitter_id,name"`
	ID            int       `bun:"id,pk,autoincrement"`
	Msgid         string    `bun:"msgid,notnull"`
	Date          time.Time `bun:"date,notnull" index:""`
	Headers       string    `bun:"headers,notnull"`
	SubmitterID   int       `bun:"submitter_id,notnull" fk:"person.id"`
	Content       *string   `bun:"content"`
	ProjectID     int       `bun:"project_id,notnull" fk:"project.id,cascade"`
	Name          string    `bun:"name,notnull"`
	Diff          *string   `bun:"diff"`
	CommitRef     *string   `bun:"commit_ref"`
	PullURL       *string   `bun:"pull_url"`
	DelegateID    *int      `bun:"delegate_id" fk:"auth_user.id,setnull"`
	StateID       *int      `bun:"state_id" fk:"state.id"`
	Archived      bool      `bun:"archived,notnull"`
	Hash          *string   `bun:"hash" index:""`
	SeriesID      *int      `bun:"series_id" fk:"series.id,setnull"`
	Number        *int      `bun:"number"`
	RelatedID     *int      `bun:"related_id" fk:"patch_relation.id,setnull"`
}

type patchTag0001 struct {
	bun.BaseModel `bun:"table:patch_tag" unique:"patch_id,tag_id"`
	ID            int `bun:"id,pk,autoincrement"`
	PatchID       int `bun:"patch_id,notnull" fk:"patch.id,cascade"`
	TagID         int `bun:"tag_id,notnull" fk:"tag.id"`
	Count         int `bun:"count,notnull"`
}

type patchComment0001 struct {
	bun.BaseModel `bun:"table:patch_comment" unique:"msgid,patch_id" index:"patch_id,date"`
	ID            int       `bun:"id,pk,autoincrement"`
	Msgid         string    `bun:"msgid,notnull"`
	Date          time.Time `bun:"date,notnull"`
	Headers       string    `bun:"headers,notnull"`
	SubmitterID   int       `bun:"submitter_id,notnull" fk:"person.id"`
	Content       *string   `bun:"content"`
	PatchID       int       `bun:"patch_id,notnull" fk:"patch.id,cascade"`
	Addressed     *bool     `bun:"addressed"`
}

type coverComment0001 struct {
	bun.BaseModel `bun:"table:cover_comment" unique:"msgid,cover_id" index:"cover_id,date"`
	ID            int       `bun:"id,pk,autoincrement"`
	Msgid         string    `bun:"msgid,notnull"`
	Date          time.Time `bun:"date,notnull"`
	Headers       string    `bun:"headers,notnull"`
	SubmitterID   int       `bun:"submitter_id,notnull" fk:"person.id"`
	Content       *string   `bun:"content"`
	CoverID       int       `bun:"cover_id,notnull" fk:"cover.id,cascade"`
	Addressed     *bool     `bun:"addressed"`
}

type check0001 struct {
	bun.BaseModel `bun:"table:ci_check" unique:"patch_id,context,user_id"`
	ID            int       `bun:"id,pk,autoincrement"`
	PatchID       int       `bun:"patch_id,notnull" fk:"patch.id,cascade"`
	UserID        *int      `bun:"user_id" fk:"auth_user.id,setnull"`
	Date          time.Time `bun:"date,notnull"`
	State         int       `bun:"state,notnull"`
	TargetURL     string    `bun:"target_url,notnull"`
	Context       string    `bun:"context,notnull" index:""`
	Description   string    `bun:"description,notnull"`
}

type bundle0001 struct {
	bun.BaseModel `bun:"table:bundle" unique:"owner_id,name"`
	ID            int    `bun:"id,pk,autoincrement"`
	OwnerID       int    `bun:"owner_id,notnull" fk:"auth_user.id,cascade"`
	ProjectID     int    `bun:"project_id,notnull" fk:"project.id,cascade"`
	Name          string `bun:"name,notnull"`
	Public        bool   `bun:"public,notnull"`
}

type bundlePatch0001 struct {
	bun.BaseModel `bun:"table:bundle_patch" unique:"bundle_id,patch_id"`
	ID            int `bun:"id,pk,autoincrement"`
	BundleID      int `bun:"bundle_id,notnull" fk:"bundle.id,cascade"`
	PatchID       int `bun:"patch_id,notnull" fk:"patch.id,cascade"`
	Order         int `bun:"order,notnull"`
}

type event0001 struct {
	bun.BaseModel      `bun:"table:event" index:"project_id,category,date"`
	ID                 int       `bun:"id,pk,autoincrement"`
	ProjectID          int       `bun:"project_id,notnull" fk:"project.id,cascade"`
	Category           string    `bun:"category,notnull" index:""`
	Date               time.Time `bun:"date,notnull"`
	ActorID            *int      `bun:"actor_id" fk:"auth_user.id,setnull"`
	PatchID            *int      `bun:"patch_id" fk:"patch.id,cascade"`
	SeriesID           *int      `bun:"series_id" fk:"series.id,cascade"`
	CoverID            *int      `bun:"cover_id" fk:"cover.id,cascade"`
	PreviousStateID    *int      `bun:"previous_state_id" fk:"state.id"`
	CurrentStateID     *int      `bun:"current_state_id" fk:"state.id"`
	PreviousDelegateID *int      `bun:"previous_delegate_id" fk:"auth_user.id,setnull"`
	CurrentDelegateID  *int      `bun:"current_delegate_id" fk:"auth_user.id,setnull"`
	PreviousRelationID *int      `bun:"previous_relation_id" fk:"patch_relation.id,setnull"`
	CurrentRelationID  *int      `bun:"current_relation_id" fk:"patch_relation.id,setnull"`
	CreatedCheckID     *int      `bun:"created_check_id" fk:"ci_check.id,cascade"`
	CoverCommentID     *int      `bun:"cover_comment_id" fk:"cover_comment.id,cascade"`
	PatchCommentID     *int      `bun:"patch_comment_id" fk:"patch_comment.id,cascade"`
}

type emailConfirmation0001 struct {
	bun.BaseModel `bun:"table:email_confirmation"`
	ID            int       `bun:"id,pk,autoincrement"`
	Type          string    `bun:"type,notnull"`
	Email         string    `bun:"email,notnull"`
	UserID        *int      `bun:"user_id" fk:"auth_user.id,cascade"`
	Key           string    `bun:"key,notnull,unique"`
	Date          time.Time `bun:"date,notnull"`
	Active        bool      `bun:"active,notnull"`
}

type webhook0001 struct {
	bun.BaseModel `bun:"table:webhook" unique:"project_id,url"`
	ID            int       `bun:"id,pk,autoincrement"`
	ProjectID     int       `bun:"project_id,notnull" fk:"project.id,cascade"`
	URL           string    `bun:"url,notnull"`
	Secret        string    `bun:"secret,notnull"`
	Events        string    `bun:"events,notnull"`
	Active        bool      `bun:"active,notnull"`
	CreatorID     int       `bun:"creator_id,notnull" fk:"auth_user.id"`
	Created       time.Time `bun:"created,notnull"`
}

var tables0001 = []any{
	(*user0001)(nil),
	(*authToken0001)(nil),
	(*session0001)(nil),
	(*state0001)(nil),
	(*tag0001)(nil),
	(*project0001)(nil),
	(*projectMaintainer0001)(nil),
	(*delegationRule0001)(nil),
	(*person0001)(nil),
	(*patchRelation0001)(nil),
	(*cover0001)(nil),
	(*series0001)(nil),
	(*seriesReference0001)(nil),
	(*seriesMetadata0001)(nil),
	(*seriesDependencies0001)(nil),
	(*patch0001)(nil),
	(*patchTag0001)(nil),
	(*patchComment0001)(nil),
	(*coverComment0001)(nil),
	(*check0001)(nil),
	(*bundle0001)(nil),
	(*bundlePatch0001)(nil),
	(*event0001)(nil),
	(*emailConfirmation0001)(nil),
	(*webhook0001)(nil),
}

func up0001(ctx context.Context, tx bun.Tx) error {
	if err := db.CreateSchemaFrom(ctx, tx, tables0001); err != nil {
		return err
	}

	states := []state0001{
		{Name: "New", Slug: "new", Ordering: 0, ActionRequired: true},
		{Name: "Under Review", Slug: "under-review", Ordering: 1, ActionRequired: true},
		{Name: "Accepted", Slug: "accepted", Ordering: 2},
		{Name: "Rejected", Slug: "rejected", Ordering: 3},
		{Name: "RFC", Slug: "rfc", Ordering: 4},
		{Name: "Not Applicable", Slug: "not-applicable", Ordering: 5},
		{Name: "Changes Requested", Slug: "changes-requested", Ordering: 6},
		{Name: "Awaiting Upstream", Slug: "awaiting-upstream", Ordering: 7},
		{Name: "Superseded", Slug: "superseded", Ordering: 8},
		{Name: "Deferred", Slug: "deferred", Ordering: 9},
	}
	for i := range states {
		if _, err := tx.NewInsert().Model(&states[i]).
			On("CONFLICT DO NOTHING").Exec(ctx); err != nil {
			return fmt.Errorf("seed state: %w", err)
		}
	}

	tags := []tag0001{
		{Name: "Acked-by", Pattern: `^Acked-by:`, Abbrev: "A", ShowColumn: true},
		{Name: "Reviewed-by", Pattern: `^Reviewed-by:`, Abbrev: "R", ShowColumn: true},
		{Name: "Tested-by", Pattern: `^Tested-by:`, Abbrev: "T", ShowColumn: true},
	}
	for i := range tags {
		if _, err := tx.NewInsert().Model(&tags[i]).
			On("CONFLICT DO NOTHING").Exec(ctx); err != nil {
			return fmt.Errorf("seed tag: %w", err)
		}
	}

	return nil
}
