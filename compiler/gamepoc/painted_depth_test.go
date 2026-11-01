package gamepoc

import (
	"math"
	"strings"
	"testing"
)

func TestPaintedElevatedDepthBoundaryValidation(t *testing.T) {
	fixture := func() Compiled {
		c := paintedFixture()
		c.Art.Painted[0].Occluders = []PaintedOccluder{{ID: "overhead_duct", Polygon: []PaintedPoint{{100, 50}, {300, 50}, {300, 100}, {100, 100}}, DepthY: 250, DepthPolygon: []PaintedPoint{{100, 0}, {300, 0}, {300, 200}, {100, 250}}}}
		return c
	}
	if err := checkPaintedScenes(fixture()); err != nil {
		t.Fatal(err)
	}
	for name, points := range map[string][]PaintedPoint{
		"two points":    {{100, 100}, {200, 200}},
		"nonfinite":     {{100, 100}, {200, 200}, {math.NaN(), 300}},
		"out of canvas": {{100, 100}, {200, 200}, {600, 300}},
	} {
		t.Run(name, func(t *testing.T) {
			c := fixture()
			c.Art.Painted[0].Occluders[0].DepthPolygon = points
			if err := checkPaintedScenes(c); err == nil || !strings.Contains(err.Error(), "depth_polygon") {
				t.Fatalf("invalid elevated boundary accepted: %v", err)
			}
		})
	}
}
