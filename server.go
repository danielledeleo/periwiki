package main

import (
	"log"
	"net/http"
	"os"

	"github.com/jagger27/iwikii/templater"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

var t *templater.Templater

func main() {
	router := mux.NewRouter()
	// bm := bluemonday.UGCPolicy()
	// bm.AllowAttrs("class").Matching(regexp.MustCompile("^language-[a-zA-Z0-9]+$")).OnElements("code")
	// md := []byte("# Hello world \n``` go\nint main() {}\n```")
	t = templater.New()
	t.Load("templates/layouts/*.html", "templates/*.html")
	// output := bm.Sanitize(string(blackfriday.Run(md)))
	fs := http.FileServer(http.Dir("./static"))

	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fs))
	router.HandleFunc("/", homeHandler)
	router.HandleFunc("/wiki/{article}", articleHandler)

	logger := handlers.LoggingHandler(os.Stdout, router)
	http.ListenAndServe(":8080", logger)
}

type Article struct {
	Title string
	Body  string
}

func homeHandler(rw http.ResponseWriter, req *http.Request) {

	a := make(map[string]string)
	a["Url"] = req.URL.Path
	a["Title"] = "Home"
	a["Body"] = "Welcome to iwikii!"

	err := t.RenderTemplate(rw, "article.html", "index.html", a)
	check(err)
}

func articleHandler(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	a := make(map[string]string)
	a["Url"] = req.URL.Path
	a["Title"] = vars["article"]
	a["Body"] = "This is " + vars["article"]

	if req.Method == "POST" {
		http.Redirect(rw, req, a["Url"], http.StatusSeeOther) // To prevent "browser must resend..."
	}
	if _, ok := req.URL.Query()["edit"]; ok {
		editHandler(rw, req)
		return
	}

	err := t.RenderTemplate(rw, "article.html", "index.html", a)
	check(err)
}

func editHandler(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	a := make(map[string]string)
	a["Url"] = req.URL.Path
	a["Title"] = vars["article"]
	a["Body"] = vars["article"]

	err := t.RenderTemplate(rw, "edit.html", "index.html", a)
	check(err)

}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
