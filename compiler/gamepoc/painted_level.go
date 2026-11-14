package gamepoc

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// PaintedLevel is world.painted_location.levels[] (painted market v1, the game's design/painted-location/market-v1/BRIEF.md):
// a whole level drawn as one painting of the fixed game camera's view plane ("the plate", orthographic, PxPerM pixels a
// metre), cut into square pages the client streams in around the view. The paint step (the game's
// tools/painted-location/market.py) renders the level's static meshes from the game camera, paints its 4 m plan tiles
// with Paint's painter (an image model with a style sample, or the corner's approved Firefly tiles), registers, checks,
// harmonises to the style key, blends and packs the pages; LiveParts — everything whose state the server changes — stay
// real-time 3D and out of the render. ANG checks the data, that every server-driven thing of the level is live (a gate, a
// train, the crane bridge, a lift deck, a service cell's doors, a torgomat's parts, the goods and covers of a counter, a
// lamp's lens), and, once Picture is "pages", the packed pages themselves.
type PaintedLevel struct {
	ID              string       `json:"id"`
	LevelID         string       `json:"level_id"`
	Why             string       `json:"why"`
	Picture         string       `json:"picture"`
	Pages           PaintedPages `json:"pages"`
	DepthToleranceM float64      `json:"depth_tolerance_m"`
	Strength        float64      `json:"strength"`
	Look            string       `json:"look"`
	PaintingLook    PaintingLook `json:"painting_look"`
	// Frames: the level's own frames — the proof's frozen moments and the paint step's order (the first is the start
	// frame; at_hero: the proof takes it from the hero, the plan point is the fallback).
	Frames    []PaintedFrame    `json:"frames"`
	LiveParts []PaintedLivePart `json:"live_parts"`
	Paint     PaintStep         `json:"paint"`
}

// PaintedFrame: a plan point at the frame's focus height (x east, y south, z up) and the frame's height in metres.
type PaintedFrame struct {
	Name   string     `json:"name"`
	Plan   [3]float64 `json:"plan"`
	SizeM  float64    `json:"size_m"`
	AtHero bool       `json:"at_hero"`
}

// PaintedPages: where the packed pages are (Folder/index.json: the plate and its pages), their size with the gutter of
// neighbouring pixels, the plate's pixels a metre, the band around the view the client keeps loaded and how many pages
// it may hold.
type PaintedPages struct {
	Folder   string  `json:"folder"`
	PagePx   int     `json:"page_px"`
	GutterPx int     `json:"gutter_px"`
	PxPerM   float64 `json:"px_per_m"`
	MarginM  float64 `json:"margin_m"`
	Slots    int     `json:"slots"`
}

// PaintStep: the paint step's data — the style sample and key, the default painter and its model, prompt, seed and
// steps, the tiles another painter keeps, and the checks.
type PaintStep struct {
	// Classes: the level's object classes for the harmonisation (tools/painted-location/*.json; the paint step reads it).
	Classes     string `json:"classes"`
	StyleSample string `json:"style_sample"`
	StyleKey    struct {
		Tile     string `json:"tile"`
		Painting string `json:"painting"`
	} `json:"style_key"`
	Painter  string `json:"painter"`
	Painters struct {
		Flux2Klein4B PaintModel `json:"flux2_klein_4b"`
	} `json:"painters"`
	Tiles        []PaintTile `json:"tiles"`
	Firefly      string      `json:"firefly"`
	ForeignDE    float64     `json:"foreign_de"`
	ForeignShare float64     `json:"foreign_share"`
	MaxDriftDE   float64     `json:"max_drift_de"`
	Calm         struct {
		Mottle float64 `json:"mottle"`
		Middle float64 `json:"middle"`
	} `json:"calm"`
}

// PaintModel: a local image model (outside Git, no network) and its job.
type PaintModel struct {
	Model    string `json:"model"`
	Tool     string `json:"tool"`
	Version  string `json:"version"`
	Prompt   string `json:"prompt"`
	Seed     int    `json:"seed"`
	Steps    int    `json:"steps"`
	WindowPx [2]int `json:"window_px"`
	InputPx  [2]int `json:"input_px"`
	OutputPx [2]int `json:"output_px"`
	LowRAM   bool   `json:"low_ram"`
}

