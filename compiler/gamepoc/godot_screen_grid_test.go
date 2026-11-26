package gamepoc

import (
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
)

func gridFixture(t *testing.T) (string, Compiled, *normalizer.GRPCDef) {
	root, c, d := screenFixture(t)
	list := &c.GodotScreens.Screens[0].Controls[0]
	list.Action = list.Children[0].Action
	list.Children = []ScreenControl{{ID: "name", Kind: "label", FontPX: 19, WidthPX: 400, Lines: 8,
		Bind: map[string]ScreenValue{"text": {Path: "row.name"}}}}
	list.Mode = "commands"
	list.Columns = 3
	list.Filters = []ScreenFilter{{Value: ScreenValue{Path: "row.id"}, In: ScreenValue{Path: "state.inventory.allowed"}}}
	d.Messages[1].Fields = append(d.Messages[1].Fields, normalizer.GRPCField{
		Name: "allowed", Type: "string", Repeated: true,
	})
	c.GodotScreens.Screens[0].Focus = []string{"items"}
	c.InputActions = testInputActions()
	c.GodotScreens.Screens[0].Hotkeys = []ScreenHotkey{{Action: "choose_1", Control: "items"}}
	return root, c, d
}

func TestGodotScreenCommandGridIsData(t *testing.T) {
	root, c, d := gridFixture(t)
	out, err := compileGodotScreens(root, c, d)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"state.inventory.allowed", "(row as C.Item).id", "q.item_id = read("} {
		if !strings.Contains(string(out), expected) {
			t.Fatalf("missing typed grid binding %s", expected)
		}
	}
}

func TestGodotScreenGridNegative(t *testing.T) {
	for _, test := range []struct {
		name, field string
		mutate      func(*Compiled)
	}{
		{"columns", "controls.items.columns", func(c *Compiled) {
			c.GodotScreens.Screens[0].Controls[0].Columns = 10
		}},
		{"filter path", "controls.items.filters.0.in", func(c *Compiled) {
			c.GodotScreens.Screens[0].Controls[0].Filters[0].In.Path = "state.inventory.absent"
		}},
		{"filter shape", "controls.items.filters.0.in", func(c *Compiled) {
			c.GodotScreens.Screens[0].Controls[0].Filters[0].In.Path = "state.epoch"
		}},
		{"hotkey control", "hotkeys.0.control", func(c *Compiled) {
			c.GodotScreens.Screens[0].Hotkeys[0].Control = "absent"
		}},
		{"hotkey action", "hotkeys.0.action", func(c *Compiled) {
			c.GodotScreens.Screens[0].Hotkeys[0].Action = "absent"
		}},
		{"hotkey action without a key", "hotkeys.0.action", func(c *Compiled) {
			c.GodotScreens.Screens[0].Hotkeys[0].Action = "pad_only"
		}},
		{"initial focus", "initial_focus", func(c *Compiled) {
			c.GodotScreens.Screens[0].InitialFocus = "absent"
		}},
		{"status", "status_id", func(c *Compiled) {
			c.GodotScreens.Screens[0].StatusID = "absent"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, c, d := gridFixture(t)
			test.mutate(&c)
			_, err := compileGodotScreens(root, c, d)
			if err == nil || !strings.Contains(err.Error(), "screen bag field "+test.field) {
				t.Fatalf("expected named field %s: %v", test.field, err)
			}
		})
	}
}

func TestGodotScreenRegionUsesViewport(t *testing.T) {
	root, c, d := screenFixture(t)
	region := ScreenControl{ID: "hud", Kind: "region", Rect: []float64{0, 0, 1280, 800},
		FontPX: 19, WidthPX: 1236, Lines: 8}
	c.GodotScreens.Screens[0].Controls = append(c.GodotScreens.Screens[0].Controls, region)
	if _, err := compileGodotScreens(root, c, d); err != nil {
		t.Fatal(err)
	}
	c.GodotScreens.Screens[0].Controls[1].Rect[2] = 1281
	if _, err := compileGodotScreens(root, c, d); err == nil || !strings.Contains(err.Error(), "controls.hud.rect") {
		t.Fatalf("viewport boundary is not checked: %v", err)
	}
}
