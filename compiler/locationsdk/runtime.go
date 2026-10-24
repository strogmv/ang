package locationsdk

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

const RuntimeSchema = "ang.location-runtime.v1"

// RuntimeOptions keeps movement on the caller's authoritative tick clock. The
// existing integer-cell movement kernel consumes this navigation unchanged;
// only the explicitly declared metric cell size is 500 mm in these scenes.
type RuntimeOptions struct {
	TickHz         int    `json:"tick_hz"`
	MoveEveryTicks int64  `json:"move_every_ticks"`
	InitialTimeID  string `json:"initial_time_id,omitempty"`
	InitialEventID string `json:"initial_event_id,omitempty"`
}

// RuntimePath preserves the authored ordered centreline for procedural floor
// guides, beside the expanded reservation cells used by the geometry compiler.
type RuntimePath struct {
	ID     string `json:"id"`
	Points []Cell `json:"points"`
	Width  int    `json:"width"`
	Cells  []Cell `json:"cells"`
	Reason string `json:"reason"`
}

// RuntimeDecoration carries a complete visual box in global grid coordinates.
// Anchor is the rotated socket pivot retained from the compiled authoring view;
// At is the minimum AABB cell, and width/depth have already been rotated. A
// renderer draws this box directly and must not rotate it for a second time.
type RuntimeDecoration struct {
	ID           string `json:"id"`
	InstanceID   string `json:"instance_id"`
	SocketID     string `json:"socket_id"`
	DecorationID string `json:"decoration_id"`
	ImageRef     string `json:"image_ref,omitempty"`
	At           Cell   `json:"at"`
	Anchor       Cell   `json:"anchor"`
	Z            int    `json:"z"`
	Width        int    `json:"width"`
	Depth        int    `json:"depth"`
	Height       int    `json:"height"`
	Rotation     int    `json:"rotation"`
	Reason       string `json:"reason"`
}

type RuntimeView struct {
	TimeID         string              `json:"time_id"`
	EventID        string              `json:"event_id"`
	StructuralHash string              `json:"structural_hash"`
	Brightness     float64             `json:"brightness"`
	AccentColor    string              `json:"accent_color,omitempty"`
	ActivityTags   []string            `json:"activity_tags"`
	Decorations    []RuntimeDecoration `json:"decorations"`
}

// RuntimeLevel is deliberately flat for Unity JsonUtility as well as Go JSON.
// Coordinates in all footprints, ports, clearances and floor cells are already
// world-grid coordinates; renderers must not apply instance rotation twice.
// Actor cell (x,y) means the metric centre ((x+.5)*cell_meters,
// (y+.5)*cell_meters). In Unity these horizontal coordinates map to X/Z;
// floor Y is elevation*cell_meters and module height is height*cell_meters.
type RuntimeLevel struct {
	ID                 string              `json:"id"`
	Scenario           string              `json:"scenario"`
	UnitMM             int                 `json:"unit_mm"`
	CellMeters         float64             `json:"cell_meters"`
	Seed               int64               `json:"seed"`
	Width              int                 `json:"width"`
	Depth              int                 `json:"depth"`
	Elevation          int                 `json:"elevation"`
	Grounded           bool                `json:"grounded"`
	Spawn              Cell                `json:"spawn"`
	Entries            []Cell              `json:"entries"`
	Terrain            []string            `json:"terrain"`
	WalkableGlyphs     []string            `json:"walkable_glyphs"`
	OpaqueGlyphs       []string            `json:"opaque_glyphs"`
	InitialLayoutKnown bool                `json:"initial_layout_known"`
	MoveEveryTicks     int64               `json:"move_every_ticks"`
	MovementStepNS     int64               `json:"movement_step_ns"`
	StructuralHash     string              `json:"structural_hash"`
	Instances          []CompiledInstance  `json:"instances"`
	Ports              []CompiledPort      `json:"ports"`
	Clearances         []CompiledClearance `json:"clearances"`
	ReservedPaths      []RuntimePath       `json:"reserved_paths"`
	Barriers           []Barrier           `json:"barriers"`
	Drops              []DropAnnotation    `json:"drops"`
	Views              []RuntimeView       `json:"views"`
	Floor              []PreviewFloor      `json:"floor"`
	Diagnostics        []Diagnostic        `json:"diagnostics"`
}

