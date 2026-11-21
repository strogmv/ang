package gamepoc

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

func validHeroFile() HeroFile {
	var h HeroFile
	h.Skills = []string{"observation", "speech", "medicine"}
	h.Profiles = []HeroFileProfile{
		{ID: "technician", Title: "t", Summary: "s", Icon: "eye", Canon: true, Skills: map[string]int{"observation": 45, "speech": 20, "medicine": 10}},
		{ID: "negotiator", Title: "t", Summary: "s", Icon: "speech", Skills: map[string]int{"observation": 40, "speech": 30, "medicine": 5}},
	}
	h.Strengths = HeroFileStrengths{Count: 2, Bonus: 15, StartCap: 55, LearnPermille: 1500}
	h.Episodes = []HeroFileEpisode{
		{ID: "receiving", Title: "t", Plus: "p", Minus: "m", First: "f", Topic: "topic.cassette_norm", Fact: "fact.file.receiving"},
		{ID: "medblock", Title: "t", Plus: "p", Minus: "m", First: "f", Topic: "topic.medblock_inside", Fact: "fact.file.medblock"},
		{ID: "schedules", Title: "t", Plus: "p", Minus: "m", First: "f", Topic: "topic.ep_schedules", Fact: "fact.file.schedules"},
	}
	h.EpisodesPick = 2
	h.Club, h.NoTrait = "club", "none"
	add := HeroFileEffect{Skill: &struct {
		ID  string `json:"id"`
		Add int    `json:"add"`
	}{ID: "speech", Add: 5}}
	h.Traits = []HeroFileTrait{{ID: "stubborn", Title: "t", Plus: "p", Minus: "m", First: "f", Effects: []HeroFileEffect{add, {WoundAPRelief: 1}}}}
	h.Portraits = []HeroFilePortrait{{ID: "portrait_1"}}
	h.Default = HeroFileChoice{Profile: "technician", Strengths: []string{"observation", "medicine"}, Episodes: []string{"receiving", "medblock"}, Portrait: "portrait_1"}
	h.StillSeconds = 20
	h.Line.SureLevel, h.Line.SurePercent, h.Line.HopePercent, h.Line.Shown = "hard", 80, 20, 2
	h.Usage = HeroFileUsage{ActOne: []string{"01", "18"}, FirstHour: []string{"18"}, MinAct: 2, MinFirstHour: 1,
		Reads: map[string]map[string]int{"01": {"topic.cassette_norm": 1, "fact.file.receiving": 1, "topic.medblock_inside": 2}, "18": {"fact.file.medblock": 1, "topic.ep_schedules": 1, "fact.file.schedules": 1}}}
	h.Recheck = HeroFileRecheck{Level: "level.dorm", NightFromH: 21, NightToH: 6, Title: "t", Body: "b", Do: "d", Confirm: "c", Skip: "s", Done: "x"}
	return h
}

func TestHeroFileDataIsConsistent(t *testing.T) {
	if err := validHeroFile().validate(); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*HeroFile){
		"profiles of unequal sums": func(h *HeroFile) { h.Profiles[1].Skills["speech"]++ },
		"two canons":               func(h *HeroFile) { h.Profiles[1].Canon = true },
		"no canon":                 func(h *HeroFile) { h.Profiles[0].Canon = false },
		"a profile without a skill": func(h *HeroFile) {
			delete(h.Profiles[1].Skills, "medicine")
			h.Profiles[1].Skills["speech"] += 5
		},
		"a profile value above the cap": func(h *HeroFile) { h.Profiles[0].Skills["observation"], h.Profiles[0].Skills["speech"] = 60, 5 },
		"a default of one strength":     func(h *HeroFile) { h.Default.Strengths = h.Default.Strengths[:1] },
		"a default of a foreign skill":  func(h *HeroFile) { h.Default.Strengths = []string{"observation", "astral"} },
		"a default of an unknown trait": func(h *HeroFile) { h.Default.Trait = "lucky" },
		"two episodes one topic":        func(h *HeroFile) { h.Episodes[1].Topic = h.Episodes[0].Topic },
		"an effect of two kinds": func(h *HeroFile) {
			h.Traits[0].Effects[1].StepLoudnessPermille = 800
		},
		"a trait's skill not of the file": func(h *HeroFile) { h.Traits[0].Effects[0].Skill.ID = "astral" },
		"a first hour outside the act":    func(h *HeroFile) { h.Usage.FirstHour = []string{"19"} },
		"no learning boost":               func(h *HeroFile) { h.Strengths.LearnPermille = 1000 },
	} {
		h := validHeroFile()
		h.Profiles = append([]HeroFileProfile(nil), h.Profiles...)
		for i := range h.Profiles {
			skills := map[string]int{}
			for k, v := range h.Profiles[i].Skills {
				skills[k] = v
			}
			h.Profiles[i].Skills = skills
		}
		mutate(&h)
		if h.validate() == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}

// The hero of the book: the profile, the strengths' bonus within the start cap, the trait's skill.
func TestHeroFileStartOfAChoice(t *testing.T) {
	h := validHeroFile()
	got := h.start(h.Default)
	if got["observation"] != 55 || got["medicine"] != 25 || got["speech"] != 20 {
		t.Fatalf("default start %v", got)
	}
	c := h.Default
	c.Trait = "stubborn"
	if h.start(c)["speech"] != 25 {
		t.Fatal("the trait's skill not added")
	}
}

