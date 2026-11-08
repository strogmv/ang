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
	wc := SanitaryCatalog{CommonEntryCell: Point{X: 100, Y: 100}, Units: []SanitaryUnit{{ID: "old", Kind: "wc", LevelID: "primary", Provider: "policy.wc"}}}
	if err := appendSDKSanitary(world, c, &wc); err != nil {
		t.Fatal(err)
	}
	if len(wc.Units) != 1 || wc.Units[0].LevelID != "primary" || wc.CommonEntryCell != (Point{X: 100, Y: 100}) || len(wc.Levels["room"]) != 1 || wc.Levels["room"][0].Provider != "policy.wc" {
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
