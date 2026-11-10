// Package locationintent derives everything the location pipeline needs from
// one authored intent: zones in metres, exits, fixtures on walls, mechanisms and
// light rules. The author never writes cells, wall rectangles, lintels, lamp
// positions, Life edges or registry entries; those are outputs of Derive.
//
// The package reads no files and runs no engine. It reuses locationsdk for
// navigation and the godot/placement packages for presentation, so an intent
// level goes through exactly the same compiler as a hand-written SDK layout.
package locationintent

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler/godot"
	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/placement"
)

type Text struct {
	// NameRU/NameEN: the short object name of an inspection.
	NameRU  string `json:"name_ru,omitempty"`
	NameEN  string `json:"name_en,omitempty"`
	Key     string `json:"key"`
	RU      string `json:"ru"`
	EN      string `json:"en"`
	Context string `json:"context"`
}

// RectM is plan metres: X east, Y south, multiples of 0.5 m.
type RectM struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type Zone struct {
	ID      string `json:"id"`
	Rect    RectM  `json:"rect_m"`
	Purpose string `json:"purpose"`
	Title   *Text  `json:"title,omitempty"`
	Public  bool   `json:"public"`
	Seed    *int   `json:"seed,omitempty"`
}

// Solid is declared mass inside the envelope (a technical thickness, a pier).
// Anything else inside the envelope that is not a zone or a mechanism void is
// an authoring hole and stops derivation.
type Solid struct {
	ID     string `json:"id"`
	Rect   RectM  `json:"rect_m"`
	Reason string `json:"reason"`
}

type MarkerRef struct {
	Number  int    `json:"number"`
	Key     string `json:"key"`
	RU      string `json:"ru,omitempty"`
	EN      string `json:"en,omitempty"`
	Context string `json:"context,omitempty"`
}

type Fare struct {
	Currency        string `json:"currency"`
	AmountMinor     int    `json:"amount_minor"`
	ReceiverAccount string `json:"receiver_account"`
}

type Exit struct {
	ID            string     `json:"id"`
	To            string     `json:"to"`
	Edge          string     `json:"edge"`
	ReverseEdge   string     `json:"reverse_edge"`
	Mode          string     `json:"mode"`
	Wall          string     `json:"wall"`
	Zone          string     `json:"zone"`
	AtM           *float64   `json:"at_m,omitempty"`
	WidthM        float64    `json:"width_m"`
	HeightM       float64    `json:"height_m"`
	TravelSeconds int        `json:"travel_seconds"`
	Seats         int        `json:"seats"`
	Status        string     `json:"status"`
	Marker        *MarkerRef `json:"marker,omitempty"`
	Fare          *Fare      `json:"fare,omitempty"`
	Name          *Text      `json:"name,omitempty"`
	ReverseName   *Text      `json:"reverse_name,omitempty"`
	// status "closed": the edges exist and the server refuses them with this key.
	ClosedReasonKey string `json:"closed_reason_key,omitempty"`
	ClosedReason    *Text  `json:"closed_reason,omitempty"`
	RegionLink      string `json:"region_link,omitempty"`
	RegionNode      string `json:"region_node,omitempty"`
	Peer            *Peer  `json:"peer,omitempty"`
}

// Peer names the neighbour's cells as CUE expressions of package life, for a
// neighbour that is still hand-written. Exit is where the reverse edge leaves
// the neighbour, Entry where this exit's edge lands there.
type Peer struct {
	Exit  string `json:"exit"`
	Entry string `json:"entry"`
}
type Fixture struct {
	ID         string      `json:"id"`
	Kit        string      `json:"kit"`
	Zone       string      `json:"zone"`
	On         string      `json:"on"`
	Align      string      `json:"align"`
	AlignM     *float64    `json:"align_m,omitempty"`
	AtM        *[2]float64 `json:"at_m,omitempty"`
	Rotation   int         `json:"rotation,omitempty"`
	SizeM      *[3]float64 `json:"size_m,omitempty"`
	Purpose    string      `json:"purpose"`
	Owner      string      `json:"owner"`
	Inspection *Text       `json:"inspection,omitempty"`
	Marker     *MarkerRef  `json:"marker,omitempty"`
	Seed       *int        `json:"seed,omitempty"`
}

