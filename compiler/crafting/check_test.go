package crafting

import (
	"strings"
	"testing"

	runtime "github.com/strogmv/ang/compiler/crafting/runtime"
)

func fixture() runtime.Model {
	tiers := []runtime.Tier{{Tier: 1, Points: 0, Name: "t1"}, {Tier: 2, Points: 15, Name: "t2"}, {Tier: 3, Points: 35, Name: "t3"}, {Tier: 4, Points: 60, Name: "t4"}, {Tier: 5, Points: 85, Name: "t5"}}
	src := []runtime.SourceRef{{Path: "docs/world-sim/93-crafting-schematics.md", Note: "proposal"}}
	return runtime.Model{
		Level: "level.test",
		Parts: []runtime.PartClass{
			{ID: "cell", Name: "p.cell", Items: []runtime.PartItem{{Item: "item.cell", Origin: "handmade", Quality: 2}, {Item: "item.sys_cell", Origin: "system", Quality: 3}}},
			{ID: "housing", Name: "p.housing", Items: []runtime.PartItem{{Item: "item.part.housing", Origin: "handmade", Quality: 2}}},
			{ID: "board", Name: "p.board", Items: []runtime.PartItem{{Item: "item.part.board", Origin: "system", Quality: 3}}},
		},
		Disciplines: []runtime.Discipline{{ID: "electrics", Name: "d.e", Skills: []string{"maintenance"}, Tiers: tiers}},
		Tools:       []runtime.ToolDef{{Item: "item.tool.iron", Kind: "soldering_iron", Quality: 2, WearPPM: 20000}},
		Stations:    []runtime.Station{{ID: "bench.m", Name: "s.m", Kind: "soldering", Owner: "char.m", Level: "level.test", Reach: 2, Tools: []runtime.ToolUse{{Kind: "soldering_iron", Quality: 4}}, RentMinorPerHour: 50, Account: "acc.m"}},
		Recipes: []runtime.Recipe{
			{ID: "craft.block", Kind: "craft", Name: "r.block", Hint: "h", Discipline: "electrics", Tier: 2, Inputs: []runtime.Input{{Class: "cell", N: 4, MinQuality: 2}, {Class: "housing", N: 1, MinQuality: 1}, {Class: "board", N: 1, MinQuality: 1}}, Tools: []string{"soldering_iron"}, Minutes: 40, Base: 3, RiskPermille: 120, Legal: "legal", Basis: "proposal", Output: &runtime.Output{Item: "item.block", N: 1, Origin: "handmade"}, Learn: []runtime.Learn{{Kind: "tier"}}, Sources: src},
			{ID: "craft.lamp", Kind: "craft", Name: "r.lamp", Hint: "h", Discipline: "electrics", Tier: 3, Inputs: []runtime.Input{{Class: "housing", N: 1, MinQuality: 1}, {Class: "cell", N: 1, MinQuality: 1}}, Station: "soldering", Minutes: 60, Base: 3, RiskPermille: 100, Legal: "legal", Basis: "proposal", Output: &runtime.Output{Item: "item.lamp", N: 1, Origin: "handmade"}, Learn: []runtime.Learn{{Kind: "teardown", From: "item.lamp"}}, Sources: src},
			{ID: "repair.radio", Kind: "repair", Name: "r.radio", Hint: "h", Discipline: "electrics", Tier: 2, Target: "item.radio", Inputs: []runtime.Input{{Class: "board", N: 1, MinQuality: 1}}, Tools: []string{"soldering_iron"}, Minutes: 30, Base: 3, RestorePPM: 900000, RiskPermille: 150, Legal: "legal", Basis: "proposal", Learn: []runtime.Learn{{Kind: "start"}}, Heirloom: true, Sources: src},
			{ID: "disassemble.lamp", Kind: "disassemble", Name: "r.dl", Hint: "h", Discipline: "electrics", Tier: 1, Target: "item.lamp", Minutes: 15, Base: 3, RiskPermille: 100, Legal: "legal", Basis: "proposal", Yields: []runtime.Yield{{Class: "housing", Min: 1, Max: 1, Quality: 2, Origin: "handmade"}, {Class: "cell", Min: 0, Max: 1, Quality: 2, Origin: "handmade"}}, Teaches: "craft.lamp", TeachPermille: 1000, Learn: []runtime.Learn{{Kind: "tier"}}, Sources: src},
		},
		Masters:   []runtime.Master{{Actor: "char.m", Name: "m", Shop: "shop.m", Station: "bench.m", Level: "level.test", Reach: 2, Points: map[string]int64{"electrics": 70}, Offers: []string{"craft.block", "repair.radio"}, Known: []string{"repair.radio"}, LaborMinorPerHour: 60, Stores: []string{"stock.m"}, OrderStore: "orders.m", Account: "acc.m"}},
		Hero:      runtime.Hero{Actor: "char.hero", Points: map[string]int64{"electrics": 15}, Known: []string{"repair.radio"}, WorkStore: "work.hero", Account: "acc.hero"},
		Knowledge: []runtime.Knowledge{{Kind: "tier", Name: "k.tier", Executable: true}},
		Tuning: runtime.Tuning{GainMilli: map[string]int64{"craft": 4000, "repair": 3000, "maintain": 1000, "disassemble": 2000}, FailureGainPermille: 500, CapPoints: 100, PartWeight: 60, SkillWeight: 2, ToolWeight: 30,
			RiskPerPoint: 3, RiskPerTool: 15, MinRisk: 20, MaxRisk: 800, CriticalPermille: 250, RestorePerQuality: 30000, CriticalDamagePPM: 100000, SurvivePermille: 700, SurvivePerPoint: 5, WornBelowPPM: 500000},
		Prices: map[string]int64{"item.cell": 50, "item.sys_cell": 80, "item.part.housing": 60, "item.part.board": 120, "item.block": 450},
	}
}

