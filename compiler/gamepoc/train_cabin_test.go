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
		Seats:                  []TrainCabinSpot{{XM: 0, YM: .93}, {XM: .6, YM: .93, Face: "ahead"}, {XM: 0, YM: -.93}},
		Standing:               []TrainCabinSpot{{XM: 3.1}, {XM: -3.1}},
		BackgroundPerCarByHour: hours, SpreadPermille: 250,
		Look: TrainCabinLook{
			GlassMaterials: []string{"MAT_glass"}, StripMaterials: []string{"MAT_lamp"}, InteriorMaterials: []string{"MAT_liner"},
			StripColor: "#E4EDFF", StripEnergy: 4, InteriorEmission: .25, LightColor: "#E4EDFF", LightEnergy: 1.6,
			LightRangeM: 5, LightAttenuation: 1.2, LightHeightM: 1.95, GlassTint: "#B9C8CC", GlassAlpha: .22,
			GlassSpecular: .12, GlassRoughness: .18, Looks: []TrainCabinCrowdLook{{"copper", 34}, {"bronze", 26}},
			Recolors: []TrainCabinRecolor{{"MAT_liner", "#4C5356"}},
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
		"no seats":                func(k *TrainCabin) { k.Seats = nil },
		"seat outside the body":   func(k *TrainCabin) { k.Seats[0].YM = 1.35 },
		"seat beyond the end":     func(k *TrainCabin) { k.Seats[0].XM = 6.7 },
		"overlapping places":      func(k *TrainCabin) { k.Standing[0] = TrainCabinSpot{XM: .1, YM: .9} },
		"seat faces nowhere":      func(k *TrainCabin) { k.Seats[0].Face = "up" },
		"non-finite place":        func(k *TrainCabin) { k.Seats[1].XM = math.NaN() },
		"short day":               func(k *TrainCabin) { k.BackgroundPerCarByHour = k.BackgroundPerCarByHour[:23] },
		"more than the car":       func(k *TrainCabin) { k.BackgroundPerCarByHour[3] = 6 },
		"negative passengers":     func(k *TrainCabin) { k.BackgroundPerCarByHour[3] = -1 },
		"spread beyond the mean":  func(k *TrainCabin) { k.SpreadPermille = 1001 },
		"seat below the floor":    func(k *TrainCabin) { k.SeatM = k.FloorM },
		"floating floor":          func(k *TrainCabin) { k.FloorM = 2 },
		"no glass role":           func(k *TrainCabin) { k.Look.GlassMaterials = nil },
		"empty material name":     func(k *TrainCabin) { k.Look.StripMaterials = []string{" "} },
		"named colour":            func(k *TrainCabin) { k.Look.StripColor = "white" },
		"blinding strip":          func(k *TrainCabin) { k.Look.StripEnergy = 17 },
		"no light reach":          func(k *TrainCabin) { k.Look.LightRangeM = 0 },
		"invisible glass":         func(k *TrainCabin) { k.Look.GlassAlpha = 0 },
		"no crowd look":           func(k *TrainCabin) { k.Look.Looks = nil },
		"unshared crowd look":     func(k *TrainCabin) { k.Look.Looks[0].Share = 0 },
		"unsafe crowd look":       func(k *TrainCabin) { k.Look.Looks[0].Look = "../copper" },
		"recolour of no interior": func(k *TrainCabin) { k.Look.Recolors[0].Material = "MAT_glass" },
		"recolour named colour":   func(k *TrainCabin) { k.Look.Recolors[0].Albedo = "grey" },
		"recolour given twice":    func(k *TrainCabin) { k.Look.Recolors = append(k.Look.Recolors, k.Look.Recolors[0]) },
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

func trainCabinCozyFixture() *TrainCabin {
	k := trainCabinFixture()
	k.Riders = &TrainCabinRiders{
		Looks: []TrainCabinRiderLook{{"bronze", 6, "m", "#A3A591"}, {"bronze_home", 5, "f", "#C9B79B"}},
		Child: TrainCabinChild{Look: "child", AfterLook: "bronze_home", Permille: 200, Pose: "child_window", Tint: "#9C6B4E"},
		Poses: []TrainCabinPose{{ID: "sit", At: "seat", Share: 3}, {ID: "doze", At: "seat", Share: 1, HeadDeg: [3]float64{26, 0, 18}, Toward: "window"},
			{ID: "stand", At: "standing", Share: 1}, {ID: "child_window", At: "seat"}},
		Home: TrainCabinHome{Levels: []string{"level.poc_housing"}},
	}
	d := &TrainCabinDressing{Kit: "res://assets/train/kit.glb"}
	d.Shade.Material, d.Shade.Emission, d.Shade.Energy = "cozy.MAT_shade", "#FFA552", 1.6
	d.Curtains.Material, d.Curtains.Palette, d.Curtains.Backlight, d.Curtains.Glow = "cozy.MAT_curtain", []string{"#8E4630"}, "#FF9C55", .55
	w := &d.WindowLight
	w.Color, w.Energy, w.RangeM, w.AngleDeg, w.HeightM, w.DownDeg, w.Attenuation, w.PatternPx, w.PatternSoft = "#FFB36B", 2, 7, 60, 1.5, 30, 1.4, [2]int{512, 256}, .35
	g := &d.Glass
	g.Tint, g.Alpha, g.Fog, g.FogHeightM, g.FogTint, g.Drops, g.Sheen, g.Rim = "#6E7E80", .16, .4, .5, "#E9D6BC", .3, .2, .3
	d.Haze.Color, d.Haze.Alpha, d.Haze.WidthM, d.Haze.HeightM, d.Haze.RiseMPS = "#FFD3A6", .2, 1, 1.5, .1
	d.HideMaterials = []string{"MAT_train_seat_fabric"}
	d.Lamps = &TrainCabinLamps{At: [][3]float64{{1.15, -1.03, 1.03}}}
	d.Lamps.Bulb.Material, d.Lamps.Bulb.Emission, d.Lamps.Bulb.Energy = "cozy.MAT_lamp_bulb", "#FFD7A0", 4
	d.Lamps.Light.Color, d.Lamps.Light.Energy, d.Lamps.Light.RangeM, d.Lamps.Light.Attenuation = "#FFB36B", .8, 1.3, 1.4
	k.Dressing = d
	p := &TrainCabinPeople{RealWithinM: 16, RealBelowMPS: 2.5, AnimateEvery: 3}
	p.Pool = append(p.Pool, struct {
		Look  string `json:"look"`
		Count int    `json:"count"`
	}{"bronze", 4})
	p.Clips.Seated, p.Clips.Standing = "Sitting", "Idle"
	p.Cards.Atlas, p.Cards.Mask, p.Cards.Grid = "res://assets/train/cards.png", "res://assets/train/cards-mask.png", [2]int{4, 4}
	cell := func(pose, body, view string, x, y int) {
		p.Cards.Cells = append(p.Cards.Cells, struct {
			Pose string `json:"pose"`
			Body string `json:"body"`
			View string `json:"view"`
			Cell [2]int `json:"cell"`
		}{pose, body, view, [2]int{x, y}})
	}
	i := 0
	for _, pose := range []string{"sit", "doze", "stand"} {
		for _, body := range []string{"m", "f"} {
			for _, view := range []string{"front", "back"} {
				cell(pose, body, view, i%4, i/4)
				i++
			}
		}
	}
	cell("child_window", "child", "front", i%4, i/4)
	cell("child_window", "child", "back", (i+1)%4, (i+1)/4)
	p.Cards.SizeM, p.Cards.SeatDropM, p.Cards.Breath.Amp, p.Cards.Breath.Hz, p.Cards.TintKeep, p.Cards.BackLight = [2]float64{1, 2}, .5, .008, .25, .5, .5
	k.People = p
	k.Paint = &TrainCabinPaint{SidePos: "res://a.png", SideNeg: "res://b.png", Roof: "res://c.png", HalfLengthM: 6.85, HalfWidthM: 1.42, Z0M: -.6, Z1M: 3, Materials: []string{"MAT_train_paint"}, Strength: 1, Tint: "#9E9886"}
	return k
}

// Cozy v1: the riders' lists the server indexes and the client's dressing, people and paint are checked before a build.
func TestTrainCabinCozyRejectsBrokenRidersDressingAndCards(t *testing.T) {
	life := json.RawMessage(`{"locations":{"a":{},"b":{},"c":{}}}`)
	valid := trainMotionFixture()
	valid.Cabin = trainCabinCozyFixture()
	if err := valid.validate(life); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*TrainCabin){
		"no rider looks":             func(k *TrainCabin) { k.Riders.Looks = nil },
		"rider look twice":           func(k *TrainCabin) { k.Riders.Looks[1].Look = "bronze" },
		"rider without card body":    func(k *TrainCabin) { k.Riders.Looks[0].Body = "x" },
		"child of nobody":            func(k *TrainCabin) { k.Riders.Child.AfterLook = "copper" },
		"child is a rider look":      func(k *TrainCabin) { k.Riders.Child.Look = "bronze" },
		"child stands":               func(k *TrainCabin) { k.Riders.Child.Pose = "stand" },
		"no standing pose":           func(k *TrainCabin) { k.Riders.Poses[2].Share = 0 },
		"pose twice":                 func(k *TrainCabin) { k.Riders.Poses[1].ID = "sit" },
		"head turned around":         func(k *TrainCabin) { k.Riders.Poses[1].HeadDeg[1] = 90 },
		"unknown prop":               func(k *TrainCabin) { k.Riders.Poses[1].Prop = "bottle" },
		"leans at nothing":           func(k *TrainCabin) { k.Riders.Poses[1].Toward = "door" },
		"arms of nothing":            func(k *TrainCabin) { k.Riders.Poses[1].Arms = "wave" },
		"card of no pose":            func(k *TrainCabin) { k.Riders.Poses[1].CardAs = "dance" },
		"kit not a resource":         func(k *TrainCabin) { k.Dressing.Kit = "/tmp/kit.glb" },
		"no curtain colour":          func(k *TrainCabin) { k.Dressing.Curtains.Palette = nil },
		"window light straight up":   func(k *TrainCabin) { k.Dressing.WindowLight.DownDeg = 95 },
		"opaque fog beyond glass":    func(k *TrainCabin) { k.Dressing.Glass.FogHeightM = 2 },
		"named haze colour":          func(k *TrainCabin) { k.Dressing.Haze.Color = "warm" },
		"hidden material unnamed":    func(k *TrainCabin) { k.Dressing.HideMaterials = []string{" "} },
		"lamp outside the car":       func(k *TrainCabin) { k.Dressing.Lamps.At[0][1] = 3 },
		"lamp without a bulb":        func(k *TrainCabin) { k.Dressing.Lamps.Bulb.Material = "" },
		"lamp floodlight":            func(k *TrainCabin) { k.Dressing.Lamps.Light.RangeM = 9 },
		"brighter by day":            func(k *TrainCabin) { k.Dressing.DayKeep = 1.5 },
		"paint glows at night":       func(k *TrainCabin) { k.Paint.NightFloor = 0.9 },
		"kit blazes":                 func(k *TrainCabin) { g := 5.0; k.Dressing.KitGlow = &g },
		"pool of no rider":           func(k *TrainCabin) { k.People.Pool[0].Look = "copper" },
		"pool too big":               func(k *TrainCabin) { k.People.Pool[0].Count = 25 },
		"card outside the grid":      func(k *TrainCabin) { k.People.Cards.Cells[0].Cell = [2]int{4, 0} },
		"card cell twice":            func(k *TrainCabin) { k.People.Cards.Cells[1].Cell = k.People.Cards.Cells[0].Cell },
		"pose without its back card": func(k *TrainCabin) { k.People.Cards.Cells = k.People.Cards.Cells[:len(k.People.Cards.Cells)-1] },
		"people without riders":      func(k *TrainCabin) { k.Riders = nil },
		"wild breathing":             func(k *TrainCabin) { k.People.Cards.Breath.Amp = .5 },
		"paint below the beam":       func(k *TrainCabin) { k.Paint.Z0M = -4 },
		"paint of no material":       func(k *TrainCabin) { k.Paint.Materials = nil },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			c := trainMotionFixture()
			c.Cabin = trainCabinCozyFixture()
			mutate(c.Cabin)
			if err := c.validate(life); err == nil {
				t.Fatal("invalid cozy cabin accepted")
			}
		})
	}
}
