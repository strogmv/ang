package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureSDKTargetAcceptsMissingEmptyAndPreviousOutput(t *testing.T) {
	root := t.TempDir()
	if err := ensureSDKTarget(filepath.Join(root, "absent")); err != nil {
		t.Fatalf("missing dir: %v", err)
	}
	empty := filepath.Join(root, "empty")
	os.MkdirAll(empty, 0755)
	if err := ensureSDKTarget(empty); err != nil {
		t.Fatalf("empty dir: %v", err)
	}
	previous := filepath.Join(root, "previous")
	os.MkdirAll(previous, 0755)
	os.WriteFile(filepath.Join(previous, sdkManifestName), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(previous, "index.ts"), []byte(""), 0644)
	if err := ensureSDKTarget(previous); err != nil {
		t.Fatalf("previous output: %v", err)
	}
}

func TestEnsureSDKTargetRefusesProjectCheckout(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "app")
	os.MkdirAll(filepath.Join(app, "src"), 0755)
	os.MkdirAll(filepath.Join(app, ".git"), 0755)
	os.WriteFile(filepath.Join(app, "package.json"), []byte("{}"), 0644)
	err := ensureSDKTarget(app)
	if err == nil {
		t.Fatal("expected refusal for a project checkout")
	}
	for _, want := range []string{".git", "package.json", "src", filepath.Join("src", "@sdk")} {
		if !contains(err.Error(), want) {
			t.Errorf("error should mention %q: %v", want, err)
		}
	}
}

func TestEnsureSDKTargetRefusesUnknownContent(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "notes")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "todo.txt"), []byte("x"), 0644)
	if err := ensureSDKTarget(dir); err == nil {
		t.Fatal("expected refusal for a non-empty directory without a manifest")
	}
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
