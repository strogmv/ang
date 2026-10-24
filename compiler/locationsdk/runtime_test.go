package locationsdk

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func runtimeTestInput() Input { return Input{Catalog: testCatalog(), Layouts: testLayouts()} }

func runtimeTestOptions() RuntimeOptions {
	return RuntimeOptions{TickHz: 10, MoveEveryTicks: 3, InitialTimeID: "dim", InitialEventID: "festival"}
}

func requireRuntime(t *testing.T, in Input, options RuntimeOptions) RuntimeBundle {
	t.Helper()
	out, err := CompileRuntime(in, options)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestRuntimeAdapterUsesSameGeometryHalfMeterNavigationAndAuthoredSpawn(t *testing.T) {
	in := runtimeTestInput()
	bundle := requireRuntime(t, in, runtimeTestOptions())
	if bundle.Schema != RuntimeSchema || bundle.UnitMM != 500 || bundle.CellMeters != .5 || !bundle.PublicArchitecture || bundle.CollisionModel != "cardinal_cell_occupancy" || bundle.MovementStepNS != 300000000 || len(bundle.ContentHash) != 64 {
		t.Fatalf("wrong runtime metric/clock contract: %#v", bundle)
	}
	if bundle.InitialTimeID != "dim" || bundle.InitialEventID != "festival" || len(bundle.Levels) != 3 {
		t.Fatal("initial presentation or whole input batch lost")
	}
	for _, level := range bundle.Levels {
		var source Layout
		for _, l := range in.Layouts {
			if l.ID == level.ID {
				source = l
			}
		}
		compiled := requireBuilt(t, in.Catalog, source)
		if level.Scenario != "sdk:"+source.ID || level.Seed != source.Seed || level.Spawn != source.Entries[0] || !reflect.DeepEqual(level.Entries, source.Entries) {
			t.Fatal("adapter invented a scenario ID, seed or spawn")
		}
		if level.UnitMM != 500 || level.CellMeters != .5 || level.MoveEveryTicks != 3 || level.MovementStepNS != 300000000 || level.StructuralHash != compiled.StructuralHash || !level.InitialLayoutKnown {
			t.Fatal("per-level metric/clock/structure contract drifted")
		}
		if !reflect.DeepEqual(level.Terrain, compiled.Terrain) || !reflect.DeepEqual(level.Instances, compiled.Instances) || !reflect.DeepEqual(level.Ports, compiled.Ports) || !reflect.DeepEqual(level.Clearances, compiled.Clearances) || !reflect.DeepEqual(level.Drops, compiled.Drops) || !reflect.DeepEqual(level.Barriers, compiled.Barriers) || !reflect.DeepEqual(level.Floor, compiled.Preview.Floor) {
			t.Fatal("runtime adapter changed compiled geometry, metadata or presentation")
		}
		for j, view := range level.Views {
			sourceView := compiled.Views[j]
			if view.TimeID != sourceView.TimeID || view.EventID != sourceView.EventID || view.StructuralHash != sourceView.StructuralHash || view.Brightness != sourceView.Brightness || view.AccentColor != sourceView.AccentColor || !reflect.DeepEqual(view.ActivityTags, sourceView.ActivityTags) || len(view.Decorations) != len(sourceView.Decorations) {
				t.Fatal("runtime presentation changed time/event/geometry or invented attachments")
			}
			for k, decoration := range view.Decorations {
				sourceDecoration := sourceView.Decorations[k]
				if decoration.Anchor != sourceDecoration.At || decoration.ID != sourceDecoration.ID || decoration.InstanceID != sourceDecoration.InstanceID || decoration.Z != sourceDecoration.Z || decoration.Reason != sourceDecoration.Reason {
					t.Fatal("runtime attachment lost its authoring identity/pivot/purpose")
				}
			}
		}
		if !reflect.DeepEqual(level.WalkableGlyphs, []string{"."}) || !reflect.DeepEqual(level.OpaqueGlyphs, []string{"#", " "}) {
			t.Fatal("blocked or unsupported floor gained movement")
		}
		if level.Terrain[level.Spawn.Y][level.Spawn.X] != '.' {
			t.Fatal("runtime spawn is blocked")
		}
		for _, view := range level.Views {
			if view.TimeID == "dim" && view.Brightness != .64 {
				t.Fatal("night festival acquired daylight")
			}
		}
		for _, p := range level.ReservedPaths {
			for _, authored := range source.ReservedPaths {
				if p.ID == authored.ID && (!reflect.DeepEqual(p.Points, authored.Points) || p.Width != authored.Width || p.Reason != authored.Reason) {
					t.Fatal("floor guide lost authored centreline order, width or purpose")
				}
			}
		}
	}
}

func TestRuntimeContractIsFlatAndRoundTripsWithoutUnityMapsOrHiddenActors(t *testing.T) {
	bundle := requireRuntime(t, runtimeTestInput(), runtimeTestOptions())
	data, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Levels []map[string]json.RawMessage `json:"levels"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	for _, level := range wire.Levels {
		for _, field := range []string{"id", "scenario", "unit_mm", "cell_meters", "spawn", "entries", "terrain", "instances", "ports", "clearances", "reserved_paths", "barriers", "drops", "views", "floor"} {
			if _, ok := level[field]; !ok {
				t.Fatal("missing flat Unity field " + field)
			}
		}
		for _, hidden := range []string{"scene", "actors", "npcs", "items", "edges", "inventory"} {
			if _, ok := level[hidden]; ok {
				t.Fatal("runtime adapter invented nested scene, actors, items or connections")
			}
		}
	}
	var restored RuntimeBundle
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored, bundle) {
		t.Fatal("Go/Unity JSON contract lost typed data on round trip")
	}
}

func TestRuntimeClockUsesAuthoritativeIntegerTickQuantization(t *testing.T) {
	options := runtimeTestOptions()
	options.TickHz, options.MoveEveryTicks = 60, 7
	bundle := requireRuntime(t, runtimeTestInput(), options)
	want := int64(1000000000/60) * 7
	if bundle.MovementStepNS != want {
		t.Fatalf("step interval %d differs from existing server moveTicks*tickNS %d", bundle.MovementStepNS, want)
	}
	for _, l := range bundle.Levels {
		if l.MovementStepNS != want || l.CellMeters != .5 {
			t.Fatal("level uses a second visual clock or changes grid size")
		}
	}
}

func TestRuntimeAdapterRejectsWholeInvalidBatchAndBadClockOrPresentation(t *testing.T) {
	tests := []struct {
		name, code string
		edit       func(*Input, *RuntimeOptions)
	}{
		{"empty batch", "runtime_batch", func(i *Input, o *RuntimeOptions) { i.Layouts = nil }},
		{"duplicate level", "duplicate_layout", func(i *Input, o *RuntimeOptions) { i.Layouts = append(i.Layouts, i.Layouts[0]) }},
		{"invalid final plan", "unknown_module", func(i *Input, o *RuntimeOptions) { i.Layouts[len(i.Layouts)-1].Instances[0].ModuleID = "missing" }},
		{"wrong half-metre units", "unsupported_units", func(i *Input, o *RuntimeOptions) { i.Layouts[1].UnitMM = 1000 }},
		{"unverified artwork", "unsupported_unverified_asset", func(i *Input, o *RuntimeOptions) { i.Catalog.Modules[0].Variants[0].ImageRef = "unverified.png" }},
		{"no server tick rate", "runtime_clock", func(i *Input, o *RuntimeOptions) { o.TickHz = 0 }},
		{"unsupported tick rate", "runtime_clock", func(i *Input, o *RuntimeOptions) { o.TickHz = 241 }},
		{"no movement cadence", "runtime_clock", func(i *Input, o *RuntimeOptions) { o.MoveEveryTicks = 0 }},
		{"clock interval overflow", "runtime_clock", func(i *Input, o *RuntimeOptions) { o.MoveEveryTicks = math.MaxInt64 }},
		{"unknown initial time", "runtime_presentation", func(i *Input, o *RuntimeOptions) { o.InitialTimeID = "absent" }},
		{"unknown initial event", "runtime_presentation", func(i *Input, o *RuntimeOptions) { o.InitialEventID = "absent" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			in, options := runtimeTestInput(), runtimeTestOptions()
			test.edit(&in, &options)
			bundle, err := CompileRuntime(in, options)
			requireCode(t, err, test.code)
			if !reflect.DeepEqual(bundle, RuntimeBundle{}) {
				t.Fatal("failed runtime compilation exposed a partial bundle")
			}
			if test.name == "invalid final plan" {
				var invalid *ValidationError
				if !errors.As(err, &invalid) || !strings.HasPrefix(invalid.Diagnostics[0].Path, "layouts[2].") {
					t.Fatal("runtime error lost the input layout path")
				}
			}
		})
	}
}

func TestRuntimeAdapterCanonicalOutputAndNoMutableInputAliases(t *testing.T) {
	in, options := runtimeTestInput(), runtimeTestOptions()
	before, _ := json.Marshal(in)
	bundle := requireRuntime(t, in, options)
	canonical, _ := json.Marshal(bundle)
	reordered := copyValue(t, in)
	slices.Reverse(reordered.Layouts)
	slices.Reverse(reordered.Catalog.Geometries)
	slices.Reverse(reordered.Catalog.Modules)
	slices.Reverse(reordered.Catalog.Events)
	slices.Reverse(reordered.Catalog.Times)
	for i := range reordered.Layouts {
		slices.Reverse(reordered.Layouts[i].Instances)
		slices.Reverse(reordered.Layouts[i].ReservedPaths)
	}
	other := requireRuntime(t, reordered, options)
	otherJSON, _ := json.Marshal(other)
	if string(canonical) != string(otherJSON) {
		t.Fatal("input ordering changed runtime JSON/hash")
	}
	for i := range bundle.Levels {
		level := &bundle.Levels[i]
		level.Entries[0].X = 999
		level.Terrain[0] = "edited output"
		if len(level.Instances) > 0 {
			level.Instances[0].Footprint[0].X = 999
		}
		if len(level.ReservedPaths) > 0 {
			level.ReservedPaths[0].Points[0].X = 999
			level.ReservedPaths[0].Cells[0].X = 999
		}
		if len(level.Views[0].ActivityTags) > 0 {
			level.Views[0].ActivityTags[0] = "edited output"
		}
	}
	after, _ := json.Marshal(in)
	if string(before) != string(after) {
		t.Fatal("runtime result aliases or mutates authoring input")
	}
	rebuilt := requireRuntime(t, in, options)
	rebuiltJSON, _ := json.Marshal(rebuilt)
	if string(canonical) != string(rebuiltJSON) {
		t.Fatal("editing prior runtime output changed a later compilation")
	}
}

func TestRuntimeInitialPresentationComesFromAuthoredAxisOrder(t *testing.T) {
	in := runtimeTestInput()
	slices.Reverse(in.Catalog.Times)
	bundle := requireRuntime(t, in, RuntimeOptions{TickHz: 10, MoveEveryTicks: 3})
	if bundle.InitialTimeID != in.Catalog.Times[0].ID || bundle.InitialEventID != in.Catalog.Events[0].ID {
		t.Fatal("initial choice came from sorted output or hardcoded state names rather than the author")
	}
	if bundle.InitialEventID == bundle.Levels[0].Views[0].EventID {
		t.Fatal("test needs a source event order different from canonical output order")
	}
}

func TestRuntimeDecorationBoxDimensionsAndOriginFollowQuarterTurns(t *testing.T) {
	for _, tc := range []struct {
		rotation   int
		at, anchor Cell
		w, d       int
	}{
		{0, Cell{8, 5}, Cell{8, 5}, 2, 1},
		{90, Cell{7, 6}, Cell{7, 6}, 1, 2},
		{180, Cell{6, 5}, Cell{7, 5}, 2, 1},
		{270, Cell{7, 4}, Cell{7, 5}, 1, 2},
	} {
		in := runtimeTestInput()
		in.Layouts = in.Layouts[:1]
		source := &in.Layouts[0]
		source.Instances = source.Instances[:1]
		source.Instances[0].At = Cell{6, 4}
		source.Instances[0].Rotation = tc.rotation
		source.ReservedPaths = nil
		source.Elevation = -5
		in.Catalog.Geometries[0].Width, in.Catalog.Geometries[0].Depth = 4, 3
		in.Catalog.Geometries[0].Sockets[0] = Socket{ID: "top", At: Cell{2, 1}, Z: 2, Width: 2, Depth: 2, Height: 1}
		in.Catalog.Decorations[0].Width, in.Catalog.Decorations[0].Depth = 2, 1
		bundle := requireRuntime(t, in, runtimeTestOptions())
		level := bundle.Levels[0]
		for _, view := range level.Views {
			if view.EventID != "festival" {
				if len(view.Decorations) != 0 {
					t.Fatal("ordinary event gained a physical item")
				}
				continue
			}
			if len(view.Decorations) != 1 {
				t.Fatal("missing compiled socket attachment")
			}
			item := view.Decorations[0]
			if item.At != tc.at || item.Anchor != tc.anchor || item.Width != tc.w || item.Depth != tc.d || item.Height != 1 || item.Z != -3 || item.Rotation != tc.rotation {
				t.Fatalf("rotation%d lost the real attachment box, metric elevation or pivot: %#v", tc.rotation, item)
			}
			instance := level.Instances[0]
			if item.At.X < instance.At.X || item.At.Y < instance.At.Y || item.Width > instance.Width-(item.At.X-instance.At.X) || item.Depth > instance.Depth-(item.At.Y-instance.At.Y) {
				t.Fatal("rotated runtime attachment leaves its validated module")
			}
		}
	}
}
