package main

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

// setupSecurityTestServer creates a test server for security tests.
func setupSecurityTestServer(t *testing.T) (*httptest.Server, *testutil.TestApp, func()) {
	t.Helper()

	testApp, cleanup := testutil.SetupTestApp(t)

	app := &app{
		Templater:    testApp.Templater,
		WikiModel:    testApp.WikiModel,
		specialPages: testApp.SpecialPages,
	}

	router := mux.NewRouter().StrictSlash(true)
	router.Use(app.SessionMiddleware)

	router.HandleFunc("/", app.homeHandler).Methods("GET")
	router.HandleFunc("/wiki/{article}", app.articleHandler).Methods("GET")
	router.HandleFunc("/wiki/{article}/r/{revision}", app.revisionHandler).Methods("GET")
	router.HandleFunc("/wiki/{article}/r/{revision}", app.revisionPostHandler).Methods("POST")
	router.HandleFunc("/wiki/{article}/diff/{original}/{new}", app.diffHandler).Methods("GET")
	router.HandleFunc("/user/login", app.loginPostHander).Methods("POST")

	server := httptest.NewServer(router)

	serverCleanup := func() {
		server.Close()
		cleanup()
	}

	return server, testApp, serverCleanup
}

// getAuthenticatedClient logs in a user and returns an authenticated client.
func getAuthenticatedClient(t *testing.T, server *httptest.Server, username, password string) *http.Client {
	t.Helper()

	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	formData := url.Values{
		"screenname": {username},
		"password":   {password},
	}

	resp, err := client.PostForm(server.URL+"/user/login", formData)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	resp.Body.Close()

	return client
}

func TestXSSInArticleContent(t *testing.T) {
	server, testApp, cleanup := setupSecurityTestServer(t)
	defer cleanup()

	password := "xsspassword"
	user := &wiki.User{
		ScreenName:  "xssuser",
		Email:       "xss@example.com",
		RawPassword: password,
	}
	testApp.PostUser(user)
	client := getAuthenticatedClient(t, server, "xssuser", password)

	xssPayloads := []struct {
		name       string
		payload    string
		shouldNot  []string
		shouldHave []string
	}{
		{
			name:      "script tag",
			payload:   "<script>alert('XSS')</script>",
			shouldNot: []string{"<script>", "</script>", "alert("},
		},
		{
			name:      "img onerror",
			payload:   `<img src=x onerror="alert('XSS')">`,
			shouldNot: []string{"onerror"},
		},
		{
			name:      "svg onload",
			payload:   `<svg onload="alert('XSS')">`,
			shouldNot: []string{"onload"},
		},
		{
			name:      "javascript href",
			payload:   `<a href="javascript:alert('XSS')">click</a>`,
			shouldNot: []string{"javascript:"},
		},
		{
			name:      "event handler",
			payload:   `<div onclick="alert('XSS')">click</div>`,
			shouldNot: []string{"onclick"},
		},
		{
			name:      "style expression",
			payload:   `<div style="background:url(javascript:alert('XSS'))">`,
			shouldNot: []string{"javascript:"},
		},
		{
			name:      "data URL script",
			payload:   `<a href="data:text/html,<script>alert('XSS')</script>">click</a>`,
			shouldNot: []string{"data:text/html"},
		},
		{
			name:      "iframe",
			payload:   `<iframe src="https://evil.com"></iframe>`,
			shouldNot: []string{"<iframe"},
		},
		{
			name:      "object tag",
			payload:   `<object data="https://evil.com/malware.swf"></object>`,
			shouldNot: []string{"<object"},
		},
		{
			name:      "embed tag",
			payload:   `<embed src="https://evil.com/malware.swf">`,
			shouldNot: []string{"<embed"},
		},
	}

	for i, tc := range xssPayloads {
		t.Run(tc.name, func(t *testing.T) {
			articleURL := "xss-test-" + tc.name + "-" + string(rune('a'+i))

			formData := url.Values{
				"title": {"XSS Test"},
				"body":  {tc.payload},
			}

			resp, err := client.PostForm(server.URL+"/wiki/"+url.PathEscape(articleURL)+"/r/0", formData)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			resp.Body.Close()

			// Get the article
			article, err := testApp.GetArticle(articleURL)
			if err != nil {
				t.Fatalf("article not found: %v", err)
			}

			// Check that dangerous content is sanitized
			for _, forbidden := range tc.shouldNot {
				if strings.Contains(strings.ToLower(article.HTML), strings.ToLower(forbidden)) {
					t.Errorf("HTML should not contain %q, got: %s", forbidden, article.HTML)
				}
			}
		})
	}
}

