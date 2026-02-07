package periwiki

import (
	"io/fs"
	"testing"
	"testing/fstest"
)

func TestOverlayFS_DiskFileWins(t *testing.T) {
	base := fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("embedded")},
	}
	override := fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("disk")},
	}
	ofs := &overlayFS{base: base, override: override}

	f, err := ofs.Open("file.txt")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()

	data, err := fs.ReadFile(ofs, "file.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "disk" {
		t.Errorf("expected disk content, got %q", string(data))
	}
}

func TestOverlayFS_FallbackToEmbedded(t *testing.T) {
	base := fstest.MapFS{
		"file.txt": &fstest.MapFile{Data: []byte("embedded")},
	}
	override := fstest.MapFS{} // empty, no override

	ofs := &overlayFS{base: base, override: override}

	data, err := fs.ReadFile(ofs, "file.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "embedded" {
		t.Errorf("expected embedded content, got %q", string(data))
	}
}

func TestOverlayFS_DirectoryFromBase(t *testing.T) {
	base := fstest.MapFS{
		"dir/a.txt": &fstest.MapFile{Data: []byte("a")},
		"dir/b.txt": &fstest.MapFile{Data: []byte("b")},
	}
	override := fstest.MapFS{}

	ofs := &overlayFS{base: base, override: override}

	// Directory listings should come from base
	entries, err := fs.ReadDir(ofs, "dir")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}

func TestOverlayFS_DirectoryNotOverridden(t *testing.T) {
	base := fstest.MapFS{
		"dir/a.txt": &fstest.MapFile{Data: []byte("a")},
	}
	// Override has a "directory" (represented by having a file in it)
	override := fstest.MapFS{
		"dir/a.txt": &fstest.MapFile{Data: []byte("disk-a")},
	}

	ofs := &overlayFS{base: base, override: override}

	// Open the directory — should come from base (we skip disk dirs)
	f, err := ofs.Open("dir")
	if err != nil {
		t.Fatalf("Open dir: %v", err)
	}
	info, err := f.Stat()
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected dir")
	}
	f.Close()

	// But individual files should come from override
	data, err := fs.ReadFile(ofs, "dir/a.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "disk-a" {
		t.Errorf("expected disk-a, got %q", string(data))
	}
}

func TestListContentFiles(t *testing.T) {
	// ListContentFiles walks embeddedFS, so we can just verify it returns entries
	files, err := ListContentFiles()
	if err != nil {
		t.Fatalf("ListContentFiles: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("expected at least one content file")
	}

	// Check for known embedded files
	found := map[string]bool{}
	for _, f := range files {
		found[f.Path] = true
		if f.Source != "embedded" && f.Source != "disk" {
			t.Errorf("unexpected source %q for %s", f.Source, f.Path)
		}
	}

	for _, expected := range []string{
		"help/Syntax.md",
		"static/main.css",
		"templates/layouts/index.html",
	} {
		if !found[expected] {
			t.Errorf("expected %q in content files", expected)
		}
	}

	// Files should be sorted
	for i := 1; i < len(files); i++ {
		if files[i].Path < files[i-1].Path {
			t.Errorf("files not sorted: %q before %q", files[i-1].Path, files[i].Path)
			break
		}
	}
}

func TestContentOverrides(t *testing.T) {
	entries := []ContentFileEntry{
		{Path: "a.txt", Source: "embedded"},
		{Path: "b.txt", Source: "disk"},
		{Path: "c.txt", Source: "embedded"},
		{Path: "d.txt", Source: "disk"},
	}

	overrides := ContentOverrides(entries)
	if len(overrides) != 2 {
		t.Fatalf("expected 2 overrides, got %d", len(overrides))
	}
	if overrides[0].Path != "b.txt" || overrides[1].Path != "d.txt" {
		t.Errorf("unexpected overrides: %v", overrides)
	}
}
