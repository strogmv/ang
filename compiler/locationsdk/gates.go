package locationsdk

import (
	"fmt"
	"regexp"
	"strings"
)

// GateKit is the runtime contract of an animated barrier kit (doc 85). The
// engineering passport stays in its authoring package; only the passage port,
// timings, fail-safe speed and the presentation node contract reach runtime.
type GateKit struct {
	ID            string       `json:"id"`
	Kind          string       `json:"kind"`
	GeometryID    string       `json:"geometry_id"`
	PortID        string       `json:"port_id"`
	ClearHeightMM int          `json:"clear_height_mm"`
	SightClearMM  int          `json:"sight_clear_mm"`
	GravityMMPerS int          `json:"gravity_mm_per_s"`
	TimingMS      GateTiming   `json:"timing_ms"`
	Leaves        []GateLeaf   `json:"leaves"`
	Cables        []GateCable  `json:"cables,omitempty"`
	Signals       []GateSignal `json:"signals"`
}

// GateTiming values are physical milliseconds. Ramp is the drive's
// acceleration and deceleration time at both ends of a powered travel
// (trapezoidal speed); 0 keeps constant speed. One ramp per drive: every leaf
// and cable of the gate hangs on the same drums, so they share one ease.
type GateTiming struct {
	Warning         int64 `json:"warning"`
	Travel          int64 `json:"travel"`
	Ramp            int64 `json:"ramp,omitempty"`
	Retry           int64 `json:"retry"`
	LockdownWarning int64 `json:"lockdown_warning"`
	LockdownRetry   int64 `json:"lockdown_retry"`
	PowerWarning    int64 `json:"power_warning"`
}

// GateLeaf names one moving node of the visual; it moves travel_mm × progress.
type GateLeaf struct {
	Node     string `json:"node"`
	Axis     string `json:"axis"`
	TravelMM int    `json:"travel_mm"`
}

// GateCable names a hoist rope of the visual hanging from a fixed drum: Unity
// shortens it along its vertical axis from its own pivot (the drum) to
// (length_mm - travel_mm × lift) / length_mm. The lift is the gate progress.
type GateCable struct {
	Node     string `json:"node"`
	LengthMM int    `json:"length_mm"`
	TravelMM int    `json:"travel_mm"`
}

// GateSignal lights every node matching Node by the first rule that matches the
// authoritative state. Signals without backup power are dark on a power loss.
type GateSignal struct {
	ID          string           `json:"id"`
	Node        string           `json:"node"`
	Match       string           `json:"match"`
	Color       string           `json:"color"`
	Intensity   float64          `json:"intensity"`
	BackupPower bool             `json:"backup_power"`
	Rules       []GateSignalRule `json:"rules"`
}

type GateSignalRule struct {
	State        string `json:"state"`
	Cause        string `json:"cause,omitempty"`
	Direction    int    `json:"direction,omitempty"`
	Motion       string `json:"motion"`
	Pattern      string `json:"pattern"`
	PeriodMS     int    `json:"period_ms"`
	DutyPermille int    `json:"duty_permille"`
	Color        string `json:"color,omitempty"`
}

type GateSchedule struct {
	OpenSecond  int64 `json:"open_second"`
	CloseSecond int64 `json:"close_second"`
	LockAtNight bool  `json:"lock_at_night"`
}

// GatePlacement binds a kit to one instance of its geometry in a layout.
type GatePlacement struct {
	ID         string       `json:"id"`
	InstanceID string       `json:"instance_id"`
	KitID      string       `json:"kit_id"`
	Schedule   GateSchedule `json:"schedule"`
	Reason     string       `json:"reason"`
}

