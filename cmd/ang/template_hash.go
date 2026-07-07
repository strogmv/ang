package main

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"sort"

	angtemplates "github.com/strogmv/ang/templates"
)

// calculateEmbeddedTemplateHash fingerprints the templates that are compiled
// into the ANG binary. Hashing the configured on-disk directory is not enough:
// the emitter prefers the embedded filesystem and installed binaries may not
// have a templates directory at all.
func calculateEmbeddedTemplateHash() (string, error) {
	return calculateTemplateFSHash(angtemplates.FS)
}

func calculateTemplateFSHash(templateFS fs.FS) (string, error) {
	var paths []string
	if err := fs.WalkDir(templateFS, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		return "", fmt.Errorf("walk embedded templates: %w", err)
	}
	if len(paths) == 0 {
		return "", fmt.Errorf("embedded template filesystem is empty")
	}

	sort.Strings(paths)
	h := sha256.New()
	for _, path := range paths {
		data, err := fs.ReadFile(templateFS, path)
		if err != nil {
			return "", fmt.Errorf("read embedded template %s: %w", path, err)
		}
		_, _ = h.Write([]byte(path))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write(data)
		_, _ = h.Write([]byte{0})
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