func world() World {
	return World{Items: map[string]WorldItem{
		"item.cell":         {PriceMinor: 50, StackLimit: 100, Sold: true},
		"item.sys_cell":     {PriceMinor: 80, StackLimit: 100, Sold: true},
		"item.part.housing": {PriceMinor: 60, StackLimit: 50, Sold: true},
		"item.part.board":   {PriceMinor: 120, StackLimit: 50, Sold: true},
		"item.block":        {PriceMinor: 450, StackLimit: 10},
		"item.lamp":         {PriceMinor: 400, StackLimit: 1, Sold: true},
		"item.radio":        {PriceMinor: 0, StackLimit: 1, Present: true},
		"item.tool.iron":    {PriceMinor: 400, StackLimit: 1, Sold: true},
	}, Localized: func(string) bool { return true }}
}

func TestContentChecksPassAndCatchEachViolation(t *testing.T) {
	if err := Check(fixture(), world()); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*runtime.Model, *World){
		"unreachable": func(m *runtime.Model, w *World) {
			it := w.Items["item.part.board"]
			it.Sold = false
			w.Items["item.part.board"] = it
		},
		"no CRD price": func(m *runtime.Model, w *World) {
			it := w.Items["item.cell"]
			it.PriceMinor = -1
			w.Items["item.cell"] = it
		},
		"worth less than its parts": func(m *runtime.Model, w *World) {
			it := w.Items["item.block"]
			it.PriceMinor = 100
			w.Items["item.block"] = it
		},
		"more than the thing": func(m *runtime.Model, w *World) { m.Recipes[3].Yields[0].Max = 9 },
		"copies parts":        func(m *runtime.Model, w *World) { m.Recipes[3].Yields[1].Max = 2 },
		"salvaged definition itself": func(m *runtime.Model, w *World) {
			m.Parts[0].Items = append(m.Parts[0].Items, runtime.PartItem{Item: "item.lamp", Origin: "handmade", Quality: 1})
		},
		"without a canon":  func(m *runtime.Model, w *World) { m.Recipes[0].Basis = "canon" },
		"must say so":      func(m *runtime.Model, w *World) { m.Recipes[0].Sources = []runtime.SourceRef{{Path: "x"}} },
		"localization key": func(m *runtime.Model, w *World) { w.Localized = func(k string) bool { return k != "r.lamp" } },
		"heirloom": func(m *runtime.Model, w *World) {
			it := w.Items["item.radio"]
			it.StackLimit = 5
			w.Items["item.radio"] = it
		},
		"unknown item": func(m *runtime.Model, w *World) { delete(w.Items, "item.block") },
	}
	for want, edit := range cases {
		m, w := fixture(), world()
		items := map[string]WorldItem{}
		for k, v := range w.Items {
			items[k] = v
		}
		w.Items = items
		edit(&m, &w)
		err := Check(m, w)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("%s: not reported: %v", want, err)
		}
	}
}
