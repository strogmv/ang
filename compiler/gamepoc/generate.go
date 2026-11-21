// Package gamepoc generates the first local game host and its thin Unity view.
// It is deliberately a limited slice, not the full world/session-v2 runtime.
package gamepoc

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/strogmv/ang/compiler/determinism"
	inventoryRuntime "github.com/strogmv/ang/compiler/inventory/runtime"
	goformat "go/format"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/load"
	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler/barks"
	barksRuntime "github.com/strogmv/ang/compiler/barks/runtime"
	"github.com/strogmv/ang/compiler/behavior"
	"github.com/strogmv/ang/compiler/clothing"
	clothingRuntime "github.com/strogmv/ang/compiler/clothing/runtime"
	"github.com/strogmv/ang/compiler/crafting"
	craftingRuntime "github.com/strogmv/ang/compiler/crafting/runtime"
	"github.com/strogmv/ang/compiler/emitter"
	"github.com/strogmv/ang/compiler/inventory"
	inv "github.com/strogmv/ang/compiler/inventory/runtime"
	"github.com/strogmv/ang/compiler/locationsdk"
	"github.com/strogmv/ang/compiler/schemavendor"
	"github.com/strogmv/ang/compiler/wounds"
	woundsRuntime "github.com/strogmv/ang/compiler/wounds/runtime"
)

//go:embed templates/*
var sources embed.FS

type Visibility struct {
	PlayerRange  int    `json:"player_range_cells"`
	Field        string `json:"field"`
	Remember     string `json:"remember"`
	UnknownGlyph string `json:"unknown_glyph"`
	HearingSound string `json:"debug_hearing_sound"`
}

type Config struct {
	GodotTargets          []string          `json:"godot_targets,omitempty"`
	Rendering             UnityRendering    `json:"rendering"`
	DebugPerception       bool              `json:"debug_perception"`
	CaptureDemo           DemoConfig        `json:"capture_demo"`
	WorldDir              string            `json:"world_dir"`
	LevelID               string            `json:"level_id"`
	UnityDir              string            `json:"unity_dir"`
	Title                 string            `json:"title"`
	Subtitle              string            `json:"subtitle"`
	Seed                  int64             `json:"seed"`
	SchedulerHz           int               `json:"scheduler_hz"`
	TickHz                int               `json:"tick_hz"`
	GameSecondsPerTick    int64             `json:"game_seconds_per_tick"`
	MoveEveryTicks        int64             `json:"move_every_ticks"`
	InputAckBudgetMS      int               `json:"input_ack_budget_ms"`
	MinClientFPS          int               `json:"min_client_fps"`
	PresentationLeadMS    int               `json:"presentation_lead_ms"`
	PresentationHorizonMS int               `json:"presentation_horizon_ms"`
	QueueCapacity         int               `json:"queue_capacity"`
	MaxCatchupSteps       int               `json:"max_catchup_steps"`
	SaveSlots             []string          `json:"save_slots"`
	SaveVersion           int               `json:"save_version"`
	SaveCompatibleHashes  []string          `json:"save_compatible_hashes,omitempty"`
	MaxSaveBytes          int64             `json:"max_save_bytes"`
	Labels                map[string]string `json:"labels"`
	// Saves of the other kinds (owner 19.11): the timed autosave and the quick save, each in its own slots rotating
	// (templates/saves.go.tmpl). Without them a game has only the manual save_slots.
	AutoSave  *AutoSavePolicy  `json:"auto_save,omitempty"`
	QuickSave *QuickSavePolicy `json:"quick_save,omitempty"`
	// The save key's classification of the server's runtime package (save_key.go, owner 20.11): read into
	// Compiled.SaveKeyPolicy and never written to world.json, which carries the computed key instead.
	SaveKey *SaveKeyPolicy `json:"save_key,omitempty"`
}

// AutoSavePolicy: the server saves on its own into Slots (the oldest is overwritten) after EveryPlayMinutes of play — the
// world running, not paused — since the last save of any kind, the start of a new game or a load; never while the hero
// is in combat or under an alarm (it waits for them to end).
type AutoSavePolicy struct {
	Slots            []string `json:"slots"`
	EveryPlayMinutes int      `json:"every_play_minutes"`
}

// QuickSavePolicy: SaveKey saves at once into Slots (the oldest is overwritten), LoadKey loads the newest of them; the
// client shows a short note for NoteSeconds. Godot key names (OS.find_keycode_from_string).
type QuickSavePolicy struct {
	Slots       []string `json:"slots"`
	SaveKey     string   `json:"save_key"`
	LoadKey     string   `json:"load_key"`
	NoteSeconds float64  `json:"note_seconds"`
}

// validSaveSlotID: a save slot names its file in the data folder.
func validSaveSlotID(slot string) bool {
	return slot != "" && slot != "." && !strings.ContainsAny(slot, "/\\.")
}

// validateSaveKinds: every slot of every kind is a distinct file name; the numbers are sane.
func (c Config) validateSaveKinds() error {
	seen := map[string]bool{}
	for _, slot := range c.SaveSlots {
		if !validSaveSlotID(slot) || seen[slot] {
			return fmt.Errorf("invalid or duplicate save slot %q", slot)
		}
		seen[slot] = true
	}
	kinds := map[string][]string{}
	if a := c.AutoSave; a != nil {
		if a.EveryPlayMinutes < 1 || a.EveryPlayMinutes > 24*60 {
			return fmt.Errorf("auto_save: every_play_minutes must be 1..1440")
		}
		kinds["auto_save"] = a.Slots
	}
	if q := c.QuickSave; q != nil {
		if q.SaveKey == "" || q.LoadKey == "" || q.SaveKey == q.LoadKey {
			return fmt.Errorf("quick_save: save_key and load_key must be two different keys")
		}
		if q.NoteSeconds < 0.5 || q.NoteSeconds > 10 {
			return fmt.Errorf("quick_save: note_seconds must be 0.5..10")
		}
		kinds["quick_save"] = q.Slots
	}
	for _, kind := range []string{"auto_save", "quick_save"} {
		slots, ok := kinds[kind]
		if !ok {
			continue
		}
		if len(slots) == 0 || len(slots) > 32 {
			return fmt.Errorf("%s: 1..32 slots", kind)
		}
		for _, slot := range slots {
			if !validSaveSlotID(slot) || seen[slot] {
				return fmt.Errorf("%s: invalid or duplicate save slot %q", kind, slot)
			}
			seen[slot] = true
		}
	}
	return nil
}

type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}
type Entity struct {
	Vectors  []string `json:"vectors"`
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Position Point    `json:"position"`
	Faction  string   `json:"faction"`
}

// Camera is a level sensor: it produces observations for the monitoring post only.
type Camera struct {
	ID       string `json:"id"`
	Position Point  `json:"position"`
	Range    int    `json:"range_cells"`
	Online   bool   `json:"online"`
}

// Scene carries the level's hearing-related data: ambient noise and per-tile noise sources.
type Scene struct {
	Cameras      []Camera       `json:"cameras"`
	AmbientNoise int            `json:"ambient_noise"`
	TileNoise    map[string]int `json:"tile_noise"`
}

// ArtSprite is one sprite recipe from CUE (world.art.sprites). Colors map a role to a palette key.
type ArtSprite struct {
	Shape    string            `json:"shape"`
	Style    string            `json:"style"`
	Height   int               `json:"height"`
	Variants int               `json:"variants"`
	Weights  []int             `json:"weights"`
	Colors   map[string]string `json:"colors"`
}

// Art carries the style tokens (docs/art/00-art-bible.md) used to draw sprites and by the Unity view.
// CharacterFrame uses top-left atlas coordinates and local feet coordinates.
type CharacterFrame struct {
	FlipX        bool `json:"flip_x,omitempty"`
	Row          int  `json:"row"`
	Column       int  `json:"column"`
	X            int  `json:"x"`
	Y            int  `json:"y"`
	W            int  `json:"w"`
	H            int  `json:"h"`
	FeetX        int  `json:"feet_x"`
	FeetY        int  `json:"feet_y"`
	BodyHeightPX int  `json:"body_height_px"`
}
type CharacterSheet struct {
	Frames       []CharacterFrame `json:"frames,omitempty"`
	SmoothFilter bool             `json:"smooth_filter,omitempty"`
	Image        string           `json:"image"`
	IdleImage    string           `json:"idle_image,omitempty"`
	HeightMM     int              `json:"height_mm,omitempty"`
	BodyHeightPX int              `json:"body_height_px,omitempty"`
	FrameW       int              `json:"frame_w"`
	FrameH       int              `json:"frame_h"`
	DrawSize     int              `json:"draw_size"`
	FeetY        int              `json:"feet_y"`
	Idle         int              `json:"idle"`
	Walk         []int            `json:"walk"`
	Rows         []int            `json:"rows"`
}
type CharacterArt struct {
	DefaultSheets []string                  `json:"default_sheets,omitempty"`
	FPS           float64                   `json:"fps"`
	RateVariation float64                   `json:"rate_variation"`
	Sheets        map[string]CharacterSheet `json:"sheets"`
	Actors        map[string]string         `json:"actors"`
}
type PresentationCamera struct {
	ZoomDefault float64 `json:"zoom_default"`
	ZoomMin     float64 `json:"zoom_min"`
	ZoomMax     float64 `json:"zoom_max"`
}
type Art struct {
	HUD        *HudArt            `json:"hud,omitempty"`
	Floor      *FloorMaterial     `json:"floor,omitempty"`
	Scenes     []ArtScene         `json:"scenes"`
	Painted    []PaintedScene     `json:"painted"`
	Camera     PresentationCamera `json:"camera"`
	Characters CharacterArt       `json:"characters"`
	// 3D looks of people (looks.go): the model, its record and its glowing surfaces; who wears which.
	Looks       map[string]Look   `json:"looks"`
	PersonLooks map[string]string `json:"person_looks"`
	// The crowd (looks.go, market people v1): a population look (the `look` of a visitor kind) -> its 3D look and the
	// colourways its wearers are drawn in, with their shares.
	PopulationLooks map[string]PopulationLook `json:"population_looks"`
	// Body shapes of persons who wear their own look (looks_shapes.go, characters v1 job 05): person -> {blend shape of his
	// look's model: 0..1} — one set of clothes on many bodies.
	PersonShapes map[string]map[string]float64 `json:"person_shapes,omitempty"`
	// People of the world drawn as wearers of a population look by one general rule in CUE (looks_parts.go, market people v1
	// part B: the traders by caste, body and age): person -> the population look and his age.
	PersonPopulations map[string]PersonPopulation `json:"person_populations"`
	// The painted people test (looks_painttest.go): which people are drawn in which version (now, sprite, painted3d).
	PaintTest *PaintTest `json:"paint_test,omitempty"`
	// What age adds to a look's posture (looks_motion.go): bands from an age up, the highest reached counts.
	PostureAge []LookPostureAge `json:"posture_age,omitempty"`
	// The clothing system v1 (wardrobe.go): base bodies and items checked against the wardrobe build's record; outfit rules
	// (the crowd's kinds and the people of the world by a general rule) and people's own outfits.
	Wardrobe      *Wardrobe               `json:"wardrobe,omitempty"`
	OutfitRules   map[string]OutfitRule   `json:"outfit_rules,omitempty"`
	PersonOutfits map[string]PersonOutfit `json:"person_outfits,omitempty"`
	Tile          struct{ W, H int }      `json:"tile"`
	Palette       map[string]string       `json:"palette"`
	Sprites       map[string]ArtSprite    `json:"sprites"`
}

// LoadingScreen is a full-screen picture shown while the server answers a new game or load request (world.loading).
type LoadingScreen struct {
	Image   string `json:"image"`
	Context string `json:"context"`
	Note    string `json:"note"`
}

// Loading carries the loading-screen set; the caption is drawn by Unity so it can be translated.
type Loading struct {
	Caption string                   `json:"caption"`
	MinMS   int                      `json:"min_ms"`
	Screens map[string]LoadingScreen `json:"screens"`
}

// Menu is the main-menu backdrop picture (world.menu); buttons and title are drawn by Unity.
type Menu struct {
	Background string `json:"background"`
	Note       string `json:"note"`
}

