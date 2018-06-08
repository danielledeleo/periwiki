package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/jagger27/iwikii/db"
	"github.com/jagger27/iwikii/model"
	"github.com/jagger27/iwikii/templater"
	"github.com/microcosm-cc/bluemonday"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

type app struct {
	*templater.Templater
	*model.WikiModel
}

func main() {
	router := mux.NewRouter()
	bm := bluemonday.UGCPolicy()
	bm.AllowAttrs("class").Matching(regexp.MustCompile("^sourceCode(| [a-zA-Z0-9]+)$")).
		OnElements("pre", "code")

	bm.AllowAttrs("data-line-number", "class").Matching(regexp.MustCompile("^[0-9]+$")).OnElements("a")

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

	router.HandleFunc("/user/register", app.userHandler).Methods("GET")
	router.HandleFunc("/user/register", app.userPostHandler).Methods("POST")
	router.HandleFunc("/user/login", app.loginHander).Methods("GET")
	router.HandleFunc("/user/login", app.loginPostHander).Methods("POST")
	router.HandleFunc("/user/logout", app.logoutPostHander).Methods("POST")

	logger := handlers.LoggingHandler(os.Stdout, router)
	http.ListenAndServe(":8080", logger)
}

func (a *app) userHandler(rw http.ResponseWriter, req *http.Request) {
	err := a.RenderTemplate(rw, "register.html", "index.html", map[string]interface{}{"Article": map[string]string{"Title": "Register"}})
	check(err)
}
func (a *app) userPostHandler(rw http.ResponseWriter, req *http.Request) {
	user := &model.User{}

	user.Email = req.PostFormValue("email")
	user.ScreenName = req.PostFormValue("screenname")
	user.RawPassword = req.PostFormValue("password")

	render := map[string]string{
		"Title":          "Register",
		"calloutClasses": "iw-success",
		"calloutMessage": "Successfully registered!",
		"formClasses":    "hidden",
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

	err = a.RenderTemplate(rw, "register.html", "index.html", map[string]interface{}{"Article": render})
	check(err)

}

func (a *app) loginHander(rw http.ResponseWriter, req *http.Request) {
	err := a.RenderTemplate(rw, "login.html", "index.html", map[string]interface{}{
		"Article": map[string]string{
			"Title":         "Login",
			"referrerValue": req.Referer(),
		}})
	check(err)
}

func (a *app) loginPostHander(rw http.ResponseWriter, req *http.Request) {
	user := &model.User{}
	user.ScreenName = req.PostFormValue("screenname")
	user.RawPassword = req.PostFormValue("password")
	referrer := req.PostFormValue("referrer")

	err := a.CheckUserPassword(user)

	render := map[string]string{
		"Title":          "Login",
		"calloutClasses": "iw-success",
		"calloutMessage": "Successfully logged in!",
		"formClasses":    "hidden",
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
	check(err)
	session.Options.MaxAge = 86400 // a day
	session.Values["username"] = user.ScreenName
	err = session.Save(req, rw)
	check(err)

	if referrer == "" {
		referrer = "/"
	}
	http.Redirect(rw, req, referrer, http.StatusSeeOther)
}

func (a *app) logoutPostHander(rw http.ResponseWriter, req *http.Request) {
	session, err := a.GetCookie(req, "iwikii-login")
	check(err)

	err = a.DeleteCookie(req, rw, session)
	check(err)

	http.Redirect(rw, req, "/", http.StatusSeeOther)
}

func (a *app) homeHandler(rw http.ResponseWriter, req *http.Request) {
	data := make(map[string]interface{})
	user := req.Context().Value(userKey)

	if user != nil && user.(*model.User).ID != 0 {
		data["User"] = user
	}
	data["Article"] = &model.Article{
		Revision: &model.Revision{
			Title: "Home",
			HTML:  "Welcome to iwikii! Why don't you check out <a href='/wiki/test'>Test</a>?",
		},
	}

	err := a.RenderTemplate(rw, "home.html", "index.html", data)
	check(err)
}

func (a *app) articleHandler(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	render := map[string]interface{}{}
	article, err := a.GetArticle(vars["article"])

	check(err)
	user := req.Context().Value(userKey)

	if user != nil && user.(*model.User).ID != 0 {
		render["User"] = user.(*model.User)
	}

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
		article.Creator = render["User"].(*model.User)
		a.articlePostHandler(article, rw, req)
		return
	}

	render["Article"] = article
	if _, ok := req.URL.Query()["edit"]; ok {
		a.editHandler(render, rw, req)
		return
	}

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
	check(err)

	err = a.RenderTemplate(rw, "article_history.html", "index.html", map[string]interface{}{
		"Article": map[string]interface{}{
			"URL":   url,
			"Title": "History of " + revisions[0].Title,
		},
		"Revisions": revisions})
	check(err)
}

func (a *app) revisionHandler(rw http.ResponseWriter, req *http.Request) {
	render := map[string]interface{}{}
	vars := mux.Vars(req)
	if _, ok := req.URL.Query()["edit"]; ok {
		a.revisionEditHandler(rw, req)
		return
	}
	article, err := a.GetArticleByRevisionHash(vars["article"], vars["revision"])
	if err != nil {
		log.Panic(err)
	}
	render["Article"] = article
	err = a.RenderTemplate(rw, "article.html", "index.html", render)
	// log.Println(article.Title, err)
}

func (a *app) revisionEditHandler(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	article, err := a.GetArticleByRevisionHash(vars["article"], vars["revision"])
	if err == sql.ErrNoRows {
		article = model.NewArticle(vars["article"], strings.Title(vars["article"]), "")
		article.Hash = "new"
	} else if err != nil {
		log.Panic(err)
	}

	other := make(map[string]interface{})
	other["preview"] = false

	err = a.RenderTemplate(rw, "article_edit.html", "index.html", map[string]interface{}{"Article": article, "Other": other})
	if err != nil {
		log.Panic(err)
	}
}

func (a *app) revisionPostHandler(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	article := &model.Article{}

	article.URL = vars["article"]
	article.Revision = &model.Revision{}

	article.Title = req.PostFormValue("title")
	article.Markdown = req.PostFormValue("body")
	article.Comment = req.PostFormValue("comment")

	article.Creator = req.Context().Value(userKey).(*model.User)
	article.PreviousHash = vars["revision"]
	if req.PostFormValue("action") == "preview" {
		a.articlePreviewHandler(article, rw, req)
		return
	}
	a.articlePostHandler(article, rw, req)
}

func (a *app) editHandler(data interface{}, rw http.ResponseWriter, req *http.Request) {
	err := a.RenderTemplate(rw, "article_edit.html", "index.html", data)
	check(err)
}
func (a *app) articlePreviewHandler(article *model.Article, rw http.ResponseWriter, req *http.Request) {
	html, err := a.PreviewMarkdown(article.Markdown)
	article.HTML = html
	article.Hash = article.PreviousHash
	if err != nil {
		log.Panic(err)
	}
	other := make(map[string]interface{})
	other["Preview"] = true

	err = a.RenderTemplate(rw, "article_edit.html", "index.html", map[string]interface{}{"Article": article, "Other": other})
	if err != nil {
		log.Panic(err)
	}
}
func (a *app) articlePostHandler(article *model.Article, rw http.ResponseWriter, req *http.Request) {

	err := a.PostArticle(article)
	check(err)
	http.Redirect(rw, req, "/wiki/"+article.URL, http.StatusSeeOther) // To prevent "browser must resend..."
}

func check(err error) {
	if err != nil {
		log.Println(err)
	}
}
