package main

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/danielledeleo/periwiki/special"
	"github.com/danielledeleo/periwiki/templater"
	"github.com/danielledeleo/periwiki/wiki"
	"github.com/danielledeleo/periwiki/wiki/service"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/gorilla/mux"

	"github.com/sergi/go-diff/diffmatchpatch"
)

type app struct {
	*templater.Templater
	articles     service.ArticleService
	users        service.UserService
	sessions     service.SessionService
	rendering    service.RenderingService
	preferences  service.PreferenceService
	specialPages *special.Registry
	config       *wiki.Config
}

// responseWriter wraps http.ResponseWriter to capture the status code
type responseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.size += n
	return n, err
}

// slogLoggingMiddleware logs HTTP requests using slog
func slogLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.status,
			"size", wrapped.size,
			"duration", time.Since(start),
			"remote", r.RemoteAddr,
		)
	})
}

func main() {
	app, renderQueue := Setup()

	router := mux.NewRouter().StrictSlash(true)

	router.Use(app.SessionMiddleware)

	// Routes are documented in docs/urls.md — update it when adding or changing routes.
	fs := http.FileServer(http.Dir("./static"))
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fs))
	router.HandleFunc("/", app.homeHandler).Methods("GET")

	router.HandleFunc("/wiki/Special:{page}", app.specialPageHandler).Methods("GET")

	router.HandleFunc("/wiki/{article}", app.articleDispatcher).Methods("GET", "POST")

	router.HandleFunc("/user/register", app.registerHandler).Methods("GET")
	router.HandleFunc("/user/register", app.registerPostHandler).Methods("POST")
	router.HandleFunc("/user/login", app.loginHander).Methods("GET")
	router.HandleFunc("/user/login", app.loginPostHander).Methods("POST")
	router.HandleFunc("/user/logout", app.logoutPostHander).Methods("POST")

	manageRouter := mux.NewRouter().PathPrefix("/manage").Subrouter()
	manageRouter.HandleFunc("/{page}", func(rw http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		fmt.Fprintln(rw, vars["page"])
	})
	router.Handle("/manage/{page}", manageRouter)

	handler := slogLoggingMiddleware(router)

	srv := &http.Server{
		Addr:    app.config.Host,
		Handler: handler,
	}

	// Start server in goroutine
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	slog.Info("server starting", "url", "http://"+app.config.Host)

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown HTTP server first (stop accepting new requests)
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}

	// Shutdown render queue (wait for in-flight jobs)
	slog.Info("shutting down render queue...")
	if err := renderQueue.Shutdown(ctx); err != nil {
		slog.Error("render queue shutdown error", "error", err)
	}

	slog.Info("server stopped")
}

func (a *app) registerHandler(rw http.ResponseWriter, req *http.Request) {
	err := a.RenderTemplate(rw, "register.html", "index.html",
		map[string]interface{}{
			"Article": map[string]string{"Title": "Register"},
			"Context": req.Context()})
	check(err)
}

