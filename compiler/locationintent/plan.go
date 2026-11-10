package locationintent

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"

	"github.com/strogmv/ang/compiler/locationsdk"
)

// Plan draws the derived level from above: zones, walls, openings, fixtures,
// mechanisms, lamps, markers, inspections, and every failed check in red.
// Text is a built-in 5x7 face (Latin, digits): IDs are ASCII by schema.
func Plan(d *Derived, diags []Diagnostic) ([]byte, error) {
	scale := 14
	if d.Layout.Width > 100 {
		scale = 10
	}
	const ox, oy = 30, 44
	legend := 22 + 12*min(len(diags), 24)
	img := image.NewRGBA(image.Rect(0, 0, ox*2+d.Layout.Width*scale, oy+d.Layout.Depth*scale+legend+20))
	fill := func(x0, y0, x1, y1 int, c color.RGBA) {
		for y := max(y0, 0); y < min(y1, img.Bounds().Dy()); y++ {
			for x := max(x0, 0); x < min(x1, img.Bounds().Dx()); x++ {
				if c.A == 255 {
					img.SetRGBA(x, y, c)
				} else {
					o := img.RGBAAt(x, y)
					a := int(c.A)
					img.SetRGBA(x, y, color.RGBA{uint8((int(o.R)*(255-a) + int(c.R)*a) / 255), uint8((int(o.G)*(255-a) + int(c.G)*a) / 255), uint8((int(o.B)*(255-a) + int(c.B)*a) / 255), 255})
				}
			}
		}
	}
	cellBox := func(c locationsdk.Cell, inset int, col color.RGBA) {
		fill(ox+c.X*scale+inset, oy+c.Y*scale+inset, ox+(c.X+1)*scale-inset, oy+(c.Y+1)*scale-inset, col)
	}
	rect := func(r locationsdk.Rect, col color.RGBA) {
		fill(ox+r.X*scale, oy+r.Y*scale, ox+(r.X+r.W)*scale, oy+(r.Y+r.H)*scale, col)
	}
	outline := func(r locationsdk.Rect, col color.RGBA) {
		x0, y0, x1, y1 := ox+r.X*scale, oy+r.Y*scale, ox+(r.X+r.W)*scale, oy+(r.Y+r.H)*scale
		fill(x0, y0, x1, y0+2, col)
		fill(x0, y1-2, x1, y1, col)
		fill(x0, y0, x0+2, y1, col)
		fill(x1-2, y0, x1, y1, col)
	}
	text := func(x, y int, s string, col color.RGBA) {
		for _, r := range strings.ToUpper(s) {
			glyph, ok := font5x7[r]
			if !ok {
				glyph = font5x7['?']
			}
			for row := 0; row < 7; row++ {
				for bit := 0; bit < 5; bit++ {
					if glyph[row]&(1<<(4-bit)) != 0 && x+bit >= 0 && y+row >= 0 && x+bit < img.Bounds().Dx() && y+row < img.Bounds().Dy() {
						img.SetRGBA(x+bit, y+row, col)
					}
				}
			}
			x += 6
		}
	}
	var (
		paper   = color.RGBA{246, 244, 239, 255}
		wall    = color.RGBA{72, 64, 58, 255}
		floorA  = color.RGBA{226, 220, 208, 255}
		floorB  = color.RGBA{214, 208, 196, 255}
		fixture = color.RGBA{120, 150, 195, 255}
		grey    = color.RGBA{158, 46, 140, 255}
		mech    = color.RGBA{226, 128, 32, 110}
		open    = color.RGBA{40, 150, 70, 255}
		planned = color.RGBA{150, 150, 150, 255}
		lamp    = color.RGBA{235, 190, 40, 255}
		marker  = color.RGBA{40, 110, 200, 255}
		insp    = color.RGBA{120, 60, 160, 255}
		bad     = color.RGBA{210, 40, 40, 255}
		ink     = color.RGBA{30, 30, 30, 255}
		voidC   = color.RGBA{150, 150, 165, 200}
	)
	fill(0, 0, img.Bounds().Dx(), img.Bounds().Dy(), paper)
	status := "COHERENCE PASS"
	statusColor := open
	if len(diags) > 0 {
		status, statusColor = "COHERENCE FAIL", bad
	}
	if len(d.Capped) > 0 {
		status += "  CAPPED POCKETS: " + itoa(len(d.Capped))
	}
	text(ox, 10, d.Intent.ID+"  "+itoa(d.Layout.Width)+"X"+itoa(d.Layout.Depth)+" CELLS 0.5 M  TIER "+d.Intent.Tier, ink)
	text(ox, 24, status+"  X EAST, Y SOUTH", statusColor)
	for _, m := range d.Masses {
		rect(m, wall)
	}
	i := 0
	for _, z := range d.Intent.Zones {
		r := d.Zones[z.ID]
		col := floorA
		if i%2 == 1 {
			col = floorB
		}
		i++
		rect(r, col)
		text(ox+r.X*scale+4, oy+r.Y*scale+4, z.ID, color.RGBA{90, 90, 90, 255})
	}
	for _, m := range d.Mechanisms {
		if m.Void != nil {
			rect(*m.Void, voidC)
		}
		rect(m.Rect, mech)
		outline(m.Rect, color.RGBA{226, 128, 32, 255})
		text(ox+m.Rect.X*scale+4, oy+m.Rect.Y*scale+14, m.ID, color.RGBA{180, 90, 10, 255})
	}
	for _, w := range d.Walls {
		if w.Kind != "wall" {
			continue
		}
		x0, y0 := ox+w.Cell.X*scale, oy+w.Cell.Y*scale
		switch w.Side {
		case "north":
			fill(x0, y0-4, x0+scale, y0, wall)
		case "south":
			fill(x0, y0+scale, x0+scale, y0+scale+4, wall)
		case "west":
			fill(x0-4, y0, x0, y0+scale, wall)
		default:
			fill(x0+scale, y0, x0+scale+4, y0+scale, wall)
		}
	}
	for _, e := range d.Exits {
		col := open
		if e.Status == "planned" {
			col = planned
		} else if e.Status == "closed" {
			col = color.RGBA{200, 140, 40, 255}
		}
		for _, c := range e.Threshold {
			cellBox(c, 0, col)
		}
		text(ox+e.At.X*scale+scale+2, oy+e.At.Y*scale-9, e.ID, col)
		cellBox(e.Landing, 4, col)
	}
	for _, f := range d.Fixtures {
		col := fixture
		if f.ModelRef == "" {
			col = grey
		}
		rect(locationsdk.Rect{X: f.At.X, Y: f.At.Y, W: f.W, H: f.D}, col)
		cellBox(f.Approach, 5, color.RGBA{60, 80, 120, 255})
		text(ox+f.At.X*scale+2, oy+(f.At.Y+f.D)*scale+2, f.ID, color.RGBA{40, 60, 100, 255})
	}
	dot := func(xm, ym float64, r int, col color.RGBA) {
		cx, cy := ox+int(xm/unitM*float64(scale)), oy+int(ym/unitM*float64(scale))
		for y := -r; y <= r; y++ {
			for x := -r; x <= r; x++ {
				if x*x+y*y <= r*r {
					fill(cx+x, cy+y, cx+x+1, cy+y+1, col)
				}
			}
		}
	}
	for _, l := range d.Lamps {
		dot(l.AtM[0], l.AtM[1], 4, color.RGBA{90, 70, 0, 255})
		dot(l.AtM[0], l.AtM[1], 3, lamp)
	}
	for _, m := range d.World.Markers {
		dot((float64(m.At.X)+0.5)*unitM, (float64(m.At.Y)+0.5)*unitM, 8, marker)
		dot((float64(m.At.X)+0.5)*unitM, (float64(m.At.Y)+0.5)*unitM, 6, paper)
		text(ox+m.At.X*scale+scale/2-2, oy+m.At.Y*scale+scale/2-3, itoa(m.Number), marker)
	}
	for _, n := range d.World.Inspections {
		dot((float64(n.At.X)+0.5)*unitM, (float64(n.At.Y)+0.5)*unitM, 3, insp)
	}
	y := oy + d.Layout.Depth*scale + 10
	for n, g := range diags {
		if g.Cell != nil {
			outline(locationsdk.Rect{X: g.Cell.X - 1, Y: g.Cell.Y - 1, W: 3, H: 3}, bad)
			cellBox(*g.Cell, 2, bad)
		}
		if n < 24 {
			line := g.Code + ": " + g.ID
			if g.Cell != nil {
				line += " (" + itoa(g.Cell.X) + "," + itoa(g.Cell.Y) + ")"
			}
			text(ox, y, line+" - "+g.Hint, bad)
			y += 12
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	sign := ""
	if n < 0 {
		sign, n = "-", -n
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return sign + s
}

// font5x7: each glyph is seven rows of five bits, most significant bit left.
var font5x7 = map[rune][7]byte{
	' ': {}, '?': {14, 17, 1, 2, 4, 0, 4}, '.': {0, 0, 0, 0, 0, 12, 12}, ',': {0, 0, 0, 0, 12, 4, 8}, ':': {0, 12, 12, 0, 12, 12, 0},
	'-': {0, 0, 0, 31, 0, 0, 0}, '_': {0, 0, 0, 0, 0, 0, 31}, '(': {2, 4, 8, 8, 8, 4, 2}, ')': {8, 4, 2, 2, 2, 4, 8}, '/': {1, 1, 2, 4, 8, 16, 16},
	'"': {10, 10, 0, 0, 0, 0, 0}, '\'': {4, 4, 0, 0, 0, 0, 0}, ';': {0, 12, 12, 0, 12, 4, 8}, '=': {0, 0, 31, 0, 31, 0, 0}, '+': {0, 4, 4, 31, 4, 4, 0}, '#': {10, 10, 31, 10, 31, 10, 10},
	'0': {14, 17, 19, 21, 25, 17, 14}, '1': {4, 12, 4, 4, 4, 4, 14}, '2': {14, 17, 1, 2, 4, 8, 31}, '3': {31, 2, 4, 2, 1, 17, 14}, '4': {2, 6, 10, 18, 31, 2, 2},
	'5': {31, 16, 30, 1, 1, 17, 14}, '6': {6, 8, 16, 30, 17, 17, 14}, '7': {31, 1, 2, 4, 8, 8, 8}, '8': {14, 17, 17, 14, 17, 17, 14}, '9': {14, 17, 17, 15, 1, 2, 12},
	'A': {14, 17, 17, 31, 17, 17, 17}, 'B': {30, 17, 17, 30, 17, 17, 30}, 'C': {14, 17, 16, 16, 16, 17, 14}, 'D': {28, 18, 17, 17, 17, 18, 28}, 'E': {31, 16, 16, 30, 16, 16, 31},
	'F': {31, 16, 16, 30, 16, 16, 16}, 'G': {14, 17, 16, 23, 17, 17, 15}, 'H': {17, 17, 17, 31, 17, 17, 17}, 'I': {14, 4, 4, 4, 4, 4, 14}, 'J': {7, 2, 2, 2, 2, 18, 12},
	'K': {17, 18, 20, 24, 20, 18, 17}, 'L': {16, 16, 16, 16, 16, 16, 31}, 'M': {17, 27, 21, 21, 17, 17, 17}, 'N': {17, 17, 25, 21, 19, 17, 17}, 'O': {14, 17, 17, 17, 17, 17, 14},
	'P': {30, 17, 17, 30, 16, 16, 16}, 'Q': {14, 17, 17, 17, 21, 18, 13}, 'R': {30, 17, 17, 30, 20, 18, 17}, 'S': {15, 16, 16, 14, 1, 1, 30}, 'T': {31, 4, 4, 4, 4, 4, 4},
	'U': {17, 17, 17, 17, 17, 17, 14}, 'V': {17, 17, 17, 17, 17, 10, 4}, 'W': {17, 17, 17, 21, 21, 21, 10}, 'X': {17, 17, 10, 4, 10, 17, 17}, 'Y': {17, 17, 17, 10, 4, 4, 4}, 'Z': {31, 1, 2, 4, 8, 16, 31},
}
