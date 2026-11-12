package gamepoc

import (
	"encoding/json"
	"strings"
	"testing"
)

func accessZoneFixture() (AccessZone, []string, *TurretWatchCatalog) {
	z := AccessZone{ID: "zone.lift", LevelID: "level.hub", PolygonM: [][2]float64{{2, 1}, {2, 3}, {6, 3}, {6, 1}}, Marked: []bool{true, true, false, false},
		GuardPost: "post", Turrets: []string{"t.left"}, ScanSeconds: 3, CountdownSeconds: 10, Fire: "never",
		Marking: AccessZoneMarking{StripWidthM: 0.08, HatchWidthM: 0.3, HatchBarM: 0.07, HatchSpacingM: 0.2, HatchAngleDeg: 45, SteelColor: "#A7ADB2", PaintColor: "#D9DCD8"}}
	z.Access.Castes = []string{"silver"}
	z.Access.EscortByGuards = true
	terrain := []string{strings.Repeat(".", 20), strings.Repeat(".", 20), strings.Repeat(".", 20), strings.Repeat(".", 20), strings.Repeat(".", 20), strings.Repeat(".", 20), strings.Repeat(".", 20), strings.Repeat(".", 20)}
	turrets := &TurretWatchCatalog{Rule: TurretWatchRule{RangeM: 5}, Posts: []TurretWatchPost{{ID: "t.left", LevelID: "level.hub", GuardPost: "post"}}}
	turrets.Posts[0].Object.XM, turrets.Posts[0].Object.YM = 4, 1
	return z, terrain, turrets
}

func TestAccessZoneAcceptedAndProjected(t *testing.T) {
	z, terrain, turrets := accessZoneFixture()
	if err := checkAccessZone(z, true, terrain, 500, turrets, map[string]bool{}); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(clientAccessZones([]AccessZone{z}))
	if !strings.Contains(string(raw), `"polygon_m":[[2,1],[2,3],[6,3],[6,1]]`) || !strings.Contains(string(raw), `"marked":[true,true,false,false]`) || strings.Contains(string(raw), "countdown") {
		t.Fatalf("client zone: %s", raw)
	}
}

func TestAccessZoneRejected(t *testing.T) {
	cases := map[string]func(z *AccessZone, terrain *[]string, turrets **TurretWatchCatalog){
		"id":    func(z *AccessZone, _ *[]string, _ **TurretWatchCatalog) { z.ID = "lift" },
		"level": func(_ *AccessZone, terrain *[]string, _ **TurretWatchCatalog) { *terrain = nil },
		"two_vertices": func(z *AccessZone, _ *[]string, _ **TurretWatchCatalog) {
			z.PolygonM = z.PolygonM[:2]
			z.Marked = z.Marked[:2]
		},
		"marked_count":  func(z *AccessZone, _ *[]string, _ **TurretWatchCatalog) { z.Marked = z.Marked[:3] },
		"outside_level": func(z *AccessZone, _ *[]string, _ **TurretWatchCatalog) { z.PolygonM[2] = [2]float64{60, 3} },
		"self_crossing": func(z *AccessZone, _ *[]string, _ **TurretWatchCatalog) {
			z.PolygonM[1], z.PolygonM[2] = z.PolygonM[2], z.PolygonM[1]
		},
		"no_area": func(z *AccessZone, _ *[]string, _ **TurretWatchCatalog) {
			z.PolygonM = [][2]float64{{2, 1}, {3, 1}, {4, 1}}
			z.Marked = []bool{true, true, true}
		},
		"nothing_marked": func(z *AccessZone, _ *[]string, _ **TurretWatchCatalog) {
			z.Marked = []bool{false, false, false, false}
		},
		"no_turrets":     func(_ *AccessZone, _ *[]string, turrets **TurretWatchCatalog) { *turrets = nil },
		"unknown_turret": func(z *AccessZone, _ *[]string, _ **TurretWatchCatalog) { z.Turrets = []string{"t.none"} },
		"other_post":     func(z *AccessZone, _ *[]string, _ **TurretWatchCatalog) { z.GuardPost = "post.other" },
		"beyond_range":   func(z *AccessZone, _ *[]string, _ **TurretWatchCatalog) { z.PolygonM[2] = [2]float64{10, 3} },
		"nobody_may_enter": func(z *AccessZone, _ *[]string, _ **TurretWatchCatalog) {
			z.Access.Castes = nil
			z.Access.EscortByGuards = false
		},
		"unknown_caste": func(z *AccessZone, _ *[]string, _ **TurretWatchCatalog) { z.Access.Castes = []string{"iron"} },
		// never on approach: fire needs a scan and a countdown, and its runtime is not generated yet
		"fire_on_approach": func(z *AccessZone, _ *[]string, _ **TurretWatchCatalog) {
			z.Fire = "after_scan_and_countdown"
			z.CountdownSeconds = 0
		},
		"fire_not_built":  func(z *AccessZone, _ *[]string, _ **TurretWatchCatalog) { z.Fire = "after_scan_and_countdown" },
		"fire_unknown":    func(z *AccessZone, _ *[]string, _ **TurretWatchCatalog) { z.Fire = "on_approach" },
		"short_countdown": func(z *AccessZone, _ *[]string, _ **TurretWatchCatalog) { z.CountdownSeconds = 2 },
		"marking": func(z *AccessZone, _ *[]string, _ **TurretWatchCatalog) {
			z.Marking.HatchSpacingM = z.Marking.HatchBarM
		},
		"colour": func(z *AccessZone, _ *[]string, _ **TurretWatchCatalog) { z.Marking.PaintColor = "white" },
	}
	for name, spoil := range cases {
		t.Run(name, func(t *testing.T) {
			z, terrain, turrets := accessZoneFixture()
			spoil(&z, &terrain, &turrets)
			if err := checkAccessZone(z, terrain != nil, terrain, 500, turrets, map[string]bool{}); err == nil {
				t.Fatal("accepted")
			}
		})
	}
	z, terrain, turrets := accessZoneFixture()
	seen := map[string]bool{}
	if err := checkAccessZone(z, true, terrain, 500, turrets, seen); err != nil {
		t.Fatal(err)
	}
	if err := checkAccessZone(z, true, terrain, 500, turrets, seen); err == nil {
		t.Fatal("a duplicate zone id was accepted")
	}
}
