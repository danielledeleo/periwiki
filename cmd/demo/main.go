//go:build js && wasm

package main

import (
	"log/slog"
	"syscall/js"

	periwiki "github.com/danielledeleo/periwiki"
	"github.com/danielledeleo/periwiki/extensions"
	"github.com/danielledeleo/periwiki/internal/embedded"
	"github.com/danielledeleo/periwiki/internal/server"
	"github.com/danielledeleo/periwiki/internal/storage"
	"github.com/danielledeleo/periwiki/render"
	"github.com/danielledeleo/periwiki/templater"
	"github.com/danielledeleo/periwiki/wiki"
	"github.com/danielledeleo/periwiki/wiki/service"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
)

func main() {
	contentFS := periwiki.ContentFS // uses embeddedFS via content_js.go

	// Open in-memory SQLite
	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		panic("failed to open database: " + err.Error())
	}

	// Run migrations
	if err := storage.RunMigrations(db, storage.DialectSQLite); err != nil {
		panic("failed to run migrations: " + err.Error())
	}

	// Load runtime config from DB (auto-seeds defaults)
	runtimeConfig, err := wiki.LoadRuntimeConfig(db.DB)
	if err != nil {
		panic("failed to load runtime config: " + err.Error())
	}
	runtimeConfig.AllowAnonymousEditsGlobal = true
	runtimeConfig.AllowSignups = true

	// Initialize storage
	database, err := storage.Init(db, runtimeConfig)
	if err != nil {
		panic("failed to init storage: " + err.Error())
	}

	bm := server.NewSanitizer()

	// Load templates
	t := templater.New(contentFS)
	if err := t.Load("templates/layouts/*.html", "templates/*.html", "templates/special/*.html", "templates/manage/*.html"); err != nil {
		panic("failed to load templates: " + err.Error())
	}

	footnoteTemplates, err := t.LoadExtensionTemplates("templates", "footnote", []string{
		"link", "backlink", "list", "item",
	})
	if err != nil {
		panic("failed to load footnote templates: " + err.Error())
	}

	wikiLinkTemplates, err := t.LoadExtensionTemplates("templates", "wikilink", []string{
		"link",
	})
	if err != nil {
		panic("failed to load wikilink templates: " + err.Error())
	}

	// Create existence checker for wiki links.
	// Embedded and SpecialPages are set after creation (circular dependency).
	existenceChecker, existenceState := server.NewExistenceChecker(database)

	renderer, err := render.NewHTMLRenderer(
		contentFS,
		existenceChecker,
		[]extensions.WikiLinkRendererOption{extensions.WithWikiLinkTemplates(wikiLinkTemplates)},
		[]extensions.FootnoteOption{extensions.WithFootnoteTemplates(footnoteTemplates)},
	)
	if err != nil {
		panic("failed to create renderer: " + err.Error())
	}

	renderingService := service.NewRenderingService(renderer, bm)
	sessionService := service.NewSessionService(database)
	userService := service.NewUserService(database, runtimeConfig.MinimumPasswordLength)
	articleService := service.NewArticleService(database, renderingService, nil, database, render.NewLinkExtractor()) // nil queue = synchronous

	embeddedArticles, err := embedded.New(contentFS, renderingService.Render)
	if err != nil {
		panic("failed to load embedded articles: " + err.Error())
	}
	existenceState.Embedded = embeddedArticles
	articleServiceWrapped := service.NewEmbeddedArticleService(articleService, embeddedArticles)

	// Run first-time setup (creates demo admin + seeds Main_Page)
	runDemoSetup(userService, articleServiceWrapped)

	preferenceService := service.NewPreferenceService(database)

	specialPages := server.RegisterSpecialPages(articleServiceWrapped, t, "")
	existenceState.SpecialPages = specialPages

	// Re-render embedded articles so the existence checker can see everything
	if err := embeddedArticles.RenderAll(renderingService.Render); err != nil {
		slog.Error("failed to re-render embedded articles", "error", err)
	}

	// Build content info from the embedded filesystem
	files, err := periwiki.ListContentFiles()
	if err != nil {
		slog.Error("failed to list content files", "error", err)
	}
	contentInfo := &server.ContentInfo{}
	for _, f := range files {
		contentInfo.Files = append(contentInfo.Files, server.ContentFileEntry{
			Path:   f.Path,
			Source: f.Source,
		})
	}

	app := &server.App{
		Templater:     t,
		Articles:      articleServiceWrapped,
		Users:         userService,
		Sessions:      sessionService,
		Rendering:     renderingService,
		Preferences:   preferenceService,
		SpecialPages:  specialPages,
		Config:        &wiki.Config{Host: "localhost", BaseURL: ""},
		RuntimeConfig: runtimeConfig,
		ContentInfo:   contentInfo,
		DB:            db.DB,
	}

	router := mux.NewRouter().StrictSlash(true)
	app.RegisterRoutes(router, contentFS)

	// Expose request handler to JS
	js.Global().Set("__periwikiHandleRequest", js.FuncOf(func(this js.Value, args []js.Value) any {
		return handleRequest(router, args[0])
	}))

	// Signal readiness
	if cb := js.Global().Get("__periwikiReady"); !cb.IsUndefined() {
		cb.Invoke()
	}

	slog.Info("periwiki demo ready")

	// Block forever
	select {}
}
