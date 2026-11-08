package gamepoc

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGodotPublicPOGolden(t *testing.T) {
	c := localizationFixture()
	c.Messages = append(c.Messages, LocalizationMessage{Key: "ui.escape", Context: "Quotes and lines\nTranslator note", Texts: map[string]string{"ru": "\"RU\"\n{{literal}} {0}", "en": "\"EN\"\n{{literal}} {0}"}})
	delete(c.Messages[2].Texts, "ru")
	got, err := godotPO(c)
	if err != nil {
		t.Fatal(err)
	}
	for locale, raw := range got {
		golden := filepath.Join("testdata", "godot-"+locale+".po")
		expected, err := os.ReadFile(golden)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(raw, expected) {
			t.Fatalf("%s differs from golden:\n%s", locale, raw)
		}
		if bytes.Contains(raw, []byte("SECRET")) || bytes.Contains(raw, []byte("secret_room")) {
			t.Fatal("private world text leaked")
		}
	}
	c.Messages[0], c.Messages[3] = c.Messages[3], c.Messages[0]
	c.Locales[0], c.Locales[1] = c.Locales[1], c.Locales[0]
	again, err := godotPO(c)
	if err != nil {
		t.Fatal(err)
	}
	for locale, raw := range got {
		if !bytes.Equal(raw, again[locale]) {
			t.Fatal("nondeterministic PO")
		}
	}
	c.Messages[0].Texts["en"] = "No argument"
	if _, err := godotPO(c); err == nil {
		t.Fatal("placeholder inconsistency accepted")
	}
}

func TestGodotTranslationRegistration(t *testing.T) {
	c := localizationFixture()
	root := t.TempDir()
	target := filepath.Join(root, "client")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	original := "config_version=5\n[application]\nconfig/name=\"Example\"\n[internationalization]\nlocale/fallback=\"de\"\n[rendering]\nrenderer/rendering_method=\"gl_compatibility\"\n"
	if err := os.WriteFile(filepath.Join(target, "project.godot"), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := writeGodotTranslations(root, []string{"client"}, c); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(filepath.Join(target, "project.godot"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(raw), "locale/translations=") != 1 || !strings.Contains(string(raw), "config/name=\"Example\"") || !strings.Contains(string(raw), "locale/fallback=\"en\"") {
		t.Fatalf("registration: %s", raw)
	}
	for _, locale := range c.Locales {
		if _, err := os.Stat(filepath.Join(target, "generated/translations", locale+".po")); err != nil {
			t.Fatal(err)
		}
	}
	if string(raw) != godotProjectLocales(string(raw), c) {
		t.Fatal("registration not idempotent")
	}
	if err := writeGodotTranslations(root, []string{"missing"}, c); err == nil {
		t.Fatal("missing project silently accepted")
	}
}
