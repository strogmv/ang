package barks

import (
	"encoding/json"
	"reflect"
	"testing"
)

func ms(text string) map[string]Measure {
	return map[string]Measure{"ru": MeasureText(text), "en": MeasureText(text)}
}

// fixture: a yatai with a seated regular, the owner at his till, a gate place with a place fact, and three combat voices.
func fixture() Model {
	m := Model{
		TickHz: 10, MaxActive: 3, MovingTicks: 20, Locales: []string{"ru", "en"},
		Timing: Timing{CommaMS: 90, StopMS: 170, FixPauseMS: 220, StrikeMS: 180, SetMS: 120, StampMinMS: 140, HoldMinMS: 3200, HoldBaseMS: 900, WordsPerMinute: 156, FadeMS: 600, OffscreenFadeMS: 300, GestureMS: 1600},
		Layout: Layout{EMPx: 18, MaxWidthEM: 17, MaxLines: 2, LineLimitRunes: 52},
		Faces: []Face{
			{Caste: "bronze", Face: "stencil", Reveal: "stamped", Correction: "restamp", CPS: 26, Capitals: true, Punctuation: true, Colour: "#f0d9a8", Speaks: true, WordsMax: 8},
			{Caste: "copper", Face: "chalk", Reveal: "written", Correction: "cross_out", CPS: 18, Colour: "#c8b183", Speaks: true, WordsMax: 8},
			{Caste: "none", Face: "chalk", Reveal: "written", Correction: "cross_out", CPS: 22, Capitals: true, Punctuation: true, Colour: "#d8d2c4", Speaks: true, WordsMax: 8},
			{Caste: "gold", Face: "matrix", Reveal: "set", Correction: "none", CPS: 30, Colour: "#f2e6c4", WordsMax: 10, Undecided: true},
		},
		Castes: map[string]string{"char.loader": "bronze", "char.owner": "bronze", "char.porter": "copper", "char.hero": "none"},
		Events: []Event{{ID: "gate.locked", Source: "go", Scope: "place"}, {ID: "crane.stopped", Source: "stub", Scope: "world"}, {ID: "craft.order", Source: "go", Scope: "speaker"}},
		Places: []Place{
			{ID: "food.yatai", Kind: "seated_stall", Level: "hub", Spots: []Cell{{10, 10}, {12, 9}}},
			{ID: "gate.entry", Kind: "gate", Level: "hub", Rects: []Rect{{30, 0, 40, 6}}, Subjects: []string{"gate.entry"}},
			{ID: "hall", Kind: "public", Level: "hub", Rects: []Rect{{0, 0, 60, 60}}},
		},
		Packs: []Pack{
			{Place: "food.yatai", Periods: []Period{{ID: "evening", From: 18 * 3600, To: 22 * 3600}}, HeroWithinCells: 8, GapSeconds: 300, MaxPerHour: 4, MeanWaitSeconds: 1,
				Lines:   []Line{{ID: "bark.yatai.top_up", Speaker: "char.loader", Caste: "bronze", To: "char.owner", When: []string{"evening"}, Requires: Requires{SpeakerSeated: true, TargetOnPost: true, TargetWithinCells: 6}, Weight: 1, CooldownSeconds: 1800, Reply: "reply.yatai.coming", Measure: ms("Эй, долей!")}},
				Replies: []Reply{{ID: "reply.yatai.coming", Speaker: "char.owner", Caste: "bronze", Kind: "line", Key: "bark.yatai.reply_coming", DelaySeconds: 30, Measure: ms("Сейчас.")}}},
			{Place: "gate.entry", Periods: []Period{{ID: "night", From: 22 * 3600, To: 6 * 3600}}, HeroWithinCells: 10, GapSeconds: 60, MaxPerHour: 6, MeanWaitSeconds: 1,
				Lines: []Line{
					{ID: "bark.gate.shut", Speaker: Anyone, Caste: "copper", To: "hall", When: []string{"night"}, React: []React{{Event: "gate.locked", WithinSeconds: 3600}}, Weight: 1, CooldownSeconds: 600, Measure: ms("опять закрыли")},
					{ID: "bark.gate.crane", Speaker: Anyone, Caste: "copper", To: "hall", When: []string{"night"}, React: []React{{Event: "crane.stopped", WithinSeconds: 3600}}, Weight: 9, CooldownSeconds: 600, Measure: ms("кран встал опять")},
				}},
			{Place: "hall", Periods: []Period{{ID: "day", From: 0, To: 86400}}, HeroWithinCells: 6, GapSeconds: 60, MaxPerHour: 30, MeanWaitSeconds: 1,
				Lines: []Line{
					{ID: "bark.hall.push", Speaker: Anyone, Caste: "copper", To: "hall", When: []string{"day"}, Requires: Requires{HeroPassingCells: 2}, Weight: 1, CooldownSeconds: 600, Measure: ms("куда прёшь")},
					{ID: "bark.hall.order", Speaker: Anyone, Caste: "bronze", To: "hall", When: []string{"day"}, React: []React{{Event: "craft.order", WithinSeconds: 7200}}, Weight: 1, CooldownSeconds: 600, Measure: ms("Заказ сам себя не сделает.")},
				}},
		},
		Combat: Combat{HeroWithinCells: 24, Intents: []string{"spotted", "wounded_retreat", "lure", "surround"}, RollMax: 20, CunningRoles: []string{"leader"},
			Voices:     []Voice{{ID: "guard", Caste: "bronze"}, {ID: "bandit", Caste: "copper"}, {ID: "pack", Beast: true}},
			Speakers:   map[string]string{"char.guard": "guard"},
			Archetypes: map[string]string{"arch.outlaw": "bandit", "arch.wolf": "pack"},
			Lines: []CombatLine{
				{ID: "bark.guard.spotted", Voice: "guard", Intent: "spotted", Weight: 1, CooldownSeconds: 30, Measure: ms("Стоять! Проверка.")},
				{ID: "bark.bandit.wounded", Voice: "bandit", Intent: "wounded_retreat", Weight: 1, CooldownSeconds: 30, Measure: ms("зацепило отхожу")},
				{ID: "bark.pack.surround", Voice: "pack", Intent: "surround", Weight: 1, CooldownSeconds: 30, Sign: "howl", Sfx: "sfx.pack.howl"},
			},
			Deceptions: []Deception{{Declared: "wounded_retreat", Truth: "lure", Tells: []Tell{{Key: "bark.tell.no_blood", Skill: "medicine", Threshold: 40, Measure: ms("Рана не кровит.")}, {Key: "bark.tell.too_even", Skill: "observation", Threshold: 55, Measure: ms("Отходит слишком ровно.")}}}},
		},
	}
	return m.Sorted()
}

