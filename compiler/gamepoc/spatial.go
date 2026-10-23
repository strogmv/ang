package gamepoc

import (
	"fmt"
	hashfnv "hash/fnv"
	"strconv"
	"strings"

	"cuelang.org/go/cue"
)

// SpatialCatalog is the shared authored geometry and visual binding catalog.
// Terrain remains authoritative: styles and decorations never create collision.
type SpatialCatalog struct {
	Seed             int64           `json:"seed"`
	ModuleSnapMeters int             `json:"module_snap_m"`
	Modules          []SpatialModule `json:"modules"`
	Styles           []SpatialStyle  `json:"styles"`
	Levels           []SpatialLevel  `json:"levels"`
}

// Modules describe construction interfaces, not simulated upper floors. A
// definition_only module must never create collision or an actor destination.
type SpatialModule struct {
	ID              string                   `json:"id"`
	Kind            string                   `json:"kind"`
	Footprint       SpatialFootprint         `json:"footprint"`
	HeightMM        int                      `json:"height_mm"`
	ElevationMM     int                      `json:"elevation_mm"`
	SnapMeters      int                      `json:"snap_m"`
	Level           int                      `json:"level"`
	Ports           []SpatialModulePort      `json:"ports"`
	Clearances      []SpatialModuleClearance `json:"clearances"`
	RiseMM          int                      `json:"rise_mm"`
	Placement       string                   `json:"placement"`
	LandingModuleID string                   `json:"landing_module_id,omitempty"`
}

type SpatialFootprint struct {
	W int `json:"w"`
	H int `json:"h"`
}

type SpatialModulePort struct {
	ID   string `json:"id"`
	At   Point  `json:"at"`
	Side string `json:"side"`
	Kind string `json:"kind"`
}

type SpatialModuleClearance struct {
	ID          string `json:"id"`
	Side        string `json:"side"`
	DepthMeters int    `json:"depth_m"`
	Purpose     string `json:"purpose"`
}

type SpatialPlacedClearance struct {
	ID   string      `json:"id"`
	Rect SpatialRect `json:"rect"`
}

type SpatialStyle struct {
	ID          string `json:"id"`
	AssetStyle  string `json:"asset_style"`
	FloorColor  string `json:"floor_color"`
	AccentColor string `json:"accent_color"`
	Variants    int    `json:"variants"`
}

type SpatialRect struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type SpatialLevel struct {
	Lighting         *SpatialLighting     `json:"lighting,omitempty"`
	Lights           []SpatialLight       `json:"lights,omitempty"`
	Routes           []SpatialRoute       `json:"routes,omitempty"`
	ID               string               `json:"id"`
	Width            int                  `json:"width"`
	Height           int                  `json:"height"`
	CellMeters       float64              `json:"cell_meters"`
	CoverAllFloor    bool                 `json:"cover_all_floor"`
	Terrain          []string             `json:"terrain"`
	Zones            []SpatialZone        `json:"zones"`
	Corridors        []SpatialCorridor    `json:"corridors"`
	Doors            []SpatialDoor        `json:"doors"`
	Exits            []SpatialExit        `json:"exits"`
	Props            []SpatialProp        `json:"props"`
	Decorations      []SpatialDecoration  `json:"decorations"`
	InteractionSpots []SpatialInteraction `json:"interaction_spots"`
}

type SpatialZone struct {
	ID      string      `json:"id"`
	Kind    string      `json:"kind"`
	Rect    SpatialRect `json:"rect"`
	StyleID string      `json:"style_id"`
	Purpose string      `json:"purpose"`
	// ResolvedVariant is stable under catalog reordering or unrelated additions.
	ResolvedVariant int `json:"resolved_variant"`
}

type SpatialCorridor struct {
	ID          string      `json:"id"`
	Rect        SpatialRect `json:"rect"`
	WidthMeters float64     `json:"width_m"`
}

type SpatialDoor struct {
	ID      string `json:"id"`
	At      Point  `json:"at"`
	Purpose string `json:"purpose"`
}

type SpatialExit struct {
	ID          string `json:"id"`
	At          Point  `json:"at"`
	Landing     Point  `json:"landing"`
	Destination string `json:"destination"`
}

