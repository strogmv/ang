package emitter

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
)

func TestPythonFastAPISnapshot(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	em := New(tmp, "", "templates")
	em.Version = "0.9.0"

	entities := []normalizer.Entity{
		{
			Name: "User",
			Fields: []normalizer.Field{
				{Name: "id", Type: "string"},
				{Name: "email", Type: "string"},
				{Name: "bio", Type: "string", IsOptional: true},
			},
		},
	}
	endpoints := []normalizer.Endpoint{
		{Method: "GET", Path: "/users/{id}", ServiceName: "User", RPC: "GetUser"},
		{Method: "POST", Path: "/users", ServiceName: "User", RPC: "GetUser"},
	}
	repos := []normalizer.Repository{
		{
			Name:   "UserRepository",
			Entity: "User",
			Finders: []normalizer.RepositoryFinder{
				{Name: "FindByEmail"},
			},
		},
	}
	project := &normalizer.ProjectDef{
		Name:    "Snapshot Service",
		Version: "0.2.0",
	}

	if err := em.EmitPythonFastAPIBackend(entities, endpoints, repos, project); err != nil {
		t.Fatalf("emit python fastapi backend: %v", err)
	}

	got := readAppSnapshot(t, filepath.Join(tmp, "app"))

	root := repoRootFrom(t, ".")
	snapPath := filepath.Join(root, "tests", "snapshots", "python_fastapi.app.json")
	if os.Getenv("UPDATE_PY_FASTAPI_SNAPSHOTS") == "1" {
		writeJSONSnapshot(t, snapPath, got)
		return
	}

	want := map[string]string{}
	data, err := os.ReadFile(snapPath)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatalf("parse snapshot: %v", err)
	}

	gotJSON, _ := json.MarshalIndent(got, "", "  ")
	wantJSON, _ := json.MarshalIndent(want, "", "  ")
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("python fastapi snapshot mismatch\nwant:\n%s\ngot:\n%s", string(wantJSON), string(gotJSON))
	}
}

func readAppSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		key := filepath.ToSlash(rel)
		out[key] = strings.TrimSpace(string(data))
		return nil
	})
	if err != nil {
		t.Fatalf("walk app snapshot: %v", err)
	}
	return out
}

func writeJSONSnapshot(t *testing.T, path string, snapshot map[string]string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir snapshot dir: %v", err)
	}
	keys := make([]string, 0, len(snapshot))
	for k := range snapshot {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	ordered := make(map[string]string, len(snapshot))
	for _, k := range keys {
		ordered[k] = snapshot[k]
	}
	data, err := json.MarshalIndent(ordered, "", "  ")
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}
}

func repoRootFrom(t *testing.T, start string) string {
	t.Helper()
	dir, err := filepath.Abs(start)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("repo root not found from %s", start)
		}
		dir = parent
	}
}

