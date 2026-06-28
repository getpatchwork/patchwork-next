// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import "time"

// API response types — separate from db models so the OpenAPI spec only
// exposes what the API contract requires.
//
// Fields tagged with `since:"X.Y"` are only included in responses for API
// version X.Y and later. They must use pointer types with `omitempty` so that
// the version transformer can nil them out and have them disappear from the
// JSON output.

type UserEmbedded struct {
	ID        int    `json:"id"`
	URL       string `json:"url" format:"uri"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email" format:"email"`
}

type PersonEmbedded struct {
	ID    int    `json:"id"`
	URL   string `json:"url" format:"uri"`
	Name  string `json:"name"`
	Email string `json:"email" format:"email"`
}

type ProjectEmbedded struct {
	ID                   int     `json:"id"`
	URL                  string  `json:"url" format:"uri"`
	Name                 string  `json:"name"`
	LinkName             string  `json:"link_name"`
	ListID               string  `json:"list_id"`
	ListEmail            string  `json:"list_email" format:"email"`
	WebURL               string  `json:"web_url" format:"uri"`
	ScmURL               string  `json:"scm_url" format:"uri"`
	WebScmURL            string  `json:"webscm_url" format:"uri"`
	ListArchiveURL       *string `json:"list_archive_url,omitempty" format:"uri" since:"1.2"`
	ListArchiveURLFormat *string `json:"list_archive_url_format,omitempty" format:"uri" since:"1.2"`
	CommitURLFormat      *string `json:"commit_url_format,omitempty" since:"1.2"`
}

type SeriesEmbedded struct {
	ID      int     `json:"id"`
	URL     string  `json:"url" format:"uri"`
	WebURL  *string `json:"web_url,omitempty" format:"uri" since:"1.1"`
	Name    *string `json:"name"`
	Date    string  `json:"date" format:"date-time"`
	Version int     `json:"version"`
	Mbox    string  `json:"mbox" format:"uri"`
}

type PatchEmbedded struct {
	ID             int     `json:"id"`
	URL            string  `json:"url" format:"uri"`
	WebURL         *string `json:"web_url,omitempty" format:"uri" since:"1.1"`
	Msgid          string  `json:"msgid"`
	ListArchiveURL *string `json:"list_archive_url,omitempty" since:"1.2"`
	Date           string  `json:"date" format:"date-time"`
	Name           string  `json:"name"`
	Mbox           string  `json:"mbox" format:"uri"`
}

type CoverEmbedded struct {
	ID             int     `json:"id"`
	URL            string  `json:"url" format:"uri"`
	WebURL         *string `json:"web_url,omitempty" format:"uri" since:"1.1"`
	Msgid          string  `json:"msgid"`
	ListArchiveURL *string `json:"list_archive_url,omitempty" since:"1.2"`
	Date           string  `json:"date" format:"date-time"`
	Name           string  `json:"name"`
	Mbox           string  `json:"mbox" format:"uri"`
}

type CommentEmbedded struct {
	ID             int     `json:"id"`
	URL            *string `json:"url,omitempty" format:"uri" since:"1.3"`
	WebURL         *string `json:"web_url,omitempty" format:"uri" since:"1.1"`
	Msgid          string  `json:"msgid"`
	ListArchiveURL *string `json:"list_archive_url,omitempty" since:"1.2"`
	Date           string  `json:"date" format:"date-time"`
	Name           string  `json:"name"`
}

type ProjectResponse struct {
	ID                   int            `json:"id"`
	URL                  string         `json:"url" format:"uri"`
	Name                 string         `json:"name"`
	LinkName             string         `json:"link_name"`
	ListID               string         `json:"list_id"`
	ListEmail            string         `json:"list_email" format:"email"`
	WebURL               string         `json:"web_url" format:"uri"`
	ScmURL               string         `json:"scm_url" format:"uri"`
	WebScmURL            string         `json:"webscm_url" format:"uri"`
	Maintainers          []UserEmbedded `json:"maintainers"`
	SubjectMatch         *string        `json:"subject_match,omitempty" since:"1.1"`
	ListArchiveURL       *string        `json:"list_archive_url,omitempty" format:"uri" since:"1.2"`
	ListArchiveURLFormat *string        `json:"list_archive_url_format,omitempty" format:"uri" since:"1.2"`
	CommitURLFormat      *string        `json:"commit_url_format,omitempty" since:"1.2"`
	ShowDependencies     *bool          `json:"show_dependencies,omitempty" since:"1.4"`
}

