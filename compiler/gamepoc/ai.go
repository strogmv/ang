package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

func canonicalJSONBytes(data []byte) ([]byte, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

// clientArt is the view projection of world.art: lists instead of maps (Unity JsonUtility), sorted for stable bytes.
func clientArt(c Compiled) map[string]any {
	glyphs := make([]string, 0, len(c.TileSprites))
	for g := range c.TileSprites {
		glyphs = append(glyphs, g)
	}
	sort.Strings(glyphs)
	tiles := make([]map[string]any, 0, len(glyphs))
	for _, g := range glyphs {
		tiles = append(tiles, map[string]any{"glyph": g, "sprite": c.TileSprites[g]})
	}
	names := make([]string, 0, len(c.Art.Sprites))
	for n := range c.Art.Sprites {
		names = append(names, n)
	}
	sort.Strings(names)
	sprites := make([]map[string]any, 0, len(names))
	for _, n := range names {
		s := c.Art.Sprites[n]
		weights := s.Weights
		if len(weights) == 0 {
			weights = make([]int, s.Variants)
			for i := range weights {
				weights[i] = 1
			}
		}
		sprites = append(sprites, map[string]any{"name": n, "shape": s.Shape, "style": s.Style, "height": s.Height, "variants": s.Variants, "weights": weights})
	}
	return map[string]any{"hud": c.Art.HUD, "floor": c.Art.Floor, "scenes": clientArtScenes(c.Art.Scenes), "painted": c.Art.Painted, "camera": c.Art.Camera, "tile_w": c.Art.Tile.W, "tile_h": c.Art.Tile.H, "tiles": tiles, "sprites": sprites, "characters": clientCharacters(c.Art.Characters)}
}

// clientLoading is the view projection of world.loading: a sorted list of screens (Unity JsonUtility has no maps).
func clientLoading(c Compiled) map[string]any {
	ids := make([]string, 0, len(c.Loading.Screens))
	for id := range c.Loading.Screens {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	screens := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		s := c.Loading.Screens[id]
		screens = append(screens, map[string]any{"id": id, "image": s.Image, "context": s.Context})
	}
	return map[string]any{"caption": c.Loading.Caption, "min_ms": c.Loading.MinMS, "screens": screens}
}

// clientMenu is the view projection of world.menu.
func clientMenu(c Compiled) map[string]any {
	return map[string]any{"background": c.Menu.Background}
}

func clientResource(c Compiled) ([]byte, error) {
	cfg := c.Config
	return json.MarshalIndent(map[string]any{
		"content_hash": c.ContentHash,
		"spatial":      clientSpatial(c.Spatial),
		"art":          clientArt(c),
		"loading":      clientLoading(c),
		"menu":         clientMenu(c),
		"config": map[string]any{
			"title": cfg.Title, "subtitle": cfg.Subtitle, "seed": cfg.Seed, "tick_hz": cfg.TickHz,
			"queue_capacity": cfg.QueueCapacity, "input_ack_budget_ms": cfg.InputAckBudgetMS,
			"min_client_fps": cfg.MinClientFPS, "presentation_lead_ms": cfg.PresentationLeadMS,
			"presentation_horizon_ms": cfg.PresentationHorizonMS, "labels": cfg.Labels,
			"capture_min_contact_members":    c.Inventory.Preparation.MinContactMembers,
			"capture_support_distance_cells": c.Inventory.Preparation.SupportDistance,
		},
	}, "", "  ")
}

func validateAIFixtureFacts(data []byte, facts map[string]bool) error {
	var c struct {
		Facts map[string]json.RawMessage `json:"facts"`
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return err
	}
	for key := range facts {
		if _, ok := c.Facts[key]; !ok {
			return fmt.Errorf("unknown AI fixture fact %s", key)
		}
		switch key {
		case "target.alive", "target.dead", "group.formed", "group.in_position", "custody.captured", "custody.held", "custody.at_medblock", "custody.handed_over":
			return fmt.Errorf("AI fixture cannot override live state %s", key)
		}
	}
	return nil
}
