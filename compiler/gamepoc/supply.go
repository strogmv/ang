package gamepoc

import (
	"encoding/json"
	"fmt"
	inv "github.com/strogmv/ang/compiler/inventory/runtime"
)

// A delivery post is a real walkable routine destination, not an arbitrary
// marker on the picture. Public-floor topology is checked by checkPaintedScenes.
func validateCommerce(c Compiled) error {
	if len(c.Inventory.Deliveries) == 0 && len(c.Inventory.Shopfronts) == 0 {
		return nil
	}
	var life struct {
		Locations map[string]struct {
			Terrain []string `json:"terrain"`
		} `json:"locations"`
		Actors []struct {
			ID    string `json:"id"`
			Steps []struct {
				Level string   `json:"level"`
				At    inv.Cell `json:"at"`
			} `json:"steps"`
		} `json:"actors"`
	}
	if err := json.Unmarshal(c.Life, &life); err != nil {
		return err
	}
	valid := func(p inv.DeliveryPoint) bool {
		rows := life.Locations[p.Level].Terrain
		return p.At.Y >= 0 && p.At.Y < len(rows) && p.At.X >= 0 && p.At.X < len(rows[p.At.Y]) && c.Walkable[string(rows[p.At.Y][p.At.X])]
	}
	serves := func(actor string, p inv.DeliveryPoint) bool {
		for _, a := range life.Actors {
			if a.ID == actor {
				for _, step := range a.Steps {
					if step.Level == p.Level && step.At == p.At {
						return true
					}
				}
			}
		}
		return false
	}
	anchor := func(id string) inv.Anchor {
		a := c.Inventory.Stores[id].Anchor
		if p, ok := c.PaintedInventoryAnchors[a.Level][id]; ok {
			a.At = p
		}
		return a
	}
	near := func(a inv.Anchor, p inv.DeliveryPoint) bool {
		x, y := a.At.X-p.At.X, a.At.Y-p.At.Y
		if x < 0 {
			x = -x
		}
		if y < 0 {
			y = -y
		}
		return a.Kind == "world" && a.Level == p.Level && x+y <= 1
	}
	for _, d := range c.Inventory.Deliveries {
		if !valid(d.Pickup) || !valid(d.Dropoff) || !serves(d.Carrier, d.Pickup) || !serves(d.Carrier, d.Dropoff) || !near(anchor(d.SourceStore), d.Pickup) || !near(anchor(d.DestinationStore), d.Dropoff) {
			return fmt.Errorf("delivery %s has no physical routine/storage approach", d.ID)
		}
	}
	for _, s := range c.Inventory.Shopfronts {
		if !valid(s.Service) || !serves(s.Owner, s.Service) || !near(anchor(s.DisplayStore), s.Service) || !near(anchor(s.ReserveStore), s.Service) {
			return fmt.Errorf("shop %s has no physical service/storage approach", s.ID)
		}
	}
	return nil
}
