package special

import (
	"encoding/xml"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/danielledeleo/periwiki/wiki"
)

// ArticleLister is the interface needed by SitemapPage.
type ArticleLister interface {
	GetAllArticles() ([]*wiki.ArticleSummary, error)
}

// SitemapTemplater renders the HTML sitemap template.
type SitemapTemplater interface {
	RenderTemplate(w io.Writer, name string, base string, data map[string]interface{}) error
}

// SitemapPage handles Special:Sitemap and Special:Sitemap.xml requests.
type SitemapPage struct {
	lister    ArticleLister
	templater SitemapTemplater
	baseURL   string
}

// NewSitemapPage creates a new Sitemap special page handler.
func NewSitemapPage(lister ArticleLister, templater SitemapTemplater, baseURL string) *SitemapPage {
	return &SitemapPage{
		lister:    lister,
		templater: templater,
		baseURL:   strings.TrimSuffix(baseURL, "/"),
	}
}

// Handle serves the sitemap in XML or HTML format based on URL.
func (p *SitemapPage) Handle(rw http.ResponseWriter, req *http.Request) {
	articles, err := p.lister.GetAllArticles()
	if err != nil {
		slog.Error("failed to get articles for sitemap", "category", "special", "error", err)
		http.Error(rw, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Detect format from URL path
	if strings.HasSuffix(req.URL.Path, ".xml") {
		p.handleXML(rw, articles)
	} else {
		p.handleHTML(rw, req, articles)
	}
}

// xmlURLSet represents the sitemap XML structure.
type xmlURLSet struct {
	XMLName xml.Name `xml:"urlset"`
	XMLNS   string   `xml:"xmlns,attr"`
	URLs    []xmlURL `xml:"url"`
}

// xmlURL represents a single URL entry in the sitemap.
type xmlURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

func (p *SitemapPage) handleXML(rw http.ResponseWriter, articles []*wiki.ArticleSummary) {
	urlset := xmlURLSet{
		XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  make([]xmlURL, len(articles)),
	}

	for i, article := range articles {
		urlset.URLs[i] = xmlURL{
			Loc:     p.baseURL + "/wiki/" + article.URL,
			LastMod: article.LastModified.UTC().Format("2006-01-02T15:04:05Z"),
		}
	}

	rw.Header().Set("Content-Type", "application/xml; charset=utf-8")
	rw.Write([]byte(xml.Header))
	encoder := xml.NewEncoder(rw)
	encoder.Indent("", "  ")
	if err := encoder.Encode(urlset); err != nil {
		slog.Error("failed to encode sitemap XML", "category", "special", "error", err)
	}
}

func (p *SitemapPage) handleHTML(rw http.ResponseWriter, req *http.Request, articles []*wiki.ArticleSummary) {
	data := map[string]interface{}{
		"Page":     wiki.NewStaticPage("Sitemap"),
		"Articles": articles,
		"Context":  req.Context(),
	}

	if err := p.templater.RenderTemplate(rw, "sitemap.html", "index.html", data); err != nil {
		slog.Error("failed to render sitemap template", "category", "special", "error", err)
		http.Error(rw, "Internal server error", http.StatusInternalServerError)
	}
}