type Void struct {
	Kind  string `json:"kind"`
	Rect  RectM  `json:"rect_m"`
	Cover bool   `json:"cover"`
}

// MechanismEdges: Out leaves this level through the mechanism, In arrives.
type MechanismEdges struct {
	Out           string `json:"out"`
	In            string `json:"in"`
	To            string `json:"to"`
	TravelSeconds int    `json:"travel_seconds"`
	Seats         int    `json:"seats,omitempty"`
	Name          *Text  `json:"name,omitempty"`
	InName        *Text  `json:"in_name,omitempty"`
	RegionLink    string `json:"region_link,omitempty"`
	RegionNode    string `json:"region_node,omitempty"`
	Peer          *Peer  `json:"peer,omitempty"`
}

type Mechanism struct {
	ID     string          `json:"id"`
	Kind   string          `json:"kind"`
	Zone   string          `json:"zone"`
	Rect   RectM           `json:"rect_m"`
	Void   *Void           `json:"void,omitempty"`
	Edges  *MechanismEdges `json:"edges,omitempty"`
	Params map[string]any  `json:"params"`
}

type LightRule struct {
	ID      string      `json:"id"`
	Rule    string      `json:"rule"`
	Zone    string      `json:"zone,omitempty"`
	On      string      `json:"on,omitempty"`
	StepM   float64     `json:"step_m,omitempty"`
	Fixture string      `json:"fixture,omitempty"`
	AtM     *[2]float64 `json:"at_m,omitempty"`
	HeightM float64     `json:"height_m,omitempty"`
	Kind    string      `json:"kind,omitempty"`
	Mode    *[3]float64 `json:"mode,omitempty"`
	Feed    string      `json:"feed,omitempty"`
}

// FreeMarker and FreeInspection are for things that are not fixtures of this
// file. They must still stand at a real object; the coherence check enforces it.
type FreeMarker struct {
	ID      string     `json:"id"`
	Number  int        `json:"number"`
	Key     string     `json:"key"`
	RU      string     `json:"ru,omitempty"`
	EN      string     `json:"en,omitempty"`
	Context string     `json:"context,omitempty"`
	Zone    string     `json:"zone"`
	AtM     [2]float64 `json:"at_m"`
}

type FreeInspection struct {
	ID   string     `json:"id"`
	Zone string     `json:"zone"`
	AtM  [2]float64 `json:"at_m"`
	Text Text       `json:"text"`
}

type Hidden struct {
	ID     string `json:"id"`
	Ref    string `json:"ref"`
	Zone   string `json:"zone"`
	Volume struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
		Z float64 `json:"z"`
		W float64 `json:"w"`
		H float64 `json:"h"`
		D float64 `json:"d"`
	} `json:"volume_m"`
	TitleRU string `json:"title_ru"`
}

type Person struct {
	Char  string `json:"char"`
	Post  string `json:"post"`
	Label string `json:"label"`
	Hours *struct {
		From string `json:"from"`
		To   string `json:"to"`
	} `json:"hours,omitempty"`
}

type Region struct {
	Deck        string     `json:"deck"`
	Knowledge   string     `json:"knowledge"`
	Neighbours  []string   `json:"neighbours"`
	Node        string     `json:"node,omitempty"`
	Kind        string     `json:"kind,omitempty"`
	Name        Bilingual  `json:"name"`
	Description Bilingual  `json:"description"`
	Box         *RegionBox `json:"box,omitempty"`
}

type Bilingual struct {
	RU string `json:"ru"`
	EN string `json:"en"`
}

