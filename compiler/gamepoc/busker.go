package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"cuelang.org/go/cue"
	inv "github.com/strogmv/ang/compiler/inventory/runtime"
)

// The street musician of a market level (world.busker; market musician v1, design/market-musician/v1/BRIEF.md): one named
// person of the Life with a spot of his own (a stool, a case on legs with a CRD reader, the cells a payer stands on, the
// listening places), a day the server draws from the seed (whether he comes, when, the sets and the breaks), the tunes he
// plays (phrases: the bellows change direction at each, the length is the server's clock of the tune), the hall's visitors
// as listeners and payers, and the hero's few lines. ANG checks the spot against the floor, the walking rules of the hall
// (the aisles stay their width, no public place of anybody else is taken) and the people and money it names, and gives the
// server its runtime (busker.go.tmpl) and the client the view (presentation.json "busker").
type BuskerCell struct {
	X int `json:"x"`
	Y int `json:"y"`
}
type BuskerMM struct {
	X int64 `json:"x"`
	Y int64 `json:"y"`
	Z int64 `json:"z"`
}
type BuskerRange struct {
	Min int64 `json:"min"`
	Max int64 `json:"max"`
}
type BuskerListenSpot struct {
	At   BuskerCell `json:"at"`
	Face struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	} `json:"face"`
}
type BuskerKeep struct {
	ID   string `json:"id"`
	Rect struct {
		X int `json:"x"`
		Y int `json:"y"`
		W int `json:"w"`
		H int `json:"h"`
	} `json:"rect"`
	Axis string  `json:"axis"`
	MinM float64 `json:"min_m"`
}
type BuskerSpot struct {
	Seat       BuskerCell         `json:"seat"`
	SeatMM     BuskerMM           `json:"seat_mm"`
	YawDeg     float64            `json:"yaw_deg"`
	Case       BuskerCell         `json:"case"`
	CaseMM     BuskerMM           `json:"case_mm"`
	CaseYawDeg float64            `json:"case_yaw_deg"`
	ReaderMM   BuskerMM           `json:"reader_mm"`
	Pay        []BuskerCell       `json:"pay"`
	Listen     []BuskerListenSpot `json:"listen"`
	Keep       []BuskerKeep       `json:"keep"`
}
type BuskerBreakPlace struct {
	Level string     `json:"level"`
	At    BuskerCell `json:"at"`
}
type BuskerBreak struct {
	ID        string            `json:"id"`
	Seconds   BuskerRange       `json:"seconds"`
	Weight    int64             `json:"weight"`
	MaxPerDay int               `json:"max_per_day"`
	Label     string            `json:"label"`
	At        *BuskerBreakPlace `json:"at,omitempty"`
	Seats     []string          `json:"seats,omitempty"`
	// filled by ANG: the seated place whose regular he is, and the cells of its seats (same order as Seats)
	Place     string       `json:"place,omitempty"`
	SeatCells []BuskerCell `json:"seat_cells,omitempty"`
}
type BuskerDay struct {
	AbsentPermille int64 `json:"absent_permille"`
	Arrive         struct {
		From int64 `json:"from"`
		To   int64 `json:"to"`
	} `json:"arrive"`
	Leave struct {
		From int64 `json:"from"`
		To   int64 `json:"to"`
	} `json:"leave"`
	LeadSeconds   int64         `json:"lead_seconds"`
	SetSeconds    BuskerRange   `json:"set_seconds"`
	UnpackSeconds int64         `json:"unpack_seconds"`
	PackSeconds   int64         `json:"pack_seconds"`
	Breaks        []BuskerBreak `json:"breaks"`
	Labels        struct {
		Walk   string `json:"walk"`
		Unpack string `json:"unpack"`
		Play   string `json:"play"`
		Rest   string `json:"rest"`
		Pack   string `json:"pack"`
		Home   string `json:"home"`
	} `json:"labels"`
}
type BuskerPhrase struct {
	Bars  int     `json:"bars"`
	Dyn   float64 `json:"dyn"`
	Notes string  `json:"notes,omitempty"`
	// filled by ANG: the phrase's beats (bars x meter) and its bellows direction in the first pass
	Beats   float64 `json:"beats,omitempty"`
	Bellows string  `json:"bellows,omitempty"`
}
type BuskerTune struct {
	ID   string `json:"id"`
	Name struct {
		RU string `json:"ru"`
		EN string `json:"en"`
	} `json:"name"`
	Origin  string         `json:"origin"`
	Key     string         `json:"key"`
	BPM     int            `json:"bpm"`
	Meter   int            `json:"meter"`
	Phrases []BuskerPhrase `json:"phrases"`
	Passes  int            `json:"passes"`
	// filled by ANG: the tune's localization key, its length in seconds and in server ticks
	NameKey string  `json:"name_key,omitempty"`
	Seconds float64 `json:"seconds,omitempty"`
	Ticks   int64   `json:"ticks,omitempty"`
}
type BuskerListeners struct {
	HearCells     int         `json:"hear_cells"`
	SlowMoveTicks int64       `json:"slow_move_ticks"`
	SlowPermille  int64       `json:"slow_permille"`
	StopPermille  int64       `json:"stop_permille"`
	Max           int         `json:"max"`
	ListenTicks   BuskerRange `json:"listen_ticks"`
	PayPermille   int64       `json:"pay_permille"`
	PayMinor      BuskerRange `json:"pay_minor"`
	PayTicks      int64       `json:"pay_ticks"`
	RingTicks     int64       `json:"ring_ticks"`
}
type BuskerHero struct {
	ReachCells int     `json:"reach_cells"`
	LeaveCells int     `json:"leave_cells"`
	TipsMinor  []int64 `json:"tips_minor"`
}
type BuskerCatalog struct {
	Actor      string            `json:"actor"`
	Level      string            `json:"level"`
	Account    string            `json:"account"`
	Kit        []string          `json:"kit"`
	Spot       BuskerSpot        `json:"spot"`
	Day        BuskerDay         `json:"day"`
	GapSeconds BuskerRange       `json:"gap_seconds"`
	Tunes      []BuskerTune      `json:"tunes"`
	Listeners  BuskerListeners   `json:"listeners"`
	Hero       BuskerHero        `json:"hero"`
	Lines      map[string]string `json:"lines"`
	// how the client holds the bayan on his thighs and sways (presentation only)
	View struct {
		Hold struct {
			ThighM   float64 `json:"thigh_m"`
			ForwardM float64 `json:"forward_m"`
			LeftM    float64 `json:"left_m"`
			TiltDeg  float64 `json:"tilt_deg"`
		} `json:"hold"`
		Sway struct {
			ChestRollDeg float64 `json:"chest_roll_deg"`
			HeadNodDeg   float64 `json:"head_nod_deg"`
		} `json:"sway"`
		Lamp *struct {
			At          [3]float64 `json:"at"`
			Aim         [3]float64 `json:"aim"`
			BeamDeg     float64    `json:"beam_deg"`
			Energy      float64    `json:"energy"`
			RangeM      float64    `json:"range_m"`
			Attenuation float64    `json:"attenuation"`
			Color       string     `json:"color"`
		} `json:"lamp,omitempty"`
	} `json:"view"`
	Sound struct {
		Clips  string  `json:"clips"`
		GainDB float64 `json:"gain_db"`
		MinM   float64 `json:"min_m"`
		MaxM   float64 `json:"max_m"`
		FadeS  float64 `json:"fade_s"`
	} `json:"sound"`
	// filled by ANG: the gap between two tunes in server ticks, the server's tick rate
	GapTicks BuskerRange `json:"gap_ticks"`
	TickHz   int         `json:"tick_hz"`
}

