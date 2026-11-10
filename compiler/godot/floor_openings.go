package godot

import (
	"fmt"
	"math"

	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
)

// FloorOpening removes only a rendered slab rectangle. The owner mechanism
// still supplies authoritative closed-leaf support and dynamic navigation.
type FloorOpening struct {
	ID            string     `json:"id"`
	SurfaceID     string     `json:"surface_id"`
	OwnerInstance string     `json:"owner_instance"`
	Rect          [4]float64 `json:"rect_m"` // x0, south0, x1, south1
	Reason        string     `json:"reason"`
	CornerCutM    float64    `json:"corner_cut_m,omitempty"`
}

func validateFloorOpenings(level locationsdk.CompiledLevel, manifest placement.Manifest, f *Finishing) (map[string][][4]float64, error) {
	out := map[string][][4]float64{}
	if f == nil {
		return out, nil
	}
	surfaces := map[string]bool{}
	if level.Navigation != nil {
		for _, s := range level.Navigation.Surfaces {
			surfaces[s.ID] = true
		}
	} else {
		for _, cell := range level.Preview.Floor {
			surfaces[cell.SourceID] = true
		}
	}
	seen := map[string]bool{}
	for _, o := range f.FloorOpenings {
		if o.ID == "" || seen[o.ID] || o.Reason == "" || !surfaces[o.SurfaceID] || o.OwnerInstance == "" || manifest.Instances[o.OwnerInstance].ModelRef == "" {
			return nil, fmt.Errorf("finishing.floor_openings[%q]: unique ID, reason, floor surface and bound owner instance required", o.ID)
		}
		for _, v := range o.Rect {
			if !finite(v) {
				return nil, fmt.Errorf("finishing.floor_openings[%q]: finite metric rectangle required", o.ID)
			}
		}
		if o.Rect[0] >= o.Rect[2] || o.Rect[1] >= o.Rect[3] {
			return nil, fmt.Errorf("finishing.floor_openings[%q]: positive metric rectangle required", o.ID)
		}
		if !finite(o.CornerCutM) || o.CornerCutM < 0 || 2*o.CornerCutM >= math.Min(o.Rect[2]-o.Rect[0], o.Rect[3]-o.Rect[1]) {
			return nil, fmt.Errorf("finishing.floor_openings[%q]: corner cut must fit inside frame", o.ID)
		}
		for _, prior := range f.FloorOpenings {
			if prior.ID == o.ID {
				break
			}
			if prior.SurfaceID == o.SurfaceID && (prior.CornerCutM > 0 || o.CornerCutM > 0) && math.Min(prior.Rect[2], o.Rect[2]) > math.Max(prior.Rect[0], o.Rect[0]) && math.Min(prior.Rect[3], o.Rect[3]) > math.Max(prior.Rect[1], o.Rect[1]) {
				return nil, fmt.Errorf("finishing.floor_openings[%q]: chamfered frame openings must not overlap", o.ID)
			}
		}
		seen[o.ID] = true
		out[o.SurfaceID] = append(out[o.SurfaceID], o.Rect)
	}
	return out, nil
}

func cutFloorRect(rect [4]float64, openings [][4]float64) [][4]float64 {
	parts := [][4]float64{rect}
	for _, hole := range openings {
		next := [][4]float64{}
		for _, r := range parts {
			x0, y0 := math.Max(r[0], hole[0]), math.Max(r[1], hole[1])
			x1, y1 := math.Min(r[2], hole[2]), math.Min(r[3], hole[3])
			if x0 >= x1 || y0 >= y1 {
				next = append(next, r)
				continue
			}
			for _, piece := range [][4]float64{{r[0], r[1], r[2], y0}, {r[0], y1, r[2], r[3]}, {r[0], y0, x0, y1}, {x1, y0, r[2], y1}} {
				if piece[0] < piece[2] && piece[1] < piece[3] {
					next = append(next, piece)
				}
			}
		}
		parts = next
	}
	return parts
}

// FloorRecess keeps the walking surface at the authored elevation, while
// clearing equipment below it. Its cover thickness is an authoring dimension,
// not a structural certification. Collision/navigation support is unchanged.
type FloorRecess struct {
	ID              string     `json:"id"`
	SurfaceID       string     `json:"surface_id"`
	OwnerInstance   string     `json:"owner_instance"`
	Rect            [4]float64 `json:"rect_m"`
	CoverThicknessM float64    `json:"cover_thickness_m"`
	Reason          string     `json:"reason"`
}

type floorSlabPart struct {
	Rect           [4]float64
	Top, Thickness float64
}

func floorSlabParts(rect [4]float64, top, thickness float64, openings [][4]float64, recesses []FloorRecess) []floorSlabPart {
	parts := []floorSlabPart{}
	for _, r := range cutFloorRect(rect, openings) {
		parts = append(parts, floorSlabPart{r, top, thickness})
	}
	for _, recess := range recesses {
		next := []floorSlabPart{}
		for _, part := range parts {
			r := part.Rect
			x0, y0 := math.Max(r[0], recess.Rect[0]), math.Max(r[1], recess.Rect[1])
			x1, y1 := math.Min(r[2], recess.Rect[2]), math.Min(r[3], recess.Rect[3])
			if x0 >= x1 || y0 >= y1 {
				next = append(next, part)
				continue
			}
			for _, outside := range cutFloorRect(r, [][4]float64{recess.Rect}) {
				next = append(next, floorSlabPart{outside, part.Top, part.Thickness})
			}
			next = append(next, floorSlabPart{[4]float64{x0, y0, x1, y1}, part.Top, math.Min(part.Thickness, recess.CoverThicknessM)})
		}
		parts = next
	}
	return parts
}

