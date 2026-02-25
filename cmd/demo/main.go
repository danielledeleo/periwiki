//go:build js && wasm

package main

import (
	"io/fs"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"syscall/js"

	periwiki "github.com/danielledeleo/periwiki"
	"github.com/danielledeleo/periwiki/extensions"
	"github.com/danielledeleo/periwiki/internal/embedded"
	"github.com/danielledeleo/periwiki/internal/server"
	"github.com/danielledeleo/periwiki/internal/storage"
	"github.com/danielledeleo/periwiki/render"
	"github.com/danielledeleo/periwiki/special"
	"github.com/danielledeleo/periwiki/templater"
	"github.com/danielledeleo/periwiki/wiki"
	"github.com/danielledeleo/periwiki/wiki/service"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"github.com/microcosm-cc/bluemonday"
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

	// Bluemonday sanitizer (matches production setup.go)
	bm := bluemonday.UGCPolicy()
	bm.AllowAttrs("class").Globally()
	bm.AllowAttrs("data-line-number").Matching(regexp.MustCompile("^[0-9]+$")).OnElements("a")
	bm.AllowAttrs("style").OnElements("ins", "del")
	bm.AllowAttrs("style").Matching(regexp.MustCompile(`^text-align:\s+(left|right|center);$`)).OnElements("td", "th")
	bm.AllowElements("input", "label")
	bm.AllowAttrs("type", "id", "class", "checked").OnElements("input")
	bm.AllowAttrs("for").OnElements("label")

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

	var embeddedArticles *embedded.EmbeddedArticles
	var specialPages *special.Registry

	existenceChecker := func(url string) bool {
		const prefix = "/wiki/"
		if len(url) > len(prefix) {
			url = url[len(prefix):]
		}
		article, _ := database.SelectArticle(url)
		if article != nil {
			return true
		}
		if embeddedArticles != nil && embedded.IsEmbeddedURL(url) {
			return embeddedArticles.Get(url) != nil
		}
		if specialPages != nil && strings.HasPrefix(url, "Special:") {
			return specialPages.Has(strings.TrimPrefix(url, "Special:"))
		}
		return false
	}

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
	articleService := service.NewArticleService(database, renderingService, nil) // nil queue = synchronous

	embeddedArticles, err = embedded.New(contentFS, renderingService.Render)
	if err != nil {
		panic("failed to load embedded articles: " + err.Error())
	}
	articleServiceWrapped := service.NewEmbeddedArticleService(articleService, embeddedArticles)

	// Run first-time setup (creates demo admin + seeds Main_Page)
	runDemoSetup(userService, articleServiceWrapped)

	preferenceService := service.NewPreferenceService(database)

	specialPages = special.NewRegistry()
	specialPages.Register("Random", special.NewRandomPage(articleServiceWrapped))
	specialPages.Register("Sitemap", special.NewSitemapPage(articleServiceWrapped, t, ""))
	specialPages.Register("RerenderAll", special.NewRerenderAllPage(articleServiceWrapped, t))

	// Re-render embedded articles so the existence checker can see everything
	if err := embeddedArticles.RenderAll(renderingService.Render); err != nil {
		slog.Error("failed to re-render embedded articles", "error", err)
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
		DB:            db.DB,
	}

	// Build router (mirrors cmd/periwiki/main.go)
	router := mux.NewRouter().StrictSlash(true)
	router.Use(app.SessionMiddleware)

	staticSub, _ := fs.Sub(contentFS, "static")
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
	router.HandleFunc("/manage/settings/reset-main-page", app.ResetMainPageHandler).Methods("POST")
	router.HandleFunc("/manage/content", app.ManageContentHandler).Methods("GET")

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