func readBusker(world cue.Value) (*BuskerCatalog, error) {
	v := world.LookupPath(cue.ParsePath("busker"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.busker: %w", err)
	}
	var out BuskerCatalog
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&out); err != nil {
		return nil, fmt.Errorf("world.busker: %w", err)
	}
	return &out, nil
}

// buskerNoteBeats: the beats of a phrase's melody — "pitch:beats" tokens (C4, F#5, Bb3) and rests "r:beats"; an error names
// the first token that is not one.
func buskerNoteBeats(notes string) (float64, error) {
	total := 0.0
	for _, tok := range strings.Fields(notes) {
		pitch, dur, ok := strings.Cut(tok, ":")
		if !ok {
			return 0, fmt.Errorf("note %q: want pitch:beats", tok)
		}
		if pitch != "r" {
			if _, err := BuskerMIDI(pitch); err != nil {
				return 0, fmt.Errorf("note %q: %v", tok, err)
			}
		}
		beats, err := strconv.ParseFloat(dur, 64)
		if err != nil || beats <= 0 || beats > 16 {
			return 0, fmt.Errorf("note %q: beats", tok)
		}
		total += beats
	}
	return total, nil
}

// BuskerMIDI: the MIDI number of a pitch name (C4 = 60; sharps "#", flats "b"); the bayan's range is E2..C7.
func BuskerMIDI(p string) (int, error) {
	if len(p) < 2 {
		return 0, fmt.Errorf("pitch %q", p)
	}
	steps := map[byte]int{'C': 0, 'D': 2, 'E': 4, 'F': 5, 'G': 7, 'A': 9, 'B': 11}
	n, ok := steps[p[0]]
	if !ok {
		return 0, fmt.Errorf("pitch %q", p)
	}
	rest := p[1:]
	switch rest[0] {
	case '#':
		n++
		rest = rest[1:]
	case 'b':
		n--
		rest = rest[1:]
	}
	octave, err := strconv.Atoi(rest)
	if err != nil {
		return 0, fmt.Errorf("pitch %q", p)
	}
	m := 12*(octave+1) + n
	if m < 40 || m > 96 {
		return 0, fmt.Errorf("pitch %q out of the bayan's range", p)
	}
	return m, nil
}

