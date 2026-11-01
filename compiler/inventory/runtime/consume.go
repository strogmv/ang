package inventory

import "errors"

// Consume removes a finite quantity already owned and carried by the actor.
// It cannot affect another owner's goods, reservations, containers or money.
func (s *State) Consume(m Model, actor, id string, quantity int64) error {
	item, ok := s.Items[id]
	if !ok || !m.Actors[actor] || item.Owner != actor || quantity < 1 || quantity > item.Quantity || !s.carriedBy(m, item, actor) || s.Reserved(m, id) || m.Definitions[item.Definition].ContainerVolume > 0 {
		return errors.New("invalid consumption")
	}
	item.Quantity -= quantity
	if item.Quantity == 0 {
		delete(s.Items, id)
	} else {
		item.Revision++
		s.Items[id] = item
	}
	s.Revision++
	return nil
}
