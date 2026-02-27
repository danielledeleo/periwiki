package service_test

import (
	"strings"
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

func TestLazyRerendering(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, app.DB, "testuser", "test@example.com", "password123")

	// Create an article with two revisions.
	article := wiki.NewArticle("lazy-test", "Version 1")
	article.Creator = user
	article.PreviousID = 0
	if err := app.Articles.PostArticle(article); err != nil {
		t.Fatalf("PostArticle (v1) failed: %v", err)
	}

	article2 := wiki.NewArticle("lazy-test", "Version 2 content")
	article2.Creator = user
	article2.PreviousID = article.ID
	if err := app.Articles.PostArticle(article2); err != nil {
		t.Fatalf("PostArticle (v2) failed: %v", err)
	}

	// Sanity check: both revisions start with HTML.
	v1, err := app.Articles.GetArticleByRevisionID("lazy-test", 1)
	if err != nil {
		t.Fatalf("GetArticleByRevisionID(1) failed: %v", err)
	}
	if v1.HTML == "" {
		t.Fatal("v1 should have HTML before invalidation")
	}
	originalV1HTML := v1.HTML

	v2, err := app.Articles.GetArticle("lazy-test")
	if err != nil {
		t.Fatalf("GetArticle failed: %v", err)
	}
	if v2.HTML == "" {
		t.Fatal("v2 (head) should have HTML before invalidation")
	}
	originalV2HTML := v2.HTML

	t.Run("InvalidateNonHeadRevisionHTML nullifies only old revisions", func(t *testing.T) {
		affected, err := app.DB.InvalidateNonHeadRevisionHTML()
		if err != nil {
			t.Fatalf("InvalidateNonHeadRevisionHTML failed: %v", err)
		}
		if affected != 1 {
			t.Errorf("expected 1 affected row, got %d", affected)
		}

		// Head revision (v2) should still have HTML.
		var headHTML string
		err = app.DB.Conn().Get(&headHTML,
			`SELECT COALESCE(html, '') FROM Revision
			 WHERE id = 2 AND article_id = (SELECT id FROM Article WHERE url = 'lazy-test')`)
		if err != nil {
			t.Fatalf("querying head HTML: %v", err)
		}
		if headHTML == "" {
			t.Error("head revision HTML should not be nullified")
		}

		// Old revision (v1) should have NULL HTML.
		var oldHTML *string
		err = app.DB.Conn().Get(&oldHTML,
			`SELECT html FROM Revision
			 WHERE id = 1 AND article_id = (SELECT id FROM Article WHERE url = 'lazy-test')`)
		if err != nil {
			t.Fatalf("querying old HTML: %v", err)
		}
		if oldHTML != nil {
			t.Error("old revision HTML should be NULL after invalidation")
		}
	})

	t.Run("fetching invalidated revision triggers lazy re-render", func(t *testing.T) {
		v1Again, err := app.Articles.GetArticleByRevisionID("lazy-test", 1)
		if err != nil {
			t.Fatalf("GetArticleByRevisionID(1) after invalidation: %v", err)
		}
		if v1Again.HTML == "" {
			t.Fatal("lazy re-render should have produced HTML")
		}
		// The re-rendered HTML should be equivalent (same markdown, same templates).
		if v1Again.HTML != originalV1HTML {
			t.Errorf("lazy-rendered HTML differs from original:\n  got:  %q\n  want: %q", v1Again.HTML, originalV1HTML)
		}

		// Verify the HTML was persisted back to the database.
		var persistedHTML string
		err = app.DB.Conn().Get(&persistedHTML,
			`SELECT COALESCE(html, '') FROM Revision
			 WHERE id = 1 AND article_id = (SELECT id FROM Article WHERE url = 'lazy-test')`)
		if err != nil {
			t.Fatalf("querying persisted HTML: %v", err)
		}
		if persistedHTML == "" {
			t.Error("lazy-rendered HTML should be persisted to the database")
		}
	})

	t.Run("head revision is not affected by ensureHTML", func(t *testing.T) {
		head, err := app.Articles.GetArticle("lazy-test")
		if err != nil {
			t.Fatalf("GetArticle after invalidation: %v", err)
		}
		if head.HTML != originalV2HTML {
			t.Errorf("head HTML should be unchanged:\n  got:  %q\n  want: %q", head.HTML, originalV2HTML)
		}
	})
}

