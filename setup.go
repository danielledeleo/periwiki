package main

import (
	"log"
	"regexp"

	"github.com/jagger27/iwikii/db"
	"github.com/jagger27/iwikii/model"
	"github.com/jagger27/iwikii/templater"
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

	t := templater.New()

	if err := t.Load("templates/layouts/*.html", "templates/*.html"); err != nil {
		log.Println(err)
	}

	database, err := db.Init(modelConf)
	check(err)
	model := model.New(database, modelConf, bm)
	return &app{t, model}
}
