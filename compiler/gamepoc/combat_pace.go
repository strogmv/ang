package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"cuelang.org/go/cue"
)

// CombatPace is world.combat_pace (owner 20.11, doc 09 «Бой: пошаговый и быстрый пошаговый, как в Arcanum»): two ways
// to SHOW one turn-based fight. The server resolves an order and every NPC action up to the hero's next turn at once and
// publishes the events; it never waits on a clock and never learns the pace. Presentation only: the block goes to the
// client (presentation.json, the Godot combat playback) and never into the server's world (json:"-" on Compiled), so
// no pace can change an outcome. The two modes the owner named must both exist; the settings entry that holds the
// choice offers exactly these modes, in this order, with the pace's default.
type CombatPace struct {
	Default      string             `json:"default"`
	Modes        []CombatPaceMode   `json:"modes"`
	Setting      string             `json:"setting"`
	Key          string             `json:"key"`
	Beats        map[string]float64 `json:"beats"`
	BeatDefaultS float64            `json:"beat_default_s"`
	Shifts       map[string]float64 `json:"shifts"`
	HandoverS    float64            `json:"handover_s"`
	CameraLagS   float64            `json:"camera_lag_s"`
	CameraSettle float64            `json:"camera_settle_s"`
}

// CombatPaceMode: Move "walk" (cell by cell with the walk clip at WalkMPS) or "slide" (an actor's run of steps as one
// slide to its last cell in SlideS); BeatShare of every beat is shown; PauseS before another actor's action.
type CombatPaceMode struct {
	ID        string  `json:"id"`
	TextKey   string  `json:"text_key"`
	Move      string  `json:"move"`
	WalkMPS   float64 `json:"walk_mps,omitempty"`
	SlideS    float64 `json:"slide_s,omitempty"`
	BeatShare float64 `json:"beat_share"`
	PauseS    float64 `json:"pause_s"`
}

// combatPaceModes are the two modes of the owner's decision, the first is the turn-based default of today.
var combatPaceModes = []string{"turn_based", "fast_turn_based"}

func readCombatPace(world cue.Value) (*CombatPace, error) {
	v := world.LookupPath(cue.ParsePath("combat_pace"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.combat_pace: %w", err)
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var out CombatPace
	if err := d.Decode(&out); err != nil {
		return nil, fmt.Errorf("world.combat_pace: %w", err)
	}
	if err := out.validate(); err != nil {
		return nil, fmt.Errorf("world.combat_pace: %w", err)
	}
	return &out, nil
}

func (p CombatPace) mode(id string) (CombatPaceMode, bool) {
	for _, m := range p.Modes {
		if m.ID == id {
			return m, true
		}
	}
	return CombatPaceMode{}, false
}

func (p CombatPace) validate() error {
	if len(p.Modes) != len(combatPaceModes) {
		return fmt.Errorf("modes: want exactly %v, got %d", combatPaceModes, len(p.Modes))
	}
	for i, id := range combatPaceModes {
		if p.Modes[i].ID != id {
			return fmt.Errorf("modes[%d]: want %s (the order of the setting), got %q", i, id, p.Modes[i].ID)
		}
	}
	if _, ok := p.mode(p.Default); !ok || p.Default != "turn_based" {
		return fmt.Errorf("default: the default pace is turn_based (owner 20.11), got %q", p.Default)
	}
	for _, m := range p.Modes {
		if m.TextKey == "" {
			return fmt.Errorf("modes %s: text_key required", m.ID)
		}
		switch m.Move {
		case "walk":
			if m.WalkMPS <= 0 || m.WalkMPS > 4 || m.SlideS != 0 {
				return fmt.Errorf("modes %s: a walk needs walk_mps in (0, 4] and no slide_s", m.ID)
			}
		case "slide":
			if m.SlideS <= 0 || m.SlideS > 0.5 || m.WalkMPS != 0 {
				return fmt.Errorf("modes %s: a slide needs slide_s in (0, 0.5] and no walk_mps", m.ID)
			}
		default:
			return fmt.Errorf("modes %s: move %q is walk or slide", m.ID, m.Move)
		}
		if m.BeatShare <= 0 || m.BeatShare > 1 || m.PauseS < 0 || m.PauseS > 1 {
			return fmt.Errorf("modes %s: beat_share in (0, 1], pause_s in [0, 1]", m.ID)
		}
	}
	turn, fast := p.Modes[0], p.Modes[1]
	// Fast turn-based (doc 09): the step is not walked, and nothing waits longer than in turn-based.
	if turn.Move != "walk" || fast.Move != "slide" {
		return fmt.Errorf("modes: turn_based walks its steps and fast_turn_based slides them")
	}
	if fast.BeatShare > turn.BeatShare || fast.PauseS > turn.PauseS {
		return fmt.Errorf("modes fast_turn_based: its beat_share and pause_s may not exceed turn_based's")
	}
	if p.Setting == "" || p.Key == "" {
		return fmt.Errorf("setting and key are required")
	}
	for kind, s := range p.Beats {
		if kind == "" || s < 0 || s > 2 {
			return fmt.Errorf("beat %q: %.3f s outside [0, 2]", kind, s)
		}
	}
	for kind, s := range p.Shifts {
		if _, both := p.Beats[kind]; both {
			return fmt.Errorf("event %q is both a beat and a shift", kind)
		}
		if kind == "" || s <= 0 || s > 1 {
			return fmt.Errorf("shift %q: %.3f s outside (0, 1]", kind, s)
		}
	}
	if _, walked := p.Beats["move"]; walked {
		return fmt.Errorf("beats: a move is walked or slid by its mode, never a beat")
	}
	if p.BeatDefaultS < 0 || p.BeatDefaultS > 2 || p.HandoverS <= 0 || p.HandoverS > 0.3 || p.CameraLagS < 0 || p.CameraLagS > 0.5 ||
		p.CameraSettle < 0 || p.CameraSettle > 3 {
		return fmt.Errorf("beat_default_s in [0, 2], handover_s in (0, 0.3], camera_lag_s in [0, 0.5], camera_settle_s in [0, 3]")
	}
	return nil
}

// validateCombatPace: the pace's texts exist in every locale, and the settings entry that holds the choice offers
// exactly its modes, in order, with its default (one source: a second default in the settings could disagree).
func validateCombatPace(c Compiled) error {
	p := c.CombatPace
	if p == nil {
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
	for _, m := range p.Modes {
		for _, locale := range names {
			if texts[m.TextKey][locale] == "" {
				return fmt.Errorf("world.combat_pace: modes %s: no %s text for %s", m.ID, locale, m.TextKey)
			}
		}
	}
	if c.GodotSettings == nil {
		return fmt.Errorf("world.combat_pace: setting %s: the world has no godot_settings", p.Setting)
	}
	for _, s := range c.GodotSettings.Entries {
		if s.ID != p.Setting {
			continue
		}
		if s.Kind != "choice" || len(s.Options) != len(p.Modes) || s.Default != p.Default {
			return fmt.Errorf("world.combat_pace: setting %s: a choice of the %d modes with the default %s", s.ID, len(p.Modes), p.Default)
		}
		for i, o := range s.Options {
			if o.Value != p.Modes[i].ID || o.Key != p.Modes[i].TextKey {
				return fmt.Errorf("world.combat_pace: setting %s option %d: want %s (%s)", s.ID, i, p.Modes[i].ID, p.Modes[i].TextKey)
			}
		}
		return nil
	}
	return fmt.Errorf("world.combat_pace: setting %s: no such settings entry", p.Setting)
}
