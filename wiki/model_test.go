package wiki_test

import (
	"testing"
	"time"

	"github.com/danielledeleo/periwiki/testutil"
	"github.com/danielledeleo/periwiki/wiki"
)

func TestGetArticle(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	// Create a test user
	user := testutil.CreateTestUser(t, app.DB, "testuser", "test@example.com", "password123")

	// Create a test article
	testutil.CreateTestArticle(t, app, "test-article", "Test Article", "# Hello\n\nThis is a test.", user)

	t.Run("existing article", func(t *testing.T) {
		article, err := app.GetArticle("test-article")
		if err != nil {
			t.Fatalf("GetArticle failed: %v", err)
		}

		if article.URL != "test-article" {
			t.Errorf("expected URL 'test-article', got %q", article.URL)
		}
		if article.Title != "Test Article" {
			t.Errorf("expected Title 'Test Article', got %q", article.Title)
		}
	})

	t.Run("non-existent article", func(t *testing.T) {
		_, err := app.GetArticle("nonexistent")
		if err != wiki.ErrGenericNotFound {
			t.Errorf("expected ErrGenericNotFound, got: %v", err)
		}
	})
}

func TestPostArticle(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, app.DB, "testuser", "test@example.com", "password123")

	t.Run("creates new article with rendered HTML", func(t *testing.T) {
		article := wiki.NewArticle("new-article", "New Article", "# Heading\n\nParagraph text.")
		article.Creator = user
		article.PreviousID = 0

		err := app.PostArticle(article)
		if err != nil {
			t.Fatalf("PostArticle failed: %v", err)
		}

		// Verify article was created
		created, err := app.GetArticle("new-article")
		if err != nil {
			t.Fatalf("GetArticle failed: %v", err)
		}

		// Verify markdown was rendered to HTML
		if created.HTML == "" {
			t.Error("expected non-empty HTML")
		}
		if created.HTML == created.Markdown {
			t.Error("expected HTML to be rendered from markdown")
		}

		// Verify hash was generated
		if created.Hash == "" {
			t.Error("expected non-empty hash")
		}
	})

	t.Run("creates new revision for existing article", func(t *testing.T) {
		// Create initial article
		article := wiki.NewArticle("revision-test", "Revision Test", "Version 1")
		article.Creator = user
		article.PreviousID = 0

		err := app.PostArticle(article)
		if err != nil {
			t.Fatalf("PostArticle (v1) failed: %v", err)
		}

		// Small delay to ensure unique timestamps
		time.Sleep(10 * time.Millisecond)

		// Get the article to find its revision ID
		v1, err := app.GetArticle("revision-test")
		if err != nil {
			t.Fatalf("GetArticle failed: %v", err)
		}

		// Create new revision with a fresh article object and different content
		article2 := wiki.NewArticle("revision-test", "Revision Test Updated", "Version 2 content")
		article2.Creator = user
		article2.PreviousID = v1.ID

		err = app.PostArticle(article2)
		if err != nil {
			t.Fatalf("PostArticle (v2) failed: %v", err)
		}

		// Verify latest is v2
		latest, err := app.GetArticle("revision-test")
		if err != nil {
			t.Fatalf("GetArticle failed: %v", err)
		}

		if latest.Markdown != "Version 2 content" {
			t.Errorf("expected 'Version 2 content', got %q", latest.Markdown)
		}
		if latest.ID != 2 {
			t.Errorf("expected revision ID 2, got %d", latest.ID)
		}
	})
}

func TestPostArticleNotModified(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, app.DB, "testuser", "test@example.com", "password123")

	// Create initial article
	article := wiki.NewArticle("not-modified-test", "Test", "Same content")
	article.Creator = user
	article.PreviousID = 0

	err := app.PostArticle(article)
	if err != nil {
		t.Fatalf("PostArticle failed: %v", err)
	}

	created, err := app.GetArticle("not-modified-test")
	if err != nil {
		t.Fatalf("GetArticle failed: %v", err)
	}

	// Try to post identical content
	article.PreviousID = created.ID

	err = app.PostArticle(article)
	if err != wiki.ErrArticleNotModified {
		t.Errorf("expected ErrArticleNotModified, got: %v", err)
	}
}

