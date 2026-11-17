package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"cuelang.org/go/cue"
)

// TrainMotionCatalog projects one persistent route; its dispatch uses the shared Life timetable.
// Distances are metres on one closed, ordered path; bindings only project it.
type TrainMotionCatalog struct {
	BerthDwellSeconds float64            `json:"berth_dwell_seconds"`
	BerthEdgeID       string             `json:"berth_edge_id"`
	BoardingDoorGroup string             `json:"boarding_door_group"`
	DoorTravelM       float64            `json:"door_travel_m"`
	BerthLevelID      string             `json:"berth_level_id"`
	Timetable         *TrainTimetable    `json:"timetable,omitempty"`
	AccelerationMPS2  float64            `json:"acceleration_mps2"`
	BrakingMPS2       float64            `json:"braking_mps2"`
	BerthDistanceM    float64            `json:"berth_distance_m"`
	DoorCloseSeconds  float64            `json:"door_close_seconds"`
	NameKey           string             `json:"name_key,omitempty"`
	AdditionalFleet   []TrainMotionStock `json:"additional_fleet,omitempty"`
	PrefabResource    string             `json:"prefab_resource"`
	ClockDomain       string             `json:"clock_domain"`
	TrainID           string             `json:"train_id"`
	StockID           string             `json:"stock_id"`
	RouteID           string             `json:"route_id"`
	RouteVersion      int                `json:"route_version"`
	SpeedMPS          float64            `json:"speed_mps"`
	Points            []TrainRoutePoint  `json:"points"`
	Modules           []TrainModule      `json:"modules"`
	Bindings          []TrainBinding     `json:"bindings"`
	// Train v3: who rides inside each passenger car and how it is lit (optional; legacy catalogues omit it).
	Cabin *TrainCabin `json:"cabin,omitempty"`
	// Market polish v1 (design/market-polish/v1/BRIEF.md B2): speed limits on stretches of the route.
	SlowSections []TrainSlowSection `json:"slow_sections,omitempty"`
}

// TrainSlowSection: a speed limit on route metres [StartM, EndM] seen from one level (its binding): the whole train runs at
// SpeedMPS while any car is on it — the head enters at the limit, braking before it at the catalog's braking, the tail
// leaves it before the pull-away at the catalog's acceleration (the server's lineProfile, the client's train_link.gd).
type TrainSlowSection struct {
	ID       string  `json:"id"`
	LevelID  string  `json:"level_id"`
	StartM   float64 `json:"start_m"`
	EndM     float64 `json:"end_m"`
	SpeedMPS float64 `json:"speed_mps"`
	Why      string  `json:"why"`
}

// Timetable data is copied from transit.c7_loop by CUE, never assigned here.
type TrainTimetable struct {
	PeriodSeconds             int64 `json:"period_seconds"`
	NightPeriodSeconds        int64 `json:"night_period_seconds"`
	DayStartSecond            int64 `json:"day_start_second"`
	DayEndSecond              int64 `json:"day_end_second"`
	OffsetSeconds             int64 `json:"offset_seconds"`
	BoardingWindowSeconds     int64 `json:"boarding_window_seconds"`
	FreightOffsetSeconds      int64 `json:"freight_offset_seconds"`
	FreightNightOffsetSeconds int64 `json:"freight_night_offset_seconds"`
	// Market polish v1: the freight's own day period (0: the passenger's; then its day offset stays inside the passenger
	// period) and the clear time on the shared track between a passenger run and a freight path the passenger yields to
	// (physical seconds; the dispatcher's rule is in templates/train_dispatch.go.tmpl).
	FreightPeriodSeconds int64   `json:"freight_period_seconds,omitempty"`
	FreightGapSeconds    float64 `json:"freight_gap_seconds,omitempty"`
}

// Additional stock shares the route, module geometry and physical speed; only
// persistent identity, resource and initial phase differ. Legacy single stock
// catalogues omit this list and retain their original behavior.
type TrainMotionStock struct {
	NameKey        string `json:"name_key,omitempty"`
	PrefabResource string `json:"prefab_resource"`
	TrainID        string `json:"train_id"`
	StockID        string `json:"stock_id"`
}

