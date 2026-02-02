package wiki

import (
	"encoding/json"
	"regexp"

	"github.com/danielledeleo/nestedtext"
	"github.com/microcosm-cc/bluemonday"
)

// strictPolicy strips all HTML tags from frontmatter values.
var strictPolicy = bluemonday.StrictPolicy()

// frontmatterRegex matches YAML-style fences at document start.
// Requires newline or end-of-string after closing fence.
var frontmatterRegex = regexp.MustCompile(`(?s)\A---\r?\n(.*?)(?:\r?\n)?---(?:\r?\n|\z)`)

// knownFields maps JSON field names to their NestedText field names.
var knownFields = map[string]string{
	"display_title": "display_title",
}

// Frontmatter holds parsed article metadata.
// Known fields are typed; unknown fields go in Extra.
// All string values are sanitized to strip HTML on parse.
type Frontmatter struct {
	DisplayTitle string            `json:"display_title,omitempty"`
	Extra        map[string]string `json:"extra,omitempty"`
}

// sanitize strips HTML from all string fields.
// Called automatically by ParseFrontmatter.
func (fm *Frontmatter) sanitize() {
	fm.DisplayTitle = strictPolicy.Sanitize(fm.DisplayTitle)
	for k, v := range fm.Extra {
		fm.Extra[k] = strictPolicy.Sanitize(v)
	}
}

// MarshalJSON flattens Extra fields into the top level.
func (fm Frontmatter) MarshalJSON() ([]byte, error) {
	m := make(map[string]string)

	// Add extra fields first (so known fields take precedence)
	for k, v := range fm.Extra {
		m[k] = v
	}

	// Add known fields
	if fm.DisplayTitle != "" {
		m["display_title"] = fm.DisplayTitle
	}

	return json.Marshal(m)
}

// UnmarshalJSON reads known fields into struct, extras into map.
func (fm *Frontmatter) UnmarshalJSON(data []byte) error {
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	if v, ok := m["display_title"]; ok {
		fm.DisplayTitle = v
		delete(m, "display_title")
	}

	if len(m) > 0 {
		fm.Extra = m
	}

	return nil
}

// ParseFrontmatter extracts NestedText frontmatter from markdown.
// Returns parsed metadata and content with frontmatter stripped.
// On parse error, returns zero Frontmatter and original markdown.
func ParseFrontmatter(markdown string) (Frontmatter, string) {
	match := frontmatterRegex.FindStringSubmatch(markdown)
	if match == nil {
		return Frontmatter{}, markdown
	}

	// Parse into a map first to capture all fields
	var raw map[string]string
	if err := nestedtext.Unmarshal([]byte(match[1]), &raw); err != nil {
		return Frontmatter{}, markdown
	}

	// Extract known fields, put rest in Extra
	fm := Frontmatter{}
	if v, ok := raw["display_title"]; ok {
		fm.DisplayTitle = v
		delete(raw, "display_title")
	}
	if len(raw) > 0 {
		fm.Extra = raw
	}

	fm.sanitize()
	return fm, markdown[len(match[0]):]
}
