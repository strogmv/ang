package locationsdk

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

// Multi-deck layouts (document 80). One shared plan grid carries several flat
// surfaces; stairs join them with ordinary walking steps. The movement kernel
// keeps its integer 2D cells: every walkable surface becomes one band of a
// navigation plane (rows offset by surface), each stair becomes its own small
// block of nodes, and the only non-adjacent graph edges are explicit links at
// the stair ends. Occupancy, fog memory and paths are therefore per surface.
// This is a declared-geometry check, not a structural or code-compliance
// calculation; every number below is a documented working rule.

const (
	NavigationSchema = "ang.multideck.v1"
	SightEyeMM       = 1600
	// Hero sight range on multi-deck SDK stands: 12 m, the market's metric range.
	SightRangeCells        = 24
	DefaultClearanceMM     = 2100
	MaxSlabSpanMM          = 4000
	MaxBeamSpanMM          = 8000
	DefaultBeamDepthMM     = 400
	MinRiserMM             = 150
	MaxRiserMM             = 180
	MinGoingMM             = 280
	MaxGoingMM             = 320
	MinStepRuleMM          = 600
	MaxStepRuleMM          = 660
	MaxStairSlopeDegrees   = 35.0
	DropRailingThresholdMM = 500
	MinGuardHeightMM       = 1100
	MaxGuardHeightMM       = 1500
	MinStairWidthCells     = 2
	maxSurfaceCount        = 32
	maxConnectorCount      = 64
)

// SightTargetMM are the heights above a standing cell that make it visible:
// a torso and a head point. A cell is seen when either is in line of sight.
var SightTargetMM = []int{1000, 1700}

type Navigation struct {
	Schema        string            `json:"schema"`
	PlanWidth     int               `json:"plan_width"`
	PlanDepth     int               `json:"plan_depth"`
	NavWidth      int               `json:"nav_width"`
	NavDepth      int               `json:"nav_depth"`
	EyeMM         int               `json:"eye_mm"`
	TargetMM      []int             `json:"target_mm"`
	SightRange    int               `json:"sight_range_cells"`
	BaseSurface   string            `json:"base_surface"`
	Surfaces      []NavSurface      `json:"surfaces"`
	Connectors    []NavConnector    `json:"connectors"`
	Links         []NavLink         `json:"links"`
	Obstacles     []NavObstacle     `json:"obstacles"`
	Railings      []NavRailing      `json:"railings"`
	Supports      []NavSupport      `json:"supports"`
	CutawayGroups []NavCutawayGroup `json:"cutaway_groups"`
	// Sight blockers as absolute vertical intervals over plan rectangles and
	// plan cell edges. Slabs come from Surfaces; these are everything else.
	Solids []SightSpan `json:"solids"`
	Edges  []SightEdge `json:"edges"`
}

// NavSurface: NavRow is the first navigation row of this surface's band, or -1
// for a non-walkable roof. Plan cell (x,y) of a walkable surface is navigation
// cell (x, NavRow+y). The base surface always has NavRow 0.
type NavSurface struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Geometry     string `json:"geometry"`
	Index        int    `json:"index"`
	ElevationMM  int    `json:"elevation_mm"`
	SlabMM       int    `json:"slab_mm"`
	Walkable     bool   `json:"walkable"`
	Grounded     bool   `json:"grounded,omitempty"`
	NavRow       int    `json:"nav_row"`
	CutawayGroup string `json:"cutaway_group,omitempty"`
	Slab         []Rect `json:"slab"`
	Reason       string `json:"reason"`
}

type NavFlight struct {
	Risers    int     `json:"risers"`
	Length    int     `json:"length"`
	StartStep int     `json:"start_step"`
	BaseMM    int     `json:"base_mm"`
	RiserMM   float64 `json:"riser_mm"`
	GoingMM   float64 `json:"going_mm"`
}

type NavLanding struct {
	Length    int `json:"length"`
	StartStep int `json:"start_step"`
	HeightMM  int `json:"height_mm"`
}

type NavNode struct {
	Nav      Cell `json:"nav"`
	Plan     Cell `json:"plan"`
	Lane     int  `json:"lane"`
	Step     int  `json:"step"`
	HeightMM int  `json:"height_mm"`
}

type NavConnector struct {
	ID           string       `json:"id"`
	Kind         string       `json:"kind"`
	FromSurface  string       `json:"from_surface"`
	ToSurface    string       `json:"to_surface"`
	From         Cell         `json:"from"`
	To           Cell         `json:"to"`
	Direction    Side         `json:"direction"`
	Width        int          `json:"width"`
	Length       int          `json:"length"`
	ClearanceMM  int          `json:"clearance_mm"`
	NavOrigin    Cell         `json:"nav_origin"`
	BottomMM     int          `json:"bottom_mm"`
	TopMM        int          `json:"top_mm"`
	Flights      []NavFlight  `json:"flights"`
	Landings     []NavLanding `json:"landings"`
	Balustrades  Balustrades  `json:"balustrades"`
	CutawayGroup string       `json:"cutaway_group,omitempty"`
	Construction string       `json:"construction,omitempty"`
	Footprint    []Cell       `json:"footprint"`
	Nodes        []NavNode    `json:"nodes"`
	Reason       string       `json:"reason"`
}

// NavLink is an undirected navigation edge between two cells that are not
// grid neighbours in the navigation plane (a stair end and its landing).
type NavLink struct {
	A         Cell   `json:"a"`
	B         Cell   `json:"b"`
	Connector string `json:"connector"`
}

type NavObstacle struct {
	ID             string `json:"id"`
	Surface        string `json:"surface"`
	Style          string `json:"style"`
	Rect           Rect   `json:"rect"`
	BottomMM       int    `json:"bottom_mm"`
	TopMM          int    `json:"top_mm"`
	BlocksMovement bool   `json:"blocks_movement"`
	BlocksSight    bool   `json:"blocks_sight"`
	Reason         string `json:"reason"`
}

type NavRailing struct {
	ID          string `json:"id"`
	Surface     string `json:"surface"`
	From        Cell   `json:"from"`
	To          Cell   `json:"to"`
	Side        Side   `json:"side"`
	BaseMM      int    `json:"base_mm"`
	HeightMM    int    `json:"height_mm"`
	BlocksSight bool   `json:"blocks_sight"`
	Reason      string `json:"reason"`
}

type NavSupport struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Surface  string `json:"surface"`
	RestsOn  string `json:"rests_on,omitempty"`
	Cells    []Cell `json:"cells"`
	BottomMM int    `json:"bottom_mm"`
	TopMM    int    `json:"top_mm"`
	Reason   string `json:"reason"`
}

type NavCutawayGroup struct {
	ID         string   `json:"id"`
	Policy     string   `json:"policy"`
	Surfaces   []string `json:"surfaces"`
	Connectors []string `json:"connectors"`
	// Floor instances of an occluding group (a tall fixture in front of the hero), demo-hub v12-vending.
	Instances []string `json:"instances,omitempty"`
	Interior  []Rect   `json:"interior"`
	BottomMM  int      `json:"bottom_mm"`
	TopMM     int      `json:"top_mm"`
	Reason    string   `json:"reason"`
}

type SightSpan struct {
	Rect     Rect   `json:"rect"`
	BottomMM int    `json:"bottom_mm"`
	TopMM    int    `json:"top_mm"`
	Source   string `json:"source"`
}

type SightEdge struct {
	At       Cell   `json:"at"`
	Side     Side   `json:"side"`
	BottomMM int    `json:"bottom_mm"`
	TopMM    int    `json:"top_mm"`
	Source   string `json:"source"`
}

type CompiledResident struct {
	ID     string                 `json:"id"`
	Name   string                 `json:"name"`
	Reason string                 `json:"reason"`
	Steps  []CompiledResidentStep `json:"steps"`
}

type CompiledResidentStep struct {
	AtMinute   int    `json:"at_minute"`
	Surface    string `json:"surface"`
	At         Cell   `json:"at"`
	Nav        Cell   `json:"nav"`
	Activity   string `json:"activity"`
	Label      string `json:"label"`
	MinSeconds int    `json:"min_seconds"`
}

// IsMultideck reports any multi-surface authoring. Such fields without at
// least one deck are rejected rather than silently compiled as a flat floor.
func (l Layout) IsMultideck() bool {
	return len(l.Decks) > 0 || len(l.Connectors) > 0 || len(l.Supports) > 0 || len(l.Railings) > 0 || len(l.Obstacles) > 0 || len(l.CutawayGroups) > 0 || l.BaseSurfaceID != ""
}

type mdSurface struct {
	id, kind, group, reason string
	index, elevation, slab  int
	walkable, grounded      bool
	navRow                  int
	hasSlab                 []bool
	blocked                 []bool // movement-blocked on this surface
	sightClear              []bool // blocked but transparent at standing height
}

type mdInterval struct {
	bottom, top int
	source      string
}

type mdBuilder struct {
	layout   Layout
	out      *CompiledLevel
	w, d     int
	surfaces []*mdSurface
	byID     map[string]*mdSurface
	ids      map[string]string // element ID -> element kind (global uniqueness)
	groups   map[string]CutawayGroup
	// vertical movement/sight blockers per plan cell (absolute mm)
	walls      map[int][]mdInterval // movement-blocking verticals (walls, columns, glazing, instances)
	sightSolid map[int][]mdInterval
	nodeAt     map[int]NavNode // plan index -> stair node (one connector per plan cell)
	nodeOwner  map[int]string
	landingOf  map[mdKey]string // (surface, plan) -> connector ID
	headEdges  map[mdEdgeKey]string
	beams      map[*mdSurface]map[int]int // plan index -> beam bottom mm
	supportAt  map[*mdSurface]map[int]bool
	nav        *Navigation
}

type mdKey struct {
	surface string
	at      Cell
}

type mdEdgeKey struct {
	surface string
	at      Cell
	side    Side
}

