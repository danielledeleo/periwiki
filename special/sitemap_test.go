package special

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielledeleo/periwiki/wiki"
)

// mockArticleLister implements ArticleLister for testing.
type mockArticleLister struct {
	articles []*wiki.ArticleSummary
	err      error
}

func (m *mockArticleLister) GetAllArticles() ([]*wiki.ArticleSummary, error) {
	return m.articles, m.err
}

// mockSitemapTemplater implements SitemapTemplater for testing.
type mockSitemapTemplater struct {
	rendered bool
	data     map[string]interface{}
	err      error
}

func (m *mockSitemapTemplater) RenderTemplate(w io.Writer, name string, base string, data map[string]interface{}) error {
	m.rendered = true
	m.data = data
	if m.err != nil {
		return m.err
	}
	w.Write([]byte("rendered"))
	return nil
}

func TestSitemapXML(t *testing.T) {
	t.Run("returns valid XML sitemap", func(t *testing.T) {
		articles := []*wiki.ArticleSummary{
			{URL: "Main_Page", LastModified: time.Date(2026, 1, 20, 15, 30, 0, 0, time.UTC)},
			{URL: "Test_Article", LastModified: time.Date(2026, 1, 21, 10, 0, 0, 0, time.UTC)},
		}
		mock := &mockArticleLister{articles: articles}
		handler := NewSitemapPage(mock, nil, "https://example.com")

		req := httptest.NewRequest("GET", "/wiki/Special:Sitemap.xml", nil)
		rr := httptest.NewRecorder()

		handler.Handle(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		contentType := rr.Header().Get("Content-Type")
		if !strings.Contains(contentType, "application/xml") {
			t.Errorf("expected Content-Type application/xml, got %q", contentType)
		}

		body := rr.Body.String()

		// Check XML declaration
		if !strings.HasPrefix(body, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>") {
			t.Error("expected XML declaration")
		}

		// Check namespace
		if !strings.Contains(body, `xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"`) {
			t.Error("expected sitemaps.org namespace")
		}

		// Check URLs
		if !strings.Contains(body, "<loc>https://example.com/wiki/Main_Page</loc>") {
			t.Error("expected Main_Page URL")
		}
		if !strings.Contains(body, "<loc>https://example.com/wiki/Test_Article</loc>") {
			t.Error("expected Test_Article URL")
		}

		// Check lastmod format
		if !strings.Contains(body, "<lastmod>2026-01-20T15:30:00Z</lastmod>") {
			t.Error("expected lastmod in ISO 8601 format")
		}
	})

	t.Run("handles empty article list", func(t *testing.T) {
		mock := &mockArticleLister{articles: []*wiki.ArticleSummary{}}
		handler := NewSitemapPage(mock, nil, "https://example.com")

		req := httptest.NewRequest("GET", "/wiki/Special:Sitemap.xml", nil)
		rr := httptest.NewRecorder()

		handler.Handle(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		body := rr.Body.String()
		if !strings.Contains(body, "<urlset") {
			t.Error("expected urlset element")
		}
	})

	t.Run("strips trailing slash from baseURL", func(t *testing.T) {
		articles := []*wiki.ArticleSummary{
			{URL: "Test", LastModified: time.Now()},
		}
		mock := &mockArticleLister{articles: articles}
		handler := NewSitemapPage(mock, nil, "https://example.com/")

		req := httptest.NewRequest("GET", "/wiki/Special:Sitemap.xml", nil)
		rr := httptest.NewRecorder()

		handler.Handle(rr, req)

		body := rr.Body.String()
		if strings.Contains(body, "https://example.com//wiki/") {
			t.Error("should not have double slashes in URL")
		}
		if !strings.Contains(body, "https://example.com/wiki/Test") {
			t.Error("expected properly formed URL")
		}
	})
}

func TestSitemapHTML(t *testing.T) {
	t.Run("renders HTML template", func(t *testing.T) {
		articles := []*wiki.ArticleSummary{
			{URL: "Main_Page", LastModified: time.Now()},
		}
		mock := &mockArticleLister{articles: articles}
		templater := &mockSitemapTemplater{}
		handler := NewSitemapPage(mock, templater, "https://example.com")

		req := httptest.NewRequest("GET", "/wiki/Special:Sitemap", nil)
		rr := httptest.NewRecorder()

		handler.Handle(rr, req)

		if !templater.rendered {
			t.Error("expected template to be rendered")
		}

		entries, ok := templater.data["Entries"].([]SitemapEntry)
		if !ok {
			t.Fatal("expected Entries in template data")
		}
		if len(entries) != 1 || entries[0].Article.URL != "Main_Page" {
			t.Errorf("expected 1 entry for Main_Page, got %v", entries)
		}
	})

	t.Run("pairs talk pages with their subject article", func(t *testing.T) {
		articles := []*wiki.ArticleSummary{
			{URL: "Main_Page", LastModified: time.Now()},
			{URL: "Talk:Main_Page", LastModified: time.Now()},
			{URL: "Test_Article", LastModified: time.Now()},
		}
		mock := &mockArticleLister{articles: articles}
		templater := &mockSitemapTemplater{}
		handler := NewSitemapPage(mock, templater, "https://example.com")

		req := httptest.NewRequest("GET", "/wiki/Special:Sitemap", nil)
		rr := httptest.NewRecorder()

		handler.Handle(rr, req)

		entries, ok := templater.data["Entries"].([]SitemapEntry)
		if !ok {
			t.Fatal("expected Entries in template data")
		}
		if len(entries) != 2 {
			t.Fatalf("expected 2 entries (Talk pages excluded), got %d", len(entries))
		}
		if entries[0].Article.URL != "Main_Page" || entries[0].TalkURL != "Talk:Main_Page" {
			t.Errorf("expected Main_Page with Talk:Main_Page, got %q with talk %q", entries[0].Article.URL, entries[0].TalkURL)
		}
		if entries[1].Article.URL != "Test_Article" || entries[1].TalkURL != "" {
			t.Errorf("expected Test_Article with no talk page, got %q with talk %q", entries[1].Article.URL, entries[1].TalkURL)
		}
	})

	t.Run("returns 500 on template error", func(t *testing.T) {
		mock := &mockArticleLister{articles: []*wiki.ArticleSummary{}}
		templater := &mockSitemapTemplater{err: errors.New("template error")}
		handler := NewSitemapPage(mock, templater, "https://example.com")

		req := httptest.NewRequest("GET", "/wiki/Special:Sitemap", nil)
		rr := httptest.NewRecorder()

		handler.Handle(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

func TestSitemapMarkdown(t *testing.T) {
	t.Run("returns markdown list of articles", func(t *testing.T) {
		articles := []*wiki.ArticleSummary{
			{URL: "Main_Page", LastModified: time.Now()},
			{URL: "Test_Article", Title: "Custom Title", LastModified: time.Now()},
		}
		mock := &mockArticleLister{articles: articles}
		handler := NewSitemapPage(mock, nil, "https://example.com")

		req := httptest.NewRequest("GET", "/wiki/Special:Sitemap.md", nil)
		rr := httptest.NewRecorder()

		handler.Handle(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		contentType := rr.Header().Get("Content-Type")
		if contentType != "text/plain; charset=utf-8" {
			t.Errorf("expected Content-Type text/plain; charset=utf-8, got %q", contentType)
		}

		body := rr.Body.String()

		if !strings.Contains(body, "# Sitemap") {
			t.Error("expected markdown heading")
		}
		if !strings.Contains(body, "[Main Page](https://example.com/wiki/Main_Page)") {
			t.Errorf("expected Main_Page link with inferred title, got %q", body)
		}
		if !strings.Contains(body, "[Custom Title](https://example.com/wiki/Test_Article)") {
			t.Errorf("expected Test_Article link with custom title, got %q", body)
		}
	})

	t.Run("excludes talk pages", func(t *testing.T) {
		articles := []*wiki.ArticleSummary{
			{URL: "Main_Page", LastModified: time.Now()},
			{URL: "Talk:Main_Page", LastModified: time.Now()},
		}
		mock := &mockArticleLister{articles: articles}
		handler := NewSitemapPage(mock, nil, "https://example.com")

		req := httptest.NewRequest("GET", "/wiki/Special:Sitemap.md", nil)
		rr := httptest.NewRecorder()

		handler.Handle(rr, req)

		body := rr.Body.String()
		if strings.Contains(body, "Talk:") {
			t.Errorf("expected talk pages to be excluded, got %q", body)
		}
	})

	t.Run("handles empty article list", func(t *testing.T) {
		mock := &mockArticleLister{articles: []*wiki.ArticleSummary{}}
		handler := NewSitemapPage(mock, nil, "https://example.com")

		req := httptest.NewRequest("GET", "/wiki/Special:Sitemap.md", nil)
		rr := httptest.NewRecorder()

		handler.Handle(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}

		body := rr.Body.String()
		if !strings.Contains(body, "# Sitemap") {
			t.Error("expected markdown heading even with no articles")
		}
	})
}

func TestSitemapErrors(t *testing.T) {
	t.Run("returns 500 on database error", func(t *testing.T) {
		mock := &mockArticleLister{err: errors.New("database error")}
		handler := NewSitemapPage(mock, nil, "https://example.com")

		req := httptest.NewRequest("GET", "/wiki/Special:Sitemap.xml", nil)
		rr := httptest.NewRecorder()

		handler.Handle(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}

// Ensure mockSitemapTemplater's Write is compatible with http.ResponseWriter
var _ io.Writer = (*httptest.ResponseRecorder)(nil)
