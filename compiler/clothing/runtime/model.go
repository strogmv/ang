// Package clothing evaluates clothing surfaces and observed social cues. It owns
// no custody, equipment slots, medical state, NPC controller or network commands.
package clothing

import (
	"fmt"
	"regexp"
	"strings"
)

// Contamination is visible surface fouling, not a diagnosis or pathogen load.
type Contamination struct {
	Dirt     int `json:"dirt"`
	Blood    int `json:"blood"`
	Sanitary int `json:"sanitary"`
}
type Material struct {
	Name        string        `json:"name"`
	Washability Contamination `json:"washability"`
}
type LayerDefinition struct {
	ID       string `json:"id"`
	Material string `json:"material"`
	Color    string `json:"color"`
	Order    int    `json:"order"`
}
type GarmentDefinition struct {
	Name            string            `json:"name"`
	AppearanceStyle string            `json:"appearance_style"`
	Layers          []LayerDefinition `json:"layers"`
	Sources         []string          `json:"sources"`
}
type ReactionRule struct {
	ID              string          `json:"id"`
	Role            string          `json:"role"`
	AppearanceStyle string          `json:"appearance_style"`
	When            map[string]bool `json:"when"`
	Metric          string          `json:"metric"`
	Threshold       int             `json:"threshold"`
	Signal          string          `json:"signal"`
	Weight          int             `json:"weight"`
}
type WashFacility struct {
	Name              string        `json:"name"`
	Kind              string        `json:"kind"`
	WaterML           int64         `json:"water_ml"`
	CleanerUnits      int64         `json:"cleaner_units"`
	DurationTicks     int64         `json:"duration_ticks"`
	MinWaterQuality   int           `json:"min_water_quality"`
	Removal           Contamination `json:"removal"`
	DirtyWaterDeposit Contamination `json:"dirty_water_deposit"`
	WetnessAfter      int           `json:"wetness_after"`
}
type Model struct {
	Materials      map[string]Material          `json:"materials"`
	Garments       map[string]GarmentDefinition `json:"garments"`
	Reactions      []ReactionRule               `json:"reactions"`
	WashFacilities map[string]WashFacility      `json:"wash_facilities"`
}

var stableName = regexp.MustCompile(`^[a-z][a-z0-9_.-]*$`)
var colorCode = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func bounded(v int) bool { return v >= 0 && v <= 1000 }
func (c Contamination) valid() bool {
	return bounded(c.Dirt) && bounded(c.Blood) && bounded(c.Sanitary)
}
func (m Model) Validate() error {
	for id, material := range m.Materials {
		if !stableName.MatchString(id) || strings.TrimSpace(material.Name) == "" || !material.Washability.valid() {
			return fmt.Errorf("material %q: invalid name/washability", id)
		}
	}
	for id, garment := range m.Garments {
		if !stableName.MatchString(id) || strings.TrimSpace(garment.Name) == "" || !stableName.MatchString(garment.AppearanceStyle) || len(garment.Layers) == 0 {
			return fmt.Errorf("garment %q: invalid identity/style/layers", id)
		}
		layers := map[string]bool{}
		orders := map[int]bool{}
		for _, layer := range garment.Layers {
			if !stableName.MatchString(layer.ID) || layers[layer.ID] || orders[layer.Order] || layer.Order < 0 || !colorCode.MatchString(layer.Color) {
				return fmt.Errorf("garment %q: invalid/duplicate layer %q", id, layer.ID)
			}
			if _, ok := m.Materials[layer.Material]; !ok {
				return fmt.Errorf("garment %q: unknown material %q", id, layer.Material)
			}
			layers[layer.ID] = true
			orders[layer.Order] = true
		}
	}
	rules := map[string]bool{}
	for _, rule := range m.Reactions {
		if !stableName.MatchString(rule.ID) || rules[rule.ID] || !stableName.MatchString(rule.Signal) || rule.Threshold < 1 || rule.Threshold > 1000 || rule.Weight < 1 || rule.Weight > 1000 {
			return fmt.Errorf("reaction %q: invalid identity/threshold/weight", rule.ID)
		}
		if rule.Role != "" && !stableName.MatchString(rule.Role) || rule.AppearanceStyle != "" && !stableName.MatchString(rule.AppearanceStyle) {
			return fmt.Errorf("reaction %q: invalid selector", rule.ID)
		}
		switch rule.Metric {
		case "dirt", "blood", "sanitary", "wetness", "wear", "sanitary_odor":
		default:
			return fmt.Errorf("reaction %q: unknown metric", rule.ID)
		}
		for fact := range rule.When {
			if !stableName.MatchString(fact) {
				return fmt.Errorf("reaction %q: invalid contextual fact", rule.ID)
			}
		}
		rules[rule.ID] = true
	}
	for id, wash := range m.WashFacilities {
		if !stableName.MatchString(id) || strings.TrimSpace(wash.Name) == "" || !stableName.MatchString(wash.Kind) || wash.WaterML <= 0 || wash.CleanerUnits < 0 || wash.DurationTicks <= 0 || !bounded(wash.MinWaterQuality) || !wash.Removal.valid() || !wash.DirtyWaterDeposit.valid() || !bounded(wash.WetnessAfter) {
			return fmt.Errorf("wash facility %q: invalid requirements/effect", id)
		}
	}
	return nil
}