func (b *mdBuilder) inside(c Cell) bool { return c.X >= 0 && c.Y >= 0 && c.X < b.w && c.Y < b.d }
func (b *mdBuilder) index(c Cell) int   { return c.Y*b.w + c.X }

func (b *mdBuilder) claim(id, kind, path string) *ValidationError {
	if !stableID.MatchString(id) {
		return issue("invalid_id", path+".id", kind+" needs a stable identifier")
	}
	if previous, ok := b.ids[id]; ok {
		return issue("duplicate_element_id", path+".id", fmt.Sprintf("%s ID %q is already used by a %s; override registries address every element by its stable ID", kind, id, previous), id)
	}
	b.ids[id] = kind
	return nil
}

func rectCells(r Rect) []Cell {
	out := make([]Cell, 0, r.W*r.H)
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			out = append(out, Cell{x, y})
		}
	}
	return out
}

func (b *mdBuilder) rectInside(r Rect) bool {
	return r.X >= 0 && r.Y >= 0 && r.W >= 1 && r.H >= 1 && r.X < b.w && r.Y < b.d && r.W <= b.w-r.X && r.H <= b.d-r.Y
}

// cellsToRects merges cells into row runs, then stacks identical runs.
func cellsToRects(cells []bool, width int) []Rect {
	type run struct{ x, w int }
	var out []Rect
	open := map[run]int{}
	depth := len(cells) / width
	for y := 0; y < depth; y++ {
		current := map[run]int{}
		for x := 0; x < width; {
			if !cells[y*width+x] {
				x++
				continue
			}
			start := x
			for x < width && cells[y*width+x] {
				x++
			}
			r := run{start, x - start}
			if i, ok := open[r]; ok {
				out[i].H++
				current[r] = i
			} else {
				out = append(out, Rect{X: start, Y: y, W: r.w, H: 1})
				current[r] = len(out) - 1
			}
		}
		open = current
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Y != out[j].Y {
			return out[i].Y < out[j].Y
		}
		return out[i].X < out[j].X
	})
	return out
}

func intervalsCover(list []mdInterval, lo, hi int) bool {
	sorted := append([]mdInterval{}, list...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].bottom < sorted[j].bottom })
	reach := lo
	for _, iv := range sorted {
		if iv.bottom > reach {
			break
		}
		if iv.top > reach {
			reach = iv.top
		}
		if reach >= hi {
			return true
		}
	}
	return reach >= hi
}

func lateralOf(direction Side) Cell {
	if direction == North || direction == South {
		return Cell{1, 0}
	}
	return Cell{0, 1}
}

func oppositeSide(s Side) Side {
	switch s {
	case North:
		return South
	case South:
		return North
	case East:
		return West
	}
	return East
}

// compileMultideck runs after the planar base checks. It extends the base
// surface with obstacles, columns and stair bodies, adds the authored decks,
// validates the engineering rules, and replaces out.Terrain by the full
// navigation plane. The base band keeps plan coordinates (navigation row = y).
func compileMultideck(p preparedCatalog, layout Layout, out *CompiledLevel, support, blocked []bool, material map[Cell]string, sightClear map[int]bool) *ValidationError {
	b := &mdBuilder{layout: layout, out: out, w: layout.Width, d: layout.Depth, byID: map[string]*mdSurface{}, ids: map[string]string{}, groups: map[string]CutawayGroup{}, walls: map[int][]mdInterval{}, sightSolid: map[int][]mdInterval{}, nodeAt: map[int]NavNode{}, nodeOwner: map[int]string{}, landingOf: map[mdKey]string{}, headEdges: map[mdEdgeKey]string{}, beams: map[*mdSurface]map[int]int{}, supportAt: map[*mdSurface]map[int]bool{}}
	if len(layout.Decks) == 0 {
		return issue("multideck_without_decks", "layout", "connectors, supports, railings, obstacles, cutaway groups and base_surface_id require at least one authored deck")
	}
	if len(layout.Decks) > maxSurfaceCount-1 || len(layout.Connectors) > maxConnectorCount || len(layout.Supports) > 4096 || len(layout.Railings) > 4096 || len(layout.Obstacles) > 4096 || len(layout.CutawayGroups) > 256 {
		return issue("multideck_budget", "layout", "multi-deck layout exceeds 31 decks, 64 connectors or 4096 supports/railings/obstacles")
	}
	if len(layout.ReservedPaths) > 0 {
		return issue("unsupported_multideck_guides", "layout.reserved_paths", "floor guides/wayfinding lines are single-surface only; multi-deck guides are a later step")
	}
	count := b.w * b.d
	baseID := layout.BaseSurfaceID
	if baseID == "" {
		baseID = "base"
	}
	if err := b.claim(baseID, "surface", "layout.base_surface_id"); err != nil {
		return err
	}
	for _, instance := range out.Instances {
		b.ids[instance.ID] = "instance"
	}
	baseKind := "deck"
	if layout.Grounded {
		baseKind = "ground"
	}
	base := &mdSurface{id: baseID, kind: baseKind, walkable: true, grounded: layout.Grounded, elevation: layout.Elevation * UnitMM, slab: 250, reason: "Base surface of the layout (planar floor, instances and drops).", hasSlab: append([]bool{}, support...), blocked: append([]bool{}, blocked...), sightClear: make([]bool, count)}
	for n := range sightClear {
		base.sightClear[n] = true
	}
	b.surfaces = append(b.surfaces, base)
	b.byID[baseID] = base
	// Base instances are vertical blockers with their real module heights.
	for _, instance := range out.Instances {
		low := cellSet(instance.SightClearCells)
		for _, at := range instance.Blocked {
			n := b.index(at)
			iv := mdInterval{base.elevation, base.elevation + instance.Height*UnitMM, instance.ID}
			b.walls[n] = append(b.walls[n], iv)
			if !instance.SightClear && !low[at] {
				b.sightSolid[n] = append(b.sightSolid[n], iv)
			}
		}
	}
	for i, group := range layout.CutawayGroups {
		path := fmt.Sprintf("layout.cutaway_groups[%d]", i)
		// Presentation groups have their own namespace; a deck may share its name.
		if !stableID.MatchString(group.ID) {
			return issue("invalid_id", path+".id", "cutaway group needs a stable identifier")
		}
		if _, dup := b.groups[group.ID]; dup {
			return issue("duplicate_id", path+".id", "cutaway group ID is repeated", group.ID)
		}
		if strings.TrimSpace(group.Reason) == "" {
			return issue("cutaway_group", path, "cutaway group needs an authored reason")
		}
		switch group.Policy {
		case "occluding":
			if len(group.Interior) > 0 {
				return issue("cutaway_group", path+".interior", "an occluding group fades when it hides the hero; interior volumes belong to building groups")
			}
		case "building":
			if len(group.Interior) == 0 {
				return issue("cutaway_group", path+".interior", "a building group needs at least one interior rectangle")
			}
			for j, r := range group.Interior {
				if !b.rectInside(r) {
					return issue("cutaway_group", fmt.Sprintf("%s.interior[%d]", path, j), "interior rectangle must lie inside the plan")
				}
			}
		default:
			return issue("unsupported_cutaway_policy", path+".policy", "supported cutaway policies are occluding and building")
		}
		b.groups[group.ID] = group
	}
	for i, instance := range layout.Instances {
		if instance.CutawayGroup == "" {
			continue
		}
		if g, ok := b.groups[instance.CutawayGroup]; !ok || g.Policy != "occluding" {
			return issue("instance_cutaway_group", fmt.Sprintf("layout.instances[%d].cutaway_group", i), "an instance may only join a declared occluding cutaway group", instance.CutawayGroup)
		}
	}
	decks := append([]Deck{}, layout.Decks...)
	sort.Slice(decks, func(i, j int) bool { return decks[i].ID < decks[j].ID })
	for i, deck := range layout.Decks {
		path := fmt.Sprintf("layout.decks[%d]", i)
		if err := b.claim(deck.ID, "surface", path); err != nil {
			return err
		}
		if strings.TrimSpace(deck.Reason) == "" {
			return issue("deck_definition", path+".reason", "every deck keeps its authored purpose")
		}
		switch deck.Kind {
		case "deck":
			if !deck.Walkable {
				return issue("deck_definition", path+".walkable", "a deck is a walkable floor; use kind roof for a slab without access")
			}
		case "roof":
		case "ground":
			return issue("unsupported_ground_surface", path+".kind", "additional ground surfaces (terrain with local height) are reserved for outdoor maps and not implemented")
		default:
			return issue("deck_definition", path+".kind", "supported surface kinds are deck and roof; ground is reserved")
		}
		if deck.Geometry != "" && deck.Geometry != "flat" {
			if deck.Geometry == "heightfield" {
				return issue("unsupported_surface_geometry", path+".geometry", "heightfield surfaces (slopes, local heights) are reserved; only flat decks are implemented")
			}
			return issue("deck_definition", path+".geometry", "surface geometry must be flat (heightfield is reserved)")
		}
		if deck.ElevationMM < -1000000 || deck.ElevationMM > 1000000 || deck.SlabMM < 100 || deck.SlabMM > 1500 {
			return issue("deck_definition", path, "deck elevation must be within ±1000 m of the map zero and slab thickness 100..1500 mm")
		}
		if _, ok := b.groups[deck.CutawayGroup]; !ok {
			return issue("deck_cutaway_group", path+".cutaway_group", "every deck and roof names its explicit cutaway group", deck.CutawayGroup)
		}
		if len(deck.Floor) == 0 {
			return issue("deck_definition", path+".floor", "a deck needs at least one slab rectangle")
		}
	}
	for _, deck := range decks {
		s := &mdSurface{id: deck.ID, kind: deck.Kind, group: deck.CutawayGroup, reason: deck.Reason, walkable: deck.Walkable, elevation: deck.ElevationMM, slab: deck.SlabMM, hasSlab: make([]bool, count), blocked: make([]bool, count), sightClear: make([]bool, count)}
		floorIDs := map[string]bool{}
		for j, area := range deck.Floor {
			if !stableID.MatchString(area.ID) || floorIDs[area.ID] || strings.TrimSpace(area.Reason) == "" {
				return issue("floor_identity", fmt.Sprintf("deck %s floor[%d]", deck.ID, j), "each deck slab rectangle needs a unique ID and an authored reason", deck.ID)
			}
			floorIDs[area.ID] = true
			if !b.rectInside(area.Rect) {
				return issue("floor_out_of_bounds", fmt.Sprintf("deck %s floor[%d]", deck.ID, j), "deck slab rectangle must lie within the plan", deck.ID)
			}
			for _, at := range rectCells(area.Rect) {
				s.hasSlab[b.index(at)] = true
			}
		}
		for j, area := range deck.Openings {
			if !stableID.MatchString(area.ID) || floorIDs[area.ID] || strings.TrimSpace(area.Reason) == "" || !b.rectInside(area.Rect) {
				return issue("deck_opening", fmt.Sprintf("deck %s openings[%d]", deck.ID, j), "an opening needs a unique ID, reason and a rectangle inside the plan", deck.ID)
			}
			floorIDs[area.ID] = true
			for _, at := range rectCells(area.Rect) {
				if !s.hasSlab[b.index(at)] {
					return issue("deck_opening", fmt.Sprintf("deck %s openings[%d]", deck.ID, j), fmt.Sprintf("opening cell (%d,%d) is not inside the deck slab", at.X, at.Y), deck.ID)
				}
			}
		}
		for _, area := range deck.Openings {
			for _, at := range rectCells(area.Rect) {
				s.hasSlab[b.index(at)] = false
			}
		}
		s.index = len(b.surfaces)
		b.surfaces = append(b.surfaces, s)
		b.byID[s.id] = s
	}
	// Slabs never intersect, and a grounded base has no basement beneath it.
	for n := 0; n < count; n++ {
		var slabs []mdInterval
		for _, s := range b.surfaces {
			if !s.hasSlab[n] {
				continue
			}
			if s != base && base.grounded && base.hasSlab[n] && s.elevation < base.elevation {
				return issue("unsupported_basement", "layout.decks", fmt.Sprintf("deck %s lies under the grounded base floor at (%d,%d); a basement needs its own supported base slab (not implemented)", s.id, n%b.w, n/b.w), s.id)
			}
			slabs = append(slabs, mdInterval{s.elevation - s.slab, s.elevation, s.id})
		}
		for i := range slabs {
			for j := i + 1; j < len(slabs); j++ {
				if slabs[i].bottom < slabs[j].top && slabs[j].bottom < slabs[i].top {
					return issue("slab_overlap", "layout.decks", fmt.Sprintf("slabs %s and %s intersect at (%d,%d)", slabs[i].source, slabs[j].source, n%b.w, n/b.w), slabs[i].source, slabs[j].source)
				}
			}
		}
	}
	if err := b.obstacles(); err != nil {
		return err
	}
	if err := b.columns(); err != nil {
		return err
	}
	if err := b.connectors(); err != nil {
		return err
	}
	if err := b.wallsAndBeams(); err != nil {
		return err
	}
	if err := b.supportCoverage(); err != nil {
		return err
	}
	if err := b.railings(); err != nil {
		return err
	}
	if err := b.edgeProtection(); err != nil {
		return err
	}
	if err := b.headroom(); err != nil {
		return err
	}
	// Base movement changes must not obstruct planar access already accepted.
	for i, entry := range layout.Entries {
		if base.blocked[b.index(entry)] {
			return issue("entry_obstructed", fmt.Sprintf("layout.entries[%d]", i), "entry is covered by an obstacle, column or stair body")
		}
	}
	for _, port := range out.Ports {
		for _, at := range append(append([]Cell{port.At}, port.Opening...), port.Approach...) {
			if base.blocked[b.index(at)] && material[at] == "" {
				return issue("port_obstructed", "layout", fmt.Sprintf("module port access (%d,%d) is covered by a multi-deck element", at.X, at.Y), port.InstanceID, port.ID)
			}
		}
	}
	for _, clearance := range out.Clearances {
		for _, at := range clearance.Cells {
			if base.blocked[b.index(at)] {
				return issue("clearance_obstructed", "layout", fmt.Sprintf("%s access at (%d,%d) is covered by a multi-deck element", clearance.Kind, at.X, at.Y), clearance.InstanceID)
			}
		}
	}
	for _, drop := range out.Drops {
		if base.blocked[b.index(drop.At)] {
			return issue("drop_annotation", "layout.drops", "drop origin is covered by a multi-deck element", drop.ID)
		}
	}
	if err := b.navigation(); err != nil {
		return err
	}
	if layout.Camera != nil && layout.Camera.NearCut != nil {
		out.NearCut = b.nearCut(*layout.Camera)
	}
	return nil
}