type TrainRoutePoint struct {
	X float64 `json:"x"`
	Z float64 `json:"z"`
}
type TrainModule struct {
	ID            string    `json:"id"`
	OffsetM       float64   `json:"offset_m"`
	WidthM        float64   `json:"width_m"`
	LengthM       float64   `json:"length_m"`
	BogieYM       float64   `json:"bogie_y_m"`
	BogieOffsetsM []float64 `json:"bogie_offsets_m"`
}
type TrainBinding struct {
	NameKey         string  `json:"name_key,omitempty"`
	ApproachSeconds float64 `json:"approach_seconds,omitempty"`
	FloorElevationM float64 `json:"floor_elevation_m,omitempty"`
	PassageID       string  `json:"passage_id,omitempty"`
	LevelID         string  `json:"level_id"`
	StartM          float64 `json:"start_m"`
	EndM            float64 `json:"end_m"`
	X               float64 `json:"x"`
	Y               float64 `json:"y"`
	Z               float64 `json:"z"`
	YawDegrees      float64 `json:"yaw_degrees"`
	// All v1 bindings are exterior pass-throughs, not boarding or cargo interfaces.
	EntryAllowed bool `json:"entry_allowed"`
}

func readTrainMotion(world cue.Value, life json.RawMessage) (*TrainMotionCatalog, error) {
	v := world.LookupPath(cue.ParsePath("train_motion"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.train_motion: %w", err)
	}
	var c TrainMotionCatalog
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err = d.Decode(&c); err != nil {
		return nil, fmt.Errorf("world.train_motion: %w", err)
	}
	if err = c.validate(life); err != nil {
		return nil, fmt.Errorf("world.train_motion: %w", err)
	}
	if err = c.deriveCabinRiderModule(life); err != nil {
		return nil, fmt.Errorf("world.train_motion: %w", err)
	}
	keys := append([]string{c.NameKey}, trainHintKeys(c)...)
	for _, key := range keys {
		if key == "" {
			continue
		}
		var localization LocalizationCatalog
		if err := world.LookupPath(cue.ParsePath("localization")).Decode(&localization); err != nil {
			return nil, fmt.Errorf("train hint localization: %w", err)
		}
		found := false
		for _, message := range localization.Messages {
			if message.Key == key && message.Texts["ru"] != "" && message.Texts["en"] != "" {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("train hint localization key %s requires RU/EN", key)
		}
	}
	return &c, nil
}
func (c TrainMotionCatalog) validate(life json.RawMessage) error {
	for _, key := range append([]string{c.NameKey}, trainHintKeys(c)...) {
		if key != "" && (!strings.HasPrefix(key, "ui.") || len(key) > 128) {
			return fmt.Errorf("train hints require a ui.* localization key")
		}
	}
	for _, b := range c.Bindings {
		if math.IsNaN(b.ApproachSeconds) || math.IsInf(b.ApproachSeconds, 0) || b.ApproachSeconds < 0 || b.ApproachSeconds > 30 || (b.ApproachSeconds > 0 && b.NameKey == "") {
			return fmt.Errorf("invalid train approach cue")
		}
	}
	if c.BerthDwellSeconds <= 0 || c.BerthDwellSeconds > 300 || math.IsNaN(c.BerthDwellSeconds) || math.IsInf(c.BerthDwellSeconds, 0) {
		return fmt.Errorf("physical berth dwell required")
	}
	if c.Timetable == nil {
		return fmt.Errorf("operational train timetable required")
	}
	if c.BoardingDoorGroup == "" || len(c.BoardingDoorGroup) > 96 || strings.IndexFunc(c.BoardingDoorGroup, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_')
	}) >= 0 {
		return fmt.Errorf("boarding_door_group: explicit authored door group required")
	}
	if c.Timetable != nil {
		t := c.Timetable
		if t.PeriodSeconds <= 0 || t.NightPeriodSeconds <= 0 || t.DayStartSecond < 0 || t.DayEndSecond <= t.DayStartSecond || t.DayEndSecond >= 86400 || t.OffsetSeconds < 0 || t.OffsetSeconds >= t.PeriodSeconds || t.OffsetSeconds >= t.NightPeriodSeconds || t.BoardingWindowSeconds <= 0 || t.FreightOffsetSeconds < 0 || t.FreightOffsetSeconds >= t.PeriodSeconds && t.FreightPeriodSeconds == 0 || t.FreightOffsetSeconds >= t.NightPeriodSeconds {
			return fmt.Errorf("invalid operational train timetable")
		}
		if t.FreightPeriodSeconds != 0 && (t.FreightPeriodSeconds < t.PeriodSeconds || t.FreightPeriodSeconds > t.DayEndSecond-t.DayStartSecond || t.FreightOffsetSeconds >= t.FreightPeriodSeconds) {
			return fmt.Errorf("freight_period_seconds %d: one passenger period or more, at most the day, longer than the freight offset %d", t.FreightPeriodSeconds, t.FreightOffsetSeconds)
		}
		if math.IsNaN(t.FreightGapSeconds) || t.FreightGapSeconds < 0 || t.FreightGapSeconds > 120 {
			return fmt.Errorf("freight_gap_seconds %v is 0..120", t.FreightGapSeconds)
		}
		if (math.IsNaN(c.DoorTravelM) || math.IsInf(c.DoorTravelM, 0)) || c.DoorTravelM <= 0 || c.DoorTravelM > 3 || c.AccelerationMPS2 <= 0 || c.AccelerationMPS2 > 10 || math.IsNaN(c.AccelerationMPS2) || c.BrakingMPS2 <= 0 || c.BrakingMPS2 > 10 || math.IsNaN(c.BrakingMPS2) || c.BerthDistanceM <= c.SpeedMPS*c.SpeedMPS/(2*c.BrakingMPS2) || c.DoorCloseSeconds <= 0 || c.DoorCloseSeconds > 10 {
			return fmt.Errorf("invalid berth or acceleration/braking/door timing")
		}
	}
	if len(c.PrefabResource) > 256 || !locationSDKSurfaceResourceKey.MatchString(c.PrefabResource) {
		return fmt.Errorf("unsafe extensionless train prefab Resources path")
	}
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && math.Abs(v) <= 1e6 }
	if c.ClockDomain != "simulation" || c.TrainID == "" || c.StockID == "" || c.RouteID == "" || c.RouteVersion < 1 || !finite(c.SpeedMPS) || c.SpeedMPS <= 0 || c.SpeedMPS > 30 {
		return fmt.Errorf("invalid identity, epoch or motion")
	}
	if len(c.Points) < 4 || len(c.Points) > 256 || c.Points[0] != c.Points[len(c.Points)-1] {
		return fmt.Errorf("route must be a closed ordered metric path")
	}
	length := 0.0
	cuts := []float64{0}
	for i, p := range c.Points {
		if !finite(p.X) || !finite(p.Z) {
			return fmt.Errorf("non-finite route point")
		}
		if i > 0 {
			q := c.Points[i-1]
			d := math.Hypot(p.X-q.X, p.Z-q.Z)
			if d < 1 {
				return fmt.Errorf("degenerate route segment")
			}
			length += d
			cuts = append(cuts, length)
		}
	}
	if length > 1e6 || len(c.Modules) == 0 || len(c.Modules) > 16 {
		return fmt.Errorf("invalid route length or modules")
	}
	ids := map[string]bool{}
	tail := 0.0
	for i, m := range c.Modules {
		if m.ID == "" || ids[m.ID] || !finite(m.OffsetM) || m.OffsetM > 0 || !finite(m.LengthM) || m.LengthM <= 0 || m.LengthM > 100 || !finite(m.WidthM) || m.WidthM <= 0 || m.WidthM > 20 || len(m.BogieOffsetsM) != 2 || !finite(m.BogieYM) {
			return fmt.Errorf("invalid or duplicate module")
		}
		if i == 0 && m.OffsetM != 0 {
			return fmt.Errorf("leading module must define distance zero")
		}
		if i > 0 {
			previous := c.Modules[i-1]
			if previous.OffsetM-m.OffsetM < (previous.LengthM+m.LengthM)/2 {
				return fmt.Errorf("modules overlap or are out of order")
			}
		}
		ids[m.ID] = true
		for _, b := range m.BogieOffsetsM {
			if !finite(b) || math.Abs(b) >= m.LengthM/2 {
				return fmt.Errorf("bogie outside module")
			}
		}
		if m.BogieOffsetsM[0] <= m.BogieOffsetsM[1] {
			return fmt.Errorf("bogies must be ordered front to rear")
		}
		tail = math.Max(tail, -m.OffsetM+m.LengthM/2)
	}
	if len(c.AdditionalFleet) > 1 {
		return fmt.Errorf("one passenger stock and at most one freight stock per occupied route")
	}
	fleet := append([]TrainMotionStock{{PrefabResource: c.PrefabResource, TrainID: c.TrainID, StockID: c.StockID}}, c.AdditionalFleet...)
	trainIDs, stockIDs := map[string]bool{}, map[string]bool{}
	for _, stock := range fleet {
		if stock.TrainID == "" || stock.StockID == "" || trainIDs[stock.TrainID] || stockIDs[stock.StockID] {
			return fmt.Errorf("duplicate or empty persistent train/stock identity")
		}
		if len(stock.PrefabResource) > 256 || !locationSDKSurfaceResourceKey.MatchString(stock.PrefabResource) {
			return fmt.Errorf("unsafe fleet prefab Resources path")
		}
		trainIDs[stock.TrainID] = true
		stockIDs[stock.StockID] = true

	}
	if tail*3 >= length {
		return fmt.Errorf("train is too long for route")
	}
	var l struct {
		Locations map[string]json.RawMessage `json:"locations"`
		Edges     []struct {
			ID   string `json:"id"`
			From string `json:"from"`
			Mode string `json:"mode"`
		} `json:"edges"`
	}
	if err := json.Unmarshal(life, &l); err != nil {
		return fmt.Errorf("Life locations required: %w", err)
	}
	if c.BerthEdgeID != "" {
		found := false
		for _, e := range l.Edges {
			if e.ID == c.BerthEdgeID && e.From == c.BerthLevelID && e.Mode == "rail" {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("berth edge must be an existing rail exit of its level")
		}
	}
	levels := map[string]bool{}
	for i, b := range c.Bindings {
		if b.LevelID == "" || levels[b.LevelID] || l.Locations[b.LevelID] == nil || b.EntryAllowed {
			return fmt.Errorf("binding must reference distinct existing exterior level")
		}
		if i > 0 && c.Bindings[i-1].EndM > b.StartM {
			return fmt.Errorf("bindings must follow the ordered route without overlap")
		}
		levels[b.LevelID] = true
		if !finite(b.StartM) || !finite(b.EndM) || b.StartM < 0 || b.EndM <= b.StartM || b.EndM > length || b.EndM-b.StartM < tail || !finite(b.X) || !finite(b.Y) || !finite(b.FloorElevationM) || !finite(b.Z) || !finite(b.YawDegrees) {
			return fmt.Errorf("invalid metric binding")
		}
		// V1 exterior views cannot contain an unengineered sharp corner.
		for _, cut := range cuts {
			if cut > b.StartM && cut < b.EndM {
				return fmt.Errorf("visible binding crosses route corner")
			}
		}
	}
	berth := false
	for _, b := range c.Bindings {
		if b.LevelID == c.BerthLevelID {
			berth = c.BerthDistanceM-tail >= b.StartM && c.BerthDistanceM+c.Modules[0].LengthM/2 <= b.EndM
		}
	}
	if !berth {
		return fmt.Errorf("stopped consist must fit its authored berth binding")
	}
	if len(levels) < 3 {
		return fmt.Errorf("at least three distinct level bindings required")
	}
	if err := c.validateSlowSections(length, tail); err != nil {
		return err
	}
	return c.validateCabin()
}

// validateSlowSections: each limit is a real slowdown on a stretch its level shows, in route order, and the run reaches line
// speed between them: the first braking begins after the passenger train has pulled away from the berth to line speed (the
// approach to the berth and the freight's first metres stay at line speed), each next one after the previous pull-away, the
// last pull-away inside the route. The whole train is slow from its head entering to its tail leaving (front, tail metres).
func (c TrainMotionCatalog) validateSlowSections(length, tail float64) error {
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
	after := c.BerthDistanceM + c.SpeedMPS*c.SpeedMPS/(2*c.AccelerationMPS2)
	ids := map[string]bool{}
	front := c.Modules[0].LengthM / 2
	for _, s := range c.SlowSections {
		if s.ID == "" || ids[s.ID] || strings.TrimSpace(s.Why) == "" {
			return fmt.Errorf("slow section: id (unique) and why required")
		}
		ids[s.ID] = true
		if !finite(s.StartM) || !finite(s.EndM) || !finite(s.SpeedMPS) || s.SpeedMPS <= 0 || s.SpeedMPS >= c.SpeedMPS || s.EndM <= s.StartM {
			return fmt.Errorf("slow section %s: a limit below the line speed %v m/s on a stretch [start, end)", s.ID, c.SpeedMPS)
		}
		seen := false
		for _, b := range c.Bindings {
			if b.LevelID == s.LevelID && s.StartM >= b.StartM && s.EndM <= b.EndM {
				seen = true
			}
		}
		if !seen {
			return fmt.Errorf("slow section %s: must lie on the route stretch its level %s shows", s.ID, s.LevelID)
		}
		drop := c.SpeedMPS*c.SpeedMPS - s.SpeedMPS*s.SpeedMPS
		brake := s.StartM - front - drop/(2*c.BrakingMPS2)
		if brake < after-1e-9 {
			return fmt.Errorf("slow section %s: braking for it begins at %.1f m, before the run is at line speed (%.1f m)", s.ID, brake, after)
		}
		after = s.EndM + tail + drop/(2*c.AccelerationMPS2)
		if after > length {
			return fmt.Errorf("slow section %s: the pull-away after it ends past the route (%.1f of %.1f m)", s.ID, after, length)
		}
	}
	return nil
}

func trainHintKeys(c TrainMotionCatalog) []string {
	var keys []string
	for _, stock := range c.AdditionalFleet {
		keys = append(keys, stock.NameKey)
	}
	for _, binding := range c.Bindings {
		keys = append(keys, binding.NameKey)
	}
	return keys
}

// Painted bindings name their own rail aperture; a berth is never an exterior
// motion target. Native SDK bindings have no painted passage identifier.
func (c TrainMotionCatalog) validatePainted(scenes []PaintedScene) error {
	bound := map[string]bool{}
	for _, b := range c.Bindings {
		var scene *PaintedScene
		for i := range scenes {
			if scenes[i].LevelID == b.LevelID {
				scene = &scenes[i]
				break
			}
		}
		if scene == nil {
			if b.PassageID != "" {
				return fmt.Errorf("native binding %s cannot name a painted passage", b.LevelID)
			}
			continue
		}
		count := 0
		for _, p := range scene.AmbientPassages {
			if p.ID != b.PassageID {
				continue
			}
			count++
			if p.MetricRail == nil || p.StopProgress != 0 || p.DwellSeconds != 0 || len(p.BerthDoors) > 0 {
				return fmt.Errorf("train binding %s must target a metric pass-through, never passenger berth", b.LevelID)
			}
			if scene.FloorElevationM == nil || math.Abs(*scene.FloorElevationM+b.Y-p.ElevationM) > .001 || (b.FloorElevationM != 0 && math.Abs(b.FloorElevationM-*scene.FloorElevationM) > .001) {
				return fmt.Errorf("train binding %s has inconsistent relative floor elevation", b.LevelID)
			}
		}
		if b.PassageID == "" || count != 1 {
			return fmt.Errorf("train binding %s requires one exact existing passage_id", b.LevelID)
		}
		bound[b.LevelID+"/"+b.PassageID] = true
	}
	for _, s := range scenes {
		for _, p := range s.AmbientPassages {
			if p.MotionSource == "train_motion" && !bound[s.LevelID+"/"+p.ID] {
				return fmt.Errorf("external rail %s/%s has no train owner", s.LevelID, p.ID)
			}
		}
	}
	return nil
}
