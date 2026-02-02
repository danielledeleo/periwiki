package storage

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/danielledeleo/periwiki/wiki"
	"github.com/jmoiron/sqlx"
	"github.com/michaeljs1990/sqlitestore"
	_ "modernc.org/sqlite"
)

// projectRoot returns the root directory of the project.
func projectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	// Go up two levels: internal/storage -> internal -> project root
	return filepath.Dir(filepath.Dir(filepath.Dir(filename)))
}

// setupTestDB creates an in-memory SQLite database for testing.
func setupTestDB(t *testing.T) (*sqliteDb, func()) {
	t.Helper()

	conn, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory database: %v", err)
	}

	// Read schema from project root
	schemaPath := filepath.Join(projectRoot(), "internal", "storage", "schema.sql")
	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		conn.Close()
		t.Fatalf("failed to read schema.sql: %v", err)
	}

	_, err = conn.Exec(string(schema))
	if err != nil {
		conn.Close()
		t.Fatalf("failed to execute schema: %v", err)
	}

	db := &sqliteDb{conn: conn}

	// Initialize session store with test secret
	testSecret := []byte("test-secret-key-for-sessions-32b")
	db.SqliteStore, err = sqlitestore.NewSqliteStoreFromConnection(conn, "sessions", "/", 86400, testSecret)
	if err != nil {
		conn.Close()
		t.Fatalf("failed to create session store: %v", err)
	}

	// Initialize prepared statements using shared function
	db.PreparedStatements, err = InitializeStatements(conn)
	if err != nil {
		conn.Close()
		t.Fatalf("failed to initialize prepared statements: %v", err)
	}

	cleanup := func() {
		conn.Close()
	}

	return db, cleanup
}

// createTestUser creates a user directly in the database for testing.
func createTestUser(t *testing.T, db *sqliteDb, screenname, email, passwordHash string) int64 {
	t.Helper()

	result, err := db.conn.Exec(`INSERT INTO User(screenname, email) VALUES (?, ?)`, screenname, email)
	if err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}

	userID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("failed to get user ID: %v", err)
	}

	_, err = db.conn.Exec(`INSERT INTO Password(user_id, passwordhash) VALUES (?, ?)`, userID, passwordHash)
	if err != nil {
		t.Fatalf("failed to insert password: %v", err)
	}

	return userID
}

func TestInsertAndSelectArticle(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create a test user first
	userID := createTestUser(t, db, "testuser", "test@example.com", "hashedpassword")

	article := &wiki.Article{
		URL: "test-article",
		Revision: &wiki.Revision{
			Markdown:   "# Hello World\n\nThis is a test.",
			HTML:       "<h1>Hello World</h1><p>This is a test.</p>",
			Hash:       "testhash123",
			Creator:    &wiki.User{ID: int(userID)},
			PreviousID: 0,
			Comment:    "Initial creation",
		},
	}

	// Insert the article
	err := db.InsertArticle(article)
	if err != nil {
		t.Fatalf("InsertArticle failed: %v", err)
	}

	// Select the article
	retrieved, err := db.SelectArticle("test-article")
	if err != nil {
		t.Fatalf("SelectArticle failed: %v", err)
	}

	// Verify the data
	if retrieved.URL != article.URL {
		t.Errorf("expected URL %q, got %q", article.URL, retrieved.URL)
	}
	if retrieved.Markdown != article.Markdown {
		t.Errorf("expected Markdown %q, got %q", article.Markdown, retrieved.Markdown)
	}
	if retrieved.HTML != article.HTML {
		t.Errorf("expected HTML %q, got %q", article.HTML, retrieved.HTML)
	}
	if retrieved.Hash != article.Hash {
		t.Errorf("expected Hash %q, got %q", article.Hash, retrieved.Hash)
	}
	if retrieved.ID != 1 {
		t.Errorf("expected revision ID 1, got %d", retrieved.ID)
	}
}

