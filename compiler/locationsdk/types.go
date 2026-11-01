// Package locationsdk compiles authored 2.5D location layouts: one planar
// base surface, optionally with additional flat decks joined by stairs
// (multideck.go, document 80).
// All dimensions and coordinates use 500 mm units. It does not invent rooms,
// certify building safety, spawn actors, or implement a runtime renderer.
package locationsdk

const UnitMM = 500

type Cell struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type Rect struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type Side string

const (
	North Side = "north"
	East  Side = "east"
	South Side = "south"
	West  Side = "west"
)

type Input struct {
	Catalog Catalog      `json:"catalog"`
	Layouts []Layout     `json:"layouts"`
	Locks   []LayoutLock `json:"locks,omitempty"`
}

// A lock is an explicit author-approved expectation, not a stored scene or an
// automatic acceptance policy. The CLI prints a candidate; the author updates it.
type LayoutLock struct {
	LayoutID     string `json:"layout_id"`
	ExpectedHash string `json:"expected_hash"`
}

// Geometry is local to a module's unrotated origin. Footprint means physical
// material, not the entire bounding box: an empty shell's interior remains free
// for furniture. Blocked is a subset of Footprint. Clearance and port approach
// cells may lie outside the bounding box, e.g. in front of a cabinet.
type Geometry struct {
	SightClear bool `json:"sight_clear,omitempty"`
	// SightClearCells: blocked cells of low parts only (counters, casings below the
	// eye) that do not stop sight at standing height, inside an otherwise opaque
	// module (demo-hub: a container's shop counter). Omitted = per geometry.
	SightClearCells      []Cell   `json:"sight_clear_cells,omitempty"`
	ID                   string   `json:"id"`
	Width                int      `json:"width"`
	Depth                int      `json:"depth"`
	Height               int      `json:"height"`
	Footprint            []Cell   `json:"footprint"`
	Blocked              []Cell   `json:"blocked"`
	UseClearance         []Cell   `json:"use_clearance"`
	MaintenanceClearance []Cell   `json:"maintenance_clearance"`
	Ports                []Port   `json:"ports"`
	Sockets              []Socket `json:"sockets,omitempty"`
}

type Port struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"` // door, connection, or use
	At       Cell   `json:"at"`
	Opening  []Cell `json:"opening,omitempty"`
	Side     Side   `json:"side"`
	Approach []Cell `json:"approach"`
}

// A socket is a nonblocking visual attachment. Its Z and box are in 500 mm
// units, too. A physical object requires a module, never an event decoration.
type Socket struct {
	ID       string `json:"id"`
	At       Cell   `json:"at"`
	Z        int    `json:"z"`
	Width    int    `json:"width"`
	Depth    int    `json:"depth"`
	Height   int    `json:"height"`
	Blocking bool   `json:"blocking"`
}

type Module struct {
	ID         string    `json:"id"`
	GeometryID string    `json:"geometry_id"`
	Variants   []Variant `json:"variants"`
}

// Variants reference shared geometry instead of carrying geometry copies.
// A different geometry ID is permitted only if its canonical structural
// signature is identical, including ports, access and sockets.
type Variant struct {
	ID         string `json:"id"`
	GeometryID string `json:"geometry_id"`
	ImageRef   string `json:"image_ref,omitempty"`
	SurfaceID  string `json:"surface_id,omitempty"`
}

// Surface textures repeat over authoritative top/side mesh faces; they carry no
// silhouette, pivot, footprint or collision. Image names are extensionless Unity
// Resources keys. File existence and import settings belong to the game adapter.
type Surface struct {
	ID           string          `json:"id"`
	TopImage     string          `json:"top_image"`
	SideImage    string          `json:"side_image"`
	TopRepeatMM  SurfaceRepeatMM `json:"top_repeat_mm"`
	SideRepeatMM SurfaceRepeatMM `json:"side_repeat_mm"`
}

// Repeat is the physical size of one complete texture tile, in millimetres.
// Top X/Y map to horizontal mesh axes; side X/Y mean horizontal distance/height.
type SurfaceRepeatMM struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type TimeState struct {
	ID           string   `json:"id"`
	Brightness   float64  `json:"brightness"`
	ActivityTags []string `json:"activity_tags,omitempty"`
}