// NearCut is the presentation rule of the walls nearest to the camera (owner 31.10):
// a wall segment (a cell of a wall-style obstacle) is drawn cut at HeightMM with a
// dark cap when, seen from the layout camera, it would hide a standing body (above
// the cut) on some walkable cell of any surface behind it. Such a segment's face
// toward the camera is necessarily a south or east face of the mirrored 35°/37°
// camera; back walls and walls with nothing walkable behind keep their full height.
// Physics, Go terrain and NPC sight are untouched.
type NearCut struct {
	HeightMM int              `json:"height_mm"`
	Segments []NearCutSegment `json:"segments"`
}

type NearCutSegment struct {
	Obstacle string `json:"obstacle"`
	Cells    []Cell `json:"cells"`
}

// nearCut marches, from every walkable cell, two body points (just above the cut and
// at head height) toward the camera; the wall cells their rays pass through between
// the wall's bottom and top are cut. Explicitly listed obstacles are cut entirely.
func (b *mdBuilder) nearCut(camera LayoutCamera) *NearCut {
	cut := *camera.NearCut
	el, az := camera.ElevationDegrees*math.Pi/180, camera.AzimuthDegrees*math.Pi/180
	ux, uy, rise := math.Sin(az), math.Cos(az), math.Tan(el) // toward the camera in plan (x east, y south); m up per m
	type wallPart struct {
		id          string
		bottom, top int
	}
	walls := map[int][]wallPart{}
	maxTop := 0
	for _, o := range b.layout.Obstacles {
		s := b.byID[o.Surface]
		// Parts wholly above the cut (a lintel over an opening) count too: hiding a body, they are not drawn at all.
		if s == nil || o.Style != "wall" {
			continue
		}
		for _, at := range rectCells(o.Rect) {
			walls[b.index(at)] = append(walls[b.index(at)], wallPart{o.ID, s.elevation + o.BottomMM, s.elevation + o.TopMM})
			maxTop = max(maxTop, s.elevation+o.TopMM)
		}
	}
	hit := map[string]map[Cell]bool{}
	mark := func(id string, at Cell) {
		if hit[id] == nil {
			hit[id] = map[Cell]bool{}
		}
		hit[id][at] = true
	}
	for _, id := range cut.Obstacles {
		for _, o := range b.layout.Obstacles {
			if o.ID == id {
				for _, at := range rectCells(o.Rect) {
					mark(id, at)
				}
			}
		}
	}
	const step = 0.25 // cells
	for _, s := range b.surfaces {
		if !s.walkable {
			continue
		}
		for y := 0; y < b.d; y++ {
			for x := 0; x < b.w; x++ {
				n := y*b.w + x
				if !s.hasSlab[n] || s.blocked[n] {
					continue
				}
				for _, body := range []int{cut.HeightMM + 100, 1800} {
					h0 := float64(s.elevation + body)
					for t := step; ; t += step {
						px, py := float64(x)+.5+ux*t, float64(y)+.5+uy*t
						h := h0 + t*float64(UnitMM)*rise
						if h > float64(maxTop) || px < 0 || py < 0 || px >= float64(b.w) || py >= float64(b.d) {
							break
						}
						at := Cell{int(px), int(py)}
						if at.X == x && at.Y == y {
							continue
						}
						for _, part := range walls[b.index(at)] {
							if float64(part.bottom) <= h && h < float64(part.top) {
								mark(part.id, at)
							}
						}
					}
				}
			}
		}
	}
	out := &NearCut{HeightMM: cut.HeightMM, Segments: []NearCutSegment{}}
	for _, o := range b.layout.Obstacles {
		cells := hit[o.ID]
		if len(cells) == 0 {
			continue
		}
		seg := NearCutSegment{Obstacle: o.ID}
		for _, at := range rectCells(o.Rect) {
			if cells[at] {
				seg.Cells = append(seg.Cells, at)
			}
		}
		out.Segments = append(out.Segments, seg)
	}
	return out
}

func (b *mdBuilder) surfaceRef(ref SurfaceRef, path string) (*mdSurface, *ValidationError) {
	if ref.Location != "" && ref.Location != b.layout.ID {
		return nil, issue("cross_location_connector", path+".location", "connectors that end in another location are reserved (cable capsules, outdoor maps) and not implemented", ref.Location)
	}
	s, ok := b.byID[ref.Surface]
	if !ok {
		return nil, issue("unknown_surface", path+".surface", "endpoint references an unknown surface", ref.Surface)
	}
	if !b.inside(ref.Cell) {
		return nil, issue("connector_out_of_bounds", path+".cell", "endpoint cell lies outside the plan")
	}
	return s, nil
}