func TestInsertRevision(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	userID := createTestUser(t, db, "testuser", "test@example.com", "hashedpassword")

	// Create initial article
	article := &wiki.Article{
		URL: "test-article",
		Revision: &wiki.Revision{
			Markdown:   "Version 1",
			HTML:       "<p>Version 1</p>",
			Hash:       "hash1",
			Creator:    &wiki.User{ID: int(userID)},
			PreviousID: 0,
		},
	}

	err := db.InsertArticle(article)
	if err != nil {
		t.Fatalf("InsertArticle (v1) failed: %v", err)
	}

	// Small delay to ensure unique timestamps
	time.Sleep(10 * time.Millisecond)

	// Create second revision
	article.Markdown = "Version 2"
	article.HTML = "<p>Version 2</p>"
	article.Hash = "hash2"
	article.PreviousID = 1

	err = db.InsertArticle(article)
	if err != nil {
		t.Fatalf("InsertArticle (v2) failed: %v", err)
	}

	// Small delay to ensure unique timestamps
	time.Sleep(10 * time.Millisecond)

	// Create third revision
	article.Markdown = "Version 3"
	article.HTML = "<p>Version 3</p>"
	article.Hash = "hash3"
	article.PreviousID = 2

	err = db.InsertArticle(article)
	if err != nil {
		t.Fatalf("InsertArticle (v3) failed: %v", err)
	}

	// Verify latest revision is returned
	latest, err := db.SelectArticle("test-article")
	if err != nil {
		t.Fatalf("SelectArticle failed: %v", err)
	}

	if latest.Markdown != "Version 3" {
		t.Errorf("expected latest markdown 'Version 3', got %q", latest.Markdown)
	}
	if latest.ID != 3 {
		t.Errorf("expected latest revision ID 3, got %d", latest.ID)
	}

	// Verify we can retrieve specific revision by ID
	v1, err := db.SelectArticleByRevisionID("test-article", 1)
	if err != nil {
		t.Fatalf("SelectArticleByRevisionID failed: %v", err)
	}
	if v1.Markdown != "Version 1" {
		t.Errorf("expected v1 markdown 'Version 1', got %q", v1.Markdown)
	}

	// Verify we can retrieve specific revision by hash
	v2, err := db.SelectArticleByRevisionHash("test-article", "hash2")
	if err != nil {
		t.Fatalf("SelectArticleByRevisionHash failed: %v", err)
	}
	if v2.Markdown != "Version 2" {
		t.Errorf("expected v2 markdown 'Version 2', got %q", v2.Markdown)
	}
}

func TestSelectRandomArticleURL(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Test empty database
	_, err := db.SelectRandomArticleURL()
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows for empty database, got: %v", err)
	}

	userID := createTestUser(t, db, "testuser", "test@example.com", "hashedpassword")

	// Add some articles
	urls := []string{"article-one", "article-two", "article-three"}
	for i, url := range urls {
		article := &wiki.Article{
			URL: url,
			Revision: &wiki.Revision{
				Markdown:   "Content",
				HTML:       "<p>Content</p>",
				Hash:       "hash" + url,
				Creator:    &wiki.User{ID: int(userID)},
				PreviousID: i,
			},
		}
		err := db.InsertArticle(article)
		if err != nil {
			t.Fatalf("InsertArticle failed: %v", err)
		}
	}

	// Test random selection returns valid URL
	for i := 0; i < 10; i++ {
		url, err := db.SelectRandomArticleURL()
		if err != nil {
			t.Fatalf("SelectRandomArticleURL failed: %v", err)
		}

		found := false
		for _, valid := range urls {
			if url == valid {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("SelectRandomArticleURL returned invalid URL: %q", url)
		}
	}
}

