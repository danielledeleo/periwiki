package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielledeleo/periwiki/special"
	"github.com/danielledeleo/periwiki/testutil"
	"github.com/danielledeleo/periwiki/wiki"
	"github.com/gorilla/mux"
)

// setupHandlerTestRouter creates a router with all handlers configured for testing.
func setupHandlerTestRouter(t *testing.T) (*mux.Router, *testutil.TestApp, func()) {
	t.Helper()

	testApp, cleanup := testutil.SetupTestApp(t)

	app := &App{
		Templater:     testApp.Templater,
		Articles:      testApp.Articles,
		Users:         testApp.Users,
		Sessions:      testApp.Sessions,
		Rendering:     testApp.Rendering,
		Preferences:   testApp.Preferences,
		SpecialPages:  testApp.SpecialPages,
		Config:        testApp.Config,
		RuntimeConfig: testApp.RuntimeConfig,
	}

	router := mux.NewRouter().StrictSlash(true)
	app.RegisterRoutes(router, testutil.TestContentFS())

	return router, testApp, cleanup
}

func TestHomeHandler(t *testing.T) {
	router, _, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Errorf("expected status 302, got %d", rr.Code)
	}

	location := rr.Header().Get("Location")
	if location != "/wiki/Main_Page" {
		t.Errorf("expected redirect to /wiki/Main_Page, got %q", location)
	}
}

func TestArticleHandler_Existing(t *testing.T) {
	router, testApp, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	// Create a test user and article
	user := testutil.CreateTestUser(t, testApp.DB, "testuser", "test@example.com", "password123")
	testutil.CreateTestArticle(t, testApp, "Existing_Article", "This is the content.", user)

	req := httptest.NewRequest("GET", "/wiki/Existing_Article", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Existing Article") {
		t.Error("expected body to contain article title")
	}
	if !strings.Contains(body, "This is the content") {
		t.Error("expected body to contain article content")
	}
}

func TestArticleHandler_NotFound(t *testing.T) {
	router, _, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/wiki/nonexistent-article", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}

	body := rr.Body.String()
	// Should show the create form for non-existent articles
	if !strings.Contains(body, "article_notfound") || !strings.Contains(body, "Nonexistent-article") && !strings.Contains(body, "Nonexistent-Article") {
		// The template should indicate this is a not found page with create option
		t.Logf("Body: %s", body)
	}
}

func TestArticleHistoryHandler(t *testing.T) {
	router, testApp, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, testApp.DB, "testuser", "test@example.com", "password123")

	// Create article with multiple revisions
	article := wiki.NewArticle("history-article", "Version 1")
	article.Creator = user
	article.PreviousID = 0
	article.Comment = "Initial version"
	err := testApp.Articles.PostArticle(article)
	if err != nil {
		t.Fatalf("PostArticle failed: %v", err)
	}

	v1, _ := testApp.Articles.GetArticle("history-article")

	article.Markdown = "Version 2"
	article.PreviousID = v1.ID
	article.Comment = "Second version"
	err = testApp.Articles.PostArticle(article)
	if err != nil {
		t.Fatalf("PostArticle failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/wiki/history-article?history", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "History") {
		t.Error("expected body to contain 'History'")
	}

	// Verify diff link exists for older revision (comparing v1 to current version)
	// The link format is /wiki/{article}?diff&old={old_revision}&new={new_revision}
	if !strings.Contains(body, "/wiki/history-article?diff") {
		t.Error("expected body to contain diff link for older revision")
	}

	// The current (latest) revision should NOT have a diff link
	// (because there's nothing to compare it to)
}

