package main

import (
	"log/slog"
	"regexp"

	"github.com/danielledeleo/periwiki/db"
	"github.com/danielledeleo/periwiki/special"
	"github.com/danielledeleo/periwiki/templater"
	"github.com/danielledeleo/periwiki/wiki"
	"github.com/microcosm-cc/bluemonday"
)

func Setup() *app {
	modelConf := SetupConfig()

	bm := bluemonday.UGCPolicy()

	bm.AllowAttrs("class").Matching(regexp.MustCompile("^sourceCode(| [a-zA-Z0-9]+)(| lineNumbers)$")).
		OnElements("pre", "code")
	bm.AllowAttrs("class").Matching(regexp.MustCompile(`^infobox$`)).OnElements("div")
	bm.AllowAttrs("data-line-number", "class").Matching(regexp.MustCompile("^[0-9]+$")).OnElements("a")
	bm.AllowAttrs("style").OnElements("ins", "del")
	bm.AllowAttrs("class").Matching(regexp.MustCompile(`^footnote-ref$`)).OnElements("a")
	bm.AllowAttrs("class").Matching(regexp.MustCompile(`^footnotes$`)).OnElements("section")
	bm.AllowAttrs("style").Matching(regexp.MustCompile(`^text-align:\s+(left|right|center);$`)).OnElements("td", "th")
	bm.AllowAttrs("class").Matching(regexp.MustCompile(`^pw-deadlink$`)).OnElements("a")

	t := templater.New()

	if err := t.Load("templates/layouts/*.html", "templates/*.html"); err != nil {
		slog.Error("failed to load templates", "error", err)
	}

	database, err := db.Init(modelConf)
	check(err)
	model := wiki.New(database, modelConf, bm)

	specialPages := special.NewRegistry()
	specialPages.Register("Random", special.NewRandomPage(model))

	return &app{t, model, specialPages}
}