func evening(tick int64, hero Cell, actors ...Actor) Frame {
	return Frame{Now: 19*3600 + tick, Tick: tick, Seed: 7, Hero: Actor{ID: "char.hero", Level: "hub", At: hero}, Actors: actors}
}

var (
	loaderSeated = Actor{ID: "char.loader", Level: "hub", At: Cell{10, 10}, Seated: true}
	ownerOnPost  = Actor{ID: "char.owner", Level: "hub", At: Cell{12, 9}, OnPost: true}
)

func TestModelValidates(t *testing.T) {
	if err := fixture().Validate(); err != nil {
		t.Fatal(err)
	}
}

func run(m Model, s *State, from, to int64, frame func(int64) Frame) []Speech {
	var out []Speech
	for tick := from; tick <= to; tick++ {
		out = append(out, m.Step(s, frame(tick))...)
	}
	return out
}

// The line plays only at its place, with the speaker seated and the owner on his post, and the owner answers after the delay.
func TestLineAtItsPlaceAndReplyChain(t *testing.T) {
	m := fixture()
	s := State{}
	said := run(m, &s, 1, 200, func(tk int64) Frame { return evening(tk, Cell{14, 12}, loaderSeated, ownerOnPost) })
	if len(said) < 2 || said[0].Line != "bark.yatai.top_up" || said[1].Kind != "reply" || said[1].Speaker != "char.owner" || said[1].Key != "bark.yatai.reply_coming" {
		t.Fatalf("expected line + reply, got %+v", said)
	}
	if said[1].Start < said[0].Until-m.ticks(m.Timing.FadeMS) || said[1].Second < said[0].Second+30 {
		t.Fatalf("reply came before its delay: %+v", said)
	}
	// not seated -> silent; owner off his post -> silent
	s2 := State{}
	standing := loaderSeated
	standing.Seated = false
	if got := run(m, &s2, 1, 200, func(tk int64) Frame { return evening(tk, Cell{14, 12}, standing, ownerOnPost) }); len(got) != 0 {
		t.Fatalf("a standing loader spoke the stool's line: %+v", got)
	}
	away := ownerOnPost
	away.OnPost = false
	s3 := State{}
	if got := run(m, &s3, 1, 200, func(tk int64) Frame { return evening(tk, Cell{14, 12}, loaderSeated, away) }); len(got) != 0 {
		t.Fatalf("spoke to an owner who is not at his till: %+v", got)
	}
	// out of period (noon)
	s4 := State{}
	noon := func(tk int64) Frame {
		f := evening(tk, Cell{14, 12}, loaderSeated, ownerOnPost)
		f.Now = 12*3600 + tk
		return f
	}
	if got := run(m, &s4, 1, 200, noon); len(got) != 0 {
		t.Fatalf("an evening line at noon: %+v", got)
	}
}