type EventState struct {
	ID           string   `json:"id"`
	AccentColor  string   `json:"accent_color,omitempty"`
	ActivityTags []string `json:"activity_tags,omitempty"`
}

type Decoration struct {
	ID       string `json:"id"`
	Width    int    `json:"width"`
	Depth    int    `json:"depth"`
	Height   int    `json:"height"`
	Blocking bool   `json:"blocking"`
	ImageRef string `json:"image_ref,omitempty"`
}

type Catalog struct {
	UnitMM              int          `json:"unit_mm"`
	Geometries          []Geometry   `json:"geometries"`
	Modules             []Module     `json:"modules"`
	Times               []TimeState  `json:"times"`
	Events              []EventState `json:"events"`
	Decorations         []Decoration `json:"decorations,omitempty"`
	Surfaces            []Surface    `json:"surfaces,omitempty"`
	DefaultFloorSurface string       `json:"default_floor_surface,omitempty"`
	GateKits            []GateKit    `json:"gate_kits,omitempty"`
}

type FloorArea struct {
	ID     string `json:"id"`
	Rect   Rect   `json:"rect"`
	Reason string `json:"reason"`
}

type DecorationBinding struct {
	ID           string `json:"id"`
	EventID      string `json:"event_id"`
	SocketID     string `json:"socket_id"`
	DecorationID string `json:"decoration_id"`
	Reason       string `json:"reason"`
}

type Instance struct {
	ID          string              `json:"id"`
	ModuleID    string              `json:"module_id"`
	At          Cell                `json:"at"`
	Rotation    int                 `json:"rotation"`
	VariantID   string              `json:"variant_id,omitempty"`
	Reason      string              `json:"reason"`
	OwnerRef    string              `json:"owner_ref,omitempty"`
	Fate        ItemFate            `json:"fate"`
	Decorations []DecorationBinding `json:"decorations,omitempty"`
}

// ItemFate is authoring intent, not an implemented deletion/event scheduler.
type ItemFate struct {
	Policy   string `json:"policy"` // keep, until_event, unknown
	EventRef string `json:"event_ref,omitempty"`
}

// ReservedPath is an orthogonal polyline. Its width is the union of a square
// stamp around each traversed cell. Even widths have the extra cell toward
// positive X/Y, making authored clearance deterministic at corners as well.
type ReservedPath struct {
	ID     string `json:"id"`
	Points []Cell `json:"points"`
	Width  int    `json:"width"`
	Reason string `json:"reason"`
}

// DropAnnotation can produce a barrier only on a computed floor/void boundary.
// No unannotated edge gets an automatic fence, including grounded interiors.
type DropAnnotation struct {
	ID             string `json:"id"`
	At             Cell   `json:"at"`
	Side           Side   `json:"side"`
	LowerElevation int    `json:"lower_elevation"`
	Barrier        bool   `json:"barrier"`
	Reason         string `json:"reason"`
}

// Deck is an additional surface above (or below) the same plan grid. Elevation is
// signed and absolute: millimetres relative to the map zero, never relative to a
// building base or a floor number. Only flat geometry is implemented; a
// heightfield ground is reserved and rejected explicitly (document 80/81).
type Deck struct {
	ID           string      `json:"id"`
	Kind         string      `json:"kind"`               // deck | roof | ground (reserved)
	Geometry     string      `json:"geometry,omitempty"` // flat | heightfield (reserved)
	ElevationMM  int         `json:"elevation_mm"`
	SlabMM       int         `json:"slab_mm"`
	Walkable     bool        `json:"walkable"`
	CutawayGroup string      `json:"cutaway_group"`
	Floor        []FloorArea `json:"floor"`
	Openings     []FloorArea `json:"openings,omitempty"`
	Reason       string      `json:"reason"`
}

// SurfaceRef addresses one cell of one surface. Location is reserved for future
// connectors that end in another map; this compiler rejects any other value.
type SurfaceRef struct {
	Surface  string `json:"surface"`
	Cell     Cell   `json:"cell"`
	Location string `json:"location,omitempty"`
}

