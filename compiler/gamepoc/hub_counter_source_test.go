package gamepoc

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/strogmv/ang/compiler/locationsdk"
)

func TestAuthoredHubCounterPassportChangesOnlyItsFourCells(t *testing.T) {
	path := os.Getenv("ANG_HUB_COUNTER_SDK_JSON")
	if path == "" {
		t.Skip("set ANG_HUB_COUNTER_SDK_JSON to the fresh live CUE sdk export")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var input locationsdk.Input
	if err = json.Unmarshal(raw, &input); err != nil {
		t.Fatal(err)
	}
	var layout *locationsdk.Layout
	for i := range input.Layouts {
		if input.Layouts[i].ID == "example.c7_transport_hub" {
			layout = &input.Layouts[i]
		}
	}
	if layout == nil {
		t.Fatal("actual source SDK has no transport layout")
	}
	candidate, err := locationsdk.Build(input.Catalog, *layout)
	if err != nil {
		t.Fatal(err)
	}
	baselineCatalog := input.Catalog
	baselineCatalog.Geometries = append([]locationsdk.Geometry{}, input.Catalog.Geometries...)
	found := false
	for i := range baselineCatalog.Geometries {
		g := &baselineCatalog.Geometries[i]
		if g.ID != "geo.th.security.post" {
			continue
		}
		found = true
		if len(g.Footprint) != 8 || len(g.Blocked) != 8 || len(g.MaintenanceClearance) != 8 {
			t.Fatal("counter does not match measured two-row passport")
		}
		// The previous authored passport occupied only the front half-metre row.
		g.Footprint = nil
		g.Blocked = nil
		g.MaintenanceClearance = nil
		for x := 0; x < 4; x++ {
			g.Footprint = append(g.Footprint, locationsdk.Cell{X: x, Y: 3})
		}
		g.Blocked = append([]locationsdk.Cell{}, g.Footprint...)
		for y := 0; y < 3; y++ {
			for x := 0; x < 4; x++ {
				g.MaintenanceClearance = append(g.MaintenanceClearance, locationsdk.Cell{X: x, Y: y})
			}
		}
	}
	if !found {
		t.Fatal("missing actual counter passport")
	}
	baseline, err := locationsdk.Build(baselineCatalog, *layout)
	if err != nil {
		t.Fatal(err)
	}
	expected := map[locationsdk.Cell]bool{}
	for x := 74; x <= 77; x++ {
		expected[locationsdk.Cell{X: x, Y: 24}] = true
	}
	changed := map[locationsdk.Cell]bool{}
	for y, row := range candidate.Terrain {
		for x, glyph := range row {
			old := baseline.Terrain[y][x]
			if byte(glyph) == old {
				continue
			}
			at := locationsdk.Cell{X: x, Y: y}
			if !expected[at] || old != '.' || glyph != '+' {
				t.Fatalf("unexpected authoritative terrain change (%d,%d): %q to %q", x, y, old, glyph)
			}
			changed[at] = true
		}
	}
	if !reflect.DeepEqual(changed, expected) {
		t.Fatal("counter terrain delta differs from the measured four cells", changed)
	}
	if !reflect.DeepEqual(candidate.Ports, baseline.Ports) || !reflect.DeepEqual(candidate.ReservedPaths, baseline.ReservedPaths) || !reflect.DeepEqual(candidate.Preview.Floor, baseline.Preview.Floor) {
		t.Fatal("counter correction changed ports, protected routes or supported floor")
	}
	a, b := candidate.Instances, baseline.Instances
	if len(a) != len(b) {
		t.Fatal("counter correction changed instances")
	}
	for i := range a {
		if a[i].ID == "th.security.post" {
			if a[i].At != b[i].At || a[i].Rotation != b[i].Rotation {
				t.Fatal("counter pivot changed")
			}
			continue
		}
		if !reflect.DeepEqual(a[i], b[i]) {
			t.Fatal("unrelated authored instance changed", a[i].ID)
		}
	}
	t.Logf("exact authored terrain delta: newly blocked (74..77,24), ports/reservations/supported floor unchanged; guard origin (72,24) remains outside footprint")
}
