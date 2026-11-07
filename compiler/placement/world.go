// Package placement owns ANG's metric world frame and engine-neutral location
// placements. Plan X is east, plan Y is south, and height is up. No client needs
// to reconstruct a pivot, swap plan axes, or convert handedness.
package placement

import (
	"math"

	"github.com/strogmv/ang/compiler/locationsdk"
)

const WorldFrame = "ang.world.east-up-north.v1"

// Vec3 is in metres, or degrees when explicitly named RotationDegrees.
type Vec3 [3]float64

// FromPlan converts source (east, up, south) metres to ANG/Godot world metres.
// This is the sole handedness conversion, shared by models, structure and camera.
func FromPlan(p Vec3) Vec3 { return Vec3{p[0], p[1], -p[2]} }

type Transform struct {
	Position        Vec3 `json:"position"`
	RotationDegrees Vec3 `json:"rotation_degrees"`
	// Source models use plan-local (east, up, south) numeric coordinates, as do
	// the oracle GLBs. Reflection is part of the transform, never asset guessing.
	Scale Vec3 `json:"scale"`
}

func PlanTransform(position Vec3, rotation float64) Transform {
	return Transform{Position: FromPlan(position), RotationDegrees: Vec3{0, rotation, 0}, Scale: FromPlan(Vec3{1, 1, 1})}
}

// Pose compensates the NW pivot using the compiled (already rotated) footprint.
func Pose(at locationsdk.Cell, width, depth int, bottom float64, rotation int) Transform {
	u := float64(locationsdk.UnitMM) / 1000
	w, d := float64(width)*u, float64(depth)*u
	if rotation == 90 || rotation == 270 {
		w, d = d, w
	}
	p := Vec3{float64(at.X) * u, bottom, float64(at.Y) * u}
	switch rotation {
	case 90:
		p[0] += d
	case 180:
		p[0] += w
		p[2] += d
	case 270:
		p[2] += w
	}
	return PlanTransform(p, float64(rotation))
}

// BasisColumns includes the model reflection; an Euler yaw alone cannot express it.
func (t Transform) BasisColumns() [3]Vec3 {
	a := t.RotationDegrees[1] * math.Pi / 180
	c, s := math.Cos(a), math.Sin(a)
	return [3]Vec3{{c * t.Scale[0], 0, -s * t.Scale[0]}, {0, t.Scale[1], 0}, {s * t.Scale[2], 0, c * t.Scale[2]}}
}

// CameraBasis returns a proper right/up/back camera basis after the same world
// conversion. The original game camera's right vector is westward, not the
// opposite legacy atlas basis that caused the mirrored crane.
func CameraBasis(elevation, azimuth float64) [3]Vec3 {
	e, a := elevation*math.Pi/180, azimuth*math.Pi/180
	return [3]Vec3{
		FromPlan(Vec3{-math.Cos(a), 0, math.Sin(a)}),
		FromPlan(Vec3{-math.Sin(e) * math.Sin(a), math.Cos(e), -math.Sin(e) * math.Cos(a)}),
		FromPlan(Vec3{math.Cos(e) * math.Sin(a), math.Sin(e), math.Cos(e) * math.Cos(a)}),
	}
}