// Connector joins two surfaces with ordinary walking steps. Only kind "stair"
// is implemented. From/To are the landing cells (lane 0) where a body steps on
// and off; the run lies strictly between them along Direction. Lanes extend
// toward +X for a north/south run and toward +Y for an east/west run.
type Connector struct {
	ID           string         `json:"id"`
	Kind         string         `json:"kind"` // stair | ladder | ramp | gangway | lift | cable_capsule
	From         SurfaceRef     `json:"from"`
	To           SurfaceRef     `json:"to"`
	Direction    Side           `json:"direction"`
	Width        int            `json:"width"`
	ClearanceMM  int            `json:"clearance_mm"`
	Flights      []StairFlight  `json:"flights"`
	Landings     []StairLanding `json:"landings,omitempty"`
	Balustrades  Balustrades    `json:"balustrades"`
	CutawayGroup string         `json:"cutaway_group,omitempty"`
	// Construction of the stair body: "" / "solid" (a closed block, sight-opaque below
	// the treads) or "steel_open" (demo-hub, passport hub.stair.2m_3.1: channel stringers,
	// grating treads without risers, the landing on posts; sight passes under it except
	// the stringers). Physics and navigation are the same for both.
	Construction string `json:"construction,omitempty"`
	Reason       string `json:"reason"`
}

// StringerDepthMM is the sight depth of an open steel stair's channel stringer under its tread line.
const StringerDepthMM = 250

type StairFlight struct {
	Risers int `json:"risers"`
	Length int `json:"length"`
}

type StairLanding struct {
	Length int `json:"length"`
}

// Left is the lane-0 side, right the last lane's side, independent of travel.
type Balustrades struct {
	Left        bool `json:"left"`
	Right       bool `json:"right"`
	HeightMM    int  `json:"height_mm"`
	BlocksSight bool `json:"blocks_sight"`
}

// Railing protects a straight run of deck edges; it is movement-blocking by
// construction (it stands on an edge to void) and declares its sight separately.
type Railing struct {
	ID          string `json:"id"`
	Surface     string `json:"surface"`
	From        Cell   `json:"from"`
	To          Cell   `json:"to"`
	Side        Side   `json:"side"`
	HeightMM    int    `json:"height_mm"`
	BlocksSight bool   `json:"blocks_sight"`
	Reason      string `json:"reason"`
}

// Support is declared structure, not a strength calculation: columns,
// bearing walls, wall consoles and beams carry one surface's slab.
type Support struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"` // column | wall_bearing | wall_console | beam | hanger (reserved)
	Surface string `json:"surface"`
	At      *Cell  `json:"at,omitempty"`
	From    *Cell  `json:"from,omitempty"`
	To      *Cell  `json:"to,omitempty"`
	RestsOn string `json:"rests_on,omitempty"`
	DepthMM int    `json:"depth_mm,omitempty"`
	Reason  string `json:"reason"`
}

// Obstacle is a vertical interval above one surface over a plan rectangle.
// Movement and sight are independent: glazing blocks movement only, a window
// gap is expressed as separate intervals. Bottom/top are relative to the surface.
type Obstacle struct {
	ID             string `json:"id"`
	Surface        string `json:"surface"`
	Rect           Rect   `json:"rect"`
	BottomMM       int    `json:"bottom_mm"`
	TopMM          int    `json:"top_mm"`
	BlocksMovement bool   `json:"blocks_movement"`
	BlocksSight    bool   `json:"blocks_sight"`
	Style          string `json:"style"` // wall | glazing | panel
	Reason         string `json:"reason"`
}

// A cutaway group is presentation only: it never removes collision or opens
// sight for any character. Groups are independent; neighbours never vanish.
type CutawayGroup struct {
	ID       string `json:"id"`
	Policy   string `json:"policy"` // occluding | building
	Interior []Rect `json:"interior,omitempty"`
	Reason   string `json:"reason"`
}

// Resident is an authored test-stand person for isolated SDK scenarios only.
type Resident struct {
	ID     string         `json:"id"`
	Name   string         `json:"name"`
	Steps  []ResidentStep `json:"steps"`
	Reason string         `json:"reason"`
}

type ResidentStep struct {
	AtMinute   int    `json:"at_minute"`
	Surface    string `json:"surface,omitempty"`
	At         Cell   `json:"at"`
	Activity   string `json:"activity"`
	Label      string `json:"label"`
	MinSeconds int    `json:"min_seconds,omitempty"`
}

