package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"cuelang.org/go/cue"
)

// HeroFile is world.hero_file: the hero's personal file before the lineup (doc 104 «v2 после критики»,
// design/hero-creation/v1/BRIEF.md). A profile, two strengths, two past episodes, an optional trait and a portrait; the
// server builds the hero's start from the choice (templates/hero_file.go.tmpl) and words every number of the screen, the
// client only shows them. The default — «the hero of the book» — is the canon: the hero's start in world.persons is
// derived from it, and ANG checks that the Life profile the game starts from is exactly that derivation.
//
// The check of the brief: a skill is offered on the screen only if act one uses it at least min_act times and the first
// hour at least min_first_hour times. ANG combines authored CUE dialogue with the remaining Markdown counts written
// by the game's tools/hero-file/skill_uses.py (doc 104 «Решения при сборке»). A
// skill short of it fails the build; the counts go into design/hero-creation/v1/skill-uses.md.
type HeroFile struct {
	Skills       []string           `json:"skills"`
	Profiles     []HeroFileProfile  `json:"profiles"`
	Strengths    HeroFileStrengths  `json:"strengths"`
	Episodes     []HeroFileEpisode  `json:"episodes"`
	EpisodesPick int                `json:"episodes_pick"`
	Club         string             `json:"club"`
	Traits       []HeroFileTrait    `json:"traits"`
	NoTrait      string             `json:"no_trait"`
	Portraits    []HeroFilePortrait `json:"portraits"`
	Default      HeroFileChoice     `json:"default"`
	StillSeconds int                `json:"still_seconds"`
	// the fight line of the skill the hero's combat profile develops (part 1b): this weapon at this opponent profile
	Fight struct {
		Weapon   string `json:"weapon"`
		Opponent string `json:"opponent"`
		Text     string `json:"text"`
	} `json:"fight"`
	Line struct {
		SureLevel   string `json:"sure_level"`
		SurePercent int    `json:"sure_percent"`
		HopePercent int    `json:"hope_percent"`
		Shown       int    `json:"shown"`
	} `json:"line"`
	// The usage gate is the build's, not the server's: never in world.json.
	Usage   HeroFileUsage   `json:"-"`
	Recheck HeroFileRecheck `json:"recheck"`
	Text    HeroFileText    `json:"text"`

	// uses by skill, counted from the scripts at the build (the report design/hero-creation/v1/skill-uses.md)
	uses   map[string]heroFileUses
	report []byte
}
type HeroFileProfile struct {
	ID      string         `json:"id"`
	Title   string         `json:"title"`
	Summary string         `json:"summary"`
	Icon    string         `json:"icon"`
	Skills  map[string]int `json:"skills"`
	Canon   bool           `json:"canon"`
}
type HeroFileStrengths struct {
	Count         int `json:"count"`
	Bonus         int `json:"bonus"`
	StartCap      int `json:"start_cap"`
	LearnPermille int `json:"learn_permille"`
}
type HeroFileEpisode struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Plus         string `json:"plus"`
	Minus        string `json:"minus"`
	First        string `json:"first"`
	Topic        string `json:"topic"`
	TopicJournal string `json:"topic_journal,omitempty"`
	// the episode's recorded fact (one fact per episode: two are true at once) in the conversations' memory
	Fact    string           `json:"fact"`
	Effects []HeroFileEffect `json:"effects"`
}
type HeroFileTrait struct {
	ID      string           `json:"id"`
	Title   string           `json:"title"`
	Plus    string           `json:"plus"`
	Minus   string           `json:"minus"`
	First   string           `json:"first"`
	Effects []HeroFileEffect `json:"effects"`
}