// The gate: a skill offered on the screen with fewer uses than the act or the first hour needs fails the build; an
// act-one script without counts or a count of an unknown skill fails too.
func TestHeroFileUsageGateFailsTheBuild(t *testing.T) {
	names := map[string]string{"observation": "Наблюдательность", "speech": "Речь", "medicine": "Медицина"}
	type n = struct {
		Checks int `json:"checks"`
		Voices int `json:"voices"`
	}
	counts := func() map[string]map[string]n {
		return map[string]map[string]n{
			"01": {"observation": {Checks: 1}, "speech": {Checks: 1}, "medicine": {Checks: 1}},
			"18": {"observation": {Voices: 1}, "speech": {Voices: 1}, "medicine": {Voices: 1}},
		}
	}
	h := validHeroFile()
	h.Usage.Counts = counts()
	if err := h.checkUsage(names); err != nil {
		t.Fatal(err)
	}
	if h.uses["observation"].Checks != 1 || h.uses["observation"].Voices != 1 || h.uses["observation"].FirstHour != 1 || !strings.Contains(string(h.report), "| Медицина | 1 | 1 | 2 | 1 |") {
		t.Fatalf("uses %+v report %s", h.uses, h.report)
	}
	h = validHeroFile()
	h.Usage.Counts = counts()
	delete(h.Usage.Counts["18"], "medicine")
	if err := h.checkUsage(names); err == nil || !strings.Contains(err.Error(), "medicine: 1 in act one (min 2), 0 in the first hour") {
		t.Fatalf("a barely used skill passed: %v", err)
	}
	h = validHeroFile()
	h.Usage.Counts = counts()
	delete(h.Usage.Counts, "18")
	if err := h.checkUsage(names); err == nil || !strings.Contains(err.Error(), "script 18 of act one is not counted") {
		t.Fatalf("an uncounted script passed: %v", err)
	}
	h = validHeroFile()
	h.Usage.Counts = counts()
	h.Usage.Counts["01"]["sword"] = n{Checks: 3}
	if err := h.checkUsage(names); err == nil || !strings.Contains(err.Error(), "unknown skill sword") {
		t.Fatalf("an unknown skill passed: %v", err)
	}
}

// The CUE block decodes strictly, the usage gate is read and stays out of world.json.
func TestHeroFileReadsTheUsageGateOutsideWorldJSON(t *testing.T) {
	h := validHeroFile()
	raw, err := json.Marshal(h)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "act_one") {
		t.Fatal("the usage gate is in world.json")
	}
	var block map[string]any
	if err := json.Unmarshal(raw, &block); err != nil {
		t.Fatal(err)
	}
	block["usage"] = h.Usage
	compile := func() cue.Value {
		data, err := json.Marshal(map[string]any{"hero_file": block})
		if err != nil {
			t.Fatal(err)
		}
		return cuecontext.New().CompileBytes(data)
	}
	got, err := readHeroFile(compile())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Usage, h.Usage) || got.Default.Profile != h.Default.Profile {
		t.Fatalf("read %+v", got.Usage)
	}
	block["stray"] = 1
	if _, err := readHeroFile(compile()); err == nil {
		t.Fatal("an unknown field passed")
	}
}

// Doc 09 21.11: an effect acts on something in this world, or the build fails.
func TestHeroFileEffectsNeedAConsumer(t *testing.T) {
	h := validHeroFile()
	all := map[string]bool{"skill": true, "step_loudness": true, "visibility": true, "wound_ap_relief": true}
	if err := h.checkConsumers(all); err != nil {
		t.Fatal(err)
	}
	noCombat := map[string]bool{"skill": true, "step_loudness": true, "visibility": true}
	if err := h.checkConsumers(noCombat); err == nil || !strings.Contains(err.Error(), "trait stubborn effect 1: wound_ap_relief acts on nothing") {
		t.Fatalf("a wound relief without a fight passed: %v", err)
	}
	h.Episodes[0].Effects = []HeroFileEffect{{StepLoudnessPermille: 800}}
	if err := h.checkConsumers(map[string]bool{"skill": true, "wound_ap_relief": true}); err == nil || !strings.Contains(err.Error(), "episode receiving effect 0: step_loudness") {
		t.Fatalf("quieter steps without the stealth passed: %v", err)
	}
	// the kinds without a consumer anywhere yet are not in the data's language at all
	raw, _ := json.Marshal(HeroFileEffect{})
	if strings.Contains(string(raw), "need") || strings.Contains(string(raw), "speech") {
		t.Fatalf("an effect kind without a consumer: %s", raw)
	}
}

// Doc 104: a topic or recorded fact of the past that no act-one script reads fails the build.
func TestHeroFilePastNeedsAReader(t *testing.T) {
	h := validHeroFile()
	if err := h.checkReaders(); err != nil {
		t.Fatal(err)
	}
	delete(h.Usage.Reads["18"], "fact.file.schedules")
	if err := h.checkReaders(); err == nil || !strings.Contains(err.Error(), "fact.file.schedules") {
		t.Fatalf("an unread episode fact passed: %v", err)
	}
	h = validHeroFile()
	h.Usage.Reads["26"] = map[string]int{"fact.file.schedules": 3}
	delete(h.Usage.Reads["18"], "fact.file.schedules")
	if h.checkReaders() == nil {
		t.Fatal("a read outside act one counted")
	}
}

// The start counts the chosen episodes' skill effects like the runtime, and a negative effect stops at 0.
func TestHeroFileStartCountsEpisodeEffects(t *testing.T) {
	h := validHeroFile()
	h.Episodes[0].Effects = []HeroFileEffect{{Skill: &struct {
		ID  string `json:"id"`
		Add int    `json:"add"`
	}{ID: "medicine", Add: -30}}}
	c := h.Default
	c.Episodes = []string{"receiving", "medblock"}
	if got := h.start(c)["medicine"]; got != 0 {
		t.Fatalf("medicine %d, want 0 (10 + 15 − 30 stops at 0)", got)
	}
}
