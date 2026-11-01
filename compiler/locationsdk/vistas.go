package locationsdk

import (
	"fmt"
	"strings"
)

// Vista is a corridor continuing beyond the plan edge from a mouth of Width cells
// starting at At (on that edge), going Direction away from the layout. It has no
// cells: presentation only, seen through the openings of the plan (demo-hub).
type Vista struct {
	ID        string `json:"id"`
	At        Cell   `json:"at"`
	Direction Side   `json:"direction"`
	Width     int    `json:"width"`
	// Inset: plan cells of the mouth (inside the plan, without floor) the vista draws too.
	Inset    int    `json:"inset,omitempty"`
	LengthMM int    `json:"length_mm"`
	HeightMM int    `json:"height_mm"`
	End      string `json:"end"`
	SignKey  string `json:"sign_key,omitempty"`
	// Networks: "procedural" (default) draws the vista's own tray, cable, pipes and lamp bodies; "kit" leaves them to a
	// registry prefab of engineering networks (demo-hub v4-content: pipe-kit batch) and keeps only the vista's lights.
	Networks string `json:"networks,omitempty"`
	Reason   string `json:"reason"`
}

var vistaEnds = map[string]bool{"turn_left": true, "turn_right": true, "closed_gate": true, "cart_barrier": true}

// validateVistas: unique IDs, a mouth on the plan edge it leaves through, bounded sizes, a known end and ui.* signs.
func validateVistas(layout Layout) *ValidationError {
	if len(layout.Vistas) > 32 {
		return issue("vista_budget", "layout.vistas", "a layout supports at most 32 vistas")
	}
	seen := map[string]bool{}
	for i, v := range layout.Vistas {
		path := fmt.Sprintf("layout.vistas[%d]", i)
		if !stableID.MatchString(v.ID) || seen[v.ID] || strings.TrimSpace(v.Reason) == "" {
			return issue("vista_definition", path, "a vista needs a unique stable ID and a reason", v.ID)
		}
		seen[v.ID] = true
		if v.Width < 2 || v.Width > 32 || v.Inset < 0 || v.Inset > 8 || v.LengthMM < 2000 || v.LengthMM > 40000 || v.HeightMM < 2100 || v.HeightMM > 20000 || !vistaEnds[v.End] || v.SignKey != "" && !strings.HasPrefix(v.SignKey, "ui.") || v.Networks != "" && v.Networks != "procedural" && v.Networks != "kit" {
			return issue("vista_definition", path, "width 2..32 cells, length 2..40 m, height 2.1..20 m, end turn_left|turn_right|closed_gate|cart_barrier, sign_key ui.*, networks procedural|kit", v.ID)
		}
		var onEdge bool
		switch v.Direction {
		case North:
			onEdge = v.At.Y == 0 && v.At.X >= 0 && v.At.X+v.Width <= layout.Width
		case South:
			onEdge = v.At.Y == layout.Depth-1 && v.At.X >= 0 && v.At.X+v.Width <= layout.Width
		case West:
			onEdge = v.At.X == 0 && v.At.Y >= 0 && v.At.Y+v.Width <= layout.Depth
		case East:
			onEdge = v.At.X == layout.Width-1 && v.At.Y >= 0 && v.At.Y+v.Width <= layout.Depth
		}
		if !onEdge {
			return issue("vista_mouth", path, "the vista mouth must lie on the plan edge it leaves through", v.ID)
		}
	}
	return nil
}
