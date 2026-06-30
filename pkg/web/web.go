// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package web

//go:generate go tool templ generate -lazy

import (
	"crypto/rand"
	"embed"
	"fmt"
	"html"
	"io/fs"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/config"
	"github.com/getpatchwork/patchwork/pkg/db"
)

//go:embed static/*
var staticFS embed.FS

func NewRouter(cfg *config.Config, database *bun.DB, bus db.EventBus, version string) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.StripSlashes)
	r.Use(db.Middleware(database, bus))

	csrfKey := make([]byte, 32)
	if _, err := rand.Read(csrfKey); err != nil {
		panic("generate CSRF key: " + err.Error())
	}
	h := &webHandler{cfg: cfg, db: database, csrfKey: csrfKey, version: version}
	r.Use(h.sessionMiddleware)

	sub, _ := fs.Sub(staticFS, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(sub))))

	r.Get("/", h.ProjectList)
	r.Get("/project/{linkname}", h.ProjectDetail)
	r.Get("/project/{linkname}/list", h.PatchList)
	r.Get("/project/{linkname}/bundles", h.ProjectBundleList)
	r.Post("/project/{linkname}/list", h.PatchListAction)
	r.Get("/project/{linkname}/patch/{msgid}", h.PatchDetailPage)
	r.Post("/project/{linkname}/patch/{msgid}", h.PatchUpdate)
	r.Post("/project/{linkname}/patch/{msgid}/comment/{commentID}/addressed", h.CommentAddressed)
	r.Get("/project/{linkname}/patch/{msgid}/mbox", h.PatchMboxPage)
	r.Get("/project/{linkname}/patch/{msgid}/raw", h.PatchRawPage)
	r.Get("/project/{linkname}/cover/{msgid}", h.CoverDetailPage)
	r.Get("/project/{linkname}/cover/{msgid}/mbox", h.CoverMboxPage)

	r.Get("/patch/{id}", h.PatchRedirect)
	r.Get("/patch/{id}/mbox", h.PatchMboxByID)
	r.Get("/patch/{id}/raw", h.PatchRawByID)
	r.Get("/cover/{id}", h.CoverRedirect)
	r.Get("/cover/{id}/mbox", h.CoverMboxByID)

	r.Get("/series/{id}/mbox", h.SeriesMbox)
	r.Get("/bundle/{username}/{bundlename}/mbox", h.BundleMbox)

	r.Get("/comment/{id}", h.CommentRedirect)

	r.Get("/user/login", h.LoginPage)
	r.Post("/user/login", h.LoginSubmit)
	r.Post("/user/logout", h.LogoutSubmit)

	r.Get("/register", h.RegisterPage)
	r.Post("/register", h.RegisterSubmit)
	r.Get("/confirm/{key}", h.ConfirmHandler)

	r.Get("/user", h.ProfilePage)
	r.Post("/user", h.ProfileUpdate)
	r.Get("/user/link", h.LinkEmail)
	r.Post("/user/link", h.LinkEmail)
	r.Post("/user/unlink/{id}", h.UnlinkEmail)
	r.Get("/user/password-change", h.ChangePassword)
	r.Post("/user/password-change", h.ChangePassword)
	r.Post("/user/generate-token", h.GenerateToken)
	r.Get("/user/password-reset", h.PasswordReset)
	r.Post("/user/password-reset", h.PasswordReset)
	r.Get("/password-reset/{key}", h.PasswordResetConfirm)
	r.Post("/password-reset/{key}", h.PasswordResetConfirm)

	r.Get("/user/todo", h.TodoLists)
	r.Get("/user/todo/{linkname}", h.todoList)
	r.Get("/user/bundles", h.BundleList)
	r.Get("/bundle/{username}/{bundlename}", h.BundleDetail)
	r.Post("/bundle/{username}/{bundlename}", h.BundleUpdate)

	r.Get("/about", func(w http.ResponseWriter, r *http.Request) {
		aboutPage(h.pageCtx(r)).Render(r.Context(), w)
	})

	return r
}

type webHandler struct {
	cfg     *config.Config
	db      *bun.DB
	csrfKey []byte
	version string
}

func intStr(n int) string {
	return strconv.Itoa(n)
}

