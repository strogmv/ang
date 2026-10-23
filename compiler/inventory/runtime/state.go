package inventory

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
)

func (s State) Validate(m Model) error {
	if err := s.validateReservations(m); err != nil {
		return err
	}
	if s.Version != 1 || s.Version != m.Version || len(s.Items) > m.MaxItems || len(s.Cursors) > len(m.Actors) {
		return errors.New("invalid inventory version or size")
	}
	for id, d := range m.Definitions {
		if d.ID != id || d.StackLimit < 1 || d.ContainerVolume < 0 || (d.Carrier != "physical" && d.Carrier != "digital") {
			return fmt.Errorf("invalid definition %s", id)
		}
		if d.UseEffect != "" && (d.EquipSlot == "" || d.UseReach < 1 || d.WearPerUse < 1 || d.WearPerUse > 1000000 || d.StackLimit != 1) {
			return errors.New("invalid usable item definition")
		}
		if d.Carrier == "physical" && (d.Mass < 1 || d.Volume < 1) {
			return fmt.Errorf("physical dimensions missing: %s", id)
		}
	}
	for _, d := range m.Definitions {
		if d.Currency != "" {
			currency, ok := m.Currencies[d.Currency]
			if !ok || d.Denomination < 1 || (d.Carrier == "physical" && currency.Carrier != "physical") || (d.Carrier == "digital" && currency.Carrier != "account") {
				return errors.New("item currency carrier mismatch")
			}
		}
	}
	for code, c := range m.Currencies {
		if code != c.Code || c.Scale < 1 || (c.Carrier != "account" && c.Carrier != "physical") {
			return errors.New("invalid currency")
		}
		if c.Carrier == "physical" {
			d, ok := m.Definitions[c.Item]
			if !ok || d.Carrier != "physical" || d.Currency != code || d.Denomination < 1 {
				return errors.New("physical currency has no matching item")
			}
		}
	}
	if len(s.Accounts) != len(m.Accounts) {
		return errors.New("account registry mismatch")
	}
	for id, a := range s.Accounts {
		initial, ok := m.Accounts[id]
		c, known := m.Currencies[a.Currency]
		if !ok || id != a.ID || a.Owner != initial.Owner || a.Currency != initial.Currency || !m.validOwner(a.Owner) || !known || c.Carrier != "account" || a.Balance < 0 || a.Balance > moneyLimit {
			return errors.New("invalid account")
		}
	}
	for id, st := range m.Stores {
		actual, ok := s.Stores[id]
		actual.Locked = st.Locked
		if !ok || actual != st {
			return fmt.Errorf("static storage mismatch: %s", id)
		}
	}
	for id, st := range s.Stores {
		if _, known := m.Stores[id]; !known && st.Anchor.Kind != "item" {
			return fmt.Errorf("unknown static storage: %s", id)
		}
		if st.ID != id || !m.validOwner(st.Owner) || st.Capacity.Mass < 1 || st.Capacity.Volume < 1 || st.Capacity.Stacks < 1 || (st.Kind != "actor" && st.Kind != "container" && st.Kind != "surface") || (st.Kind == "surface" && st.Locked) {
			return fmt.Errorf("invalid storage %s", id)
		}
		switch st.Anchor.Kind {
		case "actor":
			if !m.Actors[st.Anchor.Actor] || st.Anchor.Item != "" || st.Anchor.Level != "" {
				return errors.New("unknown storage actor")
			}
		case "world":
			if !m.validGround(Location{Kind: "ground", Level: st.Anchor.Level, At: st.Anchor.At}) || st.Anchor.Actor != "" || st.Anchor.Item != "" {
				return errors.New("invalid storage position")
			}
		case "item":
			parent, ok := s.Items[st.Anchor.Item]
			if !ok {
				return errors.New("unknown parent item")
			}
			d := m.Definitions[parent.Definition]
			if parent.Quantity != 1 || d.ContainerVolume < 1 || id != ItemStore(parent.ID) || st.Capacity != (Capacity{Mass: moneyLimit, Volume: d.ContainerVolume, Stacks: m.MaxItems}) || st.Anchor.Actor != "" || st.Anchor.Level != "" {
				return errors.New("invalid portable container")
			}
		default:
			return errors.New("storage anchor missing")
		}
	}
	children := map[string][]string{}
	slots := map[string]bool{}
	for id, i := range s.Items {
		d, ok := m.Definitions[i.Definition]
		if !ok || id != i.ID || d.Carrier != "physical" || i.Quantity < 1 || i.Quantity > d.StackLimit || i.Condition < 0 || i.Condition > 1000000 || !m.validOwner(i.Owner) || i.Origin == "" {
			return fmt.Errorf("invalid physical instance %s", id)
		}
		if d.ContainerVolume > 0 {
			if _, ok := s.Stores[ItemStore(id)]; !ok || i.Quantity != 1 {
				return errors.New("container storage missing")
			}
		}
		switch i.Location.Kind {
		case "ground":
			if !m.validGround(i.Location) {
				return errors.New("invalid ground position")
			}
		case "store":
			if _, ok := s.Stores[i.Location.Store]; !ok || i.Location.Level != "" || i.Location.At != (Cell{}) {
				return errors.New("unknown or ambiguous item storage")
			}
			children[i.Location.Store] = append(children[i.Location.Store], id)
			if i.Location.Slot != "" {
				key := i.Location.Store + "/" + i.Location.Slot
				if slots[key] {
					return errors.New("slot occupied")
				}
				slots[key] = true
			}
		default:
			return errors.New("item must have exactly one location")
		}
		if _, err := s.Root(m, i.Location); err != nil {
			return fmt.Errorf("%s: %w", id, err)
		}
	}
	for id, e := range s.Entitlements {
		d, ok := m.Definitions[e.Definition]
		if id == "" || !m.Actors[e.Actor] || !ok || d.Carrier != "digital" {
			return errors.New("invalid digital entitlement")
		}
	}
	for id, e := range m.Entitlements {
		actual, ok := s.Entitlements[id]
		if !ok || actual.Actor != e.Actor || actual.Definition != e.Definition {
			return errors.New("entitlement registry mismatch")
		}
	}
	for id, c := range s.Cursors {
		if !m.Actors[id] || c.Sequence < 1 || c.Result.Revision > s.Revision || len(c.Result.Created) > m.MaxItems {
			return errors.New("invalid command cursor")
		}
	}
	memo := map[string]int64{}
	var mass func(string) (int64, error)
	mass = func(id string) (int64, error) {
		if n, ok := memo[id]; ok {
			return n, nil
		}
		i := s.Items[id]
		d := m.Definitions[i.Definition]
		n, err := times(i.Quantity, d.Mass)
		if err != nil {
			return 0, err
		}
		for _, child := range children[ItemStore(id)] {
			v, e := mass(child)
			if e != nil {
				return 0, e
			}
			n, e = plus(n, v)
			if e != nil {
				return 0, e
			}
		}
		memo[id] = n
		return n, nil
	}
	for _, id := range sortedKeys(s.Stores) {
		st := s.Stores[id]
		if len(children[id]) > st.Capacity.Stacks {
			return fmt.Errorf("stack capacity: %s", id)
		}
		var weight, volume int64
		for _, child := range children[id] {
			i := s.Items[child]
			w, e := mass(child)
			if e != nil {
				return e
			}
			weight, e = plus(weight, w)
			if e != nil {
				return e
			}
			v, e := times(i.Quantity, m.Definitions[i.Definition].Volume)
			if e != nil {
				return e
			}
			volume, e = plus(volume, v)
			if e != nil {
				return e
			}
		}
		if weight > st.Capacity.Mass || volume > st.Capacity.Volume {
			return fmt.Errorf("storage capacity exceeded: %s", id)
		}
	}
	return nil
}

