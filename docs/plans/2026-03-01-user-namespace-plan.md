# User:, User_talk:, and Special:Contributions — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add User: and User_talk: namespaces, a Special:Contributions page, and link usernames throughout the site to user pages with red-link support.

**Architecture:** User: and User_talk: pages are stored as regular articles. User: page view uses a custom handler (profile stats + optional wiki content). User_talk: dispatches through the existing article pipeline. Username links in history/diff templates use a `HasUserPage` bool on the User struct populated via EXISTS subquery. Special:Contributions is a new special page handler.

**Tech Stack:** Go, SQLite, html/template, gorilla/mux

**Design doc:** `docs/plans/2026-03-01-user-namespace-design.md`

---

### Task 1: Domain helpers — User page URL functions

Add namespace detection and URL construction helpers for User: and User_talk: pages, following the pattern of the existing Talk: helpers.

**Files:**
- Modify: `wiki/article.go:41-55`
- Test: `wiki/article_test.go` (existing file, add tests)

**Step 1: Write the failing tests**

In `wiki/article_test.go`, add:

```go
func TestIsUserPage(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"User:Alice", true},
		{"User:alice_bob", true},
		{"User_talk:Alice", false},
		{"Talk:User:Alice", false},
		{"Alice", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsUserPage(tt.url); got != tt.want {
			t.Errorf("IsUserPage(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestIsUserTalkPage(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"User_talk:Alice", true},
		{"User:Alice", false},
		{"Talk:Alice", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsUserTalkPage(tt.url); got != tt.want {
			t.Errorf("IsUserTalkPage(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestUserPageScreenName(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"User:Alice", "Alice"},
		{"User_talk:Alice", "Alice"},
		{"User:alice_bob", "alice_bob"},
		{"User_talk:alice_bob", "alice_bob"},
	}
	for _, tt := range tests {
		if got := UserPageScreenName(tt.url); got != tt.want {
			t.Errorf("UserPageScreenName(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestUserPageURL(t *testing.T) {
	if got := UserPageURL("Alice"); got != "User:Alice" {
		t.Errorf("UserPageURL(\"Alice\") = %q, want \"User:Alice\"", got)
	}
}

func TestUserTalkPageURL(t *testing.T) {
	if got := UserTalkPageURL("Alice"); got != "User_talk:Alice" {
		t.Errorf("UserTalkPageURL(\"Alice\") = %q, want \"User_talk:Alice\"", got)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./wiki/ -run 'TestIsUserPage|TestIsUserTalkPage|TestUserPageScreenName|TestUserPageURL|TestUserTalkPageURL' -v`
Expected: FAIL — functions not defined.

**Step 3: Implement helpers**

In `wiki/article.go`, after the existing Talk helper functions (line 55), add:

```go
// IsUserPage returns true if the URL is in the User namespace (but not User_talk).
func IsUserPage(url string) bool {
	return strings.HasPrefix(url, "User:") && !strings.HasPrefix(url, "User_talk:")
}

// IsUserTalkPage returns true if the URL is in the User_talk namespace.
func IsUserTalkPage(url string) bool {
	return strings.HasPrefix(url, "User_talk:")
}

// UserPageURL returns the User namespace URL for a given screen name.
func UserPageURL(screenName string) string {
	return "User:" + screenName
}

// UserTalkPageURL returns the User_talk namespace URL for a given screen name.
func UserTalkPageURL(screenName string) string {
	return "User_talk:" + screenName
}

// UserPageScreenName extracts the screen name from a User: or User_talk: URL.
func UserPageScreenName(url string) string {
	if strings.HasPrefix(url, "User_talk:") {
		return strings.TrimPrefix(url, "User_talk:")
	}
	return strings.TrimPrefix(url, "User:")
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./wiki/ -run 'TestIsUserPage|TestIsUserTalkPage|TestUserPageScreenName|TestUserPageURL|TestUserTalkPageURL' -v`
Expected: PASS

**Step 5: Commit**

```
feat: Add User: and User_talk: URL helpers
```

---

### Task 2: HasUserPage field and template URL helpers

Add the `HasUserPage` field to `wiki.User` and add template helper functions.

**Files:**
- Modify: `wiki/user.go:16-25`
- Modify: `templater/urlhelper.go:54-66`

**Step 1: Add HasUserPage to User struct**

In `wiki/user.go`, add a field to the User struct after `IPAddress`:

```go
type User struct {
	Email        string    `db:"email"`
	ScreenName   string    `db:"screenname"`
	ID           int       `db:"id"`
	PasswordHash string    `db:"passwordhash"`
	Role         string    `db:"role"`
	CreatedAt    time.Time `db:"created_at"`
	RawPassword  string
	IPAddress    string
	HasUserPage  bool `db:"has_user_page"`
}
```

**Step 2: Add template helpers**

In `templater/urlhelper.go`, add after the existing `subjectURL` function (line 66):