// RuntimeGate is one compiled placement. Durations are motion units of the Life
// kernel (ticks; game seconds when the brain clock is on), i.e. the clock people
// walk on: physical ms × tick_hz / 1000, rounded up. Opening cells are world
// cells of the kit's door port; positions are parts per million of full lift.
type RuntimeGate struct {
	ID                   string       `json:"id"`
	InstanceID           string       `json:"instance_id"`
	KitID                string       `json:"kit_id"`
	Kind                 string       `json:"kind"`
	Opening              []Cell       `json:"opening"`
	ClearHeightMM        int          `json:"clear_height_mm"`
	SightClearPPM        int64        `json:"sight_clear_ppm"`
	WarningTicks         int64        `json:"warning_ticks"`
	TravelTicks          int64        `json:"travel_ticks"`
	RetryTicks           int64        `json:"retry_ticks"`
	LockdownWarningTicks int64        `json:"lockdown_warning_ticks"`
	LockdownRetryTicks   int64        `json:"lockdown_retry_ticks"`
	PowerWarningTicks    int64        `json:"power_warning_ticks"`
	GravityTicks         int64        `json:"gravity_ticks"`
	RampTicks            int64        `json:"ramp_ticks,omitempty"`
	OpenSecond           int64        `json:"open_second"`
	CloseSecond          int64        `json:"close_second"`
	LockAtNight          bool         `json:"lock_at_night"`
	Leaves               []GateLeaf   `json:"leaves"`
	Cables               []GateCable  `json:"cables,omitempty"`
	Signals              []GateSignal `json:"signals"`
}

var (
	gateLeafNode   = regexp.MustCompile(`^leaf\.[a-z0-9_.]+$`)
	gateCableNode  = regexp.MustCompile(`^cable\.[a-z0-9_.]+$`)
	gateSignalNode = regexp.MustCompile(`^signal\.[a-z0-9_.]+$`)
	gateColor      = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
	gateStates     = map[string]bool{"closed": true, "warning": true, "opening": true, "open": true, "closing": true, "stopped": true, "locked": true, "no_power": true, "lockdown": true}
	gateCauses     = map[string]bool{"": true, "schedule": true, "lockdown": true, "power": true}
)

func gateTicks(ms int64, tickHz int) int64 {
	return (ms*int64(tickHz) + 999) / 1000
}

func validateGateKit(kit GateKit, catalog Catalog, path string) *ValidationError {
	fail := func(code, message string, refs ...string) *ValidationError {
		return issue(code, path, message, append([]string{kit.ID}, refs...)...)
	}
	if !stableID.MatchString(kit.ID) || kit.Kind != "vertical_lift" {
		return fail("gate_kit_identity", "gate kit needs a stable ID; only vertical_lift kits are implemented")
	}
	var geometry *Geometry
	for i := range catalog.Geometries {
		if catalog.Geometries[i].ID == kit.GeometryID {
			geometry = &catalog.Geometries[i]
		}
	}
	if geometry == nil {
		return fail("gate_kit_geometry", "gate kit references an unknown geometry", kit.GeometryID)
	}
	port := false
	for _, p := range geometry.Ports {
		port = port || p.ID == kit.PortID && p.Kind == "door" && len(p.Opening) > 0
	}
	if !port {
		return fail("gate_kit_port", "gate kit needs a door port with a complete opening on its geometry", kit.PortID)
	}
	if kit.ClearHeightMM < 1000 || kit.ClearHeightMM > 20000 || kit.SightClearMM < 1 || kit.SightClearMM > kit.ClearHeightMM || kit.GravityMMPerS < 50 || kit.GravityMMPerS > 2000 {
		return fail("gate_kit_dimensions", "clear height 1..20 m, sight height within it and a governed descent of 50..2000 mm/s are required")
	}
	t := kit.TimingMS
	for _, ms := range []int64{t.Warning, t.Travel, t.Retry, t.LockdownWarning, t.LockdownRetry, t.PowerWarning} {
		if ms < 1 || ms > 600000 {
			return fail("gate_kit_timing", "every gate time must be within 1..600000 ms")
		}
	}
	if t.Ramp < 0 || 2*t.Ramp > t.Travel {
		return fail("gate_kit_timing", "the drive ramp must be within 0..travel/2 ms")
	}
	if len(kit.Leaves) < 1 || len(kit.Leaves) > 8 || len(kit.Signals) > 16 {
		return fail("gate_kit_nodes", "a gate needs 1..8 moving leaves and at most 16 signals")
	}
	nodes := map[string]bool{}
	for _, leaf := range kit.Leaves {
		if !gateLeafNode.MatchString(leaf.Node) || nodes[leaf.Node] || leaf.Axis != "up" || leaf.TravelMM < 1 || leaf.TravelMM > 2*kit.ClearHeightMM {
			return fail("gate_kit_leaf", "leaf nodes are unique leaf.* names moving up by a positive travel", leaf.Node)
		}
		nodes[leaf.Node] = true
	}
	if len(kit.Cables) > 8 {
		return fail("gate_kit_nodes", "a gate supports at most 8 cables")
	}
	for _, cable := range kit.Cables {
		if !gateCableNode.MatchString(cable.Node) || nodes[cable.Node] || cable.LengthMM < 1 || cable.LengthMM > 40000 || cable.TravelMM < 1 || cable.TravelMM >= cable.LengthMM {
			return fail("gate_kit_cable", "cable nodes are unique cable.* names that shorten by a positive travel less than their length", cable.Node)
		}
		nodes[cable.Node] = true
	}
	ids := map[string]bool{}
	for _, signal := range kit.Signals {
		if !stableID.MatchString(signal.ID) || ids[signal.ID] || !gateSignalNode.MatchString(signal.Node) || signal.Match != "exact" && signal.Match != "prefix" || !gateColor.MatchString(signal.Color) || !(signal.Intensity > 0 && signal.Intensity <= 16) || len(signal.Rules) > 32 {
			return fail("gate_kit_signal", "signals need unique IDs, signal.* nodes, exact/prefix match, a colour, intensity in (0,16] and at most 32 rules", signal.ID)
		}
		ids[signal.ID] = true
		for _, rule := range signal.Rules {
			if !gateStates[rule.State] || !gateCauses[rule.Cause] || rule.Direction < -1 || rule.Direction > 1 || rule.Motion != "any" && rule.Motion != "moving" && rule.Motion != "rest" || rule.Pattern != "steady" && rule.Pattern != "flash" || rule.PeriodMS < 100 || rule.PeriodMS > 4000 || rule.DutyPermille < 50 || rule.DutyPermille > 950 || rule.Color != "" && !gateColor.MatchString(rule.Color) {
				return fail("gate_kit_signal_rule", "signal rule has an unknown state/cause/motion/pattern or an out-of-range period/duty/colour", signal.ID, rule.State)
			}
		}
	}
	return nil
}

