package gamepoc

import (
	"fmt"
	"math"
	"regexp"
	"sort"
)

// Body shapes (characters v1, job 05: real clothes; one set of clothes on many bodies). A look's model may carry blend shapes
// of the body — the same MakeHuman body thin, heavy... (mpfb_body.py) — and every garment of the model carries them too
// (build_character.py binds each vertex to the body's surface), so a heavier wearer stays dressed. The record (look.json)
// names them; the data picks them: a crowd binding (art.population_looks[].shapes) by shares and amounts per wearer, a person
// with his own look by art.person_shapes. The client sets the blend shapes (actor_looks.gd set_shapes).

// PopulationShape is one option of a crowd binding's body shapes: its share (0..100) and the amount range (0..1); the
// option `none` (the body as built) has no amount.
type PopulationShape struct {
	Share int64   `json:"share"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
}

var shapeName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// lookShapeNone is the crowd option of no shape (the body as built).
const lookShapeNone = "none"

// checkLookShapes: the record's shape names are plain; every binding of this look names only them (or none), with shares
// 0..100 summing above 0 and amounts 0 <= min <= max <= 1; every person who wears this look as his own gives only them,
// with amounts 0..1.
func checkLookShapes(key string, a Art, shapes []string, record string) error {
	has := map[string]bool{}
	for _, s := range shapes {
		if !shapeName.MatchString(s) || s == lookShapeNone || has[s] {
			return fmt.Errorf("art.looks %s: record %s: shape %q must be a plain unique name (not %s)", key, record, s, lookShapeNone)
		}
		has[s] = true
	}
	names := make([]string, 0, len(a.PopulationLooks))
	for n := range a.PopulationLooks {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		p := a.PopulationLooks[n]
		if p.Look != key || len(p.Shapes) == 0 {
			continue
		}
		total := int64(0)
		for id, o := range p.Shapes {
			if id != lookShapeNone && !has[id] {
				return fmt.Errorf("art.population_looks %s: shape %q is not a body shape of look %s (record %s: %v)", n, id, key, record, shapes)
			}
			if o.Share < 0 || o.Share > 100 {
				return fmt.Errorf("art.population_looks %s: the share of shape %s must be within 0..100", n, id)
			}
			total += o.Share
			if id == lookShapeNone {
				if o.Min != 0 || o.Max != 0 {
					return fmt.Errorf("art.population_looks %s: shape none has no amount", n)
				}
				continue
			}
			if math.IsNaN(o.Min) || math.IsNaN(o.Max) || o.Min < 0 || o.Max > 1 || o.Min > o.Max {
				return fmt.Errorf("art.population_looks %s: shape %s: amounts 0 <= min <= max <= 1", n, id)
			}
		}
		if total == 0 {
			return fmt.Errorf("art.population_looks %s: the shapes' shares sum to 0", n)
		}
	}
	for id, values := range a.PersonShapes {
		if a.PersonLooks[id] != key {
			continue
		}
		for s, v := range values {
			if !has[s] {
				return fmt.Errorf("art.person_shapes %s: shape %q is not a body shape of his look %s (record %s: %v)", id, s, key, record, shapes)
			}
			if math.IsNaN(v) || v < 0 || v > 1 {
				return fmt.Errorf("art.person_shapes %s: shape %s: an amount within 0..1", id, s)
			}
		}
	}
	return nil
}

// checkPersonShapes: a person with body shapes wears a look of his own (art.person_looks) that exists.
func checkPersonShapes(a Art) error {
	for id := range a.PersonShapes {
		look, ok := a.PersonLooks[id]
		if !ok {
			return fmt.Errorf("art.person_shapes %s: the person wears no look of his own (art.person_looks)", id)
		}
		if _, ok := a.Looks[look]; !ok {
			return fmt.Errorf("art.person_shapes %s: unknown look %q", id, look)
		}
	}
	return nil
}

// clientPopulationShapes: the options sorted by id (the client's pick by hash is the same on every machine).
func clientPopulationShapes(shapes map[string]PopulationShape) []map[string]any {
	ids := make([]string, 0, len(shapes))
	for id := range shapes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		o := shapes[id]
		out = append(out, map[string]any{"id": id, "share": o.Share, "min": o.Min, "max": o.Max})
	}
	return out
}
