package main

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	periwiki "github.com/danielledeleo/periwiki"
	"github.com/danielledeleo/periwiki/internal/embedded"
	"github.com/danielledeleo/periwiki/internal/server"
	"github.com/gorilla/mux"
)

func main() {
	// Build content info from the embedded/overlay filesystem
	files, err := periwiki.ListContentFiles()
	if err != nil {
		slog.Error("failed to list content files", "error", err)
		os.Exit(1)
	}

	contentInfo := &server.ContentInfo{
		BuildCommit: embedded.BuildCommit,
		SourceURL:   embedded.SourceBaseURL,
	}
	for _, f := range files {
		contentInfo.Files = append(contentInfo.Files, server.ContentFileEntry{
			Path:   f.Path,
			Source: f.Source,
		})
	}

	app, renderQueue := server.Setup(periwiki.ContentFS, contentInfo)

	router := mux.NewRouter().StrictSlash(true)

	router.Use(app.SessionMiddleware)

	// Routes are documented in docs/urls.md — update it when adding or changing routes.
	staticSub, _ := fs.Sub(periwiki.ContentFS, "static")
	staticFS := http.FileServer(http.FS(staticSub))
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", staticFS))
	router.HandleFunc("/", app.HomeHandler).Methods("GET")

	router.HandleFunc("/wiki/{namespace:[^:/]+}:{page}", app.NamespaceHandler).Methods("GET", "POST")

	router.HandleFunc("/wiki/{article}", app.ArticleDispatcher).Methods("GET", "POST")

	router.HandleFunc("/user/register", app.RegisterHandler).Methods("GET")
	router.HandleFunc("/user/register", app.RegisterPostHandler).Methods("POST")
	router.HandleFunc("/user/login", app.LoginHandler).Methods("GET")
	router.HandleFunc("/user/login", app.LoginPostHandler).Methods("POST")
	router.HandleFunc("/user/logout", app.LogoutPostHandler).Methods("POST")

	router.HandleFunc("/manage/users", app.ManageUsersHandler).Methods("GET")
	router.HandleFunc("/manage/users/{id:[0-9]+}", app.ManageUserRoleHandler).Methods("POST")
	router.HandleFunc("/manage/settings", app.ManageSettingsHandler).Methods("GET")
	router.HandleFunc("/manage/settings", app.ManageSettingsPostHandler).Methods("POST")
	router.HandleFunc("/manage/content", app.ManageContentHandler).Methods("GET")

	handler := server.SlogLoggingMiddleware(router)

	srv := &http.Server{
		Addr:    app.Config.Host,
		Handler: handler,
	}

	// Start server in goroutine
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	commit := embedded.BuildCommit
	if len(commit) > 12 {
		commit = commit[:12]
	}
	slog.Info("server starting", "url", "http://"+app.Config.Host, "commit", commit)

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown HTTP server first (stop accepting new requests)
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}

	// Shutdown render queue (wait for in-flight jobs)
	slog.Info("shutting down render queue...")
	if err := renderQueue.Shutdown(ctx); err != nil {
		slog.Error("render queue shutdown error", "error", err)
	}

	slog.Info("server stopped")
}
