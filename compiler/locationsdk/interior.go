package locationsdk

import (
	"fmt"
	"strings"
)

// InteriorEnvelope is the floor-side wall envelope. Every unsupported cell
// inside it must belong to an explicitly authored track, shaft or stair.
type InteriorEnvelope struct {
	Envelope   Rect           `json:"envelope"`
	Extensions []Rect         `json:"extensions,omitempty"`
	Voids      []InteriorVoid `json:"voids"`
}
type InteriorVoid struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Rect     Rect   `json:"rect"`
	Reason   string `json:"reason"`
	CoverRef string `json:"cover_ref,omitempty"`
}

func validateInterior(layout Layout, supported []bool) error {
	if layout.Interior == nil {
		if layout.Setting == "dome_interior" {
			return issue("interior_envelope_required", "layout.interior", "dome interior needs its wall envelope and declared voids")
		}
		return nil
	}
	envelope := layout.Interior.Envelope
	validRect := func(r Rect) bool {
		return r.X >= 0 && r.Y >= 0 && r.W > 0 && r.H > 0 && r.X+r.W <= layout.Width && r.Y+r.H <= layout.Depth
	}
	if !validRect(envelope) {
		return issue("interior_bounds", "layout.interior.envelope", "wall envelope must lie within the layout")
	}
	contains := func(r Rect, x, y int) bool { return x >= r.X && y >= r.Y && x < r.X+r.W && y < r.Y+r.H }
	envelopes := append([]Rect{envelope}, layout.Interior.Extensions...)
	for _, r := range envelopes {
		if !validRect(r) {
			return issue("interior_bounds", "layout.interior.extensions", "wall envelope extension must lie within the layout")
		}
	}
	contained := func(r Rect) bool {
		for _, area := range envelopes {
			if contains(area, r.X, r.Y) && contains(area, r.X+r.W-1, r.Y+r.H-1) {
				return true
			}
		}
		return false
	}
	declared := make(map[Cell]string)
	ids := map[string]bool{}
	for i, v := range layout.Interior.Voids {
		path := fmt.Sprintf("layout.interior.voids[%d]", i)
		if !stableID.MatchString(v.ID) || ids[v.ID] || strings.TrimSpace(v.Reason) == "" || !validRect(v.Rect) || !contained(v.Rect) {
			return issue("interior_void", path, "void needs unique ID, reason and rectangle inside the wall envelope")
		}
		if v.Kind != "track" && v.Kind != "shaft" && v.Kind != "stair" {
			return issue("interior_void_kind", path, "void must be a track, shaft or stair", v.ID)
		}
		if v.CoverRef != "" && (v.Kind != "shaft" || !stableID.MatchString(v.CoverRef)) {
			return issue("interior_void_cover", path, "only a shaft can declare an identified dynamic cover", v.ID)
		}
		if v.CoverRef != "" {
			found := false
			for _, instance := range layout.Instances {
				found = found || instance.ID == v.CoverRef
			}
			if !found {
				return issue("interior_void_cover", path, "dynamic cover references no declared instance", v.ID, v.CoverRef)
			}
		}
		ids[v.ID] = true
		for y := v.Rect.Y; y < v.Rect.Y+v.Rect.H; y++ {
			for x := v.Rect.X; x < v.Rect.X+v.Rect.W; x++ {
				if supported[y*layout.Width+x] && v.CoverRef == "" {
					return issue("interior_void_supported", path, fmt.Sprintf("declared void (%d,%d) is floor", x, y), v.ID)
				}
				at := Cell{x, y}
				if declared[at] != "" {
					return issue("interior_void_overlap", path, "void declarations overlap", v.ID, declared[at])
				}
				declared[at] = v.ID
			}
		}
	}
	for _, envelope := range envelopes {
		for y := envelope.Y; y < envelope.Y+envelope.H; y++ {
			for x := envelope.X; x < envelope.X+envelope.W; x++ {
				if !supported[y*layout.Width+x] && declared[Cell{x, y}] == "" {
					return issue("undeclared_interior_drop", "layout.interior.envelope", fmt.Sprintf("no floor inside walls at (%d,%d); declare a track, shaft or stair, or restore the floor", x, y))
				}
			}
		}
	}
	return nil
}
