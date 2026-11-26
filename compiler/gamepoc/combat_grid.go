package gamepoc

import (
	"encoding/json"
	"fmt"
)

func validateCombatGridCover(raw []byte) error {
	var c struct {
		Rules struct {
			Grid struct {
				CellMM   int    `json:"cell_mm"`
				Diagonal string `json:"diagonal"`
			} `json:"grid"`
			Cover struct {
				HalfPenalty float64 `json:"half_penalty"`
				FullPenalty float64 `json:"full_penalty"`
				CrouchBonus float64 `json:"crouch_bonus"`
				ProneBonus  float64 `json:"prone_bonus"`
				FlankBonus  float64 `json:"flank_bonus"`
				RearBonus   float64 `json:"rear_bonus"`
				ProbeCells  int     `json:"probe_cells"`
				Rays        []int   `json:"rays_mm"`
				HalfRays    int     `json:"half_rays"`
				FullRays    int     `json:"full_rays"`
			} `json:"cover"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return err
	}
	g, r := c.Rules.Grid, c.Rules.Cover
	if g.CellMM == 0 && r.FullPenalty == 0 {
		return nil
	}
	if g.CellMM != 1000 && g.CellMM != 1500 || g.Diagonal != "orthogonal" {
		return fmt.Errorf("combat grid must be metric and orthogonal")
	}
	for _, n := range []float64{r.HalfPenalty, r.FullPenalty, r.CrouchBonus, r.ProneBonus, r.FlankBonus, r.RearBonus} {
		if n < 0 || n > 1 {
			return fmt.Errorf("combat cover bonus outside 0..1")
		}
	}
	if r.FullPenalty < r.HalfPenalty || r.RearBonus < r.FlankBonus || r.ProneBonus < r.CrouchBonus || r.ProbeCells < 1 || r.ProbeCells > 3 || len(r.Rays) != 3 || r.HalfRays < 1 || r.FullRays <= r.HalfRays || r.FullRays > 3 {
		return fmt.Errorf("invalid combat cover classes")
	}
	for i, h := range r.Rays {
		if h < 1 || i > 0 && h <= r.Rays[i-1] {
			return fmt.Errorf("unordered combat cover rays")
		}
	}
	return nil
}
