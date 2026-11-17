package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"

	"cuelang.org/go/cue"
)

// PlatformBeacons is world.platform_beacons (hub open edge v1, owner 16.11: «вообще без огражденйй? может пару столбов с
// мигалкой котоыре за 10 секнд до приезда поезда дадаут пару сигналов»): steel posts with an amber flashing beacon and a
// sounder at an open platform edge. LeadSeconds before the train's arrival at the level (the passage's eta_seconds from
// the server's timetable; the client keeps no timer of its own) the beacons flash and the sounder gives Signals short
// signals; they keep flashing while the train in view is in one of FlashPhases (moving in, braking, passing) and go out
// when it stands. Presentation only: movement, doors and boarding are untouched.
type PlatformBeacons struct {
	Levels        []PlatformBeaconLevel `json:"levels"`
	LeadSeconds   float64               `json:"lead_seconds"`
	FlashPhases   []string              `json:"flash_phases"`
	FlashHz       float64               `json:"flash_hz"`
	FlashDuty     float64               `json:"flash_duty"`
	Signals       int                   `json:"signals"`
	SignalSeconds float64               `json:"signal_seconds"`
	GapSeconds    float64               `json:"gap_seconds"`
	ToneHz        float64               `json:"tone_hz"`
	VolumeDB      float64               `json:"volume_db"`
	LensEnergy    float64               `json:"lens_energy"`
	LightEnergy   float64               `json:"light_energy"`
	LightRangeM   float64               `json:"light_range_m"`
	Color         string                `json:"color"`
}

// beaconFlashPhases are the phases the train_motion template gives a train (dispatchPose): the beacons may keep flashing in the
// ones where a train in view is still moving in or past.
var beaconFlashPhases = map[string]bool{"approaching": true, "braking": true, "passing": true}

// PlatformBeaconLevel: the level, the instance whose model holds the posts and the names of their parts in it.
type PlatformBeaconLevel struct {
	LevelID     string `json:"level_id"`
	InstanceID  string `json:"instance_id"`
	LampNode    string `json:"lamp_node"`
	SounderNode string `json:"sounder_node"`
	Why         string `json:"why"`
}

func readPlatformBeacons(world cue.Value, c Compiled) (*PlatformBeacons, error) {
	v := world.LookupPath(cue.ParsePath("platform_beacons"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.platform_beacons: %w", err)
	}
	var out PlatformBeacons
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&out); err != nil {
		return nil, fmt.Errorf("world.platform_beacons: %w", err)
	}
	var life struct {
		Locations map[string]json.RawMessage `json:"locations"`
	}
	if len(c.Life) > 0 {
		if err := json.Unmarshal(c.Life, &life); err != nil {
			return nil, err
		}
	}
	known := func(id string) bool { _, ok := life.Locations[id]; return ok }
	if err := checkPlatformBeacons(out, known); err != nil {
		return nil, fmt.Errorf("world.platform_beacons: %w", err)
	}
	return &out, nil
}

func checkPlatformBeacons(b PlatformBeacons, known func(string) bool) error {
	in := func(x, lo, hi float64) bool { return !math.IsNaN(x) && x >= lo && x <= hi }
	switch {
	case !in(b.LeadSeconds, 3, 30):
		return fmt.Errorf("lead_seconds %v: 3..30 s before the arrival", b.LeadSeconds)
	case !in(b.FlashHz, 0.2, 4):
		return fmt.Errorf("flash_hz %v: 0.2..4 Hz (a beacon, not a strobe)", b.FlashHz)
	case !(b.FlashDuty > 0 && b.FlashDuty < 1):
		return fmt.Errorf("flash_duty %v: the lit share of a period, 0..1", b.FlashDuty)
	case b.Signals < 1 || b.Signals > 5:
		return fmt.Errorf("signals %d: 1..5 short signals", b.Signals)
	case !in(b.SignalSeconds, 0.05, 1) || !in(b.GapSeconds, 0.05, 1):
		return fmt.Errorf("signal_seconds %v / gap_seconds %v: 0.05..1 s", b.SignalSeconds, b.GapSeconds)
	case float64(b.Signals)*b.SignalSeconds+float64(b.Signals-1)*b.GapSeconds > b.LeadSeconds:
		return fmt.Errorf("the signals (%d × %v s) do not fit in the lead of %v s", b.Signals, b.SignalSeconds, b.LeadSeconds)
	case !in(b.ToneHz, 200, 3000):
		return fmt.Errorf("tone_hz %v: 200..3000 Hz", b.ToneHz)
	case !in(b.VolumeDB, -40, 0):
		return fmt.Errorf("volume_db %v: -40..0 dB", b.VolumeDB)
	case !in(b.LensEnergy, 0.1, 16) || !in(b.LightEnergy, 0, 8) || !in(b.LightRangeM, 0.5, 12):
		return fmt.Errorf("lens_energy %v, light_energy %v, light_range_m %v out of range", b.LensEnergy, b.LightEnergy, b.LightRangeM)
	case !cutawayColor.MatchString(b.Color):
		return fmt.Errorf("color %q: #RRGGBB", b.Color)
	case len(b.Levels) == 0:
		return fmt.Errorf("no level: say where the beacons stand")
	case len(b.FlashPhases) == 0:
		return fmt.Errorf("flash_phases: name the phases of a train in view in which the beacons keep flashing")
	}
	for _, phase := range b.FlashPhases {
		if !beaconFlashPhases[phase] {
			return fmt.Errorf("flash_phases: %q is not a phase of a train moving in or past (approaching, braking, passing)", phase)
		}
	}
	seen := map[string]bool{}
	for _, l := range b.Levels {
		switch {
		case seen[l.LevelID]:
			return fmt.Errorf("level %s twice", l.LevelID)
		case !known(l.LevelID):
			return fmt.Errorf("unknown level %s", l.LevelID)
		case l.InstanceID == "" || l.LampNode == "" || l.SounderNode == "":
			return fmt.Errorf("level %s: name the instance, the lamp and the sounder", l.LevelID)
		case l.Why == "":
			return fmt.Errorf("level %s: say why the beacons stand there", l.LevelID)
		}
		seen[l.LevelID] = true
	}
	return nil
}

// clientPlatformBeacons: the numbers and, per level, the instance and its part names (the reasons stay in CUE).
func clientPlatformBeacons(b *PlatformBeacons) map[string]any {
	levels := map[string]any{}
	for _, l := range b.Levels {
		levels[l.LevelID] = map[string]any{"instance_id": l.InstanceID, "lamp_node": l.LampNode, "sounder_node": l.SounderNode}
	}
	return map[string]any{"lead_seconds": b.LeadSeconds, "flash_phases": b.FlashPhases, "flash_hz": b.FlashHz, "flash_duty": b.FlashDuty, "signals": b.Signals,
		"signal_seconds": b.SignalSeconds, "gap_seconds": b.GapSeconds, "tone_hz": b.ToneHz, "volume_db": b.VolumeDB,
		"lens_energy": b.LensEnergy, "light_energy": b.LightEnergy, "light_range_m": b.LightRangeM, "color": b.Color, "levels": levels}
}
