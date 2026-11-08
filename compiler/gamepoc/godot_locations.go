package gamepoc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/godot"
	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
)

// GodotScene is optional finishing over the generated bare location. It never
// changes navigation, discovers an exit or supplies a local travel decision.
type GodotScene struct {
	LevelID string `json:"level_id"`
	Scene   string `json:"scene"`
}
type godotLocation struct {
	Scene       string                  `json:"scene"`
	Kind        string                  `json:"kind"`
	LayoutID    string                  `json:"layout_id"`
	UnitMM      int                     `json:"unit_mm"`
	ElevationMM int                     `json:"elevation_mm"`
	Terrain     []string                `json:"terrain"`
	Navigation  *locationsdk.Navigation `json:"navigation,omitempty"`
	Neighbours  []string                `json:"neighbours"`
}

var godotLevelID = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

func writeGodotLocations(project string, c Compiled) error {
	if len(c.GodotTargets) == 0 {
		return nil
	}
	var life struct {
		Locations map[string]struct {
			Terrain []string `json:"terrain"`
		} `json:"locations"`
		Edges []struct{ From, To string } `json:"edges"`
	}
	if err := json.Unmarshal(c.Life, &life); err != nil {
		return err
	}
	// Targets are backend-relative: the normal sibling client is ../client.
	// Keep every output inside the backend's enclosing project directory.
	for _, target := range c.GodotTargets {
		root, err := filepath.Abs(filepath.Dir(project))
		if err != nil {
			return err
		}
		destination, err := filepath.Abs(filepath.Join(project, target))
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, destination)
		if err != nil || target == "" || filepath.IsAbs(target) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("Godot target escapes enclosing project: %q", target)
		}
	}
	finishes := map[string]string{}
	for _, f := range c.GodotScenes {
		if !godotLevelID.MatchString(f.LevelID) || !godotScenePath(f.Scene) || finishes[f.LevelID] != "" {
			return fmt.Errorf("invalid/duplicate Godot finishing: %q", f.LevelID)
		}
		if _, ok := life.Locations[f.LevelID]; !ok {
			return fmt.Errorf("unknown Godot level %s", f.LevelID)
		}
		finishes[f.LevelID] = f.Scene
	}
	index := map[string]godotLocation{}
	for id, l := range life.Locations {
		if !godotLevelID.MatchString(id) {
			return fmt.Errorf("unsafe Godot level ID %q", id)
		}
		row := godotLocation{Scene: "res://generated/locations/" + id + "/location.tscn", Kind: "bare_floor", LayoutID: id, UnitMM: 1000, Terrain: l.Terrain, Neighbours: []string{}}
		level := locationsdk.CompiledLevel{ID: id, UnitMM: 1000, Terrain: l.Terrain}
		for _, painted := range c.Art.Painted {
			if painted.LevelID == id && painted.FloorElevationM != nil {
				row.ElevationMM = int(*painted.FloorElevationM * 1000)
			}
		}
		for _, b := range c.ModularLocations {
			if b.LevelID != id {
				continue
			}
			if c.LocationSDK == nil {
				return fmt.Errorf("missing SDK for %s", id)
			}
			found := false
			for _, sdk := range c.LocationSDK.Levels {
				if sdk.ID != b.LayoutID {
					continue
				}
				found = true
				row.Kind = "sdk"
				row.LayoutID = sdk.ID
				row.UnitMM = sdk.UnitMM
				row.ElevationMM = sdk.Elevation * sdk.UnitMM
				row.Navigation = sdk.Navigation
				row.Terrain = sdk.Terrain
				level = locationsdk.CompiledLevel{ID: sdk.ID, UnitMM: sdk.UnitMM, Width: sdk.Width, Depth: sdk.Depth, Elevation: sdk.Elevation, Grounded: sdk.Grounded, Terrain: sdk.Terrain, Instances: sdk.Instances, Navigation: sdk.Navigation, Camera: sdk.Camera, StructuralHash: sdk.StructuralHash, Preview: locationsdk.PreviewData{Floor: sdk.Floor}}
			}
			if !found {
				return fmt.Errorf("missing SDK layout %s", b.LayoutID)
			}
		}
		if len(row.Terrain) == 0 || len(row.Terrain[0]) == 0 {
			return fmt.Errorf("missing Godot floor for %s", id)
		}
		for _, edge := range life.Edges {
			if edge.From == id {
				row.Neighbours = append(row.Neighbours, edge.To)
			}
		}
		sort.Strings(row.Neighbours)
		// Runtime metadata is not player knowledge. The client only exposes server
		// markers. Neighbour IDs schedule resource IO, never markers or map entries.
		if f := finishes[id]; f != "" {
			row.Scene = f
		}
		var manifest placement.Manifest
		if row.Kind == "sdk" {
			spec := placement.Authoring{ModuleModels: map[string]string{}}
			for _, i := range level.Instances {
				spec.ModuleModels[i.ModuleID] = ""
			}
			var err error
			manifest, err = placement.Build(level, spec)
			if err != nil {
				return err
			}
		} else {
			level.Width = len(row.Terrain[0])
			level.Depth = len(row.Terrain)
			level.Elevation = row.ElevationMM / row.UnitMM
			raw, _ := json.Marshal(row)
			sum := sha256.Sum256(raw)
			level.StructuralHash = hex.EncodeToString(sum[:])
			for y, line := range row.Terrain {
				if len(line) != level.Width {
					return fmt.Errorf("ragged floor %s", id)
				}
				for x, glyph := range line {
					if c.Walkable[string(glyph)] {
						level.Preview.Floor = append(level.Preview.Floor, locationsdk.PreviewFloor{At: locationsdk.Cell{X: x, Y: y}, SourceID: "authored_floor"})
					}
				}
			}
			// A shell contains only the actual authored walkable footprint. No walls,
			// furniture, slab thickness, openings or art are inferred from blocked cells.
			manifest = placement.Manifest{Schema: placement.Schema, LayoutID: level.ID, WorldFrame: placement.WorldFrame, UnitMM: level.UnitMM, StructuralHash: level.StructuralHash, CompiledRef: "compiled.json#/levels/0", Instances: map[string]placement.Instance{}}
		}
		scene, err := godot.Render(level, manifest, godot.Options{WorldText: true, FloorOnly: row.Kind == "bare_floor"})
		if err != nil {
			return err
		}
		for _, target := range c.GodotTargets {
			folder := filepath.Join(project, target, "generated/locations", id)
			if err := os.MkdirAll(folder, 0755); err != nil {
				return err
			}
			files := map[string]any{"placements.json": manifest, "compiled.json": map[string]any{"levels": []locationsdk.CompiledLevel{level}}}
			for name, value := range files {
				data, err := json.MarshalIndent(value, "", "  ")
				if err != nil {
					return err
				}
				if err = os.WriteFile(filepath.Join(folder, name), append(data, '\n'), 0644); err != nil {
					return err
				}
			}
			if err = os.WriteFile(filepath.Join(folder, "location.tscn"), scene, 0644); err != nil {
				return err
			}
		}
		index[id] = row
	}
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}
	for _, target := range c.GodotTargets {
		if err := os.WriteFile(filepath.Join(project, target, "generated/locations.json"), append(data, '\n'), 0644); err != nil {
			return err
		}
	}
	return nil
}
func godotScenePath(s string) bool {
	return !strings.ContainsAny(s, "\n\r\x00\\") && len(s) > 6 && s[:6] == "res://" && filepath.Ext(s) == ".tscn" && filepath.Clean(s[6:]) == s[6:] && s[6] != '/' && !regexp.MustCompile(`(^|/)\.\.(/|$)`).MatchString(s[6:])
}
