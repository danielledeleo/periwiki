// Package testutil provides test utilities for periwiki integration tests.
package testutil

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
	"time"

	"github.com/danielledeleo/periwiki/extensions"
	"github.com/danielledeleo/periwiki/internal/storage"
	"github.com/danielledeleo/periwiki/render"
	"github.com/danielledeleo/periwiki/special"
	"github.com/danielledeleo/periwiki/templater"
	"github.com/danielledeleo/periwiki/wiki"
	"github.com/danielledeleo/periwiki/wiki/service"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/jmoiron/sqlx"
	"github.com/michaeljs1990/sqlitestore"
	"github.com/microcosm-cc/bluemonday"
	_ "modernc.org/sqlite"
)

// TestDB wraps the database for testing.
type TestDB struct {
	*sqlitestore.SqliteStore
	*storage.PreparedStatements
	conn *sqlx.DB
}

// TestApp wraps the full application for integration tests.
type TestApp struct {
	*templater.Templater
	Articles     service.ArticleService
	Users        service.UserService
	Sessions     service.SessionService
	Rendering    service.RenderingService
	Preferences  service.PreferenceService
	SpecialPages *special.Registry
	Config       *wiki.Config
	Router       *mux.Router
	DB           *TestDB
}

// projectRoot returns the root directory of the project.
func projectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(filename))
}

// SetupTestDB creates an in-memory SQLite database with the schema loaded.
func SetupTestDB(t *testing.T) (*TestDB, func()) {
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

	testDB := &TestDB{conn: conn}

	// Initialize session store with test secret
	testSecret := []byte("test-secret-key-for-sessions-32b")
	testDB.SqliteStore, err = sqlitestore.NewSqliteStoreFromConnection(conn, "sessions", "/", 86400, testSecret)
	if err != nil {
		conn.Close()
		t.Fatalf("failed to create session store: %v", err)
	}

	// Initialize prepared statements using shared function from db package
	testDB.PreparedStatements, err = storage.InitializeStatements(conn)
	if err != nil {
		conn.Close()
		t.Fatalf("failed to initialize prepared statements: %v", err)
	}

	cleanup := func() {
		conn.Close()
	}

	return testDB, cleanup
}

// SetupTestApp creates a full application instance for integration tests.
func SetupTestApp(t *testing.T) (*TestApp, func()) {
	t.Helper()

	db, dbCleanup := SetupTestDB(t)

	config := &wiki.Config{
		CookieSecret:              []byte("test-secret-key-for-sessions-32b"),
		CookieExpiry:              86400,
		DatabaseFile:              ":memory:",
		MinimumPasswordLength:     8,
		Host:                      "localhost:8080",
		AllowAnonymousEditsGlobal: true,
	}

	// Create sanitizer matching production config
	bm := bluemonday.UGCPolicy()
	bm.AllowAttrs("class").Globally()
	bm.AllowAttrs("data-line-number").Matching(regexp.MustCompile("^[0-9]+$")).OnElements("a")
	bm.AllowAttrs("style").OnElements("ins", "del")
	bm.AllowAttrs("style").Matching(regexp.MustCompile(`^text-align:\s+(left|right|center);$`)).OnElements("td", "th")

	// Create templater and load templates
	tmpl := templater.New()
	templatesPath := filepath.Join(projectRoot(), "templates")
	err := tmpl.Load(
		filepath.Join(templatesPath, "layouts", "*.html"),
		filepath.Join(templatesPath, "*.html"),
	)
	if err != nil {
		dbCleanup()
		t.Fatalf("failed to load templates: %v", err)
	}

	// Load footnote templates
	footnoteTemplates, err := tmpl.LoadExtensionTemplates(templatesPath, "footnote", []string{
		"link", "backlink", "list", "item",
	})
	if err != nil {
		dbCleanup()
		t.Fatalf("failed to load footnote templates: %v", err)
	}

	// Load wikilink templates
	wikiLinkTemplates, err := tmpl.LoadExtensionTemplates(templatesPath, "wikilink", []string{
		"link",
	})
	if err != nil {
		dbCleanup()
		t.Fatalf("failed to load wikilink templates: %v", err)
	}

	// Create existence checker for wiki links
	existenceChecker := func(url string) bool {
		const prefix = "/wiki/"
		if len(url) > len(prefix) {
			url = url[len(prefix):]
		}
		article, _ := db.SelectArticle(url)
		return article != nil
	}

	// Create renderer with extension templates
	renderer := render.NewHTMLRenderer(
		existenceChecker,
		[]extensions.WikiLinkRendererOption{extensions.WithWikiLinkTemplates(wikiLinkTemplates)},
		[]extensions.FootnoteOption{extensions.WithFootnoteTemplates(footnoteTemplates)},
	)

	// Create rendering service
	renderingService := service.NewRenderingService(renderer, bm)

	// Create session service
	sessionService := service.NewSessionService(db)

	// Create user service
	userService := service.NewUserService(db, config.MinimumPasswordLength)

	// Create article service (nil queue for synchronous rendering in tests)
	articleService := service.NewArticleService(db, renderingService, nil)

	// Create preference service
	preferenceService := service.NewPreferenceService(db)

	specialPages := special.NewRegistry()
	specialPages.Register("Random", special.NewRandomPage(articleService))

	app := &TestApp{
		Templater:    tmpl,
		Articles:     articleService,
		Users:        userService,
		Sessions:     sessionService,
		Rendering:    renderingService,
		Preferences:  preferenceService,
		SpecialPages: specialPages,
		Config:       config,
		DB:           db,
	}

	cleanup := func() {
		dbCleanup()
	}

	return app, cleanup
}

