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
	"github.com/strogmv/ang/compiler/behavior"
	"github.com/strogmv/ang/compiler/clothing"
	clothingRuntime "github.com/strogmv/ang/compiler/clothing/runtime"
	"github.com/strogmv/ang/compiler/emitter"
	"github.com/strogmv/ang/compiler/inventory"
	inv "github.com/strogmv/ang/compiler/inventory/runtime"
	"github.com/strogmv/ang/compiler/locationsdk"
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
	MaxSaveBytes          int64             `json:"max_save_bytes"`
	Labels                map[string]string `json:"labels"`
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
	HUD        *HudArt              `json:"hud,omitempty"`
	Floor      *FloorMaterial       `json:"floor,omitempty"`
	Scenes     []ArtScene           `json:"scenes"`
	Painted    []PaintedScene       `json:"painted"`
	Camera     PresentationCamera   `json:"camera"`
	Characters CharacterArt         `json:"characters"`
	Tile       struct{ W, H int }   `json:"tile"`
	Palette    map[string]string    `json:"palette"`
	Sprites    map[string]ArtSprite `json:"sprites"`
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
	Wayfinding              WayfindingCatalog              `json:"wayfinding,omitempty"`
	LocationSDK             *locationsdk.RuntimeBundle     `json:"location_sdk,omitempty"`
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
}

