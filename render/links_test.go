package render

import (
	"slices"
	"testing"
)

func TestExtractLinks(t *testing.T) {
	extractor := NewLinkExtractor()

	tests := []struct {
		name     string
		markdown string
		want     []string
	}{
		{
			name:     "single link",
			markdown: "See [[Foo Bar]].",
			want:     []string{"Foo_Bar"},
		},
		{
			name:     "multiple links",
			markdown: "See [[Foo]] and [[Bar]].",
			want:     []string{"Foo", "Bar"},
		},
		{
			name:     "piped link",
			markdown: "See [[Target Page|display text]].",
			want:     []string{"Target_Page"},
		},
		{
			name:     "deduplication",
			markdown: "See [[Foo]] and also [[Foo]].",
			want:     []string{"Foo"},
		},
		{
			name:     "ignores code block",
			markdown: "```\n[[Not A Link]]\n```",
			want:     nil,
		},
		{
			name:     "ignores inline code",
			markdown: "Use `[[Not A Link]]` syntax.",
			want:     nil,
		},
		{
			name:     "with frontmatter",
			markdown: "---\ndisplay_title: My Page\n---\nSee [[Target]].",
			want:     []string{"Target"},
		},
		{
			name:     "preserves case",
			markdown: "See [[CamelCase]].",
			want:     []string{"CamelCase"},
		},
		{
			name:     "empty markdown",
			markdown: "",
			want:     nil,
		},
		{
			name:     "no links",
			markdown: "Just plain text.",
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractor.ExtractLinks(tt.markdown)
			if !slices.Equal(got, tt.want) {
				t.Errorf("ExtractLinks() = %v, want %v", got, tt.want)
			}
		})
	}
}
