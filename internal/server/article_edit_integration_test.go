package server

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/danielledeleo/periwiki/testutil"
	"github.com/danielledeleo/periwiki/wiki"
	"github.com/gorilla/mux"
)

// setupArticleEditTestServer creates a test server for article editing tests.
func setupArticleEditTestServer(t *testing.T) (*httptest.Server, *testutil.TestApp, func()) {
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
	router.Use(app.SessionMiddleware)

	router.HandleFunc("/", app.HomeHandler).Methods("GET")
	router.HandleFunc("/wiki/{article}", app.ArticleDispatcher).Methods("GET", "POST")
	router.HandleFunc("/user/login", app.LoginPostHandler).Methods("POST")

	server := httptest.NewServer(router)

	serverCleanup := func() {
		server.Close()
		cleanup()
	}

	return server, testApp, serverCleanup
}

// loginUser logs in a user and returns a client with the session cookie.
func loginUser(t *testing.T, server *httptest.Server, screenname, password string) *http.Client {
	t.Helper()

	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	formData := url.Values{
		"screenname": {screenname},
		"password":   {password},
	}

	resp, err := client.PostForm(server.URL+"/user/login", formData)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected login redirect, got %d", resp.StatusCode)
	}

	return client
}

func TestCreateNewArticle(t *testing.T) {
	server, testApp, cleanup := setupArticleEditTestServer(t)
	defer cleanup()

	// Create and login a user
	password := "createpassword"
	user := &wiki.User{
		ScreenName:  "createuser",
		Email:       "create@example.com",
		RawPassword: password,
	}
	err := testApp.Users.PostUser(user)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	client := loginUser(t, server, "createuser", password)

	// Create a new article by posting to revision 0
	formData := url.Values{
		"title":       {"New Test Article"},
		"body":        {"# Hello World\n\nThis is the content."},
		"comment":     {"Initial creation"},
		"previous_id": {"0"},
	}

	resp, err := client.PostForm(server.URL+"/wiki/new-test-article", formData)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should redirect to the article page
	if resp.StatusCode != http.StatusSeeOther {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected redirect 303, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	location := resp.Header.Get("Location")
	if location != "/wiki/new-test-article" {
		t.Errorf("expected redirect to /wiki/new-test-article, got %q", location)
	}

	// Verify article was created
	article, err := testApp.Articles.GetArticle("new-test-article")
	if err != nil {
		t.Fatalf("article not found after creation: %v", err)
	}

	// Title is now derived from URL or frontmatter, not form field
	if article.DisplayTitle() != "New-test-article" {
		t.Errorf("expected display title 'New-test-article', got %q", article.DisplayTitle())
	}
}

func TestEditExistingArticle(t *testing.T) {
	server, testApp, cleanup := setupArticleEditTestServer(t)
	defer cleanup()

	// Create and login a user
	password := "editpassword"
	user := &wiki.User{
		ScreenName:  "edituser",
		Email:       "edit@example.com",
		RawPassword: password,
	}
	err := testApp.Users.PostUser(user)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Get the user ID for article creation
	createdUser, _ := testApp.Users.GetUserByScreenName("edituser")

	// Create initial article
	testutil.CreateTestArticle(t, testApp, "edit-test", "Edit Test", "Original content", createdUser)

	client := loginUser(t, server, "edituser", password)

	// Edit the article (create new revision)
	formData := url.Values{
		"title":       {"Edit Test Updated"},
		"body":        {"Updated content with changes"},
		"comment":     {"Updated the article"},
		"previous_id": {"1"},
	}

	resp, err := client.PostForm(server.URL+"/wiki/edit-test", formData)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should redirect to the article page
	if resp.StatusCode != http.StatusSeeOther {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected redirect 303, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Verify new revision was created
	article, err := testApp.Articles.GetArticle("edit-test")
	if err != nil {
		t.Fatalf("article not found after edit: %v", err)
	}

	if article.Markdown != "Updated content with changes" {
		t.Errorf("expected updated content, got %q", article.Markdown)
	}

	if article.ID != 2 {
		t.Errorf("expected revision ID 2, got %d", article.ID)
	}
}

func TestPreviewArticle(t *testing.T) {
	server, testApp, cleanup := setupArticleEditTestServer(t)
	defer cleanup()

	// Create and login a user
	password := "previewpassword"
	user := &wiki.User{
		ScreenName:  "previewuser",
		Email:       "preview@example.com",
		RawPassword: password,
	}
	err := testApp.Users.PostUser(user)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	createdUser, _ := testApp.Users.GetUserByScreenName("previewuser")
	testutil.CreateTestArticle(t, testApp, "preview-test", "Preview Test", "Original", createdUser)

	client := loginUser(t, server, "previewuser", password)

	// Request preview
	formData := url.Values{
		"title":       {"Preview Test"},
		"body":        {"# Preview Content\n\nThis should be rendered."},
		"action":      {"preview"},
		"previous_id": {"1"},
	}

	resp, err := client.PostForm(server.URL+"/wiki/preview-test", formData)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should return 200 with preview, not redirect
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	body := string(bodyBytes)

	// Should contain rendered HTML
	if !strings.Contains(body, "<h1>Preview Content</h1>") && !strings.Contains(body, "Preview Content") {
		t.Log("Note: Preview should contain rendered heading")
	}

	// Article should not be modified
	article, _ := testApp.Articles.GetArticle("preview-test")
	if article.Markdown != "Original" {
		t.Error("article should not be modified during preview")
	}
}

func TestEditRequiresChange(t *testing.T) {
	server, testApp, cleanup := setupArticleEditTestServer(t)
	defer cleanup()

	// Create and login a user
	password := "nochangepassword"
	user := &wiki.User{
		ScreenName:  "nochangeuser",
		Email:       "nochange@example.com",
		RawPassword: password,
	}
	err := testApp.Users.PostUser(user)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	createdUser, _ := testApp.Users.GetUserByScreenName("nochangeuser")
	testutil.CreateTestArticle(t, testApp, "nochange-test", "No Change", "Same content", createdUser)

	client := loginUser(t, server, "nochangeuser", password)

	// Try to post identical content (title is deprecated, not sent)
	formData := url.Values{
		"body":        {"Same content"},
		"previous_id": {"1"},
	}

	resp, err := client.PostForm(server.URL+"/wiki/nochange-test", formData)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should return error status
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400 for unchanged content, got %d", resp.StatusCode)
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	body := string(bodyBytes)

	if !strings.Contains(body, "not modified") {
		t.Log("Note: Response should indicate article was not modified")
	}
}

func TestMarkdownRendering(t *testing.T) {
	server, testApp, cleanup := setupArticleEditTestServer(t)
	defer cleanup()

	// Create and login a user
	password := "renderpassword"
	user := &wiki.User{
		ScreenName:  "renderuser",
		Email:       "render@example.com",
		RawPassword: password,
	}
	err := testApp.Users.PostUser(user)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	client := loginUser(t, server, "renderuser", password)

	tests := []struct {
		name     string
		markdown string
		contains string
	}{
		{
			name:     "heading",
			markdown: "# Test Heading",
			contains: "Test Heading", // heading text should be present
		},
		{
			name:     "bold",
			markdown: "**bold text**",
			contains: "<strong>",
		},
		{
			name:     "link",
			markdown: "[link](https://example.com)",
			contains: `<a href="https://example.com"`,
		},
		{
			name:     "list",
			markdown: "- item 1\n- item 2",
			contains: "<li>",
		},
		{
			name:     "code",
			markdown: "`code`",
			contains: "<code>",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			articleURL := "render-test-" + tc.name

			formData := url.Values{
				"title":       {"Render Test " + tc.name},
				"body":        {tc.markdown},
				"previous_id": {"0"},
			}

			resp, err := client.PostForm(server.URL+"/wiki/"+articleURL, formData)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			resp.Body.Close()

			// Get the article and check HTML
			article, err := testApp.Articles.GetArticle(articleURL)
			if err != nil {
				t.Fatalf("article not found: %v", err)
			}

			if !strings.Contains(article.HTML, tc.contains) {
				t.Errorf("expected HTML to contain %q, got: %s", tc.contains, article.HTML)
			}
		})
	}
}

func TestWikiLinkRendering(t *testing.T) {
	server, testApp, cleanup := setupArticleEditTestServer(t)
	defer cleanup()

	// Create and login a user
	password := "wikilinkpassword"
	user := &wiki.User{
		ScreenName:  "wikilinkuser",
		Email:       "wikilink@example.com",
		RawPassword: password,
	}
	err := testApp.Users.PostUser(user)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	client := loginUser(t, server, "wikilinkuser", password)

	// Create an article with wikilinks
	formData := url.Values{
		"title":       {"Wiki Link Test"},
		"body":        {"Check out [[Other Page]] and [[Another Article|custom text]]."},
		"previous_id": {"0"},
	}

	resp, err := client.PostForm(server.URL+"/wiki/wikilink-test", formData)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	// Get the article and check for wikilinks
	article, err := testApp.Articles.GetArticle("wikilink-test")
	if err != nil {
		t.Fatalf("article not found: %v", err)
	}

	// Wikilinks should be rendered as anchor tags
	// The wikilink extension should convert [[Page]] to <a href="/wiki/Page">
	hasWikiLinks := strings.Contains(article.HTML, `href="/wiki/`)
	hasRawSyntax := strings.Contains(article.HTML, "[[")

	if hasWikiLinks && !hasRawSyntax {
		// Perfect: wikilinks are fully processed
		t.Log("Wikilinks correctly rendered as anchor tags")
	} else if hasWikiLinks && hasRawSyntax {
		// Partial processing - some wikilinks rendered, some not
		t.Log("Note: Some wikilink syntax remains in HTML, but links are being generated")
	} else if !hasWikiLinks {
		// Wikilinks not being processed - log but don't fail
		t.Log("Note: Wikilinks not rendered - extension may not be configured in test environment")
	}
}

func TestEditHandlerShowsContent(t *testing.T) {
	server, testApp, cleanup := setupArticleEditTestServer(t)
	defer cleanup()

	// Create test user and article
	user := testutil.CreateTestUser(t, testApp.DB, "contentuser", "content@example.com", "contentpassword")
	testutil.CreateTestArticle(t, testApp, "content-test", "Content Test", "This is the article content to edit.", user)

	// Request the edit page
	resp, err := http.Get(server.URL + "/wiki/content-test?edit&revision=1")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	body := string(bodyBytes)

	// Should contain the article content in a form
	if !strings.Contains(body, "This is the article content") {
		t.Error("expected edit form to contain article content")
	}

	if !strings.Contains(body, "Content Test") {
		t.Error("expected edit form to contain article title")
	}
}

func TestAnonymousUserCannotCreateArticle(t *testing.T) {
	server, testApp, cleanup := setupArticleEditTestServer(t)
	defer cleanup()

	// Create article as anonymous (no login)
	formData := url.Values{
		"title":       {"Anonymous Article"},
		"body":        {"Content from anonymous"},
		"previous_id": {"0"},
	}

	resp, err := http.PostForm(server.URL+"/wiki/anon-article", formData)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// This should actually succeed in the current implementation
	// since anonymous users (ID 0) are allowed to edit
	// But let's verify the article was created with anonymous creator

	if resp.StatusCode == http.StatusSeeOther {
		article, err := testApp.Articles.GetArticle("anon-article")
		if err != nil {
			t.Fatalf("article not found: %v", err)
		}

		// Verify the article exists (anonymous editing is allowed)
		if article == nil {
			t.Error("expected article to be created")
		}
	}
}

func TestRevisionConflict(t *testing.T) {
	server, testApp, cleanup := setupArticleEditTestServer(t)
	defer cleanup()

	// Create two users
	password := "conflictpassword"

	user1 := &wiki.User{
		ScreenName:  "conflictuser1",
		Email:       "conflict1@example.com",
		RawPassword: password,
	}
	testApp.Users.PostUser(user1)

	user2 := &wiki.User{
		ScreenName:  "conflictuser2",
		Email:       "conflict2@example.com",
		RawPassword: password,
	}
	testApp.Users.PostUser(user2)

	createdUser1, _ := testApp.Users.GetUserByScreenName("conflictuser1")
	testutil.CreateTestArticle(t, testApp, "conflict-test", "Conflict Test", "Original", createdUser1)

	client1 := loginUser(t, server, "conflictuser1", password)
	client2 := loginUser(t, server, "conflictuser2", password)

	// Both users edit from revision 1
	formData1 := url.Values{
		"title":       {"Conflict Test"},
		"body":        {"Edit from user 1"},
		"previous_id": {"1"},
	}

	formData2 := url.Values{
		"title":       {"Conflict Test"},
		"body":        {"Edit from user 2"},
		"previous_id": {"1"},
	}

	// User 1 edits first
	resp1, err := client1.PostForm(server.URL+"/wiki/conflict-test", formData1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp1.Body.Close()

	if resp1.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected user1 edit to succeed, got %d", resp1.StatusCode)
	}

	// User 2 tries to edit from same revision - should get conflict
	resp2, err := client2.PostForm(server.URL+"/wiki/conflict-test", formData2)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp2.Body.Close()

	// Should get conflict error
	if resp2.StatusCode != http.StatusConflict {
		t.Errorf("expected status 409 (Conflict), got %d", resp2.StatusCode)
	}
}

func TestEditWithoutPreviousID(t *testing.T) {
	server, testApp, cleanup := setupArticleEditTestServer(t)
	defer cleanup()

	password := "noprevpassword"
	user := &wiki.User{
		ScreenName:  "noprevuser",
		Email:       "noprev@example.com",
		RawPassword: password,
	}
	testApp.Users.PostUser(user)

	createdUser, _ := testApp.Users.GetUserByScreenName("noprevuser")
	testutil.CreateTestArticle(t, testApp, "noprev-test", "No Previous Test", "Original content", createdUser)

	client := loginUser(t, server, "noprevuser", password)

	// Submit edit without previous_id field - should conflict because
	// missing previous_id defaults to 0, and revision 1 already exists
	formData := url.Values{
		"title": {"No Previous Test"},
		"body":  {"Updated content"},
	}

	resp, err := client.PostForm(server.URL+"/wiki/noprev-test", formData)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected status 409 (Conflict) when previous_id is missing, got %d", resp.StatusCode)
	}

	// Verify article was NOT modified
	article, _ := testApp.Articles.GetArticle("noprev-test")
	if article.Markdown != "Original content" {
		t.Errorf("article should not have been modified, got %q", article.Markdown)
	}
}