func validateFloorRecesses(level locationsdk.CompiledLevel, manifest placement.Manifest, f *Finishing) (map[string][]FloorRecess, error) {
	out := map[string][]FloorRecess{}
	if f == nil {
		return out, nil
	}
	// Reuse the aperture contract: the recess must name real support and an
	// explicitly bound mechanism, so a misspelt surface cannot silently vanish.
	proxy := &Finishing{}
	for _, r := range f.FloorRecesses {
		if !finite(r.CoverThicknessM) || r.CoverThicknessM <= 0 {
			return nil, fmt.Errorf("finishing.floor_recesses[%q]: positive finite cover thickness required", r.ID)
		}
		proxy.FloorOpenings = append(proxy.FloorOpenings, FloorOpening{ID: r.ID, SurfaceID: r.SurfaceID, OwnerInstance: r.OwnerInstance, Rect: r.Rect, Reason: r.Reason})
		out[r.SurfaceID] = append(out[r.SurfaceID], r)
	}
	if _, err := validateFloorOpenings(level, manifest, proxy); err != nil {
		return nil, fmt.Errorf("finishing.floor_recesses: %w", err)
	}
	return out, nil
}

// FloorDraw is the renderer's resolved slab geometry in plan metres. Legacy
// scene builders consume this exact list instead of reinterpreting CUE cuts.
type FloorDraw struct {
	SourceID       string         `json:"source_id"`
	Rect           [4]float64     `json:"rect_m"`
	TopM           float64        `json:"top_m"`
	ThicknessM     float64        `json:"thickness_m"`
	Triangle       *[3][2]float64 `json:"triangle_m,omitempty"`
	GodotTransform string         `json:"godot_transform,omitempty"`
}

func ResolveFloorDraws(level locationsdk.CompiledLevel, manifest placement.Manifest, finishing *Finishing) ([]FloorDraw, error) {
	resolved, err := ResolveArchitectureForLevel(finishing, level)
	if err != nil {
		return nil, err
	}
	_, _, _, _, replaced, err := resolved.render(manifest, proceduralSurfaces(level, resolved))
	if err != nil {
		return nil, err
	}
	openings, err := validateFloorOpenings(level, manifest, resolved)
	if err != nil {
		return nil, err
	}
	recesses, err := validateFloorRecesses(level, manifest, resolved)
	if err != nil {
		return nil, err
	}
	out := []FloorDraw{}
	unit := float64(level.UnitMM) / 1000
	add := func(id string, r locationsdk.Rect, bottom, top int) {
		if replaced[id] || manifest.Instances[id].ModelRef != "" {
			return
		}
		rect := [4]float64{float64(r.X) * unit, float64(r.Y) * unit, float64(r.X+r.W) * unit, float64(r.Y+r.H) * unit}
		for _, p := range floorSlabParts(rect, float64(top)/1000, float64(top-bottom)/1000, openings[id], recesses[id]) {
			out = append(out, FloorDraw{SourceID: id, Rect: p.Rect, TopM: p.Top, ThicknessM: p.Thickness})
		}
		if resolved != nil {
			for _, opening := range resolved.FloorOpenings {
				if opening.SurfaceID != id || opening.CornerCutM == 0 {
					continue
				}
				for _, polygon := range openingCornerPolygons(opening) {
					polygon = clipFloorPolygon(polygon, rect)
					if len(polygon) < 3 {
						continue
					}
					parts := []floorPolygonPart{{polygon, float64(top-bottom) / 1000}}
					for _, recess := range recesses[id] {
						next := []floorPolygonPart{}
						for _, part := range parts {
							for _, outside := range subtractFloorPolygon(part.Points, recess.Rect) {
								next = append(next, floorPolygonPart{outside, part.Thickness})
							}
							inside := clipFloorPolygon(part.Points, recess.Rect)
							if len(inside) >= 3 {
								next = append(next, floorPolygonPart{inside, math.Min(part.Thickness, recess.CoverThicknessM)})
							}
						}
						parts = next
					}
					for _, part := range parts {
						for i := 1; i+1 < len(part.Points); i++ {
							triangle := [3][2]float64{part.Points[0], part.Points[i], part.Points[i+1]}
							area := floorTriangleArea(triangle)
							if math.Abs(area) < 1e-12 {
								continue
							}
							if area < 0 {
								triangle[1], triangle[2] = triangle[2], triangle[1]
							}
							out = append(out, FloorDraw{SourceID: id, TopM: float64(top) / 1000, ThicknessM: part.Thickness, Triangle: &triangle, GodotTransform: floorTriangleTransform(triangle, float64(top)/1000, part.Thickness)})
						}
					}
				}
			}
		}

	}
	if level.Navigation != nil {
		for _, surface := range level.Navigation.Surfaces {
			for _, r := range surface.Slab {
				add(surface.ID, r, surface.ElevationMM-surface.SlabMM, surface.ElevationMM)
			}
		}
	} else {
		for _, r := range level.Preview.Floor {
			add(r.SourceID, locationsdk.Rect{X: r.At.X, Y: r.At.Y, W: 1, H: 1}, level.Elevation*level.UnitMM-250, level.Elevation*level.UnitMM)
		}
	}
	return out, nil
}
