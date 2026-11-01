package inventory

import (
	"reflect"
	"testing"
)

func TestFiniteConsumptionDoesNotChangeMoneyOrOtherOwners(t *testing.T) {
	m := fixtureModel()
	s := mustState(t, m)
	id := InstanceID("ore", 0, 0)
	before := s.Clone()
	if s.Consume(m, "npc", id, 1) == nil || !reflect.DeepEqual(before, s) {
		t.Fatal("consumed unowned stock")
	}
	item := s.Items[id]
	item.Owner = ""
	s.Items[id] = item
	result, err := s.Execute(m, Action{Actor: "npc", Kind: "take", Item: id, Quantity: 1, Sequence: 1}, actionAccess("npc"))
	if err != nil {
		t.Fatal(err)
	}
	bought := result.Created[0]
	accounts := s.Clone().Accounts
	if err = s.Consume(m, "npc", bought, 1); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Items[bought]; ok {
		t.Fatal("consumed unit remains")
	}
	if !reflect.DeepEqual(accounts, s.Accounts) {
		t.Fatal("consumption changed money")
	}
	if s.Consume(m, "npc", bought, 1) == nil {
		t.Fatal("ate same item twice")
	}
}
