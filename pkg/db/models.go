// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package db

import (
	"encoding/json"
	"time"

	"github.com/uptrace/bun"
)

type User struct {
	bun.BaseModel `bun:"table:auth_user" json:"-"`

	ID           int        `bun:"id,pk,autoincrement" json:"id"`
	Username     string     `bun:"username,notnull" json:"username"`
	Password     string     `bun:"password,notnull" json:"-"`
	FirstName    string     `bun:"first_name,notnull" json:"first_name"`
	LastName     string     `bun:"last_name,notnull" json:"last_name"`
	Email        string     `bun:"email,notnull" json:"email"`
	IsAdmin      bool       `bun:"is_admin,notnull" json:"-"`
	IsActive     bool       `bun:"is_active,notnull" json:"-"`
	DateJoined   time.Time  `bun:"date_joined,notnull" json:"-"`
	LastLogin    *time.Time `bun:"last_login" json:"-"`
	SendEmail    bool       `bun:"send_email,notnull" json:"-"`
	ItemsPerPage int        `bun:"items_per_page,notnull" json:"-"`
	ShowIds      bool       `bun:"show_ids,notnull" json:"-"`

	URL string `bun:"-" json:"url,omitempty"`
}

type AuthToken struct {
	bun.BaseModel `bun:"table:auth_token" json:"-"`

	Key     string    `bun:"key,pk"`
	Created time.Time `bun:"created,notnull"`
	UserID  int       `bun:"user_id,notnull,unique" fk:"auth_user.id,cascade"`
}

type Person struct {
	bun.BaseModel `bun:"table:person" json:"-"`

	ID     int     `bun:"id,pk,autoincrement" json:"id"`
	Email  string  `bun:"email,notnull,unique" json:"email"`
	Name   *string `bun:"name" json:"name"`
	UserID *int    `bun:"user_id" json:"-" fk:"auth_user.id,setnull"`

	User *User  `bun:"rel:belongs-to,join:user_id=id" json:"user,omitempty"`
	URL  string `bun:"-" json:"url,omitempty"`
}

type Session struct {
	bun.BaseModel `bun:"table:auth_session"`

	SessionKey string    `bun:"session_key,pk"`
	UserID     int       `bun:"user_id,notnull" fk:"auth_user.id,cascade"`
	ExpireDate time.Time `bun:"expire_date,notnull"`
}

type EmailConfirmation struct {
	bun.BaseModel `bun:"table:email_confirmation"`

	ID     int       `bun:"id,pk,autoincrement"`
	Type   string    `bun:"type,notnull"`
	Email  string    `bun:"email,notnull"`
	UserID *int      `bun:"user_id" fk:"auth_user.id,cascade"`
	Key    string    `bun:"key,notnull,unique"`
	Date   time.Time `bun:"date,notnull"`
	Active bool      `bun:"active,notnull"`
}

type Project struct {
	bun.BaseModel `bun:"table:project" unique:"listid,subject_match" json:"-"`

	ID                   int    `bun:"id,pk,autoincrement" json:"id"`
	Linkname             string `bun:"linkname,notnull,unique" json:"link_name"`
	Name                 string `bun:"name,notnull,unique" json:"name"`
	Listid               string `bun:"listid,notnull" json:"list_id"`
	Listemail            string `bun:"listemail,notnull" json:"list_email"`
	SubjectMatch         string `bun:"subject_match,notnull" json:"subject_match,omitempty"`
	WebURL               string `bun:"web_url,notnull" json:"web_url,omitempty"`
	ScmURL               string `bun:"scm_url,notnull" json:"scm_url,omitempty"`
	WebScmURL            string `bun:"webscm_url,notnull" json:"webscm_url,omitempty"`
	ListArchiveURL       string `bun:"list_archive_url,notnull" json:"list_archive_url,omitempty"`
	ListArchiveURLFormat string `bun:"list_archive_url_format,notnull" json:"list_archive_url_format,omitempty"`
	CommitURLFormat      string `bun:"commit_url_format,notnull" json:"commit_url_format,omitempty"`
	SendNotifications    bool   `bun:"send_notifications,notnull" json:"-"`
	UseTags              bool   `bun:"use_tags,notnull" json:"-"`
	ShowDependencies     bool   `bun:"show_dependencies,notnull" json:"show_dependencies,omitempty"`
	AutoSupersede        bool   `bun:"auto_supersede,notnull" json:"-"`
	NavHTML              string `bun:"nav_html" json:"-"`

	APIURL      string `bun:"-" json:"url,omitempty"`
	Maintainers []User `bun:"-" json:"maintainers"`
}

