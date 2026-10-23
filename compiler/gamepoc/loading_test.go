package gamepoc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckLoadingImages(t *testing.T) {
	unity := t.TempDir()
	loading := Loading{Caption: "ЗАГРУЗКА…", MinMS: 600, Screens: map[string]LoadingScreen{"loading.a": {Image: "Loading/a", Context: "any"}}}
	err := checkLoadingImages(unity, loading)
	if err == nil || !strings.Contains(err.Error(), "loading.a") {
		t.Fatalf("missing picture must be rejected with the screen id, got %v", err)
	}
	dir := filepath.Join(unity, "Assets/Resources/Loading")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.jpg"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkLoadingImages(unity, loading); err != nil {
		t.Fatalf("existing picture must pass: %v", err)
	}
}

func TestClientLoadingIsSorted(t *testing.T) {
	c := Compiled{Loading: Loading{Caption: "X", MinMS: 600, Screens: map[string]LoadingScreen{
		"loading.b": {Image: "Loading/b", Context: "any"}, "loading.a": {Image: "Loading/a", Context: "new_game"}}}}
	screens := clientLoading(c)["screens"].([]map[string]any)
	if len(screens) != 2 || screens[0]["id"] != "loading.a" || screens[1]["id"] != "loading.b" {
		t.Fatalf("screens must be sorted by id: %v", screens)
	}
}

func TestCheckMenuImage(t *testing.T) {
	unity := t.TempDir()
	if err := checkMenuImage(unity, Menu{}); err != nil {
		t.Fatalf("empty menu must be allowed: %v", err)
	}
	if err := checkMenuImage(unity, Menu{Background: "Menu/background"}); err == nil || !strings.Contains(err.Error(), "main menu") {
		t.Fatalf("missing menu picture must be rejected, got %v", err)
	}
	dir := filepath.Join(unity, "Assets/Resources/Menu")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "background.png"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkMenuImage(unity, Menu{Background: "Menu/background"}); err != nil {
		t.Fatalf("existing picture must pass: %v", err)
	}
}
