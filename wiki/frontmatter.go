package wiki

import (
	"regexp"

	"github.com/danielledeleo/nestedtext"
	"github.com/microcosm-cc/bluemonday"
)

// strictPolicy strips all HTML tags from frontmatter values.
var strictPolicy = bluemonday.StrictPolicy()

// frontmatterRegex matches YAML-style fences at document start.
// Requires newline or end-of-string after closing fence.
var frontmatterRegex = regexp.MustCompile(`(?s)\A---\r?\n(.*?)(?:\r?\n)?---(?:\r?\n|\z)`)

// Frontmatter holds parsed article metadata.
// All string fields are sanitized to strip HTML on parse.
type Frontmatter struct {
	DisplayTitle string `nt:"display_title"`
}

// sanitize strips HTML from all string fields.
// Called automatically by ParseFrontmatter.
func (fm *Frontmatter) sanitize() {
	fm.DisplayTitle = strictPolicy.Sanitize(fm.DisplayTitle)
}

// ParseFrontmatter extracts NestedText frontmatter from markdown.
// Returns parsed metadata and content with frontmatter stripped.
// On parse error, returns zero Frontmatter and original markdown.
func ParseFrontmatter(markdown string) (Frontmatter, string) {
	match := frontmatterRegex.FindStringSubmatch(markdown)
	if match == nil {
		return Frontmatter{}, markdown
	}

	var fm Frontmatter
	if err := nestedtext.Unmarshal([]byte(match[1]), &fm); err != nil {
		return Frontmatter{}, markdown
	}

	fm.sanitize()
	return fm, markdown[len(match[0]):]
}
