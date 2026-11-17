package gamepoc

import (
	"encoding/json"
	"strings"
	"testing"

	inv "github.com/strogmv/ang/compiler/inventory/runtime"
)

// buskerFixture: a 20 x 12 hall of floor with a wall column, the musician's Life actor, a yatai place with a stool he is a
// regular of, his account and kit, the visitors of the level, the texts, and a valid world.busker.
func buskerFixture() Compiled {
	rows := []string{}
	for y := 0; y < 12; y++ {
		row := []byte(strings.Repeat(".", 20))
		if y >= 2 && y <= 9 {
			row[0] = '#'
		}
		rows = append(rows, string(row))
	}
	life := map[string]any{
		"locations": map[string]any{"level.m": map[string]any{"terrain": rows, "cell_mm": 500}},
		"actors": []any{
			map[string]any{"id": "char.m", "steps": []any{map[string]any{"level": "level.m", "at": map[string]int{"x": 19, "y": 11}}}},
			map[string]any{"id": "char.other", "steps": []any{map[string]any{"level": "level.m", "at": map[string]int{"x": 3, "y": 3}}}},
		},
		"brain": map[string]any{"places": []any{map[string]any{"id": "food.y", "kind": "seated_food", "level": "level.m", "at": map[string]int{"x": 15, "y": 1},
			"seating": map[string]any{"seats": []any{map[string]any{"id": "y/e1", "at": map[string]int{"x": 16, "y": 1}}}, "regulars": []string{"char.m"}}}}},
	}
	raw, _ := json.Marshal(life)
	texts := map[string]string{}
	msgs := []LocalizationMessage{}
	add := func(k string) {
		if texts[k] == "" {
			texts[k] = k
			msgs = append(msgs, LocalizationMessage{Key: k, Texts: map[string]string{"ru": k, "en": k}})
		}
	}
	b := &BuskerCatalog{Actor: "char.m", Level: "level.m", Account: "acc.m", Kit: []string{"item.bayan"}}
	b.Spot = BuskerSpot{Seat: BuskerCell{8, 6}, SeatMM: BuskerMM{4250, 3250, 450}, YawDeg: -70, Case: BuskerCell{8, 5}, CaseMM: BuskerMM{4250, 2750, 0},
		ReaderMM: BuskerMM{4000, 2750, 800}, Pay: []BuskerCell{{7, 5}},
		Listen: []BuskerListenSpot{{At: BuskerCell{5, 6}}, {At: BuskerCell{5, 4}}}}
	b.Spot.Listen[0].Face.X = 1
	b.Spot.Listen[1].Face.X, b.Spot.Listen[1].Face.Y = 3, 2
	k := BuskerKeep{ID: "aisle", Axis: "x", MinM: 1.0}
	k.Rect.X, k.Rect.Y, k.Rect.W, k.Rect.H = 1, 0, 3, 12
	b.Spot.Keep = []BuskerKeep{k}
	b.Spot.Front = BuskerFront{DepthM: 2.5, HalfM: 0.65, CameraAzimuthDeg: 37}
	b.Day.AbsentPermille = 150
	b.Day.Arrive.From, b.Day.Arrive.To = 30000, 33000
	b.Day.Leave.From, b.Day.Leave.To = 57000, 60000
	b.Day.SetSeconds = BuskerRange{2100, 3300}
	b.Day.UnpackSeconds, b.Day.PackSeconds = 60, 60
	b.Day.Labels.Walk, b.Day.Labels.Unpack, b.Day.Labels.Play, b.Day.Labels.Rest, b.Day.Labels.Pack, b.Day.Labels.Home = "l.walk", "l.unpack", "l.play", "l.rest", "l.pack", "l.home"
	b.Day.Breaks = []BuskerBreak{
		{ID: "rest", Seconds: BuskerRange{480, 840}, Weight: 5, MaxPerDay: 6, Label: "l.rest"},
		{ID: "tea", Seconds: BuskerRange{900, 1300}, Weight: 3, MaxPerDay: 1, Label: "l.tea", At: &BuskerBreakPlace{Level: "level.m", At: BuskerCell{16, 1}}, Seats: []string{"y/e1"}},
	}
	b.GapSeconds = BuskerRange{5, 14}
	b.Tunes = []BuskerTune{{ID: "tune.a", BPM: 100, Meter: 2, Passes: 4, Phrases: []BuskerPhrase{{Bars: 4, Dyn: 0.5, Notes: "C4:2 E4:2 G4:4"}, {Bars: 4, Dyn: 0.5}}}}
	b.Listeners = BuskerListeners{HearCells: 10, SlowMoveTicks: 7, SlowPermille: 500, StopPermille: 200, Max: 2, ListenTicks: BuskerRange{600, 1500}, PayPermille: 400, PayMinor: BuskerRange{5, 20}, PayTicks: 30, RingTicks: 6}
	b.Hero = BuskerHero{ReachCells: 2, LeaveCells: 5, TipsMinor: []int64{10, 50}}
	b.View.Hold.ThighM, b.View.Hold.ForwardM, b.View.Hold.LeftM, b.View.Hold.TiltDeg = 0.07, 0.22, 0.05, 6
	b.View.Sway.ChestRollDeg, b.View.Sway.HeadNodDeg = 2.5, 3
	b.Sound.Clips, b.Sound.GainDB, b.Sound.MinM, b.Sound.MaxM, b.Sound.FadeS = "res://assets/audio/musician/", -6, 3, 16, 1.2
	b.Lines = map[string]string{}
	for _, l := range []string{"intro_play", "intro_rest", "ask_tune", "tune", "tune_none", "ask_bayan", "bayan", "ask_why", "why", "tip", "thanks", "thanks_big", "no_money", "listen", "bye"} {
		b.Lines[l] = "line." + l
		add("line." + l)
	}
	for _, l := range []string{"l.walk", "l.unpack", "l.play", "l.rest", "l.pack", "l.home", "l.tea", "musician.tune.a"} {
		add(l)
	}
	c := Compiled{Life: raw, Walkable: map[string]bool{".": true}, Busker: b}
	c.Config.TickHz = 10
	c.Localization.Messages = msgs
	c.Inventory = inv.Model{Accounts: map[string]inv.Account{"acc.m": {ID: "acc.m", Owner: "char.m", Currency: "CRD"}}, Stores: map[string]inv.Store{}, Definitions: map[string]inv.Definition{"item.bayan": {}}}
	c.Visitors = &VisitorCatalog{Level: "level.m", MoveTicks: 5}
	return c
}