// HeroFileEffect: exactly one of the fields (validate).
type HeroFileEffect struct {
	Skill *struct {
		ID  string `json:"id"`
		Add int    `json:"add"`
	} `json:"skill,omitempty"`
	StepLoudnessPermille int `json:"step_loudness_permille,omitempty"`
	VisibilityPermille   *struct {
		Permille int    `json:"permille"`
		While    string `json:"while"`
	} `json:"visibility_permille,omitempty"`
	WoundAPRelief int `json:"wound_ap_relief,omitempty"`
}
type HeroFilePortrait struct {
	ID    string `json:"id"`
	Image string `json:"image,omitempty"`
}
type HeroFileChoice struct {
	Profile   string   `json:"profile"`
	Strengths []string `json:"strengths"`
	Episodes  []string `json:"episodes"`
	Trait     string   `json:"trait"`
	Portrait  string   `json:"portrait"`
}
type HeroFileUsage struct {
	ActOne       []string `json:"act_one"`
	FirstHour    []string `json:"first_hour"`
	MinAct       int      `json:"min_act"`
	MinFirstHour int      `json:"min_first_hour"`
	// script number -> skill id -> its checks and inner voices there
	Counts map[string]map[string]struct {
		Checks int `json:"checks"`
		Voices int `json:"voices"`
	} `json:"counts"`
	// script number -> topic or file fact (with its argument) -> how many times the script reads it
	Reads    map[string]map[string]int      `json:"reads"`
	Dialogue map[string]heroFileUsageSource `json:"dialogue,omitempty"`
}
type HeroFileRecheck struct {
	Level      string `json:"level"`
	NightFromH int    `json:"night_from_h"`
	NightToH   int    `json:"night_to_h"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	Do         string `json:"do"`
	Confirm    string `json:"confirm"`
	Skip       string `json:"skip"`
	Done       string `json:"done"`
}
type HeroFileText struct {
	Header           string            `json:"header"`
	Training         string            `json:"training"`
	Recommended      string            `json:"recommended"`
	Confirm          string            `json:"confirm"`
	Reset            string            `json:"reset"`
	All              string            `json:"all"`
	Level            string            `json:"level"`
	Raised           string            `json:"raised"`
	Capped           string            `json:"capped"`
	Strength         string            `json:"strength"`
	Chance           string            `json:"chance"`
	First            string            `json:"first"`
	NoMinus          string            `json:"no_minus"`
	Short            map[string]string `json:"short"`
	FirstSkill       map[string]string `json:"first_skill"`
	ProblemStrengths string            `json:"problem_strengths"`
	ProblemEpisodes  string            `json:"problem_episodes"`
	PastLocked       string            `json:"past_locked"`
	Invalid          string            `json:"invalid"`
	NoRecheck        string            `json:"no_recheck"`
	Separator        string            `json:"separator"`
}

// heroFileUses: the checks and voices of one skill in the act-one scripts, and in the first hour's.
type heroFileUses struct {
	Checks, Voices, FirstHour int
	Files                     map[string]int
}

var heroFileID = regexp.MustCompile(`^[a-z0-9_]+$`)

func readHeroFile(world cue.Value) (*HeroFile, error) {
	v := world.LookupPath(cue.ParsePath("hero_file"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.hero_file: %w", err)
	}
	// the usage gate is read here and kept out of world.json (HeroFile.Usage is json:"-")
	var in struct {
		HeroFile
		Usage HeroFileUsage `json:"usage"`
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err = d.Decode(&in); err != nil {
		return nil, fmt.Errorf("world.hero_file: %w", err)
	}
	h := in.HeroFile
	h.Usage = in.Usage
	if err = h.validate(); err != nil {
		return nil, err
	}
	return &h, nil
}

// validate: the data's own consistency (ids, profiles of equal sum, one canon, effects of one kind, a valid default).
func (h HeroFile) validate() error {
	fail := func(f string, a ...any) error { return fmt.Errorf("world.hero_file: "+f, a...) }
	skills := map[string]bool{}
	for _, s := range h.Skills {
		if !heroFileID.MatchString(s) || skills[s] {
			return fail("skill %q empty or repeated", s)
		}
		skills[s] = true
	}
	if len(skills) == 0 {
		return fail("no skills")
	}
	sum, canon := -1, 0
	profiles := map[string]bool{}
	for _, p := range h.Profiles {
		if !heroFileID.MatchString(p.ID) || profiles[p.ID] || p.Title == "" || p.Summary == "" || p.Icon == "" {
			return fail("profile %q: id, title, summary, icon", p.ID)
		}
		profiles[p.ID] = true
		if len(p.Skills) != len(skills) {
			return fail("profile %s: %d skills for the file's %d", p.ID, len(p.Skills), len(skills))
		}
		total := 0
		for s, v := range p.Skills {
			if !skills[s] || v < 0 || v > h.Strengths.StartCap {
				return fail("profile %s: skill %s %d is not the file's or above the start cap %d", p.ID, s, v, h.Strengths.StartCap)
			}
			total += v
		}
		if sum >= 0 && total != sum {
			return fail("profile %s: skills sum to %d, the others to %d (doc 104 v2: equal sums)", p.ID, total, sum)
		}
		sum = total
		if p.Canon {
			canon++
		}
	}
	if canon != 1 {
		return fail("%d canonical profiles, want one (the hero of the book)", canon)
	}
	st := h.Strengths
	if st.Count < 1 || st.Count > len(skills) || st.Bonus < 1 || st.StartCap < 1 || st.StartCap > 100 || st.LearnPermille <= 1000 || st.LearnPermille > 3000 {
		return fail("invalid strengths")
	}
	if h.EpisodesPick < 1 || h.EpisodesPick > len(h.Episodes) {
		return fail("pick %d of %d episodes", h.EpisodesPick, len(h.Episodes))
	}
	episodes, topics := map[string]bool{}, map[string]bool{}
	for _, e := range h.Episodes {
		if !heroFileID.MatchString(e.ID) || episodes[e.ID] || e.Title == "" || e.Plus == "" || e.Minus == "" || e.First == "" || !strings.HasPrefix(e.Topic, "topic.") || topics[e.Topic] || !strings.HasPrefix(e.Fact, "fact.file.") || topics[e.Fact] {
			return fail("episode %q: id, texts, its own topic and its own fact", e.ID)
		}
		episodes[e.ID], topics[e.Topic], topics[e.Fact] = true, true, true
		if err := h.effects("episode "+e.ID, e.Effects, skills); err != nil {
			return err
		}
	}
	traits := map[string]bool{}
	for _, t := range h.Traits {
		if !heroFileID.MatchString(t.ID) || traits[t.ID] || t.Title == "" || t.Plus == "" || t.Minus == "" || t.First == "" || len(t.Effects) == 0 {
			return fail("trait %q: id, texts and effects", t.ID)
		}
		traits[t.ID] = true
		if err := h.effects("trait "+t.ID, t.Effects, skills); err != nil {
			return err
		}
	}
	portraits := map[string]bool{}
	for _, p := range h.Portraits {
		if !heroFileID.MatchString(p.ID) || portraits[p.ID] || (p.Image != "" && !strings.HasPrefix(p.Image, "res://")) {
			return fail("portrait %q", p.ID)
		}
		portraits[p.ID] = true
	}
	if len(portraits) == 0 || h.Club == "" || h.NoTrait == "" {
		return fail("portraits, the club line, «no trait»")
	}
	if h.StillSeconds < 1 || h.Line.SureLevel == "" || h.Line.SurePercent < 50 || h.Line.SurePercent > 97 || h.Line.HopePercent < 3 || h.Line.HopePercent >= h.Line.SurePercent || h.Line.Shown < 1 || h.Line.Shown > 3 {
		return fail("invalid still seconds or skill line")
	}
	r := h.Recheck
	if !strings.HasPrefix(r.Level, "level.") || r.NightFromH < 0 || r.NightFromH > 23 || r.NightToH < 0 || r.NightToH > 23 || r.NightFromH == r.NightToH {
		return fail("invalid first check level or night")
	}
	if err := h.checkChoice(h.Default); err != nil {
		return fail("default: %v", err)
	}
	u := h.Usage
	if len(u.ActOne) == 0 || len(u.FirstHour) == 0 || u.MinAct < 1 || u.MinFirstHour < 1 {
		return fail("invalid usage gate")
	}
	act := map[string]bool{}
	for _, f := range u.ActOne {
		act[f] = true
	}
	for _, f := range u.FirstHour {
		if !act[f] {
			return fail("first-hour script %s is not in act one", f)
		}
	}
	return nil
}

func (h HeroFile) effects(where string, effects []HeroFileEffect, skills map[string]bool) error {
	for i, e := range effects {
		kinds := 0
		if e.Skill != nil {
			kinds++
			if !skills[e.Skill.ID] || e.Skill.Add == 0 {
				return fmt.Errorf("world.hero_file: %s effect %d: skill %s is not the file's", where, i, e.Skill.ID)
			}
		}
		if e.StepLoudnessPermille != 0 {
			kinds++
		}
		if e.VisibilityPermille != nil {
			kinds++
			if e.VisibilityPermille.While != "still" || e.VisibilityPermille.Permille < 250 || e.VisibilityPermille.Permille > 3000 {
				return fmt.Errorf("world.hero_file: %s effect %d: invalid visibility", where, i)
			}
		}
		if e.WoundAPRelief != 0 {
			kinds++
		}
		if kinds != 1 {
			return fmt.Errorf("world.hero_file: %s effect %d: %d kinds, want one", where, i, kinds)
		}
	}
	return nil
}

// checkChoice: a complete choice of the file's own ids (the runtime's rule, here for the default).
func (h HeroFile) checkChoice(c HeroFileChoice) error {
	ok := false
	for _, p := range h.Profiles {
		ok = ok || p.ID == c.Profile
	}
	if !ok {
		return fmt.Errorf("unknown profile %q", c.Profile)
	}
	seen := map[string]bool{}
	for _, s := range c.Strengths {
		if seen[s] || !contains(h.Skills, s) {
			return fmt.Errorf("strength %q unknown or repeated", s)
		}
		seen[s] = true
	}
	if len(c.Strengths) != h.Strengths.Count {
		return fmt.Errorf("%d strengths, want %d", len(c.Strengths), h.Strengths.Count)
	}
	for _, e := range c.Episodes {
		found := false
		for _, x := range h.Episodes {
			found = found || x.ID == e
		}
		if seen["ep:"+e] || !found {
			return fmt.Errorf("episode %q unknown or repeated", e)
		}
		seen["ep:"+e] = true
	}
	if len(c.Episodes) != h.EpisodesPick {
		return fmt.Errorf("%d episodes, want %d", len(c.Episodes), h.EpisodesPick)
	}
	if c.Trait != "" {
		found := false
		for _, t := range h.Traits {
			found = found || t.ID == c.Trait
		}
		if !found {
			return fmt.Errorf("unknown trait %q", c.Trait)
		}
	}
	found := false
	for _, p := range h.Portraits {
		found = found || p.ID == c.Portrait
	}
	if !found {
		return fmt.Errorf("unknown portrait %q", c.Portrait)
	}
	return nil
}

// start: the file's start of each of its skills for a choice — the profile, +bonus on a strength, the skill effects of
// the chosen episodes and trait, within 0..start cap (the rule of templates/hero_file.go.tmpl start and of the CUE
// _heroFileCanon; here for the canon check).
func (h HeroFile) start(c HeroFileChoice) map[string]int {
	var p HeroFileProfile
	for _, x := range h.Profiles {
		if x.ID == c.Profile {
			p = x
		}
	}
	var effects []HeroFileEffect
	for _, e := range h.Episodes {
		if contains(c.Episodes, e.ID) {
			effects = append(effects, e.Effects...)
		}
	}
	for _, t := range h.Traits {
		if t.ID == c.Trait {
			effects = append(effects, t.Effects...)
		}
	}
	out := map[string]int{}
	for _, s := range h.Skills {
		v := p.Skills[s]
		if contains(c.Strengths, s) {
			v += h.Strengths.Bonus
		}
		for _, e := range effects {
			if e.Skill != nil && e.Skill.ID == s {
				v += e.Skill.Add
			}
		}
		out[s] = min(h.Strengths.StartCap, max(0, v))
	}
	return out
}

// check against the compiled world: developed skills with a conversation tag, every text localized, the difficulty of
// the skill line, the first check's level, the canon (the hero's Life profile starts exactly as the default), and the
// usage gate over the act-one scripts.
func (h *HeroFile) check(c *Compiled) error {
	if h == nil {
		return nil
	}
	fail := func(f string, a ...any) error { return fmt.Errorf("world.hero_file: "+f, a...) }
	if c.DialogueRules == nil {
		return fail("the file's skill lines need world.dialogue_rules")
	}
	sure := false
	for _, level := range c.DialogueRules.Difficulties {
		sure = sure || level.ID == h.Line.SureLevel
	}
	if !sure {
		return fail("line: unknown difficulty %s", h.Line.SureLevel)
	}
	var life struct {
		Development struct {
			Skills []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"skills"`
			Profiles map[string]struct {
				Skills map[string]struct {
					Start int `json:"start"`
					Cap   int `json:"cap"`
				} `json:"skills"`
			} `json:"profiles"`
		} `json:"development"`
		Locations map[string]json.RawMessage `json:"locations"`
	}
	if err := json.Unmarshal(c.Life, &life); err != nil {
		return err
	}
	names := map[string]string{}
	for _, s := range life.Development.Skills {
		names[s.ID] = s.Name
	}
	for _, s := range h.Skills {
		if names[s] == "" {
			return fail("skill %s is not developed", s)
		}
		if _, ok := c.DialogueRules.Skills[s]; !ok {
			return fail("skill %s has no conversation tag", s)
		}
	}
	if _, ok := life.Locations[h.Recheck.Level]; !ok {
		return fail("first check: unknown level %s", h.Recheck.Level)
	}
	if err := h.checkFight(c); err != nil {
		return err
	}
	consumers, err := heroFileConsumers(c)
	if err != nil {
		return err
	}
	if err := h.checkConsumers(consumers); err != nil {
		return err
	}
	hero, ok := life.Development.Profiles["char.hero"]
	if !ok {
		return fail("the hero has no development profile")
	}
	canon := h.start(h.Default)
	for _, s := range h.Skills {
		got, ok := hero.Skills[s]
		if !ok || got.Start != canon[s] {
			return fail("the hero of the book starts %s at %d, the hero's profile at %d: derive world.persons from the file", s, canon[s], got.Start)
		}
		if got.Cap < h.Strengths.StartCap {
			return fail("skill %s: the hero's growth cap %d is below the start cap %d", s, got.Cap, h.Strengths.StartCap)
		}
	}
	keys := []string{h.Fight.Text, h.Club, h.NoTrait, h.Recheck.Title, h.Recheck.Body, h.Recheck.Do, h.Recheck.Confirm, h.Recheck.Skip, h.Recheck.Done}
	t := h.Text
	// every key the runtime words the screen and its refusals with comes from here (templates/hero_file.go.tmpl names no key)
	keys = append(keys, t.Header, t.Training, t.Recommended, t.Confirm, t.Reset, t.All, t.Level, t.Raised, t.Capped, t.Strength, t.Chance, t.First, t.NoMinus, t.ProblemStrengths, t.ProblemEpisodes, t.PastLocked, t.Invalid, t.NoRecheck, t.Separator)
	for _, s := range h.Skills {
		if t.Short[s] == "" || t.FirstSkill[s] == "" {
			return fail("skill %s: no short name or first-use text", s)
		}
		keys = append(keys, t.Short[s], t.FirstSkill[s])
	}
	for _, p := range h.Profiles {
		keys = append(keys, p.Title, p.Summary)
	}
	for _, e := range h.Episodes {
		keys = append(keys, e.Title, e.Plus, e.Minus, e.First)
		if e.TopicJournal != "" {
			keys = append(keys, e.TopicJournal)
		}
	}
	for _, tr := range h.Traits {
		keys = append(keys, tr.Title, tr.Plus, tr.Minus, tr.First)
	}
	if err := requireLocalized(c, "world.hero_file", keys); err != nil {
		return err
	}
	for _, key := range keys {
		for _, locale := range c.Localization.Locales {
			if !localizedIn(c, key, locale) {
				return fail("no %s text for %s", locale, key)
			}
		}
	}
	if err := h.Usage.addDialogue(c); err != nil {
		return err
	}
	return h.checkUsage(names)
}

