package godot

import (
	"fmt"
	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
	"strconv"
)

// PrimitiveDraw records the topology of an actually emitted primitive node.
// Authoring blockouts are independent validation proxies and never feed this list.
type PrimitiveDraw struct {
	Name      string `json:"name"`
	SourceID  string `json:"source_id"`
	Topology  string `json:"topology"`
	Triangles int    `json:"triangles"`
}
type ProceduralGeometry struct {
	Primitives []PrimitiveDraw `json:"primitives"`
	Triangles  int             `json:"triangles"`
}

// PlanProceduralGeometry shares the renderer's exact suppression and ordering.
// The template uses unsubdivided unit BoxMesh (12 triangles) or PlaneMesh (2).
func PlanProceduralGeometry(level locationsdk.CompiledLevel, manifest placement.Manifest, finishing *Finishing, floorOnly bool) (ProceduralGeometry, error) {
	plan := ProceduralGeometry{Primitives: []PrimitiveDraw{}}
	resolved, err := ResolveArchitectureForLevel(finishing, level)
	if err != nil {
		return plan, err
	}
	_, _, _, roles, replaced, err := resolved.render(manifest, proceduralSurfaces(level, resolved))
	if err != nil {
		return plan, err
	}
	boxes, err := proceduralBoxes(level, manifest, resolved, roles, replaced, floorOnly)
	if err != nil {
		return plan, err
	}
	topology, triangles := "BoxMesh", 12
	if floorOnly {
		topology, triangles = "PlaneMesh", 2
	}
	for _, draw := range boxes {
		plan.Primitives = append(plan.Primitives, PrimitiveDraw{draw.Name, draw.SourceID, topology, triangles})
		plan.Triangles += triangles
	}
	floorDraws, err := ResolveFloorDraws(level, manifest, resolved)
	if err != nil {
		return plan, err
	}
	cornerIndex := 0
	for _, draw := range floorDraws {
		if draw.Triangle != nil {
			plan.Primitives = append(plan.Primitives, PrimitiveDraw{fmt.Sprintf("FloorCorner_%03d", cornerIndex), draw.SourceID, "PrismMesh", 8})
			plan.Triangles += 8
			cornerIndex++
		}
	}
	return plan, nil
}

func proceduralSurfaces(level locationsdk.CompiledLevel, finishing *Finishing) map[string]bool {
	surfaces := map[string]bool{}
	for _, p := range level.Preview.Floor {
		surfaces[p.SourceID] = true
	}
	if finishing != nil && finishing.Architecture != nil {
		for _, d := range finishing.Architecture.Draws {
			surfaces[d.ID] = true
		}
	}
	if level.Navigation != nil {
		for _, a := range level.Navigation.Surfaces {
			surfaces[a.ID] = true
		}
		for _, a := range level.Navigation.Obstacles {
			surfaces[a.ID] = true
		}
	}
	return surfaces
}

