package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"regexp"

	"cuelang.org/go/cue"
)

// AccessZone is world.access_zones (owner 13.11): a zone of a level only some people may enter, a polygon in plan metres
// beside the turret post that guards it. Its floor marking (an inlaid steel strip with light hatching along the marked
// edges) is drawn by the client from this data; nobody places it by hand in a scene. The access check (a scan as before a
// medical capsule, a countdown, the turrets' fire) is the next step: until its runtime exists, `fire` must be "never".
type AccessZone struct {
	ID        string       `json:"id"`
	LevelID   string       `json:"level_id"`
	PolygonM  [][2]float64 `json:"polygon_m"`
	Marked    []bool       `json:"marked"`
	GuardPost string       `json:"guard_post"`
	Turrets   []string     `json:"turrets"`
	Access    struct {
		Castes         []string `json:"castes"`
		PassItems      []string `json:"pass_items"`
		EscortByGuards bool     `json:"escort_by_guards"`
	} `json:"access"`
	ScanSeconds      float64           `json:"scan_seconds"`
	CountdownSeconds float64           `json:"countdown_seconds"`
	Fire             string            `json:"fire"`
	Marking          AccessZoneMarking `json:"marking"`
	Why              string            `json:"why"`
}

// AccessZoneMarking is presentation only: the strip and hatching drawn on the floor along the marked edges (metres).
type AccessZoneMarking struct {
	StripWidthM   float64 `json:"strip_width_m"`
	HatchWidthM   float64 `json:"hatch_width_m"`
	HatchBarM     float64 `json:"hatch_bar_m"`
	HatchSpacingM float64 `json:"hatch_spacing_m"`
	HatchAngleDeg float64 `json:"hatch_angle_deg"`
	SteelColor    string  `json:"steel_color"`
	PaintColor    string  `json:"paint_color"`
}

var accessZoneID = regexp.MustCompile(`^zone\.[a-z0-9_.]+$`)

func readAccessZones(world cue.Value, c Compiled) ([]AccessZone, error) {
	v := world.LookupPath(cue.ParsePath("access_zones"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.access_zones: %w", err)
	}
	var zones []AccessZone
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&zones); err != nil {
		return nil, fmt.Errorf("world.access_zones: %w", err)
	}
	var life struct {
		Locations map[string]struct {
			Terrain []string `json:"terrain"`
			CellMM  int      `json:"cell_mm"`
		} `json:"locations"`
	}
	if err := json.Unmarshal(c.Life, &life); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, z := range zones {
		loc, ok := life.Locations[z.LevelID]
		if err := checkAccessZone(z, ok, loc.Terrain, loc.CellMM, c.TurretWatch, seen); err != nil {
			return nil, fmt.Errorf("world.access_zones %s: %w", z.ID, err)
		}
	}
	return zones, nil
}

