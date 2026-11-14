package gamepoc

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/locationsdk"
)

// paintedLevelFixture: a game root with the style sample and key, a level painting of level.market with every
// server-driven thing of the compiled fixture live.
func paintedLevelFixture(t *testing.T) (PaintedLevel, Compiled, func(string) bool, string) {
	t.Helper()
	root := t.TempDir()
	writePaintedPNG(t, root, "art-source/painted-location/market-v1/style-floor.png", 64, 64, true)
	writePaintedPNG(t, root, "art-source/painted-location/market-v1/style-key.png", 64, 64, true)
	l := PaintedLevel{ID: "c7_market", LevelID: "level.market", Why: "the owner prefers the painted frame", Picture: "none",
		Pages:           PaintedPages{Folder: "client-godot/assets/painted-location/c7_market", PagePx: 512, GutterPx: 8, PxPerM: 133.333, MarginM: 2, Slots: 48},
		DepthToleranceM: 0.04, Strength: 1, Look: "painting",
		LiveParts: []PaintedLivePart{{Instance: "gate.passage", Why: "the leaf lifts"}, {Instance: "vending.block", Parts: []string{"fan_*", "flap_*", "lift_*", "sash_*", "screen_*", "signal_*", "vm_*", "port_shutter_*"}, Why: "the torgomat's parts"},
			{Match: "lamp_lens_*", Why: "lenses glow"}}}
	l.PaintingLook = PaintingLook{LampGain: 1, LampWrap: 0.3, HighlightCeiling: 2, Shadow: PaintingShade{Strength: 0.5, Tint: [3]float64{0.4, 0.5, 0.6}},
		PeopleReadKeep: 0, Live: PaintingLive{Tint: [3]float64{1, 1, 1}, Saturation: 1, KeyGain: 0.8, LampGain: 0.7}}
	for _, g := range []*PaintingGrade{&l.PaintingLook.Grade.Day, &l.PaintingLook.Grade.Dusk, &l.PaintingLook.Grade.Night} {
		*g = PaintingGrade{Value: 1, Tint: [3]float64{1, 1, 1}, Saturation: 1}
	}
	l.Frames = []PaintedFrame{{Name: "start-12m", Plan: [3]float64{44.25, 16.75, 0.9}, SizeM: 12, AtHero: true}}
	l.Paint.Classes = "tools/painted-location/classes.json"
	l.Paint.StyleSample = "art-source/painted-location/market-v1/style-floor.png"
	l.Paint.StyleKey.Tile = "c0_0-t6_3"
	l.Paint.StyleKey.Painting = "art-source/painted-location/market-v1/style-key.png"
	l.Paint.Painter = "flux2_klein_4b"
	l.Paint.Painters.Flux2Klein4B = PaintModel{Model: ".local/downloads/model", Tool: ".local/downloads/venv/bin/tool", Version: "mflux 0.24.0",
		Prompt: "repaint image 1 in the paint of image 2", Seed: 1114, Steps: 4, WindowPx: [2]int{1152, 648}, InputPx: [2]int{1920, 1080}, OutputPx: [2]int{1360, 768}, LowRAM: true}
	l.Paint.Firefly = ".local/corner"
	l.Paint.ForeignDE, l.Paint.ForeignShare, l.Paint.MaxDriftDE = 15, 0.04, 15
	l.Paint.Calm.Mottle, l.Paint.Calm.Middle = 0.6, 0.3
	c := Compiled{Vending: &VendingCatalog{Instance: "vending.block", Level: "level.market"}}
	c.Vending.Units = []VendingUnit{{ID: "tm1"}}
	c.ModularLocations = []ModularLocation{{LevelID: "level.market", LayoutID: "example.market"}}
	known := func(id string) bool { return id == "level.market" }
	return l, c, known, root
}

func TestPaintedLevelAccepted(t *testing.T) {
	l, c, known, root := paintedLevelFixture(t)
	if err := checkPaintedLevels([]PaintedLevel{l}, c, known, root); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(clientPaintedLevels([]PaintedLevel{l}))
	for _, want := range []string{`"pages":"res://assets/painted-location/c7_market"`, `"picture":"none"`, `"page_px":512`, `"live_parts":["lamp_lens_*"]`,
		`"live_instances":[{"instance":"gate.passage","parts":[]},{"instance":"vending.block","parts":["fan_*"`,
		`"frames":[{"name":"start-12m","plan":[44.25,16.75,0.9],"size_m":12,"at_hero":true}]`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("client level painting lacks %s:\n%s", want, raw)
		}
	}
	if strings.Contains(string(raw), "prompt") || strings.Contains(string(raw), "why") {
		t.Fatalf("the paint step and the reasons stay in CUE: %s", raw)
	}
}