// RuntimeBundle describes an isolated SDK scenario: its static geometry is
// deliberately public, and it creates no NPCs, items, hidden rooms or inter-level
// edges. An opening is not an invented transport edge or a dynamic door.
// The collision contract is cardinal cell occupancy, not a swept body collider.
type RuntimeBundle struct {
	Schema              string         `json:"schema"`
	ContentHash         string         `json:"content_hash"`
	UnitMM              int            `json:"unit_mm"`
	CellMeters          float64        `json:"cell_meters"`
	TickHz              int            `json:"tick_hz"`
	MoveEveryTicks      int64          `json:"move_every_ticks"`
	MovementStepNS      int64          `json:"movement_step_ns"`
	CollisionModel      string         `json:"collision_model"`
	PublicArchitecture  bool           `json:"public_architecture"`
	InitialTimeID       string         `json:"initial_time_id"`
	InitialEventID      string         `json:"initial_event_id"`
	Levels              []RuntimeLevel `json:"levels"`
	Surfaces            []Surface      `json:"surfaces,omitempty"`
	DefaultFloorSurface string         `json:"default_floor_surface,omitempty"`
}

// CompileRuntime compiles and adapts the entire batch before exposing any
// result. Failures return an empty bundle plus structured diagnostics. The
// caller owns all returned data; no input slice is sorted or shared with it.
func CompileRuntime(input Input, options RuntimeOptions) (RuntimeBundle, error) {
	if options.TickHz < 1 || options.TickHz > 240 {
		return RuntimeBundle{}, issue("runtime_clock", "runtime_options.tick_hz", "authoritative tick frequency must be in 1..240 Hz")
	}
	tickNS := int64(1000000000 / options.TickHz)
	if options.MoveEveryTicks < 1 || options.MoveEveryTicks > math.MaxInt64/tickNS {
		return RuntimeBundle{}, issue("runtime_clock", "runtime_options.move_every_ticks", "movement cadence must be positive and fit an int64 nanosecond interval")
	}
	if len(input.Layouts) < 1 || len(input.Layouts) > 256 {
		return RuntimeBundle{}, issue("runtime_batch", "layouts", "runtime bundle must contain 1..256 supported layouts")
	}
	if invalid := runtimePreflight(input); invalid != nil {
		return RuntimeBundle{}, invalid
	}
	if err := ValidateLocks(input); err != nil {
		return RuntimeBundle{}, err
	}
	stepNS := options.MoveEveryTicks * tickNS
	bundle := RuntimeBundle{Schema: RuntimeSchema, UnitMM: UnitMM, CellMeters: float64(UnitMM) / 1000, TickHz: options.TickHz, MoveEveryTicks: options.MoveEveryTicks, MovementStepNS: stepNS, CollisionModel: "cardinal_cell_occupancy", PublicArchitecture: true, Levels: []RuntimeLevel{}}
	seen := map[string]bool{}
	for i, source := range input.Layouts {
		if seen[source.ID] {
			return RuntimeBundle{}, issue("duplicate_layout", fmt.Sprintf("layouts[%d].id", i), "runtime layout IDs must be unique", source.ID)
		}
		seen[source.ID] = true
		compiled, err := Build(input.Catalog, source)
		if err != nil {
			if invalid, ok := err.(*ValidationError); ok {
				issues := append([]Diagnostic{}, invalid.Diagnostics...)
				for j := range issues {
					if issues[j].Path == "layout" || strings.HasPrefix(issues[j].Path, "layout.") {
						issues[j].Path = fmt.Sprintf("layouts[%d]", i) + strings.TrimPrefix(issues[j].Path, "layout")
					}
					issues[j].References = append(append([]string{}, issues[j].References...), source.ID)
				}
				return RuntimeBundle{}, &ValidationError{Diagnostics: issues}
			}
			return RuntimeBundle{}, err
		}
		entries := append([]Cell{}, source.Entries...)
		// Build validates every source entry, so retaining the first authored
		// one is safe. Never choose a substitute cell from a visual preview.
		views, viewError := runtimeViews(compiled.Views, input.Catalog, source)
		if viewError != nil {
			return RuntimeBundle{}, viewError
		}
		level := RuntimeLevel{ID: compiled.ID, Scenario: "sdk:" + compiled.ID, UnitMM: compiled.UnitMM, CellMeters: bundle.CellMeters, Seed: source.Seed, Width: compiled.Width, Depth: compiled.Depth, Elevation: compiled.Elevation, Grounded: compiled.Grounded, Spawn: entries[0], Entries: entries, Terrain: compiled.Terrain, WalkableGlyphs: []string{"."}, OpaqueGlyphs: []string{"#", " "}, InitialLayoutKnown: true, MoveEveryTicks: options.MoveEveryTicks, MovementStepNS: stepNS, StructuralHash: compiled.StructuralHash, Instances: compiled.Instances, Ports: compiled.Ports, Clearances: compiled.Clearances, ReservedPaths: []RuntimePath{}, Barriers: compiled.Barriers, Drops: compiled.Drops, Views: views, Floor: compiled.Preview.Floor, Diagnostics: compiled.Diagnostics}
		pathSources := make(map[string]ReservedPath, len(source.ReservedPaths))
		for _, path := range source.ReservedPaths {
			pathSources[path.ID] = path
		}
		for _, expanded := range compiled.ReservedPaths {
			authored := pathSources[expanded.ID]
			level.ReservedPaths = append(level.ReservedPaths, RuntimePath{ID: expanded.ID, Points: append([]Cell{}, authored.Points...), Width: authored.Width, Cells: expanded.Cells, Reason: expanded.Reason})
		}
		bundle.Levels = append(bundle.Levels, level)
	}
	sort.Slice(bundle.Levels, func(i, j int) bool { return bundle.Levels[i].ID < bundle.Levels[j].ID })
	// Every Build above validated the shared material catalog. Copy values before
	// sorting so neither source order nor subsequent caller edits can affect it.
	bundle.Surfaces = append([]Surface(nil), input.Catalog.Surfaces...)
	sort.Slice(bundle.Surfaces, func(i, j int) bool { return bundle.Surfaces[i].ID < bundle.Surfaces[j].ID })
	bundle.DefaultFloorSurface = input.Catalog.DefaultFloorSurface
	firstViews := bundle.Levels[0].Views
	// A valid catalog always has at least one state on both independent axes.
	bundle.InitialTimeID, bundle.InitialEventID = options.InitialTimeID, options.InitialEventID
	if bundle.InitialTimeID == "" {
		bundle.InitialTimeID = input.Catalog.Times[0].ID
	}
	if bundle.InitialEventID == "" {
		bundle.InitialEventID = input.Catalog.Events[0].ID
	}
	initialFound := false
	for _, view := range firstViews {
		initialFound = initialFound || view.TimeID == bundle.InitialTimeID && view.EventID == bundle.InitialEventID
	}
	if !initialFound {
		return RuntimeBundle{}, issue("runtime_presentation", "runtime_options", "initial time/event pair must exist in the shared catalog", bundle.InitialTimeID, bundle.InitialEventID)
	}
	encoded, err := json.Marshal(bundle)
	if err != nil {
		return RuntimeBundle{}, fmt.Errorf("encode SDK runtime contract: %w", err)
	}
	hash := sha256.Sum256(encoded)
	bundle.ContentHash = hex.EncodeToString(hash[:])
	return bundle, nil
}