// checkReaders: every topic and recorded fact the past block gives has a reader in act one (doc 104 «Решения при
// сборке»: a topic or fact nobody reads fails the build). The readers are the combined Markdown and CUE usage.
func (h *HeroFile) checkReaders() error {
	read := map[string]int{}
	for _, number := range h.Usage.ActOne {
		for id, n := range h.Usage.Reads[number] {
			read[id] += n
		}
	}
	var unread []string
	for _, e := range h.Episodes {
		for _, id := range []string{e.Topic, e.Fact} {
			if read[id] == 0 {
				unread = append(unread, id)
			}
		}
	}
	if len(unread) > 0 {
		return fmt.Errorf("world.hero_file: the past gives what no act-one script reads (doc 104): %s", strings.Join(unread, ", "))
	}
	return nil
}

// heroFileConsumers: which effect kinds act on something in this world (doc 09, 21.11: the file promises only what acts
// now): a file skill — the development; quieter steps — the stealth's footsteps; being recognised sooner — the stealth's
// watchers; the wound relief — the combat rules' action points.
func heroFileConsumers(c *Compiled) (map[string]bool, error) {
	var stealth struct {
		Enabled  bool                       `json:"enabled"`
		Watchers map[string]json.RawMessage `json:"watchers"`
	}
	if len(c.Stealth) > 0 {
		if err := json.Unmarshal(c.Stealth, &stealth); err != nil {
			return nil, err
		}
	}
	var combat struct {
		Rules struct {
			APPerTurn int `json:"ap_per_turn"`
		} `json:"rules"`
	}
	if len(c.CombatRuntime) > 0 {
		if err := json.Unmarshal(c.CombatRuntime, &combat); err != nil {
			return nil, err
		}
	}
	return map[string]bool{"skill": true, "step_loudness": stealth.Enabled, "visibility": stealth.Enabled && len(stealth.Watchers) > 0, "wound_ap_relief": combat.Rules.APPerTurn > 0}, nil
}