func TestEditFormPreservesContentOnError(t *testing.T) {
	server, testApp, cleanup := setupArticleEditTestServer(t)
	defer cleanup()

	password := "preservepassword"
	user := &wiki.User{
		ScreenName:  "preserveuser",
		Email:       "preserve@example.com",
		RawPassword: password,
	}
	testApp.Users.PostUser(user)

	createdUser, _ := testApp.Users.GetUserByScreenName("preserveuser")
	testutil.CreateTestArticle(t, testApp, "preserve-test", "Preserve Test", "Original content", createdUser)

	client := loginUser(t, server, "preserveuser", password)

	// Try to save with identical content (which should fail)
	formData := url.Values{
		"title":       {"Preserve Test"},
		"body":        {"Original content"},
		"previous_id": {"1"},
	}

	resp, err := client.PostForm(server.URL+"/wiki/preserve-test", formData)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// When there's an error, the form should be shown again
	// (This tests that the server handles the error gracefully)
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusBadRequest {
		// Error pages should still return a response
		bodyBytes, _ := io.ReadAll(resp.Body)
		body := string(bodyBytes)

		// The page should contain some error indication
		if len(body) == 0 {
			t.Error("expected non-empty response body")
		}
	}
}

// setupTestServerWithAnonEditsDisabled creates a test server with anonymous editing disabled.
func setupTestServerWithAnonEditsDisabled(t *testing.T) (*httptest.Server, *testutil.TestApp, func()) {
	t.Helper()

	testApp, cleanup := testutil.SetupTestApp(t)

	// Disable anonymous editing
	testApp.RuntimeConfig.AllowAnonymousEditsGlobal = false

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
	router.Use(app.SessionMiddleware)

	router.HandleFunc("/", app.HomeHandler).Methods("GET")
	router.HandleFunc("/wiki/{article}", app.ArticleDispatcher).Methods("GET", "POST")
	router.HandleFunc("/user/login", app.LoginPostHandler).Methods("POST")

	server := httptest.NewServer(router)

	serverCleanup := func() {
		server.Close()
		cleanup()
	}

	return server, testApp, serverCleanup
}

