package locationsdk

import "testing"

// demo-hub v4-content: a vista may leave its tray, pipes and lamp bodies to a registry prefab of the layout's networks
// ("kit"); "procedural" (default) keeps its own boxes; nothing else is accepted.
func TestVistaNetworksProceduralOrKit(t *testing.T) {
	base := Layout{ID: "v", Width: 20, Depth: 10}
	v := Vista{ID: "corridor", At: Cell{4, 0}, Direction: North, Width: 6, LengthMM: 12000, HeightMM: 7500, End: "turn_left", Reason: "beyond the gate"}
	for _, n := range []string{"", "procedural", "kit"} {
		l := base
		w := v
		w.Networks = n
		l.Vistas = []Vista{w}
		if err := validateVistas(l); err != nil {
			t.Fatalf("networks %q rejected: %v", n, err)
		}
	}
	l := base
	w := v
	w.Networks = "pipes"
	l.Vistas = []Vista{w}
	if validateVistas(l) == nil {
		t.Fatal("unknown vista networks accepted")
	}
}