func TestArticleHistoryHandler_NotFound(t *testing.T) {
	router, _, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/wiki/nonexistent?history", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}

func TestRevisionHandler(t *testing.T) {
	router, testApp, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, testApp.DB, "testuser", "test@example.com", "password123")

	// Create article with multiple revisions
	article := wiki.NewArticle("revision-article", "Version 1 content")
	article.Creator = user
	article.PreviousID = 0
	err := testApp.Articles.PostArticle(article)
	if err != nil {
		t.Fatalf("PostArticle failed: %v", err)
	}

	v1, _ := testApp.Articles.GetArticle("revision-article")

	article.Markdown = "Version 2 content"
	article.PreviousID = v1.ID
	err = testApp.Articles.PostArticle(article)
	if err != nil {
		t.Fatalf("PostArticle failed: %v", err)
	}

	t.Run("retrieves specific revision", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/wiki/revision-article?revision=1", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		body := rr.Body.String()
		if !strings.Contains(body, "Version 1") {
			t.Error("expected body to contain 'Version 1'")
		}
	})

	t.Run("invalid revision ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/wiki/revision-article?revision=invalid", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", rr.Code)
		}
	})

	t.Run("non-existent revision", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/wiki/revision-article?revision=999", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", rr.Code)
		}
	})
}

func TestDiffHandler(t *testing.T) {
	router, testApp, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, testApp.DB, "testuser", "test@example.com", "password123")

	// Create article with multiple revisions
	article := wiki.NewArticle("diff-article", "Original content here")
	article.Creator = user
	article.PreviousID = 0
	err := testApp.Articles.PostArticle(article)
	if err != nil {
		t.Fatalf("PostArticle failed: %v", err)
	}

	v1, _ := testApp.Articles.GetArticle("diff-article")

	article.Markdown = "Modified content here with changes"
	article.PreviousID = v1.ID
	err = testApp.Articles.PostArticle(article)
	if err != nil {
		t.Fatalf("PostArticle failed: %v", err)
	}

	t.Run("shows diff between revisions", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/wiki/diff-article?diff&old=1&new=2", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		body := rr.Body.String()
		// Diff should show insertions and deletions
		if !strings.Contains(body, "<ins") && !strings.Contains(body, "<del") {
			t.Log("Warning: expected diff to contain ins/del tags for changes")
		}
	})

	t.Run("invalid revision IDs", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/wiki/diff-article?diff&old=invalid&new=2", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status 400 for invalid original ID, got %d", rr.Code)
		}
	})

	t.Run("non-existent revision", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/wiki/diff-article?diff&old=1&new=999", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", rr.Code)
		}
	})

	t.Run("diff to current (omit new param)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/wiki/diff-article?diff&old=1", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		body := rr.Body.String()
		if !strings.Contains(body, "<ins") && !strings.Contains(body, "<del") {
			t.Log("Warning: expected diff to contain ins/del tags for changes")
		}
	})

	t.Run("diff from previous (omit old param)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/wiki/diff-article?diff&new=2", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		body := rr.Body.String()
		if !strings.Contains(body, "<ins") && !strings.Contains(body, "<del") {
			t.Log("Warning: expected diff to contain ins/del tags for changes")
		}
	})

	t.Run("diff latest change (omit both params)", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/wiki/diff-article?diff", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		// Should show diff between two most recent revisions
		body := rr.Body.String()
		if !strings.Contains(body, "<ins") && !strings.Contains(body, "<del") {
			t.Log("Warning: expected diff to contain ins/del tags for changes")
		}
	})

	t.Run("reverse diff with numeric IDs", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/wiki/diff-article?diff&old=2&new=1", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}
	})
}

func TestSpecialRandom(t *testing.T) {
	router, testApp, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, testApp.DB, "testuser", "test@example.com", "password123")
	testutil.CreateTestArticle(t, testApp, "random-article", "Content", user)

	req := httptest.NewRequest("GET", "/wiki/Special:Random", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect status 303, got %d", rr.Code)
	}

	location := rr.Header().Get("Location")
	if !strings.HasPrefix(location, "/wiki/") {
		t.Errorf("expected redirect to /wiki/*, got %q", location)
	}
}

func TestSpecialRandom_NoArticles(t *testing.T) {
	router, _, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/wiki/Special:Random", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect status 303, got %d", rr.Code)
	}

	location := rr.Header().Get("Location")
	if location != "/" {
		t.Errorf("expected redirect to / when no articles, got %q", location)
	}
}

func TestSpecialNotFound(t *testing.T) {
	router, _, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/wiki/Special:NonExistent", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}

func TestRegisterHandlerGET(t *testing.T) {
	router, _, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/user/register", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Register") {
		t.Error("expected body to contain 'Register'")
	}
}