type SpatialProp struct {
	ModuleID        string                   `json:"module_id,omitempty"`
	Level           int                      `json:"level"`
	Rotation        int                      `json:"rotation"`
	Clearances      []SpatialPlacedClearance `json:"clearances"`
	ID              string                   `json:"id"`
	Rect            SpatialRect              `json:"rect"`
	Glyph           string                   `json:"glyph"`
	StyleID         string                   `json:"style_id"`
	Owner           string                   `json:"owner"`
	Purpose         string                   `json:"purpose"`
	ResolvedVariant int                      `json:"resolved_variant"`
}

type SpatialDecoration struct {
	RouteID string `json:"route_id,omitempty"`
	Symbol  string `json:"symbol,omitempty"`
	Shape   string `json:"shape"`
	ID      string `json:"id"`
	At      Point  `json:"at"`
	StyleID string `json:"style_id"`
	// -1 selects a deterministic variant; nonnegative values are manual overrides.
	Variant         int `json:"variant"`
	ResolvedVariant int `json:"resolved_variant"`
	// Always "floor": clients must flatten decorations, never render tall blockers.
	Surface string `json:"surface"`
}

type SpatialInteraction struct {
	ID string `json:"id"`
	At Point  `json:"at"`
}

// compileSpatial accepts world.spatial and the authoritative terrain legend.
// An absent optional catalog keeps older worlds compatible. Validation and all
// variant resolution happen at generation time, not on the simulation tick.
func compileSpatial(value cue.Value, walkable map[string]bool) (SpatialCatalog, error) {
	var catalog SpatialCatalog
	if !value.Exists() {
		return catalog, nil
	}
	if err := value.Decode(&catalog); err != nil {
		return catalog, fmt.Errorf("spatial: decode: %w", err)
	}
	if err := catalog.compile(walkable); err != nil {
		return SpatialCatalog{}, err
	}
	return catalog, nil
}

func (catalog *SpatialCatalog) compile(walkable map[string]bool) error {
	styles := make(map[string]SpatialStyle, len(catalog.Styles))
	for _, style := range catalog.Styles {
		if !spatialID(style.ID) || !spatialID(style.AssetStyle) {
			return fmt.Errorf("spatial: invalid style %q or asset style %q", style.ID, style.AssetStyle)
		}
		if _, exists := styles[style.ID]; exists {
			return fmt.Errorf("spatial: duplicate style %q", style.ID)
		}
		if !spatialColor(style.FloorColor) || !spatialColor(style.AccentColor) || style.Variants < 1 || style.Variants > 65536 {
			return fmt.Errorf("spatial: style %q needs #RRGGBB colors and 1..65536 variants", style.ID)
		}
		styles[style.ID] = style
	}
	modules, err := catalog.compileModules()
	if err != nil {
		return err
	}
	levels := make(map[string]bool, len(catalog.Levels))
	for i := range catalog.Levels {
		level := &catalog.Levels[i]
		if !spatialID(level.ID) || levels[level.ID] {
			return fmt.Errorf("spatial: invalid or duplicate level %q", level.ID)
		}
		levels[level.ID] = true
		if err := level.compile(walkable, styles, modules, catalog.Seed); err != nil {
			return fmt.Errorf("spatial level %q: %w", level.ID, err)
		}
	}
	return nil
}