// PaintTile: a tile painted by another painter than the default (firefly: the corner's approved paintings).
type PaintTile struct {
	Tile    string `json:"tile"`
	Painter string `json:"painter"`
	Why     string `json:"why"`
}

var (
	paintedLevelID   = regexp.MustCompile(`^[a-z0-9_]+$`)
	paintedTileID    = regexp.MustCompile(`^c-?[0-9]+_-?[0-9]+-t[0-9]+_[0-9]+$`)
	paintedInstance  = regexp.MustCompile(`^[a-z0-9_.-]+$`)
	paintedArtSource = regexp.MustCompile(`^art-source/painted-location/[A-Za-z0-9_./-]+\.png$`)
	paintedPagesDir  = regexp.MustCompile(`^client-godot/assets/painted-location/[a-z0-9_.-]+$`)
	paintedFrameName = regexp.MustCompile(`^[a-z0-9-]+$`)
	paintedClasses   = regexp.MustCompile(`^tools/painted-location/[A-Za-z0-9_./-]+\.json$`)
)

// checkLivePart: a name pattern (match) or a level instance (instance, with or without parts), and why.
func checkLivePart(part PaintedLivePart) error {
	switch {
	case part.Match != "" && part.Instance != "":
		return fmt.Errorf("live_parts %q / %q: a name pattern or an instance, not both", part.Match, part.Instance)
	case part.Match != "":
		if !paintedNodeName.MatchString(part.Match) {
			return fmt.Errorf("live_parts %q: a node name or a name pattern (* ?)", part.Match)
		}
		if strings.Trim(part.Match, "*?") == "" {
			return fmt.Errorf("live_parts %q: a pattern of only wildcards makes the whole level live", part.Match)
		}
		if len(part.Parts) > 0 {
			return fmt.Errorf("live_parts %q: parts belong to an instance", part.Match)
		}
	case part.Instance != "":
		if !paintedInstance.MatchString(part.Instance) {
			return fmt.Errorf("live_parts instance %q: a level instance id", part.Instance)
		}
		for _, p := range part.Parts {
			if !paintedNodeName.MatchString(p) || strings.Trim(p, "*?") == "" {
				return fmt.Errorf("live_parts instance %s: part %q is a node name or a pattern, not only wildcards", part.Instance, p)
			}
		}
	default:
		return fmt.Errorf("a live part names a node pattern (match) or an instance")
	}
	if strings.TrimSpace(part.Why) == "" {
		return fmt.Errorf("live_parts %s%s: say what the server changes in it", part.Match, part.Instance)
	}
	return nil
}

// ServerDriven: one thing of a level whose state the server changes, as its data says, and the node names the client
// moves in it (Samples: names a live part must cover; empty: the instance as a whole).
type ServerDriven struct {
	System   string
	Instance string
	Samples  []string
}

