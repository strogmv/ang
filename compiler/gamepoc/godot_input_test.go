package gamepoc

import (
	"strings"
	"testing"
)

func testInputLabel(id string) InputKeyLabel {
	return InputKeyLabel{Key: "ui.controls." + id, Context: "test", RU: id, EN: id}
}

// testInputActions: a small table of the shape of world.input_actions.
func testInputActions() *InputActions {
	return &InputActions{
		Section: "controls",
		Pads:    map[string]InputPad{"A": {Button: 0, Label: "A"}, "X": {Button: 2, Label: "X"}, "RB": {Button: 10, Label: "RB"}},
		KeyLabels: map[string]InputKeyLabel{
			"Space": {Key: "ui.key.space", Context: "test", RU: "Пробел", EN: "Space"},
		},
		Actions: []InputAction{
			{ID: "interact", Group: "world", Contexts: []string{"world", "fight"}, Keys: []string{"E"}, Pad: []string{"X"}, Remap: true, Label: testInputLabel("interact")},
			{ID: "attack", Group: "world", Contexts: []string{"world"}, Keys: []string{"F"}, Pad: []string{"RB"}, Remap: true, Label: testInputLabel("attack")},
			{ID: "choose_1", Group: "choices", Contexts: []string{"world", "inventory"}, Keys: []string{"1"}, Remap: true, Label: testInputLabel("choose_1")},
			{ID: "end_turn", Group: "combat", Contexts: []string{"fight"}, Keys: []string{"Space"}, Remap: true, Label: testInputLabel("end_turn")},
			{ID: "line_up", Group: "hero_file", Contexts: []string{"hero_file"}, Keys: []string{"Ctrl+Enter", "Meta+Kp Enter"}, Remap: true, Label: testInputLabel("line_up")},
			{ID: "pad_only", Group: "inventory", Contexts: []string{"inventory"}, Pad: []string{"X"}, Remap: true, Label: testInputLabel("pad_only")},
			{ID: "run", Group: "move", Contexts: []string{"world"}, Keys: []string{"Shift"}, Remap: true, Hold: true, Label: testInputLabel("run")},
			{ID: "perf", Group: "developer", Contexts: []string{"global"}, Keys: []string{"F8"}, Label: testInputLabel("perf")},
		},
		Screens:    map[string]string{"bag": "inventory"},
		Precedence: []InputPrecedence{{Context: "inventory", Device: "pad", Wins: []string{"pad_only"}, Reason: "test"}},
	}
}

