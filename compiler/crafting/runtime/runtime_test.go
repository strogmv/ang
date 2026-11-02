package crafting

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func fixture() Model {
	tiers := []Tier{{Tier: 1, Points: 0, Name: "t1"}, {Tier: 2, Points: 15, Name: "t2"}, {Tier: 3, Points: 35, Name: "t3"}, {Tier: 4, Points: 60, Name: "t4"}, {Tier: 5, Points: 85, Name: "t5"}}
	src := []SourceRef{{Path: "docs/world-sim/93-crafting-schematics.md", Note: "proposal"}}
	return Model{
		Level: "level.test",
		Parts: []PartClass{
			{ID: "cell", Name: "p.cell", Items: []PartItem{{Item: "item.cell", Origin: "handmade", Quality: 2}, {Item: "item.sys_cell", Origin: "system", Quality: 3}}},
			{ID: "housing", Name: "p.housing", Items: []PartItem{{Item: "item.part.housing", Origin: "handmade", Quality: 2}}},
			{ID: "board", Name: "p.board", Items: []PartItem{{Item: "item.part.board", Origin: "system", Quality: 3}}},
		},
		Disciplines: []Discipline{{ID: "electrics", Name: "d.e", Skills: []string{"maintenance"}, Tiers: tiers}},
		Tools:       []ToolDef{{Item: "item.tool.iron", Kind: "soldering_iron", Quality: 2, WearPPM: 20000}},
		Stations:    []Station{{ID: "bench.m", Name: "s.m", Kind: "soldering", Owner: "char.m", Level: "level.test", Reach: 2, Tools: []ToolUse{{Kind: "soldering_iron", Quality: 4}}, RentMinorPerHour: 50, Account: "acc.m"}},
		Recipes: []Recipe{
			{ID: "craft.block", Kind: "craft", Name: "r.block", Hint: "h", Discipline: "electrics", Tier: 2, Inputs: []Input{{Class: "cell", N: 4, MinQuality: 2}, {Class: "housing", N: 1, MinQuality: 1}, {Class: "board", N: 1, MinQuality: 1}}, Tools: []string{"soldering_iron"}, Minutes: 40, Base: 3, RiskPermille: 120, Legal: "legal", Basis: "proposal", Output: &Output{Item: "item.block", N: 1, Origin: "handmade"}, Learn: []Learn{{Kind: "tier"}}, Sources: src},
			{ID: "craft.lamp", Kind: "craft", Name: "r.lamp", Hint: "h", Discipline: "electrics", Tier: 3, Inputs: []Input{{Class: "housing", N: 1, MinQuality: 1}, {Class: "cell", N: 1, MinQuality: 1}}, Station: "soldering", Minutes: 60, Base: 3, RiskPermille: 100, Legal: "legal", Basis: "proposal", Output: &Output{Item: "item.lamp", N: 1, Origin: "handmade"}, Learn: []Learn{{Kind: "teardown", From: "item.lamp"}}, Sources: src},
			{ID: "repair.radio", Kind: "repair", Name: "r.radio", Hint: "h", Discipline: "electrics", Tier: 2, Target: "item.radio", Inputs: []Input{{Class: "board", N: 1, MinQuality: 1}}, Tools: []string{"soldering_iron"}, Minutes: 30, Base: 3, RestorePPM: 900000, RiskPermille: 150, Legal: "legal", Basis: "proposal", Learn: []Learn{{Kind: "start"}}, Heirloom: true, Sources: src},
			{ID: "disassemble.lamp", Kind: "disassemble", Name: "r.dl", Hint: "h", Discipline: "electrics", Tier: 1, Target: "item.lamp", Minutes: 15, Base: 3, RiskPermille: 100, Legal: "legal", Basis: "proposal", Yields: []Yield{{Class: "housing", Min: 1, Max: 1, Quality: 2, Origin: "handmade"}, {Class: "cell", Min: 0, Max: 1, Quality: 2, Origin: "handmade"}}, Teaches: "craft.lamp", TeachPermille: 1000, Learn: []Learn{{Kind: "tier"}}, Sources: src},
		},
		Masters:   []Master{{Actor: "char.m", Name: "m", Shop: "shop.m", Station: "bench.m", Level: "level.test", Reach: 2, Points: map[string]int64{"electrics": 70}, Offers: []string{"craft.block", "repair.radio"}, Known: []string{"repair.radio"}, LaborMinorPerHour: 60, Stores: []string{"stock.m"}, OrderStore: "orders.m", Account: "acc.m"}},
		Hero:      Hero{Actor: "char.hero", Points: map[string]int64{"electrics": 15}, Known: []string{"repair.radio"}, WorkStore: "work.hero", Account: "acc.hero"},
		Knowledge: []Knowledge{{Kind: "tier", Name: "k.tier", Executable: true}},
		Tuning: Tuning{GainMilli: map[string]int64{"craft": 4000, "repair": 3000, "maintain": 1000, "disassemble": 2000}, FailureGainPermille: 500, CapPoints: 100, PartWeight: 60, SkillWeight: 2, ToolWeight: 30,
			RiskPerPoint: 3, RiskPerTool: 15, MinRisk: 20, MaxRisk: 800, CriticalPermille: 250, RestorePerQuality: 30000, CriticalDamagePPM: 100000, SurvivePermille: 700, SurvivePerPoint: 5, WornBelowPPM: 500000},
		Prices: map[string]int64{"item.cell": 50, "item.sys_cell": 80, "item.part.housing": 60, "item.part.board": 120, "item.block": 450},
	}
}

