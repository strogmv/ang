package gamepoc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The sound of the hall (demo-hub v26-alive): generate fails on a key nobody defined, a file that is not there, a shop that does not exist, a gain above 0 dB.
func soundFixture(t *testing.T) (Compiled, string) {
	t.Helper()
	unity := t.TempDir()
	dir := filepath.Join(unity, "Assets/Resources/HallSound")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.wav"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := &SoundCatalog{LevelID: "level.x", VoicesPoint: 2, VoicesShot: 2, PublishSeconds: 1, Clips: map[string][]string{"a": {"a"}}}
	g.Crowd.RadiusM = 10
	g.Loops = []SoundLoop{{ID: "hall.bed", Clip: "a", Flat: true, GainDB: -10}}
	g.Roller.Down, g.Roller.Up, g.Vending.Clip, g.Steps.Floor, g.Steps.Steel = "a", "a", "a", "a", "a"
	g.Steps.StrideM, g.Steps.Voices = .7, 2
	var c Compiled
	c.Soundscape = g
	return c, unity
}
func TestSoundscapeValidation(t *testing.T) {
	c, unity := soundFixture(t)
	if err := validateSoundscape(c, unity); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(g *SoundCatalog){
		"unknown clip": func(g *SoundCatalog) { g.Loops[0].Clip = "nothing" },
		"missing file": func(g *SoundCatalog) { g.Clips["a"] = []string{"gone"} },
		"gain over 0":  func(g *SoundCatalog) { g.Loops[0].GainDB = 3 },
		"no shop":      func(g *SoundCatalog) { g.Loops[0].When.ShopOpen = "shop.nobody" },
		"point without a place": func(g *SoundCatalog) {
			g.Loops = append(g.Loops, SoundLoop{ID: "x", Clip: "a", MinM: 1, MaxM: 2})
		},
		"zero stride": func(g *SoundCatalog) { g.Steps.StrideM = 0 },
	}
	for name, change := range cases {
		c, unity := soundFixture(t)
		change(c.Soundscape)
		err := validateSoundscape(c, unity)
		if err == nil || !strings.Contains(err.Error(), "world.soundscape") {
			t.Errorf("%s: %v", name, err)
		}
	}
}