func TestPostUser(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	t.Run("creates valid user", func(t *testing.T) {
		user := &wiki.User{
			ScreenName:  "validuser",
			Email:       "valid@example.com",
			RawPassword: "securepassword123",
		}

		err := app.PostUser(user)
		if err != nil {
			t.Fatalf("PostUser failed: %v", err)
		}

		// Verify user was created
		retrieved, err := app.GetUserByScreenName("validuser")
		if err != nil {
			t.Fatalf("GetUserByScreenName failed: %v", err)
		}

		if retrieved.ScreenName != "validuser" {
			t.Errorf("expected screenname 'validuser', got %q", retrieved.ScreenName)
		}
		if retrieved.Email != "valid@example.com" {
			t.Errorf("expected email 'valid@example.com', got %q", retrieved.Email)
		}
	})

	t.Run("allows unicode usernames", func(t *testing.T) {
		user := &wiki.User{
			ScreenName:  "用户名", // Chinese characters
			Email:       "unicode@example.com",
			RawPassword: "securepassword123",
		}

		err := app.PostUser(user)
		if err != nil {
			t.Fatalf("PostUser with unicode failed: %v", err)
		}
	})

	t.Run("allows hyphens and underscores", func(t *testing.T) {
		user := &wiki.User{
			ScreenName:  "user-name_123",
			Email:       "special@example.com",
			RawPassword: "securepassword123",
		}

		err := app.PostUser(user)
		if err != nil {
			t.Fatalf("PostUser with special chars failed: %v", err)
		}
	})
}

func TestPostUserValidation(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	tests := []struct {
		name        string
		screenname  string
		email       string
		password    string
		expectedErr error
	}{
		{
			name:        "empty username",
			screenname:  "",
			email:       "test@example.com",
			password:    "password123",
			expectedErr: wiki.ErrEmptyUsername,
		},
		{
			name:        "short password",
			screenname:  "testuser",
			email:       "test@example.com",
			password:    "short",
			expectedErr: nil, // will contain ErrPasswordTooShort but wrapped
		},
		{
			name:        "invalid characters",
			screenname:  "user@name",
			email:       "test@example.com",
			password:    "password123",
			expectedErr: wiki.ErrBadUsername,
		},
		{
			name:        "invalid characters with spaces",
			screenname:  "user name",
			email:       "test@example.com",
			password:    "password123",
			expectedErr: wiki.ErrBadUsername,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			user := &wiki.User{
				ScreenName:  tc.screenname,
				Email:       tc.email,
				RawPassword: tc.password,
			}

			err := app.PostUser(user)

			if tc.expectedErr != nil {
				if err != tc.expectedErr {
					t.Errorf("expected error %v, got: %v", tc.expectedErr, err)
				}
			} else {
				// For short password, we just check that an error occurred
				if tc.name == "short password" && err == nil {
					t.Error("expected error for short password")
				}
			}
		})
	}
}

func TestCheckUserPassword(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	// Create a user with known password
	password := "correctpassword123"
	user := &wiki.User{
		ScreenName:  "passwordtest",
		Email:       "password@example.com",
		RawPassword: password,
	}
	err := app.PostUser(user)
	if err != nil {
		t.Fatalf("PostUser failed: %v", err)
	}

	t.Run("correct password", func(t *testing.T) {
		checkUser := &wiki.User{
			ScreenName:  "passwordtest",
			RawPassword: password,
		}
		err := app.CheckUserPassword(checkUser)
		if err != nil {
			t.Errorf("expected no error for correct password, got: %v", err)
		}
	})

	t.Run("incorrect password", func(t *testing.T) {
		checkUser := &wiki.User{
			ScreenName:  "passwordtest",
			RawPassword: "wrongpassword",
		}
		err := app.CheckUserPassword(checkUser)
		if err != wiki.ErrIncorrectPassword {
			t.Errorf("expected ErrIncorrectPassword, got: %v", err)
		}
	})

	t.Run("non-existent user", func(t *testing.T) {
		checkUser := &wiki.User{
			ScreenName:  "nonexistent",
			RawPassword: "anypassword",
		}
		err := app.CheckUserPassword(checkUser)
		if err != wiki.ErrUsernameNotFound {
			t.Errorf("expected ErrUsernameNotFound, got: %v", err)
		}
	})
}

func TestGetRandomArticleURL(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	t.Run("empty database returns error", func(t *testing.T) {
		_, err := app.GetRandomArticleURL()
		if err != wiki.ErrNoArticles {
			t.Errorf("expected ErrNoArticles, got: %v", err)
		}
	})

	t.Run("returns valid article URL", func(t *testing.T) {
		user := testutil.CreateTestUser(t, app.DB, "testuser", "test@example.com", "password123")

		urls := []string{"article-a", "article-b", "article-c"}
		for _, url := range urls {
			testutil.CreateTestArticle(t, app, url, "Title", "Content", user)
		}

		url, err := app.GetRandomArticleURL()
		if err != nil {
			t.Fatalf("GetRandomArticleURL failed: %v", err)
		}

		found := false
		for _, valid := range urls {
			if url == valid {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("returned URL %q is not in expected list", url)
		}
	})
}

func TestRender(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	tests := []struct {
		name     string
		markdown string
		contains string
	}{
		{
			name:     "heading",
			markdown: "# Hello World",
			contains: "<h1", // heading with possible id attribute
		},
		{
			name:     "paragraph",
			markdown: "This is a paragraph.",
			contains: "<p>This is a paragraph.</p>",
		},
		{
			name:     "bold text",
			markdown: "**bold**",
			contains: "<strong>bold</strong>",
		},
		{
			name:     "link",
			markdown: "[example](https://example.com)",
			contains: `<a href="https://example.com"`,
		},
		{
			name:     "code block",
			markdown: "```go\nfunc main() {}\n```",
			contains: "<pre",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			html, err := app.Render(tc.markdown)
			if err != nil {
				t.Fatalf("Render failed: %v", err)
			}

			if !containsSubstring(html, tc.contains) {
				t.Errorf("expected HTML to contain %q, got: %s", tc.contains, html)
			}
		})
	}
}