// Hearing (doc 20): beyond the pack's radius the hero hears nothing, inside he does.
func TestHearing(t *testing.T) {
	m := fixture()
	s := State{}
	if got := run(m, &s, 1, 200, func(tk int64) Frame { return evening(tk, Cell{10, 19}, loaderSeated, ownerOnPost) }); len(got) != 0 {
		t.Fatalf("heard at 9 cells with a radius of 8: %+v", got)
	}
	s = State{}
	if got := run(m, &s, 1, 200, func(tk int64) Frame { return evening(tk, Cell{10, 17}, loaderSeated, ownerOnPost) }); len(got) == 0 {
		t.Fatal("not heard at 7 cells")
	}
}

// Cooldown of the line and the gap of the place: the same line does not come back before its cooldown.
func TestCooldownAndGap(t *testing.T) {
	m := fixture()
	s := State{}
	said := run(m, &s, 1, 1700, func(tk int64) Frame { return evening(tk, Cell{14, 12}, loaderSeated, ownerOnPost) })
	n := 0
	for _, sp := range said {
		if sp.Line == "bark.yatai.top_up" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("the line repeated within its 1800 s cooldown: %d times", n)
	}
	more := run(m, &s, 1801, 2600, func(tk int64) Frame { return evening(tk, Cell{14, 12}, loaderSeated, ownerOnPost) })
	if len(more) == 0 {
		t.Fatal("the line never came back after its cooldown")
	}
}

// The cooldown survives a save: after a JSON round trip the loaded state still remembers the line.
func TestCooldownSurvivesSave(t *testing.T) {
	m := fixture()
	s := State{}
	run(m, &s, 1, 300, func(tk int64) Frame { return evening(tk, Cell{14, 12}, loaderSeated, ownerOnPost) })
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var loaded State
	if err := json.Unmarshal(raw, &loaded); err != nil {
		t.Fatal(err)
	}
	if err := m.ValidateState(loaded, 19*3600+300, 300); err != nil {
		t.Fatal(err)
	}
	if got := run(m, &loaded, 301, 1500, func(tk int64) Frame { return evening(tk, Cell{14, 12}, loaderSeated, ownerOnPost) }); len(got) != 0 {
		for _, sp := range got {
			if sp.Line == "bark.yatai.top_up" {
				t.Fatalf("the loaded game repeated the line early: %+v", sp)
			}
		}
	}
	// a fresh state (as if the cooldowns were not saved) would repeat it: the save is what prevents it
	fresh := State{}
	if got := run(m, &fresh, 301, 600, func(tk int64) Frame { return evening(tk, Cell{14, 12}, loaderSeated, ownerOnPost) }); len(got) == 0 {
		t.Fatal("control: a fresh state should speak")
	}
}

// Determinism: the same seed and frames give the same lines; another seed may differ but stays valid.
func TestDeterminism(t *testing.T) {
	m := fixture()
	frame := func(tk int64) Frame {
		f := evening(tk, Cell{14, 12}, loaderSeated, ownerOnPost)
		f.Facts = []Fact{{Event: "gate.locked", Subject: "gate.entry"}}
		return f
	}
	a, b := State{}, State{}
	x := run(m, &a, 1, 3000, frame)
	y := run(m, &b, 1, 3000, frame)
	if !reflect.DeepEqual(x, y) || !reflect.DeepEqual(a, b) {
		t.Fatal("the same seed gave different lines")
	}
}

