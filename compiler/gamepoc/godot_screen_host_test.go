package gamepoc

import (
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
	inv "github.com/strogmv/ang/compiler/inventory/runtime"
)

func TestGodotScreenHostCommandValidation(t *testing.T) {
	for _, command := range []string{"host.open", "host.setting", "host.toggle", "host.close", "host.back", "host.main", "host.quit", "host.delete_save"} {
		t.Run(command, func(t *testing.T) {
			root, c, d := screenFixture(t)
			a := c.GodotScreens.Screens[0].Controls[0].Children[0].Action
			a.Command = command
			a.Params = map[string]ScreenValue{"typo": {Value: "wrong"}}
			_, err := compileGodotScreens(root, c, d)
			// Every host command rejects unknown arguments before checking required ones.
			if err == nil || !strings.Contains(err.Error(), "screen bag field controls.take.action.params.typo: unknown host parameter") {
				t.Fatalf("unknown params passed: %v", err)
			}
		})
	}
	for _, test := range []struct{ command, param, kind string }{{"host.open", "screen", "screen"}, {"host.setting", "id", "setting"}, {"host.toggle", "control", "control"}} {
		t.Run(test.kind, func(t *testing.T) {
			root, c, d := screenFixture(t)
			a := c.GodotScreens.Screens[0].Controls[0].Children[0].Action
			a.Command = test.command
			a.Params = map[string]ScreenValue{test.param: {Value: "typo"}}
			_, err := compileGodotScreens(root, c, d)
			expected := "screen bag field controls.take.action.params." + test.param + ": unknown " + test.kind
			if err == nil || !strings.Contains(err.Error(), expected) {
				t.Fatalf("wanted %s: %v", expected, err)
			}
		})
	}
}
func TestGodotScreenItemRequiresEveryLocale(t *testing.T) {
	root, c, d := screenFixture(t)
	c.Inventory.Definitions = map[string]inv.Definition{"item.fixture": {ID: "item.fixture", Name: "ui.test.title"}}
	delete(c.Localization.Messages[0].Texts, "en")
	_, err := compileGodotScreens(root, c, d)
	if err == nil || !strings.Contains(err.Error(), "item item.fixture field name:") {
		t.Fatalf("missing item translation must name the item: %v", err)
	}
}

// A save card's held delete (the load screen, owner 19.11): host.delete_save is a declared, checked action of the custom
// widget, named beside its load, and its slot is read from the card's row, never written in.
func TestGodotScreenDeleteSaveReadsTheSlotFromTheRow(t *testing.T) {
	cards := func(t *testing.T, params map[string]ScreenValue) (string, Compiled, *normalizer.GRPCDef, *ScreenControl) {
		root, c, d := screenFixture(t)
		c.GodotScreens.Widgets = map[string]string{"cards": "res://cards.gd"}
		list := &c.GodotScreens.Screens[0].Controls[0]
		list.Kind, list.Widget, list.ActionSource, list.Children = "custom", "cards", "source", nil
		list.Action = &ScreenAction{Command: "inventory_command", Params: map[string]ScreenValue{"item_id": {Path: "row.id"}}}
		list.Actions = map[string]*ScreenAction{"delete": {Command: "host.delete_save", Params: params}}
		c.GodotScreens.Screens[0].Focus = []string{"items"}
		return root, c, d, list
	}
	root, c, d, list := cards(t, map[string]ScreenValue{"slot": {Path: "row.id"}})
	out, err := compileGodotScreens(root, c, d)
	if err != nil {
		t.Fatal(err)
	}
	ref := list.Actions["delete"].Params["slot"].Ref
	if ref == "" || !strings.Contains(string(out), "static func _read_"+ref+"(") || !strings.Contains(string(out), "(row as C.Item).id") ||
		!strings.Contains(string(out), `\"actions\":{\"delete\":{\"command\":\"host.delete_save\"`) {
		t.Fatalf("the delete's slot is not a typed read of the card's row (ref %q)", ref)
	}
	for _, test := range []struct {
		params map[string]ScreenValue
		want   string
	}{
		{map[string]ScreenValue{"slot": {Value: "slot1"}}, "controls.items.actions.delete.params.slot: expected a binding to a row's string"},
		{map[string]ScreenValue{"slot": {Path: "state.inventory"}}, "controls.items.actions.delete.params.slot: Inventory cannot bind a string"},
		{map[string]ScreenValue{}, "controls.items.actions.delete.params.slot: required"},
		{map[string]ScreenValue{"slot": {Path: "row.id"}, "screen": {Value: "bag"}}, "controls.items.actions.delete.params.screen: unknown host parameter"},
	} {
		root, c, d, _ := cards(t, test.params)
		if _, err := compileGodotScreens(root, c, d); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("wanted %s: %v", test.want, err)
		}
	}
	// named actions belong to a custom widget only
	root, c, d, list = cards(t, map[string]ScreenValue{"slot": {Path: "row.id"}})
	list.Kind = "list"
	if _, err := compileGodotScreens(root, c, d); err == nil || !strings.Contains(err.Error(), "controls.items.actions: only a custom widget takes named actions") {
		t.Fatalf("named actions on a list: %v", err)
	}
}

// The painted settings screen (25.11): ← and → step a setting back and forth through host.setting's optional direction,
// a literal 1 or -1; without it the step goes forward, as Enter does.
func TestGodotScreenSettingDirection(t *testing.T) {
	setting := func(t *testing.T, params map[string]ScreenValue) (Compiled, string, error) {
		root, c, d := screenFixture(t)
		c.GodotSettings = &GodotSettings{Appliers: map[string]SettingApplier{"bus": {Script: "res://appliers.gd", Method: "audio_bus"}}, Profiles: map[string]any{}, Entries: []GodotSetting{{ID: "volume", Section: "audio", Key: "volume", Kind: "stepped_number", Min: 0, Max: 1, Step: .25, Default: float64(0), Label: "ui.test.title", ValueKey: "ui.test.action", DisplayScale: 100, Applier: "bus", Params: map[string]any{"bus": "Master"}}}}
		a := c.GodotScreens.Screens[0].Controls[0].Children[0].Action
		a.Command = "host.setting"
		a.Params = params
		out, err := compileGodotScreens(root, c, d)
		return c, string(out), err
	}
	for _, step := range []float64{1, -1} {
		_, out, err := setting(t, map[string]ScreenValue{"id": {Value: "volume"}, "direction": {Value: step}})
		if err != nil {
			t.Fatalf("direction %v: %v", step, err)
		}
		if !strings.Contains(out, `\"direction\":{\"value\":`) {
			t.Fatalf("direction %v is not in the catalog", step)
		}
	}
	if _, _, err := setting(t, map[string]ScreenValue{"id": {Value: "volume"}}); err != nil {
		t.Fatalf("a step without a direction must stay valid: %v", err)
	}
	for _, bad := range []ScreenValue{{Value: float64(2)}, {Value: float64(0)}, {Value: "back"}, {Path: "state.epoch"}} {
		_, _, err := setting(t, map[string]ScreenValue{"id": {Value: "volume"}, "direction": bad})
		if err == nil || !strings.Contains(err.Error(), "controls.take.action.params.direction: expected 1 or -1") {
			t.Fatalf("direction %+v passed: %v", bad, err)
		}
	}
}
