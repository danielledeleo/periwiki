package embedded_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/danielledeleo/periwiki/internal/embedded"
)

func projectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Dir(filepath.Dir(filepath.Dir(filename)))
}

func TestNew(t *testing.T) {
	// Simple render function for testing
	render := func(md string) (string, error) {
		return "<p>rendered</p>", nil
	}

	ea, err := embedded.New(os.DirFS(projectRoot()), render)
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

func TestRenderAll(t *testing.T) {
	initialRender := func(md string) (string, error) {
		return "<p>initial</p>", nil
	}

	ea, err := embedded.New(os.DirFS(projectRoot()), initialRender)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Verify initial HTML
	article := ea.Get("Periwiki:Syntax")
	if article == nil {
		t.Fatal("expected Periwiki:Syntax to exist")
	}
	if article.HTML != "<p>initial</p>" {
		t.Fatalf("expected initial HTML, got %q", article.HTML)
	}

	t.Run("updates all articles with new render function", func(t *testing.T) {
		newRender := func(md string) (string, error) {
			return "<p>rerendered</p>", nil
		}

		if err := ea.RenderAll(newRender); err != nil {
			t.Fatalf("RenderAll failed: %v", err)
		}

		// Every article should now have the new HTML
		for _, url := range ea.List() {
			a := ea.Get(url)
			if a.HTML != "<p>rerendered</p>" {
				t.Errorf("%s: expected rerendered HTML, got %q", url, a.HTML)
			}
		}
	})

	t.Run("preserves original markdown", func(t *testing.T) {
		article := ea.Get("Periwiki:Syntax")
		if article.Markdown == "" {
			t.Error("expected non-empty markdown after RenderAll")
		}
	})

	t.Run("propagates render errors", func(t *testing.T) {
		failRender := func(md string) (string, error) {
			return "", errors.New("render failed")
		}

		err := ea.RenderAll(failRender)
		if err == nil {
			t.Error("expected error from RenderAll")
		}
	})
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
