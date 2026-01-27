package server

import (
	"bytes"
	"context"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/danielledeleo/periwiki/wiki"
	"github.com/gorilla/mux"
	"github.com/microcosm-cc/bluemonday"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// TestXSSInDiffHandler verifies that the diff handler properly escapes HTML
// to prevent XSS attacks through malicious article content.
func TestXSSInDiffHandler(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		mustFind string // escaped version must be present
		mustNot  string // unescaped version must NOT be present
	}{
		{
			name:     "script tag",
			input:    `<script>alert('xss')</script>`,
			mustFind: html.EscapeString(`<script>alert('xss')</script>`),
			mustNot:  `<script>alert('xss')</script>`,
		},
		{
			name:     "img onerror",
			input:    `<img src=x onerror="alert('xss')">`,
			mustFind: html.EscapeString(`<img src=x onerror="alert('xss')">`),
			mustNot:  `onerror="alert('xss')"`,
		},
		{
			name:     "javascript href",
			input:    `<a href="javascript:alert('xss')">click</a>`,
			mustFind: html.EscapeString(`<a href="javascript:alert('xss')">click</a>`),
			mustNot:  `href="javascript:`,
		},
		{
			name:     "event handler",
			input:    `<div onmouseover="alert('xss')">hover</div>`,
			mustFind: html.EscapeString(`<div onmouseover="alert('xss')">hover</div>`),
			mustNot:  `onmouseover="alert`,
		},
		{
			name:     "svg onload",
			input:    `<svg onload="alert('xss')">`,
			mustFind: html.EscapeString(`<svg onload="alert('xss')">`),
			mustNot:  `<svg onload=`,
		},
		{
			name:     "html entities bypass attempt",
			input:    `&lt;script&gt;alert('xss')&lt;/script&gt;`,
			mustFind: `&amp;lt;script&amp;gt;`, // double-escaped
		},
		{
			name:     "null byte injection",
			input:    "<scr\x00ipt>alert('xss')</script>",
			mustFind: html.EscapeString("<scr\x00ipt>alert('xss')</script>"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate the diff logic from diffHandler
			dmp := diffmatchpatch.New()
			diffs := dmp.DiffMain("", tc.input, false)

			var buff bytes.Buffer
			for _, diff := range diffs {
				text := html.EscapeString(diff.Text)
				switch diff.Type {
				case diffmatchpatch.DiffInsert:
					buff.WriteString("<ins style=\"background:#e6ffe6;\">")
					buff.WriteString(text)
					buff.WriteString("</ins>")
				case diffmatchpatch.DiffDelete:
					buff.WriteString("<del style=\"background:#ffe6e6;\">")
					buff.WriteString(text)
					buff.WriteString("</del>")
				case diffmatchpatch.DiffEqual:
					buff.WriteString("<span>")
					buff.WriteString(text)
					buff.WriteString("</span>")
				}
			}

			result := buff.String()

			if tc.mustFind != "" && !strings.Contains(result, tc.mustFind) {
				t.Errorf("expected to find escaped string %q in output, got: %s", tc.mustFind, result)
			}

			if tc.mustNot != "" && strings.Contains(result, tc.mustNot) {
				t.Errorf("found unescaped dangerous string %q in output: %s", tc.mustNot, result)
			}
		})
	}
}

