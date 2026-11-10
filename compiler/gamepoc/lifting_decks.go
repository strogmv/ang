package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"cuelang.org/go/cue"
)

type LiftingDeckCatalog struct {
	Decks []LiftingDeckDefinition `json:"decks"`
}

type LiftingDeckFreightDefinition struct {
	Resource        string  `json:"resource"`
	YawDegrees      float64 `json:"yaw_degrees"`
	FirstSecond     int64   `json:"first_second"`
	PeriodSeconds   int64   `json:"period_seconds"`
	UnloadAfterS    int64   `json:"unload_after_s"`
	ReceivingDwellS int64   `json:"receiving_dwell_s"`
	Routes          []struct {
		DeliveryID     string    `json:"delivery_id"`
		CopperStore    string    `json:"copper_store"`
		DeckStore      string    `json:"deck_store"`
		ReceivingStore string    `json:"receiving_store"`
		Quantity       int64     `json:"quantity"`
		DeckPlan       []float64 `json:"deck_plan_m"`
		ReceivingPlan  []float64 `json:"receiving_plan_m"`
	} `json:"routes"`
}

type LiftingDeckDefinition struct {
	Freight              *LiftingDeckFreightDefinition `json:"freight,omitempty"`
	ID                   string                        `json:"id"`
	LevelID              string                        `json:"level_id"`
	InstanceID           string                        `json:"instance_id"`
	NameKey              string                        `json:"name_key"`
	Access               string                        `json:"access"`
	Manual               bool                          `json:"manual"`
	ClosedWalkable       bool                          `json:"closed_walkable"`
	Rect                 struct{ X, Y, W, H int }      `json:"rect"`
	BarrierRect          struct{ X, Y, W, H int }      `json:"barrier_rect"`
	BarrierFloor         []string                      `json:"barrier_floor,omitempty"`
	Approach             struct{ X, Y int }            `json:"approach"`
	ControlPlan          []float64                     `json:"control_plan_m"`
	Plan                 []float64                     `json:"plan_m"`
	DepthM               float64                       `json:"depth_m"`
	ChamberSpeedMPS      float64                       `json:"chamber_speed_mps"`
	LockdownSpeedMPS     float64                       `json:"lockdown_speed_mps"`
	PowerSpeedMPS        float64                       `json:"power_speed_mps"`
	ShaftDepthM          float64                       `json:"shaft_depth_m"`
	ShaftSpeedMPS        float64                       `json:"shaft_speed_mps"`
	TravelEdge           string                        `json:"travel_edge"`
	ReturnEdge           string                        `json:"return_edge"`
	SignalBeforeOpenS    int64                         `json:"signal_before_open_s"`
	SignalBeforeDescendS int64                         `json:"signal_before_descend_s"`
	Runs                 struct {
		PeriodSeconds  int64   `json:"period_seconds"`
		DayStartSecond int64   `json:"day_start_second"`
		DayEndSecond   int64   `json:"day_end_second"`
		ExtraSeconds   []int64 `json:"extra_seconds"`
	} `json:"runs"`
	Timing struct {
		Unlock          int64 `json:"unlock"`
		Unseat          int64 `json:"unseat"`
		Reseat          int64 `json:"reseat"`
		Lock            int64 `json:"lock"`
		BeaconPeriod    int64 `json:"beacon_period"`
		Equalise        int64 `json:"equalise"`
		Barrier         int64 `json:"barrier"`
		Open            int64 `json:"open"`
		Loading         int64 `json:"loading"`
		Close           int64 `json:"close"`
		LockdownWarning int64 `json:"lockdown_warning"`
		LockdownBarrier int64 `json:"lockdown_barrier"`
		LockdownClose   int64 `json:"lockdown_close"`
		Retry           int64 `json:"retry"`
	} `json:"timing_ms"`
	Sounds    []LiftingDeckSound    `json:"sounds,omitempty"`
	Nodes     []LiftingDeckNode     `json:"nodes"`
	TextSlots []LiftingDeckTextSlot `json:"text_slots"`
}
type LiftingDeckSound struct {
	Node     string    `json:"node"`
	Channel  string    `json:"channel"`
	Resource string    `json:"resource"`
	Offset   []float64 `json:"offset_m"`
	VolumeDB float64   `json:"volume_db"`
	Loop     bool      `json:"loop"`
}

