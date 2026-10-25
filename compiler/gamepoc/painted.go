package gamepoc

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// PaintedScene maps an illustration onto an authoritative Life navigation plane.
// Pixels describe presentation only; actors, custody and visibility come from Go.
type PaintedPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}
type PaintedActor struct {
	ActorID        string              `json:"actor_id"`
	Image          string              `json:"image"`
	AnimationSheet string              `json:"animation_sheet,omitempty"`
	SourceRect     *ArtSceneSourceRect `json:"source_rect,omitempty"`
	FeetX          float64             `json:"feet_x"`
	FeetY          float64             `json:"feet_y"`
	Height         float64             `json:"height"`
}
type PaintedProp struct {
	ID         string              `json:"id"`
	StoreID    string              `json:"store_id"`
	Image      string              `json:"image"`
	SourceRect *ArtSceneSourceRect `json:"source_rect,omitempty"`
	At         Point               `json:"at"`
	Width      float64             `json:"width"`
	Height     float64             `json:"height"`
	FootX      float64             `json:"foot_x"`
	FootY      float64             `json:"foot_y"`
	Label      string              `json:"label"`
}
type PaintedOccluder struct {
	ID               string         `json:"id"`
	Polygon          []PaintedPoint `json:"polygon"`
	DepthY           float64        `json:"depth_y"`
	VisibilityAnchor *Point         `json:"visibility_anchor,omitempty"`
}

// Unity JsonUtility can materialize an omitted nested DTO as a zero-valued
// object. Emit presence independently; (0,0) is a legitimate authored cell.
func (o PaintedOccluder) MarshalJSON() ([]byte, error) {
	type wire PaintedOccluder
	return json.Marshal(struct {
		wire
		HasVisibilityAnchor bool `json:"has_visibility_anchor"`
	}{wire(o), o.VisibilityAnchor != nil})
}

