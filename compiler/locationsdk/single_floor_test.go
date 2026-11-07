package locationsdk

import "testing"

func TestSingleFloorObstaclesWithReservedRoute(t *testing.T) {
	layout := Layout{
		ID: "test.single_floor", UnitMM: UnitMM, Width: 12, Depth: 10,
		Grounded: true, BaseSurfaceID: "floor",
		Floor:   []FloorArea{{ID: "slab", Rect: Rect{0, 0, 12, 10}, Reason: "supported slab"}},
		Entries: []Cell{{1, 7}},
		Obstacles: []Obstacle{
			{ID: "wall", Surface: "floor", Rect: Rect{3, 2, 1, 3}, TopMM: 6000, BlocksMovement: true, BlocksSight: true, Style: "wall", Reason: "continuous wall"},
			{ID: "header", Surface: "floor", Rect: Rect{4, 2, 3, 1}, BottomMM: 2400, TopMM: 6000, BlocksSight: true, Style: "wall", Reason: "wall over open passage"},
		},
		ReservedPaths: []ReservedPath{{ID: "passage", Points: []Cell{{5, 1}, {5, 7}}, Width: 2, Reason: "route below actual header"}},
	}
	out := requireBuilt(t, testCatalog(), layout)
	if out.Navigation == nil || len(out.Navigation.Surfaces) != 1 || len(out.ReservedPaths) != 1 {
		t.Fatalf("expected one surface with vertical obstacles and reserved route: %+v", out)
	}
	if out.Terrain[2][3] != '#' || out.Terrain[2][5] != '.' {
		t.Fatal("wall must block while header keeps its full passage clear")
	}
	if len(out.Navigation.Obstacles) != 2 {
		t.Fatal("vertical obstacles must survive compilation for the renderer")
	}
}