func (c Config) validate() error {
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
	seen := map[string]bool{}
	for _, slot := range c.SaveSlots {
		if slot == "" || slot == "." || strings.ContainsAny(slot, "/\\.") || seen[slot] {
			return fmt.Errorf("invalid or duplicate save slot %q", slot)
		}
		seen[slot] = true
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
	if out.Config.CaptureDemo.TargetActor == "" {
		out.Config.CaptureDemo.TargetActor = "char.hero"
	}
	if err := out.Config.validate(); err != nil {
		return out, nil, err
	}
	w, err := cuePackage(filepath.Join(project, out.Config.WorldDir))
	if err != nil {
		return out, nil, err
	}
	world := w.LookupPath(cue.ParsePath("world"))
	if err := world.Validate(cue.Concrete(true)); err != nil {
		return out, nil, fmt.Errorf("world: %s", errors.Details(err, nil))
	}
	if err := world.LookupPath(cue.ParsePath("visibility")).Decode(&out.Visibility); err != nil {
		return out, nil, err
	}
	if out.Visibility.PlayerRange < 1 || out.Visibility.PlayerRange > 64 || out.Visibility.Field != "omnidirectional" || out.Visibility.Remember != "observed_geometry_only" || out.Visibility.UnknownGlyph != "?" {
		return out, nil, fmt.Errorf("invalid visibility policy")
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
	if err := checkArtScenes(out.Art.Scenes); err != nil {
		return out, nil, err
	}
	if anchors := world.LookupPath(cue.ParsePath("painted_inventory_anchors")); anchors.Exists() {
		if err := anchors.Decode(&out.PaintedInventoryAnchors); err != nil {
			return out, nil, fmt.Errorf("world.painted_inventory_anchors: %w", err)
		}
	}
	if err := out.Wayfinding.validate(out.Life, out.Walkable, out.Localization, out.PaintedInventoryAnchors); err != nil {
		return out, nil, err
	}
	if err := checkPresentationCamera(out.Art.Camera); err != nil {
		return out, nil, err
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
	if out.Inventory, err = inventoryModel(world, level, out.Config.LevelID); err != nil {
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
	for _, name := range []string{"PocGameState", "PocSaveRequest", "PocLoadRequest", "PocNewGameRequest", "PocSaveSlots"} {
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
	project, err := filepath.Abs(project)
	if err != nil {
		return err
	}
	compiled, def, err := readCompiled(project)
	if err != nil {
		return err
	}
	unity := filepath.Clean(filepath.Join(project, compiled.Config.UnityDir))
	worldJSON, err := json.MarshalIndent(compiled, "", "  ")
	if err != nil {
		return err
	}
	viewJSON, err := clientResource(compiled)
	if err != nil {
		return err
	}
	if compiled.LocationSDK != nil {
		if err := checkLocationSDKSurfaceImages(unity, *compiled.LocationSDK); err != nil {
			return err
		}
	}
	files := map[string][]byte{
		filepath.Join(project, "internal/poc/world.json"):       worldJSON,
		filepath.Join(unity, "Assets/Resources/PocConfig.json"): viewJSON,
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
	for _, name := range []string{"LICENSE.go-behaviortree", "NOTICE.go-behaviortree"} {
		files[filepath.Join(unity, "Assets/StreamingAssets/GameServer", name)] = behaviorFiles[name]
	}

	for name, dest := range map[string]string{
		"localization.go.tmpl":                  filepath.Join(project, "internal/poc/localization.gen.go"),
		"localization.cs.tmpl":                  filepath.Join(unity, "Assets/Generated/Poc/PocLocalization.cs"),
		"instant_travel.go.tmpl":                filepath.Join(project, "internal/poc/instant_travel.gen.go"),
		"instant_travel_test.go.tmpl":           filepath.Join(project, "internal/poc/instant_travel_test.go"),
		"transit_fare.go.tmpl":                  filepath.Join(project, "internal/poc/transit_fare.gen.go"),
		"transit_fare_test.go.tmpl":             filepath.Join(project, "internal/poc/transit_fare_test.go"),
		"wayfinding.go.tmpl":                    filepath.Join(project, "internal/poc/wayfinding.gen.go"),
		"wayfinding_test.go.tmpl":               filepath.Join(project, "internal/poc/wayfinding_test.go"),
		"region.go.tmpl":                        filepath.Join(project, "internal/poc/region.gen.go"),
		"region_test.go.tmpl":                   filepath.Join(project, "internal/poc/region_generated_test.go"),
		"region.cs.tmpl":                        filepath.Join(unity, "Assets/Generated/Poc/PocRegionMap.cs"),
		"spatial.cs.tmpl":                       filepath.Join(unity, "Assets/Generated/Poc/PocSpatialView.cs"),
		"hud.cs.tmpl":                           filepath.Join(unity, "Assets/Generated/Poc/PocHudView.cs"),
		"sdk_proof.cs.tmpl":                     filepath.Join(unity, "Assets/Generated/Poc/PocSdkProof.cs"),
		"copper_route_proof.cs.tmpl":            filepath.Join(unity, "Assets/Generated/Poc/PocCopperRouteProof.cs"),
		"transport_hub_proof.cs.tmpl":           filepath.Join(unity, "Assets/Generated/Poc/PocTransportHubProof.cs"),
		"region_proof.cs.tmpl":                  filepath.Join(unity, "Assets/Generated/Poc/PocRegionMapProof.cs"),
		"spatial_proof.cs.tmpl":                 filepath.Join(unity, "Assets/Generated/Poc/PocSpatialProof.cs"),
		"population.go.tmpl":                    filepath.Join(project, "internal/poc/population.gen.go"),
		"population_test.go.tmpl":               filepath.Join(project, "internal/poc/population_generated_test.go"),
		"development.go.tmpl":                   filepath.Join(project, "internal/poc/development.gen.go"),
		"development_test.go.tmpl":              filepath.Join(project, "internal/poc/development_generated_test.go"),
		"development.cs.tmpl":                   filepath.Join(unity, "Assets/Generated/Poc/PocDevelopmentView.cs"),
		"life.go.tmpl":                          filepath.Join(project, "internal/poc/life.gen.go"),
		"inspection.go.tmpl":                    filepath.Join(project, "internal/poc/inspection.gen.go"),
		"inspection_test.go.tmpl":               filepath.Join(project, "internal/poc/inspection_generated_test.go"),
		"inspection.cs.tmpl":                    filepath.Join(unity, "Assets/Generated/Poc/PocInspectionView.cs"),
		"life_test.go.tmpl":                     filepath.Join(project, "internal/poc/life_generated_test.go"),
		"life.cs.tmpl":                          filepath.Join(unity, "Assets/Generated/Poc/PocLifeView.cs"),
		"wayfinding.cs.tmpl":                    filepath.Join(unity, "Assets/Generated/Poc/PocWayfindingView.cs"),
		"wayfinding_proof.cs.tmpl":              filepath.Join(unity, "Assets/Generated/Poc/PocWayfindingProof.cs"),
		"wayfinding.shader.tmpl":                filepath.Join(unity, "Assets/Resources/Generated/PocWayfinding.shader"),
		"gestures.go.tmpl":                      filepath.Join(project, "internal/poc/gestures.gen.go"),
		"gestures_test.go.tmpl":                 filepath.Join(project, "internal/poc/gestures_generated_test.go"),
		"visibility.go.tmpl":                    filepath.Join(project, "internal/poc/visibility.gen.go"),
		"visibility_test.go.tmpl":               filepath.Join(project, "internal/poc/visibility_generated_test.go"),
		"visibility.cs.tmpl":                    filepath.Join(unity, "Assets/Generated/Poc/PocPerceptionView.cs"),
		"inventory_test.go.tmpl":                filepath.Join(project, "internal/poc/inventory_generated_test.go"),
		"reports.go.tmpl":                       filepath.Join(project, "internal/poc/reports.gen.go"),
		"tactics.go.tmpl":                       filepath.Join(project, "internal/poc/tactics.gen.go"),
		"equipment.go.tmpl":                     filepath.Join(project, "internal/poc/equipment.gen.go"),
		"equipment_test.go.tmpl":                filepath.Join(project, "internal/poc/equipment_generated_test.go"),
		"inventory.go.tmpl":                     filepath.Join(project, "internal/poc/inventory.gen.go"),
		"inventory.cs.tmpl":                     filepath.Join(unity, "Assets/Generated/Poc/PocInventoryView.cs"),
		"social.go.tmpl":                        filepath.Join(project, "internal/poc/social.gen.go"),
		"social_test.go.tmpl":                   filepath.Join(project, "internal/poc/social_generated_test.go"),
		"mission_test.go.tmpl":                  filepath.Join(project, "internal/poc/mission_generated_test.go"),
		"capture.go.tmpl":                       filepath.Join(project, "internal/poc/capture.gen.go"),
		"capture_test.go.tmpl":                  filepath.Join(project, "internal/poc/capture_generated_test.go"),
		"capture.cs.tmpl":                       filepath.Join(unity, "Assets/Generated/Poc/PocCaptureView.cs"),
		"mission.go.tmpl":                       filepath.Join(project, "internal/poc/mission.gen.go"),
		"runtime.go.tmpl":                       filepath.Join(project, "internal/poc/runtime.gen.go"),
		"location_sdk.go.tmpl":                  filepath.Join(project, "internal/poc/location_sdk.gen.go"),
		"location_sdk_test.go.tmpl":             filepath.Join(project, "internal/poc/location_sdk_generated_test.go"),
		"runtime_test.go.tmpl":                  filepath.Join(project, "internal/poc/runtime_generated_test.go"),
		"main.go.tmpl":                          filepath.Join(project, "cmd/game-server/main.go"),
		"art_scene.cs.tmpl":                     filepath.Join(unity, "Assets/Generated/Poc/ArtScene.cs"),
		"painted.cs.tmpl":                       filepath.Join(unity, "Assets/Generated/Poc/PocPaintedView.cs"),
		"painted_ambient_proof.cs.tmpl":         filepath.Join(unity, "Assets/Generated/Poc/PocAmbientProof.cs"),
		"painted_ambient.cs.tmpl":               filepath.Join(unity, "Assets/Generated/Poc/PocPaintedAmbient.cs"),
		"painted_motion.cs.tmpl":                filepath.Join(unity, "Assets/Generated/Poc/PocPaintedMotion.cs"),
		"painted_motion_proof.cs.tmpl":          filepath.Join(unity, "Assets/Generated/Poc/PocPaintedMotionProof.cs"),
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
		"texture_proof_editor.cs.tmpl":          filepath.Join(unity, "Assets/Generated/Poc/Editor/PocTextureProof.cs"),
		"editor.cs.tmpl":                        filepath.Join(unity, "Assets/Generated/Poc/Editor/PocBuild.cs"),
	} {
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
	csDir := filepath.Join(unity, "Assets/Generated/AngelsDemons")
	for _, svc := range def.Services {
		files[filepath.Join(csDir, "Grpc", svc.Name+"Grpc.cs")] = []byte(emitter.RenderCSharpStubs(def, svc))
	}
	for _, sm := range emitter.SessionMethods(def) {
		files[filepath.Join(csDir, "Sessions", sm.Service.Name+sm.Method.Name+"Session.cs")] = []byte(emitter.RenderCSharpSession(def, sm.Service, sm.Method))
	}
	names := make([]string, 0, len(files))
	for path := range files {
		names = append(names, path)
	}
	sort.Strings(names)
	for _, p := range names {
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(p, files[p], 0644); err != nil {
			return err
		}
	}
	// Remove only obsolete outputs owned by this generator after successful rendering.
	for _, name := range []string{"planner.gen.go", "planner_generated_test.go", "planning_scenarios.json"} {
		if err := os.Remove(filepath.Join(project, "internal/poc", name)); err != nil && !os.IsNotExist(err) {
			return err
		}
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
