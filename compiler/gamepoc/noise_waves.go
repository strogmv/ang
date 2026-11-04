package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"

	"cuelang.org/go/cue"
)

// The noise waves (world.noise_waves, combat-play/noise-waves). Presentation data only: Go (noise_waves.go.tmpl) publishes what the hero hears by the
// hearing rule, the client (noise_waves.cs.tmpl, noise_wave.shader.tmpl) draws it. The limits of the approved faint style fail `generate`: the owner asked
// for waves that do not distract, so a louder look must be a decision, not a typo.
type NoiseWaveCrest struct {
	Strength float64 `json:"strength"`
	LagS     float64 `json:"lag_s"`
}
type NoiseWaveCatalog struct {
	Enabled     bool             `json:"enabled"`
	ColorRGB    [3]int           `json:"color_rgb"`
	LinePX      float64          `json:"line_px"`
	PeakAlpha   float64          `json:"peak_alpha"`
	OccludedMul float64          `json:"occluded_mul"`
	MaxSpeedMS  float64          `json:"max_speed_m_s"`
	MinFrontS   float64          `json:"min_front_s"`
	TailS       float64          `json:"tail_s"`
	DecayPerS   float64          `json:"decay_per_s"`
	Crests      []NoiseWaveCrest `json:"crests"`
	EdgeBand    float64          `json:"edge_band"`
	PublishS    float64          `json:"publish_s"`
	MaxWaves    int              `json:"max_waves"`
	HoldCombatS float64          `json:"hold_combat_s"`
	HeardGapS   float64          `json:"heard_gap_s"`
	ShoutGapS   float64          `json:"shout_gap_s"`
}

// TacticalMode is a view, not a rule: the server publishes the same; what the mode shows joins it as its own block of data (the waves are the first).
type TacticalMode struct {
	DefaultOn bool   `json:"default_on"`
	Hotkey    string `json:"hotkey"`
	LabelOn   string `json:"label_on"`
	LabelOff  string `json:"label_off"`
}

func readTacticalMode(world cue.Value) (*TacticalMode, error) {
	v := world.LookupPath(cue.ParsePath("tactical_mode"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.tactical_mode: %w", err)
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var out TacticalMode
	if err := d.Decode(&out); err != nil {
		return nil, fmt.Errorf("world.tactical_mode: %w", err)
	}
	if out.Hotkey == "" || out.LabelOn == "" || out.LabelOff == "" {
		return nil, fmt.Errorf("world.tactical_mode: hotkey and both labels are required")
	}
	return &out, nil
}

// validateTacticalMode: its labels must exist in every locale of the world.
func validateTacticalMode(c Compiled) error {
	g := c.TacticalMode
	if g == nil {
		return nil
	}
	for _, key := range []string{g.LabelOn, g.LabelOff} {
		found := false
		for _, m := range c.Localization.Messages {
			found = found || m.Key == key
		}
		if !found {
			return fmt.Errorf("world.tactical_mode: no localization for %s", key)
		}
	}
	return nil
}

func readNoiseWaves(world cue.Value) (*NoiseWaveCatalog, error) {
	v := world.LookupPath(cue.ParsePath("noise_waves"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.noise_waves: %w", err)
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var out NoiseWaveCatalog
	if err := d.Decode(&out); err != nil {
		return nil, fmt.Errorf("world.noise_waves: %w", err)
	}
	if err := out.validate(); err != nil {
		return nil, fmt.Errorf("world.noise_waves: %w", err)
	}
	return &out, nil
}

func (c NoiseWaveCatalog) validate() error {
	if c.LinePX < 1 || c.LinePX > 1.5 {
		return fmt.Errorf("line_px %.3f outside the approved 1..1.5", c.LinePX)
	}
	if c.PeakAlpha <= 0 || c.PeakAlpha > 0.115 {
		return fmt.Errorf("peak_alpha %.3f above the approved 0.115", c.PeakAlpha)
	}
	if c.OccludedMul <= 0 || c.OccludedMul > 0.5 {
		return fmt.Errorf("occluded_mul %.3f above the approved 0.5", c.OccludedMul)
	}
	if c.MinFrontS+c.TailS < 0.5 {
		return fmt.Errorf("a short wave lives %.2f s, the approved minimum is 0.5", c.MinFrontS+c.TailS)
	}
	if c.MaxSpeedMS <= 0 || c.EdgeBand <= 0 || c.PublishS <= 0 || c.MaxWaves < 1 || len(c.Crests) == 0 || len(c.Crests) > 4 {
		return fmt.Errorf("invalid motion")
	}
	for i, x := range c.Crests {
		if x.Strength <= 0 || x.Strength > 1 || x.LagS < 0 || i == 0 && x.LagS != 0 {
			return fmt.Errorf("invalid crest %d", i)
		}
	}
	return nil
}