func checkAccessZone(z AccessZone, levelKnown bool, terrain []string, cellMM int, turrets *TurretWatchCatalog, seen map[string]bool) error {
	if !accessZoneID.MatchString(z.ID) || seen[z.ID] {
		return fmt.Errorf("the id must be zone.<name> and unique")
	}
	seen[z.ID] = true
	if !levelKnown || len(terrain) == 0 {
		return fmt.Errorf("unknown level %s", z.LevelID)
	}
	if cellMM <= 0 {
		cellMM = 1000
	}
	n := len(z.PolygonM)
	if n < 3 || len(z.Marked) != n {
		return fmt.Errorf("a polygon of at least 3 vertices with one `marked` per edge")
	}
	width, depth := float64(len(terrain[0])*cellMM)/1000, float64(len(terrain)*cellMM)/1000
	area, marked := 0.0, false
	for i, p := range z.PolygonM {
		q := z.PolygonM[(i+1)%n]
		if math.IsNaN(p[0]) || math.IsNaN(p[1]) || p[0] < 0 || p[1] < 0 || p[0] > width || p[1] > depth {
			return fmt.Errorf("vertex %d is outside the level", i)
		}
		if p == q {
			return fmt.Errorf("edge %d has no length", i)
		}
		area += p[0]*q[1] - q[0]*p[1]
		marked = marked || z.Marked[i]
		for j := i + 2; j < n; j++ {
			if i == 0 && j == n-1 {
				continue // adjacent through the closing edge
			}
			if accessSegmentsCross(p, q, z.PolygonM[j], z.PolygonM[(j+1)%n]) {
				return fmt.Errorf("edges %d and %d cross: the polygon must be simple", i, j)
			}
		}
	}
	if math.Abs(area) < 1e-6 {
		return fmt.Errorf("the polygon has no area")
	}
	if !marked {
		return fmt.Errorf("no edge is marked: a zone nobody can see on the floor is not fair")
	}
	if turrets == nil || len(z.Turrets) == 0 {
		return fmt.Errorf("a zone is guarded by the turret posts of world.turret_watch")
	}
	for _, id := range z.Turrets {
		found := false
		for _, p := range turrets.Posts {
			if p.ID == id {
				found = p.LevelID == z.LevelID && p.GuardPost == z.GuardPost
				for _, v := range z.PolygonM {
					if math.Hypot(v[0]-p.Object.XM, v[1]-p.Object.YM) > turrets.Rule.RangeM {
						return fmt.Errorf("vertex %v is beyond the watch range of turret %s", v, id)
					}
				}
			}
		}
		if !found {
			return fmt.Errorf("turret %s is not a post of %s at %s", id, z.GuardPost, z.LevelID)
		}
	}
	if len(z.Access.Castes) == 0 && len(z.Access.PassItems) == 0 && !z.Access.EscortByGuards {
		return fmt.Errorf("nobody may enter: give a caste, a pass item or the guards' escort")
	}
	for _, caste := range z.Access.Castes {
		if !lookCastes[caste] {
			return fmt.Errorf("unknown caste %q", caste)
		}
	}
	// The turrets never fire on approach (transport_hub_design turret contracts): fire only after a failed scan and a
	// countdown. The runtime of the scan and the fire is not generated yet, so a zone may not ask for it.
	switch z.Fire {
	case "never":
	case "after_scan_and_countdown":
		if z.ScanSeconds <= 0 || z.CountdownSeconds < 5 {
			return fmt.Errorf("fire needs a scan and a countdown of at least 5 s: never on approach")
		}
		return fmt.Errorf("fire after the scan and the countdown is not generated yet (the zone access runtime): keep fire \"never\"")
	default:
		return fmt.Errorf("fire must be never or after_scan_and_countdown, not %q", z.Fire)
	}
	if z.ScanSeconds <= 0 || z.ScanSeconds > 10 || z.CountdownSeconds < 5 || z.CountdownSeconds > 60 {
		return fmt.Errorf("scan 0..10 s and countdown 5..60 s")
	}
	m := z.Marking
	if m.StripWidthM <= 0 || m.HatchWidthM <= m.StripWidthM || m.HatchBarM <= 0 || m.HatchSpacingM <= m.HatchBarM || m.HatchAngleDeg < 15 || m.HatchAngleDeg > 75 ||
		!turretWatchColor.MatchString(m.SteelColor) || !turretWatchColor.MatchString(m.PaintColor) {
		return fmt.Errorf("invalid floor marking")
	}
	return nil
}

func accessCross(o, a, b [2]float64) float64 {
	return (a[0]-o[0])*(b[1]-o[1]) - (a[1]-o[1])*(b[0]-o[0])
}

// accessSegmentsCross: the two segments share a point (proper crossing or touching).
func accessSegmentsCross(p, q, r, s [2]float64) bool {
	d1, d2, d3, d4 := accessCross(r, s, p), accessCross(r, s, q), accessCross(p, q, r), accessCross(p, q, s)
	if ((d1 > 0 && d2 < 0) || (d1 < 0 && d2 > 0)) && ((d3 > 0 && d4 < 0) || (d3 < 0 && d4 > 0)) {
		return true
	}
	on := func(a, b, c [2]float64, d float64) bool {
		return d == 0 && math.Min(a[0], b[0]) <= c[0] && c[0] <= math.Max(a[0], b[0]) && math.Min(a[1], b[1]) <= c[1] && c[1] <= math.Max(a[1], b[1])
	}
	return on(r, s, p, d1) || on(r, s, q, d2) || on(p, q, r, d3) || on(p, q, s, d4)
}

// clientAccessZones: what the client draws — the polygon, which edges are marked and the marking's look.
func clientAccessZones(zones []AccessZone) []map[string]any {
	out := make([]map[string]any, 0, len(zones))
	for _, z := range zones {
		out = append(out, map[string]any{"id": z.ID, "level_id": z.LevelID, "polygon_m": z.PolygonM, "marked": z.Marked, "marking": z.Marking})
	}
	return out
}
