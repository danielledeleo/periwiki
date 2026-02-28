package server

import (
	"database/sql"
	"io/fs"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/danielledeleo/periwiki/internal/embedded"
	"github.com/danielledeleo/periwiki/special"
	"github.com/danielledeleo/periwiki/templater"
	"github.com/danielledeleo/periwiki/wiki"
	"github.com/danielledeleo/periwiki/wiki/repository"
	"github.com/danielledeleo/periwiki/wiki/service"
	"github.com/gorilla/mux"
	"github.com/microcosm-cc/bluemonday"
)

// ContentInfo holds metadata about embedded content files for the admin UI.
type ContentInfo struct {
	Files       []ContentFileEntry
	BuildCommit string
	SourceURL   string // URL to the source at this commit
}

// ContentFileEntry describes a single file in the content filesystem.
type ContentFileEntry struct {
	Path   string // e.g. "templates/layouts/index.html"
	Source string // "embedded" or "disk"
}

// App holds all application dependencies and services.
type App struct {
	*templater.Templater
	Articles      service.ArticleService
	Users         service.UserService
	Sessions      service.SessionService
	Rendering     service.RenderingService
	Preferences   service.PreferenceService
	SpecialPages  *special.Registry
	Config        *wiki.Config
	RuntimeConfig *wiki.RuntimeConfig
	ContentInfo   *ContentInfo
	DB            *sql.DB
}

// responseWriter wraps http.ResponseWriter to capture the status code
type responseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.size += n
	return n, err
}

// SlogLoggingMiddleware logs HTTP requests using slog
func SlogLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.status,
			"size", wrapped.size,
			"duration", time.Since(start),
			"remote", r.RemoteAddr,
		)
	})
}

// RegisterRoutes adds all application routes to the given router.
// Both the main server and the WASM demo call this to avoid duplication.
func (a *App) RegisterRoutes(router *mux.Router, contentFS fs.FS) {
	router.Use(a.SessionMiddleware)

	staticSub, _ := fs.Sub(contentFS, "static")
	staticFS := http.FileServer(http.FS(staticSub))
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", staticFS))

	router.HandleFunc("/", a.HomeHandler).Methods("GET")
	router.HandleFunc("/wiki/{namespace:[^:/]+}:{page}", a.NamespaceHandler).Methods("GET", "POST")
	router.HandleFunc("/wiki/{article}", a.ArticleDispatcher).Methods("GET", "POST")

	router.HandleFunc("/user/register", a.RegisterHandler).Methods("GET")
	router.HandleFunc("/user/register", a.RegisterPostHandler).Methods("POST")
	router.HandleFunc("/user/login", a.LoginHandler).Methods("GET")
	router.HandleFunc("/user/login", a.LoginPostHandler).Methods("POST")
	router.HandleFunc("/user/logout", a.LogoutPostHandler).Methods("POST")

	router.HandleFunc("/manage/users", a.ManageUsersHandler).Methods("GET")
	router.HandleFunc("/manage/users/{id:[0-9]+}", a.ManageUserRoleHandler).Methods("POST")
	router.HandleFunc("/manage/settings", a.ManageSettingsHandler).Methods("GET")
	router.HandleFunc("/manage/settings", a.ManageSettingsPostHandler).Methods("POST")
	router.HandleFunc("/manage/tools", a.ManageToolsHandler).Methods("GET")
	router.HandleFunc("/manage/tools/reset-main-page", a.ResetMainPageHandler).Methods("POST")
	router.HandleFunc("/manage/tools/backfill-links", a.BackfillLinksHandler).Methods("POST")
	router.HandleFunc("/manage/content", a.ManageContentHandler).Methods("GET")
}

// NewSanitizer creates the bluemonday HTML sanitizer with the standard policy.
// Used by both the main server and the WASM demo.
func NewSanitizer() *bluemonday.Policy {
	bm := bluemonday.UGCPolicy()
	bm.AllowAttrs("class").Globally()
	bm.AllowAttrs("data-line-number").Matching(regexp.MustCompile("^[0-9]+$")).OnElements("a")
	bm.AllowAttrs("style").OnElements("ins", "del")
	bm.AllowAttrs("style").Matching(regexp.MustCompile(`^text-align:\s+(left|right|center);$`)).OnElements("td", "th")
	bm.AllowElements("input", "label")
	bm.AllowAttrs("type", "id", "class", "checked").OnElements("input")
	bm.AllowAttrs("for").OnElements("label")
	return bm
}

// RegisterSpecialPages creates and populates a special page registry.
// Used by both the main server and the WASM demo.
func RegisterSpecialPages(articles service.ArticleService, t *templater.Templater, baseURL string) *special.Registry {
	registry := special.NewRegistry()
	registry.Register("Random", special.NewRandomPage(articles))

	sitemapHandler := special.NewSitemapPage(articles, t, baseURL)
	registry.Register("Sitemap", sitemapHandler)
	registry.Register("Sitemap.xml", sitemapHandler)

	registry.Register("RerenderAll", special.NewRerenderAllPage(articles, t))
	registry.Register("WhatLinksHere", special.NewWhatLinksHerePage(articles, t))
	registry.Register("SourceCode", special.NewSourceCodePage())
	return registry
}

// NewExistenceChecker creates the wikilink existence checker function.
// The returned ExistenceState must have its Embedded and SpecialPages fields
// set after creation (they are nil initially due to circular dependencies).
func NewExistenceChecker(db repository.ArticleRepository) (func(string) bool, *ExistenceState) {
	state := &ExistenceState{db: db}
	return state.check, state
}

// ExistenceState holds the mutable dependencies for the existence checker.
// Embedded and SpecialPages are set after initial creation.
type ExistenceState struct {
	db           repository.ArticleRepository
	Embedded     *embedded.EmbeddedArticles
	SpecialPages *special.Registry
}

func (s *ExistenceState) check(url string) bool {
	const prefix = "/wiki/"
	if len(url) > len(prefix) {
		url = url[len(prefix):]
	}

	article, _ := s.db.SelectArticle(url)
	if article != nil {
		return true
	}

	if s.Embedded != nil && embedded.IsEmbeddedURL(url) {
		return s.Embedded.Get(url) != nil
	}

	if s.SpecialPages != nil && strings.HasPrefix(url, "Special:") {
		return s.SpecialPages.Has(strings.TrimPrefix(url, "Special:"))
	}

	return false
}

func check(err error) {
	if err != nil {
		slog.Error("unexpected error", "error", err)
	}
}