// RegionBox is the schematic map box; it stays an authored picture decision.
type RegionBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type ModelSource struct {
	AngID          string `json:"ang_id"`
	Blend          string `json:"blend"`
	ResourcePrefix string `json:"resource_prefix"`
}

// Intent mirrors content/world/location_intent/schema.cue #Location.
type Intent struct {
	ID       string `json:"id"`
	LayoutID string `json:"layout_id,omitempty"`
	// Short names the generated packages and folders (<short>_sdk, godot/<short>).
	Short       string            `json:"short,omitempty"`
	LifeImports map[string]string `json:"life_imports,omitempty"`
	Area        string            `json:"area"`
	Title       Text              `json:"title"`
	Setting     string            `json:"setting"`
	Tier        string            `json:"tier"`
	ElevationM  float64           `json:"elevation_m"`
	WallHeightM float64           `json:"wall_height_m,omitempty"`
	UnitMM      int               `json:"unit_mm"`
	Seed        int               `json:"seed"`
	Camera      struct {
		HeroViewHeightM float64 `json:"hero_view_height_m"`
	} `json:"camera"`
	Zones       []Zone           `json:"zones"`
	Solids      []Solid          `json:"solids,omitempty"`
	Exits       []Exit           `json:"exits"`
	Fixtures    []Fixture        `json:"fixtures"`
	Mechanisms  []Mechanism      `json:"mechanisms"`
	Lights      []LightRule      `json:"lights"`
	Markers     []FreeMarker     `json:"markers,omitempty"`
	Inspections []FreeInspection `json:"inspections,omitempty"`
	Hidden      []Hidden         `json:"hidden"`
	People      []Person         `json:"people"`
	Region      Region           `json:"region"`
	Art         *struct {
		Blend        string        `json:"blend,omitempty"`
		ModelSources []ModelSource `json:"model_sources,omitempty"`
	} `json:"art,omitempty"`
}

// LampKind is game data: one physical fixture family and its light.
type LampKind struct {
	Color    [3]float64 `json:"color"`
	Energy   float64    `json:"energy"`
	RangeM   float64    `json:"range_m"`
	ModelRef string     `json:"model_ref,omitempty"`
	Source   string     `json:"source"`
	// Housing is a flush body drawn on a full wall when the kind has no model:
	// a frame and a lit lens, sized in metres (along the wall, height, depth).
	Housing *LampHousing `json:"housing,omitempty"`
}

type LampHousing struct {
	SizeM     [3]float64 `json:"size_m"`
	FrameRole string     `json:"frame_role"`
	LensRole  string     `json:"lens_role"`
}

// WallKit is a set of wall panel models of one thickness and height. Derived
// walls of a caste that names a kit are laid from its panels; a caste without
// a kit keeps plain boxes.
type WallKit struct {
	ThicknessM       float64        `json:"thickness_m"`
	HeightM          float64        `json:"height_m"`
	CutHeightM       float64        `json:"cut_height_m"`
	LampMountM       float64        `json:"lamp_mount_m"`
	ConduitM         float64        `json:"conduit_m"`
	ConcreteMaterial string         `json:"concrete_material"`
	SteelMaterial    string         `json:"steel_material"`
	Panels           []WallPanel    `json:"panels"` // longest first
	Triangles        map[string]int `json:"triangles,omitempty"`
}

type WallPanel struct {
	LengthM float64  `json:"length_m"`
	Full    []string `json:"full"`
	Cut     []string `json:"cut"`
	// Lamp lists, parallel to Full, the variants with the embed plate inset;
	// a panel gets one only where a lamp actually sits on it.
	Lamp     []string `json:"lamp,omitempty"`
	HasMount bool     `json:"has_mount,omitempty"`
}

