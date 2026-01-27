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
			name:     "returns empty title when revision title is empty",
			article:  NewArticle("another-url", "", "# Content"),
			expected: "",
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
