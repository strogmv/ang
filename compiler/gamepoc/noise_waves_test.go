package gamepoc

import (
	"strings"
	"testing"
)

func noiseOK() NoiseWaveCatalog {
	return NoiseWaveCatalog{Enabled: true, ColorRGB: [3]int{194, 184, 164}, LinePX: 1.25, PeakAlpha: .115, OccludedMul: .5, MaxSpeedMS: 8, MinFrontS: .55, TailS: .18, DecayPerS: .15, Crests: []NoiseWaveCrest{{1, 0}, {.36, .12}}, EdgeBand: .012, PublishS: 1, MaxWaves: 24, HoldCombatS: 2.5, HeardGapS: .45, ShoutGapS: 2}
}

func TestTacticalModeLabelsMustExist(t *testing.T) {
	c := Compiled{TacticalMode: &TacticalMode{Hotkey: "T", LabelOn: "ui.tactical.on", LabelOff: "ui.tactical.off"}}
	if err := validateTacticalMode(c); err == nil {
		t.Fatal("labels without localization must fail generate")
	}
}

// The approved faint style is a limit of the data: a louder look fails generate.
func TestNoiseWavesLimitsOfTheStyle(t *testing.T) {
	if err := noiseOK().validate(); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*NoiseWaveCatalog){
		"line_px":      func(c *NoiseWaveCatalog) { c.LinePX = 2 },
		"peak_alpha":   func(c *NoiseWaveCatalog) { c.PeakAlpha = .2 },
		"occluded_mul": func(c *NoiseWaveCatalog) { c.OccludedMul = .8 },
		"short wave":   func(c *NoiseWaveCatalog) { c.MinFrontS, c.TailS = .2, .1 },
		"crest":        func(c *NoiseWaveCatalog) { c.Crests = []NoiseWaveCrest{{1, .3}} },
	} {
		c := noiseOK()
		change(&c)
		if err := c.validate(); err == nil || !strings.Contains(err.Error()+name, name) {
			t.Fatalf("%s: want a failure, got %v", name, err)
		}
	}
}
