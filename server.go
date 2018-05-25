package main

import (
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/jagger27/iwikii/db"
	"github.com/jagger27/iwikii/model"
	"github.com/jagger27/iwikii/templater"
	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

type app struct {
	*templater.Templater
	*model.WikiModel
	*bluemonday.Policy
}

func main() {
	router := mux.NewRouter()
	bm := bluemonday.UGCPolicy()
	bm.AllowAttrs("class").Matching(regexp.MustCompile("^language-[a-zA-Z0-9]+$")).OnElements("code")
	// md := []byte("# Hello world \n``` go\nint main() {}\n```")
	// output := bm.Sanitize(string(blackfriday.Run(md)))

	t := templater.New()
	t.Load("templates/layouts/*.html", "templates/*.html")
	fs := http.FileServer(http.Dir("./static"))

	cookieKey := os.Getenv("COOKIE_SECRET")
	if cookieKey == "" {
		log.Fatal("COOKIE_SECRET environment variable not set!")
	}
	database, err := db.Init(db.SqliteConfig{DatabaseFile: "iwikii.db", CookieSecretKey: cookieKey})
	check(err)
	model := model.New(database, &model.Config{MinimumPasswordLength: 8})

	app := app{t, model, bm}

	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fs))
	router.HandleFunc("/", app.homeHandler).Methods("GET")
	router.HandleFunc("/wiki/{article}", app.articleHandler)
	router.HandleFunc("/user/register", app.userHandler).Methods("GET")
	router.HandleFunc("/user/register", app.userPostHandler).Methods("POST")
	router.HandleFunc("/user/login", app.loginHander).Methods("GET")
	router.HandleFunc("/user/login", app.loginPostHander).Methods("POST")

	logger := handlers.LoggingHandler(os.Stdout, router)
	http.ListenAndServe(":8080", logger)
}

func (a *app) userHandler(rw http.ResponseWriter, req *http.Request) {
	err := a.RenderTemplate(rw, "register.html", "index.html", map[string]string{"Title": "Register"})
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

	err = a.RenderTemplate(rw, "register.html", "index.html", render)
	check(err)

}

func (a *app) loginHander(rw http.ResponseWriter, req *http.Request) {
	err := a.RenderTemplate(rw, "login.html", "index.html", map[string]string{"Title": "Login"})
	check(err)
}

func (a *app) loginPostHander(rw http.ResponseWriter, req *http.Request) {
	user := &model.User{}
	user.ScreenName = req.PostFormValue("screenname")
	user.RawPassword = req.PostFormValue("password")

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
	} else {
		// set cookie
	}

	err = a.RenderTemplate(rw, "login.html", "index.html", render)
	check(err)
}

func (a *app) homeHandler(rw http.ResponseWriter, req *http.Request) {

	data := make(map[string]interface{})
	data["Article"] = &model.Article{
		Revision: &model.Revision{
			Title: "Home",
			HTML:  "Welcome to iwikii! Why don't you check out <a href='/wiki/test'>Test</a>?",
		},
	}
	data["User"] = model.User{ScreenName: "Jagger", Email: "jagger@twoseven.ca"}

	err := a.RenderTemplate(rw, "home.html", "index.html", data)
	check(err)
}

func (a *app) articleHandler(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	render := map[string]interface{}{}
	article, err := a.GetArticle(vars["article"])
	check(err)

	render["User"] = model.User{ScreenName: "Jagger", Email: "jagger@twoseven.ca"}

	found := article != nil

	if !found {
		article = model.NewArticle(vars["article"], strings.Title(vars["article"]), "")
		check(err)
	}

	if req.Method == "POST" {
		article.Revision = &model.Revision{}

		article.Title = req.PostFormValue("title")
		article.Markdown = req.PostFormValue("body")
		article.Creator = &model.User{ID: 1}
		// article.Title = req.PostFormValue("title")
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

func (a *app) editHandler(data interface{}, rw http.ResponseWriter, req *http.Request) {
	err := a.RenderTemplate(rw, "edit.html", "index.html", data)
	check(err)
}

func (a *app) articlePostHandler(article *model.Article, rw http.ResponseWriter, req *http.Request) {
	unsafe := blackfriday.Run([]byte(article.Markdown), blackfriday.WithExtensions(blackfriday.NoIntraEmphasis|blackfriday.HardLineBreak|blackfriday.Tables))
	article.HTML = a.Sanitize(string(unsafe))
	err := a.PostArticle(article)
	check(err)
	http.Redirect(rw, req, req.URL.Path, http.StatusSeeOther) // To prevent "browser must resend..."
}

func check(err error) {
	if err != nil {
		log.Println(err)
	}
}