// CreateTestUser creates a user in the test database and returns it.
func CreateTestUser(t *testing.T, db *TestDB, screenname, email, password string) *wiki.User {
	t.Helper()

	user := &wiki.User{
		ScreenName:  screenname,
		Email:       email,
		RawPassword: password,
	}

	err := user.SetPasswordHash()
	if err != nil {
		t.Fatalf("failed to set password hash: %v", err)
	}

	err = db.InsertUser(user)
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	// Fetch the user to get the assigned ID
	createdUser, err := db.SelectUserByScreenname(screenname, false)
	if err != nil {
		t.Fatalf("failed to fetch created user: %v", err)
	}

	return createdUser
}

// CreateTestArticle creates an article in the test database and returns it.
func CreateTestArticle(t *testing.T, app *TestApp, url, title, markdown string, creator *wiki.User) *wiki.Article {
	t.Helper()

	article := wiki.NewArticle(url, title, markdown)
	article.Creator = creator
	article.PreviousID = 0

	err := app.Articles.PostArticle(article)
	if err != nil {
		t.Fatalf("failed to create test article: %v", err)
	}

	// Fetch the article to get full data
	created, err := app.Articles.GetArticle(url)
	if err != nil {
		t.Fatalf("failed to fetch created article: %v", err)
	}

	return created
}

// Implement wiki.db interface for TestDB

// articleResult is used for scanning article queries that include user info
type articleResult struct {
	URL        string
	ID         int
	Title      string
	Markdown   string
	HTML       string
	Hash       string    `db:"hashval"`
	Created    time.Time
	PreviousID int       `db:"previous_id"`
	Comment    string
	UserID     int       `db:"user_id"`
	ScreenName string    `db:"screenname"`
}

func (r *articleResult) toArticle() *wiki.Article {
	return &wiki.Article{
		URL: r.URL,
		Revision: &wiki.Revision{
			ID:         r.ID,
			Title:      r.Title,
			Markdown:   r.Markdown,
			HTML:       r.HTML,
			Hash:       r.Hash,
			Created:    r.Created,
			PreviousID: r.PreviousID,
			Comment:    r.Comment,
			Creator:    &wiki.User{ID: r.UserID, ScreenName: r.ScreenName},
		},
	}
}

func (tdb *TestDB) SelectArticle(url string) (*wiki.Article, error) {
	result := &articleResult{}
	err := tdb.SelectArticleByLatestRevisionStmt.Get(result, url)
	if err != nil {
		return nil, err
	}
	return result.toArticle(), nil
}

func (tdb *TestDB) SelectArticleByRevisionHash(url string, hash string) (*wiki.Article, error) {
	result := &articleResult{}
	err := tdb.SelectArticleByRevisionHashStmt.Get(result, url, hash)
	if err != nil {
		return nil, err
	}
	return result.toArticle(), nil
}

func (tdb *TestDB) SelectArticleByRevisionID(url string, id int) (*wiki.Article, error) {
	result := &articleResult{}
	err := tdb.SelectArticleByRevisionIDStmt.Get(result, url, id)
	if err != nil {
		return nil, err
	}
	return result.toArticle(), nil
}

func (tdb *TestDB) SelectRevision(hash string) (*wiki.Revision, error) {
	r := &wiki.Revision{}
	err := tdb.conn.Get(r, "SELECT id, title, markdown, html, hashval, created, previous_id FROM Revision WHERE hashval = ?", hash)
	return r, err
}

