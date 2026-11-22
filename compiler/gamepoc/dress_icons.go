package gamepoc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type dressIconRecord struct {
	ID           string            `json:"icon_id"`
	Source       string            `json:"source_model"`
	Hash         string            `json:"source_sha256"`
	LFS          bool              `json:"lfs"`
	Files        map[string]string `json:"files"`
	Dependencies map[string]string `json:"dependencies"`
}

func bindDressIcons(c *Compiled, root string) error {
	if c.Inventory.Dress == nil || len(c.GodotTargets) == 0 {
		return nil
	}
	dir := filepath.Join(root, "client-godot/assets/items/icons")
	contents, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return fmt.Errorf("inventory picture manifest: %w", err)
	}
	var manifest struct {
		Items map[string]dressIconRecord `json:"items"`
	}
	if err := json.Unmarshal(contents, &manifest); err != nil {
		return fmt.Errorf("inventory picture manifest: %w", err)
	}
	check := func(path, expected string) error {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(data)
		if hex.EncodeToString(hash[:]) != expected {
			return fmt.Errorf("stale inventory picture input %s", path)
		}
		return nil
	}
	for id, row := range manifest.Items {
		d, ok := c.Inventory.Definitions[id]
		source := filepath.Clean(row.Source)
		if !ok || !row.LFS || row.ID != id || filepath.Base(id) != id ||
			source != row.Source || !(strings.HasPrefix(source, "client-godot/") || strings.HasPrefix(source, "art-source/")) {
			return fmt.Errorf("invalid inventory picture row %q", id)
		}
		if err := check(filepath.Join(root, source), row.Hash); err != nil {
			return err
		}
		for dependency, digest := range row.Dependencies {
			if filepath.Clean(dependency) != dependency || !strings.HasPrefix(dependency, "client-godot/") {
				return fmt.Errorf("%s: invalid picture dependency %s", id, dependency)
			}
			if err := check(filepath.Join(root, dependency), digest); err != nil {
				return err
			}
		}
		for _, suffix := range []string{".png", ".96.png"} {
			name := id + suffix
			if err := check(filepath.Join(dir, name), row.Files[name]); err != nil {
				return err
			}
		}
		d.IconID = row.ID
		c.Inventory.Definitions[id] = d
	}
	var missing []string
	for id, definition := range c.Inventory.Definitions {
		if definition.IconID == "" {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		log.Printf("inventory definitions without pictures (%d): %s", len(missing), strings.Join(missing, ", "))
	}
	for _, id := range c.Inventory.Dress.RequiredIconItems {
		if c.Inventory.Definitions[id].IconID == "" {
			return fmt.Errorf("obtainable Act I item %s requires an inventory picture", id)
		}
	}
	return nil
}