// Root derives custody/position through nested containers. It never copies a
// carrier's coordinates into items, so walking and LOD changes cannot detach them.
func (s State) Root(m Model, l Location) (Anchor, error) {
	seen := map[string]bool{}
	for depth := 0; depth <= m.MaxDepth; depth++ {
		if l.Kind == "ground" {
			if !m.validGround(l) {
				return Anchor{}, errors.New("invalid ground root")
			}
			return Anchor{Kind: "world", Level: l.Level, At: l.At}, nil
		}
		if l.Kind != "store" || seen[l.Store] {
			return Anchor{}, errors.New("storage cycle or missing location")
		}
		seen[l.Store] = true
		st, ok := s.Stores[l.Store]
		if !ok {
			return Anchor{}, errors.New("unknown storage")
		}
		if st.Anchor.Kind != "item" {
			return st.Anchor, nil
		}
		parent, ok := s.Items[st.Anchor.Item]
		if !ok {
			return Anchor{}, errors.New("missing container item")
		}
		l = parent.Location
	}
	return Anchor{}, errors.New("storage nesting limit")
}
func (s State) accessible(m Model, l Location) error {
	for depth := 0; depth <= m.MaxDepth && l.Kind == "store"; depth++ {
		st, ok := s.Stores[l.Store]
		if !ok {
			return errors.New("unknown storage")
		}
		if st.Locked {
			return errors.New("storage locked")
		}
		if st.Anchor.Kind != "item" {
			return nil
		}
		parent, ok := s.Items[st.Anchor.Item]
		if !ok {
			return errors.New("missing container")
		}
		l = parent.Location
	}
	return nil
}