// applyBusker checks world.busker against the compiled Life, inventory, vending, peddlers, visitors and localization and
// fills the derived numbers. Runs after the visitors (their places are public places the spot must not take).
func applyBusker(c *Compiled) error {
	b := c.Busker
	if b == nil {
		return nil
	}
	fail := func(format string, args ...any) error {
		return fmt.Errorf("world.busker: %s", fmt.Sprintf(format, args...))
	}
	var life struct {
		Locations map[string]struct {
			Terrain []string `json:"terrain"`
			CellMM  int      `json:"cell_mm"`
		} `json:"locations"`
		Actors []struct {
			ID    string `json:"id"`
			Steps []struct {
				Level string     `json:"level"`
				At    BuskerCell `json:"at"`
			} `json:"steps"`
		} `json:"actors"`
		Brain struct {
			Places []struct {
				ID      string     `json:"id"`
				Kind    string     `json:"kind"`
				Level   string     `json:"level"`
				At      BuskerCell `json:"at"`
				Seating *struct {
					Seats []struct {
						ID string     `json:"id"`
						At BuskerCell `json:"at"`
					} `json:"seats"`
					Queue []struct {
						At BuskerCell `json:"at"`
					} `json:"queue"`
					Pickup []struct {
						At BuskerCell `json:"at"`
					} `json:"pickup"`
					Posts []struct {
						At BuskerCell `json:"at"`
					} `json:"posts"`
					Regulars []string `json:"regulars"`
				} `json:"seating"`
			} `json:"places"`
		} `json:"brain"`
	}
	if err := json.Unmarshal(c.Life, &life); err != nil {
		return fail("life: %v", err)
	}
	walkOn := func(level string, at BuskerCell) bool {
		loc, ok := life.Locations[level]
		return ok && at.Y >= 0 && at.Y < len(loc.Terrain) && at.X >= 0 && at.X < len(loc.Terrain[at.Y]) && c.Walkable[string(loc.Terrain[at.Y][at.X])]
	}
	loc, ok := life.Locations[b.Level]
	if !ok {
		return fail("unknown level %s", b.Level)
	}
	cellMM := int64(loc.CellMM)
	if cellMM <= 0 {
		return fail("level %s has no cell size", b.Level)
	}
	walk := func(at BuskerCell) bool { return walkOn(b.Level, at) }
	// the person: a Life actor with a routine; the account is his
	found := false
	for _, a := range life.Actors {
		if a.ID == b.Actor {
			found = len(a.Steps) > 0
		}
	}
	if !found {
		return fail("actor %s is not a Life actor with a routine", b.Actor)
	}
	if acc, ok := c.Inventory.Accounts[b.Account]; !ok || acc.Owner != b.Actor || acc.Currency != "CRD" {
		return fail("account %s: not a CRD account of %s", b.Account, b.Actor)
	}
	// his kit: one store he carries (the case on his back), with every thing of the kit in it once
	kit := BuskerKitStore(b.Actor)
	if _, dup := c.Inventory.Stores[kit]; dup {
		return fail("duplicate store %s", kit)
	}
	c.Inventory.Stores[kit] = inv.Store{ID: kit, Name: "busker.store.kit", Kind: "container", Anchor: inv.Anchor{Kind: "actor", Actor: b.Actor}, Owner: b.Actor, Locked: true,
		Capacity: inv.Capacity{Mass: 30000, Volume: 130000, Stacks: 8}}
	seenKit := map[string]bool{}
	for k, item := range b.Kit {
		if _, ok := c.Inventory.Definitions[item]; !ok || seenKit[item] {
			return fail("kit item %s: unknown or twice", item)
		}
		seenKit[item] = true
		c.Inventory.Sources = append(c.Inventory.Sources, inv.Source{ID: fmt.Sprintf("busker.kit.%02d", k), Definition: item, Quantity: 1, Condition: 1000000, Owner: b.Actor, Location: inv.Location{Kind: "store", Store: kit}})
	}
	// ---------------------------------------------------------------- the spot
	s := b.Spot
	inCell := func(at BuskerCell, mm BuskerMM) bool {
		return mm.X >= int64(at.X)*cellMM && mm.X < int64(at.X+1)*cellMM && mm.Y >= int64(at.Y)*cellMM && mm.Y < int64(at.Y+1)*cellMM
	}
	cheb := func(a, b BuskerCell) int { return max(absInt(a.X-b.X), absInt(a.Y-b.Y)) }
	if !walk(s.Seat) || !walk(s.Case) {
		return fail("the seat %v and the case %v must stand on the floor", s.Seat, s.Case)
	}
	if !inCell(s.Seat, s.SeatMM) || s.SeatMM.Z < 300 || s.SeatMM.Z > 600 {
		return fail("seat_mm %v: in the seat cell, 0.30-0.60 m high", s.SeatMM)
	}
	if !inCell(s.Case, s.CaseMM) || s.CaseMM.Z != 0 {
		return fail("case_mm %v: in the case cell, on the floor", s.CaseMM)
	}
	if cheb(s.Seat, s.Case) != 1 {
		return fail("the case %v stands right beside the seat %v", s.Case, s.Seat)
	}
	// the reader: in the case's cell (its end wall), at the height of a hand held out, in reach of every pay cell
	if s.ReaderMM.Z < 600 || s.ReaderMM.Z > 1300 || s.ReaderMM.X < int64(s.Case.X)*cellMM-cellMM/2 || s.ReaderMM.X > int64(s.Case.X+1)*cellMM+cellMM/2 ||
		s.ReaderMM.Y < int64(s.Case.Y)*cellMM-cellMM/2 || s.ReaderMM.Y > int64(s.Case.Y+1)*cellMM+cellMM/2 {
		return fail("reader_mm %v: on the case, 0.60-1.30 m high", s.ReaderMM)
	}
	if len(s.Pay) == 0 {
		return fail("no pay cell")
	}
	used := map[BuskerCell]string{s.Seat: "seat", s.Case: "case"}
	for _, p := range s.Pay {
		if !walk(p) || absInt(p.X-s.Case.X)+absInt(p.Y-s.Case.Y) != 1 {
			return fail("pay cell %v: floor, beside the case", p)
		}
		cx, cy := float64(p.X)*float64(cellMM)+float64(cellMM)/2, float64(p.Y)*float64(cellMM)+float64(cellMM)/2
		if d := math.Hypot(float64(s.ReaderMM.X)-cx, float64(s.ReaderMM.Y)-cy); d > 750 {
			return fail("pay cell %v: the reader is %.0f mm away (a hand reaches 750)", p, d)
		}
		if used[p] != "" {
			return fail("pay cell %v is also the %s", p, used[p])
		}
		used[p] = "pay cell"
	}
	if len(s.Listen) < b.Listeners.Max || b.Listeners.Max < 1 {
		return fail("listeners.max %d needs as many listening places (have %d)", b.Listeners.Max, len(s.Listen))
	}
	for _, l := range s.Listen {
		if !walk(l.At) {
			return fail("listening place %v is not floor", l.At)
		}
		if used[l.At] != "" {
			return fail("listening place %v is also the %s", l.At, used[l.At])
		}
		used[l.At] = "listening place"
		d := math.Hypot(float64(l.At.X-s.Seat.X), float64(l.At.Y-s.Seat.Y))
		if d < 2 || d > float64(b.Listeners.HearCells) {
			return fail("listening place %v is %.1f cells from the seat (2..hear_cells)", l.At, d)
		}
		// he faces the musician (within 50 degrees)
		fx, fy := l.Face.X, l.Face.Y
		tx, ty := float64(s.Seat.X-l.At.X), float64(s.Seat.Y-l.At.Y)
		n := math.Hypot(fx, fy) * math.Hypot(tx, ty)
		if n == 0 || (fx*tx+fy*ty)/n < math.Cos(50*math.Pi/180) {
			return fail("listening place %v does not face the musician", l.At)
		}
	}
	// the aisles keep their width: every cell of a keep rect is floor and none of the spot's cells is in it
	for _, k := range s.Keep {
		for y := k.Rect.Y; y < k.Rect.Y+k.Rect.H; y++ {
			for x := k.Rect.X; x < k.Rect.X+k.Rect.W; x++ {
				at := BuskerCell{X: x, Y: y}
				if !walk(at) {
					return fail("keep %s: cell %v of the aisle is not floor", k.ID, at)
				}
				if used[at] != "" {
					return fail("keep %s: the %s %v narrows the aisle", k.ID, used[at], at)
				}
			}
		}
		across := k.Rect.W
		if k.Axis == "y" {
			across = k.Rect.H
		}
		if float64(int64(across)*cellMM)/1000 < k.MinM {
			return fail("keep %s: %d cells across is under %.1f m", k.ID, across, k.MinM)
		}
	}
	// nobody else's public place: the torgomats' panels, lanes and service cells, the visitors' places, the peddlers'
	// stops, the brain's places (seats, queues, pickups, posts) and every other person's routine cells on the level
	public := map[BuskerCell]string{}
	if v := c.Vending; v != nil && v.Level == b.Level {
		for _, u := range v.Units {
			public[BuskerCell{u.Panel.X, u.Panel.Y}] = "panel of " + u.ID
			for _, a := range u.Approach {
				public[BuskerCell{a.X, a.Y}] = "approach of " + u.ID
			}
			for _, a := range u.Lane {
				public[BuskerCell{a.X, a.Y}] = "lane of " + u.ID
			}
		}
	}
	if v := c.Visitors; v != nil && v.Level == b.Level {
		for _, t := range v.Targets {
			public[BuskerCell{t.At.X, t.At.Y}] = "visitors' place " + t.ID
			for _, a := range t.Spots {
				public[BuskerCell{a.X, a.Y}] = "standing cell of " + t.ID
			}
		}
		for _, e := range v.Entries {
			public[BuskerCell{e.At.X, e.At.Y}] = "visitors' entry " + e.ID
		}
	}
	if p := c.Peddlers; p != nil {
		for _, pd := range p.Peddlers {
			for _, st := range pd.Stops {
				public[BuskerCell{st.At.X, st.At.Y}] = "stop " + st.ID + " of " + pd.Actor
			}
		}
	}
	for _, pl := range life.Brain.Places {
		if pl.Level != b.Level {
			continue
		}
		public[pl.At] = "brain place " + pl.ID
		if q := pl.Seating; q != nil {
			for _, x := range q.Seats {
				public[x.At] = "seat " + x.ID
			}
			for _, g := range [][]struct {
				At BuskerCell `json:"at"`
			}{q.Queue, q.Pickup, q.Posts} {
				for _, x := range g {
					public[x.At] = "spot of " + pl.ID
				}
			}
		}
	}
	for _, a := range life.Actors {
		if a.ID == b.Actor {
			continue
		}
		for _, st := range a.Steps {
			if st.Level == b.Level {
				public[st.At] = "routine cell of " + a.ID
			}
		}
	}
	for at, what := range used {
		if p := public[at]; p != "" {
			return fail("the %s %v takes the %s", what, at, p)
		}
		// the torgomat block's end cells for the service robot: no spot cell right beside a unit's service approach either
	}
	// ---------------------------------------------------------------- the day
	d := &b.Day
	if d.Arrive.From < 0 || d.Arrive.To <= d.Arrive.From || d.Leave.From <= d.Arrive.To || d.Leave.To < d.Leave.From || d.Leave.To >= 86400 {
		return fail("day: arrive %v / leave %v", d.Arrive, d.Leave)
	}
	if d.SetSeconds.Min < 300 || d.SetSeconds.Max < d.SetSeconds.Min {
		return fail("day: set_seconds %v", d.SetSeconds)
	}
	rest := false
	for i := range d.Breaks {
		br := &d.Breaks[i]
		if br.Seconds.Min < 60 || br.Seconds.Max < br.Seconds.Min || br.Weight < 1 || br.MaxPerDay < 1 || br.Label == "" {
			return fail("break %s: numbers", br.ID)
		}
		switch br.ID {
		case "rest":
			rest = true
			if br.At != nil || len(br.Seats) > 0 {
				return fail("break rest stays at the spot")
			}
		default:
			if br.At == nil || !walkOn(br.At.Level, br.At.At) {
				return fail("break %s: a floor cell to go to", br.ID)
			}
		}
		if len(br.Seats) > 0 {
			// the seats of a seated place he is a regular of (he holds the stool as a routine post)
			for _, pl := range life.Brain.Places {
				q := pl.Seating
				if pl.Kind != "seated_food" || q == nil || pl.Level != br.At.Level {
					continue
				}
				cells := []BuskerCell{}
				for _, id := range br.Seats {
					for _, x := range q.Seats {
						if x.ID == id {
							cells = append(cells, x.At)
						}
					}
				}
				if len(cells) == len(br.Seats) {
					regular := false
					for _, r := range q.Regulars {
						regular = regular || r == b.Actor
					}
					if !regular {
						return fail("break %s: %s is not a regular of %s", br.ID, b.Actor, pl.ID)
					}
					br.Place, br.SeatCells = pl.ID, cells
				}
			}
			if br.Place == "" {
				return fail("break %s: seats %v of no seated place", br.ID, br.Seats)
			}
		}
	}
	if !rest {
		return fail("a rest break at the spot is needed (the place he plays from)")
	}
	// ---------------------------------------------------------------- tunes
	hz := c.Config.TickHz
	if hz < 1 {
		return fail("config tick_hz %d", hz)
	}
	b.TickHz = hz
	if b.GapSeconds.Min < 1 || b.GapSeconds.Max < b.GapSeconds.Min {
		return fail("gap_seconds %v", b.GapSeconds)
	}
	b.GapTicks = BuskerRange{Min: b.GapSeconds.Min * int64(hz), Max: b.GapSeconds.Max * int64(hz)}
	texts := map[string]bool{}
	for _, m := range c.Localization.Messages {
		if m.Texts["ru"] != "" && m.Texts["en"] != "" {
			texts[m.Key] = true
		}
	}
	seen := map[string]bool{}
	for i := range b.Tunes {
		t := &b.Tunes[i]
		if seen[t.ID] {
			return fail("tune %s twice", t.ID)
		}
		seen[t.ID] = true
		t.NameKey = "musician.tune." + strings.TrimPrefix(t.ID, "tune.")
		if !texts[t.NameKey] {
			return fail("tune %s: no text %s", t.ID, t.NameKey)
		}
		beats := 0.0
		for k := range t.Phrases {
			p := &t.Phrases[k]
			p.Beats = float64(p.Bars * t.Meter)
			p.Bellows = "open"
			if k%2 == 1 {
				p.Bellows = "close"
			}
			if p.Notes != "" {
				got, err := buskerNoteBeats(p.Notes)
				if err != nil {
					return fail("tune %s phrase %d: %v", t.ID, k+1, err)
				}
				if math.Abs(got-p.Beats) > 1e-6 {
					return fail("tune %s phrase %d: the notes hold %.3f beats, the phrase %v", t.ID, k+1, got, p.Beats)
				}
			}
			beats += p.Beats
		}
		if len(t.Phrases)%2 == 1 && t.Passes > 1 {
			return fail("tune %s: an odd number of phrases turns the bellows the other way on the next pass", t.ID)
		}
		t.Seconds = beats * float64(t.Passes) * 60 / float64(t.BPM)
		t.Ticks = int64(math.Round(t.Seconds * float64(hz)))
		if t.Seconds < 30 || t.Seconds > 600 {
			return fail("tune %s: %.0f s (30..600)", t.ID, t.Seconds)
		}
	}
	// ---------------------------------------------------------------- listeners, hero, lines
	l := b.Listeners
	if l.ListenTicks.Min < int64(hz) || l.ListenTicks.Max < l.ListenTicks.Min || l.PayMinor.Min < 1 || l.PayMinor.Max < l.PayMinor.Min || l.RingTicks > l.PayTicks {
		return fail("listeners: numbers")
	}
	if c.Visitors == nil || c.Visitors.Level != b.Level {
		return fail("the listeners are the hall's visitors of %s (world.visitors)", b.Level)
	}
	if l.SlowMoveTicks < c.Visitors.MoveTicks {
		return fail("listeners.slow_move_ticks %d is faster than the visitors' pace %d", l.SlowMoveTicks, c.Visitors.MoveTicks)
	}
	if h := b.View.Hold; h.ThighM < 0 || h.ThighM > 0.2 || h.ForwardM < 0.1 || h.ForwardM > 0.4 || math.Abs(h.LeftM) > 0.2 || math.Abs(h.TiltDeg) > 25 ||
		b.View.Sway.ChestRollDeg < 0 || b.View.Sway.ChestRollDeg > 8 || b.View.Sway.HeadNodDeg < 0 || b.View.Sway.HeadNodDeg > 10 {
		return fail("view: the hold or the sway out of range")
	}
	// the lamp hangs over the spot: within 2 m of the seat across the plan, above head height, aimed down
	if l := b.View.Lamp; l != nil {
		sx, sy := float64(s.SeatMM.X)/1000, float64(s.SeatMM.Y)/1000
		if math.Hypot(l.At[0]-sx, l.At[1]-sy) > 2 || l.At[2] < 1.6 || l.Aim[2] >= l.At[2] || math.Hypot(l.Aim[0]-sx, l.Aim[1]-sy) > 1 {
			return fail("view.lamp %v -> %v does not light the seat %v", l.At, l.Aim, s.SeatMM)
		}
	}
	if s := b.Sound; !strings.HasPrefix(s.Clips, "res://") || !strings.HasSuffix(s.Clips, "/") || s.GainDB > 0 || s.MinM <= 0 || s.MaxM <= s.MinM || s.FadeS <= 0 {
		return fail("sound: numbers")
	}
	if len(b.Hero.TipsMinor) == 0 || b.Hero.ReachCells < 1 || b.Hero.LeaveCells <= b.Hero.ReachCells {
		return fail("hero: numbers")
	}
	for _, label := range []string{d.Labels.Walk, d.Labels.Unpack, d.Labels.Play, d.Labels.Rest, d.Labels.Pack, d.Labels.Home} {
		if !texts[label] {
			return fail("day label %q has no text", label)
		}
	}
	for _, br := range d.Breaks {
		if !texts[br.Label] {
			return fail("break %s: label %q has no text", br.ID, br.Label)
		}
	}
	for _, want := range []string{"intro_play", "intro_rest", "ask_tune", "tune", "tune_none", "ask_bayan", "bayan", "ask_why", "why", "tip", "thanks", "thanks_big", "no_money", "listen", "bye"} {
		key := b.Lines[want]
		if key == "" || !texts[key] {
			return fail("line %s (%q) has no text", want, key)
		}
	}
	return nil
}