type LiftingDeckNode struct {
	Name     string  `json:"name"`
	Channel  string  `json:"channel"`
	Axis     string  `json:"axis"`
	Min      float64 `json:"min"`
	Max      float64 `json:"max"`
	Rotation bool    `json:"rotation"`
}

type LiftingDeckTextSlot struct {
	Node            string    `json:"node"`
	Field           string    `json:"field"`
	Key             string    `json:"key"`
	WidthMM         int       `json:"width_mm"`
	HeightMM        int       `json:"height_mm"`
	RotationDegrees []float64 `json:"rotation_degrees"`
}

func readLiftingDecks(world cue.Value, life json.RawMessage) (*LiftingDeckCatalog, error) {
	v := world.LookupPath(cue.ParsePath("lifting_decks"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.lifting_decks: %w", err)
	}
	var c LiftingDeckCatalog
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err = d.Decode(&c); err != nil {
		return nil, fmt.Errorf("world.lifting_decks: %w", err)
	}
	if err = c.validate(life); err != nil {
		return nil, fmt.Errorf("world.lifting_decks: %w", err)
	}

	hasFreight := false
	for _, d := range c.Decks {
		hasFreight = hasFreight || d.Freight != nil
	}
	if hasFreight {
		var commerce struct {
			Deliveries []struct {
				ID          string `json:"id"`
				Carrier     string `json:"carrier"`
				SourceStore string `json:"source_store"`
				Quantity    int64  `json:"quantity"`
			} `json:"deliveries"`
			Stores []struct {
				ID    string `json:"id"`
				Owner string `json:"owner"`
			} `json:"business_stores"`
		}
		if err := world.LookupPath(cue.ParsePath("inventory")).Decode(&commerce); err != nil {
			return nil, fmt.Errorf("lifting-deck freight inventory: %w", err)
		}
		owners := map[string]string{}
		for _, store := range commerce.Stores {
			owners[store.ID] = store.Owner
		}
		for _, d := range c.Decks {
			if d.Freight == nil {
				continue
			}
			for _, route := range d.Freight.Routes {
				found := false
				for _, delivery := range commerce.Deliveries {
					if delivery.ID != route.DeliveryID {
						continue
					}
					found = true
					if route.ReceivingStore != delivery.SourceStore || route.Quantity != delivery.Quantity || owners[route.CopperStore] != delivery.Carrier || owners[route.DeckStore] != delivery.Carrier || owners[route.ReceivingStore] != delivery.Carrier {
						return nil, fmt.Errorf("lifting deck %s: freight must transfer the same existing owned consignment %s", d.ID, route.DeliveryID)
					}
				}
				if !found {
					return nil, fmt.Errorf("lifting deck %s: unknown existing freight delivery %s", d.ID, route.DeliveryID)
				}
			}
		}
	}
	// The live overlay is applied repeatedly to a session model. Keep the
	// authored bytes so lowering a barrier restores its floor, including an
	// intentionally blocked legacy market footprint.
	var base struct {
		Locations map[string]struct{ Terrain []string }
	}
	if err = json.Unmarshal(life, &base); err != nil {
		return nil, err
	}
	for i := range c.Decks {
		d := &c.Decks[i]
		d.BarrierFloor = make([]string, d.BarrierRect.H)
		for y := range d.BarrierFloor {
			d.BarrierFloor[y] = base.Locations[d.LevelID].Terrain[d.BarrierRect.Y+y][d.BarrierRect.X : d.BarrierRect.X+d.BarrierRect.W]
		}
	}
	var translations LocalizationCatalog
	if err = world.LookupPath(cue.ParsePath("localization")).Decode(&translations); err != nil {
		return nil, fmt.Errorf("lifting-deck localization: %w", err)
	}
	keys := map[string]bool{}
	for _, message := range translations.Messages {
		if message.Context != "" && message.Texts["ru"] != "" && message.Texts["en"] != "" {
			keys[message.Key] = true
		}
	}
	for _, deck := range c.Decks {
		if !keys[deck.NameKey] {
			return nil, fmt.Errorf("lifting deck %s: missing RU/EN/context for %s", deck.ID, deck.NameKey)
		}
		for _, slot := range deck.TextSlots {
			if !keys[slot.Key] {
				return nil, fmt.Errorf("lifting deck %s: missing RU/EN/context for text slot %s", deck.ID, slot.Key)
			}
		}
	}
	return &c, nil
}
func (c LiftingDeckCatalog) validate(life json.RawMessage) error {
	var l struct {
		Locations map[string]struct{ Terrain []string }
		Edges     []struct {
			ID, From, To  string
			TravelSeconds int64 `json:"travel_seconds"`
		}
	}
	if err := json.Unmarshal(life, &l); err != nil {
		return err
	}
	ids := map[string]bool{}
	instances := map[string]bool{}
	for _, d := range c.Decks {
		if d.ID == "" || ids[d.ID] || d.LevelID == "" || d.InstanceID == "" || instances[d.LevelID+"/"+d.InstanceID] || !strings.HasPrefix(d.NameKey, "ui.") || (d.Access != "open" && d.Access != "permit") {
			return fmt.Errorf("invalid identity/access for %s", d.ID)
		}
		ids[d.ID] = true
		instances[d.LevelID+"/"+d.InstanceID] = true
		level, ok := l.Locations[d.LevelID]
		if !ok || len(level.Terrain) == 0 {
			return fmt.Errorf("%s: unknown/empty level", d.ID)
		}
		if d.Rect.X < 0 || d.Rect.Y < 0 || d.Rect.W <= 0 || d.Rect.H <= 0 || d.Rect.X+d.Rect.W > len(level.Terrain[0]) || d.Rect.Y+d.Rect.H > len(level.Terrain) {
			return fmt.Errorf("%s: opening outside level", d.ID)
		}
		if d.BarrierRect.X < 0 || d.BarrierRect.Y < 0 || d.BarrierRect.W <= 0 || d.BarrierRect.H <= 0 || d.BarrierRect.X+d.BarrierRect.W > len(level.Terrain[0]) || d.BarrierRect.Y+d.BarrierRect.H > len(level.Terrain) {
			return fmt.Errorf("%s: barrier outside level", d.ID)
		}
		if d.Rect.X < d.BarrierRect.X+d.BarrierRect.W && d.BarrierRect.X < d.Rect.X+d.Rect.W && d.Rect.Y < d.BarrierRect.Y+d.BarrierRect.H && d.BarrierRect.Y < d.Rect.Y+d.Rect.H {
			return fmt.Errorf("%s: barrier floor must not overlap the lifting aperture", d.ID)
		}
		if d.Approach.X < 0 || d.Approach.Y < 0 || d.Approach.Y >= len(level.Terrain) || d.Approach.X >= len(level.Terrain[0]) || level.Terrain[d.Approach.Y][d.Approach.X] != '.' {
			return fmt.Errorf("%s: clear supported approach required", d.ID)
		}
		if (d.Approach.X >= d.Rect.X && d.Approach.X < d.Rect.X+d.Rect.W && d.Approach.Y >= d.Rect.Y && d.Approach.Y < d.Rect.Y+d.Rect.H) || (d.Approach.X >= d.BarrierRect.X && d.Approach.X < d.BarrierRect.X+d.BarrierRect.W && d.Approach.Y >= d.BarrierRect.Y && d.Approach.Y < d.BarrierRect.Y+d.BarrierRect.H) {
			return fmt.Errorf("%s: waiting approach must be outside aperture and barrier", d.ID)
		}
		for y := d.Rect.Y; y < d.Rect.Y+d.Rect.H; y++ {
			for x := d.Rect.X; x < d.Rect.X+d.Rect.W; x++ {
				if level.Terrain[y][x] != '.' && (d.ClosedWalkable || level.Terrain[y][x] != '+') {
					return fmt.Errorf("%s: closed leaves need supported clear floor at (%d,%d)", d.ID, x, y)
				}
			}
		}
		if len(d.Plan) != 3 || len(d.ControlPlan) != 3 {
			return fmt.Errorf("%s: plan_m/control_plan_m require x,height,y", d.ID)
		}
		for _, n := range append(append([]float64{}, d.ControlPlan...), append(d.Plan, d.DepthM, d.ChamberSpeedMPS, d.LockdownSpeedMPS, d.PowerSpeedMPS, d.ShaftDepthM, d.ShaftSpeedMPS)...) {
			if math.IsNaN(n) || math.IsInf(n, 0) {
				return fmt.Errorf("%s: nonfinite geometry", d.ID)
			}
		}
		if d.DepthM <= 0 || d.ChamberSpeedMPS <= 0 || d.LockdownSpeedMPS <= 0 || d.PowerSpeedMPS <= 0 || d.ShaftDepthM <= 0 || d.ShaftSpeedMPS <= 0 {
			return fmt.Errorf("%s: positive travel/speed required", d.ID)
		}
		if d.Runs.PeriodSeconds < 1 || d.Runs.DayStartSecond < 0 || d.Runs.DayEndSecond <= d.Runs.DayStartSecond || d.Runs.DayEndSecond > 86400 {
			return fmt.Errorf("%s: invalid run calendar", d.ID)
		}
		if d.SignalBeforeOpenS < 10 || d.SignalBeforeOpenS > 3600 || d.SignalBeforeDescendS < 10 || d.SignalBeforeDescendS > 3600 {
			return fmt.Errorf("%s: at least ten physical seconds of normal warning required", d.ID)
		}
		for _, t := range []int64{d.Timing.Unlock, d.Timing.Unseat, d.Timing.Reseat, d.Timing.Lock, d.Timing.BeaconPeriod, d.Timing.Equalise, d.Timing.Barrier, d.Timing.Open, d.Timing.Loading, d.Timing.Close, d.Timing.LockdownWarning, d.Timing.LockdownBarrier, d.Timing.LockdownClose, d.Timing.Retry} {
			if t <= 0 || t > 3600000 {
				return fmt.Errorf("%s: invalid physical timing", d.ID)
			}
		}
		if f := d.Freight; f != nil {
			if !strings.HasPrefix(f.Resource, "res://") || strings.Contains(f.Resource, "..") || f.FirstSecond < 0 || f.FirstSecond >= 86400 || f.PeriodSeconds < 1 || f.UnloadAfterS < 1 || f.ReceivingDwellS < 1 || f.UnloadAfterS*1000 >= d.Timing.Loading || len(f.Routes) == 0 {
				return fmt.Errorf("%s: invalid physical freight definition", d.ID)
			}
			seen := map[string]bool{}
			for _, route := range f.Routes {
				if route.DeliveryID == "" || seen[route.DeliveryID] || route.CopperStore == "" || route.DeckStore == "" || route.ReceivingStore == "" || route.CopperStore == route.DeckStore || route.DeckStore == route.ReceivingStore || route.Quantity < 1 || len(route.DeckPlan) != 3 || len(route.ReceivingPlan) != 3 {
					return fmt.Errorf("%s: invalid finite freight route", d.ID)
				}
				seen[route.DeliveryID] = true
				for _, v := range append(append([]float64{}, route.DeckPlan...), route.ReceivingPlan...) {
					if math.IsInf(v, 0) || math.IsNaN(v) {
						return fmt.Errorf("%s: nonfinite freight pose", d.ID)
					}
				}
			}
		}

		for _, s := range d.Runs.ExtraSeconds {
			if s < 0 || s >= 86400 {
				return fmt.Errorf("%s: invalid extra run", d.ID)
			}
		}
		names := map[string]bool{}
		for _, n := range d.Nodes {
			if n.Name == "" || names[n.Name] || (n.Axis != "x" && n.Axis != "y" && n.Axis != "z") || (n.Channel != "leaves" && n.Channel != "deck" && n.Channel != "barrier" && n.Channel != "unlock" && n.Channel != "damper" && n.Channel != "release") {
				return fmt.Errorf("%s: invalid motion node %s", d.ID, n.Name)
			}
			names[n.Name] = true
		}
		for _, sound := range d.Sounds {
			if sound.Node == "" || (sound.Channel != "horn" && sound.Channel != "equalising" && sound.Channel != "equalising_start") || !strings.HasPrefix(sound.Resource, "res://assets/audio/") || !strings.HasSuffix(sound.Resource, ".wav") || strings.Contains(sound.Resource, "..") || len(sound.Offset) != 3 || math.IsNaN(sound.VolumeDB) || math.IsInf(sound.VolumeDB, 0) || sound.VolumeDB < -80 || sound.VolumeDB > 0 {
				return fmt.Errorf("%s: invalid sound binding", d.ID)
			}
			for _, value := range sound.Offset {
				if math.IsNaN(value) || math.IsInf(value, 0) {
					return fmt.Errorf("%s: nonfinite sound offset", d.ID)
				}
			}
		}
		slotNames := map[string]bool{}
		for _, slot := range d.TextSlots {
			if slot.Node == "" || slotNames[slot.Node] || slot.Key == "" || slot.WidthMM <= 0 || slot.HeightMM <= 0 || len(slot.RotationDegrees) != 3 || (slot.Field != "weight_text" && slot.Field != "depth_text") {
				return fmt.Errorf("%s: invalid text slot %s", d.ID, slot.Node)
			}
			slotNames[slot.Node] = true
		}
		for _, id := range []string{d.TravelEdge, d.ReturnEdge} {
			if id == "" {
				continue
			}
			found := false
			for _, e := range l.Edges {
				if e.ID == id {
					found = true
					if float64(e.TravelSeconds) != d.ShaftDepthM/d.ShaftSpeedMPS {
						return fmt.Errorf("%s: Life shaft time differs from geometry/speed", d.ID)
					}
				}
			}
			if !found {
				return fmt.Errorf("%s: unknown Life edge %s", d.ID, id)
			}
		}
	}
	return nil
}

// filterLiftingDeckPopulationAnchors keeps permanent crowd spawn/rest anchors
// off every moving aperture. The static closed-floor mesh still remains real
// walking floor; transient traversal is decided by the live navigation overlay.
func filterLiftingDeckPopulationAnchors(life json.RawMessage, c *LiftingDeckCatalog) (json.RawMessage, error) {
	if c == nil || len(c.Decks) == 0 {
		return life, nil
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(life, &root); err != nil {
		return nil, err
	}
	if len(root["population"]) == 0 {
		return life, nil
	}
	var population map[string]json.RawMessage
	if err := json.Unmarshal(root["population"], &population); err != nil {
		return nil, err
	}
	var anchors map[string][]struct {
		X int `json:"x"`
		Y int `json:"y"`
	}
	if err := json.Unmarshal(population["anchors"], &anchors); err != nil {
		return nil, err
	}
	changed := false
	for level, cells := range anchors {
		kept := cells[:0]
		for _, at := range cells {
			excluded := false
			for _, d := range c.Decks {
				if d.LevelID == level && ((at.X >= d.Rect.X && at.Y >= d.Rect.Y && at.X < d.Rect.X+d.Rect.W && at.Y < d.Rect.Y+d.Rect.H) || (at.X >= d.BarrierRect.X && at.Y >= d.BarrierRect.Y && at.X < d.BarrierRect.X+d.BarrierRect.W && at.Y < d.BarrierRect.Y+d.BarrierRect.H)) {
					excluded = true
					break
				}
			}
			if excluded {
				changed = true
			} else {
				kept = append(kept, at)
			}
		}
		anchors[level] = kept
	}
	if !changed {
		return life, nil
	}
	raw, err := json.Marshal(anchors)
	if err != nil {
		return nil, err
	}
	population["anchors"] = raw
	raw, err = json.Marshal(population)
	if err != nil {
		return nil, err
	}
	root["population"] = raw
	return json.Marshal(root)
}
