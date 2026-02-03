package server

import (
	"bytes"
	"fmt"
	"html"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	"github.com/danielledeleo/periwiki/wiki"
	"github.com/gorilla/mux"
	"github.com/sergi/go-diff/diffmatchpatch"
)

func (a *App) RegisterHandler(rw http.ResponseWriter, req *http.Request) {
	err := a.RenderTemplate(rw, "register.html", "index.html",
		map[string]interface{}{
			"Page":    wiki.NewStaticPage("Register"),
			"Article": map[string]string{"Title": "Register"},
			"Context": req.Context()})
	check(err)
}

func (a *App) RegisterPostHandler(rw http.ResponseWriter, req *http.Request) {
	user := &wiki.User{}

	user.Email = req.PostFormValue("email")
	user.ScreenName = req.PostFormValue("screenname")
	user.RawPassword = req.PostFormValue("password")

	render := map[string]interface{}{
		"Page":           wiki.NewStaticPage("Register"),
		"Article":        map[string]string{"Title": "Register"},
		"calloutClasses": "pw-success",
		"calloutMessage": "Successfully registered!",
		"formClasses":    "hidden",
		"Context":        req.Context(),
	}

	// fill form with previously submitted values and display registration errors
	err := a.Users.PostUser(user)
	if err != nil {
		slog.Warn("registration failed", "category", "auth", "action", "register", "username", user.ScreenName, "reason", err.Error(), "ip", req.RemoteAddr)
		render["calloutMessage"] = err.Error()
		render["calloutClasses"] = "pw-error"
		render["formClasses"] = ""
		render["screennameValue"] = user.ScreenName
		render["emailValue"] = user.Email
	} else {
		slog.Info("user registered", "category", "auth", "action", "register", "username", user.ScreenName, "ip", req.RemoteAddr)
	}

	err = a.RenderTemplate(rw, "register.html", "index.html", render)
	check(err)

}

func (a *App) LoginHandler(rw http.ResponseWriter, req *http.Request) {
	render := map[string]interface{}{
		"Page": wiki.NewStaticPage("Login"),
		"Article": map[string]string{
			"Title": "Login",
		},
		"Context": req.Context(),
	}

	// Check if redirected here because login is required
	if req.URL.Query().Get("reason") == "login_required" {
		render["loginRequired"] = true
		render["referrerValue"] = req.URL.Query().Get("referrer")
	} else {
		render["referrerValue"] = req.Referer()
	}

	err := a.RenderTemplate(rw, "login.html", "index.html", render)
	check(err)
}

func (a *App) LoginPostHandler(rw http.ResponseWriter, req *http.Request) {
	user := &wiki.User{}
	user.ScreenName = req.PostFormValue("screenname")
	user.RawPassword = req.PostFormValue("password")
	referrer := req.PostFormValue("referrer")

	err := a.Users.CheckUserPassword(user)

	render := map[string]interface{}{
		"Page":           wiki.NewStaticPage("Login"),
		"Article":        map[string]string{"Title": "Login"},
		"calloutClasses": "pw-success",
		"calloutMessage": "Successfully logged in!",
		"formClasses":    "hidden",
		"Context":        req.Context(),
	}

	if err != nil {
		slog.Warn("login failed", "username", user.ScreenName, "reason", err.Error(), "ip", req.RemoteAddr)
		render["calloutMessage"] = err.Error()
		render["calloutClasses"] = "pw-error"
		render["formClasses"] = ""
		render["screennameValue"] = user.ScreenName
		rw.WriteHeader(http.StatusUnauthorized)
		err = a.RenderTemplate(rw, "login.html", "index.html", render)
		check(err)
		return
	}

	session, err := a.Sessions.GetCookie(req, "periwiki-login")
	if err != nil {
		// GetCookie returns an error when the existing cookie can't be decoded
		// (e.g., signed with a different secret). In this case, it also returns
		// a new valid session we can use. Only fail if we didn't get a session.
		if session == nil {
			a.ErrorHandler(http.StatusInternalServerError, rw, req, err)
			return
		}
		// Log the error but proceed with the new session
		slog.Debug("existing session cookie invalid, creating new session", "error", err)
	}
	session.Options.MaxAge = a.RuntimeConfig.CookieExpiry
	session.Values["username"] = user.ScreenName
	err = session.Save(req, rw)
	if err != nil {
		a.ErrorHandler(http.StatusInternalServerError, rw, req, err)
		return
	}

	slog.Info("user logged in", "category", "auth", "action", "login", "username", user.ScreenName, "ip", req.RemoteAddr)

	if referrer == "" {
		referrer = "/"
	}
	http.Redirect(rw, req, referrer, http.StatusSeeOther)
}