type ProjectMaintainer struct {
	bun.BaseModel `bun:"table:project_maintainer" unique:"user_id,project_id" json:"-"`

	ID        int `bun:"id,pk,autoincrement"`
	UserID    int `bun:"user_id,notnull" fk:"auth_user.id,cascade"`
	ProjectID int `bun:"project_id,notnull" fk:"project.id,cascade"`

	User    *User    `bun:"rel:belongs-to,join:user_id=id" json:"-"`
	Project *Project `bun:"rel:belongs-to,join:project_id=id" json:"-"`
}

type DelegationRule struct {
	bun.BaseModel `bun:"table:delegation_rule" unique:"path,project_id" json:"-"`

	ID        int    `bun:"id,pk,autoincrement" json:"id"`
	Path      string `bun:"path,notnull" json:"path"`
	Priority  int    `bun:"priority,notnull" json:"priority"`
	ProjectID int    `bun:"project_id,notnull" json:"-" fk:"project.id,cascade"`
	UserID    int    `bun:"user_id,notnull" json:"-" fk:"auth_user.id,cascade"`
}

type State struct {
	bun.BaseModel `bun:"table:state" json:"-"`

	ID             int    `bun:"id,pk,autoincrement" json:"id"`
	Name           string `bun:"name,notnull,unique" json:"name"`
	Slug           string `bun:"slug,notnull,unique" json:"slug"`
	Ordering       int    `bun:"ordering,notnull,unique" json:"ordering"`
	ActionRequired bool   `bun:"action_required,notnull" json:"action_required"`
}

type Tag struct {
	bun.BaseModel `bun:"table:tag" json:"-"`

	ID         int    `bun:"id,pk,autoincrement" json:"id"`
	Name       string `bun:"name,notnull" json:"name"`
	Pattern    string `bun:"pattern,notnull" json:"-"`
	Abbrev     string `bun:"abbrev,notnull,unique" json:"abbrev"`
	ShowColumn bool   `bun:"show_column,notnull" json:"-"`
}

type Series struct {
	bun.BaseModel `bun:"table:series" json:"-"`

	ID               int       `bun:"id,pk,autoincrement" json:"id"`
	ProjectID        *int      `bun:"project_id" json:"-" fk:"project.id,cascade"`
	CoverLetterID    *int      `bun:"cover_letter_id" json:"-" fk:"cover.id,setnull"`
	PreviousSeriesID *int      `bun:"previous_series_id" json:"-" fk:"series.id,setnull"`
	Name             *string   `bun:"name" json:"name"`
	Date             time.Time `bun:"date,notnull" json:"date"`
	SubmitterID      int       `bun:"submitter_id,notnull" json:"-" fk:"person.id"`
	Version          int       `bun:"version,notnull" json:"version"`
	Total            int       `bun:"total,notnull" json:"total"`

	Submitter *Person  `bun:"rel:belongs-to,join:submitter_id=id" json:"submitter,omitempty"`
	Project   *Project `bun:"rel:belongs-to,join:project_id=id" json:"project,omitempty"`

	URL            string            `bun:"-" json:"url,omitempty"`
	WebURL         string            `bun:"-" json:"web_url,omitempty"`
	MboxURL        string            `bun:"-" json:"mbox,omitempty"`
	ReceivedTotal  int               `bun:"-" json:"received_total"`
	ReceivedAll    bool              `bun:"-" json:"received_all"`
	CoverLetter    *Cover            `bun:"-" json:"cover_letter"`
	Patches        []Patch           `bun:"-" json:"patches"`
	Metadata       map[string]string `bun:"-" json:"metadata"`
	Dependencies   []string          `bun:"-" json:"dependencies"`
	Dependents     []string          `bun:"-" json:"dependents"`
	PreviousSeries *string           `bun:"-" json:"previous_series"`
	NextSeries     []string          `bun:"-" json:"next_series"`
}

type SeriesMetadata struct {
	bun.BaseModel `bun:"table:series_metadata" unique:"series_id,key" json:"-"`

	ID       int    `bun:"id,pk,autoincrement"`
	SeriesID int    `bun:"series_id,notnull" fk:"series.id,cascade"`
	Key      string `bun:"key,notnull" index:""`
	Value    string `bun:"value,notnull"`
}

type SeriesReference struct {
	bun.BaseModel `bun:"table:series_reference" unique:"project_id,msgid" json:"-"`

	ID        int    `bun:"id,pk,autoincrement" json:"id"`
	Msgid     string `bun:"msgid,notnull" json:"msgid"`
	ProjectID int    `bun:"project_id,notnull" json:"-" fk:"project.id,cascade"`
	SeriesID  int    `bun:"series_id,notnull" json:"-" fk:"series.id,cascade"`
}