```go
func isUserPage(urlPath string) bool {
	return wiki.IsUserPage(urlPath)
}

func isUserTalkPage(urlPath string) bool {
	return wiki.IsUserTalkPage(urlPath)
}

func userPageURL(screenName string) string {
	return wiki.UserPageURL(screenName)
}

func userTalkPageURL(screenName string) string {
	return wiki.UserTalkPageURL(screenName)
}

func userPageScreenName(urlPath string) string {
	return wiki.UserPageScreenName(urlPath)
}

func contributionsURL(screenName string) string {
	return "/wiki/Special:Contributions/" + url.PathEscape(screenName)
}

func userPageArticleURL(screenName string) string {
	return "/wiki/" + url.PathEscape(wiki.UserPageURL(screenName))
}
```

Then register these in the template FuncMap. Find where the existing helpers are registered (in `templater/templater.go` or wherever `FuncMap` is built) and add the new functions.

**Step 3: Run build to verify compilation**

Run: `go build ./...`
Expected: PASS (or at minimum `go vet ./...`)

**Step 4: Commit**

```
feat: Add HasUserPage field and template URL helpers
```

---

### Task 3: Repository — HasUserPage subquery in existing queries

Add `EXISTS(SELECT 1 FROM Article a2 WHERE a2.url = 'User:' || User.screenname) AS has_user_page` to revision history queries and the prepared article statements.

**Files:**
- Modify: `internal/storage/sqlite.go:24-28` (prepared statement base query)
- Modify: `internal/storage/article_repo.go:18-29,91-127` (articleResult struct + SelectRevisionHistory)
- Modify: `testutil/testutil.go:322-333,384-422` (articleResult struct + SelectRevisionHistory)

**Step 1: Update prepared statement base query**

In `internal/storage/sqlite.go`, change the base query (line 24-28):

```go
q := `SELECT url, Revision.id, markdown, COALESCE(html, '') AS html, hashval, created, previous_id, comment, User.id AS user_id, User.screenname,
		EXISTS(SELECT 1 FROM Article a2 WHERE a2.url = 'User:' || User.screenname) AS has_user_page
		FROM Article
		JOIN Revision ON Article.id = Revision.article_id
		JOIN User ON Revision.user_id = User.id
		WHERE Article.url = ?`
```

**Step 2: Update production articleResult struct**

In `internal/storage/article_repo.go`, add `HasUserPage` to the `articleResult` struct (line 18-29):

```go
type articleResult struct {
	URL         string
	ID          int
	Markdown    string
	HTML        string
	Hash        string    `db:"hashval"`
	Created     time.Time
	PreviousID  int       `db:"previous_id"`
	Comment     string
	UserID      int       `db:"user_id"`
	ScreenName  string    `db:"screenname"`
	HasUserPage bool      `db:"has_user_page"`
}
```

Update `toArticle()` (line 31-45) to pass HasUserPage:

```go
Creator: &wiki.User{ID: r.UserID, ScreenName: r.ScreenName, HasUserPage: r.HasUserPage},
```

**Step 3: Update production SelectRevisionHistory**

In `internal/storage/article_repo.go`, change the query at line 92-96:

```go
rows, err := db.conn.Queryx(
	`SELECT Revision.id, hashval, created, comment, previous_id, User.screenname, length(markdown),
		EXISTS(SELECT 1 FROM Article a2 WHERE a2.url = 'User:' || User.screenname) AS has_user_page
		FROM Article JOIN Revision ON Article.id = Revision.article_id
				     JOIN User ON Revision.user_id = User.id
		WHERE Article.url = ? ORDER BY Revision.id DESC`, url)
```

Update the scan struct (line 100-106) to include `HasUserPage`:

```go
result := struct {
	Hashval, Comment, Screenname string
	ID                           int
	PreviousID                   int       `db:"previous_id"`
	Length                       int       `db:"length(markdown)"`
	Created                      time.Time
	HasUserPage                  bool `db:"has_user_page"`
}{}
```

And set it on the revision (after line 120):

```go
rev.Creator.HasUserPage = result.HasUserPage
```

**Step 4: Mirror all changes in testutil**

In `testutil/testutil.go`, update the `articleResult` struct (line 322-333):

```go
type articleResult struct {
	URL         string
	ID          int
	Markdown    string
	HTML        string
	Hash        string    `db:"hashval"`
	Created     time.Time
	PreviousID  int       `db:"previous_id"`
	Comment     string
	UserID      int       `db:"user_id"`
	ScreenName  string    `db:"screenname"`
	HasUserPage bool      `db:"has_user_page"`
}
```

Update `toArticle()` (line 335-349):

```go
Creator: &wiki.User{ID: r.UserID, ScreenName: r.ScreenName, HasUserPage: r.HasUserPage},
```

Update `SelectRevisionHistory` (line 384-422) with the same query and scan changes as production.

**Step 5: Run tests**

Run: `go test ./... -count=1`
Expected: PASS — all existing tests should pass with the new column.

**Step 6: Commit**

```
feat: Add HasUserPage EXISTS subquery to article and history queries
```

---

### Task 4: Repository — SelectRevisionsByScreenName and SelectUserEditCount

Add new query methods for the contributions page and user profile stats.

