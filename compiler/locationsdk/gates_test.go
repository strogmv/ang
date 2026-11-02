package locationsdk

import (
	"bytes"
	"encoding/json"
	"testing"
)

// A two-cell vertical-lift gate between two wall stubs; floor on both faces.
func gateTestInput() Input {
	c := testCatalog()
	gate := Geometry{ID: "shape.gate", Width: 4, Depth: 1, Height: 20, Footprint: []Cell{{0, 0}, {3, 0}}, Blocked: []Cell{{0, 0}, {3, 0}}, Ports: []Port{{ID: "passage", Kind: "door", At: Cell{1, 0}, Opening: []Cell{{1, 0}, {2, 0}}, Side: South, Approach: []Cell{{1, 1}, {2, 1}}}}}
	stub := Geometry{ID: "shape.stub", Width: 2, Depth: 1, Height: 20, Footprint: []Cell{{0, 0}, {1, 0}}, Blocked: []Cell{{0, 0}, {1, 0}}}
	c.Geometries = append(c.Geometries, gate, stub)
	for _, g := range []Geometry{gate, stub} {
		c.Modules = append(c.Modules, Module{ID: "module." + g.ID, GeometryID: g.ID, Variants: []Variant{{ID: "a", GeometryID: g.ID}}})
	}
	c.GateKits = []GateKit{{ID: "kit.gate", Kind: "vertical_lift", GeometryID: "shape.gate", PortID: "passage", ClearHeightMM: 6000, SightClearMM: 2100, GravityMMPerS: 300,
		TimingMS: GateTiming{Warning: 3000, Travel: 13000, Ramp: 1500, Retry: 5000, LockdownWarning: 3000, LockdownRetry: 5000, PowerWarning: 3000},
		Leaves:   []GateLeaf{{Node: "leaf.lower", Axis: "up", TravelMM: 6000}, {Node: "leaf.upper", Axis: "up", TravelMM: 3000}},
		Cables:   []GateCable{{Node: "cable.rope_l", LengthMM: 3090, TravelMM: 3000}, {Node: "cable.rope_r", LengthMM: 3090, TravelMM: 3000}},
		Signals:  []GateSignal{{ID: "beacon", Node: "signal.beacon", Match: "prefix", Color: "#FF8C12", Intensity: 1.5, BackupPower: true, Rules: []GateSignalRule{{State: "warning", Motion: "any", Pattern: "flash", PeriodMS: 500, DutyPermille: 500}}}}}}
	layout := Layout{ID: "author.gate", UnitMM: UnitMM, Seed: 5, Width: 8, Depth: 7, Grounded: true, StartSecond: 21 * 3600,
		Floor: []FloorArea{{ID: "all", Rect: Rect{0, 0, 8, 7}, Reason: "both faces"}}, Entries: []Cell{{1, 5}},
		Instances: []Instance{{ID: "gate", ModuleID: "module.shape.gate", At: Cell{2, 3}, Reason: "gate bay"}, {ID: "wall.w", ModuleID: "module.shape.stub", At: Cell{0, 3}, Reason: "wall"}, {ID: "wall.e", ModuleID: "module.shape.stub", At: Cell{6, 3}, Reason: "wall"}},
		Gates:     []GatePlacement{{ID: "gate.one", InstanceID: "gate", KitID: "kit.gate", Schedule: GateSchedule{OpenSecond: 6 * 3600, CloseSecond: 22 * 3600, LockAtNight: true}, Reason: "market hours"}},
		Residents: []Resident{{ID: "porter", Name: "Porter", Reason: "crosses", Steps: []ResidentStep{{AtMinute: 0, At: Cell{1, 1}, Activity: "work", Label: "north"}, {AtMinute: 30, At: Cell{6, 5}, Activity: "work", Label: "south"}}}},
	}
	return Input{Catalog: c, Layouts: []Layout{layout}}
}

