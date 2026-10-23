package inventory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
)

// Action is shared by player input, NPC plans and server scripts. Authority and
// observations come from the host; the caller never supplies prices or wear.
type Action struct {
	Actor, Kind, Item, Store, Receiver, Target, Reservation, RequestTag string
	Sequence                                                            uint64
	Quantity                                                            int64
	ExpectedRevision                                                    uint64
}
type Access struct {
	Level    string
	At       Cell
	Known    func(Item) bool
	Reach    func(Anchor) bool
	Withdraw func(Item) bool
	Deposit  func(Store) bool
	Receive  func(string, Item) bool
	Target   func(string) (Cell, bool)
}
type Wear struct {
	Item             string
	ExpectedRevision uint64
	Amount           int64
}

func (a Action) tag() string {
	if a.RequestTag != "" {
		return a.RequestTag
	}
	b, _ := json.Marshal(a)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func (s State) Replay(actor string, sequence uint64, tag string) (Result, bool, error) {
	c := s.Cursors[actor]
	if sequence > c.Sequence {
		return Result{}, false, nil
	}
	if sequence == 0 || sequence != c.Sequence || tag == "" || tag != c.RequestTag {
		return Result{}, false, errors.New("stale or changed action")
	}
	result := c.Result
	result.Created = append([]string(nil), result.Created...)
	result.Replayed = true
	return result, true, nil
}
func (s State) carriedBy(m Model, item Item, actor string) bool {
	root, e := s.Root(m, item.Location)
	return e == nil && root.Kind == "actor" && root.Actor == actor
}
func (s State) TradeAccounts(m Model, actor, owner, level, currency string) (Account, Account, error) {
	var from, to Account
	market := m.Markets[m.LevelSettlement[level]]
	accepted := false
	for _, c := range market.Accepted {
		if c == currency {
			accepted = true
		}
	}
	if !accepted || m.Currencies[currency].Carrier != "account" {
		return from, to, errors.New("currency not accepted")
	}
	for _, id := range sortedKeys(s.Accounts) {
		a := s.Accounts[id]
		if a.Owner == actor && a.Currency == currency {
			if from.ID != "" {
				return from, to, errors.New("ambiguous payer account")
			}
			from = a
		}
		if a.Owner == owner && a.Currency == currency {
			if to.ID != "" {
				return from, to, errors.New("ambiguous seller account")
			}
			to = a
		}
	}
	if from.ID == "" || to.ID == "" || from.ID == to.ID {
		return from, to, errors.New("trade account missing")
	}
	return from, to, nil
}
func (s *State) Execute(m Model, a Action, access Access) (Result, error) {
	if !m.Actors[a.Actor] || a.Sequence == 0 {
		return Result{}, errors.New("unknown action actor")
	}
	tag := a.tag()
	if result, done, err := s.Replay(a.Actor, a.Sequence, tag); done || err != nil {
		return result, err
	}
	item, ok := s.Items[a.Item]
	if !ok || item.Revision != a.ExpectedRevision || a.Quantity < 1 || a.Quantity > item.Quantity {
		return Result{}, errors.New("item changed or missing")
	}
	own := s.carriedBy(m, item, a.Actor)
	if !own && (access.Known == nil || !access.Known(item)) {
		return Result{}, errors.New("item not known to actor")
	}
	root, err := s.Root(m, item.Location)
	if err != nil || access.Reach == nil || !access.Reach(root) {
		return Result{}, errors.New("item out of reach")
	}
	if err = s.accessible(m, item.Location); err != nil {
		return Result{}, err
	}
	d := m.Definitions[item.Definition]
	tx := Transaction{Actor: a.Actor, Sequence: a.Sequence, RequestTag: tag}
	move := Move{Item: item.ID, Quantity: a.Quantity, ExpectedRevision: item.Revision, Reservation: a.Reservation, To: Location{Kind: "store", Store: ActorStore(a.Actor)}}
	switch a.Kind {
	case "take":
		if item.Owner != "" && item.Owner != a.Actor && (access.Withdraw == nil || !access.Withdraw(item)) {
			return Result{}, errors.New("withdrawal not authorized")
		}
		if item.Owner == "" {
			move.ChangeOwner = true
			move.Owner = a.Actor
		}
	case "issue":
		if access.Withdraw == nil || !access.Withdraw(item) || a.Reservation == "" {
			return Result{}, errors.New("issue requires an authorized reservation")
		}
	case "drop":
		if !own {
			return Result{}, errors.New("item not carried")
		}
		move.To = Location{Kind: "ground", Level: access.Level, At: access.At}
	case "put":
		st, ok := s.Stores[a.Store]
		if !own || !ok || access.Deposit == nil || !access.Deposit(st) {
			return Result{}, errors.New("destination not authorized")
		}
		dest, err := s.Root(m, Location{Kind: "store", Store: a.Store})
		if err != nil || !access.Reach(dest) {
			return Result{}, errors.New("destination out of reach")
		}
		move.To = Location{Kind: "store", Store: a.Store}
	case "transfer":
		if !own || !m.Actors[a.Receiver] || a.Receiver == a.Actor || access.Receive == nil || !access.Receive(a.Receiver, item) {
			return Result{}, errors.New("receiver did not accept transfer")
		}
		move.To = Location{Kind: "store", Store: ActorStore(a.Receiver)}
	case "buy":
		if !item.ForSale || item.Owner == "" || item.Owner == a.Actor {
			return Result{}, errors.New("item not for sale")
		}
		amount, err := Quote(d, a.Quantity)
		if err != nil {
			return Result{}, err
		}
		from, to, err := s.TradeAccounts(m, a.Actor, item.Owner, access.Level, d.PriceUnit)
		if err != nil {
			return Result{}, err
		}
		if amount > 0 {
			tx.Payments = []Payment{{From: from.ID, To: to.ID, Currency: d.PriceUnit, Amount: amount, FromRevision: from.Revision, ToRevision: to.Revision}}
		}
		move.ChangeOwner = true
		move.Owner = a.Actor
	case "equip":
		if !own || a.Quantity != 1 || item.Quantity != 1 || d.EquipSlot == "" || item.Condition <= 0 {
			return Result{}, errors.New("item cannot be equipped")
		}
		move.To.Slot = d.EquipSlot
	case "unequip":
		if !own || item.Location.Store != ActorStore(a.Actor) || item.Location.Slot == "" {
			return Result{}, errors.New("item is not equipped")
		}
	case "use":
		if !own || a.Quantity != 1 || item.Quantity != 1 || d.UseEffect == "" || d.EquipSlot == "" || item.Location != (Location{Kind: "store", Store: ActorStore(a.Actor), Slot: d.EquipSlot}) || item.Condition < d.WearPerUse || item.Condition <= 0 || d.WearPerUse <= 0 {
			return Result{}, errors.New("item not ready for use")
		}
		if access.Target == nil {
			return Result{}, errors.New("target not observed")
		}
		at, known := access.Target(a.Target)
		distance := abs(at.X-access.At.X) + abs(at.Y-access.At.Y)
		if !known || a.Target == a.Actor || distance > d.UseReach {
			return Result{}, errors.New("target not in observed reach")
		}
		tx.Wear = []Wear{{Item: item.ID, ExpectedRevision: item.Revision, Amount: d.WearPerUse}}
	default:
		return Result{}, errors.New("unknown inventory action")
	}
	if a.Kind != "use" {
		tx.Moves = []Move{move}
	}
	return s.Apply(m, tx)
}
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// ExecuteBatch provides an all-or-nothing group barrier for resource use. Hosts
// apply the corresponding world effect only after every authorized use succeeds.
func (s *State) ExecuteBatch(m Model, actions []Action, access func(string) Access) ([]Result, error) {
	if len(actions) == 0 || len(actions) > 128 {
		return nil, errors.New("invalid action batch")
	}
	next := s.Clone()
	results := make([]Result, 0, len(actions))
	for _, action := range actions {
		result, err := next.Execute(m, action, access(action.Actor))
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	*s = next
	return results, nil
}
