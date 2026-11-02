package crafting

import runtime "github.com/strogmv/ang/compiler/crafting/runtime"

// checkSources enforces the stage-2 content rules (doc 93 §4, doc 84 п. 21) against the compiled world:
//   - every schematic carrier is reachable (sold, present at the start) and priced: a source the player can meet;
//   - every source has an owner and a place: a carrier at a stall / a store of an owner / a cache with an entry cell,
//     a teaching master with the schematic in clean memory, a person who tells it;
//   - every recipe with no general (tier / start) source has at least one source that runs;
//   - the rebinding has a law: a bypass recipe needs the acts of the justice ladder with a rung and sanctions, the
//     system trust above its start and a terminal of the vending block;
//   - the names of flaws, caches and crimes are translated.
func checkSources(m runtime.Model, w World, fail func(string, ...any)) []string {
	keys := []string{}
	for _, c := range m.Sources.Carriers {
		i, ok := w.Items[c.Item]
		switch {
		case !ok:
			fail("carrier %s: unknown item definition", c.Item)
		case !i.Sold && !i.Present:
			fail("carrier %s is neither sold nor present at the start: nobody can learn %s from it", c.Item, c.Teaches)
		case i.PriceMinor < 1:
			fail("carrier %s has no CRD price", c.Item)
		}
		if c.Place.Kind == "store" && c.Place.Owner == "" || c.Place.Kind == "shop" && c.Place.Shop == "" {
			fail("carrier %s has no owner or place", c.Item)
		}
	}
	for _, c := range m.Sources.Caches {
		keys = append(keys, c.Name)
		if c.Level == "" || c.Store == "" {
			fail("cache %s has no place in the hub", c.ID)
		}
	}
	for _, f := range m.Sources.Flaws {
		keys = append(keys, f.Name)
	}
	for _, r := range m.Recipes {
		if r.Kind == "bypass" {
			hack, ok := m.Act("hack")
			if !ok || hack.Rung == "" || len(hack.Sanctions) == 0 || hack.Crime == "" {
				fail("recipe %s: a chip bypass without a crime of the justice ladder", r.ID)
			}
			if m.Law.Trust.RebindMinPermille <= m.Law.Trust.StartPermille || len(m.Law.Rebind.Terminals) == 0 || m.Law.Rebind.Account == "" {
				fail("recipe %s: the lawful rebinding needs a trust threshold above the start and a terminal with an account", r.ID)
			}
		}
		general := false
		for _, l := range r.Learn {
			general = general || l.Kind == "tier" || l.Kind == "start"
		}
		if general {
			continue
		}
		runs := false
		for _, l := range r.Learn {
			switch l.Kind {
			case "copy", "manual", "find":
				i := w.Items[l.From]
				runs = runs || i.Sold || i.Present
			case "teardown":
				i := w.Items[l.From]
				runs = runs || i.Sold || i.Present
			default:
				runs = true // apprentice / oral: the engine's Validate checked the teacher knows it
			}
		}
		for _, ms := range m.Masters {
			for _, k := range ms.Known {
				runs = runs || k == r.ID
			}
		}
		if !runs {
			fail("recipe %s: no source of its schematic can be met in the world", r.ID)
		}
	}
	for _, a := range m.Law.Acts {
		keys = append(keys, a.Name)
	}
	return keys
}