// BuskerKitStore: the store of the musician's kit (carried).
func BuskerKitStore(actor string) string {
	return "busker/" + strings.TrimPrefix(actor, "char.") + "/kit"
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// clientBusker: what the client draws of him — the spot, the props' place and the tunes' phrase plans (the bellows) and
// tempo (the sway); the day, the money and the listeners' rule stay on the server.
func clientBusker(b *BuskerCatalog) map[string]any {
	tunes := make([]map[string]any, 0, len(b.Tunes))
	for _, t := range b.Tunes {
		phrases := make([]map[string]any, 0, len(t.Phrases))
		for _, p := range t.Phrases {
			phrases = append(phrases, map[string]any{"beats": p.Beats, "dyn": p.Dyn, "bellows": p.Bellows})
		}
		tunes = append(tunes, map[string]any{"id": t.ID, "bpm": t.BPM, "meter": t.Meter, "passes": t.Passes, "seconds": t.Seconds, "phrases": phrases})
	}
	return map[string]any{
		"actor": b.Actor, "level": b.Level, "tick_hz": b.TickHz,
		"seat_mm": b.Spot.SeatMM, "yaw_deg": b.Spot.YawDeg, "case_mm": b.Spot.CaseMM, "case_yaw_deg": b.Spot.CaseYawDeg,
		"reader_mm": b.Spot.ReaderMM, "tunes": tunes, "hold": b.View.Hold, "sway": b.View.Sway, "lamp": b.View.Lamp, "sound": b.Sound,
	}
}
