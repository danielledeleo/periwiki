package templater

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"text/template"
	"unicode"
	"unicode/utf8"

	"github.com/jagger27/periwiki/wiki"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Templater ecapsulates the map to prevent direct access. See RenderTemplate
type Templater struct {
	templates map[string]*template.Template
	funcs     map[string]interface{}
}

// HTMLItem is used to inject attributes and text into HTML templates.
type HTMLItem struct {
	Text string
	attr map[string][]string
}

// Attributes returns a formatted string of attributes (set by AddAttribute)
func (item HTMLItem) Attributes() string {
	result := ""
	for key, val := range item.attr {
		attrs := ""
		for _, attr := range val {
			attrs += attr + " "
		}
		attrs = attrs[:len(attrs)-1]
		result += fmt.Sprintf(`%s="%s" `, key, attrs)
	}
	return result
}

// AddAttribute creates a key/value pair to represent and format HTML attributes into a string
// e.g. class="hidden pw-error"
// Setting a key twice overwrites it. Empty keys are ignored.
func (item HTMLItem) AddAttribute(key string, value ...string) {
	if key == "" {
		return
	}
	item.attr[key] = value
}

func New() *Templater {
	return &Templater{}
}

// Load loads or reloads template files from the filesystem. Here, baseGlob
// refers to base templates (usually a wrapper containing headers and such)
// and mainGlob refers to to the templates that are used to fill the base
// templates. Globs are of the standard Go format, i.e. templates/*.html
func (t *Templater) Load(baseGlob, mainGlob string) error {
	t.templates = make(map[string]*template.Template)
	layouts, err := filepath.Glob(mainGlob)
	if err != nil {
		return err
	}

	base, err := filepath.Glob(baseGlob)
	if err != nil {
		return err
	}

	titler := cases.Title(language.AmericanEnglish) // TODO: locales!

	t.funcs = template.FuncMap{
		"title":       titler.String,
		"capitalize":  capitalize, // TODO: maybe replace this with something more robust?
		"pathEscape":  url.PathEscape,
		"queryEscape": url.QueryEscape,
		"statusText":  http.StatusText,
	}

	// Generate our templates map from our layouts/ and includes/ directories
	for _, layout := range layouts {
		files := append(base, layout)
		t.templates[filepath.Base(layout)] = template.Must(template.New(filepath.Base(layout)).Funcs(t.funcs).ParseFiles(files...))
	}
	return nil
}

// RenderTemplate makes sure templates exist and renders them. Don't mix up name and base!
func (t *Templater) RenderTemplate(w io.Writer, name string, base string, data map[string]interface{}) error {
	// Ensure the template exists in the map.
	tmpl, ok := t.templates[name]
	if !ok {
		return fmt.Errorf("content template %s does not exist", name)
	}

	b := t.templates[name].Lookup(base)
	if b == nil {
		return fmt.Errorf("base template %s does not exist", name)
	}

	if data["Context"] != nil {
		data["User"] = data["Context"].(context.Context).Value(wiki.UserKey).(*wiki.User)
	}

	return tmpl.ExecuteTemplate(w, base, data)
}

func capitalize(s string) string {
	if s == "" {
		return ""
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToTitle(r)) + s[size:]
}
