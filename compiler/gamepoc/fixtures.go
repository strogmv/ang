package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"cuelang.org/go/cue"
)

// Walkers v1 (world.fixtures): what never moves on a level belongs to its floor. Every fixture placed on a level bound to
// a Life location — an SDK module instance of the level's layout, or a kit placed outside the SDK (world.fixtures.placed:
// the gallery halves of the hall) — is checked from its kit data (rectangle, footprint, doors), never from hand-listed
// cells:
//   - a floor fixture has a footprint, and no footprint cell is floor for anybody;
//   - an overhead, wall-mounted or zone fixture has no footprint;
//   - the walkable cells of its rectangle that the open floor reaches only through its doors are its inside: declared
//     "work" (the people whose posts, routine steps or own brain places lie there are its workers, everybody else stays
//     out) or "walk_in" (a room of its users); an undeclared inside is a build error;
//   - no public place (a brain place for anybody, a visitors' entry, stall or panel cell, an exit or entry of an edge, a
//     marker, a vending panel) lies in work cells; population anchors leave them out.
// The work areas go to life.locations[level].work: {id, cells, workers}; the host keeps everybody else off them.

type FixtureRule struct {
	Mount  string `json:"mount"`
	Inside string `json:"inside,omitempty"`
	Reason string `json:"reason"`
}
type FixtureRect struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}
type PlacedFixture struct {
	Level     string      `json:"level"`
	ID        string      `json:"id"`
	Kit       string      `json:"kit"`
	Rect      FixtureRect `json:"rect"`
	Footprint []Point     `json:"footprint"`
	Doors     []Point     `json:"doors"`
	Reason    string      `json:"reason"`
}
type FixtureCatalog struct {
	Rules  map[string]FixtureRule `json:"rules"`
	Placed []PlacedFixture        `json:"placed"`
}

type lifeWorkArea struct {
	ID      string   `json:"id"`
	Cells   []Point  `json:"cells"`
	Workers []string `json:"workers"`
}

func readFixtures(world cue.Value) (*FixtureCatalog, error) {
	v := world.LookupPath(cue.ParsePath("fixtures"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.fixtures: %w", err)
	}
	var out FixtureCatalog
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&out); err != nil {
		return nil, fmt.Errorf("world.fixtures: %w", err)
	}
	return &out, nil
}

type fixtureOnLevel struct {
	level, id, kit string
	rect           FixtureRect
	footprint      []Point
	doors          []Point
}

// fixtureInside: the walkable cells of the rectangle that the open floor outside it does not reach without crossing one of
// the fixture's doors, plus those doors when there is such an inside.
func fixtureInside(terrain []string, walks func(byte) bool, f fixtureOnLevel) []Point {
	in := func(p Point) bool {
		return p.X >= f.rect.X && p.Y >= f.rect.Y && p.X < f.rect.X+f.rect.W && p.Y < f.rect.Y+f.rect.H
	}
	floor := func(p Point) bool {
		return p.Y >= 0 && p.Y < len(terrain) && p.X >= 0 && p.X < len(terrain[p.Y]) && walks(terrain[p.Y][p.X])
	}
	door := map[Point]bool{}
	for _, d := range f.doors {
		door[d] = true
	}
	steps := []Point{{X: 1}, {Y: 1}, {X: -1}, {Y: -1}}
	reached := map[Point]bool{}
	var queue []Point
	for y := f.rect.Y; y < f.rect.Y+f.rect.H; y++ {
		for x := f.rect.X; x < f.rect.X+f.rect.W; x++ {
			p := Point{X: x, Y: y}
			if !floor(p) || door[p] {
				continue
			}
			for _, d := range steps {
				q := Point{X: x + d.X, Y: y + d.Y}
				if !in(q) && floor(q) && !door[q] {
					reached[p] = true
					queue = append(queue, p)
					break
				}
			}
		}
	}
	for head := 0; head < len(queue); head++ {
		for _, d := range steps {
			q := Point{X: queue[head].X + d.X, Y: queue[head].Y + d.Y}
			if in(q) && floor(q) && !door[q] && !reached[q] {
				reached[q] = true
				queue = append(queue, q)
			}
		}
	}
	var inside []Point
	for y := f.rect.Y; y < f.rect.Y+f.rect.H; y++ {
		for x := f.rect.X; x < f.rect.X+f.rect.W; x++ {
			p := Point{X: x, Y: y}
			if floor(p) && !door[p] && !reached[p] {
				inside = append(inside, p)
			}
		}
	}
	if len(inside) == 0 {
		return nil
	}
	for y := f.rect.Y; y < f.rect.Y+f.rect.H; y++ {
		for x := f.rect.X; x < f.rect.X+f.rect.W; x++ {
			if p := (Point{X: x, Y: y}); door[p] && floor(p) {
				inside = append(inside, p)
			}
		}
	}
	sort.Slice(inside, func(i, j int) bool {
		if inside[i].Y != inside[j].Y {
			return inside[i].Y < inside[j].Y
		}
		return inside[i].X < inside[j].X
	})
	return inside
}