// serverDrivenOn lists what the server changes on a level, from the compiled systems (never a hand list): the SDK gates of
// its layout, the trains whose route runs through it, the cargo cranes' bridges, lift decks, turrets, service cells
// (sanitary, medical), the torgomat block's parts, the goods and covers of the island counters, the lenses of its lamps.
func serverDrivenOn(c Compiled, level string) []ServerDriven {
	var out []ServerDriven
	layout := ""
	for _, m := range c.ModularLocations {
		if m.LevelID == level {
			layout = m.LayoutID
		}
	}
	if c.LocationSDK != nil && layout != "" {
		for _, l := range c.LocationSDK.Levels {
			if l.ID != layout {
				continue
			}
			for _, g := range l.Gates {
				out = append(out, ServerDriven{System: "gate", Instance: g.InstanceID})
			}
			lamps := false
			for _, i := range l.Instances {
				if strings.HasPrefix(i.ModuleID, "kit.geo.light.") {
					lamps = true
				}
			}
			if lamps {
				out = append(out, ServerDriven{System: "lamps (the lens glows while the lamp is on)", Samples: []string{"lamp_lens_0"}})
			}
		}
	}
	if t := c.TrainMotion; t != nil {
		on := false
		for _, b := range t.Bindings {
			on = on || b.LevelID == level
		}
		if on {
			out = append(out, ServerDriven{System: "train", Instance: t.TrainID})
			for _, s := range t.AdditionalFleet {
				out = append(out, ServerDriven{System: "train", Instance: s.TrainID})
			}
		}
	}
	if k := c.CargoConvoys; k != nil {
		for _, cr := range k.Cranes {
			if cr.LevelID == level {
				out = append(out, ServerDriven{System: "cargo crane", Instance: "crane.bridge"})
			}
		}
	}
	if d := c.LiftingDecks; d != nil {
		for _, x := range d.Decks {
			if x.LevelID == level {
				out = append(out, ServerDriven{System: "lifting deck " + x.ID, Instance: x.InstanceID, Samples: []string{"deck_small"}})
			}
		}
	}
	if w := c.TurretWatch; w != nil {
		for _, p := range w.Posts {
			if p.LevelID == level {
				out = append(out, ServerDriven{System: "turret", Instance: p.InstanceID})
			}
		}
	}
	if m := c.MedicalCapsules; m != nil {
		for _, u := range append(append([]MedicalUnit{}, m.Units...), m.Levels[level]...) {
			if u.LevelID == level {
				out = append(out, ServerDriven{System: "medical capsule " + u.ID, Instance: u.InstanceID})
			}
		}
	}
	if s := c.SanitaryServices; s != nil {
		for _, u := range s.Levels[level] {
			out = append(out, ServerDriven{System: "sanitary cell " + u.ID, Instance: u.InstanceID})
		}
	}
	if v := c.Vending; v != nil && v.Level == level {
		var samples []string
		for _, u := range clientVendingUnitIDs(v) {
			samples = append(samples, "flap_"+u, "lift_"+u, "fan_"+u, "sash_"+u, "screen_"+u, "signal_"+u)
		}
		samples = append(samples, "vm_goods", "port_shutter_east")
		out = append(out, ServerDriven{System: "torgomat block", Instance: v.Instance, Samples: samples})
	}
	if g := c.Craft; g != nil && g.Level == level {
		for _, row := range clientCraftCounters(g) {
			prefixes, _ := row["goods_prefixes"].([]string)
			var samples []string
			for _, p := range prefixes {
				samples = append(samples, p+"front")
			}
			samples = append(samples, "close_cage_0")
			out = append(out, ServerDriven{System: fmt.Sprintf("counter %v (goods by the shop's hours, covers while shut)", row["place"]),
				Instance: fmt.Sprint(row["island"]), Samples: samples})
		}
	}
	// one row per instance and system
	seen := map[string]bool{}
	var uniq []ServerDriven
	for _, x := range out {
		k := x.System + "|" + x.Instance + "|" + strings.Join(x.Samples, ",")
		if !seen[k] {
			seen[k] = true
			uniq = append(uniq, x)
		}
	}
	return uniq
}

func clientVendingUnitIDs(v *VendingCatalog) []string {
	var ids []string
	for _, u := range v.Units {
		ids = append(ids, u.ID)
	}
	return ids
}

// liveCovers: whether the level's live parts make this server-driven thing live — the instance as a whole, or every
// sample name matched by a name pattern (anywhere) or by the parts of an entry of its instance.
func liveCovers(parts []PaintedLivePart, x ServerDriven) bool {
	match := func(pattern, name string) bool {
		ok, err := path.Match(pattern, name)
		return err == nil && ok
	}
	for _, p := range parts {
		if x.Instance != "" && p.Instance == x.Instance && len(p.Parts) == 0 {
			return true
		}
	}
	if len(x.Samples) == 0 {
		return false
	}
	for _, s := range x.Samples {
		covered := false
		for _, p := range parts {
			if p.Match != "" && match(p.Match, s) {
				covered = true
			}
			if p.Instance != "" && p.Instance == x.Instance {
				for _, q := range p.Parts {
					covered = covered || match(q, s)
				}
			}
		}
		if !covered {
			return false
		}
	}
	return true
}