func (b *mdBuilder) obstacles() *ValidationError {
	for i, o := range b.layout.Obstacles {
		path := fmt.Sprintf("layout.obstacles[%d]", i)
		if err := b.claim(o.ID, "obstacle", path); err != nil {
			return err
		}
		s, ok := b.byID[o.Surface]
		if !ok {
			return issue("unknown_surface", path+".surface", "obstacle references an unknown surface", o.Surface)
		}
		if strings.TrimSpace(o.Reason) == "" || !b.rectInside(o.Rect) || o.BottomMM < 0 || o.TopMM <= o.BottomMM || o.TopMM > 100000 {
			return issue("obstacle_definition", path, "obstacle needs a reason, a rectangle inside the plan and 0 <= bottom_mm < top_mm <= 100000 above its surface")
		}
		if o.Style != "wall" && o.Style != "glazing" && o.Style != "panel" {
			return issue("obstacle_definition", path+".style", "obstacle style is wall, glazing or panel")
		}
		if !o.BlocksMovement && !o.BlocksSight {
			return issue("obstacle_without_effect", path, "an obstacle must block movement, sight or both; decoration belongs to presentation")
		}
		if o.BlocksMovement && o.BottomMM >= 1800 {
			return issue("obstacle_definition", path, "a movement blocker must begin below the 1.8 m body height; a lintel or high window bar is a sight-only interval")
		}
		bottom, top := s.elevation+o.BottomMM, s.elevation+o.TopMM
		for _, at := range rectCells(o.Rect) {
			n := b.index(at)
			if !s.hasSlab[n] {
				return issue("unsupported_obstacle", path, fmt.Sprintf("obstacle cell (%d,%d) has no slab on surface %s", at.X, at.Y, s.id), o.ID)
			}
			if s == b.surfaces[0] && b.out != nil {
				for _, instance := range b.out.Instances {
					for _, c := range instance.Footprint {
						if c == at {
							return issue("physical_overlap", path, fmt.Sprintf("obstacle overlaps instance %s at (%d,%d)", instance.ID, at.X, at.Y), o.ID, instance.ID)
						}
					}
				}
			}
			for _, other := range b.surfaces {
				if other != s && other.hasSlab[n] && bottom < other.elevation && other.elevation-other.slab < top {
					return issue("obstacle_crosses_slab", path, fmt.Sprintf("obstacle %s passes through the slab of %s at (%d,%d); end it at the slab underside", o.ID, other.id, at.X, at.Y), o.ID, other.id)
				}
			}
			iv := mdInterval{bottom, top, o.ID}
			// Stacked parts of one wall (sill panel, glazing, lintel) share a
			// cell; physically intersecting volumes are rejected.
			for _, other := range b.walls[n] {
				if other.bottom < top && bottom < other.top {
					return issue("physical_overlap", path, fmt.Sprintf("obstacle %s intersects %s at (%d,%d)", o.ID, other.source, at.X, at.Y), o.ID, other.source)
				}
			}
			b.walls[n] = append(b.walls[n], iv)
			if o.BlocksMovement {
				if !s.blocked[n] {
					s.sightClear[n] = true
				}
				s.blocked[n] = true
				// Opaque at standing height if any movement part blocks sight.
				if o.BlocksSight {
					s.sightClear[n] = false
				}
			}
			if o.BlocksSight {
				b.sightSolid[n] = append(b.sightSolid[n], iv)
			}
		}
	}
	return nil
}

func (b *mdBuilder) columns() *ValidationError {
	for i, support := range b.layout.Supports {
		path := fmt.Sprintf("layout.supports[%d]", i)
		if err := b.claim(support.ID, "support", path); err != nil {
			return err
		}
		if strings.TrimSpace(support.Reason) == "" {
			return issue("support_definition", path+".reason", "every support keeps its authored purpose")
		}
		carried, ok := b.byID[support.Surface]
		if !ok || carried == b.surfaces[0] {
			return issue("support_definition", path+".surface", "a support carries one authored deck or roof", support.Surface)
		}
		switch support.Kind {
		case "column", "wall_bearing", "wall_console", "beam":
		case "hanger":
			return issue("unsupported_support_kind", path+".kind", "hangers/suspended structure are reserved and not implemented")
		default:
			return issue("unsupported_support_kind", path+".kind", "supported kinds are column, wall_bearing, wall_console and beam")
		}
		if support.Kind != "column" {
			continue
		}
		if support.At == nil || support.From != nil || support.To != nil || !b.inside(*support.At) {
			return issue("support_definition", path, "a column has one plan cell `at` inside the plan")
		}
		rests, ok := b.byID[support.RestsOn]
		if !ok || rests.elevation >= carried.elevation {
			return issue("support_definition", path+".rests_on", "a column stands on a lower authored surface", support.RestsOn)
		}
		at := *support.At
		n := b.index(at)
		if !rests.hasSlab[n] {
			return issue("unsupported_column", path, fmt.Sprintf("column base (%d,%d) has no slab on %s", at.X, at.Y, rests.id), support.ID)
		}
		if !carried.hasSlab[n] {
			return issue("unsupported_column", path, fmt.Sprintf("column head (%d,%d) is not under the slab of %s", at.X, at.Y, carried.id), support.ID)
		}
		for _, other := range b.surfaces {
			if other != rests && other != carried && other.hasSlab[n] && other.elevation > rests.elevation && other.elevation < carried.elevation {
				return issue("column_crosses_slab", path, fmt.Sprintf("column passes through the slab of %s; stand it on the nearest lower surface", other.id), support.ID, other.id)
			}
		}
		if rests.blocked[n] {
			return issue("physical_overlap", path, fmt.Sprintf("column at (%d,%d) overlaps another movement blocker on %s", at.X, at.Y, rests.id), support.ID)
		}
		rests.blocked[n] = true
		iv := mdInterval{rests.elevation, carried.elevation - carried.slab, support.ID}
		b.walls[n] = append(b.walls[n], iv)
		b.sightSolid[n] = append(b.sightSolid[n], iv)
		b.markSupport(carried, at)
		b.nav0().Supports = append(b.nav0().Supports, NavSupport{ID: support.ID, Kind: support.Kind, Surface: carried.id, RestsOn: rests.id, Cells: []Cell{at}, BottomMM: iv.bottom, TopMM: iv.top, Reason: support.Reason})
	}
	return nil
}

// physicalAt lists walls/columns/stair bodies plus every slab at plan n.
func (b *mdBuilder) physicalAt(n int) []mdInterval {
	out := append([]mdInterval{}, b.walls[n]...)
	for _, s := range b.surfaces {
		if s.hasSlab[n] && !(s.grounded && s == b.surfaces[0]) {
			out = append(out, mdInterval{s.elevation - s.slab, s.elevation, s.id})
		}
	}
	return out
}

func (b *mdBuilder) nav0() *Navigation {
	if b.nav == nil {
		b.nav = &Navigation{Schema: NavigationSchema, PlanWidth: b.w, PlanDepth: b.d, EyeMM: SightEyeMM, TargetMM: append([]int{}, SightTargetMM...), SightRange: SightRangeCells, BaseSurface: b.surfaces[0].id, Surfaces: []NavSurface{}, Connectors: []NavConnector{}, Links: []NavLink{}, Obstacles: []NavObstacle{}, Railings: []NavRailing{}, Supports: []NavSupport{}, CutawayGroups: []NavCutawayGroup{}, Solids: []SightSpan{}, Edges: []SightEdge{}}
	}
	return b.nav
}

func (b *mdBuilder) markSupport(s *mdSurface, at Cell) {
	if b.supportAt[s] == nil {
		b.supportAt[s] = map[int]bool{}
	}
	b.supportAt[s][b.index(at)] = true
}

func (b *mdBuilder) linePoints(support Support, path string) ([]Cell, *ValidationError) {
	if support.From == nil || support.To == nil || support.At != nil || !b.inside(*support.From) || !b.inside(*support.To) {
		return nil, issue("support_definition", path, support.Kind+" needs orthogonal from/to plan cells inside the plan")
	}
	from, to := *support.From, *support.To
	if from.X != to.X && from.Y != to.Y {
		return nil, issue("support_definition", path, support.Kind+" line must be orthogonal")
	}
	step := Cell{sign(to.X - from.X), sign(to.Y - from.Y)}
	cells := []Cell{from}
	for at := from; at != to; {
		at = Cell{at.X + step.X, at.Y + step.Y}
		cells = append(cells, at)
	}
	return cells, nil
}

func sign(v int) int {
	if v < 0 {
		return -1
	}
	if v > 0 {
		return 1
	}
	return 0
}

