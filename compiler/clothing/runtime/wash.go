package clothing

import "fmt"

// WashRequest is a completed, server-verified cycle. The host must reserve and
// spend resources atomically with inventory's command sequence. Supplying this
// value is not permission to teleport to a washer or skip the action's duration.
type WashRequest struct {
	Facility              string `json:"facility"`
	ElapsedTicks          int64  `json:"elapsed_ticks"`
	AvailableWaterML      int64  `json:"available_water_ml"`
	AvailableCleanerUnits int64  `json:"available_cleaner_units"`
	WaterQuality          int    `json:"water_quality"`
	NowTick               int64  `json:"now_tick"`
}
type WashResult struct {
	State             State `json:"state"`
	SpentWaterML      int64 `json:"spent_water_ml"`
	SpentCleanerUnits int64 `json:"spent_cleaner_units"`
	DurationTicks     int64 `json:"duration_ticks"`
}

func washed(value, removal, washability, quality, deposit int) int {
	removed := int(int64(value) * int64(removal) * int64(washability) * int64(quality) / 1000000000)
	added := (1000 - quality) * deposit / 1000
	return clamp(value - removed + added)
}
func Wash(m Model, s State, request WashRequest) (WashResult, error) {
	if err := s.Validate(m, request.NowTick); err != nil {
		return WashResult{}, err
	}
	facility, ok := m.WashFacilities[request.Facility]
	if !ok {
		return WashResult{}, fmt.Errorf("unknown wash facility")
	}
	if !bounded(request.WaterQuality) || request.WaterQuality < facility.MinWaterQuality || request.ElapsedTicks < facility.DurationTicks || request.ElapsedTicks > request.NowTick || request.AvailableWaterML < facility.WaterML || request.AvailableCleanerUnits < facility.CleanerUnits {
		return WashResult{}, fmt.Errorf("wash requirements not satisfied")
	}
	out := s.Clone()
	for _, definition := range m.Garments[s.Definition].Layers {
		material := m.Materials[definition.Material]
		layer := s.Layers[definition.ID]
		layer.Dirt = washed(layer.Dirt, facility.Removal.Dirt, material.Washability.Dirt, request.WaterQuality, facility.DirtyWaterDeposit.Dirt)
		layer.Blood = washed(layer.Blood, facility.Removal.Blood, material.Washability.Blood, request.WaterQuality, facility.DirtyWaterDeposit.Blood)
		layer.Sanitary = washed(layer.Sanitary, facility.Removal.Sanitary, material.Washability.Sanitary, request.WaterQuality, facility.DirtyWaterDeposit.Sanitary)
		layer.Wetness = facility.WetnessAfter
		if request.WaterQuality < 1000 && ((1000-request.WaterQuality)*facility.DirtyWaterDeposit.Dirt/1000 > 0 || (1000-request.WaterQuality)*facility.DirtyWaterDeposit.Blood/1000 > 0 || (1000-request.WaterQuality)*facility.DirtyWaterDeposit.Sanitary/1000 > 0) {
			layer.LastStainTick = request.NowTick
			layer.StainSource = "wash:" + request.Facility
		}
		if layer.Dirt == 0 && layer.Blood == 0 && layer.Sanitary == 0 {
			layer.StainSource = ""
			layer.LastStainTick = 0
		}
		out.Layers[definition.ID] = layer
	}
	out.UpdatedTick = request.NowTick
	return WashResult{State: out, SpentWaterML: facility.WaterML, SpentCleanerUnits: facility.CleanerUnits, DurationTicks: facility.DurationTicks}, nil
}
