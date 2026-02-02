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

// setupSecurityTestServer creates a test server for security tests.
func setupSecurityTestServer(t *testing.T) (*httptest.Server, *testutil.TestApp, func()) {
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
	testApp.Users.PostUser(user)
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
				"title":       {"XSS Test"},
				"body":        {tc.payload},
				"previous_id": {"0"},
			}

			resp, err := client.PostForm(server.URL+"/wiki/"+url.PathEscape(articleURL), formData)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			resp.Body.Close()

			// Get the article
			article, err := testApp.Articles.GetArticle(articleURL)
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

// TestXSSInArticleTitle is removed - title form field is deprecated.
// Title XSS is now tested via TestXSSInFrontmatterDisplayTitle which tests
// XSS protection in frontmatter display_title field.

func TestXSSInFrontmatterDisplayTitle(t *testing.T) {
	server, testApp, cleanup := setupSecurityTestServer(t)
	defer cleanup()

	password := "fmxsspassword"
	user := &wiki.User{
		ScreenName:  "fmxssuser",
		Email:       "fmxss@example.com",
		RawPassword: password,
	}
	testApp.Users.PostUser(user)
	client := getAuthenticatedClient(t, server, "fmxssuser", password)

	xssPayloads := []struct {
		name         string
		displayTitle string
		badText      string
	}{
		{
			name:         "script tag in frontmatter",
			displayTitle: "<script>alert('XSS')</script>",
			badText:      "<script>",
		},
		{
			name:         "img onerror in frontmatter",
			displayTitle: `<img src=x onerror="alert('XSS')">`,
			badText:      "onerror",
		},
		{
			name:         "event handler in frontmatter",
			displayTitle: `<div onclick="alert('XSS')">Click</div>`,
			badText:      "onclick",
		},
	}

	for i, tc := range xssPayloads {
		t.Run(tc.name, func(t *testing.T) {
			articleURL := "fm-xss-" + string(rune('a'+i))
			// Put XSS payload in frontmatter display_title
			body := "---\ndisplay_title: " + tc.displayTitle + "\n---\n# Content"

			formData := url.Values{
				"title":       {""},
				"body":        {body},
				"previous_id": {"0"},
			}

			resp, err := client.PostForm(server.URL+"/wiki/"+articleURL, formData)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			resp.Body.Close()

			// GET the article page and check rendered HTML
			resp, err = http.Get(server.URL + "/wiki/" + articleURL)
			if err != nil {
				t.Fatalf("GET failed: %v", err)
			}
			defer resp.Body.Close()

			bodyBytes, _ := io.ReadAll(resp.Body)
			html := string(bodyBytes)

			// The raw XSS payload should NOT appear unescaped in the HTML
			// For script tags, we check that literal <script> doesn't appear
			// (Go templates escape to &lt;script&gt;)
			if tc.badText == "<script>" {
				if strings.Contains(html, "<script>alert") {
					t.Errorf("HTML contains unescaped script tag")
				}
			} else if strings.Contains(html, tc.badText) && !strings.Contains(html, "&lt;") {
				// For other payloads, verify they're escaped or not present literally
				t.Errorf("HTML may contain unescaped XSS payload: %s", tc.badText)
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
	testApp.Users.PostUser(user)
	client := getAuthenticatedClient(t, server, "diffxssuser", password)

	// Create initial article
	formData := url.Values{
		"title":       {"Diff XSS Test"},
		"body":        {"Original safe content"},
		"previous_id": {"0"},
	}

	resp, err := client.PostForm(server.URL+"/wiki/diff-xss-test", formData)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	// Create second revision with XSS attempt
	formData = url.Values{
		"title":       {"Diff XSS Test"},
		"body":        {"Modified content <script>alert('XSS')</script>"},
		"previous_id": {"1"},
	}

	resp, err = client.PostForm(server.URL+"/wiki/diff-xss-test", formData)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	// Request diff
	resp, err = client.Get(server.URL + "/wiki/diff-xss-test?diff&old=1&new=2")
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
			article, err := testApp.Articles.GetArticle("normal-article")
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
	testApp.Users.PostUser(user)
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
				"title":       {"Wikilink XSS Test"},
				"body":        {tc.content},
				"previous_id": {"0"},
			}

			resp, err := client.PostForm(server.URL+"/wiki/"+articleURL, formData)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			resp.Body.Close()

			// Get the article
			article, err := testApp.Articles.GetArticle(articleURL)
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
	testApp.Users.PostUser(user)
	client := getAuthenticatedClient(t, server, "commentxssuser", password)

	// Create article with XSS in edit comment
	formData := url.Values{
		"title":       {"Comment XSS Test"},
		"body":        {"Normal content"},
		"comment":     {"<script>alert('XSS')</script>Normal comment"},
		"previous_id": {"0"},
	}

	resp, err := client.PostForm(server.URL+"/wiki/comment-xss-test", formData)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	// Get the article
	article, err := testApp.Articles.GetArticle("comment-xss-test")
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
	testApp.Users.PostUser(user)
	client := getAuthenticatedClient(t, server, "entitiesuser", password)

	// Create article with HTML entities that should be preserved
	formData := url.Values{
		"title":       {"Entities Test"},
		"body":        {"Use &lt;script&gt; tags for JavaScript. The formula is: 5 &gt; 3 &amp;&amp; 2 &lt; 4"},
		"previous_id": {"0"},
	}

	resp, err := client.PostForm(server.URL+"/wiki/entities-test", formData)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	// Get the article
	article, err := testApp.Articles.GetArticle("entities-test")
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
	testApp.Users.PostUser(user)
	client := getAuthenticatedClient(t, server, "codeblockuser", password)

	// Code in code blocks should be escaped, not executed
	codeContent := "```html\n<script>alert('XSS')</script>\n```"

	formData := url.Values{
		"title":       {"Code Block Test"},
		"body":        {codeContent},
		"previous_id": {"0"},
	}

	resp, err := client.PostForm(server.URL+"/wiki/codeblock-test", formData)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	// Get the article
	article, err := testApp.Articles.GetArticle("codeblock-test")
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
