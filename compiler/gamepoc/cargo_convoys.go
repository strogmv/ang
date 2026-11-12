package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	inv "github.com/strogmv/ang/compiler/inventory/runtime"
)

// Cargo to the market v1, part B (design/cargo-market/v1/PART-B.md, coordinator's depth trim 11.11): a convoy is a tug
// that pulls one trailer of a fleet along a route of legs — drive on a level, ride a lifting deck, cross a vehicle-only
// Life edge (a timed transfer), work at a stand (load, unload, unhitch, hitch, charge) and wait while a crane swaps the
// trailers. The compiler rounds every track once and checks the whole route against the floor, the decks, the edges,
// the stands and the crane, so the runtime only walks finished polylines; no physics and no steering model at run time.
type CargoConvoyCatalog struct {
	Convoys []CargoConvoy `json:"convoys"`
	Stands  []CargoStand  `json:"stands"`
	Cranes  []CargoCrane  `json:"cranes"`
	Lots    []CargoLot    `json:"lots"`
	// floor height of every level the convoys touch, in the level's plan frame (the hub stands at +18)
	Floors          map[string]float64 `json:"floors"`
	DeliveryTrolley struct {
		Resource         string  `json:"resource"`
		CouplingForwardM float64 `json:"coupling_forward_m"`
	} `json:"delivery_trolley"`
}
type CargoPost struct {
	X int `json:"x"`
	Y int `json:"y"`
}
type CargoCrewAt struct {
	Seat bool       `json:"seat"`
	At   *CargoPost `json:"at,omitempty"`
}
type CargoSeat struct {
	ID   string     `json:"id"`
	M    [3]float64 `json:"m"`
	Side string     `json:"side"`
	Pose string     `json:"pose"`
}
type CargoPose struct {
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	YawDegrees float64 `json:"yaw_degrees"`
}
type CargoLeg struct {
	Kind        string        `json:"kind"`
	Label       string        `json:"label"`
	Posts       []CargoCrewAt `json:"posts,omitempty"`
	LevelID     string        `json:"level_id,omitempty"`
	Path        [][2]float64  `json:"path,omitempty"`
	Reverse     bool          `json:"reverse,omitempty"`
	TurnRadiusM float64       `json:"turn_radius_m,omitempty"`
	DeckID      string        `json:"deck_id,omitempty"`
	LifeEdge    string        `json:"life_edge,omitempty"`
	StandID     string        `json:"stand_id,omitempty"`
	Bay         string        `json:"bay,omitempty"`
	CraneID     string        `json:"crane_id,omitempty"`
	Moves       []struct {
		From string `json:"from"`
		To   string `json:"to"`
	} `json:"moves,omitempty"`
	FromSecond int64 `json:"from_second,omitempty"`
	Seconds    int64 `json:"seconds,omitempty"`
	// Compiled: the level the leg starts on, the level it ends on (a deck or an edge changes it) and whether a trailer
	// is hitched during the leg.
	Level   string `json:"level"`
	ToLevel string `json:"to_level"`
	Trailer bool   `json:"trailer"`
	rawEnd  [2][2]float64
}
type CargoConvoy struct {
	ID  string `json:"id"`
	Tug struct {
		Resource   string      `json:"resource"`
		HitchBackM float64     `json:"hitch_back_m"`
		NoseM      float64     `json:"nose_m"`
		RearM      float64     `json:"rear_m"`
		HalfWidthM float64     `json:"half_width_m"`
		MassKG     int64       `json:"mass_kg"`
		Seats      []CargoSeat `json:"seats"`
		ChargeM    [3]float64  `json:"charge_m"`
	} `json:"tug"`
	Trailer struct {
		Resource   string  `json:"resource"`
		EyeM       float64 `json:"eye_m"`
		TailM      float64 `json:"tail_m"`
		BodyFrontM float64 `json:"body_front_m"`
		HalfWidthM float64 `json:"half_width_m"`
		HeightM    float64 `json:"height_m"`
		MassKG     int64   `json:"mass_kg"`
	} `json:"trailer"`
	Trailers []struct {
		ID    string `json:"id"`
		Start struct {
			Hitched bool   `json:"hitched"`
			StandID string `json:"stand_id,omitempty"`
			Bay     string `json:"bay,omitempty"`
		} `json:"start"`
	} `json:"trailers"`
	SpeedMMPerSecond   int64   `json:"speed_mm_per_second"`
	ReverseMMPerSecond int64   `json:"reverse_mm_per_second"`
	TurnRadiusM        float64 `json:"turn_radius_m"`
	WalkMMPerSecond    int64   `json:"walk_mm_per_second"`
	RiderMassKG        int64   `json:"rider_mass_kg"`
	Crew               []struct {
		ID   string `json:"id"`
		Seat string `json:"seat"`
	} `json:"crew"`
	Home struct {
		LevelID    string     `json:"level_id"`
		At         [2]float64 `json:"at"`
		YawDegrees float64    `json:"yaw_degrees"`
	} `json:"home"`
	Gates []struct {
		LevelID string `json:"level_id"`
		GateID  string `json:"gate_id"`
	} `json:"gates"`
	Runs []struct {
		ID          string     `json:"id"`
		StartSecond int64      `json:"start_second"`
		Legs        []CargoLeg `json:"legs"`
	} `json:"runs"`
}
type CargoStand struct {
	ID      string `json:"id"`
	LevelID string `json:"level_id"`
	Bays    []struct {
		ID      string    `json:"id"`
		Trailer CargoPose `json:"trailer"`
		Rect    struct {
			X [2]float64 `json:"x"`
			Y [2]float64 `json:"y"`
		} `json:"rect"`
	} `json:"bays"`
	Marks []struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
		Rect struct {
			X [2]float64 `json:"x"`
			Y [2]float64 `json:"y"`
			Z [2]float64 `json:"z"`
		} `json:"rect"`
	} `json:"marks"`
	Lamps []struct {
		ID     string     `json:"id"`
		At     [3]float64 `json:"at"`
		RangeM float64    `json:"range_m"`
	} `json:"lamps"`
	ChargePostM [3]float64 `json:"charge_post_m"`
	LampsOn     struct {
		FromSecond int64 `json:"from_second"`
		ToSecond   int64 `json:"to_second"`
	} `json:"lamps_on"`
}
type CargoCrane struct {
	ID      string `json:"id"`
	LevelID string `json:"level_id"`
	StandID string `json:"stand_id"`
	Park    struct {
		BridgeYM float64 `json:"bridge_y_m"`
		HoistXM  float64 `json:"hoist_x_m"`
		HookZM   float64 `json:"hook_z_m"`
	} `json:"park"`
	Travel struct {
		BridgeYM [2]float64 `json:"bridge_y_m"`
		HoistXM  [2]float64 `json:"hoist_x_m"`
		HookZM   [2]float64 `json:"hook_z_m"`
	} `json:"travel"`
	Speed struct {
		BridgeMPS float64 `json:"bridge_mps"`
		HoistMPS  float64 `json:"hoist_mps"`
		HookMPS   float64 `json:"hook_mps"`
		TurnDPS   float64 `json:"turn_dps"`
	} `json:"speed"`
	AttachZM       float64 `json:"attach_z_m"`
	LiftM          float64 `json:"lift_m"`
	AttachSeconds  int64   `json:"attach_seconds"`
	ReleaseSeconds int64   `json:"release_seconds"`
	Band           struct {
		X int `json:"x"`
		Y int `json:"y"`
		W int `json:"w"`
		H int `json:"h"`
	} `json:"band"`
}
type CargoLotItem struct {
	Item string `json:"item"`
	Full int64  `json:"full"`
}
type CargoLot struct {
	Shop        string `json:"shop"`
	Owner       string `json:"owner"`
	Kind        string `json:"kind"`
	CopperStore string `json:"copper_store"`
	LockerStore string `json:"locker_store"`
	Quantity    int64  `json:"quantity,omitempty"`
	Copper      *struct {
		Level string   `json:"level"`
		At    inv.Cell `json:"at"`
	} `json:"copper,omitempty"`
	Locker *struct {
		Level string   `json:"level"`
		At    inv.Cell `json:"at"`
	} `json:"locker,omitempty"`
	// Compiled: the delivery of a delivery lot; the definitions and full counts of a supply lot (from the shop's own
	// supply rule: the visitors' consignment or the produce stall's heaps); the trailer store of each trailer.
	Delivery      string            `json:"delivery,omitempty"`
	Definition    string            `json:"definition,omitempty"`
	Items         []CargoLotItem    `json:"items,omitempty"`
	TrailerStores map[string]string `json:"trailer_stores"`
}