type SeriesDependencies struct {
	bun.BaseModel `bun:"table:series_dependencies" unique:"from_series_id,to_series_id" json:"-"`

	ID           int `bun:"id,pk,autoincrement"`
	FromSeriesID int `bun:"from_series_id,notnull" fk:"series.id,cascade"`
	ToSeriesID   int `bun:"to_series_id,notnull" fk:"series.id,cascade"`
}

type PatchRef struct {
	ID   int    `json:"id"`
	URL  string `json:"url,omitempty"`
	Name string `json:"name"`
}

type SeriesRef struct {
	ID   int     `json:"id"`
	URL  string  `json:"url,omitempty"`
	Name *string `json:"name"`
}

type Patch struct {
	bun.BaseModel `bun:"table:patch" unique:"msgid,project_id;series_id,number" index:"archived,state_id,delegate_id,date,project_id,submitter_id,name" json:"-"`

	ID          int       `bun:"id,pk,autoincrement" json:"id"`
	Msgid       string    `bun:"msgid,notnull" json:"msgid"`
	Date        time.Time `bun:"date,notnull" json:"date" index:""`
	Headers     string    `bun:"headers,notnull" json:"headers,omitempty"`
	SubmitterID int       `bun:"submitter_id,notnull" json:"-" fk:"person.id"`
	Content     *string   `bun:"content" json:"content,omitempty"`
	ProjectID   int       `bun:"project_id,notnull" json:"-" fk:"project.id,cascade"`
	Name        string    `bun:"name,notnull" json:"name"`
	Diff        *string   `bun:"diff" json:"diff,omitempty"`
	CommitRef   *string   `bun:"commit_ref" json:"commit_ref,omitempty"`
	PullURL     *string   `bun:"pull_url" json:"pull_url,omitempty"`
	DelegateID  *int      `bun:"delegate_id" json:"-" fk:"auth_user.id,setnull"`
	StateID     *int      `bun:"state_id" json:"-" fk:"state.id"`
	Archived    bool      `bun:"archived,notnull" json:"archived"`
	Hash        *string   `bun:"hash" json:"hash,omitempty" index:""`
	SeriesID    *int      `bun:"series_id" json:"-" fk:"series.id,setnull"`
	Number      *int      `bun:"number" json:"-"`
	RelatedID   *int      `bun:"related_id" json:"-" fk:"patch_relation.id,setnull"`

	Submitter *Person  `bun:"rel:belongs-to,join:submitter_id=id" json:"submitter,omitempty"`
	Project   *Project `bun:"rel:belongs-to,join:project_id=id" json:"project,omitempty"`
	State     *State   `bun:"rel:belongs-to,join:state_id=id" json:"state,omitempty"`
	Delegate  *User    `bun:"rel:belongs-to,join:delegate_id=id" json:"delegate,omitempty"`

	Related []PatchRef `bun:"-" json:"related"`

	URL            string         `bun:"-" json:"url,omitempty"`
	WebURL         string         `bun:"-" json:"web_url,omitempty"`
	MboxURL        string         `bun:"-" json:"mbox,omitempty"`
	ListArchiveURL string         `bun:"-" json:"list_archive_url,omitempty"`
	CommentsURL    string         `bun:"-" json:"comments,omitempty"`
	ChecksURL      string         `bun:"-" json:"checks,omitempty"`
	CombinedCheck  *string        `bun:"-" json:"check,omitempty"`
	CheckCounts    [4]int         `bun:"-" json:"-"`
	Tags           map[string]int `bun:"-" json:"tags"`
	SeriesList     []SeriesRef    `bun:"-" json:"series"`
}

type PatchTag struct {
	bun.BaseModel `bun:"table:patch_tag" unique:"patch_id,tag_id" json:"-"`

	ID      int `bun:"id,pk,autoincrement" json:"id"`
	PatchID int `bun:"patch_id,notnull" json:"-" fk:"patch.id,cascade"`
	TagID   int `bun:"tag_id,notnull" json:"-" fk:"tag.id"`
	Count   int `bun:"count,notnull" json:"count"`
}

