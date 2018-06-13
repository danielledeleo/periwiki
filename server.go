package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/jagger27/iwikii/db"
	"github.com/jagger27/iwikii/model"
	"github.com/jagger27/iwikii/templater"
	"github.com/microcosm-cc/bluemonday"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	"github.com/sergi/go-diff/diffmatchpatch"
)

type app struct {
	*templater.Templater
	*model.WikiModel
}

func main() {
	router := mux.NewRouter()
	bm := bluemonday.UGCPolicy()
	bm.AllowAttrs("class").Matching(regexp.MustCompile("^sourceCode(| [a-zA-Z0-9]+)(| lineNumbers)$")).
		OnElements("pre", "code")

	bm.AllowAttrs("data-line-number", "class").Matching(regexp.MustCompile("^[0-9]+$")).OnElements("a")
	bm.AllowAttrs("style").OnElements("ins", "del")
	t := templater.New()
	t.Load("templates/layouts/*.html", "templates/*.html")
	fs := http.FileServer(http.Dir("./static"))

	cookieKey := os.Getenv("COOKIE_SECRET")
	if cookieKey == "" {
		log.Fatal("COOKIE_SECRET environment variable not set!")
	}
	database, err := db.Init(db.SqliteConfig{DatabaseFile: "iwikii.db", CookieSecretKey: cookieKey})
	// database, err := db.InitPg(db.PgConfig{"", cookieKey})
	check(err)
	model := model.New(database, &model.Config{MinimumPasswordLength: 8}, bm)

	app := app{t, model}

	router.Use(app.SessionMiddleware)

	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fs))
	router.HandleFunc("/", app.homeHandler).Methods("GET")

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

	logger := handlers.LoggingHandler(os.Stdout, router)
	http.ListenAndServe(":8080", logger)
}

