package wiki

import (
	"reflect"
	"testing"
)

func boolPtr(b bool) *bool { return &b }

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectedFM      Frontmatter
		expectedContent string
	}{
		{
			name:            "no frontmatter",
			input:           "# Hello",
			expectedFM:      Frontmatter{},
			expectedContent: "# Hello",
		},
		{
			name:            "valid frontmatter with display_title",
			input:           "---\ndisplay_title: Custom Title\n---\n# Hello",
			expectedFM:      Frontmatter{DisplayTitle: "Custom Title"},
			expectedContent: "# Hello",
		},
		{
			name:            "empty frontmatter block",
			input:           "---\n---\n# Hello",
			expectedFM:      Frontmatter{},
			expectedContent: "# Hello",
		},
		{
			name:            "invalid nestedtext returns original",
			input:           "---\n: bad\n---\n# Hello",
			expectedFM:      Frontmatter{},
			expectedContent: "---\n: bad\n---\n# Hello",
		},
		{
			name:            "CRLF line endings",
			input:           "---\r\ndisplay_title: Windows Title\r\n---\r\n# Hello",
			expectedFM:      Frontmatter{DisplayTitle: "Windows Title"},
			expectedContent: "# Hello",
		},
		{
			name:            "frontmatter not at document start",
			input:           "# Hello\n---\ndisplay_title: Not At Start\n---",
			expectedFM:      Frontmatter{},
			expectedContent: "# Hello\n---\ndisplay_title: Not At Start\n---",
		},
		{
			name:            "frontmatter with trailing content",
			input:           "---\ndisplay_title: With Content\n---\n\n# Heading\n\nBody text here.",
			expectedFM:      Frontmatter{DisplayTitle: "With Content"},
			expectedContent: "\n# Heading\n\nBody text here.",
		},
		{
			name:            "layout field parsed",
			input:           "---\nlayout: mainpage\n---\n# Hello",
			expectedFM:      Frontmatter{Layout: "mainpage"},
			expectedContent: "# Hello",
		},
		{
			name:            "layout and display_title together",
			input:           "---\nlayout: mainpage\ndisplay_title: Main Page\n---\n# Hello",
			expectedFM:      Frontmatter{Layout: "mainpage", DisplayTitle: "Main Page"},
			expectedContent: "# Hello",
		},
		{
			name:            "toc false disables table of contents",
			input:           "---\ntoc: false\n---\n# Hello",
			expectedFM:      Frontmatter{TOC: boolPtr(false)},
			expectedContent: "# Hello",
		},
		{
			name:            "toc true keeps table of contents",
			input:           "---\ntoc: true\n---\n# Hello",
			expectedFM:      Frontmatter{TOC: boolPtr(true)},
			expectedContent: "# Hello",
		},
		{
			name:            "unknown fields captured in Extra",
			input:           "---\ndisplay_title: Known\nunknown_field: Value\n---\n# Hello",
			expectedFM:      Frontmatter{DisplayTitle: "Known", Extra: map[string]string{"unknown_field": "Value"}},
			expectedContent: "# Hello",
		},
		{
			name:            "display_title with wikilink",
			input:           "---\ndisplay_title: See [[Related Article]]\n---\n# Hello",
			expectedFM:      Frontmatter{DisplayTitle: "See [[Related Article]]"},
			expectedContent: "# Hello",
		},
		{
			name:            "only opening fence",
			input:           "---\ndisplay_title: Incomplete\n# No closing fence",
			expectedFM:      Frontmatter{},
			expectedContent: "---\ndisplay_title: Incomplete\n# No closing fence",
		},
		{
			name:            "empty document",
			input:           "",
			expectedFM:      Frontmatter{},
			expectedContent: "",
		},
		{
			name:            "frontmatter with no trailing newline after closing fence",
			input:           "---\ndisplay_title: No Trailing\n---# Hello",
			expectedFM:      Frontmatter{},
			expectedContent: "---\ndisplay_title: No Trailing\n---# Hello",
		},
		// Security edge cases - HTML is stripped from frontmatter values
		{
			name:            "HTML tags in display_title stripped",
			input:           "---\ndisplay_title: <script>alert('xss')</script>\n---\n# Hello",
			expectedFM:      Frontmatter{DisplayTitle: ""},
			expectedContent: "# Hello",
		},
		{
			name:            "HTML with text in display_title keeps text",
			input:           "---\ndisplay_title: Hello <b>World</b>!\n---\n# Hello",
			expectedFM:      Frontmatter{DisplayTitle: "Hello World!"},
			expectedContent: "# Hello",
		},
		{
			name:            "template syntax in display_title preserved",
			input:           "---\ndisplay_title: {{.Secret}}\n---\n# Hello",
			expectedFM:      Frontmatter{DisplayTitle: "{{.Secret}}"},
			expectedContent: "# Hello",
		},
		{
			name:            "ampersand in display_title encoded",
			input:           "---\ndisplay_title: Tom & Jerry\n---\n# Hello",
			expectedFM:      Frontmatter{DisplayTitle: "Tom &amp; Jerry"},
			expectedContent: "# Hello",
		},
		{
			name:            "unicode in display_title",
			input:           "---\ndisplay_title: 日本語タイトル 🎉\n---\n# Hello",
			expectedFM:      Frontmatter{DisplayTitle: "日本語タイトル 🎉"},
			expectedContent: "# Hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, content := ParseFrontmatter(tt.input)
			if !reflect.DeepEqual(fm, tt.expectedFM) {
				t.Errorf("ParseFrontmatter() frontmatter = %+v, want %+v", fm, tt.expectedFM)
			}
			if content != tt.expectedContent {
				t.Errorf("ParseFrontmatter() content = %q, want %q", content, tt.expectedContent)
			}
		})
	}
}