func (b *mdBuilder) wallsAndBeams() *ValidationError {
	// Walls first: beam ends may rest on them.
	for pass := 0; pass < 2; pass++ {
		for i, support := range b.layout.Supports {
			path := fmt.Sprintf("layout.supports[%d]", i)
			carried := b.byID[support.Surface]
			wall := support.Kind == "wall_bearing" || support.Kind == "wall_console"
			if pass == 0 && !wall || pass == 1 && support.Kind != "beam" {
				continue
			}
			cells, err := b.linePoints(support, path)
			if err != nil {
				return err
			}
			if support.RestsOn != "" {
				return issue("support_definition", path+".rests_on", "only columns name the surface they stand on; walls and beams are found from geometry")
			}
			underside := carried.elevation - carried.slab
			switch support.Kind {
			case "wall_bearing":
				for _, at := range cells {
					n := b.index(at)
					if !carried.hasSlab[n] {
						return issue("unsupported_wall_support", path, fmt.Sprintf("bearing cell (%d,%d) is not under the slab of %s", at.X, at.Y, carried.id), support.ID)
					}
					bears := false
					for _, iv := range b.walls[n] {
						bears = bears || iv.top == underside && iv.bottom < underside
					}
					if !bears {
						return issue("unsupported_wall_support", path, fmt.Sprintf("no wall at (%d,%d) reaches the slab underside %d mm of %s", at.X, at.Y, underside, carried.id), support.ID)
					}
					b.markSupport(carried, at)
				}
			case "wall_console":
				for _, at := range cells {
					n := b.index(at)
					if !carried.hasSlab[n] {
						return issue("unsupported_wall_support", path, fmt.Sprintf("console cell (%d,%d) is not a slab cell of %s", at.X, at.Y, carried.id), support.ID)
					}
					anchored := false
					for _, d := range [...]Cell{{0, -1}, {1, 0}, {0, 1}, {-1, 0}} {
						next := Cell{at.X + d.X, at.Y + d.Y}
						if !b.inside(next) {
							continue
						}
						// The wall continues at least 500 mm below and up to the deck.
						if intervalsCover(b.physicalAt(b.index(next)), underside-500, carried.elevation) {
							anchored = true
						}
					}
					if !anchored {
						return issue("unsupported_wall_support", path, fmt.Sprintf("console cell (%d,%d) has no adjacent wall continuous from 500 mm below the slab to the deck of %s", at.X, at.Y, carried.id), support.ID)
					}
					b.markSupport(carried, at)
				}
			case "beam":
				depth := support.DepthMM
				if depth == 0 {
					depth = DefaultBeamDepthMM
				}
				if depth < 150 || depth > 1500 {
					return issue("support_definition", path+".depth_mm", "beam depth must be 150..1500 mm")
				}
				if b.beams[carried] == nil {
					b.beams[carried] = map[int]int{}
				}
				// Point supports along the line: column heads, wall supports and
				// cells of previously declared beams of the same surface.
				var points []int
				for k, at := range cells {
					n := b.index(at)
					if !carried.hasSlab[n] {
						return issue("unsupported_beam", path, fmt.Sprintf("beam cell (%d,%d) is not under the slab of %s", at.X, at.Y, carried.id), support.ID)
					}
					if b.supportAt[carried][n] {
						points = append(points, k)
					}
				}
				if len(points) == 0 || points[0] != 0 || points[len(points)-1] != len(cells)-1 {
					return issue("unsupported_beam", path, "both beam ends must rest on a column, a wall support or an earlier beam of the same deck", support.ID)
				}
				for k := 1; k < len(points); k++ {
					if (points[k]-points[k-1])*UnitMM > MaxBeamSpanMM {
						return issue("beam_span", path, fmt.Sprintf("beam span between supports is %d mm; at most %d mm is accepted", (points[k]-points[k-1])*UnitMM, MaxBeamSpanMM), support.ID)
					}
				}
				for _, at := range cells {
					n := b.index(at)
					b.beams[carried][n] = underside - depth
					b.markSupport(carried, at)
					iv := mdInterval{underside - depth, underside, support.ID}
					b.sightSolid[n] = append(b.sightSolid[n], iv)
				}
				b.nav0().Supports = append(b.nav0().Supports, NavSupport{ID: support.ID, Kind: support.Kind, Surface: carried.id, Cells: cells, BottomMM: underside - depth, TopMM: underside, Reason: support.Reason})
				continue
			}
			b.nav0().Supports = append(b.nav0().Supports, NavSupport{ID: support.ID, Kind: support.Kind, Surface: carried.id, Cells: cells, BottomMM: underside, TopMM: carried.elevation, Reason: support.Reason})
		}
	}
	return nil
}

func (b *mdBuilder) supportCoverage() *ValidationError {
	reach := MaxSlabSpanMM / 2
	for _, s := range b.surfaces[1:] {
		points := b.supportAt[s]
		if len(points) == 0 {
			return issue("unsupported_slab", "layout.decks", fmt.Sprintf("deck %s declares no columns, wall supports or beams; floating slabs are rejected", s.id), s.id)
		}
		list := make([]Cell, 0, len(points))
		for n := range points {
			list = append(list, Cell{n % b.w, n / b.w})
		}
		for n, has := range s.hasSlab {
			if !has {
				continue
			}
			at := Cell{n % b.w, n / b.w}
			best := math.MaxFloat64
			for _, p := range list {
				dx, dy := float64(p.X-at.X), float64(p.Y-at.Y)
				if d := math.Sqrt(dx*dx+dy*dy) * UnitMM; d < best {
					best = d
				}
			}
			if best > float64(reach) {
				return issue("unsupported_slab", "layout.decks", fmt.Sprintf("slab cell (%d,%d) of %s is %.0f mm from the nearest declared support; slab spans are limited to %d mm", at.X, at.Y, s.id, best, MaxSlabSpanMM), s.id)
			}
		}
	}
	return nil
}

// floorBelow returns the highest walking/standing level under `above` at plan n:
// a slab top of another surface or a stair node. ok=false: nothing below.
func (b *mdBuilder) floorBelow(n, above int) (int, bool) {
	best, found := 0, false
	for _, s := range b.surfaces {
		if s.hasSlab[n] && s.elevation < above && (!found || s.elevation > best) {
			best, found = s.elevation, true
		}
	}
	if node, ok := b.nodeAt[n]; ok && node.HeightMM < above && (!found || node.HeightMM > best) {
		best, found = node.HeightMM, true
	}
	return best, found
}