// LayoutCamera is presentation only. Angles describe the camera offset from its
// focus in Unity X/Y/Z: (cos(e)*sin(a), sin(e), cos(e)*cos(a)).
// Omission retains the original 30-degree elevation / 45-degree azimuth view.
// The supported quadrant keeps the existing front-facing artwork convention.
type LayoutCamera struct {
	Projection       string  `json:"projection"`
	ElevationDegrees float64 `json:"elevation_degrees"`
	AzimuthDegrees   float64 `json:"azimuth_degrees"`
	HeroViewHeightM  float64 `json:"hero_view_height_m,omitempty"`
	// NearCut draws the walls nearest to this camera cut at a fixed height with a
	// dark cap, always (not only while they hide the hero). Presentation only: the
	// obstacles keep blocking movement and sight on their full height.
	NearCut *LayoutNearCut `json:"near_cut,omitempty"`
}

// LayoutNearCut: the walls nearest to the camera are drawn cut at HeightMM above
// their surface by a rule of the compiler (demo-hub; renders v13-v18 of the market hub).
type LayoutNearCut struct {
	HeightMM int `json:"height_mm"`
	// Obstacles: optional, cut entirely in addition to the rule (NearCut in multideck.go).
	Obstacles []string `json:"obstacles,omitempty"`
}

// LayoutDaylight is presentation only: the calendar of passive optical light-guide
// delivery (doc 43/72). Brightness is night_brightness outside the day and eases to
// day_brightness over twilight_seconds centred on sunrise/sunset (game seconds).
type LayoutDaylight struct {
	Source          string  `json:"source"`
	SunriseSecond   int     `json:"sunrise_second"`
	SunsetSecond    int     `json:"sunset_second"`
	TwilightSeconds int     `json:"twilight_seconds"`
	DayBrightness   float64 `json:"day_brightness"`
	NightBrightness float64 `json:"night_brightness"`
}

type Layout struct {
	Camera        *LayoutCamera    `json:"camera,omitempty"`
	Daylight      *LayoutDaylight  `json:"daylight,omitempty"`
	ID            string           `json:"id"`
	UnitMM        int              `json:"unit_mm"`
	Seed          int64            `json:"seed"`
	Width         int              `json:"width"`
	Depth         int              `json:"depth"`
	Elevation     int              `json:"elevation"`
	Grounded      bool             `json:"grounded"`
	Floor         []FloorArea      `json:"floor"`
	Entries       []Cell           `json:"entries"`
	Instances     []Instance       `json:"instances"`
	ReservedPaths []ReservedPath   `json:"reserved_paths"`
	Drops         []DropAnnotation `json:"drops"`
	Decks         []Deck           `json:"decks,omitempty"`
	// Multi-deck authoring (document 80). All omitted for planar layouts, so
	// their JSON, locks and structural hashes remain byte-identical.
	BaseSurfaceID string         `json:"base_surface_id,omitempty"`
	Connectors    []Connector    `json:"connectors,omitempty"`
	Supports      []Support      `json:"supports,omitempty"`
	Railings      []Railing      `json:"railings,omitempty"`
	Obstacles     []Obstacle     `json:"obstacles,omitempty"`
	CutawayGroups []CutawayGroup `json:"cutaway_groups,omitempty"`
	// Optional SDK scenario data (doc 85): new-game calendar second and animated
	// gates on door ports. Omitted = previous layouts.
	StartSecond int             `json:"start_second,omitempty"`
	Gates       []GatePlacement `json:"gates,omitempty"`
	// Corridors continuing beyond the plan edge (demo-hub): presentation only.
	Vistas    []Vista    `json:"vistas,omitempty"`
	Residents []Resident `json:"residents,omitempty"`
	// How the people of the layout are drawn (demo-hub v3-people): presentation only.
	Actors *LayoutActors `json:"actors,omitempty"`
}

type Diagnostic struct {
	Code       string   `json:"code"`
	Path       string   `json:"path"`
	Message    string   `json:"message"`
	References []string `json:"references,omitempty"`
}

type ValidationError struct {
	Diagnostics []Diagnostic `json:"diagnostics"`
}

func (e *ValidationError) Error() string {
	if len(e.Diagnostics) == 0 {
		return "location SDK validation failed"
	}
	return e.Diagnostics[0].Path + ": " + e.Diagnostics[0].Code + ": " + e.Diagnostics[0].Message
}

