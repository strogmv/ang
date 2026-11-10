package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"cuelang.org/go/cue"
)

// A cargo convoy is a tug with one trailer that follows authored tracks in plan
// metres. The compiler rounds the corners of every track once, so the runtime
// only walks a finished polyline: no physics and no steering model at run time.
type CargoConvoyCatalog struct {
	Convoys         []CargoConvoy `json:"convoys"`
	DeliveryTrolley struct {
		Resource         string  `json:"resource"`
		CouplingForwardM float64 `json:"coupling_forward_m"`
	} `json:"delivery_trolley"`
}
type CargoPost struct {
	X int `json:"x"`
	Y int `json:"y"`
}
type CargoConvoy struct {
	ID     string `json:"id"`
	DeckID string `json:"deck_id"`
	Tug    struct {
		Resource   string    `json:"resource"`
		HitchBackM float64   `json:"hitch_back_m"`
		NoseM      float64   `json:"nose_m"`
		HalfWidthM float64   `json:"half_width_m"`
		SeatM      []float64 `json:"seat_m"`
	} `json:"tug"`
	Trailer struct {
		Resource   string  `json:"resource"`
		EyeM       float64 `json:"eye_m"`
		TailM      float64 `json:"tail_m"`
		HalfWidthM float64 `json:"half_width_m"`
	} `json:"trailer"`
	SpeedMMPerSecond int64    `json:"speed_mm_per_second"`
	TurnRadiusM      float64  `json:"turn_radius_m"`
	Driver           string   `json:"driver"`
	Loaders          []string `json:"loaders"`
	LoadSecond       int64    `json:"load_second"`
	BoardSecond      int64    `json:"board_second"`
	UnloadSeconds    int64    `json:"unload_seconds"`
	Labels           struct {
		Rest      string `json:"rest"`
		Loading   string `json:"loading"`
		Driving   string `json:"driving"`
		Riding    string `json:"riding"`
		Unloading string `json:"unloading"`
		Waiting   string `json:"waiting"`
	} `json:"labels"`
	Lower struct {
		LevelID string       `json:"level_id"`
		Board   [][2]float64 `json:"board"`
		Leave   [][2]float64 `json:"leave"`
		Rest    []CargoPost  `json:"rest"`
		Loading []CargoPost  `json:"loading"`
		Deck    []CargoPost  `json:"deck"`
	} `json:"lower"`
	Upper struct {
		LevelID   string       `json:"level_id"`
		Leave     [][2]float64 `json:"leave"`
		Turn      [][2]float64 `json:"turn"`
		Board     [][2]float64 `json:"board"`
		Unloading []CargoPost  `json:"unloading"`
		Staging   []CargoPost  `json:"staging"`
		Deck      []CargoPost  `json:"deck"`
	} `json:"upper"`
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
	var life struct {
		Locations map[string]struct {
			Terrain []string `json:"terrain"`
			CellMM  int      `json:"cell_mm"`
		} `json:"locations"`
		Actors []struct {
			ID    string `json:"id"`
			Steps []struct {
				Level string `json:"level"`
			} `json:"steps"`
		} `json:"actors"`
	}
	if err := json.Unmarshal(w.Life, &life); err != nil {
		return err
	}
	if !strings.HasPrefix(c.DeliveryTrolley.Resource, "res://") || strings.Contains(c.DeliveryTrolley.Resource, "..") || c.DeliveryTrolley.CouplingForwardM < .5 || c.DeliveryTrolley.CouplingForwardM > 1 {
		return fmt.Errorf("invalid delivery trolley")
	}
	home := map[string]string{}
	for _, a := range life.Actors {
		if len(a.Steps) > 0 {
			home[a.ID] = a.Steps[0].Level
		}
	}
	ids, people := map[string]bool{}, map[string]bool{}
	for i := range c.Convoys {
		v := &c.Convoys[i]
		if v.ID == "" || ids[v.ID] {
			return fmt.Errorf("duplicate or empty convoy id %q", v.ID)
		}
		ids[v.ID] = true
		var deck *LiftingDeckDefinition
		if w.LiftingDecks != nil {
			for k := range w.LiftingDecks.Decks {
				if w.LiftingDecks.Decks[k].ID == v.DeckID {
					deck = &w.LiftingDecks.Decks[k]
				}
			}
		}
		if deck == nil || deck.Lower == nil || deck.Lower.LevelID != v.Lower.LevelID || deck.LevelID != v.Upper.LevelID {
			return fmt.Errorf("%s: a convoy needs a pass-through deck between its two levels", v.ID)
		}
		if deck.Freight == nil || deck.Freight.ConvoyID != v.ID {
			return fmt.Errorf("%s: the deck freight must name this convoy", v.ID)
		}
		if v.BoardSecond >= deck.Freight.FirstSecond || v.LoadSecond >= v.BoardSecond {
			return fmt.Errorf("%s: loading, boarding and the freight run must follow in this order", v.ID)
		}
		for _, n := range append([]float64{v.Tug.HitchBackM, v.Tug.NoseM, v.Tug.HalfWidthM, v.Trailer.EyeM, v.Trailer.TailM, v.Trailer.HalfWidthM, v.TurnRadiusM}, v.Tug.SeatM...) {
			if math.IsNaN(n) || math.IsInf(n, 0) || math.Abs(n) > 100 {
				return fmt.Errorf("%s: nonfinite geometry", v.ID)
			}
		}
		if len(v.Tug.SeatM) != 3 || v.Tug.HitchBackM <= 0 || v.Tug.NoseM <= 0 || v.Trailer.EyeM <= 0 || v.Trailer.TailM <= 0 || v.SpeedMMPerSecond < 20 || v.SpeedMMPerSecond > 400 || v.UnloadSeconds < 30 || !strings.HasPrefix(v.Tug.Resource, "res://") || !strings.HasPrefix(v.Trailer.Resource, "res://") {
			return fmt.Errorf("%s: invalid vehicle definition", v.ID)
		}
		crew := append([]string{v.Driver}, v.Loaders...)
		for _, id := range crew {
			if people[id] || home[id] != v.Lower.LevelID {
				return fmt.Errorf("%s: crew member %s must be one Life actor whose day starts on the lower level", v.ID, id)
			}
			people[id] = true
		}
		for _, key := range []string{v.Labels.Rest, v.Labels.Loading, v.Labels.Driving, v.Labels.Riding, v.Labels.Unloading, v.Labels.Waiting} {
			if key == "" {
				return fmt.Errorf("%s: every crew activity needs a label key", v.ID)
			}
		}
		walk := func(level string, p CargoPost) bool {
			t := life.Locations[level].Terrain
			return p.Y >= 0 && p.Y < len(t) && p.X >= 0 && p.X < len(t[p.Y]) && t[p.Y][p.X] == '.'
		}
		inside := func(r struct{ X, Y, W, H int }, p CargoPost) bool {
			return p.X >= r.X && p.X < r.X+r.W && p.Y >= r.Y && p.Y < r.Y+r.H
		}
		for name, set := range map[string]struct {
			level string
			posts []CargoPost
			deck  *struct{ X, Y, W, H int }
		}{
			"lower.rest": {v.Lower.LevelID, v.Lower.Rest, nil}, "lower.loading": {v.Lower.LevelID, v.Lower.Loading, nil}, "lower.deck": {v.Lower.LevelID, v.Lower.Deck, &deck.Lower.Rect},
			"upper.unloading": {v.Upper.LevelID, v.Upper.Unloading, nil}, "upper.staging": {v.Upper.LevelID, v.Upper.Staging, nil}, "upper.deck": {v.Upper.LevelID, v.Upper.Deck, &deck.Rect},
		} {
			if len(set.posts) != len(crew) {
				return fmt.Errorf("%s: %s needs one post per crew member (driver first)", v.ID, name)
			}
			seen := map[CargoPost]bool{}
			for _, p := range set.posts {
				if seen[p] || !walk(set.level, p) {
					return fmt.Errorf("%s: %s post (%d,%d) is not a free walkable cell", v.ID, name, p.X, p.Y)
				}
				seen[p] = true
				if set.deck != nil && !inside(*set.deck, p) {
					return fmt.Errorf("%s: %s post (%d,%d) must stand on the deck", v.ID, name, p.X, p.Y)
				}
				if set.deck == nil && (inside(deck.Rect, p) && set.level == deck.LevelID || inside(deck.Lower.Rect, p) && set.level == deck.Lower.LevelID) {
					return fmt.Errorf("%s: %s post (%d,%d) must stand off the deck", v.ID, name, p.X, p.Y)
				}
			}
		}
		// Tracks: rounded once here; every sample of the swept convoy stays on floor,
		// and the on-deck ends keep the whole convoy inside the deck.
		length := v.Tug.NoseM + v.Tug.HitchBackM + v.Trailer.EyeM + v.Trailer.TailM
		for name, track := range map[string]*[][2]float64{"lower.board": &v.Lower.Board, "lower.leave": &v.Lower.Leave, "upper.leave": &v.Upper.Leave, "upper.turn": &v.Upper.Turn, "upper.board": &v.Upper.Board} {
			rounded, err := cargoFillet(*track, v.TurnRadiusM)
			if err != nil {
				return fmt.Errorf("%s: %s: %w", v.ID, name, err)
			}
			level := v.Upper.LevelID
			if strings.HasPrefix(name, "lower.") {
				level = v.Lower.LevelID
			}
			cell := float64(life.Locations[level].CellMM) / 1000
			if cell <= 0 {
				return fmt.Errorf("%s: %s: level has no metric grid", v.ID, name)
			}
			for _, p := range rounded {
				if !walk(level, CargoPost{X: int(math.Floor(p[0] / cell)), Y: int(math.Floor(p[1] / cell))}) {
					return fmt.Errorf("%s: %s leaves the floor at (%.2f, %.2f) m", v.ID, name, p[0], p[1])
				}
			}
			*track = rounded
		}
		join := func(a, b [][2]float64) bool {
			return math.Hypot(a[len(a)-1][0]-b[0][0], a[len(a)-1][1]-b[0][1]) < 1e-6
		}
		if !join(v.Upper.Leave, v.Upper.Turn) || !join(v.Upper.Turn, v.Upper.Board) || !join(v.Lower.Leave, v.Lower.Board) {
			return fmt.Errorf("%s: tracks must join end to start (leave → turn → board, lower leave → lower board)", v.ID)
		}
		// Pass-through: the place on the deck is the same deck-relative pose at both landings.
		rel := func(p [2]float64, plan []float64) [2]float64 { return [2]float64{p[0] - plan[0], p[1] - plan[2]} }
		same := func(a, b [2]float64) bool { return math.Hypot(a[0]-b[0], a[1]-b[1]) < 1e-3 }
		upPlan, lowPlan := []float64{float64(deck.Rect.X*2+deck.Rect.W) / 4, 0, float64(deck.Rect.Y*2+deck.Rect.H) / 4}, []float64{float64(deck.Lower.Rect.X*2+deck.Lower.Rect.W) / 4, 0, float64(deck.Lower.Rect.Y*2+deck.Lower.Rect.H) / 4}
		if !same(rel(v.Lower.Board[len(v.Lower.Board)-1], lowPlan), rel(v.Upper.Leave[0], upPlan)) || !same(rel(v.Upper.Board[len(v.Upper.Board)-1], upPlan), rel(v.Lower.Leave[0], lowPlan)) {
			return fmt.Errorf("%s: the convoy must stand at the same place of the deck at both landings", v.ID)
		}
		halfDeck := float64(deck.Rect.H) / 4
		for _, end := range [][2]float64{rel(v.Upper.Leave[0], upPlan), rel(v.Upper.Board[len(v.Upper.Board)-1], upPlan)} {
			// The convoy stands along the deck's short axis, nose towards the side it will leave by.
			nose, tail := math.Abs(end[1])+v.Tug.NoseM, length-v.Tug.NoseM-math.Abs(end[1])
			if nose > halfDeck || tail > halfDeck || math.Abs(end[0])+math.Max(v.Tug.HalfWidthM, v.Trailer.HalfWidthM) > float64(deck.Rect.W)/4 {
				return fmt.Errorf("%s: the %.2f m convoy does not fit the deck at its stand", v.ID, length)
			}
		}
	}
	return nil
}
