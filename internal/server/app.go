package server

import (
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielledeleo/periwiki/special"
	"github.com/danielledeleo/periwiki/templater"
	"github.com/danielledeleo/periwiki/wiki"
	"github.com/danielledeleo/periwiki/wiki/service"
)

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

func check(err error) {
	if err != nil {
		slog.Error("unexpected error", "error", err)
	}
}