type PatchListResponse struct {
	ID             int              `json:"id"`
	URL            string           `json:"url" format:"uri"`
	WebURL         *string          `json:"web_url,omitempty" format:"uri" since:"1.1"`
	Project        ProjectEmbedded  `json:"project"`
	Msgid          string           `json:"msgid"`
	ListArchiveURL *string          `json:"list_archive_url,omitempty" since:"1.2"`
	Date           time.Time        `json:"date"`
	Name           string           `json:"name"`
	CommitRef      *string          `json:"commit_ref,omitempty"`
	PullURL        *string          `json:"pull_url,omitempty" format:"uri"`
	State          string           `json:"state"`
	Archived       bool             `json:"archived"`
	Hash           string           `json:"hash"`
	Submitter      PersonEmbedded   `json:"submitter"`
	Delegate       *UserEmbedded    `json:"delegate"`
	Mbox           string           `json:"mbox" format:"uri"`
	Series         []SeriesEmbedded `json:"series"`
	Comments       *string          `json:"comments,omitempty" format:"uri" since:"1.1"`
	Check          string           `json:"check" enum:"pending,success,warning,fail"`
	Checks         string           `json:"checks" format:"uri"`
	Tags           map[string]int   `json:"tags"`
	Related        []PatchEmbedded  `json:"related" since:"1.2"`
}

type PatchDetailResponse struct {
	PatchListResponse
	Headers  map[string]string `json:"headers"`
	Content  string            `json:"content"`
	Diff     string            `json:"diff"`
	Prefixes []string          `json:"prefixes"`
}

type PersonResponse struct {
	ID    int           `json:"id"`
	URL   string        `json:"url" format:"uri"`
	Name  string        `json:"name"`
	Email string        `json:"email" format:"email"`
	User  *UserEmbedded `json:"user"`
}

type UserResponse struct {
	ID        int    `json:"id"`
	URL       string `json:"url" format:"uri"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email" format:"email"`
}

type UserDetailResponse struct {
	UserResponse
	Settings *UserSettings `json:"settings,omitempty"`
}

type UserSettings struct {
	SendEmail    bool `json:"send_email"`
	ItemsPerPage int  `json:"items_per_page"`
	ShowIds      bool `json:"show_ids"`
}

type CoverListResponse struct {
	ID             int              `json:"id"`
	URL            string           `json:"url" format:"uri"`
	WebURL         *string          `json:"web_url,omitempty" format:"uri" since:"1.1"`
	Project        ProjectEmbedded  `json:"project"`
	Msgid          string           `json:"msgid"`
	ListArchiveURL *string          `json:"list_archive_url,omitempty" since:"1.2"`
	Date           time.Time        `json:"date"`
	Name           string           `json:"name"`
	Submitter      PersonEmbedded   `json:"submitter"`
	Mbox           string           `json:"mbox" format:"uri"`
	Series         []SeriesEmbedded `json:"series"`
	Comments       *string          `json:"comments,omitempty" format:"uri" since:"1.1"`
}

type CoverDetailResponse struct {
	CoverListResponse
	Headers map[string]string `json:"headers"`
	Content string            `json:"content"`
}

type CommentResponse struct {
	ID             int               `json:"id"`
	URL            *string           `json:"url,omitempty" format:"uri" since:"1.3"`
	WebURL         *string           `json:"web_url,omitempty" format:"uri" since:"1.1"`
	Msgid          string            `json:"msgid"`
	ListArchiveURL *string           `json:"list_archive_url,omitempty" since:"1.2"`
	Date           time.Time         `json:"date" format:"date-time"`
	Subject        string            `json:"subject"`
	Submitter      PersonEmbedded    `json:"submitter"`
	Content        string            `json:"content"`
	Headers        map[string]string `json:"headers"`
	Addressed      *bool             `json:"addressed" since:"1.3"`
}

type CheckResponse struct {
	ID          int           `json:"id"`
	URL         string        `json:"url" format:"uri"`
	User        *UserEmbedded `json:"user"`
	Date        time.Time     `json:"date" format:"date-time"`
	State       string        `json:"state" enum:"pending,success,warning,fail"`
	TargetURL   *string       `json:"target_url" format:"uri"`
	Context     string        `json:"context"`
	Description *string       `json:"description"`
}