func (a *App) LogoutPostHandler(rw http.ResponseWriter, req *http.Request) {
	session, err := a.Sessions.GetCookie(req, "periwiki-login")
	if err != nil {
		// If we can't decode the cookie, the user is effectively already logged out.
		// Just redirect to home. Only fail if we got a nil session (shouldn't happen).
		if session == nil {
			a.ErrorHandler(http.StatusInternalServerError, rw, req, err)
			return
		}
		slog.Debug("logout with invalid session cookie, redirecting to home", "error", err)
		http.Redirect(rw, req, "/", http.StatusSeeOther)
		return
	}

	// Capture username before session is deleted
	username, _ := session.Values["username"].(string)

	err = a.Sessions.DeleteCookie(req, rw, session)
	if err != nil {
		a.ErrorHandler(http.StatusInternalServerError, rw, req, err)
		return
	}

	slog.Info("user logged out", "category", "auth", "action", "logout", "username", username, "ip", req.RemoteAddr)
	http.Redirect(rw, req, "/", http.StatusSeeOther)
}

func (a *App) HomeHandler(rw http.ResponseWriter, req *http.Request) {
	data := make(map[string]interface{})

	article := &wiki.Article{
		URL: "Home",
		Revision: &wiki.Revision{
			HTML: "Welcome to periwiki! Why don't you check out <a href='/wiki/Test'>Test</a>?",
		},
	}
	data["Page"] = article
	data["Article"] = article
	data["Context"] = req.Context()

	err := a.RenderTemplate(rw, "home.html", "index.html", data)
	check(err)
}

func (a *App) ArticleHandler(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	render := map[string]interface{}{}
	article, err := a.Articles.GetArticle(vars["article"])

	if err != wiki.ErrGenericNotFound && err != nil {
		a.ErrorHandler(http.StatusInternalServerError, rw, req, err)
		return
	}
	user, ok := req.Context().Value(wiki.UserKey).(*wiki.User)
	if !ok || user == nil {
		a.ErrorHandler(http.StatusInternalServerError, rw, req, fmt.Errorf("user context not set"))
		return
	}

	found := article != nil

	if !found {
		article = wiki.NewArticle(vars["article"], "")
		article.Hash = "new"
	}

	if req.Method == "POST" {
		article.Revision = &wiki.Revision{}

		article.Markdown = req.PostFormValue("body")
		article.Creator = user
		a.articlePostHandler(article, rw, req)
		return
	}

	render["Page"] = article
	render["Article"] = article
	render["Context"] = req.Context()

	if !found {
		slog.Debug("article not found", "category", "article", "action", "view", "article", vars["article"])
		rw.WriteHeader(http.StatusNotFound)
		err = a.RenderTemplate(rw, "article_notfound.html", "index.html", render)
		check(err)
		return
	}

	slog.Debug("article viewed", "category", "article", "action", "view", "article", vars["article"])
	err = a.RenderTemplate(rw, "article.html", "index.html", render)
	check(err)
}

// ArticleDispatcher routes article requests based on query parameters.
// URL scheme:
//   - /wiki/{article} - view article (current revision)
//   - /wiki/{article}?revision=N - view specific revision
//   - /wiki/{article}?edit - edit current revision
//   - /wiki/{article}?edit&revision=N - edit/restore revision N
//   - /wiki/{article}?history - view revision history
//   - /wiki/{article}?diff&old=N&new=M - diff between revisions
//   - /wiki/{article}?diff&old=N - diff from N to current
//   - /wiki/{article}?diff&new=M - diff from previous to M
//   - /wiki/{article}?diff - diff between two most recent revisions
//   - /wiki/{article}?rerender - force re-render current revision
//   - /wiki/{article}?rerender&revision=N - force re-render revision N
func (a *App) ArticleDispatcher(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	params := req.URL.Query()

	// Handle POST requests
	if req.Method == "POST" {
		a.handleArticlePost(rw, req, vars["article"])
		return
	}

	// Handle diff requests
	if params.Has("diff") {
		a.handleDiff(rw, req, vars["article"], params)
		return
	}

	// Handle history
	if params.Has("history") {
		a.handleHistory(rw, req, vars["article"])
		return
	}

	// Handle edit
	if params.Has("edit") {
		a.handleEdit(rw, req, vars["article"], params)
		return
	}

	// Handle rerender
	if params.Has("rerender") {
		a.handleRerender(rw, req, vars["article"], params)
		return
	}

	// Default: view article
	a.handleView(rw, req, vars["article"], params)
}

