package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"cuelang.org/go/cue"
)

// The sound of the hall (demo-hub v26-alive, world.soundscape, market_hub/soundscape.cue). Data only: Go (soundscape.go.tmpl) decides from its own state
// what is on and which one-shots happen; the client (soundscape.cs.tmpl) plays the keys at the given places. The checks here fail `generate`: a clip key
// nobody defined, a file that is not in Resources/HallSound, a shop that does not exist, a gain above 0 dB, a position outside the level.
type SoundWhen struct {
	ShopOpen string `json:"shop_open,omitempty"`
	Window   *struct {
		From int64 `json:"from"`
		To   int64 `json:"to"`
	} `json:"window,omitempty"`
	Vending bool `json:"vending,omitempty"`
}
type SoundCell struct {
	X int `json:"x"`
	Y int `json:"y"`
}
type SoundLoop struct {
	ID     string     `json:"id"`
	Clip   string     `json:"clip"`
	Flat   bool       `json:"flat"`
	At     *SoundCell `json:"at,omitempty"`
	UpM    float64    `json:"up_m"`
	When   SoundWhen  `json:"when"`
	GainDB float64    `json:"gain_db"`
	MinM   float64    `json:"min_m"`
	MaxM   float64    `json:"max_m"`
}
type SoundShot struct {
	ID        string      `json:"id"`
	Clips     []string    `json:"clips"`
	At        []SoundCell `json:"at"`
	UpM       float64     `json:"up_m"`
	EveryS    float64     `json:"every_s"`
	ChancePPM int         `json:"chance_ppm"`
	When      SoundWhen   `json:"when"`
	GainDB    float64     `json:"gain_db"`
	MinM      float64     `json:"min_m"`
	MaxM      float64     `json:"max_m"`
}
type SoundLayer struct {
	Clip       string  `json:"clip"`
	PeopleFrom float64 `json:"people_from"`
	PeopleFull float64 `json:"people_full"`
	GainDB     float64 `json:"gain_db"`
}
type SoundCatalog struct {
	LevelID  string `json:"level_id"`
	Listener struct {
		UpM         float64 `json:"up_m"`
		PanWidth    float64 `json:"pan_width"`
		FadeSeconds float64 `json:"fade_seconds"`
	} `json:"listener"`
	VoicesPoint int                 `json:"voices_point"`
	VoicesShot  int                 `json:"voices_shot"`
	Clips       map[string][]string `json:"clips"`
	Loops       []SoundLoop         `json:"loops"`
	Crowd       struct {
		RadiusM float64      `json:"radius_m"`
		Layers  []SoundLayer `json:"layers"`
	} `json:"crowd"`
	Shots  []SoundShot `json:"shots"`
	Roller struct {
		Down        string  `json:"down"`
		Up          string  `json:"up"`
		UpM         float64 `json:"up_m"`
		GainDB      float64 `json:"gain_db"`
		MinM        float64 `json:"min_m"`
		MaxM        float64 `json:"max_m"`
		DownSeconds float64 `json:"down_seconds"`
		UpSeconds   float64 `json:"up_seconds"`
	} `json:"roller"`
	Vending struct {
		Clip   string  `json:"clip"`
		UpM    float64 `json:"up_m"`
		GainDB float64 `json:"gain_db"`
		MinM   float64 `json:"min_m"`
		MaxM   float64 `json:"max_m"`
	} `json:"vending"`
	Steps struct {
		Floor        string  `json:"floor"`
		Steel        string  `json:"steel"`
		SteelAboveM  float64 `json:"steel_above_m"`
		StrideM      float64 `json:"stride_m"`
		Voices       int     `json:"voices"`
		HeroGainDB   float64 `json:"hero_gain_db"`
		PeopleGainDB float64 `json:"people_gain_db"`
		MinM         float64 `json:"min_m"`
		MaxM         float64 `json:"max_m"`
		PitchSpread  float64 `json:"pitch_spread"`
		MaxRate      int     `json:"max_rate"`
	} `json:"steps"`
	PublishSeconds float64 `json:"publish_seconds"`
}

