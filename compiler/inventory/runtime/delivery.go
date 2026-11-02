package inventory

import (
	"errors"
	"fmt"
)

// Delivery is a finite consignment. Scheduling permits an attempt; it never
// creates stock or moves the carrier. Pose must come from the movement engine.
type DeliveryPoint struct {
	Level string `json:"level"`
	At    Cell   `json:"at"`
}
type Delivery struct {
	ID               string        `json:"id"`
	Carrier          string        `json:"carrier"`
	SourceStore      string        `json:"source_store"`
	DestinationStore string        `json:"destination_store"`
	Pickup           DeliveryPoint `json:"pickup"`
	Dropoff          DeliveryPoint `json:"dropoff"`
	Definition       string        `json:"definition"`
	Quantity         int64         `json:"quantity"`
	FirstSecond      int64         `json:"first_second"`
	PeriodSeconds    int64         `json:"period_seconds"`
}
type Cargo struct {
	Item     string
	Quantity int64
}
type DeliveryProgress struct {
	NextDue    int64
	Cycle      uint64
	Phase      string
	Cargo      []Cargo
	LastSecond int64
}
type Shopfront struct {
	InspectionID string        `json:"inspection_id,omitempty"`
	ID           string        `json:"id"`
	Owner        string        `json:"owner"`
	Service      DeliveryPoint `json:"service"`
	DisplayStore string        `json:"display_store"`
	ReserveStore string        `json:"reserve_store"`
	PackAway     bool          `json:"pack_away"`
	OpenSecond   int64         `json:"open_second"`
	CloseSecond  int64         `json:"close_second"`
	// Stops make the shopfront mobile (a peddler, demo-hub v23): it serves at any of these points (Service is the first)
	// and its display and reserve are carried by the owner (stores anchored to the owner actor, not to the world).
	Stops []DeliveryPoint `json:"stops,omitempty"`
}

// Mobile: a peddler's shopfront that opens wherever its owner stands at one of its stops.
func (sh Shopfront) Mobile() bool { return len(sh.Stops) > 0 }

// ServesCell: the shop serves a proprietor standing on this cell (its service point; for a mobile shop any stop).
func (sh Shopfront) ServesCell(level string, at Cell) bool {
	if !sh.Mobile() {
		return sh.Service.Level == level && sh.Service.At == at
	}
	for _, p := range sh.Stops {
		if p.Level == level && p.At == at {
			return true
		}
	}
	return false
}

// ServesAt: the proprietor's pose stands on a service point of the shop.
func (sh Shopfront) ServesAt(p DeliveryPose) bool { return p.Present && sh.ServesCell(p.Level, p.At) }

type ShopProgress struct {
	Break      bool
	Open       bool
	LastSecond int64
}
type DeliveryPose struct {
	Break   bool
	Level   string
	At      Cell
	Present bool
}