// applyFixtures runs after the visitors (their places are checked against the work cells) and before crafting.
func applyFixtures(c *Compiled, in *FixtureCatalog) error {
	if in == nil {
		return nil
	}
	fail := func(format string, args ...any) error {
		return fmt.Errorf("world.fixtures: %s", fmt.Sprintf(format, args...))
	}
	var life map[string]json.RawMessage
	if err := json.Unmarshal(c.Life, &life); err != nil {
		return err
	}
	var locations map[string]map[string]json.RawMessage
	if err := json.Unmarshal(life["locations"], &locations); err != nil {
		return err
	}
	walks := func(g byte) bool { return c.Walkable[string(g)] }
	terrainOf := map[string][]string{}
	terrain := func(level string) ([]string, error) {
		if t, ok := terrainOf[level]; ok {
			return t, nil
		}
		loc, ok := locations[level]
		if !ok {
			return nil, fail("unknown level %s", level)
		}
		var t []string
		if err := json.Unmarshal(loc["terrain"], &t); err != nil {
			return nil, err
		}
		terrainOf[level] = t
		return t, nil
	}
	floorAt := func(t []string, p Point) bool {
		return p.Y >= 0 && p.Y < len(t) && p.X >= 0 && p.X < len(t[p.Y]) && walks(t[p.Y][p.X])
	}
	var fixtures []fixtureOnLevel
	for _, b := range c.ModularLocations {
		layout := layoutOf(c.LocationSDK, b.LayoutID)
		if layout == nil {
			continue
		}
		doors := map[string][]Point{}
		for _, port := range layout.Ports {
			if port.Kind != "door" {
				continue
			}
			doors[port.InstanceID] = append(doors[port.InstanceID], Point{X: port.At.X, Y: port.At.Y})
			for _, o := range port.Opening {
				doors[port.InstanceID] = append(doors[port.InstanceID], Point{X: o.X, Y: o.Y})
			}
		}
		for _, i := range layout.Instances {
			f := fixtureOnLevel{level: b.LevelID, id: i.ID, kit: i.ModuleID, rect: FixtureRect{X: i.At.X, Y: i.At.Y, W: i.Width, H: i.Depth}, doors: doors[i.ID]}
			for _, p := range i.Footprint {
				f.footprint = append(f.footprint, Point{X: p.X, Y: p.Y})
			}
			fixtures = append(fixtures, f)
		}
	}
	for _, p := range in.Placed {
		if _, ok := locations[p.Level]; !ok {
			return fail("placed fixture %s: unknown level %s", p.ID, p.Level)
		}
		fixtures = append(fixtures, fixtureOnLevel{level: p.Level, id: p.ID, kit: p.Kit, rect: p.Rect, footprint: p.Footprint, doors: p.Doors})
	}
	for kit, rule := range in.Rules {
		switch rule.Mount {
		case "floor", "overhead", "wall", "zone":
		default:
			return fail("rule %s: unknown mount %q", kit, rule.Mount)
		}
		if rule.Inside != "" && (rule.Mount != "floor" || (rule.Inside != "work" && rule.Inside != "walk_in")) {
			return fail("rule %s: inside %q of a %s fixture", kit, rule.Inside, rule.Mount)
		}
	}
	work := map[string][]lifeWorkArea{}
	ids := map[string]bool{}
	for _, f := range fixtures {
		key := f.level + "/" + f.id
		if ids[key] {
			return fail("fixture %s twice on %s", f.id, f.level)
		}
		ids[key] = true
		rule, ok := in.Rules[f.kit]
		if !ok {
			rule = FixtureRule{Mount: "floor"}
		}
		t, err := terrain(f.level)
		if err != nil {
			return err
		}
		if rule.Mount != "floor" {
			if len(f.footprint) > 0 {
				return fail("%s (%s) on %s is declared %s but stands on %d floor cells", f.id, f.kit, f.level, rule.Mount, len(f.footprint))
			}
			continue
		}
		if len(f.footprint) == 0 {
			return fail("%s (%s) on %s has no footprint: a fixture on the floor needs one, else declare it overhead, wall-mounted or a zone", f.id, f.kit, f.level)
		}
		for _, p := range f.footprint {
			if floorAt(t, p) {
				return fail("%s (%s) on %s: footprint cell (%d,%d) is open floor", f.id, f.kit, f.level, p.X, p.Y)
			}
		}
		inside := fixtureInside(t, walks, f)
		switch {
		case len(inside) > 0 && rule.Inside == "":
			return fail("%s (%s) on %s: %d cells inside it (first (%d,%d)) are open to everybody; declare its inside work or walk_in", f.id, f.kit, f.level, len(inside), inside[0].X, inside[0].Y)
		case len(inside) == 0 && rule.Inside != "":
			return fail("%s (%s) on %s declares an inside (%s) it does not have", f.id, f.kit, f.level, rule.Inside)
		case rule.Inside == "work":
			work[f.level] = append(work[f.level], lifeWorkArea{ID: f.id, Cells: inside})
		}
	}
	if len(work) == 0 {
		return nil
	}
	// who works where: routine steps, counter posts, a person's own brain places
	var actors []struct {
		ID    string `json:"id"`
		Steps []struct {
			Level string `json:"level"`
			At    Point  `json:"at"`
		} `json:"steps"`
	}
	if err := json.Unmarshal(life["actors"], &actors); err != nil {
		return err
	}
	var brain struct {
		CounterPosts []struct {
			Actor string `json:"actor"`
			Level string `json:"level"`
			Spot  struct {
				At Point `json:"at"`
			} `json:"spot"`
		} `json:"counter_posts"`
		Places []struct {
			ID    string  `json:"id"`
			Actor string  `json:"actor"`
			Level string  `json:"level"`
			At    Point   `json:"at"`
			Slots []Point `json:"slots"`
		} `json:"places"`
	}
	if len(life["brain"]) > 0 {
		if err := json.Unmarshal(life["brain"], &brain); err != nil {
			return err
		}
	}
	var edges []struct {
		ID          string `json:"id"`
		From, To    string
		Exit, Entry Point
	}
	if err := json.Unmarshal(life["edges"], &edges); err != nil {
		return err
	}
	areaAt := map[string]map[Point]int{}
	for level, areas := range work {
		sort.Slice(areas, func(i, j int) bool { return areas[i].ID < areas[j].ID })
		work[level] = areas
		areaAt[level] = map[Point]int{}
		for i, a := range areas {
			for _, p := range a.Cells {
				areaAt[level][p] = i
			}
		}
	}
	workers := map[string]map[int]map[string]bool{}
	hire := func(level string, at Point, actor string) {
		i, ok := areaAt[level][at]
		if !ok || actor == "" {
			return
		}
		if workers[level] == nil {
			workers[level] = map[int]map[string]bool{}
		}
		if workers[level][i] == nil {
			workers[level][i] = map[string]bool{}
		}
		workers[level][i][actor] = true
	}
	for _, a := range actors {
		for _, s := range a.Steps {
			hire(s.Level, s.At, a.ID)
		}
	}
	for _, p := range brain.CounterPosts {
		hire(p.Level, p.Spot.At, p.Actor)
	}
	for _, p := range brain.Places {
		if p.Actor != "" {
			hire(p.Level, p.At, p.Actor)
		}
	}
	public := func(level string, at Point, what string) error {
		if i, ok := areaAt[level][at]; ok {
			return fail("%s at (%d,%d) on %s lies in the work cells of %s", what, at.X, at.Y, level, work[level][i].ID)
		}
		return nil
	}
	for _, p := range brain.Places {
		if p.Actor != "" {
			continue
		}
		if err := public(p.Level, p.At, "brain place "+p.ID); err != nil {
			return err
		}
		for _, s := range p.Slots {
			if err := public(p.Level, s, "a slot of brain place "+p.ID); err != nil {
				return err
			}
		}
	}
	for _, e := range edges {
		if err := public(e.From, e.Exit, "the exit of edge "+e.ID); err != nil {
			return err
		}
		if err := public(e.To, e.Entry, "the entry of edge "+e.ID); err != nil {
			return err
		}
	}
	for level, loc := range locations {
		var markers []struct {
			Name string `json:"name"`
			At   Point  `json:"at"`
		}
		if len(loc["markers"]) > 0 {
			if err := json.Unmarshal(loc["markers"], &markers); err != nil {
				return err
			}
		}
		for _, m := range markers {
			if err := public(level, m.At, "marker "+m.Name); err != nil {
				return err
			}
		}
	}
	if v := c.Visitors; v != nil {
		for _, e := range v.Entries {
			if err := public(v.Level, Point{X: e.At.X, Y: e.At.Y}, "visitors' entry "+e.ID); err != nil {
				return err
			}
		}
		for _, t := range v.Targets {
			if err := public(v.Level, Point{X: t.At.X, Y: t.At.Y}, "visitors' place "+t.ID); err != nil {
				return err
			}
			for _, s := range t.Spots {
				if err := public(v.Level, Point{X: s.X, Y: s.Y}, "a standing cell of visitors' place "+t.ID); err != nil {
					return err
				}
			}
		}
	}
	if v := c.Vending; v != nil {
		for _, u := range v.Units {
			for _, s := range append([]Point{{X: u.Panel.X, Y: u.Panel.Y}}, pointsOf(u.Approach)...) {
				if err := public(v.Level, s, "vending panel cell of "+u.ID); err != nil {
					return err
				}
			}
		}
	}
	levels := make([]string, 0, len(work))
	for level := range work {
		levels = append(levels, level)
	}
	sort.Strings(levels)
	for _, level := range levels {
		areas := work[level]
		for i := range areas {
			for id := range workers[level][i] {
				areas[i].Workers = append(areas[i].Workers, id)
			}
			sort.Strings(areas[i].Workers)
			if areas[i].Workers == nil {
				areas[i].Workers = []string{}
			}
		}
		locations[level]["work"], _ = json.Marshal(areas)
	}
	// ambient population never idles in work cells
	if len(life["population"]) > 0 {
		var population map[string]json.RawMessage
		if err := json.Unmarshal(life["population"], &population); err != nil {
			return err
		}
		if len(population["anchors"]) > 0 {
			var anchors map[string][]Point
			if err := json.Unmarshal(population["anchors"], &anchors); err != nil {
				return err
			}
			changed := false
			for level, cells := range anchors {
				kept := cells[:0]
				for _, p := range cells {
					if _, ok := areaAt[level][p]; ok {
						changed = true
						continue
					}
					kept = append(kept, p)
				}
				if len(kept) == 0 && len(cells) > 0 {
					return fail("level %s has no population floor outside the work cells", level)
				}
				anchors[level] = kept
			}
			if changed {
				population["anchors"], _ = json.Marshal(anchors)
				life["population"], _ = json.Marshal(population)
			}
		}
	}
	life["locations"], _ = json.Marshal(locations)
	raw, err := json.Marshal(life)
	if err != nil {
		return err
	}
	c.Life = raw
	return nil
}

func pointsOf[T interface{ ~struct{ X, Y int } }](cells []T) []Point {
	out := make([]Point, len(cells))
	for i, c := range cells {
		v := struct{ X, Y int }(c)
		out[i] = Point{X: v.X, Y: v.Y}
	}
	return out
}