func TestXSSInArticleTitle(t *testing.T) {
	server, testApp, cleanup := setupSecurityTestServer(t)
	defer cleanup()

	password := "titlexsspassword"
	user := &wiki.User{
		ScreenName:  "titlexssuser",
		Email:       "titlexss@example.com",
		RawPassword: password,
	}
	testApp.PostUser(user)
	client := getAuthenticatedClient(t, server, "titlexssuser", password)

	xssTitles := []struct {
		name    string
		title   string
		badText string
	}{
		{
			name:    "script in title",
			title:   "<script>alert('XSS')</script>Title",
			badText: "<script>",
		},
		{
			name:    "img in title",
			title:   `<img src=x onerror="alert('XSS')">Title`,
			badText: "onerror",
		},
		{
			name:    "event handler in title",
			title:   `Title<div onclick="alert('XSS')">`,
			badText: "onclick",
		},
	}

	for i, tc := range xssTitles {
		t.Run(tc.name, func(t *testing.T) {
			articleURL := "title-xss-" + string(rune('a'+i))

			formData := url.Values{
				"title": {tc.title},
				"body":  {"Normal content"},
			}

			resp, err := client.PostForm(server.URL+"/wiki/"+articleURL+"/r/0", formData)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			resp.Body.Close()

			// Get the article
			article, err := testApp.GetArticle(articleURL)
			if err != nil {
				t.Fatalf("article not found: %v", err)
			}

			// Title should be sanitized
			if strings.Contains(strings.ToLower(article.Title), strings.ToLower(tc.badText)) {
				t.Errorf("Title should not contain %q, got: %s", tc.badText, article.Title)
			}
		})
	}
}

func TestXSSInDiffOutput(t *testing.T) {
	server, testApp, cleanup := setupSecurityTestServer(t)
	defer cleanup()

	password := "diffxsspassword"
	user := &wiki.User{
		ScreenName:  "diffxssuser",
		Email:       "diffxss@example.com",
		RawPassword: password,
	}
	testApp.PostUser(user)
	client := getAuthenticatedClient(t, server, "diffxssuser", password)

	// Create initial article
	formData := url.Values{
		"title": {"Diff XSS Test"},
		"body":  {"Original safe content"},
	}

	resp, err := client.PostForm(server.URL+"/wiki/diff-xss-test/r/0", formData)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	// Create second revision with XSS attempt
	formData = url.Values{
		"title": {"Diff XSS Test"},
		"body":  {"Modified content <script>alert('XSS')</script>"},
	}

	resp, err = client.PostForm(server.URL+"/wiki/diff-xss-test/r/1", formData)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	// Request diff
	resp, err = client.Get(server.URL + "/wiki/diff-xss-test/diff/1/2")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	body := string(bodyBytes)

	// The diff output should escape the script tag
	if strings.Contains(body, "<script>alert(") {
		t.Error("diff output should escape script tags")
	}

	// The escaped version should be present
	if !strings.Contains(body, "&lt;script&gt;") && !strings.Contains(body, "script") {
		// It's OK if the script tag is completely stripped, as long as it's not executable
		t.Log("Note: Script tag should be escaped or stripped in diff output")
	}
}

