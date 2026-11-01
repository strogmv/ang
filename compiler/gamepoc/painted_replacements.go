package gamepoc

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
)

// A calibrated surface substitution inside existing solid architecture. It
// changes neither the architectural silhouette nor authoritative navigation.
type PaintedSurfaceReplacement struct {
	ID               string         `json:"id"`
	AssetID          string         `json:"asset_id"`
	Image            string         `json:"image"`
	BackgroundSHA256 string         `json:"background_sha256"`
	OccluderID       string         `json:"occluder_id"`
	Quad             []PaintedPoint `json:"quad"` // top-left, top-right, bottom-right, bottom-left
	SolidCells       []Point        `json:"solid_cells"`
}

func checkPaintedReplacements(s PaintedScene, stand func(Point) bool) error {
	if len(s.SurfaceReplacements) > 64 {
		return fmt.Errorf("painted %s: surface replacement budget", s.LevelID)
	}
	ids := map[string]bool{}
	for _, r := range s.SurfaceReplacements {
		bad := func(detail string) error {
			return fmt.Errorf("painted %s surface replacement %q: %s", s.LevelID, r.ID, detail)
		}
		hash, err := hex.DecodeString(r.BackgroundSHA256)
		if r.ID == "" || r.AssetID == "" || ids[r.ID] || !artSceneImagePath(r.Image) || err != nil || len(hash) != 32 {
			return bad("invalid identity, image or background hash")
		}
		ids[r.ID] = true
		if len(r.Quad) != 4 {
			return bad("quad requires four ordered corners")
		}
		for _, p := range r.Quad {
			if !finitePainted(p.X, p.Y) || p.X < 0 || p.Y < 0 || p.X > float64(s.Width) || p.Y > float64(s.Height) {
				return bad("quad outside canvas")
			}
		}
		a, b, c, d := r.Quad[0], r.Quad[1], r.Quad[2], r.Quad[3]
		det := (b.X-a.X)*(d.Y-a.Y) - (b.Y-a.Y)*(d.X-a.X)
		if det <= 1 || math.Abs(a.X+c.X-b.X-d.X) > .001 || math.Abs(a.Y+c.Y-b.Y-d.Y) > .001 {
			return bad("quad must be a nondegenerate clockwise affine rectangle")
		}
		var owner *PaintedOccluder
		for i := range s.Occluders {
			if s.Occluders[i].ID == r.OccluderID {
				owner = &s.Occluders[i]
			}
		}
		if owner == nil {
			return bad("unknown existing occluder")
		}
		// Check edges as well as corners: an insert cannot bridge a recess or doorway.
		for i, p := range r.Quad {
			q := r.Quad[(i+1)%4]
			for j, a := range owner.Polygon {
				if ambientEdgesCross(p, q, a, owner.Polygon[(j+1)%len(owner.Polygon)]) {
					return bad("quad crosses its existing occluder boundary")
				}
			}
			for n := 0; n <= 32; n++ {
				t := float64(n) / 32
				if !ambientInside(owner.Polygon, PaintedPoint{p.X + (q.X-p.X)*t, p.Y + (q.Y-p.Y)*t}, false) {
					return bad("quad must remain inside its existing occluder")
				}
			}
		}
		if len(r.SolidCells) < 1 || len(r.SolidCells) > 64 {
			return bad("existing solid cells required")
		}
		cells := map[Point]bool{}
		for _, p := range r.SolidCells {
			if p.Y < 0 || p.Y >= len(s.Navigation) || p.X < 0 || p.X >= len(s.Navigation[p.Y]) || stand(p) || cells[p] || !ambientInside(owner.Polygon, s.project(p), false) {
				return bad("solid cells must already be distinct blocked cells")
			}
			cells[p] = true
		}
		for y, row := range s.Navigation {
			for x := range row {
				p := s.project(Point{x, y})
				if stand(Point{x, y}) && ambientInside(r.Quad, p, false) {
					return bad("surface covers a walkable floor centre")
				}
			}
		}
	}
	for i, a := range s.SurfaceReplacements {
		for _, b := range s.SurfaceReplacements[i+1:] {
			if ambientPolygonsOverlap(a.Quad, b.Quad) {
				return fmt.Errorf("painted %s: overlapping surface replacements %q and %q", s.LevelID, a.ID, b.ID)
			}
		}
	}
	return nil
}

func checkPaintedReplacementImages(unity string, s PaintedScene) error {
	if len(s.SurfaceReplacements) == 0 {
		return nil
	}
	base, err := os.ReadFile(filepath.Join(unity, "Assets", "Resources", filepath.FromSlash(s.Background)+".png"))
	if err != nil {
		return err
	}
	sum := fmt.Sprintf("%x", sha256.Sum256(base))
	for _, r := range s.SurfaceReplacements {
		if sum != r.BackgroundSHA256 {
			return fmt.Errorf("painted %s replacement %q: background hash changed; review calibration", s.LevelID, r.ID)
		}
		f, err := os.Open(filepath.Join(unity, "Assets", "Resources", filepath.FromSlash(r.Image)+".png"))
		if err != nil {
			return err
		}
		im, _, err := image.Decode(f)
		_ = f.Close()
		if err != nil {
			return err
		}
		bounds := im.Bounds()
		if bounds.Dx() > 2048 || bounds.Dy() > 2048 {
			return fmt.Errorf("painted replacement %q: image budget", r.ID)
		}
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				_, _, _, alpha := im.At(x, y).RGBA()
				if alpha != 65535 {
					return fmt.Errorf("painted replacement %q: surface image must be opaque, old surface must not show through", r.ID)
				}
			}
		}
	}
	return nil
}
