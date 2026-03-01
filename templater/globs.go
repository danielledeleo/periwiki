package templater

// Template glob patterns used by both production setup and test setup.
// These must stay in sync — the TestAllTemplateSubdirsHaveGlobs meta-test
// verifies that every template subdirectory is covered.

// LayoutGlob is the base/layout template glob (first argument to Load).
var LayoutGlob = "templates/layouts/*.html"

// ContentGlobs are the content template globs (remaining arguments to Load).
var ContentGlobs = []string{
	"templates/*.html",
	"templates/special/*.html",
	"templates/manage/*.html",
}

// FootnoteTemplateNames are the required templates for the footnote extension.
var FootnoteTemplateNames = []string{"link", "backlink", "list", "item"}

// WikiLinkTemplateNames are the required templates for the wikilink extension.
var WikiLinkTemplateNames = []string{"link"}
