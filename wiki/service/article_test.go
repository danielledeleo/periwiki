package service_test

import (
	"testing"

	"github.com/danielledeleo/periwiki/testutil"
	"github.com/danielledeleo/periwiki/wiki"
)

func TestGetArticle(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	// Create a test user
	user := testutil.CreateTestUser(t, app.DB, "testuser", "test@example.com", "password123")

	// Create a test article
	testutil.CreateTestArticle(t, app, "test-article", "# Hello\n\nThis is a test.", user)

	t.Run("existing article", func(t *testing.T) {
		article, err := app.Articles.GetArticle("test-article")
		if err != nil {
			t.Fatalf("GetArticle failed: %v", err)
		}

		if article.URL != "test-article" {
			t.Errorf("expected URL 'test-article', got %q", article.URL)
		}
	})

	t.Run("non-existent article", func(t *testing.T) {
		_, err := app.Articles.GetArticle("nonexistent")
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
		article := wiki.NewArticle("new-article", "# Heading\n\nParagraph text.")
		article.Creator = user
		article.PreviousID = 0

		err := app.Articles.PostArticle(article)
		if err != nil {
			t.Fatalf("PostArticle failed: %v", err)
		}

		// Verify article was created
		created, err := app.Articles.GetArticle("new-article")
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
		article := wiki.NewArticle("revision-test", "Version 1")
		article.Creator = user
		article.PreviousID = 0

		err := app.Articles.PostArticle(article)
		if err != nil {
			t.Fatalf("PostArticle (v1) failed: %v", err)
		}

		// PostArticle should update article.ID - no need to re-fetch
		if article.ID != 1 {
			t.Fatalf("expected article.ID = 1 after PostArticle, got %d", article.ID)
		}

		// Create new revision using the updated ID directly
		article2 := wiki.NewArticle("revision-test", "Version 2 content")
		article2.Creator = user
		article2.PreviousID = article.ID

		err = app.Articles.PostArticle(article2)
		if err != nil {
			t.Fatalf("PostArticle (v2) failed: %v", err)
		}

		if article2.ID != 2 {
			t.Fatalf("expected article2.ID = 2 after PostArticle, got %d", article2.ID)
		}

		// Verify latest is v2
		latest, err := app.Articles.GetArticle("revision-test")
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

	t.Run("chains multiple revisions without re-fetching", func(t *testing.T) {
		// This test verifies that PostArticle correctly updates article.ID,
		// allowing revision chaining without intermediate GetArticle calls.
		article := wiki.NewArticle("chain-test", "Version 1")
		article.Creator = user
		article.PreviousID = 0

		if err := app.Articles.PostArticle(article); err != nil {
			t.Fatalf("revision 1: %v", err)
		}
		if article.ID != 1 {
			t.Fatalf("after rev 1: expected ID=1, got %d", article.ID)
		}

		// Chain revisions using only the updated article.ID
		for i := 2; i <= 5; i++ {
			next := wiki.NewArticle("chain-test", "Version "+string(rune('0'+i)))
			next.Creator = user
			next.PreviousID = article.ID

			if err := app.Articles.PostArticle(next); err != nil {
				t.Fatalf("revision %d: %v", i, err)
			}
			if next.ID != i {
				t.Fatalf("after rev %d: expected ID=%d, got %d", i, i, next.ID)
			}

			article = next // carry forward for next iteration
		}

		// Verify final state
		latest, err := app.Articles.GetArticle("chain-test")
		if err != nil {
			t.Fatalf("GetArticle: %v", err)
		}
		if latest.ID != 5 {
			t.Errorf("expected latest revision ID=5, got %d", latest.ID)
		}

		history, err := app.Articles.GetRevisionHistory("chain-test")
		if err != nil {
			t.Fatalf("GetRevisionHistory: %v", err)
		}
		if len(history) != 5 {
			t.Errorf("expected 5 revisions in history, got %d", len(history))
		}
	})
}

func TestPostArticleNotModified(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, app.DB, "testuser", "test@example.com", "password123")

	// Create initial article
	article := wiki.NewArticle("not-modified-test", "Same content")
	article.Creator = user
	article.PreviousID = 0

	err := app.Articles.PostArticle(article)
	if err != nil {
		t.Fatalf("PostArticle failed: %v", err)
	}

	created, err := app.Articles.GetArticle("not-modified-test")
	if err != nil {
		t.Fatalf("GetArticle failed: %v", err)
	}

	// Try to post identical content
	article.PreviousID = created.ID

	err = app.Articles.PostArticle(article)
	if err != wiki.ErrArticleNotModified {
		t.Errorf("expected ErrArticleNotModified, got: %v", err)
	}
}

func TestGetRandomArticleURL(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	t.Run("empty database returns error", func(t *testing.T) {
		_, err := app.Articles.GetRandomArticleURL()
		if err != wiki.ErrNoArticles {
			t.Errorf("expected ErrNoArticles, got: %v", err)
		}
	})

	t.Run("returns valid article URL", func(t *testing.T) {
		user := testutil.CreateTestUser(t, app.DB, "testuser", "test@example.com", "password123")

		urls := []string{"article-a", "article-b", "article-c"}
		for _, url := range urls {
			testutil.CreateTestArticle(t, app, url, "Content", user)
		}

		url, err := app.Articles.GetRandomArticleURL()
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

func TestGetArticleByRevisionID(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, app.DB, "testuser", "test@example.com", "password123")

	// Create article with multiple revisions
	article := wiki.NewArticle("rev-test", "Version 1")
	article.Creator = user
	article.PreviousID = 0
	err := app.Articles.PostArticle(article)
	if err != nil {
		t.Fatalf("PostArticle failed: %v", err)
	}

	v1, _ := app.Articles.GetArticle("rev-test")

	article.Markdown = "Version 2"
	article.PreviousID = v1.ID
	err = app.Articles.PostArticle(article)
	if err != nil {
		t.Fatalf("PostArticle failed: %v", err)
	}

	t.Run("retrieves specific revision", func(t *testing.T) {
		rev1, err := app.Articles.GetArticleByRevisionID("rev-test", 1)
		if err != nil {
			t.Fatalf("GetArticleByRevisionID failed: %v", err)
		}
		if rev1.Markdown != "Version 1" {
			t.Errorf("expected 'Version 1', got %q", rev1.Markdown)
		}

		rev2, err := app.Articles.GetArticleByRevisionID("rev-test", 2)
		if err != nil {
			t.Fatalf("GetArticleByRevisionID failed: %v", err)
		}
		if rev2.Markdown != "Version 2" {
			t.Errorf("expected 'Version 2', got %q", rev2.Markdown)
		}
	})

	t.Run("non-existent revision", func(t *testing.T) {
		_, err := app.Articles.GetArticleByRevisionID("rev-test", 999)
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
	article := wiki.NewArticle("history-test", "Version 1")
	article.Creator = user
	article.PreviousID = 0
	article.Comment = "Initial"
	err := app.Articles.PostArticle(article)
	if err != nil {
		t.Fatalf("PostArticle failed: %v", err)
	}

	// PostArticle updates article.ID, so we can chain directly
	article2 := wiki.NewArticle("history-test", "Version 2 - Updated content")
	article2.Creator = user
	article2.PreviousID = article.ID
	article2.Comment = "Update"
	err = app.Articles.PostArticle(article2)
	if err != nil {
		t.Fatalf("PostArticle (v2) failed: %v", err)
	}

	history, err := app.Articles.GetRevisionHistory("history-test")
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
