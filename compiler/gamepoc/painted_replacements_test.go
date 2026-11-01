package gamepoc

import (
	"crypto/sha256"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func surfaceFixture() PaintedScene {
	return PaintedScene{LevelID: "market", Background: "MarketArt/base", Width: 100, Height: 100, Navigation: []string{"##..", "##.."}, Origin: PaintedPoint{20, 55}, AxisX: PaintedPoint{2, 0}, AxisY: PaintedPoint{0, 2}, Occluders: []PaintedOccluder{{ID: "wall", Polygon: []PaintedPoint{{0, 0}, {60, 0}, {60, 60}, {0, 60}}}}, SurfaceReplacements: []PaintedSurfaceReplacement{{ID: "insert", AssetID: "panel.v1", Image: "MarketArt/part", BackgroundSHA256: strings.Repeat("a", 64), OccluderID: "wall", Quad: []PaintedPoint{{10, 10}, {40, 15}, {40, 45}, {10, 40}}, SolidCells: []Point{{0, 0}}}}}
}
func TestPaintedSurfaceReplacementKeepsExistingArchitecture(t *testing.T) {
	stand := func(p Point) bool { return p.X >= 2 }
	if err := checkPaintedReplacements(surfaceFixture(), stand); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*PaintedScene){
		"unknown frame":  func(s *PaintedScene) { s.SurfaceReplacements[0].OccluderID = "door" },
		"walkable solid": func(s *PaintedScene) { s.SurfaceReplacements[0].SolidCells = []Point{{2, 0}} },
		"wrong orientation": func(s *PaintedScene) {
			s.SurfaceReplacements[0].Quad[0], s.SurfaceReplacements[0].Quad[2] = s.SurfaceReplacements[0].Quad[2], s.SurfaceReplacements[0].Quad[0]
		},
		"nonaffine": func(s *PaintedScene) { s.SurfaceReplacements[0].Quad[2].X++ },
		"outside frame": func(s *PaintedScene) {
			for i := range s.SurfaceReplacements[0].Quad {
				s.SurfaceReplacements[0].Quad[i].X += 40
			}
		},
		"overlap": func(s *PaintedScene) {
			b := s.SurfaceReplacements[0]
			b.ID = "second"
			s.SurfaceReplacements = append(s.SurfaceReplacements, b)
		},
		"bad hash": func(s *PaintedScene) { s.SurfaceReplacements[0].BackgroundSHA256 = "no" },
	} {
		t.Run(name, func(t *testing.T) {
			s := surfaceFixture()
			mutate(&s)
			if checkPaintedReplacements(s, stand) == nil {
				t.Fatal("invalid surface accepted")
			}
		})
	}
}
func TestPaintedSurfaceReplacementLocksBaseAndRequiresCoverage(t *testing.T) {
	unity := t.TempDir()
	dir := filepath.Join(unity, "Assets", "Resources", "MarketArt")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	write := func(name string, opaque bool) []byte {
		im := image.NewNRGBA(image.Rect(0, 0, 8, 8))
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				im.SetNRGBA(x, y, color.NRGBA{30, 50, 60, 255})
			}
		}
		if !opaque {
			im.SetNRGBA(3, 3, color.NRGBA{})
		}
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		if err = png.Encode(f, im); err != nil {
			t.Fatal(err)
		}
		if err = f.Close(); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	base := write("base", true)
	write("part", true)
	s := surfaceFixture()
	s.SurfaceReplacements[0].BackgroundSHA256 = fmt.Sprintf("%x", sha256.Sum256(base))
	if err := checkPaintedReplacementImages(unity, s); err != nil {
		t.Fatal(err)
	}
	write("part", false)
	if err := checkPaintedReplacementImages(unity, s); err == nil || !strings.Contains(err.Error(), "opaque") {
		t.Fatalf("incomplete replacement accepted: %v", err)
	}
	write("part", true)
	write("base", false)
	if err := checkPaintedReplacementImages(unity, s); err == nil || !strings.Contains(err.Error(), "hash changed") {
		t.Fatalf("moved background accepted: %v", err)
	}
}

// A recess narrower than an edge sampling interval must still reject the insert.
func TestPaintedSurfaceReplacementRejectsNarrowRecess(t *testing.T) {
	s := surfaceFixture()
	s.Occluders[0].Polygon = []PaintedPoint{{0, 0}, {23.72, 0}, {23.72, 25}, {23.74, 25}, {23.74, 0}, {60, 0}, {60, 60}, {0, 60}}
	if err := checkPaintedReplacements(s, func(p Point) bool { return p.X >= 2 }); err == nil {
		t.Fatal("insert bridges a narrow existing recess")
	}
}
