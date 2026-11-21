package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"

	"cuelang.org/go/cue"
)

// ChipHud is world.hud (design/hud/v1/BRIEF.md; doc 09 «20.11 HUD — интерфейс чипа героя»): what the hero's chip shows
// him beyond the fight's own view — its functions and which are open, the chip's link by place, the needs it reads from
// the body, the words for how much the hero shows and how loud he is, the fixed quick-item slots and the fight's feed.
// Presentation only: the generated server (templates/chip_hud.go.tmpl) reads state it already keeps and sends
// state.hud; no rule reads this block, so it never changes an outcome (save key: presentation).
type ChipHud struct {
	// the ПРЕДМЕТЫ deck's slots (review 21.11): an item keeps its slot while carried, a new one takes the first empty
	ItemSlots int               `json:"item_slots"`
	Feed      ChipHudFeed       `json:"feed"`
	Quick     []ChipHudQuick    `json:"quick_items"`
	Needs     []ChipHudNeed     `json:"needs"`
	Seen      []ChipHudLevel    `json:"seen"`
	Noise     ChipHudNoise      `json:"noise"`
	Functions []ChipHudFunction `json:"functions"`
	Link      ChipHudLink       `json:"link"`
	Sources   json.RawMessage   `json:"sources,omitempty"`
}

// ChipHudFeed: the last Lines events of a fight as short sentences; an event's word by its type, a blow's by its mode.
type ChipHudFeed struct {
	Lines    int               `json:"lines"`
	HitKey   string            `json:"hit_key"`
	MissKey  string            `json:"miss_key"`
	ActKey   string            `json:"act_key"`
	Events   map[string]string `json:"events"`
	Modes    map[string]string `json:"modes"`
	OtherKey string            `json:"other_key"`
}

// ChipHudQuick: a fixed slot for an item definition (the panel never reorders itself); KindKey says what it is, IdleKey
// why the slot does nothing now (none carried, or no wound it treats).
type ChipHudQuick struct {
	Definition string `json:"definition"`
	KindKey    string `json:"kind_key"`
	IdleKey    string `json:"idle_key"`
}

// ChipHudNeed: one of the body's needs the brain keeps (LifeBrain.Needs[BrainIndex], 0 sated .. 1e6), and its word.
type ChipHudNeed struct {
	ID         string `json:"id"`
	BrainIndex int    `json:"brain_index"`
	TextKey    string `json:"text_key"`
}

// ChipHudLevel: the first level whose Below is above the value names it; the last has no bound.
type ChipHudLevel struct {
	Below   *float64 `json:"below,omitempty"`
	TextKey string   `json:"text_key"`
}

// ChipHudNoise: the words for the hero's own steps: standing, the quiet step, walking, running.
type ChipHudNoise struct {
	StillKey string `json:"still_key"`
	QuietKey string `json:"quiet_key"`
	WalkKey  string `json:"walk_key"`
	RunKey   string `json:"run_key"`
}

// ChipHudFunction: one of the chip's functions, in the HUD's order: open from the start, or locked with its reason and
// when it opens. Story conditions (doc 103 facts) are the next step; v1 knows start and locked.
type ChipHudFunction struct {
	ID        string `json:"id"`
	Icon      string `json:"icon"`
	NameKey   string `json:"name_key"`
	TextKey   string `json:"text_key"`
	Unlock    string `json:"unlock"`
	ReasonKey string `json:"reason_key,omitempty"`
	LaterKey  string `json:"later_key,omitempty"`
}

// ChipHudLink: the chip's link by level (doc 33: coverage of the dome or a transmitter); a level without a row has Default.
type ChipHudLink struct {
	Default string                      `json:"default"`
	Levels  map[string]string           `json:"levels"`
	States  map[string]ChipHudLinkState `json:"states"`
}

type ChipHudLinkState struct {
	TextKey string `json:"text_key"`
	Bars    int    `json:"bars"`
}

var chipHudID = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func readChipHud(world cue.Value) (*ChipHud, error) {
	v := world.LookupPath(cue.ParsePath("hud"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.hud: %w", err)
	}
	var c ChipHud
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		return nil, fmt.Errorf("world.hud: %w", err)
	}
	c.Sources = nil // provenance stays in CUE
	return &c, nil
}

