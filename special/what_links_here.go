package special

import (
	"io"
	"log/slog"
	"net/http"

	"github.com/danielledeleo/periwiki/wiki"
)

// BacklinkLister retrieves backlinks for a given article slug.
type BacklinkLister interface {
	GetBacklinks(slug string) ([]*wiki.ArticleSummary, error)
	GetArticle(url string) (*wiki.Article, error)
}

// WhatLinksHereTemplater renders the what_links_here template.
type WhatLinksHereTemplater interface {
	RenderTemplate(w io.Writer, name string, base string, data map[string]interface{}) error
}

// WhatLinksHerePage handles Special:WhatLinksHere requests.
type WhatLinksHerePage struct {
	backlinks BacklinkLister
	templater WhatLinksHereTemplater
}

// NewWhatLinksHerePage creates a new WhatLinksHere special page handler.
func NewWhatLinksHerePage(backlinks BacklinkLister, templater WhatLinksHereTemplater) *WhatLinksHerePage {
	return &WhatLinksHerePage{
		backlinks: backlinks,
		templater: templater,
	}
}

// Handle serves the "What links here" page.
func (p *WhatLinksHerePage) Handle(rw http.ResponseWriter, req *http.Request) {
	data := map[string]interface{}{
		"Page":    wiki.NewStaticPage("What links here?"),
		"Context": req.Context(),
	}

	slug := req.URL.Query().Get("page")
	if slug == "" {
		// Show the search form
		if err := p.templater.RenderTemplate(rw, "what_links_here.html", "index.html", data); err != nil {
			slog.Error("failed to render what_links_here template", "error", err)
			http.Error(rw, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	backlinks, err := p.backlinks.GetBacklinks(slug)
	if err != nil {
		slog.Error("failed to get backlinks", "slug", slug, "error", err)
		http.Error(rw, "Internal server error", http.StatusInternalServerError)
		return
	}

	_, getErr := p.backlinks.GetArticle(slug)
	data["TargetSlug"] = slug
	data["TargetTitle"] = wiki.InferTitle(slug)
	data["TargetExists"] = getErr == nil
	data["Backlinks"] = backlinks

	if err := p.templater.RenderTemplate(rw, "what_links_here.html", "index.html", data); err != nil {
		slog.Error("failed to render what_links_here template", "error", err)
		http.Error(rw, "Internal server error", http.StatusInternalServerError)
	}
}