// cargoFillet rounds every inner corner of a track with the given radius and
// samples the arc every 7.5 degrees. A corner whose legs are too short for the
// radius is an authoring error: the convoy could not make that turn.
func cargoFillet(path [][2]float64, radius float64) ([][2]float64, error) {
	if len(path) < 2 {
		return nil, fmt.Errorf("a track needs two points")
	}
	out := [][2]float64{path[0]}
	used := 0.0 // length of the incoming leg already taken by the previous corner
	for i := 1; i < len(path)-1; i++ {
		a, b, c := path[i-1], path[i], path[i+1]
		ux, uy := b[0]-a[0], b[1]-a[1]
		vx, vy := c[0]-b[0], c[1]-b[1]
		lu, lv := math.Hypot(ux, uy), math.Hypot(vx, vy)
		if lu < 1e-9 || lv < 1e-9 {
			return nil, fmt.Errorf("repeated track point %d", i)
		}
		ux, uy, vx, vy = ux/lu, uy/lu, vx/lv, vy/lv
		cross, dot := ux*vy-uy*vx, ux*vx+uy*vy
		turn := math.Atan2(cross, dot)
		if math.Abs(turn) < 1e-6 {
			used = 0
			continue
		}
		if math.Abs(turn) > math.Pi-1e-3 {
			return nil, fmt.Errorf("track reverses at point %d", i)
		}
		cut := radius * math.Tan(math.Abs(turn)/2)
		if cut > lu-used+1e-9 || cut > lv+1e-9 {
			return nil, fmt.Errorf("corner %d needs %.2f m on each leg for a %.2f m radius", i, cut, radius)
		}
		sign := 1.0
		if turn < 0 {
			sign = -1
		}
		sx, sy := b[0]-ux*cut, b[1]-uy*cut
		cx, cy := sx-uy*radius*sign, sy+ux*radius*sign
		start := math.Atan2(sy-cy, sx-cx)
		steps := int(math.Ceil(math.Abs(turn) / (7.5 * math.Pi / 180)))
		for k := 0; k <= steps; k++ {
			t := start + turn*float64(k)/float64(steps)
			out = append(out, [2]float64{math.Round((cx+radius*math.Cos(t))*1e6) / 1e6, math.Round((cy+radius*math.Sin(t))*1e6) / 1e6})
		}
		used = cut
	}
	out = append(out, path[len(path)-1])
	clean := out[:1]
	for _, p := range out[1:] {
		if math.Hypot(p[0]-clean[len(clean)-1][0], p[1]-clean[len(clean)-1][1]) > 1e-6 {
			clean = append(clean, p)
		}
	}
	return clean, nil
}

func cargoTrackLength(track [][2]float64) float64 {
	n := 0.0
	for i := 1; i < len(track); i++ {
		n += math.Hypot(track[i][0]-track[i-1][0], track[i][1]-track[i-1][1])
	}
	return n
}

// cargoTrackAt: a point at distance s along a track; before the start and past the end the track continues straight.
func cargoTrackAt(track [][2]float64, s float64) (float64, float64) {
	if s <= 0 {
		dx, dy := track[1][0]-track[0][0], track[1][1]-track[0][1]
		l := math.Hypot(dx, dy)
		return track[0][0] + dx/l*s, track[0][1] + dy/l*s
	}
	for i := 1; i < len(track); i++ {
		dx, dy := track[i][0]-track[i-1][0], track[i][1]-track[i-1][1]
		l := math.Hypot(dx, dy)
		if s <= l || i == len(track)-1 {
			return track[i-1][0] + dx/l*s, track[i-1][1] + dy/l*s
		}
		s -= l
	}
	return track[len(track)-1][0], track[len(track)-1][1]
}