// TierDefaults make a bare intent already look like its caste.
type TierDefaults struct {
	FloorRole      string  `json:"floor_role"`
	SteelRole      string  `json:"steel_role"`
	ConcreteRole   string  `json:"concrete_role"`
	Age            float64 `json:"age"`
	WallHeightM    float64 `json:"wall_height_m"`
	WallThicknessM float64 `json:"wall_thickness_m"`
	NearCutM       float64 `json:"near_cut_m"`
	LampAimM       float64 `json:"lamp_aim_m,omitempty"`
	// LightGain scales lamp energy of the caste and LampRangeM overrides the
	// fixture range, so a blockout reads at the level of installed locations.
	LightGain   float64 `json:"light_gain,omitempty"`
	LampRangeM  float64 `json:"lamp_range_m,omitempty"`
	MarkerColor string  `json:"marker_color,omitempty"`
	// WallKit names a panel kit in defaults.wall_kits; empty keeps box walls.
	WallKit string `json:"wall_kit,omitempty"`
	// LampStandoffM moves the light point of a wall lamp off the wall into the
	// room, so the floor is lit and the wall is not burnt out.
	LampStandoffM float64 `json:"lamp_standoff_m,omitempty"`
	// Silver and Gold hide their decay: one hidden caries hint is mandatory there.
	CariesStrength float64                   `json:"caries_strength,omitempty"`
	Enamel         bool                      `json:"enamel,omitempty"`
	LampKind       string                    `json:"lamp_kind"`
	LampHeightM    float64                   `json:"lamp_height_m"`
	LampStepM      float64                   `json:"lamp_step_m"`
	Mode           [3]float64                `json:"mode"`
	DutyMode       [3]float64                `json:"duty_mode"`
	Environment    *godot.EnvironmentProfile `json:"environment"`
}

// Defaults come from the game's CUE (location_intent.defaults); ANG holds no
// colours, energies or resource paths of its own.
type Defaults struct {
	Materials   map[string]godot.Material `json:"materials"`
	Tiers       map[string]TierDefaults   `json:"tiers"`
	LampKinds   map[string]LampKind       `json:"lamp_kinds"`
	GreyboxRole string                    `json:"greybox_role"`
	MarkRole    string                    `json:"mark_role"`
	// Models: kit module ID, instance ID or lamp model_ref -> res:// PackedScene.
	// An ID absent here is drawn as a greybox carrying that ID; adding the entry
	// (after a Blender block with that ang_id is exported) swaps it in.
	Models   map[string]string  `json:"models"`
	WallKits map[string]WallKit `json:"wall_kits,omitempty"`
}

// Diagnostic is one stopped check: a stable code, the authored ID, the cell
// in layout coordinates when there is one, and what to change.
type Diagnostic struct {
	Code    string            `json:"code"`
	ID      string            `json:"id"`
	Cell    *locationsdk.Cell `json:"cell,omitempty"`
	Message string            `json:"message"`
	Hint    string            `json:"hint"`
}

type Error struct{ Diagnostics []Diagnostic }

func (e *Error) Error() string {
	lines := make([]string, 0, len(e.Diagnostics))
	for _, d := range e.Diagnostics {
		at := ""
		if d.Cell != nil {
			at = fmt.Sprintf(" at cell (%d,%d)", d.Cell.X, d.Cell.Y)
		}
		lines = append(lines, fmt.Sprintf("%s: %s%s: %s; %s", d.Code, d.ID, at, d.Message, d.Hint))
	}
	return strings.Join(lines, "\n")
}

type WallEdge struct {
	Cell locationsdk.Cell `json:"cell"` // floor cell
	Side string           `json:"side"`
	Zone string           `json:"zone"`
	Kind string           `json:"kind"` // wall | guard | open
	Exit string           `json:"exit,omitempty"`
}

type DerivedExit struct {
	ID        string             `json:"id"`
	To        string             `json:"to"`
	Wall      string             `json:"wall"`
	Status    string             `json:"status"`
	Edge      string             `json:"edge"`
	Reverse   string             `json:"reverse_edge"`
	At        locationsdk.Cell   `json:"at"`
	Landing   locationsdk.Cell   `json:"landing"`
	Threshold []locationsdk.Cell `json:"threshold"`
}

