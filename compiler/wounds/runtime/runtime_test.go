package wounds

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// fixture: a small catalog with a limb, a junction, a belly and an arm (the real one is world.wounds, tested by the
// generated catalog test with its CUE scenarios).
func fixture() Model {
	sharp := map[string]int64{"cut": Million, "stab": Million, "puncture": Million, "blunt": 700000}
	return Model{
		Version:    1,
		Difficulty: Difficulty{ID: "standard", BleedPermille: 1000, DoomPermille: 1000, AidPermille: 1000, PricePermille: 1000},
		Blood:      Blood{VolumeML: 5000, RegenMLH: 20, StepS: 1, StoppedBelowMLMin: 1},
		Pressure:   Pressure{SlopePermille: 1400, FloorPermille: 200},
		States:     States{WeakPermille: 150, ConfusedPermille: 300, DownPermille: 400, DeadPermille: 500, WakeBelowPermille: 350, Weak: StateMod{AP: 1, PacePermille: 800}, Confused: StateMod{AP: 2, PacePermille: 500}},
		Effects: map[string]EffectDef{
			"pain": {AP: 1, PacePermille: 900, HoldWeapon: true}, "drop_weapon": {PacePermille: 1000}, "limp": {AP: 1, PacePermille: 450, HoldWeapon: true},
			"no_walk": {AP: 2, Falls: true, HoldWeapon: true}, "breathless": {AP: 2, PacePermille: 600, HoldWeapon: true}, "concussion": {AP: 1, PacePermille: 800, HoldWeapon: true},
		},
		Walk: Walk{NormalMMS: 1300, SplintedPacePermille: 250},
		Mechanisms: map[string]Mechanism{
			"cut": {Name: "m.cut", BleedPermille: 1000}, "stab": {Name: "m.stab", BleedPermille: 1000}, "puncture": {Name: "m.p", BleedPermille: 500},
			"blunt": {Name: "m.b", BleedPermille: 300}, "electric": {Name: "m.e", Incapacitate: &Span{Min: 3, Max: 10}}, "choke": {Name: "m.c"},
		},
		Chips: map[string]Chip{"none": {ClotPermille: 1000, RegenPermille: 1000}, "bronze": {Monitor: true, AlarmAt: "confused", ClotPermille: 1000, RegenPermille: 1000}, "gold": {Monitor: true, AlarmAt: "confused", ClotPermille: 500, RegenPermille: 4000, DownBonusPermille: 30}, "platinum": {Monitor: true, AlarmAt: "confused", ClotPermille: 500, RegenPermille: 4000, Protected: true}},
		Zones: []Zone{
			{ID: "thigh", Name: "z.thigh", Part: "legs", Tourniquet: true, Structures: []Structure{
				{ID: "thigh_muscle", Name: "s.tm", Kind: "capillary", MinDepth: "graze", Exposure: sharp, BleedMLMin: 8, ClotHalfS: 150, Severity: 1, HealDays: 10, CapsuleMin: 15, Sign: "blood"},
				{ID: "thigh_vein", Name: "s.tv", Kind: "deep_vein", MinDepth: "deep", Exposure: map[string]int64{"stab": 200000}, BleedMLMin: 30, ClotHalfS: 5400, Severity: 4, HealDays: 21, CapsuleMin: 90, Sign: "blood_flow"},
				{ID: "femoral_artery", Name: "s.fa", Kind: "artery", MinDepth: "deep", Exposure: map[string]int64{"stab": 200000, "cut": 120000}, BleedMLMin: 720, Severity: 9, HealDays: 45, CapsuleMin: 240, Sign: "blood_spurt"},
				{ID: "femur", Name: "s.femur", Kind: "bone", MinDepth: "deep", Exposure: map[string]int64{"blunt": 120000}, BleedMLMin: 15, ClotHalfS: 3600, Internal: true, Effects: []string{"no_walk", "pain"}, Severity: 7, HealDays: 90, CapsuleMin: 720, Sign: "still"},
			}},
			{ID: "groin", Name: "z.groin", Part: "torso", Junctional: true, Structures: []Structure{
				{ID: "groin_skin", Name: "s.gs", Kind: "capillary", MinDepth: "graze", Exposure: sharp, BleedMLMin: 6, ClotHalfS: 180, Severity: 1, HealDays: 7, CapsuleMin: 15, Sign: "blood"},
				{ID: "groin_artery", Name: "s.ga", Kind: "artery", MinDepth: "deep", Exposure: map[string]int64{"stab": 250000}, BleedMLMin: 900, Severity: 9, HealDays: 45, CapsuleMin: 240, Sign: "blood_spurt"},
			}},
			{ID: "abdomen", Name: "z.abd", Part: "torso", Cavity: true, Structures: []Structure{
				{ID: "belly_wall", Name: "s.bw", Kind: "capillary", MinDepth: "graze", Exposure: sharp, BleedMLMin: 6, ClotHalfS: 180, Severity: 1, HealDays: 10, CapsuleMin: 15, Sign: "blood"},
				{ID: "liver", Name: "s.liver", Kind: "organ", MinDepth: "deep", Exposure: map[string]int64{"stab": 300000}, BleedMLMin: 40, Internal: true, Effects: []string{"pain"}, Severity: 8, HealDays: 45, CapsuleMin: 360, Sign: "clutch"},
			}},
			{ID: "arm", Name: "z.arm", Part: "arms", Tourniquet: true, Structures: []Structure{
				{ID: "arm_muscle", Name: "s.am", Kind: "capillary", MinDepth: "graze", Exposure: sharp, BleedMLMin: 6, ClotHalfS: 120, Severity: 1, HealDays: 7, CapsuleMin: 10, Sign: "blood"},
				{ID: "arm_bone", Name: "s.ab", Kind: "bone", MinDepth: "deep", Exposure: map[string]int64{"blunt": 450000}, Effects: []string{"drop_weapon", "pain"}, Severity: 5, HealDays: 42, CapsuleMin: 360, Sign: "clutch"},
			}},
			{ID: "neck", Name: "z.neck", Part: "head", Junctional: true, Structures: []Structure{
				{ID: "carotid", Name: "s.car", Kind: "critical", MinDepth: "deep", Exposure: map[string]int64{"cut": 200000}, BleedMLMin: 1500, Knockout: &Span{Min: 8, Max: 15}, Severity: 10, HealDays: 60, CapsuleMin: 300, Sign: "blood_spurt"},
			}},
		},
		Aim:    map[string]map[string]int64{"legs": {"thigh": 1}, "torso": {"abdomen": 3, "groin": 1}, "arms": {"arm": 1}, "head": {"neck": 1}},
		Angles: map[string]map[string]int64{"front": {}, "rear": {"neck": 1}},
		Weapons: []Weapon{
			{Weapon: "item.knife", Name: "w.knife", SkillDeepenPPM: 2000, Modes: map[string]Mode{"quick": {Mechanism: "cut", Depth: map[string]int64{"graze": 40, "shallow": 45, "deep": 15}}, "*": {Mechanism: "stab", Depth: map[string]int64{"graze": 10, "shallow": 30, "deep": 50, "through": 10}}}},
			{Weapon: "item.baton", Name: "w.baton", SkillDeepenPPM: 1500, Modes: map[string]Mode{"*": {Mechanism: "blunt", Depth: map[string]int64{"graze": 10, "shallow": 35, "deep": 55}}}},
		},
		Techniques: []Technique{{ID: "chokehold", Name: "t.choke", Mechanism: "choke", Zone: "neck", Angle: "rear", Depth: map[string]int64{"graze": 1}, Knockout: &Span{Min: 8, Max: 14}, Wake: &Span{Min: 10, Max: 40}, SafeAfterKOS: 20, DeathPPMPerS: 30000, ArrestPPM: 2000}},
		Armor:      []Armor{{Item: "item.jacket", Zones: []string{"abdomen", "arm"}, Stop: map[string]int64{"cut": 400000}, Shift: map[string]int64{"stab": 400000, "blunt": 300000}}},
		Aid: Aid{
			Kinds: map[string]AidKind{
				"pressure":   {Name: "a.pressure", Seconds: 5, Held: true, Zones: []string{"thigh", "groin", "abdomen", "arm", "neck"}, Reduce: map[string]int64{"capillary": 950, "vein": 900, "deep_vein": 800, "artery": 700, "critical": 300}},
				"tourniquet": {Name: "a.tq", Seconds: 20, Zones: []string{"thigh", "arm"}, Reduce: map[string]int64{"capillary": 1000, "vein": 1000, "deep_vein": 1000, "artery": 1000, "critical": 1000}, LimbInside: true, LimbSafeS: 7200, LimbLostS: 21600, LimbEffects: map[string][]string{"legs": {"limp"}, "arms": {"drop_weapon"}}, LimbLostFx: map[string][]string{"legs": {"no_walk"}, "arms": {"drop_weapon"}}},
				"packing":    {Name: "a.pack", Seconds: 180, Zones: []string{"thigh", "groin", "arm", "neck"}, Reduce: map[string]int64{"capillary": 1000, "vein": 950, "deep_vein": 900, "artery": 850, "critical": 500}},
				"dressing":   {Name: "a.dress", Seconds: 30, Zones: []string{"thigh", "groin", "abdomen", "arm", "neck"}, Reduce: map[string]int64{"capillary": 1000, "vein": 900, "deep_vein": 600, "artery": 300, "critical": 100}},
				"seal":       {Name: "a.seal", Seconds: 15, Zones: []string{"abdomen"}, Reduce: map[string]int64{"capillary": 900}, Ease: []string{"breathless"}},
				"medkit":     {Name: "a.kit", Seconds: 60, Zones: []string{"thigh", "groin", "abdomen", "arm", "neck"}, Reduce: map[string]int64{}},
			},
			Items: []AidItem{{Item: "item.tq", Kind: "tourniquet", SuccessPPM: 950000}, {Item: "item.scarf", Kind: "tourniquet", SuccessPPM: 550000, Improvised: true}, {Item: "item.gauze", Kind: "packing", SuccessPPM: 900000}, {Item: "item.patch", Kind: "dressing", SuccessPPM: 950000}},
			Medkits: []Medkit{
				{Item: "item.kit1", Level: 1, SuccessPPM: 950000, MaxDepth: "shallow", Treats: []string{"capillary", "vein"}, PainOff: true, PainOffS: 14400},
				{Item: "item.kit2", Level: 2, SuccessPPM: 950000, MaxDepth: "deep", Treats: []string{"capillary", "vein", "deep_vein", "nerve"}, Tourniquet: true, Packing: true, Splint: true, PainOff: true, PainOffS: 14400},
			},
			Skill: AidSkill{Skill: "medicine", PerPointPPM: 1000, CapPPM: 990000},
		},
		Providers: []Provider{
			{ID: "capsule", Name: "p.capsule", Operator: "system", Places: []string{"hub"}, Chip: "own", Price: Price{BaseMinor: 200, PerSeverityMinor: 50, PerHourMinor: 50}, TimePermille: 1000, BloodMinPer100ML: 5, Stabilize: Stabilize{Min: 15, RestoreToPermille: 140}, Cures: []string{"wounds", "blood", "fractures", "concussion", "poison", "disease", "burn", "infection"}, Journal: []string{"stab", "puncture"}},
			{ID: "healer", Name: "p.healer", Operator: "grey", Places: []string{"grey_market"}, Chip: "any", Price: Price{BaseMinor: 100, PerSeverityMinor: 25, PerHourMinor: 10, StabilizeMinor: 50}, TimePermille: 2500, Stabilize: Stabilize{Min: 30, RestoreToPermille: 1000}, Cures: []string{"wounds", "fractures"}, RefusesKinds: []string{"organ", "brain", "critical"}, ComplicationPPM: 150000, Complication: "infection.wound"},
		},
		Conditions: []ConditionDef{
			{ID: "poison.ration", Kind: "poison", Name: "c.poison", Onset: Span{Min: 3600, Max: 10800}, Effects: []string{"pain"}, Doom: &Span{Min: 43200, Max: 108000}, Severity: 6, CapsuleMin: 90, HealDays: 3, Sign: "clutch"},
			{ID: "infection.wound", Kind: "infection", Name: "c.inf", Onset: Span{Min: 86400, Max: 172800}, Effects: []string{"pain"}, Doom: &Span{Min: 259200, Max: 604800}, Severity: 5, CapsuleMin: 240, HealDays: 14, Sign: "none"},
		},
		Resolve: Resolve{SeriousSeverity: 4, CourageBase: 50, Reactions: map[string]map[string]int64{
			"fight": {"fight_on": 600, "flee": 150, "fall": 100, "surrender": 150}, "flight": {"fight_on": 100, "flee": 700, "fall": 100, "surrender": 100},
			"freeze": {"fight_on": 100, "flee": 100, "fall": 600, "surrender": 200}, "appease": {"fight_on": 50, "flee": 150, "fall": 100, "surrender": 700},
		}},
		Justice: Justice{Death: "crime.killing", Serious: "crime.maiming", Hit: "crime.assault", Hero: "char.hero"},
	}
}

