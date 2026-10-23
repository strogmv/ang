package clothing

import "fmt"

type Surface struct {
	Dirt     int `json:"dirt"`
	Blood    int `json:"blood"`
	Sanitary int `json:"sanitary"`
	Wetness  int `json:"wetness"`
	// Wear is cosmetic abrasion/fading. Inventory condition remains the sole
	// mechanical durability authority; clothing never repairs or damages it.
	Wear          int    `json:"wear"`
	LastStainTick int64  `json:"last_stain_tick"`
	StainSource   string `json:"stain_source"`
}

func (s Surface) valid() bool {
	return bounded(s.Dirt) && bounded(s.Blood) && bounded(s.Sanitary) && bounded(s.Wetness) && bounded(s.Wear) && s.LastStainTick >= 0
}

type State struct {
	// ItemID must be supplied by the inventory adapter. This package never creates,
	// moves or equips an item, and does not claim its wearer has a caste or rights.
	ItemID      string             `json:"item_id"`
	Definition  string             `json:"definition"`
	Provenance  string             `json:"provenance"`
	Layers      map[string]Surface `json:"layers"`
	UpdatedTick int64              `json:"updated_tick"`
}
type Exposure struct {
	Dirt     int    `json:"dirt"`
	Blood    int    `json:"blood"`
	Sanitary int    `json:"sanitary"`
	Wetness  int    `json:"wetness"`
	Wear     int    `json:"wear"`
	Source   string `json:"source"`
}

func NewState(m Model, itemID, definition, provenance string, tick int64) (State, error) {
	if itemID == "" || tick < 0 {
		return State{}, fmt.Errorf("invalid item identity/time")
	}
	garment, ok := m.Garments[definition]
	if !ok {
		return State{}, fmt.Errorf("unknown garment %q", definition)
	}
	s := State{ItemID: itemID, Definition: definition, Provenance: provenance, Layers: map[string]Surface{}, UpdatedTick: tick}
	for _, layer := range garment.Layers {
		s.Layers[layer.ID] = Surface{}
	}
	return s, nil
}
func (s State) Clone() State {
	copy := s
	copy.Layers = make(map[string]Surface, len(s.Layers))
	for id, layer := range s.Layers {
		copy.Layers[id] = layer
	}
	return copy
}
func (s State) Validate(m Model, now int64) error {
	garment, ok := m.Garments[s.Definition]
	if !ok || s.ItemID == "" || now < 0 || s.UpdatedTick < 0 || s.UpdatedTick > now || len(s.Layers) != len(garment.Layers) {
		return fmt.Errorf("invalid garment state identity/layers/time")
	}
	for _, layer := range garment.Layers {
		surface, ok := s.Layers[layer.ID]
		if !ok || !surface.valid() || surface.LastStainTick > s.UpdatedTick {
			return fmt.Errorf("invalid layer state %q", layer.ID)
		}
	}
	return nil
}
func clamp(v int) int {
	if v > 1000 {
		return 1000
	}
	if v < 0 {
		return 0
	}
	return v
}
func Expose(m Model, s State, layerID string, e Exposure, now int64) (State, error) {
	if err := s.Validate(m, now); err != nil {
		return State{}, err
	}
	if !bounded(e.Dirt) || !bounded(e.Blood) || !bounded(e.Sanitary) || !bounded(e.Wetness) || !bounded(e.Wear) {
		return State{}, fmt.Errorf("invalid exposure")
	}
	layer, ok := s.Layers[layerID]
	if !ok {
		return State{}, fmt.Errorf("unknown exposed layer %q", layerID)
	}
	layer.Dirt = clamp(layer.Dirt + e.Dirt)
	layer.Blood = clamp(layer.Blood + e.Blood)
	layer.Sanitary = clamp(layer.Sanitary + e.Sanitary)
	layer.Wetness = clamp(layer.Wetness + e.Wetness)
	layer.Wear = clamp(layer.Wear + e.Wear)
	if e.Dirt > 0 || e.Blood > 0 || e.Sanitary > 0 {
		layer.LastStainTick = now
		layer.StainSource = e.Source
	}
	out := s.Clone()
	out.Layers[layerID] = layer
	out.UpdatedTick = now
	return out, nil
}
