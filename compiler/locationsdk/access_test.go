package locationsdk

import "testing"

func TestPortApproachRejectsDisconnectedAndDiagonalIslands(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cells []Cell
		code  string
	}{
		{"remote island", []Cell{{0, 1}, {0, 4}}, "port_approach_disconnected"},
		{"diagonal island", []Cell{{0, 1}, {1, 2}}, "port_approach_disconnected"},
		{"entirely remote", []Cell{{0, 3}, {0, 4}}, "port_approach_too_far"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, l := testCatalog(), testLayouts()[0]
			c.Geometries[0].Ports[0].Approach = tc.cells
			_, err := Build(c, l)
			requireCode(t, err, tc.code)
		})
	}
}

func TestOpeningMayJoinSeparateFrontageStrips(t *testing.T) {
	c, l := testCatalog(), testLayouts()[0]
	// Two strips need not touch each other: the continuous opening joins them.
	c.Geometries[2].Ports[0].Approach = []Cell{{0, 1}, {1, 1}, {0, 2}, {1, 2}}
	l.Instances = []Instance{{ID: "passage", ModuleID: "module.shape.opening", At: Cell{4, 2}, Reason: "shared approach through a wide opening"}}
	l.ReservedPaths = nil
	requireBuilt(t, c, l)
	// A genuinely separate strip is connected by the opening itself.
	c.Geometries[2].Width = 3
	c.Geometries[2].Ports[0].Opening = []Cell{{0, 0}, {1, 0}, {2, 0}}
	c.Geometries[2].Ports[0].Approach = []Cell{{0, 1}, {2, 1}}
	requireBuilt(t, c, l)
}

func TestConnectionOpeningRejectsBarrierAfterEveryRotation(t *testing.T) {
	for _, tc := range []struct {
		rotation     int
		origin, edge Cell
		side         Side
	}{
		{0, Cell{3, 9}, Cell{4, 9}, South},
		{90, Cell{0, 3}, Cell{0, 4}, West},
		{180, Cell{3, 0}, Cell{3, 0}, North},
		{270, Cell{13, 3}, Cell{13, 3}, East},
	} {
		c, l := testCatalog(), testLayouts()[0]
		c.Geometries[2].Ports[0].Kind = "connection"
		l.ReservedPaths = nil
		l.Instances = []Instance{{ID: "bridge.connection", ModuleID: "module.shape.opening", At: tc.origin, Rotation: tc.rotation, Reason: "authored connection, not a door"}}
		l.Drops = []DropAnnotation{{ID: "ledge", At: tc.edge, Side: tc.side, LowerElevation: -4, Barrier: true, Reason: "invalid fence across connection"}}
		_, err := Build(c, l)
		requireCode(t, err, "barrier_across_connection")
		l.Drops[0].Barrier = false
		requireBuilt(t, c, l)
	}
}

func TestEntryCannotSpawnInsideNonblockingPhysicalMaterial(t *testing.T) {
	c, l := testCatalog(), testLayouts()[0]
	c.Geometries[0].Blocked = nil
	l.Entries = []Cell{l.Instances[0].At}
	_, err := Build(c, l)
	requireCode(t, err, "entry_obstructed")
	l.Entries = []Cell{{0, 5}}
	out := requireBuilt(t, c, l)
	if out.Terrain[2][3] != '.' {
		t.Fatal("entry safety must not silently change the authored movement footprint")
	}
}

func TestPortApproachBudgetRejectsBeforeExpansion(t *testing.T) {
	c := testCatalog()
	c.Geometries[0].Ports[0].Approach = make([]Cell, 65537)
	_, err := Build(c, testLayouts()[0])
	requireCode(t, err, "port_approach_budget")
}
