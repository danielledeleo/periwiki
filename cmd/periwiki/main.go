package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/danielledeleo/periwiki/internal/server"
	"github.com/gorilla/mux"
)

func main() {
	app, renderQueue := server.Setup()

	router := mux.NewRouter().StrictSlash(true)

	router.Use(app.SessionMiddleware)

	// Routes are documented in docs/urls.md — update it when adding or changing routes.
	fs := http.FileServer(http.Dir("./static"))
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fs))
	router.HandleFunc("/", app.HomeHandler).Methods("GET")

	router.HandleFunc("/wiki/Special:{page}", app.SpecialPageHandler).Methods("GET")

	router.HandleFunc("/wiki/{article}", app.ArticleDispatcher).Methods("GET", "POST")

	router.HandleFunc("/user/register", app.RegisterHandler).Methods("GET")
	router.HandleFunc("/user/register", app.RegisterPostHandler).Methods("POST")
	router.HandleFunc("/user/login", app.LoginHandler).Methods("GET")
	router.HandleFunc("/user/login", app.LoginPostHandler).Methods("POST")
	router.HandleFunc("/user/logout", app.LogoutPostHandler).Methods("POST")

	manageRouter := mux.NewRouter().PathPrefix("/manage").Subrouter()
	manageRouter.HandleFunc("/{page}", func(rw http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		fmt.Fprintln(rw, vars["page"])
	})
	router.Handle("/manage/{page}", manageRouter)

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

	slog.Info("server starting", "url", "http://"+app.Config.Host)

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
