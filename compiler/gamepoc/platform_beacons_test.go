package gamepoc

import (
	"encoding/json"
	"strings"
	"testing"
)

func platformBeaconsFixture() (PlatformBeacons, func(string) bool) {
	b := PlatformBeacons{LeadSeconds: 10, FlashPhases: []string{"approaching", "braking", "passing"}, FlashHz: 1.5, FlashDuty: 0.5, Signals: 2, SignalSeconds: 0.25, GapSeconds: 0.2, ToneHz: 880,
		VolumeDB: -14, LensEnergy: 6, LightEnergy: 1.2, LightRangeM: 5, Color: "#FFA000",
		Levels: []PlatformBeaconLevel{{LevelID: "level.hub", InstanceID: "th.platform", LampNode: "beacon_lamp", SounderNode: "beacon_sounder",
			Why: "the open platform edge"}}}
	return b, func(id string) bool { return id == "level.hub" }
}

func TestPlatformBeaconsAcceptedAndProjected(t *testing.T) {
	b, known := platformBeaconsFixture()
	if err := checkPlatformBeacons(b, known); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(clientPlatformBeacons(&b))
	for _, want := range []string{`"lead_seconds":10`, `"signals":2`, `"tone_hz":880`, `"color":"#FFA000"`,
		`"flash_phases":["approaching","braking","passing"]`, `"level.hub":{"instance_id":"th.platform","lamp_node":"beacon_lamp","sounder_node":"beacon_sounder"}`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("client beacons lack %s: %s", want, raw)
		}
	}
	if strings.Contains(string(raw), "why") {
		t.Fatalf("the reasons stay in CUE: %s", raw)
	}
}

func TestPlatformBeaconsRejected(t *testing.T) {
	cases := map[string]func(b *PlatformBeacons){
		"lead_short":    func(b *PlatformBeacons) { b.LeadSeconds = 1 },
		"strobe":        func(b *PlatformBeacons) { b.FlashHz = 10 },
		"duty":          func(b *PlatformBeacons) { b.FlashDuty = 1 },
		"no_signal":     func(b *PlatformBeacons) { b.Signals = 0 },
		"signals_long":  func(b *PlatformBeacons) { b.Signals = 5; b.SignalSeconds = 1; b.GapSeconds = 1; b.LeadSeconds = 5 },
		"tone":          func(b *PlatformBeacons) { b.ToneHz = 50 },
		"loud":          func(b *PlatformBeacons) { b.VolumeDB = 3 },
		"colour":        func(b *PlatformBeacons) { b.Color = "amber" },
		"no_level":      func(b *PlatformBeacons) { b.Levels = nil },
		"unknown_level": func(b *PlatformBeacons) { b.Levels[0].LevelID = "level.nowhere" },
		"level_twice":   func(b *PlatformBeacons) { b.Levels = append(b.Levels, b.Levels[0]) },
		"no_lamp":       func(b *PlatformBeacons) { b.Levels[0].LampNode = "" },
		"no_reason":     func(b *PlatformBeacons) { b.Levels[0].Why = "" },
		"no_phases":     func(b *PlatformBeacons) { b.FlashPhases = nil },
		"berthed_phase": func(b *PlatformBeacons) { b.FlashPhases = []string{"berthed"} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			b, known := platformBeaconsFixture()
			mutate(&b)
			if err := checkPlatformBeacons(b, known); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}