func TestSelectAllArticles(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Test empty database
	articles, err := db.SelectAllArticles()
	if err != nil {
		t.Fatalf("SelectAllArticles failed on empty database: %v", err)
	}
	if len(articles) != 0 {
		t.Errorf("expected 0 articles for empty database, got %d", len(articles))
	}

	userID := createTestUser(t, db, "testuser", "test@example.com", "hashedpassword")

	// Add some articles
	articlesToAdd := []string{"zebra", "apple", "banana"}

	for _, url := range articlesToAdd {
		article := &wiki.Article{
			URL: url,
			Revision: &wiki.Revision{
				Markdown:   "Content for " + url,
				HTML:       "<p>Content for " + url + "</p>",
				Hash:       "hash-" + url,
				Creator:    &wiki.User{ID: int(userID)},
				PreviousID: 0,
			},
		}
		err := db.InsertArticle(article)
		if err != nil {
			t.Fatalf("InsertArticle failed: %v", err)
		}
		time.Sleep(10 * time.Millisecond) // Ensure unique timestamps
	}

	// Get all articles
	articles, err = db.SelectAllArticles()
	if err != nil {
		t.Fatalf("SelectAllArticles failed: %v", err)
	}

	if len(articles) != 3 {
		t.Fatalf("expected 3 articles, got %d", len(articles))
	}

	// Verify ordering (by URL ASC)
	if articles[0].URL != "apple" {
		t.Errorf("expected first article URL 'apple', got %q", articles[0].URL)
	}
	if articles[1].URL != "banana" {
		t.Errorf("expected second article URL 'banana', got %q", articles[1].URL)
	}
	if articles[2].URL != "zebra" {
		t.Errorf("expected third article URL 'zebra', got %q", articles[2].URL)
	}

	// Verify URLs match
	if articles[0].URL != "apple" {
		t.Errorf("expected URL 'apple', got %q", articles[0].URL)
	}

	// Verify LastModified is set and recent
	for _, a := range articles {
		if a.LastModified.IsZero() {
			t.Errorf("expected non-zero LastModified for article %q", a.URL)
		}
		// Should be within the last minute
		if time.Since(a.LastModified) > time.Minute {
			t.Errorf("LastModified too old for article %q: %v", a.URL, a.LastModified)
		}
	}
}