// handleView handles viewing an article or specific revision.
func (a *App) handleView(rw http.ResponseWriter, req *http.Request, articleURL string, params url.Values) {
	// Check if viewing a specific revision
	if revisionStr := params.Get("revision"); revisionStr != "" {
		revisionID, err := strconv.Atoi(revisionStr)
		if err != nil {
			a.ErrorHandler(http.StatusBadRequest, rw, req, err)
			return
		}
		article, err := a.Articles.GetArticleByRevisionID(articleURL, revisionID)
		if err != nil {
			a.ErrorHandler(http.StatusNotFound, rw, req, err)
			return
		}

		// Check if this is an old revision by comparing with current
		templateData := map[string]interface{}{
			"Page":    article,
			"Article": article,
			"Context": req.Context(),
		}
		if current, err := a.Articles.GetArticle(articleURL); err == nil && current.ID != article.ID {
			templateData["IsOldRevision"] = true
			templateData["CurrentRevisionID"] = current.ID
			templateData["CurrentRevisionCreated"] = current.Created
		}

		err = a.RenderTemplate(rw, "article.html", "index.html", templateData)
		check(err)
		return
	}

	// View current revision (delegate to existing handler logic)
	a.ArticleHandler(rw, req)
}

// handleRerender handles re-rendering an article revision.
// Supports ?rerender or ?rerender&revision=N
func (a *App) handleRerender(rw http.ResponseWriter, req *http.Request, articleURL string, params url.Values) {
	user := req.Context().Value(wiki.UserKey).(*wiki.User)

	// Get revision ID (0 means current)
	var revisionID int
	if revisionStr := params.Get("revision"); revisionStr != "" {
		var err error
		revisionID, err = strconv.Atoi(revisionStr)
		if err != nil {
			a.ErrorHandler(http.StatusBadRequest, rw, req, err)
			return
		}
	}

	// Re-render the revision
	if err := a.Articles.RerenderRevision(req.Context(), articleURL, revisionID); err != nil {
		slog.Error("rerender failed", "article", articleURL, "revision", revisionID, "error", err)
		a.ErrorHandler(http.StatusInternalServerError, rw, req, err)
		return
	}

	slog.Info("article rerendered", "article", articleURL, "revision", revisionID, "user", user.ScreenName)

	// Redirect back to the article (with revision if specified)
	redirectURL := "/wiki/" + articleURL
	if revisionID != 0 {
		redirectURL += "?revision=" + strconv.Itoa(revisionID)
	}
	http.Redirect(rw, req, redirectURL, http.StatusSeeOther)
}

// handleHistory handles viewing revision history.
func (a *App) handleHistory(rw http.ResponseWriter, req *http.Request, articleURL string) {
	revisions, err := a.Articles.GetRevisionHistory(articleURL)
	if err != nil {
		a.ErrorHandler(http.StatusNotFound, rw, req, err)
		return
	}

	slog.Debug("article history viewed", "category", "article", "action", "history", "article", articleURL)

	var currentRevisionID int
	if len(revisions) > 0 {
		currentRevisionID = revisions[0].ID
	}

	err = a.RenderTemplate(rw, "article_history.html", "index.html", map[string]interface{}{
		"Page": wiki.NewStaticPage("History of " + articleURL),
		"Article": map[string]interface{}{
			"URL":   articleURL,
			"Title": "History of " + articleURL},
		"Context":           req.Context(),
		"Revisions":         revisions,
		"CurrentRevisionID": currentRevisionID})
	check(err)
}