type Move struct {
	Reservation      string
	Item             string
	Quantity         int64
	ExpectedRevision uint64
	To               Location
	ChangeOwner      bool
	Owner            string
}
type Payment struct {
	From, To, Currency       string
	Amount                   int64
	FromRevision, ToRevision uint64
}

// The authoritative adapter authorizes the actor, access rights, reach, claims
// and prices. This transaction validates structural state and commits all legs.
type Transaction struct {
	RequestTag string
	Actor      string
	Sequence   uint64
	Moves      []Move
	Payments   []Payment
	Wear       []Wear
}

func (s *State) Apply(m Model, t Transaction) (Result, error) {
	if !m.Actors[t.Actor] || t.Sequence == 0 || len(t.Moves)+len(t.Payments)+len(t.Wear) == 0 || len(t.Moves)+len(t.Payments)+len(t.Wear) > 128 {
		return Result{}, errors.New("invalid transaction")
	}
	b, _ := json.Marshal(t)
	digest := sha256.Sum256(b)
	cursor := s.Cursors[t.Actor]
	if t.Sequence == cursor.Sequence {
		if digest != cursor.Digest {
			return Result{}, errors.New("sequence payload mismatch")
		}
		r := cursor.Result
		r.Created = append([]string(nil), r.Created...)
		r.Replayed = true
		return r, nil
	}
	if cursor.Sequence == math.MaxUint64 || t.Sequence != cursor.Sequence+1 {
		return Result{}, errors.New("stale or out-of-order transaction")
	}
	if s.Revision == math.MaxUint64 {
		return Result{}, errors.New("revision exhausted")
	}
	next := s.Clone()
	result := Result{Revision: s.Revision + 1}
	for index, move := range t.Moves {
		if err := next.allowReservedMove(m, t.Actor, move); err != nil {
			return Result{}, err
		}
		i, ok := next.Items[move.Item]
		if !ok || i.Revision != move.ExpectedRevision || move.Quantity < 1 || move.Quantity > i.Quantity || i.Revision == math.MaxUint64 {
			return Result{}, errors.New("unknown item, stale revision or invalid quantity")
		}
		if err := next.accessible(m, i.Location); err != nil {
			return Result{}, err
		}
		if err := next.accessible(m, move.To); err != nil {
			return Result{}, err
		}
		moved := i
		if move.Quantity < i.Quantity {
			if m.Definitions[i.Definition].ContainerVolume > 0 {
				return Result{}, errors.New("cannot split a container")
			}
			i.Quantity -= move.Quantity
			i.Revision++
			next.Items[i.ID] = i
			moved.ID = fmt.Sprintf("instance:tx:%s:%d:%d", t.Actor, t.Sequence, index)
			if _, exists := next.Items[moved.ID]; exists {
				return Result{}, errors.New("split identity collision")
			}
			moved.Quantity = move.Quantity
			result.Created = append(result.Created, moved.ID)
		}
		moved.Location = move.To
		moved.Revision++
		if move.ChangeOwner {
			moved.Owner = move.Owner
			moved.ForSale = false
		}
		next.Items[moved.ID] = moved
		if move.Reservation != "" {
			delete(next.Reservations, move.Item)
		}
	}
	for _, wear := range t.Wear {
		item, ok := next.Items[wear.Item]
		if !ok || item.Revision != wear.ExpectedRevision || wear.Amount < 1 || wear.Amount > item.Condition || item.Revision == math.MaxUint64 || next.Reserved(m, item.ID) {
			return Result{}, errors.New("invalid item use")
		}
		item.Condition -= wear.Amount
		item.Revision++
		next.Items[item.ID] = item
	}
	for _, p := range t.Payments {
		from, ok1 := next.Accounts[p.From]
		to, ok2 := next.Accounts[p.To]
		c, ok3 := m.Currencies[p.Currency]
		if !ok1 || !ok2 || !ok3 || c.Carrier != "account" || p.From == p.To || p.Currency != from.Currency || p.Currency != to.Currency || p.Amount < 1 || p.Amount > from.Balance || to.Balance > moneyLimit-p.Amount || from.Revision != p.FromRevision || to.Revision != p.ToRevision || from.Revision == math.MaxUint64 || to.Revision == math.MaxUint64 {
			return Result{}, errors.New("invalid account payment")
		}
		from.Balance -= p.Amount
		to.Balance += p.Amount
		from.Revision++
		to.Revision++
		next.Accounts[p.From] = from
		next.Accounts[p.To] = to
	}
	next.Revision = result.Revision
	next.Cursors[t.Actor] = Cursor{RequestTag: t.RequestTag, Sequence: t.Sequence, Digest: digest, Result: Result{Revision: result.Revision, Created: append([]string(nil), result.Created...)}}
	if err := next.Validate(m); err != nil {
		return Result{}, err
	}
	*s = next
	return result, nil
}
