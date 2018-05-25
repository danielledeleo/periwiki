package templater

import (
	"fmt"
	"io"
	"path/filepath"
	"text/template"
)

// Templater ecapsulates the map to prevent direct access. See RenderTemplate
type Templater struct {
	templates map[string]*template.Template
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
	// Generate our templates map from our layouts/ and includes/ directories
	for _, layout := range layouts {
		files := append(base, layout)
		t.templates[filepath.Base(layout)] = template.Must(template.ParseFiles(files...))
	}
	return nil
}

// RenderTemplate just makes sure the templates exist. Don't mix up name and base!
func (t *Templater) RenderTemplate(w io.Writer, name string, base string, data interface{}) error {
	// Ensure the template exists in the map.
	tmpl, ok := t.templates[name]
	if !ok {
		return fmt.Errorf("content template %s does not exist", name)
	}

	b := t.templates[name].Lookup(base)
	if b == nil {
		return fmt.Errorf("base template %s does not exist", name)
	}
	return tmpl.ExecuteTemplate(w, base, data)
}
