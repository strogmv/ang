package inventory

import (
	"errors"
	"math"
)

// Grade is the make of one physical instance: quality 1-5, provenance (prewar | system | handmade) and the chip binding
// of system gear. The zero grade means "as the definition": the crafting catalog supplies the defaults. Moves and splits
// copy it with the instance, so a stack of prewar connectors stays prewar wherever it goes.
type Grade struct {
	Quality    int    `json:"quality,omitempty"`
	Provenance string `json:"provenance,omitempty"`
	Bound      string `json:"bound,omitempty"`
}

func (g Grade) valid() bool {
	switch g.Provenance {
	case "", "prewar", "system", "handmade":
	default:
		return false
	}
	return g.Quality >= 0 && g.Quality <= 5 && len(g.Bound) <= 128
}

// Relocate moves a quantity of an instance to a location; a partial quantity is split off under the caller's id.
type Relocate struct {
	Item     string
	Quantity int64
	Split    string
	To       Location
	Owner    string
}

// Spend uses up a quantity of an instance (parts worked into a product, spoiled parts).
type Spend struct {
	Item     string
	Quantity int64
}

// Make creates a new instance under the caller's deterministic id (a crafted product, salvaged parts).
type Make struct {
	ID         string
	Definition string
	Quantity   int64
	Condition  int64
	Owner      string
	Origin     string
	Location   Location
	Grade      Grade
}

// Restore sets the condition of an instance (repair, maintenance, damage of a failed repair).
type Restore struct {
	Item      string
	Condition int64
}

// Regrade sets the make of an instance (crafting stage 2: the chip binding written by a terminal or forged by a bypass).
type Regrade struct {
	Item  string
	Grade Grade
}

// Transform is the trusted physical change of a crafting job: relocations into and out of work stores, spending, making,
// condition changes and account payments, all or nothing. Unlike Apply it has no client cursor and ignores store locks:
// the authoritative adapter has already checked the actor, the place, the rules and the money. The whole state is
// validated before it is committed, so capacity, ownership and custody invariants hold as for every other transaction.
type Transform struct {
	Relocate []Relocate
	Spend    []Spend
	Make     []Make
	Restore  []Restore
	Payments []Payment
	Regrade  []Regrade
}

func (s *State) Transform(m Model, t Transform) (Result, error) {
	n := len(t.Relocate) + len(t.Spend) + len(t.Make) + len(t.Restore) + len(t.Payments) + len(t.Regrade)
	if n == 0 || n > 256 || s.Revision == math.MaxUint64 {
		return Result{}, errors.New("invalid transform")
	}
	next := s.Clone()
	result := Result{Revision: s.Revision + 1}
	for _, r := range t.Relocate {
		i, ok := next.Items[r.Item]
		if !ok || r.Quantity < 1 || r.Quantity > i.Quantity || i.Revision == math.MaxUint64 || next.Reserved(m, i.ID) {
			return Result{}, errors.New("invalid relocation")
		}
		if r.To.Kind == "store" {
			if _, ok := next.Stores[r.To.Store]; !ok {
				return Result{}, errors.New("unknown relocation store")
			}
		}
		moved := i
		if r.Quantity < i.Quantity {
			if _, exists := next.Items[r.Split]; exists || r.Split == "" || m.Definitions[i.Definition].ContainerVolume > 0 {
				return Result{}, errors.New("invalid split")
			}
			i.Quantity -= r.Quantity
			i.Revision++
			next.Items[i.ID] = i
			moved.ID = r.Split
			moved.Quantity = r.Quantity
			result.Created = append(result.Created, moved.ID)
		}
		moved.Location = r.To
		moved.Revision++
		if r.Owner != "" {
			moved.Owner = r.Owner
			moved.ForSale = false
		}
		next.Items[moved.ID] = moved
	}
	for _, sp := range t.Spend {
		i, ok := next.Items[sp.Item]
		if !ok || sp.Quantity < 1 || sp.Quantity > i.Quantity || next.Reserved(m, i.ID) || m.Definitions[i.Definition].ContainerVolume > 0 {
			return Result{}, errors.New("invalid spending")
		}
		i.Quantity -= sp.Quantity
		if i.Quantity == 0 {
			delete(next.Items, i.ID)
		} else {
			i.Revision++
			next.Items[i.ID] = i
		}
	}
	for _, mk := range t.Make {
		d, ok := m.Definitions[mk.Definition]
		if !ok || d.Carrier != "physical" || d.ContainerVolume > 0 || mk.ID == "" || mk.Origin == "" || mk.Quantity < 1 || mk.Quantity > d.StackLimit || mk.Condition < 0 || mk.Condition > 1000000 || !mk.Grade.valid() {
			return Result{}, errors.New("invalid making")
		}
		if _, exists := next.Items[mk.ID]; exists || len(next.Items) >= m.MaxItems {
			return Result{}, errors.New("making identity collision or instance limit")
		}
		next.Items[mk.ID] = Item{ID: mk.ID, Definition: mk.Definition, Quantity: mk.Quantity, Condition: mk.Condition, Owner: mk.Owner, Location: mk.Location, Origin: mk.Origin, Grade: mk.Grade}
		result.Created = append(result.Created, mk.ID)
	}
	for _, rs := range t.Restore {
		i, ok := next.Items[rs.Item]
		if !ok || rs.Condition < 0 || rs.Condition > 1000000 || i.Revision == math.MaxUint64 {
			return Result{}, errors.New("invalid condition change")
		}
		i.Condition = rs.Condition
		i.Revision++
		next.Items[i.ID] = i
	}
	for _, g := range t.Regrade {
		i, ok := next.Items[g.Item]
		if !ok || !g.Grade.valid() || i.Revision == math.MaxUint64 {
			return Result{}, errors.New("invalid regrade")
		}
		i.Grade = g.Grade
		i.Revision++
		next.Items[i.ID] = i
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
	if err := next.Validate(m); err != nil {
		return Result{}, err
	}
	*s = next
	return result, nil
}