// validateChipHud: every text in every locale, quick slots name real items once each, needs read the brain's five
// needs, the levels of the words climb and end unbounded, functions are unique and open or locked with a reason, and
// every link state named exists.
func validateChipHud(c Compiled) error {
	h := c.Hud
	if h == nil {
		return nil
	}
	locales := map[string]bool{}
	texts := map[string]map[string]string{}
	for _, m := range c.Localization.Messages {
		texts[m.Key] = m.Texts
		for locale := range m.Texts {
			locales[locale] = true
		}
	}
	names := make([]string, 0, len(locales))
	for locale := range locales {
		names = append(names, locale)
	}
	sort.Strings(names)
	text := func(field, key string) error {
		if key == "" {
			return fmt.Errorf("world.hud: %s: empty text key", field)
		}
		for _, locale := range names {
			if texts[key][locale] == "" {
				return fmt.Errorf("world.hud: %s: no %s text for %s", field, locale, key)
			}
		}
		return nil
	}
	if h.ItemSlots < 1 || h.ItemSlots > 8 {
		return fmt.Errorf("world.hud: item_slots: expected 1..8")
	}
	if h.Feed.Lines < 1 || h.Feed.Lines > 12 {
		return fmt.Errorf("world.hud: feed.lines: expected 1..12")
	}
	for _, field := range []struct{ name, key string }{{"feed.hit_key", h.Feed.HitKey}, {"feed.miss_key", h.Feed.MissKey},
		{"feed.act_key", h.Feed.ActKey}, {"feed.other_key", h.Feed.OtherKey}} {
		if err := text(field.name, field.key); err != nil {
			return err
		}
	}
	for _, k := range sortedScreenKeys(h.Feed.Events) {
		if err := text("feed.events."+k, h.Feed.Events[k]); err != nil {
			return err
		}
	}
	for _, k := range sortedScreenKeys(h.Feed.Modes) {
		if err := text("feed.modes."+k, h.Feed.Modes[k]); err != nil {
			return err
		}
	}
	seen := map[string]bool{}
	for i, q := range h.Quick {
		if _, ok := c.Inventory.Definitions[q.Definition]; !ok || seen[q.Definition] {
			return fmt.Errorf("world.hud: quick_items.%d: unknown or repeated item %q", i, q.Definition)
		}
		seen[q.Definition] = true
		if err := text(fmt.Sprintf("quick_items.%d.kind_key", i), q.KindKey); err != nil {
			return err
		}
		if err := text(fmt.Sprintf("quick_items.%d.idle_key", i), q.IdleKey); err != nil {
			return err
		}
	}
	needs := map[string]bool{}
	for i, n := range h.Needs {
		if !chipHudID.MatchString(n.ID) || needs[n.ID] || n.BrainIndex < 0 || n.BrainIndex > 4 {
			return fmt.Errorf("world.hud: needs.%d: invalid/duplicate id or brain index outside 0..4", i)
		}
		needs[n.ID] = true
		if err := text(fmt.Sprintf("needs.%d.text_key", i), n.TextKey); err != nil {
			return err
		}
	}
	if len(h.Seen) == 0 || h.Seen[len(h.Seen)-1].Below != nil {
		return fmt.Errorf("world.hud: seen: the last level has no bound")
	}
	for i, l := range h.Seen {
		if i < len(h.Seen)-1 && (l.Below == nil || (i > 0 && *l.Below <= *h.Seen[i-1].Below)) {
			return fmt.Errorf("world.hud: seen.%d: bounds must climb", i)
		}
		if err := text(fmt.Sprintf("seen.%d.text_key", i), l.TextKey); err != nil {
			return err
		}
	}
	for _, field := range []struct{ name, key string }{{"noise.still_key", h.Noise.StillKey}, {"noise.quiet_key", h.Noise.QuietKey},
		{"noise.walk_key", h.Noise.WalkKey}, {"noise.run_key", h.Noise.RunKey}} {
		if err := text(field.name, field.key); err != nil {
			return err
		}
	}
	functions := map[string]bool{}
	for i, f := range h.Functions {
		field := fmt.Sprintf("functions.%d", i)
		if !chipHudID.MatchString(f.ID) || functions[f.ID] || !chipHudID.MatchString(f.Icon) {
			return fmt.Errorf("world.hud: %s: invalid/duplicate id or icon", field)
		}
		functions[f.ID] = true
		if err := text(field+".name_key", f.NameKey); err != nil {
			return err
		}
		if err := text(field+".text_key", f.TextKey); err != nil {
			return err
		}
		switch f.Unlock {
		case "start":
			if f.ReasonKey != "" || f.LaterKey != "" {
				return fmt.Errorf("world.hud: %s: an open function has no lock reason", field)
			}
		case "locked":
			if err := text(field+".reason_key", f.ReasonKey); err != nil {
				return err
			}
			if err := text(field+".later_key", f.LaterKey); err != nil {
				return err
			}
		default:
			return fmt.Errorf("world.hud: %s.unlock: expected start or locked", field)
		}
	}
	if _, ok := h.Link.States[h.Link.Default]; !ok {
		return fmt.Errorf("world.hud: link.default: unknown state %q", h.Link.Default)
	}
	for _, level := range sortedScreenKeys(h.Link.Levels) {
		if _, ok := h.Link.States[h.Link.Levels[level]]; !ok {
			return fmt.Errorf("world.hud: link.levels.%s: unknown state %q", level, h.Link.Levels[level])
		}
	}
	for _, id := range sortedScreenKeys(h.Link.States) {
		s := h.Link.States[id]
		if !chipHudID.MatchString(id) || s.Bars < 0 || s.Bars > 4 {
			return fmt.Errorf("world.hud: link.states.%s: invalid id or bars outside 0..4", id)
		}
		if err := text("link.states."+id+".text_key", s.TextKey); err != nil {
			return err
		}
	}
	return nil
}