func proceduralBoxes(level locationsdk.CompiledLevel, manifest placement.Manifest, finishing *Finishing, roles map[string]string, replaced map[string]bool, floorOnly bool) ([]box, error) {
	unit := float64(level.UnitMM) / 1000
	openings, err := validateFloorOpenings(level, manifest, finishing)
	if err != nil {
		return nil, err
	}
	recesses, err := validateFloorRecesses(level, manifest, finishing)
	if err != nil {
		return nil, err
	}
	boxes := []box{}
	addBox := func(id string, r locationsdk.Rect, bottom, top int, material string) {
		if top <= bottom || replaced[id] {
			return
		}
		if material == "floor" && (len(openings[id]) > 0 || len(recesses[id]) > 0) {
			rect := [4]float64{float64(r.X) * unit, float64(r.Y) * unit, float64(r.X+r.W) * unit, float64(r.Y+r.H) * unit}
			ref := roles["object:"+id]
			if ref == "" {
				ref = roles[material]
			}
			if ref == "" {
				ref = "SubResource(" + strconv.Quote(material) + ")"
			}
			for _, slab := range floorSlabParts(rect, float64(top)/1000, float64(top-bottom)/1000, openings[id], recesses[id]) {
				part := slab.Rect
				p := placement.FromPlan(placement.Vec3{(part[0] + part[2]) / 2, slab.Top - slab.Thickness/2, (part[1] + part[3]) / 2})
				if floorOnly {
					p[1] = float64(top) / 1000
				}
				boxes = append(boxes, box{fmt.Sprintf("Structure_%03d", len(boxes)), id, vector(p), vector(placement.Vec3{part[2] - part[0], slab.Thickness, part[3] - part[1]}), ref})
			}
			return
		}
		p := placement.FromPlan(placement.Vec3{(float64(r.X) + float64(r.W)/2) * unit, float64(bottom+top) / 2000, (float64(r.Y) + float64(r.H)/2) * unit})
		if floorOnly {
			p[1] = float64(top) / 1000
		}
		ref := roles["object:"+id]
		if ref == "" {
			ref = roles[material]
		}
		if ref == "" {
			ref = "SubResource(" + strconv.Quote(material) + ")"
		}
		boxes = append(boxes, box{fmt.Sprintf("Structure_%03d", len(boxes)), id, vector(p), vector(placement.Vec3{float64(r.W) * unit, float64(top-bottom) / 1000, float64(r.H) * unit}), ref})
	}
	if n := level.Navigation; n != nil {
		for _, surface := range n.Surfaces {
			if manifest.Instances[surface.ID].ModelRef != "" {
				continue
			}
			for _, r := range surface.Slab {
				addBox(surface.ID, r, surface.ElevationMM-surface.SlabMM, surface.ElevationMM, "floor")
			}
		}
		for _, o := range n.Obstacles {
			if manifest.Instances[o.ID].ModelRef == "" {
				role := "wall"
				if o.Style == "panel" && roles["steel"] != "" {
					role = "steel"
				}
				if role == "wall" && finishing != nil {
					materialRole := finishing.SurfaceRoles[role]
					if r := finishing.ObjectRoles[o.ID]; r != "" {
						materialRole = r
					}
					if _, e := ValidateWallMaterial(finishing, o.ID, materialRole); e != nil {
						return nil, e
					}
				}
				addBox(o.ID, o.Rect, o.BottomMM, o.TopMM, role)
			}
		}
	} else {
		for _, r := range level.Preview.Floor {
			addBox(r.SourceID, locationsdk.Rect{X: r.At.X, Y: r.At.Y, W: 1, H: 1}, level.Elevation*level.UnitMM-250, level.Elevation*level.UnitMM, "floor")
		}
	}
	if f := finishing; f != nil && f.Architecture != nil {
		seen := map[string]bool{}
		for _, d := range f.Architecture.Draws {
			if d.ID == "" || seen[d.ID] || d.Reason == "" || (d.Kind != "wall" && d.Kind != "steel" && d.Kind != "trim") {
				return nil, fmt.Errorf("architecture.draws: unique ID, kind wall/steel/trim and source reason required: %q", d.ID)
			}
			seen[d.ID] = true
			if cut := d.NearCut; cut != nil {
				if d.Kind != "wall" || !finite(cut.HeightM) || cut.HeightM < .3 || cut.HeightM >= d.Size[1] || cut.Reason == "" {
					return nil, fmt.Errorf("architecture.draws[%q].near_cut: wall requires a height of at least 0.3 m strictly below its full height and a written reason", d.ID)
				}
			}
			for _, v := range append(d.Position[:], d.Size[:]...) {
				if !finite(v) {
					return nil, fmt.Errorf("architecture.draws[%q]: nonfinite metric geometry", d.ID)
				}
			}
			for _, v := range d.Size {
				if v <= 0 {
					return nil, fmt.Errorf("architecture.draws[%q]: positive dimensions required", d.ID)
				}
			}
			if d.Kind == "wall" {
				if _, e := ValidateWallMaterial(f, d.ID, d.MaterialRole); e != nil {
					return nil, e
				}
			}
			ref := roles["object:"+d.ID]
			if ref == "" {
				return nil, fmt.Errorf("architecture.draws[%q]: declare object_roles material %q", d.ID, d.MaterialRole)
			}
			boxes = append(boxes, box{fmt.Sprintf("Architecture_%03d", len(boxes)), d.ID, vector(placement.FromPlan(placement.Vec3(d.Position))), vector(placement.Vec3(d.Size)), ref})
		}
	}
	return boxes, nil
}
