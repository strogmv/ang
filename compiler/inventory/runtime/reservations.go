package inventory

import (
	"errors"
	"math"
)

// Reservation is an exclusive lease on a concrete physical instance. Quantity
// may be smaller than its stack; the rest remains in place until withdrawal.
// Deadlines are server ticks and never extend on a repeated request.
type ReservationPolicy struct {
	MaxTicks    int64 `json:"max_ticks"`
	MaxPerActor int   `json:"max_per_actor"`
	MaxTotal    int   `json:"max_total"`
}
type Reservation struct {
	ID, Item, Actor, Purpose string
	Quantity                 int64
	ExpectedRevision         uint64
	CreatedTick, ExpiresTick int64
}

func (s *State) Advance(tick int64) error {
	if tick < s.Clock {
		return errors.New("inventory clock moved backwards")
	}
	changed := false
	for _, r := range s.Reservations {
		if tick >= r.ExpiresTick {
			changed = true
			break
		}
	}
	if changed && s.Revision == math.MaxUint64 {
		return errors.New("revision exhausted")
	}
	s.Clock = tick
	for id, r := range s.Reservations {
		if tick >= r.ExpiresTick {
			delete(s.Reservations, id)
		}
	}
	if changed {
		s.Revision++
	}
	return nil
}

func (s State) ancestorItems(m Model, item string) []string {
	result := []string{item}
	i, ok := s.Items[item]
	if !ok {
		return result
	}
	loc := i.Location
	for depth := 0; depth < m.MaxDepth && loc.Kind == "store"; depth++ {
		store, ok := s.Stores[loc.Store]
		if !ok || store.Anchor.Kind != "item" {
			break
		}
		result = append(result, store.Anchor.Item)
		parent, ok := s.Items[store.Anchor.Item]
		if !ok {
			break
		}
		loc = parent.Location
	}
	return result
}
func containsItem(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}
func (s State) relatedReservation(m Model, item string) (Reservation, bool) {
	chain := s.ancestorItems(m, item)
	for _, id := range sortedKeys(s.Reservations) {
		r := s.Reservations[id]
		if r.ExpiresTick > s.Clock && (containsItem(chain, r.Item) || containsItem(s.ancestorItems(m, r.Item), item)) {
			return r, true
		}
	}
	return Reservation{}, false
}
func (s State) Reserved(m Model, item string) bool { _, ok := s.relatedReservation(m, item); return ok }

// ReserveBatch is invoked only after the host has authorized remote reservation
// of the disclosed stock. Every lease is checked before any lease is committed.
func (s *State) ReserveBatch(m Model, claims []Reservation) error {
	if len(claims) == 0 || len(claims) > 128 {
		return errors.New("invalid reservation batch")
	}
	next := s.Clone()
	changed := false
	for _, r := range claims {
		i, ok := next.Items[r.Item]
		if !ok || !m.Actors[r.Actor] || r.ID == "" || r.Purpose == "" || r.Quantity < 1 || r.Quantity > i.Quantity || r.ExpectedRevision != i.Revision || r.CreatedTick < 0 || r.CreatedTick > s.Clock || r.ExpiresTick <= s.Clock || r.ExpiresTick-r.CreatedTick > m.Reservation.MaxTicks {
			return errors.New("invalid reservation")
		}
		if old, exists := next.Reservations[r.Item]; exists {
			if old == r {
				continue
			}
			return errors.New("item already reserved")
		}
		if _, blocked := next.relatedReservation(m, r.Item); blocked {
			return errors.New("ancestor or contents already reserved")
		}
		if err := next.accessible(m, i.Location); err != nil {
			return err
		}
		next.Reservations[r.Item] = r
		changed = true
	}
	if !changed {
		return nil
	}
	if next.Revision == math.MaxUint64 {
		return errors.New("revision exhausted")
	}
	next.Revision++
	if err := next.validateReservations(m); err != nil {
		return err
	}
	*s = next
	return nil
}
func (s *State) ReleasePurpose(purpose string) {
	changed := false
	for id, r := range s.Reservations {
		if r.Purpose == purpose {
			delete(s.Reservations, id)
			changed = true
		}
	}
	if changed && s.Revision < math.MaxUint64 {
		s.Revision++
	}
}
func (s State) validateReservations(m Model) error {
	if s.Clock < 0 || m.Reservation.MaxTicks < 1 || m.Reservation.MaxPerActor < 1 || m.Reservation.MaxTotal < 1 || len(s.Reservations) > m.Reservation.MaxTotal {
		return errors.New("invalid reservation limits")
	}
	counts := map[string]int{}
	ids := map[string]bool{}
	for key, r := range s.Reservations {
		item, ok := s.Items[key]
		if !ok || r.Item != key || r.ID == "" || ids[r.ID] || !m.Actors[r.Actor] || r.Purpose == "" || r.Quantity < 1 || r.Quantity > item.Quantity || r.ExpectedRevision != item.Revision || r.CreatedTick < 0 || r.CreatedTick > s.Clock || r.ExpiresTick <= s.Clock || r.ExpiresTick-r.CreatedTick > m.Reservation.MaxTicks {
			return errors.New("invalid saved reservation")
		}
		ids[r.ID] = true
		counts[r.Actor]++
		if counts[r.Actor] > m.Reservation.MaxPerActor {
			return errors.New("actor reservation limit")
		}
		for _, parent := range s.ancestorItems(m, key)[1:] {
			if _, ok := s.Reservations[parent]; ok {
				return errors.New("overlapping reservation")
			}
		}
	}
	return nil
}
func (s State) allowReservedMove(m Model, actor string, move Move) error {
	if r, ok := s.relatedReservation(m, move.Item); ok {
		if r.Item != move.Item || r.Actor != actor || r.ID != move.Reservation || move.Quantity != r.Quantity || move.ExpectedRevision != r.ExpectedRevision {
			return errors.New("item reserved for another operation")
		}
	} else if move.Reservation != "" {
		return errors.New("reservation expired or missing")
	}
	if move.To.Kind == "store" {
		st, ok := s.Stores[move.To.Store]
		if ok && st.Anchor.Kind == "item" && s.Reserved(m, st.Anchor.Item) {
			return errors.New("destination container reserved")
		}
	}
	return nil
}