type Compiled struct {
	ExitZones    *ExitZoneCatalog    `json:"exit_zones,omitempty"`
	LiftingDecks *LiftingDeckCatalog `json:"lifting_decks,omitempty"`
	TurretWatch  *TurretWatchCatalog `json:"turret_watch,omitempty"`
	// The armour's biomonitor (biomonitor.go): the shared table and who wears a look with one.
	Biomonitor *BiomonitorCatalog `json:"biomonitor,omitempty"`
	// The hero's chip HUD (chip_hud.go, owner 20.11): what the chip shows him; presentation, read by no rule.
	Hud *ChipHud `json:"hud,omitempty"`
	// Access zones (access_zones.go): the floor marking for the client, the scan, countdown and fire for the server.
	AccessZones      []AccessZone           `json:"access_zones,omitempty"`
	GodotScenes      []GodotScene           `json:"-"`
	GodotTargets     []string               `json:"-"`
	GodotTheme       *GodotTheme            `json:"-"`
	GodotSettings    *GodotSettings         `json:"-"`
	GodotScreens     *GodotScreens          `json:"-"`
	WebSocket        bool                   `json:"-"`
	ViewDelivery     bool                   `json:"-"`
	CartParking      *CartParkingCatalog    `json:"cart_parking,omitempty"`
	CargoConvoys     *CargoConvoyCatalog    `json:"cargo_convoys,omitempty"`
	MedicalCapsules  *MedicalCapsuleCatalog `json:"medical_capsules,omitempty"`
	SanitaryServices *SanitaryCatalog       `json:"sanitary_services,omitempty"`
	TrainMotion      *TrainMotionCatalog    `json:"train_motion,omitempty"`
	PlayableScenes   json.RawMessage        `json:"playable_scenes,omitempty"`
	CombatRuntime    json.RawMessage        `json:"combat_runtime,omitempty"`
	Illumination     json.RawMessage        `json:"illumination_runtime,omitempty"`
	Stealth          json.RawMessage        `json:"stealth_runtime,omitempty"`
	CombatLife       json.RawMessage        `json:"combat_life,omitempty"`
	// Body spacing of a fight (doc 96 v2, world.combat_spacing); absent = cell occupancy only.
	CombatSpacing    json.RawMessage            `json:"combat_spacing,omitempty"`
	Wayfinding       WayfindingCatalog          `json:"wayfinding,omitempty"`
	ModularLocations []ModularLocation          `json:"modular_locations,omitempty"`
	LocationSDK      *locationsdk.RuntimeBundle `json:"location_sdk,omitempty"`
	// Fonts of world.world_text; per-locale layouts use engine-neutral font metrics.
	WorldText               WorldTextCatalog               `json:"-"`
	WorldTextDrawn          *WorldTextRuntime              `json:"-"`
	Localization            LocalizationCatalog            `json:"localization,omitempty"`
	Region                  RegionCatalog                  `json:"region_map,omitempty"`
	PaintedInventoryAnchors map[string]map[string]inv.Cell `json:"painted_inventory_anchors,omitempty"`
	Clothing                clothingRuntime.Model          `json:"clothing"`
	Spatial                 SpatialCatalog                 `json:"spatial"`
	Life                    json.RawMessage                `json:"life"`
	Visibility              Visibility                     `json:"visibility"`
	Opaque                  map[string]bool                `json:"opaque"`
	Inventory               inv.Model                      `json:"inventory"`
	Scene                   Scene                          `json:"scene"`
	AI                      json.RawMessage                `json:"ai"`
	AIScenarios             json.RawMessage                `json:"-"`
	AIUtility               json.RawMessage                `json:"-"`
	Psyche                  PsycheTable                    `json:"psyche"`
	Config                  Config                         `json:"config"`
	ContentHash             string                         `json:"content_hash"`
	Terrain                 []string                       `json:"terrain"`
	Walkable                map[string]bool                `json:"walkable"`
	MoveCost                map[string]int                 `json:"move_cost"`
	TileSprites             map[string]string              `json:"-"`
	Art                     Art                            `json:"-"`
	Loading                 Loading                        `json:"-"`
	Menu                    Menu                           `json:"-"`
	Spawn                   Point                          `json:"spawn"`
	Entities                []Entity                       `json:"entities"`

	// Dome-system vending terminals (demo-hub v12-vending, world.vending); nil in worlds without them.
	Vending *VendingCatalog `json:"vending,omitempty"`
	// Peddlers of the hub (demo-hub v23, world.peddlers): mobile shopfronts on a round of stops; nil in worlds without them.
	Peddlers *PeddlerCatalog `json:"peddlers,omitempty"`
	// Hall visitors (world.visitors): light buyers from the other sectors and the stalls' consignments; nil without them.
	Visitors *VisitorCatalog `json:"visitors,omitempty"`
	// The street musician of a market level (market musician v1, world.busker): his spot, day, tunes, listeners; nil without him.
	Busker            *BuskerCatalog     `json:"busker,omitempty"`
	CalendarEncounter *CalendarEncounter `json:"calendar_encounter,omitempty"`
	// Conversation like Disco Elysium (owner 20.11, world.dialogue_rules): the chance table of skill checks, the colours
	// and names of skills and castes in the log (dialogue.go); nil without it.
	DialogueRules *DialogueRules `json:"dialogue_rules,omitempty"`
	// Story memory (doc 103 step 1, world.memory and world.conversations; memory.go): facts, topics, interjections, who
	// opens which conversation, and every conversation the one server executor runs. nil without them.
	Memory        *Memory                  `json:"memory,omitempty"`
	Conversations map[string]*Conversation `json:"conversations,omitempty"`
	memoryLinks   *memoryLinks
	linksPage     []byte
	// The hero's personal file before the lineup (doc 104 v2, world.hero_file, hero_file.go): profiles, strengths, past,
	// traits and the words of the screen; the server builds the hero's start from the new game's choice. nil without it.
	HeroFile *HeroFile `json:"hero_file,omitempty"`
	// Craft places of the hub islands (demo-hub v16-goods, world.market_goods); nil in worlds without them.
	Craft *CraftCatalog `json:"craft,omitempty"`
	// The sound of the hall (demo-hub v26-alive, world.soundscape); nil in worlds without it.
	Soundscape *SoundCatalog `json:"soundscape,omitempty"`
	// What a seller does at his post in the closing and opening phases (demo-hub v26-alive, world.hall_closing).
	HallClosing *HallClosing `json:"hall_closing,omitempty"`
	// The noise waves (combat-play/noise-waves, world.noise_waves); nil in worlds without them.
	NoiseWaves *NoiseWaveCatalog `json:"noise_waves,omitempty"`
	// Camera cutaway (camera_cutaway.go): occluder kinds per level and the cut's numbers, presentation only.
	CameraCutaway *CameraCutaway `json:"-"`
	// Edge fog (edge_fog.go): every level fades into black at the edge of the hero's reach, presentation only.
	EdgeFog *EdgeFog `json:"-"`
	// Platform beacons (platform_beacons.go): flashing posts at an open platform edge before a train arrives, presentation only.
	PlatformBeacons *PlatformBeacons `json:"-"`
	// Foot contact (foot_contact.go): the client's step that keeps soles on the floor, presentation only.
	FootContact *FootContact `json:"-"`
	// Cloth motion (cloth_motion.go): strip curtains people push and drapes a named draft sways, presentation only.
	ClothMotion *ClothMotion `json:"-"`
	// People read (people_read.go): per level, the surroundings step back and the people step forward, presentation only.
	PeopleRead *PeopleRead `json:"-"`
	// Painted location (painted_location.go): a corner's painted albedo projected back from the game camera, presentation only.
	PaintedLocation *PaintedLocation `json:"-"`
	// Items display v1 (item_display.go): the look of every item in every context, presentation only; nil without it.
	ItemDisplay *ItemDisplayCatalog `json:"-"`
	// The tactical view (owner 04.11, world.tactical_mode): its label and switch, common to all it will show.
	TacticalMode *TacticalMode `json:"tactical_mode,omitempty"`
	// Combat pace (combat_pace.go, owner 20.11): how the client plays one turn-based fight, turn-based or fast
	// turn-based; presentation only, never in the server's world, so no pace can change an outcome.
	CombatPace *CombatPace `json:"-"`
	// Crafting stage 1 (doc 93, world.crafting): the engine catalog completed from the compiled world; nil without it.
	Crafting *craftingRuntime.Model `json:"crafting,omitempty"`
	// Barks (doc 94, world.barks): the compiled catalog of the lines over the head (internal/barks/catalog.json); a world
	// without barks gets a silent default so the host adapter always compiles.
	Barks *barksRuntime.Model `json:"-"`
	// BarksView (barks v2): how the client lays a line on the screen; a default when the world has none.
	BarksView *BarksView `json:"-"`
	// demo-hub v26-alive: how the client shows an exchange
	BarksTalk     *BarksTalkView `json:"-"`
	barksIn       *barksInput
	barksWarnings []string
	barksReport   []byte
	// Wounds instead of hit points (doc 97, world.wounds): the standalone engine internal/wounds and the timeline table
	// design/wounds/timelines.md; nil without world.wounds. Not called by combat yet (design/wounds/codex-handoff.md).
	Wounds       *woundsRuntime.Model `json:"-"`
	woundsIn     *woundsInput
	woundsReport []byte

	// People placement v1: the own floors of fixtures' insides by level (fixtures.go), only for the Godot location catalog.
	FixtureFloors map[string][]FixtureFloor `json:"-"`

	// What a save depends on (save_key.go, owner 20.11): the policy from GamePOC.save_key; the computed key goes into
	// world.json as its last section when the server package is written.
	SaveKeyPolicy *SaveKeyPolicy `json:"-"`
}

func (c Config) validate() error {
	seenHashes := map[string]bool{}
	for _, hash := range c.SaveCompatibleHashes {
		raw, err := hex.DecodeString(hash)
		if err != nil || len(raw) != sha256.Size || strings.ToLower(hash) != hash || seenHashes[hash] {
			return fmt.Errorf("invalid save-compatible catalog hash")
		}
		seenHashes[hash] = true
	}

	if err := c.Rendering.validate(); err != nil {
		return err
	}
	if err := c.CaptureDemo.validate(); err != nil {
		return err
	}
	if c.TickHz < 1 || c.TickHz > 60 || c.MoveEveryTicks < 1 || c.GameSecondsPerTick < 1 || c.QueueCapacity < 8 || c.QueueCapacity > 1024 || c.MaxCatchupSteps < 1 || c.MaxCatchupSteps > 16 || c.SaveVersion != 1 || c.MaxSaveBytes < 1024 || c.MaxSaveBytes > 16<<20 {
		return fmt.Errorf("invalid or unsupported GamePOC timing, queue or save policy")
	}
	if c.SchedulerHz < c.TickHz || c.SchedulerHz > 1000 || c.SchedulerHz*c.MaxCatchupSteps < c.TickHz*10 {
		return fmt.Errorf("scheduler cannot sustain maximum world speed")
	}
	if c.InputAckBudgetMS < 1 || c.InputAckBudgetMS > 100 || c.MinClientFPS < 1 || c.MinClientFPS > 240 || c.PresentationLeadMS < 0 || c.PresentationLeadMS > c.InputAckBudgetMS || c.PresentationHorizonMS < 100 || c.PresentationHorizonMS > 500 {
		return fmt.Errorf("invalid GamePOC input or presentation policy")
	}
	if c.LevelID == "" || c.WorldDir == "" || c.UnityDir == "" || c.Title == "" || len(c.SaveSlots) == 0 {
		return fmt.Errorf("GamePOC requires world_dir, unity_dir, level_id, title and save_slots")
	}
	if err := c.validateSaveKinds(); err != nil {
		return err
	}
	for _, key := range []string{"new_game", "load_game", "save_game", "options", "continue_game", "main_menu", "quit"} {
		if c.Labels[key] == "" {
			return fmt.Errorf("missing label %s", key)
		}
	}
	return nil
}

func cuePackage(dir string) (cue.Value, error) {
	v := cuecontext.New().BuildInstance(load.Instances([]string{"."}, &load.Config{Dir: dir})[0])
	if err := v.Err(); err != nil {
		return v, fmt.Errorf("CUE %s: %s", dir, errors.Details(err, nil))
	}
	return v, nil
}