func (p DeliveryPose) AtPoint(at DeliveryPoint) bool {
	return p.Present && p.Level == at.Level && p.At == at.At
}
func (s State) shopStock(store string) bool {
	for _, i := range s.Items {
		if i.Location.Kind == "store" && i.Location.Store == store && i.Quantity > 0 {
			return true
		}
	}
	return false
}
func (s State) ShopStatus(shop Shopfront) (bool, bool) {
	return s.Shopfronts[shop.ID].Open, s.shopStock(shop.DisplayStore) || s.shopStock(shop.ReserveStore)
}
func (s State) ShopAccess(m Model, actor, store string, seconds int64) bool {
	// Enforce every enclosing store, including portable containers on a closed
	// counter. An item/<box> address cannot tunnel through the counter shutter.
	for depth := 0; depth <= m.MaxDepth; depth++ {
		for _, d := range m.Deliveries {
			if d.SourceStore == store && actor != d.Carrier {
				return false
			}
		}
		for _, sh := range m.Shopfronts {
			if sh.ReserveStore == store && actor != sh.Owner {
				return false
			}
			if sh.DisplayStore == store && actor != sh.Owner && (!s.Shopfronts[sh.ID].Open || seconds%86400 < sh.OpenSecond || seconds%86400 >= sh.CloseSecond) {
				return false
			}
		}
		st, ok := s.Stores[store]
		if !ok {
			return false
		}
		if st.Anchor.Kind != "item" {
			return true
		}
		parent, ok := s.Items[st.Anchor.Item]
		if !ok {
			return false
		}
		if parent.Location.Kind != "store" {
			return true
		}
		store = parent.Location.Store
	}
	return false
}
func (s State) validateDeliveries(m Model) error {
	if len(m.Deliveries) > 0 || len(m.Shopfronts) > 0 {
		if s.CommerceVersion != 1 || len(s.Deliveries) != len(m.Deliveries) || len(s.Shopfronts) != len(m.Shopfronts) {
			return errors.New("commerce state registry/version mismatch; explicit save migration required")
		}
	} else if s.CommerceVersion != 0 && s.CommerceVersion != 1 {
		return errors.New("invalid commerce version")
	}
	ids := map[string]bool{}
	carriers := map[string]bool{}
	for _, d := range m.Deliveries {
		if !m.validGround(Location{Kind: "ground", Level: d.Pickup.Level, At: d.Pickup.At}) || !m.validGround(Location{Kind: "ground", Level: d.Dropoff.Level, At: d.Dropoff.At}) {
			return fmt.Errorf("invalid delivery point %s", d.ID)
		}
		source, sok := m.Stores[d.SourceStore]
		dest, dok := m.Stores[d.DestinationStore]
		def, known := m.Definitions[d.Definition]
		if d.ID == "" || ids[d.ID] || carriers[d.Carrier] || !m.Actors[d.Carrier] || !sok || !dok || source.Owner != d.Carrier || dest.Owner != d.Carrier || source.Anchor.Kind != "world" || dest.Anchor.Kind != "world" || source.Anchor.Level != d.Pickup.Level || source.Anchor.At != d.Pickup.At || dest.Anchor.Level != d.Dropoff.Level || d.SourceStore == d.DestinationStore || !known || def.Carrier != "physical" || d.Quantity < 1 || d.Quantity > 100000 || d.FirstSecond < 0 || d.PeriodSeconds < 1 {
			return fmt.Errorf("invalid delivery %s", d.ID)
		}
		ids[d.ID] = true
		carriers[d.Carrier] = true
		p, ok := s.Deliveries[d.ID]
		if !ok {
			continue
		} // legacy save: initialized lazily, no stock created
		if p.LastSecond < 0 || p.NextDue < d.FirstSecond || (p.Phase != "" && p.Phase != "carrying") || (p.Phase == "" && len(p.Cargo) != 0) || (p.Phase == "carrying" && len(p.Cargo) == 0) {
			return errors.New("invalid delivery progress")
		}
		seen := map[string]bool{}
		total := int64(0)
		for _, c := range p.Cargo {
			i, ok := s.Items[c.Item]
			if !ok || seen[c.Item] || c.Quantity < 1 || i.Quantity != c.Quantity || i.Definition != d.Definition || i.Owner != d.Carrier || i.Location != (Location{Kind: "store", Store: ActorStore(d.Carrier)}) {
				return errors.New("delivery manifest differs from physical cargo")
			}
			seen[c.Item] = true
			total += c.Quantity
		}
		if total > d.Quantity {
			return errors.New("delivery cargo exceeds batch")
		}
	}
	for id := range s.Deliveries {
		if !ids[id] {
			return errors.New("unknown saved delivery")
		}
	}
	ids = map[string]bool{}
	for _, sh := range m.Shopfronts {
		if !m.validGround(Location{Kind: "ground", Level: sh.Service.Level, At: sh.Service.At}) {
			return fmt.Errorf("invalid shop service point %s", sh.ID)
		}
		display, ok := m.Stores[sh.DisplayStore]
		reserve, rok := m.Stores[sh.ReserveStore]
		placed := display.Anchor.Kind == "world" && reserve.Anchor.Kind == "world" && display.Anchor.Level == sh.Service.Level && reserve.Anchor.Level == sh.Service.Level
		if sh.Mobile() {
			// carried by the owner; every stop a valid point of the service level, the first one the service point
			placed = display.Anchor == (Anchor{Kind: "actor", Actor: sh.Owner}) && reserve.Anchor == display.Anchor && sh.Stops[0] == sh.Service
			seen := map[DeliveryPoint]bool{}
			for _, p := range sh.Stops {
				if p.Level != sh.Service.Level || seen[p] || !m.validGround(Location{Kind: "ground", Level: p.Level, At: p.At}) {
					placed = false
				}
				seen[p] = true
			}
		}
		if sh.ID == "" || ids[sh.ID] || !m.Actors[sh.Owner] || !ok || !rok || display.Owner != sh.Owner || reserve.Owner != sh.Owner || sh.DisplayStore == sh.ReserveStore || !placed || sh.OpenSecond < 0 || sh.CloseSecond > 86400 || sh.OpenSecond >= sh.CloseSecond {
			return fmt.Errorf("invalid shopfront %s", sh.ID)
		}
		ids[sh.ID] = true
		if s.Shopfronts[sh.ID].LastSecond < 0 {
			return errors.New("invalid shop clock")
		}
	}
	for id := range s.Shopfronts {
		if !ids[id] {
			return errors.New("unknown saved shop")
		}
	}
	return nil
}