**Files:**
- Modify: `wiki/revision.go` (add ContributionEntry type)
- Modify: `wiki/repository/article.go` (add interface methods)
- Modify: `internal/storage/article_repo.go` (SQLite implementation)
- Modify: `testutil/testutil.go` (TestDB implementation)
- Modify: `wiki/service/article.go:18-59` (add service methods)
- Test: `wiki/service/article_test.go` (add tests)

**Step 1: Add ContributionEntry type**

In `wiki/revision.go`, add:

```go
// ContributionEntry represents a single edit by a user, for the contributions page.
type ContributionEntry struct {
	ArticleURL   string
	RevisionID   int
	PreviousID   int
	Created      time.Time
	Comment      string
	MarkdownSize int
}
```

**Step 2: Add interface methods**

In `wiki/repository/article.go`, add to the `ArticleRepository` interface:

```go
	// SelectRevisionsByScreenName returns all revisions by a user, newest first.
	SelectRevisionsByScreenName(screenName string) ([]*wiki.ContributionEntry, error)

	// SelectUserEditCount returns the total number of edits by a user.
	SelectUserEditCount(userID int) (int, error)
```

**Step 3: Implement in SQLite storage**

In `internal/storage/article_repo.go`, add:

```go
func (db *sqliteDb) SelectRevisionsByScreenName(screenName string) ([]*wiki.ContributionEntry, error) {
	rows, err := db.conn.Queryx(
		`SELECT Article.url, Revision.id, Revision.previous_id, Revision.created, Revision.comment, length(Revision.markdown)
			FROM Revision
			JOIN User ON Revision.user_id = User.id
			JOIN Article ON Revision.article_id = Article.id
			WHERE User.screenname = ?
			ORDER BY Revision.created DESC`, screenName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := struct {
		URL        string    `db:"url"`
		ID         int       `db:"id"`
		PreviousID int       `db:"previous_id"`
		Created    time.Time `db:"created"`
		Comment    string    `db:"comment"`
		Length     int       `db:"length(Revision.markdown)"`
	}{}
	var entries []*wiki.ContributionEntry
	for rows.Next() {
		if err := rows.StructScan(&result); err != nil {
			return nil, err
		}
		entries = append(entries, &wiki.ContributionEntry{
			ArticleURL:   result.URL,
			RevisionID:   result.ID,
			PreviousID:   result.PreviousID,
			Created:      result.Created,
			Comment:      result.Comment,
			MarkdownSize: result.Length,
		})
	}
	return entries, rows.Err()
}

func (db *sqliteDb) SelectUserEditCount(userID int) (int, error) {
	var count int
	err := db.conn.Get(&count, `SELECT COUNT(*) FROM Revision WHERE user_id = ?`, userID)
	return count, err
}
```

**Step 4: Implement in TestDB**

In `testutil/testutil.go`, add matching implementations:

```go
func (tdb *TestDB) SelectRevisionsByScreenName(screenName string) ([]*wiki.ContributionEntry, error) {
	rows, err := tdb.conn.Queryx(
		`SELECT Article.url, Revision.id, Revision.previous_id, Revision.created, Revision.comment, length(Revision.markdown)
			FROM Revision
			JOIN User ON Revision.user_id = User.id
			JOIN Article ON Revision.article_id = Article.id
			WHERE User.screenname = ?
			ORDER BY Revision.created DESC`, screenName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := struct {
		URL     string    `db:"url"`
		ID      int       `db:"id"`
		PrevID  int       `db:"previous_id"`
		Created time.Time `db:"created"`
		Comment string    `db:"comment"`
		Length  int       `db:"length(Revision.markdown)"`
	}{}
	var entries []*wiki.ContributionEntry
	for rows.Next() {
		if err := rows.StructScan(&result); err != nil {
			return nil, err
		}
		entries = append(entries, &wiki.ContributionEntry{
			ArticleURL:   result.URL,
			RevisionID:   result.ID,
			PreviousID:   result.PrevID,
			Created:      result.Created,
			Comment:      result.Comment,
			MarkdownSize: result.Length,
		})
	}
	return entries, rows.Err()
}

func (tdb *TestDB) SelectUserEditCount(userID int) (int, error) {
	var count int
	err := tdb.conn.Get(&count, `SELECT COUNT(*) FROM Revision WHERE user_id = ?`, userID)
	return count, err
}
```

**Step 5: Add service methods**

In `wiki/service/article.go`, add to the `ArticleService` interface:

```go
	// GetRevisionsByScreenName returns all contributions by a user.
	GetRevisionsByScreenName(screenName string) ([]*wiki.ContributionEntry, error)

	// GetUserEditCount returns the total number of edits by a user.
	GetUserEditCount(userID int) (int, error)
```

Add implementations on `articleService`:

```go
func (s *articleService) GetRevisionsByScreenName(screenName string) ([]*wiki.ContributionEntry, error) {
	return s.repo.SelectRevisionsByScreenName(screenName)
}

func (s *articleService) GetUserEditCount(userID int) (int, error) {
	return s.repo.SelectUserEditCount(userID)
}
```