// checkConsumers: every effect of every episode and trait acts on something in this world.
func (h *HeroFile) checkConsumers(consumers map[string]bool) error {
	check := func(where string, effects []HeroFileEffect) error {
		for i, e := range effects {
			kind := "skill"
			switch {
			case e.StepLoudnessPermille != 0:
				kind = "step_loudness"
			case e.VisibilityPermille != nil:
				kind = "visibility"
			case e.WoundAPRelief != 0:
				kind = "wound_ap_relief"
			}
			if !consumers[kind] {
				return fmt.Errorf("world.hero_file: %s effect %d: %s acts on nothing in this world (doc 09 21.11: the file promises only what acts now)", where, i, kind)
			}
		}
		return nil
	}
	for _, e := range h.Episodes {
		if err := check("episode "+e.ID, e.Effects); err != nil {
			return err
		}
	}
	for _, t := range h.Traits {
		if err := check("trait "+t.ID, t.Effects); err != nil {
			return err
		}
	}
	return nil
}

// checkFight: the fight line names a melee weapon of the combat catalog and an opponent profile, and the hero's combat
// profile develops one of the file's skills (part 1b: the strength the screen offers is the fight's number).
func (h *HeroFile) checkFight(c *Compiled) error {
	fail := func(f string, a ...any) error { return fmt.Errorf("world.hero_file: fight: "+f, a...) }
	var combat struct {
		Profiles map[string]struct {
			Develops *struct {
				Skill string `json:"skill"`
			} `json:"develops"`
		} `json:"profiles"`
		Actors  map[string]string `json:"actors"`
		Weapons map[string]struct {
			Kind     string `json:"kind"`
			Magazine int    `json:"magazine"`
		} `json:"weapons"`
	}
	if len(c.CombatRuntime) == 0 {
		return fail("the world has no combat_runtime")
	}
	if err := json.Unmarshal(c.CombatRuntime, &combat); err != nil {
		return err
	}
	w, ok := combat.Weapons[h.Fight.Weapon]
	if !ok || w.Kind == "firearm" || w.Magazine > 0 {
		return fail("weapon %s is not a melee weapon of the combat catalog", h.Fight.Weapon)
	}
	if _, ok := combat.Profiles[h.Fight.Opponent]; !ok {
		return fail("unknown opponent profile %s", h.Fight.Opponent)
	}
	hero := combat.Profiles[combat.Actors["char.hero"]]
	if hero.Develops == nil || !contains(h.Skills, hero.Develops.Skill) {
		return fail("the hero's combat profile develops no skill of the file")
	}
	return nil
}