// checkPaintedLocationLevels: the level paintings of world.painted_location, against the compiled world.
func checkPaintedLocationLevels(c Compiled, root string) error {
	if c.PaintedLocation == nil || len(c.PaintedLocation.Levels) == 0 {
		return nil
	}
	var life struct {
		Locations map[string]json.RawMessage `json:"locations"`
	}
	if err := json.Unmarshal(c.Life, &life); err != nil {
		return err
	}
	known := func(id string) bool { _, ok := life.Locations[id]; return ok }
	if err := checkPaintedLevels(c.PaintedLocation.Levels, c, known, root); err != nil {
		return fmt.Errorf("world.painted_location: %w", err)
	}
	return nil
}

func checkPaintedLevels(levels []PaintedLevel, c Compiled, known func(string) bool, root string) error {
	ids := map[string]bool{}
	on := map[string]bool{}
	for _, l := range levels {
		if !paintedLevelID.MatchString(l.ID) || ids[l.ID] {
			return fmt.Errorf("level painting id %q: lower case, once", l.ID)
		}
		ids[l.ID] = true
		if on[l.LevelID] {
			return fmt.Errorf("level %s painted twice", l.LevelID)
		}
		on[l.LevelID] = true
		if err := checkPaintedLevel(l, c, known, root); err != nil {
			return fmt.Errorf("level painting %s: %w", l.ID, err)
		}
	}
	return nil
}