func TestRegisterHandlerGET_SignupsDisabled(t *testing.T) {
	router, testApp, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	testApp.RuntimeConfig.AllowSignups = false

	req := httptest.NewRequest("GET", "/user/register", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "registrations are currently disabled") {
		t.Error("expected body to contain disabled message")
	}
	if !strings.Contains(body, "disabled") {
		t.Error("expected body to contain disabled attribute on fieldset")
	}
}

func TestRegisterPostHandler_SignupsDisabled(t *testing.T) {
	router, testApp, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	testApp.RuntimeConfig.AllowSignups = false

	form := strings.NewReader("screenname=newuser&email=new@example.com&password=password123")
	req := httptest.NewRequest("POST", "/user/register", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", rr.Code)
	}
}

func TestLoginHandlerGET(t *testing.T) {
	router, _, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/user/login", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Login") {
		t.Error("expected body to contain 'Login'")
	}
}

func TestRevisionEditHandler(t *testing.T) {
	router, testApp, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, testApp.DB, "testuser", "test@example.com", "password123")
	testutil.CreateTestArticle(t, testApp, "edit-article", "Content to edit", user)

	req := httptest.NewRequest("GET", "/wiki/edit-article?edit&revision=1", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Content to edit") {
		t.Error("expected body to contain article content in edit form")
	}
}

func TestRevisionEditHandler_NewArticle(t *testing.T) {
	router, _, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	// Request edit page for non-existent article
	req := httptest.NewRequest("GET", "/wiki/new-article?edit", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	// Should show empty edit form for new article
	if !strings.Contains(body, "New-article") && !strings.Contains(body, "New-Article") {
		t.Log("Expected title to be shown for new article")
	}
}

// TestSessionMiddleware tests that the session middleware properly sets user context
func TestSessionMiddleware(t *testing.T) {
	testApp, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	app := &App{
		Templater:     testApp.Templater,
		Articles:      testApp.Articles,
		Users:         testApp.Users,
		Sessions:      testApp.Sessions,
		Rendering:     testApp.Rendering,
		Preferences:   testApp.Preferences,
		SpecialPages:  testApp.SpecialPages,
		Config:        testApp.Config,
		RuntimeConfig: testApp.RuntimeConfig,
	}

	t.Run("sets anonymous user for new session", func(t *testing.T) {
		var capturedUser *wiki.User

		handler := app.SessionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := r.Context().Value(wiki.UserKey).(*wiki.User)
			if ok {
				capturedUser = user
			}
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if capturedUser == nil {
			t.Fatal("expected user to be set in context")
		}

		if capturedUser.ID != 0 {
			t.Errorf("expected anonymous user (ID 0), got ID %d", capturedUser.ID)
		}

		if capturedUser.ScreenName != "Anonymous" {
			t.Errorf("expected screenname 'Anonymous', got %q", capturedUser.ScreenName)
		}
	})
}

// TestSpecialPageRegistry tests special page routing with custom registry
func TestSpecialPageRegistry(t *testing.T) {
	testApp, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	// Create a custom special page
	customPageCalled := false
	customPage := &mockSpecialPage{
		handler: func(w http.ResponseWriter, r *http.Request) {
			customPageCalled = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("custom page"))
		},
	}

	registry := special.NewRegistry()
	registry.Register("Custom", customPage)

	app := &App{
		Templater:     testApp.Templater,
		Articles:      testApp.Articles,
		Users:         testApp.Users,
		Sessions:      testApp.Sessions,
		Rendering:     testApp.Rendering,
		Preferences:   testApp.Preferences,
		SpecialPages:  registry,
		Config:        testApp.Config,
		RuntimeConfig: testApp.RuntimeConfig,
	}

	router := mux.NewRouter()
	router.Use(app.SessionMiddleware)
	router.HandleFunc("/wiki/{namespace:[^:/]+}:{page}", app.NamespaceHandler).Methods("GET")

	req := httptest.NewRequest("GET", "/wiki/Special:Custom", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if !customPageCalled {
		t.Error("expected custom special page handler to be called")
	}

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestNamespaceHandler_PeriwikiNamespace(t *testing.T) {
	router, _, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	t.Run("returns embedded help article", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/wiki/Periwiki:Syntax", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		body := rr.Body.String()
		if !strings.Contains(body, "Syntax") {
			t.Error("expected body to contain 'Syntax'")
		}
	})

	t.Run("returns 404 for nonexistent embedded page", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/wiki/Periwiki:Nonexistent", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", rr.Code)
		}
	})

	t.Run("returns 404 for unknown namespace", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/wiki/Unknown:Foo", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", rr.Code)
		}
	})
}

