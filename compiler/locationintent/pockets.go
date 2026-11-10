package locationintent

import (
	"fmt"

	"github.com/strogmv/ang/compiler/locationsdk"
)

// Pocket is a region between the rooms' outer walls that is neither floor nor
// solid: from the game camera it reads as a dark pit, a room without a floor.
type Pocket struct {
	Cells  int              `json:"cells"`
	Bounds locationsdk.Rect `json:"bounds"`
	First  locationsdk.Cell `json:"first"`
	set    map[cell]bool
}

// HullPockets finds every region inside the hull (the bounding rectangle of the
// walkable area) that is not covered by floor, solid mass or a declared void.
// The outer walls of the rooms flank such a region on at least two sides, so
// from the 35-degree game camera it is a walled pit with no floor, whether or
// not it is closed on every side. Space beyond the hull is real outside.
func HullPockets(hull locationsdk.Rect, covered func(locationsdk.Cell) bool) []Pocket {
	inside := func(c cell) bool {
		return c.X >= hull.X && c.Y >= hull.Y && c.X < hull.X+hull.W && c.Y < hull.Y+hull.H && !covered(c)
	}
	seen := map[cell]bool{}
	var pockets []Pocket
	for _, c := range cellsOf(hull) {
		if !inside(c) || seen[c] {
			continue
		}
		pocket := Pocket{First: c, set: map[cell]bool{}}
		minX, minY, maxX, maxY := c.X, c.Y, c.X, c.Y
		blob := []cell{c}
		seen[c] = true
		for len(blob) > 0 {
			p := blob[0]
			blob = blob[1:]
			pocket.set[p] = true
			pocket.Cells++
			minX, minY, maxX, maxY = min(minX, p.X), min(minY, p.Y), max(maxX, p.X), max(maxY, p.Y)
			for _, side := range sideOrder {
				n := cell{X: p.X + sideStep[side].X, Y: p.Y + sideStep[side].Y}
				if inside(n) && !seen[n] {
					seen[n] = true
					blob = append(blob, n)
				}
			}
		}
		pocket.Bounds = locationsdk.Rect{X: minX, Y: minY, W: maxX - minX + 1, H: maxY - minY + 1}
		pockets = append(pockets, pocket)
	}
	return pockets
}

// LayoutPockets applies the rule to any SDK layout, hand-written ones included:
// floor rectangles are floor, movement-blocking obstacles from the floor up are
// solid. The result is what a location must cap or declare.
func LayoutPockets(layout locationsdk.Layout) []Diagnostic {
	floor, solid := map[cell]bool{}, map[cell]bool{}
	for _, f := range layout.Floor {
		for _, c := range cellsOf(f.Rect) {
			floor[c] = true
		}
	}
	for _, o := range layout.Obstacles {
		if o.BlocksMovement && o.BottomMM == 0 {
			for _, c := range cellsOf(o.Rect) {
				solid[c] = true
			}
		}
	}
	voids := map[cell]bool{}
	if layout.Interior != nil {
		for _, v := range layout.Interior.Voids {
			for _, c := range cellsOf(v.Rect) {
				voids[c] = true
			}
		}
	}
	// Only a declared dome interior has walls all round; a station at a canyon
	// or an open yard legitimately has nothing beside its floor.
	if len(floor) == 0 || layout.Interior == nil {
		return nil
	}
	minX, minY, maxX, maxY := layout.Width, layout.Depth, 0, 0
	for c := range floor {
		minX, minY, maxX, maxY = min(minX, c.X), min(minY, c.Y), max(maxX, c.X+1), max(maxY, c.Y+1)
	}
	var out []Diagnostic
	for _, p := range HullPockets(locationsdk.Rect{X: minX, Y: minY, W: maxX - minX, H: maxY - minY}, func(c cell) bool { return floor[c] || solid[c] || voids[c] }) {
		at := p.First
		out = append(out, Diagnostic{Code: "enclosed_pocket", ID: layout.ID, Cell: &at,
			Message: fmt.Sprintf("%d cells inside the hull of the rooms have neither floor nor mass (X %.1f–%.1f m, Y %.1f–%.1f m): the camera sees a dark pit", p.Cells, float64(p.Bounds.X)*unitM, float64(p.Bounds.X+p.Bounds.W)*unitM, float64(p.Bounds.Y)*unitM, float64(p.Bounds.Y+p.Bounds.H)*unitM),
			Hint:    "cap it: add a wall-height obstacle over the pocket (solid mass), or give it floor, or declare it as a track/shaft void"})
	}
	return out
}
