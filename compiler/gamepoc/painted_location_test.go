package gamepoc

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

const paintedDir = "client-godot/assets/painted-location/c7_market-torgomat/"

// paintedCornerFixture: a game root with the stand-in and the depth map at 1280x800 (depth RGB, stand-in RGBA is fine).
func paintedCornerFixture(t *testing.T) (PaintedLocation, func(string) bool, string) {
	t.Helper()
	root := t.TempDir()
	writePaintedPNG(t, root, paintedDir+"stand-in.png", 1280, 800, false)
	writePaintedPNG(t, root, paintedDir+"depth.png", 1280, 800, true)
	c := PaintedCorner{ID: "c7_market.torgomat_corner", LevelID: "level.market", Why: "the owner compares the frame with Pillars",
		FocusPlan: [3]float64{32.5, 21.0, 1.0}, SizeM: 12, RenderPx: [2]int{1280, 800}, Picture: "stand_in",
		StandIn: paintedDir + "stand-in.png", Painted: paintedDir + "painted.png", Depth: paintedDir + "depth.png",
		DepthToleranceM: 0.04, EdgeM: 1.5, Strength: 1,
		LiveParts: []PaintedLivePart{{Match: "flap_tm*", Why: "the dispense flap opens"}, {Match: "gate.passage", Why: "its leaves move"}}}
	known := func(id string) bool { return id == "level.market" || id == "level.hub" }
	return PaintedLocation{Mode: "now", Corners: []PaintedCorner{c}}, known, root
}

func writePaintedPNG(t *testing.T, root, rel string, w, h int, rgb bool) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var img image.Image
	if rgb {
		// Opaque RGBA encodes as colour type 2 (RGB) in Go's encoder.
		m := image.NewRGBA(image.Rect(0, 0, w, h))
		for i := range m.Pix {
			m.Pix[i] = 255
		}
		img = m
	} else {
		m := image.NewNRGBA(image.Rect(0, 0, w, h))
		m.Set(0, 0, color.NRGBA{10, 20, 30, 0})
		img = m
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestPaintedLocationAcceptedAndProjected(t *testing.T) {
	p, known, root := paintedCornerFixture(t)
	if err := checkPaintedLocation(p, known, root); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(clientPaintedLocation(&p))
	for _, want := range []string{`"mode":"now"`, `"id":"c7_market.torgomat_corner"`, `"level_id":"level.market"`,
		`"picture":"res://assets/painted-location/c7_market-torgomat/stand-in.png"`, `"picture_kind":"stand_in"`,
		`"depth":"res://assets/painted-location/c7_market-torgomat/depth.png"`, `"focus_plan":[32.5,21,1]`, `"render_px":[1280,800]`,
		`"depth_tolerance_m":0.04`, `"edge_m":1.5`, `"strength":1`, `"live_parts":["flap_tm*","gate.passage"]`, `"size_m":12`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("client painted location lacks %s:\n%s", want, raw)
		}
	}
	if strings.Contains(string(raw), "why") || strings.Contains(string(raw), "stand_in\":\"client-godot") {
		t.Fatalf("the reason and the source paths stay in CUE: %s", raw)
	}
	// The painted mode with the coordinator's painting in place; edges of the numbers.
	writePaintedPNG(t, root, paintedDir+"painted.png", 1280, 800, false)
	for _, edge := range []func(p *PaintedLocation){
		func(p *PaintedLocation) { p.Mode, p.Corners[0].Picture = "painted", "painted" },
		func(p *PaintedLocation) { p.Corners[0].SizeM, p.Corners[0].EdgeM = 9, 0 },
		func(p *PaintedLocation) { p.Corners[0].DepthToleranceM, p.Corners[0].Strength = 0.2, 0.01 },
		func(p *PaintedLocation) { p.Corners[0].LiveParts = nil },
		func(p *PaintedLocation) {
			p.Corners = append(p.Corners, p.Corners[0])
			p.Corners[1].ID, p.Corners[1].LevelID = "hub.x", "level.hub"
		},
	} {
		p, known, _ := paintedCornerFixture(t)
		edge(&p)
		if err := checkPaintedLocation(p, known, root); err != nil {
			t.Fatalf("%+v: %v", p, err)
		}
	}
	p.Corners[0].Picture = "painted"
	raw, _ = json.Marshal(clientPaintedLocation(&p))
	if !strings.Contains(string(raw), `"picture":"res://assets/painted-location/c7_market-torgomat/painted.png"`) {
		t.Fatalf("the painted picture is not chosen: %s", raw)
	}
}

