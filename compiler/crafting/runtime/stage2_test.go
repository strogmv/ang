package crafting

import (
	"reflect"
	"testing"
)

// Stage 2 engine rules: copies and their errors, learning from a clean source, the law's curve, belief, reports, fines,
// sanctions, the market's scarcity price and the swap.
func stage2() Model {
	m := fixture()
	m.Sources = Sources{
		Carriers: []Carrier{{Item: "item.schematic.lamp", Kind: "copy", Teaches: "craft.lamp", StudyMinutes: 30, FlawPermille: 500, Place: CarrierPlace{Kind: "shop", Shop: "shop.m", N: 1}}},
		Flaws:    []Flaw{{Kind: "quality", Name: "f.q", QualityDelta: -1}, {Kind: "risk", Name: "f.r", RiskPermille: 150}},
		Teaching: []Teaching{{Master: "char.m", Recipe: "repair.radio", MinRelationPermille: 200, Hours: 2, FeeMinor: 100}},
	}
	m.Recipes[1].Learn = append(m.Recipes[1].Learn, Learn{Kind: "copy", From: "item.schematic.lamp"})
	m.Law = Law{KnownPermille: 700, SuspectedPermille: 300, DecayPPMPerDay: 20000, ReportDelaySeconds: 86400, Obliged: []string{"char.guard"},
		Crowd: []Point{{0, 0}, {200, 300}, {500, 700}, {1000, 950}}, CrowdPeak: 6, WitnessCells: 8,
		Acts: []Act{
			{ID: "hack", Crime: "crime.chip_bypass", Rung: "rung.grievous", Rank: 3, Reaction: "call_wide", Sanctions: []string{"account_freeze", "expulsion", "journal_entry"}, Evidence: "journal_entry", WeightPermille: 800, DetectPermille: 1000},
			{ID: "carry", Crime: "crime.chip_bypass", Rung: "rung.grievous", Rank: 3, Reaction: "call_wide", Sanctions: []string{"account_freeze"}, Evidence: "chip_log", WeightPermille: 1000, DetectPermille: 700},
			{ID: "scan", Crime: "crime.chip_bypass", Rung: "rung.grievous", Rank: 3, Reaction: "call_wide", Sanctions: []string{"account_freeze"}, Evidence: "chip_log", WeightPermille: 1000, DetectPermille: 900},
			{ID: "raid", Crime: "crime.chip_bypass", Rung: "rung.grievous", Rank: 3, Reaction: "call_wide", Sanctions: []string{"account_freeze"}, Evidence: "chip_log", WeightPermille: 1000, DetectPermille: 1000},
			{ID: "theft", Crime: "crime.petty_theft", Rung: "rung.petty_theft", Rank: 1, Reaction: "settle_or_call", Sanctions: []string{"ration_fine", "journal_entry"}, Evidence: "journal_entry", WeightPermille: 800, DetectPermille: 1000},
		},
		Ledger: Ledger{BaseMinor: 300, GrowthPermille: 1500, CapPermille: 8000},
		Trust:  Trust{StartPermille: 200, RebindMinPermille: 600, GainLegalJob: 10, GainCleanDay: 5, LossJournalEntry: 150},
		Rebind: Rebind{TerminalKind: "equipment", PriceMinor: 1500, Reach: 2, MisfirePermille: 150, InjurePermille: 60, Raid: Raid{EveryDays: 4, Hour: 21, Level: "level.test"}, ConfiscateStore: "locker"},
	}
	m.Economy = Economy{Scarcity: []Point{{0, 4000}, {250, 2200}, {500, 1500}, {1000, 1000}, {2000, 800}}, RiskPermille: map[string]int64{"legal": 1000, "grey": 1300, "forbidden": 2000}, Par: map[string]int64{"cell": 10, "board": 4}}
	m.Market = Market{Honesty: []Honesty{{Master: "char.m", SwapBelowPermille: 200, SwapPermille: 1000}}, Refund: Refund{WaitingPermille: 1000, WorkingPermille: 500}}
	return m
}

func TestStage2CatalogValidatesAndEverySourceRuns(t *testing.T) {
	m := stage2()
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	broken := stage2()
	broken.Recipes[1].Learn = append(broken.Recipes[1].Learn, Learn{Kind: "apprentice", From: "char.nobody"})
	if broken.Validate() == nil {
		t.Fatal("a learn row without a teacher accepted")
	}
	broken = stage2()
	broken.Law.Trust.RebindMinPermille = 100
	if broken.Validate() == nil {
		t.Fatal("a rebinding threshold below the starting trust accepted")
	}
}

