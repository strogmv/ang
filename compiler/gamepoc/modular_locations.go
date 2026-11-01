package gamepoc

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/strogmv/ang/compiler/locationsdk"
)

// ModularLocation binds a compiled physical layout to an ordinary Life location.
// It changes presentation/terrain, never replaces the world's actors or stores.
type ModularLocation struct {
	LevelID  string `json:"level_id"`
	LayoutID string `json:"layout_id"`
}

func compileModularLocations(raw json.RawMessage, bundle *locationsdk.RuntimeBundle) (json.RawMessage, []ModularLocation, error) {
	if len(raw) == 0 {
		return raw, nil, nil
	}
	var life map[string]json.RawMessage
	if err := json.Unmarshal(raw, &life); err != nil {
		return nil, nil, err
	}
	var locations map[string]map[string]json.RawMessage
	if err := json.Unmarshal(life["locations"], &locations); err != nil {
		return nil, nil, err
	}
	layouts := map[string]locationsdk.RuntimeLevel{}
	if bundle != nil {
		for _, l := range bundle.Levels {
			layouts[l.ID] = l
		}
	}
	ids := make([]string, 0, len(locations))
	for id := range locations {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var bindings []ModularLocation
	used := map[string]bool{}
	for _, id := range ids {
		loc := locations[id]
		value, ok := loc["modular_layout"]
		if !ok {
			if _, terrain := loc["terrain"]; !terrain {
				return nil, nil, fmt.Errorf("location %s requires terrain or modular_layout", id)
			}
			continue
		}
		var layout string
		if err := json.Unmarshal(value, &layout); err != nil || layout == "" {
			return nil, nil, fmt.Errorf("modular location %s: invalid layout reference", id)
		}
		l, exists := layouts[layout]
		if !exists {
			return nil, nil, fmt.Errorf("modular location %s: unknown layout %q", id, layout)
		}
		if used[layout] {
			return nil, nil, fmt.Errorf("modular layout %s bound to multiple story locations", layout)
		}
		used[layout] = true
		if _, exists := loc["terrain"]; exists {
			return nil, nil, fmt.Errorf("modular location %s: terrain must come only from the compiled layout", id)
		}
		if len(l.Terrain) != l.Depth || !l.InitialLayoutKnown {
			return nil, nil, fmt.Errorf("modular location %s: unsupported layout", id)
		}
		// Document 80 + demo-hub: a multi-deck layout binds to a story location as
		// its whole navigation plane (one band per walkable surface, one block per
		// stair) with the stair-end links; the runtime adds the 3D sight volume.
		// Floor guides, painted backgrounds and SDK stand residents stay single-
		// surface / SDK-only: they are rejected rather than flattened.
		if l.Navigation != nil && len(l.ReservedPaths) > 0 {
			return nil, nil, fmt.Errorf("modular location %s: multi-deck layout %s carries floor guides (single-surface only)", id, layout)
		}
		if len(l.Residents) > 0 {
			return nil, nil, fmt.Errorf("modular location %s: layout %s residents are SDK test-stand people, not story actors", id, layout)
		}
		delete(loc, "modular_layout")
		loc["terrain"], _ = json.Marshal(l.Terrain)
		// Movement v2 (document 87): the controlled hero walks this story
		// location as a continuous body measured in the layout's cells.
		if _, exists := loc["cell_mm"]; exists {
			return nil, nil, fmt.Errorf("modular location %s: cell_mm must come only from the compiled layout", id)
		}
		loc["cell_mm"], _ = json.Marshal(l.UnitMM)
		if l.Navigation != nil {
			if _, exists := loc["links"]; exists {
				return nil, nil, fmt.Errorf("modular location %s: links must come only from the compiled layout", id)
			}
			loc["links"], _ = json.Marshal(l.Navigation.Links)
		}
		// Server-authoritative gates (doc 85) of the layout animate their door
		// ports in the story exactly as on their stand.
		if len(l.Gates) > 0 {
			if _, exists := loc["gates"]; exists {
				return nil, nil, fmt.Errorf("modular location %s: gates must come only from the compiled layout", id)
			}
			loc["gates"], _ = json.Marshal(l.Gates)
		}
		// This renderer supports remembered public architecture only. Do not silently
		// convert a private/unknown location into a public model.
		if known, exists := loc["initial_layout_known"]; exists && string(known) != "true" {
			return nil, nil, fmt.Errorf("modular location %s requires known public architecture", id)
		}
		loc["initial_layout_known"] = json.RawMessage("true")
		bindings = append(bindings, ModularLocation{LevelID: id, LayoutID: layout})
	}
	if len(bindings) == 0 {
		return raw, nil, nil
	}
	if err := materializeModularPopulation(life, locations, bindings, bundle); err != nil {
		return nil, nil, err
	}
	life["locations"], _ = json.Marshal(locations)
	result, err := json.Marshal(life)
	return result, bindings, err
}

// Population floor candidates are derived after layout compilation as well.
// Never manufacture anchors from a placeholder CUE terrain.
func materializeModularPopulation(life map[string]json.RawMessage, locations map[string]map[string]json.RawMessage, bindings []ModularLocation, bundle *locationsdk.RuntimeBundle) error {
	if len(life["population"]) == 0 {
		return nil
	}
	var population map[string]json.RawMessage
	if err := json.Unmarshal(life["population"], &population); err != nil {
		return err
	}
	anchors := map[string][]Point{}
	if len(population["anchors"]) > 0 {
		if err := json.Unmarshal(population["anchors"], &anchors); err != nil {
			return err
		}
	}
	type position struct {
		Level string `json:"level"`
		At    Point  `json:"at"`
	}
	var actors []struct {
		Steps []position `json:"steps"`
	}
	if err := json.Unmarshal(life["actors"], &actors); err != nil {
		return err
	}
	var edges []struct {
		From, To    string
		Exit, Entry Point
	}
	if err := json.Unmarshal(life["edges"], &edges); err != nil {
		return err
	}
	var practice []position
	if len(life["practice_stations"]) > 0 {
		if err := json.Unmarshal(life["practice_stations"], &practice); err != nil {
			return err
		}
	}
	var maintenance struct {
		Assets []struct {
			Level      string
			At         Point
			Workplaces []Point
		}
	}
	if len(life["maintenance"]) > 0 {
		if err := json.Unmarshal(life["maintenance"], &maintenance); err != nil {
			return err
		}
	}
	// demo-hub v5-yatai: seats, queue places and staff posts of a seated place (Life brain "seated_food") are for eating
	// and serving, not for idling: a resident loitering on a seat would hold it for hours.
	var brain struct {
		Places []struct {
			Level   string `json:"level"`
			Seating *struct {
				Seats, Queue, Posts []struct {
					At Point `json:"at"`
				}
			} `json:"seating"`
		} `json:"places"`
	}
	if len(life["brain"]) > 0 {
		if err := json.Unmarshal(life["brain"], &brain); err != nil {
			return err
		}
	}
	for _, b := range bindings {
		reserved := map[Point]bool{}
		if bundle != nil {
			for _, layout := range bundle.Levels {
				if layout.ID != b.LayoutID {
					continue
				}
				for _, clearance := range layout.Clearances {
					if clearance.Kind == "maintenance" {
						for _, cell := range clearance.Cells {
							reserved[Point{X: cell.X, Y: cell.Y}] = true
						}
					}
				}
				for _, path := range layout.ReservedPaths {
					for _, cell := range path.Cells {
						reserved[Point{X: cell.X, Y: cell.Y}] = true
					}
				}
			}
		}

		for _, a := range actors {
			for _, s := range a.Steps {
				if s.Level == b.LevelID {
					reserved[s.At] = true
				}
			}
		}
		for _, e := range edges {
			if e.From == b.LevelID {
				reserved[e.Exit] = true
			}
			if e.To == b.LevelID {
				reserved[e.Entry] = true
			}
		}
		for _, p := range practice {
			if p.Level == b.LevelID {
				reserved[p.At] = true
			}
		}
		for _, p := range brain.Places {
			if p.Level == b.LevelID && p.Seating != nil {
				for _, group := range [][]struct {
					At Point `json:"at"`
				}{p.Seating.Seats, p.Seating.Queue, p.Seating.Posts} {
					for _, spot := range group {
						reserved[spot.At] = true
					}
				}
			}
		}
		for _, a := range maintenance.Assets {
			if a.Level == b.LevelID {
				reserved[a.At] = true
				for _, p := range a.Workplaces {
					reserved[p] = true
				}
			}
		}
		if len(anchors[b.LevelID]) > 0 {
			return fmt.Errorf("modular location %s: population anchors must be compiler-derived", b.LevelID)
		}
		var terrain []string
		if err := json.Unmarshal(locations[b.LevelID]["terrain"], &terrain); err != nil {
			return err
		}
		inside := modularPopulationArea(terrain, layoutOf(bundle, b.LayoutID), reserved)
		for y, row := range terrain {
			for x, glyph := range row {
				p := Point{X: x, Y: y}
				if glyph == '.' && !reserved[p] && (inside == nil || inside[p]) {
					anchors[b.LevelID] = append(anchors[b.LevelID], p)
				}
			}
		}
		if len(anchors[b.LevelID]) == 0 {
			return fmt.Errorf("modular location %s has no unreserved population floor", b.LevelID)
		}
	}
	population["anchors"], _ = json.Marshal(anchors)
	life["population"], _ = json.Marshal(population)
	return nil
}

func layoutOf(bundle *locationsdk.RuntimeBundle, id string) *locationsdk.RuntimeLevel {
	if bundle == nil {
		return nil
	}
	for i := range bundle.Levels {
		if bundle.Levels[i].ID == id {
			return &bundle.Levels[i]
		}
	}
	return nil
}

// modularPopulationArea keeps ambient population on the public side of its
// gates: cells reachable from the layout's first entry with every gate opening
// closed (stair-end links included), never a gate opening or a stair node.
// Planar layouts without gates keep every floor cell (nil = no restriction).
func modularPopulationArea(terrain []string, level *locationsdk.RuntimeLevel, reserved map[Point]bool) map[Point]bool {
	if level == nil || (len(level.Gates) == 0 && level.Navigation == nil) || len(level.Entries) == 0 {
		return nil
	}
	closed := map[Point]bool{}
	for _, gate := range level.Gates {
		for _, c := range gate.Opening {
			closed[Point{X: c.X, Y: c.Y}] = true
			reserved[Point{X: c.X, Y: c.Y}] = true
		}
	}
	stair := map[Point]bool{}
	links := map[Point][]Point{}
	if level.Navigation != nil {
		for _, connector := range level.Navigation.Connectors {
			for _, node := range connector.Nodes {
				stair[Point{X: node.Nav.X, Y: node.Nav.Y}] = true
			}
		}
		for _, link := range level.Navigation.Links {
			a, b := Point{X: link.A.X, Y: link.A.Y}, Point{X: link.B.X, Y: link.B.Y}
			links[a] = append(links[a], b)
			links[b] = append(links[b], a)
		}
	}
	walk := func(p Point) bool {
		return p.Y >= 0 && p.Y < len(terrain) && p.X >= 0 && p.X < len(terrain[p.Y]) && terrain[p.Y][p.X] == '.' && !closed[p]
	}
	start := Point{X: level.Entries[0].X, Y: level.Entries[0].Y}
	if !walk(start) {
		return nil
	}
	seen := map[Point]bool{start: true}
	queue := []Point{start}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		next := append([]Point{{p.X + 1, p.Y}, {p.X - 1, p.Y}, {p.X, p.Y + 1}, {p.X, p.Y - 1}}, links[p]...)
		for _, q := range next {
			if !seen[q] && walk(q) {
				seen[q] = true
				queue = append(queue, q)
			}
		}
	}
	for p := range stair {
		delete(seen, p)
	}
	return seen
}