// AdvanceCommerce commits inventory and progress together. The host supplies
// all actor poses from authoritative Life, including offscreen floors.
func (s *State) AdvanceCommerce(m Model, seconds int64, poses map[string]DeliveryPose) error {
	if seconds < 0 {
		return errors.New("negative commerce time")
	}
	if err := s.validateDeliveries(m); err != nil {
		return err
	}
	next := s.Clone()
	if next.Deliveries == nil {
		next.Deliveries = map[string]DeliveryProgress{}
	}
	if next.Shopfronts == nil {
		next.Shopfronts = map[string]ShopProgress{}
	}
	for _, d := range m.Deliveries {
		p := next.Deliveries[d.ID]
		if seconds < p.LastSecond {
			return errors.New("delivery clock reversed")
		}
		p.LastSecond = seconds
		next.Deliveries[d.ID] = p
		if p.Phase == "carrying" {
			if !poses[d.Carrier].AtPoint(d.Dropoff) {
				continue
			}
			moves := []Move{}
			for _, c := range p.Cargo {
				i := next.Items[c.Item]
				moves = append(moves, Move{Item: i.ID, Quantity: c.Quantity, ExpectedRevision: i.Revision, To: Location{Kind: "store", Store: d.DestinationStore}})
			}
			work := next.Clone()
			p.Phase = ""
			p.Cargo = nil
			p.NextDue = d.FirstSecond + ((seconds-d.FirstSecond)/d.PeriodSeconds+1)*d.PeriodSeconds
			p.Cycle++
			work.Deliveries[d.ID] = p
			if _, err := work.commerceApply(m, Transaction{Actor: d.Carrier, Sequence: work.Cursors[d.Carrier].Sequence + 1, RequestTag: fmt.Sprintf("%s/drop/%d", d.ID, p.Cycle), Moves: moves}); err == nil {
				next = work
			}
			continue
		}
		if seconds < p.NextDue || !poses[d.Carrier].AtPoint(d.Pickup) {
			continue
		}
		remaining := d.Quantity
		moves := []Move{}
		cargo := []Cargo{}
		seq := next.Cursors[d.Carrier].Sequence + 1
		for _, id := range sortedKeys(next.Items) {
			i := next.Items[id]
			if remaining == 0 || len(moves) >= 128 {
				break
			}
			if i.Location != (Location{Kind: "store", Store: d.SourceStore}) || i.Definition != d.Definition || i.Owner != d.Carrier || next.Reserved(m, id) {
				continue
			}
			qty := i.Quantity
			if qty > remaining {
				qty = remaining
			}
			cid := id
			if qty < i.Quantity {
				cid = fmt.Sprintf("instance:tx:%s:%d:%d", d.Carrier, seq, len(moves))
			}
			cargo = append(cargo, Cargo{Item: cid, Quantity: qty})
			moves = append(moves, Move{Item: id, Quantity: qty, ExpectedRevision: i.Revision, To: Location{Kind: "store", Store: ActorStore(d.Carrier)}})
			remaining -= qty
		}
		if len(moves) == 0 {
			p.NextDue = d.FirstSecond + ((seconds-d.FirstSecond)/d.PeriodSeconds+1)*d.PeriodSeconds
			next.Deliveries[d.ID] = p
			continue
		}
		work := next.Clone()
		p.Phase = "carrying"
		p.Cargo = cargo
		work.Deliveries[d.ID] = p
		if _, err := work.commerceApply(m, Transaction{Actor: d.Carrier, Sequence: seq, RequestTag: fmt.Sprintf("%s/pickup/%d", d.ID, p.Cycle), Moves: moves}); err == nil {
			next = work
		}
	}
	for _, sh := range m.Shopfronts {
		p := next.Shopfronts[sh.ID]
		if seconds < p.LastSecond {
			return errors.New("shop clock reversed")
		}
		p.LastSecond = seconds
		present := sh.ServesAt(poses[sh.Owner])
		working := seconds%86400 >= sh.OpenSecond && seconds%86400 < sh.CloseSecond
		p.Break = working && poses[sh.Owner].Break
		opening := present && working && !p.Break
		// Service availability is independent of successful shelf replenishment.
		// Customers may buy existing display stock to free capacity.
		p.Open = opening
		next.Shopfronts[sh.ID] = p
		from, to := "", ""
		if opening {
			from, to = sh.ReserveStore, sh.DisplayStore
		} else if sh.PackAway && present && !p.Break {
			from, to = sh.DisplayStore, sh.ReserveStore
		}
		if from == "" || !next.shopStock(from) {
			p.Open = opening
			next.Shopfronts[sh.ID] = p
			continue
		}
		moves := []Move{}
		for _, id := range sortedKeys(next.Items) {
			i := next.Items[id]
			if from != "" && i.Location == (Location{Kind: "store", Store: from}) && i.Owner == sh.Owner {
				moves = append(moves, Move{Item: id, Quantity: i.Quantity, ExpectedRevision: i.Revision, To: Location{Kind: "store", Store: to}})
			}
		}
		if len(moves) == 0 {
			p.Open = opening
			next.Shopfronts[sh.ID] = p
			continue
		}
		work := next.Clone()
		ok := true
		if len(moves) > 0 {
			_, err := work.commerceApply(m, Transaction{Actor: sh.Owner, Sequence: work.Cursors[sh.Owner].Sequence + 1, RequestTag: fmt.Sprintf("shop/%s/%d/%t", sh.ID, seconds, opening), Moves: moves})
			ok = err == nil
		}
		if ok {
			p.Open = opening
			work.Shopfronts[sh.ID] = p
			next = work
		}
	}
	if err := next.Validate(m); err != nil {
		return err
	}
	*s = next
	return nil
}

// The owner can operate their own consignment/stock lock at the physical post.
// Only this trusted adapter unlocks it, and the lock is restored before commit.
func (s *State) commerceApply(m Model, t Transaction) (Result, error) {
	locks := map[string]bool{}
	for _, move := range t.Moves {
		for _, loc := range []Location{s.Items[move.Item].Location, move.To} {
			if loc.Kind != "store" {
				continue
			}
			st := s.Stores[loc.Store]
			if st.Locked {
				if st.Owner != t.Actor {
					return Result{}, errors.New("foreign locked stock")
				}
				locks[st.ID] = true
				st.Locked = false
				s.Stores[st.ID] = st
			}
		}
	}
	result, err := s.Apply(m, t)
	for id := range locks {
		st := s.Stores[id]
		st.Locked = true
		s.Stores[id] = st
	}
	return result, err
}
