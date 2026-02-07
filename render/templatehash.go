package render

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

// HashRenderTemplates computes a SHA-256 hash of all files under dir within
// the given filesystem. Files are processed in sorted order so the hash is
// deterministic.
func HashRenderTemplates(fsys fs.FS, dir string) (string, error) {
	var paths []string
	err := fs.WalkDir(fsys, dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walking render templates dir: %w", err)
	}

	sort.Strings(paths)

	h := sha256.New()
	for _, path := range paths {
		// Include the relative path in the hash so renames are detected.
		// Trim the dir prefix to get a consistent relative name.
		rel := strings.TrimPrefix(path, dir+"/")
		h.Write([]byte(rel))

		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return "", fmt.Errorf("reading %s: %w", path, err)
		}
		h.Write(data)
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
