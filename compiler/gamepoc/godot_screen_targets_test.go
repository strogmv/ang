package gamepoc

import (
	"github.com/strogmv/ang/angir/normalizer"
	"strings"
	"testing"
)

func targetFixture(t *testing.T) (string, Compiled, *normalizer.GRPCDef) {
	root, c, d := screenFixture(t)
	c.GodotScreens.Screens[0].Controls[0].RowKey = "id"
	target := ScreenTarget{ID: "stall", Source: ScreenValue{Path: "state.inventory.items"}, Screen: "bag", List: "items", RowKey: "id", RadiusPX: 52, Bind: map[string]ScreenValue{}}
	fields := map[string]string{"id": "string", "label": "string", "x": "int32", "y": "int32", "walk_x": "int32", "walk_y": "int32", "walk_deck": "int32", "visible": "bool", "reachable": "bool", "routable": "bool", "world_x": "double", "world_y": "double", "world_z": "double", "use_world": "bool"}
	for _, key := range sortedScreenKeys(fields) {
		if key != "id" {
			d.Messages[2].Fields = append(d.Messages[2].Fields, normalizer.GRPCField{Name: key, Type: fields[key]})
		}
		target.Bind[key] = ScreenValue{Path: "row." + key}
	}
	c.GodotScreens.Targets = []ScreenTarget{target}
	return root, c, d
}
func TestGodotTargetSecondLayoutByData(t *testing.T) {
	root, c, d := targetFixture(t)
	first, err := compileGodotScreens(root, c, d)
	if err != nil {
		t.Fatal(err)
	}
	c.GodotScreens.Targets[0].ID = "another_location_terminal"
	c.GodotScreens.Targets[0].Source.Path = "state.inventory.other_layout"
	d.Messages[1].Fields = append(d.Messages[1].Fields, normalizer.GRPCField{Name: "other_layout", Type: "Item", Repeated: true})
	second, err := compileGodotScreens(root, c, d)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(second), "state.inventory.other_layout") || string(first) == string(second) {
		t.Fatal("second data layout did not generate typed target bindings")
	}
}
func TestGodotTargetsNegative(t *testing.T) {
	for _, test := range []struct {
		name, field string
		mutate      func(*ScreenTarget)
	}{
		{"source", "source", func(t *ScreenTarget) { t.Source.Path = "state.inventory.typo" }},
		{"id", "id", func(t *ScreenTarget) { t.ID = "" }},
		{"position", "bind.x", func(t *ScreenTarget) { t.Bind["x"] = ScreenValue{Path: "row.typo"} }},
		{"label", "bind.label", func(t *ScreenTarget) { t.Bind["label"] = ScreenValue{Path: "row.typo"} }},
		{"reach", "bind.reachable", func(t *ScreenTarget) { t.Bind["reachable"] = ScreenValue{Path: "row.x"} }},
		{"required", "bind.walk_x", func(t *ScreenTarget) { delete(t.Bind, "walk_x") }},
		{"unknown binding", "bind.typo", func(t *ScreenTarget) { t.Bind["typo"] = ScreenValue{Path: "row.x"} }},
		{"destination", "screen", func(t *ScreenTarget) { t.Screen = "typo" }},
		{"list", "list", func(t *ScreenTarget) { t.List = "typo" }},
		{"row key", "row_key", func(t *ScreenTarget) { t.RowKey = "typo" }},
		{"position shape", "position", func(t *ScreenTarget) { t.Position = "typo" }},
		{"actor source", "bind.actor", func(t *ScreenTarget) { t.Position = "actor" }},
		{"id path", "bind.id", func(t *ScreenTarget) { t.Bind["id"] = ScreenValue{Path: "row.typo"} }},
		{"walk deck", "bind.walk_deck", func(t *ScreenTarget) { t.Bind["walk_deck"] = ScreenValue{Path: "row.typo"} }},
		{"radius", "radius_px", func(t *ScreenTarget) { t.RadiusPX = 0 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, c, d := targetFixture(t)
			test.mutate(&c.GodotScreens.Targets[0])
			_, err := compileGodotScreens(root, c, d)
			if err == nil || !strings.Contains(err.Error(), "field targets.") || !strings.Contains(err.Error(), test.field) {
				t.Fatalf("expected target field %s: %v", test.field, err)
			}
		})
	}
}

func TestGodotTargetEveryBindingRejectsUnknownPath(t *testing.T) {
	for _, field := range []string{
		"id", "label", "x", "y", "walk_x", "walk_y", "walk_deck",
		"visible", "reachable", "routable", "world_x", "world_y", "world_z", "use_world",
		"actor", "text", "fragments",
	} {
		t.Run(field, func(t *testing.T) {
			root, c, contract := targetFixture(t)
			c.GodotScreens.Targets[0].Bind[field] = ScreenValue{Path: "row.typo"}
			_, err := compileGodotScreens(root, c, contract)
			want := "screen stall field targets.stall.bind." + field + ":"
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("wanted %s: %v", want, err)
			}
		})
	}
}

func TestGodotWorldInputNegative(t *testing.T) {
	for _, test := range []struct {
		field  string
		mutate func(*ScreenWorldInput)
	}{
		{"peaceful.key", func(i *ScreenWorldInput) { i.Peaceful.Key = "typo" }},
		{"peaceful.pad", func(i *ScreenWorldInput) { i.Peaceful.Pad = "typo" }},
		{"hostile.key", func(i *ScreenWorldInput) { i.Hostile.Key = "typo" }},
		{"hostile.pad", func(i *ScreenWorldInput) { i.Hostile.Pad = "typo" }},
		{"engage_list", func(i *ScreenWorldInput) { i.EngageList = "typo" }},
		{"engage_key", func(i *ScreenWorldInput) { i.EngageKey = "typo" }},
		{"engage_screen", func(i *ScreenWorldInput) { i.EngageScreen = "typo" }},
		{"fallback", func(i *ScreenWorldInput) { i.Fallback = "typo" }},
	} {
		t.Run(test.field, func(t *testing.T) {
			root, c, d := targetFixture(t)
			c.GodotScreens.WorldInput = &ScreenWorldInput{Peaceful: ScreenShortcut{Key: "E", Pad: "X"}, Hostile: ScreenShortcut{Key: "F", Pad: "R1"}, Fallback: "bag", EngageScreen: "bag", EngageList: "items", EngageKey: "id"}
			test.mutate(c.GodotScreens.WorldInput)
			_, err := compileGodotScreens(root, c, d)
			if err == nil || !strings.Contains(err.Error(), "field world_input."+test.field) {
				t.Fatalf("expected field %s: %v", test.field, err)
			}
		})
	}
}