func readSoundscape(world cue.Value) (*SoundCatalog, error) {
	v := world.LookupPath(cue.ParsePath("soundscape"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.soundscape: %w", err)
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var out SoundCatalog
	if err := d.Decode(&out); err != nil {
		return nil, fmt.Errorf("world.soundscape: %w", err)
	}
	return &out, nil
}

// validateSoundscape: against the compiled world and the files of the client.
func validateSoundscape(c Compiled, unity string) error {
	g := c.Soundscape
	if g == nil {
		return nil
	}
	var fail []string
	bad := func(format string, args ...any) { fail = append(fail, fmt.Sprintf(format, args...)) }
	for key, files := range g.Clips {
		if len(files) == 0 {
			bad("clip %s has no file", key)
		}
		for _, f := range files {
			if unity == "" {
				continue
			}
			if _, err := os.Stat(filepath.Join(unity, "Assets/Resources/HallSound", f+".wav")); err != nil {
				bad("clip %s: no file Resources/HallSound/%s.wav", key, f)
			}
		}
	}
	clip := func(owner, key string) {
		if _, ok := g.Clips[key]; !ok {
			bad("%s: unknown clip %q", owner, key)
		}
	}
	shops := map[string]bool{}
	for _, sh := range c.Inventory.Shopfronts {
		shops[sh.ID] = true
	}
	when := func(owner string, w SoundWhen) {
		if w.ShopOpen != "" && !shops[w.ShopOpen] {
			bad("%s: no shop %q", owner, w.ShopOpen)
		}
		if w.Vending && c.Vending == nil {
			bad("%s: no vending block in this world", owner)
		}
	}
	seen := map[string]bool{}
	for _, l := range g.Loops {
		if seen[l.ID] {
			bad("duplicate loop %s", l.ID)
		}
		seen[l.ID] = true
		clip("loop "+l.ID, l.Clip)
		when("loop "+l.ID, l.When)
		if l.GainDB > 0 || (!l.Flat && (l.At == nil || l.MinM <= 0 || l.MaxM <= l.MinM)) {
			bad("loop %s: gain above 0 dB, no place or wrong distances", l.ID)
		}
	}
	for _, s := range g.Shots {
		for _, k := range s.Clips {
			clip("shot "+s.ID, k)
		}
		when("shot "+s.ID, s.When)
		if s.GainDB > 0 || s.EveryS <= 0 || len(s.At) == 0 || s.MaxM <= s.MinM {
			bad("shot %s: gain above 0 dB, no period, no place or wrong distances", s.ID)
		}
	}
	for _, l := range g.Crowd.Layers {
		clip("crowd", l.Clip)
		if l.PeopleFull <= l.PeopleFrom || l.GainDB > 0 {
			bad("crowd layer %s: wrong count range or gain", l.Clip)
		}
	}
	clip("roller", g.Roller.Down)
	clip("roller", g.Roller.Up)
	clip("vending", g.Vending.Clip)
	clip("steps", g.Steps.Floor)
	clip("steps", g.Steps.Steel)
	if g.Steps.StrideM <= 0 || g.Steps.Voices < 1 || g.VoicesPoint < 1 || g.VoicesShot < 1 || g.PublishSeconds <= 0 || g.Crowd.RadiusM <= 0 {
		bad("zero stride, voices, publish time or crowd radius")
	}
	if len(fail) > 0 {
		return fmt.Errorf("world.soundscape:\n  %s", joinLines(fail))
	}
	return nil
}

func joinLines(lines []string) string {
	var b bytes.Buffer
	for i, l := range lines {
		if i > 0 {
			b.WriteString("\n  ")
		}
		b.WriteString(l)
	}
	return b.String()
}

// HallClosing: the clips of a seller in the closing and opening phases of his post (market_hub/closing.cue). The rest of the closing (who leaves when) is Life's own (population closures).
type HallClosingClip struct {
	ID      string  `json:"id"`
	Clip    string  `json:"clip"`
	State   string  `json:"state"`
	Src     string  `json:"src"`
	Seconds float64 `json:"seconds"`
	Weight  float64 `json:"weight"`
	Hand    string  `json:"hand"`
}
type HallClosing struct {
	PhaseSeconds  int64             `json:"phase_seconds"`
	LingerSeconds int64             `json:"linger_seconds"`
	SlotSeconds   float64           `json:"slot_seconds"`
	Closing       []HallClosingClip `json:"closing"`
	Opening       []HallClosingClip `json:"opening"`
}

func readHallClosing(world cue.Value) (*HallClosing, error) {
	v := world.LookupPath(cue.ParsePath("hall_closing"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.hall_closing: %w", err)
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var out HallClosing
	if err := d.Decode(&out); err != nil {
		return nil, fmt.Errorf("world.hall_closing: %w", err)
	}
	if out.PhaseSeconds < 60 || out.SlotSeconds < 1 || len(out.Closing) == 0 || len(out.Opening) == 0 {
		return nil, fmt.Errorf("world.hall_closing: phase, slot or clips missing")
	}
	return &out, nil
}