// World facts: a line about the gate plays only while the gate is locked; a stub (no crane state in Go) never plays.
func TestFactsAndStubs(t *testing.T) {
	m := fixture()
	porter := Actor{ID: "char.porter", Level: "hub", At: Cell{35, 3}}
	night := func(locked bool) func(int64) Frame {
		return func(tk int64) Frame {
			f := Frame{Now: 23*3600 + tk, Tick: tk, Seed: 3, Hero: Actor{ID: "char.hero", Level: "hub", At: Cell{36, 5}}, Actors: []Actor{porter}}
			if locked {
				f.Facts = []Fact{{Event: "gate.locked", Subject: "gate.entry", Level: "hub"}, {Event: "crane.stopped", Level: "hub"}}
			}
			return f
		}
	}
	s := State{}
	if got := run(m, &s, 1, 400, night(false)); len(got) != 0 {
		t.Fatalf("spoke of a locked gate while it is open: %+v", got)
	}
	s = State{}
	got := run(m, &s, 1, 400, night(true))
	if len(got) == 0 {
		t.Fatal("silent at a locked gate")
	}
	for _, sp := range got {
		if sp.Line == "bark.gate.crane" {
			t.Fatal("a stub event fired")
		}
	}
	// a speaker fact is his own: an order of another master does not make this one speak
	s = State{}
	trader := Actor{ID: "char.loader", Level: "hub", At: Cell{50, 50}}
	other := func(tk int64) Frame {
		return Frame{Now: 12*3600 + tk, Tick: tk, Seed: 3, Hero: Actor{ID: "char.hero", Level: "hub", At: Cell{50, 52}}, Actors: []Actor{trader}, Facts: []Fact{{Event: "craft.order", Subject: "char.owner", Since: 12 * 3600}}}
	}
	for _, sp := range run(m, &s, 1, 300, other) {
		if sp.Line == "bark.hall.order" {
			t.Fatal("a master spoke of another master's order")
		}
	}
	own := func(tk int64) Frame { f := other(tk); f.Facts[0].Subject = "char.loader"; return f }
	found := false
	for _, sp := range run(m, &s, 301, 900, own) {
		found = found || sp.Line == "bark.hall.order"
	}
	if !found {
		t.Fatal("a master with an order never spoke of it")
	}
}

// A passer-by: only when the hero pushes past within two cells, never when he stands still next to him.
func TestPassingBy(t *testing.T) {
	m := fixture()
	porter := Actor{ID: "char.porter", Level: "hub", At: Cell{20, 20}}
	still := func(tk int64) Frame {
		return Frame{Now: 10*3600 + tk, Tick: tk, Seed: 5, Hero: Actor{ID: "char.hero", Level: "hub", At: Cell{21, 21}}, Actors: []Actor{porter}}
	}
	s := State{}
	if got := run(m, &s, 1, 300, still); len(got) != 0 {
		t.Fatalf("a still hero was told off: %+v", got)
	}
	walking := func(tk int64) Frame { f := still(tk); f.Hero.At = Cell{16 + int(tk%8), 21}; return f }
	s = State{}
	got := run(m, &s, 1, 300, walking)
	if len(got) == 0 || got[0].Line != "bark.hall.push" || got[0].To != "hall" {
		t.Fatalf("the passer-by said nothing: %+v", got)
	}
}

