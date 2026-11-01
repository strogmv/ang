package gamepoc

import (
	"fmt"
	"math"
	"strings"
)

// Storefronts describe architectural presentation. Live open/stocked values
// arrive from authoritative commerce; this geometry never moves items or NPCs.
type PaintedStorefrontRect struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}
type PaintedStorefrontLayer struct {
	Image      string                `json:"image"`
	SourceRect *ArtSceneSourceRect   `json:"source_rect,omitempty"`
	Rect       PaintedStorefrontRect `json:"rect"`
}
type PaintedStorefrontWindow struct {
	StartSecond int64 `json:"start_second"`
	EndSecond   int64 `json:"end_second"`
}
type PaintedStorefront struct {
	ID              string                    `json:"id"`
	MerchantID      string                    `json:"merchant_id"`
	ObservationCell Point                     `json:"observation_cell"`
	Quad            []PaintedPoint            `json:"quad"`
	OpenSecond      int64                     `json:"open_second"`
	CloseSecond     int64                     `json:"close_second"`
	OpeningMS       int                       `json:"opening_ms"`
	ClosingMS       int                       `json:"closing_ms"`
	ClosureMode     string                    `json:"closure_mode"`
	Recess          PaintedStorefrontLayer    `json:"recess"`
	Goods           PaintedStorefrontLayer    `json:"goods"`
	Shutter         PaintedStorefrontLayer    `json:"shutter"`
	Festival        *PaintedStorefrontLayer   `json:"festival,omitempty"`
	HolidayWindows  []PaintedStorefrontWindow `json:"holiday_windows"`
}

func storefrontLayerValid(layer PaintedStorefrontLayer, full bool) bool {
	r := layer.Rect
	if !ambientImage(layer.Image, layer.SourceRect) || !finitePainted(r.X, r.Y, r.W, r.H) || r.X < 0 || r.Y < 0 || r.W <= 0 || r.H <= 0 || r.X+r.W > 1.000001 || r.Y+r.H > 1.000001 {
		return false
	}
	return !full || (r.X == 0 && r.Y == 0 && r.W == 1 && r.H == 1)
}

func checkPaintedStorefronts(s PaintedScene, fronts []PaintedStorefront, walkable, actors map[string]bool) error {
	if len(fronts) > 64 {
		return fmt.Errorf("painted %s: at most 64 storefronts", s.LevelID)
	}
	ids := map[string]bool{}
	for index, f := range fronts {
		bad := func(reason string) error { return fmt.Errorf("painted %s storefront %s: %s", s.LevelID, f.ID, reason) }
		if strings.TrimSpace(f.ID) == "" || ids[f.ID] {
			return bad("empty or duplicate stable identity")
		}
		ids[f.ID] = true
		if !actors[f.MerchantID] {
			return bad("merchant_id must identify a Life actor")
		}
		p := f.ObservationCell
		if p.Y < 0 || p.Y >= len(s.Navigation) || p.X < 0 || p.X >= len(s.Navigation[p.Y]) || !walkable[string(s.Navigation[p.Y][p.X])] {
			return bad("observation_cell must be on the real walkable approach")
		}
		if len(f.Quad) != 4 {
			return bad("quad requires TL/TR/BR/BL canvas points")
		}
		area := 0.0
		for i, a := range f.Quad {
			b, c := f.Quad[(i+1)%4], f.Quad[(i+2)%4]
			if !finitePainted(a.X, a.Y) || a.X < 0 || a.X > float64(s.Width) || a.Y < 0 || a.Y > float64(s.Height) {
				return bad("quad lies outside canvas")
			}
			if ambientCross(a, b, c) <= .001 {
				return bad("quad must be strictly convex in TL/TR/BR/BL order")
			}
			area += a.X*b.Y - a.Y*b.X
		}
		if area < 8 {
			return bad("quad must cover at least four canvas pixels")
		}
		// Sills should face downward on screen. This catches reversed UV order
		// without requiring a rectangular or perfectly affine artistic facade.
		if (f.Quad[0].Y + f.Quad[1].Y) >= (f.Quad[2].Y + f.Quad[3].Y) {
			return bad("quad top and sill are reversed")
		}
		for _, old := range fronts[:index] {
			if ambientPolygonsOverlap(f.Quad, old.Quad) {
				return bad("facade overlaps another storefront")
			}
		}
		if f.OpenSecond < 0 || f.OpenSecond >= 86400 || f.CloseSecond < 0 || f.CloseSecond >= 86400 || f.OpenSecond == f.CloseSecond {
			return bad("schedule needs distinct daily opening and closing times")
		}
		if f.OpeningMS < 100 || f.OpeningMS > 10000 || f.ClosingMS < 100 || f.ClosingMS > 10000 {
			return bad("animation durations must be 100..10000 ms")
		}
		if f.ClosureMode != "pack_away" && f.ClosureMode != "protected_display" {
			return bad("closure_mode must be pack_away or protected_display")
		}
		if !storefrontLayerValid(f.Recess, true) || !storefrontLayerValid(f.Shutter, true) {
			return bad("recess and shutter need valid images and complete opaque local rectangles")
		}
		if !storefrontLayerValid(f.Goods, false) {
			return bad("goods layer must fit inside the calibrated facade")
		}
		if f.Festival != nil && !storefrontLayerValid(*f.Festival, false) {
			return bad("festival layer must fit inside the calibrated facade")
		}
		if len(f.HolidayWindows) > 128 {
			return bad("at most 128 authored holiday windows")
		}
		for i, w := range f.HolidayWindows {
			if f.Festival == nil || w.StartSecond < 0 || w.EndSecond <= w.StartSecond || w.EndSecond > math.MaxInt64-86400 {
				return bad("holiday needs an asset and a finite positive absolute interval")
			}
			if i > 0 && w.StartSecond < f.HolidayWindows[i-1].EndSecond {
				return bad("holiday windows must be ordered and disjoint")
			}
		}
	}
	return nil
}