func (tdb *TestDB) SelectRevisionHistory(url string) ([]*wiki.Revision, error) {
	rows, err := tdb.conn.Queryx(
		`SELECT Revision.id, title, hashval, created, comment, previous_id, User.screenname, length(markdown)
			FROM Article JOIN Revision ON Article.id = Revision.article_id
					     JOIN User ON Revision.user_id = User.id
			WHERE Article.url = ? ORDER BY created DESC`, url)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := struct {
		Title, Hashval, Comment, Screenname string
		ID                                  int
		PreviousID                          int       `db:"previous_id"`
		Length                              int       `db:"length(markdown)"`
		Created                             time.Time `db:"created"`
	}{}
	results := make([]*wiki.Revision, 0)
	for rows.Next() {
		rev := &wiki.Revision{Creator: &wiki.User{}}
		err := rows.StructScan(&result)
		if err != nil {
			return nil, err
		}
		rev.Title = result.Title
		rev.Hash = result.Hashval
		rev.ID = result.ID
		rev.PreviousID = result.PreviousID
		rev.Comment = result.Comment
		rev.Created = result.Created
		rev.Markdown = string(rune(result.Length))
		rev.Creator.ScreenName = result.Screenname
		results = append(results, rev)
	}
	if len(results) < 1 {
		return nil, wiki.ErrGenericNotFound
	}
	return results, nil
}

func (tdb *TestDB) SelectRandomArticleURL() (string, error) {
	var url string
	err := tdb.conn.Get(&url, `SELECT url FROM Article ORDER BY ABS(RANDOM()) LIMIT 1`)
	return url, err
}

