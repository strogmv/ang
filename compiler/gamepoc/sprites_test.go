package gamepoc

import (
	"bytes"
	"image/color"
	"testing"
)

func testArt() Art {
	art := Art{Palette: map[string]string{"a": "#445566", "b": "#554433", "c": "#334455", "k": "#222222", "t": "#778899", "l": "#556677", "r": "#334455"}}
	art.Tile.W, art.Tile.H = 64, 32
	art.Sprites = map[string]ArtSprite{
		"floor": {Shape: "ground", Style: "floor", Variants: 3, Colors: map[string]string{"a": "a", "b": "b", "c": "c", "seam": "k", "edge": "k", "scuff": "t", "rust": "b", "rust_dark": "k", "rivet": "k"}},
		"wall":  {Shape: "block", Style: "wall", Height: 44, Variants: 1, Colors: map[string]string{"top": "t", "left": "l", "right": "r", "trim": "t", "scrawl": "t", "rust": "b", "rust_dark": "k", "led": "a", "led_alt": "b"}},
		"soil":  {Shape: "block", Style: "soil", Height: 14, Variants: 1, Colors: map[string]string{"top": "b", "top_dark": "k", "left": "l", "right": "r", "slat": "k", "sprout": "a", "sprout_light": "t"}},
	}
	return art
}

func TestSpriteFilesAreDeterministicAndSized(t *testing.T) {
	art := testArt()
	first, err := SpriteFiles(art)
	if err != nil {
		t.Fatal(err)
	}
	second, _ := SpriteFiles(art)
	if len(first) != 5 {
		t.Fatalf("want 3 floor + wall + soil files, got %d", len(first))
	}
	for name, data := range first {
		if !bytes.Equal(data, second[name]) {
			t.Fatalf("%s differs between runs", name)
		}
	}
	for name, h := range map[string]int{"floor_0.png": 32, "wall.png": 32 + 44, "soil.png": 10 + 32 + 14} {
		img, err := pngDecode(first[name])
		if err != nil {
			t.Fatal(err)
		}
		if b := img.Bounds(); b.Dx() != 64 || b.Dy() != h {
			t.Fatalf("%s is %dx%d, want 64x%d", name, b.Dx(), b.Dy(), h)
		}
	}
}

func TestSpriteUnknownPaletteKeyRejected(t *testing.T) {
	art := testArt()
	s := art.Sprites["wall"]
	s.Colors = map[string]string{"top": "missing"}
	art.Sprites["wall"] = s
	if _, err := SpriteFiles(art); err == nil {
		t.Fatal("unknown palette key must be rejected")
	}
}

func TestSpriteVariantsGiveDifferentFloorPlates(t *testing.T) {
	files, err := SpriteFiles(testArt())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(files["floor_0.png"], files["floor_1.png"]) || bytes.Equal(files["floor_1.png"], files["floor_2.png"]) {
		t.Fatal("floor variants must differ")
	}
}

func cargoTestArt() Art {
	art := testArt()
	art.Palette["cargo_top"] = "#78868b"
	art.Palette["cargo_left"] = "#4c5d64"
	art.Palette["cargo_right"] = "#33434b"
	art.Palette["cargo_seam"] = "#202d35"
	art.Palette["cargo_frame"] = "#9ba9ad"
	art.Palette["cargo_label"] = "#d7ddd5"
	art.Palette["cargo_lock"] = "#b7a574"
	art.Sprites["crate"] = ArtSprite{Shape: "block", Style: "crate", Height: 20, Variants: 2, Colors: map[string]string{"top": "cargo_top", "left": "cargo_left", "right": "cargo_right", "slat": "cargo_seam", "trim": "cargo_frame", "label": "cargo_label", "lock": "cargo_lock"}}
	return art
}