// Every thing the server changes must be live: a torgomat part left out of the live parts fails the build.
func TestPaintedLevelServerDrivenMustBeLive(t *testing.T) {
	l, c, known, root := paintedLevelFixture(t)
	l.LiveParts[1].Parts = []string{"fan_*", "lift_*", "sash_*", "screen_*", "signal_*", "vm_*", "port_shutter_*"}
	err := checkPaintedLevels([]PaintedLevel{l}, c, known, root)
	if err == nil || !strings.Contains(err.Error(), "torgomat block") || !strings.Contains(err.Error(), "flap_tm1") {
		t.Fatalf("a torgomat without its flaps live must fail, got %v", err)
	}
	// a name pattern anywhere covers it too
	l.LiveParts = append(l.LiveParts, PaintedLivePart{Match: "flap_tm*", Why: "flaps open"})
	if err := checkPaintedLevels([]PaintedLevel{l}, c, known, root); err != nil {
		t.Fatal(err)
	}
	// craft counters: goods and covers
	c.Craft = &CraftCatalog{Level: "level.market"}
	if got := serverDrivenOn(c, "level.market"); len(got) != 1 {
		t.Fatalf("no counters, no counter rows: %v", got)
	}
}

func TestPaintedLevelRejects(t *testing.T) {
	cases := map[string]func(*PaintedLevel){
		"picture":      func(l *PaintedLevel) { l.Picture = "png" },
		"look":         func(l *PaintedLevel) { l.Look = "lit" },
		"page size":    func(l *PaintedLevel) { l.Pages.PagePx = 510 },
		"gutter":       func(l *PaintedLevel) { l.Pages.GutterPx = 1 },
		"scale":        func(l *PaintedLevel) { l.Pages.PxPerM = 20 },
		"steps":        func(l *PaintedLevel) { l.Paint.Painters.Flux2Klein4B.Steps = 0 },
		"model in git": func(l *PaintedLevel) { l.Paint.Painters.Flux2Klein4B.Model = "client-godot/model" },
		"tile twice": func(l *PaintedLevel) {
			l.Paint.Tiles = []PaintTile{{"c0_0-t6_3", "firefly", "key"}, {"c0_0-t6_3", "firefly", "key"}}
		},
		"bad live": func(l *PaintedLevel) { l.LiveParts = append(l.LiveParts, PaintedLivePart{Match: "*", Why: "all"}) },
		"both": func(l *PaintedLevel) {
			l.LiveParts = append(l.LiveParts, PaintedLivePart{Match: "a*", Instance: "b", Why: "x"})
		},
		"no why":          func(l *PaintedLevel) { l.LiveParts = append(l.LiveParts, PaintedLivePart{Instance: "gate.entry"}) },
		"style sample":    func(l *PaintedLevel) { l.Paint.StyleSample = "art-source/painted-location/missing.png" },
		"foreign share":   func(l *PaintedLevel) { l.Paint.ForeignShare = 2 },
		"pages unbuilt":   func(l *PaintedLevel) { l.Picture = "pages" },
		"gate not live":   func(l *PaintedLevel) { l.LiveParts = l.LiveParts[1:] },
		"unknown level":   func(l *PaintedLevel) { l.LevelID = "level.nowhere" },
		"slots":           func(l *PaintedLevel) { l.Pages.Slots = 2 },
		"calm":            func(l *PaintedLevel) { l.Paint.Calm.Mottle = 1.5 },
		"wrong painter":   func(l *PaintedLevel) { l.Paint.Painter = "firefly" },
		"tile painter":    func(l *PaintedLevel) { l.Paint.Tiles = []PaintTile{{"c0_0-t6_3", "flux", "key"}} },
		"depth tolerance": func(l *PaintedLevel) { l.DepthToleranceM = 0.5 },
		"no frames":       func(l *PaintedLevel) { l.Frames = nil },
		"frame twice":     func(l *PaintedLevel) { l.Frames = append(l.Frames, l.Frames[0]) },
		"frame size":      func(l *PaintedLevel) { l.Frames[0].SizeM = 40 },
		"classes":         func(l *PaintedLevel) { l.Paint.Classes = "../classes.json" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			l, c, known, root := paintedLevelFixture(t)
			c.LocationSDK = nil
			if name == "gate not live" {
				c = gateFixture(c)
			}
			mutate(&l)
			if err := checkPaintedLevels([]PaintedLevel{l}, c, known, root); err == nil {
				t.Fatalf("%s: accepted", name)
			}
		})
	}
}