// cargoPoses: the tug's rear axle is on the track at s and looks along the chord (backwards on a reverse leg); a hitched
// trailer's rear axle follows the same track one drawbar behind and looks at the tug's hitch.
func (v CargoConvoy) cargoPoses(track [][2]float64, s float64, reverse bool) (CargoPose, CargoPose) {
	ax, ay := cargoTrackAt(track, s)
	fx, fy := cargoTrackAt(track, s+0.4)
	bx, by := cargoTrackAt(track, s-0.4)
	yaw := math.Atan2(fy-by, fx-bx)
	if reverse {
		yaw += math.Pi
	}
	hx, hy := ax-math.Cos(yaw)*v.Tug.HitchBackM, ay-math.Sin(yaw)*v.Tug.HitchBackM
	tx, ty := cargoTrackAt(track, s-v.Tug.HitchBackM-v.Trailer.EyeM)
	return CargoPose{ax, ay, cargoDegrees(yaw)}, CargoPose{tx, ty, cargoDegrees(math.Atan2(hy-ty, hx-tx))}
}
func cargoDegrees(rad float64) float64 { d := math.Mod(rad*180/math.Pi+720, 360); return d }
func cargoTurn(a, b float64) float64   { d := math.Mod(a-b+540, 360) - 180; return math.Abs(d) }

// cargoRectCells: the cells of a level whose middles lie inside a body rectangle: back..front along the heading, ±half
// across it, at the pose (plan metres; cell size in metres).
func cargoRectCells(p CargoPose, back, front, half, cell float64, add func(x, y int)) {
	yaw := p.YawDegrees * math.Pi / 180
	fx, fy := math.Cos(yaw), math.Sin(yaw)
	lx, ly := math.Sin(yaw), -math.Cos(yaw)
	minX, maxX, minY, maxY := math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)
	for _, a := range [2]float64{-back, front} {
		for _, b := range [2]float64{-half, half} {
			x, y := p.X+fx*a+lx*b, p.Y+fy*a+ly*b
			minX, maxX, minY, maxY = math.Min(minX, x), math.Max(maxX, x), math.Min(minY, y), math.Max(maxY, y)
		}
	}
	for cy := int(math.Floor(minY / cell)); cy <= int(math.Floor(maxY/cell)); cy++ {
		for cx := int(math.Floor(minX / cell)); cx <= int(math.Floor(maxX/cell)); cx++ {
			mx, my := (float64(cx)+0.5)*cell-p.X, (float64(cy)+0.5)*cell-p.Y
			along, across := mx*fx+my*fy, mx*lx+my*ly
			if along >= -back && along <= front && math.Abs(across) <= half {
				add(cx, cy)
			}
		}
	}
}

type cargoLifeData struct {
	Locations map[string]struct {
		Terrain []string `json:"terrain"`
		CellMM  int      `json:"cell_mm"`
		Gates   []struct {
			ID      string `json:"id"`
			Opening []struct {
				X int `json:"X"`
				Y int `json:"Y"`
			} `json:"opening"`
			Permits []struct {
				Vehicle string `json:"vehicle"`
			} `json:"permits"`
		} `json:"gates"`
	} `json:"locations"`
	Edges []struct {
		ID      string `json:"id"`
		From    string `json:"from"`
		To      string `json:"to"`
		Vehicle bool   `json:"vehicle"`
	} `json:"edges"`
	Actors []struct {
		ID    string `json:"id"`
		Steps []struct {
			Level string `json:"level"`
		} `json:"steps"`
	} `json:"actors"`
}

