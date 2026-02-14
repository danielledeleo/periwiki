package server

import (
	"context"
	"database/sql"
	_ "embed"
	"io/fs"
	"log/slog"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/danielledeleo/periwiki/extensions"
	"github.com/danielledeleo/periwiki/internal/config"
	"github.com/danielledeleo/periwiki/internal/embedded"
	"github.com/danielledeleo/periwiki/internal/renderqueue"
	"github.com/danielledeleo/periwiki/internal/storage"
	"github.com/danielledeleo/periwiki/render"
	"github.com/danielledeleo/periwiki/special"
	"github.com/danielledeleo/periwiki/templater"
	"github.com/danielledeleo/periwiki/wiki"
	"github.com/danielledeleo/periwiki/wiki/repository"
	"github.com/danielledeleo/periwiki/wiki/service"
	"github.com/jmoiron/sqlx"
	"github.com/microcosm-cc/bluemonday"
)

//go:embed default_main_page.md
var defaultMainPageContent string

// Setup initializes the application and returns the App instance along with
// the render queue (which must be shut down when the server stops).
func Setup(contentFS fs.FS, contentInfo *ContentInfo) (*App, *renderqueue.Queue) {
	// Phase 1: Load file-based config
	modelConf := config.SetupConfig()

	// Phase 2: Open database connection
	db, err := sqlx.Open("sqlite3", modelConf.DatabaseFile)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}

	// Phase 3: Run migrations
	if err := storage.RunMigrations(db); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	// Phase 4: Load runtime config from database
	runtimeConfig, err := wiki.LoadRuntimeConfig(db.DB)
	if err != nil {
		slog.Error("failed to load runtime config", "error", err)
		os.Exit(1)
	}

	// Phase 5: Initialize storage with runtime config
	database, err := storage.Init(db, runtimeConfig)
	check(err)

	bm := bluemonday.UGCPolicy()

	bm.AllowAttrs("class").Globally()
	bm.AllowAttrs("data-line-number").Matching(regexp.MustCompile("^[0-9]+$")).OnElements("a")
	bm.AllowAttrs("style").OnElements("ins", "del")
	bm.AllowAttrs("style").Matching(regexp.MustCompile(`^text-align:\s+(left|right|center);$`)).OnElements("td", "th")

	// Allow checkbox and label for TOC toggle
	bm.AllowElements("input", "label")
	bm.AllowAttrs("type", "id", "class", "checked").OnElements("input")
	bm.AllowAttrs("for").OnElements("label")

	t := templater.New(contentFS)

	if err := t.Load("templates/layouts/*.html", "templates/*.html", "templates/special/*.html", "templates/manage/*.html"); err != nil {
		slog.Error("failed to load templates", "error", err)
		os.Exit(1)
	}

	// Load extension templates
	footnoteTemplates, err := t.LoadExtensionTemplates("templates", "footnote", []string{
		"link", "backlink", "list", "item",
	})
	if err != nil {
		slog.Error("failed to load footnote templates", "error", err)
		os.Exit(1)
	}

	wikiLinkTemplates, err := t.LoadExtensionTemplates("templates", "wikilink", []string{
		"link",
	})
	if err != nil {
		slog.Error("failed to load wikilink templates", "error", err)
		os.Exit(1)
	}

	// Declare these before the checker so the closure captures them by reference.
	// They are assigned real values later in this function.
	var embeddedArticles *embedded.EmbeddedArticles
	var specialPages *special.Registry

	// Create existence checker for wiki links
	existenceChecker := func(url string) bool {
		const prefix = "/wiki/"
		if len(url) > len(prefix) {
			url = url[len(prefix):]
		}

		// Check database
		article, _ := database.SelectArticle(url)
		if article != nil {
			return true
		}

		// Check embedded articles (Periwiki:* namespace)
		if embeddedArticles != nil && embedded.IsEmbeddedURL(url) {
			return embeddedArticles.Get(url) != nil
		}

		// Check special pages (Special:* namespace)
		if specialPages != nil && strings.HasPrefix(url, "Special:") {
			return specialPages.Has(strings.TrimPrefix(url, "Special:"))
		}

		return false
	}

	// Create renderer with extension templates
	renderer, err := render.NewHTMLRenderer(
		contentFS,
		existenceChecker,
		[]extensions.WikiLinkRendererOption{extensions.WithWikiLinkTemplates(wikiLinkTemplates)},
		[]extensions.FootnoteOption{extensions.WithFootnoteTemplates(footnoteTemplates)},
	)
	if err != nil {
		slog.Error("failed to create HTML renderer", "error", err)
		os.Exit(1)
	}

	// Create rendering service
	renderingService := service.NewRenderingService(renderer, bm)

	// Create render queue
	workerCount := runtimeConfig.RenderWorkers
	if workerCount == 0 {
		workerCount = runtime.NumCPU()
	}
	renderQueue := renderqueue.New(workerCount, renderingService.Render)
	slog.Info("render queue initialized", "workers", workerCount)

	// Create session service
	sessionService := service.NewSessionService(database)

	// Create user service
	userService := service.NewUserService(database, runtimeConfig.MinimumPasswordLength)

	// Create article service
	articleService := service.NewArticleService(database, renderingService, renderQueue)

	// Create embedded articles and wrap the article service.
	// contentFS has help/ at the root, which is what embedded.New expects.
	embeddedArticles, err = embedded.New(contentFS, renderingService.Render)
	if err != nil {
		slog.Error("failed to load embedded articles", "error", err)
		os.Exit(1)
	}
	articleService = service.NewEmbeddedArticleService(articleService, embeddedArticles)

	// Run first-time setup tasks (e.g. seed Main_Page)
	runFirstTimeSetup(db.DB, articleService)

	// Check for stale render templates and invalidate cached HTML if needed
	checkRenderTemplateStaleness(contentFS, db.DB, database, articleService)

	// Create preference service
	preferenceService := service.NewPreferenceService(database)

	specialPages = special.NewRegistry()
	specialPages.Register("Random", special.NewRandomPage(articleService))

	sitemapHandler := special.NewSitemapPage(articleService, t, modelConf.BaseURL)
	specialPages.Register("Sitemap", sitemapHandler)
	specialPages.Register("Sitemap.xml", sitemapHandler)
	specialPages.Register("RerenderAll", special.NewRerenderAllPage(articleService, t))

	// Re-render embedded articles now that the existence checker can see
	// embeddedArticles and specialPages (they were nil during initial render).
	if err := embeddedArticles.RenderAll(renderingService.Render); err != nil {
		slog.Error("failed to re-render embedded articles", "error", err)
		os.Exit(1)
	}

	// Log content override summary
	if contentInfo != nil {
		var overrideCount int
		for _, f := range contentInfo.Files {
			if f.Source == "disk" {
				slog.Debug("content override", "path", f.Path)
				overrideCount++
			}
		}
		slog.Info("content files loaded", "total", len(contentInfo.Files), "overrides", overrideCount)
	}

	return &App{
		Templater:     t,
		Articles:      articleService,
		Users:         userService,
		Sessions:      sessionService,
		Rendering:     renderingService,
		Preferences:   preferenceService,
		SpecialPages:  specialPages,
		Config:        modelConf,
		RuntimeConfig: runtimeConfig,
		ContentInfo:   contentInfo,
		DB:            db.DB,
	}, renderQueue
}