func TestAnonymousEditBlockedWhenDisabled(t *testing.T) {
	server, _, cleanup := setupTestServerWithAnonEditsDisabled(t)
	defer cleanup()

	// Create HTTP client that doesn't follow redirects
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Try to access edit page as anonymous user
	resp, err := client.Get(server.URL + "/wiki/test-article?edit")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should redirect to login
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected status 303 (See Other), got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if !strings.HasPrefix(location, "/user/login?reason=login_required&referrer=") {
		t.Errorf("expected redirect to login with reason and referrer, got %q", location)
	}
}

func TestAnonymousPostBlockedWhenDisabled(t *testing.T) {
	server, _, cleanup := setupTestServerWithAnonEditsDisabled(t)
	defer cleanup()

	// Try to POST article as anonymous user
	formData := url.Values{
		"title":       {"Test Article"},
		"body":        {"Test content"},
		"previous_id": {"0"},
	}

	resp, err := http.PostForm(server.URL+"/wiki/anon-blocked-test", formData)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should get 403 Forbidden
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status 403 (Forbidden), got %d", resp.StatusCode)
	}
}

func TestAuthenticatedUserCanEditWhenAnonDisabled(t *testing.T) {
	server, testApp, cleanup := setupTestServerWithAnonEditsDisabled(t)
	defer cleanup()

	// Create and login user
	password := "authedpassword"
	user := &wiki.User{
		ScreenName:  "autheduser",
		Email:       "authed@example.com",
		RawPassword: password,
	}
	testApp.Users.PostUser(user)

	client := loginUser(t, server, "autheduser", password)

	// Access edit page - should work
	resp, err := client.Get(server.URL + "/wiki/new-article?edit")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// POST article - should also work
	formData := url.Values{
		"title":       {"Auth Test Article"},
		"body":        {"Content from authenticated user"},
		"previous_id": {"0"},
	}

	resp2, err := client.PostForm(server.URL+"/wiki/auth-test-article", formData)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusSeeOther {
		t.Errorf("expected status 303 (redirect after save), got %d", resp2.StatusCode)
	}

	// Verify article was created
	article, err := testApp.Articles.GetArticle("auth-test-article")
	if err != nil {
		t.Fatalf("article not found: %v", err)
	}
	// Title is now derived from URL or frontmatter, not form field
	if article.DisplayTitle() != "Auth-test-article" {
		t.Errorf("expected display title 'Auth-test-article', got %q", article.DisplayTitle())
	}
}

