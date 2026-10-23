package gamepoc

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
)

// FloorMaterial describes presentation panels spanning several navigation cells.
// Four boundary masks are enough for every panel size; no per-cell image cache.
type FloorMaterial struct {
	PanelCellsX  int    `json:"panel_cells_x"`
	PanelCellsY  int    `json:"panel_cells_y"`
	TextureWidth int    `json:"texture_width"`
	SeamWidthMM  int    `json:"seam_width_mm"`
	BevelWidthMM int    `json:"bevel_width_mm"`
	Base         string `json:"base"`
	Seam         string `json:"seam"`
	Bevel        string `json:"bevel"`
}

func checkFloorMaterial(f *FloorMaterial) error {
	if f == nil {
		return nil
	}
	if f.PanelCellsX < 2 || f.PanelCellsX > 16 || f.PanelCellsY < 2 || f.PanelCellsY > 16 {
		return fmt.Errorf("art.floor: panel cells must be 2..16")
	}
	if f.TextureWidth < 128 || f.TextureWidth > 1024 || f.TextureWidth%4 != 0 {
		return fmt.Errorf("art.floor: texture_width must be 128..1024 and divisible by4")
	}
	if f.SeamWidthMM < 2 || f.SeamWidthMM > 30 || f.BevelWidthMM < 0 || f.BevelWidthMM > 30 {
		return fmt.Errorf("art.floor: invalid seam or bevel width")
	}
	for _, c := range []string{f.Base, f.Seam, f.Bevel} {
		if _, err := parseHex(c); err != nil {
			return fmt.Errorf("art.floor: %w", err)
		}
	}
	return nil
}

func addFloorMaterialFiles(out map[string][]byte, f *FloorMaterial) error {
	if err := checkFloorMaterial(f); err != nil {
		return err
	}
	if f == nil {
		return nil
	}
	for mask := 0; mask < 4; mask++ {
		img := drawFloorMaterial(f, mask)
		var b bytes.Buffer
		if err := png.Encode(&b, img); err != nil {
			return err
		}
		out[fmt.Sprintf("panel_floor_%d.png", mask)] = b.Bytes()
	}
	return nil
}

// Leading x/y boundaries only: adjacent cells within one broad panel share a
// flat, unbroken surface. No rust, bolts, noise, or outline per metre cell.
func drawFloorMaterial(f *FloorMaterial, mask int) *image.NRGBA {
	w, h := f.TextureWidth, f.TextureWidth/2
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	base, _ := parseHex(f.Base)
	seam, _ := parseHex(f.Seam)
	bevel, _ := parseHex(f.Bevel)
	for py := 0; py < h; py++ {
		for px := 0; px < w; px++ {
			sx, sy := (float64(px)+.5)/float64(w), (float64(py)+.5)/float64(h)
			x, y := sy+sx-.5, sy-sx+.5
			if x < 0 || x > 1 || y < 0 || y > 1 {
				continue
			}
			distance := 2.0
			if mask&1 != 0 {
				distance = x
			}
			if mask&2 != 0 && y < distance {
				distance = y
			}
			img.SetNRGBA(px, py, floorMaterialColor(base, seam, bevel, distance, f))
		}
	}
	return img
}

func floorMaterialColor(base, seam, bevel color.NRGBA, distance float64, f *FloorMaterial) color.NRGBA {
	if distance < float64(f.SeamWidthMM)/1000 {
		return seam
	}
	if distance < float64(f.SeamWidthMM+f.BevelWidthMM)/1000 {
		return bevel
	}
	return base
}
