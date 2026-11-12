package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"github.com/strogmv/ang/compiler/locationsdk"
)

type ExitZoneCatalog struct {
	Zones []ExitZone `json:"zones"`
}
type ExitZone struct {
	EdgeID         string             `json:"edge_id"`
	TextKey        string             `json:"text_key"`
	DestinationKey string             `json:"destination_key"`
	Kind           string             `json:"kind"`
	LevelID        string             `json:"level_id"`
	Cells          []locationsdk.Cell `json:"cells"`
	Radius         int                `json:"derive_radius_cells"`
	PortInstances  []string           `json:"port_instances"`
	Source         string             `json:"source"`
}
type exitZoneLife struct {
	Locations map[string]struct {
		Terrain []string `json:"terrain"`
		CellMM  int      `json:"cell_mm"`
	} `json:"locations"`
	Edges []struct {
		ID, From, To, Mode string
		Exit               locationsdk.Cell
		// a vehicle-only edge (cargo freight road) is no exit of people: it has no floor zone
		Vehicle bool `json:"vehicle"`
	} `json:"edges"`
}

func readExitZones(world cue.Value, c Compiled) (*ExitZoneCatalog, error) {
	v := world.LookupPath(cue.ParsePath("exit_zones"))
	if !v.Exists() {
		var life exitZoneLife
		_ = json.Unmarshal(c.Life, &life)
		if len(c.GodotTargets) > 0 && len(life.Edges) > 0 {
			return nil, fmt.Errorf("world.exit_zones: physical Life exits require floor zones and localized text")
		}
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.exit_zones: %w", err)
	}
	var out ExitZoneCatalog
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&out); err != nil {
		return nil, fmt.Errorf("world.exit_zones: %w", err)
	}
	if err := out.compile(c); err != nil {
		return nil, fmt.Errorf("world.exit_zones: %w", err)
	}
	return &out, nil
}
func (c *ExitZoneCatalog) compile(compiled Compiled) error {
	var life exitZoneLife
	if err := json.Unmarshal(compiled.Life, &life); err != nil {
		return err
	}
	edges := map[string]int{}
	for i, e := range life.Edges {
		edges[e.ID] = i
	}
	messages := map[string]LocalizationMessage{}
	for _, m := range compiled.Localization.Messages {
		messages[m.Key] = m
	}
	seen := map[string]bool{}
	for i := range c.Zones {
		z := &c.Zones[i]
		index, ok := edges[z.EdgeID]
		if !ok {
			return fmt.Errorf("zone %q has no Life edge", z.EdgeID)
		}
		if seen[z.EdgeID] {
			return fmt.Errorf("edge %q has more than one zone", z.EdgeID)
		}
		seen[z.EdgeID] = true
		e := life.Edges[index]
		z.LevelID = e.From
		for _, key := range []string{z.TextKey, z.DestinationKey} {
			message := messages[key]
			if key == "" || message.Context == "" || strings.TrimSpace(message.Texts["ru"]) == "" || strings.TrimSpace(message.Texts["en"]) == "" {
				return fmt.Errorf("edge %q zone text %q requires context, RU and EN", e.ID, key)
			}
		}
		if z.Kind != "walk" && z.Kind != "train" && z.Kind != "deck" {
			return fmt.Errorf("edge %q has invalid zone kind", e.ID)
		}
		if z.Radius < 0 || z.Radius > 8 {
			return fmt.Errorf("edge %q zone radius out of range", e.ID)
		}
		location := life.Locations[e.From]
		walks := func(at locationsdk.Cell) bool {
			return at.Y >= 0 && at.Y < len(location.Terrain) && at.X >= 0 && at.X < len(location.Terrain[at.Y]) && compiled.Walkable[string(location.Terrain[at.Y][at.X])]
		}
		if len(z.Cells) == 0 {
			seeds := []locationsdk.Cell{e.Exit}
			z.Source = "life_exit"
			layout := exitZoneLayout(compiled, e.From)
			if layout != nil {
				unit := location.CellMM
				if unit == 0 {
					unit = 1000
				}
				convert := func(at locationsdk.Cell) locationsdk.Cell {
					return locationsdk.Cell{X: int(math.Floor((float64(at.X) + .5) * float64(layout.UnitMM) / float64(unit))), Y: int(math.Floor((float64(at.Y) + .5) * float64(layout.UnitMM) / float64(unit)))}
				}
				// Ports carry authored doorway width/approach. Explicit instance families
				// (e.g. eight railway doors) intentionally combine into one boarding zone.
				best := math.MaxInt
				var chosen []locationsdk.CompiledPort
				for _, p := range layout.Ports {
					selected := false
					for _, id := range z.PortInstances {
						selected = selected || p.InstanceID == id
					}
					if len(z.PortInstances) > 0 {
						if selected {
							chosen = append(chosen, p)
						}
						continue
					}
					if p.Kind != "door" && p.Kind != "connection" && p.Kind != "use" {
						continue
					}
					at := convert(p.At)
					distance := exitZoneAbs(at.X-e.Exit.X) + exitZoneAbs(at.Y-e.Exit.Y)
					if distance < best {
						best = distance
						chosen = []locationsdk.CompiledPort{p}
					} else if distance == best {
						chosen = append(chosen, p)
					}
				}
				if len(z.PortInstances) > 0 && len(chosen) == 0 {
					return fmt.Errorf("edge %q references no SDK port", e.ID)
				}
				if len(z.PortInstances) > 0 || best <= 2 {
					for _, p := range chosen {
						for _, at := range p.Approach {
							seeds = append(seeds, convert(at))
						}
						for _, at := range p.Opening {
							seeds = append(seeds, convert(at))
						}
					}
					z.Source = "sdk_port_approach"
				}
			}
			points := map[locationsdk.Cell]bool{}
			for _, seed := range seeds {
				for y := seed.Y - z.Radius; y <= seed.Y+z.Radius; y++ {
					for x := seed.X - z.Radius; x <= seed.X+z.Radius; x++ {
						at := locationsdk.Cell{X: x, Y: y}
						if walks(at) {
							points[at] = true
						}
					}
				}
			}
			for at := range points {
				z.Cells = append(z.Cells, at)
			}
		} else {
			z.Source = "authored_cells"
		}
		if len(z.Cells) == 0 {
			return fmt.Errorf("edge %q has no zone on walkable floor", e.ID)
		}
		unique := map[locationsdk.Cell]bool{}
		for _, at := range z.Cells {
			if !walks(at) {
				return fmt.Errorf("edge %q zone cell (%d,%d) is not walkable floor", e.ID, at.X, at.Y)
			}
			if unique[at] {
				return fmt.Errorf("edge %q repeats zone cell", e.ID)
			}
			unique[at] = true
		}
		sort.Slice(z.Cells, func(i, j int) bool { a, b := z.Cells[i], z.Cells[j]; return a.Y < b.Y || a.Y == b.Y && a.X < b.X })
	}
	for _, e := range life.Edges {
		if !seen[e.ID] && !e.Vehicle {
			return fmt.Errorf("Life edge %q has no exit zone", e.ID)
		}
		if seen[e.ID] && e.Vehicle {
			return fmt.Errorf("vehicle-only Life edge %q has an exit zone", e.ID)
		}
	}
	sort.Slice(c.Zones, func(i, j int) bool { return c.Zones[i].EdgeID < c.Zones[j].EdgeID })
	return nil
}
func exitZoneLayout(c Compiled, level string) *locationsdk.RuntimeLevel {
	layout := ""
	for _, b := range c.ModularLocations {
		if b.LevelID == level {
			layout = b.LayoutID
		}
	}
	for _, b := range c.GodotScenes {
		if b.LevelID == level && b.LayoutID != "" {
			layout = b.LayoutID
		}
	}
	if c.LocationSDK != nil {
		for i := range c.LocationSDK.Levels {
			l := &c.LocationSDK.Levels[i]
			if l.ID == layout {
				return l
			}
		}
	}
	return nil
}

func exitZoneAbs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