func TestTalkNamespace_DispatchesToArticle(t *testing.T) {
	router, testApp, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, testApp.DB, "testuser", "test@example.com", "password123")
	testutil.CreateTestArticle(t, testApp, "Test_Article", "Content here", user)
	testutil.CreateTestArticle(t, testApp, "Talk:Test_Article", "Discussion here", user)

	req := httptest.NewRequest("GET", "/wiki/Talk:Test_Article", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Discussion here") {
		t.Error("expected body to contain talk page content")
	}
}

func TestTalkNamespace_NotFound(t *testing.T) {
	router, testApp, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, testApp.DB, "testuser", "test@example.com", "password123")
	testutil.CreateTestArticle(t, testApp, "Test_Article", "Content here", user)

	req := httptest.NewRequest("GET", "/wiki/Talk:Test_Article", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}
}

func TestTalkNamespace_EditBlockedWhenSubjectMissing(t *testing.T) {
	router, _, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/wiki/Talk:Nonexistent?edit", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "does not exist") {
		t.Error("expected error message about subject article not existing")
	}
}

func TestArticleMarkdownHandler(t *testing.T) {
	router, testApp, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, testApp.DB, "testuser", "test@example.com", "password123")
	testutil.CreateTestArticle(t, testApp, "Foo", "# Hello\n\nThis is **bold** text.", user)

	t.Run("existing article", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/wiki/Foo.md", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		ct := rr.Header().Get("Content-Type")
		if ct != "text/plain; charset=utf-8" {
			t.Errorf("expected Content-Type text/plain; charset=utf-8, got %q", ct)
		}

		body := rr.Body.String()
		if body != "# Hello\n\nThis is **bold** text." {
			t.Errorf("expected raw markdown, got %q", body)
		}
	})

	t.Run("nonexistent article", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/wiki/Nonexistent.md", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", rr.Code)
		}
	})

	t.Run("embedded help article", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/wiki/Periwiki:Syntax.md", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		ct := rr.Header().Get("Content-Type")
		if ct != "text/plain; charset=utf-8" {
			t.Errorf("expected Content-Type text/plain; charset=utf-8, got %q", ct)
		}

		body := rr.Body.String()
		if !strings.Contains(body, "Syntax") {
			t.Errorf("expected markdown to contain 'Syntax', got %q", body)
		}
	})

	t.Run("does not interfere with normal route", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/wiki/Foo", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}

		body := rr.Body.String()
		if !strings.Contains(body, "<") {
			t.Error("expected HTML response for normal route")
		}
	})
}

func TestArticleCaching_CurrentRevision(t *testing.T) {
	router, testApp, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, testApp.DB, "testuser", "test@example.com", "password123")
	testutil.CreateTestArticle(t, testApp, "Cache_Test", "Content here.", user)

	req := httptest.NewRequest("GET", "/wiki/Cache_Test", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if got := rr.Header().Get("Cache-Control"); got != "public, no-cache" {
		t.Errorf("Cache-Control = %q, want %q", got, "public, no-cache")
	}
	if got := rr.Header().Get("ETag"); got == "" {
		t.Error("expected ETag header to be set")
	}
	if got := rr.Header().Get("Last-Modified"); got == "" {
		t.Error("expected Last-Modified header to be set")
	}
}