// The table is checked alone: names, contexts, key and button names, and no two defaults on one key in a context.
func TestInputActionsTableNegative(t *testing.T) {
	if err := testInputActions().validate(); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, want string
		mutate     func(*InputActions)
	}{
		{"section", "section", func(a *InputActions) { a.Section = "" }},
		{"duplicate id", "actions.interact.id: invalid or duplicate", func(a *InputActions) { a.Actions[1].ID = "interact"; a.Actions[1].Contexts = []string{"menu"} }},
		{"bad id", "actions.Open-Bag.id", func(a *InputActions) { a.Actions[1].ID = "Open-Bag" }},
		{"group", "actions.attack.group", func(a *InputActions) { a.Actions[1].Group = "camera_zoom" }},
		{"no context", "actions.attack.contexts", func(a *InputActions) { a.Actions[1].Contexts = nil }},
		{"context", "actions.attack.contexts", func(a *InputActions) { a.Actions[1].Contexts = []string{"street"} }},
		{"key name", "actions.attack.keys", func(a *InputActions) { a.Actions[1].Keys = []string{"Escpe"} }},
		{"Godot's Mac name", "actions.attack.keys", func(a *InputActions) { a.Actions[1].Keys = []string{"Option"} }},
		{"modifier order", "actions.attack.keys", func(a *InputActions) { a.Actions[1].Keys = []string{"Ctrl+Shift+F"} }},
		{"modifier on itself", "actions.attack.keys", func(a *InputActions) { a.Actions[1].Keys = []string{"Shift+Shift"} }},
		{"button", "actions.attack.pad", func(a *InputActions) { a.Actions[1].Pad = []string{"R1"} }},
		{"nothing to press", "actions.attack: neither", func(a *InputActions) { a.Actions[1].Keys, a.Actions[1].Pad = nil, nil }},
		{"label", "actions.attack.label", func(a *InputActions) { a.Actions[1].Label.Key = "ui.attack" }},
		{"a key twice", "actions.attack: key:F twice", func(a *InputActions) { a.Actions[1].Keys = []string{"F", "F"} }},
		{"one key, one context", "key:E in the context world: interact, attack", func(a *InputActions) { a.Actions[1].Keys = []string{"E"} }},
		{"one button, one context", "pad:X in the context fight: interact, end_turn", func(a *InputActions) {
			a.Actions[3].Pad = []string{"X"}
		}},
		// a modifier bound alone to a held action is free in its contexts: Shift+F is the attack's too (Shift runs)
		{"a free modifier", "key:Shift+F in the context world: attack, choose_1", func(a *InputActions) {
			a.Actions[2].Keys = []string{"Shift+F"}
		}},
		{"a global key meets every context", "key:F8 in the context inventory: choose_1, perf", func(a *InputActions) {
			a.Actions[2].Keys = []string{"F8"}
		}},
		{"two winners", "pad:X in the context inventory: interact, pad_only", func(a *InputActions) {
			a.Actions[0].Contexts = []string{"world", "fight", "inventory"}
			a.Precedence[0].Wins = []string{"pad_only", "interact"}
		}},
		{"a winner of another context", "precedence.0.wins", func(a *InputActions) { a.Precedence[0].Wins = []string{"attack"} }},
		{"a precedence device", "precedence.0", func(a *InputActions) { a.Precedence[0].Device = "mouse" }},
		{"a screen's context", "screens.bag", func(a *InputActions) { a.Screens["bag"] = "street" }},
		{"pad index", "pads", func(a *InputActions) { a.Pads["B"] = InputPad{Button: 2, Label: "B"} }},
		{"key label", "key_labels", func(a *InputActions) { a.KeyLabels["Spacebar"] = InputKeyLabel{Key: "ui.key.space"} }},
	} {
		t.Run(test.name, func(t *testing.T) {
			a := testInputActions()
			test.mutate(a)
			err := a.validate()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("wanted %q: %v", test.want, err)
			}
		})
	}
	// the same key in two contexts that never meet is no conflict
	a := testInputActions()
	a.Actions[5].Keys = []string{"Space"}
	if err := a.validate(); err != nil {
		t.Fatalf("Space in the fight and in the inventory: %v", err)
	}
	// a winner takes the button it shares: X in the inventory is pad_only's, the interact's X is idle there
	a = testInputActions()
	a.Actions[0].Contexts = []string{"world", "fight", "inventory"}
	if err := a.validate(); err != nil {
		t.Fatalf("a precedence rule resolves the shared X: %v", err)
	}
	// Shift+Tab and Tab are two bindings; Ctrl is not free anywhere, so Ctrl+F does not meet F
	a = testInputActions()
	a.Actions[2].Keys = []string{"Ctrl+F"}
	if err := a.validate(); err != nil {
		t.Fatalf("Ctrl+F and F: %v", err)
	}
	if got := a.effectiveKeys(a.Actions[0], "E", a.freeModifiers()); strings.Join(got, ",") != "E,Shift+E" {
		t.Fatalf("the walk's E with the run's Shift held: %v", got)
	}
}

func inputCatalogFixture() Compiled {
	c := Compiled{InputActions: testInputActions()}
	c.Localization.Locales = []string{"ru", "en"}
	add := func(key, ru, en string) {
		c.Localization.Messages = append(c.Localization.Messages, LocalizationMessage{Key: key, Texts: map[string]string{"ru": ru, "en": en}})
	}
	for _, one := range c.InputActions.Actions {
		add(one.Label.Key, one.Label.RU, one.Label.EN)
	}
	add("ui.key.space", "Пробел", "Space")
	add("ui.hud.use", "Действие ({key:interact})", "Use ({key:interact})")
	add("ui.hud.end", "[{key:end_turn}] · {pad:interact}", "[{key:end_turn}] · {pad:interact}")
	add("ui.plain", "Без клавиш {0}", "No keys {0}")
	return c
}

