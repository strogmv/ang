package inventory

import (
	"encoding/json"
	"reflect"
	"testing"
)

func fixtureModel() Model {
	m := Model{Reservation: ReservationPolicy{MaxTicks: 600, MaxPerActor: 16, MaxTotal: 128}, Version: 1, MaxDepth: 8, MaxItems: 1000, Actors: map[string]bool{"hero": true, "npc": true}, Owners: map[string]bool{"hero": true, "npc": true}, Levels: map[string]Cell{"map": {X: 10, Y: 10}}, Definitions: map[string]Definition{
		"bag":   {ID: "bag", Carrier: "physical", Mass: 10, Volume: 20, StackLimit: 1, ContainerVolume: 100},
		"ore":   {ID: "ore", Carrier: "physical", Mass: 2, Volume: 1, StackLimit: 100},
		"salt":  {ID: "salt", Carrier: "physical", Mass: 4, Volume: 3, StackLimit: 100, Currency: "SLT", Denomination: 1},
		"quota": {ID: "quota", Carrier: "digital", StackLimit: 1},
	}, Currencies: map[string]Currency{"CRD": {Code: "CRD", Carrier: "account", Scale: 100}, "SLT": {Code: "SLT", Carrier: "physical", Scale: 1, Item: "salt"}}, Stores: map[string]Store{}, Accounts: map[string]Account{"a": {ID: "a", Owner: "hero", Currency: "CRD", Balance: 1000}, "b": {ID: "b", Owner: "npc", Currency: "CRD"}}, Entitlements: map[string]Entitlement{"water": {Actor: "hero", Definition: "quota"}}}
	for _, actor := range []string{"hero", "npc"} {
		id := ActorStore(actor)
		m.Stores[id] = Store{ID: id, Kind: "actor", Anchor: Anchor{Kind: "actor", Actor: actor}, Owner: actor, Capacity: Capacity{Mass: 1000, Volume: 1000, Stacks: 100}}
	}
	m.Sources = []Source{{ID: "bag", Definition: "bag", Quantity: 1, Condition: 1000000, Owner: "hero", Location: Location{Kind: "store", Store: ActorStore("hero")}}, {ID: "ore", Definition: "ore", Quantity: 20, Condition: 500000, Owner: "npc", Location: Location{Kind: "ground", Level: "map", At: Cell{X: 2, Y: 2}}}}
	return m
}
func mustState(t *testing.T, m Model) State {
	t.Helper()
	s, e := New(m, 123)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func digestState(s State) string { b, _ := json.Marshal(s); return string(b) }
func TestMoveSplitPreservesQuantityClaimAndReplay(t *testing.T) {
	m := fixtureModel()
	s := mustState(t, m)
	id := InstanceID("ore", 0, 0)
	tx := Transaction{Actor: "hero", Sequence: 1, Moves: []Move{{Item: id, Quantity: 3, To: Location{Kind: "store", Store: ActorStore("hero")}}}}
	r, err := s.Apply(m, tx)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Created) != 1 || s.Items[id].Quantity != 17 || s.Items[r.Created[0]].Quantity != 3 || s.Items[r.Created[0]].Owner != "npc" {
		t.Fatal("split duplicated quantity or changed ownership")
	}
	root, err := s.Root(m, s.Items[r.Created[0]].Location)
	if err != nil || root.Actor != "hero" {
		t.Fatal("custody did not move")
	}
	snapshot := digestState(s)
	again, err := s.Apply(m, tx)
	if err != nil || !again.Replayed || digestState(s) != snapshot {
		t.Fatal("replay changed state")
	}
	tx.Moves[0].Quantity = 4
	if _, err = s.Apply(m, tx); err == nil {
		t.Fatal("same sequence changed payload")
	}
}
func TestNestedWeightCycleAndPaymentAreAtomic(t *testing.T) {
	m := fixtureModel()
	st := m.Stores[ActorStore("hero")]
	st.Capacity.Mass = 20
	m.Stores[st.ID] = st
	s := mustState(t, m)
	before := digestState(s)
	_, err := s.Apply(m, Transaction{Actor: "hero", Sequence: 1, Moves: []Move{{Item: InstanceID("ore", 0, 0), Quantity: 6, To: Location{Kind: "store", Store: ItemStore(InstanceID("bag", 0, 0))}}}})
	if err == nil || digestState(s) != before {
		t.Fatal("nested contents escaped ancestor mass limit or partially committed")
	}
	_, err = s.Apply(m, Transaction{Actor: "hero", Sequence: 1, Moves: []Move{{Item: InstanceID("bag", 0, 0), Quantity: 1, To: Location{Kind: "store", Store: ItemStore(InstanceID("bag", 0, 0))}}}})
	if err == nil || digestState(s) != before {
		t.Fatal("container stored inside itself")
	}
	_, err = s.Apply(m, Transaction{Actor: "hero", Sequence: 1, Moves: []Move{{Item: InstanceID("ore", 0, 0), Quantity: 1, To: Location{Kind: "store", Store: ActorStore("npc")}}}, Payments: []Payment{{From: "a", To: "b", Currency: "CRD", Amount: 1001}}})
	if err == nil || digestState(s) != before {
		t.Fatal("failed payment left an item moved")
	}
}
func TestCurrencyDomainsAndCommittedSequences(t *testing.T) {
	m := fixtureModel()
	m.Currencies["LOCAL"] = Currency{Code: "LOCAL", Carrier: "account", Scale: 1}
	m.Accounts["local"] = Account{ID: "local", Owner: "npc", Currency: "LOCAL", Balance: 20}
	s := mustState(t, m)
	_, err := s.Apply(m, Transaction{Actor: "hero", Sequence: 1, Payments: []Payment{{From: "a", To: "local", Currency: "CRD", Amount: 1}}})
	if err == nil {
		t.Fatal("currencies implicitly converted")
	}
	tx := Transaction{Actor: "hero", Sequence: 1, Payments: []Payment{{From: "a", To: "b", Currency: "CRD", Amount: 300}}}
	if _, err = s.Apply(m, tx); err != nil {
		t.Fatal(err)
	}
	if s.Accounts["a"].Balance != 700 || s.Accounts["b"].Balance != 300 {
		t.Fatal("money not conserved")
	}
	if _, err = s.Apply(m, Transaction{Actor: "hero", Sequence: 2, Payments: []Payment{{From: "a", To: "b", Currency: "CRD", Amount: 100, FromRevision: 1, ToRevision: 1}}}); err != nil {
		t.Fatal(err)
	}
	before := digestState(s)
	if _, err = s.Apply(m, tx); err == nil || digestState(s) != before {
		t.Fatal("an old committed request was paid twice")
	}
	m.Accounts["bad"] = Account{ID: "bad", Owner: "hero", Currency: "SLT", Balance: 2}
	if _, err = New(m, 1); err == nil {
		t.Fatal("physical salt represented as account balance")
	}
}
func TestLootSaveAndSnapshotDoNotRerollOrAlias(t *testing.T) {
	m := fixtureModel()
	m.Sources = append(m.Sources, Source{ID: "loot", Location: Location{Kind: "ground", Level: "map", At: Cell{X: 3, Y: 3}}, Rolls: 8, NothingWeight: 1, Loot: []Loot{{Definition: "salt", Weight: 10, Min: 1, Max: 20, ConditionMin: 200000, ConditionMax: 800000}}})
	a := mustState(t, m)
	b := mustState(t, m)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("same seed rerolled loot")
	}
	encoded, _ := json.Marshal(a)
	var restored State
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	if err := restored.Validate(m); err != nil {
		t.Fatal(err)
	}
	if digestState(a) != digestState(restored) {
		t.Fatal("save changed item identities")
	}
	copy := a.Clone()
	id := InstanceID("ore", 0, 0)
	i := copy.Items[id]
	i.Quantity = 1
	copy.Items[id] = i
	if a.Items[id].Quantity == 1 {
		t.Fatal("snapshot aliases live inventory")
	}
}
func TestUnlocatedDigitalAndAmbiguousItemsAreRejected(t *testing.T) {
	for _, mutate := range []func(*Model){func(m *Model) { m.Sources[0].Definition = "quota" }, func(m *Model) { m.Sources[0].Location = Location{} }, func(m *Model) {
		m.Sources[0].Location = Location{Kind: "ground", Level: "map", At: Cell{X: 1, Y: 1}, Store: ActorStore("hero")}
	}} {
		m := fixtureModel()
		mutate(&m)
		if _, err := New(m, 1); err == nil {
			t.Fatal("invalid item location/carrier accepted")
		}
	}
}

func TestFractionalPricesAreExactAndCannotMintChange(t *testing.T) {
	d := Definition{PriceNumerator: 1, PriceDenominator: 2}
	if _, err := Quote(d, 1); err == nil {
		t.Fatal("half a physical denomination rounded into money")
	}
	amount, err := Quote(d, 2)
	if err != nil || amount != 1 {
		t.Fatal("compatible quantity has wrong price")
	}
}

func TestRestoreRejectsInventedStorageAndCurrencyCarrierMismatch(t *testing.T) {
	m := fixtureModel()
	s := mustState(t, m)
	s.Stores["invented"] = Store{ID: "invented", Kind: "surface", Anchor: Anchor{Kind: "world", Level: "map", At: Cell{X: 2, Y: 2}}, Capacity: Capacity{Mass: 100, Volume: 100, Stacks: 2}}
	if err := s.Validate(m); err == nil {
		t.Fatal("invented static storage accepted")
	}
	d := m.Definitions["ore"]
	d.Currency = "CRD"
	d.Denomination = 1
	m.Definitions["ore"] = d
	if _, err := New(m, 1); err == nil {
		t.Fatal("physical item minted account currency")
	}
}
