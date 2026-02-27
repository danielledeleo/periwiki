package special

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/danielledeleo/periwiki/wiki"
)

// mockBacklinkLister implements BacklinkLister for testing.
type mockBacklinkLister struct {
	backlinks    []*wiki.ArticleSummary
	err          error
	lastSlug     string
	articleExists bool
}

func (m *mockBacklinkLister) GetBacklinks(slug string) ([]*wiki.ArticleSummary, error) {
	m.lastSlug = slug
	return m.backlinks, m.err
}

func (m *mockBacklinkLister) GetArticle(url string) (*wiki.Article, error) {
	if m.articleExists {
		return &wiki.Article{URL: url}, nil
	}
	return nil, wiki.ErrGenericNotFound
}

// mockWLHTemplater implements WhatLinksHereTemplater for testing.
type mockWLHTemplater struct {
	rendered bool
	data     map[string]interface{}
	err      error
}

func (m *mockWLHTemplater) RenderTemplate(w io.Writer, name string, base string, data map[string]interface{}) error {
	m.rendered = true
	m.data = data
	if m.err != nil {
		return m.err
	}
	w.Write([]byte("rendered"))
	return nil
}

func TestWhatLinksHere(t *testing.T) {
	t.Run("shows search form when no page param", func(t *testing.T) {
		tmpl := &mockWLHTemplater{}
		handler := NewWhatLinksHerePage(&mockBacklinkLister{}, tmpl)

		req := httptest.NewRequest("GET", "/wiki/Special:WhatLinksHere", nil)
		rr := httptest.NewRecorder()

		handler.Handle(rr, req)

		if !tmpl.rendered {
			t.Fatal("expected template to be rendered")
		}
		if tmpl.data["TargetSlug"] != nil {
			t.Error("expected TargetSlug to be nil when no page param")
		}
	})

	t.Run("renders backlinks for given page", func(t *testing.T) {
		backlinks := []*wiki.ArticleSummary{
			{URL: "Page_A", LastModified: time.Now()},
			{URL: "Page_B", LastModified: time.Now()},
		}
		mock := &mockBacklinkLister{backlinks: backlinks, articleExists: true}
		tmpl := &mockWLHTemplater{}
		handler := NewWhatLinksHerePage(mock, tmpl)

		req := httptest.NewRequest("GET", "/wiki/Special:WhatLinksHere?page=Target_Page", nil)
		rr := httptest.NewRecorder()

		handler.Handle(rr, req)

		if !tmpl.rendered {
			t.Fatal("expected template to be rendered")
		}
		if mock.lastSlug != "Target_Page" {
			t.Errorf("expected slug %q, got %q", "Target_Page", mock.lastSlug)
		}
		if tmpl.data["TargetSlug"] != "Target_Page" {
			t.Errorf("expected TargetSlug %q, got %v", "Target_Page", tmpl.data["TargetSlug"])
		}
		if tmpl.data["TargetTitle"] != "Target Page" {
			t.Errorf("expected TargetTitle %q, got %v", "Target Page", tmpl.data["TargetTitle"])
		}
		if tmpl.data["TargetExists"] != true {
			t.Error("expected TargetExists to be true for existing article")
		}
		bl, ok := tmpl.data["Backlinks"].([]*wiki.ArticleSummary)
		if !ok || len(bl) != 2 {
			t.Errorf("expected 2 backlinks, got %v", tmpl.data["Backlinks"])
		}
	})

	t.Run("marks nonexistent target as redlink", func(t *testing.T) {
		mock := &mockBacklinkLister{articleExists: false}
		tmpl := &mockWLHTemplater{}
		handler := NewWhatLinksHerePage(mock, tmpl)

		req := httptest.NewRequest("GET", "/wiki/Special:WhatLinksHere?page=Missing_Page", nil)
		rr := httptest.NewRecorder()

		handler.Handle(rr, req)

		if !tmpl.rendered {
			t.Fatal("expected template to be rendered")
		}
		if tmpl.data["TargetExists"] != false {
			t.Error("expected TargetExists to be false for nonexistent article")
		}
	})

	t.Run("renders empty message for page with no backlinks", func(t *testing.T) {
		mock := &mockBacklinkLister{backlinks: nil, articleExists: true}
		tmpl := &mockWLHTemplater{}
		handler := NewWhatLinksHerePage(mock, tmpl)

		req := httptest.NewRequest("GET", "/wiki/Special:WhatLinksHere?page=Lonely_Page", nil)
		rr := httptest.NewRecorder()

		handler.Handle(rr, req)

		if !tmpl.rendered {
			t.Fatal("expected template to be rendered")
		}
		bl, ok := tmpl.data["Backlinks"].([]*wiki.ArticleSummary)
		if !ok {
			t.Fatalf("expected Backlinks to be []*wiki.ArticleSummary, got %T", tmpl.data["Backlinks"])
		}
		if len(bl) != 0 {
			t.Errorf("expected empty backlinks, got %v", bl)
		}
	})

	t.Run("returns 500 on GetBacklinks error", func(t *testing.T) {
		mock := &mockBacklinkLister{err: errors.New("database error")}
		tmpl := &mockWLHTemplater{}
		handler := NewWhatLinksHerePage(mock, tmpl)

		req := httptest.NewRequest("GET", "/wiki/Special:WhatLinksHere?page=Some_Page", nil)
		rr := httptest.NewRecorder()

		handler.Handle(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("returns 500 on template error", func(t *testing.T) {
		mock := &mockBacklinkLister{backlinks: nil}
		tmpl := &mockWLHTemplater{err: errors.New("template error")}
		handler := NewWhatLinksHerePage(mock, tmpl)

		req := httptest.NewRequest("GET", "/wiki/Special:WhatLinksHere?page=Test", nil)
		rr := httptest.NewRecorder()

		handler.Handle(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})
}