func (b *mdBuilder) connectors() *ValidationError {
	for i, c := range b.layout.Connectors {
		path := fmt.Sprintf("layout.connectors[%d]", i)
		if err := b.claim(c.ID, "connector", path); err != nil {
			return err
		}
		switch c.Kind {
		case "stair":
		case "ladder", "ramp", "gangway":
			return issue("unsupported_connector_kind", path+".kind", c.Kind+" connectors are reserved (schema keeps kind/endpoints/width/clearance) and not implemented; only stair is supported")
		case "lift", "cable_capsule":
			return issue("transport_connector_reserved", path+".kind", c.Kind+" is a transport connector (boarding ports, timetable, capacity), not a walking edge; reserved and not implemented")
		default:
			return issue("unsupported_connector_kind", path+".kind", "unknown connector kind")
		}
		if strings.TrimSpace(c.Reason) == "" {
			return issue("connector_definition", path+".reason", "every connector keeps its authored purpose")
		}
		from, err := b.surfaceRef(c.From, path+".from")
		if err != nil {
			return err
		}
		to, err := b.surfaceRef(c.To, path+".to")
		if err != nil {
			return err
		}
		if !from.walkable || !to.walkable {
			return issue("connector_definition", path, "both stair endpoints must be walkable surfaces")
		}
		if to.elevation <= from.elevation {
			return issue("stair_direction", path, "a stair ascends from `from` to `to`; swap the endpoints")
		}
		if c.CutawayGroup != "" {
			if _, ok := b.groups[c.CutawayGroup]; !ok {
				return issue("deck_cutaway_group", path+".cutaway_group", "unknown cutaway group", c.CutawayGroup)
			}
		}
		step, ok := sideDelta(c.Direction)
		if !ok {
			return issue("connector_definition", path+".direction", "direction must be cardinal")
		}
		lat := lateralOf(c.Direction)
		if c.Construction != "" && c.Construction != "solid" && c.Construction != "steel_open" {
			return issue("connector_definition", path+".construction", "stair construction is solid or steel_open", c.Construction)
		}
		dx, dy := c.To.Cell.X-c.From.Cell.X, c.To.Cell.Y-c.From.Cell.Y
		run := dx*step.X + dy*step.Y
		if dx*lat.X+dy*lat.Y != 0 || run < 2 {
			return issue("connector_geometry", path, "landing cells must lie on one line along the direction with at least one stair cell between them")
		}
		length := run - 1
		if c.Width < MinStairWidthCells || c.Width > 16 {
			return issue("stair_width", path+".width", fmt.Sprintf("stair width must be %d..16 cells (at least 1.0 m)", MinStairWidthCells))
		}
		clearance := c.ClearanceMM
		if clearance == 0 {
			clearance = DefaultClearanceMM
		}
		if clearance < DefaultClearanceMM || clearance > 10000 {
			return issue("connector_definition", path+".clearance_mm", fmt.Sprintf("stair headroom clearance must be at least %d mm", DefaultClearanceMM))
		}
		if len(c.Flights) < 1 || len(c.Flights) > 8 || len(c.Landings) != len(c.Flights)-1 {
			return issue("stair_flights", path, "a stair has 1..8 flights and exactly one landing between consecutive flights")
		}
		risers, cells := 0, 0
		for _, f := range c.Flights {
			if f.Risers < 3 || f.Length < 1 {
				return issue("stair_flights", path+".flights", "each flight needs at least 3 risers and a positive length")
			}
			risers += f.Risers
			cells += f.Length
		}
		for _, l := range c.Landings {
			if l.Length < c.Width || l.Length < 2 {
				return issue("stair_landing", path+".landings", "an intermediate landing is at least as long as the stair is wide and at least 1.0 m")
			}
			cells += l.Length
		}
		if cells != length {
			return issue("connector_geometry", path, fmt.Sprintf("flights and landings cover %d cells but the landings are %d cells apart", cells, length))
		}
		rise := to.elevation - from.elevation
		riser := float64(rise) / float64(risers)
		if riser < MinRiserMM || riser > MaxRiserMM {
			return issue("stair_riser", path, fmt.Sprintf("riser height %.1f mm (%d mm over %d risers) is outside %d..%d mm", riser, rise, risers, MinRiserMM, MaxRiserMM))
		}
		nc := NavConnector{ID: c.ID, Kind: c.Kind, FromSurface: from.id, ToSurface: to.id, From: c.From.Cell, To: c.To.Cell, Direction: c.Direction, Width: c.Width, Length: length, ClearanceMM: clearance, BottomMM: from.elevation, TopMM: to.elevation, Flights: []NavFlight{}, Landings: []NavLanding{}, Balustrades: c.Balustrades, CutawayGroup: c.CutawayGroup, Construction: c.Construction, Footprint: []Cell{}, Nodes: []NavNode{}, Reason: c.Reason}
		heights := make([]int, length)
		offset, base, used := 0, from.elevation, 0
		for k, f := range c.Flights {
			going := float64(f.Length*UnitMM) / float64(f.Risers-1)
			if going < MinGoingMM || going > MaxGoingMM {
				return issue("stair_going", path, fmt.Sprintf("flight %d tread going %.0f mm is outside %d..%d mm", k, going, MinGoingMM, MaxGoingMM))
			}
			if rule := 2*riser + going; rule < MinStepRuleMM || rule > MaxStepRuleMM {
				return issue("stair_step_rule", path, fmt.Sprintf("flight %d: 2R+G = %.0f mm is outside the comfortable %d..%d mm range", k, rule, MinStepRuleMM, MaxStepRuleMM))
			}
			if slope := math.Atan(riser/going) * 180 / math.Pi; slope > MaxStairSlopeDegrees {
				return issue("stair_slope", path, fmt.Sprintf("flight %d pitch %.1f° exceeds %.0f°", k, slope, MaxStairSlopeDegrees))
			}
			nc.Flights = append(nc.Flights, NavFlight{Risers: f.Risers, Length: f.Length, StartStep: offset, BaseMM: base, RiserMM: riser, GoingMM: going})
			for j := 0; j < f.Length; j++ {
				centre := (float64(j) + .5) * UnitMM
				tread := int(centre / going)
				if tread > f.Risers-2 {
					tread = f.Risers - 2
				}
				heights[offset+j] = base + int(math.Round(float64(tread+1)*riser))
			}
			used += f.Risers
			offset += f.Length
			base = from.elevation + int(math.Round(float64(used)*riser))
			if k < len(c.Landings) {
				l := c.Landings[k]
				nc.Landings = append(nc.Landings, NavLanding{Length: l.Length, StartStep: offset, HeightMM: base})
				for j := 0; j < l.Length; j++ {
					heights[offset+j] = base
				}
				offset += l.Length
			}
		}
		guard := c.Balustrades.HeightMM
		if (c.Balustrades.Left || c.Balustrades.Right) && (guard < 900 || guard > MaxGuardHeightMM) {
			return issue("stair_balustrade", path+".balustrades.height_mm", fmt.Sprintf("stair balustrade height must be 900..%d mm", MaxGuardHeightMM))
		}
		for lane := 0; lane < c.Width; lane++ {
			footCell := Cell{c.From.Cell.X + lat.X*lane, c.From.Cell.Y + lat.Y*lane}
			headCell := Cell{c.To.Cell.X + lat.X*lane, c.To.Cell.Y + lat.Y*lane}
			for _, landing := range []struct {
				s  *mdSurface
				at Cell
			}{{from, footCell}, {to, headCell}} {
				if !b.inside(landing.at) || !landing.s.hasSlab[b.index(landing.at)] || landing.s.blocked[b.index(landing.at)] {
					return issue("connector_landing", path, fmt.Sprintf("landing (%d,%d) on %s is not free walkable floor", landing.at.X, landing.at.Y, landing.s.id), c.ID)
				}
				key := mdKey{landing.s.id, landing.at}
				if other, taken := b.landingOf[key]; taken {
					return issue("connector_landing", path, fmt.Sprintf("landing (%d,%d) on %s is shared with %s", landing.at.X, landing.at.Y, landing.s.id, other), c.ID, other)
				}
				b.landingOf[key] = c.ID
			}
			b.headEdges[mdEdgeKey{to.id, headCell, oppositeSide(c.Direction)}] = c.ID
			for s := 0; s < length; s++ {
				at := Cell{footCell.X + step.X*(s+1), footCell.Y + step.Y*(s+1)}
				if !b.inside(at) {
					return issue("connector_out_of_bounds", path, "stair run leaves the plan")
				}
				n := b.index(at)
				if !from.hasSlab[n] {
					return issue("unsupported_stair", path, fmt.Sprintf("stair cell (%d,%d) does not rest on the slab of %s", at.X, at.Y, from.id), c.ID)
				}
				if from.blocked[n] {
					return issue("stair_overlap", path, fmt.Sprintf("stair cell (%d,%d) overlaps an obstacle, column or another stair on %s", at.X, at.Y, from.id), c.ID)
				}
				if owner, taken := b.nodeOwner[n]; taken {
					return issue("stair_overlap", path, fmt.Sprintf("stair cell (%d,%d) overlaps %s", at.X, at.Y, owner), c.ID, owner)
				}
				for key, owner := range b.landingOf {
					if key.at == at && key.surface == from.id {
						return issue("stair_overlap", path, fmt.Sprintf("stair cell (%d,%d) covers a landing of %s", at.X, at.Y, owner), c.ID, owner)
					}
				}
				if from == b.surfaces[0] && b.out != nil {
					for _, instance := range b.out.Instances {
						for _, cell := range instance.Footprint {
							if cell == at {
								return issue("stair_overlap", path, fmt.Sprintf("stair cell (%d,%d) overlaps instance %s", at.X, at.Y, instance.ID), c.ID, instance.ID)
							}
						}
					}
				}
				from.blocked[n] = true
				node := NavNode{Plan: at, Lane: lane, Step: s, HeightMM: heights[s]}
				b.nodeAt[n] = node
				b.nodeOwner[n] = c.ID
				iv := mdInterval{from.elevation, heights[s], c.ID}
				b.walls[n] = append(b.walls[n], iv)
				if c.Construction != "steel_open" {
					b.sightSolid[n] = append(b.sightSolid[n], iv)
				} else {
					// Open steel stair: sight passes under the treads and the landing; only the two channel
					// stringers on the outer sides of the run stop it (a band under the tread line).
					for _, side := range []struct {
						lane int
						s    Side
					}{{0, sideOf(Cell{-lat.X, -lat.Y})}, {c.Width - 1, sideOf(lat)}} {
						if lane == side.lane {
							b.nav0().Edges = append(b.nav0().Edges, SightEdge{At: at, Side: side.s, BottomMM: max(from.elevation, heights[s]-StringerDepthMM), TopMM: heights[s], Source: c.ID + ".stringer"})
						}
					}
				}
				nc.Footprint = append(nc.Footprint, at)
				nc.Nodes = append(nc.Nodes, node)
			}
		}
		// Open stair sides need a balustrade unless a wall rises past the guard.
		for _, side := range []struct {
			name  string
			lane  int
			delta int
			on    bool
		}{{"left", 0, -1, c.Balustrades.Left}, {"right", c.Width - 1, 1, c.Balustrades.Right}} {
			if side.on {
				continue
			}
			for s := 0; s < length; s++ {
				at := Cell{c.From.Cell.X + lat.X*side.lane + step.X*(s+1) + lat.X*side.delta, c.From.Cell.Y + lat.Y*side.lane + step.Y*(s+1) + lat.Y*side.delta}
				h := heights[s]
				below := math.MinInt / 2
				if b.inside(at) {
					if v, found := b.floorBelow(b.index(at), h+1); found {
						below = v
					}
				}
				if h-below < DropRailingThresholdMM {
					continue
				}
				if b.inside(at) && intervalsCover(b.physicalAt(b.index(at)), h, h+MinGuardHeightMM) {
					continue
				}
				return issue("stair_unprotected_side", path, fmt.Sprintf("open %s side of the stair at step %d drops %d mm with no balustrade or wall", side.name, s, h-below), c.ID)
			}
		}
		b.nav0().Connectors = append(b.nav0().Connectors, nc)
	}
	return nil
}

func (b *mdBuilder) railings() *ValidationError {
	for i, r := range b.layout.Railings {
		path := fmt.Sprintf("layout.railings[%d]", i)
		if err := b.claim(r.ID, "railing", path); err != nil {
			return err
		}
		s, ok := b.byID[r.Surface]
		if !ok || s == b.surfaces[0] || !s.walkable {
			return issue("railing_definition", path+".surface", "railings protect the edges of a walkable deck; the base uses drops", r.Surface)
		}
		if strings.TrimSpace(r.Reason) == "" || r.HeightMM < MinGuardHeightMM || r.HeightMM > MaxGuardHeightMM {
			return issue("railing_definition", path, fmt.Sprintf("railing needs a reason and a guard height of %d..%d mm", MinGuardHeightMM, MaxGuardHeightMM))
		}
		step, valid := sideDelta(r.Side)
		if !valid || !b.inside(r.From) || !b.inside(r.To) {
			return issue("railing_definition", path, "railing needs a cardinal side and plan cells inside the plan")
		}
		alongX := r.Side == North || r.Side == South
		if alongX && r.From.Y != r.To.Y || !alongX && r.From.X != r.To.X {
			return issue("railing_definition", path, "a railing runs along its edge: constant Y for north/south, constant X for east/west")
		}
		dir := Cell{sign(r.To.X - r.From.X), sign(r.To.Y - r.From.Y)}
		for at := r.From; ; at = (Cell{at.X + dir.X, at.Y + dir.Y}) {
			n := b.index(at)
			next := Cell{at.X + step.X, at.Y + step.Y}
			if !s.hasSlab[n] || s.blocked[n] || b.inside(next) && s.hasSlab[b.index(next)] {
				return issue("railing_not_at_edge", path, fmt.Sprintf("railing cell (%d,%d) side %s is not an edge of walkable floor on %s", at.X, at.Y, r.Side, s.id), r.ID)
			}
			if owner, head := b.headEdges[mdEdgeKey{s.id, at, r.Side}]; head {
				return issue("railing_across_connector", path, fmt.Sprintf("railing at (%d,%d) would close the head of %s", at.X, at.Y, owner), r.ID, owner)
			}
			key := mdEdgeKey{s.id, at, r.Side}
			if _, dup := b.headEdges[mdEdgeKey{"railing:" + s.id, at, r.Side}]; dup {
				return issue("duplicate_drop_edge", path, "this edge already has a railing", r.ID)
			}
			b.headEdges[mdEdgeKey{"railing:" + s.id, at, r.Side}] = r.ID
			_ = key
			if r.BlocksSight {
				b.nav0().Edges = append(b.nav0().Edges, SightEdge{At: at, Side: r.Side, BottomMM: s.elevation, TopMM: s.elevation + r.HeightMM, Source: r.ID})
			}
			if at == r.To {
				break
			}
		}
		b.nav0().Railings = append(b.nav0().Railings, NavRailing{ID: r.ID, Surface: s.id, From: r.From, To: r.To, Side: r.Side, BaseMM: s.elevation, HeightMM: r.HeightMM, BlocksSight: r.BlocksSight, Reason: r.Reason})
	}
	// Stair balustrades declared by connectors are sight edges, too.
	for _, c := range b.nav0().Connectors {
		if !c.Balustrades.BlocksSight {
			continue
		}
		lat := lateralOf(c.Direction)
		for _, node := range c.Nodes {
			for _, side := range []struct {
				on   bool
				lane int
				s    Side
			}{{c.Balustrades.Left, 0, sideOf(Cell{-lat.X, -lat.Y})}, {c.Balustrades.Right, c.Width - 1, sideOf(lat)}} {
				if side.on && node.Lane == side.lane {
					b.nav.Edges = append(b.nav.Edges, SightEdge{At: node.Plan, Side: side.s, BottomMM: node.HeightMM, TopMM: node.HeightMM + c.Balustrades.HeightMM, Source: c.ID})
				}
			}
		}
	}
	return nil
}