func (tdb *TestDB) SelectAllArticles() ([]*wiki.ArticleSummary, error) {
	rows, err := tdb.conn.Queryx(`
		SELECT a.url, r.title, MAX(r.created) as last_modified
		FROM Article a
		JOIN Revision r ON a.id = r.article_id
		GROUP BY a.id
		ORDER BY r.title ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []*wiki.ArticleSummary
	for rows.Next() {
		var url, title, lastModStr string
		if err := rows.Scan(&url, &title, &lastModStr); err != nil {
			return nil, err
		}
		lastMod, err := time.Parse("2006-01-02 15:04:05.000", lastModStr)
		if err != nil {
			return nil, err
		}
		articles = append(articles, &wiki.ArticleSummary{
			URL:          url,
			Title:        title,
			LastModified: lastMod,
		})
	}
	return articles, rows.Err()
}

func (tdb *TestDB) SelectUserByScreenname(screenname string, withHash bool) (*wiki.User, error) {
	user := &wiki.User{}
	var err error
	if withHash {
		err = tdb.SelectUserScreennameWithHashStmt.Get(user, screenname)
	} else {
		err = tdb.SelectUserScreennameStmt.Get(user, screenname)
	}
	return user, err
}

func (tdb *TestDB) InsertArticle(article *wiki.Article) error {
	testArticle, insertErr := tdb.SelectArticle(article.URL)

	tx, err := tdb.conn.Beginx()
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	if insertErr != nil && insertErr.Error() == "sql: no rows in result set" {
		// New article
		if _, err = tx.Exec(`INSERT INTO Article (url) VALUES (?);`, article.URL); err != nil {
			return err
		}

		_, err = tx.Exec(`INSERT INTO Revision (id, title, hashval, markdown, html, article_id, user_id, created, previous_id, comment)
			VALUES (?, ?, ?, ?, ?, last_insert_rowid(), ?, strftime("%Y-%m-%d %H:%M:%f", "now"), ?, ?)`,
			article.PreviousID+1,
			article.Title,
			article.Hash,
			article.Markdown,
			article.HTML,
			article.Creator.ID,
			article.PreviousID,
			article.Comment)

		return err
	} else if insertErr == nil && testArticle != nil {
		// New revision to article
		_, err = tx.Exec(`INSERT INTO Revision (id, title, hashval, markdown, html, article_id, user_id, created, previous_id, comment)
			VALUES (?, ?, ?, ?, ?, (SELECT Article.id FROM Article WHERE url = ?), ?, strftime("%Y-%m-%d %H:%M:%f", "now"), ?, ?)`,
			article.PreviousID+1,
			article.Title,
			article.Hash,
			article.Markdown,
			article.HTML,
			article.URL,
			article.Creator.ID,
			article.PreviousID,
			article.Comment)

		if err != nil && err.Error() == "UNIQUE constraint failed: Revision.id, Revision.article_id" {
			return wiki.ErrRevisionAlreadyExists
		}
		return err
	}

	return insertErr
}

func (tdb *TestDB) InsertArticleQueued(article *wiki.Article) (revisionID int64, err error) {
	testArticle, insertErr := tdb.SelectArticle(article.URL)

	tx, err := tdb.conn.Beginx()
	if err != nil {
		return 0, err
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	newRevisionID := int64(article.PreviousID + 1)

	if insertErr != nil && insertErr.Error() == "sql: no rows in result set" {
		// New article
		if _, err = tx.Exec(`INSERT INTO Article (url) VALUES (?);`, article.URL); err != nil {
			return 0, err
		}

		_, err = tx.Exec(`INSERT INTO Revision (id, title, hashval, markdown, html, article_id, user_id, created, previous_id, comment, render_status)
			VALUES (?, ?, ?, ?, '', last_insert_rowid(), ?, strftime("%Y-%m-%d %H:%M:%f", "now"), ?, ?, 'queued')`,
			newRevisionID,
			article.Title,
			article.Hash,
			article.Markdown,
			article.Creator.ID,
			article.PreviousID,
			article.Comment)

		if err != nil {
			return 0, err
		}
	} else if insertErr == nil && testArticle != nil {
		// New revision to article
		_, err = tx.Exec(`INSERT INTO Revision (id, title, hashval, markdown, html, article_id, user_id, created, previous_id, comment, render_status)
			VALUES (?, ?, ?, ?, '', (SELECT Article.id FROM Article WHERE url = ?), ?, strftime("%Y-%m-%d %H:%M:%f", "now"), ?, ?, 'queued')`,
			newRevisionID,
			article.Title,
			article.Hash,
			article.Markdown,
			article.URL,
			article.Creator.ID,
			article.PreviousID,
			article.Comment)

		if err != nil && err.Error() == "UNIQUE constraint failed: Revision.id, Revision.article_id" {
			return 0, wiki.ErrRevisionAlreadyExists
		}
		if err != nil {
			return 0, err
		}
	} else {
		return 0, insertErr
	}

	return newRevisionID, nil
}

func (tdb *TestDB) UpdateRevisionHTML(url string, revisionID int, html string, renderStatus string) error {
	result, err := tdb.conn.Exec(`
		UPDATE Revision
		SET html = ?, render_status = ?
		WHERE id = ? AND article_id = (SELECT id FROM Article WHERE url = ?)`,
		html, renderStatus, revisionID, url)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return wiki.ErrRevisionNotFound
	}

	return nil
}

func (tdb *TestDB) InsertUser(user *wiki.User) error {
	tx, err := tdb.conn.Beginx()
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	_, err = tx.Exec(`INSERT INTO User(screenname, email) VALUES (?, ?)`, user.ScreenName, user.Email)
	if err != nil {
		if err.Error() == "UNIQUE constraint failed: User.screenname" {
			return wiki.ErrUsernameTaken
		} else if err.Error() == "UNIQUE constraint failed: User.email" {
			return wiki.ErrEmailTaken
		}
		return err
	}

	_, err = tx.Exec(`INSERT INTO Password(user_id, passwordhash) VALUES (last_insert_rowid(), ?)`, user.PasswordHash)
	return err
}

func (tdb *TestDB) InsertPreference(pref *wiki.Preference) error {
	_, err := tdb.conn.Exec(`INSERT OR REPLACE INTO Preference (pref_label, pref_type, help_text, pref_int, pref_text, pref_selection)
		VALUES(?, ?, ?, ?, ?, ?)`,
		pref.Label,
		pref.Type,
		pref.HelpText,
		pref.IntValue,
		pref.TextValue,
		pref.SelectionValue,
	)
	return err
}

func (tdb *TestDB) SelectPreference(key string) (*wiki.Preference, error) {
	pref := &wiki.Preference{}
	err := tdb.conn.Get(pref, `SELECT * FROM Preference WHERE pref_label = ?`, key)
	if err != nil {
		return nil, err
	}
	return pref, nil
}

// Get implements sessions.Store
func (tdb *TestDB) Get(r *http.Request, name string) (*sessions.Session, error) {
	return tdb.SqliteStore.Get(r, name)
}

// New implements sessions.Store
func (tdb *TestDB) New(r *http.Request, name string) (*sessions.Session, error) {
	return tdb.SqliteStore.New(r, name)
}

// Save implements sessions.Store
func (tdb *TestDB) Save(r *http.Request, w http.ResponseWriter, s *sessions.Session) error {
	return tdb.SqliteStore.Save(r, w, s)
}

// Delete implements the delete method for session store
func (tdb *TestDB) Delete(r *http.Request, w http.ResponseWriter, s *sessions.Session) error {
	return tdb.SqliteStore.Delete(r, w, s)
}

// RequestWithUser creates a request with a user context attached.
func RequestWithUser(r *http.Request, user *wiki.User) *http.Request {
	ctx := context.WithValue(r.Context(), wiki.UserKey, user)
	return r.WithContext(ctx)
}

// AnonymousUser returns an anonymous user for testing.
func AnonymousUser() *wiki.User {
	return &wiki.User{
		ID:         0,
		ScreenName: "Anonymous",
		IPAddress:  "127.0.0.1",
	}
}

// MakeTestRequest creates a test request with optional user context.
func MakeTestRequest(method, url string, user *wiki.User) *http.Request {
	req := httptest.NewRequest(method, url, nil)
	if user != nil {
		req = RequestWithUser(req, user)
	} else {
		req = RequestWithUser(req, AnonymousUser())
	}
	return req
}
