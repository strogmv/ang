package locationintent

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/godot"
	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
)

type cell = locationsdk.Cell

const unitM = float64(locationsdk.UnitMM) / 1000

var sideStep = map[string]cell{"north": {X: 0, Y: -1}, "east": {X: 1, Y: 0}, "south": {X: 0, Y: 1}, "west": {X: -1, Y: 0}}
var sideOrder = []string{"north", "east", "south", "west"}

// A passport's unrotated back is north and its front south; these rotations
// put the back against the named wall (locationsdk rotates clockwise).
var wallRotation = map[string]int{"north": 0, "east": 90, "south": 180, "west": 270}

// rotateCell mirrors locationsdk.rotateCell; derive_test compares both through Build.
func rotateCell(at cell, w, d, rotation int) cell {
	switch rotation {
	case 90:
		return cell{X: d - 1 - at.Y, Y: at.X}
	case 180:
		return cell{X: w - 1 - at.X, Y: d - 1 - at.Y}
	case 270:
		return cell{X: at.Y, Y: w - 1 - at.X}
	}
	return at
}

type state struct {
	in      Intent
	def     Defaults
	tier    TierDefaults
	short   string
	off     cell
	width   int
	depth   int
	zones   map[string]locationsdk.Rect
	floor   map[cell]string // zone floor
	void    map[cell]string
	solid   map[cell]string
	thresh  map[cell]string // exit id
	wall    map[cell]bool
	blocked map[cell]string // fixture id
	objects map[cell]string // anything a marker may stand at
	diags   []Diagnostic
}

func (s *state) fail(code, id string, at *cell, message, hint string) {
	s.diags = append(s.diags, Diagnostic{Code: code, ID: id, Cell: at, Message: message, Hint: hint})
}

func onGrid(v float64) (int, bool) {
	n := math.Round(v / unitM)
	return int(n), math.Abs(v/unitM-n) < 1e-9
}

func (s *state) rect(id string, r RectM) (locationsdk.Rect, bool) {
	x, a := onGrid(r.X)
	y, b := onGrid(r.Y)
	w, c := onGrid(r.W)
	h, d := onGrid(r.H)
	if !(a && b && c && d) || w <= 0 || h <= 0 || x < 0 || y < 0 {
		s.fail("off_grid", id, nil, fmt.Sprintf("rect_m %+v is not a positive multiple of 0.5 m", r), "use metres in steps of 0.5")
		return locationsdk.Rect{}, false
	}
	return locationsdk.Rect{X: x, Y: y, W: w, H: h}, true
}

func cellsOf(r locationsdk.Rect) []cell {
	out := make([]cell, 0, r.W*r.H)
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			out = append(out, cell{X: x, Y: y})
		}
	}
	return out
}

func sortCells(c []cell) {
	sort.Slice(c, func(i, j int) bool { return c[i].Y < c[j].Y || c[i].Y == c[j].Y && c[i].X < c[j].X })
}

// mergeRects covers a cell set with disjoint rectangles, deterministic.
func mergeRects(set map[cell]bool) []locationsdk.Rect {
	cells := make([]cell, 0, len(set))
	for c := range set {
		cells = append(cells, c)
	}
	sortCells(cells)
	used := map[cell]bool{}
	var out []locationsdk.Rect
	for _, c := range cells {
		if used[c] {
			continue
		}
		w := 1
		for set[cell{X: c.X + w, Y: c.Y}] && !used[cell{X: c.X + w, Y: c.Y}] {
			w++
		}
		h := 1
	grow:
		for {
			for x := c.X; x < c.X+w; x++ {
				n := cell{X: x, Y: c.Y + h}
				if !set[n] || used[n] {
					break grow
				}
			}
			h++
		}
		r := locationsdk.Rect{X: c.X, Y: c.Y, W: w, H: h}
		for _, u := range cellsOf(r) {
			used[u] = true
		}
		out = append(out, r)
	}
	return out
}

func stableShort(id string) string { return strings.TrimPrefix(id, "level.") }