func heroWorker(m Model) Worker {
	s := m.NewState()
	return s.Actors["char.hero"].Worker("char.hero")
}

func bag() []Stack {
	return []Stack{
		{ID: "a", Definition: "item.cell", Quantity: 3, Condition: Million},
		{ID: "b", Definition: "item.sys_cell", Quantity: 2, Condition: Million},
		{ID: "c", Definition: "item.part.housing", Quantity: 1, Condition: Million, Quality: 4, Origin: "prewar"},
		{ID: "d", Definition: "item.part.board", Quantity: 2, Condition: Million},
		{ID: "t", Definition: "item.tool.iron", Quantity: 1, Condition: Million},
	}
}

func TestCatalogValidates(t *testing.T) {
	if err := fixture().Validate(); err != nil {
		t.Fatal(err)
	}
	broken := fixture()
	broken.Recipes[0].Inputs[0].Class = "missing"
	if broken.Validate() == nil {
		t.Fatal("unknown part class accepted")
	}
	broken = fixture()
	broken.Masters[0].Offers = []string{"craft.lamp"}
	if broken.Validate() == nil {
		t.Fatal("a master offers a schematic he does not know")
	}
}

func TestCraftPlanUsesExactlyTheInputsAndIsDeterministic(t *testing.T) {
	m := fixture()
	w := heroWorker(m)
	p, err := m.Plan(w, Request{Recipe: "craft.block", JobID: "job.1", Seed: 7}, bag(), nil, Bench{})
	if err != nil {
		t.Fatal(err)
	}
	units := map[string]int64{}
	for _, u := range p.Uses {
		cls, _, _ := m.PartOf(u.Definition)
		units[cls.ID] += u.Quantity
	}
	if !reflect.DeepEqual(units, map[string]int64{"cell": 4, "housing": 1, "board": 1}) {
		t.Fatalf("plan does not use exactly the schematic inputs: %v", units)
	}
	// the best parts first: both system cells (q3) before the handmade ones (q2)
	if p.Uses[0].Stack != "b" || p.Uses[0].Quantity != 2 || p.Uses[1].Stack != "a" || p.Uses[1].Quantity != 2 {
		t.Fatalf("parts are not taken best-first: %+v", p.Uses)
	}
	if len(p.Wear) != 1 || p.Wear[0].Stack != "t" || p.Seconds != 2400 || p.Station != "" || p.RentMinor != 0 {
		t.Fatalf("carried soldering iron, time or rent wrong: %+v", p)
	}
	again, _ := m.Plan(w, Request{Recipe: "craft.block", JobID: "job.1", Seed: 7}, bag(), nil, Bench{})
	if !reflect.DeepEqual(p, again) {
		t.Fatal("same seed and job id gave a different plan")
	}
	outcomes := map[string]int{}
	for i := 0; i < 400; i++ {
		x, _ := m.Plan(w, Request{Recipe: "craft.block", JobID: fmt.Sprintf("job.%d", i), Seed: 7}, bag(), nil, Bench{})
		outcomes[x.Outcome]++
		if x.Outcome == "success" && (len(x.Makes) != 1 || x.Makes[0].Definition != "item.block" || len(x.Spoil) != 0) {
			t.Fatal("success must make the output and spoil nothing")
		}
		if x.Outcome != "success" && (len(x.Makes) != 0 || len(x.Spoil) == 0) {
			t.Fatal("failure must make nothing and spoil parts")
		}
	}
	// risk 120‰ at the tier threshold with a q2 tool: (120+15) = 135‰ ± noise
	if fail := outcomes["failure"] + outcomes["critical"]; fail < 30 || fail > 85 {
		t.Fatalf("failure rate off the rule: %v", outcomes)
	}
}

