package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"

	"cuelang.org/go/cue"
)

// TurretWatchCatalog is world.turret_watch (turrets v1, owner 11.11): when anyone who is not a guard comes
// closer to the guarded object than the rule's range, a turret turns to them; of several, the one nearest
// to the object. Watching only: no fire, alarm or law. The generated server picks the target and the aim
// (deterministic, from the saved Life state); the client turns the model's own nodes at the same rates.
// Turret state exists only on the hero's level (detailed where the player is, nothing elsewhere).
type TurretWatchCatalog struct {
	Rule  TurretWatchRule   `json:"rule"`
	Look  TurretWatchLook   `json:"look"`
	Posts []TurretWatchPost `json:"posts"`
}

type TurretWatchRule struct {
	RangeM          float64 `json:"range_m"`
	HoldMS          int64   `json:"hold_ms"`
	SwitchMarginM   float64 `json:"switch_margin_m"`
	AimHeightM      float64 `json:"aim_height_m"`
	PanSlewDegS     float64 `json:"pan_slew_deg_s"`
	TiltSlewDegS    float64 `json:"tilt_slew_deg_s"`
	ReturnSlewDegS  float64 `json:"return_slew_deg_s"`
	PanHalfRangeDeg float64 `json:"pan_half_range_deg"`
	TiltMinDeg      float64 `json:"tilt_min_deg"`
	TiltMaxDeg      float64 `json:"tilt_max_deg"`
	EyeRiseMS       int64   `json:"eye_rise_ms"`
	EyeFallMS       int64   `json:"eye_fall_ms"`
}

// TurretWatchLook is presentation only (Godot eye material and its small light).
type TurretWatchLook struct {
	EyeRestEnergy  float64 `json:"eye_rest_energy"`
	EyeWatchEnergy float64 `json:"eye_watch_energy"`
	EyeLightEnergy float64 `json:"eye_light_energy"`
	EyeLightRangeM float64 `json:"eye_light_range_m"`
	EyeLightColor  string  `json:"eye_light_color"`
}

type TurretWatchPost struct {
	ID         string `json:"id"`
	LevelID    string `json:"level_id"`
	InstanceID string `json:"instance_id"`
	GuardPost  string `json:"guard_post"`
	StaffLabel string `json:"staff_label"`
	// Derived in CUE from the Life schedules (whoever has the post's duty), never a name list in code.
	Guards []string `json:"guards"`
	Object struct {
		ID string  `json:"id"`
		XM float64 `json:"x_m"`
		YM float64 `json:"y_m"`
	} `json:"object"`
	PivotM    []float64 `json:"pivot_m"`
	FacingDeg float64   `json:"facing_deg"`
	RestAimM  []float64 `json:"rest_aim_m"`
	PanNode   string    `json:"pan_node"`
	TiltNode  string    `json:"tilt_node"`
	EyeNodes  []string  `json:"eye_nodes"`
	// Derived here, never authored: the rest aim as a plan heading and a pitch (millidegrees).
	RestHeadingMdeg int64 `json:"rest_heading_mdeg"`
	RestPitchMdeg   int64 `json:"rest_pitch_mdeg"`
}

var turretWatchColor = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

