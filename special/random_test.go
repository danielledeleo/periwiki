package special

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielledeleo/periwiki/wiki"
)

// mockRandomGetter implements RandomArticleGetter for testing.
type mockRandomGetter struct {
	url string
	err error
}

func (m *mockRandomGetter) GetRandomArticleURL() (string, error) {
	return m.url, m.err
}

func TestRandomPageHandler(t *testing.T) {
	t.Run("redirects to random article", func(t *testing.T) {
		mock := &mockRandomGetter{url: "test-article", err: nil}
		handler := NewRandomPage(mock)

		req := httptest.NewRequest("GET", "/wiki/Special:Random", nil)
		rr := httptest.NewRecorder()

		handler.Handle(rr, req)

		if rr.Code != http.StatusSeeOther {
			t.Errorf("expected status %d, got %d", http.StatusSeeOther, rr.Code)
		}

		location := rr.Header().Get("Location")
		expected := "/wiki/test-article"
		if location != expected {
			t.Errorf("expected redirect to %q, got %q", expected, location)
		}
	})

	t.Run("redirects to home when no articles", func(t *testing.T) {
		mock := &mockRandomGetter{url: "", err: wiki.ErrNoArticles}
		handler := NewRandomPage(mock)

		req := httptest.NewRequest("GET", "/wiki/Special:Random", nil)
		rr := httptest.NewRecorder()

		handler.Handle(rr, req)

		if rr.Code != http.StatusSeeOther {
			t.Errorf("expected status %d, got %d", http.StatusSeeOther, rr.Code)
		}

		location := rr.Header().Get("Location")
		if location != "/" {
			t.Errorf("expected redirect to /, got %q", location)
		}
	})

	t.Run("returns 500 on database error", func(t *testing.T) {
		mock := &mockRandomGetter{url: "", err: errors.New("database connection failed")}
		handler := NewRandomPage(mock)

		req := httptest.NewRequest("GET", "/wiki/Special:Random", nil)
		rr := httptest.NewRecorder()

		handler.Handle(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
	})

	t.Run("handles URL with special characters", func(t *testing.T) {
		mock := &mockRandomGetter{url: "article-with-dash_and_underscore", err: nil}
		handler := NewRandomPage(mock)

		req := httptest.NewRequest("GET", "/wiki/Special:Random", nil)
		rr := httptest.NewRecorder()

		handler.Handle(rr, req)

		if rr.Code != http.StatusSeeOther {
			t.Errorf("expected status %d, got %d", http.StatusSeeOther, rr.Code)
		}

		location := rr.Header().Get("Location")
		expected := "/wiki/article-with-dash_and_underscore"
		if location != expected {
			t.Errorf("expected redirect to %q, got %q", expected, location)
		}
	})
}
