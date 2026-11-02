package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"

	"cuelang.org/go/cue"
	"github.com/strogmv/ang/compiler/wounds"
	woundsRuntime "github.com/strogmv/ang/compiler/wounds/runtime"
)

// Wounds instead of hit points (doc 97, world.wounds): the catalog is checked against the compiled world (aid items,
// combat weapons and armour, justice crimes, the hero, the drives' stress responses, RU/EN of every key, the CUE
// scenarios' expectations) and emitted as the standalone engine internal/wounds with the timeline table
// design/wounds/timelines.md. Nothing in the game calls it yet: the combat layer connects it (design/wounds).
type woundsInput struct {
	model woundsRuntime.Model
	world wounds.World
}

func readWounds(world cue.Value) (*woundsInput, error) {
	v := world.LookupPath(cue.ParsePath("wounds"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.wounds: %w", err)
	}
	in := woundsInput{world: wounds.World{Items: map[string]string{}, Weapons: map[string]bool{}, Armor: map[string]bool{}, Crimes: map[string]string{}, Persons: map[string]bool{}, Responses: map[string]bool{}}}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&in.model); err != nil {
		return nil, fmt.Errorf("world.wounds: %w", err)
	}
	fields := func(path string, each func(id string, v cue.Value) error) error {
		it, err := world.LookupPath(cue.ParsePath(path)).Fields()
		if err != nil {
			return fmt.Errorf("wounds: world.%s: %w", path, err)
		}
		for it.Next() {
			if err := each(it.Selector().Unquoted(), it.Value()); err != nil {
				return err
			}
		}
		return nil
	}
	str := func(v cue.Value, path string) string {
		s, _ := v.LookupPath(cue.ParsePath(path)).String()
		return s
	}
	if err := fields("items", func(id string, v cue.Value) error { in.world.Items[id] = str(v, "carrier"); return nil }); err != nil {
		return nil, err
	}
	if err := fields("combat_runtime.weapons", func(id string, _ cue.Value) error { in.world.Weapons[id] = true; return nil }); err != nil {
		return nil, err
	}
	if err := fields("combat_runtime.profiles", func(_ string, v cue.Value) error { in.world.Weapons[str(v, "weapon.id")] = true; return nil }); err != nil {
		return nil, err
	}
	if err := fields("combat_runtime.armor", func(id string, _ cue.Value) error { in.world.Armor[id] = true; return nil }); err != nil {
		return nil, err
	}
	if err := fields("justice.crimes", func(id string, v cue.Value) error { in.world.Crimes[id] = str(v, "outcome"); return nil }); err != nil {
		return nil, err
	}
	if err := fields("persons", func(id string, _ cue.Value) error { in.world.Persons[id] = true; return nil }); err != nil {
		return nil, err
	}
	if err := fields("drives", func(_ string, v cue.Value) error { in.world.Responses[str(v, "stress_response")] = true; return nil }); err != nil {
		return nil, err
	}
	return &in, nil
}

// compileWounds checks the catalog against the world and its texts and renders the timeline table; nil without it.
func compileWounds(c *Compiled) error {
	in := c.woundsIn
	if in == nil {
		return nil
	}
	texts := map[string]map[string]string{}
	for _, msg := range c.Localization.Messages {
		texts[msg.Key] = msg.Texts
	}
	w := in.world
	w.Texts, w.Locales = texts, append([]string(nil), c.Localization.Locales...)
	if err := wounds.Check(in.model, w); err != nil {
		return err
	}
	m := in.model
	report, err := wounds.Report(m, func(key string) string {
		if t := texts[key]["ru"]; t != "" {
			return t
		}
		return key
	})
	if err != nil {
		return err
	}
	c.Wounds, c.woundsReport = &m, report
	return nil
}