func TestBuskerSpotDayAndTunesAgainstTheHall(t *testing.T) {
	c := buskerFixture()
	if err := applyBusker(&c); err != nil {
		t.Fatalf("a valid musician rejected: %v", err)
	}
	if b := c.Busker; b.Tunes[0].Ticks != int64(b.Tunes[0].Seconds*10+0.5) || b.Tunes[0].Phrases[1].Bellows != "close" || b.Day.Breaks[1].Place != "food.y" {
		t.Fatalf("derived numbers: %+v", b.Tunes[0])
	}
	if _, ok := c.Inventory.Stores[BuskerKitStore("char.m")]; !ok || len(c.Inventory.Sources) != 1 {
		t.Fatal("no carried kit store with the bayan in it")
	}
	// the clear front: from the seat (8,6) toward the camera at azimuth 37 (x east, y south), 2.5 m deep, 0.65 m either side
	want := []BuskerCell{{9, 6}, {8, 7}, {9, 7}, {10, 7}, {8, 8}, {9, 8}, {10, 8}, {11, 8}, {9, 9}, {10, 9}, {11, 9}, {10, 10}, {11, 10}}
	if got := c.Busker.Spot.Clear; len(got) != len(want) {
		t.Fatalf("clear front %v, want %v", got, want)
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("clear front %v, want %v", got, want)
			}
		}
	}
	terrain := func(c *Compiled, edit func(rows []string) []string) {
		var life map[string]any
		_ = json.Unmarshal(c.Life, &life)
		loc := life["locations"].(map[string]any)["level.m"].(map[string]any)
		var rows []string
		for _, r := range loc["terrain"].([]any) {
			rows = append(rows, r.(string))
		}
		loc["terrain"] = edit(rows)
		c.Life, _ = json.Marshal(life)
	}
	cases := []struct {
		name, want string
		edit       func(c *Compiled)
	}{
		{"seat on a wall", "on the floor", func(c *Compiled) {
			c.Busker.Spot.Seat = BuskerCell{0, 6}
			c.Busker.Spot.SeatMM = BuskerMM{250, 3250, 450}
		}},
		{"case far from the seat", "right beside the seat", func(c *Compiled) {
			c.Busker.Spot.Case = BuskerCell{8, 2}
			c.Busker.Spot.CaseMM = BuskerMM{4250, 1250, 0}
		}},
		{"reader out of reach", "a hand reaches", func(c *Compiled) {
			c.Busker.Spot.Pay = []BuskerCell{{8, 4}}
			c.Busker.Spot.ReaderMM = BuskerMM{4490, 2990, 800}
		}},
		{"listener in the aisle", "narrows the aisle", func(c *Compiled) {
			c.Busker.Spot.Listen[1].At = BuskerCell{3, 6}
			c.Busker.Spot.Listen[1].Face.X, c.Busker.Spot.Listen[1].Face.Y = 1, 0
		}},
		{"listener facing away", "does not face", func(c *Compiled) { c.Busker.Spot.Listen[0].Face.X = -1 }},
		{"another person's post", "routine cell of char.other", func(c *Compiled) {
			c.Busker.Spot.Listen[1].At = BuskerCell{3, 3}
			c.Busker.Spot.Keep = nil
			c.Busker.Spot.Listen[1].Face.X, c.Busker.Spot.Listen[1].Face.Y = 5, 3
		}},
		{"aisle too narrow", "is under", func(c *Compiled) { c.Busker.Spot.Keep[0].MinM = 2.0 }},
		{"more listeners than places", "listening places", func(c *Compiled) { c.Busker.Listeners.Max = 3 }},
		{"tea seat of no place he is regular of", "is not a regular", func(c *Compiled) {
			var life map[string]any
			_ = json.Unmarshal(c.Life, &life)
			life["brain"].(map[string]any)["places"].([]any)[0].(map[string]any)["seating"].(map[string]any)["regulars"] = []string{}
			c.Life, _ = json.Marshal(life)
		}},
		{"notes do not fill the phrase", "the notes hold", func(c *Compiled) { c.Busker.Tunes[0].Phrases[0].Notes = "C4:2 E4:2" }},
		{"a note out of the bayan", "range", func(c *Compiled) { c.Busker.Tunes[0].Phrases[0].Notes = "C1:8" }},
		{"odd phrases turn the bellows", "odd number of phrases", func(c *Compiled) {
			c.Busker.Tunes[0].Phrases = append(c.Busker.Tunes[0].Phrases, BuskerPhrase{Bars: 4, Dyn: 0.5})
		}},
		{"a tune without a name", "no text", func(c *Compiled) { c.Busker.Tunes[0].ID = "tune.b" }},
		{"a line without a text", "has no text", func(c *Compiled) { c.Busker.Lines["bye"] = "line.none" }},
		{"no rest break", "rest break", func(c *Compiled) { c.Busker.Day.Breaks = c.Busker.Day.Breaks[1:] }},
		{"the account is somebody else's", "not a CRD account", func(c *Compiled) {
			c.Inventory.Accounts["acc.m"] = inv.Account{ID: "acc.m", Owner: "char.other", Currency: "CRD"}
		}},
		{"listeners slower than walking", "faster than the visitors", func(c *Compiled) { c.Busker.Listeners.SlowMoveTicks = 3 }},
		{"a listener between him and the camera", "stands between him and the camera", func(c *Compiled) {
			c.Busker.Spot.Listen[0].At = BuskerCell{10, 8}
			c.Busker.Spot.Listen[0].Face.X, c.Busker.Spot.Listen[0].Face.Y = -1, -1
		}},
		{"a counter in front of him", "is not floor (it hides him)", func(c *Compiled) {
			terrain(c, func(rows []string) []string {
				r := []byte(rows[9])
				r[10] = '#'
				rows[9] = string(r)
				return rows
			})
		}},
		{"the front cuts a visitors' place off", "no longer reach t", func(c *Compiled) {
			// rows 10-11 walled but for the front's (10,10)/(11,10) and the place (10,11) under them
			terrain(c, func(rows []string) []string {
				r10, r11 := []byte(strings.Repeat("#", 20)), []byte(strings.Repeat("#", 20))
				r10[10], r10[11], r11[10] = '.', '.', '.'
				rows[10], rows[11] = string(r10), string(r11)
				return rows
			})
			c.Busker.Spot.Keep = nil
			c.Visitors.Entries = []VisitorEntry{{ID: "e", At: inv.Cell{X: 19, Y: 0}}}
			c.Visitors.Targets = []VisitorTarget{{ID: "t", At: inv.Cell{X: 10, Y: 11}}}
		}},
		{"a front too shallow", "depth_m 1.5-3.5", func(c *Compiled) { c.Busker.Spot.Front.DepthM = 1.0 }},
	}
	for _, tc := range cases {
		c := buskerFixture()
		tc.edit(&c)
		err := applyBusker(&c)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: want an error with %q, got %v", tc.name, tc.want, err)
		}
	}
}