func personName(p *db.Person) string {
	if p == nil {
		return ""
	}
	if p.Name != nil && *p.Name != "" {
		return *p.Name
	}
	return p.Email
}

func patchURL(linkname, msgid string) string {
	clean := strings.TrimPrefix(msgid, "<")
	clean = strings.TrimSuffix(clean, ">")
	return fmt.Sprintf("/project/%s/patch/%s/", linkname, url.PathEscape(clean))
}

func coverURL(linkname, msgid string) string {
	clean := strings.TrimPrefix(msgid, "<")
	clean = strings.TrimSuffix(clean, ">")
	return fmt.Sprintf("/project/%s/cover/%s/", linkname, url.PathEscape(clean))
}

func sortURL(d patchListData, newSort string) string {
	return fmt.Sprintf("/project/%s/list/?order=%s%s",
		d.Project.Linkname, newSort, d.BaseQuery)
}

func pageURL(baseQuery string, page int) string {
	if strings.Contains(baseQuery, "page=") {
		return baseQuery
	}
	sep := "&"
	if baseQuery == "" {
		sep = "?"
	}
	return fmt.Sprintf("%s%spage=%d", baseQuery, sep, page)
}

func highlightDiff(diff string) string {
	var b strings.Builder
	for _, line := range strings.Split(diff, "\n") {
		escaped := html.EscapeString(line)
		switch {
		case strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "--- "):
			b.WriteString("<b>")
			b.WriteString(escaped)
			b.WriteString("</b>\n")
		case strings.HasPrefix(line, "+"):
			b.WriteString("<ins>")
			b.WriteString(escaped)
			b.WriteString("</ins>\n")
		case strings.HasPrefix(line, "-"):
			b.WriteString("<del>")
			b.WriteString(escaped)
			b.WriteString("</del>\n")
		case strings.HasPrefix(line, "@@"):
			b.WriteString("<mark>")
			b.WriteString(escaped)
			b.WriteString("</mark>\n")
		case strings.HasPrefix(line, "diff "):
			b.WriteString("<b>")
			b.WriteString(escaped)
			b.WriteString("</b>\n")
		default:
			b.WriteString(escaped)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

var diffstatRe = regexp.MustCompile(`^(\s\S+\s+\|\s+\d+\s)([+-]+)$`)

func highlightMessage(text string) string {
	var b strings.Builder
	for _, line := range strings.Split(text, "\n") {
		escaped := html.EscapeString(line)
		lower := strings.ToLower(line)
		switch {
		case strings.HasPrefix(lower, "signed-off-by:"):
			b.WriteString("<sob>")
			b.WriteString(escaped)
			b.WriteString("</sob>\n")
		case strings.HasPrefix(lower, "acked-by:"),
			strings.HasPrefix(lower, "reviewed-by:"),
			strings.HasPrefix(lower, "tested-by:"),
			strings.HasPrefix(lower, "reported-by:"),
			strings.HasPrefix(lower, "nacked-by:"):
			b.WriteString("<trailer>")
			b.WriteString(escaped)
			b.WriteString("</trailer>\n")
		case strings.HasPrefix(line, "> ") || line == ">":
			b.WriteString("<q>")
			b.WriteString(escaped)
			b.WriteString("</q>\n")
		case diffstatRe.MatchString(line):
			m := diffstatRe.FindStringSubmatch(line)
			prefix := html.EscapeString(m[1])
			bar := m[2]
			b.WriteString(html.EscapeString(prefix))
			for _, c := range bar {
				if c == '+' {
					b.WriteString(`<ins>+</ins>`)
				} else {
					b.WriteString(`<del>-</del>`)
				}
			}
			b.WriteByte('\n')
		default:
			b.WriteString(escaped)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func commitMessage(p db.Patch) string {
	msg := p.Name
	if p.Content != nil && *p.Content != "" {
		msg += "\n\n" + *p.Content
	}
	return msg
}

func coverMessage(c db.Cover) string {
	msg := c.Name
	if c.Content != nil && *c.Content != "" {
		msg += "\n\n" + *c.Content
	}
	return msg
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func (h *webHandler) pageCtx(r *http.Request) pageContext {
	return pageContext{
		User:      getWebUser(r),
		CSRFToken: h.csrfToken(r),
		Version:   h.version,
	}
}

func notFoundPage(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte("Not found"))
}