// Combat: the shout follows the declared intent and the caste's face; a lie is allowed only to a leader, and seen only
// when the hero's skill + roll reaches a tell; beasts have no words.
func TestCombatCues(t *testing.T) {
	m := fixture()
	hero := Sense{Actor: Actor{ID: "char.hero", Level: "hub", At: Cell{0, 0}}, Skills: map[string]int{}}
	s := State{}
	out := m.BarkEvent(&s, 1000, 10, 9, Cue{Speaker: "char.guard", Declared: "spotted", Level: "hub", At: Cell{5, 5}}, hero)
	if out.Said == nil || out.Said.Line != "bark.guard.spotted" || out.Said.Caste != "bronze" {
		t.Fatalf("guard did not shout by his voice: %+v", out)
	}
	// a plain outlaw cannot lie: declared wounded_retreat but truly luring -> he shouts the truth (no line for lure)
	s = State{}
	out = m.BarkEvent(&s, 1000, 10, 9, Cue{Speaker: "char.o1", Archetype: "arch.outlaw", Declared: "wounded_retreat", Truth: "lure", Level: "hub", At: Cell{5, 5}}, hero)
	if out.Deceived || out.Said != nil {
		t.Fatalf("a rank-and-file outlaw lied: %+v", out)
	}
	// the leader lies; an unskilled hero does not see it
	s = State{}
	out = m.BarkEvent(&s, 1000, 10, 9, Cue{Speaker: "char.o2", Archetype: "arch.outlaw", Role: "leader", Declared: "wounded_retreat", Truth: "lure", Level: "hub", At: Cell{5, 5}}, hero)
	if !out.Deceived || out.Said == nil || out.Said.Line != "bark.bandit.wounded" || out.Thought != nil {
		t.Fatalf("leader's lie or blind hero wrong: %+v", out)
	}
	// a medic sees the dry wound: the thought is the hero's, never over the enemy
	s = State{}
	medic := hero
	medic.Skills = map[string]int{"medicine": 60}
	out = m.BarkEvent(&s, 1000, 10, 9, Cue{Speaker: "char.o2", Archetype: "arch.outlaw", Role: "leader", Declared: "wounded_retreat", Truth: "lure", Level: "hub", At: Cell{5, 5}}, medic)
	if out.Thought == nil || out.Thought.Speaker != "char.hero" || out.Thought.Kind != "thought" || out.Thought.Key != "bark.tell.no_blood" {
		t.Fatalf("a skilled hero did not see through the lie: %+v", out)
	}
	// an honest wounded retreat never gives a thought, however skilled the hero
	s = State{}
	out = m.BarkEvent(&s, 1000, 10, 9, Cue{Speaker: "char.o3", Archetype: "arch.outlaw", Role: "leader", Declared: "wounded_retreat", Level: "hub", At: Cell{5, 5}}, medic)
	if out.Deceived || out.Thought != nil || out.Said == nil {
		t.Fatalf("an honest retreat was flagged as a lie: %+v", out)
	}
	// wolves: a sound and a sign, no words
	s = State{}
	out = m.BarkEvent(&s, 1000, 10, 9, Cue{Speaker: "wolf.1", Archetype: "arch.wolf", Declared: "surround", Level: "hub", At: Cell{5, 5}}, hero)
	if out.Said == nil || out.Said.Kind != "sign" || out.Said.Key != "" || out.Said.Sign != "howl" || out.Said.Sfx != "sfx.pack.howl" {
		t.Fatalf("a beast spoke or made no sign: %+v", out)
	}
	// no voice, out of hearing: silent
	s = State{}
	if out = m.BarkEvent(&s, 1000, 10, 9, Cue{Speaker: "char.x", Declared: "spotted", Level: "hub", At: Cell{5, 5}}, hero); out.Said != nil {
		t.Fatal("a speaker without a voice shouted")
	}
	if out = m.BarkEvent(&s, 1000, 10, 9, Cue{Speaker: "char.guard", Declared: "spotted", Level: "hub", At: Cell{50, 50}}, hero); out.Said != nil {
		t.Fatal("heard a shout across the hall")
	}
}

// The screen never holds more than MaxActive audible lines.
func TestMaxActive(t *testing.T) {
	m := fixture()
	m.MaxActive = 1
	s := State{}
	porter := Actor{ID: "char.porter", Level: "hub", At: Cell{35, 3}}
	frame := func(tk int64) Frame {
		f := evening(tk, Cell{14, 12}, loaderSeated, ownerOnPost, porter)
		f.Now = 23*3600 + tk
		f.Facts = []Fact{{Event: "gate.locked", Subject: "gate.entry", Level: "hub"}}
		return f
	}
	for tk := int64(1); tk < 600; tk++ {
		m.Step(&s, frame(tk))
		if n := s.audible(tk); n > 1 {
			t.Fatalf("tick %d: %d lines at once", tk, n)
		}
	}
}