// The packed pages: index.json and every page at its size (picture RGB or RGBA, depth plain RGB).
func TestPaintedLevelPages(t *testing.T) {
	l, c, known, root := paintedLevelFixture(t)
	l.Picture = "pages"
	dir := filepath.Join(root, filepath.FromSlash(l.Pages.Folder))
	writePaintedPNG(t, root, l.Pages.Folder+"/picture/0_0.png", 512, 512, false)
	writePaintedPNG(t, root, l.Pages.Folder+"/depth/0_0.png", 512, 512, true)
	idx := `{"plate":{"px":[1000,800],"origin_px":[-10,-20],"px_per_m":133.333,"origin_world":[0,0,0],"floor_k":1.4,"depth_unit_m":0.002},
		"page_px":512,"gutter_px":8,"grid":[3,2],"pages":[{"at":[0,0],"picture":"picture/0_0.png","depth":"depth/0_0.png"}]}`
	if err := os.WriteFile(filepath.Join(dir, "index.json"), []byte(idx), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkPaintedLevels([]PaintedLevel{l}, c, known, root); err != nil {
		t.Fatal(err)
	}
	// a depth page with alpha is not plain bytes of depth
	writePaintedPNG(t, root, l.Pages.Folder+"/depth/0_0.png", 512, 512, false)
	if err := checkPaintedLevels([]PaintedLevel{l}, c, known, root); err == nil || !strings.Contains(err.Error(), "8-bit RGB") {
		t.Fatalf("a depth page with alpha must fail, got %v", err)
	}
	// a page outside the grid
	writePaintedPNG(t, root, l.Pages.Folder+"/depth/0_0.png", 512, 512, true)
	bad := strings.Replace(idx, `"at":[0,0]`, `"at":[3,0]`, 1)
	if err := os.WriteFile(filepath.Join(dir, "index.json"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkPaintedLevels([]PaintedLevel{l}, c, known, root); err == nil {
		t.Fatal("a page outside the grid must fail")
	}
}

// BC7 picture pages: one zstd frame stating the DDS size of a page (148-byte header + every mipmap in 4 x 4 blocks;
// Godot 4.7.2 writes 349 700 bytes for a 512 px page).
func TestPaintedLevelPagesBC7(t *testing.T) {
	if got := bc7Bytes(512); got != 349552 {
		t.Fatalf("bc7Bytes(512) = %d, Godot's BPTC with mipmaps is 349552", got)
	}
	l, c, known, root := paintedLevelFixture(t)
	l.Picture = "pages"
	dir := filepath.Join(root, filepath.FromSlash(l.Pages.Folder))
	writePaintedPNG(t, root, l.Pages.Folder+"/depth/0_0.png", 512, 512, true)
	frame := func(size uint32) []byte {
		b := []byte{0x28, 0xB5, 0x2F, 0xFD, 0xA0, 0, 0, 0, 0, 1, 0, 0}
		binary.LittleEndian.PutUint32(b[5:], size)
		return b
	}
	write := func(rel string, b []byte) {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, rel)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, rel), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("picture/0_0.dds.zst", frame(349700))
	idx := `{"plate":{"px":[1000,800],"origin_px":[-10,-20],"px_per_m":133.333,"origin_world":[0,0,0],"floor_k":1.4,"depth_unit_m":0.002},
		"page_px":512,"gutter_px":8,"grid":[3,2],"picture_format":"bc7","picture_bytes":349700,
		"pages":[{"at":[0,0],"picture":"picture/0_0.dds.zst","depth":"depth/0_0.png"}]}`
	write("index.json", []byte(idx))
	if err := checkPaintedLevels([]PaintedLevel{l}, c, known, root); err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		index string
		page  []byte
		want  string
	}{
		"page size":  {idx, frame(349699), "a bc7 page is"},
		"not zstd":   {idx, []byte("DDS |not compressed"), "not a zstd frame"},
		"index size": {strings.Replace(idx, `"picture_bytes":349700`, `"picture_bytes":349552`, 1), frame(349700), "pack again"},
		"format":     {strings.Replace(idx, `"bc7"`, `"astc"`, 1), frame(349700), "picture_format"},
	} {
		write("index.json", []byte(tc.index))
		write("picture/0_0.dds.zst", tc.page)
		if err := checkPaintedLevels([]PaintedLevel{l}, c, known, root); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: want an error with %q, got %v", name, tc.want, err)
		}
	}
}

// gateFixture: the compiled SDK layout of level.market with one gate (the server lifts its leaf).
func gateFixture(c Compiled) Compiled {
	c.LocationSDK = &locationsdk.RuntimeBundle{Levels: []locationsdk.RuntimeLevel{{ID: "example.market", Gates: []locationsdk.RuntimeGate{{InstanceID: "gate.passage"}}}}}
	return c
}