func TestLazyRerenderingMultipleArticles(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, app.DB, "testuser", "test@example.com", "password123")

	// Create two articles, each with two revisions.
	for _, url := range []string{"article-a", "article-b"} {
		a := wiki.NewArticle(url, "First version of "+url)
		a.Creator = user
		a.PreviousID = 0
		if err := app.Articles.PostArticle(a); err != nil {
			t.Fatalf("PostArticle(%s, v1): %v", url, err)
		}

		b := wiki.NewArticle(url, "Second version of "+url)
		b.Creator = user
		b.PreviousID = a.ID
		if err := app.Articles.PostArticle(b); err != nil {
			t.Fatalf("PostArticle(%s, v2): %v", url, err)
		}
	}

	affected, err := app.DB.InvalidateNonHeadRevisionHTML()
	if err != nil {
		t.Fatalf("InvalidateNonHeadRevisionHTML: %v", err)
	}
	if affected != 2 {
		t.Errorf("expected 2 affected rows (one per article), got %d", affected)
	}

	// Both old revisions should lazily re-render on access.
	for _, url := range []string{"article-a", "article-b"} {
		v1, err := app.Articles.GetArticleByRevisionID(url, 1)
		if err != nil {
			t.Errorf("GetArticleByRevisionID(%s, 1): %v", url, err)
			continue
		}
		if v1.HTML == "" {
			t.Errorf("%s: old revision should have been lazily re-rendered", url)
		}
	}
}

func TestPostArticle_PersistsLinks(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, app.DB, "testuser", "test@example.com", "password123")

	t.Run("extracts and persists wikilinks", func(t *testing.T) {
		article := wiki.NewArticle("Source_Page", "Links to [[Target_A]] and [[Target B]].")
		article.Creator = user
		article.PreviousID = 0

		if err := app.Articles.PostArticle(article); err != nil {
			t.Fatalf("PostArticle failed: %v", err)
		}

		// Verify links were persisted in the database
		count, err := app.DB.CountLinks()
		if err != nil {
			t.Fatalf("CountLinks failed: %v", err)
		}
		if count != 2 {
			t.Errorf("expected 2 links, got %d", count)
		}

		// Verify backlinks from Target_A's perspective
		backlinks, err := app.DB.SelectBacklinks("Target_A")
		if err != nil {
			t.Fatalf("SelectBacklinks failed: %v", err)
		}
		if len(backlinks) != 1 || backlinks[0].URL != "Source_Page" {
			t.Errorf("expected backlink from Source_Page, got %v", backlinks)
		}

		// Verify Target_B (space -> underscore normalization)
		backlinks, err = app.DB.SelectBacklinks("Target_B")
		if err != nil {
			t.Fatalf("SelectBacklinks failed: %v", err)
		}
		if len(backlinks) != 1 || backlinks[0].URL != "Source_Page" {
			t.Errorf("expected backlink from Source_Page to Target_B, got %v", backlinks)
		}
	})

	t.Run("updates links on article edit", func(t *testing.T) {
		// Edit Source_Page to change links
		edited := wiki.NewArticle("Source_Page", "Now links to [[New_Target]] only.")
		edited.Creator = user
		edited.PreviousID = 1

		if err := app.Articles.PostArticle(edited); err != nil {
			t.Fatalf("PostArticle (edit) failed: %v", err)
		}

		// Old targets should no longer have backlinks from Source_Page
		backlinks, err := app.DB.SelectBacklinks("Target_A")
		if err != nil {
			t.Fatalf("SelectBacklinks failed: %v", err)
		}
		if len(backlinks) != 0 {
			t.Errorf("expected no backlinks to Target_A after edit, got %v", backlinks)
		}

		// New target should have the backlink
		backlinks, err = app.DB.SelectBacklinks("New_Target")
		if err != nil {
			t.Fatalf("SelectBacklinks failed: %v", err)
		}
		if len(backlinks) != 1 || backlinks[0].URL != "Source_Page" {
			t.Errorf("expected backlink from Source_Page to New_Target, got %v", backlinks)
		}
	})
}