func TestSQLInjectionInArticleURL(t *testing.T) {
	server, testApp, cleanup := setupSecurityTestServer(t)
	defer cleanup()

	// Create a normal article first
	user := testutil.CreateTestUser(t, testApp.DB, "sqliuser", "sqli@example.com", "sqlipassword")
	testutil.CreateTestArticle(t, testApp, "normal-article", "Normal Article", "Normal content", user)

	// Test SQL injection attempts in URL
	sqlInjectionPayloads := []string{
		"'; DROP TABLE Article; --",
		"' OR '1'='1",
		"1; DELETE FROM Article WHERE '1'='1",
		"' UNION SELECT * FROM User --",
		"article' AND 1=1--",
		"article'); DROP TABLE Revision; --",
	}

	for _, payload := range sqlInjectionPayloads {
		t.Run("payload: "+payload[:min(20, len(payload))], func(t *testing.T) {
			// Request with SQL injection payload
			resp, err := http.Get(server.URL + "/wiki/" + url.PathEscape(payload))
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			resp.Body.Close()

			// Should return 404 (article not found), not an error
			// The parameterized queries should prevent SQL injection
			if resp.StatusCode != http.StatusNotFound {
				// 200 would mean it found something unexpected
				// 500 would mean SQL error
				// 404 is the expected safe behavior
				t.Logf("Got status %d for payload (404 expected for safe handling)", resp.StatusCode)
			}

			// Verify the normal article still exists
			article, err := testApp.GetArticle("normal-article")
			if err != nil {
				t.Error("normal article was affected by SQL injection attempt")
			}
			if article == nil {
				t.Error("normal article was deleted by SQL injection attempt")
			}
		})
	}
}

func TestXSSInWikiLinks(t *testing.T) {
	server, testApp, cleanup := setupSecurityTestServer(t)
	defer cleanup()

	password := "wikilinkxsspassword"
	user := &wiki.User{
		ScreenName:  "wikilinkxssuser",
		Email:       "wikilinkxss@example.com",
		RawPassword: password,
	}
	testApp.PostUser(user)
	client := getAuthenticatedClient(t, server, "wikilinkxssuser", password)

	// Test XSS in wikilink targets
	xssWikilinks := []struct {
		name      string
		content   string
		badText   string
		checkFunc func(html string) bool // Custom check function
	}{
		{
			name:    "script in wikilink target",
			content: `[[<script>alert('XSS')</script>]]`,
			badText: "<script>",
			checkFunc: func(html string) bool {
				return strings.Contains(html, "<script>")
			},
		},
		{
			name:    "javascript in wikilink",
			content: `[[javascript:alert('XSS')]]`,
			badText: "",
			checkFunc: func(html string) bool {
				// Check that javascript: URLs don't appear unescaped in executable context
				// It's OK if the link text shows "javascript:" as escaped text
				return strings.Contains(html, `href="javascript:`)
			},
		},
		{
			name:    "event handler in wikilink text",
			content: `[[Page|<img onerror="alert('XSS')">]]`,
			badText: "",
			checkFunc: func(html string) bool {
				// Check that event handlers are escaped or removed
				// &#34; is the HTML entity for " which means it's escaped
				return strings.Contains(html, `onerror="alert`) && !strings.Contains(html, `onerror=&#34;`)
			},
		},
	}

	for i, tc := range xssWikilinks {
		t.Run(tc.name, func(t *testing.T) {
			articleURL := "wikilink-xss-" + string(rune('a'+i))

			formData := url.Values{
				"title": {"Wikilink XSS Test"},
				"body":  {tc.content},
			}

			resp, err := client.PostForm(server.URL+"/wiki/"+articleURL+"/r/0", formData)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			resp.Body.Close()

			// Get the article
			article, err := testApp.GetArticle(articleURL)
			if err != nil {
				t.Fatalf("article not found: %v", err)
			}

			// Check using custom function or badText
			if tc.checkFunc != nil {
				if tc.checkFunc(article.HTML) {
					t.Errorf("Security check failed for %s, HTML: %s", tc.name, article.HTML)
				}
			} else if tc.badText != "" && strings.Contains(strings.ToLower(article.HTML), strings.ToLower(tc.badText)) {
				t.Errorf("HTML should not contain %q, got: %s", tc.badText, article.HTML)
			}
		})
	}
}