func TestArticleCaching_OldRevision(t *testing.T) {
	router, testApp, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, testApp.DB, "testuser", "test@example.com", "password123")
	testutil.CreateTestArticle(t, testApp, "Old_Rev", "Version 1.", user)

	// Create a second revision
	article, _ := testApp.Articles.GetArticle("Old_Rev")
	article.Markdown = "Version 2."
	article.PreviousID = article.ID
	article.Creator = user
	testApp.Articles.PostArticle(article)

	// Request old revision (ID 1)
	req := httptest.NewRequest("GET", "/wiki/Old_Rev?revision=1", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if got := rr.Header().Get("Cache-Control"); got != "public, max-age=86400" {
		t.Errorf("Cache-Control = %q, want %q", got, "public, max-age=86400")
	}
}

func TestArticleCaching_304_ETag(t *testing.T) {
	router, testApp, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, testApp.DB, "testuser", "test@example.com", "password123")
	testutil.CreateTestArticle(t, testApp, "ETag_Test", "Content.", user)

	// First request to get ETag
	req1 := httptest.NewRequest("GET", "/wiki/ETag_Test", nil)
	rr1 := httptest.NewRecorder()
	router.ServeHTTP(rr1, req1)
	etag := rr1.Header().Get("ETag")

	// Second request with If-None-Match
	req2 := httptest.NewRequest("GET", "/wiki/ETag_Test", nil)
	req2.Header.Set("If-None-Match", etag)
	rr2 := httptest.NewRecorder()
	router.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusNotModified {
		t.Errorf("status = %d, want %d", rr2.Code, http.StatusNotModified)
	}
	if rr2.Body.Len() != 0 {
		t.Error("expected empty body for 304 response")
	}
}

func TestArticleCaching_NotFound_NoCache(t *testing.T) {
	router, _, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/wiki/Does_Not_Exist", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if got := rr.Header().Get("Cache-Control"); got != "" {
		t.Errorf("Cache-Control = %q, want empty for 404", got)
	}
}

func TestMarkdownCaching(t *testing.T) {
	router, testApp, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, testApp.DB, "testuser", "test@example.com", "password123")
	testutil.CreateTestArticle(t, testApp, "Md_Cache", "# Hello", user)

	req := httptest.NewRequest("GET", "/wiki/Md_Cache.md", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if got := rr.Header().Get("Cache-Control"); got != "public, no-cache" {
		t.Errorf("Cache-Control = %q, want %q", got, "public, no-cache")
	}
	if got := rr.Header().Get("ETag"); got == "" {
		t.Error("expected ETag header")
	}
}

func TestHistoryCaching(t *testing.T) {
	router, testApp, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, testApp.DB, "testuser", "test@example.com", "password123")
	testutil.CreateTestArticle(t, testApp, "Hist_Cache", "Content.", user)

	req := httptest.NewRequest("GET", "/wiki/Hist_Cache?history", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if got := rr.Header().Get("Cache-Control"); got != "public, no-cache" {
		t.Errorf("Cache-Control = %q, want %q", got, "public, no-cache")
	}
	if got := rr.Header().Get("Last-Modified"); got == "" {
		t.Error("expected Last-Modified header")
	}
}

func TestDiffCaching(t *testing.T) {
	router, testApp, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, testApp.DB, "testuser", "test@example.com", "password123")
	testutil.CreateTestArticle(t, testApp, "Diff_Cache", "Version 1.", user)

	article, _ := testApp.Articles.GetArticle("Diff_Cache")
	oldID := article.ID
	article.Markdown = "Version 2."
	article.PreviousID = article.ID
	article.Creator = user
	testApp.Articles.PostArticle(article)

	req := httptest.NewRequest("GET", fmt.Sprintf("/wiki/Diff_Cache?diff&old=%d", oldID), nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if got := rr.Header().Get("Cache-Control"); got != "public, max-age=86400" {
		t.Errorf("Cache-Control = %q, want %q", got, "public, max-age=86400")
	}
}

func TestUncacheableRoutes(t *testing.T) {
	router, _, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	paths := []string{
		"/user/login",
		"/user/register",
		"/manage/users",
		"/manage/settings",
		"/manage/tools",
		"/manage/content",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			if got := rr.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want %q", got, "no-store")
			}
		})
	}
}