Also add these methods to the `EmbeddedArticleService` wrapper (in the file where it's defined — check `wiki/service/embedded.go` or similar). The wrapper just delegates:

```go
func (s *embeddedArticleService) GetRevisionsByScreenName(screenName string) ([]*wiki.ContributionEntry, error) {
	return s.inner.GetRevisionsByScreenName(screenName)
}

func (s *embeddedArticleService) GetUserEditCount(userID int) (int, error) {
	return s.inner.GetUserEditCount(userID)
}
```

**Step 6: Write and run service tests**

In `wiki/service/article_test.go`, add tests for GetRevisionsByScreenName and GetUserEditCount. Create a user, create several articles by that user, then verify the results.

**Step 7: Run all tests**

Run: `go test ./... -count=1`
Expected: PASS

**Step 8: Commit**

```
feat: Add repository and service methods for user contributions
```

---

### Task 5: Special:Contributions handler

Create the contributions special page that lists all edits by a user.

**Files:**
- Create: `special/contributions.go`
- Create: `special/contributions_test.go`
- Create: `templates/special/contributions.html`
- Modify: `internal/server/app.go:156-169` (register in `RegisterSpecialPages`)
- Modify: `testutil/testutil.go:232-233` (register in test setup)

**Step 1: Create the template**

Create `templates/special/contributions.html`:

```html
{{define "content"}}
<div id="article-area">
    <article>
        <h1>Contributions by {{.ScreenName}}</h1>
        <div class="pw-article-content">
            {{if .Contributions}}
            <ul>
                {{range .Contributions}}
                <li>
                    <a href="{{revisionURL .ArticleURL .RevisionID}}">
                        {{.Created.Format "2006, Jan _2 3:04 MST"}}
                    </a>
                    <a href="{{articleURL .ArticleURL}}">{{inferTitle .ArticleURL}}</a>
                    ({{.MarkdownSize}} bytes){{if .Comment}}
                    <em>({{.Comment}})</em>{{end}}{{if gt .PreviousID 0}}
                    (<a href="{{diffURL .ArticleURL .PreviousID .RevisionID}}">diff</a>){{end}}
                </li>
                {{end}}
            </ul>
            {{else}}
            <p>This user has not made any edits.</p>
            {{end}}
        </div>
    </article>
</div>
{{end}}
```

**Step 2: Create the handler**

Create `special/contributions.go`:

```go
package special

import (
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/danielledeleo/periwiki/wiki"
)

// ContributionsLister retrieves contributions for a user.
type ContributionsLister interface {
	GetRevisionsByScreenName(screenName string) ([]*wiki.ContributionEntry, error)
}

// ContributionsTemplater renders the contributions template.
type ContributionsTemplater interface {
	RenderTemplate(w io.Writer, name string, base string, data map[string]interface{}) error
}

// ContributionsPage handles Special:Contributions requests.
type ContributionsPage struct {
	contributions ContributionsLister
	users         UserChecker
	templater     ContributionsTemplater
}

// UserChecker verifies a user exists.
type UserChecker interface {
	GetUserByScreenName(screenName string) (*wiki.User, error)
}

// NewContributionsPage creates a new contributions special page handler.
func NewContributionsPage(contributions ContributionsLister, users UserChecker, templater ContributionsTemplater) *ContributionsPage {
	return &ContributionsPage{
		contributions: contributions,
		users:         users,
		templater:     templater,
	}
}

// Handle serves the contributions page for a user.
// URL format: /wiki/Special:Contributions/ScreenName
func (p *ContributionsPage) Handle(rw http.ResponseWriter, req *http.Request) {
	// Extract screen name from URL path
	path := req.URL.Path
	const prefix = "/wiki/Special:Contributions/"
	screenName := ""
	if idx := strings.Index(path, prefix); idx >= 0 {
		screenName = path[idx+len(prefix):]
	}

	if screenName == "" {
		http.Error(rw, "Usage: Special:Contributions/Username", http.StatusBadRequest)
		return
	}

	// Verify user exists
	if _, err := p.users.GetUserByScreenName(screenName); err != nil {
		http.Error(rw, "User not found", http.StatusNotFound)
		return
	}

	entries, err := p.contributions.GetRevisionsByScreenName(screenName)
	if err != nil {
		slog.Error("failed to get contributions", "screenName", screenName, "error", err)
		http.Error(rw, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Page":          wiki.NewStaticPage("Contributions by " + screenName),
		"Context":       req.Context(),
		"ScreenName":    screenName,
		"Contributions": entries,
	}

	if err := p.templater.RenderTemplate(rw, "contributions.html", "index.html", data); err != nil {
		slog.Error("failed to render contributions template", "error", err)
		http.Error(rw, "Internal server error", http.StatusInternalServerError)
	}
}
```

**Step 3: Register in production setup**

In `internal/server/app.go`, update `RegisterSpecialPages` (line 156-169). Add the `UserService` parameter:

```go
func RegisterSpecialPages(articles service.ArticleService, users service.UserService, t *templater.Templater, baseURL string) *special.Registry {
```

Add after the existing registrations:

```go
	registry.Register("Contributions", special.NewContributionsPage(articles, users, t))
```

Update the caller of `RegisterSpecialPages` in `setup.go` (or wherever it's called) to pass the users service.

**Step 4: Register in test setup**

In `testutil/testutil.go`, around line 232-233, add:

```go
specialPages.Register("Contributions", special.NewContributionsPage(articleServiceWrapped, userService, tmpl))
```

**Step 5: Write unit tests**

Create `special/contributions_test.go` following the pattern of `special/what_links_here_test.go`. Use mock types for the interfaces.

**Step 6: Run tests**

Run: `go test ./... -count=1`
Expected: PASS

**Step 7: Commit**

```
feat: Add Special:Contributions page
```

---

### Task 6: Route regex and NamespaceHandler — User and User_talk

Update the route regex to support slashes in page names (for `Special:Contributions/Username`), and add User:/User_talk: cases to NamespaceHandler.

**Files:**
- Modify: `internal/server/app.go:120` (route regex)
- Modify: `internal/server/handlers.go:775-831` (NamespaceHandler)

**Step 1: Update route regex**

In `internal/server/app.go`, line 120, change `{page}` to `{page:.+}`:

```go
router.HandleFunc("/wiki/{namespace:[^:/]+}:{page:.+}", a.NamespaceHandler).Methods("GET", "POST")
```

This allows slashes in the page portion (e.g., `Contributions/Alice`).

**Step 2: Update NamespaceHandler for Special subpage lookup**

In the Special case of `NamespaceHandler` (handlers.go:780-789), add prefix matching:

```go
if strings.EqualFold(namespace, "special") {
	handler, ok := a.SpecialPages.Get(page)
	if !ok {
		// Support subpage URLs like Contributions/Username
		if idx := strings.IndexByte(page, '/'); idx >= 0 {
			handler, ok = a.SpecialPages.Get(page[:idx])
		}
	}
	if !ok {
		a.ErrorHandler(http.StatusNotFound, rw, req,
			fmt.Errorf("special page '%s' does not exist", page))
		return
	}
	handler.Handle(rw, req)
	return
}
```

**Step 3: Add User_talk case**

Add before the Periwiki case, after the Talk case (after line 799):

```go
// User_talk namespace: discussion pages for user pages
if strings.EqualFold(namespace, "user_talk") {
	if mdPage, ok := strings.CutSuffix(page, ".md"); ok {
		a.serveArticleMarkdown(rw, req, "User_talk:"+mdPage)
		return
	}
	a.dispatchArticle(rw, req, "User_talk:"+page)
	return
}
```

**Step 4: Add User case**

Add after the User_talk case:

```go
// User namespace: user profile pages
if strings.EqualFold(namespace, "user") {
	a.dispatchUserPage(rw, req, "User:"+page)
	return
}
```

**Step 5: Run build**

Run: `go build ./...`
Expected: Will fail until Task 7 adds `dispatchUserPage`. That's OK — complete Task 7 before testing.

**Step 6: Commit (together with Task 7)**

---

### Task 7: User page handler and template

Create the user page handler that renders the two-section layout (custom content + stats), and the `dispatchUserPage` dispatcher.

**Files:**
- Modify: `internal/server/handlers.go` (add dispatchUserPage and handleUserPage)
- Create: `templates/user_page.html`

**Step 1: Create user_page.html template**

Create `templates/user_page.html`:

```html
{{define "content"}}
<div id="article-area">
    {{template "tabs" .}}
    <article>
        <h1>{{.UserProfile.ScreenName}}</h1>
        <div class="pw-article-content">
            {{if .CustomContent}}
            {{.CustomContent}}
            <hr>
            {{end}}
            <h2>User info</h2>
            <ul>
                <li>Member since {{.UserProfile.CreatedAt.Format "January 2, 2006"}}</li>
                <li>{{.EditCount}} {{if eq .EditCount 1}}edit{{else}}edits{{end}}</li>
            </ul>
        </div>
    </article>
</div>
{{end}}
```

**Step 2: Add dispatchUserPage and handleUserPage**

In `internal/server/handlers.go`, add after `dispatchArticle`:

```go
// dispatchUserPage routes User: namespace requests.
// Default view uses handleUserPage; edit/history/diff use the standard article handlers.
func (a *App) dispatchUserPage(rw http.ResponseWriter, req *http.Request, articleURL string) {
	params := req.URL.Query()

	if req.Method == "POST" {
		a.handleArticlePost(rw, req, articleURL)
		return
	}
	if params.Has("diff") {
		a.handleDiff(rw, req, articleURL, params)
		return
	}
	if params.Has("history") {
		a.handleHistory(rw, req, articleURL)
		return
	}
	if params.Has("edit") {
		a.handleEdit(rw, req, articleURL, params)
		return
	}
	a.handleUserPage(rw, req, articleURL)
}

// handleUserPage renders a user profile page with optional custom content and stats.
func (a *App) handleUserPage(rw http.ResponseWriter, req *http.Request, articleURL string) {
	screenName := wiki.UserPageScreenName(articleURL)

	// Verify the user exists
	userProfile, err := a.Users.GetUserByScreenName(screenName)
	if err != nil {
		a.ErrorHandler(http.StatusNotFound, rw, req, err)
		return
	}

	// Try to get the user's custom page content
	var article *wiki.Article
	var customContent template.HTML
	article, err = a.Articles.GetArticle(articleURL)
	if err != nil {
		// No custom page — create a placeholder for tabs
		article = wiki.NewArticle(articleURL, "")
	} else {
		customContent = template.HTML(article.HTML)
	}

	// Get edit count
	editCount, _ := a.Articles.GetUserEditCount(userProfile.ID)

	// Check if current user can edit
	currentUser := req.Context().Value(wiki.UserKey).(*wiki.User)
	canEdit := currentUser.ID == userProfile.ID || currentUser.IsAdmin()

	err = a.RenderTemplate(rw, "user_page.html", "index.html", map[string]interface{}{
		"Page":          wiki.NewStaticPage(screenName),
		"Article":       article,
		"Context":       req.Context(),
		"ActiveTab":     "userpage",
		"UserProfile":   userProfile,
		"EditCount":     editCount,
		"CustomContent": customContent,
		"HideEdit":      !canEdit,
	})
	check(err)
}
```

Note: you'll need `"html/template"` imported for `template.HTML`.

**Step 3: Run tests**

Run: `go test ./... -count=1`
Expected: PASS

**Step 4: Commit (together with Task 6)**

```
feat: Add User: and User_talk: namespace handling with user page view
```

---

### Task 8: Edit restrictions for User: pages

Add authorization guards so only the page owner (or admin) can edit User: pages, and User_talk: pages require the user to exist.

**Files:**
- Modify: `internal/server/handlers.go:482-555` (handleEdit)
- Modify: `internal/server/handlers.go:666-709` (handleArticlePost)

**Step 1: Add User: page edit guard to handleEdit**

In `handleEdit`, after the Talk page guard (line 496) and before line 498 (`rw.Header().Set("Cache-Control"...`), add:

```go
// Block editing User: pages unless the current user is the owner or admin
if wiki.IsUserPage(articleURL) {
	user := req.Context().Value(wiki.UserKey).(*wiki.User)
	ownerName := wiki.UserPageScreenName(articleURL)
	if user.IsAnonymous() || (user.ScreenName != ownerName && !user.IsAdmin()) {
		a.ErrorHandler(http.StatusForbidden, rw, req, wiki.ErrForbidden)
		return
	}
}

// Block editing User_talk: pages when the user doesn't exist
if wiki.IsUserTalkPage(articleURL) {
	ownerName := wiki.UserPageScreenName(articleURL)
	if _, err := a.Users.GetUserByScreenName(ownerName); err != nil {
		a.ErrorHandler(http.StatusNotFound, rw, req,
			fmt.Errorf("cannot create user talk page: user %q does not exist", ownerName))
		return
	}
}
```

**Step 2: Add User: page save guard to handleArticlePost**

In `handleArticlePost`, after the anonymous editing check (line 685) and before `article.Creator = user` (line 687), add:

```go
// Block saving User: pages unless the current user is the owner or admin
if wiki.IsUserPage(articleURL) {
	ownerName := wiki.UserPageScreenName(articleURL)
	if user.IsAnonymous() || (user.ScreenName != ownerName && !user.IsAdmin()) {
		a.ErrorHandler(http.StatusForbidden, rw, req, wiki.ErrForbidden)
		return
	}
}
```

**Step 3: Write integration tests**

In `internal/server/handlers_integration_test.go`, add tests:

```go
func TestUserNamespace_EditBlockedForNonOwner(t *testing.T) {
	// Create two users. Log in as user2. Try to edit User:user1. Expect 403.
}

func TestUserNamespace_EditAllowedForOwner(t *testing.T) {
	// Create user. Log in as that user. Edit User:user. Expect 200 (edit form).
}

func TestUserNamespace_EditAllowedForAdmin(t *testing.T) {
	// Create user and admin. Log in as admin. Edit User:user. Expect 200.
}

func TestUserTalkNamespace_EditBlockedWhenUserMissing(t *testing.T) {
	// Try to edit User_talk:Nonexistent. Expect 404.
}

func TestUserTalkNamespace_EditAllowedForAnyUser(t *testing.T) {
	// Create user1 and user2. Log in as user2. Edit User_talk:user1. Expect 200.
}
```

Follow the pattern in `TestTalkNamespace_EditBlockedWhenSubjectMissing` (line 714-731). Set up the session cookie for authenticated requests.

**Step 4: Run tests**

Run: `go test ./internal/server/ -run 'TestUserNamespace|TestUserTalkNamespace' -v`
Expected: PASS

**Step 5: Commit**

```
feat: Add edit restrictions for User: pages and User_talk: creation guard
```

---

### Task 9: Template updates — tabs, history, diff, navbar

Update all templates to support User:/User_talk: pages and username linking.

**Files:**
- Modify: `templates/layouts/_tabs.html`
- Modify: `templates/article_history.html`
- Modify: `templates/diff.html`
- Modify: `templates/layouts/index.html:15`

**Step 1: Update _tabs.html**

Replace the full content of `templates/layouts/_tabs.html` with:

```html
{{define "tabs"}}
<ul class="pw-tabs">
    {{- if isUserTalkPage .Article.URL}}
    <li><a href="{{articleURL (userPageURL (userPageScreenName .Article.URL))}}">User page</a></li>
    <li class="pw-active"><a href="{{articleURL .Article.URL}}">Talk</a></li>
    <li><a href="{{contributionsURL (userPageScreenName .Article.URL)}}">Contribs</a></li>
    {{- else if isUserPage .Article.URL}}
    <li class="pw-active"><a href="{{articleURL .Article.URL}}">User page</a></li>
    <li><a href="{{articleURL (userTalkPageURL (userPageScreenName .Article.URL))}}">Talk</a></li>
    <li><a href="{{contributionsURL (userPageScreenName .Article.URL)}}">Contribs</a></li>
    {{- else if isTalkPage .Article.URL}}
    <li><a href="{{articleURL (subjectURL .Article.URL)}}">Article</a></li>
    <li class="pw-active"><a href="{{articleURL .Article.URL}}">Talk</a></li>
    {{- else}}
    <li class="pw-active"><a href="{{articleURL .Article.URL}}">
        {{- if and .Article.Layout (eq .Article.Layout "mainpage")}}Main page{{else}}Article{{end -}}
    </a></li>
    {{- if not .Article.ReadOnly}}
    <li><a href="{{articleURL (talkPageURL .Article.URL)}}">Talk</a></li>
    {{- end}}
    {{- end}}
    {{- if not .Article.ReadOnly}}
    {{- if not (index . "HideEdit")}}
    {{- if or .Article.ID (eq .ActiveTab "history")}}
    <li class="pw-tab-right{{if eq .ActiveTab "history"}} pw-active{{end}}"><a href="{{historyURL .Article.URL}}">History</a></li>
    <li{{if eq .ActiveTab "edit"}} class="pw-active"{{end}}><a href="{{editURL .Article.URL .Article.ID}}">Edit</a></li>
    {{- else}}
    <li class="pw-tab-right{{if eq .ActiveTab "edit"}} pw-active{{end}}"><a href="{{editURL .Article.URL .Article.ID}}">Edit</a></li>
    {{- end}}
    {{- else}}{{/* HideEdit is true — show only History */}}
    {{- if or .Article.ID (eq .ActiveTab "history")}}
    <li class="pw-tab-right{{if eq .ActiveTab "history"}} pw-active{{end}}"><a href="{{historyURL .Article.URL}}">History</a></li>
    {{- end}}
    {{- end}}
    {{- else if .EmbeddedSourceURL}}
    <li class="pw-tab-right"><a href="{{.EmbeddedSourceURL}}" target="_blank">View source</a></li>
    {{- end}}
</ul>
{{end}}
```

Key changes:
- New `isUserTalkPage` and `isUserPage` branches at the top
- `HideEdit` check via `index . "HideEdit"` — when true, hides Edit but keeps History
- User/User_talk branches include "Contribs" tab

**Step 2: Update article_history.html**

In `templates/article_history.html`, change line 14 from:

```html
                    </a> by {{.Creator.ScreenName}} ({{.Markdown}} bytes){{if .Comment}} ...
```

to:

```html
                    </a> by <a href="{{userPageArticleURL .Creator.ScreenName}}"{{if not .Creator.HasUserPage}} class="pw-deadlink"{{end}}>{{.Creator.ScreenName}}</a> (<a href="{{articleURL (userTalkPageURL .Creator.ScreenName)}}">talk</a> | <a href="{{contributionsURL .Creator.ScreenName}}">contribs</a>) ({{.Markdown}} bytes){{if .Comment}} ...
```

**Step 3: Update diff.html**

In `templates/diff.html`, change line 11 from:

```html
                {{if .Creator}}by {{.Creator.ScreenName}}{{end}}
```

to:

```html
                {{if .Creator}}by <a href="{{userPageArticleURL .Creator.ScreenName}}"{{if not .Creator.HasUserPage}} class="pw-deadlink"{{end}}>{{.Creator.ScreenName}}</a> (<a href="{{articleURL (userTalkPageURL .Creator.ScreenName)}}">talk</a> | <a href="{{contributionsURL .Creator.ScreenName}}">contribs</a>){{end}}
```

Make the same change on line 17 for `$.NewRevision.Creator`.

**Step 4: Update navbar**

In `templates/layouts/index.html`, change line 15 from:

```html
                <li><a href="/profile/{{ pathEscape .User.ScreenName }}">{{.User.ScreenName}}</a></li>
```

to:

```html
                <li><a href="/wiki/User:{{ pathEscape .User.ScreenName }}">{{.User.ScreenName}}</a></li>
```

**Step 5: Run tests**

Run: `go test ./... -count=1`
Expected: PASS

**Step 6: Commit**

```
feat: Update templates with user page tabs, username links, and navbar
```

---

### Task 10: ExistenceChecker — Support Special: subpage URLs

Update the existence checker so `[[Special:Contributions/Alice]]` wikilinks resolve as blue links when the Contributions handler exists.

**Files:**
- Modify: `internal/server/app.go:187-207` (ExistenceState.check)
- Modify: `testutil/testutil.go:174-192` (test existence checker)

**Step 1: Update production ExistenceState.check**

In `internal/server/app.go`, change the Special: check (line 202-204):

```go
if s.SpecialPages != nil && strings.HasPrefix(url, "Special:") {
	name := strings.TrimPrefix(url, "Special:")
	if s.SpecialPages.Has(name) {
		return true
	}
	// Support subpage URLs like Special:Contributions/Username
	if idx := strings.IndexByte(name, '/'); idx >= 0 {
		return s.SpecialPages.Has(name[:idx])
	}
}
```

**Step 2: Update test existence checker**

In `testutil/testutil.go`, around line 187-189, make the same change:

```go
if specialPages != nil && strings.HasPrefix(url, "Special:") {
	name := strings.TrimPrefix(url, "Special:")
	if specialPages.Has(name) {
		return true
	}
	if idx := strings.IndexByte(name, '/'); idx >= 0 {
		return specialPages.Has(name[:idx])
	}
}
```

**Step 3: Run tests**

Run: `go test ./... -count=1`
Expected: PASS

**Step 4: Commit**

```
feat: Support Special: subpage URLs in existence checker
```

---

### Task 11: Integration tests — Full end-to-end

Add comprehensive integration tests covering the whole feature.

**Files:**
- Modify: `internal/server/handlers_integration_test.go`

**Step 1: Write integration tests**

Add to `handlers_integration_test.go`:

```go
func TestUserNamespace_ViewProfile(t *testing.T) {
	router, testApp, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, testApp.DB, "alice", "alice@example.com", "password123")
	testutil.CreateTestArticle(t, testApp, "Test_Article", "Content", user)

	req := httptest.NewRequest("GET", "/wiki/User:alice", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "alice") {
		t.Error("expected body to contain username")
	}
	if !strings.Contains(body, "edit") {
		// edit count should appear
	}
}

func TestUserNamespace_ViewProfile_NotFound(t *testing.T) {
	router, _, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/wiki/User:nonexistent", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestUserNamespace_ViewProfileWithCustomContent(t *testing.T) {
	router, testApp, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, testApp.DB, "alice", "alice@example.com", "password123")
	testutil.CreateTestArticle(t, testApp, "User:alice", "Hello, I'm Alice!", user)

	req := httptest.NewRequest("GET", "/wiki/User:alice", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Alice") {
		t.Error("expected custom content")
	}
}

func TestUserTalkNamespace_DispatchesToArticle(t *testing.T) {
	router, testApp, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, testApp.DB, "alice", "alice@example.com", "password123")
	testutil.CreateTestArticle(t, testApp, "User_talk:alice", "Discussion here", user)

	req := httptest.NewRequest("GET", "/wiki/User_talk:alice", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Discussion here") {
		t.Error("expected talk page content")
	}
}

func TestSpecialContributions(t *testing.T) {
	router, testApp, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, testApp.DB, "alice", "alice@example.com", "password123")
	testutil.CreateTestArticle(t, testApp, "Test_Article", "Content", user)

	req := httptest.NewRequest("GET", "/wiki/Special:Contributions/alice", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Test_Article") || !strings.Contains(body, "alice") {
		t.Error("expected contributions to list the article and username")
	}
}

func TestHistoryPage_UsernameLinks(t *testing.T) {
	router, testApp, cleanup := setupHandlerTestRouter(t)
	defer cleanup()

	user := testutil.CreateTestUser(t, testApp.DB, "alice", "alice@example.com", "password123")
	testutil.CreateTestArticle(t, testApp, "Test_Article", "Content", user)

	req := httptest.NewRequest("GET", "/wiki/Test_Article?history", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "/wiki/User:alice") {
		t.Error("expected history page to link to User:alice")
	}
	if !strings.Contains(body, "talk") {
		t.Error("expected history page to contain talk link")
	}
	if !strings.Contains(body, "contribs") {
		t.Error("expected history page to contain contribs link")
	}
}
```

**Step 2: Run all tests**

Run: `go test ./... -count=1`
Expected: PASS

**Step 3: Commit**

```
test: Add integration tests for User:, User_talk:, and Special:Contributions
```

---

### Task 12: Final verification and cleanup

**Step 1: Run full test suite**

Run: `go test ./... -count=1 -race`
Expected: PASS

**Step 2: Manual smoke test**

Run: `make && ./periwiki`

Verify:
- Navigate to `/wiki/User:yourname` — see stats, edit link if logged in as that user
- Create User: page content — see it rendered above stats
- Navigate to `/wiki/User_talk:yourname` — create/view talk page
- Navigate to `/wiki/Special:Contributions/yourname` — see edit list
- Check article history page — usernames link to User: pages with (talk | contribs)
- Check diff page — same username links
- Check navbar — username links to User: page
- Red link test: create a new user, check another user's history — new user's link should be red (no User: article yet)

**Step 3: Commit any fixes**

```
fix: Address issues found during smoke testing
```