func readCompiled(project string) (Compiled, *normalizer.GRPCDef, error) {
	var out Compiled
	v, err := cuePackage(filepath.Join(project, "cue/poc"))
	if err != nil {
		return out, nil, err
	}
	cv := v.LookupPath(cue.ParsePath("GamePOC"))
	data, err := cv.MarshalJSON()
	if err != nil {
		return out, nil, err
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&out.Config); err != nil {
		return out, nil, err
	}
	out.GodotTargets = out.Config.GodotTargets
	out.Config.GodotTargets = nil
	out.SaveKeyPolicy, out.Config.SaveKey = out.Config.SaveKey, nil
	if err := out.SaveKeyPolicy.validate(); err != nil {
		return out, nil, err
	}
	out.Config.Rendering = out.Config.Rendering.normalized()
	if out.Config.CaptureDemo.TargetActor == "" {
		out.Config.CaptureDemo.TargetActor = "char.hero"
	}
	if err := out.Config.validate(); err != nil {
		return out, nil, err
	}
	phases.mark("read: poc config CUE")
	w, err := cuePackage(filepath.Join(project, out.Config.WorldDir))
	if err != nil {
		return out, nil, err
	}
	phases.mark("read: world CUE load+build")
	world := w.LookupPath(cue.ParsePath("world"))
	if scenes := world.LookupPath(cue.ParsePath("godot_scenes")); scenes.Exists() {
		if err := scenes.Decode(&out.GodotScenes); err != nil {
			return out, nil, err
		}
	}
	if err := world.Validate(cue.Concrete(true)); err != nil {
		return out, nil, fmt.Errorf("world: %s", errors.Details(err, nil))
	}
	phases.mark("read: world CUE validate")
	if err := world.LookupPath(cue.ParsePath("visibility")).Decode(&out.Visibility); err != nil {
		return out, nil, err
	}
	if out.Visibility.PlayerRange < 1 || out.Visibility.PlayerRange > 64 || out.Visibility.Field != "omnidirectional" || out.Visibility.Remember != "observed_geometry_only" || out.Visibility.UnknownGlyph != "?" {
		return out, nil, fmt.Errorf("invalid visibility policy")
	}
	if combat := world.LookupPath(cue.ParsePath("combat_runtime")); combat.Exists() {
		out.CombatRuntime, err = combat.MarshalJSON()
		if err != nil {
			return out, nil, err
		}
	}
	if combat := world.LookupPath(cue.ParsePath("combat_life")); combat.Exists() {
		out.CombatLife, err = combat.MarshalJSON()
		if err != nil {
			return out, nil, err
		}
	}

	if spacing := world.LookupPath(cue.ParsePath("combat_spacing")); spacing.Exists() {
		out.CombatSpacing, err = spacing.MarshalJSON()
		if err != nil {
			return out, nil, err
		}
	}
	if scenes := world.LookupPath(cue.ParsePath("playable_scenes")); scenes.Exists() {
		out.PlayableScenes, err = scenes.MarshalJSON()
		if err != nil {
			return out, nil, err
		}
	}
	if light := world.LookupPath(cue.ParsePath("illumination_runtime")); light.Exists() {
		out.Illumination, err = light.MarshalJSON()
		if err != nil {
			return out, nil, err
		}
	}
	if stealth := world.LookupPath(cue.ParsePath("stealth_runtime")); stealth.Exists() {
		out.Stealth, err = stealth.MarshalJSON()
		if err != nil {
			return out, nil, err
		}
	}

	if life := world.LookupPath(cue.ParsePath("life")); life.Exists() {
		out.Life, err = life.MarshalJSON()
		if err != nil {
			return out, nil, err
		}
	}
	if sdk := world.LookupPath(cue.ParsePath("location_sdk")); sdk.Exists() {
		var input locationsdk.Input
		raw, decodeErr := sdk.MarshalJSON()
		if decodeErr != nil {
			return out, nil, fmt.Errorf("location SDK: %w", decodeErr)
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			return out, nil, fmt.Errorf("location SDK: %w", err)
		}
		cadence := struct {
			MoveTicks int64                      `json:"move_ticks"`
			Locations map[string]json.RawMessage `json:"locations"`
		}{MoveTicks: out.Config.MoveEveryTicks}
		if len(out.Life) > 0 {
			if err := json.Unmarshal(out.Life, &cadence); err != nil {
				return out, nil, err
			}
		}
		bundle, err := locationsdk.CompileRuntime(input, locationsdk.RuntimeOptions{TickHz: out.Config.TickHz, MoveEveryTicks: cadence.MoveTicks})
		if err != nil {
			return out, nil, fmt.Errorf("location SDK: %w", err)
		}
		for _, level := range bundle.Levels {
			if _, exists := cadence.Locations[level.ID]; exists || level.ID == out.Config.LevelID {
				return out, nil, fmt.Errorf("location SDK level %q conflicts with a story location", level.ID)
			}
		}
		out.LocationSDK = &bundle
	}
	out.Life, out.ModularLocations, err = compileModularLocations(out.Life, out.LocationSDK)
	if err != nil {
		return out, nil, err
	}
	if region := world.LookupPath(cue.ParsePath("region_map")); region.Exists() {
		if err := region.Decode(&out.Region); err != nil {
			return out, nil, err
		}
		if err := out.Region.validate(out.Life); err != nil {
			return out, nil, err
		}
	}
	if ways := world.LookupPath(cue.ParsePath("wayfinding")); ways.Exists() {
		if err := ways.Decode(&out.Wayfinding); err != nil {
			return out, nil, err
		}
	}
	if out.Vending, err = readVending(world); err != nil {
		return out, nil, err
	}
	if out.CalendarEncounter, err = readCalendarEncounter(world); err != nil {
		return out, nil, err
	}
	if out.DialogueRules, err = readDialogueRules(world); err != nil {
		return out, nil, err
	}
	if out.Memory, out.Conversations, err = readMemory(world); err != nil {
		return out, nil, err
	}
	if out.HeroFile, err = readHeroFile(world); err != nil {
		return out, nil, err
	}
	if err = rejectUnrunDialogueFields(world); err != nil {
		return out, nil, err
	}
	if out.Peddlers, err = readPeddlers(world); err != nil {
		return out, nil, err
	}
	if out.Visitors, err = readVisitors(world); err != nil {
		return out, nil, err
	}
	if out.Busker, err = readBusker(world); err != nil {
		return out, nil, err
	}
	if out.Soundscape, err = readSoundscape(world); err != nil {
		return out, nil, err
	}
	if out.HallClosing, err = readHallClosing(world); err != nil {
		return out, nil, err
	}
	if out.LiftingDecks, err = readLiftingDecks(world, out.Life); err != nil {
		return out, nil, err
	}
	if out.Life, err = filterLiftingDeckPopulationAnchors(out.Life, out.LiftingDecks); err != nil {
		return out, nil, err
	}
	if out.TurretWatch, err = readTurretWatch(world, out.Life, out.CombatRuntime); err != nil {
		return out, nil, err
	}
	if out.AccessZones, err = readAccessZones(w, world, out); err != nil {
		return out, nil, err
	}
	if out.CameraCutaway, err = readCameraCutaway(world, out); err != nil {
		return out, nil, err
	}
	if out.FootContact, err = readFootContact(world); err != nil {
		return out, nil, err
	}
	if out.ClothMotion, err = readClothMotion(world, out); err != nil {
		return out, nil, err
	}
	if out.PeopleRead, err = readPeopleRead(world, out); err != nil {
		return out, nil, err
	}
	if out.PaintedLocation, err = readPaintedLocation(world, out, filepath.Dir(project)); err != nil {
		return out, nil, err
	}
	if out.TrainMotion, err = readTrainMotion(world, out.Life); err != nil {
		return out, nil, err
	}
	if out.NoiseWaves, err = readNoiseWaves(world); err != nil {
		return out, nil, err
	}
	if out.TacticalMode, err = readTacticalMode(world); err != nil {
		return out, nil, err
	}
	if out.CombatPace, err = readCombatPace(world); err != nil {
		return out, nil, err
	}
	if out.Hud, err = readChipHud(world); err != nil {
		return out, nil, err
	}
	if out.Craft, err = readCraft(world); err != nil {
		return out, nil, err
	}
	// Items display v1 (item_display.go): one look per item, checked against the build record and the places that draw items.
	if len(out.GodotTargets) > 0 {
		if out.ItemDisplay, err = readItemDisplay(world, filepath.Dir(project), filepath.Join(project, out.GodotTargets[0]), out.Config.CaptureDemo.TargetActor, out.Vending, out.Craft); err != nil {
			return out, nil, err
		}
	}
	craftingIn, err := readCrafting(world)
	if err != nil {
		return out, nil, err
	}
	if out.barksIn, err = readBarks(world); err != nil {
		return out, nil, err
	}
	if out.woundsIn, err = readWounds(world); err != nil {
		return out, nil, err
	}
	if text := world.LookupPath(cue.ParsePath("world_text")); text.Exists() {
		raw, err := text.MarshalJSON()
		if err != nil {
			return out, nil, err
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&out.WorldText); err != nil {
			return out, nil, fmt.Errorf("world_text: %w", err)
		}
	}
	if theme := world.LookupPath(cue.ParsePath("godot_theme")); theme.Exists() {
		if err := theme.Decode(&out.GodotTheme); err != nil {
			return out, nil, err
		}
	}
	if settings := world.LookupPath(cue.ParsePath("godot_settings")); settings.Exists() {
		if err := settings.Decode(&out.GodotSettings); err != nil {
			return out, nil, fmt.Errorf("godot_settings: %w", err)
		}
	}
	if screens := world.LookupPath(cue.ParsePath("godot_screens")); screens.Exists() {
		if err := screens.Decode(&out.GodotScreens); err != nil {
			return out, nil, fmt.Errorf("godot_screens: %w", err)
		}
	}
	if localization := world.LookupPath(cue.ParsePath("localization")); localization.Exists() {
		if err := localization.Decode(&out.Localization); err != nil {
			return out, nil, err
		}
		if err := out.Localization.Validate(); err != nil {
			return out, nil, err
		}
	}
	out.AI, err = world.LookupPath(cue.ParsePath("ai")).MarshalJSON()
	if err != nil {
		return out, nil, err
	}
	if err := behavior.ValidateCatalog(out.AI, out.Config.CaptureDemo.MissionRoot); err != nil {
		return out, nil, err
	}
	var roleCatalog struct {
		Roles map[string]json.RawMessage `json:"roles"`
	}
	if err := json.Unmarshal(out.AI, &roleCatalog); err != nil {
		return out, nil, err
	}
	for _, id := range []string{out.Config.CaptureDemo.ResponderRole, out.Config.CaptureDemo.ResidentRole} {
		if _, ok := roleCatalog.Roles[id]; !ok {
			return out, nil, fmt.Errorf("unknown CUE role %s", id)
		}
	}
	if err := validateAIFixtureFacts(out.AI, out.Config.CaptureDemo.KnownFacts); err != nil {
		return out, nil, err
	}
	var hearingCatalog struct {
		Policy struct {
			Perception struct {
				Hearing struct {
					Sounds map[string]json.RawMessage `json:"sounds"`
				} `json:"hearing"`
			} `json:"perception"`
			Danger struct {
				Levels []json.RawMessage `json:"levels"`
			} `json:"danger"`
		} `json:"policy"`
	}
	if err := json.Unmarshal(out.AI, &hearingCatalog); err != nil {
		return out, nil, err
	}
	if _, ok := hearingCatalog.Policy.Perception.Hearing.Sounds[out.Config.CaptureDemo.HeroAction.Sound]; !ok {
		return out, nil, fmt.Errorf("hero_action.sound %q is not a declared sound kind", out.Config.CaptureDemo.HeroAction.Sound)
	}
	if levels := len(hearingCatalog.Policy.Danger.Levels); out.Config.CaptureDemo.DangerLevel >= levels || out.Config.CaptureDemo.HeroAction.WitnessLevel >= levels {
		return out, nil, fmt.Errorf("capture demo danger levels exceed policy.danger.levels")
	}
	psyche := psycheCompiler{root: w, world: world}
	aiValue := world.LookupPath(cue.ParsePath("ai"))
	if errs := behavior.ValidateUtility(out.AI, psyche.vocabulary()); len(errs) > 0 {
		return out, nil, behavior.JoinErrors(errs, func(path string) string { return catalogPosition(aiValue, path) })
	}
	var profileCatalog struct {
		Profiles map[string]struct {
			ForbidActions []string `json:"forbid_actions"`
		} `json:"profiles"`
	}
	if err := json.Unmarshal(out.AI, &profileCatalog); err != nil {
		return out, nil, err
	}
	for _, id := range []string{out.Config.CaptureDemo.ResponderProfile, out.Config.CaptureDemo.ResidentProfile} {
		if _, ok := profileCatalog.Profiles[id]; !ok {
			return out, nil, fmt.Errorf("unknown CUE profile %s", id)
		}
	}
	if out.Psyche.Law, err = psyche.law(); err != nil {
		return out, nil, err
	}
	out.Psyche.Actors = map[string]psycheData{}
	for _, a := range out.Config.CaptureDemo.Agents {
		if out.Psyche.Actors[a.ID], err = psyche.actorPsyche("", cue.Value{}, cue.Value{}, a.Vectors, profileCatalog.Profiles[out.Config.CaptureDemo.ResponderProfile].ForbidActions); err != nil {
			return out, nil, fmt.Errorf("demo agent %s: %w", a.ID, err)
		}
	}
	phases.mark("read: world decode (to AI)")
	cases, err := cuePackage(filepath.Join(project, out.Config.WorldDir, "ai/scenarios"))
	if err != nil {
		return out, nil, err
	}
	out.AIScenarios, err = cases.MarshalJSON()
	if err != nil {
		return out, nil, err
	}
	if out.AIUtility, err = psyche.utilityFixtures(cases, out.Psyche.Law); err != nil {
		return out, nil, err
	}
	phases.mark("read: AI scenarios CUE")
	wb, err := world.MarshalJSON()
	if err != nil {
		return out, nil, err
	}
	wb, err = canonicalJSONBytes(wb)
	if err != nil {
		return out, nil, err
	}
	data, err = canonicalJSONBytes(data)
	if err != nil {
		return out, nil, err
	}
	hash := sha256.New()
	hash.Write(wb)
	hash.Write([]byte{0})
	hash.Write(data)
	out.ContentHash = hex.EncodeToString(hash.Sum(nil))
	phases.mark("read: content hash (world JSON)")
	level := world.LookupPath(cue.MakePath(cue.Str("levels"), cue.Str(out.Config.LevelID)))
	if err := level.LookupPath(cue.ParsePath("layers.terrain")).Decode(&out.Terrain); err != nil {
		return out, nil, err
	}
	if err := level.LookupPath(cue.ParsePath("entries.from_corridor.at")).Decode(&out.Spawn); err != nil {
		return out, nil, err
	}
	if len(out.Terrain) == 0 || len(out.Terrain[0]) == 0 {
		return out, nil, fmt.Errorf("empty level")
	}
	for _, row := range out.Terrain {
		if len(row) != len(out.Terrain[0]) {
			return out, nil, fmt.Errorf("unequal terrain rows")
		}
	}
	out.Opaque = map[string]bool{}
	out.Walkable = map[string]bool{}
	out.MoveCost = map[string]int{}
	out.TileSprites = map[string]string{}
	out.Scene.TileNoise = map[string]int{}
	if noise, err := level.LookupPath(cue.ParsePath("ambient.noise_default")).Int64(); err == nil {
		out.Scene.AmbientNoise = int(noise)
	}
	var cameras []struct {
		ID    string `json:"id"`
		At    Point  `json:"at"`
		Range int    `json:"range_cells"`
		State string `json:"state"`
	}
	if effective := level.LookupPath(cue.ParsePath("effective_cameras")); effective.Exists() {
		if err := effective.Decode(&cameras); err != nil {
			return out, nil, fmt.Errorf("level cameras: %w", err)
		}
	}
	for _, c := range cameras {
		out.Scene.Cameras = append(out.Scene.Cameras, Camera{ID: c.ID, Position: c.At, Range: c.Range, Online: c.State == "online"})
	}
	legendID, _ := level.LookupPath(cue.ParsePath("legend")).String()
	tiles, err := world.LookupPath(cue.MakePath(cue.Str("legends"), cue.Str(legendID), cue.Str("tiles"))).Fields()
	if err != nil {
		return out, nil, err
	}
	for tiles.Next() {
		key := tiles.Selector().Unquoted()
		out.Opaque[key], _ = tiles.Value().LookupPath(cue.ParsePath("opaque")).Bool()
		out.Walkable[key], _ = tiles.Value().LookupPath(cue.ParsePath("walk")).Bool()
		cost, _ := tiles.Value().LookupPath(cue.ParsePath("move_cost")).Int64()
		out.MoveCost[key] = int(cost)
		out.TileSprites[key], _ = tiles.Value().LookupPath(cue.ParsePath("sprite")).String()
		if noise, err := tiles.Value().LookupPath(cue.ParsePath("noise")).Int64(); err == nil && noise > 0 {
			out.Scene.TileNoise[key] = int(noise)
		}
	}
	if out.EdgeFog, err = readEdgeFog(world, out); err != nil {
		return out, nil, err
	}
	if out.PlatformBeacons, err = readPlatformBeacons(world, out); err != nil {
		return out, nil, err
	}
	if out.Spatial, err = compileSpatial(world.LookupPath(cue.ParsePath("spatial")), out.Walkable); err != nil {
		return out, nil, fmt.Errorf("world.spatial: %w", err)
	}
	if garments := world.LookupPath(cue.ParsePath("clothing")); garments.Exists() {
		if err := garments.Decode(&out.Clothing); err != nil {
			return out, nil, fmt.Errorf("world.clothing: %w", err)
		}
	}
	if err := out.Clothing.Validate(); err != nil {
		return out, nil, fmt.Errorf("world.clothing: %w", err)
	}
	if art := world.LookupPath(cue.ParsePath("art")); art.Exists() {
		if err := art.Decode(&out.Art); err != nil {
			return out, nil, fmt.Errorf("world.art: %w", err)
		}
	}
	var trainViews TrainMotionCatalog
	if out.TrainMotion != nil {
		trainViews = *out.TrainMotion
	}
	if err := trainViews.validatePainted(out.Art.Painted); err != nil {
		return out, nil, err
	}
	if err := checkArtScenes(out.Art.Scenes); err != nil {
		return out, nil, err
	}
	if anchors := world.LookupPath(cue.ParsePath("painted_inventory_anchors")); anchors.Exists() {
		if err := anchors.Decode(&out.PaintedInventoryAnchors); err != nil {
			return out, nil, fmt.Errorf("world.painted_inventory_anchors: %w", err)
		}
	}
	if out.ExitZones, err = readExitZones(world, out); err != nil {
		return out, nil, err
	}
	if err := out.Wayfinding.validate(out.Life, out.Walkable, out.Localization, out.PaintedInventoryAnchors); err != nil {
		return out, nil, err
	}
	if err := checkPresentationCamera(out.Art.Camera); err != nil {
		return out, nil, err
	}
	if out.Biomonitor, err = readBiomonitor(world); err != nil {
		return out, nil, err
	}
	if err := checkLooks(filepath.Dir(project), out.Art, func(id string) bool {
		return world.LookupPath(cue.MakePath(cue.Str("persons"), cue.Str(id))).Exists()
	}, out.TurretWatch, out.Biomonitor); err != nil {
		return out, nil, err
	}
	if err := checkPopulationLooks(out.Art, out.Visitors); err != nil {
		return out, nil, err
	}
	ww, err := readWardrobeWorld(world, out.Clothing.Garments)
	if err != nil {
		return out, nil, err
	}
	if err := checkWardrobe(filepath.Dir(project), out.Art, func(id string) bool {
		return world.LookupPath(cue.MakePath(cue.Str("persons"), cue.Str(id))).Exists()
	}, ww); err != nil {
		return out, nil, err
	}
	if err := checkPersonPopulations(out.Art, func(id string) bool {
		return world.LookupPath(cue.MakePath(cue.Str("persons"), cue.Str(id))).Exists()
	}); err != nil {
		return out, nil, err
	}
	if err := checkPaintTest(out.Art, func(id string) bool {
		return world.LookupPath(cue.MakePath(cue.Str("persons"), cue.Str(id))).Exists()
	}); err != nil {
		return out, nil, err
	}
	if cards := world.LookupPath(cue.ParsePath("character_cards")); cards.Exists() {
		var decoded map[string]CharacterCard
		if err := cards.Decode(&decoded); err != nil {
			return out, nil, fmt.Errorf("world.character_cards: %w", err)
		}
		if err := checkCharacterCards(filepath.Dir(project), decoded, out.Art, func(id string) bool {
			return world.LookupPath(cue.MakePath(cue.Str("persons"), cue.Str(id))).Exists()
		}); err != nil {
			return out, nil, err
		}
	}
	if out.Biomonitor != nil {
		out.Biomonitor.Wearers = biomonitorWearers(out.Art)
	}
	if loading := world.LookupPath(cue.ParsePath("loading")); loading.Exists() {
		if err := loading.Decode(&out.Loading); err != nil {
			return out, nil, fmt.Errorf("world.loading: %w", err)
		}
	}
	if menu := world.LookupPath(cue.ParsePath("menu")); menu.Exists() {
		if err := menu.Decode(&out.Menu); err != nil {
			return out, nil, fmt.Errorf("world.menu: %w", err)
		}
	}
	for glyph, sprite := range out.TileSprites {
		if _, ok := out.Art.Sprites[sprite]; !ok {
			return out, nil, fmt.Errorf("legend tile %q: sprite %q has no recipe in world.art.sprites", glyph, sprite)
		}
	}
	sites := level.LookupPath(cue.ParsePath("sites"))
	persons, err := world.LookupPath(cue.ParsePath("persons")).Fields()
	if err != nil {
		return out, nil, err
	}
	for persons.Next() {
		p := persons.Value()
		post, _ := p.LookupPath(cue.ParsePath("post")).String()
		if post == "" {
			continue
		}
		site := sites.LookupPath(cue.MakePath(cue.Str(post)))
		if !site.Exists() {
			continue
		}
		var e Entity
		e.ID, _ = p.LookupPath(cue.ParsePath("id")).String()
		e.Name, _ = p.LookupPath(cue.ParsePath("short_name")).String()
		if e.Name == "" {
			e.Name, _ = p.LookupPath(cue.ParsePath("name")).String()
		}
		if err := site.LookupPath(cue.ParsePath("anchor")).Decode(&e.Position); err != nil {
			return out, nil, err
		}
		arch, _ := p.LookupPath(cue.ParsePath("archetype")).String()
		e.Faction, _ = world.LookupPath(cue.MakePath(cue.Str("archetypes"), cue.Str(arch), cue.Str("faction"))).String()
		vectors := world.LookupPath(cue.MakePath(cue.Str("archetypes"), cue.Str(arch), cue.Str("vectors")))
		if override := p.LookupPath(cue.ParsePath("overrides.vectors")); override.Exists() {
			vectors = override
		}
		if err := vectors.Decode(&e.Vectors); err != nil {
			return out, nil, fmt.Errorf("person %s vectors: %w", e.ID, err)
		}
		if out.Psyche.Actors[e.ID], err = psyche.actorPsyche(arch, p.LookupPath(cue.ParsePath("overrides")), p.LookupPath(cue.ParsePath("relations")), e.Vectors, profileCatalog.Profiles[out.Config.CaptureDemo.ResidentProfile].ForbidActions); err != nil {
			return out, nil, fmt.Errorf("person %s: %w", e.ID, err)
		}
		out.Entities = append(out.Entities, e)
	}
	if operator := out.Config.CaptureDemo.Monitoring.Operator; operator != "" {
		found := false
		for _, e := range out.Entities {
			if e.ID == operator {
				found = true
			}
		}
		if !found {
			return out, nil, fmt.Errorf("monitoring.operator %s is not a scene resident", operator)
		}
	}
	for i := range out.Entities {
		e := &out.Entities[i]
		if at, ok := out.Config.CaptureDemo.ResidentPositions[e.ID]; ok {
			e.Position = at
		}
		p := e.Position
		if p.X < 0 || p.Y < 0 || p.Y >= len(out.Terrain) || p.X >= len(out.Terrain[0]) || !out.Walkable[string(out.Terrain[p.Y][p.X])] {
			return out, nil, fmt.Errorf("resident %s requires a walkable CUE standing position", e.ID)
		}
	}
	if out.Inventory, err = inventoryModel(world, level, out.Config.LevelID, out.Life); err != nil {
		return out, nil, err
	}
	if out.SanitaryServices, err = readSanitaryServices(world, out); err != nil {
		return out, nil, err
	}
	if out.CartParking, err = readCartParking(world, out); err != nil {
		return out, nil, err
	}
	if out.CargoConvoys, err = readCargoConvoys(world, out); err != nil {
		return out, nil, err
	}
	if out.MedicalCapsules, err = readMedicalCapsules(world, out); err != nil {
		return out, nil, err
	}
	if err := validateCommerce(out); err != nil {
		return out, nil, err
	}
	if err := validateVending(out); err != nil {
		return out, nil, err
	}
	if err := validateCraft(out); err != nil {
		return out, nil, err
	}
	// Painted market v1: a level painting is checked once every system it must leave live is compiled.
	if err := checkPaintedLocationLevels(out, filepath.Dir(project)); err != nil {
		return out, nil, err
	}
	if err := applyPeddlers(&out); err != nil {
		return out, nil, err
	}
	if err := applyVisitors(&out); err != nil {
		return out, nil, err
	}
	if err := applyBusker(&out); err != nil {
		return out, nil, err
	}
	if err := applyCargoLots(&out); err != nil {
		return out, nil, err
	}
	fixtures, err := readFixtures(world)
	if err != nil {
		return out, nil, err
	}
	if err := applyFixtures(&out, fixtures); err != nil {
		return out, nil, err
	}
	if err := applyCrafting(&out, craftingIn); err != nil {
		return out, nil, err
	}
	if err := out.DialogueRules.check(&out); err != nil {
		return out, nil, err
	}
	if err := out.HeroFile.check(&out); err != nil {
		return out, nil, err
	}
	if err := checkCalendarEncounter(&out); err != nil {
		return out, nil, err
	}
	if err := checkPeddlerClasses(&out); err != nil {
		return out, nil, err
	}
	if err := validateTransitFares(out); err != nil {
		return out, nil, err
	}
	if err := checkPaintedScenes(out); err != nil {
		return out, nil, err
	}
	// Demonstration actors use the same inventory registry as canonical scene actors.
	for _, a := range out.Config.CaptureDemo.Agents {
		if out.Inventory.Actors[a.ID] {
			return out, nil, fmt.Errorf("duplicate actor %s", a.ID)
		}
		out.Inventory.Actors[a.ID] = true
		out.Inventory.Owners[a.ID] = true
		sid := inventoryRuntime.ActorStore(a.ID)
		capacity := out.Inventory.Stores[inventoryRuntime.ActorStore("char.hero")].Capacity
		out.Inventory.Stores[sid] = inventoryRuntime.Store{ID: sid, Name: a.Name, Kind: "actor", Anchor: inventoryRuntime.Anchor{Kind: "actor", Actor: a.ID}, Owner: a.ID, Capacity: capacity}
	}
	target := out.Config.CaptureDemo.TargetActor
	if target != "char.hero" {
		found := false
		for _, a := range out.Entities {
			if a.ID == target {
				found = true
			}
		}
		for _, a := range out.Config.CaptureDemo.Agents {
			if a.ID == target {
				found = true
			}
		}
		if !found {
			return out, nil, fmt.Errorf("mission target %s is not on the scene", target)
		}
	}
	if err := out.validateDemoTerrain(); err != nil {
		return out, nil, err
	}
	sort.Slice(out.Entities, func(i, j int) bool { return out.Entities[i].ID < out.Entities[j].ID })
	phases.mark("read: world decode and checks")
	api, err := cuePackage(filepath.Join(project, "cue/api"))
	if err != nil {
		return out, nil, err
	}
	def, err := normalizer.New().ExtractGRPC(api)
	if err != nil {
		return out, nil, err
	}
	if def == nil || def.Package != "game.v1" || def.CSharpNamespace != "AngelsDemons.Game.V1" {
		return out, nil, fmt.Errorf("POC currently requires game.v1 / AngelsDemons.Game.V1")
	}
	for _, name := range []string{"PocGameState", "PocSaveRequest", "PocLoadRequest", "PocDeleteSaveRequest", "PocNewGameRequest", "PocSaveSlots"} {
		found := false
		for _, m := range def.Messages {
			if m.Name == name {
				found = true
			}
		}
		if !found {
			return out, nil, fmt.Errorf("missing CUE message %s", name)
		}
	}
	return out, def, nil
}