func TestSelectRevisionHistory(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	userID := createTestUser(t, db, "testuser", "test@example.com", "hashedpassword")

	// Create article with multiple revisions
	article := &wiki.Article{
		URL: "test-article",
		Revision: &wiki.Revision{
			Markdown:   "Version 1",
			HTML:       "<p>Version 1</p>",
			Hash:       "hash1",
			Creator:    &wiki.User{ID: int(userID)},
			PreviousID: 0,
			Comment:    "Initial version",
		},
	}

	err := db.InsertArticle(article)
	if err != nil {
		t.Fatalf("InsertArticle failed: %v", err)
	}

	// Add a small delay to ensure different timestamps
	time.Sleep(10 * time.Millisecond)

	article.Markdown = "Version 2"
	article.HTML = "<p>Version 2</p>"
	article.Hash = "hash2"
	article.PreviousID = 1
	article.Comment = "Second version"
	err = db.InsertArticle(article)
	if err != nil {
		t.Fatalf("InsertArticle failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	article.Markdown = "Version 3"
	article.HTML = "<p>Version 3</p>"
	article.Hash = "hash3"
	article.PreviousID = 2
	article.Comment = "Third version"
	err = db.InsertArticle(article)
	if err != nil {
		t.Fatalf("InsertArticle failed: %v", err)
	}

	// Get revision history
	history, err := db.SelectRevisionHistory("test-article")
	if err != nil {
		t.Fatalf("SelectRevisionHistory failed: %v", err)
	}

	if len(history) != 3 {
		t.Fatalf("expected 3 revisions, got %d", len(history))
	}

	// Verify ordering (newest first)
	if history[0].Hash != "hash3" {
		t.Errorf("expected newest revision first (hash3), got %q", history[0].Hash)
	}
	if history[2].Hash != "hash1" {
		t.Errorf("expected oldest revision last (hash1), got %q", history[2].Hash)
	}

	// Verify comments
	if history[0].Comment != "Third version" {
		t.Errorf("expected comment 'Third version', got %q", history[0].Comment)
	}

	// Test non-existent article
	_, err = db.SelectRevisionHistory("non-existent")
	if err != wiki.ErrGenericNotFound {
		t.Errorf("expected ErrGenericNotFound for non-existent article, got: %v", err)
	}
}

func TestInsertUser(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	user := &wiki.User{
		ScreenName:   "newuser",
		Email:        "newuser@example.com",
		PasswordHash: "$2a$04$hashedpassword",
	}

	err := db.InsertUser(user)
	if err != nil {
		t.Fatalf("InsertUser failed: %v", err)
	}

	// Verify user was created
	retrieved, err := db.SelectUserByScreenname("newuser", false)
	if err != nil {
		t.Fatalf("SelectUserByScreenname failed: %v", err)
	}

	if retrieved.ScreenName != user.ScreenName {
		t.Errorf("expected screenname %q, got %q", user.ScreenName, retrieved.ScreenName)
	}
	if retrieved.Email != user.Email {
		t.Errorf("expected email %q, got %q", user.Email, retrieved.Email)
	}
	if retrieved.ID == 0 {
		t.Error("expected non-zero user ID")
	}
}

func TestUniqueConstraints(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create first user
	user1 := &wiki.User{
		ScreenName:   "uniqueuser",
		Email:        "unique@example.com",
		PasswordHash: "$2a$04$hash1",
	}
	err := db.InsertUser(user1)
	if err != nil {
		t.Fatalf("InsertUser (first) failed: %v", err)
	}

	t.Run("duplicate screenname", func(t *testing.T) {
		user2 := &wiki.User{
			ScreenName:   "uniqueuser", // Same screenname
			Email:        "different@example.com",
			PasswordHash: "$2a$04$hash2",
		}
		err := db.InsertUser(user2)
		if err != wiki.ErrUsernameTaken {
			t.Errorf("expected ErrUsernameTaken, got: %v", err)
		}
	})

	t.Run("duplicate email", func(t *testing.T) {
		user3 := &wiki.User{
			ScreenName:   "differentuser",
			Email:        "unique@example.com", // Same email
			PasswordHash: "$2a$04$hash3",
		}
		err := db.InsertUser(user3)
		if err != wiki.ErrEmailTaken {
			t.Errorf("expected ErrEmailTaken, got: %v", err)
		}
	})
}

func TestSelectUserByScreenname(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	passwordHash := "$2a$04$testhash"
	userID := createTestUser(t, db, "testuser", "test@example.com", passwordHash)

	t.Run("without password hash", func(t *testing.T) {
		user, err := db.SelectUserByScreenname("testuser", false)
		if err != nil {
			t.Fatalf("SelectUserByScreenname failed: %v", err)
		}

		if user.ScreenName != "testuser" {
			t.Errorf("expected screenname 'testuser', got %q", user.ScreenName)
		}
		if user.ID != int(userID) {
			t.Errorf("expected user ID %d, got %d", userID, user.ID)
		}
		// Password hash should not be returned
		if user.PasswordHash != "" {
			t.Error("expected empty password hash when withHash=false")
		}
	})

	t.Run("with password hash", func(t *testing.T) {
		user, err := db.SelectUserByScreenname("testuser", true)
		if err != nil {
			t.Fatalf("SelectUserByScreenname failed: %v", err)
		}

		if user.PasswordHash != passwordHash {
			t.Errorf("expected password hash %q, got %q", passwordHash, user.PasswordHash)
		}
	})

	t.Run("non-existent user", func(t *testing.T) {
		_, err := db.SelectUserByScreenname("nonexistent", false)
		if err != sql.ErrNoRows {
			t.Errorf("expected sql.ErrNoRows, got: %v", err)
		}
	})
}

func TestArticleNotFound(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, err := db.SelectArticle("nonexistent")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got: %v", err)
	}
}

func TestRevisionAlreadyExists(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	userID := createTestUser(t, db, "testuser", "test@example.com", "hashedpassword")

	article := &wiki.Article{
		URL: "test-article",
		Revision: &wiki.Revision{
			Markdown:   "Version 1",
			HTML:       "<p>Version 1</p>",
			Hash:       "hash1",
			Creator:    &wiki.User{ID: int(userID)},
			PreviousID: 0,
		},
	}

	err := db.InsertArticle(article)
	if err != nil {
		t.Fatalf("InsertArticle (first) failed: %v", err)
	}

	// Try to insert with same previous_id (should create revision 1 again)
	article.Markdown = "Different content"
	article.Hash = "different-hash"
	// PreviousID is still 0, so it will try to create revision 1 again

	err = db.InsertArticle(article)
	if err != wiki.ErrRevisionAlreadyExists {
		t.Errorf("expected ErrRevisionAlreadyExists, got: %v", err)
	}
}