// An anonymous person of the population speaks the lines of "anyone" of his profile's caste, never a named line.
func TestPopulationSpeaksByCaste(t *testing.T) {
	m := fixture()
	walker := Actor{ID: "pop.17", Level: "hub", At: Cell{20, 20}, Caste: "copper"}
	walking := func(tk int64) Frame {
		return Frame{Now: 10*3600 + tk, Tick: tk, Seed: 5, Hero: Actor{ID: "char.hero", Level: "hub", At: Cell{16 + int(tk%8), 21}}, Actors: []Actor{walker}}
	}
	s := State{}
	got := run(m, &s, 1, 300, walking)
	if len(got) == 0 || got[0].Speaker != "pop.17" || got[0].Line != "bark.hall.push" {
		t.Fatalf("the copper passer-by of the population said nothing: %+v", got)
	}
	walker.Caste = "bronze"
	s = State{}
	for _, sp := range run(m, &s, 1, 300, walking) {
		if sp.Line == "bark.hall.push" {
			t.Fatal("a bronze passer-by spoke a copper line")
		}
	}
}

// A react pinned to a subject: gate 6's lock, wherever the line is spoken.
func TestReactSubject(t *testing.T) {
	m := fixture()
	m.Packs[1].Lines[0].React[0].Subject = "gate.passage"
	porter := Actor{ID: "char.porter", Level: "hub", At: Cell{35, 3}}
	frame := func(subject string) func(int64) Frame {
		return func(tk int64) Frame {
			return Frame{Now: 23*3600 + tk, Tick: tk, Seed: 3, Hero: Actor{ID: "char.hero", Level: "hub", At: Cell{36, 5}}, Actors: []Actor{porter}, Facts: []Fact{{Event: "gate.locked", Subject: subject, Level: "hub"}}}
		}
	}
	s := State{}
	for _, sp := range run(m, &s, 1, 400, frame("gate.entry")) {
		if sp.Line == "bark.gate.shut" {
			t.Fatal("a line pinned to gate 6 spoke of gate 7")
		}
	}
	s = State{}
	found := false
	for _, sp := range run(m, &s, 1, 400, frame("gate.passage")) {
		found = found || sp.Line == "bark.gate.shut"
	}
	if !found {
		t.Fatal("the pinned line never spoke of its own gate")
	}
}

