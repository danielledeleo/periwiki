package service_test

import (
	"testing"

	"github.com/danielledeleo/periwiki/internal/embedded"
	"github.com/danielledeleo/periwiki/testutil"
	"github.com/danielledeleo/periwiki/wiki"
	"github.com/danielledeleo/periwiki/wiki/service"
)

// mockRender is a simple render function for tests that avoids template loading
func mockRender(md string) (string, error) {
	return "<p>rendered: " + md[:min(20, len(md))] + "...</p>", nil
}

func TestEmbeddedArticleService_GetArticle(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	// Create embedded articles with mock renderer to avoid template loading issues
	ea, err := embedded.New(mockRender)
	if err != nil {
		t.Fatalf("failed to create embedded articles: %v", err)
	}

	// Wrap the article service
	wrappedService := service.NewEmbeddedArticleService(app.Articles, ea)

	t.Run("returns embedded article", func(t *testing.T) {
		article, err := wrappedService.GetArticle("Periwiki:Syntax")
		if err != nil {
			t.Fatalf("GetArticle failed: %v", err)
		}
		if !article.ReadOnly {
			t.Error("expected ReadOnly to be true")
		}
	})

	t.Run("falls through to base service", func(t *testing.T) {
		user := testutil.CreateTestUser(t, app.DB, "testuser", "test@example.com", "password123")
		testutil.CreateTestArticle(t, app, "Regular-Article", "# Hello", user)

		article, err := wrappedService.GetArticle("Regular-Article")
		if err != nil {
			t.Fatalf("GetArticle failed: %v", err)
		}
		if article.ReadOnly {
			t.Error("expected ReadOnly to be false for regular article")
		}
	})
}

func TestEmbeddedArticleService_PostArticle(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	ea, _ := embedded.New(mockRender)
	wrappedService := service.NewEmbeddedArticleService(app.Articles, ea)

	t.Run("rejects post to embedded URL", func(t *testing.T) {
		article := wiki.NewArticle("Periwiki:Syntax", "modified content")
		err := wrappedService.PostArticle(article)
		if err != wiki.ErrReadOnlyArticle {
			t.Errorf("expected ErrReadOnlyArticle, got: %v", err)
		}
	})
}