func TestStage2CopyErrorIsPerSheetAndCleanSourcesCorrectIt(t *testing.T) {
	m := stage2()
	c := m.Sources.Carriers[0]
	flawed, clean := 0, 0
	for _, sheet := range []string{"s1", "s2", "s3", "s4", "s5", "s6", "s7", "s8", "s9", "s10", "s11", "s12"} {
		f := m.CopyFlaw(5, sheet, c)
		if f != m.CopyFlaw(5, sheet, c) {
			t.Fatal("a sheet's error is not stable")
		}
		if f == "" {
			clean++
		} else {
			flawed++
		}
	}
	if flawed == 0 || clean == 0 {
		t.Fatalf("a 50%% copy gave %d flawed and %d clean sheets", flawed, clean)
	}
	a := ActorState{}
	if !a.Learn("craft.lamp", "quality") || a.Flaws["craft.lamp"] != "quality" || a.Learn("craft.lamp", "risk") {
		t.Fatal("a flawed copy must enter memory once and not stack errors")
	}
	if !a.Learn("craft.lamp", "") || a.Flaws["craft.lamp"] != "" {
		t.Fatal("a clean source did not correct the error")
	}
	w := heroWorker(m)
	w.Known["craft.lamp"], w.Points["electrics"] = true, 40000
	stacks := []Stack{{ID: "h", Definition: "item.part.housing", Quantity: 1, Condition: Million}, {ID: "c", Definition: "item.cell", Quantity: 1, Condition: Million}}
	bench := Bench{Station: "bench.m", Present: true}
	honest, _ := m.Plan(w, Request{Recipe: "craft.lamp", JobID: "j", Seed: 1}, stacks, nil, bench)
	w.Flaws = map[string]string{"craft.lamp": "quality"}
	worse, _ := m.Plan(w, Request{Recipe: "craft.lamp", JobID: "j", Seed: 1}, stacks, nil, bench)
	if worse.Flaw != "quality" || worse.Quality != max(1, honest.Quality-1) {
		t.Fatalf("the error does not lower the build: %d vs %d", worse.Quality, honest.Quality)
	}
}

func TestStage2LawCurveBeliefReportsAndFines(t *testing.T) {
	m := stage2()
	if Curve(m.Law.Crowd, 100) != 150 || Curve(m.Law.Crowd, 5000) != 950 || Curve(m.Law.Crowd, -1) != 0 {
		t.Fatal("piecewise-linear curve")
	}
	c := Case{Evidence: []Evidence{{Kind: "journal_entry", Weight: 800, Second: 0}}}
	if m.Status(c, 0) != "known" || m.Status(c, 30*86400) == "known" {
		t.Fatalf("a report must be known at once and decay over a month (%d)", m.Belief(c, 30*86400))
	}
	weak := Case{Evidence: []Evidence{{Kind: "loot_provenance", Weight: 500, Second: 0}}}
	if m.Status(weak, 0) != "suspected" {
		t.Fatal("half a proof must be a suspicion")
	}
	act, _ := m.Act("hack")
	obliged := m.Reports(9, "case.1", act, []string{"char.guard"}, 100)
	if len(obliged) != 1 || obliged[0].Second != 100 {
		t.Fatal("an obliged witness must report at once")
	}
	crowd := []string{"a", "b", "c", "d", "e", "f"}
	r1, r2 := m.Reports(9, "case.1", act, crowd, 100), m.Reports(9, "case.1", act, crowd, 100)
	if !reflect.DeepEqual(r1, r2) || len(r1) == 0 || r1[0].Second != 100+m.Law.ReportDelaySeconds {
		t.Fatal("a crowd must report by the seed after the law's delay")
	}
	if m.Fine(1) != 300 || m.Fine(2) != 450 || m.Fine(20) != 2400 {
		t.Fatalf("ledger fines %d %d %d", m.Fine(1), m.Fine(2), m.Fine(20))
	}
	s := LawState{Trust: 200}
	if fine := m.Sanction(&s, act); fine != 0 || !s.Frozen || !s.Expulsion || !s.Wanted || s.Trust != 50 {
		t.Fatalf("top-rung sanctions not applied: %+v", s)
	}
	theft, _ := m.Act("theft")
	s = LawState{Trust: 200}
	if fine := m.Sanction(&s, theft); fine != 300 || s.Frozen || s.Offences != 1 {
		t.Fatalf("petty theft must fine by the ledger: %+v", s)
	}
}

func TestStage2ScarcityPriceAndSwap(t *testing.T) {
	m := stage2()
	ms := m.Masters[0]
	w := m.MasterWorker(ms)
	stacks := []Stack{{ID: "a", Definition: "item.cell", Quantity: 4, Condition: Million}, {ID: "h", Definition: "item.part.housing", Quantity: 1, Condition: Million},
		{ID: "d", Definition: "item.part.board", Quantity: 1, Condition: Million}, {ID: "e", Definition: "item.part.board", Quantity: 1, Condition: Million, Quality: 2, Origin: "handmade"}}
	q := Request{Recipe: "craft.block", JobID: "j", Seed: 3}
	bench := Bench{Station: "bench.m", Present: true}
	p, err := m.Plan(w, q, stacks, nil, bench)
	if err != nil {
		t.Fatal(err)
	}
	full := m.PriceOrder(ms, p, p.Uses, map[string]int64{"cell": 10, "board": 4})
	scarce := m.PriceOrder(ms, p, p.Uses, map[string]int64{"cell": 1, "board": 4})
	if scarce <= full {
		t.Fatalf("scarce cells did not raise the price: %d → %d", full, scarce)
	}
	worked, sw := m.SwapPlan(w, q, ms.Actor, 0, p, stacks, nil, bench)
	if sw.Class != "board" || sw.From != "system" {
		t.Fatalf("a master below his honesty must swap the system board: %+v", sw)
	}
	for _, u := range worked.Uses {
		if u.Stack == "d" {
			t.Fatal("the swapped plan still uses the system board")
		}
	}
	if _, none := m.SwapPlan(w, q, ms.Actor, 500, p, stacks, nil, bench); none.Class != "" {
		t.Fatal("a well-disposed master swapped")
	}
	if m.RefundPermille(0, false) != 1000 || m.RefundPermille(10, false) != 500 || m.RefundPermille(10, true) != 0 {
		t.Fatal("refund shares")
	}
}
