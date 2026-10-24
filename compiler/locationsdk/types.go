// Package locationsdk compiles authored, single-deck 2.5D location layouts.
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
	Catalog Catalog  `json:"catalog"`
	Layouts []Layout `json:"layouts"`
}

// Geometry is local to a module's unrotated origin. Footprint means physical
// material, not the entire bounding box: an empty shell's interior remains free
// for furniture. Blocked is a subset of Footprint. Clearance and port approach
// cells may lie outside the bounding box, e.g. in front of a cabinet.
type Geometry struct {
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
	UnitMM      int          `json:"unit_mm"`
	Geometries  []Geometry   `json:"geometries"`
	Modules     []Module     `json:"modules"`
	Times       []TimeState  `json:"times"`
	Events      []EventState `json:"events"`
	Decorations []Decoration `json:"decorations,omitempty"`
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

type Deck struct {
	ID        string `json:"id"`
	Elevation int    `json:"elevation"`
}

type Layout struct {
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
	ID           string   `json:"id"`
	ModuleID     string   `json:"module_id"`
	VariantID    string   `json:"variant_id"`
	ImageRef     string   `json:"image_ref,omitempty"`
	At           Cell     `json:"at"`
	Rotation     int      `json:"rotation"`
	Width        int      `json:"width"`
	Depth        int      `json:"depth"`
	Height       int      `json:"height"`
	GeometryHash string   `json:"geometry_hash"`
	Footprint    []Cell   `json:"footprint"`
	Blocked      []Cell   `json:"blocked"`
	Reason       string   `json:"reason"`
	OwnerRef     string   `json:"owner_ref"`
	Fate         ItemFate `json:"fate"`
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
}