func (a *app) registerPostHandler(rw http.ResponseWriter, req *http.Request) {
	user := &wiki.User{}

	user.Email = req.PostFormValue("email")
	user.ScreenName = req.PostFormValue("screenname")
	user.RawPassword = req.PostFormValue("password")

	render := map[string]interface{}{
		"Article":        map[string]string{"Title": "Register"},
		"calloutClasses": "pw-success",
		"calloutMessage": "Successfully registered!",
		"formClasses":    "hidden",
		"Context":        req.Context(),
	}

	// fill form with previously submitted values and display registration errors
	err := a.users.PostUser(user)
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

func (a *app) loginHander(rw http.ResponseWriter, req *http.Request) {
	render := map[string]interface{}{
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

func (a *app) loginPostHander(rw http.ResponseWriter, req *http.Request) {
	user := &wiki.User{}
	user.ScreenName = req.PostFormValue("screenname")
	user.RawPassword = req.PostFormValue("password")
	referrer := req.PostFormValue("referrer")

	err := a.users.CheckUserPassword(user)

	render := map[string]interface{}{
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

	session, err := a.sessions.GetCookie(req, "periwiki-login")
	if err != nil {
		a.errorHandler(http.StatusInternalServerError, rw, req, err)
		return
	}
	session.Options.MaxAge = a.config.CookieExpiry
	session.Values["username"] = user.ScreenName
	err = session.Save(req, rw)
	if err != nil {
		a.errorHandler(http.StatusInternalServerError, rw, req, err)
		return
	}

	slog.Info("user logged in", "category", "auth", "action", "login", "username", user.ScreenName, "ip", req.RemoteAddr)

	if referrer == "" {
		referrer = "/"
	}
	http.Redirect(rw, req, referrer, http.StatusSeeOther)
}

func (a *app) logoutPostHander(rw http.ResponseWriter, req *http.Request) {
	session, err := a.sessions.GetCookie(req, "periwiki-login")
	if err != nil {
		a.errorHandler(http.StatusInternalServerError, rw, req, err)
		return
	}

	// Capture username before session is deleted
	username, _ := session.Values["username"].(string)

	err = a.sessions.DeleteCookie(req, rw, session)
	if err != nil {
		a.errorHandler(http.StatusInternalServerError, rw, req, err)
		return
	}

	slog.Info("user logged out", "category", "auth", "action", "logout", "username", username, "ip", req.RemoteAddr)
	http.Redirect(rw, req, "/", http.StatusSeeOther)
}

func (a *app) homeHandler(rw http.ResponseWriter, req *http.Request) {
	data := make(map[string]interface{})

	data["Article"] = &wiki.Article{
		Revision: &wiki.Revision{
			Title: "Home",
			HTML:  "Welcome to periwiki! Why don't you check out <a href='/wiki/Test'>Test</a>?",
		},
	}
	data["Context"] = req.Context()

	err := a.RenderTemplate(rw, "home.html", "index.html", data)
	check(err)
}

func (a *app) articleHandler(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	render := map[string]interface{}{}
	article, err := a.articles.GetArticle(vars["article"])

	if err != wiki.ErrGenericNotFound && err != nil {
		a.errorHandler(http.StatusInternalServerError, rw, req, err)
		return
	}
	user, ok := req.Context().Value(wiki.UserKey).(*wiki.User)
	if !ok || user == nil {
		a.errorHandler(http.StatusInternalServerError, rw, req, fmt.Errorf("user context not set"))
		return
	}

	found := article != nil

	if !found {
		article = wiki.NewArticle(vars["article"], cases.Title(language.AmericanEnglish).String(vars["article"]), "")
		article.Hash = "new"
	}

	if req.Method == "POST" {
		article.Revision = &wiki.Revision{}

		article.Title = req.PostFormValue("title")
		article.Markdown = req.PostFormValue("body")
		article.Creator = user
		a.articlePostHandler(article, rw, req)
		return
	}

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

// articleDispatcher routes article requests based on query parameters.
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
//   - /wiki/{article}?rerender - force re-render current revision (auth required)
//   - /wiki/{article}?rerender&revision=N - force re-render revision N (auth required)
func (a *app) articleDispatcher(rw http.ResponseWriter, req *http.Request) {
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
func (a *app) handleView(rw http.ResponseWriter, req *http.Request, articleURL string, params url.Values) {
	// Check if viewing a specific revision
	if revisionStr := params.Get("revision"); revisionStr != "" {
		revisionID, err := strconv.Atoi(revisionStr)
		if err != nil {
			a.errorHandler(http.StatusBadRequest, rw, req, err)
			return
		}
		article, err := a.articles.GetArticleByRevisionID(articleURL, revisionID)
		if err != nil {
			a.errorHandler(http.StatusNotFound, rw, req, err)
			return
		}
		err = a.RenderTemplate(rw, "article.html", "index.html", map[string]interface{}{
			"Article": article,
			"Context": req.Context(),
		})
		check(err)
		return
	}

	// View current revision (delegate to existing handler logic)
	a.articleHandler(rw, req)
}

// handleRerender handles re-rendering an article revision.
// Requires authentication. Supports ?rerender or ?rerender&revision=N
func (a *app) handleRerender(rw http.ResponseWriter, req *http.Request, articleURL string, params url.Values) {
	user := req.Context().Value(wiki.UserKey).(*wiki.User)

	// Require authenticated user (not anonymous)
	if user == nil || user.ID == 0 {
		http.Redirect(rw, req, "/user/login?reason=login_required&referrer="+url.QueryEscape(req.URL.String()), http.StatusSeeOther)
		return
	}

	// Get revision ID (0 means current)
	var revisionID int
	if revisionStr := params.Get("revision"); revisionStr != "" {
		var err error
		revisionID, err = strconv.Atoi(revisionStr)
		if err != nil {
			a.errorHandler(http.StatusBadRequest, rw, req, err)
			return
		}
	}

	// Re-render the revision
	if err := a.articles.RerenderRevision(req.Context(), articleURL, revisionID); err != nil {
		slog.Error("rerender failed", "article", articleURL, "revision", revisionID, "error", err)
		a.errorHandler(http.StatusInternalServerError, rw, req, err)
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
func (a *app) handleHistory(rw http.ResponseWriter, req *http.Request, articleURL string) {
	revisions, err := a.articles.GetRevisionHistory(articleURL)
	if err != nil {
		a.errorHandler(http.StatusNotFound, rw, req, err)
		return
	}

	slog.Debug("article history viewed", "category", "article", "action", "history", "article", articleURL)

	var currentRevisionID int
	if len(revisions) > 0 {
		currentRevisionID = revisions[0].ID
	}

	err = a.RenderTemplate(rw, "article_history.html", "index.html", map[string]interface{}{
		"Article": map[string]interface{}{
			"URL":   articleURL,
			"Title": "History of " + articleURL},
		"Context":           req.Context(),
		"Revisions":         revisions,
		"CurrentRevisionID": currentRevisionID})
	check(err)
}

// handleEdit handles the edit form display.
func (a *app) handleEdit(rw http.ResponseWriter, req *http.Request, articleURL string, params url.Values) {
	// Check if anonymous editing is allowed
	user := req.Context().Value(wiki.UserKey).(*wiki.User)
	if !a.config.AllowAnonymousEditsGlobal && user.IsAnonymous() {
		loginURL := "/user/login?reason=login_required&referrer=" + url.QueryEscape(req.URL.String())
		http.Redirect(rw, req, loginURL, http.StatusSeeOther)
		return
	}

	var article *wiki.Article
	var err error

	// Check if editing a specific revision (for restore)
	if revisionStr := params.Get("revision"); revisionStr != "" {
		revisionID, err := strconv.Atoi(revisionStr)
		if err != nil {
			a.errorHandler(http.StatusBadRequest, rw, req, err)
			return
		}
		article, err = a.articles.GetArticleByRevisionID(articleURL, revisionID)
		if err == wiki.ErrRevisionNotFound {
			article = wiki.NewArticle(articleURL, cases.Title(language.AmericanEnglish).String(articleURL), "")
			article.Hash = "new"
		} else if err != nil {
			a.errorHandler(http.StatusInternalServerError, rw, req, err)
			return
		}
	} else {
		// Edit current revision
		article, err = a.articles.GetArticle(articleURL)
		if err == wiki.ErrGenericNotFound {
			article = wiki.NewArticle(articleURL, cases.Title(language.AmericanEnglish).String(articleURL), "")
			article.Hash = "new"
		} else if err != nil {
			a.errorHandler(http.StatusInternalServerError, rw, req, err)
			return
		}
	}

	other := make(map[string]interface{})
	other["Preview"] = false

	err = a.RenderTemplate(rw, "article_edit.html", "index.html", map[string]interface{}{
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
func (a *app) handleDiff(rw http.ResponseWriter, req *http.Request, articleURL string, params url.Values) {
	var oldArticle, newArticle *wiki.Article
	var err error

	oldStr := params.Get("old")
	newStr := params.Get("new")

	// Determine the "new" revision
	if newStr != "" {
		newID, err := strconv.Atoi(newStr)
		if err != nil {
			a.errorHandler(http.StatusBadRequest, rw, req, err)
			return
		}
		newArticle, err = a.articles.GetArticleByRevisionID(articleURL, newID)
		if err != nil {
			a.errorHandler(http.StatusNotFound, rw, req, err)
			return
		}
	} else {
		// Default to current revision
		newArticle, err = a.articles.GetArticle(articleURL)
		if err != nil {
			a.errorHandler(http.StatusNotFound, rw, req, err)
			return
		}
	}

	// Determine the "old" revision
	if oldStr != "" {
		oldID, err := strconv.Atoi(oldStr)
		if err != nil {
			a.errorHandler(http.StatusBadRequest, rw, req, err)
			return
		}
		oldArticle, err = a.articles.GetArticleByRevisionID(articleURL, oldID)
		if err != nil {
			a.errorHandler(http.StatusNotFound, rw, req, err)
			return
		}
	} else {
		// Default to previous revision of the "new" revision
		if newArticle.PreviousID > 0 {
			oldArticle, err = a.articles.GetArticleByRevisionID(articleURL, newArticle.PreviousID)
			if err != nil {
				a.errorHandler(http.StatusNotFound, rw, req, err)
				return
			}
		} else {
			// No previous revision, diff against empty
			oldArticle = wiki.NewArticle(articleURL, "", "")
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
	current, _ := a.articles.GetArticle(articleURL)
	var currentRevisionID int
	if current != nil {
		currentRevisionID = current.ID
	}

	err = a.RenderTemplate(rw, "diff.html", "index.html", map[string]interface{}{
		"Article":           oldArticle,
		"NewRevision":       newArticle,
		"DiffString":        template.HTML(buff.String()),
		"Context":           req.Context(),
		"CurrentRevisionID": currentRevisionID,
	})
	check(err)
}

// handleArticlePost handles POST requests for article editing.
func (a *app) handleArticlePost(rw http.ResponseWriter, req *http.Request, articleURL string) {
	article := &wiki.Article{}
	article.URL = articleURL
	article.Revision = &wiki.Revision{}

	article.Title = req.PostFormValue("title")
	article.Markdown = req.PostFormValue("body")
	article.Comment = req.PostFormValue("comment")

	user, ok := req.Context().Value(wiki.UserKey).(*wiki.User)
	if !ok || user == nil {
		a.errorHandler(http.StatusInternalServerError, rw, req, fmt.Errorf("user context not set"))
		return
	}

	// Check if anonymous editing is allowed
	if !a.config.AllowAnonymousEditsGlobal && user.IsAnonymous() {
		a.errorHandler(http.StatusForbidden, rw, req, fmt.Errorf("anonymous editing is disabled"))
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
			a.errorHandler(http.StatusBadRequest, rw, req, err)
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

func (a *app) articlePreviewHandler(article *wiki.Article, rw http.ResponseWriter, req *http.Request) {
	html, err := a.rendering.PreviewMarkdown(article.Markdown)
	if err != nil {
		a.errorHandler(http.StatusInternalServerError, rw, req, err)
		return
	}
	article.HTML = html

	other := make(map[string]interface{})
	other["Preview"] = true

	err = a.RenderTemplate(rw, "article_edit.html", "index.html",
		map[string]interface{}{
			"Article": article,
			"Context": req.Context(),
			"Other":   other})
	check(err)
}
func (a *app) articlePostHandler(article *wiki.Article, rw http.ResponseWriter, req *http.Request) {
	err := a.articles.PostArticle(article)
	if err != nil {
		username := "anonymous"
		if article.Creator != nil {
			username = article.Creator.ScreenName
		}
		slog.Warn("article save failed", "category", "article", "action", "save", "article", article.URL, "username", username, "reason", err.Error())
		if err == wiki.ErrRevisionAlreadyExists {
			a.errorHandler(http.StatusConflict, rw, req, err)
			return
		}
		a.errorHandler(http.StatusBadRequest, rw, req, err)
		return
	}
	username := "anonymous"
	if article.Creator != nil {
		username = article.Creator.ScreenName
	}
	slog.Info("article saved", "category", "article", "action", "save", "article", article.URL, "username", username, "revision", article.ID)
	http.Redirect(rw, req, "/wiki/"+article.URL, http.StatusSeeOther) // To prevent "browser must resend..."
}

func check(err error) {
	if err != nil {
		slog.Error("unexpected error", "error", err)
	}
}

func (a *app) errorHandler(responseCode int, rw http.ResponseWriter, req *http.Request, errors ...error) {
	rw.WriteHeader(responseCode)
	err := a.RenderTemplate(rw, "error.html", "index.html",
		map[string]interface{}{
			"Article": &wiki.Article{Revision: &wiki.Revision{Title: fmt.Sprintf("%d: %s", responseCode, http.StatusText(responseCode))}},
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

func (a *app) specialPageHandler(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	pageName := vars["page"]

	handler, ok := a.specialPages.Get(pageName)
	if !ok {
		a.errorHandler(http.StatusNotFound, rw, req,
			fmt.Errorf("special page '%s' does not exist", pageName))
		return
	}

	handler.Handle(rw, req)
}
