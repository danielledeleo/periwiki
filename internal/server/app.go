package server

import (
	"database/sql"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielledeleo/periwiki/internal/embedded"
	"github.com/danielledeleo/periwiki/special"
	"github.com/danielledeleo/periwiki/templater"
	"github.com/danielledeleo/periwiki/wiki"
	"github.com/danielledeleo/periwiki/wiki/service"
	"github.com/gorilla/mux"
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

	router.HandleFunc("/source.tar.gz", func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Content-Type", "application/gzip")
		rw.Header().Set("Content-Disposition", "attachment; filename=periwiki-source.tar.gz")
		rw.Write(embedded.SourceTarball)
	}).Methods("GET")

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
	router.HandleFunc("/manage/settings/reset-main-page", a.ResetMainPageHandler).Methods("POST")
	router.HandleFunc("/manage/content", a.ManageContentHandler).Methods("GET")
}

func check(err error) {
	if err != nil {
		slog.Error("unexpected error", "error", err)
	}
}