func readCargoConvoys(world cue.Value, c Compiled) (*CargoConvoyCatalog, error) {
	v := world.LookupPath(cue.ParsePath("cargo_convoys"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.cargo_convoys: %w", err)
	}
	var out CargoConvoyCatalog
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err = d.Decode(&out); err != nil {
		return nil, fmt.Errorf("world.cargo_convoys: %w", err)
	}
	if err = out.validate(c); err != nil {
		return nil, fmt.Errorf("world.cargo_convoys: %w", err)
	}
	return &out, nil
}

func (c *CargoConvoyCatalog) validate(w Compiled) error {
	var life cargoLifeData
	if err := json.Unmarshal(w.Life, &life); err != nil {
		return err
	}
	if !strings.HasPrefix(c.DeliveryTrolley.Resource, "res://") || strings.Contains(c.DeliveryTrolley.Resource, "..") || c.DeliveryTrolley.CouplingForwardM < .5 || c.DeliveryTrolley.CouplingForwardM > 1 {
		return fmt.Errorf("invalid delivery trolley")
	}
	cellOf := func(level string) float64 { return float64(life.Locations[level].CellMM) / 1000 }
	glyph := func(level string, x, y int) byte {
		t := life.Locations[level].Terrain
		if y < 0 || y >= len(t) || x < 0 || x >= len(t[y]) {
			return ' '
		}
		return t[y][x]
	}
	walks := func(level string, p CargoPost) bool { return glyph(level, p.X, p.Y) == '.' }
	stands := map[string]*CargoStand{}
	for i := range c.Stands {
		s := &c.Stands[i]
		if s.ID == "" || stands[s.ID] != nil || cellOf(s.LevelID) <= 0 || len(s.Bays) == 0 {
			return fmt.Errorf("invalid stand %q", s.ID)
		}
		stands[s.ID] = s
		seen := map[string]bool{}
		for _, b := range s.Bays {
			if b.ID == "" || seen[b.ID] || b.Rect.X[0] >= b.Rect.X[1] || b.Rect.Y[0] >= b.Rect.Y[1] {
				return fmt.Errorf("stand %s: invalid bay %q", s.ID, b.ID)
			}
			seen[b.ID] = true
		}
		for _, m := range s.Marks {
			if m.ID == "" || m.Rect.X[0] > m.Rect.X[1] || m.Rect.Y[0] > m.Rect.Y[1] || m.Rect.Z[0] > m.Rect.Z[1] {
				return fmt.Errorf("stand %s: invalid mark %q", s.ID, m.ID)
			}
		}
		if s.LampsOn.FromSecond < 0 || s.LampsOn.FromSecond >= s.LampsOn.ToSecond || s.LampsOn.ToSecond > 86400 {
			return fmt.Errorf("stand %s: invalid lamp hours", s.ID)
		}
	}
	bay := func(stand, id string) (CargoPose, bool) {
		s := stands[stand]
		if s == nil {
			return CargoPose{}, false
		}
		for _, b := range s.Bays {
			if b.ID == id {
				return b.Trailer, true
			}
		}
		return CargoPose{}, false
	}
	cranes := map[string]*CargoCrane{}
	for i := range c.Cranes {
		k := &c.Cranes[i]
		s := stands[k.StandID]
		if k.ID == "" || cranes[k.ID] != nil || s == nil || s.LevelID != k.LevelID || k.Speed.BridgeMPS <= 0 || k.Speed.HoistMPS <= 0 || k.Speed.HookMPS <= 0 || k.Speed.TurnDPS <= 0 || k.AttachSeconds < 1 || k.ReleaseSeconds < 1 || k.LiftM <= 0 || k.Band.W < 1 || k.Band.H < 1 {
			return fmt.Errorf("invalid crane %q", k.ID)
		}
		in := func(v float64, r [2]float64) bool { return v >= r[0]-1e-9 && v <= r[1]+1e-9 }
		if !in(k.Park.BridgeYM, k.Travel.BridgeYM) || !in(k.Park.HoistXM, k.Travel.HoistXM) || !in(k.Park.HookZM, k.Travel.HookZM) || !in(k.AttachZM, k.Travel.HookZM) || !in(k.AttachZM+k.LiftM, k.Travel.HookZM) {
			return fmt.Errorf("crane %s: park or lift outside its travel", k.ID)
		}
		cranes[k.ID] = k
	}
	lifeEdges := map[string]struct {
		from, to string
		vehicle  bool
	}{}
	for _, e := range life.Edges {
		lifeEdges[e.ID] = struct {
			from, to string
			vehicle  bool
		}{e.From, e.To, e.Vehicle}
	}
	home := map[string]string{}
	for _, a := range life.Actors {
		if len(a.Steps) > 0 {
			home[a.ID] = a.Steps[0].Level
		}
	}
	decks := map[string]*LiftingDeckDefinition{}
	if w.LiftingDecks != nil {
		for k := range w.LiftingDecks.Decks {
			decks[w.LiftingDecks.Decks[k].ID] = &w.LiftingDecks.Decks[k]
		}
	}
	for _, d := range decks {
		if d.Lower == nil || len(d.Plan) < 2 || len(d.Lower.Plan) < 2 {
			continue
		}
		for level, y := range map[string]float64{d.LevelID: d.Plan[1], d.Lower.LevelID: d.Lower.Plan[1]} {
			if f, ok := c.Floors[level]; ok && math.Abs(f-y) > 1e-6 {
				return fmt.Errorf("floor of %s is %.2f m, deck %s stands at %.2f m", level, f, d.ID, y)
			}
		}
	}
	people, ids, trailerIDs := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for ci := range c.Convoys {
		v := &c.Convoys[ci]
		if v.ID == "" || ids[v.ID] {
			return fmt.Errorf("duplicate or empty convoy id %q", v.ID)
		}
		ids[v.ID] = true
		for _, n := range []float64{v.Tug.HitchBackM, v.Tug.NoseM, v.Tug.RearM, v.Tug.HalfWidthM, v.Trailer.EyeM, v.Trailer.TailM, v.Trailer.BodyFrontM, v.Trailer.HalfWidthM, v.Trailer.HeightM, v.TurnRadiusM} {
			if !(n > 0 && n < 100) {
				return fmt.Errorf("%s: nonfinite or empty geometry", v.ID)
			}
		}
		if v.SpeedMMPerSecond < 20 || v.SpeedMMPerSecond > 400 || v.ReverseMMPerSecond < 20 || v.ReverseMMPerSecond > 400 || v.WalkMMPerSecond < 300 || v.Tug.MassKG < 1 || v.Trailer.MassKG < 1 || v.RiderMassKG < 1 || !strings.HasPrefix(v.Tug.Resource, "res://") || !strings.HasPrefix(v.Trailer.Resource, "res://") {
			return fmt.Errorf("%s: invalid vehicle definition", v.ID)
		}
		seats := map[string]bool{}
		for _, s := range v.Tug.Seats {
			if s.ID == "" || seats[s.ID] || (s.Side != "left" && s.Side != "right") || (s.Pose != "drive" && s.Pose != "sit" && s.Pose != "stand") {
				return fmt.Errorf("%s: invalid seat %q", v.ID, s.ID)
			}
			seats[s.ID] = true
		}
		if len(v.Crew) == 0 || len(v.Crew) > len(v.Tug.Seats) {
			return fmt.Errorf("%s: the crew needs a seat each", v.ID)
		}
		taken := map[string]bool{}
		for i, m := range v.Crew {
			if people[m.ID] || home[m.ID] != v.Home.LevelID || !seats[m.Seat] || taken[m.Seat] {
				return fmt.Errorf("%s: crew member %s must be one Life actor whose day starts at home and has a seat of his own", v.ID, m.ID)
			}
			if i == 0 && m.Seat != v.Tug.Seats[0].ID {
				return fmt.Errorf("%s: the driver (first of the crew) takes the first seat", v.ID)
			}
			people[m.ID], taken[m.Seat] = true, true
		}
		hitched := ""
		parked := map[[2]string]bool{}
		for _, t := range v.Trailers {
			if t.ID == "" || trailerIDs[t.ID] {
				return fmt.Errorf("%s: duplicate trailer %q", v.ID, t.ID)
			}
			trailerIDs[t.ID] = true
			if t.Start.Hitched {
				if hitched != "" {
					return fmt.Errorf("%s: two trailers start hitched", v.ID)
				}
				hitched = t.ID
				continue
			}
			key := [2]string{t.Start.StandID, t.Start.Bay}
			if _, ok := bay(t.Start.StandID, t.Start.Bay); !ok || parked[key] {
				return fmt.Errorf("%s: trailer %s starts on an unknown or taken bay", v.ID, t.ID)
			}
			parked[key] = true
		}
		for _, g := range v.Gates {
			named := false
			for _, gate := range life.Locations[g.LevelID].Gates {
				if gate.ID == g.GateID {
					for _, p := range gate.Permits {
						named = named || p.Vehicle == v.ID
					}
				}
			}
			if !named {
				return fmt.Errorf("%s: gate %s on %s holds no permit for this convoy", v.ID, g.GateID, g.LevelID)
			}
		}
		if len(v.Runs) == 0 {
			return fmt.Errorf("%s: a convoy needs a run", v.ID)
		}
		for ri := range v.Runs {
			for _, leg := range v.Runs[ri].Legs {
				if leg.LevelID != "" {
					if _, ok := c.Floors[leg.LevelID]; !ok {
						return fmt.Errorf("%s: no floor height for %s", v.ID, leg.LevelID)
					}
				}
			}
			run := &v.Runs[ri]
			if err := c.validateRun(v, run.ID, run.StartSecond, run.Legs, hitched != "", parked, life, cellOf, glyph, walks, bay, cranes, decks, lifeEdges, stands); err != nil {
				return fmt.Errorf("%s: %s: %w", v.ID, run.ID, err)
			}
		}
	}
	return nil
}

// validateRun walks the route once as the runtime will: level by level, pose by pose, trailer by trailer.
func (c *CargoConvoyCatalog) validateRun(v *CargoConvoy, runID string, start int64, legs []CargoLeg, hitched bool, parked map[[2]string]bool, life cargoLifeData,
	cellOf func(string) float64, glyph func(string, int, int) byte, walks func(string, CargoPost) bool, bay func(string, string) (CargoPose, bool),
	cranes map[string]*CargoCrane, decks map[string]*LiftingDeckDefinition, lifeEdges map[string]struct {
		from, to string
		vehicle  bool
	}, stands map[string]*CargoStand) error {
	if start < 0 || start >= 86400 || len(legs) < 2 || legs[0].Kind != "gather" || legs[len(legs)-1].Kind != "release" {
		return fmt.Errorf("a run starts with gather and ends with release")
	}
	level := v.Home.LevelID
	pose := CargoPose{v.Home.At[0], v.Home.At[1], cargoDegrees(v.Home.YawDegrees * math.Pi / 180)}
	known := true // the pose is known (false after an edge: the next drive places the convoy)
	bays := map[[2]string]bool{}
	for k := range parked {
		bays[k] = true
	}
	same := func(a, b CargoPose) bool {
		return math.Hypot(a.X-b.X, a.Y-b.Y) < 0.05 && cargoTurn(a.YawDegrees, b.YawDegrees) < 1
	}
	trailerOf := func(tug CargoPose) CargoPose {
		yaw := tug.YawDegrees * math.Pi / 180
		back := v.Tug.HitchBackM + v.Trailer.EyeM
		return CargoPose{tug.X - math.Cos(yaw)*back, tug.Y - math.Sin(yaw)*back, tug.YawDegrees}
	}
	// A body may stand on floor or beyond the built level (void, off the grid: a corridor drawn as a vista), never on a
	// wall, an obstacle or a closed thing ('+'), except the openings of the level's gates (the pass raises them).
	gateCells := func(level string) map[[2]int]bool {
		out := map[[2]int]bool{}
		for _, g := range life.Locations[level].Gates {
			for _, o := range g.Opening {
				out[[2]int{o.X, o.Y}] = true
			}
		}
		return out
	}
	bodyOK := func(level string, tug CargoPose, trailer *CargoPose) error {
		cell := cellOf(level)
		gates := gateCells(level)
		bad := ""
		check := func(x, y int) {
			g := glyph(level, x, y)
			if g != '.' && g != ' ' && !gates[[2]int{x, y}] && bad == "" {
				bad = fmt.Sprintf("(%d,%d) '%c'", x, y, g)
			}
		}
		cargoRectCells(tug, v.Tug.RearM, v.Tug.NoseM, v.Tug.HalfWidthM, cell, check)
		if trailer != nil {
			cargoRectCells(*trailer, v.Trailer.TailM, v.Trailer.BodyFrontM, v.Trailer.HalfWidthM, cell, check)
		}
		if bad != "" {
			return fmt.Errorf("the convoy at (%.2f, %.2f) stands on %s", tug.X, tug.Y, bad)
		}
		return nil
	}
	posts := func(leg *CargoLeg) error {
		if len(leg.Posts) == 0 {
			return nil
		}
		if len(leg.Posts) != len(v.Crew) {
			return fmt.Errorf("leg %s: posts must name every crew member (driver first)", leg.Kind)
		}
		seen := map[CargoPost]bool{}
		for i, p := range leg.Posts {
			if p.Seat == (p.At != nil) {
				return fmt.Errorf("leg %s: a crew member is either seated or at a cell", leg.Kind)
			}
			if p.At != nil && (!walks(level, *p.At) || seen[*p.At]) {
				return fmt.Errorf("leg %s: crew post (%d,%d) is no free floor of %s", leg.Kind, p.At.X, p.At.Y, level)
			}
			if p.At != nil && i == 0 && leg.Kind != "gather" && leg.Kind != "load" {
				return fmt.Errorf("leg %s: the driver stays with the tug", leg.Kind)
			}
			if p.At != nil {
				seen[*p.At] = true
			}
		}
		return nil
	}
	for li := range legs {
		leg := &legs[li]
		leg.Level, leg.Trailer = level, hitched
		if !strings.HasPrefix(leg.Label, "life.cargo.") {
			return fmt.Errorf("leg %d (%s): a crew activity key", li, leg.Kind)
		}
		if err := posts(leg); err != nil {
			return err
		}
		switch leg.Kind {
		case "gather", "load", "release":
			if leg.LevelID != level || level != v.Home.LevelID || !known {
				return fmt.Errorf("leg %d (%s) happens at home, %s", li, leg.Kind, v.Home.LevelID)
			}
			if leg.Kind == "load" && (!hitched || leg.Seconds < 30) {
				return fmt.Errorf("leg %d: loading needs a hitched trailer and time", li)
			}
		case "drive", "deck":
			if leg.Kind == "drive" && leg.LevelID != level {
				return fmt.Errorf("leg %d: drive on %s while the convoy is on %s", li, leg.LevelID, level)
			}
			radius := v.TurnRadiusM
			if leg.TurnRadiusM > 0 {
				if hitched {
					return fmt.Errorf("leg %d: a tighter turn than the convoy's is for the tug alone", li)
				}
				radius = leg.TurnRadiusM
			}
			if leg.Reverse && hitched {
				return fmt.Errorf("leg %d: only the tug alone backs up", li)
			}
			track, err := cargoFillet(leg.Path, radius)
			if err != nil {
				return fmt.Errorf("leg %d: %w", li, err)
			}
			first, firstTrailer := v.cargoPoses(track, 0, leg.Reverse)
			// the start heading is the authored first segment (the chord of ±0.4 m reads a turning start a few degrees off)
			leg.rawEnd = [2][2]float64{leg.Path[len(leg.Path)-2], leg.Path[len(leg.Path)-1]}
			startYaw := cargoDegrees(math.Atan2(leg.Path[1][1]-leg.Path[0][1], leg.Path[1][0]-leg.Path[0][0]))
			if leg.Reverse {
				startYaw = cargoDegrees(math.Atan2(leg.Path[1][1]-leg.Path[0][1], leg.Path[1][0]-leg.Path[0][0]) + math.Pi)
			}
			if known && (math.Hypot(first.X-pose.X, first.Y-pose.Y) > 0.05 || cargoTurn(startYaw, pose.YawDegrees) > 1) {
				return fmt.Errorf("leg %d starts at (%.2f, %.2f) %.0f°, the convoy stands at (%.2f, %.2f) %.0f°", li, first.X, first.Y, first.YawDegrees, pose.X, pose.Y, pose.YawDegrees)
			}
			_ = firstTrailer
			leg.Path = track
			length := cargoTrackLength(track)
			for s := 0.0; ; s += 0.25 {
				if s > length {
					s = length
				}
				tug, trailer := v.cargoPoses(track, s, leg.Reverse)
				var tp *CargoPose
				if hitched {
					tp = &trailer
				}
				if err := bodyOK(level, tug, tp); err != nil {
					return fmt.Errorf("leg %d (%s on %s): %w", li, leg.Kind, level, err)
				}
				// the posts of the crew afoot lie off the swept body
				for _, p := range leg.Posts {
					if p.At == nil {
						continue
					}
					hit := false
					cargoRectCells(tug, v.Tug.RearM, v.Tug.NoseM, v.Tug.HalfWidthM, cellOf(level), func(x, y int) { hit = hit || (x == p.At.X && y == p.At.Y) })
					if hit {
						return fmt.Errorf("leg %d: crew post (%d,%d) lies on the tug's way", li, p.At.X, p.At.Y)
					}
				}
				if s >= length {
					break
				}
			}
			// the end pose: the end of the track, looking along the authored last segment (a track may end in a turn's arc)
			raw := leg.rawEnd
			pose, _ = v.cargoPoses(track, length, leg.Reverse)
			pose.YawDegrees = cargoDegrees(math.Atan2(raw[1][1]-raw[0][1], raw[1][0]-raw[0][0]))
			if leg.Reverse {
				pose.YawDegrees = cargoDegrees(math.Atan2(raw[1][1]-raw[0][1], raw[1][0]-raw[0][0]) + math.Pi)
			}
			known = true
			leg.ToLevel = level
			if leg.Kind == "deck" {
				d := decks[leg.DeckID]
				if d == nil || d.Lower == nil || (level != d.LevelID && level != d.Lower.LevelID) {
					return fmt.Errorf("leg %d: deck %q is no pass-through deck at %s", li, leg.DeckID, level)
				}
				upPlan := [2]float64{float64(d.Rect.X*2+d.Rect.W) / 4, float64(d.Rect.Y*2+d.Rect.H) / 4}
				lowPlan := [2]float64{float64(d.Lower.Rect.X*2+d.Lower.Rect.W) / 4, float64(d.Lower.Rect.Y*2+d.Lower.Rect.H) / 4}
				from, to, other := lowPlan, upPlan, d.LevelID
				rect := d.Lower.Rect
				if level == d.LevelID {
					from, to, other = upPlan, lowPlan, d.Lower.LevelID
					rect = d.Rect
				}
				cell := cellOf(level)
				inside := true
				check := func(x, y int) {
					inside = inside && x >= rect.X && x < rect.X+rect.W && y >= rect.Y && y < rect.Y+rect.H
				}
				tug, trailer := v.cargoPoses(track, length, false)
				cargoRectCells(tug, v.Tug.RearM, v.Tug.NoseM, v.Tug.HalfWidthM, cell, check)
				if hitched {
					cargoRectCells(trailer, v.Trailer.TailM, v.Trailer.BodyFrontM, v.Trailer.HalfWidthM, cell, check)
				}
				if !inside {
					return fmt.Errorf("leg %d: the convoy does not stand wholly on the deck", li)
				}
				pose = CargoPose{pose.X - from[0] + to[0], pose.Y - from[1] + to[1], pose.YawDegrees}
				level = other
				leg.ToLevel = other
			}
		case "edge":
			e, ok := lifeEdges[leg.LifeEdge]
			if !ok || !e.vehicle || e.from != level {
				return fmt.Errorf("leg %d: %q is no vehicle-only Life edge from %s", li, leg.LifeEdge, level)
			}
			if li+1 >= len(legs) || legs[li+1].Kind != "drive" || legs[li+1].LevelID != e.to {
				return fmt.Errorf("leg %d: an edge is followed by a drive on %s", li, e.to)
			}
			level, known = e.to, false
			leg.ToLevel = e.to
		case "charge", "unload", "unhitch", "hitch", "crane":
			s := stands[leg.StandID]
			if s == nil || s.LevelID != level || !known {
				return fmt.Errorf("leg %d (%s): stand %q is not here (%s)", li, leg.Kind, leg.StandID, level)
			}
			switch leg.Kind {
			case "unload":
				if !hitched || leg.Seconds < 30 {
					return fmt.Errorf("leg %d: unloading needs a hitched trailer and time", li)
				}
			case "unhitch", "hitch":
				b, ok := bay(leg.StandID, leg.Bay)
				key := [2]string{leg.StandID, leg.Bay}
				if !ok {
					return fmt.Errorf("leg %d: unknown bay %q", li, leg.Bay)
				}
				if leg.Kind == "unhitch" {
					if !hitched || bays[key] || !same(trailerOf(pose), b) {
						return fmt.Errorf("leg %d: the trailer does not stand on free bay %s (trailer at %+v, bay %+v)", li, leg.Bay, trailerOf(pose), b)
					}
					bays[key], hitched = true, false
				} else {
					if hitched || !bays[key] || !same(trailerOf(pose), b) {
						return fmt.Errorf("leg %d: the tug is not at the drawbar of the trailer on bay %s (would couple at %+v, bay %+v)", li, leg.Bay, trailerOf(pose), b)
					}
					delete(bays, key)
					hitched = true
				}
			case "crane":
				k := cranes[leg.CraneID]
				if k == nil || k.StandID != leg.StandID || len(leg.Moves) == 0 {
					return fmt.Errorf("leg %d: crane %q does not serve stand %s", li, leg.CraneID, leg.StandID)
				}
				for _, m := range leg.Moves {
					from, okFrom := bay(leg.StandID, m.From)
					to, okTo := bay(leg.StandID, m.To)
					kf, kt := [2]string{leg.StandID, m.From}, [2]string{leg.StandID, m.To}
					if !okFrom || !okTo || !bays[kf] || bays[kt] {
						return fmt.Errorf("leg %d: crane move %s -> %s needs a trailer at the first bay and a free second one", li, m.From, m.To)
					}
					// a load carried over a trailer parked on another bay hangs clear of its roof
					for k2, occupied := range bays {
						if !occupied || k2 == kf || k2[0] != leg.StandID {
							continue
						}
						under, ok := bay(leg.StandID, k2[1])
						fx, fy := v.cargoLiftPoint(from)
						tx, ty := v.cargoLiftPoint(to)
						ux, uy := v.cargoLiftPoint(under)
						if ok && !cargoCraneClears([2]float64{fx, fy}, [2]float64{tx, ty}, [2]float64{ux, uy}, v.Trailer.TailM+v.Trailer.BodyFrontM, k.LiftM, v.Trailer.HeightM) {
							return fmt.Errorf("leg %d: crane move %s -> %s carries the trailer over the one on bay %s with lift_m %.2f, under its roof (%.2f m + %.2f m clearance)", li, m.From, m.To, k2[1], k.LiftM, v.Trailer.HeightM, cargoCraneClearanceM)
						}
					}
					for _, b := range []CargoPose{from, to} {
						x, y := v.cargoLiftPoint(b)
						if x < k.Travel.HoistXM[0]-1e-9 || x > k.Travel.HoistXM[1]+1e-9 || y < k.Travel.BridgeYM[0]-1e-9 || y > k.Travel.BridgeYM[1]+1e-9 {
							return fmt.Errorf("leg %d: bay at (%.2f, %.2f) is out of the crane's reach", li, x, y)
						}
					}
					delete(bays, kf)
					bays[kt] = true
				}
				// the tug stands clear of the band while the crane works
				clear := true
				cargoRectCells(pose, v.Tug.RearM, v.Tug.NoseM, v.Tug.HalfWidthM, cellOf(level), func(x, y int) {
					clear = clear && !(x >= k.Band.X && x < k.Band.X+k.Band.W && y >= k.Band.Y && y < k.Band.Y+k.Band.H)
				})
				if !clear {
					return fmt.Errorf("leg %d: the tug stands inside the crane's band", li)
				}
				for _, p := range leg.Posts {
					if p.At != nil && p.At.X >= k.Band.X && p.At.X < k.Band.X+k.Band.W && p.At.Y >= k.Band.Y && p.At.Y < k.Band.Y+k.Band.H {
						return fmt.Errorf("leg %d: a crew post lies inside the crane's band", li)
					}
				}
			}
			leg.ToLevel = level
		default:
			return fmt.Errorf("leg %d: unknown kind %q", li, leg.Kind)
		}
		if leg.ToLevel == "" {
			leg.ToLevel = level
		}
	}
	if level != v.Home.LevelID || !known || math.Hypot(pose.X-v.Home.At[0], pose.Y-v.Home.At[1]) > 0.05 || cargoTurn(pose.YawDegrees, cargoDegrees(v.Home.YawDegrees*math.Pi/180)) > 1 {
		return fmt.Errorf("the run ends at (%.2f, %.2f) on %s, not at home", pose.X, pose.Y, level)
	}
	if len(bays) != len(parked) {
		return fmt.Errorf("the run leaves the stand with a different number of parked trailers")
	}
	// The next day starts with the same trailers on the same bays only when the fleet comes back in turn: the run must
	// leave each bay taken as it found it (which trailer stands there may change).
	for k := range parked {
		if !bays[k] {
			return fmt.Errorf("the run leaves bay %s/%s empty", k[0], k[1])
		}
	}
	if !hitched {
		return fmt.Errorf("the run ends without a hitched trailer")
	}
	return nil
}

// cargoLiftPoint: where the crane hook stands over a trailer — the middle of its box.
// cargoCraneClearanceM: the air left between a carried trailer and the roof of one parked under its way.
const cargoCraneClearanceM = 0.25

// cargoCraneClears: a crane carries a trailer straight between two lift points (body middles) with its underside lift
// above the floor; it clears a trailer parked with its middle at under when its way keeps a body length from it or it
// hangs above that trailer's roof.
func cargoCraneClears(from, to, under [2]float64, length, lift, height float64) bool {
	if lift >= height+cargoCraneClearanceM {
		return true
	}
	for i := 0; i <= 32; i++ {
		u := float64(i) / 32
		if math.Hypot(from[0]+(to[0]-from[0])*u-under[0], from[1]+(to[1]-from[1])*u-under[1]) < length {
			return false
		}
	}
	return true
}

func (v CargoConvoy) cargoLiftPoint(p CargoPose) (float64, float64) {
	mid := (v.Trailer.BodyFrontM - v.Trailer.TailM) / 2
	yaw := p.YawDegrees * math.Pi / 180
	return p.X + math.Cos(yaw)*mid, p.Y + math.Sin(yaw)*mid
}

// applyCargoLots moves the lots into the inventory: a trailer store of each lot in each trailer of the convoy; a supply
// lot's whole stock (the shop's consignment of `days`) moves to the Copper dock, and the shop's supply store becomes its
// locker in store 17. Runs after the visitors and the craft places made their consignments. Nothing is created: the
// same initial sources change place.
func applyCargoLots(c *Compiled) error {
	cat := c.CargoConvoys
	if cat == nil {
		return nil
	}
	fail := func(format string, args ...any) error {
		return fmt.Errorf("world.cargo_convoys: lots: %s", fmt.Sprintf(format, args...))
	}
	m := &c.Inventory
	var trailers []string
	var level string
	for _, v := range cat.Convoys {
		for _, t := range v.Trailers {
			trailers = append(trailers, t.ID)
		}
	}
	if len(cat.Stands) > 0 {
		level = cat.Stands[0].LevelID
	}
	if len(cat.Lots) > 0 && (len(trailers) == 0 || level == "") {
		return fail("lots need a trailer and a stand")
	}
	supplies := map[string][]CargoLotItem{}
	if c.Visitors != nil {
		for _, r := range c.Visitors.Supply.Rules {
			for _, l := range r.Lots {
				supplies[r.Store] = append(supplies[r.Store], CargoLotItem(l))
			}
		}
	}
	if c.Craft != nil {
		for _, s := range c.Craft.Stalls {
			if s.Supply == nil {
				continue
			}
			for _, h := range s.Heaps {
				supplies[s.Supply.Store] = append(supplies[s.Supply.Store], CargoLotItem{Item: h.Item, Full: h.Full})
			}
		}
	}
	shops := map[string]bool{}
	for i := range cat.Lots {
		lot := &cat.Lots[i]
		if lot.Shop == "" || shops[lot.Shop] || !m.Actors[lot.Owner] {
			return fail("invalid lot %q", lot.Shop)
		}
		shops[lot.Shop] = true
		copperStore, cok := m.Stores[lot.CopperStore]
		locker, lok := m.Stores[lot.LockerStore]
		switch lot.Kind {
		case "delivery":
			for _, d := range m.Deliveries {
				if d.SourceStore == lot.LockerStore && d.Carrier == lot.Owner {
					lot.Delivery, lot.Definition = d.ID, d.Definition
				}
			}
			if lot.Delivery == "" || !cok || !lok || copperStore.Owner != lot.Owner || copperStore.Anchor.Kind != "world" || lot.Quantity < 1 {
				return fail("%s: a delivery lot needs the owner's delivery from its locker and a Copper store", lot.Shop)
			}
		case "supply":
			items := supplies[lot.LockerStore]
			if len(items) == 0 || !lok || locker.Owner != lot.Owner || lot.Copper == nil || lot.Locker == nil || cok {
				return fail("%s: a supply lot moves the shop's own consignment %s (and makes its Copper store)", lot.Shop, lot.LockerStore)
			}
			sort.Slice(items, func(a, b int) bool { return items[a].Item < items[b].Item })
			lot.Items = items
			copper := locker
			copper.ID, copper.Name = lot.CopperStore, "market.store.copper_lot"
			copper.Anchor = inv.Anchor{Kind: "world", Level: lot.Copper.Level, At: lot.Copper.At}
			m.Stores[lot.CopperStore] = copper
			locker.Anchor = inv.Anchor{Kind: "world", Level: lot.Locker.Level, At: lot.Locker.At}
			m.Stores[lot.LockerStore] = locker
			moved := 0
			for k := range m.Sources {
				if m.Sources[k].Location == (inv.Location{Kind: "store", Store: lot.LockerStore}) {
					m.Sources[k].Location = inv.Location{Kind: "store", Store: lot.CopperStore}
					moved++
				}
			}
			if moved == 0 {
				return fail("%s: the consignment holds no stock", lot.Shop)
			}
		default:
			return fail("%s: unknown lot kind %q", lot.Shop, lot.Kind)
		}
		lot.TrailerStores = map[string]string{}
		for _, t := range trailers {
			id := "commerce/" + lot.Shop + "/" + t
			if _, dup := m.Stores[id]; dup {
				return fail("store %s exists", id)
			}
			m.Stores[id] = inv.Store{ID: id, Name: "market.store.trailer", Kind: "container", Owner: lot.Owner, Locked: true, Anchor: inv.Anchor{Kind: "world", Level: level, At: inv.Cell{X: 11, Y: 25}}, Capacity: inv.Capacity{Mass: 200000, Volume: 300000, Stacks: 64}}
			lot.TrailerStores[t] = id
		}
	}
	if _, err := inv.New(*m, 425); err != nil {
		return fail("inventory with the lots: %v", err)
	}
	return nil
}