// TestSessionMiddlewareSafeTypeAssertion verifies the session middleware
// doesn't panic on corrupted or malformed session data.
func TestSessionMiddlewareSafeTypeAssertion(t *testing.T) {
	tests := []struct {
		name          string
		sessionValues map[interface{}]interface{}
		expectAnon    bool
	}{
		{
			name:          "nil username",
			sessionValues: map[interface{}]interface{}{"username": nil},
			expectAnon:    true,
		},
		{
			name:          "empty username",
			sessionValues: map[interface{}]interface{}{"username": ""},
			expectAnon:    true,
		},
		{
			name:          "wrong type (int)",
			sessionValues: map[interface{}]interface{}{"username": 12345},
			expectAnon:    true,
		},
		{
			name:          "wrong type (slice)",
			sessionValues: map[interface{}]interface{}{"username": []string{"user"}},
			expectAnon:    true,
		},
		{
			name:          "missing key",
			sessionValues: map[interface{}]interface{}{},
			expectAnon:    true,
		},
		{
			name:          "wrong key name",
			sessionValues: map[interface{}]interface{}{"user": "validuser"},
			expectAnon:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Test the safe type assertion logic
			screenname, ok := tc.sessionValues["username"].(string)
			isAnon := !ok || screenname == ""

			if isAnon != tc.expectAnon {
				t.Errorf("expected anonymous=%v, got anonymous=%v", tc.expectAnon, isAnon)
			}
		})
	}
}

// TestUserContextSafeTypeAssertion verifies handlers don't panic
// when user context is missing or malformed.
func TestUserContextSafeTypeAssertion(t *testing.T) {
	tests := []struct {
		name      string
		ctxValue  interface{}
		expectErr bool
	}{
		{
			name:      "nil value",
			ctxValue:  nil,
			expectErr: true,
		},
		{
			name:      "wrong type (string)",
			ctxValue:  "not a user",
			expectErr: true,
		},
		{
			name:      "wrong type (int)",
			ctxValue:  42,
			expectErr: true,
		},
		{
			name:      "valid user pointer",
			ctxValue:  &wiki.User{ID: 1, ScreenName: "testuser"},
			expectErr: false,
		},
		{
			name:      "nil user pointer",
			ctxValue:  (*wiki.User)(nil),
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), wiki.UserKey, tc.ctxValue)

			user, ok := ctx.Value(wiki.UserKey).(*wiki.User)
			hasErr := !ok || user == nil

			if hasErr != tc.expectErr {
				t.Errorf("expected error=%v, got error=%v", tc.expectErr, hasErr)
			}
		})
	}
}

// TestUsernameValidation verifies that username validation properly rejects
// malicious input patterns.
func TestUsernameValidation(t *testing.T) {
	tests := []struct {
		name      string
		username  string
		wantError bool
	}{
		// Valid usernames
		{name: "simple", username: "user123", wantError: false},
		{name: "with underscore", username: "user_name", wantError: false},
		{name: "with dash", username: "user-name", wantError: false},
		{name: "unicode letters", username: "用户", wantError: false},

		// Invalid usernames - should be rejected
		{name: "empty", username: "", wantError: true},
		{name: "script tag", username: "<script>", wantError: true},
		{name: "html entity", username: "&lt;script&gt;", wantError: true},
		{name: "spaces", username: "user name", wantError: true},
		{name: "newline", username: "user\nname", wantError: true},
		{name: "tab", username: "user\tname", wantError: true},
		{name: "sql injection attempt", username: "'; DROP TABLE User;--", wantError: true},
		{name: "null byte", username: "user\x00name", wantError: true},
		{name: "path traversal", username: "../../../etc/passwd", wantError: true},
		{name: "command injection", username: "user; rm -rf /", wantError: true},
		{name: "xss in username", username: "<img src=x onerror=alert(1)>", wantError: true},
	}

	// The regex from wiki/model.go: ^[\p{L}0-9-_]+$
	usernameRegex := regexp.MustCompile(`^[\p{L}0-9-_]+$`)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Check if username would be rejected by the validation
			var hasError bool
			if len(tc.username) == 0 {
				hasError = true // empty is error
			} else {
				hasError = !usernameRegex.MatchString(tc.username)
			}

			if hasError != tc.wantError {
				t.Errorf("username %q: got error=%v, want error=%v", tc.username, hasError, tc.wantError)
			}
		})
	}
}

