package wiki

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var whitespaceRun = regexp.MustCompile(`\s+`)

// TitleToSlug converts a display title to a URL/anchor slug.
// It trims whitespace and collapses runs of whitespace into underscores.
// Case and punctuation are preserved.
func TitleToSlug(title string) string {
	s := strings.TrimSpace(title)
	return whitespaceRun.ReplaceAllString(s, "_")
}

// InferTitle converts a URL slug to a display title.
// It replaces underscores with spaces and capitalizes only the first character.
func InferTitle(url string) string {
	if url == "" {
		return ""
	}
	// Replace underscores with spaces
	title := strings.ReplaceAll(url, "_", " ")
	// Capitalize first character only
	r, size := utf8.DecodeRuneInString(title)
	if r == utf8.RuneError {
		return title
	}
	return string(unicode.ToUpper(r)) + title[size:]
}

// Page represents any page that can be displayed with a title.
type Page interface {
	DisplayTitle() string
}

// StaticPage represents a non-article page like login or error pages.
type StaticPage struct {
	title string
}

// NewStaticPage creates a new StaticPage with the given title.
func NewStaticPage(title string) *StaticPage {
	return &StaticPage{title: title}
}

// DisplayTitle returns the page's display title.
func (p *StaticPage) DisplayTitle() string {
	return p.title
}
