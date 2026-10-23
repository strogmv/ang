package inventory

import "errors"

// Load counts the shell of every carried container and every nested physical
// item exactly once. Outer volume excludes contents already inside a container.
type Load struct {
	Mass, Volume int64
	Stacks       int
}

func (s State) ActorLoad(m Model, actor string) (Load, error) {
	var result Load
	if !m.Actors[actor] {
		return result, errors.New("unknown actor")
	}
	for _, i := range s.Items {
		root, err := s.Root(m, i.Location)
		if err != nil {
			return Load{}, err
		}
		if root.Kind != "actor" || root.Actor != actor {
			continue
		}
		d, ok := m.Definitions[i.Definition]
		if !ok {
			return Load{}, errors.New("unknown item definition")
		}
		mass, err := times(i.Quantity, d.Mass)
		if err != nil {
			return Load{}, err
		}
		result.Mass, err = plus(result.Mass, mass)
		if err != nil {
			return Load{}, err
		}
		if i.Location.Kind == "store" && i.Location.Store == ActorStore(actor) {
			v, err := times(i.Quantity, d.Volume)
			if err != nil {
				return Load{}, err
			}
			result.Volume, err = plus(result.Volume, v)
			if err != nil {
				return Load{}, err
			}
			result.Stacks++
		}
	}
	return result, nil
}