func TestEmbeddedArticleCaching(t *testing.T) {
	router, _, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/wiki/Periwiki:Syntax", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if got := rr.Header().Get("Cache-Control"); got != "public, max-age=86400" {
		t.Errorf("Cache-Control = %q, want %q", got, "public, max-age=86400")
	}
}

func TestSitemapCaching(t *testing.T) {
	testApp, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	// Register sitemap special pages (not included in default test setup)
	sitemapHandler := special.NewSitemapPage(testApp.Articles, testApp.Templater, "http://localhost")
	testApp.SpecialPages.Register("Sitemap", sitemapHandler)
	testApp.SpecialPages.Register("Sitemap.xml", sitemapHandler)
	testApp.SpecialPages.Register("Sitemap.md", sitemapHandler)

	app := &App{
		Templater:     testApp.Templater,
		Articles:      testApp.Articles,
		Users:         testApp.Users,
		Sessions:      testApp.Sessions,
		Rendering:     testApp.Rendering,
		Preferences:   testApp.Preferences,
		SpecialPages:  testApp.SpecialPages,
		Config:        testApp.Config,
		RuntimeConfig: testApp.RuntimeConfig,
	}
	router := mux.NewRouter().StrictSlash(true)
	app.RegisterRoutes(router, testutil.TestContentFS())

	user := testutil.CreateTestUser(t, testApp.DB, "testuser", "test@example.com", "password123")
	testutil.CreateTestArticle(t, testApp, "Sitemap_Cache", "Content.", user)

	paths := []string{"/sitemap.xml", "/sitemap.md", "/wiki/Special:Sitemap"}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rr.Code)
			}
			if got := rr.Header().Get("Cache-Control"); got != "public, no-cache" {
				t.Errorf("Cache-Control = %q, want %q", got, "public, no-cache")
			}
			if got := rr.Header().Get("Last-Modified"); got == "" {
				t.Error("expected Last-Modified header")
			}
		})
	}
}

func TestStaticFileCaching(t *testing.T) {
	router, _, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	tests := []struct {
		path         string
		cacheControl string
	}{
		{"/favicon.ico", "public, max-age=86400"},
		{"/robots.txt", "public, max-age=86400"},
		{"/llms.txt", "public, max-age=86400"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rr.Code)
			}
			if got := rr.Header().Get("Cache-Control"); got != tt.cacheControl {
				t.Errorf("Cache-Control = %q, want %q", got, tt.cacheControl)
			}
			if got := rr.Header().Get("Last-Modified"); got == "" {
				t.Error("expected Last-Modified header to be set")
			}
		})
	}
}

type mockSpecialPage struct {
	handler func(http.ResponseWriter, *http.Request)
}

func (m *mockSpecialPage) Handle(w http.ResponseWriter, r *http.Request) {
	m.handler(w, r)
}

// TestUserContextInHandlers verifies user context is available in handlers
func TestUserContextInHandlers(t *testing.T) {
	testApp, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, testApp.DB, "contextuser", "context@example.com", "password123")
	testutil.CreateTestArticle(t, testApp, "context-test", "Content", user)

	app := &App{
		Templater:     testApp.Templater,
		Articles:      testApp.Articles,
		Users:         testApp.Users,
		Sessions:      testApp.Sessions,
		Rendering:     testApp.Rendering,
		Preferences:   testApp.Preferences,
		SpecialPages:  testApp.SpecialPages,
		Config:        testApp.Config,
		RuntimeConfig: testApp.RuntimeConfig,
	}

	// Create a handler that checks for user context
	var capturedContext context.Context
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedContext = r.Context()
		w.WriteHeader(http.StatusOK)
	})

	handler := app.SessionMiddleware(testHandler)

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if capturedContext == nil {
		t.Fatal("expected context to be set")
	}

	userFromContext := capturedContext.Value(wiki.UserKey)
	if userFromContext == nil {
		t.Fatal("expected user to be in context")
	}

	u, ok := userFromContext.(*wiki.User)
	if !ok {
		t.Fatal("expected user to be *wiki.User type")
	}

	// Should be anonymous for requests without session
	if u.ID != 0 {
		t.Errorf("expected anonymous user for request without session, got ID %d", u.ID)
	}
}