// handleEdit handles the edit form display.
func (a *App) handleEdit(rw http.ResponseWriter, req *http.Request, articleURL string, params url.Values) {
	// Check if anonymous editing is allowed
	user := req.Context().Value(wiki.UserKey).(*wiki.User)
	if !a.RuntimeConfig.AllowAnonymousEditsGlobal && user.IsAnonymous() {
		loginURL := "/user/login?reason=login_required&referrer=" + url.QueryEscape(req.URL.String())
		http.Redirect(rw, req, loginURL, http.StatusSeeOther)
		return
	}

	var article *wiki.Article
	var err error

	other := make(map[string]interface{})
	other["Preview"] = false

	// Check if editing a specific revision (for restore)
	if revisionStr := params.Get("revision"); revisionStr != "" {
		revisionID, err := strconv.Atoi(revisionStr)
		if err != nil {
			a.ErrorHandler(http.StatusBadRequest, rw, req, err)
			return
		}
		article, err = a.Articles.GetArticleByRevisionID(articleURL, revisionID)
		if err == wiki.ErrRevisionNotFound {
			article = wiki.NewArticle(articleURL, "")
			article.Hash = "new"
		} else if err != nil {
			a.ErrorHandler(http.StatusInternalServerError, rw, req, err)
			return
		}

		// Fetch current revision for conflict detection
		if current, currentErr := a.Articles.GetArticle(articleURL); currentErr == nil && current.ID != article.ID {
			other["IsRestoring"] = true
			other["SourceRevisionID"] = article.ID
			other["CurrentRevisionID"] = current.ID
		}
	} else {
		// Edit current revision
		article, err = a.Articles.GetArticle(articleURL)
		if err == wiki.ErrGenericNotFound {
			article = wiki.NewArticle(articleURL, "")
			article.Hash = "new"
		} else if err != nil {
			a.ErrorHandler(http.StatusInternalServerError, rw, req, err)
			return
		}
	}

	err = a.RenderTemplate(rw, "article_edit.html", "index.html", map[string]interface{}{
		"Page":    article,
		"Article": article,
		"Context": req.Context(),
		"Other":   other})
	check(err)
}

// handleDiff handles diff view between revisions.
// Smart defaults:
//   - ?diff&old=N&new=M - explicit diff between N and M
//   - ?diff&old=N - diff from N to current
//   - ?diff&new=M - diff from (M-1) to M (previous to M)
//   - ?diff - diff between two most recent revisions
func (a *App) handleDiff(rw http.ResponseWriter, req *http.Request, articleURL string, params url.Values) {
	var oldArticle, newArticle *wiki.Article
	var err error

	oldStr := params.Get("old")
	newStr := params.Get("new")

	// Determine the "new" revision
	if newStr != "" {
		newID, err := strconv.Atoi(newStr)
		if err != nil {
			a.ErrorHandler(http.StatusBadRequest, rw, req, err)
			return
		}
		newArticle, err = a.Articles.GetArticleByRevisionID(articleURL, newID)
		if err != nil {
			a.ErrorHandler(http.StatusNotFound, rw, req, err)
			return
		}
	} else {
		// Default to current revision
		newArticle, err = a.Articles.GetArticle(articleURL)
		if err != nil {
			a.ErrorHandler(http.StatusNotFound, rw, req, err)
			return
		}
	}

	// Determine the "old" revision
	if oldStr != "" {
		oldID, err := strconv.Atoi(oldStr)
		if err != nil {
			a.ErrorHandler(http.StatusBadRequest, rw, req, err)
			return
		}
		oldArticle, err = a.Articles.GetArticleByRevisionID(articleURL, oldID)
		if err != nil {
			a.ErrorHandler(http.StatusNotFound, rw, req, err)
			return
		}
	} else {
		// Default to previous revision of the "new" revision
		if newArticle.PreviousID > 0 {
			oldArticle, err = a.Articles.GetArticleByRevisionID(articleURL, newArticle.PreviousID)
			if err != nil {
				a.ErrorHandler(http.StatusNotFound, rw, req, err)
				return
			}
		} else {
			// No previous revision, diff against empty
			oldArticle = wiki.NewArticle(articleURL, "")
		}
	}

	// Generate diff
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(oldArticle.Markdown, newArticle.Markdown, true)
	diffs = dmp.DiffCleanupSemantic(diffs)

	var buff bytes.Buffer
	for _, diff := range diffs {
		text := html.EscapeString(diff.Text)
		switch diff.Type {
		case diffmatchpatch.DiffInsert:
			_, _ = buff.WriteString("<ins style=\"background:#e6ffe6;\">")
			_, _ = buff.WriteString(text)
			_, _ = buff.WriteString("</ins>")
		case diffmatchpatch.DiffDelete:
			_, _ = buff.WriteString("<del style=\"background:#ffe6e6;\">")
			_, _ = buff.WriteString(text)
			_, _ = buff.WriteString("</del>")
		case diffmatchpatch.DiffEqual:
			_, _ = buff.WriteString("<span>")
			_, _ = buff.WriteString(text)
			_, _ = buff.WriteString("</span>")
		}
	}

	// Get current revision ID for template comparisons
	current, _ := a.Articles.GetArticle(articleURL)
	var currentRevisionID int
	if current != nil {
		currentRevisionID = current.ID
	}

	err = a.RenderTemplate(rw, "diff.html", "index.html", map[string]interface{}{
		"Page":              newArticle,
		"Article":           oldArticle,
		"NewRevision":       newArticle,
		"DiffString":        template.HTML(buff.String()),
		"Context":           req.Context(),
		"CurrentRevisionID": currentRevisionID,
	})
	check(err)
}