func sideOf(d Cell) Side {
	switch d {
	case Cell{0, -1}:
		return North
	case Cell{1, 0}:
		return East
	case Cell{0, 1}:
		return South
	}
	return West
}

func (b *mdBuilder) edgeProtection() *ValidationError {
	for _, s := range b.surfaces[1:] {
		if !s.walkable {
			continue
		}
		for n, has := range s.hasSlab {
			if !has || s.blocked[n] {
				continue
			}
			at := Cell{n % b.w, n / b.w}
			for _, side := range [...]Side{North, East, South, West} {
				d, _ := sideDelta(side)
				next := Cell{at.X + d.X, at.Y + d.Y}
				if b.inside(next) && s.hasSlab[b.index(next)] {
					continue
				}
				if _, head := b.headEdges[mdEdgeKey{s.id, at, side}]; head {
					continue
				}
				if _, railed := b.headEdges[mdEdgeKey{"railing:" + s.id, at, side}]; railed {
					continue
				}
				drop := math.MaxInt / 2
				if b.inside(next) {
					nn := b.index(next)
					if v, found := b.floorBelow(nn, s.elevation); found {
						drop = s.elevation - v
					}
					if intervalsCover(b.physicalAt(nn), s.elevation, s.elevation+MinGuardHeightMM) {
						continue
					}
				}
				if drop < DropRailingThresholdMM {
					continue
				}
				dropText := "an unknown depth"
				if drop < math.MaxInt/4 {
					dropText = fmt.Sprintf("%d mm", drop)
				}
				return issue("unprotected_deck_edge", "layout.railings", fmt.Sprintf("%s edge of (%d,%d) on %s drops %s with no railing, wall or stair head", side, at.X, at.Y, s.id, dropText), s.id)
			}
		}
	}
	return nil
}

func (b *mdBuilder) headroom() *ValidationError {
	check := func(owner string, at Cell, h, need int) *ValidationError {
		n := b.index(at)
		for _, s := range b.surfaces {
			if !s.hasSlab[n] || s.elevation <= h {
				continue
			}
			under := s.elevation - s.slab
			if bottom, beam := b.beams[s][n]; beam && bottom < under {
				under = bottom
			}
			if under-h < need {
				return issue("insufficient_headroom", "layout", fmt.Sprintf("%s at (%d,%d), level %d mm, has %d mm clear height under %s; at least %d mm is required", owner, at.X, at.Y, h, under-h, s.id, need), owner, s.id)
			}
		}
		return nil
	}
	for _, s := range b.surfaces {
		if !s.walkable {
			continue
		}
		for n, has := range s.hasSlab {
			if has && !s.blocked[n] {
				if err := check(s.id, Cell{n % b.w, n / b.w}, s.elevation, DefaultClearanceMM); err != nil {
					return err
				}
			}
		}
	}
	for _, c := range b.nav0().Connectors {
		for _, node := range c.Nodes {
			if err := check(c.ID, node.Plan, node.HeightMM, c.ClearanceMM); err != nil {
				return err
			}
		}
	}
	return nil
}

// navigation assembles bands, stair blocks, links and the full terrain, then
// verifies that every walkable navigation cell is reachable from the entry.
func (b *mdBuilder) navigation() *ValidationError {
	nav := b.nav0()
	row := 0
	for _, s := range b.surfaces {
		s.navRow = -1
		if s.walkable {
			s.navRow = row
			row += b.d + 1
		}
	}
	// Shelf-pack stair blocks below the surface bands, one void column apart.
	x, shelf, shelfHeight := 0, row, 0
	for i := range nav.Connectors {
		c := &nav.Connectors[i]
		if c.Width > b.w {
			return issue("connector_geometry", "layout.connectors", "stair is wider than the plan", c.ID)
		}
		if x+c.Width > b.w {
			x, shelf, shelfHeight = 0, shelf+shelfHeight+1, 0
		}
		c.NavOrigin = Cell{x, shelf}
		for j := range c.Nodes {
			c.Nodes[j].Nav = Cell{x + c.Nodes[j].Lane, shelf + c.Nodes[j].Step}
		}
		x += c.Width + 1
		if c.Length > shelfHeight {
			shelfHeight = c.Length
		}
	}
	depth := shelf + shelfHeight
	if len(nav.Connectors) == 0 {
		depth = row - 1
	}
	if b.w*depth > RuntimeCellBudget*4 {
		return issue("navigation_budget", "layout", "navigation plane exceeds the supported cell budget")
	}
	nav.NavWidth, nav.NavDepth = b.w, depth
	grid := make([][]byte, depth)
	for y := range grid {
		grid[y] = []byte(strings.Repeat(" ", b.w))
	}
	for _, s := range b.surfaces {
		if s.navRow < 0 {
			continue
		}
		for n, has := range s.hasSlab {
			at := Cell{n % b.w, n / b.w}
			glyph := byte(' ')
			if has {
				glyph = '.'
				if s.blocked[n] {
					glyph = '#'
					if s.sightClear[n] {
						glyph = '+'
					}
				}
			}
			grid[s.navRow+at.Y][at.X] = glyph
		}
	}
	for _, c := range nav.Connectors {
		for _, node := range c.Nodes {
			grid[node.Nav.Y][node.Nav.X] = '.'
		}
		from, to := b.byID[c.FromSurface], b.byID[c.ToSurface]
		step, _ := sideDelta(c.Direction)
		lat := lateralOf(c.Direction)
		for lane := 0; lane < c.Width; lane++ {
			foot := Cell{c.From.X + lat.X*lane, from.navRow + c.From.Y + lat.Y*lane}
			head := Cell{c.To.X + lat.X*lane, to.navRow + c.To.Y + lat.Y*lane}
			first := Cell{c.NavOrigin.X + lane, c.NavOrigin.Y}
			last := Cell{c.NavOrigin.X + lane, c.NavOrigin.Y + c.Length - 1}
			nav.Links = append(nav.Links, NavLink{A: foot, B: first, Connector: c.ID}, NavLink{A: last, B: head, Connector: c.ID})
		}
		_ = step
	}
	terrain := make([]string, depth)
	for y := range grid {
		terrain[y] = string(grid[y])
	}
	// Reachability over the whole navigation plane, including links.
	walk := func(c Cell) bool { return c.X >= 0 && c.Y >= 0 && c.X < b.w && c.Y < depth && grid[c.Y][c.X] == '.' }
	links := map[Cell][]Cell{}
	for _, l := range nav.Links {
		links[l.A] = append(links[l.A], l.B)
		links[l.B] = append(links[l.B], l.A)
	}
	reached := map[Cell]bool{b.layout.Entries[0]: true}
	queue := []Cell{b.layout.Entries[0]}
	for head := 0; head < len(queue); head++ {
		at := queue[head]
		next := []Cell{{at.X, at.Y - 1}, {at.X + 1, at.Y}, {at.X, at.Y + 1}, {at.X - 1, at.Y}}
		next = append(next, links[at]...)
		for _, c := range next {
			if walk(c) && !reached[c] {
				reached[c] = true
				queue = append(queue, c)
			}
		}
	}
	for y := 0; y < depth; y++ {
		for x := 0; x < b.w; x++ {
			c := Cell{x, y}
			if grid[y][x] == '.' && !reached[c] {
				return issue("surface_unreachable", "layout", fmt.Sprintf("walkable %s cannot be reached from the entry", b.describeNav(c)), b.surfaceAtNav(c))
			}
		}
	}
	for _, s := range b.surfaces {
		ns := NavSurface{ID: s.id, Kind: s.kind, Geometry: "flat", Index: s.index, ElevationMM: s.elevation, SlabMM: s.slab, Walkable: s.walkable, Grounded: s.grounded, NavRow: s.navRow, CutawayGroup: s.group, Slab: cellsToRects(s.hasSlab, b.w), Reason: s.reason}
		nav.Surfaces = append(nav.Surfaces, ns)
	}
	for _, o := range b.layout.Obstacles {
		s := b.byID[o.Surface]
		nav.Obstacles = append(nav.Obstacles, NavObstacle{ID: o.ID, Surface: o.Surface, Style: o.Style, Rect: o.Rect, BottomMM: s.elevation + o.BottomMM, TopMM: s.elevation + o.TopMM, BlocksMovement: o.BlocksMovement, BlocksSight: o.BlocksSight, Reason: o.Reason})
	}
	sort.Slice(nav.Obstacles, func(i, j int) bool { return nav.Obstacles[i].ID < nav.Obstacles[j].ID })
	sort.Slice(nav.Supports, func(i, j int) bool { return nav.Supports[i].ID < nav.Supports[j].ID })
	sort.Slice(nav.Railings, func(i, j int) bool { return nav.Railings[i].ID < nav.Railings[j].ID })
	// Sight solids: group equal intervals into rectangles per source.
	type spanKey struct {
		bottom, top int
		source      string
	}
	cellsBySpan := map[spanKey][]bool{}
	for n, list := range b.sightSolid {
		for _, iv := range list {
			k := spanKey(iv)
			if cellsBySpan[k] == nil {
				cellsBySpan[k] = make([]bool, b.w*b.d)
			}
			cellsBySpan[k][n] = true
		}
	}
	keys := make([]spanKey, 0, len(cellsBySpan))
	for k := range cellsBySpan {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, c := keys[i], keys[j]
		if a.source != c.source {
			return a.source < c.source
		}
		if a.bottom != c.bottom {
			return a.bottom < c.bottom
		}
		return a.top < c.top
	})
	for _, k := range keys {
		for _, r := range cellsToRects(cellsBySpan[k], b.w) {
			nav.Solids = append(nav.Solids, SightSpan{Rect: r, BottomMM: k.bottom, TopMM: k.top, Source: k.source})
		}
	}
	sort.Slice(nav.Edges, func(i, j int) bool {
		a, c := nav.Edges[i], nav.Edges[j]
		if a.Source != c.Source {
			return a.Source < c.Source
		}
		if a.At != c.At {
			return lessCell(a.At, c.At)
		}
		return a.Side < c.Side
	})
	// Cutaway membership and building volumes.
	groupIDs := make([]string, 0, len(b.groups))
	for id := range b.groups {
		groupIDs = append(groupIDs, id)
	}
	sort.Strings(groupIDs)
	for _, id := range groupIDs {
		g := b.groups[id]
		ng := NavCutawayGroup{ID: g.ID, Policy: g.Policy, Surfaces: []string{}, Connectors: []string{}, Interior: append([]Rect{}, g.Interior...), Reason: g.Reason}
		top, bottom := math.MinInt, math.MaxInt
		for _, s := range b.surfaces {
			if s.group == id {
				ng.Surfaces = append(ng.Surfaces, s.id)
				if s.elevation > top {
					top = s.elevation
				}
			}
		}
		for _, c := range nav.Connectors {
			if c.CutawayGroup == id {
				ng.Connectors = append(ng.Connectors, c.ID)
			}
		}
		for _, instance := range b.layout.Instances {
			if instance.CutawayGroup == id {
				ng.Instances = append(ng.Instances, instance.ID)
				if h := b.layout.Elevation*UnitMM + b.instanceHeightMM(instance.ID); h > top {
					top = h
				}
			}
		}
		if len(ng.Surfaces) == 0 && len(ng.Instances) == 0 {
			return issue("cutaway_group", "layout.cutaway_groups", "cutaway group has no member surface", id)
		}
		for _, r := range g.Interior {
			for _, at := range rectCells(r) {
				for _, s := range b.surfaces {
					if s.walkable && s.hasSlab[b.index(at)] && s.elevation < bottom && s.elevation < top {
						bottom = s.elevation
					}
				}
			}
		}
		if g.Policy == "building" && bottom == math.MaxInt {
			return issue("cutaway_group", "layout.cutaway_groups", "building interior contains no walkable floor below its top surface", id)
		}
		if bottom == math.MaxInt {
			bottom = top
		}
		ng.BottomMM, ng.TopMM = bottom, top
		nav.CutawayGroups = append(nav.CutawayGroups, ng)
	}
	b.out.Terrain = terrain
	b.out.Navigation = nav
	return nil
}

