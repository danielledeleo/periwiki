package wiki

import (
	"encoding/json"
	"maps"
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
// Known fields are typed; unknown fields go in Extra.
// All string values are sanitized to strip HTML on parse.
type Frontmatter struct {
	DisplayTitle string            `json:"display_title,omitempty"`
	Layout       string            `json:"layout,omitempty"`
	TOC          *bool             `json:"toc,omitempty"`
	Extra        map[string]string `json:"extra,omitempty"`
}

// sanitize strips HTML from all string fields.
// Called automatically by ParseFrontmatter.
func (fm *Frontmatter) sanitize() {
	fm.DisplayTitle = strictPolicy.Sanitize(fm.DisplayTitle)
	fm.Layout = strictPolicy.Sanitize(fm.Layout)
	for k, v := range fm.Extra {
		fm.Extra[k] = strictPolicy.Sanitize(v)
	}
}

// MarshalJSON flattens Extra fields into the top level.
func (fm Frontmatter) MarshalJSON() ([]byte, error) {
	m := make(map[string]string)

	// Add extra fields first (so known fields take precedence)
	maps.Copy(m, fm.Extra)

	// Add known fields
	if fm.DisplayTitle != "" {
		m["display_title"] = fm.DisplayTitle
	}
	if fm.Layout != "" {
		m["layout"] = fm.Layout
	}
	if fm.TOC != nil {
		if *fm.TOC {
			m["toc"] = "true"
		} else {
			m["toc"] = "false"
		}
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
	if v, ok := m["layout"]; ok {
		fm.Layout = v
		delete(m, "layout")
	}
	if v, ok := m["toc"]; ok {
		b := v != "false"
		fm.TOC = &b
		delete(m, "toc")
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
	if v, ok := raw["layout"]; ok {
		fm.Layout = v
		delete(raw, "layout")
	}
	if v, ok := raw["toc"]; ok {
		b := v != "false"
		fm.TOC = &b
		delete(raw, "toc")
	}
	if len(raw) > 0 {
		fm.Extra = raw
	}

	fm.sanitize()
	return fm, markdown[len(match[0]):]
}
