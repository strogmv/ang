package gamepoc

import (
	"bytes"
	"image/color"
	"testing"
)

func floorTestMaterial() *FloorMaterial {
	return &FloorMaterial{PanelCellsX: 4, PanelCellsY: 2, TextureWidth: 512, SeamWidthMM: 8, BevelWidthMM: 5, Base: "#505b59", Seam: "#323e3d", Bevel: "#697370"}
}
func TestFloorMaterialDeterministicBoundedAndClean(t *testing.T) {
	art := testArt()
	art.Floor = floorTestMaterial()
	a, e := SpriteFiles(art)
	if e != nil {
		t.Fatal(e)
	}
	b, e := SpriteFiles(art)
	if e != nil {
		t.Fatal(e)
	}
	if len(a) != 9 {
		t.Fatalf("want5 legacy+4 fragments got%d", len(a))
	}
	for name, data := range a {
		if !bytes.Equal(data, b[name]) {
			t.Fatal(name)
		}
	}
	base, _ := parseHex(art.Floor.Base)
	for mask := 0; mask < 4; mask++ {
		img := drawFloorMaterial(art.Floor, mask)
		if img.Bounds().Dx() != 512 || img.Bounds().Dy() != 256 {
			t.Fatal(img.Bounds())
		}
		colors := map[color.NRGBA]bool{}
		for y := 0; y < 256; y++ {
			for x := 0; x < 512; x++ {
				c := img.NRGBAAt(x, y)
				if c.A != 0 {
					colors[c] = true
				}
				if mask == 0 && c.A != 0 && c != base {
					t.Fatal("interior must have no metre-cell seam or noise")
				}
			}
		}
		if mask == 0 && len(colors) != 1 || mask != 0 && len(colors) != 3 {
			t.Fatalf("mask%d colors%d", mask, len(colors))
		}
		if img.NRGBAAt(0, 0).A != 0 || img.NRGBAAt(256, 128).A != 255 {
			t.Fatal("diamond footprint")
		}
	}
}
func TestFloorMaterialOptionalValidationAndProjection(t *testing.T) {
	if e := checkFloorMaterial(nil); e != nil {
		t.Fatal(e)
	}
	f := floorTestMaterial()
	got := clientArt(Compiled{Art: Art{Floor: f}})["floor"]
	if got != f {
		t.Fatal("floor material missing from client projection")
	}
	for _, mutate := range []func(*FloorMaterial){func(f *FloorMaterial) { f.PanelCellsX = 0 }, func(f *FloorMaterial) { f.PanelCellsY = 17 }, func(f *FloorMaterial) { f.TextureWidth = 127 }, func(f *FloorMaterial) { f.TextureWidth = 258 }, func(f *FloorMaterial) { f.SeamWidthMM = 31 }, func(f *FloorMaterial) { f.BevelWidthMM = -1 }, func(f *FloorMaterial) { f.Base = "bad" }} {
		v := *f
		mutate(&v)
		if e := checkFloorMaterial(&v); e == nil {
			t.Fatalf("accepted%+v", v)
		}
	}
}