type SeriesResponse struct {
	ID            int                `json:"id"`
	URL           string             `json:"url" format:"uri"`
	WebURL        *string            `json:"web_url,omitempty" format:"uri" since:"1.1"`
	Project       ProjectEmbedded    `json:"project"`
	Name          *string            `json:"name"`
	Date          time.Time          `json:"date" format:"date-time"`
	Submitter     PersonEmbedded     `json:"submitter"`
	Version       int                `json:"version"`
	Total         int                `json:"total"`
	ReceivedTotal int                `json:"received_total"`
	ReceivedAll   bool               `json:"received_all"`
	Mbox          string             `json:"mbox" format:"uri"`
	CoverLetter   *CoverEmbedded     `json:"cover_letter"`
	Patches       []PatchEmbedded    `json:"patches"`
	Dependencies  []string           `json:"dependencies" since:"1.4"`
	Dependents    []string           `json:"dependents" since:"1.4"`
	Metadata      *map[string]string `json:"metadata"`
}

type BundleResponse struct {
	ID      int             `json:"id"`
	URL     string          `json:"url" format:"uri"`
	WebURL  *string         `json:"web_url,omitempty" format:"uri" since:"1.1"`
	Project ProjectEmbedded `json:"project"`
	Name    string          `json:"name"`
	Owner   *UserEmbedded   `json:"owner"`
	Patches []PatchEmbedded `json:"patches"`
	Public  bool            `json:"public"`
	Mbox    string          `json:"mbox" format:"uri"`
}

type EventResponse struct {
	ID       int             `json:"id"`
	Category string          `json:"category"`
	Project  ProjectEmbedded `json:"project"`
	Date     time.Time       `json:"date" format:"date-time"`
	Actor    *UserEmbedded   `json:"actor,omitempty" since:"1.2"`
	Payload  any             `json:"payload"`
}

type WebhookResponse struct {
	ID      int       `json:"id"`
	URL     string    `json:"url" format:"uri"`
	Events  string    `json:"events"`
	Active  bool      `json:"active"`
	Created time.Time `json:"created" format:"date-time"`
}

// Mutation body types

type PatchUpdateBody struct {
	State     *string `json:"state,omitempty"`
	Delegate  *int    `json:"delegate,omitempty"`
	Archived  *bool   `json:"archived,omitempty"`
	CommitRef *string `json:"commit_ref,omitempty"`
	PullURL   *string `json:"pull_url,omitempty" format:"uri"`
	Related   *[]int  `json:"related,omitempty"`
}

type ProjectUpdateBody struct {
	WebURL               *string `json:"web_url,omitempty" format:"uri"`
	ScmURL               *string `json:"scm_url,omitempty" format:"uri"`
	WebScmURL            *string `json:"webscm_url,omitempty" format:"uri"`
	ListArchiveURL       *string `json:"list_archive_url,omitempty" format:"uri"`
	ListArchiveURLFormat *string `json:"list_archive_url_format,omitempty" format:"uri"`
	CommitURLFormat      *string `json:"commit_url_format,omitempty"`
}

type CommentUpdateBody struct {
	Addressed *bool `json:"addressed,omitempty"`
}

type UserUpdateBody struct {
	FirstName *string `json:"first_name,omitempty"`
	LastName  *string `json:"last_name,omitempty"`
}

type SeriesUpdateBody struct {
	Version  *int               `json:"version,omitempty"`
	Metadata *map[string]string `json:"metadata,omitempty"`
}

type CheckCreateBody struct {
	State       string  `json:"state" enum:"pending,success,warning,fail"`
	TargetURL   *string `json:"target_url,omitempty" format:"uri"`
	Context     string  `json:"context"`
	Description *string `json:"description,omitempty"`
}

type BundleCreateUpdateBody struct {
	Name    *string `json:"name,omitempty"`
	Public  *bool   `json:"public,omitempty"`
	Patches *[]int  `json:"patches,omitempty"`
}

type WebhookCreateBody struct {
	URL    string `json:"url" format:"uri"`
	Secret string `json:"secret"`
	Events string `json:"events"`
	Active *bool  `json:"active,omitempty"`
}

type WebhookUpdateBody struct {
	URL    *string `json:"url,omitempty" format:"uri"`
	Secret *string `json:"secret,omitempty"`
	Events *string `json:"events,omitempty"`
	Active *bool   `json:"active,omitempty"`
}
