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
			name:     "returns revision title",
			article:  NewArticle("test-url", "Test Article", "# Content"),
			expected: "Test Article",
		},
		{
			name:     "falls back to inferred title when revision title is empty",
			article:  NewArticle("another-url", "", "# Content"),
			expected: "Another-url",
		},
		{
			name:     "returns title with special characters",
			article:  NewArticle("special-url", "Article: A & B", "# Content"),
			expected: "Article: A & B",
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
		title    string
		expected string
	}{
		{
			name:     "title set takes precedence",
			url:      "test_article",
			title:    "Custom Title",
			expected: "Custom Title",
		},
		{
			name:     "empty title falls back to inferred title",
			url:      "test_article",
			title:    "",
			expected: "Test article",
		},
		{
			name:     "infers title from URL with underscores",
			url:      "My_Page",
			title:    "",
			expected: "My Page",
		},
		{
			name:     "title takes precedence over URL",
			url:      "some_url",
			title:    "Explicit Title",
			expected: "Explicit Title",
		},
		{
			name:     "empty URL with empty title returns empty",
			url:      "",
			title:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			article := NewArticle(tt.url, tt.title, "# Content")
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
