package locationsdk

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
)

var lockHash = regexp.MustCompile(`^[0-9a-f]{64}$`)

// LayoutFingerprint returns the complete authored/effective scene identity,
// excluding lock expectations themselves and unrelated layouts. PNG bytes are
// deliberately not read here: Resources keys are locked; physical files must be
// versioned separately. This read-only operation ignores old lock expectations
// so an author can inspect and explicitly approve a changed scene.
func LayoutFingerprint(input Input, layoutID string) (string, error) {
	var source Layout
	found := false
	for _, layout := range input.Layouts {
		if layout.ID == layoutID {
			if found {
				return "", issue("duplicate_layout", "layouts", "fingerprinting requires unique layout IDs", layoutID)
			}
			source, found = layout, true
		}
	}
	if !found {
		return "", issue("unknown_layout", "layout_id", "requested layout does not exist; a deleted or renamed layout cannot retain its old lock", layoutID)
	}
	compiled, err := Build(input.Catalog, source)
	if err != nil {
		return "", err
	}
	prepared, invalid := prepare(input.Catalog)
	if invalid != nil {
		return "", invalid
	}
	views, invalid := runtimeViews(compiled.Views, input.Catalog, source)
	if invalid != nil {
		return "", invalid
	}
	dependencies := Catalog{UnitMM: input.Catalog.UnitMM, Times: prepared.times, Events: prepared.events, DefaultFloorSurface: input.Catalog.DefaultFloorSurface}
	moduleIDs, geometryIDs, decorIDs, surfaceIDs := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, instance := range source.Instances {
		moduleIDs[instance.ModuleID] = true
		for _, binding := range instance.Decorations {
			decorIDs[binding.DecorationID] = true
		}
	}
	for id := range moduleIDs {
		module := prepared.modules[id]
		dependencies.Modules = append(dependencies.Modules, module)
		geometryIDs[module.GeometryID] = true
		for _, variant := range module.Variants {
			geometryIDs[variant.GeometryID] = true
		}
	}
	for id := range geometryIDs {
		dependencies.Geometries = append(dependencies.Geometries, prepared.geometries[id])
	}
	for id := range decorIDs {
		dependencies.Decorations = append(dependencies.Decorations, prepared.decor[id])
	}
	for _, instance := range compiled.Instances {
		if instance.SurfaceID != "" {
			surfaceIDs[instance.SurfaceID] = true
		}
	}
	if dependencies.DefaultFloorSurface != "" {
		surfaceIDs[dependencies.DefaultFloorSurface] = true
	}
	for id := range surfaceIDs {
		dependencies.Surfaces = append(dependencies.Surfaces, prepared.surfaces[id])
	}
	sort.Slice(dependencies.Modules, func(i, j int) bool { return dependencies.Modules[i].ID < dependencies.Modules[j].ID })
	sort.Slice(dependencies.Geometries, func(i, j int) bool { return dependencies.Geometries[i].ID < dependencies.Geometries[j].ID })
	sort.Slice(dependencies.Decorations, func(i, j int) bool { return dependencies.Decorations[i].ID < dependencies.Decorations[j].ID })
	sort.Slice(dependencies.Surfaces, func(i, j int) bool { return dependencies.Surfaces[i].ID < dependencies.Surfaces[j].ID })
	value := struct {
		Schema         string        `json:"schema"`
		Source         Layout        `json:"source"`
		Dependencies   Catalog       `json:"dependencies"`
		Compiled       CompiledLevel `json:"compiled"`
		Views          []RuntimeView `json:"views"`
		InitialTimeID  string        `json:"initial_time_id"`
		InitialEventID string        `json:"initial_event_id"`
	}{"ang.location-lock.v1", canonicalLockLayout(source), dependencies, compiled, views, input.Catalog.Times[0].ID, input.Catalog.Events[0].ID}
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode layout lock fingerprint: %w", err)
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// ValidateLocks is shared by the authoring CLI and runtime compilation. It must
// succeed before any output is published. An absent list preserves legacy input.
func ValidateLocks(input Input) error {
	if len(input.Locks) > 256 {
		return issue("lock_budget", "locks", "at most 256 layouts may be locked")
	}
	seen := map[string]bool{}
	for i, lock := range input.Locks {
		path := fmt.Sprintf("locks[%d]", i)
		if !stableID.MatchString(lock.LayoutID) || seen[lock.LayoutID] {
			return issue("lock_identity", path+".layout_id", "lock needs a unique existing layout ID", lock.LayoutID)
		}
		seen[lock.LayoutID] = true
		if !lockHash.MatchString(lock.ExpectedHash) {
			return issue("lock_hash", path+".expected_hash", "expected_hash must contain exactly 64 lowercase SHA-256 hexadecimal characters; print a candidate with ang location-sdk -fingerprint <layout-id> <CUE-directory>", lock.LayoutID)
		}
		actual, err := LayoutFingerprint(input, lock.LayoutID)
		if err != nil {
			if invalid, ok := err.(*ValidationError); ok && invalid.Diagnostics[0].Code == "unknown_layout" {
				return issue("lock_unknown_layout", path+".layout_id", "locked layout was deleted or renamed; restore it or explicitly remove its lock", lock.LayoutID)
			}
			return err
		}
		if actual != lock.ExpectedHash {
			return issue("lock_mismatch", path+".expected_hash", fmt.Sprintf("locked layout %q or its used dependencies changed: expected %s, actual %s; restore the change or review `ang location-sdk -fingerprint %s <CUE-directory>` and explicitly replace expected_hash after approval", lock.LayoutID, lock.ExpectedHash, actual, lock.LayoutID), lock.LayoutID)
		}
	}
	return nil
}

func canonicalLockLayout(source Layout) Layout {
	source.Floor = append([]FloorArea{}, source.Floor...)
	source.Entries = append([]Cell{}, source.Entries...)
	source.Instances = append([]Instance{}, source.Instances...)
	source.ReservedPaths = append([]ReservedPath{}, source.ReservedPaths...)
	source.Drops = append([]DropAnnotation{}, source.Drops...)
	source.Decks = append([]Deck{}, source.Decks...)
	for i := range source.Instances {
		source.Instances[i].Decorations = append([]DecorationBinding{}, source.Instances[i].Decorations...)
		sort.Slice(source.Instances[i].Decorations, func(a, b int) bool {
			return source.Instances[i].Decorations[a].ID < source.Instances[i].Decorations[b].ID
		})
	}
	sort.Slice(source.Floor, func(i, j int) bool { return source.Floor[i].ID < source.Floor[j].ID })
	sort.Slice(source.Instances, func(i, j int) bool { return source.Instances[i].ID < source.Instances[j].ID })
	sort.Slice(source.ReservedPaths, func(i, j int) bool { return source.ReservedPaths[i].ID < source.ReservedPaths[j].ID })
	sort.Slice(source.Drops, func(i, j int) bool { return source.Drops[i].ID < source.Drops[j].ID })
	return source
}
