package godot

import "github.com/strogmv/ang/compiler/placement"

type floorPolygonPart struct {
	Points    [][2]float64
	Thickness float64
}

func openingCornerPolygons(o FloorOpening) [][][2]float64 {
	x0, y0, x1, y1, c := o.Rect[0], o.Rect[1], o.Rect[2], o.Rect[3], o.CornerCutM
	return [][][2]float64{{{x0, y0}, {x0 + c, y0}, {x0, y0 + c}}, {{x1, y0}, {x1, y0 + c}, {x1 - c, y0}}, {{x1, y1}, {x1 - c, y1}, {x1, y1 - c}}, {{x0, y1}, {x0, y1 - c}, {x0 + c, y1}}}
}
func clipFloorHalfPlane(p [][2]float64, axis int, limit float64, greater bool) [][2]float64 {
	out := [][2]float64{}
	if len(p) == 0 {
		return out
	}
	inside := func(v [2]float64) bool {
		if greater {
			return v[axis] >= limit
		}
		return v[axis] <= limit
	}
	for i, b := range p {
		a := p[(i+len(p)-1)%len(p)]
		ia, ib := inside(a), inside(b)
		if ia != ib {
			t := (limit - a[axis]) / (b[axis] - a[axis])
			out = append(out, [2]float64{a[0] + t*(b[0]-a[0]), a[1] + t*(b[1]-a[1])})
		}
		if ib {
			out = append(out, b)
		}
	}
	return out
}
func clipFloorPolygon(p [][2]float64, r [4]float64) [][2]float64 {
	p = clipFloorHalfPlane(p, 0, r[0], true)
	p = clipFloorHalfPlane(p, 0, r[2], false)
	p = clipFloorHalfPlane(p, 1, r[1], true)
	return clipFloorHalfPlane(p, 1, r[3], false)
}
func subtractFloorPolygon(p [][2]float64, r [4]float64) [][][2]float64 {
	out := [][][2]float64{}
	for _, q := range [][][2]float64{clipFloorHalfPlane(p, 1, r[1], false), clipFloorHalfPlane(p, 1, r[3], true), clipFloorHalfPlane(clipFloorHalfPlane(clipFloorHalfPlane(p, 1, r[1], true), 1, r[3], false), 0, r[0], false), clipFloorHalfPlane(clipFloorHalfPlane(clipFloorHalfPlane(p, 1, r[1], true), 1, r[3], false), 0, r[2], true)} {
		if len(q) >= 3 {
			out = append(out, q)
		}
	}
	return out
}
func floorTriangleArea(t [3][2]float64) float64 {
	return ((t[1][0]-t[0][0])*(t[2][1]-t[0][1]) - (t[1][1]-t[0][1])*(t[2][0]-t[0][0])) / 2
}

// Unit PrismMesh left_to_right0 has XY triangle(-.5,-.5),(.5,-.5),(-.5,.5).
// Its localZ extrusion becomes worldY; any clipped triangle uses the same
// primitive by a deterministic affine basis, with a positive determinant.
func floorTriangleTransform(t [3][2]float64, top, thickness float64) string {
	u := placement.Vec3{t[1][0] - t[0][0], 0, -(t[1][1] - t[0][1])}
	v := placement.Vec3{t[2][0] - t[0][0], 0, -(t[2][1] - t[0][1])}
	p := placement.Vec3{t[0][0] + (u[0]+v[0])/2, top - thickness/2, -t[0][1] + (u[2]+v[2])/2}
	return transform([3]placement.Vec3{u, v, {0, thickness, 0}}, p)
}