// checkRenderTemplateStaleness computes a hash of all render-time templates
// (templates/_render/) and compares it against the stored hash in the Setting
// table. If the templates have changed, it invalidates cached HTML for old
// revisions and queues head revisions for re-rendering.
func checkRenderTemplateStaleness(contentFS fs.FS, db *sql.DB, repo repository.ArticleRepository, articles service.ArticleService) {
	templateHash, err := render.HashRenderTemplates(contentFS, "templates/_render")
	if err != nil {
		slog.Error("failed to hash render templates", "error", err)
		return
	}
	// Include the build commit so code changes to the rendering pipeline
	// (not just template changes) also trigger a re-render.
	currentHash := templateHash + ":" + embedded.BuildCommit

	storedHash, err := wiki.GetOrCreateSetting(db, wiki.SettingRenderTemplateHash, func() string {
		return currentHash
	})
	if err != nil {
		slog.Error("failed to read render template hash setting", "error", err)
		return
	}

	if storedHash == currentHash {
		slog.Debug("render templates unchanged")
		return
	}

	// Split "templateHash:buildCommit" to log which part changed.
	storedTemplate, storedCommit, _ := strings.Cut(storedHash, ":")
	templatesChanged := storedTemplate != templateHash
	binaryChanged := storedCommit != embedded.BuildCommit

	if templatesChanged {
		slog.Info("render templates changed", "old", storedTemplate[:12], "new", templateHash[:12])
	}
	if binaryChanged {
		slog.Info("binary changed", "old", storedCommit[:7], "new", embedded.BuildCommit[:7])
	}

	slog.Info("invalidating cached HTML")

	// Null out HTML for all non-head revisions
	invalidated, err := repo.InvalidateNonHeadRevisionHTML()
	if err != nil {
		slog.Error("failed to invalidate old revision HTML", "error", err)
		return
	}
	slog.Info("invalidated old revision HTML", "count", invalidated)

	// Queue re-render of all head revisions
	allArticles, err := articles.GetAllArticles()
	if err != nil {
		slog.Error("failed to get articles for re-render", "error", err)
		return
	}

	var queued int
	for _, article := range allArticles {
		_, err := articles.QueueRerenderRevision(context.Background(), article.URL, 0)
		if err != nil {
			slog.Error("failed to queue head revision re-render", "url", article.URL, "error", err)
			continue
		}
		queued++
	}
	slog.Info("queued head revision re-renders", "count", queued)

	// Update the stored hash
	if err := wiki.UpdateSetting(db, wiki.SettingRenderTemplateHash, currentHash); err != nil {
		slog.Error("failed to update render template hash setting", "error", err)
	}
}

