package gamepoc

import (
	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"encoding/json"
	"github.com/strogmv/ang/compiler/locationsdk"
	"os"
	"strings"
	"testing"
)

func exitZoneFixture() (Compiled, ExitZoneCatalog) {
	c := Compiled{GodotTargets: []string{"../client"}, Life: json.RawMessage(`{"locations":{"room":{"cell_mm":500,"terrain":["#####","#...#","#...#","#...#","#####"]}},"edges":[{"id":"leave","from":"room","to":"other","exit":{"x":1,"y":2}}]}`), Walkable: map[string]bool{".": true}, Localization: LocalizationCatalog{Messages: []LocalizationMessage{{Key: "ui.zone", Context: "destination, method and duration", Texts: map[string]string{"ru": "Выход", "en": "Exit"}}}}}
	c.ModularLocations = []ModularLocation{{LevelID: "room", LayoutID: "layout.room"}}
	c.LocationSDK = &locationsdk.RuntimeBundle{Levels: []locationsdk.RuntimeLevel{{ID: "layout.room", UnitMM: 500, Ports: []locationsdk.CompiledPort{{ID: "door", InstanceID: "entry", Kind: "connection", At: locationsdk.Cell{X: 0, Y: 2}, Approach: []locationsdk.Cell{{X: 1, Y: 2}, {X: 1, Y: 3}}}}}}}
	return c, ExitZoneCatalog{Zones: []ExitZone{{EdgeID: "leave", TextKey: "ui.zone", DestinationKey: "ui.zone", Kind: "walk", Radius: 1}}}
}
func TestExitZonesSecondLayoutByData(t *testing.T) {
	c, z := exitZoneFixture()
	if err := z.compile(c); err != nil {
		t.Fatal(err)
	}
	if z.Zones[0].Source != "sdk_port_approach" || len(z.Zones[0].Cells) < 3 {
		t.Fatal("SDK doorway was not extended onto its floor")
	}
	c.Life = json.RawMessage(strings.ReplaceAll(string(c.Life), "room", "second"))
	c.ModularLocations[0].LevelID = "second"
	c.LocationSDK.Levels[0].Ports[0].Approach = []locationsdk.Cell{{X: 3, Y: 2}}
	z.Zones[0].Cells = nil
	if err := z.compile(c); err != nil {
		t.Fatal(err)
	}
	if z.Zones[0].LevelID != "second" {
		t.Fatal("second layout needed a code branch")
	}
}
func TestExitZonesRejectMissingDuplicateOrphanFloorAndText(t *testing.T) {
	for _, test := range []struct {
		name, want string
		mutate     func(*Compiled, *ExitZoneCatalog)
	}{
		{"missing", "no exit zone", func(c *Compiled, z *ExitZoneCatalog) { z.Zones = nil }},
		{"duplicate", "more than one", func(c *Compiled, z *ExitZoneCatalog) { z.Zones = append(z.Zones, z.Zones[0]) }},
		{"orphan", "no Life edge", func(c *Compiled, z *ExitZoneCatalog) { z.Zones[0].EdgeID = "invented" }},
		{"blocked", "not walkable floor", func(c *Compiled, z *ExitZoneCatalog) { z.Zones[0].Cells = []locationsdk.Cell{{X: 0, Y: 0}} }},
		{"unknown text", "requires context, RU and EN", func(c *Compiled, z *ExitZoneCatalog) { z.Zones[0].TextKey = "ui.absent" }},
		{"missing EN", "requires context, RU and EN", func(c *Compiled, z *ExitZoneCatalog) { delete(c.Localization.Messages[0].Texts, "en") }},
		{"no context", "requires context, RU and EN", func(c *Compiled, z *ExitZoneCatalog) { c.Localization.Messages[0].Context = "" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			c, z := exitZoneFixture()
			test.mutate(&c, &z)
			if err := z.compile(c); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("wanted %q, got %v", test.want, err)
			}
		})
	}
}
func TestExitZoneMissingCatalogFailsGenerateInput(t *testing.T) {
	c, _ := exitZoneFixture()
	if _, err := readExitZones(cuecontext.New().CompileString(`{}`), c); err == nil || !strings.Contains(err.Error(), "require floor zones") {
		t.Fatal(err)
	}
}

func TestExitZonesTwoCUELayoutsNeedNoNewCode(t *testing.T) {
	source, err := os.ReadFile("testdata/exit-zones/layouts.cue")
	if err != nil {
		t.Fatal(err)
	}
	value := cuecontext.New().CompileBytes(source)
	if err = value.Err(); err != nil {
		t.Fatal(err)
	}
	var shapes [][]locationsdk.Cell
	for _, name := range []string{"first", "second"} {
		t.Run(name, func(t *testing.T) {
			root := value.LookupPath(cue.ParsePath(name))
			raw, err := root.LookupPath(cue.ParsePath("compiled")).MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			var c Compiled
			if err = json.Unmarshal(raw, &c); err != nil {
				t.Fatal(err)
			}
			zones, err := readExitZones(root.LookupPath(cue.ParsePath("world")), c)
			if err != nil {
				t.Fatal(err)
			}
			if len(zones.Zones) != 1 || zones.Zones[0].Source != "sdk_port_approach" || len(zones.Zones[0].Cells) < 3 {
				t.Fatal("CUE floor zone was not derived from its SDK port")
			}
			shapes = append(shapes, zones.Zones[0].Cells)
		})
	}
	a, _ := json.Marshal(shapes[0])
	b, _ := json.Marshal(shapes[1])
	if string(a) == string(b) {
		t.Fatal("different CUE layouts produced the same floor zone")
	}
}