type DerivedFixture struct {
	ID       string             `json:"id"`
	Kit      string             `json:"kit"`
	Zone     string             `json:"zone"`
	At       locationsdk.Cell   `json:"at"`
	Rotation int                `json:"rotation"`
	W        int                `json:"w"`
	D        int                `json:"d"`
	H        int                `json:"h"`
	Cells    []locationsdk.Cell `json:"cells"`
	Approach locationsdk.Cell   `json:"approach"`
	Greybox  bool               `json:"greybox"`
	ModelRef string             `json:"model_ref"`
	Seed     int                `json:"seed"`
}

type DerivedLamp struct {
	ID    string           `json:"id"`
	Rule  string           `json:"rule"`
	Kind  string           `json:"kind"`
	AtM   [3]float64       `json:"at_m"` // layout metres x, y and height above the floor
	Cell  locationsdk.Cell `json:"cell"`
	Mount string           `json:"mount"` // wall:<side> | ceiling
	Feed  string           `json:"feed"`
	Mode  [3]float64       `json:"mode"`
}

type DerivedMarker struct {
	Color     string            `json:"color,omitempty"`
	GuideFrom *locationsdk.Cell `json:"guide_from,omitempty"`
	ID        string            `json:"id"`
	Number    int               `json:"marker_number"`
	Key       string            `json:"name_key"`
	Kind      string            `json:"kind"`
	At        locationsdk.Cell  `json:"at"`
	EdgeID    string            `json:"edge_id,omitempty"`
	Object    string            `json:"object,omitempty"`
}

type DerivedInspection struct {
	Text    Text             `json:"text"`
	Purpose string           `json:"purpose"`
	ID      string           `json:"id"`
	Level   string           `json:"level"`
	Object  string           `json:"object,omitempty"`
	At      locationsdk.Cell `json:"at"`
	TextKey string           `json:"text_key"`
}

type LifeEdge struct {
	ID              string             `json:"id"`
	From            string             `json:"from"`
	To              string             `json:"to"`
	Exit            *locationsdk.Cell  `json:"exit,omitempty"`
	Entry           *locationsdk.Cell  `json:"entry,omitempty"`
	PeerExit        string             `json:"peer_exit,omitempty"` // "<level>#<edge>": the neighbour owns that cell
	PeerEntry       string             `json:"peer_entry,omitempty"`
	Mode            string             `json:"mode"`
	TravelSeconds   int                `json:"travel_seconds"`
	PeriodSeconds   int                `json:"period_seconds"`
	Seats           int                `json:"seats"`
	NameKey         string             `json:"name_key,omitempty"`
	Fare            *Fare              `json:"fare,omitempty"`
	ClosedReasonKey string             `json:"closed_reason_key,omitempty"`
	ExitZone        []locationsdk.Cell `json:"exit_zone,omitempty"`
	PeerCUE         string             `json:"peer_cue,omitempty"`
}

type RegionNode struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	NameEN           string     `json:"name_en"`
	DeckID           string     `json:"deck_id"`
	LevelID          string     `json:"level_id"`
	InitialKnowledge string     `json:"initial_knowledge"`
	Description      string     `json:"description"`
	DescriptionEN    string     `json:"description_en"`
	Kind             string     `json:"kind,omitempty"`
	Box              *RegionBox `json:"box,omitempty"`
}

type RegionLink struct {
	ID           string   `json:"id"`
	FromNode     string   `json:"from_node"`
	ToNode       string   `json:"to_node"`
	Mode         string   `json:"mode"`
	InitialKnown bool     `json:"initial_known"`
	LifeEdges    []string `json:"life_edges"`
}

type PersonPost struct {
	Char  string           `json:"char"`
	Level string           `json:"level"`
	At    locationsdk.Cell `json:"at"`
	Post  string           `json:"post"`
	Label string           `json:"label"`
	From  string           `json:"from,omitempty"`
	To    string           `json:"to,omitempty"`
}