func (level *SpatialLevel) compile(walkable map[string]bool, styles map[string]SpatialStyle, modules map[string]SpatialModule, seed int64) error {
	// This POC's coordinates are meters. The dimensions are authored; 48x32 is
	// content, not an algorithm constant. Bound allocation before multiplying.
	if level.Width < 3 || level.Height < 3 || level.Width > 4096 || level.Height > 4096 || level.Width*level.Height > 1048576 || level.CellMeters != 1 {
		return fmt.Errorf("invalid dimensions or cell_meters (requires 1m cells, at most 1048576 cells)")
	}
	if len(level.Terrain) != level.Height {
		return fmt.Errorf("terrain height %d does not match %d", len(level.Terrain), level.Height)
	}
	floor := make([]bool, level.Width*level.Height)
	count, first := 0, -1
	for y, row := range level.Terrain {
		if len(row) != level.Width {
			return fmt.Errorf("terrain row %d has width %d, expected %d", y, len(row), level.Width)
		}
		for x := 0; x < len(row); x++ {
			glyph := row[x : x+1]
			canWalk, known := walkable[glyph]
			if row[x] < 32 || row[x] > 126 || !known {
				return fmt.Errorf("unknown/non-ASCII terrain glyph at %d,%d", x, y)
			}
			if canWalk {
				cell := y*level.Width + x
				floor[cell] = true
				count++
				if first < 0 {
					first = cell
				}
			}
		}
	}
	if first < 0 {
		return fmt.Errorf("terrain has no walkable floor")
	}
	ids := make(map[string]bool)
	unique := func(kind, id string) error {
		if !spatialID(id) || ids[id] {
			return fmt.Errorf("invalid or duplicate %s ID %q", kind, id)
		}
		ids[id] = true
		return nil
	}
	styleFor := func(id, owner string) (SpatialStyle, error) {
		style, found := styles[id]
		if !found {
			return style, fmt.Errorf("%s references unknown style %q", owner, id)
		}
		return style, nil
	}
	isFloor := func(at Point) bool { return level.contains(at) && floor[at.Y*level.Width+at.X] }
	covered := make([]bool, len(floor))
	for i := range level.Zones {
		zone := &level.Zones[i]
		if err := unique("zone", zone.ID); err != nil {
			return err
		}
		if !level.containsRect(zone.Rect) || !spatialID(zone.Kind) || strings.TrimSpace(zone.Purpose) == "" {
			return fmt.Errorf("zone %q has invalid rect, kind or purpose", zone.ID)
		}
		style, err := styleFor(zone.StyleID, zone.ID)
		if err != nil {
			return err
		}
		zone.ResolvedVariant = spatialVariant(seed, level.ID, "zone", zone.ID, style.Variants)
		for y := zone.Rect.Y; y < zone.Rect.Y+zone.Rect.H; y++ {
			for x := zone.Rect.X; x < zone.Rect.X+zone.Rect.W; x++ {
				cell := y*level.Width + x
				if covered[cell] {
					return fmt.Errorf("zone %q overlaps another zone at %d,%d", zone.ID, x, y)
				}
				covered[cell] = true
			}
		}
	}
	if level.CoverAllFloor {
		for y := 1; y < level.Height-1; y++ {
			for x := 1; x < level.Width-1; x++ {
				cell := y*level.Width + x
				if floor[cell] && !covered[cell] {
					return fmt.Errorf("interior floor %d,%d has no zone", x, y)
				}
			}
		}
	}
	for _, corridor := range level.Corridors {
		if err := unique("corridor", corridor.ID); err != nil {
			return err
		}
		if !level.containsRect(corridor.Rect) || corridor.WidthMeters != float64(min(corridor.Rect.W, corridor.Rect.H))*level.CellMeters {
			return fmt.Errorf("corridor %q has invalid bounds or width_m", corridor.ID)
		}
		for y := corridor.Rect.Y; y < corridor.Rect.Y+corridor.Rect.H; y++ {
			for x := corridor.Rect.X; x < corridor.Rect.X+corridor.Rect.W; x++ {
				if !isFloor(Point{X: x, Y: y}) {
					return fmt.Errorf("corridor %q blocked at %d,%d", corridor.ID, x, y)
				}
			}
		}
	}
	for _, door := range level.Doors {
		if err := unique("door", door.ID); err != nil {
			return err
		}
		if !isFloor(door.At) || strings.TrimSpace(door.Purpose) == "" {
			return fmt.Errorf("door %q must have purpose and walkable cell", door.ID)
		}
	}
	for _, exit := range level.Exits {
		if err := unique("exit", exit.ID); err != nil {
			return err
		}
		if !isFloor(exit.At) || !isFloor(exit.Landing) || !spatialID(exit.Destination) || spatialDistance(exit.At, exit.Landing) != 1 {
			return fmt.Errorf("exit %q requires a walkable cell and adjacent landing and destination", exit.ID)
		}
	}
	footprints := make([]bool, len(floor))
	for i := range level.Props {
		prop := &level.Props[i]
		if err := unique("prop", prop.ID); err != nil {
			return err
		}
		if !level.containsRect(prop.Rect) || len(prop.Glyph) != 1 || prop.Glyph[0] < 32 || prop.Glyph[0] > 126 {
			return fmt.Errorf("prop %q has invalid bounds or glyph", prop.ID)
		}
		canWalk, known := walkable[prop.Glyph]
		if !known || canWalk {
			return fmt.Errorf("prop %q must use an authored nonwalkable terrain glyph", prop.ID)
		}
		if strings.TrimSpace(prop.Owner) == "" || strings.TrimSpace(prop.Purpose) == "" {
			return fmt.Errorf("prop %q needs owner and purpose", prop.ID)
		}
		style, err := styleFor(prop.StyleID, prop.ID)
		if err != nil {
			return err
		}
		prop.ResolvedVariant = spatialVariant(seed, level.ID, "prop", prop.ID, style.Variants)
		for y := prop.Rect.Y; y < prop.Rect.Y+prop.Rect.H; y++ {
			for x := prop.Rect.X; x < prop.Rect.X+prop.Rect.W; x++ {
				cell := y*level.Width + x
				if footprints[cell] || level.Terrain[y][x:x+1] != prop.Glyph {
					return fmt.Errorf("prop %q overlaps another prop or mismatches terrain at %d,%d", prop.ID, x, y)
				}
				footprints[cell] = true
			}
		}
	}
	if err := level.validateModuleInstances(modules, isFloor); err != nil {
		return err
	}
	if err := level.compileRoutes(floor, unique); err != nil {
		return err
	}
	for i := range level.Decorations {
		decoration := &level.Decorations[i]
		if err := unique("decoration", decoration.ID); err != nil {
			return err
		}
		if !isFloor(decoration.At) {
			return fmt.Errorf("decoration %q must be on walkable floor", decoration.ID)
		}
		style, err := styleFor(decoration.StyleID, decoration.ID)
		if err != nil {
			return err
		}
		if decoration.Variant < -1 || decoration.Variant >= style.Variants {
			return fmt.Errorf("decoration %q variant outside style range", decoration.ID)
		}
		if decoration.Surface != "" && decoration.Surface != "floor" {
			return fmt.Errorf("decoration %q must be a flat floor mark", decoration.ID)
		}
		if decoration.Shape == "" {
			decoration.Shape = "mark"
		}
		if decoration.Shape != "mark" && decoration.Shape != "line_x" && decoration.Shape != "line_y" {
			return fmt.Errorf("decoration %q has unknown shape %q", decoration.ID, decoration.Shape)
		}
		decoration.Surface = "floor"
		decoration.ResolvedVariant = decoration.Variant
		if decoration.Variant == -1 {
			decoration.ResolvedVariant = spatialVariant(seed, level.ID, "decoration", decoration.ID, style.Variants)
		}
	}
	for _, spot := range level.InteractionSpots {
		if err := unique("interaction", spot.ID); err != nil {
			return err
		}
		if !isFloor(spot.At) {
			return fmt.Errorf("interaction %q must be walkable", spot.ID)
		}
	}
	// Connected floor guarantees every door, exit approach and interaction can be
	// reached. Diagonal corner contact is not a connection in the server grid.
	reached := make([]bool, len(floor))
	queue := make([]int, 1, count)
	queue[0] = first
	reached[first] = true
	for head := 0; head < len(queue); head++ {
		at := queue[head]
		x, y := at%level.Width, at/level.Width
		for _, next := range [...]Point{{X: x - 1, Y: y}, {X: x + 1, Y: y}, {X: x, Y: y - 1}, {X: x, Y: y + 1}} {
			if !level.contains(next) {
				continue
			}
			index := next.Y*level.Width + next.X
			if floor[index] && !reached[index] {
				reached[index] = true
				queue = append(queue, index)
			}
		}
	}
	if len(queue) != count {
		return fmt.Errorf("disconnected walkable floor: %d of %d cells reachable; open authored doors or remove isolated floor", len(queue), count)
	}
	return level.compileLights(floor, unique)
}

