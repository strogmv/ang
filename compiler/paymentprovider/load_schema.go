package paymentprovider

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue/load"
)

func schemaOverlay(cueDir, schemaDir string) (map[string]load.Source, error) {
	schemaDir = strings.TrimSpace(schemaDir)
	if schemaDir == "" {
		return nil, nil
	}
	cueDir, err := filepath.Abs(cueDir)
	if err != nil {
		return nil, fmt.Errorf("abs cue dir: %w", err)
	}
	schemaDir, err = filepath.Abs(schemaDir)
	if err != nil {
		return nil, fmt.Errorf("abs schema dir: %w", err)
	}
	info, err := os.Stat(schemaDir)
	if err != nil {
		return nil, fmt.Errorf("schema dir %q: %w", schemaDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("schema dir %q is not a directory", schemaDir)
	}
	entries, err := os.ReadDir(schemaDir)
	if err != nil {
		return nil, fmt.Errorf("read schema dir: %w", err)
	}
	overlay := make(map[string]load.Source)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".cue") {
			continue
		}
		srcPath := filepath.Join(schemaDir, e.Name())
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return nil, fmt.Errorf("read schema %s: %w", srcPath, err)
		}
		// CUE resolves import "…/schema" from <moduleRoot>/schema/*.cue — overlay
		// must use paths under cueDir, not the external schema_dir location.
		virtualPath := filepath.Join(cueDir, "schema", e.Name())
		overlay[filepath.ToSlash(virtualPath)] = load.FromBytes(data)
	}
	if len(overlay) == 0 {
		return nil, fmt.Errorf("schema dir %q has no .cue files", schemaDir)
	}
	return overlay, nil
}