// Exchanges (demo-hub v26-alive): the answer comes only after the first line is over, only while both are in place and heard;
// nobody in the middle of an exchange says anything else; the clock survives a save; the same seed gives the same talk.
func talkFixture() Model {
	m := fixture()
	m.Talk = TalkRules{GapSeconds: 60, MaxPerHour: 6, MeanWaitSeconds: 1, PauseMS: 300, GiveUpMS: 8000}
	m.Exchanges = []Exchange{{ID: "talk.loader_owner", A: Party{Who: "char.loader", Caste: "bronze", Posture: "post"}, B: Party{Who: "char.owner", Caste: "bronze", Posture: "post"},
		WithinCells: 6, HeroWithinCells: 12, Periods: []Period{{ID: "all", From: 0, To: 86400}}, Weight: 1, CooldownSeconds: 3600,
		Turns: []Turn{{ID: "bark.talk.one", By: 0, Gesture: "argue", Measure: ms("Что там с накладной?")}, {ID: "bark.talk.two", By: 1, Measure: ms("Завтра подпишут, не шуми.")}, {ID: "bark.talk.three", By: 0, Measure: ms("Завтра ты и вчера говорил.")}}}}
	return m.Sorted()
}
func talkFrame(now, tick int64) Frame {
	return Frame{Now: now, Tick: tick, Seed: 7, Hero: Actor{ID: "char.hero", Level: "hub", At: Cell{50, 50}}, Actors: []Actor{
		{ID: "char.loader", Level: "hub", At: Cell{50, 44}, OnPost: true}, {ID: "char.owner", Level: "hub", At: Cell{54, 44}, OnPost: true}}}
}
func talkRun(t *testing.T, m Model, s *State, mutate func(f *Frame, tick int64)) []Speech {
	t.Helper()
	var all []Speech
	for tick := int64(1); tick < 400; tick++ {
		f := talkFrame(1000+tick/10, tick)
		if mutate != nil {
			mutate(&f, tick)
		}
		for _, sp := range m.Step(s, f) {
			if sp.Kind == "talk" {
				all = append(all, sp)
			}
		}
	}
	return all
}
func TestExchangeRunsInOrderAfterTheLineIsOver(t *testing.T) {
	m := talkFixture()
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	var s State
	got := talkRun(t, m, &s, nil)
	if len(got) != 3 {
		t.Fatalf("turns %+v", got)
	}
	if got[0].Speaker != "char.loader" || got[0].To != "char.owner" || got[1].Speaker != "char.owner" || got[1].To != "char.loader" || got[2].Speaker != "char.loader" {
		t.Fatalf("who speaks to whom: %+v", got)
	}
	if got[1].Start < got[0].Until || got[2].Start < got[1].Until {
		t.Fatalf("an answer started before the line was over: %+v", got)
	}
	if got[0].Gesture != "argue" || got[1].Gesture != "" {
		t.Fatalf("gestures %+v", got)
	}
	if len(s.Talks) != 0 {
		t.Fatalf("the run stays open: %+v", s.Talks)
	}
	if err := m.ValidateState(s, 2000, 500); err != nil {
		t.Fatal(err)
	}
}
func TestExchangeStopsWhenOneLeavesOrTheHeroIsFar(t *testing.T) {
	m := talkFixture()
	var s State
	first := int64(-1)
	got := talkRun(t, m, &s, func(f *Frame, tick int64) {
		if first > 0 && tick > first {
			f.Actors[1].Moving = true
		}
		if first < 0 && len(s.Active) > 0 {
			first = tick
		}
	})
	if len(got) != 1 {
		t.Fatalf("the answer of a man who left: %+v", got)
	}
	s = State{}
	first = -1
	got = talkRun(t, m, &s, func(f *Frame, tick int64) {
		if first > 0 && tick > first {
			f.Hero.At = Cell{90, 90}
		}
		if first < 0 && len(s.Active) > 0 {
			first = tick
		}
	})
	if len(got) != 1 {
		t.Fatalf("the answer the hero cannot hear: %+v", got)
	}
}
func TestExchangeAnswerFromAnyoneNearest(t *testing.T) {
	m := talkFixture()
	m.Exchanges[0].B = Party{Who: Anyone, Caste: "copper"}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	var s State
	got := talkRun(t, m, &s, func(f *Frame, _ int64) {
		f.Actors = append(f.Actors, Actor{ID: "population.0002", Level: "hub", At: Cell{50, 49}, Caste: "copper"}, Actor{ID: "population.0001", Level: "hub", At: Cell{50, 47}, Caste: "copper"}, Actor{ID: "population.0003", Level: "hub", At: Cell{50, 46}, Caste: "bronze"})
	})
	if len(got) != 3 || got[0].To != "population.0001" || got[1].Speaker != "population.0001" || got[1].Caste != "copper" {
		t.Fatalf("the nearest of the caste answers: %+v", got)
	}
}
func TestExchangeSavedMidwayAndDeterministic(t *testing.T) {
	m := talkFixture()
	var s State
	for tick := int64(1); tick < 12; tick++ {
		m.Step(&s, talkFrame(1000+tick/10, tick))
	}
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var loaded State
	if err := json.Unmarshal(raw, &loaded); err != nil {
		t.Fatal(err)
	}
	a, b := s.Clone(), loaded
	for tick := int64(12); tick < 400; tick++ {
		x, y := m.Step(&a, talkFrame(1000+tick/10, tick)), m.Step(&b, talkFrame(1000+tick/10, tick))
		if !reflect.DeepEqual(x, y) {
			t.Fatalf("tick %d: %+v vs %+v", tick, x, y)
		}
	}
	if !reflect.DeepEqual(a.Talk, b.Talk) || a.Serial != b.Serial {
		t.Fatal("the talk clock differs after a save")
	}
}
func TestExchangeInvalidCatalogRejected(t *testing.T) {
	m := talkFixture()
	m.Exchanges[0].Turns[1].By = 0
	if m.Validate() == nil {
		t.Fatal("a side answering itself")
	}
	m = talkFixture()
	m.Exchanges[0].Turns = m.Exchanges[0].Turns[:1]
	if m.Validate() == nil {
		t.Fatal("one turn is no exchange")
	}
}
