package godot

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler/locationsdk"
)

// WallTopMass declares the structural envelope shown hatched on an authored plan.
// Its cells outside every authored floor and explicit opening receive a cap at
// wall-top height. This never adds a walkable surface or hides an interior drop.
type WallTopMass struct {
	ID           string             `json:"id"`
	Bounds       locationsdk.Rect   `json:"bounds"`
	Openings     []locationsdk.Rect `json:"openings,omitempty"`
	HeightM      float64            `json:"height_above_floor_m"`
	ThicknessM   float64            `json:"thickness_m"`
	MaterialRole string             `json:"material_role,omitempty"`
	Reason       string             `json:"reason"`
}

func resolveWallTopMass(f *Finishing, level locationsdk.CompiledLevel) (*Finishing, error) {
	if f == nil || f.Architecture == nil || len(f.Architecture.WallTopMass) == 0 || f.Architecture.massResolved {
		return f, nil
	}
	a := *f.Architecture
	if a.Setting != "dome_interior" {
		return nil, fmt.Errorf("architecture.wall_top_mass: only dome_interior has structural wall mass")
	}
	a.Draws = append([]ArchitectureDraw(nil), a.Draws...)
	floor := map[locationsdk.Cell]bool{}
	for _, c := range level.Preview.Floor {
		floor[c.At] = true
	}
	seen := map[string]bool{}
	unit := float64(level.UnitMM) / 1000
	if unit <= 0 {
		return nil, fmt.Errorf("architecture.wall_top_mass: positive SDK unit required")
	}
	for _, m := range a.WallTopMass {
		b := m.Bounds
		if strings.TrimSpace(m.ID) == "" || seen[m.ID] || strings.TrimSpace(m.Reason) == "" || b.W <= 0 || b.H <= 0 || b.X < 0 || b.Y < 0 || b.X+b.W > level.Width || b.Y+b.H > level.Depth || !finite(m.HeightM) || !finite(m.ThicknessM) || m.ThicknessM <= 0 || m.HeightM <= m.ThicknessM {
			return nil, fmt.Errorf("architecture.wall_top_mass[%q]: unique ID, in-bounds positive envelope, finite height/thickness and reason required", m.ID)
		}
		seen[m.ID] = true
		covered := map[locationsdk.Cell]bool{}
		for c := range floor {
			covered[c] = true
		}
		for _, opening := range m.Openings {
			if opening.W <= 0 || opening.H <= 0 {
				return nil, fmt.Errorf("architecture.wall_top_mass[%q]: opening dimensions must be positive", m.ID)
			}
			for y := opening.Y; y < opening.Y+opening.H; y++ {
				for x := opening.X; x < opening.X+opening.W; x++ {
					covered[locationsdk.Cell{X: x, Y: y}] = true
				}
			}
		}
		role := m.MaterialRole
		if role == "" {
			role = a.ConcreteRole
		}
		if role == "" {
			role = "concrete." + a.WallCaste
		}
		count := 0
		// Stable maximal rectangles avoid one draw per half-metre cell.
		for y := b.Y; y < b.Y+b.H; y++ {
			for x := b.X; x < b.X+b.W; x++ {
				if covered[locationsdk.Cell{X: x, Y: y}] {
					continue
				}
				w := 1
				for x+w < b.X+b.W && !covered[locationsdk.Cell{X: x + w, Y: y}] {
					w++
				}
				h := 1
				for y+h < b.Y+b.H {
					full := true
					for dx := 0; dx < w; dx++ {
						if covered[locationsdk.Cell{X: x + dx, Y: y + h}] {
							full = false
							break
						}
					}
					if !full {
						break
					}
					h++
				}
				for dy := 0; dy < h; dy++ {
					for dx := 0; dx < w; dx++ {
						covered[locationsdk.Cell{X: x + dx, Y: y + dy}] = true
					}
				}
				a.Draws = append(a.Draws, ArchitectureDraw{ID: fmt.Sprintf("%s.cap.%03d", m.ID, count), Kind: "wall", MaterialRole: role, Position: [3]float64{(float64(x) + float64(w)/2) * unit, float64(level.Elevation)*unit + m.HeightM - m.ThicknessM/2, (float64(y) + float64(h)/2) * unit}, Size: [3]float64{float64(w) * unit, m.ThicknessM, float64(h) * unit}, Reason: m.Reason})
				count++
			}
		}
	}
	a.massResolved = true
	out := *f
	out.Architecture = &a
	return &out, nil
}
