package gamepoc

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Market people v1, part B (the game's art-source/market-people/v1/briefs/02-bronze-traders.md): parts of a model that only
// some wearers wear, the age marks of the face, colourways by age, and the general binding of people of the world to a
// crowd look by caste, body and age.

// LookParts: the parts (part_* surfaces of the model's record) a wearer of the look wears. Wear: every wearer. Choose: groups
// of options of which each wearer wears one, picked by the hash of his id like his colourway; an option is a list of parts,
// empty for none. A part surface the look does not name is not drawn on its wearers.
type LookParts struct {
	Wear   []string                       `json:"wear"`
	Choose map[string]map[string][]string `json:"choose,omitempty"`
}

// LookAgeing: the surface whose second UV set holds the face painted older; none up to FromAge, all of it from FullAge.
type LookAgeing struct {
	Surface string `json:"surface"`
	FromAge int64  `json:"from_age"`
	FullAge int64  `json:"full_age"`
}

// PersonPopulation: a person of the world drawn as a wearer of a population look, with his age from data.
type PersonPopulation struct {
	Population string `json:"population"`
	Age        int64  `json:"age"`
}

var (
	lookPart       = regexp.MustCompile(`^part_[a-z0-9_]+$`)
	lookPlainName  = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	personID       = regexp.MustCompile(`^char\.[a-z0-9_.]+$`)
	ageMin, ageMax = int64(16), int64(90)
)

// checkLookParts: every named part is a part_ surface of the model (never a glow), named once over wear and all options;
// option and group names are plain; a group offers at least two options. Ageing names a surface of the model that is not a
// glow and carries a second UV set, with adult ages from < full.
func checkLookParts(key string, l Look, surfaces map[string]string, uv2 []string, record string) error {
	if p := l.Parts; p != nil {
		seen := map[string]string{}
		use := func(part, where string) error {
			if !lookPart.MatchString(part) || surfaces[part] == "" {
				return fmt.Errorf("art.looks %s: part %q (%s) is not a part_ surface of model %s in %s", key, part, where, l.Model, record)
			}
			if w, ok := seen[part]; ok && w != where {
				return fmt.Errorf("art.looks %s: part %s is named twice (%s and %s)", key, part, w, where)
			}
			seen[part] = where
			return nil
		}
		for _, part := range p.Wear {
			if err := use(part, "wear"); err != nil {
				return err
			}
		}
		for group, options := range p.Choose {
			if !lookPlainName.MatchString(group) || len(options) < 2 {
				return fmt.Errorf("art.looks %s: parts.choose %q needs a plain name and at least two options", key, group)
			}
			for opt, parts := range options {
				if !lookPlainName.MatchString(opt) {
					return fmt.Errorf("art.looks %s: parts.choose %s: option %q needs a plain name", key, group, opt)
				}
				for _, part := range parts {
					if err := use(part, "choose "+group); err != nil {
						return err
					}
				}
			}
		}
	}
	if a := l.Ageing; a != nil {
		has := false
		for _, s := range uv2 {
			has = has || s == a.Surface
		}
		switch {
		case surfaces[a.Surface] == "" || strings.HasPrefix(a.Surface, "glow_"):
			return fmt.Errorf("art.looks %s: ageing surface %q is not a surface of model %s in %s that does not glow", key, a.Surface, l.Model, record)
		case !has:
			return fmt.Errorf("art.looks %s: ageing surface %s has no second UV set in %s (the older painting)", key, a.Surface, record)
		case a.FromAge < ageMin || a.FullAge > ageMax || a.FromAge >= a.FullAge:
			return fmt.Errorf("art.looks %s: ageing from_age..full_age must lie within %d..%d with from_age < full_age", key, ageMin, ageMax)
		}
	}
	return nil
}

// clientLookParts: wear sorted; each group's options sorted by name (the client's pick by id is the same on every machine).
func clientLookParts(p LookParts) map[string]any {
	wear := append([]string{}, p.Wear...)
	sort.Strings(wear)
	groups := make([]string, 0, len(p.Choose))
	for g := range p.Choose {
		groups = append(groups, g)
	}
	sort.Strings(groups)
	choose := make([]map[string]any, 0, len(groups))
	for _, g := range groups {
		opts := make([]string, 0, len(p.Choose[g]))
		for o := range p.Choose[g] {
			opts = append(opts, o)
		}
		sort.Strings(opts)
		rows := make([]map[string]any, 0, len(opts))
		for _, o := range opts {
			parts := append([]string{}, p.Choose[g][o]...)
			sort.Strings(parts)
			rows = append(rows, map[string]any{"id": o, "parts": parts})
		}
		choose = append(choose, map[string]any{"id": g, "options": rows})
	}
	return map[string]any{"wear": wear, "choose": choose}
}

// checkPopulationAges: each band starts at an adult age, bands start at different ages, and names colourways of the look
// with shares 1..100.
func checkPopulationAges(name string, p PopulationLook, l Look) error {
	from := map[int64]bool{}
	for _, band := range p.Ages {
		if band.FromAge < ageMin || band.FromAge > ageMax || from[band.FromAge] {
			return fmt.Errorf("art.population_looks %s: ages: each band starts at a different age within %d..%d", name, ageMin, ageMax)
		}
		from[band.FromAge] = true
		if len(band.Colourways) == 0 {
			return fmt.Errorf("art.population_looks %s: the age band from %d names no colourways", name, band.FromAge)
		}
		for cw, share := range band.Colourways {
			if _, ok := l.Colourways[cw]; !ok {
				return fmt.Errorf("art.population_looks %s: ages from %d: %q is not a colourway of look %s", name, band.FromAge, cw, p.Look)
			}
			if share < 1 || share > 100 {
				return fmt.Errorf("art.population_looks %s: ages from %d: the share of colourway %s must be within 1..100", name, band.FromAge, cw)
			}
		}
		for group, options := range band.Choose {
			var offered map[string][]string
			if l.Parts != nil {
				offered = l.Parts.Choose[group]
			}
			if offered == nil || len(options) == 0 {
				return fmt.Errorf("art.population_looks %s: ages from %d: choose %q is not a parts.choose group of look %s, or names no option", name, band.FromAge, group, p.Look)
			}
			seen := map[string]bool{}
			for _, o := range options {
				if _, ok := offered[o]; !ok || seen[o] {
					return fmt.Errorf("art.population_looks %s: ages from %d: choose %s: %q is not an option of look %s (or named twice)", name, band.FromAge, group, o, p.Look)
				}
				seen[o] = true
			}
		}
	}
	return nil
}

// checkPopulationParts (job 4): each group is a parts.choose group of the look, every one of its options is named once with
// a share 0..100, and at least one share is above 0.
func checkPopulationParts(name string, p PopulationLook, l Look) error {
	for group, shares := range p.Parts {
		var offered map[string][]string
		if l.Parts != nil {
			offered = l.Parts.Choose[group]
		}
		if offered == nil {
			return fmt.Errorf("art.population_looks %s: parts %q is not a parts.choose group of look %s", name, group, p.Look)
		}
		total := int64(0)
		for opt, share := range shares {
			if _, ok := offered[opt]; !ok {
				return fmt.Errorf("art.population_looks %s: parts %s: %q is not an option of look %s", name, group, opt, p.Look)
			}
			if share < 0 || share > 100 {
				return fmt.Errorf("art.population_looks %s: parts %s: the share of option %s must be within 0..100", name, group, opt)
			}
			total += share
		}
		for opt := range offered {
			if _, ok := shares[opt]; !ok {
				return fmt.Errorf("art.population_looks %s: parts %s: option %s of look %s has no share (name every option; 0 for never)", name, group, opt, p.Look)
			}
		}
		if total == 0 {
			return fmt.Errorf("art.population_looks %s: parts %s: every share is 0", name, group)
		}
	}
	return nil
}

// clientPopulationParts: groups sorted, each group's options sorted by name with their shares (the pick by id is the same on
// every machine).
func clientPopulationParts(parts map[string]map[string]int64) []map[string]any {
	groups := make([]string, 0, len(parts))
	for g := range parts {
		groups = append(groups, g)
	}
	sort.Strings(groups)
	out := make([]map[string]any, 0, len(groups))
	for _, g := range groups {
		opts := make([]string, 0, len(parts[g]))
		for o := range parts[g] {
			opts = append(opts, o)
		}
		sort.Strings(opts)
		rows := make([]map[string]any, 0, len(opts))
		for _, o := range opts {
			rows = append(rows, map[string]any{"id": o, "share": parts[g][o]})
		}
		out = append(out, map[string]any{"id": g, "options": rows})
	}
	return out
}

func clientPopulationAges(ages []PopulationAge) []map[string]any {
	bands := append([]PopulationAge{}, ages...)
	sort.Slice(bands, func(i, j int) bool { return bands[i].FromAge < bands[j].FromAge })
	out := make([]map[string]any, 0, len(bands))
	for _, b := range bands {
		cws := make([]string, 0, len(b.Colourways))
		for cw := range b.Colourways {
			cws = append(cws, cw)
		}
		sort.Strings(cws)
		rows := make([]map[string]any, 0, len(cws))
		for _, cw := range cws {
			rows = append(rows, map[string]any{"id": cw, "share": b.Colourways[cw]})
		}
		row := map[string]any{"from_age": b.FromAge, "colourways": rows}
		if len(b.Choose) > 0 {
			choose := map[string][]string{}
			for g, opts := range b.Choose {
				o := append([]string{}, opts...)
				sort.Strings(o)
				choose[g] = o
			}
			row["choose"] = choose
		}
		out = append(out, row)
	}
	return out
}

// checkPersonPopulations: each person exists, is not also bound to a look of his own (art.person_looks: one way to dress a
// person), names a known population look and has an adult age.
func checkPersonPopulations(a Art, person func(string) bool) error {
	for id, pp := range a.PersonPopulations {
		if !personID.MatchString(id) || !person(id) {
			return fmt.Errorf("art.person_populations %s: no such person in world.persons", id)
		}
		if _, ok := a.PersonLooks[id]; ok {
			return fmt.Errorf("art.person_populations %s: the person also has a look of his own in art.person_looks", id)
		}
		_, look := a.PopulationLooks[pp.Population]
		_, rule := a.OutfitRules[pp.Population]
		if !look && !rule {
			return fmt.Errorf("art.person_populations %s: unknown population look or outfit rule %q", id, pp.Population)
		}
		if pp.Age < ageMin || pp.Age > ageMax {
			return fmt.Errorf("art.person_populations %s: age %d is not within %d..%d", id, pp.Age, ageMin, ageMax)
		}
	}
	return nil
}

// clientPersonPopulations: one row per person (sorted by id): his population look and age.
func clientPersonPopulations(a Art) []map[string]any {
	ids := make([]string, 0, len(a.PersonPopulations))
	for id := range a.PersonPopulations {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		pp := a.PersonPopulations[id]
		out = append(out, map[string]any{"id": id, "population": pp.Population, "age": pp.Age})
	}
	return out
}
