package special

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielledeleo/periwiki/wiki"
	"github.com/danielledeleo/periwiki/wiki/service"
)

// mockRerenderer implements ArticleRerenderer for testing.
type mockRerenderer struct {
	articles   []*wiki.ArticleSummary
	getErr     error
	queueErr   error
	queuedURLs []string
}

func (m *mockRerenderer) GetAllArticles() ([]*wiki.ArticleSummary, error) {
	return m.articles, m.getErr
}

func (m *mockRerenderer) QueueRerenderRevision(ctx context.Context, url string, revisionID int) (<-chan service.RerenderResult, error) {
	m.queuedURLs = append(m.queuedURLs, url)
	if m.queueErr != nil {
		return nil, m.queueErr
	}
	ch := make(chan service.RerenderResult, 1)
	ch <- service.RerenderResult{URL: url}
	close(ch)
	return ch, nil
}

// mockRerenderTemplater implements RerenderAllTemplater for testing.
type mockRerenderTemplater struct {
	rendered bool
	data     map[string]interface{}
	err      error
}

func (m *mockRerenderTemplater) RenderTemplate(w io.Writer, name string, base string, data map[string]interface{}) error {
	m.rendered = true
	m.data = data
	if m.err != nil {
		return m.err
	}
	w.Write([]byte("rendered"))
	return nil
}

func adminContext() context.Context {
	return context.WithValue(context.Background(), wiki.UserKey, &wiki.User{ID: 1, ScreenName: "admin", Role: "admin"})
}

func nonAdminContext() context.Context {
	return context.WithValue(context.Background(), wiki.UserKey, &wiki.User{ID: 2, ScreenName: "user", Role: "user"})
}

func TestNewRerenderAllPage(t *testing.T) {
	t.Run("constructor returns non-nil handler", func(t *testing.T) {
		page := NewRerenderAllPage(&mockRerenderer{}, &mockRerenderTemplater{})
		if page == nil {
			t.Fatal("expected non-nil RerenderAllPage")
		}
	})
}

func TestRerenderAllPage_GET(t *testing.T) {
	t.Run("renders form for admin", func(t *testing.T) {
		tmpl := &mockRerenderTemplater{}
		page := NewRerenderAllPage(&mockRerenderer{}, tmpl)

		req := httptest.NewRequest("GET", "/wiki/Special:RerenderAll", nil)
		req = req.WithContext(adminContext())
		rr := httptest.NewRecorder()

		page.Handle(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
		if !tmpl.rendered {
			t.Error("expected template to be rendered")
		}
	})

	t.Run("returns 403 for non-admin", func(t *testing.T) {
		page := NewRerenderAllPage(&mockRerenderer{}, &mockRerenderTemplater{})

		req := httptest.NewRequest("GET", "/wiki/Special:RerenderAll", nil)
		req = req.WithContext(nonAdminContext())
		rr := httptest.NewRecorder()

		page.Handle(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rr.Code)
		}
	})
}

func TestRerenderAllPage_POST(t *testing.T) {
	t.Run("queues all articles", func(t *testing.T) {
		articles := []*wiki.ArticleSummary{
			{URL: "Article_A"},
			{URL: "Article_B"},
			{URL: "Article_C"},
		}
		mock := &mockRerenderer{articles: articles}
		tmpl := &mockRerenderTemplater{}
		page := NewRerenderAllPage(mock, tmpl)

		req := httptest.NewRequest("POST", "/wiki/Special:RerenderAll", nil)
		req = req.WithContext(adminContext())
		rr := httptest.NewRecorder()

		page.Handle(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
		if len(mock.queuedURLs) != 3 {
			t.Errorf("expected 3 queued URLs, got %d", len(mock.queuedURLs))
		}
		msg, _ := tmpl.data["calloutMessage"].(string)
		if msg == "" {
			t.Error("expected callout message in template data")
		}
		classes, _ := tmpl.data["calloutClasses"].(string)
		if classes != "pw-success" {
			t.Errorf("expected pw-success, got %q", classes)
		}
	})

	t.Run("handles empty article list", func(t *testing.T) {
		mock := &mockRerenderer{articles: []*wiki.ArticleSummary{}}
		tmpl := &mockRerenderTemplater{}
		page := NewRerenderAllPage(mock, tmpl)

		req := httptest.NewRequest("POST", "/wiki/Special:RerenderAll", nil)
		req = req.WithContext(adminContext())
		rr := httptest.NewRecorder()

		page.Handle(rr, req)

		msg, _ := tmpl.data["calloutMessage"].(string)
		if msg != "No articles to rerender" {
			t.Errorf("expected 'No articles to rerender', got %q", msg)
		}
	})

	t.Run("handles GetAllArticles error", func(t *testing.T) {
		mock := &mockRerenderer{getErr: errors.New("database error")}
		tmpl := &mockRerenderTemplater{}
		page := NewRerenderAllPage(mock, tmpl)

		req := httptest.NewRequest("POST", "/wiki/Special:RerenderAll", nil)
		req = req.WithContext(adminContext())
		rr := httptest.NewRecorder()

		page.Handle(rr, req)

		classes, _ := tmpl.data["calloutClasses"].(string)
		if classes != "pw-error" {
			t.Errorf("expected pw-error class, got %q", classes)
		}
	})

	t.Run("handles queue errors gracefully", func(t *testing.T) {
		articles := []*wiki.ArticleSummary{
			{URL: "Article_A"},
			{URL: "Article_B"},
		}
		mock := &mockRerenderer{articles: articles, queueErr: errors.New("queue full")}
		tmpl := &mockRerenderTemplater{}
		page := NewRerenderAllPage(mock, tmpl)

		req := httptest.NewRequest("POST", "/wiki/Special:RerenderAll", nil)
		req = req.WithContext(adminContext())
		rr := httptest.NewRecorder()

		page.Handle(rr, req)

		// Should still render (not panic/500)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 even with queue errors, got %d", rr.Code)
		}
		classes, _ := tmpl.data["calloutClasses"].(string)
		if classes != "pw-info" {
			t.Errorf("expected pw-info for partial failure, got %q", classes)
		}
	})

	t.Run("returns 403 for non-admin POST", func(t *testing.T) {
		page := NewRerenderAllPage(&mockRerenderer{}, &mockRerenderTemplater{})

		req := httptest.NewRequest("POST", "/wiki/Special:RerenderAll", nil)
		req = req.WithContext(nonAdminContext())
		rr := httptest.NewRecorder()

		page.Handle(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rr.Code)
		}
	})
}
