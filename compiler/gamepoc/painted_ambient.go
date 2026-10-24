package gamepoc

import (
	"fmt"
	"math"
	"strings"
)

// Ambient passages are public architectural presentation, never Life vehicles,
// actors, travel edges, collisions, passenger seats, or live surveillance data.
type PaintedAmbientTrack struct {
	Image      string              `json:"image"`
	SourceRect *ArtSceneSourceRect `json:"source_rect,omitempty"`
	X          float64             `json:"x"`
	Y          float64             `json:"y"`
	Width      float64             `json:"width"`
	Height     float64             `json:"height"`
}

type PaintedAmbientPassage struct {
	ID                 string               `json:"id"`
	Image              string               `json:"image"`
	SourceRect         *ArtSceneSourceRect  `json:"source_rect,omitempty"`
	Width              float64              `json:"width"`
	Height             float64              `json:"height"`
	FeetX              float64              `json:"feet_x"`
	FeetY              float64              `json:"feet_y"`
	Path               []PaintedPoint       `json:"path"`
	VisibleStrip       []PaintedPoint       `json:"visible_strip"`
	ObservationCells   []Point              `json:"observation_cells"`
	PeriodSeconds      int64                `json:"period_seconds"`
	NightPeriodSeconds int64                `json:"night_period_seconds,omitempty"`
	DayStartSecond     int64                `json:"day_start_second,omitempty"`
	DayEndSecond       int64                `json:"day_end_second,omitempty"`
	DurationSeconds    int64                `json:"duration_seconds"`
	OffsetSeconds      int64                `json:"offset_seconds"`
	ElevationM         float64              `json:"elevation_m"`
	Track              *PaintedAmbientTrack `json:"track,omitempty"`
}

type PaintedDaylight struct {
	Source          string  `json:"source"`
	SunriseSecond   int64   `json:"sunrise_second"`
	SunsetSecond    int64   `json:"sunset_second"`
	TwilightSeconds int64   `json:"twilight_seconds"`
	DayBrightness   float64 `json:"day_brightness"`
	NightBrightness float64 `json:"night_brightness"`
}

