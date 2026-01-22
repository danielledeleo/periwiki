package main

import (
	"bytes"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/danielledeleo/periwiki/special"
	"github.com/danielledeleo/periwiki/templater"
	"github.com/danielledeleo/periwiki/wiki"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/gorilla/mux"

	"github.com/sergi/go-diff/diffmatchpatch"
)

type app struct {
	*templater.Templater
	*wiki.WikiModel
	specialPages *special.Registry
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
	app := Setup()

	router := mux.NewRouter().StrictSlash(true)

	router.Use(app.SessionMiddleware)

	fs := http.FileServer(http.Dir("./static"))
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fs))
	router.HandleFunc("/", app.homeHandler).Methods("GET")

	router.HandleFunc("/wiki/Special:{page}", app.specialPageHandler).Methods("GET")

	router.HandleFunc("/wiki/{article}", app.articleHandler).Methods("GET")
	router.HandleFunc("/wiki/{article}/history", app.articleHistoryHandler).Methods("GET")
	router.HandleFunc("/wiki/{article}/r/{revision}", app.revisionHandler).Methods("GET")
	router.HandleFunc("/wiki/{article}/r/{revision}", app.revisionPostHandler).Methods("POST")
	router.HandleFunc("/wiki/{article}/r/{revision}/edit", app.revisionEditHandler).Methods("GET")
	router.HandleFunc("/wiki/{article}/diff/{original}/{new}", app.diffHandler).Methods("GET")

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

	slog.Info("server starting", "url", "http://"+app.Config.Host)
	err := http.ListenAndServe(app.Config.Host, handler)

	if err != nil {
		slog.Error("server failed to start", "error", err)
		os.Exit(1)
	}
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
	err := a.PostUser(user)
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
	err := a.RenderTemplate(rw, "login.html", "index.html", map[string]interface{}{
		"Article": map[string]string{
			"Title":         "Login",
			"referrerValue": req.Referer(),
		},
		"Context": req.Context(),
	})
	check(err)
}

func (a *app) loginPostHander(rw http.ResponseWriter, req *http.Request) {
	user := &wiki.User{}
	user.ScreenName = req.PostFormValue("screenname")
	user.RawPassword = req.PostFormValue("password")
	referrer := req.PostFormValue("referrer")

	err := a.CheckUserPassword(user)

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

	session, err := a.GetCookie(req, "periwiki-login")
	if err != nil {
		a.errorHandler(http.StatusInternalServerError, rw, req, err)
		return
	}
	session.Options.MaxAge = a.CookieExpiry
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
	session, err := a.GetCookie(req, "periwiki-login")
	if err != nil {
		a.errorHandler(http.StatusInternalServerError, rw, req, err)
		return
	}

	// Capture username before session is deleted
	username, _ := session.Values["username"].(string)

	err = a.DeleteCookie(req, rw, session)
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
			HTML:  "Welcome to periwiki! Why don't you check out <a href='/wiki/test'>Test</a>?",
		},
	}
	data["Context"] = req.Context()

	err := a.RenderTemplate(rw, "home.html", "index.html", data)
	check(err)
}

func (a *app) articleHandler(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	render := map[string]interface{}{}
	article, err := a.GetArticle(vars["article"])

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

func (a *app) articleHistoryHandler(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	url := vars["article"]

	revisions, err := a.GetRevisionHistory(url)
	if err != nil {
		a.errorHandler(http.StatusNotFound, rw, req, err)
		return
	}

	slog.Debug("article history viewed", "category", "article", "action", "history", "article", url)
	err = a.RenderTemplate(rw, "article_history.html", "index.html", map[string]interface{}{
		"Article": map[string]interface{}{
			"URL":   url,
			"Title": "History of " + url},
		"Context":   req.Context(),
		"Revisions": revisions})
	check(err)
}

func (a *app) revisionHandler(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	revisionID, err := strconv.Atoi(vars["revision"])
	if err != nil {
		a.errorHandler(http.StatusBadRequest, rw, req, err)
		return
	}
	article, err := a.GetArticleByRevisionID(vars["article"], revisionID)
	if err != nil {
		a.errorHandler(http.StatusNotFound, rw, req, err)
		return
	}
	err = a.RenderTemplate(rw, "article.html", "index.html", map[string]interface{}{
		"Article": article,
		"Context": req.Context(),
	})
	check(err)
}

func (a *app) revisionEditHandler(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	revisionID, err := strconv.Atoi(vars["revision"])
	if err != nil {
		a.errorHandler(http.StatusBadRequest, rw, req, err)
		return
	}
	article, err := a.GetArticleByRevisionID(vars["article"], revisionID)
	if err == wiki.ErrRevisionNotFound {
		article = wiki.NewArticle(vars["article"], cases.Title(language.AmericanEnglish).String(vars["article"]), "")
		article.Hash = "new"
	} else if err != nil {
		a.errorHandler(http.StatusInternalServerError, rw, req, err)
		return
	}

	other := make(map[string]interface{})
	other["Preview"] = false

	err = a.RenderTemplate(rw, "article_edit.html", "index.html", map[string]interface{}{
		"Article": article,
		"Context": req.Context(),
		"Other":   other})
	check(err)
}

func (a *app) revisionPostHandler(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	article := &wiki.Article{}

	article.URL = vars["article"]
	article.Revision = &wiki.Revision{}

	article.Title = req.PostFormValue("title")
	article.Markdown = req.PostFormValue("body")
	article.Comment = req.PostFormValue("comment")

	user, ok := req.Context().Value(wiki.UserKey).(*wiki.User)
	if !ok || user == nil {
		a.errorHandler(http.StatusInternalServerError, rw, req, fmt.Errorf("user context not set"))
		return
	}
	article.Creator = user
	previousID, err := strconv.Atoi(vars["revision"])
	if err != nil {
		a.errorHandler(http.StatusBadRequest, rw, req, err)
		return
	}
	article.PreviousID = previousID

	if req.PostFormValue("action") == "preview" {
		article.ID = previousID
		a.articlePreviewHandler(article, rw, req)
		return
	}
	a.articlePostHandler(article, rw, req)
}

func (a *app) articlePreviewHandler(article *wiki.Article, rw http.ResponseWriter, req *http.Request) {
	html, err := a.PreviewMarkdown(article.Markdown)
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
	err := a.PostArticle(article)
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

func (a *app) diffHandler(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	originalID, err := strconv.Atoi(vars["original"])
	if err != nil {
		a.errorHandler(http.StatusBadRequest, rw, req, err)
		return
	}
	newID, err := strconv.Atoi(vars["new"])
	if err != nil {
		a.errorHandler(http.StatusBadRequest, rw, req, err)
		return
	}

	orginal, err := a.GetArticleByRevisionID(vars["article"], originalID)
	if err != nil {
		a.errorHandler(http.StatusNotFound, rw, req, err)
		return
	}

	new, err := a.GetArticleByRevisionID(vars["article"], newID)
	if err != nil {
		a.errorHandler(http.StatusNotFound, rw, req, err)
		return
	}

	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(orginal.Markdown, new.Markdown, false)

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
	pretty := buff.String()

	slog.Debug("diff viewed", "category", "article", "action", "diff", "article", vars["article"], "from", originalID, "to", newID)
	err = a.RenderTemplate(rw, "diff.html", "index.html", map[string]interface{}{
		"Article": orginal,
		"Context": req.Context(),
		"Other": map[string]interface{}{
			"DiffString": pretty,
		}})

	if err != nil {
		slog.Error("failed to render diff template", "error", err)
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
