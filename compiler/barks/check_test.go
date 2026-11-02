package barks

import (
	"strings"
	"testing"

	runtime "github.com/strogmv/ang/compiler/barks/runtime"
)

func measure(ru, en string) map[string]runtime.Measure {
	return map[string]runtime.Measure{"ru": runtime.MeasureText(ru), "en": runtime.MeasureText(en)}
}

// A small valid world: a yatai with one regular and the owner, a gate, the combat voices; texts and fonts in memory.
func fixture() (runtime.Model, map[string]map[string]string, World) {
	faces := []runtime.Face{
		{Caste: "copper", Face: "chalk", Reveal: "written", Correction: "cross_out", CPS: 18, Colour: "#c8b183", Speaks: true, WordsMax: 8},
		{Caste: "bronze", Face: "stencil", Reveal: "stamped", Correction: "restamp", CPS: 26, Capitals: true, Punctuation: true, Colour: "#f0d9a8", Speaks: true, WordsMax: 8},
		{Caste: "silver", Face: "matrix", Reveal: "set", Correction: "reprint", CPS: 30, Capitals: true, Punctuation: true, Colour: "#cfe6ff", Speaks: true, WordsMax: 10},
		{Caste: "silver_demoted", Face: "matrix", Reveal: "set", Correction: "reprint", CPS: 30, Capitals: true, Punctuation: true, Colour: "#cfe6ff", Speaks: true, WordsMax: 10},
		{Caste: "gold", Face: "matrix", Reveal: "set", Correction: "none", CPS: 30, Capitals: true, Punctuation: true, Colour: "#f2e6c4", WordsMax: 10, Undecided: true},
		{Caste: "platinum", Face: "matrix", Reveal: "set", Correction: "none", CPS: 30, Capitals: true, Punctuation: true, Colour: "#e8dcff", WordsMax: 10, Undecided: true},
		{Caste: "freeman", Face: "chalk", Reveal: "written", Correction: "cross_out", CPS: 22, Capitals: true, Punctuation: true, Colour: "#f4efe2", Speaks: true, WordsMax: 8},
		{Caste: "none", Face: "chalk", Reveal: "written", Correction: "cross_out", CPS: 22, Capitals: true, Punctuation: true, Colour: "#d8d2c4", Speaks: true, WordsMax: 8},
	}
	texts := map[string]map[string]string{
		"bark.yatai.top_up":       {"ru": "Эй, тряпка, долей мне!", "en": "Hey, rag, top me up!"},
		"bark.yatai.reply_coming": {"ru": "Сейчас.", "en": "Coming."},
		"bark.yatai.knee":         {"ru": "опять смена в ночь колено ноет", "en": "night shift again my knee aches"},
		"bark.yatai.knee.fix":     {"ru": "спина", "en": "back"},
		"bark.guard.spotted":      {"ru": "Стоять! Проверка!", "en": "Halt! Check!"},
		"bark.tell.no_blood":      {"ru": "Рана не кровит.", "en": "The wound isn't bleeding."},
	}
	m := runtime.Model{
		TickHz: 10, MaxActive: 3, MovingTicks: 20, Locales: []string{"ru", "en"},
		Timing: runtime.Timing{CommaMS: 90, StopMS: 170, FixPauseMS: 220, StrikeMS: 180, SetMS: 120, StampMinMS: 140, HoldMinMS: 3200, HoldBaseMS: 900, WordsPerMinute: 156, FadeMS: 600, OffscreenFadeMS: 300, GestureMS: 1600},
		Layout: runtime.Layout{EMPx: 18, MaxWidthEM: 17, MaxLines: 2, LineLimitRunes: 52},
		Faces:  faces,
		Castes: map[string]string{"char.loader": "bronze", "char.owner": "bronze", "char.porter": "copper", "char.hero": "none", "char.inspector": "silver", "char.gold": "gold"},
		Events: []runtime.Event{{ID: "gate.locked", Source: "go", Scope: "place"}, {ID: "crane.stopped", Source: "stub", Scope: "world"}},
		Places: []runtime.Place{{ID: "food.yatai", Kind: "seated_stall", Level: "hub", Spots: []runtime.Cell{{X: 1, Y: 1}, {X: 2, Y: 1}}}},
		Packs: []runtime.Pack{{Place: "food.yatai", Periods: []runtime.Period{{ID: "evening", From: 64800, To: 81000}}, HeroWithinCells: 10, GapSeconds: 300, MaxPerHour: 4, MeanWaitSeconds: 60,
			Lines: []runtime.Line{
				{ID: "bark.yatai.top_up", Speaker: "char.loader", Caste: "bronze", To: "char.owner", When: []string{"evening"}, Requires: runtime.Requires{TargetOnPost: true}, Weight: 3, CooldownSeconds: 1800, Reply: "reply.yatai.coming", Measure: measure("Эй, тряпка, долей мне!", "Hey, rag, top me up!")},
				{ID: "bark.yatai.knee", Speaker: "char.porter", Caste: "copper", To: "crew", When: []string{"evening"}, Weight: 1, CooldownSeconds: 1800, Measure: measure("опять смена в ночь колено ноет", "night shift again my knee aches"),
					Fix: map[string]runtime.Fix{"ru": {From: 19, To: 25, Key: "bark.yatai.knee.fix", Measure: runtime.MeasureText("спина")}, "en": {From: 20, To: 24, Key: "bark.yatai.knee.fix", Measure: runtime.MeasureText("back")}}},
			},
			Replies: []runtime.Reply{{ID: "reply.yatai.coming", Speaker: "char.owner", Caste: "bronze", Kind: "line", Key: "bark.yatai.reply_coming", DelaySeconds: 30, Measure: measure("Сейчас.", "Coming.")}},
		}},
		Combat: runtime.Combat{HeroWithinCells: 24, Intents: []string{"spotted", "wounded_retreat", "lure", "surround"}, RollMax: 20, CunningRoles: []string{"leader"},
			Voices:     []runtime.Voice{{ID: "guard", Caste: "bronze"}, {ID: "pack", Beast: true}},
			Archetypes: map[string]string{"arch.wolf": "pack"},
			Lines: []runtime.CombatLine{
				{ID: "bark.guard.spotted", Voice: "guard", Intent: "spotted", Weight: 1, CooldownSeconds: 30, Measure: measure("Стоять! Проверка!", "Halt! Check!")},
				{ID: "bark.pack.surround", Voice: "pack", Intent: "surround", Weight: 1, CooldownSeconds: 30, Sign: "howl", Sfx: "sfx.pack.howl"},
			},
			Deceptions: []runtime.Deception{{Declared: "wounded_retreat", Truth: "lure", Tells: []runtime.Tell{{Key: "bark.tell.no_blood", Skill: "medicine", Threshold: 40, Measure: measure("Рана не кровит.", "The wound isn't bleeding.")}}}},
		},
	}
	adv := map[rune]float64{}
	for _, r := range " !\"'(),-.:;?«»—–ЁАБВГДЕЖЗИЙКЛМНОПРСТУФХЦЧШЩЪЫЬЭЮЯабвгдежзийклмнопрстуфхцчшщъыьэюяёABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789" {
		adv[r] = 0.5
	}
	font := Font{Advance: adv, Tracking: 0.02}
	w := World{
		Texts:    func(key string) map[string]string { return texts[key] },
		Fonts:    map[string]Font{"chalk": font, "stencil": font, "matrix": font},
		Fillers:  []string{"привет", "hello"},
		Walkable: func(level string, at runtime.Cell) bool { return level == "hub" },
		Hidden: []Hidden{{Person: "char.owner",
			Secret: []string{"Тайный глава преступного мира бронзы: держит серый обмен рынка и долги зала."},
			Public: []string{"Списан после аварии котла; держит ятай для души, кормит в долг."}}},
	}
	return m.Sorted(), texts, w
}