// Derive turns one intent into the layout, presentation and world data.
// A nil result means the intent is structurally unusable; otherwise the
// diagnostics (possibly empty) are coherence failures that must stop
// generation but can still be drawn on the plan.
func Derive(in Intent, catalog locationsdk.Catalog, def Defaults) (*Derived, []Diagnostic) {
	s := &state{in: in, def: def, short: stableShort(in.ID), zones: map[string]locationsdk.Rect{}, floor: map[cell]string{}, void: map[cell]string{}, solid: map[cell]string{}, thresh: map[cell]string{}, wall: map[cell]bool{}, blocked: map[cell]string{}, objects: map[cell]string{}}
	if in.Short != "" {
		s.short = in.Short
	}
	tier, ok := def.Tiers[in.Tier]
	if !ok || len(in.Zones) == 0 || in.ID == "" {
		s.fail("intent_incomplete", in.ID, nil, "id, a tier with game defaults and at least one zone are required", "check tier spelling and location_intent defaults.tiers")
		return nil, s.diags
	}
	s.tier = tier
	wallH := in.WallHeightM
	if wallH == 0 {
		wallH = tier.WallHeightM
	}
	hero := in.Camera.HeroViewHeightM
	if hero == 0 {
		hero = 12
	}

	// --- zones, envelope, offset ---------------------------------------
	raw := map[string]locationsdk.Rect{}
	minX, minY, maxX, maxY := math.MaxInt32, math.MaxInt32, math.MinInt32, math.MinInt32
	for _, z := range in.Zones {
		r, ok := s.rect(z.ID, z.Rect)
		if !ok {
			continue
		}
		if _, dup := raw[z.ID]; dup || z.ID == "" {
			s.fail("duplicate_id", z.ID, nil, "zone ID repeats or is empty", "give every zone its own ID")
			continue
		}
		raw[z.ID] = r
		minX, minY = min(minX, r.X), min(minY, r.Y)
		maxX, maxY = max(maxX, r.X+r.W), max(maxY, r.Y+r.H)
	}
	for _, m := range in.Solids {
		if r, ok := s.rect(m.ID, m.Rect); ok {
			minX, minY = min(minX, r.X), min(minY, r.Y)
			maxX, maxY = max(maxX, r.X+r.W), max(maxY, r.Y+r.H)
		}
	}
	if len(s.diags) > 0 {
		return nil, s.diags
	}
	s.off = cell{X: 1 - minX, Y: 1 - minY} // one wall cell around the envelope
	s.width, s.depth = maxX-minX+2, maxY-minY+2
	shift := func(r locationsdk.Rect) locationsdk.Rect {
		return locationsdk.Rect{X: r.X + s.off.X, Y: r.Y + s.off.Y, W: r.W, H: r.H}
	}
	envelope := locationsdk.Rect{X: 1, Y: 1, W: maxX - minX, H: maxY - minY}
	for _, z := range in.Zones {
		r := shift(raw[z.ID])
		s.zones[z.ID] = r
		for _, c := range cellsOf(r) {
			if other, taken := s.floor[c]; taken {
				at := c
				s.fail("zone_overlap", z.ID, &at, "zone overlaps zone "+other, "zones are disjoint pieces of one floor; shrink one of them")
				break
			}
			s.floor[c] = z.ID
		}
	}
	for _, m := range in.Solids {
		r, _ := s.rect(m.ID, m.Rect)
		if strings.TrimSpace(m.Reason) == "" {
			s.fail("solid_without_reason", m.ID, nil, "declared mass needs a written reason", "say what the thickness is for")
		}
		for _, c := range cellsOf(shift(r)) {
			if z, taken := s.floor[c]; taken {
				at := c
				s.fail("solid_overlaps_zone", m.ID, &at, "declared mass overlaps zone "+z, "a cell is either floor or mass")
				break
			}
			s.solid[c] = m.ID
		}
	}

	// --- mechanisms and their declared voids ---------------------------------
	d := &Derived{Intent: in, Short: s.short, Offset: s.off, Envelope: envelope, Zones: s.zones, ZoneSeeds: map[string]int{}, Models: map[string]string{}}
	for i, z := range in.Zones {
		seed := in.Seed*131 + i*7919
		if z.Seed != nil {
			seed = *z.Seed
		}
		d.ZoneSeeds[z.ID] = seed
	}
	for _, m := range in.Mechanisms {
		r, ok := s.rect(m.ID, m.Rect)
		if !ok {
			continue
		}
		dm := DerivedMechanism{ID: m.ID, Kind: m.Kind, Rect: shift(r)}
		if _, known := s.zones[m.Zone]; !known {
			s.fail("unknown_zone", m.ID, nil, "mechanism names zone "+m.Zone, "use a zone ID of this file")
		}
		if m.Void != nil {
			if vr, ok := s.rect(m.ID+".void", m.Void.Rect); ok {
				v := shift(vr)
				dm.Void = &v
				for _, c := range cellsOf(v) {
					if _, isFloor := s.floor[c]; !isFloor {
						at := c
						s.fail("void_outside_floor", m.ID, &at, "a mechanism void must be cut from zone floor", "move the void inside its zone")
						break
					}
					delete(s.floor, c)
					s.void[c] = m.ID
				}
			}
		}
		for _, c := range cellsOf(dm.Rect) {
			s.objects[c] = m.ID
		}
		d.Mechanisms = append(d.Mechanisms, dm)
	}

	// --- pockets inside the hull are capped ------------------------------------
	// Whatever lies inside the bounding rectangle of the rooms and is neither a
	// zone, declared mass nor a mechanism void would be a floorless pit in the
	// picture. It becomes mass capped at the wall top and is reported.
	for n, pocket := range HullPockets(envelope, func(c cell) bool { return s.floor[c] != "" || s.solid[c] != "" || s.void[c] != "" }) {
		id := fmt.Sprintf("cap.%d", n)
		for c := range pocket.set {
			s.solid[c] = id
		}
		d.Capped = append(d.Capped, CappedPocket{ID: id, Cells: pocket.Cells, Bounds: pocket.Bounds})
	}

	// --- exits: opening through an outer wall ---------------------------------
	for _, e := range in.Exits {
		z, known := s.zones[e.Zone]
		step, sideOK := sideStep[e.Wall]
		if !known || !sideOK {
			s.fail("unknown_zone", e.ID, nil, "exit names zone "+e.Zone+" wall "+e.Wall, "use a zone ID of this file and north/east/south/west")
			continue
		}
		status := e.Status
		if status == "" {
			status = "live"
		}
		if status == "closed" && !strings.HasPrefix(e.ClosedReasonKey, "error.") {
			s.fail("exit_unbound", e.ID, nil, "a closed exit needs closed_reason_key (error.*) so the server can say why", "give closed_reason_key and its RU/EN text")
		}
		if status != "planned" && (e.To == "" || e.Edge == "" || e.ReverseEdge == "" || e.TravelSeconds <= 0) {
			s.fail("exit_unbound", e.ID, nil, "a live exit needs to, edge, reverse_edge and travel_seconds", "bind the exit to both Life edges or mark it status: \"planned\"")
		}
		horizontal := e.Wall == "north" || e.Wall == "south"
		span := z.H
		if horizontal {
			span = z.W
		}
		width, _ := onGrid(e.WidthM)
		if width <= 0 {
			width = 3
		}
		centre := float64(span) / 2
		if e.AtM != nil {
			centre = *e.AtM / unitM
		}
		first := int(math.Floor(centre - float64(width)/2))
		if first < 0 || first+width > span {
			s.fail("exit_does_not_fit", e.ID, nil, fmt.Sprintf("opening of %d cells at %.1f does not fit the %d-cell %s wall of zone %s", width, centre, span, e.Wall, e.Zone), "move at_m or narrow width_m")
			continue
		}
		edgeCell := func(k int) cell {
			switch e.Wall {
			case "north":
				return cell{X: z.X + k, Y: z.Y}
			case "south":
				return cell{X: z.X + k, Y: z.Y + z.H - 1}
			case "west":
				return cell{X: z.X, Y: z.Y + k}
			}
			return cell{X: z.X + z.W - 1, Y: z.Y + k}
		}
		mid := first + width/2
		if mid >= first+width {
			mid = first + width - 1
		}
		de := DerivedExit{ID: e.ID, To: e.To, Wall: e.Wall, Status: status, Edge: e.Edge, Reverse: e.ReverseEdge, Landing: edgeCell(mid)}
		bad := false
		for k := first; k < first+width; k++ {
			in := edgeCell(k)
			out := cell{X: in.X + step.X, Y: in.Y + step.Y}
			if s.floor[out] != "" || s.void[out] != "" {
				at := in
				s.fail("exit_not_on_outer_wall", e.ID, &at, "the "+e.Wall+" side of zone "+e.Zone+" borders floor here, not a wall", "put the exit on a side that is an outer wall")
				bad = true
				break
			}
			if other := s.thresh[out]; other != "" {
				at := out
				s.fail("exit_overlap", e.ID, &at, "opening overlaps exit "+other, "move one of the exits")
				bad = true
				break
			}
			de.Threshold = append(de.Threshold, out)
		}
		if bad {
			continue
		}
		for _, c := range de.Threshold {
			s.thresh[c] = e.ID
			delete(s.solid, c)
			s.objects[c] = e.ID
		}
		de.At = cell{X: de.Landing.X + step.X, Y: de.Landing.Y + step.Y}
		d.Exits = append(d.Exits, de)
	}

	// --- walls: everything that touches floor and is not floor ----------------
	for c := range s.floor {
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				n := cell{X: c.X + dx, Y: c.Y + dy}
				if s.floor[n] == "" && s.void[n] == "" && s.thresh[n] == "" {
					s.wall[n] = true
				}
			}
		}
	}
	for c := range s.solid {
		s.wall[c] = true
	}
	floorCells := make([]cell, 0, len(s.floor))
	for c := range s.floor {
		floorCells = append(floorCells, c)
	}
	sortCells(floorCells)
	for _, c := range floorCells {
		for _, side := range sideOrder {
			n := cell{X: c.X + sideStep[side].X, Y: c.Y + sideStep[side].Y}
			switch {
			case s.floor[n] != "":
			case s.thresh[n] != "":
				d.Walls = append(d.Walls, WallEdge{Cell: c, Side: side, Zone: s.floor[c], Kind: "open", Exit: s.thresh[n]})
			case s.void[n] != "":
				d.Walls = append(d.Walls, WallEdge{Cell: c, Side: side, Zone: s.floor[c], Kind: "guard"})
			default:
				d.Walls = append(d.Walls, WallEdge{Cell: c, Side: side, Zone: s.floor[c], Kind: "wall"})
			}
		}
	}

	// --- fixtures -------------------------------------------------------
	geometries := map[string]locationsdk.Geometry{}
	for _, g := range catalog.Geometries {
		geometries[g.ID] = g
	}
	moduleGeometry := map[string]string{}
	for _, m := range catalog.Modules {
		moduleGeometry[m.ID] = m.GeometryID
	}
	extras := KitExtras{Geometries: []locationsdk.Geometry{}, Modules: []locationsdk.Module{}}
	addPassport := func(module string, g locationsdk.Geometry) {
		extras.Geometries = append(extras.Geometries, g)
		extras.Modules = append(extras.Modules, locationsdk.Module{ID: module, GeometryID: g.ID, Variants: []locationsdk.Variant{{ID: "neutral", GeometryID: g.ID}}})
		geometries[g.ID] = g
		moduleGeometry[module] = g.ID
	}
	instances := []locationsdk.Instance{}
	moduleModels := map[string]string{}
	instanceSpecs := map[string]placement.InstanceSpec{}
	var draws []godot.ArchitectureDraw
	fixtureByID := map[string]DerivedFixture{}
	for i, f := range in.Fixtures {
		z, known := s.zones[f.Zone]
		if !known {
			s.fail("unknown_zone", f.ID, nil, "fixture names zone "+f.Zone, "use a zone ID of this file")
			continue
		}
		geometryID, hasPassport := moduleGeometry[f.Kit]
		// size_m says: there is no shared passport, place a greybox with this ID.
		// The generated kit of the level echoes that greybox back into the
		// catalog; an echo is not a real passport.
		echo := hasPassport && f.SizeM != nil
		if !hasPassport || echo {
			if f.SizeM == nil {
				s.fail("kit_without_passport", f.ID, nil, "kit "+f.Kit+" has no passport in the catalog", "name an existing kit from docs/kit-catalog.md or give size_m so a greybox with this ID is placed")
				continue
			}
			w, _ := onGrid(f.SizeM[0])
			dp, _ := onGrid(f.SizeM[1])
			h := max(1, int(math.Ceil(f.SizeM[2]/unitM)))
			if w <= 0 || dp <= 0 {
				s.fail("off_grid", f.ID, nil, "size_m must be positive multiples of 0.5 m in plan", "use steps of 0.5")
				continue
			}
			g := locationsdk.Geometry{ID: strings.TrimPrefix(f.Kit, "kit."), Width: w, Depth: dp, Height: h, Footprint: []cell{}, Blocked: []cell{}, UseClearance: []cell{}, MaintenanceClearance: []cell{}, Ports: []locationsdk.Port{}}
			for y := 0; y < dp; y++ {
				for x := 0; x < w; x++ {
					g.Footprint = append(g.Footprint, cell{X: x, Y: y})
					g.Blocked = append(g.Blocked, cell{X: x, Y: y})
				}
			}
			for x := 0; x < w; x++ {
				g.UseClearance = append(g.UseClearance, cell{X: x, Y: dp})
			}
			if echo {
				before, _ := json.Marshal(geometries[geometryID])
				now, _ := json.Marshal(g)
				if string(before) != string(now) {
					s.fail("kit_has_passport", f.ID, nil, "kit "+f.Kit+" now has a real passport in the catalog", "remove size_m from the fixture: the passport decides the size")
					continue
				}
				hasPassport = false
			}
			addPassport(f.Kit, g)
			geometryID = g.ID
			d.Missing = append(d.Missing, MissingKit{ID: f.Kit, For: f.ID, SizeCells: [3]int{w, dp, h}, Reason: f.Purpose})
		}
		g := geometries[geometryID]
		rotation := f.Rotation
		if f.On != "floor" {
			rotation = wallRotation[f.On]
		}
		rw, rd := g.Width, g.Depth
		if rotation == 90 || rotation == 270 {
			rw, rd = g.Depth, g.Width
		}
		horizontal := f.On == "north" || f.On == "south"
		span := z.H
		along := rd
		if horizontal {
			span, along = z.W, rw
		}
		pos := (span - along) / 2
		switch {
		case f.AlignM != nil:
			pos, _ = onGrid(*f.AlignM)
		case f.Align == "start":
			pos = 0
		case f.Align == "end":
			pos = span - along
		}
		// Service space behind the passport (a counter's staff side) keeps the
		// fixture off the wall by exactly that depth.
		gap := 0
		behind := append(append([]cell{}, g.MaintenanceClearance...), g.UseClearance...)
		for _, c := range behind {
			gap = max(gap, -c.Y)
		}
		var at cell
		switch f.On {
		case "north":
			at = cell{X: z.X + pos, Y: z.Y + gap}
		case "south":
			at = cell{X: z.X + pos, Y: z.Y + z.H - rd - gap}
		case "west":
			at = cell{X: z.X + gap, Y: z.Y + pos}
		case "east":
			at = cell{X: z.X + z.W - rw - gap, Y: z.Y + pos}
		case "floor":
			if f.AtM == nil {
				s.fail("fixture_without_position", f.ID, nil, "on: \"floor\" needs at_m", "give the corner in metres of the zone or put the fixture on a wall")
				continue
			}
			x, _ := onGrid(f.AtM[0])
			y, _ := onGrid(f.AtM[1])
			at = cell{X: z.X + x, Y: z.Y + y}
		default:
			s.fail("unknown_wall", f.ID, nil, "on must be a wall side or floor", "use north/east/south/west/floor")
			continue
		}
		if at.X < z.X || at.Y < z.Y || at.X+rw > z.X+z.W || at.Y+rd > z.Y+z.H {
			c := at
			s.fail("fixture_outside_zone", f.ID, &c, fmt.Sprintf("%d×%d cells do not fit zone %s at that place", rw, rd, f.Zone), "change align/align_m or pick a larger zone")
			continue
		}
		if f.On != "floor" {
			// The whole back must face a real wall, not a neighbouring zone.
			noWall := false
			for k := 0; k < along && !noWall; k++ {
				var probe cell
				switch f.On {
				case "north":
					probe = cell{X: at.X + k, Y: z.Y - 1}
				case "south":
					probe = cell{X: at.X + k, Y: z.Y + z.H}
				case "west":
					probe = cell{X: z.X - 1, Y: at.Y + k}
				default:
					probe = cell{X: z.X + z.W, Y: at.Y + k}
				}
				noWall = !s.wall[probe]
			}
			if noWall {
				c := at
				s.fail("fixture_without_wall", f.ID, &c, "the "+f.On+" side of zone "+f.Zone+" is not a wall here", "choose a side that is an outer wall, or on: \"floor\"")
				continue
			}
		}
		world := func(local cell) cell {
			r := rotateCell(local, g.Width, g.Depth, rotation)
			return cell{X: r.X + at.X, Y: r.Y + at.Y}
		}
		df := DerivedFixture{ID: f.ID, Kit: f.Kit, Zone: f.Zone, At: at, Rotation: rotation, W: rw, D: rd, H: g.Height, Greybox: !hasPassport, Seed: in.Seed*131 + i*104729}
		if f.Seed != nil {
			df.Seed = *f.Seed
		}
		for _, c := range g.Blocked {
			w := world(c)
			df.Cells = append(df.Cells, w)
			s.blocked[w] = f.ID
		}
		// Anything of the passport marks a real object, including door interfaces
		// and semantic reservations without a footprint.
		object := append(append([]cell{}, g.Footprint...), g.Blocked...)
		for _, p := range g.Ports {
			object = append(object, p.At)
			object = append(object, p.Opening...)
		}
		for _, c := range object {
			s.objects[world(c)] = f.ID
		}
		// Approach: the use clearance cell nearest to the front centre.
		front := world(cell{X: g.Width / 2, Y: g.Depth})
		best, bestDistance := front, math.MaxInt32
		candidates := append([]cell{}, g.UseClearance...)
		for _, p := range g.Ports {
			candidates = append(candidates, p.Approach...)
		}
		for _, c := range candidates {
			w := world(c)
			if distance := abs(w.X-front.X) + abs(w.Y-front.Y); distance < bestDistance {
				best, bestDistance = w, distance
			}
		}
		df.Approach = best
		// Bound art or a greybox that carries the ID.
		ref := ""
		if res := def.Models[f.ID]; res != "" {
			ref = f.ID
			d.Models[ref] = res
			instanceSpecs[f.ID] = placement.InstanceSpec{ModelRef: &ref}
		} else if res := def.Models[f.Kit]; res != "" {
			ref = f.Kit
			d.Models[ref] = res
		}
		if _, set := moduleModels[f.Kit]; !set {
			moduleModels[f.Kit] = ""
			if def.Models[f.Kit] != "" {
				moduleModels[f.Kit] = f.Kit
			}
		}
		df.ModelRef = ref
		if ref == "" {
			d.Greybox = append(d.Greybox, f.ID)
			h := float64(g.Height) * unitM
			draws = append(draws, godot.ArchitectureDraw{ID: f.ID + ".greybox", Kind: "trim", MaterialRole: def.GreyboxRole,
				Position: [3]float64{(float64(at.X) + float64(rw)/2) * unitM, in.ElevationM + h/2, (float64(at.Y) + float64(rd)/2) * unitM},
				Size:     [3]float64{float64(rw) * unitM, h, float64(rd) * unitM},
				Reason:   "Greybox placeholder with ang_id " + f.Kit + " for " + f.ID + ": " + f.Purpose})
		}
		owner := f.Owner
		if owner == "" {
			owner = "unknown"
		}
		instances = append(instances, locationsdk.Instance{ID: f.ID, ModuleID: f.Kit, At: at, Rotation: rotation, Reason: f.Purpose, OwnerRef: owner, Fate: locationsdk.ItemFate{Policy: "keep"}})
		d.Fixtures = append(d.Fixtures, df)
		fixtureByID[f.ID] = df
	}
	// A mechanism is not an SDK instance: its model, motion and navigation come
	// from its own kit (lifting_deck_kit). Here it is a place with an outline.
	d.Extra = extras

	// --- floor, walls, lintels ---------------------------------------------
	// Walls are caste concrete drawn along every floor edge without a neighbour.
	// Navigation needs no wall object: there is simply no floor beyond the edge.
	thickness := tier.WallThicknessM
	if thickness == 0 {
		thickness = 0.25
	}
	var floor []locationsdk.FloorArea
	interior := &locationsdk.InteriorEnvelope{Voids: []locationsdk.InteriorVoid{}}
	addFloor := func(id string, r locationsdk.Rect, reason string) {
		floor = append(floor, locationsdk.FloorArea{ID: id, Rect: r, Reason: reason})
	}
	for n, z := range in.Zones {
		set := map[cell]bool{}
		for _, c := range cellsOf(s.zones[z.ID]) {
			if s.floor[c] == z.ID {
				set[c] = true
			}
		}
		rects := mergeRects(set)
		for k, r := range rects {
			id := s.short + "." + z.ID
			if len(rects) > 1 {
				id = fmt.Sprintf("%s.%d", id, k)
			}
			addFloor(id, r, z.Purpose)
		}
		if n == 0 {
			interior.Envelope = s.zones[z.ID]
		} else {
			interior.Extensions = append(interior.Extensions, s.zones[z.ID])
		}
	}
	for _, m := range d.Mechanisms {
		if m.Void != nil {
			interior.Voids = append(interior.Voids, locationsdk.InteriorVoid{ID: m.ID + ".void", Kind: "shaft", Rect: *m.Void, Reason: "Declared void of mechanism " + m.ID + "; its edges are guarded."})
		}
	}
	top := wallH
	var extrasPlacement []placement.Extra
	// Panel kit of the caste, if its size matches this room's walls.
	var kit *WallKit
	if k, ok := def.WallKits[tier.WallKit]; ok && math.Abs(k.HeightM-top) < 1e-6 && math.Abs(k.ThicknessM-thickness) < 1e-6 {
		kit = &k
	}
	type wallMount struct {
		side  string
		x, y  float64
		extra int    // index of the panel in extrasPlacement
		lamp  string // its variant with the embed plate
	}
	type kitRun struct {
		side   string
		x0, y0 float64
		x1, y1 float64
	}
	var mounts []wallMount
	var kitRuns []kitRun
	// layPanels covers the wall stretch a..b (metres along the wall) with kit
	// panels, longest first; what no panel fits is returned for a plain box.
	layPanels := func(id, side string, line int, a, b float64, cutWall bool) float64 {
		pos, k := a, 0
		for b-pos > 1e-6 {
			var pick *WallPanel
			for i := range kit.Panels {
				if kit.Panels[i].LengthM <= b-pos+1e-6 {
					pick = &kit.Panels[i]
					break
				}
			}
			if pick == nil {
				break
			}
			variants := pick.Full
			if cutWall {
				variants = pick.Cut
			}
			pickAt := (line*7 + int(math.Round(pos*2))*3) % len(variants)
			ref := variants[pickAt]
			centre := pos + pick.LengthM/2
			var x, y float64
			switch side {
			case "north":
				x, y = centre, float64(line)*unitM
			case "south":
				x, y = centre, float64(line+1)*unitM
			case "west":
				x, y = float64(line)*unitM, centre
			default:
				x, y = float64(line+1)*unitM, centre
			}
			d.Models[ref] = def.Models[ref]
			d.WallPanels++
			d.WallTriangles += kit.Triangles[ref]
			extrasPlacement = append(extrasPlacement, placement.Extra{InstanceID: fmt.Sprintf("%s.p%d", id, k), ModelRef: ref, SourceKind: "authored_fixture", PositionPlanM: placement.Vec3{x, in.ElevationM, y}, RotationDegrees: float64(wallRotation[side]),
				Reason: "Panel of the " + in.Tier + " wall kit on the " + side + " side; the wall run is laid from panels, cut pieces at its ends."})
			if pick.HasMount && !cutWall && pickAt < len(pick.Lamp) {
				mounts = append(mounts, wallMount{side: side, x: x, y: y, extra: len(extrasPlacement) - 1, lamp: pick.Lamp[pickAt]})
			}
			pos += pick.LengthM
			k++
		}
		return pos
	}
	near := func(side string) *godot.ArchitectureNearCut {
		// The camera stands south-east: a wall on the south or east side of a
		// floor would hide it, so it is drawn cut. The whole wall stays in data.
		if (side == "south" || side == "east") && tier.NearCutM > 0 && tier.NearCutM < top {
			return &godot.ArchitectureNearCut{HeightM: tier.NearCutM, Reason: "Camera-near wall of the default south-east view: drawn cut so the floor and the hero's feet stay visible."}
		}
		return nil
	}
	// A narrow way beyond an exit is hidden whole behind a 1.2 m near wall at
	// the 35 degree camera, so its camera-near walls are cut down to a kerb.
	kerb := func(side string) *godot.ArchitectureNearCut {
		if side == "south" || side == "east" {
			return &godot.ArchitectureNearCut{HeightM: 0.3, Reason: "Camera-near wall of the way beyond an exit: cut to a kerb so the threshold and the corridor floor stay visible."}
		}
		return nil
	}
	wallDraw := func(id, kind, role string, x0, y0, x1, y1, bottom, height float64, cut *godot.ArchitectureNearCut, reason string) {
		if height <= bottom || x1 <= x0 || y1 <= y0 {
			return
		}
		draws = append(draws, godot.ArchitectureDraw{ID: id, Kind: kind, MaterialRole: role, NearCut: cut,
			Position: [3]float64{(x0 + x1) / 2, in.ElevationM + (bottom+height)/2, (y0 + y1) / 2}, Size: [3]float64{x1 - x0, height - bottom, y1 - y0}, Reason: reason})
	}
	// Runs of collinear wall edges become one box each.
	type run struct {
		side         string
		line, from   int
		to           int
		kind, exitID string
	}
	var runs []run
	bySide := map[string][]WallEdge{}
	for _, w := range d.Walls {
		if w.Kind != "guard" {
			bySide[w.Side] = append(bySide[w.Side], w)
		}
	}
	for _, side := range sideOrder {
		edges := bySide[side]
		horizontal := side == "north" || side == "south"
		sort.Slice(edges, func(i, j int) bool {
			li, lj, ai, aj := edges[i].Cell.Y, edges[j].Cell.Y, edges[i].Cell.X, edges[j].Cell.X
			if !horizontal {
				li, lj, ai, aj = ai, aj, li, lj
			}
			return li < lj || li == lj && ai < aj
		})
		for _, w := range edges {
			line, along := w.Cell.Y, w.Cell.X
			if !horizontal {
				line, along = along, line
			}
			if n := len(runs); n > 0 && runs[n-1].side == side && runs[n-1].line == line && runs[n-1].to == along && runs[n-1].kind == w.Kind && runs[n-1].exitID == w.Exit {
				runs[n-1].to = along + 1
				continue
			}
			runs = append(runs, run{side: side, line: line, from: along, to: along + 1, kind: w.Kind, exitID: w.Exit})
		}
	}
	entries := []cell{}
	exitHeight := map[string]float64{}
	exitStatus := map[string]string{}
	for i, e := range d.Exits {
		src := in.Exits[indexExit(in.Exits, e.ID)]
		h := src.HeightM
		if h == 0 {
			h = 2.2
		}
		exitHeight[e.ID], exitStatus[e.ID] = h, e.Status
		set := map[cell]bool{}
		for _, c := range e.Threshold {
			set[c] = true
		}
		for k, r := range mergeRects(set) {
			addFloor(fmt.Sprintf("%s.threshold.%s.%d", s.short, e.ID, k), r, "Threshold of exit "+e.ID+" through the wall.")
			interior.Extensions = append(interior.Extensions, r)
		}
		if e.Status != "planned" {
			entries = append(entries, d.Exits[i].Landing)
		}
	}
	for n, r := range runs {
		// Outer face of the floor edge in metres, wall body beyond it.
		a, b := float64(r.from)*unitM, float64(r.to)*unitM
		var x0, y0, x1, y1 float64
		// A north/south run closes the outer corner at each end, unless floor or
		// a doorway continues there: the wall never stands on walkable floor.
		corner := func(along, row int) float64 {
			c := cell{X: along, Y: row}
			if s.floor[c] != "" || s.thresh[c] != "" {
				return 0
			}
			return thickness
		}
		switch r.side {
		case "north":
			x0, x1, y1 = a-corner(r.from-1, r.line-1), b+corner(r.to, r.line-1), float64(r.line)*unitM
			y0 = y1 - thickness
		case "south":
			x0, x1, y0 = a-corner(r.from-1, r.line+1), b+corner(r.to, r.line+1), float64(r.line+1)*unitM
			y1 = y0 + thickness
		case "west":
			y0, y1, x1 = a, b, float64(r.line)*unitM
			x0 = x1 - thickness
		default:
			y0, y1, x0 = a, b, float64(r.line+1)*unitM
			x1 = x0 + thickness
		}
		cut := near(r.side)
		if r.kind == "wall" && kit != nil && (cut == nil || math.Abs(cut.HeightM-kit.CutHeightM) < 1e-6) {
			id := fmt.Sprintf("%s.wall.%s.%d", s.short, r.side, n)
			reason := "Derived " + in.Tier + " concrete wall: corner or remainder piece beside the panels."
			done := layPanels(id, r.side, r.line, a, b, cut != nil)
			if r.side == "north" || r.side == "south" {
				wallDraw(id+".c0", "wall", tier.ConcreteRole, x0, y0, a, y1, 0, top, cut, reason)
				wallDraw(id+".c1", "wall", tier.ConcreteRole, done, y0, x1, y1, 0, top, cut, reason)
			} else {
				wallDraw(id+".c1", "wall", tier.ConcreteRole, x0, done, x1, y1, 0, top, cut, reason)
			}
			if cut == nil {
				kitRuns = append(kitRuns, kitRun{side: r.side, x0: x0, y0: y0, x1: x1, y1: y1})
			}
			continue
		}
		if r.kind == "wall" {
			wallDraw(fmt.Sprintf("%s.wall.%s.%d", s.short, r.side, n), "wall", tier.ConcreteRole, x0, y0, x1, y1, 0, top, cut, "Derived "+in.Tier+" concrete wall: the floor has no neighbour on its "+r.side+" side.")
			continue
		}
		// An opening: keep the corner overlap out of the doorway.
		if r.side == "north" || r.side == "south" {
			x0, x1 = a, b
		}
		if cut == nil {
			wallDraw(fmt.Sprintf("%s.exit.%s.lintel", s.short, r.exitID), "wall", tier.ConcreteRole, x0, y0, x1, y1, exitHeight[r.exitID], top, nil, "Wall above the opening of exit "+r.exitID+".")
		}
		if exitStatus[r.exitID] != "live" {
			leaf := exitHeight[r.exitID]
			if cut != nil {
				leaf = math.Min(leaf, cut.HeightM)
			}
			// The leaf hangs on the outer face of the threshold, beyond walkable floor.
			const leafM = 0.12
			switch r.side {
			case "north":
				y1 = float64(r.line-1) * unitM
				y0 = y1 - leafM
			case "south":
				y0 = float64(r.line+2) * unitM
				y1 = y0 + leafM
			case "west":
				x1 = float64(r.line-1) * unitM
				x0 = x1 - leafM
			default:
				x0 = float64(r.line+2) * unitM
				x1 = x0 + leafM
			}
			wallDraw(fmt.Sprintf("%s.exit.%s.leaf", s.short, r.exitID), "steel", tier.SteelRole, x0, y0, x1, y1, 0, leaf, nil, "Shut steel leaf of exit "+r.exitID+": the way beyond is closed or not built; the server reason is authoritative.")
		}
	}
	// A mass or a capped pocket must not stand in the camera's view of the
	// floor behind it. The camera looks from the south-east: floor right to its
	// north or west gets the near cut; a threshold of an exit within the sight
	// line (up to 2 m north or west) gets a kerb, so the doorway always reads.
	massCut := func(r locationsdk.Rect) *godot.ArchitectureNearCut {
		for c := range s.thresh {
			if c.X >= r.X-4 && c.X < r.X+r.W+1 && c.Y >= r.Y-4 && c.Y < r.Y+r.H+1 && !(c.X >= r.X && c.X < r.X+r.W && c.Y >= r.Y && c.Y < r.Y+r.H) {
				return kerb("south")
			}
		}
		for _, c := range cellsOf(r) {
			if s.floor[cell{X: c.X, Y: c.Y - 1}] != "" || s.floor[cell{X: c.X - 1, Y: c.Y}] != "" {
				return near("south")
			}
		}
		return nil
	}
	for _, capped := range d.Capped {
		set := map[cell]bool{}
		for c, id := range s.solid {
			if id == capped.ID {
				set[c] = true
			}
		}
		for k, r := range mergeRects(set) {
			wallDraw(fmt.Sprintf("%s.%s.%d", s.short, capped.ID, k), "wall", tier.ConcreteRole, float64(r.X)*unitM, float64(r.Y)*unitM, float64(r.X+r.W)*unitM, float64(r.Y+r.H)*unitM, 0, top, massCut(r), "Enclosed pocket outside the walkable area, capped at the wall top by default so the camera never sees a floorless pit.")
		}
	}
	// An open way shows where it leads: beyond the threshold a short stub of
	// the neighbour's corridor, with floor, side walls and an end wall where it
	// turns. Presentation only; nobody walks there, the Life edge takes over.
	const stubM = 3.0
	var beyondLights [][3]float64
	var beyondIDs []string
	for _, e := range d.Exits {
		if e.Status != "live" {
			continue
		}
		minX, minY, maxX, maxY := math.MaxInt32, math.MaxInt32, math.MinInt32, math.MinInt32
		for _, c := range e.Threshold {
			minX, minY, maxX, maxY = min(minX, c.X), min(minY, c.Y), max(maxX, c.X+1), max(maxY, c.Y+1)
		}
		x0, y0, x1, y1 := float64(minX)*unitM, float64(minY)*unitM, float64(maxX)*unitM, float64(maxY)*unitM
		fx0, fy0, fx1, fy1 := x0, y0, x1, y1 // floor of the stub
		switch e.Wall {
		case "north":
			fy0, fy1 = y0-stubM, y0
		case "south":
			fy0, fy1 = y1, y1+stubM
		case "west":
			fx0, fx1 = x0-stubM, x0
		default:
			fx0, fx1 = x1, x1+stubM
		}
		id := s.short + ".exit." + e.ID + ".beyond"
		beyondLights = append(beyondLights, [3]float64{(fx0 + fx1) / 2, 2.4, (fy0 + fy1) / 2})
		beyondIDs = append(beyondIDs, id+".light")
		draws = append(draws, godot.ArchitectureDraw{ID: id + ".floor", Kind: "trim", MaterialRole: tier.FloorRole,
			Position: [3]float64{(fx0 + fx1) / 2, in.ElevationM - 0.125, (fy0 + fy1) / 2}, Size: [3]float64{fx1 - fx0, 0.25, fy1 - fy0}, Reason: "Floor of the way beyond exit " + e.ID + ": the entrance shows where it leads."})
		if e.Wall == "north" || e.Wall == "south" {
			wallDraw(id+".west", "wall", tier.ConcreteRole, fx0-thickness, fy0, fx0, fy1, 0, top, nil, "Side wall of the way beyond exit "+e.ID+".")
			wallDraw(id+".east", "wall", tier.ConcreteRole, fx1, fy0, fx1+thickness, fy1, 0, top, kerb("east"), "Side wall of the way beyond exit "+e.ID+".")
			endY := fy0 - thickness
			if e.Wall == "south" {
				endY = fy1
			}
			wallDraw(id+".end", "wall", tier.ConcreteRole, fx0-thickness, endY, fx1+thickness, endY+thickness, 0, top, kerb(e.Wall), "The way beyond exit "+e.ID+" turns here.")
		} else {
			wallDraw(id+".north", "wall", tier.ConcreteRole, fx0, fy0-thickness, fx1, fy0, 0, top, nil, "Side wall of the way beyond exit "+e.ID+".")
			wallDraw(id+".south", "wall", tier.ConcreteRole, fx0, fy1, fx1, fy1+thickness, 0, top, kerb("south"), "Side wall of the way beyond exit "+e.ID+".")
			endX := fx0 - thickness
			if e.Wall == "east" {
				endX = fx1
			}
			wallDraw(id+".end", "wall", tier.ConcreteRole, endX, fy0-thickness, endX+thickness, fy1+thickness, 0, top, kerb(e.Wall), "The way beyond exit "+e.ID+" turns here.")
		}
	}
	// Declared mass is drawn solid to the wall top: no dark floorless pocket.
	for _, m := range in.Solids {
		r, _ := s.rect(m.ID, m.Rect)
		r = shift(r)
		cut := massCut(r)
		wallDraw(s.short+".mass."+m.ID, "wall", tier.ConcreteRole, float64(r.X)*unitM, float64(r.Y)*unitM, float64(r.X+r.W)*unitM, float64(r.Y+r.H)*unitM, 0, top, cut, "Declared mass, capped at the wall top: "+m.Reason)
	}
	// Mechanisms get the worn floor outline of their frame.
	if def.MarkRole != "" {
		for _, m := range d.Mechanisms {
			x0, y0, x1, y1 := float64(m.Rect.X)*unitM, float64(m.Rect.Y)*unitM, float64(m.Rect.X+m.Rect.W)*unitM, float64(m.Rect.Y+m.Rect.H)*unitM
			for side, q := range map[string][4]float64{"north": {x0, y0, x1, y0 + 0.08}, "south": {x0, y1 - 0.08, x1, y1}, "west": {x0, y0, x0 + 0.08, y1}, "east": {x1 - 0.08, y0, x1, y1}} {
				wallDraw(m.ID+".mark."+side, "trim", def.MarkRole, q[0], q[1], q[2], q[3], 0, 0.004, nil, "Floor outline of mechanism "+m.ID+".")
			}
		}
	}
	sort.Slice(draws, func(i, j int) bool { return draws[i].ID < draws[j].ID })
	masses := map[cell]bool{}
	for c := range s.solid {
		masses[c] = true
	}
	d.Masses = mergeRects(masses)
	drops := []locationsdk.DropAnnotation{}
	elevation := int(math.Round(in.ElevationM / unitM))
	for _, w := range d.Walls {
		if w.Kind == "guard" {
			drops = append(drops, locationsdk.DropAnnotation{ID: fmt.Sprintf("%s.drop.%d.%d.%s", s.short, w.Cell.X, w.Cell.Y, w.Side), At: w.Cell, Side: locationsdk.Side(w.Side), LowerElevation: elevation - 12, Barrier: true,
				Reason: "Edge of a declared mechanism void; guarded."})
		}
	}

	// --- lamps by rule ---------------------------------------------------
	rules := in.Lights
	if len(rules) == 0 {
		rules = s.defaultLightRules()
	}
	kindOf := func(r LightRule) string {
		if r.Kind != "" {
			return r.Kind
		}
		return tier.LampKind
	}
	addLamp := func(r LightRule, id string, x, y float64, mount string, mode [3]float64) {
		h := r.HeightM
		if h == 0 {
			h = tier.LampHeightM
		}
		feed := r.Feed
		if feed == "" {
			feed = "board." + s.short
		}
		d.Lamps = append(d.Lamps, DerivedLamp{ID: id, Rule: r.ID, Kind: kindOf(r), AtM: [3]float64{x, y, h}, Cell: cell{X: int(math.Floor(x / unitM)), Y: int(math.Floor(y / unitM))}, Mount: mount, Feed: feed, Mode: mode})
	}
	for _, r := range rules {
		mode := tier.Mode
		if r.Mode != nil {
			mode = *r.Mode
		}
		if _, known := def.LampKinds[kindOf(r)]; !known {
			s.fail("unknown_lamp_kind", r.ID, nil, "lamp kind "+kindOf(r)+" is not in defaults.lamp_kinds", "use a kind from location_intent defaults")
			continue
		}
		switch r.Rule {
		case "wall_every_m":
			z, known := s.zones[r.Zone]
			step, sideOK := sideStep[r.On]
			if !known || !sideOK {
				s.fail("unknown_zone", r.ID, nil, "light rule names zone "+r.Zone+" wall "+r.On, "use a zone ID of this file and a wall side")
				continue
			}
			stepM := r.StepM
			if stepM == 0 {
				stepM = tier.LampStepM
			}
			horizontal := r.On == "north" || r.On == "south"
			span := z.H
			if horizontal {
				span = z.W
			}
			count := max(1, int(math.Floor(float64(span)*unitM/stepM)))
			edgeAt := func(k int) cell {
				switch r.On {
				case "north":
					return cell{X: z.X + k, Y: z.Y}
				case "south":
					return cell{X: z.X + k, Y: z.Y + z.H - 1}
				case "west":
					return cell{X: z.X, Y: z.Y + k}
				}
				return cell{X: z.X + z.W - 1, Y: z.Y + k}
			}
			isWall := func(k int) bool {
				if k < 0 || k >= span {
					return false
				}
				c := edgeAt(k)
				return s.wall[cell{X: c.X + step.X, Y: c.Y + step.Y}]
			}
			for i := 0; i < count; i++ {
				id := fmt.Sprintf("%s.%d", r.ID, i)
				k := int(math.Floor((float64(i) + 0.5) * float64(span) / float64(count)))
				if !isWall(k) {
					// A door under the lamp: slide to the nearest wall cell of the same side.
					c := edgeAt(k)
					out := cell{X: c.X + step.X, Y: c.Y + step.Y}
					if s.thresh[out] == "" {
						s.fail("lamp_unmounted", id, &c, "the "+r.On+" side of zone "+r.Zone+" has no wall here (it borders "+s.neighbourName(out)+")", "put the rule on a side that is an outer wall, or use rule over_fixture / at")
						continue
					}
					moved := false
					for delta := 1; delta < span && !moved; delta++ {
						for _, candidate := range []int{k - delta, k + delta} {
							if isWall(candidate) {
								k, moved = candidate, true
								break
							}
						}
					}
					if !moved {
						s.fail("lamp_over_opening", id, &c, "the whole "+r.On+" side of zone "+r.Zone+" is an opening", "use another side")
						continue
					}
				}
				c := edgeAt(k)
				x, y := (float64(c.X)+0.5)*unitM, (float64(c.Y)+0.5)*unitM
				switch r.On {
				case "north":
					y = float64(c.Y)*unitM + 0.05
				case "south":
					y = float64(c.Y+1)*unitM - 0.05
				case "west":
					x = float64(c.X)*unitM + 0.05
				case "east":
					x = float64(c.X+1)*unitM - 0.05
				}
				addLamp(r, id, x, y, "wall:"+r.On, mode)
			}
		case "over_fixture":
			f, known := fixtureByID[r.Fixture]
			if !known {
				s.fail("lamp_unmounted", r.ID, nil, "rule over_fixture names fixture "+r.Fixture+" which is not placed", "use a fixture ID of this file")
				continue
			}
			addLamp(r, r.ID, (float64(f.At.X)+float64(f.W)/2)*unitM, (float64(f.At.Y)+float64(f.D)/2)*unitM, "ceiling", mode)
		case "at":
			z, known := s.zones[r.Zone]
			if !known || r.AtM == nil {
				s.fail("unknown_zone", r.ID, nil, "rule at needs a zone and at_m", "give zone and at_m in metres of the zone")
				continue
			}
			x, y := float64(z.X)*unitM+r.AtM[0], float64(z.Y)*unitM+r.AtM[1]
			c := cell{X: int(math.Floor(x / unitM)), Y: int(math.Floor(y / unitM))}
			if s.floor[c] == "" {
				s.fail("lamp_unmounted", r.ID, &c, "there is no floor and no ceiling under this point", "move at_m inside the zone floor")
				continue
			}
			addLamp(r, r.ID, x, y, "ceiling", mode)
		case "duty_night":
			duty := tier.DutyMode
			if r.Mode != nil {
				duty = *r.Mode
			}
			for _, e := range d.Exits {
				if e.Status != "planned" {
					addLamp(LightRule{ID: r.ID, Kind: r.Kind, HeightM: exitHeight[e.ID] + 0.3, Feed: r.Feed}, r.ID+".exit."+e.ID, (float64(e.Landing.X)+0.5)*unitM, (float64(e.Landing.Y)+0.5)*unitM, "wall:"+e.Wall, duty)
				}
			}
			for _, m := range d.Mechanisms {
				addLamp(r, r.ID+".mechanism."+m.ID, (float64(m.Rect.X)+float64(m.Rect.W)/2)*unitM, (float64(m.Rect.Y)+float64(m.Rect.H)/2)*unitM, "ceiling", duty)
			}
		default:
			s.fail("unknown_light_rule", r.ID, nil, "rule "+r.Rule, "use wall_every_m, over_fixture, at or duty_night")
		}
	}

	// --- presentation ----------------------------------------------------
	finishing := &godot.Finishing{Environment: tier.Environment, Materials: map[string]godot.Material{}, SurfaceRoles: map[string]string{"floor": tier.FloorRole, "wall": tier.ConcreteRole, "steel": tier.SteelRole}, ObjectRoles: map[string]string{}}
	for k, m := range def.Materials {
		finishing.Materials[k] = m
	}
	if kit != nil && d.WallPanels > 0 {
		finishing.MaterialBindings = map[string]string{kit.ConcreteMaterial: tier.ConcreteRole, kit.SteelMaterial: tier.SteelRole}
	}
	var anchors []godot.ConcreteAnchor
	var anchorIDs []string
	conduitDone := map[int]bool{}
	for li, l := range d.Lamps {
		kind := def.LampKinds[l.Kind]
		// On a panel wall the lamp sits on the embed plate of the nearest panel
		// and is fed by a conduit along the panels' conduit line.
		if side, isWall := strings.CutPrefix(l.Mount, "wall:"); isWall && kit != nil {
			best, bestD := -1, 1.6
			for i, m := range mounts {
				if m.side != side {
					continue
				}
				across, along := math.Abs(m.y-l.AtM[1]), math.Abs(m.x-l.AtM[0])
				if side == "west" || side == "east" {
					across, along = along, across
				}
				if across < 0.2 && along < bestD {
					best, bestD = i, along
				}
			}
			if best >= 0 {
				m := mounts[best]
				if side == "north" || side == "south" {
					l.AtM[0] = m.x
				} else {
					l.AtM[1] = m.y
				}
				l.AtM[2] = kit.LampMountM
				d.Lamps[li] = l
				// Only this panel shows the embed plate inset.
				if plain := extrasPlacement[m.extra].ModelRef; plain != m.lamp {
					extrasPlacement[m.extra].ModelRef = m.lamp
					d.Models[m.lamp] = def.Models[m.lamp]
					d.WallTriangles += kit.Triangles[m.lamp] - kit.Triangles[plain]
				}
				for ri, kr := range kitRuns {
					if kr.side != side || m.x < kr.x0-0.3 || m.x > kr.x1+0.3 || m.y < kr.y0-0.3 || m.y > kr.y1+0.3 {
						continue
					}
					const c = 0.03
					fx, fy := float64(sideStep[side].X), float64(sideStep[side].Y)
					// The wall face is the run's edge on the room side.
					if !conduitDone[ri] {
						conduitDone[ri] = true
						if side == "north" || side == "south" {
							face := kr.y1
							if side == "south" {
								face = kr.y0
							}
							q0, q1 := face-fy*c-c/2-fy*c/2, face-fy*c+c/2-fy*c/2
							wallDraw(fmt.Sprintf("%s.conduit.%s.%d", s.short, side, ri), "trim", tier.SteelRole, kr.x0+thickness, math.Min(q0, q1), kr.x1-thickness, math.Max(q0, q1), kit.ConduitM-c/2, kit.ConduitM+c/2, nil, "Conduit along the panels' conduit line: it feeds the lamps of this wall from the board.")
						} else {
							face := kr.x1
							if side == "east" {
								face = kr.x0
							}
							q0, q1 := face-fx*c-c/2-fx*c/2, face-fx*c+c/2-fx*c/2
							wallDraw(fmt.Sprintf("%s.conduit.%s.%d", s.short, side, ri), "trim", tier.SteelRole, math.Min(q0, q1), kr.y0, math.Max(q0, q1), kr.y1, kit.ConduitM-c/2, kit.ConduitM+c/2, nil, "Conduit along the panels' conduit line: it feeds the lamps of this wall from the board.")
						}
					}
					dx0, dy0 := m.x-fx*c*1.5-c/2, m.y-fy*c*1.5-c/2
					wallDraw(l.ID+".drop", "trim", tier.SteelRole, dx0, dy0, dx0+c, dy0+c, kit.LampMountM+0.05, kit.ConduitM, nil, "Drop of the conduit to lamp "+l.ID+".")
					break
				}
			}
		}
		at := placement.Vec3{l.AtM[0], in.ElevationM + l.AtM[2], l.AtM[1]}
		aim := placement.Vec3{l.AtM[0], in.ElevationM, l.AtM[1]}
		yaw := 0.0
		if side, isWall := strings.CutPrefix(l.Mount, "wall:"); isWall {
			reach := tier.LampAimM
			if reach == 0 {
				reach = 1.5
			}
			aim[0] -= float64(sideStep[side].X) * reach
			aim[2] -= float64(sideStep[side].Y) * reach
			yaw = float64(wallRotation[side])
			if kind.Housing != nil && !((side == "south" || side == "east") && tier.NearCutM > 0) {
				// A flush housing on the wall: a steel frame and a lit lens.
				hs := kind.Housing.SizeM
				fx, fy := float64(sideStep[side].X), float64(sideStep[side].Y)
				face := [2]float64{l.AtM[0] + fx*0.05, l.AtM[1] + fy*0.05}
				part := func(id, role string, along, high, depth, lift float64) {
					cx, cy := face[0]-fx*depth/2, face[1]-fy*depth/2
					wx, wy := along, depth
					if side == "west" || side == "east" {
						wx, wy = depth, along
					}
					draws = append(draws, godot.ArchitectureDraw{ID: id, Kind: "trim", MaterialRole: role, Position: [3]float64{cx, in.ElevationM + l.AtM[2] + lift, cy}, Size: [3]float64{wx, high, wy}, Reason: "Flush housing of lamp " + l.ID + " on its wall."})
				}
				part(l.ID+".frame", kind.Housing.FrameRole, hs[0], hs[1], hs[2], 0)
				part(l.ID+".lens", kind.Housing.LensRole, hs[0]-0.06, hs[1]-0.06, hs[2]+0.01, 0)
			}
			if tier.LampStandoffM > 0 {
				at[0] -= float64(sideStep[side].X) * tier.LampStandoffM
				at[2] -= float64(sideStep[side].Y) * tier.LampStandoffM
			}
			if len(anchors) < 24 {
				anchors = append(anchors, godot.ConcreteAnchor{ID: l.ID, Kind: "lamp", SourceRef: l.ID, Position: at, Strength: 0.5, RadiusM: 0.48})
				anchorIDs = append(anchorIDs, l.ID)
			}
		}
		energy, reachM := kind.Energy, kind.RangeM
		if tier.LightGain > 0 {
			energy *= tier.LightGain
		}
		if tier.LampRangeM > 0 {
			reachM = tier.LampRangeM
		}
		finishing.Lights = append(finishing.Lights, godot.Light{ID: l.ID, At: at, Aim: aim, Color: kind.Color, Energy: energy, Range: reachM, Mode: l.Mode, PresentationOnly: true,
			Source: kind.Source + "; intent rule " + l.Rule + ", fed from " + l.Feed})
		// A lamp body is shown only where it visibly hangs on something: a full
		// wall. On a camera-cut wall or under the unseen ceiling it would float.
		onFullWall := strings.HasPrefix(l.Mount, "wall:") && !((l.Mount == "wall:south" || l.Mount == "wall:east") && tier.NearCutM > 0)
		if kind.ModelRef != "" && def.Models[kind.ModelRef] != "" && onFullWall {
			d.Models[kind.ModelRef] = def.Models[kind.ModelRef]
			extrasPlacement = append(extrasPlacement, placement.Extra{InstanceID: l.ID, ModelRef: kind.ModelRef, SourceKind: "authored_fixture", PositionPlanM: at, RotationDegrees: yaw,
				Reason: "Lamp body of rule " + l.Rule + " on its mount (" + l.Mount + ")."})
		}
	}
	// The way beyond an open exit is lit by the neighbour's own lamp, so the
	// threshold and the corridor floor read from the room.
	for i, at := range beyondLights {
		kind := def.LampKinds[tier.LampKind]
		energy := kind.Energy
		if tier.LightGain > 0 {
			energy *= tier.LightGain
		}
		finishing.Lights = append(finishing.Lights, godot.Light{ID: beyondIDs[i], At: placement.Vec3{at[0], in.ElevationM + at[1], at[2]}, Aim: placement.Vec3{at[0], in.ElevationM, at[2]},
			Color: kind.Color, Energy: energy, Range: 6, Mode: tier.Mode, PresentationOnly: true, Source: kind.Source + "; light of the way beyond an open exit"})
	}
	sort.Slice(draws, func(i, j int) bool { return draws[i].ID < draws[j].ID })
	if in.Setting == "dome_interior" {
		function := []string{}
		if len(anchorIDs) > 0 {
			function = append(function, "lamp")
		}
		finishing.Architecture = &godot.Architecture{Setting: in.Setting, WallCaste: in.Tier, ConcreteRole: tier.ConcreteRole, SourceAnchors: anchors, Draws: draws,
			WallZones: map[string]godot.WallZone{}, WallOverrides: map[string]godot.WallOverride{},
			Concrete: &godot.ConcreteInstance{Tier: in.Tier, Age: tier.Age, Function: function, Exposure: "public", Seed: in.Seed, FloorM: in.ElevationM, Handwear: true, Anchors: anchorIDs}}
		finishing.Architecture.Concrete.Enamel = tier.Enamel
		if tier.CariesStrength > 0 {
			// The outwardly intact tooth: a small hidden damp film at the foot of a
			// real wall, by the first way in. The source is that wall's own draw.
			source, at := "", placement.Vec3{float64(s.width) * unitM / 2, in.ElevationM + 0.07, float64(s.depth) * unitM / 2}
			for _, w := range draws {
				if w.Kind == "wall" {
					source = w.ID
					break
				}
			}
			if len(d.Exits) > 0 {
				e := d.Exits[0]
				at = placement.Vec3{(float64(e.Landing.X) + 0.5) * unitM, in.ElevationM + 0.07, (float64(e.Landing.Y) + 0.5) * unitM}
			}
			if source != "" {
				finishing.Architecture.Concrete.Caries = &godot.ConcreteCaries{Strength: tier.CariesStrength, Hints: []godot.ConcreteAnchor{{ID: s.short + ".hidden.damp", Kind: "steel", Effect: "damp", SourceRef: source, Position: at, Strength: tier.CariesStrength, RadiusM: 0.05, Hidden: true}}}
			}
		}
	} else {
		finishing.Architecture = &godot.Architecture{Setting: in.Setting, Draws: draws}
	}
	d.Finishing = finishing

	// Camera: the centre of the walkable floor, so the room fills the frame.
	sumX, sumY := 0.0, 0.0
	for c := range s.floor {
		sumX += float64(c.X) + 0.5
		sumY += float64(c.Y) + 0.5
	}
	focus := placement.Vec3{sumX / float64(len(s.floor)) * unitM, in.ElevationM + 1, sumY / float64(len(s.floor)) * unitM}
	layoutID := "intent." + s.short
	if in.LayoutID != "" {
		layoutID = in.LayoutID
	}
	d.Authoring = placement.Authoring{LayoutID: layoutID, Camera: &placement.Camera{FocusPlanM: focus, SizeM: hero, DistanceM: 120}, ModuleModels: moduleModels, Instances: instanceSpecs, Extras: extrasPlacement}
	d.Layout = locationsdk.Layout{ID: layoutID, UnitMM: locationsdk.UnitMM, Seed: int64(in.Seed), Width: s.width, Depth: s.depth, Elevation: elevation, Grounded: true,
		Camera: &locationsdk.LayoutCamera{Projection: "orthographic", ElevationDegrees: 35, AzimuthDegrees: 37, HeroViewHeightM: hero},
		Floor:  floor, Entries: entries, Instances: instances, ReservedPaths: []locationsdk.ReservedPath{}, Drops: drops}
	if in.Setting == "dome_interior" {
		d.Layout.Setting, d.Layout.Interior = in.Setting, interior
	}
	d.Catalog = catalog
	own := map[string]bool{}
	for _, g := range extras.Geometries {
		own[g.ID] = true
	}
	d.Catalog.Geometries, d.Catalog.Modules = append([]locationsdk.Geometry{}, extras.Geometries...), append([]locationsdk.Module{}, extras.Modules...)
	for _, g := range catalog.Geometries {
		if !own[g.ID] {
			d.Catalog.Geometries = append(d.Catalog.Geometries, g)
		}
	}
	for _, m := range catalog.Modules {
		if !own[m.GeometryID] {
			d.Catalog.Modules = append(d.Catalog.Modules, m)
		}
	}

	s.world(d, fixtureByID)
	s.coherence(d, entries)
	return d, s.diags
}

