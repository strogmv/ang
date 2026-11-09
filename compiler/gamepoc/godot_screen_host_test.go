package gamepoc

import (
	inv "github.com/strogmv/ang/compiler/inventory/runtime"
	"strings"
	"testing"
)

func TestGodotScreenHostCommandValidation(t *testing.T) {
	for _, command := range []string{"host.open", "host.setting", "host.toggle", "host.close", "host.back", "host.main", "host.quit"} {
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
