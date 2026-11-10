package godot

import (
	"fmt"
	"math"

	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
)

// ProceduralSolid is an actually emitted solid in plan coordinates X east,
// Y south, Z up. A triangle is an exact floor-corner prism, not its AABB.
type ProceduralSolid struct {
	SourceID string
	Min, Max [3]float64
	Triangle *[3][2]float64
}

func (s ProceduralSolid) IntersectsBox(minimum, maximum [3]float64) bool {
	for axis := range 3 {
		if s.Max[axis] <= minimum[axis]+1e-9 || s.Min[axis] >= maximum[axis]-1e-9 {
			return false
		}
	}
	if s.Triangle == nil {
		return true
	}
	polygon := clipFloorPolygon(s.Triangle[:], [4]float64{minimum[0], minimum[1], maximum[0], maximum[1]})
	area := 0.
	for i, p := range polygon {
		q := polygon[(i+1)%len(polygon)]
		area += p[0]*q[1] - p[1]*q[0]
	}
	return math.Abs(area) > 1e-12
}

// ResolveProceduralSolids uses the renderer's final replacements, slab cuts,
// pocket roofs and corner pieces. It includes default SDK obstacle boxes and
// authored architecture; no art can hide from the empty-volume guard merely
// because it is procedural rather than a model mesh.
func ResolveProceduralSolids(level locationsdk.CompiledLevel, manifest placement.Manifest, finishing *Finishing) ([]ProceduralSolid, error) {
	resolved, err := ResolveArchitectureForLevel(finishing, level)
	if err != nil {
		return nil, err
	}
	_, _, _, roles, replaced, err := resolved.render(manifest, proceduralSurfaces(level, resolved))
	if err != nil {
		return nil, err
	}
	boxes, err := proceduralBoxes(level, manifest, resolved, roles, replaced, false)
	if err != nil {
		return nil, err
	}
	solids := []ProceduralSolid{}
	for _, draw := range boxes {
		// Both strings are emitted by vector() above, never supplied by a client.
		var position, size [3]float64
		if n, e := fmt.Sscanf(draw.Position, "Vector3(%f, %f, %f)", &position[0], &position[1], &position[2]); e != nil || n != 3 {
			return nil, fmt.Errorf("procedural position %q: %v", draw.Name, e)
		}
		if n, e := fmt.Sscanf(draw.Size, "Vector3(%f, %f, %f)", &size[0], &size[1], &size[2]); e != nil || n != 3 {
			return nil, fmt.Errorf("procedural size %q: %v", draw.Name, e)
		}
		centre := [3]float64{position[0], -position[2], position[1]}
		extent := [3]float64{size[0], size[2], size[1]}
		s := ProceduralSolid{SourceID: draw.SourceID}
		for axis := range 3 {
			s.Min[axis] = centre[axis] - extent[axis]/2
			s.Max[axis] = centre[axis] + extent[axis]/2
		}
		solids = append(solids, s)
	}
	floors, err := ResolveFloorDraws(level, manifest, resolved)
	if err != nil {
		return nil, err
	}
	for _, draw := range floors {
		if draw.Triangle != nil {
			s := ProceduralSolid{SourceID: draw.SourceID, Triangle: draw.Triangle, Min: [3]float64{math.Inf(1), math.Inf(1), draw.TopM - draw.ThicknessM}, Max: [3]float64{math.Inf(-1), math.Inf(-1), draw.TopM}}
			for _, p := range draw.Triangle {
				for axis := range 2 {
					s.Min[axis] = math.Min(s.Min[axis], p[axis])
					s.Max[axis] = math.Max(s.Max[axis], p[axis])
				}
			}
			solids = append(solids, s)
		}
	}
	return solids, nil
}