func (level SpatialLevel) contains(at Point) bool {
	return at.X >= 0 && at.Y >= 0 && at.X < level.Width && at.Y < level.Height
}
func (level SpatialLevel) containsRect(rect SpatialRect) bool {
	return rect.X >= 0 && rect.Y >= 0 && rect.W > 0 && rect.H > 0 && rect.X < level.Width && rect.Y < level.Height && rect.W <= level.Width-rect.X && rect.H <= level.Height-rect.Y
}
func spatialDistance(a, b Point) int {
	x, y := a.X-b.X, a.Y-b.Y
	if x < 0 {
		x = -x
	}
	if y < 0 {
		y = -y
	}
	return x + y
}
func spatialVariant(seed int64, level, kind, id string, variants int) int {
	hash := hashfnv.New64a()
	// NUL separators prevent ambiguous concatenations. IDs cannot contain NUL.
	for _, part := range []string{strconv.FormatInt(seed, 10), level, kind, id} {
		_, _ = hash.Write([]byte(part))
		_, _ = hash.Write([]byte{0})
	}
	return int(hash.Sum64() % uint64(variants))
}
func spatialID(id string) bool {
	if id == "" {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-' || c == '.' || c == ':' || c == '/') {
			return false
		}
	}
	return true
}
func spatialColor(color string) bool {
	if len(color) != 7 || color[0] != '#' {
		return false
	}
	for _, c := range color[1:] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}