func ambientCross(a, b, c PaintedPoint) float64 { return (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X) }
func ambientOnEdge(a, b, p PaintedPoint) bool {
	return math.Abs(ambientCross(a, b, p)) < 1e-7 && p.X >= math.Min(a.X, b.X)-1e-7 && p.X <= math.Max(a.X, b.X)+1e-7 && p.Y >= math.Min(a.Y, b.Y)-1e-7 && p.Y <= math.Max(a.Y, b.Y)+1e-7
}
func ambientEdgesCross(a, b, c, d PaintedPoint) bool {
	x, y, u, v := ambientCross(a, b, c), ambientCross(a, b, d), ambientCross(c, d, a), ambientCross(c, d, b)
	return ((x > 1e-7 && y < -1e-7) || (x < -1e-7 && y > 1e-7)) && ((u > 1e-7 && v < -1e-7) || (u < -1e-7 && v > 1e-7))
}
func ambientInside(poly []PaintedPoint, p PaintedPoint, strict bool) bool {
	inside := false
	for i, a := range poly {
		b := poly[(i+1)%len(poly)]
		if ambientOnEdge(a, b, p) {
			return !strict
		}
		if (a.Y > p.Y) != (b.Y > p.Y) && p.X < (b.X-a.X)*(p.Y-a.Y)/(b.Y-a.Y)+a.X {
			inside = !inside
		}
	}
	return inside
}
func ambientPolygonsOverlap(a, b []PaintedPoint) bool {
	for _, p := range a {
		if ambientInside(b, p, true) {
			return true
		}
	}
	for _, p := range b {
		if ambientInside(a, p, true) {
			return true
		}
	}
	for i, p := range a {
		q := a[(i+1)%len(a)]
		for j, u := range b {
			if ambientEdgesCross(p, q, u, b[(j+1)%len(b)]) {
				return true
			}
		}
	}
	// Coincident boundaries can still enclose the same positive area.
	for i, p := range a {
		q := a[(i+1)%len(a)]
		mid := PaintedPoint{(p.X + q.X) / 2, (p.Y + q.Y) / 2}
		if ambientInside(b, mid, true) {
			return true
		}
	}
	center := PaintedPoint{}
	for _, p := range b {
		center.X += p.X / float64(len(b))
		center.Y += p.Y / float64(len(b))
	}
	return ambientInside(a, center, true)
}
func ambientImage(image string, r *ArtSceneSourceRect) bool {
	return artSceneImagePath(image) && (r == nil || (r.X >= 0 && r.Y >= 0 && r.W > 0 && r.H > 0 && r.X <= 4096-r.W && r.Y <= 4096-r.H))
}
func checkPaintedAmbient(s PaintedScene, walkable map[string]bool) error {
	if len(s.AmbientPassages) > 4 {
		return fmt.Errorf("painted %s: at most four ambient passages", s.LevelID)
	}
	if d := s.Daylight; d != nil {
		if d.Source != "passive_lightguides" || d.SunriseSecond < 0 || d.SunsetSecond <= d.SunriseSecond || d.SunsetSecond > 86400 || d.TwilightSeconds < 0 || d.TwilightSeconds > 7200 || d.SunriseSecond*2 < d.TwilightSeconds || (86400-d.SunsetSecond)*2 < d.TwilightSeconds || d.TwilightSeconds > d.SunsetSecond-d.SunriseSecond || !finitePainted(d.DayBrightness, d.NightBrightness) || d.NightBrightness <= 0 || d.DayBrightness < d.NightBrightness || d.DayBrightness > 1 {
			return fmt.Errorf("painted %s: invalid passive daylight profile", s.LevelID)
		}
	}
	ids := map[string]bool{}
	for _, p := range s.AmbientPassages {
		bad := func(why string) error { return fmt.Errorf("painted %s ambient %s: %s", s.LevelID, p.ID, why) }
		if strings.TrimSpace(p.ID) == "" || ids[p.ID] {
			return bad("empty/duplicate identity")
		}
		ids[p.ID] = true
		if s.FloorElevationM == nil || !finitePainted(*s.FloorElevationM, p.ElevationM) || p.ElevationM > *s.FloorElevationM {
			return bad("passage elevation must be explicitly at or below the floor")
		}
		if !ambientImage(p.Image, p.SourceRect) || !finitePainted(p.Width, p.Height, p.FeetX, p.FeetY) || p.Width <= 0 || p.Height <= 0 || p.Width > 4096 || p.Height > 4096 || p.FeetX < 0 || p.FeetX > p.Width || p.FeetY < 0 || p.FeetY > p.Height {
			return bad("invalid sprite/image feet")
		}
		if p.PeriodSeconds < 1 || p.PeriodSeconds > 86400 || p.DurationSeconds < 1 || p.DurationSeconds > p.PeriodSeconds || p.OffsetSeconds < 0 || p.OffsetSeconds >= p.PeriodSeconds {
			return bad("invalid authoritative schedule")
		}
		if p.NightPeriodSeconds != 0 || p.DayStartSecond != 0 || p.DayEndSecond != 0 {
			if p.NightPeriodSeconds < 1 || p.NightPeriodSeconds > 86400 || p.DurationSeconds > p.NightPeriodSeconds || p.OffsetSeconds >= p.NightPeriodSeconds || p.DayStartSecond < 0 || p.DayEndSecond <= p.DayStartSecond || p.DayEndSecond >= 86400 {
				return bad("invalid day/night authoritative schedule")
			}
		}
		if len(p.Path) < 2 || len(p.Path) > 64 {
			return bad("path needs two to 64 points")
		}
		length := 0.0
		for i, at := range p.Path {
			if !finitePainted(at.X, at.Y) || at.X < -2*float64(s.Width) || at.X > 3*float64(s.Width) || at.Y < -2*float64(s.Height) || at.Y > 3*float64(s.Height) {
				return bad("invalid path point")
			}
			if i > 0 {
				length += math.Hypot(at.X-p.Path[i-1].X, at.Y-p.Path[i-1].Y)
			}
		}
		if length < 1 {
			return bad("empty movement path")
		}
		if len(p.VisibleStrip) < 3 || len(p.VisibleStrip) > 64 {
			return bad("visible strip needs three to 64 points")
		}
		area := 0.0
		for i, a := range p.VisibleStrip {
			b := p.VisibleStrip[(i+1)%len(p.VisibleStrip)]
			if !finitePainted(a.X, a.Y) || a.X < 0 || a.Y < 0 || a.X > float64(s.Width) || a.Y > float64(s.Height) || math.Hypot(a.X-b.X, a.Y-b.Y) < .01 {
				return bad("invalid visible strip boundary")
			}
			area += a.X*b.Y - a.Y*b.X
			for j, c := range p.VisibleStrip {
				if j <= i+1 || (i == 0 && j == len(p.VisibleStrip)-1) {
					continue
				}
				if ambientEdgesCross(a, b, c, p.VisibleStrip[(j+1)%len(p.VisibleStrip)]) {
					return bad("self-crossing visible strip")
				}
			}
		}
		if math.Abs(area) < 2 {
			return bad("empty visible strip")
		}
		if len(p.ObservationCells) < 1 || len(p.ObservationCells) > 128 {
			return bad("observation cells required")
		}
		observed := map[Point]bool{}
		for _, at := range p.ObservationCells {
			if observed[at] || at.Y < 0 || at.Y >= len(s.Navigation) || at.X < 0 || at.X >= len(s.Navigation[at.Y]) || !walkable[string(s.Navigation[at.Y][at.X])] {
				return bad("invalid platform observation cell")
			}
			observed[at] = true
		}
		for y, row := range s.Navigation {
			for x := range row {
				if !walkable[string(row[x])] {
					continue
				}
				floor := []PaintedPoint{
					{s.Origin.X + s.AxisX.X*(float64(x)-.5) + s.AxisY.X*(float64(y)-.5), s.Origin.Y + s.AxisX.Y*(float64(x)-.5) + s.AxisY.Y*(float64(y)-.5)},
					{s.Origin.X + s.AxisX.X*(float64(x)+.5) + s.AxisY.X*(float64(y)-.5), s.Origin.Y + s.AxisX.Y*(float64(x)+.5) + s.AxisY.Y*(float64(y)-.5)},
					{s.Origin.X + s.AxisX.X*(float64(x)+.5) + s.AxisY.X*(float64(y)+.5), s.Origin.Y + s.AxisX.Y*(float64(x)+.5) + s.AxisY.Y*(float64(y)+.5)},
					{s.Origin.X + s.AxisX.X*(float64(x)-.5) + s.AxisY.X*(float64(y)+.5), s.Origin.Y + s.AxisX.Y*(float64(x)-.5) + s.AxisY.Y*(float64(y)+.5)},
				}
				if ambientPolygonsOverlap(p.VisibleStrip, floor) {
					return bad(fmt.Sprintf("visible train strip intersects playable floor (%d,%d)", x, y))
				}
			}
		}
		if t := p.Track; t != nil {
			if !ambientImage(t.Image, t.SourceRect) || !finitePainted(t.X, t.Y, t.Width, t.Height) || t.Width <= 0 || t.Height <= 0 || t.Width > 16384 || t.Height > 16384 {
				return bad("invalid static track overlay")
			}
		}
	}
	return nil
}
