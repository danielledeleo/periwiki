package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielledeleo/periwiki/special"
	"github.com/danielledeleo/periwiki/wiki"
	"github.com/gorilla/mux"
)

// mockRandomGetter implements special.RandomArticleGetter for testing.
type mockRandomGetter struct {
	url string
	err error
}

func (m *mockRandomGetter) GetRandomArticleURL() (string, error) {
	return m.url, m.err
}

// setupTestRouter creates a router with special page routes for testing.
func setupTestRouter(specialPages *special.Registry) *mux.Router {
	router := mux.NewRouter()

	// Namespace route (must come before article route) - catches all Foo:Bar URLs
	router.HandleFunc("/wiki/{namespace:[^:/]+}:{page}", func(rw http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		namespace := vars["namespace"]
		pageName := vars["page"]

		if !strings.EqualFold(namespace, "special") {
			rw.WriteHeader(http.StatusNotFound)
			return
		}

		handler, ok := specialPages.Get(pageName)
		if !ok {
			rw.WriteHeader(http.StatusNotFound)
			return
		}
		handler.Handle(rw, req)
	}).Methods("GET")

	// Article route (catch-all for /wiki/*)
	router.HandleFunc("/wiki/{article}", func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusOK)
		rw.Write([]byte("article page"))
	}).Methods("GET")

	return router
}

func TestSpecialRandomIntegration(t *testing.T) {
	t.Run("redirects to random article with 303", func(t *testing.T) {
		mock := &mockRandomGetter{url: "test-article", err: nil}
		registry := special.NewRegistry()
		registry.Register("Random", special.NewRandomPage(mock))

		router := setupTestRouter(registry)

		req := httptest.NewRequest("GET", "/wiki/Special:Random", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusSeeOther {
			t.Errorf("expected status %d, got %d", http.StatusSeeOther, rr.Code)
		}

		location := rr.Header().Get("Location")
		if location != "/wiki/test-article" {
			t.Errorf("expected redirect to /wiki/test-article, got %q", location)
		}
	})

	t.Run("redirects to home when no articles exist", func(t *testing.T) {
		mock := &mockRandomGetter{url: "", err: wiki.ErrNoArticles}
		registry := special.NewRegistry()
		registry.Register("Random", special.NewRandomPage(mock))

		router := setupTestRouter(registry)

		req := httptest.NewRequest("GET", "/wiki/Special:Random", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusSeeOther {
			t.Errorf("expected status %d, got %d", http.StatusSeeOther, rr.Code)
		}

		location := rr.Header().Get("Location")
		if location != "/" {
			t.Errorf("expected redirect to /, got %q", location)
		}
	})

	t.Run("multiple requests return valid redirects", func(t *testing.T) {
		articles := []string{"article1", "article2", "article3"}
		callCount := 0

		registry := special.NewRegistry()
		registry.Register("Random", &cyclingRandomPage{
			articles: articles,
			index:    &callCount,
		})

		router := setupTestRouter(registry)

		for i := 0; i < 5; i++ {
			req := httptest.NewRequest("GET", "/wiki/Special:Random", nil)
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			if rr.Code != http.StatusSeeOther {
				t.Errorf("request %d: expected status %d, got %d", i, http.StatusSeeOther, rr.Code)
			}

			location := rr.Header().Get("Location")
			if !strings.HasPrefix(location, "/wiki/") {
				t.Errorf("request %d: expected redirect to start with /wiki/, got %q", i, location)
			}
		}
	})
}

func TestSpecialPageNotFoundIntegration(t *testing.T) {
	t.Run("returns 404 for non-existent special page", func(t *testing.T) {
		registry := special.NewRegistry()
		// Don't register any pages

		router := setupTestRouter(registry)

		req := httptest.NewRequest("GET", "/wiki/Special:NonExistent", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
		}
	})

	t.Run("special page names are case sensitive", func(t *testing.T) {
		mock := &mockRandomGetter{url: "test", err: nil}
		registry := special.NewRegistry()
		registry.Register("Random", special.NewRandomPage(mock))

		router := setupTestRouter(registry)

		// Try lowercase - should 404
		req := httptest.NewRequest("GET", "/wiki/Special:random", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected 404 for lowercase 'random', got %d", rr.Code)
		}

		// Try uppercase - should 404
		req = httptest.NewRequest("GET", "/wiki/Special:RANDOM", nil)
		rr = httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected 404 for uppercase 'RANDOM', got %d", rr.Code)
		}

		// Try correct case - should redirect
		req = httptest.NewRequest("GET", "/wiki/Special:Random", nil)
		rr = httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusSeeOther {
			t.Errorf("expected 303 for 'Random', got %d", rr.Code)
		}
	})
}

func TestSpecialRouteOrdering(t *testing.T) {
	t.Run("special page route takes precedence over article route", func(t *testing.T) {
		mock := &mockRandomGetter{url: "redirected", err: nil}
		registry := special.NewRegistry()
		registry.Register("Random", special.NewRandomPage(mock))

		router := setupTestRouter(registry)

		req := httptest.NewRequest("GET", "/wiki/Special:Random", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		// Should get redirect (special page), not 200 (article page)
		if rr.Code != http.StatusSeeOther {
			t.Errorf("expected special page handler (303), got status %d", rr.Code)
		}
	})

	t.Run("regular articles still work", func(t *testing.T) {
		registry := special.NewRegistry()
		router := setupTestRouter(registry)

		req := httptest.NewRequest("GET", "/wiki/regular-article", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 for regular article, got %d", rr.Code)
		}
	})
}

// cyclingRandomPage is a test helper that cycles through a list of articles.
type cyclingRandomPage struct {
	articles []string
	index    *int
}

func (p *cyclingRandomPage) Handle(rw http.ResponseWriter, req *http.Request) {
	article := p.articles[*p.index%len(p.articles)]
	*p.index++
	http.Redirect(rw, req, "/wiki/"+article, http.StatusSeeOther)
}
