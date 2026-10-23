package inventory

import "testing"

func actionAccess(actor string) Access {
	return Access{Level: "map", At: Cell{2, 2}, Known: func(Item) bool { return true }, Reach: func(Anchor) bool { return true }, Withdraw: func(Item) bool { return true }, Deposit: func(Store) bool { return true }, Receive: func(string, Item) bool { return true }, Target: func(string) (Cell, bool) { return Cell{2, 3}, true }}
}
func TestPlayerAndNPCUseSameActionsAndStableReplay(t *testing.T) {
	for _, actor := range []string{"hero", "npc"} {
		t.Run(actor, func(t *testing.T) {
			m := fixtureModel()
			s := mustState(t, m)
			id := InstanceID("ore", 0, 0)
			item := s.Items[id]
			item.Owner = ""
			s.Items[id] = item
			action := Action{Actor: actor, Kind: "take", Item: id, Quantity: 1, Sequence: 1}
			result, err := s.Execute(m, action, actionAccess(actor))
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Created) != 1 {
				t.Fatal("partial take did not preserve split identity")
			}
			part := s.Items[result.Created[0]]
			if !s.carriedBy(m, part, actor) || part.Owner != actor {
				t.Fatal("wrong actor custody")
			}
			// A committed input may be retried after the actor moved away.
			again, err := s.Execute(m, action, Access{})
			if err != nil || !again.Replayed {
				t.Fatal("replay was re-authorized against a changed world")
			}
			if _, err = s.Execute(m, Action{Actor: actor, Kind: "drop", Item: part.ID, Quantity: 1, Sequence: 2, ExpectedRevision: part.Revision}, actionAccess(actor)); err != nil {
				t.Fatal(err)
			}
			part = s.Items[part.ID]
			if part.Location.Kind != "ground" {
				t.Fatal("drop did not place actual item")
			}
		})
	}
}
func TestExclusiveLeaseCannotBeBypassedAndExpires(t *testing.T) {
	m := fixtureModel()
	s := mustState(t, m)
	id := InstanceID("ore", 0, 0)
	claim := Reservation{ID: "lease.one", Actor: "hero", Item: id, Purpose: "mission.one", Quantity: 1, CreatedTick: 0, ExpiresTick: 10}
	if err := s.ReserveBatch(m, []Reservation{claim}); err != nil {
		t.Fatal(err)
	}
	before := digestState(s)
	other := claim
	other.ID = "lease.two"
	other.Actor = "npc"
	other.Purpose = "mission.two"
	if s.ReserveBatch(m, []Reservation{other}) == nil || digestState(s) != before {
		t.Fatal("double reservation committed")
	}
	action := Action{Actor: "npc", Kind: "take", Item: id, Quantity: 1, Sequence: 1}
	if _, err := s.Execute(m, action, actionAccess("npc")); err == nil || digestState(s) != before {
		t.Fatal("take bypassed somebody else's lease")
	}
	// A retry must not prolong the lease.
	if err := s.Advance(3); err != nil {
		t.Fatal(err)
	}
	if err := s.ReserveBatch(m, []Reservation{claim}); err != nil || s.Reservations[id].ExpiresTick != 10 {
		t.Fatal("lease retry extended deadline")
	}
	if err := s.Advance(10); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Execute(m, action, actionAccess("npc")); err != nil {
		t.Fatal("expired lease still blocks stock")
	}
}
func TestLeaseProtectsNestedStockAndAuthorizedIssue(t *testing.T) {
	m := fixtureModel()
	s := mustState(t, m)
	bag := InstanceID("bag", 0, 0)
	ore := InstanceID("ore", 0, 0)
	i := s.Items[ore]
	i.Location = Location{Kind: "store", Store: ItemStore(bag)}
	s.Items[ore] = i
	claim := Reservation{ID: "lease.ore", Actor: "npc", Item: ore, Purpose: "equipment", Quantity: 1, CreatedTick: 0, ExpiresTick: 100}
	if err := s.ReserveBatch(m, []Reservation{claim}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Execute(m, Action{Actor: "hero", Kind: "drop", Item: bag, Quantity: 1, Sequence: 1}, actionAccess("hero")); err == nil {
		t.Fatal("container move stole leased contents")
	}
	result, err := s.Execute(m, Action{Actor: "npc", Kind: "issue", Item: ore, Quantity: 1, Sequence: 1, Reservation: claim.ID}, actionAccess("npc"))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Reservations) != 0 || len(result.Created) != 1 || s.Items[result.Created[0]].Owner != "npc" {
		t.Fatal("issue lost quantity, legal owner or lease")
	}
}
func TestEquipmentUseAndGroupFailureAreAtomic(t *testing.T) {
	m := fixtureModel()
	m.Definitions["stunner"] = Definition{ID: "stunner", Carrier: "physical", Mass: 1, Volume: 1, StackLimit: 1, EquipSlot: "hand.main", UseEffect: "stun", UseReach: 1, WearPerUse: 250000}
	for _, actor := range []string{"hero", "npc"} {
		m.Sources = append(m.Sources, Source{ID: "stunner." + actor, Definition: "stunner", Quantity: 1, Condition: 1000000, Owner: actor, Location: Location{Kind: "store", Store: ActorStore(actor)}})
	}
	s := mustState(t, m)
	for _, actor := range []string{"hero", "npc"} {
		if _, err := s.Execute(m, Action{Actor: actor, Kind: "equip", Item: InstanceID("stunner."+actor, 0, 0), Quantity: 1, Sequence: 1}, actionAccess(actor)); err != nil {
			t.Fatal(err)
		}
	}
	actions := []Action{}
	for _, actor := range []string{"hero", "npc"} {
		id := InstanceID("stunner."+actor, 0, 0)
		actions = append(actions, Action{Actor: actor, Kind: "use", Item: id, Quantity: 1, ExpectedRevision: s.Items[id].Revision, Sequence: 2, Target: "offender"})
	}
	before := digestState(s)
	bad := append([]Action(nil), actions...)
	bad[1].ExpectedRevision = 99
	if _, err := s.ExecuteBatch(m, bad, actionAccess); err == nil || before != digestState(s) {
		t.Fatal("failed group use partially consumed resources")
	}
	if _, err := s.ExecuteBatch(m, actions, actionAccess); err != nil {
		t.Fatal(err)
	}
	for _, action := range actions {
		if s.Items[action.Item].Condition != 750000 {
			t.Fatal("item use did not consume actual condition")
		}
	}
}
func TestUnknownStockAndUnacceptedTransferAreDenied(t *testing.T) {
	m := fixtureModel()
	s := mustState(t, m)
	id := InstanceID("ore", 0, 0)
	if _, err := s.Execute(m, Action{Actor: "hero", Kind: "take", Item: id, Quantity: 1, Sequence: 1}, Access{Reach: func(Anchor) bool { return true }}); err == nil {
		t.Fatal("actor selected unknown world item")
	}
	id = InstanceID("bag", 0, 0)
	access := actionAccess("hero")
	access.Receive = nil
	if _, err := s.Execute(m, Action{Actor: "hero", Kind: "transfer", Item: id, Quantity: 1, Sequence: 1, Receiver: "npc"}, access); err == nil {
		t.Fatal("forced transfer to another actor")
	}
}
func TestDroppingBagReleasesNestedLoadAndItemsStayInIt(t *testing.T) {
	m := fixtureModel()
	s := mustState(t, m)
	bag := InstanceID("bag", 0, 0)
	ore := InstanceID("ore", 0, 0)
	i := s.Items[ore]
	i.Location = Location{Kind: "store", Store: ItemStore(bag)}
	i.Owner = "hero"
	s.Items[ore] = i
	before, err := s.ActorLoad(m, "hero")
	if err != nil || before.Mass != 50 {
		t.Fatalf("nested mass: %v %v", before, err)
	}
	if _, err = s.Execute(m, Action{Actor: "hero", Kind: "drop", Item: bag, Quantity: 1, Sequence: 1}, actionAccess("hero")); err != nil {
		t.Fatal(err)
	}
	after, err := s.ActorLoad(m, "hero")
	if err != nil || after.Mass != 0 {
		t.Fatal("dropped contents still burden actor")
	}
	if s.Items[ore].Location.Store != ItemStore(bag) || s.Items[bag].Owner != "hero" {
		t.Fatal("bag drop detached contents or erased title")
	}
	// Once the bag is left behind, its contents cannot be used remotely.
	far := actionAccess("hero")
	far.Reach = func(Anchor) bool { return false }
	if _, err = s.Execute(m, Action{Actor: "hero", Kind: "take", Item: ore, Quantity: 1, Sequence: 2}, far); err == nil {
		t.Fatal("remote backpack contents remained accessible")
	}
}
