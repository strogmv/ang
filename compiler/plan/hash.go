package plan

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func ComputeWorkspaceHash(root string) (string, error) {
	h := sha256.New()
	root = filepath.Clean(root)

	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if info.IsDir() {
			if rel == ".git" || strings.HasPrefix(rel, ".git/") {
				return filepath.SkipDir
			}
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return "", err
	}

	sort.Strings(files)
	for _, path := range files {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return "", err
		}
		if _, err := io.WriteString(h, filepath.ToSlash(rel)+"\n"); err != nil {
			return "", err
		}
		f, err := os.Open(path)
		if err != nil {
			return "", err
		}
		_, copyErr := io.Copy(h, f)
		closeErr := f.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