// PaintedInspectionHotspot is artwork geometry only. Prose and inspection
// eligibility are projected separately by the authoritative server.
type PaintedInspectionHotspot struct {
	ObjectID string  `json:"object_id"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Radius   float64 `json:"radius"`
}
type PaintedScene struct {
	LevelID            string                     `json:"level_id"`
	Background         string                     `json:"background"`
	Width              int                        `json:"width"`
	Height             int                        `json:"height"`
	ViewWidth          int                        `json:"view_width,omitempty"`
	Origin             PaintedPoint               `json:"origin"`
	AxisX              PaintedPoint               `json:"axis_x"`
	AxisY              PaintedPoint               `json:"axis_y"`
	ActorHeight        float64                    `json:"actor_height"`
	KnownArchitecture  bool                       `json:"known_architecture"`
	Navigation         []string                   `json:"navigation"`
	ProofWaypoints     []Point                    `json:"proof_waypoints"`
	ProofBlocked       Point                      `json:"proof_blocked"`
	Actors             []PaintedActor             `json:"actors"`
	Props              []PaintedProp              `json:"props"`
	Occluders          []PaintedOccluder          `json:"occluders"`
	InspectionHotspots []PaintedInspectionHotspot `json:"inspection_hotspots,omitempty"`
	FloorElevationM    *float64                   `json:"floor_elevation_m,omitempty"`
	AmbientPassages    []PaintedAmbientPassage    `json:"ambient_passages,omitempty"`
	Daylight           *PaintedDaylight           `json:"daylight,omitempty"`
}

func (s PaintedScene) project(p Point) PaintedPoint {
	return PaintedPoint{s.Origin.X + s.AxisX.X*float64(p.X) + s.AxisY.X*float64(p.Y), s.Origin.Y + s.AxisX.Y*float64(p.X) + s.AxisY.Y*float64(p.Y)}
}
func finitePainted(v ...float64) bool {
	for _, n := range v {
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return false
		}
	}
	return true
}
func checkPaintedScenes(c Compiled) error {
	var life struct {
		Locations map[string]struct {
			Terrain []string `json:"terrain"`
		} `json:"locations"`
		Actors []struct {
			ID string `json:"id"`
		} `json:"actors"`
		Inspection struct {
			Objects []struct {
				ID    string `json:"id"`
				Level string `json:"level"`
			} `json:"objects"`
		} `json:"inspection"`
	}
	if len(c.Art.Painted) == 0 && len(c.PaintedInventoryAnchors) == 0 {
		return nil
	}
	if err := json.Unmarshal(c.Life, &life); err != nil {
		return fmt.Errorf("painted: Life catalog: %w", err)
	}
	inspectionLevels := make(map[string]string, len(life.Inspection.Objects))
	for _, obj := range life.Inspection.Objects {
		inspectionLevels[obj.ID] = obj.Level
	}
	if len(c.Art.Painted) > 32 {
		return fmt.Errorf("art.painted: at most 32 scenes")
	}
	actors := map[string]bool{}
	for _, a := range life.Actors {
		actors[a.ID] = true
	}
	seen := map[string]bool{}
	for _, s := range c.Art.Painted {
		if err := checkPaintedAmbient(s, c.Walkable); err != nil {
			return err
		}
		level, ok := life.Locations[s.LevelID]
		if !ok || seen[s.LevelID] || len(level.Terrain) == 0 {
			return fmt.Errorf("painted: duplicate or unknown Life level %q", s.LevelID)
		}
		seen[s.LevelID] = true
		if s.ViewWidth != 0 && (s.ViewWidth < 256 || s.ViewWidth > 4096) {
			return fmt.Errorf("painted %s: view_width must be zero or 256..4096", s.LevelID)
		}
		if s.Width < 1 || s.Width > 4096 || s.Height < 1 || s.Height > 4096 || !artSceneImagePath(s.Background) || !finitePainted(s.ActorHeight, s.Origin.X, s.Origin.Y, s.AxisX.X, s.AxisX.Y, s.AxisY.X, s.AxisY.Y) || s.ActorHeight < 20 || s.ActorHeight > 1024 || math.Abs(s.AxisX.X*s.AxisY.Y-s.AxisX.Y*s.AxisY.X) < .001 {
			return fmt.Errorf("painted %s: invalid canvas/projection", s.LevelID)
		}
		if len(s.Navigation) != len(level.Terrain) {
			return fmt.Errorf("painted %s: navigation differs from Go", s.LevelID)
		}
		stand := func(p Point) bool {
			return p.Y >= 0 && p.Y < len(level.Terrain) && p.X >= 0 && p.X < len(level.Terrain[p.Y]) && c.Walkable[string(level.Terrain[p.Y][p.X])]
		}
		for y, row := range s.Navigation {
			if row != level.Terrain[y] {
				return fmt.Errorf("painted %s: navigation differs from Go at row %d", s.LevelID, y)
			}
			for x := range len(row) {
				if stand(Point{x, y}) {
					p := s.project(Point{x, y})
					if p.X < 0 || p.Y < 0 || p.X >= float64(s.Width) || p.Y >= float64(s.Height) {
						return fmt.Errorf("painted %s: walkable cell outside artwork", s.LevelID)
					}
				}
			}
		}
		if len(s.ProofWaypoints) < 2 || s.ProofWaypoints[0] == s.ProofWaypoints[1] || stand(s.ProofBlocked) {
			return fmt.Errorf("painted %s: invalid movement proof anchors", s.LevelID)
		}
		for _, p := range s.ProofWaypoints {
			if !stand(p) {
				return fmt.Errorf("painted %s: blocked proof waypoint", s.LevelID)
			}
		}
		if len(s.Actors) > 512 || len(s.Props) > 512 || len(s.Occluders) > 32 || len(s.InspectionHotspots) > 512 {
			return fmt.Errorf("painted %s: too many presentation members", s.LevelID)
		}
		for _, hotspot := range s.InspectionHotspots {
			if strings.TrimSpace(hotspot.ObjectID) == "" || inspectionLevels[hotspot.ObjectID] != s.LevelID || !finitePainted(hotspot.X, hotspot.Y, hotspot.Radius) || hotspot.X < 0 || hotspot.Y < 0 || hotspot.X >= float64(s.Width) || hotspot.Y >= float64(s.Height) || hotspot.Radius <= 0 || hotspot.Radius > math.Max(float64(s.Width), float64(s.Height)) {
				return fmt.Errorf("painted %s: unbound/invalid inspection hotspot %q", s.LevelID, hotspot.ObjectID)
			}
		}
		bound := map[string]bool{}
		for _, a := range s.Actors {
			if !actors[a.ActorID] || bound[a.ActorID] || !artSceneImagePath(a.Image) || a.SourceRect == nil || !finitePainted(a.FeetX, a.FeetY, a.Height) || a.Height < 20 || a.Height > 1024 || a.FeetX < 0 || a.FeetX > float64(a.SourceRect.W) || a.FeetY < 0 || a.FeetY > float64(a.SourceRect.H) {
				return fmt.Errorf("painted %s: invalid actor binding %q", s.LevelID, a.ActorID)
			}
			bound[a.ActorID] = true
			if a.AnimationSheet != "" {
				if _, exists := c.Art.Characters.Sheets[a.AnimationSheet]; !exists {
					return fmt.Errorf("painted %s: actor %q references unknown animation sheet %q", s.LevelID, a.ActorID, a.AnimationSheet)
				}
			}
		}
		members := map[string]bool{}
		for _, p := range s.Props {
			at, placed := c.PaintedInventoryAnchors[s.LevelID][p.StoreID]
			if strings.TrimSpace(p.ID) == "" || members[p.ID] || !placed || p.At.X != at.X || p.At.Y != at.Y || !artSceneImagePath(p.Image) || !finitePainted(p.Width, p.Height, p.FootX, p.FootY) || p.Width <= 0 || p.Width > 1024 || p.Height <= 0 || p.Height > 1024 || p.FootX < 0 || p.FootX > p.Width || p.FootY < 0 || p.FootY > p.Height {
				return fmt.Errorf("painted %s: unbound/invalid storage prop %q", s.LevelID, p.ID)
			}
			members[p.ID] = true
		}
		area := 0.0
		for _, o := range s.Occluders {
			if o.ID == "" || members[o.ID] || len(o.Polygon) < 3 || len(o.Polygon) > 64 || !finitePainted(o.DepthY) || o.DepthY < 0 || o.DepthY > float64(s.Height) {
				return fmt.Errorf("painted %s: invalid foreground %q", s.LevelID, o.ID)
			}
			if o.VisibilityAnchor != nil && !stand(*o.VisibilityAnchor) {
				return fmt.Errorf("painted %s: foreground %q visibility_anchor must be a walkable cell", s.LevelID, o.ID)
			}
			members[o.ID] = true
			minX, minY, maxX, maxY := float64(s.Width), float64(s.Height), 0.0, 0.0
			for _, p := range o.Polygon {
				if !finitePainted(p.X, p.Y) || p.X < 0 || p.Y < 0 || p.X > float64(s.Width) || p.Y > float64(s.Height) {
					return fmt.Errorf("painted %s: foreground outside canvas", s.LevelID)
				}
				minX = math.Min(minX, p.X)
				minY = math.Min(minY, p.Y)
				maxX = math.Max(maxX, p.X)
				maxY = math.Max(maxY, p.Y)
			}
			area += (maxX - minX) * (maxY - minY)
		}
		if area > 4*float64(s.Width*s.Height) {
			return fmt.Errorf("painted %s: foreground texture budget exceeded", s.LevelID)
		}
	}
	for id, stores := range c.PaintedInventoryAnchors {
		level, ok := life.Locations[id]
		if !ok || !seen[id] {
			return fmt.Errorf("painted inventory: unknown scene %s", id)
		}
		for store, p := range stores {
			st, ok := c.Inventory.Stores[store]
			if !ok || st.Anchor.Kind != "world" || st.Anchor.Level != id || p.X < 0 || p.Y < 0 || p.Y >= len(level.Terrain) || p.X >= len(level.Terrain[p.Y]) {
				return fmt.Errorf("painted inventory: invalid static store %s", store)
			}
			reachable := false
			for _, d := range []Point{{0, 0}, {0, 1}, {1, 0}, {0, -1}, {-1, 0}} {
				x, y := p.X+d.X, p.Y+d.Y
				if y >= 0 && y < len(level.Terrain) && x >= 0 && x < len(level.Terrain[y]) && c.Walkable[string(level.Terrain[y][x])] {
					reachable = true
				}
			}
			if !reachable {
				return fmt.Errorf("painted inventory: inaccessible store %s", store)
			}
		}
	}
	return nil
}

func checkPaintedImages(unity string, scenes []PaintedScene) error {
	// Reuse the PNG/crop checks without treating the presentation as a fake actor list.
	for _, s := range scenes {
		preview := ArtScene{ID: s.LevelID, Title: s.LevelID, Width: s.Width, Height: s.Height, Background: s.Background}
		for i, a := range s.Actors {
			preview.Layers = append(preview.Layers, ArtSceneLayer{ID: fmt.Sprintf("actor.%d", i), Image: a.Image, Width: 1, Height: 1, SourceRect: a.SourceRect})
		}
		for i, p := range s.Props {
			preview.Layers = append(preview.Layers, ArtSceneLayer{ID: fmt.Sprintf("prop.%d", i), Image: p.Image, Width: 1, Height: 1, SourceRect: p.SourceRect})
		}
		for i, p := range s.AmbientPassages {
			preview.Layers = append(preview.Layers, ArtSceneLayer{ID: fmt.Sprintf("ambient.%d", i), Image: p.Image, Width: 1, Height: 1, SourceRect: p.SourceRect})
			if p.Track != nil {
				preview.Layers = append(preview.Layers, ArtSceneLayer{ID: fmt.Sprintf("ambient.track.%d", i), Image: p.Track.Image, Width: 1, Height: 1, SourceRect: p.Track.SourceRect})
			}
		}
		if err := checkArtSceneImages(unity, []ArtScene{preview}); err != nil {
			return err
		}
	}
	return nil
}
