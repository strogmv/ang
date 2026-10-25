package gamepoc

import (
	"fmt"
	"math"
	"sort"
)

// The renderer uses the authored points without lane offsets or free curves.
// This capsule covers its .055-cell half-width and bounded joins conservatively.
const wayfindingGuideRadius = .10

func wayfindingGuideSupport(d WayfindingDestination, rows []string, walks func(WayfindingCell) bool) ([]WayfindingCell, error) {
	if d.GuideFrom == nil || len(d.GuideRoute) < 2 || len(d.GuideRoute) > 64 {
		return nil, fmt.Errorf("requires guide_from and 2..64 points")
	}
	first, last := d.GuideRoute[0], d.GuideRoute[len(d.GuideRoute)-1]
	if first != (WayfindingPoint{X: float64(d.GuideFrom.X), Y: float64(d.GuideFrom.Y)}) || last != (WayfindingPoint{X: float64(d.At.X), Y: float64(d.At.Y)}) {
		return nil, fmt.Errorf("endpoints must equal guide_from and at")
	}
	for i, p := range d.GuideRoute {
		if math.IsNaN(p.X) || math.IsNaN(p.Y) || math.IsInf(p.X, 0) || math.IsInf(p.Y, 0) || p.Y < 0 || p.Y > float64(len(rows)-1) || p.X < 0 || p.X > float64(len(rows[int(p.Y)])-1) {
			return nil, fmt.Errorf("point %d is not finite inside the floor grid", i)
		}
		if i > 0 && p == d.GuideRoute[i-1] {
			return nil, fmt.Errorf("point %d duplicates the preceding point", i)
		}
	}
	seen := map[WayfindingCell]bool{}
	for i := 1; i < len(d.GuideRoute); i++ {
		a, b := d.GuideRoute[i-1], d.GuideRoute[i]
		// Traverse only cells crossed by the centreline; a one-cell neighbourhood
		// contains every square touched by the radius < .5 capsule. Exact distance
		// tests below avoid both a huge segment bounding box and diagonal holes.
		at := WayfindingCell{X: int(math.Floor(a.X + .5)), Y: int(math.Floor(a.Y + .5))}
		end := WayfindingCell{X: int(math.Floor(b.X + .5)), Y: int(math.Floor(b.Y + .5))}
		dx, dy := b.X-a.X, b.Y-a.Y
		stepX, stepY, tx, ty, deltaX, deltaY := 0, 0, math.Inf(1), math.Inf(1), math.Inf(1), math.Inf(1)
		if dx != 0 {
			stepX = 1
			if dx < 0 {
				stepX = -1
			}
			tx = (float64(at.X) + .5*float64(stepX) - a.X) / dx
			deltaX = 1 / math.Abs(dx)
		}
		if dy != 0 {
			stepY = 1
			if dy < 0 {
				stepY = -1
			}
			ty = (float64(at.Y) + .5*float64(stepY) - a.Y) / dy
			deltaY = 1 / math.Abs(dy)
		}
		for {
			for y := at.Y - 1; y <= at.Y+1; y++ {
				for x := at.X - 1; x <= at.X+1; x++ {
					p := WayfindingCell{X: x, Y: y}
					if seen[p] || wayfindingSegmentSquareDistanceSquared(a, b, p) > wayfindingGuideRadius*wayfindingGuideRadius+1e-12 {
						continue
					}
					if !walks(p) {
						return nil, fmt.Errorf("segment %d buffered paint crosses blocked cell (%d,%d)", i-1, x, y)
					}
					seen[p] = true
					if len(seen) > 16384 {
						return nil, fmt.Errorf("buffered support exceeds 16384 cells")
					}
				}
			}
			if at == end {
				break
			}
			oldX, oldY := tx, ty
			if oldX <= oldY {
				at.X += stepX
				tx += deltaX
			}
			if oldY <= oldX {
				at.Y += stepY
				ty += deltaY
			}
		}
	}
	out := make([]WayfindingCell, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Y == out[j].Y {
			return out[i].X < out[j].X
		}
		return out[i].Y < out[j].Y
	})
	return out, nil
}

func wayfindingSegmentSquareDistanceSquared(a, b WayfindingPoint, cell WayfindingCell) float64 {
	left, right, top, bottom := float64(cell.X)-.5, float64(cell.X)+.5, float64(cell.Y)-.5, float64(cell.Y)+.5
	lo, hi := 0., 1.
	for _, axis := range [][4]float64{{a.X, b.X - a.X, left, right}, {a.Y, b.Y - a.Y, top, bottom}} {
		if axis[1] == 0 {
			if axis[0] < axis[2] || axis[0] > axis[3] {
				lo = 2
				break
			}
			continue
		}
		u, v := (axis[2]-axis[0])/axis[1], (axis[3]-axis[0])/axis[1]
		if u > v {
			u, v = v, u
		}
		lo = math.Max(lo, u)
		hi = math.Min(hi, v)
	}
	if lo <= hi {
		return 0
	}
	pointRect := func(p WayfindingPoint) float64 {
		dx := math.Max(math.Max(left-p.X, 0), p.X-right)
		dy := math.Max(math.Max(top-p.Y, 0), p.Y-bottom)
		return dx*dx + dy*dy
	}
	best := math.Min(pointRect(a), pointRect(b))
	dx, dy := b.X-a.X, b.Y-a.Y
	for _, p := range []WayfindingPoint{{X: left, Y: top}, {X: right, Y: top}, {X: left, Y: bottom}, {X: right, Y: bottom}} {
		t := math.Max(0, math.Min(1, ((p.X-a.X)*dx+(p.Y-a.Y)*dy)/(dx*dx+dy*dy)))
		x, y := p.X-a.X-t*dx, p.Y-a.Y-t*dy
		best = math.Min(best, x*x+y*y)
	}
	return best
}