func TestCheckAcceptsTheFixture(t *testing.T) {
	m, _, w := fixture()
	if err := Check(m, w); err != nil {
		t.Fatal(err)
	}
}

// Every rule has a negative case: the broken world must fail with a message naming the rule.
func TestCheckRejects(t *testing.T) {
	cases := []struct {
		name string
		want string
		edit func(m *runtime.Model, texts map[string]map[string]string, w *World)
	}{
		{"silent caste", "silent in act one", func(m *runtime.Model, _ map[string]map[string]string, _ *World) {
			m.Packs[0].Lines[0].Speaker, m.Packs[0].Lines[0].Caste = "char.gold", "gold"
		}},
		{"too many words", "words", func(_ *runtime.Model, t map[string]map[string]string, _ *World) {
			t["bark.yatai.top_up"]["ru"] = "Эй, тряпка, долей мне чаю ещё одну кружку, слышишь меня?"
		}},
		{"filler", "empty greeting", func(_ *runtime.Model, t map[string]map[string]string, _ *World) {
			t["bark.yatai.top_up"]["ru"] = "Привет, тряпка, долей мне!"
		}},
		{"copper capitals", "capitals", func(_ *runtime.Model, t map[string]map[string]string, _ *World) {
			t["bark.yatai.knee"]["ru"] = "Опять смена в ночь колено ноет"
		}},
		{"copper punctuation", "almost without", func(_ *runtime.Model, t map[string]map[string]string, _ *World) {
			t["bark.yatai.knee"]["ru"] = "опять смена. в ночь, колено ноет."
		}},
		{"missing translation", "has no en text", func(_ *runtime.Model, t map[string]map[string]string, _ *World) {
			delete(t["bark.yatai.reply_coming"], "en")
		}},
		{"glyph missing", "does not fit", func(_ *runtime.Model, t map[string]map[string]string, _ *World) {
			t["bark.yatai.top_up"]["en"] = "Hey, rag, top me up ☕!"
		}},
		{"too wide", "lines of", func(m *runtime.Model, _ map[string]map[string]string, _ *World) { m.Layout.MaxWidthEM = 3 }},
		{"spoiler", "touches the secret", func(_ *runtime.Model, t map[string]map[string]string, _ *World) {
			t["bark.yatai.top_up"]["ru"] = "Эй, преступник, долей мне!"
		}},
		{"silver corrects", "does not correct in place", func(m *runtime.Model, t map[string]map[string]string, _ *World) {
			m.Castes["char.porter"] = "silver"
			m.Packs[0].Lines[1].Caste = "silver"
			t["bark.yatai.knee"]["ru"] = "Опять смена в ночь, колено ноет."
			t["bark.yatai.knee"]["en"] = "Night shift again, my knee aches."
		}},
		{"fix out of range", "outside the text", func(m *runtime.Model, _ map[string]map[string]string, _ *World) {
			m.Packs[0].Lines[1].Fix["ru"] = runtime.Fix{From: 20, To: 90, Key: "bark.yatai.knee.fix"}
		}},
		{"caste mismatch", "differs from the speaker", func(m *runtime.Model, _ map[string]map[string]string, _ *World) {
			m.Packs[0].Lines[0].Caste = "copper"
		}},
		{"unknown speaker", "unknown speaker", func(m *runtime.Model, _ map[string]map[string]string, _ *World) {
			m.Packs[0].Lines[0].Speaker = "char.nobody"
		}},
		{"unused reply", "nobody answers", func(m *runtime.Model, _ map[string]map[string]string, _ *World) {
			m.Packs[0].Lines[0].Reply = ""
		}},
		{"nowhere to stand", "no walkable cell", func(_ *runtime.Model, _ map[string]map[string]string, w *World) {
			w.Walkable = func(string, runtime.Cell) bool { return false }
		}},
		{"beast speaks", "never words", func(_ *runtime.Model, t map[string]map[string]string, _ *World) {
			t["bark.pack.surround"] = map[string]string{"ru": "Ррр", "en": "Grr"}
		}},
		{"long shout", "words", func(_ *runtime.Model, t map[string]map[string]string, _ *World) {
			t["bark.guard.spotted"]["ru"] = "Стоять на месте, это проверка чипов купола!"
		}},
		{"nobody lies", "nobody may lie", func(m *runtime.Model, _ map[string]map[string]string, _ *World) { m.Combat.CunningRoles = nil }},
		{"target needs a person", "needs a known person", func(m *runtime.Model, _ map[string]map[string]string, _ *World) {
			m.Packs[0].Lines[0].To = "hall"
		}},
		{"undecided speaks", "undecided", func(m *runtime.Model, _ map[string]map[string]string, _ *World) {
			for i := range m.Faces {
				if m.Faces[i].Caste == "gold" {
					m.Faces[i].Speaks = true
				}
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, texts, w := fixture()
			c.edit(&m, texts, &w)
			err := Check(m, w)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want %q, got %v", c.want, err)
			}
		})
	}
}

