package gamepoc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cuelang.org/go/cue"
)

// readCombatLook takes world.combat_look (content/world/combat_play.cue): how a fight is drawn — the clips, props,
// contact points, reactions and sounds of every blow. Presentation only: the Godot combat playback reads it from
// presentation.json "combat_look" and the server never does (json:"-" on Compiled). The playable scenes still carry
// their own copy for the retired Unity view.
//
// Its `godot` block names where the Godot client finds what the look names: the shared clip library of every fighter,
// the folder of the combat sounds and, per clip family (the part of a clip's `source` before the slash), the folder of
// its takes in art-source (or "ual" for the UAL library the rig itself comes from). A clip of a family without a folder
// would never be packed, so every family in use must have one.
func readCombatLook(world cue.Value) (json.RawMessage, error) {
	look := world.LookupPath(cue.ParsePath("combat_look"))
	if !look.Exists() {
		return nil, nil
	}
	raw, err := look.MarshalJSON()
	if err != nil {
		return nil, err
	}
	if err := validateCombatLook(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

type combatLookGodot struct {
	Library  string            `json:"library"`
	SoundDir string            `json:"sound_dir"`
	Sources  map[string]string `json:"sources"`
	Bodies   struct {
		PushGroups []string          `json:"push_groups"`
		PushSkip   []string          `json:"push_skip"`
		Arms       map[string]string `json:"arms"`
		BladeHand  string            `json:"blade_hand"`
	} `json:"bodies"`
}

type combatLookSolver struct {
	Capsules []struct {
		ID    string `json:"id"`
		Group string `json:"group"`
	} `json:"capsules"`
}

func validateCombatLook(raw json.RawMessage) error {
	var look map[string]any
	if err := json.Unmarshal(raw, &look); err != nil {
		return err
	}
	block, ok := look["godot"]
	if !ok {
		return fmt.Errorf("combat_look: no godot block (library, sound_dir, sources)")
	}
	encoded, _ := json.Marshal(block)
	var g combatLookGodot
	if err := json.Unmarshal(encoded, &g); err != nil {
		return fmt.Errorf("combat_look.godot: %w", err)
	}
	if !strings.HasPrefix(g.Library, "res://") || !strings.HasPrefix(g.SoundDir, "res://") {
		return fmt.Errorf("combat_look.godot: library and sound_dir must be res:// paths")
	}
	if err := validateCombatLookBodies(look, g); err != nil {
		return err
	}
	missing := map[string]bool{}
	for _, source := range combatLookSources(look) {
		family, take, found := strings.Cut(source, "/")
		if !found || take == "" {
			return fmt.Errorf("combat_look: clip source %q is not <family>/<take>", source)
		}
		if g.Sources[family] == "" {
			missing[family] = true
		}
	}
	if len(missing) > 0 {
		names := make([]string, 0, len(missing))
		for f := range missing {
			names = append(names, f)
		}
		sort.Strings(names)
		return fmt.Errorf("combat_look.godot.sources: no folder for the clip families %v", names)
	}
	return nil
}

// The Godot solver's lists name the capsules of combat_look.body_solver: its groups and ids (a typo would silently push
// nothing apart, or leave the blade hand in the evidence).
func validateCombatLookBodies(look map[string]any, g combatLookGodot) error {
	encoded, _ := json.Marshal(look["body_solver"])
	var solver combatLookSolver
	if err := json.Unmarshal(encoded, &solver); err != nil {
		return fmt.Errorf("combat_look.body_solver: %w", err)
	}
	ids, groups := map[string]bool{}, map[string]bool{}
	for _, c := range solver.Capsules {
		ids[c.ID] = true
		groups[c.Group] = true
	}
	b := g.Bodies
	if len(b.PushGroups) == 0 || b.BladeHand == "" || b.Arms["Left"] == "" || b.Arms["Right"] == "" {
		return fmt.Errorf("combat_look.godot.bodies: push_groups, arms (Left, Right) and blade_hand are required")
	}
	for _, name := range append(append([]string{}, b.PushGroups...), b.Arms["Left"], b.Arms["Right"]) {
		if !groups[name] {
			return fmt.Errorf("combat_look.godot.bodies: %q is no group of body_solver.capsules", name)
		}
	}
	for _, id := range append(append([]string{}, b.PushSkip...), b.BladeHand) {
		if !ids[id] {
			return fmt.Errorf("combat_look.godot.bodies: %q is no capsule of body_solver.capsules", id)
		}
	}
	return nil
}

// combatLookSources: the `source` of every clip of the look (an object with a state and a source), sorted, once each.
func combatLookSources(v any) []string {
	seen := map[string]bool{}
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if s, ok := x["source"].(string); ok {
				if _, clip := x["state"]; clip {
					seen[s] = true
				}
			}
			for _, child := range x {
				walk(child)
			}
		case []any:
			for _, child := range x {
				walk(child)
			}
		}
	}
	walk(v)
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// checkCombatLookTakes: every take of the look is a file of its family's folder (`<folder>/<take>.fbx` under the game
// root; the family "ual" is the rig's own library), so the Godot pack never meets a missing take. A look-less world
// passes.
func checkCombatLookTakes(root string, raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var look map[string]any
	if err := json.Unmarshal(raw, &look); err != nil {
		return err
	}
	encoded, _ := json.Marshal(look["godot"])
	var g combatLookGodot
	if err := json.Unmarshal(encoded, &g); err != nil {
		return fmt.Errorf("combat_look.godot: %w", err)
	}
	var missing []string
	for _, source := range combatLookSources(look) {
		family, take, _ := strings.Cut(source, "/")
		folder := g.Sources[family]
		if folder == "ual" {
			continue
		}
		dir := filepath.Join(root, folder)
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			missing = append(missing, folder+"/")
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, take+".fbx")); err != nil {
			missing = append(missing, filepath.Join(folder, take+".fbx"))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("combat_look: takes missing on disk: %v", missing)
	}
	return nil
}