type HiddenVolume struct {
	ID     string     `json:"id"`
	Ref    string     `json:"ref"`
	MinM   [3]float64 `json:"min_m"`
	MaxM   [3]float64 `json:"max_m"`
	Reason string     `json:"reason"`
}

type MissingKit struct {
	ID        string `json:"id"`
	For       string `json:"for"`
	SizeCells [3]int `json:"size_cells"`
	Reason    string `json:"reason"`
}

// World is what the server-side CUE needs for one level.
type World struct {
	LevelID          string              `json:"level_id"`
	LayoutID         string              `json:"layout_id"`
	NameKey          string              `json:"name_key"`
	PlayerRangeCells int                 `json:"player_range_cells"`
	LifeEdges        []LifeEdge          `json:"life_edges"`
	Markers          []DerivedMarker     `json:"markers"`
	Inspections      []DerivedInspection `json:"inspections"`
	RegionNode       RegionNode          `json:"region_node"`
	RegionLinks      []RegionLink        `json:"region_links"`
	People           []PersonPost        `json:"people"`
	Hidden           []HiddenVolume      `json:"hidden"`
	Localization     []Text              `json:"localization"`
	Mechanisms       []Mechanism         `json:"mechanisms"`
}

// Derived is the whole output for one intent. Layout+Catalog feed
// locationsdk.Build; Authoring, Finishing and Models feed placement.Build and
// godot.Render; World is emitted as generated CUE for the server.
type Derived struct {
	Intent    Intent              `json:"-"`
	Short     string              `json:"short"`
	Offset    locationsdk.Cell    `json:"offset_cells"`
	Envelope  locationsdk.Rect    `json:"envelope"`
	Layout    locationsdk.Layout  `json:"layout"`
	Catalog   locationsdk.Catalog `json:"-"` // game catalog plus greybox passports of this level
	Extra     KitExtras           `json:"kit_extras"`
	Authoring placement.Authoring `json:"placement"`
	Finishing *godot.Finishing    `json:"finishing"`
	Models    map[string]string   `json:"models"`
	World     World               `json:"world"`

	Zones      map[string]locationsdk.Rect `json:"zones"`
	ZoneSeeds  map[string]int              `json:"zone_seeds"`
	Walls      []WallEdge                  `json:"walls"`
	Exits      []DerivedExit               `json:"exits"`
	Fixtures   []DerivedFixture            `json:"fixtures"`
	Mechanisms []DerivedMechanism          `json:"mechanisms"`
	Lamps      []DerivedLamp               `json:"lamps"`
	Missing    []MissingKit                `json:"missing_kit"`
	Greybox    []string                    `json:"greybox_ids"`
	Capped     []CappedPocket              `json:"capped_pockets"`
	Masses     []locationsdk.Rect          `json:"masses"`
	// WallPanels and WallTriangles count the kit panels laid as walls.
	WallPanels    int `json:"wall_panels,omitempty"`
	WallTriangles int `json:"wall_triangles,omitempty"`
}

type DerivedMechanism struct {
	ID   string            `json:"id"`
	Kind string            `json:"kind"`
	Rect locationsdk.Rect  `json:"rect"`
	Void *locationsdk.Rect `json:"void,omitempty"`
}

// KitExtras are passports this level adds to the shared catalog: greybox
// boxes for kit IDs that have no passport yet and semantic reservations of
// mechanisms. They carry the authored ID, so real art replaces them by ID.
type KitExtras struct {
	Geometries []locationsdk.Geometry `json:"geometries"`
	Modules    []locationsdk.Module   `json:"modules"`
}

// CappedPocket is an enclosed non-walkable region the derivation closed with
// mass at the wall top. It is reported so the author can turn it into floor.
type CappedPocket struct {
	ID     string           `json:"id"`
	Cells  int              `json:"cells"`
	Bounds locationsdk.Rect `json:"bounds"`
}