func readTurretWatch(world cue.Value, life json.RawMessage) (*TurretWatchCatalog, error) {
	v := world.LookupPath(cue.ParsePath("turret_watch"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.turret_watch: %w", err)
	}
	var c TurretWatchCatalog
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err = d.Decode(&c); err != nil {
		return nil, fmt.Errorf("world.turret_watch: %w", err)
	}
	if err = c.validate(life); err != nil {
		return nil, fmt.Errorf("world.turret_watch: %w", err)
	}
	return &c, nil
}

// turretWatchMdeg rounds degrees to whole millidegrees, the unit of the server's aim.
func turretWatchMdeg(deg float64) int64 { return int64(math.Round(deg * 1000)) }

// turretWatchWrap folds an angle in millidegrees into (-180000, 180000].
func turretWatchWrap(mdeg int64) int64 {
	mdeg %= 360000
	if mdeg <= -180000 {
		mdeg += 360000
	}
	if mdeg > 180000 {
		mdeg -= 360000
	}
	return mdeg
}

func (c *TurretWatchCatalog) validate(life json.RawMessage) error {
	finite := func(values ...float64) bool {
		for _, v := range values {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return false
			}
		}
		return true
	}
	r := c.Rule
	if !finite(r.RangeM, r.SwitchMarginM, r.AimHeightM, r.PanSlewDegS, r.TiltSlewDegS, r.ReturnSlewDegS, r.PanHalfRangeDeg, r.TiltMinDeg, r.TiltMaxDeg) {
		return fmt.Errorf("rule numbers must be finite")
	}
	if r.RangeM < 1 || r.RangeM > 30 || r.HoldMS < 0 || r.HoldMS > 30000 || r.SwitchMarginM < 0 || r.SwitchMarginM > 3 || r.AimHeightM < .3 || r.AimHeightM > 2 {
		return fmt.Errorf("rule range 1..30 m, hold 0..30 s, switch margin 0..3 m and aim height 0.3..2 m required")
	}
	for _, rate := range []float64{r.PanSlewDegS, r.TiltSlewDegS, r.ReturnSlewDegS} {
		if rate <= 0 || rate > 720 {
			return fmt.Errorf("slew rates must be 0..720 degrees per second")
		}
	}
	if r.PanHalfRangeDeg <= 0 || r.PanHalfRangeDeg > 180 || r.TiltMinDeg < -90 || r.TiltMinDeg >= 0 || r.TiltMaxDeg <= 0 || r.TiltMaxDeg > 60 {
		return fmt.Errorf("pan half range 0..180 and tilt -90..0..60 degrees required")
	}
	if r.EyeRiseMS <= 0 || r.EyeRiseMS > 10000 || r.EyeFallMS <= 0 || r.EyeFallMS > 10000 {
		return fmt.Errorf("eye fades of 1..10000 ms required")
	}
	l := c.Look
	if !finite(l.EyeRestEnergy, l.EyeWatchEnergy, l.EyeLightEnergy, l.EyeLightRangeM) || l.EyeRestEnergy < 0 || l.EyeWatchEnergy <= l.EyeRestEnergy || l.EyeWatchEnergy > 16 || l.EyeLightEnergy < 0 || l.EyeLightEnergy > 8 || l.EyeLightRangeM <= 0 || l.EyeLightRangeM > 4 || !turretWatchColor.MatchString(l.EyeLightColor) {
		return fmt.Errorf("look: eye energies rest < watch <= 16, light 0..8 within 0..4 m and a #RRGGBB colour required")
	}
	var m struct {
		Locations map[string]struct {
			CellMM  int      `json:"cell_mm"`
			Terrain []string `json:"terrain"`
		} `json:"locations"`
		Actors []struct {
			ID    string `json:"id"`
			Steps []struct {
				Level string `json:"level"`
				Label string `json:"label"`
			} `json:"steps"`
		} `json:"actors"`
	}
	if err := json.Unmarshal(life, &m); err != nil {
		return fmt.Errorf("Life: %w", err)
	}
	ids, instances := map[string]bool{}, map[string]bool{}
	for i := range c.Posts {
		p := &c.Posts[i]
		if p.ID == "" || ids[p.ID] || p.InstanceID == "" || instances[p.LevelID+"/"+p.InstanceID] {
			return fmt.Errorf("post %d: unique id and instance required", i)
		}
		ids[p.ID], instances[p.LevelID+"/"+p.InstanceID] = true, true
		level, ok := m.Locations[p.LevelID]
		if !ok || len(level.Terrain) == 0 || level.CellMM <= 0 {
			return fmt.Errorf("%s: unknown Life level %q or one without a cell size", p.ID, p.LevelID)
		}
		cell := float64(level.CellMM) / 1000
		inside := func(x, y float64) bool {
			return finite(x, y) && x >= 0 && y >= 0 && int(x/cell) < len(level.Terrain[0]) && int(y/cell) < len(level.Terrain)
		}
		if len(p.PivotM) != 3 || len(p.RestAimM) != 3 || !finite(p.PivotM[2], p.RestAimM[2], p.FacingDeg) || !inside(p.PivotM[0], p.PivotM[1]) || !inside(p.RestAimM[0], p.RestAimM[1]) || !inside(p.Object.XM, p.Object.YM) || p.Object.ID == "" {
			return fmt.Errorf("%s: pivot, rest aim and object must be finite plan points inside the level", p.ID)
		}
		if p.PivotM[2] <= r.AimHeightM || p.PivotM[2] > 20 || p.FacingDeg < -180 || p.FacingDeg > 180 {
			return fmt.Errorf("%s: the turret must hang above the people it watches", p.ID)
		}
		if math.Hypot(p.Object.XM-p.PivotM[0], p.Object.YM-p.PivotM[1]) > r.RangeM {
			return fmt.Errorf("%s: the guarded object lies outside the turret's range", p.ID)
		}
		names := map[string]bool{}
		for _, n := range append([]string{p.PanNode, p.TiltNode}, p.EyeNodes...) {
			if strings.TrimSpace(n) == "" || names[n] || len(n) > 128 {
				return fmt.Errorf("%s: pan, tilt and eye node names must be unique and non-empty", p.ID)
			}
			names[n] = true
		}
		if len(p.EyeNodes) == 0 || len(p.EyeNodes) > 8 {
			return fmt.Errorf("%s: 1..8 eye nodes required", p.ID)
		}
		if p.GuardPost == "" || p.StaffLabel == "" || len(p.Guards) == 0 {
			return fmt.Errorf("%s: a guard post with its staff is required", p.ID)
		}
		seen := map[string]bool{}
		for _, g := range p.Guards {
			if seen[g] {
				return fmt.Errorf("%s: guard %s listed twice", p.ID, g)
			}
			seen[g] = true
			staff := false
			for _, a := range m.Actors {
				if a.ID != g {
					continue
				}
				for _, s := range a.Steps {
					staff = staff || (s.Label == p.StaffLabel && s.Level == p.LevelID)
				}
			}
			if !staff {
				return fmt.Errorf("%s: guard %s has no %s duty on %s in Life", p.ID, g, p.StaffLabel, p.LevelID)
			}
		}
		dx, dy, dh := p.RestAimM[0]-p.PivotM[0], p.RestAimM[1]-p.PivotM[1], p.RestAimM[2]-p.PivotM[2]
		if math.Hypot(dx, dy) < .1 {
			return fmt.Errorf("%s: the rest aim must lie away from the mount", p.ID)
		}
		p.RestHeadingMdeg = turretWatchWrap(turretWatchMdeg(math.Atan2(dy, dx) * 180 / math.Pi))
		p.RestPitchMdeg = turretWatchMdeg(math.Atan2(dh, math.Hypot(dx, dy)) * 180 / math.Pi)
		rel := turretWatchWrap(p.RestHeadingMdeg - turretWatchMdeg(p.FacingDeg))
		if rel < -turretWatchMdeg(r.PanHalfRangeDeg) || rel > turretWatchMdeg(r.PanHalfRangeDeg) || p.RestPitchMdeg < turretWatchMdeg(r.TiltMinDeg) || p.RestPitchMdeg > turretWatchMdeg(r.TiltMaxDeg) {
			return fmt.Errorf("%s: the rest aim lies outside the turret's pan or tilt limits", p.ID)
		}
	}
	return nil
}