// Generate writes only the explicitly owned POC artifacts and the shared GRPC
// outputs using ANG's existing normalizer/emitters. Other project files are untouched.
func Generate(project string) error {
	return generatePOC(project, true)
}

// GenerateActive writes the server and Godot client, without rendering retired Unity outputs.
func GenerateActive(project string) error {
	return generatePOC(project, false)
}

func generatePOC(project string, legacyUnity bool) error {
	project, err := filepath.Abs(project)
	if err != nil {
		return err
	}
	phases = startPhaseClock()
	defer func() { phases = nil }()
	if _, err := schemavendor.Sync(project, schemavendor.Options{Always: true}); err != nil {
		return err
	}
	phases.mark("schema vendor sync")
	compiled, def, err := readCompiled(project)
	if err != nil {
		return err
	}
	phases.mark("read: api CUE, normalize")
	compiled.ViewDelivery = def.ViewSubscription != ""
	compiled.WebSocket = def.WebSocket
	if err := writeGodotScreens(project, compiled, def); err != nil {
		return err
	}
	phases.mark("emit: godot screens")
	unity := filepath.Clean(filepath.Join(project, compiled.Config.UnityDir))
	worldJSON, err := json.MarshalIndent(compiled, "", "  ")
	if err != nil {
		return err
	}
	phases.mark("emit: world.json marshal")
	compiled.WorldTextDrawn, err = compileWorldText(project, compiled)
	if err != nil {
		return err
	}
	phases.mark("emit: world text")
	if err := compileBarks(project, &compiled); err != nil {
		return err
	}
	phases.mark("emit: barks")
	// story memory: the links of facts, topics and conversations (castes and bark lines come from the barks)
	if err := checkMemory(&compiled); err != nil {
		return err
	}
	if err := compileWounds(&compiled); err != nil {
		return err
	}
	if compiled.Barks == nil {
		compiled.Barks = silentBarks(compiled)
	}
	if compiled.BarksTalk == nil {
		compiled.BarksTalk = defaultBarksTalk()
	}
	if err := validateSoundscape(compiled, unity); err != nil {
		return err
	}
	if err := validateTacticalMode(compiled); err != nil {
		return err
	}
	if err := validateCombatPace(compiled); err != nil {
		return err
	}
	if err := validateChipHud(compiled); err != nil {
		return err
	}
	if compiled.BarksView == nil {
		compiled.BarksView = defaultBarksView()
	}
	phases.mark("emit: wounds, checks")
	if err := writeGodotLocations(project, compiled); err != nil {
		return err
	}
	phases.mark("emit: godot locations")
	if err := writeGodotTheme(project, compiled.GodotTheme); err != nil {
		return err
	}
	if err := writeGodotTranslations(project, compiled.GodotTargets, compiled.Localization); err != nil {
		return err
	}
	if err := writeGodotWorldText(project, compiled); err != nil {
		return err
	}
	phases.mark("emit: godot theme, translations, world text")
	viewJSON, err := clientResource(compiled)
	if err != nil {
		return err
	}
	phases.mark("emit: presentation.json")
	if compiled.LocationSDK != nil {
		if err := checkLocationSDKSurfaceImages(unity, *compiled.LocationSDK); err != nil {
			return err
		}
	}
	files := map[string][]byte{
		filepath.Join(project, "internal/poc/world.json"):       worldJSON,
		filepath.Join(unity, "Assets/Resources/PocConfig.json"): viewJSON,
	}
	for _, target := range compiled.GodotTargets {
		files[filepath.Join(project, target, "generated/presentation.json")] = viewJSON
	}
	if compiled.CalendarEncounter != nil {
		files[filepath.Join(unity, "Assets/Resources/Generated/CalendarEncounterPresentation.json")] = compiled.CalendarEncounter.Presentation
	}
	if err := checkArtSceneImages(unity, compiled.Art.Scenes); err != nil {
		return err
	}
	if err := checkPaintedImages(unity, compiled.Art.Painted); err != nil {
		return err
	}
	if err := checkHudImage(unity, compiled.Art.HUD); err != nil {
		return err
	}
	if err := checkCharacterImages(unity, compiled.Art.Characters); err != nil {
		return err
	}
	if err := checkLoadingImages(unity, compiled.Loading); err != nil {
		return err
	}
	if err := checkMenuImage(unity, compiled.Menu); err != nil {
		return err
	}
	sprites, err := SpriteFiles(compiled.Art)
	if err != nil {
		return err
	}
	for name, data := range sprites {
		files[filepath.Join(unity, "Assets/Resources/Sprites/generated", name)] = data
	}
	phases.mark("emit: image checks, sprites")
	clothingJSON, err := json.MarshalIndent(compiled.Clothing, "", "  ")
	if err != nil {
		return err
	}
	clothingFiles, err := clothing.Render(clothingJSON)
	if err != nil {
		return err
	}
	for name, data := range clothingFiles {
		files[filepath.Join(project, "internal/clothing", name)] = data
	}
	craftingFiles, err := crafting.RenderEngine()
	if compiled.Crafting != nil {
		var catalog []byte
		if catalog, err = json.MarshalIndent(compiled.Crafting, "", "  "); err == nil {
			craftingFiles, err = crafting.Render(catalog)
		}
	}
	if err != nil {
		return err
	}
	for name, data := range craftingFiles {
		files[filepath.Join(project, "internal/crafting", name)] = data
	}
	barksFiles, err := barks.Render(*compiled.Barks)
	if err != nil {
		return err
	}
	for name, data := range barksFiles {
		files[filepath.Join(project, "internal/barks", name)] = data
	}
	if len(compiled.barksReport) > 0 {
		files[filepath.Join(project, "..", "design/barks/barks-report.md")] = compiled.barksReport
	}
	// the map of links of the conversations (doc 103 §6), generated
	if len(compiled.linksPage) > 0 {
		files[filepath.Join(project, "..", "design/dialogue/links.html")] = compiled.linksPage
	}
	if compiled.HeroFile != nil && len(compiled.HeroFile.report) > 0 {
		files[filepath.Join(project, "..", "design/hero-creation/v1/skill-uses.md")] = compiled.HeroFile.report
	}
	if compiled.Wounds != nil {
		woundFiles, err := wounds.Render(*compiled.Wounds)
		if err != nil {
			return err
		}
		for name, data := range woundFiles {
			files[filepath.Join(project, "internal/wounds", name)] = data
		}
		files[filepath.Join(project, "..", "design/wounds/timelines.md")] = compiled.woundsReport
	}
	// The save key from the server's runtime package as written: world.json (without the key itself) and the barks and
	// wounds catalogs the server embeds besides it.
	keySections := map[string]json.RawMessage{}
	if err := json.Unmarshal(worldJSON, &keySections); err != nil {
		return err
	}
	keySections["barks_catalog"] = barksFiles["catalog.json"]
	if catalog, ok := files[filepath.Join(project, "internal/wounds", "catalog.json")]; ok {
		keySections["wounds_catalog"] = catalog
	}
	saveKey, err := computeSaveKey(compiled.SaveKeyPolicy, keySections)
	if err != nil {
		return err
	}
	if files[filepath.Join(project, "internal/poc/world.json")], err = withSaveKey(worldJSON, saveKey); err != nil {
		return err
	}
	phases.mark("emit: save key")
	inventoryFiles, err := inventory.Render()
	if err != nil {
		return err
	}
	for name, data := range inventoryFiles {
		files[filepath.Join(project, "internal/inventory", name)] = data
	}
	behaviorFiles, err := behavior.RenderWith(compiled.AI, behavior.Fixtures{Planning: compiled.AIScenarios, Utility: compiled.AIUtility})
	if err != nil {
		return err
	}
	for name, data := range behaviorFiles {
		files[filepath.Join(project, "internal/behavior", name)] = data
	}
	phases.mark("emit: clothing, crafting, barks, wounds, inventory, behavior")
	for _, name := range []string{"LICENSE.go-behaviortree", "NOTICE.go-behaviortree"} {
		files[filepath.Join(unity, "Assets/StreamingAssets/GameServer", name)] = behaviorFiles[name]
	}

	for name, dest := range map[string]string{
		"localization.go.tmpl":            filepath.Join(project, "internal/poc/localization.gen.go"),
		"localization.cs.tmpl":            filepath.Join(unity, "Assets/Generated/Poc/PocLocalization.cs"),
		"instant_travel.go.tmpl":          filepath.Join(project, "internal/poc/instant_travel.gen.go"),
		"instant_travel_test.go.tmpl":     filepath.Join(project, "internal/poc/instant_travel_test.go"),
		"transit_fare.go.tmpl":            filepath.Join(project, "internal/poc/transit_fare.gen.go"),
		"transit_fare_test.go.tmpl":       filepath.Join(project, "internal/poc/transit_fare_test.go"),
		"wayfinding.go.tmpl":              filepath.Join(project, "internal/poc/wayfinding.gen.go"),
		"wayfinding_test.go.tmpl":         filepath.Join(project, "internal/poc/wayfinding_test.go"),
		"region.go.tmpl":                  filepath.Join(project, "internal/poc/region.gen.go"),
		"region_rumours.go.tmpl":          filepath.Join(project, "internal/poc/region_rumours.gen.go"),
		"region_observation_test.go.tmpl": filepath.Join(project, "internal/poc/region_observation_generated_test.go"),
		"region_rumours_test.go.tmpl":     filepath.Join(project, "internal/poc/region_rumours_generated_test.go"),
		"region_test.go.tmpl":             filepath.Join(project, "internal/poc/region_generated_test.go"),
		"region.cs.tmpl":                  filepath.Join(unity, "Assets/Generated/Poc/PocRegionMap.cs"),
		"spatial.cs.tmpl":                 filepath.Join(unity, "Assets/Generated/Poc/PocSpatialView.cs"),
		"hud.cs.tmpl":                     filepath.Join(unity, "Assets/Generated/Poc/PocHudView.cs"),
		"sdk_proof.cs.tmpl":               filepath.Join(unity, "Assets/Generated/Poc/PocSdkProof.cs"),
		"copper_route_proof.cs.tmpl":      filepath.Join(unity, "Assets/Generated/Poc/PocCopperRouteProof.cs"),
		"transport_hub_proof.cs.tmpl":     filepath.Join(unity, "Assets/Generated/Poc/PocTransportHubProof.cs"),
		"region_proof.cs.tmpl":            filepath.Join(unity, "Assets/Generated/Poc/PocRegionMapProof.cs"),
		"spatial_proof.cs.tmpl":           filepath.Join(unity, "Assets/Generated/Poc/PocSpatialProof.cs"),
		"population.go.tmpl":              filepath.Join(project, "internal/poc/population.gen.go"),
		"world_report_test.go.tmpl":       filepath.Join(project, "internal/poc/world_report_generated_test.go"),
		"npc_stage0_test.go.tmpl":         filepath.Join(project, "internal/poc/npc_stage0_generated_test.go"),
		"capture_dropout.go.tmpl":         filepath.Join(project, "internal/poc/capture_dropout.gen.go"),
		"population_test.go.tmpl":         filepath.Join(project, "internal/poc/population_generated_test.go"),
		"development.go.tmpl":             filepath.Join(project, "internal/poc/development.gen.go"),
		"development_test.go.tmpl":        filepath.Join(project, "internal/poc/development_generated_test.go"),
		"development.cs.tmpl":             filepath.Join(unity, "Assets/Generated/Poc/PocDevelopmentView.cs"),
		"npc_stage1_test.go.tmpl":         filepath.Join(project, "internal/poc/npc_stage1_generated_test.go"),
		"life_economy.go.tmpl":            filepath.Join(project, "internal/poc/life_economy.gen.go"),
		"life.go.tmpl":                    filepath.Join(project, "internal/poc/life.gen.go"),
		"storefront_inspection.go.tmpl":   filepath.Join(project, "internal/poc/storefront_inspection.gen.go"),
		"inspection.go.tmpl":              filepath.Join(project, "internal/poc/inspection.gen.go"),
		"inspection_test.go.tmpl":         filepath.Join(project, "internal/poc/inspection_generated_test.go"),
		"inspection.cs.tmpl":              filepath.Join(unity, "Assets/Generated/Poc/PocInspectionView.cs"),
		"life_test.go.tmpl":               filepath.Join(project, "internal/poc/life_generated_test.go"),
		"life.cs.tmpl":                    filepath.Join(unity, "Assets/Generated/Poc/PocLifeView.cs"),
		"wayfinding.cs.tmpl":              filepath.Join(unity, "Assets/Generated/Poc/PocWayfindingView.cs"),
		"wayfinding_route.cs.tmpl":        filepath.Join(unity, "Assets/Generated/Poc/PocWayfindingRoute.cs"),
		"wayfinding_proof.cs.tmpl":        filepath.Join(unity, "Assets/Generated/Poc/PocWayfindingProof.cs"),
		"wayfinding.shader.tmpl":          filepath.Join(unity, "Assets/Resources/Generated/PocWayfinding.shader"),
		"gestures.go.tmpl":                filepath.Join(project, "internal/poc/gestures.gen.go"),
		"gestures_test.go.tmpl":           filepath.Join(project, "internal/poc/gestures_generated_test.go"),
		"visibility.go.tmpl":              filepath.Join(project, "internal/poc/visibility.gen.go"),
		"visibility_test.go.tmpl":         filepath.Join(project, "internal/poc/visibility_generated_test.go"),
		"visibility.cs.tmpl":              filepath.Join(unity, "Assets/Generated/Poc/PocPerceptionView.cs"),
		"inventory_test.go.tmpl":          filepath.Join(project, "internal/poc/inventory_generated_test.go"),
		"item_display_test.go.tmpl":       filepath.Join(project, "internal/poc/item_display_generated_test.go"),
		"reports.go.tmpl":                 filepath.Join(project, "internal/poc/reports.gen.go"),
		"combat.go.tmpl":                  filepath.Join(project, "internal/poc/combat.gen.go"),
		"combat_inventory.go.tmpl":        filepath.Join(project, "internal/poc/combat_inventory.gen.go"),
		"combat_ammo_packs.go.tmpl":       filepath.Join(project, "internal/poc/combat_ammo_packs.gen.go"),
		"combat_ammo_packs_test.go.tmpl":  filepath.Join(project, "internal/poc/combat_ammo_packs_generated_test.go"),
		"combat_crafting.go.tmpl":         filepath.Join(project, "internal/poc/combat_crafting.gen.go"),
		"combat_wounds_seed_test.go.tmpl": filepath.Join(project, "internal/poc/combat_wounds_seed_generated_test.go"),
		"combat_wounds_test.go.tmpl":      filepath.Join(project, "internal/poc/combat_wounds_generated_test.go"),

		"combat_wounds_aid.go.tmpl":             filepath.Join(project, "internal/poc/combat_wounds_aid.gen.go"),
		"combat_wounds_save.go.tmpl":            filepath.Join(project, "internal/poc/combat_wounds_save.gen.go"),
		"combat_wounds_view.go.tmpl":            filepath.Join(project, "internal/poc/combat_wounds_view.gen.go"),
		"combat_wounds.go.tmpl":                 filepath.Join(project, "internal/poc/combat_wounds.gen.go"),
		"combat_care.go.tmpl":                   filepath.Join(project, "internal/poc/combat_care.gen.go"),
		"combat_care_test.go.tmpl":              filepath.Join(project, "internal/poc/combat_care_generated_test.go"),
		"combat_player_care.go.tmpl":            filepath.Join(project, "internal/poc/combat_player_care.gen.go"),
		"combat_player_care_test.go.tmpl":       filepath.Join(project, "internal/poc/combat_player_care_generated_test.go"),
		"combat_projection_test.go.tmpl":        filepath.Join(project, "internal/poc/combat_projection_generated_test.go"),
		"combat_projection.go.tmpl":             filepath.Join(project, "internal/poc/combat_projection.gen.go"),
		"combat_collateral.go.tmpl":             filepath.Join(project, "internal/poc/combat_collateral.gen.go"),
		"combat_takedown.go.tmpl":               filepath.Join(project, "internal/poc/combat_takedown.gen.go"),
		"combat_takedown_test.go.tmpl":          filepath.Join(project, "internal/poc/combat_takedown_generated_test.go"),
		"combat_stun_test.go.tmpl":              filepath.Join(project, "internal/poc/combat_stun_generated_test.go"),
		"combat_choke_test.go.tmpl":             filepath.Join(project, "internal/poc/combat_choke_generated_test.go"),
		"playable_scene.go.tmpl":                filepath.Join(project, "internal/poc/playable_scene.gen.go"),
		"playable_scene_test.go.tmpl":           filepath.Join(project, "internal/poc/playable_scene_generated_test.go"),
		"playable_scene_fixture.cs.tmpl":        filepath.Join(unity, "Assets/Generated/Poc/PocSceneFixture.cs"),
		"playable_scene.cs.tmpl":                filepath.Join(unity, "Assets/Generated/Poc/PocPlayableScene.cs"),
		"illumination.go.tmpl":                  filepath.Join(project, "internal/poc/illumination.gen.go"),
		"illumination_stealth_test.go.tmpl":     filepath.Join(project, "internal/poc/illumination_stealth_generated_test.go"),
		"illumination_test.go.tmpl":             filepath.Join(project, "internal/poc/illumination_generated_test.go"),
		"stealth.go.tmpl":                       filepath.Join(project, "internal/poc/stealth.gen.go"),
		"stealth_test.go.tmpl":                  filepath.Join(project, "internal/poc/stealth_generated_test.go"),
		"combat_life.go.tmpl":                   filepath.Join(project, "internal/poc/combat_life.gen.go"),
		"combat_life_test.go.tmpl":              filepath.Join(project, "internal/poc/combat_life_generated_test.go"),
		"combat_resources_test.go.tmpl":         filepath.Join(project, "internal/poc/combat_resources_generated_test.go"),
		"combat_crafting_test.go.tmpl":          filepath.Join(project, "internal/poc/combat_crafting_generated_test.go"),
		"combat_inventory_test.go.tmpl":         filepath.Join(project, "internal/poc/combat_inventory_generated_test.go"),
		"combat_test.go.tmpl":                   filepath.Join(project, "internal/poc/combat_generated_test.go"),
		"combat.cs.tmpl":                        filepath.Join(unity, "Assets/Generated/Poc/PocCombat.cs"),
		"combat_panel.go.tmpl":                  filepath.Join(project, "internal/poc/combat_panel.gen.go"),
		"combat_panel_test.go.tmpl":             filepath.Join(project, "internal/poc/combat_panel_generated_test.go"),
		"barks_combat.go.tmpl":                  filepath.Join(project, "internal/poc/barks_combat.gen.go"),
		"combat_play.cs.tmpl":                   filepath.Join(unity, "Assets/Generated/Poc/PocCombatPlay.cs"),
		"combat_panel.cs.tmpl":                  filepath.Join(unity, "Assets/Generated/Poc/PocCombatPanel.cs"),
		"combat_autoplay.cs.tmpl":               filepath.Join(unity, "Assets/Generated/Poc/PocCombatAutoplay.cs"),
		"combat_choke.cs.tmpl":                  filepath.Join(unity, "Assets/Generated/Poc/PocCombatChoke.cs"),
		"tactics.go.tmpl":                       filepath.Join(project, "internal/poc/tactics.gen.go"),
		"equipment.go.tmpl":                     filepath.Join(project, "internal/poc/equipment.gen.go"),
		"equipment_test.go.tmpl":                filepath.Join(project, "internal/poc/equipment_generated_test.go"),
		"supply_test.go.tmpl":                   filepath.Join(project, "internal/poc/supply_generated_test.go"),
		"supply.go.tmpl":                        filepath.Join(project, "internal/poc/supply.gen.go"),
		"inventory.go.tmpl":                     filepath.Join(project, "internal/poc/inventory.gen.go"),
		"inventory.cs.tmpl":                     filepath.Join(unity, "Assets/Generated/Poc/PocInventoryView.cs"),
		"social.go.tmpl":                        filepath.Join(project, "internal/poc/social.gen.go"),
		"social_test.go.tmpl":                   filepath.Join(project, "internal/poc/social_generated_test.go"),
		"mission_test.go.tmpl":                  filepath.Join(project, "internal/poc/mission_generated_test.go"),
		"capture.go.tmpl":                       filepath.Join(project, "internal/poc/capture.gen.go"),
		"capture_test.go.tmpl":                  filepath.Join(project, "internal/poc/capture_generated_test.go"),
		"capture.cs.tmpl":                       filepath.Join(unity, "Assets/Generated/Poc/PocCaptureView.cs"),
		"mission.go.tmpl":                       filepath.Join(project, "internal/poc/mission.gen.go"),
		"sanitary_services.go.tmpl":             filepath.Join(project, "internal/poc/sanitary_services.gen.go"),
		"sanitary_services_view.go.tmpl":        filepath.Join(project, "internal/poc/sanitary_services_view.gen.go"),
		"sanitary_services_test.go.tmpl":        filepath.Join(project, "internal/poc/sanitary_services_generated_test.go"),
		"medical_capsules.go.tmpl":              filepath.Join(project, "internal/poc/medical_capsules.gen.go"),
		"medical_capsules_view.go.tmpl":         filepath.Join(project, "internal/poc/medical_capsules_view.gen.go"),
		"medical_capsules_test.go.tmpl":         filepath.Join(project, "internal/poc/medical_capsules_generated_test.go"),
		"train_boarding.go.tmpl":                filepath.Join(project, "internal/poc/train_boarding.gen.go"),
		"train_boarding_test.go.tmpl":           filepath.Join(project, "internal/poc/train_boarding_generated_test.go"),
		"train_view.go.tmpl":                    filepath.Join(project, "internal/poc/train_view.gen.go"),
		"train_dispatch.go.tmpl":                filepath.Join(project, "internal/poc/train_dispatch.gen.go"),
		"lifting_decks.go.tmpl":                 filepath.Join(project, "internal/poc/lifting_decks.gen.go"),
		"lifting_decks_test.go.tmpl":            filepath.Join(project, "internal/poc/lifting_decks_generated_test.go"),
		"turret_watch.go.tmpl":                  filepath.Join(project, "internal/poc/turret_watch.gen.go"),
		"turret_watch_test.go.tmpl":             filepath.Join(project, "internal/poc/turret_watch_generated_test.go"),
		"access_zone.go.tmpl":                   filepath.Join(project, "internal/poc/access_zone.gen.go"),
		"access_zone_test.go.tmpl":              filepath.Join(project, "internal/poc/access_zone_generated_test.go"),
		"world_hit_test.go.tmpl":                filepath.Join(project, "internal/poc/world_hit_generated_test.go"),
		"biomonitor.go.tmpl":                    filepath.Join(project, "internal/poc/biomonitor.gen.go"),
		"chip_hud.go.tmpl":                      filepath.Join(project, "internal/poc/chip_hud.gen.go"),
		"chip_hud_test.go.tmpl":                 filepath.Join(project, "internal/poc/chip_hud_generated_test.go"),
		"biomonitor_test.go.tmpl":               filepath.Join(project, "internal/poc/biomonitor_generated_test.go"),
		"train_motion.go.tmpl":                  filepath.Join(project, "internal/poc/train_motion.gen.go"),
		"train_motion_test.go.tmpl":             filepath.Join(project, "internal/poc/train_motion_generated_test.go"),
		"train_fleet_fixture_test.go.tmpl":      filepath.Join(project, "internal/poc/train_fleet_fixture_generated_test.go"),
		"view_delivery_test.go.tmpl":            filepath.Join(project, "internal/poc/view_delivery_generated_test.go"),
		"runtime.go.tmpl":                       filepath.Join(project, "internal/poc/runtime.gen.go"),
		"saves.go.tmpl":                         filepath.Join(project, "internal/poc/saves.gen.go"),
		"saves_test.go.tmpl":                    filepath.Join(project, "internal/poc/saves_generated_test.go"),
		"save_compat_test.go.tmpl":              filepath.Join(project, "internal/poc/save_compat_generated_test.go"),
		"location_sdk.go.tmpl":                  filepath.Join(project, "internal/poc/location_sdk.gen.go"),
		"gates.go.tmpl":                         filepath.Join(project, "internal/poc/gates.gen.go"),
		"vending.go.tmpl":                       filepath.Join(project, "internal/poc/vending.gen.go"),
		"vending_test.go.tmpl":                  filepath.Join(project, "internal/poc/vending_generated_test.go"),
		"vending.cs.tmpl":                       filepath.Join(unity, "Assets/Generated/Poc/PocVending.cs"),
		"peddlers.go.tmpl":                      filepath.Join(project, "internal/poc/peddlers.gen.go"),
		"visitors.go.tmpl":                      filepath.Join(project, "internal/poc/visitors.gen.go"),
		"busker.go.tmpl":                        filepath.Join(project, "internal/poc/busker.gen.go"),
		"busker_test.go.tmpl":                   filepath.Join(project, "internal/poc/busker_generated_test.go"),
		"visitors_test.go.tmpl":                 filepath.Join(project, "internal/poc/visitors_generated_test.go"),
		"walkers.go.tmpl":                       filepath.Join(project, "internal/poc/walkers.gen.go"),
		"walkers_test.go.tmpl":                  filepath.Join(project, "internal/poc/walkers_generated_test.go"),
		"cart_parking.go.tmpl":                  filepath.Join(project, "internal/poc/cart_parking.gen.go"),
		"cargo_convoys.go.tmpl":                 filepath.Join(project, "internal/poc/cargo_convoys.gen.go"),
		"cargo_convoys_test.go.tmpl":            filepath.Join(project, "internal/poc/cargo_convoys_generated_test.go"),
		"cart_parking_test.go.tmpl":             filepath.Join(project, "internal/poc/cart_parking_generated_test.go"),
		"peddlers_test.go.tmpl":                 filepath.Join(project, "internal/poc/peddlers_generated_test.go"),
		"calendar_encounter.go.tmpl":            filepath.Join(project, "internal/poc/calendar_encounter.gen.go"),
		"conversation.go.tmpl":                  filepath.Join(project, "internal/poc/conversation.gen.go"),
		"conversation_test.go.tmpl":             filepath.Join(project, "internal/poc/conversation_generated_test.go"),
		"dialogue.go.tmpl":                      filepath.Join(project, "internal/poc/dialogue.gen.go"),
		"hero_file.go.tmpl":                     filepath.Join(project, "internal/poc/hero_file.gen.go"),
		"hero_file_test.go.tmpl":                filepath.Join(project, "internal/poc/hero_file_generated_test.go"),
		"dialogue_test.go.tmpl":                 filepath.Join(project, "internal/poc/dialogue_generated_test.go"),
		"calendar_encounter_test.go.tmpl":       filepath.Join(project, "internal/poc/calendar_encounter_generated_test.go"),
		"conversation.cs.tmpl":                  filepath.Join(unity, "Assets/Generated/Poc/PocConversation.cs"),
		"peddlers.cs.tmpl":                      filepath.Join(unity, "Assets/Generated/Poc/PocPeddlers.cs"),
		"craft.go.tmpl":                         filepath.Join(project, "internal/poc/craft.gen.go"),
		"craft_heaps_test.go.tmpl":              filepath.Join(project, "internal/poc/craft_heaps_generated_test.go"),
		"craft.cs.tmpl":                         filepath.Join(unity, "Assets/Generated/Poc/PocCraft.cs"),
		"soundscape.go.tmpl":                    filepath.Join(project, "internal/poc/soundscape.gen.go"),
		"soundscape_test.go.tmpl":               filepath.Join(project, "internal/poc/soundscape_generated_test.go"),
		"hall_closing_test.go.tmpl":             filepath.Join(project, "internal/poc/hall_closing_generated_test.go"),
		"soundscape.cs.tmpl":                    filepath.Join(unity, "Assets/Generated/Poc/PocSoundscape.cs"),
		"noise_waves.go.tmpl":                   filepath.Join(project, "internal/poc/noise_waves.gen.go"),
		"noise_waves_test.go.tmpl":              filepath.Join(project, "internal/poc/noise_waves_generated_test.go"),
		"noise_waves.cs.tmpl":                   filepath.Join(unity, "Assets/Generated/Poc/PocNoiseWaves.cs"),
		"noise_wave.shader.tmpl":                filepath.Join(unity, "Assets/Resources/Generated/PocNoiseWave.shader"),
		"barks.go.tmpl":                         filepath.Join(project, "internal/poc/barks.gen.go"),
		"barks_facts.go.tmpl":                   filepath.Join(project, "internal/poc/barks_facts.gen.go"),
		"barks_test.go.tmpl":                    filepath.Join(project, "internal/poc/barks_generated_test.go"),
		"barks.cs.tmpl":                         filepath.Join(unity, "Assets/Generated/Poc/PocBarks.cs"),
		"bark.shader.tmpl":                      filepath.Join(unity, "Assets/Resources/Generated/PocBark.shader"),
		"bark_speaker.shader.tmpl":              filepath.Join(unity, "Assets/Resources/Generated/PocBarkSpeaker.shader"),
		"crafting.go.tmpl":                      filepath.Join(project, "internal/poc/crafting.gen.go"),
		"crafting_test.go.tmpl":                 filepath.Join(project, "internal/poc/crafting_generated_test.go"),
		"crafting.cs.tmpl":                      filepath.Join(unity, "Assets/Generated/Poc/PocCrafting.cs"),
		"crafting_commands.go.tmpl":             filepath.Join(project, "internal/poc/crafting_commands.gen.go"),
		"crafting_time.go.tmpl":                 filepath.Join(project, "internal/poc/crafting_time.gen.go"),
		"godot_screen_views_test.go.tmpl":       filepath.Join(project, "internal/poc/screens_views_generated_test.go"),
		"godot_screen_combat.go.tmpl":           filepath.Join(project, "internal/poc/screens_combat.gen.go"),
		"godot_screen_crafting.go.tmpl":         filepath.Join(project, "internal/poc/screens_crafting.gen.go"),
		"godot_screen_services.go.tmpl":         filepath.Join(project, "internal/poc/screens_services.gen.go"),
		"godot_screen_inventory.go.tmpl":        filepath.Join(project, "internal/poc/screens_inventory.gen.go"),
		"exit_zones.go.tmpl":                    filepath.Join(project, "internal/poc/exit_zones.gen.go"),
		"exit_zones_test.go.tmpl":               filepath.Join(project, "internal/poc/exit_zones_generated_test.go"),
		"godot_screen_targets.go.tmpl":          filepath.Join(project, "internal/poc/screens_targets.gen.go"),
		"godot_screen_views.go.tmpl":            filepath.Join(project, "internal/poc/screens_views.gen.go"),
		"crafting_view.go.tmpl":                 filepath.Join(project, "internal/poc/crafting_view.gen.go"),
		"crafting_sources.go.tmpl":              filepath.Join(project, "internal/poc/crafting_sources.gen.go"),
		"crafting_sources_view.go.tmpl":         filepath.Join(project, "internal/poc/crafting_sources_view.gen.go"),
		"crafting_law.go.tmpl":                  filepath.Join(project, "internal/poc/crafting_law.gen.go"),
		"crafting_market.go.tmpl":               filepath.Join(project, "internal/poc/crafting_market.gen.go"),
		"crafting_sources_test.go.tmpl":         filepath.Join(project, "internal/poc/crafting_sources_generated_test.go"),
		"crafting_law_test.go.tmpl":             filepath.Join(project, "internal/poc/crafting_law_generated_test.go"),
		"gates_test.go.tmpl":                    filepath.Join(project, "internal/poc/gates_generated_test.go"),
		"location_sdk_gates.cs.tmpl":            filepath.Join(unity, "Assets/Generated/Poc/PocLocationSDKGates.cs"),
		"location_sdk_test.go.tmpl":             filepath.Join(project, "internal/poc/location_sdk_generated_test.go"),
		"multideck_test.go.tmpl":                filepath.Join(project, "internal/poc/multideck_generated_test.go"),
		"hero_body_test.go.tmpl":                filepath.Join(project, "internal/poc/hero_body_generated_test.go"),
		"runtime_test.go.tmpl":                  filepath.Join(project, "internal/poc/runtime_generated_test.go"),
		"main.go.tmpl":                          filepath.Join(project, "cmd/game-server/main.go"),
		"art_scene.cs.tmpl":                     filepath.Join(unity, "Assets/Generated/Poc/ArtScene.cs"),
		"painted.cs.tmpl":                       filepath.Join(unity, "Assets/Generated/Poc/PocPaintedView.cs"),
		"storefront.cs.tmpl":                    filepath.Join(unity, "Assets/Generated/Poc/PocStorefront.cs"),
		"storefront_override.cs.tmpl":           filepath.Join(unity, "Assets/Generated/Poc/PocStorefrontVisualOverride.cs"),
		"storefront_proof.cs.tmpl":              filepath.Join(unity, "Assets/Generated/Poc/PocStorefrontProof.cs"),
		"storefront.shader.tmpl":                filepath.Join(unity, "Assets/Resources/Generated/PocStorefront.shader"),
		"painted_ambient_proof.cs.tmpl":         filepath.Join(unity, "Assets/Generated/Poc/PocAmbientProof.cs"),
		"train_motion.cs.tmpl":                  filepath.Join(unity, "Assets/Generated/Poc/PocTrainMotion.cs"),
		"train_motion_painted.cs.tmpl":          filepath.Join(unity, "Assets/Generated/Poc/PocTrainMotionPainted.cs"),
		"train_motion_proof.cs.tmpl":            filepath.Join(unity, "Assets/Generated/Poc/PocTrainMotionProof.cs"),
		"train_fleet_proof.cs.tmpl":             filepath.Join(unity, "Assets/Generated/Poc/PocTrainFleetProof.cs"),
		"sanitary_services.cs.tmpl":             filepath.Join(unity, "Assets/Generated/Poc/PocSanitaryServices.cs"),
		"sanitary_services_panel.cs.tmpl":       filepath.Join(unity, "Assets/Generated/Poc/PocSanitaryServicesPanel.cs"),
		"sanitary_services_proof.cs.tmpl":       filepath.Join(unity, "Assets/Generated/Poc/PocSanitaryServicesProof.cs"),
		"medical_scan.cs.tmpl":                  filepath.Join(unity, "Assets/Generated/Poc/PocMedicalScan.cs"),
		"medical_scan.shader.tmpl":              filepath.Join(unity, "Assets/Resources/Generated/PocMedicalScan.shader"),
		"medical_capsules.cs.tmpl":              filepath.Join(unity, "Assets/Generated/Poc/PocMedicalCapsules.cs"),
		"medical_capsules_panel.cs.tmpl":        filepath.Join(unity, "Assets/Generated/Poc/PocMedicalCapsulesPanel.cs"),
		"medical_capsules_proof.cs.tmpl":        filepath.Join(unity, "Assets/Generated/Poc/PocMedicalCapsulesProof.cs"),
		"painted_ambient.cs.tmpl":               filepath.Join(unity, "Assets/Generated/Poc/PocPaintedAmbient.cs"),
		"painted_motion.cs.tmpl":                filepath.Join(unity, "Assets/Generated/Poc/PocPaintedMotion.cs"),
		"painted_motion_proof.cs.tmpl":          filepath.Join(unity, "Assets/Generated/Poc/PocPaintedMotionProof.cs"),
		"station_life_proof.cs.tmpl":            filepath.Join(unity, "Assets/Generated/Poc/PocStationLifeProof.cs"),
		"station_berth_proof.cs.tmpl":           filepath.Join(unity, "Assets/Generated/Poc/PocStationBerthProof.cs"),
		"painted_passage.shader.tmpl":           filepath.Join(unity, "Assets/Resources/Generated/PocPaintedPassage.shader"),
		"painted_proof.cs.tmpl":                 filepath.Join(unity, "Assets/Generated/Poc/PocPaintedProof.cs"),
		"unity.cs.tmpl":                         filepath.Join(unity, "Assets/Generated/Poc/AngelsDemonsPoc.cs"),
		"location_sdk.cs.tmpl":                  filepath.Join(unity, "Assets/Generated/Poc/PocLocationSDK.cs"),
		"location_sdk_overrides.cs.tmpl":        filepath.Join(unity, "Assets/Generated/Poc/AngelsDemonsLocationOverrideRegistry.cs"),
		"location_sdk_overrides_editor.cs.tmpl": filepath.Join(unity, "Assets/Generated/Poc/Editor/AngelsDemonsLocationOverridesEditor.cs"),
		"location_sdk_proof.cs.tmpl":            filepath.Join(unity, "Assets/Generated/Poc/PocLocationSDKProof.cs"),
		"location_sdk_surface.shader.tmpl":      filepath.Join(unity, "Assets/Resources/LocationSDK/SDKSurface.shader"),
		"location_sdk_actor.shader.tmpl":        filepath.Join(unity, "Assets/Resources/LocationSDK/SDKActor.shader"),
		"location_sdk_depth_sprite.shader.tmpl": filepath.Join(unity, "Assets/Resources/LocationSDK/SDKDepthSprite.shader"),
		"location_sdk_depth_proof.cs.tmpl":      filepath.Join(unity, "Assets/Generated/Poc/PocLocationSDKDepthProof.cs"),
		"texture_proof_editor.cs.tmpl":          filepath.Join(unity, "Assets/Generated/Poc/Editor/PocTextureProof.cs"),
		"editor.cs.tmpl":                        filepath.Join(unity, "Assets/Generated/Poc/Editor/PocBuild.cs"),
		"urp_proof.cs.tmpl":                     filepath.Join(unity, "Assets/Generated/Poc/PocURPProof.cs"),
		"urp.cs.tmpl":                           filepath.Join(unity, "Assets/Generated/Poc/PocRendering.cs"),
		"urp_feature.cs.tmpl":                   filepath.Join(unity, "Assets/Generated/Poc/PocObservedActorFeature.cs"),
		"urp_editor.cs.tmpl":                    filepath.Join(unity, "Assets/Generated/Poc/Editor/PocURPSetup.cs"),
		"location_sdk_diagnostic.shader.tmpl":   filepath.Join(unity, "Assets/Resources/LocationSDK/SDKDiagnostic.shader"),
		"location_sdk_glass.shader.tmpl":        filepath.Join(unity, "Assets/Resources/LocationSDK/SDKGlass.shader"),
		"location_sdk_dither.shader.tmpl":       filepath.Join(unity, "Assets/Resources/LocationSDK/SDKInstanceDither.shader"),
		"world_text.cs.tmpl":                    filepath.Join(unity, "Assets/Generated/Poc/PocWorldText.cs"),
		"world_text.shader.tmpl":                filepath.Join(unity, "Assets/Resources/LocationSDK/SDKWorldText.shader"),
		"location_sdk_multideck.cs.tmpl":        filepath.Join(unity, "Assets/Generated/Poc/PocLocationSDKMultideck.cs"),
		"location_sdk_humanoid.cs.tmpl":         filepath.Join(unity, "Assets/Generated/Poc/PocLocationSDKHumanoid.cs"),
		"hero_body.cs.tmpl":                     filepath.Join(unity, "Assets/Generated/Poc/PocHeroBody.cs"),
		"location_sdk_multideck_proof.cs.tmpl":  filepath.Join(unity, "Assets/Generated/Poc/PocLocationSDKMultideckProof.cs"),
	} {
		if strings.HasPrefix(name, "godot_screen_") && compiled.GodotScreens == nil {
			continue
		}
		if strings.HasPrefix(name, "item_display_") && compiled.ItemDisplay == nil {
			continue
		}
		if !legacyUnity && strings.HasPrefix(dest, unity+string(filepath.Separator)) {
			continue
		}
		data, err := sources.ReadFile("templates/" + name)
		if err != nil {
			return err
		}
		t, err := template.New(name).Delims("<<ANG", "ANG>>").Parse(string(data))
		if err != nil {
			return err
		}
		var buf bytes.Buffer
		if err := t.Execute(&buf, compiled); err != nil {
			return err
		}
		b := buf.Bytes()
		if strings.HasSuffix(dest, ".go") {
			b, err = goformat.Source(b)
			if err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
		}
		files[dest] = b
	}
	phases.mark("emit: templates (Go, gofmt)")
	if def.GDScriptOut != "" {
		gd, err := emitter.RenderGDScriptContract(def, "angelsdemons/server")
		if err != nil {
			return err
		}
		files[filepath.Join(project, def.GDScriptOut)] = gd
	}
	protoRel := emitter.GRPCProtoPath(def)
	protoText := []byte(emitter.RenderGRPCProto(def, "angelsdemons/server"))
	files[filepath.Join(project, "api/grpc", protoRel)] = protoText
	files[filepath.Join(unity, "Assets/Proto", protoRel)] = protoText
	pbName, pb, err := emitter.GeneratePBGo(def, "angelsdemons/server")
	if err != nil {
		return err
	}
	pbDir := filepath.Join(project, "internal/transport/grpc/pb/game/v1")
	files[filepath.Join(pbDir, pbName)] = pb
	stubs, err := goformat.Source(emitter.RenderGRPCStubs(def, "gamev1"))
	if err != nil {
		return err
	}
	files[filepath.Join(pbDir, "game_grpc.pb.go")] = stubs
	if def.WebSocket {
		imp, pkg := emitter.GRPCGoPackage(def, "angelsdemons/server")
		ws, err := emitter.RenderGRPCWebSocket(def, imp, pkg)
		if err != nil {
			return err
		}
		files[filepath.Join(project, "internal/transport/grpc/websocket.gen.go")] = ws
		files[filepath.Join(project, "internal/transport/grpc/server.gen.go")] = emitter.RenderGRPCServer(def, imp, pkg)
	}

	delivery, err := emitter.RenderGRPCDelivery(def, "gamev1")
	if err != nil {
		return err
	}
	if delivery != nil {
		files[filepath.Join(pbDir, "game_delivery.pb.go")] = delivery
	}
	csDir := filepath.Join(unity, "Assets/Generated/AngelsDemons")
	for _, svc := range def.Services {
		if !legacyUnity {
			continue
		}
		files[filepath.Join(csDir, "Grpc", svc.Name+"Grpc.cs")] = []byte(emitter.RenderCSharpStubs(def, svc))
	}
	for _, sm := range emitter.SessionMethods(def) {
		if !legacyUnity {
			continue
		}
		files[filepath.Join(csDir, "Sessions", sm.Service.Name+sm.Method.Name+"Session.cs")] = []byte(emitter.RenderCSharpSession(def, sm.Service, sm.Method))
	}
	phases.mark("emit: GDScript contract, proto, pb, gRPC")
	names := make([]string, 0, len(files))
	for path := range files {
		if !legacyUnity && strings.HasPrefix(path, unity+string(filepath.Separator)) {
			continue
		}
		names = append(names, path)
	}
	sort.Strings(names)
	for _, p := range names {
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			return err
		}
		if strings.HasSuffix(p, ".go") {
			if err := determinism.CheckImports(p, files[p]); err != nil {
				return err
			}
		}
		if err := os.WriteFile(p, files[p], 0644); err != nil {
			return err
		}
	}
	phases.mark("write: files")
	// Remove only obsolete outputs owned by this generator after successful rendering.
	for _, name := range []string{"planner.gen.go", "planner_generated_test.go", "planning_scenarios.json"} {
		if err := os.Remove(filepath.Join(project, "internal/poc", name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if !legacyUnity {
		fmt.Printf("Generated active server and Godot from CUE: %s, content %s\n", compiled.Config.LevelID, compiled.ContentHash[:12])
		return nil
	}
	if err := os.MkdirAll(filepath.Join(csDir, "Messages"), 0755); err != nil {
		return err
	}
	cmd := exec.Command("protoc", "-I", filepath.Join(project, "api/grpc"), "--csharp_out="+filepath.Join(csDir, "Messages"), protoRel)
	if b, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("protoc: %w: %s", err, b)
	}
	fmt.Printf("Generated local POC from CUE: %s, %d x %d, %d world NPCs + %d demo NPCs, content %s\n", compiled.Config.LevelID, len(compiled.Terrain[0]), len(compiled.Terrain), len(compiled.Entities), len(compiled.Config.CaptureDemo.Agents), compiled.ContentHash[:12])
	return nil
}