func TestPaintedLocationRejected(t *testing.T) {
	cases := map[string]func(p *PaintedLocation, root string){
		"mode_unknown": func(p *PaintedLocation, _ string) { p.Mode = "baked" },
		"no_corners":   func(p *PaintedLocation, _ string) { p.Corners = nil },
		"bad_id":       func(p *PaintedLocation, _ string) { p.Corners[0].ID = "Corner One" },
		"twice":        func(p *PaintedLocation, _ string) { p.Corners = append(p.Corners, p.Corners[0]) },
		"level_twice": func(p *PaintedLocation, _ string) {
			p.Corners = append(p.Corners, p.Corners[0])
			p.Corners[1].ID = "c7_market.other"
		},
		"unknown_level":    func(p *PaintedLocation, _ string) { p.Corners[0].LevelID = "level.moon" },
		"no_reason":        func(p *PaintedLocation, _ string) { p.Corners[0].Why = " " },
		"size_small":       func(p *PaintedLocation, _ string) { p.Corners[0].SizeM = 8 },
		"size_large":       func(p *PaintedLocation, _ string) { p.Corners[0].SizeM = 17 },
		"size_nan":         func(p *PaintedLocation, _ string) { p.Corners[0].SizeM = math.NaN() },
		"focus_inf":        func(p *PaintedLocation, _ string) { p.Corners[0].FocusPlan[1] = math.Inf(1) },
		"render_portrait":  func(p *PaintedLocation, _ string) { p.Corners[0].RenderPx = [2]int{800, 1280} },
		"render_tiny":      func(p *PaintedLocation, _ string) { p.Corners[0].RenderPx = [2]int{320, 200} },
		"render_coarse":    func(p *PaintedLocation, _ string) { p.Corners[0].SizeM = 16; p.Corners[0].RenderPx = [2]int{1200, 750} },
		"tolerance_tight":  func(p *PaintedLocation, _ string) { p.Corners[0].DepthToleranceM = 0.001 },
		"tolerance_loose":  func(p *PaintedLocation, _ string) { p.Corners[0].DepthToleranceM = 0.5 },
		"edge_negative":    func(p *PaintedLocation, _ string) { p.Corners[0].EdgeM = -1 },
		"edge_whole_frame": func(p *PaintedLocation, _ string) { p.Corners[0].EdgeM = 6 },
		"strength_zero":    func(p *PaintedLocation, _ string) { p.Corners[0].Strength = 0 },
		"strength_over":    func(p *PaintedLocation, _ string) { p.Corners[0].Strength = 1.2 },
		"picture_unknown":  func(p *PaintedLocation, _ string) { p.Corners[0].Picture = "photo" },
		"painted_missing":  func(p *PaintedLocation, _ string) { p.Corners[0].Picture = "painted" },
		"depth_missing":    func(p *PaintedLocation, root string) { os.Remove(filepath.Join(root, paintedDir+"depth.png")) },
		"stand_in_missing": func(p *PaintedLocation, root string) { os.Remove(filepath.Join(root, paintedDir+"stand-in.png")) },
		"depth_rgba": func(p *PaintedLocation, root string) {
			writePaintedPNG(t, root, paintedDir+"depth.png", 1280, 800, false)
		},
		"depth_wrong_size": func(p *PaintedLocation, root string) {
			writePaintedPNG(t, root, paintedDir+"depth.png", 1280, 801, true)
		},
		"picture_wrong": func(p *PaintedLocation, root string) {
			writePaintedPNG(t, root, paintedDir+"stand-in.png", 2560, 1600, false)
		},
		"picture_not_png": func(p *PaintedLocation, root string) {
			os.WriteFile(filepath.Join(root, paintedDir+"stand-in.png"), []byte("not a png at all, really not"), 0o644)
		},
		"path_outside": func(p *PaintedLocation, _ string) { p.Corners[0].Depth = "client-godot/assets/depth.png" },
		"path_escape":  func(p *PaintedLocation, _ string) { p.Corners[0].StandIn = paintedDir + "../../../../etc/x.png" },
		"path_not_png": func(p *PaintedLocation, _ string) { p.Corners[0].Painted = paintedDir + "painted.jpg" },
		"same_files":   func(p *PaintedLocation, _ string) { p.Corners[0].Painted = p.Corners[0].StandIn },
		"live_part_path": func(p *PaintedLocation, _ string) {
			p.Corners[0].LiveParts = []PaintedLivePart{{Match: "Location/Visitors", Why: "people"}}
		},
		"live_part_twice": func(p *PaintedLocation, _ string) {
			p.Corners[0].LiveParts = []PaintedLivePart{{Match: "fan_*", Why: "turns"}, {Match: "fan_*", Why: "turns"}}
		},
		"live_part_empty": func(p *PaintedLocation, _ string) { p.Corners[0].LiveParts = []PaintedLivePart{{Match: "", Why: "x"}} },
		"live_part_everything": func(p *PaintedLocation, _ string) {
			p.Corners[0].LiveParts = []PaintedLivePart{{Match: "*", Why: "all"}}
		},
		"live_part_no_reason": func(p *PaintedLocation, _ string) {
			p.Corners[0].LiveParts = []PaintedLivePart{{Match: "fan_*", Why: " "}}
		},
		"tolerance_nan":     func(p *PaintedLocation, _ string) { p.Corners[0].DepthToleranceM = math.NaN() },
		"render_huge_width": func(p *PaintedLocation, _ string) { p.Corners[0].RenderPx = [2]int{9000, 1600} },
	}
	for name, spoil := range cases {
		t.Run(name, func(t *testing.T) {
			p, known, root := paintedCornerFixture(t)
			spoil(&p, root)
			if err := checkPaintedLocation(p, known, root); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}

func TestPaintedLocationRead(t *testing.T) {
	_, _, root := paintedCornerFixture(t)
	ctx := cuecontext.New()
	c := Compiled{Life: json.RawMessage(`{"locations":{"level.market":{},"level.hub":{}}}`)}
	corner := `{id: "c7_market.torgomat_corner", level_id: "level.market", why: "Pillars", focus_plan: [32.5, 21.0, 1.0], size_m: 12,
		render_px: [1280, 800], picture: "stand_in", stand_in: "` + paintedDir + `stand-in.png", painted: "` + paintedDir + `painted.png",
		depth: "` + paintedDir + `depth.png", depth_tolerance_m: 0.04, edge_m: 1.5, strength: 1, live_parts: [{match: "fan_*", why: "turns"}]%s}`
	p, err := readPaintedLocation(ctx.CompileString(`painted_location: {mode: "now", corners: [`+strings.Replace(corner, "%s", "", 1)+`]}`), c, root)
	if err != nil || p == nil || len(p.Corners) != 1 || p.Corners[0].FocusPlan != [3]float64{32.5, 21, 1} || p.Corners[0].RenderPx != [2]int{1280, 800} {
		t.Fatalf("read %+v, %v", p, err)
	}
	if p, err := readPaintedLocation(ctx.CompileString(`other: 1`), c, root); err != nil || p != nil {
		t.Fatalf("a world without the block: %+v, %v", p, err)
	}
	for name, src := range map[string]string{
		"unknown_field":             `painted_location: {mode: "now", corners: [` + strings.Replace(corner, "%s", ", bake_light: true", 1) + `]}`,
		"short_focus":               `painted_location: {mode: "now", corners: [` + strings.Replace(strings.Replace(corner, "%s", "", 1), "[32.5, 21.0, 1.0]", "[32.5, 21.0]", 1) + `]}`,
		"short_render":              `painted_location: {mode: "now", corners: [` + strings.Replace(strings.Replace(corner, "%s", "", 1), "[1280, 800]", "[640]", 1) + `]}`,
		"string_size":               `painted_location: {mode: "now", corners: [` + strings.Replace(strings.Replace(corner, "%s", "", 1), "size_m: 12", `size_m: "12"`, 1) + `]}`,
		"no_corners":                `painted_location: {mode: "now", corners: []}`,
		"mode_painted_missing_file": `painted_location: {mode: "painted", corners: [` + strings.Replace(strings.Replace(corner, "%s", "", 1), `picture: "stand_in"`, `picture: "painted"`, 1) + `]}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := readPaintedLocation(ctx.CompileString(src), c, root); err == nil || !strings.Contains(err.Error(), "world.painted_location") {
				t.Fatalf("accepted or unnamed: %v", err)
			}
		})
	}
}
