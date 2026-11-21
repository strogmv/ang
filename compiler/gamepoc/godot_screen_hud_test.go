package gamepoc

import (
	"strings"
	"testing"
)

// The chip HUD (design/hud/v1): one persistent screen named by routes.hud, layout hud, never opened by the server.
func TestGodotScreenPersistentHud(t *testing.T) {
	root, c, d := screenFixture(t)
	s := &c.GodotScreens.Screens[0]
	s.Persistent, s.Layout = true, "hud"
	c.GodotScreens.Routes = map[string]string{"hud": "bag"}
	if _, err := compileGodotScreens(root, c, d); err != nil {
		t.Fatal(err)
	}
	s.Layout = "page"
	if _, err := compileGodotScreens(root, c, d); err == nil || !strings.Contains(err.Error(), "field persistent") {
		t.Fatalf("a persistent page passed: %v", err)
	}
	s.Layout = "hud"
	delete(c.GodotScreens.Routes, "hud")
	if _, err := compileGodotScreens(root, c, d); err == nil || !strings.Contains(err.Error(), "routes.hud") {
		t.Fatalf("a persistent screen without routes.hud passed: %v", err)
	}
	s.Persistent = false
	c.GodotScreens.Routes = map[string]string{"hud": "bag"}
	if _, err := compileGodotScreens(root, c, d); err == nil || !strings.Contains(err.Error(), "routes.hud") {
		t.Fatalf("routes.hud on a screen that is not persistent passed: %v", err)
	}
}

// A live widget updates in place: only a custom widget may be live.
func TestGodotScreenLiveOnlyOnCustom(t *testing.T) {
	root, c, d := screenFixture(t)
	c.GodotScreens.Screens[0].Controls[0].Live = true
	if _, err := compileGodotScreens(root, c, d); err == nil || !strings.Contains(err.Error(), "controls.items.live") {
		t.Fatalf("a live list passed: %v", err)
	}
}

// host.world: the HUD's touches of the world take one of the declared verbs, as a literal.
func TestGodotScreenHostWorldVerbs(t *testing.T) {
	root, c, d := screenFixture(t)
	a := c.GodotScreens.Screens[0].Controls[0].Children[0].Action
	a.Command, a.Params = "host.world", map[string]ScreenValue{"verb": {Value: "use"}}
	if _, err := compileGodotScreens(root, c, d); err != nil {
		t.Fatal(err)
	}
	a.Params = map[string]ScreenValue{"verb": {Value: "fly"}}
	if _, err := compileGodotScreens(root, c, d); err == nil || !strings.Contains(err.Error(), "params.verb: unknown verb") {
		t.Fatalf("an unknown verb passed: %v", err)
	}
}

// A custom widget's literal data (its look, a deck's slots): every *_key is a public text in every locale.
func TestGodotScreenCustomValueKeys(t *testing.T) {
	root, c, d := screenFixture(t)
	c.GodotScreens.Widgets = map[string]string{"deck": "res://deck.gd"}
	c.GodotScreens.Screens[0].Controls = append(c.GodotScreens.Screens[0].Controls, ScreenControl{ID: "deck", Kind: "custom",
		Widget: "deck", Passive: true, FontPX: 19, WidthPX: 900, Lines: 8,
		Bind: map[string]ScreenValue{"decks": {Value: []any{map[string]any{"text_key": "ui.test.title",
			"slots": []any{map[string]any{"hint_key": "ui.test.action", "icon": "eye"}}}}}}})
	if _, err := compileGodotScreens(root, c, d); err != nil {
		t.Fatal(err)
	}
	c.GodotScreens.Screens[0].Controls[1].Bind["decks"] = ScreenValue{Value: []any{map[string]any{"text_key": "ui.test.title",
		"slots": []any{map[string]any{"hint_key": "ui.test.absent"}}}}}
	if _, err := compileGodotScreens(root, c, d); err == nil || !strings.Contains(err.Error(), "bind.decks") ||
		!strings.Contains(err.Error(), "hint_key") {
		t.Fatalf("a missing key in a widget's data passed: %v", err)
	}
}

// A control may read the chosen row of a list that stands after it (the zone card reads the chosen command).
func TestGodotScreenSelectedOfALaterList(t *testing.T) {
	root, c, d := screenFixture(t)
	c.GodotScreens.Widgets = map[string]string{"card": "res://card.gd"}
	s := &c.GodotScreens.Screens[0]
	s.Controls = append([]ScreenControl{{ID: "card", Kind: "custom", Widget: "card", Passive: true, FontPX: 19, WidthPX: 900,
		Lines: 8, Bind: map[string]ScreenValue{"chosen": {Path: "selected.items.id"}}}}, s.Controls...)
	if _, err := compileGodotScreens(root, c, d); err != nil {
		t.Fatal(err)
	}
}
