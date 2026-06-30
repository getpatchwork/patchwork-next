// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/uptrace/bun"

	"github.com/getpatchwork/patchwork/pkg/config"
	"github.com/getpatchwork/patchwork/pkg/db"
)

const (
	defaultPerPage = 30
	maxPerPage     = 250
)

type PageParams struct {
	Page    int `query:"page" minimum:"1" default:"1" doc:"Page number"`
	PerPage int `query:"per_page" minimum:"1" maximum:"250" default:"30" doc:"Results per page"`
}

type SearchParams struct {
	Q     string `query:"q" doc:"Search term"`
	Order string `query:"order" doc:"Ordering field"`
}

// ctxKey is an unexported type used as key for context.WithValue.
// Using a dedicated type prevents collisions with context keys from
// other packages, even if the underlying integer values are the same.
type ctxKey int

const (
	// httpRequestKey stores the original *http.Request so that huma
	// handlers (which only receive context.Context) can access it to
	// build absolute URLs.
	httpRequestKey ctxKey = iota
	// apiVersionKey stores the parsed API version (e.g. 1.4) so that
	// handlers and the response transformer know which version the
	// client requested.
	apiVersionKey
	// authUserKey stores the *db.User set by the auth middleware when
	// a valid token is present in the request.
	authUserKey
)

func setRequest(ctx context.Context, r *http.Request) context.Context {
	return context.WithValue(ctx, httpRequestKey, r)
}

func getRequest(ctx context.Context) *http.Request {
	r, _ := ctx.Value(httpRequestKey).(*http.Request)
	return r
}

func getVersion(ctx context.Context) apiVersion {
	v, _ := ctx.Value(apiVersionKey).(apiVersion)
	return v
}

func setAuthUser(ctx context.Context, user *db.User) context.Context {
	return context.WithValue(ctx, authUserKey, user)
}

func getAuthUser(ctx context.Context) *db.User {
	u, _ := ctx.Value(authUserKey).(*db.User)
	return u
}

var supportedVersions = []apiVersion{
	{1, 0}, {1, 1}, {1, 2}, {1, 3}, {1, 4},
}

type handler struct {
	cfg     *config.Config
	db      *bun.DB
	baseURL string
}

var (
	AuthRequiredErr = huma.Error401Unauthorized("Authentication required.")
	ForbiddenErr    = huma.Error403Forbidden("You do not have permission to perform this action.")
)

func (h *handler) requireUser(ctx context.Context) (*db.User, error) {
	user := getAuthUser(ctx)
	if user == nil {
		return nil, AuthRequiredErr
	}
	return user, nil
}

func (h *handler) requireMaintainer(ctx context.Context, projectID int) (*db.User, error) {
	user := getAuthUser(ctx)
	if user == nil {
		return nil, AuthRequiredErr
	}
	if !db.GetQueries(ctx).IsMaintainer(user, projectID) {
		return nil, ForbiddenErr
	}
	return user, nil
}

func (h *handler) apiBase(ctx context.Context) string {
	ver := getVersion(ctx)
	if h.baseURL != "" {
		return fmt.Sprintf("%s/api/%d.%d",
			strings.TrimRight(h.baseURL, "/"),
			ver.Major, ver.Minor)
	}
	r := getRequest(ctx)
	if r == nil {
		return ""
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
		scheme = fwd
	}
	return fmt.Sprintf("%s://%s/api/%d.%d",
		scheme, r.Host, ver.Major, ver.Minor)
}

type IndexOutput struct {
	Body struct {
		Patches  string `json:"patches" format:"uri"`
		Covers   string `json:"covers" format:"uri"`
		Series   string `json:"series" format:"uri"`
		Projects string `json:"projects" format:"uri"`
		People   string `json:"people" format:"uri"`
		Users    string `json:"users" format:"uri"`
		Events   string `json:"events" format:"uri"`
		Bundles  string `json:"bundles" format:"uri"`
	}
}

func registerIndexRoute(api huma.API, h *handler, prefix string, mw huma.Middlewares) {
	huma.Register(api, huma.Operation{
		Method:      http.MethodGet,
		Path:        prefix,
		OperationID: fmt.Sprintf("api-index-v%s", prefix[5:]),
		Middlewares: mw,
	}, func(ctx context.Context, _ *struct{}) (*IndexOutput, error) {
		base := h.apiBase(ctx)
		out := &IndexOutput{}
		out.Body.Patches = base + "/patches/"
		out.Body.Covers = base + "/covers/"
		out.Body.Series = base + "/series/"
		out.Body.Projects = base + "/projects/"
		out.Body.People = base + "/people/"
		out.Body.Users = base + "/users/"
		out.Body.Events = base + "/events/"
		out.Body.Bundles = base + "/bundles/"
		return out, nil
	})
}

func versionTransformer(ctx huma.Context, status string, v any) (any, error) {
	ver := getVersion(ctx.Context())
	return stripVersionedFields(v, ver), nil
}

func NewRouter(cfg *config.Config, database *bun.DB, baseURL string, bus db.EventBus) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.StripSlashes)
	r.Use(db.Middleware(database, bus))

	h := &handler{cfg: cfg, db: database, baseURL: baseURL}

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(setRequest(r.Context(), r)))
		})
	})
	r.Use(authMiddleware(database))

	config := huma.DefaultConfig("Patchwork API", "1.4")
	config.DocsPath = "/api/docs"
	config.OpenAPIPath = "/api/openapi"
	config.SchemasPath = "/api/schemas"
	config.Transformers = append(
		config.Transformers, versionTransformer,
	)
	api := humachi.New(r, config)

	versionMiddleware := func(ver apiVersion) func(huma.Context, func(huma.Context)) {
		return func(ctx huma.Context, next func(huma.Context)) {
			ctx = huma.WithValue(ctx, apiVersionKey, ver)
			next(ctx)
		}
	}

	for _, ver := range supportedVersions {
		prefix := fmt.Sprintf("/api/%d.%d", ver.Major, ver.Minor)
		mw := huma.Middlewares{versionMiddleware(ver)}

		registerIndexRoute(api, h, prefix, mw)
		registerProjectRoutes(api, h, prefix, mw)
		registerPatchRoutes(api, h, prefix, mw)
		registerCoverRoutes(api, h, prefix, mw)
		registerCommentRoutes(api, h, prefix, mw)
		registerCheckRoutes(api, h, prefix, mw)
		registerSeriesRoutes(api, h, prefix, mw)
		registerPeopleRoutes(api, h, prefix, mw)
		registerUserRoutes(api, h, prefix, mw)
		registerBundleRoutes(api, h, prefix, mw)
		registerWebhookRoutes(api, h, prefix, mw)
		registerEventRoutes(api, h, prefix, mw)
	}

	latest := supportedVersions[len(supportedVersions)-1]
	latestPrefix := fmt.Sprintf("/api/%d.%d", latest.Major, latest.Minor)
	r.Get("/api/*", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/api")
		http.Redirect(w, r, latestPrefix+rest, http.StatusMovedPermanently)
	})

	return r
}