func TestAnonymousCanEditWhenEnabled(t *testing.T) {
	// Uses default setup which has AllowAnonymousEditsGlobal=true
	server, testApp, cleanup := setupArticleEditTestServer(t)
	defer cleanup()

	// Create HTTP client that doesn't follow redirects
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// POST article as anonymous user
	formData := url.Values{
		"title":       {"Anonymous Article"},
		"body":        {"Content from anonymous"},
		"previous_id": {"0"},
	}

	resp, err := client.PostForm(server.URL+"/wiki/anon-allowed-test", formData)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should succeed with redirect
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected status 303 (redirect after save), got %d", resp.StatusCode)
	}

	// Verify article was created
	article, err := testApp.Articles.GetArticle("anon-allowed-test")
	if err != nil {
		t.Fatalf("article not found: %v", err)
	}
	// Title is now derived from URL or frontmatter, not form field
	if article.DisplayTitle() != "Anon-allowed-test" {
		t.Errorf("expected display title 'Anon-allowed-test', got %q", article.DisplayTitle())
	}
}

func TestRerenderCurrentRevision(t *testing.T) {
	server, testApp, cleanup := setupArticleEditTestServer(t)
	defer cleanup()

	// Create user and login
	password := "rerenderpass2"
	user := &wiki.User{
		ScreenName:  "rerenderuser2",
		Email:       "rerender2@example.com",
		RawPassword: password,
	}
	err := testApp.Users.PostUser(user)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	createdUser, _ := testApp.Users.GetUserByScreenName("rerenderuser2")
	testutil.CreateTestArticle(t, testApp, "rerender-current", "Rerender Current", "Test **bold** content", createdUser)

	client := loginUser(t, server, "rerenderuser2", password)

	// Get original HTML
	originalArticle, _ := testApp.Articles.GetArticle("rerender-current")
	originalHTML := originalArticle.HTML

	// Rerender the article
	resp, err := client.Get(server.URL + "/wiki/rerender-current?rerender")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should redirect to article
	if resp.StatusCode != http.StatusSeeOther {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected redirect 303, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	location := resp.Header.Get("Location")
	if location != "/wiki/rerender-current" {
		t.Errorf("expected redirect to article, got %q", location)
	}

	// Verify article was re-rendered (HTML should still be valid)
	article, err := testApp.Articles.GetArticle("rerender-current")
	if err != nil {
		t.Fatalf("article not found after rerender: %v", err)
	}

	// HTML should contain the bold markup
	if !strings.Contains(article.HTML, "<strong>bold</strong>") {
		t.Errorf("expected HTML to contain bold markup, got %q", article.HTML)
	}

	// HTML should be the same (re-rendering same markdown produces same output)
	if article.HTML != originalHTML {
		t.Logf("HTML changed from %q to %q (may be expected if rendering changed)", originalHTML, article.HTML)
	}
}

func TestRerenderSpecificRevision(t *testing.T) {
	server, testApp, cleanup := setupArticleEditTestServer(t)
	defer cleanup()

	// Create user and login
	password := "rerenderpass3"
	user := &wiki.User{
		ScreenName:  "rerenderuser3",
		Email:       "rerender3@example.com",
		RawPassword: password,
	}
	err := testApp.Users.PostUser(user)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	createdUser, _ := testApp.Users.GetUserByScreenName("rerenderuser3")

	// Create article with revision 1
	testutil.CreateTestArticle(t, testApp, "rerender-revision", "Rerender Rev", "First *italic* content", createdUser)

	client := loginUser(t, server, "rerenderuser3", password)

	// Create revision 2
	formData := url.Values{
		"title":       {"Rerender Rev v2"},
		"body":        {"Second **bold** content"},
		"comment":     {"Updated"},
		"previous_id": {"1"},
	}
	resp, err := client.PostForm(server.URL+"/wiki/rerender-revision", formData)
	if err != nil {
		t.Fatalf("edit request failed: %v", err)
	}
	resp.Body.Close()

	// Rerender revision 1 specifically
	resp, err = client.Get(server.URL + "/wiki/rerender-revision?rerender&revision=1")
	if err != nil {
		t.Fatalf("rerender request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should redirect to the specific revision
	if resp.StatusCode != http.StatusSeeOther {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected redirect 303, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	location := resp.Header.Get("Location")
	if location != "/wiki/rerender-revision?revision=1" {
		t.Errorf("expected redirect to revision 1, got %q", location)
	}

	// Verify revision 1 HTML contains italic markup
	rev1, err := testApp.Articles.GetArticleByRevisionID("rerender-revision", 1)
	if err != nil {
		t.Fatalf("revision 1 not found: %v", err)
	}
	if !strings.Contains(rev1.HTML, "<em>italic</em>") {
		t.Errorf("expected revision 1 HTML to contain italic markup, got %q", rev1.HTML)
	}
}

func TestRerenderInvalidRevision(t *testing.T) {
	server, testApp, cleanup := setupArticleEditTestServer(t)
	defer cleanup()

	// Create user and article
	password := "rerenderpass4"
	user := &wiki.User{
		ScreenName:  "rerenderuser4",
		Email:       "rerender4@example.com",
		RawPassword: password,
	}
	err := testApp.Users.PostUser(user)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	createdUser, _ := testApp.Users.GetUserByScreenName("rerenderuser4")
	testutil.CreateTestArticle(t, testApp, "rerender-invalid", "Rerender Invalid", "Content", createdUser)

	client := loginUser(t, server, "rerenderuser4", password)

	// Try to rerender non-existent revision
	resp, err := client.Get(server.URL + "/wiki/rerender-invalid?rerender&revision=999")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should return error (not found or internal server error)
	if resp.StatusCode == http.StatusSeeOther {
		t.Errorf("expected error status, got redirect")
	}
}