func runtimeViews(views []PresentationView, catalog Catalog, layout Layout) ([]RuntimeView, *ValidationError) {
	definitions := map[string]Decoration{}
	for _, definition := range catalog.Decorations {
		definitions[definition.ID] = definition
	}
	instances := map[string]Instance{}
	for _, instance := range layout.Instances {
		instances[instance.ID] = instance
	}
	out := make([]RuntimeView, 0, len(views))
	for _, view := range views {
		runtime := RuntimeView{TimeID: view.TimeID, EventID: view.EventID, StructuralHash: view.StructuralHash, Brightness: view.Brightness, AccentColor: view.AccentColor, ActivityTags: append([]string{}, view.ActivityTags...), Decorations: []RuntimeDecoration{}}
		for _, decoration := range view.Decorations {
			definition, hasDefinition := definitions[decoration.DecorationID]
			instance, hasInstance := instances[decoration.InstanceID]
			if !hasDefinition || !hasInstance {
				return nil, issue("runtime_decoration", "views.decorations", "compiled decoration must retain its source definition and owning instance", decoration.ID)
			}
			at, width, depth := decoration.At, definition.Width, definition.Depth
			switch instance.Rotation {
			case 90:
				at.X -= definition.Depth - 1
				width, depth = definition.Depth, definition.Width
			case 180:
				at.X -= definition.Width - 1
				at.Y -= definition.Depth - 1
			case 270:
				at.Y -= definition.Width - 1
				width, depth = definition.Depth, definition.Width
			}
			runtime.Decorations = append(runtime.Decorations, RuntimeDecoration{ID: decoration.ID, InstanceID: decoration.InstanceID, SocketID: decoration.SocketID, DecorationID: decoration.DecorationID, ImageRef: decoration.ImageRef, At: at, Anchor: decoration.At, Z: decoration.Z, Width: width, Depth: depth, Height: definition.Height, Rotation: instance.Rotation, Reason: decoration.Reason})
		}
		out = append(out, runtime)
	}
	return out, nil
}