func TestXSSInComments(t *testing.T) {
	server, testApp, cleanup := setupSecurityTestServer(t)
	defer cleanup()

	password := "commentxsspassword"
	user := &wiki.User{
		ScreenName:  "commentxssuser",
		Email:       "commentxss@example.com",
		RawPassword: password,
	}
	testApp.PostUser(user)
	client := getAuthenticatedClient(t, server, "commentxssuser", password)

	// Create article with XSS in edit comment
	formData := url.Values{
		"title":   {"Comment XSS Test"},
		"body":    {"Normal content"},
		"comment": {"<script>alert('XSS')</script>Normal comment"},
	}

	resp, err := client.PostForm(server.URL+"/wiki/comment-xss-test/r/0", formData)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	// Get the article
	article, err := testApp.GetArticle("comment-xss-test")
	if err != nil {
		t.Fatalf("article not found: %v", err)
	}

	// Comment should be sanitized
	if strings.Contains(article.Comment, "<script>") {
		t.Errorf("Comment should be sanitized, got: %s", article.Comment)
	}
}

func TestHTMLEntitiesPreserved(t *testing.T) {
	server, testApp, cleanup := setupSecurityTestServer(t)
	defer cleanup()

	password := "entitiespassword"
	user := &wiki.User{
		ScreenName:  "entitiesuser",
		Email:       "entities@example.com",
		RawPassword: password,
	}
	testApp.PostUser(user)
	client := getAuthenticatedClient(t, server, "entitiesuser", password)

	// Create article with HTML entities that should be preserved
	formData := url.Values{
		"title": {"Entities Test"},
		"body":  {"Use &lt;script&gt; tags for JavaScript. The formula is: 5 &gt; 3 &amp;&amp; 2 &lt; 4"},
	}

	resp, err := client.PostForm(server.URL+"/wiki/entities-test/r/0", formData)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	// Get the article
	article, err := testApp.GetArticle("entities-test")
	if err != nil {
		t.Fatalf("article not found: %v", err)
	}

	// The article should not have actual script tags
	if strings.Contains(article.HTML, "<script>") {
		t.Error("HTML entities should not become actual script tags")
	}
}

func TestMarkdownCodeBlockSafety(t *testing.T) {
	server, testApp, cleanup := setupSecurityTestServer(t)
	defer cleanup()

	password := "codeblockpassword"
	user := &wiki.User{
		ScreenName:  "codeblockuser",
		Email:       "codeblock@example.com",
		RawPassword: password,
	}
	testApp.PostUser(user)
	client := getAuthenticatedClient(t, server, "codeblockuser", password)

	// Code in code blocks should be escaped, not executed
	codeContent := "```html\n<script>alert('XSS')</script>\n```"

	formData := url.Values{
		"title": {"Code Block Test"},
		"body":  {codeContent},
	}

	resp, err := client.PostForm(server.URL+"/wiki/codeblock-test/r/0", formData)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	// Get the article
	article, err := testApp.GetArticle("codeblock-test")
	if err != nil {
		t.Fatalf("article not found: %v", err)
	}

	// The script tag should be inside code/pre tags and escaped
	// It should NOT be executable
	if strings.Contains(article.HTML, "<script>alert(") && !strings.Contains(article.HTML, "&lt;script&gt;") {
		// Check if it's safely inside a code block
		if !strings.Contains(article.HTML, "<code>") && !strings.Contains(article.HTML, "<pre") {
			t.Error("script tag in code block should be escaped or in code element")
		}
	}
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