func TestRefusalsWithoutSchematicToolPartsOrStation(t *testing.T) {
	m := fixture()
	w := heroWorker(m)
	key := func(err error) string {
		var r Refusal
		if errors.As(err, &r) {
			return r.Key
		}
		return ""
	}
	if _, err := m.Plan(w, Request{Recipe: "craft.lamp", JobID: "x"}, bag(), nil, Bench{}); key(err) != "crafting.error.unknown_schematic" {
		t.Fatalf("unknown schematic: %v", err)
	}
	w.Known["craft.lamp"] = true
	if _, err := m.Plan(w, Request{Recipe: "craft.lamp", JobID: "x"}, bag(), nil, Bench{}); key(err) != "crafting.error.tier" {
		t.Fatalf("tier: %v", err)
	}
	noTool := bag()[:4]
	if _, err := m.Plan(w, Request{Recipe: "craft.block", JobID: "x"}, noTool, nil, Bench{}); key(err) != "crafting.error.no_tool" {
		t.Fatalf("no tool: %v", err)
	}
	if _, err := m.Plan(w, Request{Recipe: "craft.block", JobID: "x"}, bag()[1:], nil, Bench{}); key(err) != "crafting.error.no_parts" {
		t.Fatalf("no parts: %v", err)
	}
	// the master's station lends a better iron for rent; closed, it lends nothing
	p, err := m.Plan(w, Request{Recipe: "craft.block", JobID: "x"}, noTool, nil, Bench{Station: "bench.m", Present: true})
	if err != nil || p.Station != "bench.m" || p.RentMinor != 50 || len(p.Wear) != 0 {
		t.Fatalf("station tools / rent: %+v %v", p, err)
	}
	if _, err := m.Plan(w, Request{Recipe: "craft.block", JobID: "x"}, noTool, nil, Bench{Station: "bench.m", Present: false}); key(err) != "crafting.error.no_tool" {
		t.Fatalf("closed station: %v", err)
	}
	radio := &Stack{ID: "r", Definition: "item.radio", Quantity: 1, Condition: Million}
	if _, err := m.Plan(w, Request{Recipe: "repair.radio", JobID: "x", Target: "r"}, bag(), radio, Bench{}); key(err) != "crafting.error.intact" {
		t.Fatalf("intact target: %v", err)
	}
}

func TestProgressionOpensTheNextTierAndTeardownTeaches(t *testing.T) {
	m := fixture()
	s := m.NewState()
	hero := s.Actors["char.hero"]
	w := hero.Worker("char.hero")
	if m.TierOf(w, "electrics") != 2 {
		t.Fatal("15 points must be tier 2")
	}
	lamp := &Stack{ID: "l", Definition: "item.lamp", Quantity: 1, Condition: 300000}
	var taught bool
	for i := 0; i < 20 && m.TierOf(hero.Worker("char.hero"), "electrics") < 3; i++ {
		p, err := m.Plan(hero.Worker("char.hero"), Request{Recipe: "disassemble.lamp", JobID: fmt.Sprintf("job.%d", i), Seed: 3, Target: "l"}, nil, lamp, Bench{})
		if err != nil {
			t.Fatal(err)
		}
		for _, mk := range p.Makes {
			if mk.Quality != 1 || mk.Quantity > 1 {
				t.Fatalf("a worn sample must yield worn parts within the yield: %+v", mk)
			}
		}
		taught = taught || p.Learn == "craft.lamp"
		m.Award(&hero, "electrics", p)
	}
	if !taught || !hero.Known["craft.lamp"] {
		t.Fatal("taking the lamp apart never taught its schematic")
	}
	if m.TierOf(hero.Worker("char.hero"), "electrics") != 3 {
		t.Fatalf("points %d did not open tier 3", hero.Points["electrics"])
	}
	if !m.Knows(hero.Worker("char.hero"), m.Recipes[1]) {
		t.Fatal("learnt schematic is not known")
	}
	hero.Points["electrics"] = 1000000
	m.Award(&hero, "electrics", Plan{GainMilli: 5000})
	if hero.Points["electrics"] != 100000 {
		t.Fatal("points above the cap")
	}
}

func TestOrderPriceAndStateRoundTrip(t *testing.T) {
	m := fixture()
	ms, _ := m.Master("char.m")
	mw := m.MasterWorker(ms)
	stock := bag()
	p, err := m.Plan(mw, Request{Recipe: "craft.block", JobID: "job.9", Seed: 1}, stock, nil, Bench{Station: "bench.m", Present: true})
	if err != nil || p.RentMinor != 0 {
		t.Fatalf("the master pays no rent at the own bench: %+v %v", p, err)
	}
	// parts 2×80 + 2×50 + 60 + 120 = 440, labour 40 min at 60/h = 40
	if price := m.OrderPrice(ms, p); price != 480 {
		t.Fatalf("order price %d", price)
	}
	s := m.NewState()
	s.Jobs = append(s.Jobs, Job{ID: "job.9", Worker: "char.m", Client: "char.hero", Plan: p, Store: "orders.m", Due: 2400})
	if err := m.ValidateState(s); err != nil {
		t.Fatal(err)
	}
	c := s.Clone()
	c.Jobs[0].Plan.Uses[0].Quantity = 99
	c.Actors["char.hero"].Points["electrics"] = 0
	if s.Jobs[0].Plan.Uses[0].Quantity == 99 || s.Actors["char.hero"].Points["electrics"] == 0 {
		t.Fatal("clone shares memory with the state")
	}
	s.Jobs = append(s.Jobs, Job{ID: "job.10", Worker: "char.hero", Plan: Plan{Recipe: "craft.block", Kind: "craft", Worker: "char.hero", Seconds: 2400}, Store: "work.hero"}, Job{ID: "job.11", Worker: "char.hero", Plan: Plan{Recipe: "craft.block", Kind: "craft", Worker: "char.hero", Seconds: 2400}, Store: "work.hero"})
	if m.ValidateState(s) == nil {
		t.Fatal("two own jobs of the hero accepted")
	}
}