// A hint names a real action with that kind of binding, only in a text the client receives; the client fills exactly
// the hinted texts; the fit checks measure the default binding in the locale.
func TestInputHintsAreChecked(t *testing.T) {
	c := inputCatalogFixture()
	if err := checkInputActions(&c); err != nil {
		t.Fatal(err)
	}
	if strings.Join(c.InputActions.Hinted, ",") != "ui.hud.end,ui.hud.use" {
		t.Fatalf("hinted %v", c.InputActions.Hinted)
	}
	if got := expandInputHints(c, "[{key:end_turn}] · {pad:interact}", "ru"); got != "[Пробел] · X" {
		t.Fatalf("ru %q", got)
	}
	if got := expandInputHints(c, "{key:line_up} — {key:end_turn}", "en"); got != "Ctrl+Enter — Space" {
		t.Fatalf("en %q", got)
	}
	for _, test := range []struct{ name, key, text, want string }{
		{"unknown action", "ui.bad", "{key:interact_typo}", "names no input action"},
		{"no key", "ui.bad", "{key:pad_only}", "has no default key"},
		{"no button", "ui.bad", "{pad:end_turn}", "has no default pad"},
		{"server text", "npc.line", "{key:interact}", "only ui.* texts"},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := inputCatalogFixture()
			c.Localization.Messages = append(c.Localization.Messages, LocalizationMessage{Key: test.key, Texts: map[string]string{"ru": test.text, "en": test.text}})
			if err := checkInputActions(&c); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("wanted %q: %v", test.want, err)
			}
		})
	}
	c = inputCatalogFixture()
	c.Localization.Messages = c.Localization.Messages[1:]
	if err := checkInputActions(&c); err == nil || !strings.Contains(err.Error(), "actions.interact.label") {
		t.Fatalf("an action's name must be in the catalog: %v", err)
	}
	c = inputCatalogFixture()
	c.CombatPace = &CombatPace{Action: "pace_typo"}
	if err := checkInputActions(&c); err == nil || !strings.Contains(err.Error(), "world.combat_pace.action") {
		t.Fatalf("the combat pace names a real action: %v", err)
	}
	c = inputCatalogFixture()
	c.dialogueDev.ToggleAction = "dev_typo"
	if err := checkInputActions(&c); err == nil || !strings.Contains(err.Error(), "developerInput.toggle_action") {
		t.Fatalf("the developer's toggle names a real action: %v", err)
	}
	c = inputCatalogFixture()
	c.InputActions = nil
	if err := checkInputActions(&c); err == nil || !strings.Contains(err.Error(), "no world.input_actions") {
		t.Fatalf("a hint without the table: %v", err)
	}
}

// The client's file carries the table, a constant per action and the shared runtime; the settings leave the input
// actions' section of the file alone.
func TestInputActionsGodotOutput(t *testing.T) {
	c := inputCatalogFixture()
	if err := checkInputActions(&c); err != nil {
		t.Fatal(err)
	}
	out, err := compileGodotInput(c)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	for _, want := range []string{"const SECTION := \"controls\"", "const INTERACT := &\"interact\"", "const CHOOSE_1 := &\"choose_1\"",
		"static func install(file: ConfigFile)", "static func _static_init() -> void:\n\tensure()", "static func set_override(", "static func conflicts()", "\\\"hinted\\\":[\\\"ui.hud.end\\\",\\\"ui.hud.use\\\"]",
		"event.is_action_pressed(id, echo, true)", "InputMap.action_add_event", "static func typing() -> bool:", "static func canonical(", "_yields(event, id, context)", "return event.is_action_released(id, false)", "return event.is_action(id, false)"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %s", want)
		}
	}
	if none, err := compileGodotInput(Compiled{}); none != nil || err != nil {
		t.Fatalf("a world without the table writes no file: %v", err)
	}
	root, sc, d := settingsFixture(t)
	sc.InputActions = testInputActions()
	settings, err := compileGodotSettings(root, sc, d)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(settings), `\"foreign\":[\"controls\"]`) || !strings.Contains(string(settings), "foreign") {
		t.Fatalf("the settings do not leave the controls section alone")
	}
	sc.GodotSettings.Entries[0].Section = "controls"
	if _, err := compileGodotSettings(root, sc, d); err == nil || !strings.Contains(err.Error(), "field section") {
		t.Fatalf("a setting in the input actions' section: %v", err)
	}
}