func localizedIn(c *Compiled, key, locale string) bool {
	for _, m := range c.Localization.Messages {
		if m.Key == key {
			return m.Texts[locale] != ""
		}
	}
	return false
}

// checkUsage: every skill of the file is used at least min_act times in act one and min_first_hour times in the first
// hour (the CUE counts of each act-one script, all of which must be counted); the counts go into the report.
func (h *HeroFile) checkUsage(names map[string]string) error {
	first := map[string]bool{}
	for _, f := range h.Usage.FirstHour {
		first[f] = true
	}
	h.uses = map[string]heroFileUses{}
	for _, s := range h.Skills {
		h.uses[s] = heroFileUses{Files: map[string]int{}}
	}
	for _, number := range h.Usage.ActOne {
		counts, ok := h.Usage.Counts[number]
		if !ok {
			return fmt.Errorf("world.hero_file: usage: script %s of act one is not counted (rerun tools/hero-file/skill_uses.py write)", number)
		}
		for id := range counts {
			if names[id] == "" {
				return fmt.Errorf("world.hero_file: usage: script %s counts an unknown skill %s", number, id)
			}
		}
		for _, s := range h.Skills {
			c := counts[s]
			u := h.uses[s]
			u.Checks += c.Checks
			u.Voices += c.Voices
			if first[number] {
				u.FirstHour += c.Checks + c.Voices
			}
			if c.Checks+c.Voices > 0 {
				u.Files[number] = c.Checks + c.Voices
			}
			h.uses[s] = u
		}
	}
	var report bytes.Buffer
	fmt.Fprintf(&report, "# Навыки личного дела: применения в первом акте\n\nСгенерировано ANG (compiler/gamepoc/hero_file.go) из условий и реплик CUE и оставшегося счёта сценариев (`tools/hero-file/skill_uses.py`). Сценарии %s, первый час — %s. Навык на экране «Личное дело» — только при ≥ %d применениях в акте и ≥ %d в первый час (док. 104 v2 п. 10).\n\n| навык | проверки | голоса | всего | первый час | по сценариям |\n|---|---|---|---|---|---|\n",
		strings.Join([]string{h.Usage.ActOne[0], h.Usage.ActOne[len(h.Usage.ActOne)-1]}, "–"), strings.Join(h.Usage.FirstHour, ", "), h.Usage.MinAct, h.Usage.MinFirstHour)
	var short []string
	for _, s := range h.Skills {
		u := h.uses[s]
		files := make([]string, 0, len(u.Files))
		for f := range u.Files {
			files = append(files, f)
		}
		sort.Strings(files)
		parts := make([]string, len(files))
		for i, f := range files {
			parts[i] = fmt.Sprintf("%s: %d", f, u.Files[f])
		}
		fmt.Fprintf(&report, "| %s | %d | %d | %d | %d | %s |\n", names[s], u.Checks, u.Voices, u.Checks+u.Voices, u.FirstHour, strings.Join(parts, ", "))
		if u.Checks+u.Voices < h.Usage.MinAct || u.FirstHour < h.Usage.MinFirstHour {
			short = append(short, fmt.Sprintf("%s: %d in act one (min %d), %d in the first hour (min %d)", s, u.Checks+u.Voices, h.Usage.MinAct, u.FirstHour, h.Usage.MinFirstHour))
		}
	}
	if len(short) > 0 {
		return fmt.Errorf("world.hero_file: a skill the act-one scripts barely use is offered on the file screen (doc 104 v2 p. 10): %s", strings.Join(short, "; "))
	}
	if err := h.checkReaders(); err != nil {
		return err
	}
	h.report = report.Bytes()
	return nil
}
