package crafting

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	runtime "github.com/strogmv/ang/compiler/crafting/runtime"
)

// WorldItem is what the compiled world knows about a definition the catalog names.
type WorldItem struct {
	PriceMinor int64 // CRD minor units per unit (SLT converted at the canon reference rate); -1 when unpriced
	StackLimit int64
	Sold       bool // offered for sale at the start (a shop display, a vending showcase)
	Present    bool // exists somewhere at the start (found, owned, installed)
}

// World is the compiled world the content checks need: item facts and the localization catalog.
type World struct {
	Items     map[string]WorldItem
	Localized func(key string) bool // the key has a text in every locale
}

// Check enforces the crafting content rules of doc 93 §9 against the compiled world:
//   - every input of every recipe and every worked target is reachable: sold, present at the start, or made by a recipe
//     whose inputs are reachable (craft output, salvage of a reachable target);
//   - the price chain is closed: every named definition is priced, a product is worth at least its parts, salvage of a
//     thing never yields more than the thing is worth;
//   - every recipe leans on canon or is marked a proposal with a note;
//   - every text key exists in every locale;
//   - nothing is copied: salvage never yields the salvaged definition itself, nor more of a part class than the schematic of
//     the same product consumes.
func Check(m runtime.Model, w World) error {
	var errs []string
	fail := func(format string, args ...any) { errs = append(errs, fmt.Sprintf(format, args...)) }
	item := func(id, what string) (WorldItem, bool) {
		i, ok := w.Items[id]
		if !ok {
			fail("%s: unknown item definition %s", what, id)
		}
		return i, ok
	}
	classPrice := map[string]int64{}
	for _, p := range m.Parts {
		best := int64(-1)
		for _, pi := range p.Items {
			if i, ok := item(pi.Item, "part "+p.ID); ok {
				if i.PriceMinor < 1 {
					fail("part %s: %s has no CRD price", p.ID, pi.Item)
				} else if best < 0 || i.PriceMinor < best {
					best = i.PriceMinor
				}
			}
		}
		classPrice[p.ID] = best
	}
	for _, t := range m.Tools {
		if i, ok := item(t.Item, "tool"); ok && i.PriceMinor < 1 {
			fail("tool %s has no CRD price", t.Item)
		}
		if i := w.Items[t.Item]; !i.Sold && !i.Present {
			fail("tool %s is neither sold nor present at the start", t.Item)
		}
	}
	// reachability: a fixpoint over sold / present definitions, schematic outputs and salvage
	reach := map[string]bool{}
	for id, i := range w.Items {
		if i.Sold || i.Present {
			reach[id] = true
		}
	}
	inputOK := func(in runtime.Input) bool {
		if in.Item != "" {
			return reach[in.Item]
		}
		p, _ := m.Part(in.Class)
		for _, pi := range p.Items {
			if reach[pi.Item] {
				return true
			}
		}
		return false
	}
	for changed := true; changed; {
		changed = false
		for _, r := range m.Recipes {
			ready := r.Target == "" || reach[r.Target]
			for _, in := range r.Inputs {
				ready = ready && inputOK(in)
			}
			if !ready {
				continue
			}
			var made []string
			if r.Output != nil {
				made = append(made, r.Output.Item)
			}
			for _, y := range r.Yields {
				if p, ok := m.Part(y.Class); ok && y.Max > 0 {
					made = append(made, p.Items[0].Item)
				}
			}
			for _, id := range made {
				if !reach[id] {
					reach[id], changed = true, true
				}
			}
		}
	}
	keys := []string{}
	for _, d := range m.Disciplines {
		keys = append(keys, d.Name)
		for _, t := range d.Tiers {
			keys = append(keys, t.Name)
		}
	}
	for _, p := range m.Parts {
		keys = append(keys, p.Name)
	}
	for _, s := range m.Stations {
		keys = append(keys, s.Name)
	}
	for _, k := range m.Knowledge {
		keys = append(keys, k.Name)
	}
	for _, s := range m.Masters {
		keys = append(keys, s.Name)
	}
	for _, r := range m.Recipes {
		keys = append(keys, r.Name, r.Hint)
		if r.Target != "" {
			if t, ok := item(r.Target, "recipe "+r.ID+" target"); ok {
				if !reach[r.Target] {
					fail("recipe %s: target %s is unreachable (not sold, not present, not made)", r.ID, r.Target)
				}
				if r.Heirloom && t.StackLimit != 1 {
					fail("recipe %s: an heirloom must be a unique item", r.ID)
				}
			}
		}
		var parts int64
		for _, in := range r.Inputs {
			if in.Item != "" {
				if i, ok := item(in.Item, "recipe "+r.ID+" input"); ok {
					if i.PriceMinor < 0 {
						fail("recipe %s: input %s has no CRD price", r.ID, in.Item)
					}
					parts += in.N * max(i.PriceMinor, 0)
				}
			} else {
				parts += in.N * max(classPrice[in.Class], 0)
			}
			if !inputOK(in) {
				fail("recipe %s: input %s%s is unreachable (not sold, not present, not made or salvaged)", r.ID, in.Class, in.Item)
			}
		}
		if r.Output != nil {
			if out, ok := item(r.Output.Item, "recipe "+r.ID+" output"); ok {
				if out.PriceMinor < 1 {
					fail("recipe %s: output %s has no CRD price", r.ID, r.Output.Item)
				} else if out.PriceMinor*r.Output.N < parts {
					fail("recipe %s: the product (%d) is worth less than its parts (%d)", r.ID, out.PriceMinor*r.Output.N, parts)
				}
				for _, in := range r.Inputs {
					if in.Item == r.Output.Item {
						fail("recipe %s: the product is its own input", r.ID)
					}
				}
			}
		}
		if r.Kind == "disassemble" {
			var salvage int64
			for _, y := range r.Yields {
				p, _ := m.Part(y.Class)
				for _, pi := range p.Items {
					if pi.Item == r.Target {
						fail("recipe %s: salvage yields the salvaged definition itself", r.ID)
					}
				}
				salvage += y.Max * max(classPrice[y.Class], 0)
			}
			if t := w.Items[r.Target]; t.PriceMinor >= 0 && salvage > t.PriceMinor {
				fail("recipe %s: salvage is worth %d, more than the thing (%d)", r.ID, salvage, t.PriceMinor)
			}
			for _, c := range m.Recipes {
				if c.Output == nil || c.Output.Item != r.Target {
					continue
				}
				used := map[string]int64{}
				for _, in := range c.Inputs {
					if in.Class != "" {
						used[in.Class] += in.N
					}
				}
				for _, y := range r.Yields {
					if y.Max*c.Output.N > used[y.Class] {
						fail("recipe %s copies parts: up to %d %s from a product schematic %s builds with %d", r.ID, y.Max, y.Class, c.ID, used[y.Class])
					}
				}
			}
		}
		canon, noted := false, false
		for _, s := range r.Sources {
			canon = canon || strings.HasPrefix(s.Path, "canon/") || strings.HasPrefix(s.Path, "story/") || strings.HasPrefix(s.Path, "content/book/")
			noted = noted || strings.TrimSpace(s.Note) != ""
		}
		if r.Basis == "canon" && !canon {
			fail("recipe %s: basis canon without a canon or story source", r.ID)
		}
		if r.Basis == "proposal" && !noted {
			fail("recipe %s: a proposal must say so in a source note", r.ID)
		}
	}
	keys = append(keys, checkSources(m, w, fail)...)
	if w.Localized != nil {
		missing := map[string]bool{}
		for _, k := range keys {
			if k != "" && !w.Localized(k) {
				missing[k] = true
			}
		}
		list := make([]string, 0, len(missing))
		for k := range missing {
			list = append(list, k)
		}
		sort.Strings(list)
		for _, k := range list {
			fail("localization key %s has no text in every locale", k)
		}
	}
	if len(errs) > 0 {
		return errors.New("crafting content:\n  " + strings.Join(errs, "\n  "))
	}
	return nil
}