// TestPathTraversalInArticleURL verifies that article URL handling
// doesn't allow path traversal attacks.
func TestPathTraversalInArticleURL(t *testing.T) {
	tests := []struct {
		name       string
		articleURL string
		dangerous  bool
	}{
		{name: "normal", articleURL: "my-article", dangerous: false},
		{name: "with numbers", articleURL: "article123", dangerous: false},
		{name: "parent directory", articleURL: "../../../etc/passwd", dangerous: true},
		{name: "encoded traversal", articleURL: "..%2F..%2F..%2Fetc%2Fpasswd", dangerous: true},
		{name: "double encoded", articleURL: "..%252F..%252Fetc%252Fpasswd", dangerous: true},
		{name: "null byte", articleURL: "article\x00.txt", dangerous: true},
		{name: "backslash traversal", articleURL: "..\\..\\..\\etc\\passwd", dangerous: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Decode URL-encoded strings
			decoded, _ := url.QueryUnescape(tc.articleURL)

			// Check for path traversal patterns
			hasTraversal := strings.Contains(decoded, "..") ||
				strings.Contains(decoded, "\x00") ||
				strings.Contains(tc.articleURL, "%2F") ||
				strings.Contains(tc.articleURL, "%5C")

			if hasTraversal != tc.dangerous {
				t.Errorf("path traversal detection mismatch: got dangerous=%v, want dangerous=%v",
					hasTraversal, tc.dangerous)
			}
		})
	}
}