func checkPaintedLevel(l PaintedLevel, c Compiled, known func(string) bool, root string) error {
	if !known(l.LevelID) {
		return fmt.Errorf("unknown level %s", l.LevelID)
	}
	if strings.TrimSpace(l.Why) == "" {
		return fmt.Errorf("say why the level is painted")
	}
	p := l.Pages
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
	switch {
	case l.Picture != "none" && l.Picture != "pages":
		return fmt.Errorf("picture must be none or pages, not %q", l.Picture)
	case l.Look != "painting":
		return fmt.Errorf("look must be painting (the level's picture is the frame)")
	case !paintedPagesDir.MatchString(p.Folder):
		return fmt.Errorf("pages.folder %q: client-godot/assets/painted-location/<name>", p.Folder)
	case p.PagePx < 128 || p.PagePx > 2048 || p.PagePx%4 != 0:
		return fmt.Errorf("pages.page_px %d: 128..2048, a multiple of 4 (block compression)", p.PagePx)
	case p.GutterPx < 2 || p.GutterPx*4 > p.PagePx:
		return fmt.Errorf("pages.gutter_px %d: at least 2 (the depth test's neighbours), under a quarter of the page", p.GutterPx)
	case !finite(p.PxPerM) || p.PxPerM < 50 || p.PxPerM > 400:
		return fmt.Errorf("pages.px_per_m %v: 50..400 (under 50 the painting is coarser than the window at 16 m)", p.PxPerM)
	case !finite(p.MarginM) || p.MarginM < 0 || p.MarginM > 16:
		return fmt.Errorf("pages.margin_m %v: 0..16", p.MarginM)
	case p.Slots < 4 || p.Slots > 256:
		return fmt.Errorf("pages.slots %d: 4..256", p.Slots)
	case !finite(l.DepthToleranceM) || l.DepthToleranceM < 0.005 || l.DepthToleranceM > 0.2:
		return fmt.Errorf("depth_tolerance_m %v: 0.005..0.2", l.DepthToleranceM)
	case !finite(l.Strength) || l.Strength <= 0 || l.Strength > 1:
		return fmt.Errorf("strength %v: above 0, at most 1", l.Strength)
	}
	if err := checkPaintingLook(l.PaintingLook); err != nil {
		return fmt.Errorf("painting_look: %w", err)
	}
	if len(l.Frames) == 0 {
		return fmt.Errorf("frames: name the level's frames (the first is the start frame)")
	}
	frameNames := map[string]bool{}
	for _, f := range l.Frames {
		if !paintedFrameName.MatchString(f.Name) || frameNames[f.Name] {
			return fmt.Errorf("frames %q: lower case, digits and dashes, once", f.Name)
		}
		frameNames[f.Name] = true
		if !finite(f.SizeM) || f.SizeM < 3 || f.SizeM > 16 || !finite(f.Plan[0]) || !finite(f.Plan[1]) || !finite(f.Plan[2]) {
			return fmt.Errorf("frames %s: size_m 3..16 and a finite plan point", f.Name)
		}
	}
	for _, part := range l.LiveParts {
		if err := checkLivePart(part); err != nil {
			return err
		}
	}
	// Everything whose state the server changes must be live: a moving thing painted into the picture stays behind as
	// a ghost when it moves.
	var missing []string
	for _, x := range serverDrivenOn(c, l.LevelID) {
		if !liveCovers(l.LiveParts, x) {
			what := x.System
			if x.Instance != "" {
				what += " (instance " + x.Instance + ")"
			}
			if len(x.Samples) > 0 {
				what += " parts " + strings.Join(x.Samples, " ")
			}
			missing = append(missing, what)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("the server changes these, mark them live (live_parts: instance, or name patterns): %s", strings.Join(missing, "; "))
	}
	if err := checkPaintStep(l.Paint, root); err != nil {
		return fmt.Errorf("paint: %w", err)
	}
	if l.Picture == "pages" {
		if err := checkPaintedPages(p, root); err != nil {
			return fmt.Errorf("pages: %w", err)
		}
	}
	return nil
}

func checkPaintStep(s PaintStep, root string) error {
	m := s.Painters.Flux2Klein4B
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
	switch {
	case !paintedClasses.MatchString(s.Classes) || strings.Contains(s.Classes, ".."):
		return fmt.Errorf("classes %q: a JSON under tools/painted-location/", s.Classes)
	case !paintedArtSource.MatchString(s.StyleSample) || !paintedArtSource.MatchString(s.StyleKey.Painting):
		return fmt.Errorf("style_sample and style_key.painting: PNGs under art-source/painted-location/")
	case !paintedTileID.MatchString(s.StyleKey.Tile):
		return fmt.Errorf("style_key.tile %q: a tile id c<x>_<y>-t<i>_<j>", s.StyleKey.Tile)
	case s.Painter != "flux2_klein_4b":
		return fmt.Errorf("painter %q: flux2_klein_4b", s.Painter)
	case strings.TrimSpace(m.Prompt) == "" || m.Seed < 0 || m.Steps < 1 || m.Steps > 50 || strings.TrimSpace(m.Version) == "":
		return fmt.Errorf("painters.flux2_klein_4b: a prompt, a version, seed >= 0, steps 1..50")
	case !strings.HasPrefix(m.Model, ".local/") || !strings.HasPrefix(m.Tool, ".local/"):
		return fmt.Errorf("painters.flux2_klein_4b: the model and the tool live outside Git (.local/)")
	case m.WindowPx[0] < 64 || m.WindowPx[1] < 64 || m.InputPx[0] < 64 || m.InputPx[1] < 64 || m.OutputPx[0] < 64 || m.OutputPx[1] < 64:
		return fmt.Errorf("painters.flux2_klein_4b: window_px, input_px and output_px are sizes")
	case !finite(s.ForeignDE) || s.ForeignDE <= 0 || !finite(s.ForeignShare) || s.ForeignShare < 0 || s.ForeignShare > 1 || !finite(s.MaxDriftDE) || s.MaxDriftDE <= 0:
		return fmt.Errorf("foreign_de > 0, foreign_share 0..1, max_drift_de > 0")
	case s.Calm.Mottle < 0 || s.Calm.Mottle > 1 || s.Calm.Middle < 0 || s.Calm.Middle > 1:
		return fmt.Errorf("calm: mottle and middle 0..1")
	case strings.TrimSpace(s.Firefly) == "":
		return fmt.Errorf("firefly: the folder of the corner's approved paintings")
	}
	for _, f := range []string{s.StyleSample, s.StyleKey.Painting} {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f)))
		if err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
		if len(b) < 24 || string(b[1:4]) != "PNG" {
			return fmt.Errorf("%s is not a PNG", f)
		}
	}
	seen := map[string]bool{}
	for _, t := range s.Tiles {
		if !paintedTileID.MatchString(t.Tile) || seen[t.Tile] {
			return fmt.Errorf("tiles %q: a tile id, once", t.Tile)
		}
		seen[t.Tile] = true
		if t.Painter != "firefly" || strings.TrimSpace(t.Why) == "" {
			return fmt.Errorf("tiles %s: painter firefly and why", t.Tile)
		}
	}
	return nil
}