func (catalog SpatialCatalog) compileModules() (map[string]SpatialModule, error) {
	modules := make(map[string]SpatialModule, len(catalog.Modules))
	if catalog.ModuleSnapMeters < 0 || catalog.ModuleSnapMeters > 4096 || len(catalog.Modules) > 0 && catalog.ModuleSnapMeters == 0 {
		return nil, fmt.Errorf("spatial: module_snap_m must be 1..4096 when modules are defined; terrain cells remain 1m")
	}
	for _, module := range catalog.Modules {
		if !spatialID(module.ID) {
			return nil, fmt.Errorf("spatial: invalid module ID %q", module.ID)
		}
		if _, found := modules[module.ID]; found {
			return nil, fmt.Errorf("spatial: duplicate module %q", module.ID)
		}
		if err := module.validate(); err != nil {
			return nil, fmt.Errorf("spatial module %q: %w", module.ID, err)
		}
		modules[module.ID] = module
	}
	for _, module := range catalog.Modules {
		if module.Kind != "stair" {
			if module.LandingModuleID != "" {
				return nil, fmt.Errorf("spatial module %q: only stairs reference landing_module_id", module.ID)
			}
			continue
		}
		landing, found := modules[module.LandingModuleID]
		if !found || landing.Kind != "landing" {
			return nil, fmt.Errorf("spatial stair %q: landing_module_id must reference a landing prototype", module.ID)
		}
	}
	return modules, nil
}

