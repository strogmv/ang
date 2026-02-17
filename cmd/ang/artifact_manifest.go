package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler"
)

const artifactManifestSchemaVersion = "artifact-manifest/v1"

type artifactHashRecord struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
}

type artifactHashManifest struct {
	SchemaVersion   string               `json:"schemaVersion"`
	CompilerVersion string               `json:"compilerVersion"`
	IRVersion       string               `json:"irVersion"`
	InputHash       string               `json:"inputHash,omitempty"`
	TemplateHash    string               `json:"templateHash,omitempty"`
	Artifacts       []artifactHashRecord `json:"artifacts"`
}

type artifactManifestTarget struct {
	Mode     string
	Backend  string
	Frontend string
}

func writeArtifactHashManifest(projectRoot string, targets []artifactManifestTarget, irVersion, inputHash, templateHash string) error {
	manifest, err := buildArtifactHashManifest(projectRoot, targets, irVersion, inputHash, templateHash)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal artifact manifest: %w", err)
	}
	data = append(data, '\n')
	manifestPath := filepath.Join(projectRoot, ".ang", "cache", "manifest.json")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0755); err != nil {
		return fmt.Errorf("mkdir artifact manifest dir: %w", err)
	}
	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		return fmt.Errorf("write artifact manifest: %w", err)
	}
	return nil
}

func buildArtifactHashManifest(projectRoot string, targets []artifactManifestTarget, irVersion, inputHash, templateHash string) (artifactHashManifest, error) {
	rootAbs, err := filepath.Abs(projectRoot)
	if err != nil {
		return artifactHashManifest{}, fmt.Errorf("abs project root: %w", err)
	}
	seenRoots := map[string]struct{}{}
	roots := make([]string, 0, len(targets)*2+8)
	addRoot := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(rootAbs, path)
		}
		path = filepath.Clean(path)
		if _, ok := seenRoots[path]; ok {
			return
		}
		seenRoots[path] = struct{}{}
		roots = append(roots, path)
	}

	for _, t := range targets {
		mode := strings.ToLower(strings.TrimSpace(t.Mode))
		backend := strings.TrimSpace(t.Backend)
		frontend := strings.TrimSpace(t.Frontend)
		if mode == "release" {
			addRoot(backend)
			addRoot(frontend)
			continue
		}
		backendAbs := backend
		if !filepath.IsAbs(backendAbs) {
			backendAbs = filepath.Join(rootAbs, backendAbs)
		}
		backendAbs = filepath.Clean(backendAbs)
		if backendAbs == rootAbs {
			// In in_place mode for Go target, hash only generator-owned artifacts.
			addRoot(filepath.Join(rootAbs, "cmd", "server"))
			addRoot(filepath.Join(rootAbs, "internal"))
			addRoot(filepath.Join(rootAbs, "api"))
			addRoot(filepath.Join(rootAbs, "db"))
			addRoot(filepath.Join(rootAbs, "deploy"))
			addRoot(filepath.Join(rootAbs, "sdk"))
			addRoot(filepath.Join(rootAbs, "ang-manifest.json"))
			addRoot(filepath.Join(rootAbs, "atlas.hcl"))
			addRoot(filepath.Join(rootAbs, "sqlc.yaml"))
			addRoot(frontend)
			continue
		}
		addRoot(backend)
		addRoot(frontend)
	}

	paths, err := collectArtifactFiles(rootAbs, roots)
	if err != nil {
		return artifactHashManifest{}, err
	}
	records := make([]artifactHashRecord, 0, len(paths))
	for _, rel := range paths {
		abs := filepath.Join(rootAbs, filepath.FromSlash(rel))
		h, err := fileSHA256(abs)
		if err != nil {
			return artifactHashManifest{}, fmt.Errorf("hash %s: %w", rel, err)
		}
		records = append(records, artifactHashRecord{
			Path: rel,
			Hash: h,
		})
	}
	return artifactHashManifest{
		SchemaVersion:   artifactManifestSchemaVersion,
		CompilerVersion: compiler.Version,
		IRVersion:       strings.TrimSpace(irVersion),
		InputHash:       strings.TrimSpace(inputHash),
		TemplateHash:    strings.TrimSpace(templateHash),
		Artifacts:       records,
	}, nil
}

func collectArtifactFiles(projectRoot string, roots []string) ([]string, error) {
	rootAbs, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		if !filepath.IsAbs(root) {
			root = filepath.Join(rootAbs, root)
		}
		root = filepath.Clean(root)
		st, err := os.Stat(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("stat %s: %w", root, err)
		}
		if st.IsDir() {
			var files []string
			if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					// Never include generated cache manifests in artifact hashes.
					if path == filepath.Join(rootAbs, ".ang") || strings.HasPrefix(path, filepath.Join(rootAbs, ".ang")+string(filepath.Separator)) {
						return filepath.SkipDir
					}
					return nil
				}
				files = append(files, path)
				return nil
			}); err != nil {
				return nil, fmt.Errorf("walk %s: %w", root, err)
			}
			sort.Strings(files)
			for _, path := range files {
				rel, err := filepath.Rel(rootAbs, path)
				if err != nil {
					return nil, err
				}
				rel = filepath.ToSlash(filepath.Clean(rel))
				seen[rel] = struct{}{}
			}
			continue
		}
		rel, err := filepath.Rel(rootAbs, root)
		if err != nil {
			return nil, err
		}
		rel = filepath.ToSlash(filepath.Clean(rel))
		seen[rel] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out, nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func readArtifactHashManifest(projectRoot string) (artifactHashManifest, error) {
	path := filepath.Join(projectRoot, ".ang", "cache", "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return artifactHashManifest{}, err
	}
	var m artifactHashManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return artifactHashManifest{}, err
	}
	return m, nil
}
