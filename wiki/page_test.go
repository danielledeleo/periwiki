package wiki

import "testing"

func TestStaticPageDisplayTitle(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		expected string
	}{
		{
			name:     "simple title",
			title:    "Login",
			expected: "Login",
		},
		{
			name:     "title with spaces",
			title:    "Create Account",
			expected: "Create Account",
		},
		{
			name:     "empty title",
			title:    "",
			expected: "",
		},
		{
			name:     "title with special characters",
			title:    "Error: Not Found",
			expected: "Error: Not Found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page := &StaticPage{title: tt.title}
			if got := page.DisplayTitle(); got != tt.expected {
				t.Errorf("StaticPage.DisplayTitle() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestNewStaticPage(t *testing.T) {
	tests := []struct {
		name  string
		title string
	}{
		{
			name:  "creates page with simple title",
			title: "Login",
		},
		{
			name:  "creates page with empty title",
			title: "",
		},
		{
			name:  "creates page with long title",
			title: "This is a very long page title for testing purposes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page := NewStaticPage(tt.title)
			if page == nil {
				t.Fatal("NewStaticPage() returned nil")
			}
			if got := page.DisplayTitle(); got != tt.title {
				t.Errorf("NewStaticPage(%q).DisplayTitle() = %q, want %q", tt.title, got, tt.title)
			}
		})
	}
}

// Compile-time check that Article implements Page interface
var _ Page = (*Article)(nil)

func TestArticleDisplayTitle(t *testing.T) {
	tests := []struct {
		name     string
		article  *Article
		expected string
	}{
		{
			name:     "returns frontmatter display_title",
			article:  NewArticle("test-url", "", "---\ndisplay_title: Test Article\n---\n# Content"),
			expected: "Test Article",
		},
		{
			name:     "falls back to inferred title when no frontmatter",
			article:  NewArticle("another-url", "", "# Content"),
			expected: "Another-url",
		},
		{
			name:     "returns title with special characters",
			article:  NewArticle("special-url", "", "---\ndisplay_title: Article: A & B\n---\n# Content"),
			expected: "Article: A &amp; B",
		},
		{
			name:     "falls back to inferred when frontmatter has no display_title",
			article:  NewArticle("my_page", "", "---\nother_field: value\n---\n# Content"),
			expected: "My page",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.article.DisplayTitle(); got != tt.expected {
				t.Errorf("Article.DisplayTitle() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// Compile-time check that StaticPage implements Page interface
var _ Page = (*StaticPage)(nil)

func TestArticleDisplayTitleWithInference(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		markdown string
		expected string
	}{
		{
			name:     "frontmatter display_title takes precedence",
			url:      "test_article",
			markdown: "---\ndisplay_title: Custom Title\n---\n# Content",
			expected: "Custom Title",
		},
		{
			name:     "empty frontmatter falls back to inferred title",
			url:      "test_article",
			markdown: "---\n---\n# Content",
			expected: "Test article",
		},
		{
			name:     "explicitly empty display_title falls back to inferred title",
			url:      "test_article",
			markdown: "---\ndisplay_title:\n---\n# Content",
			expected: "Test article",
		},
		{
			name:     "no frontmatter infers title from URL with underscores",
			url:      "My_Page",
			markdown: "# Content",
			expected: "My Page",
		},
		{
			name:     "frontmatter title takes precedence over URL",
			url:      "some_url",
			markdown: "---\ndisplay_title: Explicit Title\n---\n# Content",
			expected: "Explicit Title",
		},
		{
			name:     "empty URL with no frontmatter returns empty",
			url:      "",
			markdown: "# Content",
			expected: "",
		},
		{
			name:     "wikilink in display_title preserved",
			url:      "test",
			markdown: "---\ndisplay_title: See [[Related]]\n---\n# Content",
			expected: "See [[Related]]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			article := NewArticle(tt.url, "", tt.markdown)
			if got := article.DisplayTitle(); got != tt.expected {
				t.Errorf("Article.DisplayTitle() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestInferTitle(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "underscores to spaces with existing capitalization",
			url:      "Provinces_of_Canada",
			expected: "Provinces of Canada",
		},
		{
			name:     "capitalizes first character only",
			url:      "mechanical_keyboards",
			expected: "Mechanical keyboards",
		},
		{
			name:     "preserves all caps",
			url:      "HTML",
			expected: "HTML",
		},
		{
			name:     "preserves mixed case after first char",
			url:      "snake_Case",
			expected: "Snake Case",
		},
		{
			name:     "single lowercase word",
			url:      "test",
			expected: "Test",
		},
		{
			name:     "single uppercase word",
			url:      "TEST",
			expected: "TEST",
		},
		{
			name:     "single character",
			url:      "a",
			expected: "A",
		},
		{
			name:     "empty string",
			url:      "",
			expected: "",
		},
		{
			name:     "already title case with underscores",
			url:      "already_Title_Case",
			expected: "Already Title Case",
		},
		{
			name:     "double underscores preserved as double spaces",
			url:      "with__double__underscores",
			expected: "With  double  underscores",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := InferTitle(tt.url); got != tt.expected {
				t.Errorf("InferTitle(%q) = %q, want %q", tt.url, got, tt.expected)
			}
		})
	}
}
