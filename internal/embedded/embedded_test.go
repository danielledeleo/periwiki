package embedded_test

import (
	"testing"

	"github.com/danielledeleo/periwiki/internal/embedded"
)

func TestNew(t *testing.T) {
	// Simple render function for testing
	render := func(md string) (string, error) {
		return "<p>rendered</p>", nil
	}

	ea, err := embedded.New(render)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Syntax.md should be loaded
	article := ea.Get("Periwiki:Syntax")
	if article == nil {
		t.Fatal("expected Periwiki:Syntax to exist")
	}

	if !article.ReadOnly {
		t.Error("expected ReadOnly to be true")
	}

	if article.URL != "Periwiki:Syntax" {
		t.Errorf("expected URL 'Periwiki:Syntax', got %q", article.URL)
	}
}

func TestIsEmbeddedURL(t *testing.T) {
	tests := []struct {
		url      string
		expected bool
	}{
		{"Periwiki:Syntax", true},
		{"Periwiki:Help", true},
		{"periwiki:syntax", false}, // case-sensitive
		{"Regular-Article", false},
		{"Periwiki", false}, // no colon
		{"", false},
	}

	for _, tc := range tests {
		t.Run(tc.url, func(t *testing.T) {
			if got := embedded.IsEmbeddedURL(tc.url); got != tc.expected {
				t.Errorf("IsEmbeddedURL(%q) = %v, want %v", tc.url, got, tc.expected)
			}
		})
	}
}
