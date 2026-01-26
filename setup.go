package main

import (
	"log/slog"
	"os"
	"regexp"

	"github.com/danielledeleo/periwiki/extensions"
	"github.com/danielledeleo/periwiki/internal/storage"
	"github.com/danielledeleo/periwiki/render"
	"github.com/danielledeleo/periwiki/special"
	"github.com/danielledeleo/periwiki/templater"
	"github.com/danielledeleo/periwiki/wiki/service"
	"github.com/microcosm-cc/bluemonday"
)

func Setup() *app {
	modelConf := SetupConfig()

	bm := bluemonday.UGCPolicy()

	bm.AllowAttrs("class").Globally()
	bm.AllowAttrs("data-line-number").Matching(regexp.MustCompile("^[0-9]+$")).OnElements("a")
	bm.AllowAttrs("style").OnElements("ins", "del")
	bm.AllowAttrs("style").Matching(regexp.MustCompile(`^text-align:\s+(left|right|center);$`)).OnElements("td", "th")

	// Allow checkbox and label for TOC toggle
	bm.AllowElements("input", "label")
	bm.AllowAttrs("type", "id", "class", "checked").OnElements("input")
	bm.AllowAttrs("for").OnElements("label")

	t := templater.New()

	if err := t.Load("templates/layouts/*.html", "templates/*.html", "templates/special/*.html"); err != nil {
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

	database, err := storage.Init(modelConf)
	check(err)

	// Create existence checker for wiki links
	existenceChecker := func(url string) bool {
		const prefix = "/wiki/"
		if len(url) > len(prefix) {
			url = url[len(prefix):]
		}
		article, _ := database.SelectArticle(url)
		return article != nil
	}

	// Create renderer with extension templates
	renderer := render.NewHTMLRenderer(
		existenceChecker,
		[]extensions.WikiLinkRendererOption{extensions.WithWikiLinkTemplates(wikiLinkTemplates)},
		[]extensions.FootnoteOption{extensions.WithFootnoteTemplates(footnoteTemplates)},
	)

	// Create rendering service
	renderingService := service.NewRenderingService(renderer, bm)

	// Create session service
	sessionService := service.NewSessionService(database)

	// Create user service
	userService := service.NewUserService(database, modelConf.MinimumPasswordLength)

	// Create article service
	articleService := service.NewArticleService(database, renderingService)

	// Create preference service
	preferenceService := service.NewPreferenceService(database)

	specialPages := special.NewRegistry()
	specialPages.Register("Random", special.NewRandomPage(articleService))

	sitemapHandler := special.NewSitemapPage(articleService, t, modelConf.BaseURL)
	specialPages.Register("Sitemap", sitemapHandler)
	specialPages.Register("Sitemap.xml", sitemapHandler)

	return &app{
		Templater:    t,
		articles:     articleService,
		users:        userService,
		sessions:     sessionService,
		rendering:    renderingService,
		preferences:  preferenceService,
		specialPages: specialPages,
		config:       modelConf,
	}
}