func TestRenderSanitizesHTML(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	tests := []struct {
		name       string
		markdown   string
		shouldHave string
		shouldNot  string
	}{
		{
			name:       "removes script tags",
			markdown:   "<script>alert('xss')</script>",
			shouldNot:  "<script>",
			shouldHave: "",
		},
		{
			name:       "removes onclick",
			markdown:   `<a href="#" onclick="alert('xss')">click</a>`,
			shouldNot:  "onclick",
			shouldHave: "", // anchor may be stripped entirely, which is fine
		},
		{
			name:       "removes javascript protocol",
			markdown:   `<a href="javascript:alert('xss')">click</a>`,
			shouldNot:  "javascript:",
			shouldHave: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			html, err := app.Render(tc.markdown)
			if err != nil {
				t.Fatalf("Render failed: %v", err)
			}

			if tc.shouldNot != "" && containsSubstring(html, tc.shouldNot) {
				t.Errorf("HTML should not contain %q, got: %s", tc.shouldNot, html)
			}

			if tc.shouldHave != "" && !containsSubstring(html, tc.shouldHave) {
				t.Errorf("HTML should contain %q, got: %s", tc.shouldHave, html)
			}
		})
	}
}

func TestGetArticleByRevisionID(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, app.DB, "testuser", "test@example.com", "password123")

	// Create article with multiple revisions
	article := wiki.NewArticle("rev-test", "Revision Test", "Version 1")
	article.Creator = user
	article.PreviousID = 0
	err := app.PostArticle(article)
	if err != nil {
		t.Fatalf("PostArticle failed: %v", err)
	}

	v1, _ := app.GetArticle("rev-test")

	article.Markdown = "Version 2"
	article.PreviousID = v1.ID
	err = app.PostArticle(article)
	if err != nil {
		t.Fatalf("PostArticle failed: %v", err)
	}

	t.Run("retrieves specific revision", func(t *testing.T) {
		rev1, err := app.GetArticleByRevisionID("rev-test", 1)
		if err != nil {
			t.Fatalf("GetArticleByRevisionID failed: %v", err)
		}
		if rev1.Markdown != "Version 1" {
			t.Errorf("expected 'Version 1', got %q", rev1.Markdown)
		}

		rev2, err := app.GetArticleByRevisionID("rev-test", 2)
		if err != nil {
			t.Fatalf("GetArticleByRevisionID failed: %v", err)
		}
		if rev2.Markdown != "Version 2" {
			t.Errorf("expected 'Version 2', got %q", rev2.Markdown)
		}
	})

	t.Run("non-existent revision", func(t *testing.T) {
		_, err := app.GetArticleByRevisionID("rev-test", 999)
		if err != wiki.ErrRevisionNotFound {
			t.Errorf("expected ErrRevisionNotFound, got: %v", err)
		}
	})
}

func TestGetRevisionHistory(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, app.DB, "testuser", "test@example.com", "password123")

	// Create article with multiple revisions
	article := wiki.NewArticle("history-test", "History Test", "Version 1")
	article.Creator = user
	article.PreviousID = 0
	article.Comment = "Initial"
	err := app.PostArticle(article)
	if err != nil {
		t.Fatalf("PostArticle failed: %v", err)
	}

	v1, _ := app.GetArticle("history-test")

	// Small delay to ensure unique timestamps
	time.Sleep(10 * time.Millisecond)

	// Create second revision with fresh article object and different content
	article2 := wiki.NewArticle("history-test", "History Test Updated", "Version 2 - Updated content")
	article2.Creator = user
	article2.PreviousID = v1.ID
	article2.Comment = "Update"
	err = app.PostArticle(article2)
	if err != nil {
		t.Fatalf("PostArticle (v2) failed: %v", err)
	}

	history, err := app.GetRevisionHistory("history-test")
	if err != nil {
		t.Fatalf("GetRevisionHistory failed: %v", err)
	}

	if len(history) != 2 {
		t.Fatalf("expected 2 revisions, got %d", len(history))
	}

	// Newest first
	if history[0].ID != 2 {
		t.Errorf("expected newest revision first (ID 2), got %d", history[0].ID)
	}
}

// Helper function
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && searchString(s, substr)))
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