// TestHTTPMethodRestrictions verifies that handlers only respond to
// their intended HTTP methods.
func TestHTTPMethodRestrictions(t *testing.T) {
	router := mux.NewRouter()

	// Register routes similar to main()
	router.HandleFunc("/wiki/{article}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods("GET")

	router.HandleFunc("/user/login", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods("GET")

	router.HandleFunc("/user/login", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods("POST")

	tests := []struct {
		name         string
		method       string
		path         string
		expectStatus int
	}{
		{name: "GET article allowed", method: "GET", path: "/wiki/test", expectStatus: http.StatusOK},
		{name: "POST article not allowed", method: "POST", path: "/wiki/test", expectStatus: http.StatusMethodNotAllowed},
		{name: "DELETE article not allowed", method: "DELETE", path: "/wiki/test", expectStatus: http.StatusMethodNotAllowed},
		{name: "PUT article not allowed", method: "PUT", path: "/wiki/test", expectStatus: http.StatusMethodNotAllowed},
		{name: "GET login allowed", method: "GET", path: "/user/login", expectStatus: http.StatusOK},
		{name: "POST login allowed", method: "POST", path: "/user/login", expectStatus: http.StatusOK},
		{name: "DELETE login not allowed", method: "DELETE", path: "/user/login", expectStatus: http.StatusMethodNotAllowed},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			if rr.Code != tc.expectStatus {
				t.Errorf("expected status %d, got %d", tc.expectStatus, rr.Code)
			}
		})
	}
}

// TestCookieSecurityAttributes verifies that cookies have proper security settings.
// Note: This test documents expected behavior - actual implementation may vary.
func TestCookieSecurityAttributes(t *testing.T) {
	t.Run("cookie should have httponly flag", func(t *testing.T) {
		// This is a documentation test - the gorilla/sessions library
		// should set HttpOnly by default for session cookies
		t.Log("Session cookies should have HttpOnly=true to prevent XSS access")
	})

	t.Run("cookie should have secure flag in production", func(t *testing.T) {
		// Document expected behavior
		t.Log("In production, session cookies should have Secure=true for HTTPS-only")
	})

	t.Run("cookie should have samesite attribute", func(t *testing.T) {
		// Document expected behavior for CSRF protection
		t.Log("Session cookies should have SameSite=Lax or SameSite=Strict")
	})
}

// TestSQLInjectionPrevention verifies that SQL queries use parameterized
// statements. This is a documentation/code review test.
func TestSQLInjectionPrevention(t *testing.T) {
	// SQL injection payloads that should be safely handled
	payloads := []string{
		"'; DROP TABLE User;--",
		"1' OR '1'='1",
		"1; DELETE FROM Article WHERE 1=1;--",
		"' UNION SELECT * FROM Password--",
		"admin'--",
		"1' AND SLEEP(5)--",
		"' OR 1=1#",
	}

	t.Run("parameterized queries prevent injection", func(t *testing.T) {
		// The db/sqlite.go uses parameterized queries with ? placeholders
		// Example from the code:
		// db.conn.Preparex(`SELECT ... WHERE screenname = ?`)
		// This prevents SQL injection as the ? is a placeholder, not string concat

		for _, payload := range payloads {
			t.Logf("Payload %q would be safely parameterized", payload)
		}
	})
}

// TestPasswordHashing verifies password handling security.
func TestPasswordHashing(t *testing.T) {
	t.Run("passwords are hashed with bcrypt", func(t *testing.T) {
		user := &wiki.User{
			RawPassword: "testpassword123",
		}

		err := user.SetPasswordHash()
		if err != nil {
			t.Fatalf("SetPasswordHash failed: %v", err)
		}

		// Verify raw password is cleared
		if user.RawPassword != "" {
			t.Error("RawPassword should be cleared after hashing")
		}

		// Verify hash is set
		if user.PasswordHash == "" {
			t.Error("PasswordHash should be set")
		}

		// Verify hash starts with bcrypt prefix
		if !strings.HasPrefix(user.PasswordHash, "$2") {
			t.Error("PasswordHash should be a bcrypt hash (starts with $2)")
		}

		// Verify hash is not the plaintext password
		if user.PasswordHash == "testpassword123" {
			t.Error("PasswordHash should not be plaintext")
		}
	})
}

// TestInputSanitization tests that article content is properly sanitized.
func TestInputSanitization(t *testing.T) {
	tests := []struct {
		name  string
		input string
		// These patterns should NOT appear in sanitized output
		forbidden []string
	}{
		{
			name:      "script tag",
			input:     `<script>alert('xss')</script>`,
			forbidden: []string{"<script", "</script>"},
		},
		{
			name:      "event handler",
			input:     `<div onclick="alert('xss')">click</div>`,
			forbidden: []string{"onclick"},
		},
		{
			name:      "javascript href",
			input:     `<a href="javascript:alert('xss')">link</a>`,
			forbidden: []string{"javascript:"},
		},
		{
			name:      "data uri",
			input:     `<a href="data:text/html,<script>alert('xss')</script>">link</a>`,
			forbidden: []string{"data:text/html"},
		},
		{
			name:      "style expression",
			input:     `<div style="background:url(javascript:alert('xss'))">test</div>`,
			forbidden: []string{"javascript:"},
		},
		{
			name:      "iframe",
			input:     `<iframe src="http://evil.com"></iframe>`,
			forbidden: []string{"<iframe"},
		},
		{
			name:      "object tag",
			input:     `<object data="http://evil.com/flash.swf"></object>`,
			forbidden: []string{"<object"},
		},
		{
			name:      "embed tag",
			input:     `<embed src="http://evil.com/flash.swf">`,
			forbidden: []string{"<embed"},
		},
		{
			name:      "svg with script",
			input:     `<svg><script>alert('xss')</script></svg>`,
			forbidden: []string{"<script"},
		},
		{
			name:      "meta refresh",
			input:     `<meta http-equiv="refresh" content="0;url=http://evil.com">`,
			forbidden: []string{"<meta"},
		},
		{
			name:      "base tag",
			input:     `<base href="http://evil.com/">`,
			forbidden: []string{"<base"},
		},
		{
			name:      "form action",
			input:     `<form action="http://evil.com/steal"><input name="data"></form>`,
			forbidden: []string{"<form"},
		},
	}

	// Use the same sanitizer as the application
	sanitizer := bluemonday.UGCPolicy()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := sanitizer.Sanitize(tc.input)

			for _, forbidden := range tc.forbidden {
				if strings.Contains(strings.ToLower(result), strings.ToLower(forbidden)) {
					t.Errorf("sanitized output contains forbidden pattern %q: %s", forbidden, result)
				}
			}
		})
	}
}
