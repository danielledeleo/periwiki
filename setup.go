package main

import (
	"log/slog"
	"os"
	"regexp"

	"github.com/danielledeleo/periwiki/internal/storage"
	"github.com/danielledeleo/periwiki/extensions"
	"github.com/danielledeleo/periwiki/render"
	"github.com/danielledeleo/periwiki/special"
	"github.com/danielledeleo/periwiki/templater"
	"github.com/danielledeleo/periwiki/wiki"
	"github.com/microcosm-cc/bluemonday"
)

func Setup() *app {
	modelConf := SetupConfig()

	bm := bluemonday.UGCPolicy()

	bm.AllowAttrs("class").Globally()
	bm.AllowAttrs("data-line-number").Matching(regexp.MustCompile("^[0-9]+$")).OnElements("a")
	bm.AllowAttrs("style").OnElements("ins", "del")
	bm.AllowAttrs("style").Matching(regexp.MustCompile(`^text-align:\s+(left|right|center);$`)).OnElements("td", "th")

	t := templater.New()

	if err := t.Load("templates/layouts/*.html", "templates/*.html"); err != nil {
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

	model := wiki.New(database, modelConf, bm, renderer)

	specialPages := special.NewRegistry()
	specialPages.Register("Random", special.NewRandomPage(model))

	return &app{t, model, specialPages}
}
