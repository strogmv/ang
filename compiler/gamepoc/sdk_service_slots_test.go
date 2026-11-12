package gamepoc

import (
	"cuelang.org/go/cue/cuecontext"
	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
	"testing"
)

func TestSDKServiceSlotUsesRealDoorAndMetricFloor(t *testing.T) {
	c, s := sdkServiceDoorFixture()
	cell, entry, inside, err := sdkServicePoses(c, s)
	if err != nil {
		t.Fatal(err)
	}
	if cell != (Point{X: 3, Y: 4}) || entry.Y != 18 || entry.X != 1.75 || entry.Z != 2.25 || inside.Y != 18 || inside.YawDegrees != 0 {
		t.Fatal(cell, entry, inside)
	}
	bad := s
	bad.PortID = "unseen"
	if _, _, _, err = sdkServicePoses(c, bad); err == nil {
		t.Fatal("invented door accepted")
	}
	bad = s
	bad.InsideLocalM[2] = 100
	if _, _, _, err = sdkServicePoses(c, bad); err == nil {
		t.Fatal("inside pose outside instance accepted")
	}
	c.LocationSDK.Levels[0].Terrain[4] = "######"
	if _, _, _, err = sdkServicePoses(c, s); err == nil {
		t.Fatal("blocked approach accepted")
	}
}

func sdkServiceDoorFixture() (Compiled, SDKServiceSlot) {
	l := locationsdk.RuntimeLevel{ID: "kit", UnitMM: 500, Elevation: 36, Terrain: []string{"......", "......", "......", "......", "......", "......"}, Instances: []locationsdk.CompiledInstance{{ID: "cab", At: locationsdk.Cell{X: 1, Y: 1}, Width: 4, Depth: 2, Rotation: 90}}, Ports: []locationsdk.CompiledPort{{ID: "door", InstanceID: "cab", Kind: "door", Side: "south", Opening: []locationsdk.Cell{{X: 3, Y: 2}}, Approach: []locationsdk.Cell{{X: 3, Y: 3}, {X: 3, Y: 4}}}}}
	c := Compiled{ModularLocations: []ModularLocation{{LevelID: "room", LayoutID: "kit"}}, LocationSDK: &locationsdk.RuntimeBundle{Levels: []locationsdk.RuntimeLevel{l}}}
	s := SDKServiceSlot{ID: "wc", LevelID: "room", InstanceID: "cab", PortID: "door", InsideLocalM: placement.Vec3{.6, 0, .75}, InsideYaw: 90}
	return c, s
}

func TestSDKServiceCatalogsKeepIndependentLevelTopology(t *testing.T) {
	c, s := sdkServiceDoorFixture()
	s.Kind = "wc"
	s.SourceUnit = "old"
	s.Reason = "existing authored door"
	world := cuecontext.New().Encode(map[string]any{"sdk_service_slots": []SDKServiceSlot{s}})
	wc := SanitaryCatalog{CommonEntryCell: &Point{X: 100, Y: 100}, QueueLimit: 3, Units: []SanitaryUnit{{ID: "old", Kind: "wc", LevelID: "primary", Provider: "svc.latrine_use", NameKey: "wc", ServiceSeconds: 240, WaterML: 3000, MaxWaitSeconds: 600}}}
	if err := appendSDKSanitary(world, c, &wc); err != nil {
		t.Fatal(err)
	}
	if len(wc.Units) != 1 || wc.Units[0].LevelID != "primary" || *wc.CommonEntryCell != (Point{X: 100, Y: 100}) || len(wc.Levels["room"]) != 1 || wc.Levels["room"][0].Provider != "svc.latrine_use" || wc.Levels["room"][0].PortID != "door" {
		t.Fatal("SDK WC joined or changed primary installation", wc)
	}
	s.Kind = "medical"
	world = cuecontext.New().Encode(map[string]any{"sdk_service_slots": []SDKServiceSlot{s}})
	med := MedicalCapsuleCatalog{Units: []MedicalUnit{{ID: "old", LevelID: "primary", Provider: "policy.medical"}}}
	if err := appendSDKMedical(world, c, &med); err != nil {
		t.Fatal(err)
	}
	if len(med.Units) != 1 || med.Units[0].LevelID != "primary" || len(med.Levels["room"]) != 1 || med.Levels["room"][0].Provider != "policy.medical" {
		t.Fatal("SDK medical joined or changed primary installation", med)
	}
	s.Kind = "wc"
	other := s
	other.ID = "other"
	other.Kind = "medical"
	world = cuecontext.New().Encode(map[string]any{"sdk_service_slots": []SDKServiceSlot{s, other}})
	if _, err := sdkServiceSlots(world, "wc"); err == nil {
		t.Fatal("one physical door accepted for two services")
	}
}

// One SDK door is one private cell: its WC, own basin and shower share the door;
// a repeated kind behind a door, a cell without a WC or two cells on one
// approach are refused.
func TestSDKSanitaryCellsShareOneDoor(t *testing.T) {
	c, s := sdkServiceDoorFixture()
	s.Reason = "existing authored door"
	donor := SanitaryUnit{LevelID: "primary", NameKey: "x", ServiceSeconds: 60, WaterML: 1000, MaxWaitSeconds: 600}
	primary := SanitaryCatalog{CommonEntryCell: &Point{X: 100, Y: 100}, QueueLimit: 3}
	var slots []SDKServiceSlot
	for _, kind := range []string{"wc", "basin", "shower"} {
		u := donor
		u.ID, u.Kind, u.Provider = "old."+kind, kind, map[string]string{"wc": "svc.latrine_use", "basin": "svc.basin_wash", "shower": "svc.shower_3min"}[kind]
		primary.Units = append(primary.Units, u)
		slot := s
		slot.ID, slot.Kind, slot.SourceUnit = "cell."+kind, kind, u.ID
		slots = append(slots, slot)
	}
	world := cuecontext.New().Encode(map[string]any{"sdk_service_slots": slots})
	cat := primary
	if err := appendSDKSanitary(world, c, &cat); err != nil {
		t.Fatal(err)
	}
	if units := cat.Levels["room"]; len(units) != 3 || units[0].PortID != "door" || units[1].EntryCell != units[0].EntryCell || units[2].VisualIndex != units[0].VisualIndex || !units[1].Door {
		t.Fatalf("cell units do not share their door: %+v", units)
	}
	twice := append(append([]SDKServiceSlot(nil), slots...), slots[0])
	twice[3].ID = "cell.wc.again"
	if _, err := sdkServiceSlots(cuecontext.New().Encode(map[string]any{"sdk_service_slots": twice}), "wc"); err == nil {
		t.Fatal("two WCs accepted behind one door")
	}
	units := cat.Levels["room"]
	noWC := []SanitaryUnit{units[1], units[2]}
	if validatePrivateCells(noWC, "room", 3, nil) == nil {
		t.Fatal("cell without a WC accepted")
	}
	other := units[0]
	other.ID, other.PortID, other.VisualIndex = "other.wc", "door.2", 1
	if validatePrivateCells(append(append([]SanitaryUnit(nil), units...), other), "room", 3, nil) == nil {
		t.Fatal("two cells on one approach accepted")
	}
	split := append([]SanitaryUnit(nil), units...)
	split[1].WaitCells = []Point{{X: 3, Y: 5}}
	if validatePrivateCells(split, "room", 3, nil) == nil {
		t.Fatal("units of one cell with different queues accepted")
	}
	if err := validatePrivateCells(units, "room", 3, nil); err != nil {
		t.Fatal(err)
	}
}