// handleArticlePost handles POST requests for article editing.
func (a *App) handleArticlePost(rw http.ResponseWriter, req *http.Request, articleURL string) {
	article := &wiki.Article{}
	article.URL = articleURL
	article.Revision = &wiki.Revision{}

	article.Markdown = req.PostFormValue("body")
	article.Comment = req.PostFormValue("comment")

	user, ok := req.Context().Value(wiki.UserKey).(*wiki.User)
	if !ok || user == nil {
		a.ErrorHandler(http.StatusInternalServerError, rw, req, fmt.Errorf("user context not set"))
		return
	}

	// Check if anonymous editing is allowed
	if !a.RuntimeConfig.AllowAnonymousEditsGlobal && user.IsAnonymous() {
		a.ErrorHandler(http.StatusForbidden, rw, req, fmt.Errorf("anonymous editing is disabled"))
		return
	}

	article.Creator = user

	// Get previous_id from form body
	previousIDStr := req.PostFormValue("previous_id")
	if previousIDStr == "" {
		// For new articles, previous_id might be 0 or not set
		article.PreviousID = 0
	} else {
		previousID, err := strconv.Atoi(previousIDStr)
		if err != nil {
			a.ErrorHandler(http.StatusBadRequest, rw, req, err)
			return
		}
		article.PreviousID = previousID
	}

	if req.PostFormValue("action") == "preview" {
		article.ID = article.PreviousID
		a.articlePreviewHandler(article, rw, req)
		return
	}
	a.articlePostHandler(article, rw, req)
}

func (a *App) articlePreviewHandler(article *wiki.Article, rw http.ResponseWriter, req *http.Request) {
	html, err := a.Rendering.PreviewMarkdown(article.Markdown)
	if err != nil {
		a.ErrorHandler(http.StatusInternalServerError, rw, req, err)
		return
	}
	article.HTML = html

	other := make(map[string]interface{})
	other["Preview"] = true

	err = a.RenderTemplate(rw, "article_edit.html", "index.html",
		map[string]interface{}{
			"Page":    article,
			"Article": article,
			"Context": req.Context(),
			"Other":   other})
	check(err)
}
func (a *App) articlePostHandler(article *wiki.Article, rw http.ResponseWriter, req *http.Request) {
	err := a.Articles.PostArticle(article)
	if err != nil {
		username := "anonymous"
		if article.Creator != nil {
			username = article.Creator.ScreenName
		}
		slog.Warn("article save failed", "category", "article", "action", "save", "article", article.URL, "username", username, "reason", err.Error())
		if err == wiki.ErrRevisionAlreadyExists {
			a.ErrorHandler(http.StatusConflict, rw, req, err)
			return
		}
		a.ErrorHandler(http.StatusBadRequest, rw, req, err)
		return
	}
	username := "anonymous"
	if article.Creator != nil {
		username = article.Creator.ScreenName
	}
	slog.Info("article saved", "category", "article", "action", "save", "article", article.URL, "username", username, "revision", article.ID)
	http.Redirect(rw, req, "/wiki/"+article.URL, http.StatusSeeOther) // To prevent "browser must resend..."
}

func (a *App) ErrorHandler(responseCode int, rw http.ResponseWriter, req *http.Request, errors ...error) {
	rw.WriteHeader(responseCode)
	errorTitle := fmt.Sprintf("%d: %s", responseCode, http.StatusText(responseCode))
	err := a.RenderTemplate(rw, "error.html", "index.html",
		map[string]interface{}{
			"Page":    wiki.NewStaticPage(errorTitle),
			"Article": &wiki.Article{URL: errorTitle, Revision: &wiki.Revision{}},
			"Context": req.Context(),
			"Error": map[string]interface{}{
				"Code":       responseCode,
				"CodeString": http.StatusText(responseCode),
				"Errors":     errors,
			}})
	if err != nil {
		slog.Error("failed to render error page", "error", err)
		panic(err)
	}
}

func (a *App) SpecialPageHandler(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	pageName := vars["page"]

	handler, ok := a.SpecialPages.Get(pageName)
	if !ok {
		a.ErrorHandler(http.StatusNotFound, rw, req,
			fmt.Errorf("special page '%s' does not exist", pageName))
		return
	}

	handler.Handle(rw, req)
}