func TestSpriteCargoDeterministicVariantsAndOriginalFootprint(t *testing.T) {
	art := cargoTestArt()
	first, err := SpriteFiles(art)
	if err != nil {
		t.Fatal(err)
	}
	second, err := SpriteFiles(art)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first["crate_0.png"], second["crate_0.png"]) || !bytes.Equal(first["crate_1.png"], second["crate_1.png"]) {
		t.Fatal("cargo rendering changed between runs")
	}
	if bytes.Equal(first["crate_0.png"], first["crate_1.png"]) {
		t.Fatal("used cassette variant is identical")
	}
	spec := art.Sprites["crate"]
	reference := spec
	reference.Style = "neutral_prism"
	prism, err := drawSprite(art, "crate", reference, 0)
	if err != nil {
		t.Fatal(err)
	}
	for variant := 0; variant < 2; variant++ {
		img, err := drawSprite(art, "crate", spec, variant)
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds() != prism.Bounds() || img.Bounds().Dx() != 64 || img.Bounds().Dy() != 52 {
			t.Fatalf("cargo changed sprite extent: %v", img.Bounds())
		}
		for y := 0; y < img.Bounds().Dy(); y++ {
			for x := 0; x < img.Bounds().Dx(); x++ {
				if img.NRGBAAt(x, y).A != prism.NRGBAAt(x, y).A {
					t.Fatalf("cargo changed footprint at %d,%d variant %d", x, y, variant)
				}
			}
		}
	}
}

func TestSpriteCargoPanelLocksAndLabelUseCUETokens(t *testing.T) {
	art := cargoTestArt()
	spec := art.Sprites["crate"]
	img, err := drawSprite(art, "crate", spec, 0)
	if err != nil {
		t.Fatal(err)
	}
	label, _ := parseHex(art.Palette["cargo_label"])
	lock, _ := parseHex(art.Palette["cargo_lock"])
	labels, locks := 0, 0
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			pixel := img.NRGBAAt(x, y)
			if pixel == label {
				labels++
			}
			if pixel == lock {
				locks++
			}
		}
	}
	if labels < 8 || locks < 12 {
		t.Fatalf("missing ID panel or corner couplings: %d label/%d lock pixels", labels, locks)
	}
	art.Palette["cargo_label"] = "#fa32dd"
	changed, err := drawSprite(art, "crate", spec, 0)
	if err != nil {
		t.Fatal(err)
	}
	changedColor, _ := parseHex(art.Palette["cargo_label"])
	count := 0
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			if img.NRGBAAt(x, y) == label {
				if changed.NRGBAAt(x, y) != changedColor {
					t.Fatal("authored label token ignored")
				}
				count++
			}
		}
	}
	if count != labels {
		t.Fatal("label panel became nondeterministic")
	}
}

func TestSpriteCargoLegacyRecipeHasOpaqueFallbacks(t *testing.T) {
	art := cargoTestArt()
	spec := art.Sprites["crate"]
	for _, role := range []string{"trim", "label", "lock"} {
		delete(spec.Colors, role)
	}
	img, err := drawSprite(art, "crate", spec, 1)
	if err != nil {
		t.Fatal(err)
	}
	reference := spec
	reference.Style = "neutral_prism"
	prism, err := drawSprite(art, "crate", reference, 1)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			if img.NRGBAAt(x, y).A != prism.NRGBAAt(x, y).A {
				t.Fatalf("optional role made transparent hole at %d,%d", x, y)
			}
		}
	}
}

func TestSpriteCargoHasNoPeriodicWoodSlats(t *testing.T) {
	art := cargoTestArt()
	c := canvas{roles: art.Sprites["crate"].Colors, palette: map[string]color.NRGBA{}}
	for key, hex := range art.Palette {
		c.palette[key], _ = parseHex(hex)
	}
	base := c.palette["cargo_left"]
	for _, v := range []int{6, 13} {
		if got := c.cargoFace(left, 10, v, 32, 20, 0, base); got != base {
			t.Fatalf("unexpected plank stripe at v=%d: %v", v, got)
		}
	}
}
