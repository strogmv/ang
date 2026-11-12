package gamepoc

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"
)

func trainCabinFixture() *TrainCabin {
	hours := make([]int, 24)
	for i := range hours {
		hours[i] = 2
	}
	hours[7] = 5
	return &TrainCabin{
		FloorM: -.04, SeatM: .475,
		Seats:                  []TrainCabinSpot{{0, .93}, {.6, .93}, {0, -.93}},
		Standing:               []TrainCabinSpot{{3.1, 0}, {-3.1, 0}},
		BackgroundPerCarByHour: hours, SpreadPermille: 250,
		Look: TrainCabinLook{
			GlassMaterials: []string{"MAT_glass"}, StripMaterials: []string{"MAT_lamp"}, InteriorMaterials: []string{"MAT_liner"},
			StripColor: "#E4EDFF", StripEnergy: 4, InteriorEmission: .25, LightColor: "#E4EDFF", LightEnergy: 1.6,
			LightRangeM: 5, LightAttenuation: 1.2, LightHeightM: 1.95, GlassTint: "#B9C8CC", GlassAlpha: .22,
			GlassSpecular: .12, GlassRoughness: .18, Looks: []TrainCabinCrowdLook{{"copper", 34}, {"bronze", 26}},
		},
	}
}

func TestTrainCabinRejectsCarsThatCannotHoldTheirPeople(t *testing.T) {
	life := json.RawMessage(`{"locations":{"a":{},"b":{},"c":{}}}`)
	valid := trainMotionFixture()
	valid.Cabin = trainCabinFixture()
	if err := valid.validate(life); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*TrainCabin){
		"no seats":               func(k *TrainCabin) { k.Seats = nil },
		"seat outside the body":  func(k *TrainCabin) { k.Seats[0].YM = 1.35 },
		"seat beyond the end":    func(k *TrainCabin) { k.Seats[0].XM = 6.7 },
		"overlapping places":     func(k *TrainCabin) { k.Standing[0] = TrainCabinSpot{.1, .9} },
		"non-finite place":       func(k *TrainCabin) { k.Seats[1].XM = math.NaN() },
		"short day":              func(k *TrainCabin) { k.BackgroundPerCarByHour = k.BackgroundPerCarByHour[:23] },
		"more than the car":      func(k *TrainCabin) { k.BackgroundPerCarByHour[3] = 6 },
		"negative passengers":    func(k *TrainCabin) { k.BackgroundPerCarByHour[3] = -1 },
		"spread beyond the mean": func(k *TrainCabin) { k.SpreadPermille = 1001 },
		"seat below the floor":   func(k *TrainCabin) { k.SeatM = k.FloorM },
		"floating floor":         func(k *TrainCabin) { k.FloorM = 2 },
		"no glass role":          func(k *TrainCabin) { k.Look.GlassMaterials = nil },
		"empty material name":    func(k *TrainCabin) { k.Look.StripMaterials = []string{" "} },
		"named colour":           func(k *TrainCabin) { k.Look.StripColor = "white" },
		"blinding strip":         func(k *TrainCabin) { k.Look.StripEnergy = 17 },
		"no light reach":         func(k *TrainCabin) { k.Look.LightRangeM = 0 },
		"invisible glass":        func(k *TrainCabin) { k.Look.GlassAlpha = 0 },
		"no crowd look":          func(k *TrainCabin) { k.Look.Looks = nil },
		"unshared crowd look":    func(k *TrainCabin) { k.Look.Looks[0].Share = 0 },
		"unsafe crowd look":      func(k *TrainCabin) { k.Look.Looks[0].Look = "../copper" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			c := trainMotionFixture()
			c.Cabin = trainCabinFixture()
			mutate(c.Cabin)
			if err := c.validate(life); err == nil {
				t.Fatal("invalid cabin accepted")
			}
		})
	}
}

func TestTrainCabinRidersSitInTheCarOfTheirDoor(t *testing.T) {
	c := trainMotionFixture()
	c.Cabin = trainCabinFixture()
	c.BerthEdgeID = "a_rail"
	// Berth 152.4 m on the first segment from x=-100: car centres at x 52.4, 38.2, 24.0, 9.8 (cells of 0.5 m).
	edges := func(x int) json.RawMessage {
		return json.RawMessage(fmt.Sprintf(`{"locations":{"a":{"cell_mm":500},"b":{},"c":{}},"edges":[{"id":"a_rail","from":"a","mode":"rail","exit":{"x":%d,"y":49}}]}`, x))
	}
	for x, want := range map[int]string{104: "passenger", 56: "freight2", 20: "freight3", 77: "freight"} {
		k := *c.Cabin
		c.Cabin = &k
		if err := c.validate(edges(x)); err != nil {
			t.Fatal(err)
		}
		if err := c.deriveCabinRiderModule(edges(x)); err != nil || c.Cabin.RiderModule != want {
			t.Fatal("exit cell", x, "gave", c.Cabin.RiderModule, err, "want", want)
		}
	}
	if err := c.deriveCabinRiderModule(edges(150)); err == nil {
		t.Fatal("an exit beside no car of the berthed train was accepted")
	}
	if err := c.deriveCabinRiderModule(json.RawMessage(`{"locations":{"a":{}},"edges":[]}`)); err == nil {
		t.Fatal("a berth level without a cell size was accepted")
	}
	legacy := trainMotionFixture()
	if err := legacy.deriveCabinRiderModule(edges(104)); err != nil || legacy.Cabin != nil {
		t.Fatal("a catalogue without a cabin changed", err)
	}
}