// PaintedPagesIndex is Folder/index.json, written by the paint step's pack: the plate (size, origin, scale, the depth's
// floor plane) and its pages (grid cell, files).
type PaintedPagesIndex struct {
	Plate struct {
		Px          [2]int     `json:"px"`
		OriginPx    [2]int     `json:"origin_px"`
		PxPerM      float64    `json:"px_per_m"`
		OriginWorld [3]float64 `json:"origin_world"`
		FloorK      float64    `json:"floor_k"`
		DepthUnitM  float64    `json:"depth_unit_m"`
	} `json:"plate"`
	PagePx   int    `json:"page_px"`
	GutterPx int    `json:"gutter_px"`
	Grid     [2]int `json:"grid"`
	// PictureFormat: "png" (or empty: RGB/RGBA PNG pages) or "bc7" (a DDS of BC7 with every mipmap, zstd-compressed:
	// the game copies it into the page array as it is); PictureBytes: a bc7 page's DDS size before zstd.
	PictureFormat string `json:"picture_format"`
	PictureBytes  int    `json:"picture_bytes"`
	Pages         []struct {
		At      [2]int `json:"at"`
		Picture string `json:"picture"`
		Depth   string `json:"depth"`
	} `json:"pages"`
	LiveWalls []string `json:"live_walls"`
}

func checkPaintedPages(p PaintedPages, root string) error {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(p.Folder), "index.json"))
	if err != nil {
		return fmt.Errorf("index.json: %w (run the paint step: tools/painted-location/market.py)", err)
	}
	var idx PaintedPagesIndex
	if err := json.Unmarshal(raw, &idx); err != nil {
		return fmt.Errorf("index.json: %w", err)
	}
	switch {
	case idx.PagePx != p.PagePx || idx.GutterPx != p.GutterPx:
		return fmt.Errorf("index.json pages %d/%d px, the data says %d/%d: pack again", idx.PagePx, idx.GutterPx, p.PagePx, p.GutterPx)
	case math.Abs(idx.Plate.PxPerM-p.PxPerM) > 1e-6:
		return fmt.Errorf("index.json %v px a metre, the data says %v: render and pack again", idx.Plate.PxPerM, p.PxPerM)
	case idx.Grid[0] < 1 || idx.Grid[1] < 1 || len(idx.Pages) == 0 || idx.Plate.DepthUnitM <= 0:
		return fmt.Errorf("index.json: no pages")
	}
	if err := checkPaintedPictureFormat(idx); err != nil {
		return err
	}
	cells := map[[2]int]bool{}
	for _, pg := range idx.Pages {
		if pg.At[0] < 0 || pg.At[1] < 0 || pg.At[0] >= idx.Grid[0] || pg.At[1] >= idx.Grid[1] || cells[pg.At] {
			return fmt.Errorf("page %v outside the grid %v or twice", pg.At, idx.Grid)
		}
		cells[pg.At] = true
		for _, f := range []struct {
			rel string
			rgb bool
		}{{pg.Picture, false}, {pg.Depth, true}} {
			if strings.Contains(f.rel, "..") || strings.HasPrefix(f.rel, "/") {
				return fmt.Errorf("page %v: %q is not a file of the folder", pg.At, f.rel)
			}
			var err error
			if f.rel == pg.Picture && idx.PictureFormat == "bc7" {
				err = checkPaintedBC7(root, path.Join(p.Folder, f.rel), idx.PictureBytes)
			} else {
				err = checkPaintedPNG(root, path.Join(p.Folder, f.rel), p.PagePx, p.PagePx, f.rgb)
			}
			if err != nil {
				return fmt.Errorf("page %v: %w", pg.At, err)
			}
		}
	}
	return nil
}

