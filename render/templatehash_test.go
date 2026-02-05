package render

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHashRenderTemplates(t *testing.T) {
	dir := t.TempDir()

	// Create two files with known content.
	writeFile(t, filepath.Join(dir, "a.html"), "content-a")
	writeFile(t, filepath.Join(dir, "b.html"), "content-b")

	hash1, err := HashRenderTemplates(dir)
	if err != nil {
		t.Fatalf("HashRenderTemplates: %v", err)
	}
	if hash1 == "" {
		t.Fatal("expected non-empty hash")
	}

	// Same content produces the same hash.
	hash2, err := HashRenderTemplates(dir)
	if err != nil {
		t.Fatalf("HashRenderTemplates: %v", err)
	}
	if hash1 != hash2 {
		t.Errorf("expected deterministic hash, got %s and %s", hash1, hash2)
	}

	// Changing file content changes the hash.
	writeFile(t, filepath.Join(dir, "b.html"), "content-b-modified")
	hash3, err := HashRenderTemplates(dir)
	if err != nil {
		t.Fatalf("HashRenderTemplates: %v", err)
	}
	if hash3 == hash1 {
		t.Error("expected different hash after content change")
	}

	// Renaming a file changes the hash (path is included).
	os.Rename(filepath.Join(dir, "b.html"), filepath.Join(dir, "c.html"))
	hash4, err := HashRenderTemplates(dir)
	if err != nil {
		t.Fatalf("HashRenderTemplates: %v", err)
	}
	if hash4 == hash3 {
		t.Error("expected different hash after file rename")
	}
}

func TestHashRenderTemplatesEmptyDir(t *testing.T) {
	dir := t.TempDir()

	hash, err := HashRenderTemplates(dir)
	if err != nil {
		t.Fatalf("HashRenderTemplates: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash even for empty directory")
	}
}

func TestHashRenderTemplatesNonexistentDir(t *testing.T) {
	_, err := HashRenderTemplates("/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}