type PatchComment struct {
	bun.BaseModel `bun:"table:patch_comment" unique:"msgid,patch_id" index:"patch_id,date" json:"-"`

	ID          int       `bun:"id,pk,autoincrement" json:"id"`
	Msgid       string    `bun:"msgid,notnull" json:"msgid"`
	Date        time.Time `bun:"date,notnull" json:"date"`
	Headers     string    `bun:"headers,notnull" json:"headers,omitempty"`
	SubmitterID int       `bun:"submitter_id,notnull" json:"-" fk:"person.id"`
	Content     *string   `bun:"content" json:"content,omitempty"`
	PatchID     int       `bun:"patch_id,notnull" json:"-" fk:"patch.id,cascade"`
	Addressed   *bool     `bun:"addressed" json:"addressed"`

	Submitter *Person `bun:"rel:belongs-to,join:submitter_id=id" json:"submitter,omitempty"`
	URL       string  `bun:"-" json:"url,omitempty"`
	Subject   string  `bun:"-" json:"subject,omitempty"`
}

type PatchRelation struct {
	bun.BaseModel `bun:"table:patch_relation" json:"-"`

	ID int `bun:"id,pk,autoincrement" json:"id"`
}

type Cover struct {
	bun.BaseModel `bun:"table:cover" unique:"msgid,project_id" index:"date,project_id,submitter_id,name" json:"-"`

	ID          int       `bun:"id,pk,autoincrement" json:"id"`
	Msgid       string    `bun:"msgid,notnull" json:"msgid"`
	Date        time.Time `bun:"date,notnull" json:"date"`
	Headers     string    `bun:"headers,notnull" json:"headers,omitempty"`
	SubmitterID int       `bun:"submitter_id,notnull" json:"-" fk:"person.id"`
	Content     *string   `bun:"content" json:"content,omitempty"`
	ProjectID   int       `bun:"project_id,notnull" json:"-" fk:"project.id,cascade"`
	Name        string    `bun:"name,notnull" json:"name"`

	Submitter *Person  `bun:"rel:belongs-to,join:submitter_id=id" json:"submitter,omitempty"`
	Project   *Project `bun:"rel:belongs-to,join:project_id=id" json:"project,omitempty"`

	URL            string      `bun:"-" json:"url,omitempty"`
	WebURL         string      `bun:"-" json:"web_url,omitempty"`
	MboxURL        string      `bun:"-" json:"mbox,omitempty"`
	ListArchiveURL string      `bun:"-" json:"list_archive_url,omitempty"`
	Comments       string      `bun:"-" json:"comments,omitempty"`
	SeriesList     []SeriesRef `bun:"-" json:"series"`
}

type CoverComment struct {
	bun.BaseModel `bun:"table:cover_comment" unique:"msgid,cover_id" index:"cover_id,date" json:"-"`

	ID          int       `bun:"id,pk,autoincrement" json:"id"`
	Msgid       string    `bun:"msgid,notnull" json:"msgid"`
	Date        time.Time `bun:"date,notnull" json:"date"`
	Headers     string    `bun:"headers,notnull" json:"headers,omitempty"`
	SubmitterID int       `bun:"submitter_id,notnull" json:"-" fk:"person.id"`
	Content     *string   `bun:"content" json:"content,omitempty"`
	CoverID     int       `bun:"cover_id,notnull" json:"-" fk:"cover.id,cascade"`
	Addressed   *bool     `bun:"addressed" json:"addressed"`

	Submitter *Person `bun:"rel:belongs-to,join:submitter_id=id" json:"submitter,omitempty"`
	URL       string  `bun:"-" json:"url,omitempty"`
	Subject   string  `bun:"-" json:"subject,omitempty"`
}

type Bundle struct {
	bun.BaseModel `bun:"table:bundle" unique:"owner_id,name" json:"-"`

	ID        int    `bun:"id,pk,autoincrement" json:"id"`
	OwnerID   int    `bun:"owner_id,notnull" json:"-" fk:"auth_user.id,cascade"`
	ProjectID int    `bun:"project_id,notnull" json:"-" fk:"project.id,cascade"`
	Name      string `bun:"name,notnull" json:"name"`
	Public    bool   `bun:"public,notnull" json:"public"`

	Owner   *User    `bun:"rel:belongs-to,join:owner_id=id" json:"owner,omitempty"`
	Project *Project `bun:"rel:belongs-to,join:project_id=id" json:"project,omitempty"`

	URL           string  `bun:"-" json:"url,omitempty"`
	WebURL        string  `bun:"-" json:"web_url,omitempty"`
	MboxURL       string  `bun:"-" json:"mbox,omitempty"`
	BundlePatches []Patch `bun:"-" json:"patches"`
	PatchCount    int     `bun:"patch_count,scanonly" json:"-"`
}

type BundlePatch struct {
	bun.BaseModel `bun:"table:bundle_patch" unique:"bundle_id,patch_id" json:"-"`

	ID       int `bun:"id,pk,autoincrement"`
	BundleID int `bun:"bundle_id,notnull" fk:"bundle.id,cascade"`
	PatchID  int `bun:"patch_id,notnull" fk:"patch.id,cascade"`
	Order    int `bun:"order,notnull"`
}

