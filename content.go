package periwiki

import (
	"embed"
	"io/fs"
	"os"
	"sort"
)

//go:embed all:templates static internal/storage/schema.sql
var embeddedFS embed.FS

// overlayFS implements fs.FS with disk overrides for individual files.
// Directory listings always come from the embedded FS; individual file
// opens check the disk first.
type overlayFS struct {
	base     fs.FS
	override fs.FS
}

func (o *overlayFS) Open(name string) (fs.File, error) {
	if f, err := o.override.Open(name); err == nil {
		if info, err := f.Stat(); err == nil && !info.IsDir() {
			return f, nil // Disk file wins
		}
		f.Close()
	}
	return o.base.Open(name) // Embedded fallback
}

// ContentFS is the filesystem used for all content access (templates, static
// assets, schema). It layers on-disk files over the embedded defaults, allowing
// per-file overrides without a rebuild.
var ContentFS fs.FS = &overlayFS{base: embeddedFS, override: os.DirFS(".")}

// ContentFileEntry describes a single file in the content filesystem.
type ContentFileEntry struct {
	Path   string // e.g. "templates/layouts/index.html"
	Source string // "embedded" or "disk"
}

// ListContentFiles walks the embedded filesystem and checks each file for a
// disk override. The returned list is sorted by path.
func ListContentFiles() ([]ContentFileEntry, error) {
	var entries []ContentFileEntry
	err := fs.WalkDir(embeddedFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		source := "embedded"
		if info, statErr := os.Stat(path); statErr == nil && !info.IsDir() {
			source = "disk"
		}
		entries = append(entries, ContentFileEntry{Path: path, Source: source})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})
	return entries, nil
}

// ContentOverrides returns only the files that have disk overrides.
func ContentOverrides(entries []ContentFileEntry) []ContentFileEntry {
	var overrides []ContentFileEntry
	for _, e := range entries {
		if e.Source == "disk" {
			overrides = append(overrides, e)
		}
	}
	return overrides
}