// The no-spoiler rule is derived from the secrets: a word of the secret that the cover also uses is not hidden.
func TestHiddenStems(t *testing.T) {
	h := HiddenStems([]Hidden{{Person: "p", Secret: []string{"Курьер ночного поезда: груз мимо учёта."}, Public: []string{"Грузчик ночного поезда"}}})
	if _, ok := h["курье"]; !ok {
		t.Fatal("the secret word is not hidden")
	}
	if _, ok := h["ночно"]; ok {
		t.Fatal("a cover word became hidden")
	}
}

func TestRenderAndReport(t *testing.T) {
	m, texts, _ := fixture()
	files, err := Render(m)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"catalog.json", "catalog_data.gen.go", "engine.gen.go", "model.gen.go", "state.gen.go", "shouts.gen.go", "runtime_generated_test.go"} {
		if len(files[name]) == 0 {
			t.Fatalf("missing %s", name)
		}
	}
	report := string(Report(m, func(k string) map[string]string { return texts[k] }, func(string) string { return "tone" }, func(id string) string { return id }, []string{"WARN x"}))
	for _, want := range []string{"food.yatai", "Эй, тряпка", "stencil/stamped", "Бой", "wounded_retreat", "WARN x"} {
		if !strings.Contains(report, want) {
			t.Fatalf("report lacks %q", want)
		}
	}
}

// A word the world already says openly (the corpus) is not a secret word; the secret's own vocabulary stays hidden.
func TestHiddenStemsCorpus(t *testing.T) {
	h := HiddenStems([]Hidden{{Person: "p", Secret: []string{"brings the rumours before anyone"}}}, "He stood there before the gate.")
	if _, ok := h["befor"]; ok {
		t.Fatal("a common word of the world became secret")
	}
	if _, ok := h["rumou"]; !ok {
		t.Fatal("the secret's own word is not hidden")
	}
}

// A react pinned to a subject fires only for that subject; a target named apart from `to` is checked.
func TestCheckTargetAndSubject(t *testing.T) {
	m, _, w := fixture()
	m.Packs[0].Lines[0].To = "hall"
	m.Packs[0].Lines[0].Requires.Target = "char.owner"
	m.Packs[0].Lines[1].React = []runtime.React{{Event: "gate.locked", Subject: "gate.passage", WithinSeconds: 3600}}
	if err := Check(m, w); err != nil {
		t.Fatal(err)
	}
}