func (b *mdBuilder) instanceHeightMM(id string) int {
	for _, instance := range b.out.Instances {
		if instance.ID == id {
			return instance.Height * UnitMM
		}
	}
	return 0
}

func (b *mdBuilder) surfaceAtNav(c Cell) string {
	for _, s := range b.surfaces {
		if s.navRow >= 0 && c.Y >= s.navRow && c.Y < s.navRow+b.d {
			return s.id
		}
	}
	for _, conn := range b.nav0().Connectors {
		for _, node := range conn.Nodes {
			if node.Nav == c {
				return conn.ID
			}
		}
	}
	return ""
}

func (b *mdBuilder) describeNav(c Cell) string {
	for _, s := range b.surfaces {
		if s.navRow >= 0 && c.Y >= s.navRow && c.Y < s.navRow+b.d {
			return fmt.Sprintf("cell (%d,%d) of %s", c.X, c.Y-s.navRow, s.id)
		}
	}
	for _, conn := range b.nav0().Connectors {
		for _, node := range conn.Nodes {
			if node.Nav == c {
				return fmt.Sprintf("step %d lane %d of %s", node.Step, node.Lane, conn.ID)
			}
		}
	}
	return fmt.Sprintf("navigation cell (%d,%d)", c.X, c.Y)
}

// NavCellOf maps a surface plan cell to its navigation cell.
func (n *Navigation) NavCellOf(surface string, at Cell) (Cell, bool) {
	for _, s := range n.Surfaces {
		if s.ID == surface && s.NavRow >= 0 && at.X >= 0 && at.Y >= 0 && at.X < n.PlanWidth && at.Y < n.PlanDepth {
			return Cell{at.X, s.NavRow + at.Y}, true
		}
	}
	return Cell{}, false
}

// compileResidents validates authored test-stand people against the final
// navigation terrain. Planar layouts address the base surface only.
func compileResidents(layout Layout, out *CompiledLevel) *ValidationError {
	if len(layout.Residents) == 0 {
		return nil
	}
	if len(layout.Residents) > 64 {
		return issue("resident_budget", "layout.residents", "an SDK test stand supports at most 64 authored residents")
	}
	baseID := layout.BaseSurfaceID
	if baseID == "" {
		baseID = "base"
	}
	navCell := func(surface string, at Cell) (Cell, bool) {
		if out.Navigation != nil {
			if surface == "" {
				surface = out.Navigation.BaseSurface
			}
			return out.Navigation.NavCellOf(surface, at)
		}
		if surface != "" && surface != baseID {
			return Cell{}, false
		}
		return at, at.X >= 0 && at.Y >= 0 && at.X < out.Width && at.Y < out.Depth
	}
	walk := func(c Cell) bool {
		return c.Y >= 0 && c.Y < len(out.Terrain) && c.X >= 0 && c.X < len(out.Terrain[c.Y]) && out.Terrain[c.Y][c.X] == '.'
	}
	seen := map[string]bool{}
	starts := map[Cell]string{}
	for _, entry := range layout.Entries[:1] {
		starts[entry] = "spawn"
	}
	for i, r := range layout.Residents {
		path := fmt.Sprintf("layout.residents[%d]", i)
		if !stableID.MatchString(r.ID) || seen[r.ID] || r.ID == "char.hero" || strings.TrimSpace(r.Name) == "" || strings.TrimSpace(r.Reason) == "" {
			return issue("resident_definition", path, "resident needs a unique stable ID (not char.hero), a name and a reason")
		}
		seen[r.ID] = true
		if len(r.Steps) < 1 || len(r.Steps) > 1440 {
			return issue("resident_definition", path+".steps", "resident routine needs 1..1440 steps")
		}
		cr := CompiledResident{ID: r.ID, Name: r.Name, Reason: r.Reason}
		for j, step := range r.Steps {
			sp := fmt.Sprintf("%s.steps[%d]", path, j)
			if step.AtMinute < 0 || step.AtMinute >= 1440 || j > 0 && r.Steps[j-1].AtMinute >= step.AtMinute || strings.TrimSpace(step.Activity) == "" || strings.TrimSpace(step.Label) == "" || step.MinSeconds < 0 {
				return issue("resident_definition", sp, "steps need increasing at_minute in 0..1439, an activity and a label")
			}
			nav, ok := navCell(step.Surface, step.At)
			if !ok || !walk(nav) {
				return issue("resident_post_blocked", sp, fmt.Sprintf("resident post (%d,%d) on %q is not free walkable floor", step.At.X, step.At.Y, step.Surface), r.ID)
			}
			surface := step.Surface
			if surface == "" {
				surface = baseID
			}
			cr.Steps = append(cr.Steps, CompiledResidentStep{AtMinute: step.AtMinute, Surface: surface, At: step.At, Nav: nav, Activity: step.Activity, Label: step.Label, MinSeconds: step.MinSeconds})
		}
		// The Life clock starts at 00:00: the initial step is the last one due.
		start := cr.Steps[len(cr.Steps)-1]
		for _, s := range cr.Steps {
			if s.AtMinute == 0 {
				start = s
			}
		}
		if other, taken := starts[start.Nav]; taken {
			return issue("resident_overlap", path, "two residents (or a resident and the spawn) start on one cell", r.ID, other)
		}
		starts[start.Nav] = r.ID
		out.Residents = append(out.Residents, cr)
	}
	sort.Slice(out.Residents, func(i, j int) bool { return out.Residents[i].ID < out.Residents[j].ID })
	return nil
}

// structuralNavigation drops purpose prose, keeping geometry and identity.
func structuralNavigation(n *Navigation) *Navigation {
	if n == nil {
		return nil
	}
	b, _ := json.Marshal(n)
	var c Navigation
	_ = json.Unmarshal(b, &c)
	for i := range c.Surfaces {
		c.Surfaces[i].Reason = ""
	}
	for i := range c.Connectors {
		c.Connectors[i].Reason = ""
	}
	for i := range c.Obstacles {
		c.Obstacles[i].Reason = ""
	}
	for i := range c.Railings {
		c.Railings[i].Reason = ""
	}
	for i := range c.Supports {
		c.Supports[i].Reason = ""
	}
	for i := range c.CutawayGroups {
		c.CutawayGroups[i].Reason = ""
	}
	return &c
}