func (module SpatialModule) validate() error {
	w, h := module.Footprint.W, module.Footprint.H
	if w < 1 || h < 1 || w > 4096 || h > 4096 || module.HeightMM < 1 || module.HeightMM > 1000000 || module.ElevationMM < 0 || module.ElevationMM > 1000000 || module.RiseMM < 0 || module.RiseMM > 1000000 || module.Level < 0 || module.Level > 1000 {
		return fmt.Errorf("invalid footprint, height, elevation, rise or level")
	}
	if module.Placement != "ground" && module.Placement != "definition_only" {
		return fmt.Errorf("unknown placement %q", module.Placement)
	}
	// Construction spacing is authored per prototype. The catalog spacing is
	// a content convention; a cassette, repaired component or larger module can
	// explicitly choose another integer grid without a kind-specific exception.
	if module.SnapMeters < 1 || module.SnapMeters > 4096 || w%module.SnapMeters != 0 || h%module.SnapMeters != 0 {
		return fmt.Errorf("footprint must align to positive snap_m in 1..4096")
	}
	capsule := module.Kind == "shop_capsule" || module.Kind == "living_capsule"
	switch module.Kind {
	case "shop_capsule", "living_capsule", "gallery", "landing", "cassette":
		if module.RiseMM != 0 {
			return fmt.Errorf("non-stair module cannot have a rise")
		}
	case "stair":
		if module.RiseMM < 1 || module.RiseMM > module.HeightMM {
			return fmt.Errorf("stair rise must be positive and fit its height")
		}
	default:
		return fmt.Errorf("unknown module kind %q", module.Kind)
	}
	if module.Placement == "ground" && (!capsule || module.Level != 0 || module.ElevationMM != 0) {
		return fmt.Errorf("only ground capsules can be placed in this POC; other modules are definition_only")
	}
	ids := make(map[string]bool)
	entries, connections := 0, 0
	for _, port := range module.Ports {
		if !spatialID(port.ID) || ids[port.ID] {
			return fmt.Errorf("invalid or duplicate port %q", port.ID)
		}
		ids[port.ID] = true
		if port.At.X < 0 || port.At.Y < 0 || port.At.X >= w || port.At.Y >= h || !spatialPortOnSide(port, w, h) {
			return fmt.Errorf("port %q must lie on its footprint boundary side", port.ID)
		}
		switch port.Kind {
		case "entry":
			entries++
		case "connection":
			connections++
		default:
			return fmt.Errorf("port %q has invalid kind", port.ID)
		}
	}
	if capsule && (entries == 0 || len(module.Clearances) == 0) {
		return fmt.Errorf("capsule needs an entry port and outside clearance")
	}
	if module.Kind == "stair" && connections != 2 {
		return fmt.Errorf("stair needs two connection ports, one for each landing")
	}
	ids = make(map[string]bool) // Port and apron identifiers have separate namespaces.
	for _, clearance := range module.Clearances {
		if !spatialID(clearance.ID) || ids[clearance.ID] {
			return fmt.Errorf("invalid or duplicate clearance %q", clearance.ID)
		}
		ids[clearance.ID] = true
		if !spatialSide(clearance.Side) || clearance.DepthMeters < 1 || clearance.DepthMeters > 4096 || clearance.DepthMeters%module.SnapMeters != 0 || strings.TrimSpace(clearance.Purpose) == "" {
			return fmt.Errorf("clearance %q has invalid side, depth_m or purpose", clearance.ID)
		}
	}
	// Each entry has a front apron; ports are connection interfaces, not holes
	// invented in the capsule's uniform ground-floor collision footprint.
	for _, port := range module.Ports {
		if port.Kind != "entry" {
			continue
		}
		covered := false
		for _, clearance := range module.Clearances {
			if clearance.Side == port.Side {
				covered = true
			}
		}
		if !covered {
			return fmt.Errorf("entry port %q lacks clearance on its side", port.ID)
		}
	}
	return nil
}