func mustModel(t *testing.T) Model {
	t.Helper()
	m := fixture()
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	return m
}

func hit(t *testing.T, m Model, b *Body, h Hit) HitResult {
	t.Helper()
	r, err := m.ApplyHit(b, h)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func aid(t *testing.T, m Model, b *Body, a AidAct) AidResult {
	t.Helper()
	r, err := m.ApplyAid(b, a)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func asJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestValidateRejectsBrokenThresholds(t *testing.T) {
	m := fixture()
	m.States.DownPermille = 200 // below confused
	if err := m.Validate(); err == nil {
		t.Fatal("non-monotonic thresholds accepted")
	}
	m = fixture()
	m.Aid.Kinds["tourniquet"] = AidKind{Name: "x", Seconds: 1, Zones: []string{"groin"}}
	if err := m.Validate(); err == nil || !strings.Contains(err.Error(), "tourniquet") {
		t.Fatalf("a tourniquet on a junction accepted: %v", err)
	}
}

// The same seed and the same acts give the same body; ticking in pieces equals ticking at once (quiet leaps included).
func TestDeterminism(t *testing.T) {
	m := mustModel(t)
	run := func(split bool) (Body, []Event) {
		b := m.NewBody("npc.a", "bronze")
		var ev []Event
		for i, part := range []string{"legs", "torso", "arms", "torso"} {
			r := hit(t, m, &b, Hit{Author: "char.hero", Weapon: "item.knife", Part: part, Skill: 40, Seed: 77 + int64(i)})
			ev = append(ev, r.Events...)
			ev = append(ev, m.Tick(&b, 7)...)
		}
		if split {
			for i := 0; i < 50; i++ {
				ev = append(ev, m.Tick(&b, 173)...)
			}
			ev = append(ev, m.Tick(&b, 86400*3-50*173)...)
		} else {
			ev = append(ev, m.Tick(&b, 86400*3)...)
		}
		return b, ev
	}
	a, ea := run(false)
	b, eb := run(true)
	if asJSON(t, a) != asJSON(t, b) || asJSON(t, ea) != asJSON(t, eb) {
		t.Fatalf("ticking in pieces differs:\n%s\n%s", asJSON(t, a), asJSON(t, b))
	}
	c, _ := run(false)
	if asJSON(t, a) != asJSON(t, c) {
		t.Fatal("same seed, different body")
	}
	d := m.NewBody("npc.a", "bronze")
	hit(t, m, &d, Hit{Author: "char.hero", Weapon: "item.knife", Part: "legs", Skill: 40, Seed: 78})
	if asJSON(t, d.Wounds) == asJSON(t, a.Wounds[:1]) && d.Wounds[0].Depth == a.Wounds[0].Depth && reflect.DeepEqual(d.Wounds[0].Structures, a.Wounds[0].Structures) {
		t.Log("seed 78 happens to repeat seed 77; fine, but rolls must depend on the seed")
	}
}

// A save mid-bleed (pressure held, a tourniquet on, a capsule running) replays exactly after loading.
func TestSaveLoadReplays(t *testing.T) {
	m := mustModel(t)
	b := m.NewBody("char.hero", "bronze")
	r := hit(t, m, &b, Hit{Author: "npc.x", Weapon: "item.knife", Zone: "thigh", Depth: "deep", Structures: []string{"thigh_muscle", "femoral_artery"}, Seed: 1})
	m.Tick(&b, 30)
	aid(t, m, &b, AidAct{Kind: "tourniquet", Wound: r.Wound.ID, Item: "item.tq", Force: true})
	g := m.NewBody("npc.y", "none")
	r2 := hit(t, m, &g, Hit{Author: "char.hero", Weapon: "item.knife", Zone: "groin", Depth: "deep", Structures: []string{"groin_artery"}, Seed: 2})
	aid(t, m, &g, AidAct{Kind: "pressure", Wound: r2.Wound.ID, By: "char.hero"})
	m.Tick(&g, 40)
	c := m.NewBody("npc.z", "bronze")
	hit(t, m, &c, Hit{Weapon: "item.baton", Zone: "arm", Depth: "deep", Structures: []string{"arm_bone"}, Seed: 3})
	q, _ := m.Quote(&c, "capsule", Payer{Chip: "own", BalanceMinor: 100000})
	if _, err := m.StartCare(&c, q, 9); err != nil {
		t.Fatal(err)
	}
	m.Tick(&c, 600)
	led := Ledger{Kills: map[string][]string{"char.hero": {"npc.q"}}}
	data, err := m.Marshal(Save{Bodies: []Body{b, g, c}, Ledger: led})
	if err != nil {
		t.Fatal(err)
	}
	s, err := m.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	if s.Ledger.Count("char.hero") != 1 {
		t.Fatal("ledger lost")
	}
	live := []Body{b, g, c} // the save sorts by id: char.hero, npc.y, npc.z
	for i := range s.Bodies {
		x, y := s.Bodies[i], live[i]
		ex, ey := m.Tick(&x, 7200), m.Tick(&y, 7200)
		if asJSON(t, x) != asJSON(t, y) || asJSON(t, ex) != asJSON(t, ey) {
			t.Fatalf("body %s diverges after load:\n%s\n%s\n%s\n%s", x.ID, asJSON(t, x), asJSON(t, y), asJSON(t, ex), asJSON(t, ey))
		}
	}
	bad := strings.Replace(string(data), `"femoral_artery"`, `"femoral_arteryX"`, 1)
	if _, err := m.Load([]byte(bad)); err == nil {
		t.Fatal("a save naming an unknown structure loaded")
	}
	if _, err := m.Load([]byte(strings.Replace(string(data), `"version":1`, `"version":1,"extra":1`, 1))); err == nil {
		t.Fatal("unknown field accepted")
	}
}

// A tourniquet closes a limb artery; on the groin it cannot go on at all — packing and pressure only slow it.
func TestTourniquetLimbButNotJunction(t *testing.T) {
	m := mustModel(t)
	b := m.NewBody("v", "bronze")
	r := hit(t, m, &b, Hit{Weapon: "item.knife", Zone: "thigh", Depth: "deep", Structures: []string{"thigh_muscle", "femoral_artery"}, Seed: 5})
	if f := m.Forecast(b, 3600); f.DownS < 150 || f.DownS > 330 || f.DeadS < 240 || f.DeadS > 480 {
		t.Fatalf("thigh artery untreated: %+v (minutes expected)", f)
	}
	m.Tick(&b, 30)
	res := aid(t, m, &b, AidAct{Kind: "tourniquet", Wound: r.Wound.ID, Item: "item.tq", Force: true})
	if !res.Applied || !res.Spent || res.Seconds != 20 {
		t.Fatalf("tourniquet: %+v", res)
	}
	if st := m.Status(b); st.ExternalULS != 0 || !contains(st.Effects, "limp") {
		t.Fatalf("a tourniquet must stop the limb and make him limp: %+v", st)
	}
	m.Tick(&b, 3600)
	if b.Dead || m.Status(b).State == "down" {
		t.Fatalf("dead or down under a tourniquet: %+v", m.Status(b))
	}
	g := m.NewBody("v2", "bronze")
	r = hit(t, m, &g, Hit{Weapon: "item.knife", Zone: "groin", Depth: "deep", Structures: []string{"groin_artery"}, Seed: 6})
	if res := aid(t, m, &g, AidAct{Kind: "tourniquet", Wound: r.Wound.ID, Item: "item.tq", Force: true}); res.Applied || res.Reason != "wounds.aid.wrong_zone" || res.Spent {
		t.Fatalf("a tourniquet went on the groin: %+v", res)
	}
	before := m.Forecast(g, 7200)
	aid(t, m, &g, AidAct{Kind: "packing", Wound: r.Wound.ID, Item: "item.gauze", Force: true})
	after := m.Forecast(g, 7200)
	if m.Status(g).ExternalULS == 0 || after.DeadS < 0 || after.DeadS <= before.DeadS {
		t.Fatalf("packing must slow, not stop, a junctional artery: before %+v after %+v", before, after)
	}
	// an improvised tourniquet can fail; the item is spent anyway
	h := m.NewBody("v3", "none")
	r = hit(t, m, &h, Hit{Weapon: "item.knife", Zone: "thigh", Depth: "deep", Structures: []string{"femoral_artery"}, Seed: 7})
	failed := false
	for s := int64(0); s < 40 && !failed; s++ {
		res := aid(t, m, &h, AidAct{Kind: "tourniquet", Wound: r.Wound.ID, Item: "item.scarf", Seed: s})
		failed = !res.Applied && res.Reason == "wounds.aid.failed" && res.Spent
		h.Wounds[0].Controls = nil
	}
	if !failed {
		t.Fatal("an improvised tourniquet never failed in 40 tries at 55 %")
	}
}

// Who dealt the wound answers for the death, even if it came minutes later; a saved man is not a kill.
func TestKillAttribution(t *testing.T) {
	m := mustModel(t)
	var led Ledger
	b := m.NewBody("npc.thug", "none")
	r := hit(t, m, &b, Hit{Author: "char.hero", Weapon: "item.knife", Zone: "thigh", Depth: "deep", Structures: []string{"thigh_muscle", "femoral_artery"}, Seed: 11})
	led.Record(r.Events)
	ev := m.Tick(&b, 600)
	led.Record(ev)
	if !b.Dead || b.Culprit != "char.hero" || led.Count("char.hero") != 1 {
		t.Fatalf("hero's wound killed after %d s: dead=%v culprit=%q kills=%d", b.DeadAt, b.Dead, b.Culprit, led.Count("char.hero"))
	}
	crime := ""
	for _, e := range ev {
		if e.Type == "dead" {
			crime = e.Crime
		}
	}
	if crime != "crime.killing" || b.DeadAt < 180 {
		t.Fatalf("the death must come minutes later and be a killing: at %d, crime %q", b.DeadAt, crime)
	}
	var led2 Ledger
	s := m.NewBody("npc.thug2", "none")
	r = hit(t, m, &s, Hit{Author: "char.hero", Weapon: "item.knife", Zone: "thigh", Depth: "deep", Structures: []string{"thigh_muscle", "femoral_artery"}, Seed: 11})
	led2.Record(r.Events)
	led2.Record(m.Tick(&s, 40))
	res := aid(t, m, &s, AidAct{Kind: "tourniquet", Wound: r.Wound.ID, Item: "item.tq", By: "char.hero", Force: true})
	led2.Record(res.Events)
	led2.Record(m.Tick(&s, 86400))
	if s.Dead || led2.Count("char.hero") != 0 || !contains(s.Wounds[0].Aiders, "char.hero") {
		t.Fatalf("the hero saved him: dead=%v kills=%d", s.Dead, led2.Count("char.hero"))
	}
	// self-inflicted (a hacked needle gun) is not a kill of anybody
	var led3 Ledger
	o := m.NewBody("char.hero", "bronze")
	r = hit(t, m, &o, Hit{Author: "char.hero", Weapon: "item.knife", Zone: "groin", Depth: "deep", Structures: []string{"groin_artery"}, Seed: 12})
	led3.Record(m.Tick(&o, 3600))
	if !o.Dead || led3.Count("char.hero") != 0 {
		t.Fatal("a self-inflicted death counted as a kill")
	}
}

// The capsule heals a fracture and a poisoning for CRD; short of CRD it only stabilises; no chip — refused.
func TestCapsuleHealsFractureAndPoison(t *testing.T) {
	m := mustModel(t)
	b := m.NewBody("char.hero", "bronze")
	hit(t, m, &b, Hit{Author: "npc.guard", Weapon: "item.baton", Zone: "arm", Depth: "deep", Structures: []string{"arm_bone"}, Seed: 21})
	if _, err := m.ApplyCondition(&b, "poison.ration", "npc.cook", 21); err != nil {
		t.Fatal(err)
	}
	if st := m.Status(b); st.HoldWeapon {
		t.Fatal("a broken arm still holds a weapon")
	}
	if q, _ := m.Quote(&b, "capsule", Payer{Chip: "none", BalanceMinor: 100000}); q.Mode != "refused" || q.Reason != "wounds.care.no_chip" {
		t.Fatalf("no chip: %+v", q)
	}
	if q, _ := m.Quote(&b, "capsule", Payer{Chip: "foreign", BalanceMinor: 100000}); q.Mode != "refused" {
		t.Fatalf("a foreign binding: %+v", q)
	}
	q, err := m.Quote(&b, "capsule", Payer{Chip: "own", BalanceMinor: 2000})
	if err != nil || q.Mode != "full" || q.PriceMinor <= 1000 || q.PriceMinor > 2000 || q.Seconds < 6*3600 {
		t.Fatalf("full treatment of a fracture and a poisoning costs more than the hero's 10 CRD: %+v %v", q, err)
	}
	balance := int64(2000) - q.PriceMinor // the host debits the quote
	if _, err := m.StartCare(&b, q, 5); err != nil {
		t.Fatal(err)
	}
	ev := m.Tick(&b, q.Seconds)
	if len(b.Wounds) != 0 || len(b.Conditions) != 0 || b.Care != nil || b.Dead || balance >= 1000 {
		t.Fatalf("after the capsule: wounds %d conditions %d care %v dead %v", len(b.Wounds), len(b.Conditions), b.Care, b.Dead)
	}
	if !strings.Contains(asJSON(t, ev), "care_done") {
		t.Fatal("no care_done event")
	}
	// short of money: stabilisation stops the bleeding and the shock, leaves the fracture and the poison
	p := m.NewBody("char.hero", "bronze")
	r := hit(t, m, &p, Hit{Author: "npc.x", Weapon: "item.knife", Zone: "thigh", Depth: "deep", Structures: []string{"femoral_artery", "femur"}, Seed: 22})
	if _, err := m.ApplyCondition(&p, "poison.ration", "npc.cook", 22); err != nil {
		t.Fatal(err)
	}
	m.Tick(&p, 150)
	q, _ = m.Quote(&p, "capsule", Payer{Chip: "own", BalanceMinor: 50})
	if q.Mode != "stabilize" || q.PriceMinor != 0 || q.FullPriceMinor <= 50 || q.Reason != "wounds.care.stabilize" {
		t.Fatalf("short of CRD: %+v", q)
	}
	if len(q.Journal) != 1 || !strings.HasPrefix(q.Journal[0], "stab:") {
		t.Fatalf("a stab wound must be journalled: %v", q.Journal)
	}
	if _, err := m.StartCare(&p, q, 6); err != nil {
		t.Fatal(err)
	}
	m.Tick(&p, q.Seconds)
	st := m.Status(p)
	if p.Dead || st.ExternalULS != 0 || st.InternalULS != 0 || p.LostPermille() > 150 || len(p.Wounds) != 1 || len(p.Conditions) != 1 || st.CanWalk {
		t.Fatalf("stabilised: %+v wounds %d conditions %d", st, len(p.Wounds), len(p.Conditions))
	}
	_ = r
	m.Tick(&p, 2*86400)
	if !p.Dead || p.Culprit != "npc.cook" {
		t.Fatalf("an untreated poisoning is lethal: dead %v culprit %q", p.Dead, p.Culprit)
	}
	// the dead are not raised
	if q, _ := m.Quote(&p, "capsule", Payer{Chip: "own", BalanceMinor: 1 << 30}); q.Reason != "wounds.care.dead" {
		t.Fatalf("a capsule raised the dead: %+v", q)
	}
}

// A medkit stops what bleeds outside and splints; it closes a wound without organs or bones, never a fracture.
func TestMedkitLevels(t *testing.T) {
	m := mustModel(t)
	b := m.NewBody("v", "bronze")
	r := hit(t, m, &b, Hit{Weapon: "item.baton", Zone: "thigh", Depth: "deep", Structures: []string{"femur"}, Seed: 31})
	if m.Status(b).Standing {
		t.Fatal("a broken femur stands")
	}
	res := aid(t, m, &b, AidAct{Kind: "medkit", Wound: r.Wound.ID, Item: "item.kit2", Force: true})
	if !res.Splinted || res.Treated || b.Wounds[0].Treated {
		t.Fatalf("a kit splints a fracture and does not heal it: %+v", res)
	}
	if st := m.Status(b); !st.CanWalk || st.PaceMMS > 400 {
		t.Fatalf("splinted: hobbles to the medblok: %+v", st)
	}
	v := m.NewBody("v2", "bronze")
	r = hit(t, m, &v, Hit{Weapon: "item.knife", Zone: "thigh", Depth: "deep", Structures: []string{"thigh_muscle", "thigh_vein"}, Seed: 32})
	if res := aid(t, m, &v, AidAct{Kind: "medkit", Wound: r.Wound.ID, Item: "item.kit1", Force: true}); res.Treated {
		t.Fatal("a household kit closed a deep wound")
	}
	if res := aid(t, m, &v, AidAct{Kind: "medkit", Wound: r.Wound.ID, Item: "item.kit2", Force: true}); !res.Treated || m.Status(v).ExternalULS != 0 {
		t.Fatalf("a guard kit closes a deep vein: %+v", res)
	}
	o := m.NewBody("v3", "bronze")
	r = hit(t, m, &o, Hit{Weapon: "item.knife", Zone: "abdomen", Depth: "deep", Structures: []string{"belly_wall", "liver"}, Seed: 33})
	aid(t, m, &o, AidAct{Kind: "medkit", Wound: r.Wound.ID, Item: "item.kit2", Force: true})
	if o.Wounds[0].Treated || m.Status(o).InternalULS == 0 {
		t.Fatal("a kit healed a liver")
	}
	if f := m.Forecast(o, Horizon); f.DeadS < 3600 || f.DeadS > 4*3600 {
		t.Fatalf("a liver kills in hours without a capsule: %+v", f)
	}
}

// Brain perfusion: a cut carotid takes consciousness in seconds; a chokehold held on kills more often.
func TestSecondsAndChokehold(t *testing.T) {
	m := mustModel(t)
	b := m.NewBody("v", "none")
	hit(t, m, &b, Hit{Weapon: "item.knife", Mode: "quick", Zone: "neck", Depth: "deep", Structures: []string{"carotid"}, Seed: 41})
	if f := m.Forecast(b, 3600); f.DownS < 8 || f.DownS > 15 || f.DeadS > 300 {
		t.Fatalf("carotid: %+v", f)
	}
	deaths := func(hold int64) int {
		n := 0
		for s := int64(0); s < 1000; s++ {
			c := m.NewBody("v", "none")
			hit(t, m, &c, Hit{Author: "char.hero", Technique: "chokehold", HoldS: hold, Seed: s})
			m.Tick(&c, 600)
			if c.Dead {
				n++
			}
		}
		return n
	}
	short, long := deaths(12), deaths(60)
	if short > 20 || long < 300 {
		t.Fatalf("chokehold deaths per 1000: released at 12 s %d, held 60 s %d", short, long)
	}
	c := m.NewBody("v", "none")
	hit(t, m, &c, Hit{Technique: "chokehold", HoldS: 14, Seed: 3})
	if st := m.Status(c); st.Standing {
		t.Fatal("held in a choke and standing")
	}
}

// The fight stops at the first serious wound by character; a man who cannot walk does not run.
func TestStopFight(t *testing.T) {
	m := mustModel(t)
	b := m.NewBody("v", "bronze")
	hit(t, m, &b, Hit{Weapon: "item.knife", Zone: "arm", Depth: "graze", Structures: []string{"arm_muscle"}, Seed: 1})
	if r := m.StopFight(b, Psyche{Response: "flight", FearPermille: 900}, 1); r.Kind != "fight_on" {
		t.Fatalf("a scratch is not serious: %+v", r)
	}
	counts := map[string]int{}
	for s := int64(0); s < 400; s++ {
		c := m.NewBody("v", "bronze")
		hit(t, m, &c, Hit{Weapon: "item.knife", Zone: "thigh", Depth: "deep", Structures: []string{"thigh_vein"}, Seed: s})
		counts[m.StopFight(c, Psyche{Response: "flight", FearPermille: 500, Courage: 30}, s).Kind]++
	}
	if counts["flee"] < 200 || counts["fight_on"] > 60 {
		t.Fatalf("a flight character mostly runs: %v", counts)
	}
	f := m.NewBody("v", "bronze")
	hit(t, m, &f, Hit{Weapon: "item.baton", Zone: "thigh", Depth: "deep", Structures: []string{"femur"}, Seed: 2})
	for s := int64(0); s < 50; s++ {
		if r := m.StopFight(f, Psyche{Response: "flight", FearPermille: 900}, s); r.Kind == "flee" {
			t.Fatal("ran on a broken femur")
		}
	}
}

// The hero sees his wounds and the time; others see only blood, a limp, paleness. Platinum is a story node.
func TestViewsAndPlatinum(t *testing.T) {
	m := mustModel(t)
	b := m.NewBody("char.hero", "bronze")
	hit(t, m, &b, Hit{Weapon: "item.knife", Zone: "thigh", Depth: "deep", Structures: []string{"thigh_muscle", "femoral_artery"}, Seed: 51})
	v := m.ViewSelf(b, 3600)
	if v.ForecastKey != "wounds.forecast.down" || v.Minutes < 3 || v.Minutes > 6 || !v.Lines[0].Danger || !contains(v.Lines[0].Structures, "s.fa") {
		t.Fatalf("self view: %+v", v)
	}
	other := m.ViewOther(b)
	if !contains(other, "wounds.sign.blood_spurt") || strings.Contains(strings.Join(other, ","), "s.fa") {
		t.Fatalf("outside view: %v", other)
	}
	o := m.NewBody("v", "bronze")
	hit(t, m, &o, Hit{Weapon: "item.knife", Zone: "abdomen", Depth: "deep", Structures: []string{"belly_wall", "liver"}, Seed: 52})
	if v := m.ViewSelf(o, Horizon); !contains(v.Lines[0].Structures, "wounds.wound.inside") || contains(v.Lines[0].Structures, "s.liver") || v.ForecastKey != "wounds.forecast.hours" {
		t.Fatalf("an organ shows as «inside» with hours: %+v", v)
	}
	p := m.NewBody("char.platinum", "platinum")
	hit(t, m, &p, Hit{Author: "char.hero", Weapon: "item.knife", Zone: "thigh", Depth: "deep", Structures: []string{"femoral_artery"}, Seed: 53})
	ev := m.Tick(&p, 3600)
	if p.Dead || !p.Story || !strings.Contains(asJSON(t, ev), "story_node") {
		t.Fatalf("platinum: dead %v story %v", p.Dead, p.Story)
	}
	// a bronze chip reports the shock once
	if n := strings.Count(asJSON(t, m.Tick(&b, 600)), "chip_alarm"); n != 1 {
		t.Fatalf("chip alarms: %d", n)
	}
}

// Armour changes chances and depth, never divides damage: a jacket stops some slashes outright.
func TestArmorStopsOrShallows(t *testing.T) {
	m := mustModel(t)
	blocked, shallower := 0, 0
	for s := int64(0); s < 500; s++ {
		b := m.NewBody("v", "none")
		r := hit(t, m, &b, Hit{Weapon: "item.knife", Mode: "quick", Zone: "arm", Depth: "shallow", Armor: []string{"item.jacket"}, Seed: s})
		if r.Blocked {
			blocked++
		} else if r.Wound.Depth == "shallow" {
			continue
		} else {
			shallower++
		}
	}
	if blocked < 150 || blocked > 250 || shallower != 0 {
		t.Fatalf("cut through a jacket: blocked %d of 500 (≈40 %%), shifted %d (cut has no shift)", blocked, shallower)
	}
}

func TestAidCannotWasteAnotherIdenticalControl(t *testing.T) {
	m := fixture()
	b := m.NewBody("v", "none")
	_, err := m.ApplyHit(&b, Hit{Weapon: "item.knife", Zone: "thigh", Depth: "deep", Structures: []string{"femoral_artery"}, Seed: 42})
	if err != nil {
		t.Fatal(err)
	}
	first, err := m.ApplyAid(&b, AidAct{Kind: "tourniquet", Item: "item.tq", Wound: b.Wounds[0].ID, By: "v", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if !first.Applied {
		t.Fatalf("fixture tourniquet: %+v", first)
	}
	second, err := m.ApplyAid(&b, AidAct{Kind: "tourniquet", Item: "item.tq", Wound: b.Wounds[0].ID, By: "v", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if second.Spent || second.Applied || second.Reason != "wounds.aid.nothing" {
		t.Fatalf("identical control wasted supply: %+v", second)
	}
}

func TestPairedLimbGripAndStrictLoad(t *testing.T) {
	m := fixture()
	m.Limbs = map[string]Limb{"right": {Name: "right", Zone: "arm", Slot: "hand.main"}, "left": {Name: "left", Zone: "arm", Slot: "hand.off"}}
	b := m.NewBody("person", "none")
	_, err := m.ApplyHit(&b, Hit{Mechanism: "blunt", Zone: "arm", Limb: "left", Depth: "deep", Structures: []string{"arm_bone"}, Seed: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !m.Status(b).HoldWeapon {
		t.Fatal("off hand injury disabled healthy armed limb")
	}
	b.Wounds[0].Limb = "right"
	if m.Status(b).HoldWeapon {
		t.Fatal("armed limb fracture did not disable grip")
	}
	b.Wounds[0].Limb = "tail"
	if m.ValidateBody(b) == nil {
		t.Fatal("unknown limb loaded")
	}
}
func TestRepeatedBluntChannelsAccumulateOnlySameZoneAndWindow(t *testing.T) {
	m := fixture()
	m.RepeatBluntWindowS = 60
	m.RepeatBluntHits = 2
	m.Weapons[0].Modes["*"] = Mode{Mechanism: "blunt", Depth: map[string]int64{"graze": 1}}
	b := m.NewBody("p", "none")
	for i := 0; i < 5; i++ {
		res, err := m.ApplyHit(&b, Hit{Weapon: m.Weapons[0].Weapon, Part: "arms", Zone: "arm", Seed: int64(i)})
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"graze", "graze", "shallow", "shallow", "deep"}[i]
		if res.Wound.Depth != want {
			t.Fatalf("hit %d: %s, want %s", i, res.Wound.Depth, want)
		}
	}
	b.Clock = 61
	res, err := m.ApplyHit(&b, Hit{Weapon: m.Weapons[0].Weapon, Zone: "arm", Seed: 8})
	if err != nil || res.Wound.Depth != "graze" {
		t.Fatal("expired impacts accumulated")
	}
}

// Medical severity must not automatically remove a still functional armed body
// from contact. Grip loss and hypovolaemia remain anatomical vetoes.
func TestArmedContactBluntTorsoResolve(t *testing.T) {
	m := mustModel(t)
	m.Resolve.ContactBluntParts = []string{"torso"}
	b := m.NewBody("v", "none")
	hit(t, m, &b, Hit{Weapon: "item.baton", Zone: "abdomen", Depth: "deep", Structures: []string{"liver"}, Seed: 1})
	if !m.Serious(b) {
		t.Fatal("medical seriousness was lost")
	}
	for seed := int64(1); seed <= 100; seed++ {
		if got := m.StopFight(b, Psyche{ArmedContact: true, Response: "flight", FearPermille: 1000}, seed); got.Kind != "fight_on" {
			t.Fatalf("functional contact ended: %+v", got)
		}
	}
	hit(t, m, &b, Hit{Weapon: "item.baton", Zone: "arm", Depth: "deep", Structures: []string{"arm_bone"}, Seed: 2})
	for seed := int64(1); seed <= 100; seed++ {
		if got := m.StopFight(b, Psyche{ArmedContact: true, Response: "flight", FearPermille: 1000}, seed); got.Kind == "fight_on" {
			t.Fatalf("lost grip ignored: %+v", got)
		}
	}
}