func TestGateCompilesOpeningTicksResidentsAndStart(t *testing.T) {
	out := requireRuntime(t, gateTestInput(), runtimeTestOptions())
	level := out.Levels[0]
	if level.StartSecond != 21*3600 || len(level.Gates) != 1 || len(level.Residents) != 1 {
		t.Fatalf("gate stand data missing: %+v", level)
	}
	g := level.Gates[0]
	if len(g.Opening) != 2 || g.Opening[0] != (Cell{3, 3}) || g.Opening[1] != (Cell{4, 3}) || g.WarningTicks != 30 || g.TravelTicks != 130 || g.GravityTicks != 200 || g.RetryTicks != 50 || g.SightClearPPM != 350000 || g.OpenSecond != 21600 || !g.LockAtNight || len(g.Leaves) != 2 || len(g.Signals) != 1 || g.RampTicks != 15 || len(g.Cables) != 2 || g.Cables[0].TravelMM != 3000 {
		t.Fatalf("compiled gate %+v", g)
	}
	if level.Residents[0].Steps[1].Nav != (Cell{6, 5}) || level.Residents[0].Steps[1].AtMinute != 30 {
		t.Fatalf("resident %+v", level.Residents[0])
	}
	if level.Terrain[3][3] != '.' || level.Terrain[3][2] != '#' {
		t.Fatal("static terrain must keep the opening as floor; the live glyph belongs to Go")
	}
	plain := requireRuntime(t, runtimeTestInput(), runtimeTestOptions())
	for _, l := range plain.Levels {
		if l.Gates != nil || l.Residents != nil || l.StartSecond != 0 {
			t.Fatal("layouts without gates changed")
		}
	}
}

func TestManualGateCompilesWithoutChangingScheduledGateData(t *testing.T) {
	in := gateTestInput()
	ordinary := requireRuntime(t, in, runtimeTestOptions()).Levels[0].Gates[0]
	if ordinary.Manual {
		t.Fatal("omitted manual flag changed a scheduled gate")
	}
	in.Layouts[0].Gates[0].Manual = true
	manual := requireRuntime(t, in, runtimeTestOptions()).Levels[0].Gates[0]
	if !manual.Manual {
		t.Fatal("manual placement was not exported to the runtime controller")
	}
	raw, err := json.Marshal(manual)
	if err != nil {
		t.Fatal(err)
	}
	var loaded RuntimeGate
	if err := json.Unmarshal(raw, &loaded); err != nil || !loaded.Manual {
		t.Fatal("manual flag was lost by runtime JSON", err)
	}
	manual.Manual = false
	a, errA := json.Marshal(ordinary)
	b, errB := json.Marshal(manual)
	if errA != nil || errB != nil || !bytes.Equal(a, b) || bytes.Contains(a, []byte(`"manual"`)) {
		t.Fatal("manual placement changed kit geometry/timing or legacy runtime bytes")
	}
}

func TestGateRejectsUnsafeOrInconsistentAuthoring(t *testing.T) {
	for code, change := range map[string]func(*Input){
		"gate_unknown_kit":       func(in *Input) { in.Layouts[0].Gates[0].KitID = "kit.none" },
		"gate_geometry_mismatch": func(in *Input) { in.Layouts[0].Gates[0].InstanceID = "wall.w" },
		"gate_instance": func(in *Input) {
			in.Layouts[0].Gates = append(in.Layouts[0].Gates, GatePlacement{ID: "gate.two", InstanceID: "gate", KitID: "kit.gate", Schedule: in.Layouts[0].Gates[0].Schedule, Reason: "twice"})
		},
		"gate_schedule": func(in *Input) {
			in.Layouts[0].Gates[0].Schedule.CloseSecond = in.Layouts[0].Gates[0].Schedule.OpenSecond
		},
		"gate_identity":         func(in *Input) { in.Layouts[0].Gates[0].Reason = " " },
		"gate_kit_port":         func(in *Input) { in.Catalog.GateKits[0].PortID = "front" },
		"gate_kit_leaf":         func(in *Input) { in.Catalog.GateKits[0].Leaves[1].Node = "leaf.lower" },
		"gate_kit_signal":       func(in *Input) { in.Catalog.GateKits[0].Signals[0].Node = "lamp.beacon" },
		"gate_kit_signal_rule":  func(in *Input) { in.Catalog.GateKits[0].Signals[0].Rules[0].State = "half_open" },
		"gate_kit_dimensions":   func(in *Input) { in.Catalog.GateKits[0].SightClearMM = 7000 },
		"gate_kit_timing":       func(in *Input) { in.Catalog.GateKits[0].TimingMS.Ramp = 7000 },
		"gate_kit_cable":        func(in *Input) { in.Catalog.GateKits[0].Cables[1].TravelMM = 3090 },
		"resident_step_cell":    func(in *Input) { in.Layouts[0].Residents[0].Steps[0].At = Cell{3, 3} },
		"resident_definition":   func(in *Input) { in.Layouts[0].Residents[0].Steps[1].AtMinute = 0 },
		"resident_post_blocked": func(in *Input) { in.Layouts[0].Residents[0].Steps[0].At = Cell{40, 40} },
		"start_second":          func(in *Input) { in.Layouts[0].StartSecond = 86400 },
	} {
		in := copyValue(t, gateTestInput())
		change(&in)
		_, err := CompileRuntime(in, runtimeTestOptions())
		t.Run(code, func(t *testing.T) { requireCode(t, err, code) })
	}
}