func (level SpatialLevel) validateModuleInstances(modules map[string]SpatialModule, isFloor func(Point) bool) error {
	for _, prop := range level.Props {
		if prop.ModuleID == "" {
			// Repaired legacy objects retain their authored 1m coordinates. They cannot
			// smuggle upper-level geometry or module-only clearance into the flat world.
			if prop.Level != 0 || prop.Rotation != 0 || len(prop.Clearances) > 0 {
				return fmt.Errorf("prop %q: module fields require module_id", prop.ID)
			}
			continue
		}
		module, found := modules[prop.ModuleID]
		if !found {
			return fmt.Errorf("prop %q: unknown module %q", prop.ID, prop.ModuleID)
		}
		if module.Placement != "ground" || prop.Level != 0 || module.Level != 0 || module.ElevationMM != 0 {
			return fmt.Errorf("prop %q: definition_only or upper-level module cannot be instantiated", prop.ID)
		}
		if prop.Rotation != 0 && prop.Rotation != 90 && prop.Rotation != 180 && prop.Rotation != 270 {
			return fmt.Errorf("prop %q: rotation must be 0,90,180,270", prop.ID)
		}
		w, h := module.Footprint.W, module.Footprint.H
		if prop.Rotation == 90 || prop.Rotation == 270 {
			w, h = h, w
		}
		if prop.Rect.W != w || prop.Rect.H != h {
			return fmt.Errorf("prop %q: footprint does not match rotated module", prop.ID)
		}
		if prop.Rect.X%module.SnapMeters != 0 || prop.Rect.Y%module.SnapMeters != 0 {
			return fmt.Errorf("prop %q: origin is off module snap grid", prop.ID)
		}
		if len(prop.Clearances) != len(module.Clearances) {
			return fmt.Errorf("prop %q: clearances do not match module", prop.ID)
		}
		seen := make(map[string]bool)
		for _, placed := range prop.Clearances {
			if seen[placed.ID] {
				return fmt.Errorf("prop %q: duplicate clearance %q", prop.ID, placed.ID)
			}
			seen[placed.ID] = true
			var definition *SpatialModuleClearance
			for i := range module.Clearances {
				if module.Clearances[i].ID == placed.ID {
					definition = &module.Clearances[i]
					break
				}
			}
			if definition == nil {
				return fmt.Errorf("prop %q: unknown clearance %q", prop.ID, placed.ID)
			}
			expected := spatialClearanceRect(module, *definition, prop.Rotation)
			expected.X += prop.Rect.X
			expected.Y += prop.Rect.Y
			if placed.Rect != expected || !level.containsRect(placed.Rect) || spatialRectsOverlap(placed.Rect, prop.Rect) {
				return fmt.Errorf("prop %q: clearance %q does not match outside module apron or is out of bounds", prop.ID, placed.ID)
			}
			for _, corridor := range level.Corridors {
				if spatialRectsOverlap(placed.Rect, corridor.Rect) {
					return fmt.Errorf("prop %q: clearance %q occupies main corridor %q", prop.ID, placed.ID, corridor.ID)
				}
			}
			for y := placed.Rect.Y; y < placed.Rect.Y+placed.Rect.H; y++ {
				for x := placed.Rect.X; x < placed.Rect.X+placed.Rect.W; x++ {
					if !isFloor(Point{X: x, Y: y}) {
						return fmt.Errorf("prop %q: clearance %q blocked at %d,%d", prop.ID, placed.ID, x, y)
					}
				}
			}
		}
		for _, port := range module.Ports {
			if port.Kind != "entry" {
				continue
			}
			approach := port.At
			switch port.Side {
			case "north":
				approach.Y--
			case "east":
				approach.X++
			case "south":
				approach.Y++
			case "west":
				approach.X--
			}
			approach = spatialRotateCell(approach, module.Footprint, prop.Rotation)
			approach.X += prop.Rect.X
			approach.Y += prop.Rect.Y
			if !isFloor(approach) {
				return fmt.Errorf("prop %q: entry port %q has no accessible approach", prop.ID, port.ID)
			}
		}
	}
	return nil
}

func spatialSide(side string) bool {
	return side == "north" || side == "east" || side == "south" || side == "west"
}
func spatialPortOnSide(port SpatialModulePort, w, h int) bool {
	switch port.Side {
	case "north":
		return port.At.Y == 0
	case "east":
		return port.At.X == w-1
	case "south":
		return port.At.Y == h-1
	case "west":
		return port.At.X == 0
	}
	return false
}
func spatialRotateCell(at Point, footprint SpatialFootprint, rotation int) Point {
	switch rotation {
	case 90:
		return Point{X: footprint.H - 1 - at.Y, Y: at.X}
	case 180:
		return Point{X: footprint.W - 1 - at.X, Y: footprint.H - 1 - at.Y}
	case 270:
		return Point{X: at.Y, Y: footprint.W - 1 - at.X}
	}
	return at
}
func spatialClearanceRect(module SpatialModule, clearance SpatialModuleClearance, rotation int) SpatialRect {
	w, h, depth := module.Footprint.W, module.Footprint.H, clearance.DepthMeters
	rect := SpatialRect{}
	switch clearance.Side {
	case "north":
		rect = SpatialRect{X: 0, Y: -depth, W: w, H: depth}
	case "east":
		rect = SpatialRect{X: w, Y: 0, W: depth, H: h}
	case "south":
		rect = SpatialRect{X: 0, Y: h, W: w, H: depth}
	case "west":
		rect = SpatialRect{X: -depth, Y: 0, W: depth, H: h}
	}
	a := spatialRotateCell(Point{X: rect.X, Y: rect.Y}, module.Footprint, rotation)
	b := spatialRotateCell(Point{X: rect.X + rect.W - 1, Y: rect.Y + rect.H - 1}, module.Footprint, rotation)
	return SpatialRect{X: min(a.X, b.X), Y: min(a.Y, b.Y), W: max(a.X, b.X) - min(a.X, b.X) + 1, H: max(a.Y, b.Y) - min(a.Y, b.Y) + 1}
}
func spatialRectsOverlap(a, b SpatialRect) bool {
	return a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H
}
