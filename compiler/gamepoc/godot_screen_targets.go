package gamepoc

import (
	"fmt"
	"math"
	"strings"
)

// Target bindings use the ordinary typed catalog reader, including nil guards.
func (g *screenCompiler) targets(s *GodotScreens) error {
	seen := map[string]bool{}
	types := map[string]string{"id": "string", "label": "string", "x": "int32", "y": "int32",
		"walk_x": "int32", "walk_y": "int32", "walk_deck": "int32", "visible": "bool", "reachable": "bool", "routable": "bool",
		"world_x": "double", "world_y": "double", "world_z": "double", "use_world": "bool"}
	optional := map[string]string{"actor": "string", "text": "string", "fragments": "message", "enabled": "bool", "zone": "string"}
	for i := range s.Targets {
		target := &s.Targets[i]
		g.id = target.ID
		field := "targets." + target.ID
		if !godotResourceID.MatchString(target.ID) || seen[target.ID] {
			return fmt.Errorf("screen %s field %s.id: invalid/duplicate target", g.id, field)
		}
		seen[target.ID] = true
		if height := target.PointerHeightM; height != nil && (math.IsNaN(*height) || math.IsInf(*height, 0) || *height < 0) {
			return fmt.Errorf("screen %s field %s.pointer_height_m: expected finite nonnegative metres", g.id, field)
		}
		if target.Position == "" {
			target.Position = "cell"
		}
		if target.Position != "cell" && target.Position != "actor" && target.Position != "world" && target.Position != "zone" {
			return fmt.Errorf("screen %s field %s.position: unknown position shape", g.id, field)
		}
		if target.Position == "actor" {
			if _, ok := target.Bind["actor"]; !ok {
				return fmt.Errorf("screen %s field %s.bind.actor: required", g.id, field)
			}
		}
		source, err := g.bind(&target.Source, "", field+".source")
		if err != nil {
			return err
		}
		if !source.repeated || g.messages[source.typ].Name == "" {
			return fmt.Errorf("screen %s field %s.source: expected message list", g.id, field)
		}
		if target.RadiusPX < 1 || target.RadiusPX > 120 {
			return fmt.Errorf("screen %s field %s.radius_px: out of range", g.id, field)
		}
		for _, key := range sortedScreenKeys(types) {
			if _, ok := target.Bind[key]; !ok {
				return fmt.Errorf("screen %s field %s.bind.%s: required", g.id, field, key)
			}
		}
		for _, key := range sortedScreenKeys(target.Bind) {
			expected, ok := types[key]
			repeated := false
			if !ok {
				expected, ok = optional[key]
				repeated = key == "fragments"
			}
			if !ok {
				return fmt.Errorf("screen %s field %s.bind.%s: unknown binding", g.id, field, key)
			}
			value := target.Bind[key]
			path, err := g.bind(&value, source.typ, field+".bind."+key)
			if err != nil {
				return err
			}
			valid := path.typ == expected && path.repeated == repeated
			if expected == "message" {
				text, ok := g.field(path.typ, "text")
				valid = path.repeated && ok && text.Type == "string" && !text.Repeated
			}
			if !valid {
				return fmt.Errorf("screen %s field %s.bind.%s: expected %s", g.id, field, key, expected)
			}
			target.Bind[key] = value
		}
		if target.Action != "" && target.Action != "life_enter" && target.Action != "talk" {
			return fmt.Errorf("screen %s field %s.action: unknown action", g.id, field)
		}
		// talk (story memory): the server opens the person's conversation; the column opens by its own state
		if target.Action == "talk" {
			if target.Screen != "" || target.Position != "actor" {
				return fmt.Errorf("screen %s field %s: a talk action is on a person and opens no screen", g.id, field)
			}
			if _, ok := target.Bind["actor"]; !ok {
				return fmt.Errorf("screen %s field %s.bind.actor: required", g.id, field)
			}
		}
		if target.Action == "life_enter" {
			if target.Screen != "" {
				return fmt.Errorf("screen %s field %s.screen: exit action cannot also open a screen", g.id, field)
			}
			if target.Position != "zone" {
				return fmt.Errorf("screen %s field %s.position: exit action requires a floor zone", g.id, field)
			}
			for _, key := range []string{"zone", "enabled"} {
				if _, ok := target.Bind[key]; !ok {
					return fmt.Errorf("screen %s field %s.bind.%s: required", g.id, field, key)
				}
			}
		}
		if target.Screen == "" {
			if target.List != "" || target.RowKey != "" {
				return fmt.Errorf("screen %s field %s.screen: required", g.id, field)
			}
			continue
		}
		var destination *GodotScreen
		for j := range s.Screens {
			if s.Screens[j].ID == target.Screen {
				destination = &s.Screens[j]
			}
		}
		if destination == nil {
			return fmt.Errorf("screen %s field %s.screen: unknown screen", g.id, field)
		}
		list := findScreenControl(destination.Controls, target.List)
		if list == nil || list.Kind != "list" {
			return fmt.Errorf("screen %s field %s.list: unknown list", g.id, field)
		}
		if list.RowKey != target.RowKey {
			return fmt.Errorf("screen %s field %s.row_key: unknown row key", g.id, field)
		}
	}
	if s.WorldInput != nil {
		for _, test := range []struct {
			field string
			key   ScreenShortcut
		}{{"peaceful", s.WorldInput.Peaceful}, {"hostile", s.WorldInput.Hostile}} {
			if len(test.key.Key) != 1 || !strings.Contains("ABCDEFGHIJKLMNOPQRSTUVWXYZ", test.key.Key) {
				return fmt.Errorf("screen world field world_input.%s.key: expected A-Z", test.field)
			}
			if !strings.Contains(" X Y Start Back L1 R1 ", " "+test.key.Pad+" ") {
				return fmt.Errorf("screen world field world_input.%s.pad: unknown button", test.field)
			}
		}
		for _, screen := range s.Screens {
			if screen.ID != s.WorldInput.EngageScreen {
				continue
			}
			list := findScreenControl(screen.Controls, s.WorldInput.EngageList)
			if list == nil || list.Kind != "list" {
				return fmt.Errorf("screen world field world_input.engage_list: unknown list")
			}
			if list.RowKey != s.WorldInput.EngageKey {
				return fmt.Errorf("screen world field world_input.engage_key: unknown row key")
			}
		}
		for field, id := range map[string]string{"fallback": s.WorldInput.Fallback, "engage_screen": s.WorldInput.EngageScreen} {
			found := false
			for _, screen := range s.Screens {
				if screen.ID == id {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("screen world field world_input.%s: unknown screen", field)
			}
		}
	}
	return nil
}
func findScreenControl(nodes []ScreenControl, id string) *ScreenControl {
	for i := range nodes {
		if nodes[i].ID == id {
			return &nodes[i]
		}
		if found := findScreenControl(nodes[i].Children, id); found != nil {
			return found
		}
	}
	return nil
}