const currentSetupVersion = 1

// runFirstTimeSetup executes versioned one-time setup tasks.
// Each task runs only once; the current version is persisted in the Setting table.
func runFirstTimeSetup(db *sql.DB, articles service.ArticleService) {
	stored := getSetupVersion(db)
	if stored >= currentSetupVersion {
		return
	}

	if stored < 1 {
		seedMainPage(articles)
	}
	// Future: if stored < 2 { ... }

	updateSetupVersion(db, currentSetupVersion)
}

func getSetupVersion(db *sql.DB) int {
	var value string
	err := db.QueryRow("SELECT value FROM Setting WHERE key = ?", wiki.SettingSetupVersion).Scan(&value)
	if err != nil {
		return 0
	}
	v, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return v
}

func updateSetupVersion(db *sql.DB, version int) {
	value := strconv.Itoa(version)
	if err := wiki.UpdateSetting(db, wiki.SettingSetupVersion, value); err != nil {
		slog.Error("failed to update setup version", "error", err)
	}
}

func seedMainPage(articles service.ArticleService) {
	// Check if Main_Page already exists
	existing, err := articles.GetArticle("Main_Page")
	if err == nil && existing != nil {
		slog.Debug("Main_Page already exists, skipping seed")
		return
	}

	article := wiki.NewArticle("Main_Page", defaultMainPageContent)
	article.Creator = &wiki.User{ID: 0}
	if err := articles.PostArticle(article); err != nil {
		slog.Error("failed to seed Main_Page", "error", err)
		return
	}
	slog.Info("seeded Main_Page article")
}