type Check struct {
	bun.BaseModel `bun:"table:ci_check,alias:ci_check" unique:"patch_id,context,user_id" json:"-"`

	ID          int        `bun:"id,pk,autoincrement" json:"id"`
	PatchID     int        `bun:"patch_id,notnull" json:"-" fk:"patch.id,cascade"`
	UserID      *int       `bun:"user_id" json:"-" fk:"auth_user.id,setnull"`
	Date        time.Time  `bun:"date,notnull" json:"date"`
	State       CheckState `bun:"state,notnull" json:"state"`
	TargetURL   string     `bun:"target_url,notnull" json:"target_url"`
	Context     string     `bun:"context,notnull" json:"context" index:""`
	Description string     `bun:"description,notnull" json:"description"`

	User *User  `bun:"rel:belongs-to,join:user_id=id" json:"user,omitempty"`
	URL  string `bun:"-" json:"url,omitempty"`
}

type CheckState int

const (
	CheckPending CheckState = 0
	CheckSuccess CheckState = 1
	CheckWarning CheckState = 2
	CheckFail    CheckState = 3
)

func (s CheckState) MarshalJSON() ([]byte, error) {
	names := map[CheckState]string{
		CheckPending: "pending",
		CheckSuccess: "success",
		CheckWarning: "warning",
		CheckFail:    "fail",
	}
	name, ok := names[s]
	if !ok {
		name = "pending"
	}
	return json.Marshal(name)
}

func (s *CheckState) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		return err
	}
	states := map[string]CheckState{
		"pending": CheckPending,
		"success": CheckSuccess,
		"warning": CheckWarning,
		"fail":    CheckFail,
	}
	*s = states[name]
	return nil
}

type Event struct {
	bun.BaseModel `bun:"table:event" index:"project_id,category,date" json:"-"`

	ID                 int       `bun:"id,pk,autoincrement" json:"id"`
	ProjectID          int       `bun:"project_id,notnull" json:"-" fk:"project.id,cascade"`
	Category           string    `bun:"category,notnull" json:"category" index:""`
	Date               time.Time `bun:"date,notnull" json:"date"`
	ActorID            *int      `bun:"actor_id" json:"-" fk:"auth_user.id,setnull"`
	PatchID            *int      `bun:"patch_id" json:"-" fk:"patch.id,cascade"`
	SeriesID           *int      `bun:"series_id" json:"-" fk:"series.id,cascade"`
	CoverID            *int      `bun:"cover_id" json:"-" fk:"cover.id,cascade"`
	PreviousStateID    *int      `bun:"previous_state_id" json:"-" fk:"state.id"`
	CurrentStateID     *int      `bun:"current_state_id" json:"-" fk:"state.id"`
	PreviousDelegateID *int      `bun:"previous_delegate_id" json:"-" fk:"auth_user.id,setnull"`
	CurrentDelegateID  *int      `bun:"current_delegate_id" json:"-" fk:"auth_user.id,setnull"`
	PreviousRelationID *int      `bun:"previous_relation_id" json:"-" fk:"patch_relation.id,setnull"`
	CurrentRelationID  *int      `bun:"current_relation_id" json:"-" fk:"patch_relation.id,setnull"`
	CreatedCheckID     *int      `bun:"created_check_id" json:"-" fk:"ci_check.id,cascade"`
	CoverCommentID     *int      `bun:"cover_comment_id" json:"-" fk:"cover_comment.id,cascade"`
	PatchCommentID     *int      `bun:"patch_comment_id" json:"-" fk:"patch_comment.id,cascade"`

	Project *Project `bun:"rel:belongs-to,join:project_id=id" json:"project,omitempty"`
	Actor   *User    `bun:"rel:belongs-to,join:actor_id=id" json:"actor"`
	Payload any      `bun:"-" json:"payload,omitempty"`
}

type Webhook struct {
	bun.BaseModel `bun:"table:webhook" unique:"project_id,url" json:"-"`

	ID        int       `bun:"id,pk,autoincrement" json:"id"`
	ProjectID int       `bun:"project_id,notnull" json:"-" fk:"project.id,cascade"`
	URL       string    `bun:"url,notnull" json:"url"`
	Secret    string    `bun:"secret,notnull" json:"-"`
	Events    string    `bun:"events,notnull" json:"events"`
	Active    bool      `bun:"active,notnull" json:"active"`
	CreatorID int       `bun:"creator_id,notnull" json:"-" fk:"auth_user.id"`
	Created   time.Time `bun:"created,notnull" json:"created"`
}
