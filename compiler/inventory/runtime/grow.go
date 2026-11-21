package inventory

import "sort"

// Growth is what Grow added to a saved inventory, by id.
type Growth struct {
	Accounts     []string
	Stores       []string
	Entitlements []string
	Deliveries   []string
	Shopfronts   []string
	Items        []string
}

// Grow (saves growth, owner 20.11) adds to a saved inventory what belongs to owners new to it (new residents), and
// nothing else: whatever else the save lacks still refuses in Validate, as before.
//   - their accounts, static stores, entitlements, deliveries and shop fronts the save lacks, at their initial values
//     (a delivery's first run: the next one due after now, not a burst of the missed ones);
//   - the things of the sources placed in the stores added here, exactly as New creates them for this seed (the same
//     instance ids, loot rolls and containers).
//
// A store the save already had keeps what the save has: if its owner becomes a resident only now, the stock the player
// took from it does not come back; and a thing already in the save under the same id is never created twice.
func (s *State) Grow(m Model, seed, now int64, owners map[string]bool) (Growth, error) {
	var g Growth
	if len(owners) == 0 {
		return g, nil
	}
	if s.Accounts == nil {
		s.Accounts = map[string]Account{}
	}
	if s.Stores == nil {
		s.Stores = map[string]Store{}
	}
	if s.Entitlements == nil {
		s.Entitlements = map[string]Entitlement{}
	}
	if s.Items == nil {
		s.Items = map[string]Item{}
	}
	for _, id := range sortedKeys(m.Accounts) {
		if _, ok := s.Accounts[id]; !ok && owners[m.Accounts[id].Owner] {
			s.Accounts[id] = m.Accounts[id]
			g.Accounts = append(g.Accounts, id)
		}
	}
	added := map[string]bool{}
	for _, id := range sortedKeys(m.Stores) {
		st := m.Stores[id]
		if _, ok := s.Stores[id]; !ok && (owners[st.Owner] || owners[st.Anchor.Actor]) {
			s.Stores[id] = st
			g.Stores = append(g.Stores, id)
			added[id] = true
		}
	}
	for _, id := range sortedKeys(m.Entitlements) {
		if _, ok := s.Entitlements[id]; !ok && owners[m.Entitlements[id].Actor] {
			s.Entitlements[id] = m.Entitlements[id]
			g.Entitlements = append(g.Entitlements, id)
		}
	}
	for _, d := range m.Deliveries {
		if _, ok := s.Deliveries[d.ID]; ok || !owners[d.Carrier] {
			continue
		}
		due := d.FirstSecond
		if now > due && d.PeriodSeconds > 0 {
			due += (now - due + d.PeriodSeconds - 1) / d.PeriodSeconds * d.PeriodSeconds
		}
		if s.Deliveries == nil {
			s.Deliveries = map[string]DeliveryProgress{}
		}
		s.Deliveries[d.ID] = DeliveryProgress{NextDue: due}
		g.Deliveries = append(g.Deliveries, d.ID)
	}
	for _, sh := range m.Shopfronts {
		if _, ok := s.Shopfronts[sh.ID]; ok || !owners[sh.Owner] {
			continue
		}
		if s.Shopfronts == nil {
			s.Shopfronts = map[string]ShopProgress{}
		}
		s.Shopfronts[sh.ID] = ShopProgress{}
		g.Shopfronts = append(g.Shopfronts, sh.ID)
	}
	if len(added) == 0 {
		return g, nil
	}
	// the sources placed in the stores added here, and then in the containers their things bring
	mine := m
	mine.Sources = nil
	chosen := map[string]bool{}
	for {
		grew := false
		for _, src := range m.Sources {
			if !chosen[src.ID] && src.Location.Kind == "store" && added[src.Location.Store] {
				chosen[src.ID] = true
				mine.Sources = append(mine.Sources, src)
				grew = true
			}
		}
		if !grew {
			break
		}
		fresh, err := New(mine, seed)
		if err != nil {
			return g, err
		}
		for id := range fresh.Items {
			if _, ok := fresh.Stores[ItemStore(id)]; ok {
				added[ItemStore(id)] = true
			}
		}
	}
	if len(mine.Sources) == 0 {
		return g, nil
	}
	fresh, err := New(mine, seed)
	if err != nil {
		return g, err
	}
	ids := make([]string, 0, len(fresh.Items))
	for id, item := range fresh.Items {
		if chosen[item.Origin] {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		if _, ok := s.Items[id]; ok {
			continue
		}
		s.Items[id] = fresh.Items[id]
		if store, ok := fresh.Stores[ItemStore(id)]; ok {
			if _, had := s.Stores[ItemStore(id)]; !had {
				s.Stores[ItemStore(id)] = store
				g.Stores = append(g.Stores, ItemStore(id))
			}
		}
		g.Items = append(g.Items, id)
	}
	return g, nil
}
