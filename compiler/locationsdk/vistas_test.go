package locationsdk

import "testing"

// demo-hub: corridor vistas are presentation only; their mouth sits on the plan edge they leave through.
func TestVistasValidateMouthSizeEndAndSigns(t *testing.T) {
	base := Layout{ID: "v", Width: 20, Depth: 10}
	good := Vista{ID: "corridor", At: Cell{4, 0}, Direction: North, Width: 6, LengthMM: 12000, HeightMM: 6000, End: "turn_left", SignKey: "ui.sign", Reason: "beyond the gate"}
	ok := base
	ok.Vistas = []Vista{good}
	if err := validateVistas(ok); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*Vista){
		"inside the plan": func(v *Vista) { v.At.Y = 3 },
		"too wide":        func(v *Vista) { v.At.X = 18 },
		"end":             func(v *Vista) { v.End = "wall" },
		"sign":            func(v *Vista) { v.SignKey = "sign.hub" },
		"length":          func(v *Vista) { v.LengthMM = 50000 },
		"reason":          func(v *Vista) { v.Reason = " " },
	} {
		bad := base
		v := good
		change(&v)
		bad.Vistas = []Vista{v}
		if validateVistas(bad) == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}

// demo-hub: a low part (a shop counter) of an opaque module lets sight pass at standing height; only blocked cells qualify.
func TestSightClearCellsMarkLowPartsOfOpaqueModules(t *testing.T) {
	c := testCatalog()
	c.Geometries[0].SightClearCells = []Cell{{1, 0}}
	level, err := Build(c, testLayouts()[0])
	if err != nil {
		t.Fatal(err)
	}
	if level.Terrain[2][3] != '#' || level.Terrain[2][4] != '+' {
		t.Fatalf("low counter cell is not sight-clear: %q", level.Terrain[2])
	}
	c.Geometries[0].SightClearCells = []Cell{{0, 1}}
	if _, err := Build(c, testLayouts()[0]); err == nil {
		t.Fatal("sight-clear cell outside the blocked material accepted")
	}
}

// demo-hub: the near-cut rule cuts the camera-side walls that would hide a standing body behind them, not the back walls.
func TestNearCutRuleCutsWallsHidingWalkableCells(t *testing.T) {
	l := testMultideck()
	l.Obstacles = append(l.Obstacles, Obstacle{ID: "wall.near", Surface: "floor", Rect: Rect{8, 20, 6, 1}, TopMM: 8000, BlocksMovement: true, BlocksSight: true, Style: "wall", Reason: "camera-side wall"})
	l.Camera = &LayoutCamera{Projection: "orthographic", ElevationDegrees: 35, AzimuthDegrees: 37, NearCut: &LayoutNearCut{HeightMM: 1200}}
	level, err := Build(multideckCatalog(), l)
	if err != nil {
		t.Fatal(err)
	}
	cut := map[string]int{}
	for _, s := range level.NearCut.Segments {
		cut[s.Obstacle] = len(s.Cells)
	}
	if cut["wall.near"] == 0 || cut["wall.north"] != 0 {
		t.Fatalf("near cut segments %v: the camera-side wall must be cut, the back wall kept", cut)
	}
}
