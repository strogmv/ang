package gamepoc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func lookFixture(t *testing.T) (string, Art, *TurretWatchCatalog) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "art-source/caste/v1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	record := `{"looks": {"mark": {"file": "art-source/caste/v1/model/mark.glb", "sha256": "` + strings.Repeat("a", 64) + `",
		"surfaces": {"main": "M_main", "glow_visor": "M_glow_visor", "glow_wrist": "M_glow_wrist"}}}}`
	if err := os.WriteFile(filepath.Join(dir, "look.json"), []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}
	brighter := 12.0
	a := Art{
		Looks: map[string]Look{"look.guard": {ID: "look.guard", Record: "art-source/caste/v1/look.json", Model: "mark", Caste: "silver",
			Glow: []LookGlow{{Surface: "glow_wrist", Energy: 5, When: "always"}, {Surface: "glow_visor", Energy: 8, When: "on_duty", TrackingEnergy: &brighter}}}},
		PersonLooks: map[string]string{"char.guard": "look.guard"},
	}
	turrets := &TurretWatchCatalog{Posts: []TurretWatchPost{
		{ID: "th.turret.right", LevelID: "level.hub", GuardPost: "post.lift", Guards: []string{"char.guard"}},
		{ID: "th.turret.left", LevelID: "level.hub", GuardPost: "post.lift", Guards: []string{"char.guard", "char.other"}},
		{ID: "th.turret.far", LevelID: "level.far", GuardPost: "post.far", Guards: []string{"char.other"}},
	}}
	return root, a, turrets
}

func TestLooksAcceptedAndProjected(t *testing.T) {
	root, a, turrets := lookFixture(t)
	person := func(id string) bool { return id == "char.guard" }
	if err := checkLooks(root, a, person, turrets); err != nil {
		t.Fatal(err)
	}
	looks, people := clientLooks(a, turrets)
	raw, err := json.Marshal(map[string]any{"looks": looks, "person_looks": people})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"looks":[{"caste":"silver","glow":[{"surface":"glow_visor","energy":8,"when":"on_duty","tracking_energy":12},{"surface":"glow_wrist","energy":5,"when":"always"}],` +
		`"id":"look.guard","model":"mark","pack":"res://actors-assets/looks/mark.scn","record":"art-source/caste/v1/look.json"}],` +
		`"person_looks":[{"duty":[{"post":"post.lift","level":"level.hub","turrets":["th.turret.left","th.turret.right"]}],"id":"char.guard","look":"look.guard"}]}`
	if string(raw) != want {
		t.Fatalf("client looks:\n got %s\nwant %s", raw, want)
	}
	// A world without looks keeps its presentation resource unchanged.
	if _, ok := clientArt(Compiled{})["looks"]; ok {
		t.Fatal("a world without looks must not get art.looks")
	}
}

func TestLooksRejected(t *testing.T) {
	person := func(id string) bool { return id == "char.guard" || id == "char.free" }
	cases := map[string]func(root string, a *Art, turrets **TurretWatchCatalog){
		"id": func(_ string, a *Art, _ **TurretWatchCatalog) {
			l := a.Looks["look.guard"]
			l.ID = "look.other"
			a.Looks["look.guard"] = l
		},
		"caste": func(_ string, a *Art, _ **TurretWatchCatalog) {
			l := a.Looks["look.guard"]
			l.Caste = "iron"
			a.Looks["look.guard"] = l
		},
		"model_name": func(_ string, a *Art, _ **TurretWatchCatalog) {
			l := a.Looks["look.guard"]
			l.Model = "../mark"
			a.Looks["look.guard"] = l
		},
		"record_traversal": func(_ string, a *Art, _ **TurretWatchCatalog) {
			l := a.Looks["look.guard"]
			l.Record = "art-source/../look.json"
			a.Looks["look.guard"] = l
		},
		"record_outside": func(_ string, a *Art, _ **TurretWatchCatalog) {
			l := a.Looks["look.guard"]
			l.Record = "content/look.json"
			a.Looks["look.guard"] = l
		},
		"record_missing": func(_ string, a *Art, _ **TurretWatchCatalog) {
			l := a.Looks["look.guard"]
			l.Record = "art-source/none/look.json"
			a.Looks["look.guard"] = l
		},
		"model_missing": func(_ string, a *Art, _ **TurretWatchCatalog) {
			l := a.Looks["look.guard"]
			l.Model = "other"
			a.Looks["look.guard"] = l
		},
		"glow_not_glow": func(_ string, a *Art, _ **TurretWatchCatalog) { a.Looks["look.guard"].Glow[0].Surface = "main" },
		"glow_unknown":  func(_ string, a *Art, _ **TurretWatchCatalog) { a.Looks["look.guard"].Glow[0].Surface = "glow_chest" },
		"glow_twice":    func(_ string, a *Art, _ **TurretWatchCatalog) { a.Looks["look.guard"].Glow[0].Surface = "glow_visor" },
		"glow_when":     func(_ string, a *Art, _ **TurretWatchCatalog) { a.Looks["look.guard"].Glow[0].When = "night" },
		"glow_energy":   func(_ string, a *Art, _ **TurretWatchCatalog) { a.Looks["look.guard"].Glow[0].Energy = 17 },
		"glow_negative": func(_ string, a *Art, _ **TurretWatchCatalog) { a.Looks["look.guard"].Glow[0].Energy = -1 },
		"tracking_always": func(_ string, a *Art, _ **TurretWatchCatalog) {
			v := 9.0
			a.Looks["look.guard"].Glow[0].TrackingEnergy = &v
		},
		"tracking_dimmer": func(_ string, a *Art, _ **TurretWatchCatalog) {
			v := 7.0
			a.Looks["look.guard"].Glow[1].TrackingEnergy = &v
		},
		"unknown_look":       func(_ string, a *Art, _ **TurretWatchCatalog) { a.PersonLooks["char.guard"] = "look.none" },
		"unknown_person":     func(_ string, a *Art, _ **TurretWatchCatalog) { a.PersonLooks["char.nobody"] = "look.guard" },
		"on_duty_not_guard":  func(_ string, a *Art, _ **TurretWatchCatalog) { a.PersonLooks["char.free"] = "look.guard" },
		"on_duty_no_turrets": func(_ string, _ *Art, turrets **TurretWatchCatalog) { *turrets = nil },
		"record_bad_sha": func(root string, _ *Art, _ **TurretWatchCatalog) {
			_ = os.WriteFile(filepath.Join(root, "art-source/caste/v1/look.json"), []byte(`{"looks":{"mark":{"file":"art-source/caste/v1/model/mark.glb","sha256":"x","surfaces":{"glow_visor":"a","glow_wrist":"b"}}}}`), 0o644)
		},
		"record_bad_file": func(root string, _ *Art, _ **TurretWatchCatalog) {
			_ = os.WriteFile(filepath.Join(root, "art-source/caste/v1/look.json"), []byte(`{"looks":{"mark":{"file":"/tmp/mark.glb","sha256":"`+strings.Repeat("a", 64)+`","surfaces":{"glow_visor":"a","glow_wrist":"b"}}}}`), 0o644)
		},
	}
	for name, spoil := range cases {
		t.Run(name, func(t *testing.T) {
			root, a, turrets := lookFixture(t)
			spoil(root, &a, &turrets)
			if err := checkLooks(root, a, person, turrets); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}
