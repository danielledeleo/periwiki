package templater

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strings"
	"text/template"
	"unicode"
	"unicode/utf8"

	"github.com/danielledeleo/periwiki/wiki"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Templater ecapsulates the map to prevent direct access. See RenderTemplate
type Templater struct {
	fsys      fs.FS
	templates map[string]*template.Template
	funcs     map[string]any
}

// HTMLItem is used to inject attributes and text into HTML templates.
type HTMLItem struct {
	Text string
	attr map[string][]string
}

// Attributes returns a formatted string of attributes (set by AddAttribute)
func (item HTMLItem) Attributes() string {
	var result strings.Builder
	for key, val := range item.attr {
		result.WriteString(key)
		result.WriteString(`="`)
		for i, attr := range val {
			if i > 0 {
				result.WriteByte(' ')
			}
			result.WriteString(attr)
		}
		result.WriteString(`" `)
	}
	return result.String()
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

func New(fsys fs.FS) *Templater {
	return &Templater{fsys: fsys}
}

// ExtensionTemplates holds parsed templates for an extension.
type ExtensionTemplates map[string]*template.Template

// LoadExtensionTemplates loads templates for a named extension from <templatesDir>/_render/<name>/.
// The templatesDir parameter should be the base templates directory (e.g., "templates").
// Returns error if any required template file is missing.
func (t *Templater) LoadExtensionTemplates(templatesDir, name string, required []string) (ExtensionTemplates, error) {
	templates := make(ExtensionTemplates)
	dir := path.Join(templatesDir, "_render", name)

	for _, tmplName := range required {
		p := path.Join(dir, tmplName+".html")
		tmpl, err := template.New(tmplName).Funcs(t.funcs).ParseFS(t.fsys, p)
		if err != nil {
			return nil, fmt.Errorf("failed to load extension template %s/%s: %w", name, tmplName, err)
		}
		// ParseFS names the template after the file basename, so we need to look it up
		templates[tmplName] = tmpl.Lookup(tmplName + ".html")
		if templates[tmplName] == nil {
			return nil, fmt.Errorf("template %s/%s.html not found after parsing", name, tmplName)
		}
	}

	return templates, nil
}

// Load loads or reloads template files from the filesystem. Here, baseGlob
// refers to base templates (usually a wrapper containing headers and such)
// and mainGlob refers to to the templates that are used to fill the base
// templates. Globs are of the standard Go format, i.e. templates/*.html
func (t *Templater) Load(baseGlob string, mainGlobs ...string) error {
	t.templates = make(map[string]*template.Template)

	var layouts []string
	for _, mainGlob := range mainGlobs {
		matches, err := fs.Glob(t.fsys, mainGlob)
		if err != nil {
			return err
		}
		layouts = append(layouts, matches...)
	}

	base, err := fs.Glob(t.fsys, baseGlob)
	if err != nil {
		return err
	}

	titler := cases.Title(language.AmericanEnglish) // TODO: locales!

	t.funcs = template.FuncMap{
		"title":       titler.String,
		"capitalize":  capitalize, // TODO: maybe replace this with something more robust?
		"pathEscape":  url.PathEscape,
		"letter":      indexToLetter,
		"queryEscape": url.QueryEscape,
		"statusText":  http.StatusText,
		// Article URL helpers
		"articleURL":  articleURL,
		"revisionURL": revisionURL,
		"editURL":     editURL,
		"historyURL":  historyURL,
		"diffURL":     diffURL,
	}

	// Generate our templates map from our layouts/ and includes/ directories
	for _, layout := range layouts {
		files := append(base, layout)
		basename := path.Base(layout)
		t.templates[basename] = template.Must(template.New(basename).Funcs(t.funcs).ParseFS(t.fsys, files...))
	}
	return nil
}

// RenderTemplate makes sure templates exist and renders them. Don't mix up name and base!
func (t *Templater) RenderTemplate(w io.Writer, name string, base string, data map[string]any) error {
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

	// Ensure Article has a display title for page title
	t.ensureTitle(data, name)

	return tmpl.ExecuteTemplate(w, base, data)
}

// ensureTitle checks if data["Article"] has a display title. If not, it derives
// a title from the template name and logs a debug message.
func (t *Templater) ensureTitle(data map[string]any, templateName string) {
	// Check if Article exists and has a display title
	if article, ok := data["Article"]; ok {
		switch a := article.(type) {
		case map[string]string:
			if a["Title"] != "" {
				return
			}
		case map[string]any:
			if title, ok := a["Title"].(string); ok && title != "" {
				return
			}
		case *wiki.Article:
			if a != nil && a.Revision != nil && a.DisplayTitle() != "" {
				return
			}
		default:
			// Unknown type, check via reflection-like approach
			return
		}
	}

	// Derive title from template name: "sitemap.html" -> "Sitemap"
	title := strings.TrimSuffix(templateName, ".html")
	title = strings.ReplaceAll(title, "_", " ")
	title = strings.ReplaceAll(title, "-", " ")
	if len(title) > 0 {
		title = strings.ToUpper(title[:1]) + title[1:]
	}

	slog.Debug("page rendered without explicit title", "template", templateName, "derived_title", title)

	// Set the derived title
	if data["Article"] == nil {
		data["Article"] = map[string]string{"Title": title}
	} else if a, ok := data["Article"].(map[string]string); ok {
		a["Title"] = title
	} else if a, ok := data["Article"].(map[string]any); ok {
		a["Title"] = title
	}
}

func capitalize(s string) string {
	if s == "" {
		return ""
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToTitle(r)) + s[size:]
}

// indexToLetter converts a 0-based index to a lowercase letter (0=a, 1=b, ..., 25=z).
// For indices >= 26, it wraps (26=a, 27=b, etc.).
func indexToLetter(i int) string {
	return string(rune('a' + i%26))
}