// checkPaintedPictureFormat: the index's picture pages are PNG, or BC7 DDS pages of the right size (a 148-byte DDS
// header with the DX10 part, then every mipmap of a page_px square down to 1 x 1 in 16-byte blocks of 4 x 4).
func checkPaintedPictureFormat(idx PaintedPagesIndex) error {
	switch idx.PictureFormat {
	case "", "png":
		return nil
	case "bc7":
		if want := 148 + bc7Bytes(idx.PagePx); idx.PictureBytes != want {
			return fmt.Errorf("index.json: a bc7 page of %d px is %d bytes before zstd, the index says %d: pack again", idx.PagePx, want, idx.PictureBytes)
		}
		return nil
	}
	return fmt.Errorf("index.json: picture_format %q (png or bc7)", idx.PictureFormat)
}

// bc7Bytes: a square BC7 picture of side px with every mipmap down to 1 x 1.
func bc7Bytes(px int) int {
	n := 0
	for s := px; ; s /= 2 {
		b := (s + 3) / 4
		n += b * b * 16
		if s <= 1 {
			return n
		}
	}
}

// checkPaintedBC7: the file is one zstd frame that states its size, and that size is the page's DDS (the pack's
// tools/painted-location/market_merge.py writes it with the game's Godot; the game checks the DDS itself as it loads).
func checkPaintedBC7(root, rel string, want int) error {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return fmt.Errorf("%s: %w", rel, err)
	}
	size, ok := zstdContentSize(raw)
	if !ok {
		return fmt.Errorf("%s is not a zstd frame with its size", rel)
	}
	if size != uint64(want) {
		return fmt.Errorf("%s holds %d bytes, a bc7 page is %d: pack again", rel, size, want)
	}
	return nil
}

// zstdContentSize: Frame_Content_Size of the zstd frame at the start of b (RFC 8878, 3.1.1), when the frame states it.
func zstdContentSize(b []byte) (uint64, bool) {
	if len(b) < 6 || binary.LittleEndian.Uint32(b) != 0xFD2FB528 {
		return 0, false
	}
	fhd := b[4]
	single := fhd&0x20 != 0
	at := 5
	if !single {
		at++ // Window_Descriptor
	}
	at += []int{0, 1, 2, 4}[fhd&3] // Dictionary_ID
	fcs := []int{0, 2, 4, 8}[fhd>>6]
	if fcs == 0 && single {
		fcs = 1
	}
	if fcs == 0 || len(b) < at+fcs {
		return 0, false
	}
	switch fcs {
	case 1:
		return uint64(b[at]), true
	case 2:
		return uint64(binary.LittleEndian.Uint16(b[at:])) + 256, true
	case 4:
		return uint64(binary.LittleEndian.Uint32(b[at:])), true
	}
	return binary.LittleEndian.Uint64(b[at:]), true
}

// clientPaintedLevels: what the client needs of the level paintings (no reasons, no paint step).
func clientPaintedLevels(levels []PaintedLevel) []any {
	res := func(rel string) string { return "res://" + strings.TrimPrefix(rel, "client-godot/") }
	out := []any{}
	for _, l := range levels {
		live := []string{}
		instances := []any{}
		for _, part := range l.LiveParts {
			if part.Match != "" {
				live = append(live, part.Match)
			} else {
				parts := part.Parts
				if parts == nil {
					parts = []string{}
				}
				instances = append(instances, map[string]any{"instance": part.Instance, "parts": parts})
			}
		}
		out = append(out, map[string]any{"id": l.ID, "level_id": l.LevelID, "picture": l.Picture, "pages": res(l.Pages.Folder),
			"page_px": l.Pages.PagePx, "gutter_px": l.Pages.GutterPx, "px_per_m": l.Pages.PxPerM, "margin_m": l.Pages.MarginM,
			"slots": l.Pages.Slots, "depth_tolerance_m": l.DepthToleranceM, "strength": l.Strength, "look": l.Look,
			"painting_look": l.PaintingLook, "live_parts": live, "live_instances": instances, "frames": l.Frames})
	}
	return out
}
