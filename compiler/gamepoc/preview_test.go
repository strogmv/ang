package gamepoc

import (
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// TestPreviewLevel renders the level with the generated sprites into a PNG for a visual check.
// Run: SPRITE_PREVIEW=out.png SPRITE_PROJECT=/path/to/server go test ./compiler/gamepoc -run PreviewLevel
func TestPreviewLevel(t *testing.T) {
	out, project := os.Getenv("SPRITE_PREVIEW"), os.Getenv("SPRITE_PROJECT")
	if out == "" || project == "" {
		t.Skip("set SPRITE_PREVIEW and SPRITE_PROJECT")
	}
	compiled, _, err := readCompiled(project)
	if err != nil {
		t.Fatal(err)
	}
	files, err := SpriteFiles(compiled.Art)
	if err != nil {
		t.Fatal(err)
	}
	load := func(name string) image.Image {
		img, err := pngDecode(files[name])
		if err != nil {
			t.Fatal(err)
		}
		return img
	}
	w, h := len(compiled.Terrain[0]), len(compiled.Terrain)
	dst := image.NewNRGBA(image.Rect(0, 0, (w+h)*32+64, (w+h)*16+160))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(image.Black), image.Point{}, draw.Src)
	ox, oy := h*32, 90
	place := func(name string, variants, height int, x, y int, weights []int) {
		v := int(hash3(uint32(x), uint32(y), 425) % uint32(sum(weights)))
		pick := 0
		for i, wt := range weights {
			if v < wt {
				pick = i
				break
			}
			v -= wt
		}
		img := load(SpriteFileName(name, variants, pick))
		b := img.Bounds()
		at := image.Pt(ox+(x-y)*32, oy+(x+y)*16-(b.Dy()-32))
		draw.Draw(dst, image.Rectangle{at, at.Add(b.Size())}, img, b.Min, draw.Over)
	}
	for _, pass := range []string{"ground", "block"} {
		for s := 0; s < w+h; s++ {
			for y := 0; y < h; y++ {
				x := s - y
				if x < 0 || x >= w {
					continue
				}
				name := compiled.TileSprites[string(compiled.Terrain[y][x])]
				spec := compiled.Art.Sprites[name]
				weights := spec.Weights
				if len(weights) == 0 {
					weights = make([]int, spec.Variants)
					for i := range weights {
						weights[i] = 1
					}
				}
				if pass == "ground" {
					floor := compiled.Art.Sprites["dome_floor"]
					if spec.Shape == "ground" {
						place(name, spec.Variants, 0, x, y, weights)
					} else {
						place("dome_floor", floor.Variants, 0, x, y, floor.Weights)
					}
				} else if spec.Shape == "block" {
					place(name, spec.Variants, spec.Height, x, y, weights)
				}
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, dst); err != nil {
		t.Fatal(err)
	}
}

func sum(a []int) int {
	t := 0
	for _, v := range a {
		t += v
	}
	return t
}

func pngDecode(b []byte) (image.Image, error) { return png.Decode(bytesReader(b)) }