type CompiledInstance struct {
	SightClear      bool     `json:"sight_clear,omitempty"`
	SightClearCells []Cell   `json:"sight_clear_cells,omitempty"`
	ID              string   `json:"id"`
	ModuleID        string   `json:"module_id"`
	VariantID       string   `json:"variant_id"`
	ImageRef        string   `json:"image_ref,omitempty"`
	SurfaceID       string   `json:"surface_id,omitempty"`
	At              Cell     `json:"at"`
	Rotation        int      `json:"rotation"`
	Width           int      `json:"width"`
	Depth           int      `json:"depth"`
	Height          int      `json:"height"`
	GeometryHash    string   `json:"geometry_hash"`
	Footprint       []Cell   `json:"footprint"`
	Blocked         []Cell   `json:"blocked"`
	Reason          string   `json:"reason"`
	OwnerRef        string   `json:"owner_ref"`
	Fate            ItemFate `json:"fate"`
}

type CompiledPort struct {
	ID         string `json:"id"`
	InstanceID string `json:"instance_id"`
	Kind       string `json:"kind"`
	At         Cell   `json:"at"`
	Opening    []Cell `json:"opening"`
	Side       Side   `json:"side"`
	Approach   []Cell `json:"approach"`
}

type CompiledClearance struct {
	InstanceID string `json:"instance_id"`
	Kind       string `json:"kind"`
	Cells      []Cell `json:"cells"`
	Reason     string `json:"reason"`
}

type CompiledPath struct {
	ID     string `json:"id"`
	Cells  []Cell `json:"cells"`
	Reason string `json:"reason"`
}

type Barrier struct {
	ID             string `json:"id"`
	SourceID       string `json:"source_id"`
	At             Cell   `json:"at"`
	Side           Side   `json:"side"`
	LowerElevation int    `json:"lower_elevation"`
	Reason         string `json:"reason"`
}

type CompiledDecoration struct {
	ID           string `json:"id"`
	InstanceID   string `json:"instance_id"`
	SocketID     string `json:"socket_id"`
	DecorationID string `json:"decoration_id"`
	ImageRef     string `json:"image_ref,omitempty"`
	At           Cell   `json:"at"`
	Z            int    `json:"z"`
	Reason       string `json:"reason"`
}

type PresentationView struct {
	TimeID         string               `json:"time_id"`
	EventID        string               `json:"event_id"`
	StructuralHash string               `json:"structural_hash"`
	Brightness     float64              `json:"brightness"`
	AccentColor    string               `json:"accent_color,omitempty"`
	ActivityTags   []string             `json:"activity_tags"`
	Decorations    []CompiledDecoration `json:"decorations"`
}

type PreviewFloor struct {
	At       Cell   `json:"at"`
	SourceID string `json:"source_id"`
	Reason   string `json:"reason"`
}

type PreviewData struct {
	UnitMM int            `json:"unit_mm"`
	Width  int            `json:"width"`
	Depth  int            `json:"depth"`
	Floor  []PreviewFloor `json:"floor"`
}

type CompiledLevel struct {
	Camera         *LayoutCamera       `json:"camera,omitempty"`
	Daylight       *LayoutDaylight     `json:"daylight,omitempty"`
	ID             string              `json:"id"`
	UnitMM         int                 `json:"unit_mm"`
	Width          int                 `json:"width"`
	Depth          int                 `json:"depth"`
	Elevation      int                 `json:"elevation"`
	Grounded       bool                `json:"grounded"`
	Terrain        []string            `json:"terrain"`
	Instances      []CompiledInstance  `json:"instances"`
	Ports          []CompiledPort      `json:"ports"`
	Clearances     []CompiledClearance `json:"clearances"`
	ReservedPaths  []CompiledPath      `json:"reserved_paths"`
	Barriers       []Barrier           `json:"barriers"`
	Drops          []DropAnnotation    `json:"drops"`
	Views          []PresentationView  `json:"views"`
	Diagnostics    []Diagnostic        `json:"diagnostics"`
	StructuralHash string              `json:"structural_hash"`
	Preview        PreviewData         `json:"preview"`
	// Multi-deck only: Terrain then holds the whole navigation plane.
	Navigation *Navigation        `json:"navigation,omitempty"`
	Residents  []CompiledResident `json:"residents,omitempty"`
	// Presentation: wall segments drawn cut for the layout camera (outside the structural hash).
	NearCut *NearCut `json:"near_cut,omitempty"`
}
