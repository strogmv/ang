package gamepoc

import (
	"encoding/json"
	"fmt"
	"regexp"
)

// WayfindingCatalog is private authoring. The wire projection is restricted to
// remembered current-room architecture; Unity never receives this catalog.
type WayfindingCatalog struct {
	Destinations []WayfindingDestination `json:"destinations"`
}
type WayfindingCell struct {
	X int `json:"x"`
	Y int `json:"y"`
}
type WayfindingDestination struct {
	MarkerNumber int             `json:"marker_number,omitempty"`
	ID           string          `json:"id"`
	NameKey      string          `json:"name_key"`
	Level        string          `json:"level"`
	Color        string          `json:"color"`
	Kind         string          `json:"kind"`
	At           WayfindingCell  `json:"at"`
	GuideFrom    *WayfindingCell `json:"guide_from,omitempty"`
	EdgeID       string          `json:"edge_id,omitempty"`
}

var wayfindingColorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

func (c WayfindingCatalog) validate(life json.RawMessage, walkable map[string]bool, texts LocalizationCatalog) error {
	if len(c.Destinations) == 0 {
		return nil
	}
	var model struct {
		Locations map[string]struct {
			Terrain []string `json:"terrain"`
		} `json:"locations"`
		Edges []struct {
			ID, From string
			Exit     WayfindingCell
		} `json:"edges"`
	}
	if err := json.Unmarshal(life, &model); err != nil {
		return fmt.Errorf("wayfinding life: %w", err)
	}
	knownText := map[string]bool{}
	for _, m := range texts.Messages {
		knownText[m.Key] = true
	}
	seen := map[string]bool{}
	counts := map[string]int{}
	walks := func(level string, p WayfindingCell) bool {
		l, ok := model.Locations[level]
		return ok && p.Y >= 0 && p.Y < len(l.Terrain) && p.X >= 0 && p.X < len(l.Terrain[p.Y]) && walkable[string(l.Terrain[p.Y][p.X])]
	}
	guideCells := map[string]int{}
	numbers := map[string]map[int]bool{}
	for _, d := range c.Destinations {
		if d.ID == "" || seen[d.ID] || d.NameKey == "" || !knownText[d.NameKey] || !wayfindingColorPattern.MatchString(d.Color) {
			return fmt.Errorf("invalid wayfinding identity/text/color %q", d.ID)
		}
		if d.MarkerNumber < 0 || d.MarkerNumber > 99 {
			return fmt.Errorf("invalid wayfinding marker %s", d.ID)
		}
		if d.MarkerNumber > 0 {
			if numbers[d.Level] == nil {
				numbers[d.Level] = map[int]bool{}
			}
			if numbers[d.Level][d.MarkerNumber] {
				return fmt.Errorf("duplicate wayfinding marker %s", d.ID)
			}
			numbers[d.Level][d.MarkerNumber] = true
		}
		seen[d.ID] = true
		counts[d.Level]++
		if counts[d.Level] > 24 {
			return fmt.Errorf("wayfinding exceeds 24 destinations in %s", d.Level)
		}
		if d.Kind != "medical" && d.Kind != "sanitary" && d.Kind != "shop" && d.Kind != "exit" && d.Kind != "landmark" {
			return fmt.Errorf("invalid wayfinding kind %s", d.ID)
		}
		if !walks(d.Level, d.At) || (d.GuideFrom != nil && !walks(d.Level, *d.GuideFrom)) {
			return fmt.Errorf("wayfinding approach outside walkable floor %s", d.ID)
		}
		rows := model.Locations[d.Level].Terrain
		if len(rows)*len(rows[0]) > 65536 {
			return fmt.Errorf("wayfinding floor exceeds 65536 cells %s", d.Level)
		}
		if d.GuideFrom != nil {
			queue := []WayfindingCell{*d.GuideFrom}
			distance := map[WayfindingCell]int{*d.GuideFrom: 1}
			found := false
			for head := 0; head < len(queue); head++ {
				at := queue[head]
				if at == d.At {
					guideCells[d.Level] += distance[at]
					found = true
					break
				}
				for _, step := range []WayfindingCell{{X: 1}, {Y: 1}, {X: -1}, {Y: -1}} {
					next := WayfindingCell{X: at.X + step.X, Y: at.Y + step.Y}
					if distance[next] == 0 && walks(d.Level, next) {
						distance[next] = distance[at] + 1
						queue = append(queue, next)
					}
				}
			}
			if !found {
				return fmt.Errorf("wayfinding guide disconnected from approach %s", d.ID)
			}
			if guideCells[d.Level] > 16384 {
				return fmt.Errorf("wayfinding guides exceed 16384 cells %s", d.Level)
			}
		}
		if d.EdgeID != "" {
			found := false
			for _, e := range model.Edges {
				if e.ID == d.EdgeID && e.From == d.Level && e.Exit == d.At {
					found = true
					break
				}
			}
			if !found || d.Kind != "exit" {
				return fmt.Errorf("wayfinding exit does not match physical Life edge %s", d.ID)
			}
		}
	}
	return nil
}