func (a *app) registerHandler(rw http.ResponseWriter, req *http.Request) {
	err := a.RenderTemplate(rw, "register.html", "index.html",
		map[string]interface{}{
			"Article": map[string]string{"Title": "Register"},
			"Context": req.Context()})
	check(err)
}
func (a *app) registerPostHandler(rw http.ResponseWriter, req *http.Request) {
	user := &model.User{}

	user.Email = req.PostFormValue("email")
	user.ScreenName = req.PostFormValue("screenname")
	user.RawPassword = req.PostFormValue("password")

	render := map[string]interface{}{
		"Title":          "Register",
		"calloutClasses": "iw-success",
		"calloutMessage": "Successfully registered!",
		"formClasses":    "hidden",
		"Context":        req.Context(),
	}

	// fill form with previously submitted values and display registration errors
	err := a.PostUser(user)
	if err != nil {
		render["calloutMessage"] = err.Error()
		render["calloutClasses"] = "iw-error"
		render["formClasses"] = ""
		render["screennameValue"] = user.ScreenName
		render["emailValue"] = user.Email
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
	user := &model.User{}
	user.ScreenName = req.PostFormValue("screenname")
	user.RawPassword = req.PostFormValue("password")
	referrer := req.PostFormValue("referrer")

	err := a.CheckUserPassword(user)

	render := map[string]interface{}{
		"Title":          "Login",
		"calloutClasses": "iw-success",
		"calloutMessage": "Successfully logged in!",
		"formClasses":    "hidden",
		"Context":        req.Context(),
	}

	if err != nil {
		render["calloutMessage"] = err.Error()
		render["calloutClasses"] = "iw-error"
		render["formClasses"] = ""
		render["screennameValue"] = user.ScreenName
		err = a.RenderTemplate(rw, "login.html", "index.html", map[string]interface{}{"Article": render})
		check(err)
		return
	}

	session, err := a.GetCookie(req, "iwikii-login")
	if err != nil {
		a.errorHandler(http.StatusInternalServerError, rw, req, err)
	}
	session.Options.MaxAge = 86400 * 7 // a week
	session.Values["username"] = user.ScreenName
	err = session.Save(req, rw)
	if err != nil {
		a.errorHandler(http.StatusInternalServerError, rw, req, err)
	}

	if referrer == "" {
		referrer = "/"
	}
	http.Redirect(rw, req, referrer, http.StatusSeeOther)
}

func (a *app) logoutPostHander(rw http.ResponseWriter, req *http.Request) {
	session, err := a.GetCookie(req, "iwikii-login")
	if err != nil {
		a.errorHandler(http.StatusInternalServerError, rw, req, err)
	}

	err = a.DeleteCookie(req, rw, session)
	if err != nil {
		a.errorHandler(http.StatusInternalServerError, rw, req, err)
	}

	http.Redirect(rw, req, "/", http.StatusSeeOther)
}

func (a *app) homeHandler(rw http.ResponseWriter, req *http.Request) {
	data := make(map[string]interface{})

	data["Article"] = &model.Article{
		Revision: &model.Revision{
			Title: "Home",
			HTML:  "Welcome to iwikii! Why don't you check out <a href='/wiki/test'>Test</a>?",
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

	if err != nil {
		a.errorHandler(http.StatusInternalServerError, rw, req, err)
	}
	user := req.Context().Value(model.UserKey)

	found := article != nil

	if !found {
		article = model.NewArticle(vars["article"], strings.Title(vars["article"]), "")
		article.Hash = "new"
		check(err)
	}

	if req.Method == "POST" {
		article.Revision = &model.Revision{}

		article.Title = req.PostFormValue("title")
		article.Markdown = req.PostFormValue("body")
		article.Creator = user.(*model.User)
		a.articlePostHandler(article, rw, req)
		return
	}

	render["Article"] = article
	render["Context"] = req.Context()

	if !found {
		rw.WriteHeader(http.StatusNotFound)
		err = a.RenderTemplate(rw, "article_notfound.html", "index.html", render)
		check(err)
		return
	}

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
	if err == model.ErrRevisionNotFound {
		article = model.NewArticle(vars["article"], strings.Title(vars["article"]), "")
		article.Hash = "new"
	} else if err != nil {
		a.errorHandler(http.StatusInternalServerError, rw, req, err)
		return
	}

	other := make(map[string]interface{})
	other["preview"] = false

	err = a.RenderTemplate(rw, "article_edit.html", "index.html", map[string]interface{}{
		"Article": article,
		"Context": req.Context(),
		"Other":   other})
	check(err)
}

func (a *app) revisionPostHandler(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	article := &model.Article{}

	article.URL = vars["article"]
	article.Revision = &model.Revision{}

	article.Title = req.PostFormValue("title")
	article.Markdown = req.PostFormValue("body")
	article.Comment = req.PostFormValue("comment")

	article.Creator = req.Context().Value(model.UserKey).(*model.User)
	previousID, err := strconv.Atoi(vars["revision"])
	if err != nil {
		a.errorHandler(http.StatusBadRequest, rw, req, err)
		return
	}
	article.PreviousID = previousID

	if req.PostFormValue("action") == "preview" {
		a.articlePreviewHandler(article, rw, req)
		return
	}
	a.articlePostHandler(article, rw, req)
}

func (a *app) articlePreviewHandler(article *model.Article, rw http.ResponseWriter, req *http.Request) {
	html, err := a.PreviewMarkdown(article.Markdown)
	if err != nil {
		a.errorHandler(http.StatusInternalServerError, rw, req, err)
		return
	}
	article.HTML = html
	// article.Hash = article.PreviousHash
	other := make(map[string]interface{})
	other["Preview"] = true

	err = a.RenderTemplate(rw, "article_edit.html", "index.html",
		map[string]interface{}{
			"Article": article,
			"Context": req.Context(),
			"Other":   other})
	check(err)
}
func (a *app) articlePostHandler(article *model.Article, rw http.ResponseWriter, req *http.Request) {

	err := a.PostArticle(article)
	if err != nil {
		a.errorHandler(http.StatusBadRequest, rw, req, err)
		return
	}
	http.Redirect(rw, req, "/wiki/"+article.URL, http.StatusSeeOther) // To prevent "browser must resend..."
}

func check(err error) {
	if err != nil {
		log.Println(err)
	}
}

func (a *app) errorHandler(responseCode int, rw http.ResponseWriter, req *http.Request, errors ...error) {
	rw.WriteHeader(responseCode)
	err := a.RenderTemplate(rw, "error.html", "index.html",
		map[string]interface{}{
			"Article": &model.Article{Revision: &model.Revision{Title: fmt.Sprintf("%d: %s", responseCode, http.StatusText(responseCode))}},
			"Context": req.Context(),
			"Error": map[string]interface{}{
				"Code":       responseCode,
				"CodeString": http.StatusText(responseCode),
				"Errors":     errors,
			}})
	if err != nil {
		log.Panic(err)
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
		// text := strings.Replace(html.EscapeString(diff.Text), "\n", "&para;<br>", -1)
		switch diff.Type {
		case diffmatchpatch.DiffInsert:
			_, _ = buff.WriteString("<ins style=\"background:#e6ffe6;\">")
			_, _ = buff.WriteString(diff.Text)
			_, _ = buff.WriteString("</ins>")
		case diffmatchpatch.DiffDelete:
			_, _ = buff.WriteString("<del style=\"background:#ffe6e6;\">")
			_, _ = buff.WriteString(diff.Text)
			_, _ = buff.WriteString("</del>")
		case diffmatchpatch.DiffEqual:
			_, _ = buff.WriteString("<span>")
			_, _ = buff.WriteString(diff.Text)
			_, _ = buff.WriteString("</span>")
		}
	}
	pretty := buff.String()

	err = a.RenderTemplate(rw, "diff.html", "index.html", map[string]interface{}{
		"Article": orginal,
		"Context": req.Context(),
		"Other": map[string]interface{}{
			"DiffString": pretty,
		}})

	// fmt.Fprintf(rw, "<pre>%s</pre>", pretty)
}