func TestGetBacklinks(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, app.DB, "testuser", "test@example.com", "password123")

	// Create several articles linking to the same target
	testutil.CreateTestArticle(t, app, "Linker_A", "See [[Common_Target]].", user)
	testutil.CreateTestArticle(t, app, "Linker_B", "Also links to [[Common_Target]].", user)
	testutil.CreateTestArticle(t, app, "Unrelated", "No links here.", user)

	t.Run("returns all backlinks", func(t *testing.T) {
		backlinks, err := app.Articles.GetBacklinks("Common_Target")
		if err != nil {
			t.Fatalf("GetBacklinks failed: %v", err)
		}
		if len(backlinks) != 2 {
			t.Fatalf("expected 2 backlinks, got %d", len(backlinks))
		}

		// Should be ordered by URL
		if backlinks[0].URL != "Linker_A" || backlinks[1].URL != "Linker_B" {
			t.Errorf("expected [Linker_A, Linker_B], got [%s, %s]", backlinks[0].URL, backlinks[1].URL)
		}
	})

	t.Run("returns empty for page with no backlinks", func(t *testing.T) {
		backlinks, err := app.Articles.GetBacklinks("Nonexistent_Target")
		if err != nil {
			t.Fatalf("GetBacklinks failed: %v", err)
		}
		if len(backlinks) != 0 {
			t.Errorf("expected no backlinks, got %d", len(backlinks))
		}
	})
}

func TestPostArticle_InvalidatesBacklinkers(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, app.DB, "testuser", "test@example.com", "password123")

	// Create article A linking to nonexistent B — should produce a deadlink.
	testutil.CreateTestArticle(t, app, "Article_A", "Links to [[Article_B]].", user)

	a, err := app.Articles.GetArticle("Article_A")
	if err != nil {
		t.Fatalf("GetArticle(A) failed: %v", err)
	}
	if !strings.Contains(a.HTML, "pw-deadlink") {
		t.Fatalf("expected pw-deadlink in HTML before target exists, got: %s", a.HTML)
	}

	// Create article B — this should trigger invalidateBacklinkers, re-rendering A.
	testutil.CreateTestArticle(t, app, "Article_B", "I exist now.", user)

	// Re-fetch A — its HTML should no longer have the deadlink class.
	a, err = app.Articles.GetArticle("Article_A")
	if err != nil {
		t.Fatalf("GetArticle(A) after B created failed: %v", err)
	}
	if strings.Contains(a.HTML, "pw-deadlink") {
		t.Errorf("expected pw-deadlink to be gone after target was created, got: %s", a.HTML)
	}
}

func TestPostArticle_SelfLink(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, app.DB, "testuser", "test@example.com", "password123")

	// Create article that links to itself. On creation (PreviousID=0),
	// invalidateBacklinkers will find this article in its own backlinks
	// and re-render it. This should not error or loop.
	testutil.CreateTestArticle(t, app, "Self_Link", "See [[Self Link]] for more.", user)

	backlinks, err := app.Articles.GetBacklinks("Self_Link")
	if err != nil {
		t.Fatalf("GetBacklinks failed: %v", err)
	}
	if len(backlinks) != 1 || backlinks[0].URL != "Self_Link" {
		t.Errorf("expected self-backlink, got %v", backlinks)
	}
}

func TestPostArticle_TalkPage(t *testing.T) {
	app, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, app.DB, "testuser", "test@example.com", "password123")

	t.Run("rejects talk page when subject does not exist", func(t *testing.T) {
		article := wiki.NewArticle("Talk:Nonexistent", "Discussion content")
		article.Creator = user
		article.PreviousID = 0

		err := app.Articles.PostArticle(article)
		if err != wiki.ErrSubjectPageNotFound {
			t.Errorf("expected ErrSubjectPageNotFound, got: %v", err)
		}
	})

	t.Run("accepts talk page when subject exists", func(t *testing.T) {
		// Create the subject article first
		testutil.CreateTestArticle(t, app, "Subject_Article", "Subject content", user)

		article := wiki.NewArticle("Talk:Subject_Article", "Discussion about subject")
		article.Creator = user
		article.PreviousID = 0

		err := app.Articles.PostArticle(article)
		if err != nil {
			t.Fatalf("PostArticle failed: %v", err)
		}

		// Verify the talk page was created
		created, err := app.Articles.GetArticle("Talk:Subject_Article")
		if err != nil {
			t.Fatalf("GetArticle failed: %v", err)
		}
		if created.URL != "Talk:Subject_Article" {
			t.Errorf("expected URL 'Talk:Subject_Article', got %q", created.URL)
		}
	})
}
