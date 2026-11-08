package gamepoc

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestGodotThemeTypedTokensAndContrast(t *testing.T) {
	a, _ := godotRGB("#000000")
	b, _ := godotRGB("#ffffff")
	if ratio := godotContrast(a, b); ratio != 21 {
		t.Fatalf("black/white WCAG ratio %g", ratio)
	}
	theme := GodotTheme{MinTextPX: 12, Tokens: map[string]float64{"body": 18}, Properties: []GodotThemeProperty{{Name: "default_font_size", Token: "body"}, {Name: "Label/colors/font_color", Color: "ink"}}, Skins: []GodotThemeSkin{{ID: "example", Roles: map[string]string{"ink": "#302c22"}}}}
	got, err := theme.render(theme.Skins[0])
	if err != nil {
		t.Fatal(err)
	}
	expected := "[gd_resource type=\"Theme\" load_steps=1 format=3]\n\n[resource]\n\ndefault_font_size=18\n\nLabel/colors/font_color=Color(0.18824, 0.17255, 0.13333, 1.00000)\n"
	if string(got) != expected {
		t.Fatalf("Theme golden:\n%s", got)
	}
	again, err := theme.render(theme.Skins[0])
	if err != nil || !bytes.Equal(got, again) {
		t.Fatal("nondeterministic Theme")
	}
	theme.Tokens["body"] = 11
	if _, err := theme.render(theme.Skins[0]); err == nil {
		t.Fatal("Steam Deck text below minimum accepted")
	}
	theme.Tokens["body"] = 18
	delete(theme.Skins[0].Roles, "ink")
	if _, err := theme.render(theme.Skins[0]); err == nil {
		t.Fatal("missing palette role accepted")
	}
	theme.Properties[1] = GodotThemeProperty{Name: "Label/colors/font_color", External: "missing"}
	if _, err := theme.render(theme.Skins[0]); err == nil {
		t.Fatal("unknown external accepted")
	}
	theme.Properties[1] = GodotThemeProperty{Name: "Label/colors/font_color", Token: "body", StringName: "Label"}
	if _, err := theme.render(theme.Skins[0]); err == nil {
		t.Fatal("ambiguous value accepted")
	}
}
func TestGodotThemeAssetDigest(t *testing.T) {
	root := t.TempDir()
	raw := []byte("approved art bytes")
	sum := sha256.Sum256(raw)
	if err := os.WriteFile(filepath.Join(root, "art.png"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := godotThemeAssetCheck(root, "res://art.png", hex.EncodeToString(sum[:])); err != nil {
		t.Fatal(err)
	}
	if err := godotThemeAssetCheck(root, "res://art.png", "wrong"); err == nil {
		t.Fatal("changed art accepted")
	}
	if err := godotThemeAssetCheck(root, "res://../art.png", hex.EncodeToString(sum[:])); err == nil {
		t.Fatal("escaping asset path accepted")
	}
}

func TestGodotThemeWritesSkins(t *testing.T) {
	root := t.TempDir()
	theme := GodotTheme{Target: "client", ResourceName: "default_theme", DefaultSkin: "example", MinTextPX: 12, Tokens: map[string]float64{"body": 18}, Properties: []GodotThemeProperty{{Name: "default_font_size", Token: "body"}}, Skins: []GodotThemeSkin{{ID: "example", Roles: map[string]string{"ink": "#000000", "paper": "#ffffff"}}}, Contrast: []GodotThemeContrast{{Name: "body", Usage: "Label font over panel", Foreground: "ink", Background: "paper"}}}
	if err := writeGodotTheme(root, &theme); err != nil {
		t.Fatal(err)
	}
	a, err := os.ReadFile(filepath.Join(root, "client/theme/example.tres"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "client/theme/default_theme.tres"))
	if err != nil || !bytes.Equal(a, b) {
		t.Fatal("default Theme differs from default skin")
	}
	c, err := os.ReadFile(filepath.Join(root, "client/generated/theme-contrast.json"))
	if err != nil || !bytes.Contains(c, []byte(`"ratio": 21`)) {
		t.Fatalf("WCAG report %s: %v", c, err)
	}
	theme.MinTextPX = 11
	if err := writeGodotTheme(root, &theme); err == nil {
		t.Fatal("minimum weakened below Deck requirement")
	}
}