func indexExit(exits []Exit, id string) int {
	for i, e := range exits {
		if e.ID == id {
			return i
		}
	}
	return 0
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func (s *state) neighbourName(c cell) string {
	if z := s.floor[c]; z != "" {
		return "zone " + z
	}
	if v := s.void[c]; v != "" {
		return "the void of " + v
	}
	return "nothing"
}

// defaultLightRules light a bare intent in the manner of its caste: lamps
// along every long outer wall and duty light at exits and mechanisms.
func (s *state) defaultLightRules() []LightRule {
	rules := []LightRule{{ID: "default.duty", Rule: "duty_night"}}
	for _, z := range s.in.Zones {
		r := s.zones[z.ID]
		for _, side := range sideOrder {
			span := r.H
			if side == "north" || side == "south" {
				span = r.W
			}
			if float64(span)*unitM < s.tier.LampStepM {
				continue
			}
			whole := true
			for k := 0; k < span && whole; k++ {
				var c cell
				switch side {
				case "north":
					c = cell{X: r.X + k, Y: r.Y - 1}
				case "south":
					c = cell{X: r.X + k, Y: r.Y + r.H}
				case "west":
					c = cell{X: r.X - 1, Y: r.Y + k}
				default:
					c = cell{X: r.X + r.W, Y: r.Y + k}
				}
				whole = s.wall[c] || s.thresh[c] != ""
			}
			if whole {
				rules = append(rules, LightRule{ID: "default." + z.ID + "." + side, Rule: "wall_every_m", Zone: z.ID, On: side})
			}
		}
	}
	return rules
}