// compileGates resolves gate placements against the compiled level. A gate
// never invents an opening: it animates the door port of its own instance.
func compileGates(catalog Catalog, layout Layout, compiled CompiledLevel, tickHz int) ([]RuntimeGate, *ValidationError) {
	if len(layout.Gates) == 0 {
		return nil, nil
	}
	kits := map[string]GateKit{}
	for i, kit := range catalog.GateKits {
		if _, duplicate := kits[kit.ID]; duplicate {
			return nil, issue("gate_kit_identity", fmt.Sprintf("catalog.gate_kits[%d]", i), "gate kit IDs must be unique", kit.ID)
		}
		if invalid := validateGateKit(kit, catalog, fmt.Sprintf("catalog.gate_kits[%d]", i)); invalid != nil {
			return nil, invalid
		}
		kits[kit.ID] = kit
	}
	modules := map[string]string{}
	for _, module := range catalog.Modules {
		modules[module.ID] = module.GeometryID
	}
	instances := map[string]Instance{}
	for _, instance := range layout.Instances {
		instances[instance.ID] = instance
	}
	if len(layout.Gates) > 64 {
		return nil, issue("gate_budget", "layout.gates", "a layout supports at most 64 gates")
	}
	out := make([]RuntimeGate, 0, len(layout.Gates))
	seenIDs, seenInstances, seenCells := map[string]bool{}, map[string]bool{}, map[Cell]string{}
	for i, placement := range layout.Gates {
		path := fmt.Sprintf("layout.gates[%d]", i)
		if !stableID.MatchString(placement.ID) || seenIDs[placement.ID] || strings.TrimSpace(placement.Reason) == "" {
			return nil, issue("gate_identity", path, "each gate needs a unique stable ID and an authored reason", placement.ID)
		}
		seenIDs[placement.ID] = true
		kit, ok := kits[placement.KitID]
		if !ok {
			return nil, issue("gate_unknown_kit", path+".kit_id", "gate references an unknown kit", placement.KitID)
		}
		instance, ok := instances[placement.InstanceID]
		if !ok || seenInstances[placement.InstanceID] {
			return nil, issue("gate_instance", path+".instance_id", "gate needs its own existing instance", placement.InstanceID)
		}
		seenInstances[placement.InstanceID] = true
		if modules[instance.ModuleID] != kit.GeometryID {
			return nil, issue("gate_geometry_mismatch", path+".instance_id", "the instance's module geometry differs from the kit geometry", placement.InstanceID, kit.GeometryID)
		}
		s := placement.Schedule
		if s.OpenSecond < 0 || s.OpenSecond >= 86400 || s.CloseSecond < 0 || s.CloseSecond >= 86400 || s.OpenSecond == s.CloseSecond {
			return nil, issue("gate_schedule", path+".schedule", "open and close are distinct seconds of the day")
		}
		var opening []Cell
		for _, port := range compiled.Ports {
			if port.InstanceID == placement.InstanceID && port.ID == kit.PortID {
				opening = append([]Cell{}, port.Opening...)
			}
		}
		if len(opening) == 0 || len(opening) > 256 {
			return nil, issue("gate_opening", path, "the kit port has no compiled opening on this instance", placement.InstanceID, kit.PortID)
		}
		for _, at := range opening {
			if previous, taken := seenCells[at]; taken {
				return nil, issue("gate_overlap", path, fmt.Sprintf("gate openings overlap at (%d,%d)", at.X, at.Y), previous, placement.ID)
			}
			if at.Y < 0 || at.Y >= len(compiled.Terrain) || at.X < 0 || at.X >= len(compiled.Terrain[at.Y]) || compiled.Terrain[at.Y][at.X] != '.' {
				return nil, issue("gate_opening", path, fmt.Sprintf("gate opening (%d,%d) is not clear floor in the compiled terrain", at.X, at.Y), placement.ID)
			}
			seenCells[at] = placement.ID
		}
		t := kit.TimingMS
		gravityMS := (int64(kit.ClearHeightMM)*1000 + int64(kit.GravityMMPerS) - 1) / int64(kit.GravityMMPerS)
		out = append(out, RuntimeGate{
			ID: placement.ID, InstanceID: placement.InstanceID, KitID: kit.ID, Kind: kit.Kind, Opening: opening,
			ClearHeightMM: kit.ClearHeightMM, SightClearPPM: int64(kit.SightClearMM) * 1000000 / int64(kit.ClearHeightMM),
			WarningTicks: gateTicks(t.Warning, tickHz), TravelTicks: gateTicks(t.Travel, tickHz), RetryTicks: gateTicks(t.Retry, tickHz),
			LockdownWarningTicks: gateTicks(t.LockdownWarning, tickHz), LockdownRetryTicks: gateTicks(t.LockdownRetry, tickHz), PowerWarningTicks: gateTicks(t.PowerWarning, tickHz),
			GravityTicks: gateTicks(gravityMS, tickHz), RampTicks: min(gateTicks(t.Ramp, tickHz), gateTicks(t.Travel, tickHz)/2), OpenSecond: s.OpenSecond, CloseSecond: s.CloseSecond, LockAtNight: s.LockAtNight,
			Leaves: append([]GateLeaf{}, kit.Leaves...), Cables: append([]GateCable(nil), kit.Cables...), Signals: append([]GateSignal{}, kit.Signals...),
		})
	}
	return out, nil
}

// residentsOutsideGates keeps stand people out of gate openings: a routine
// target inside a gate would hold the gate open forever (doc 85). Residents are
// validated on the navigation terrain by compileResidents (doc 80).
func residentsOutsideGates(residents []CompiledResident, gates []RuntimeGate) *ValidationError {
	gateCells := map[Cell]bool{}
	for _, gate := range gates {
		for _, at := range gate.Opening {
			gateCells[at] = true
		}
	}
	for i, resident := range residents {
		for j, step := range resident.Steps {
			if gateCells[step.Nav] {
				return issue("resident_step_cell", fmt.Sprintf("layout.residents[%d].steps[%d].at", i, j), fmt.Sprintf("resident target (%d,%d) is inside a gate opening", step.At.X, step.At.Y), resident.ID)
			}
		}
	}
	return nil
}
