package gamepoc

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"sort"
	"strconv"
)

// Procedural sprites drawn from CUE style tokens (world.art). The result is deterministic: the same tokens give the same bytes.
// A hand-drawn Assets/Resources/Sprites/<name>.png takes priority in Unity, so these are placeholders that already carry the style.

// spritePad is headroom above the block for things that stick up (seedlings).
var spritePad = map[string]int{"soil": 10, "counter": 12}

// SpriteFiles renders every sprite variant. Keys are file names relative to the generated sprites directory.
func SpriteFiles(art Art) (map[string][]byte, error) {
	out := map[string][]byte{}
	names := make([]string, 0, len(art.Sprites))
	for name := range art.Sprites {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		spec := art.Sprites[name]
		for v := 0; v < spec.Variants; v++ {
			img, err := drawSprite(art, name, spec, v)
			if err != nil {
				return nil, fmt.Errorf("sprite %s: %w", name, err)
			}
			var buf bytes.Buffer
			if err := png.Encode(&buf, img); err != nil {
				return nil, err
			}
			out[SpriteFileName(name, spec.Variants, v)] = buf.Bytes()
		}
	}
	if err := addFloorMaterialFiles(out, art.Floor); err != nil {
		return nil, err
	}
	return out, nil
}

// SpriteFileName is shared with the Unity view: single-variant sprites have no suffix.
func SpriteFileName(name string, variants, variant int) string {
	if variants <= 1 {
		return name + ".png"
	}
	return name + "_" + strconv.Itoa(variant) + ".png"
}

type canvas struct {
	img     *image.NRGBA
	palette map[string]color.NRGBA
	roles   map[string]string
	seed    uint32
}

func (c *canvas) col(role string) color.NRGBA { return c.palette[c.roles[role]] }

func parseHex(s string) (color.NRGBA, error) {
	if len(s) != 7 || s[0] != '#' {
		return color.NRGBA{}, fmt.Errorf("bad color %q", s)
	}
	v, err := strconv.ParseUint(s[1:], 16, 32)
	if err != nil {
		return color.NRGBA{}, err
	}
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 255}, nil
}

func hash3(a, b, c uint32) uint32 {
	h := a*0x9e3779b1 ^ b*0x85ebca6b ^ c*0xc2b2ae35
	h ^= h >> 15
	h *= 0x2c1b3c6d
	h ^= h >> 12
	h *= 0x297a2d39
	h ^= h >> 15
	return h
}

func shade(c color.NRGBA, f float64) color.NRGBA {
	cl := func(v float64) uint8 {
		if v < 0 {
			return 0
		}
		if v > 255 {
			return 255
		}
		return uint8(v + .5)
	}
	return color.NRGBA{cl(float64(c.R) * f), cl(float64(c.G) * f), cl(float64(c.B) * f), c.A}
}

func mix(a, b color.NRGBA, t float64) color.NRGBA {
	l := func(x, y uint8) uint8 { return uint8(float64(x)*(1-t) + float64(y)*t + .5) }
	return color.NRGBA{l(a.R, b.R), l(a.G, b.G), l(a.B, b.B), a.A}
}

func (c *canvas) set(x, y int, col color.NRGBA) {
	if x >= 0 && y >= 0 && x < c.img.Rect.Dx() && y < c.img.Rect.Dy() && c.img.NRGBAAt(x, y).A > 0 {
		c.img.SetNRGBA(x, y, col)
	}
}

// setAny draws even over transparent pixels (for things that stick out of the block).
func (c *canvas) setAny(x, y int, col color.NRGBA) {
	if x >= 0 && y >= 0 && x < c.img.Rect.Dx() && y < c.img.Rect.Dy() {
		c.img.SetNRGBA(x, y, col)
	}
}

func (c *canvas) noise(x, y int, salt uint32) float64 {
	return float64(hash3(uint32(x), uint32(y), c.seed+salt)%1000)/1000 - .5
}

func drawSprite(art Art, name string, spec ArtSprite, variant int) (*image.NRGBA, error) {
	tw, th := art.Tile.W, art.Tile.H
	if tw != 64 || th != 32 {
		return nil, fmt.Errorf("tile must be 64x32, got %dx%d", tw, th)
	}
	c := &canvas{palette: map[string]color.NRGBA{}, roles: spec.Colors}
	for k, v := range art.Palette {
		col, err := parseHex(v)
		if err != nil {
			return nil, err
		}
		c.palette[k] = col
	}
	for role, key := range spec.Colors {
		if _, ok := c.palette[key]; !ok {
			return nil, fmt.Errorf("role %s: unknown palette key %s", role, key)
		}
	}
	h := fnv(name)
	c.seed = h + uint32(variant)*7919
	pad := spritePad[spec.Style]
	switch spec.Shape {
	case "ground":
		c.img = image.NewNRGBA(image.Rect(0, 0, tw, th))
		c.ground(spec.Style, variant)
	case "block":
		c.img = image.NewNRGBA(image.Rect(0, 0, tw, pad+th+spec.Height))
		c.block(spec, pad, variant)
	default:
		return nil, fmt.Errorf("unknown shape %q", spec.Shape)
	}
	return c.img, nil
}

func fnv(s string) uint32 {
	h := uint32(2166136261)
	for i := 0; i < len(s); i++ {
		h = (h ^ uint32(s[i])) * 16777619
	}
	return h
}

// metric is 0 in the tile center and 1 on the diamond border.
func metric(x, y int) float64 {
	dx := (float64(x) + .5 - 32) / 32
	dy := (float64(y) + .5 - 16) / 16
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	return dx + dy
}

func (c *canvas) ground(style string, variant int) {
	for y := 0; y < 32; y++ {
		for x := 0; x < 64; x++ {
			m := metric(x, y)
			if m > 1 {
				continue
			}
			var col color.NRGBA
			switch style {
			case "water":
				base := c.col("a")
				if variant%2 == 1 {
					base = c.col("b")
				}
				col = shade(base, 1+c.noise(x, y, 1)*.12)
				if (y+variant*2)%5 == 0 && c.noise(x, y, 9) > -.1 {
					col = shade(c.col("b"), 1.25)
				}
			case "shaft":
				col = shade(c.col("a"), 1+c.noise(x, y, 3)*.4)
				if m > .88 {
					col = c.col("edge")
				}
			default:
				bases := []string{"a", "b", "c"}
				col = shade(c.col(bases[variant%3]), 1+c.noise(x, y, 5)*.07)
			}
			if m > .93 && style != "shaft" {
				col = shade(c.col("edge"), 1+c.noise(x, y, 7)*.1)
			}
			c.img.SetNRGBA(x, y, col)
		}
	}
	if style != "floor" {
		return
	}
	// Plate seam along the isometric axes in every variant: the floor is made of plates, not a flat color.
	for y := 0; y < 32; y++ {
		for x := 0; x < 64; x++ {
			if metric(x, y) > .9 {
				continue
			}
			if (x/2+y)%16 == 0 || (x/2-y+64)%16 == 0 {
				c.set(x, y, shade(c.col("seam"), 1+c.noise(x, y, 11)*.2))
			}
		}
	}
	switch variant {
	case 1: // rust stain
		for k := 0; k < 3; k++ {
			cx := 18 + int(hash3(uint32(k), c.seed, 1)%28)
			cy := 10 + int(hash3(uint32(k), c.seed, 2)%12)
			r := 3 + int(hash3(uint32(k), c.seed, 3)%4)
			for y := cy - r; y <= cy+r; y++ {
				for x := cx - r*2; x <= cx+r*2; x++ {
					d := float64((x-cx)*(x-cx))/float64(r*r*4) + float64((y-cy)*(y-cy))/float64(r*r)
					if d < 1 && metric(x, y) < .85 && c.noise(x, y, 13)+.5 > d*.5 {
						c.set(x, y, mix(c.img.NRGBAAt(x, y), c.col("rust"), .55*(1-d)))
					}
				}
			}
		}
	case 2: // rivets at the plate corners and scuffs
		for _, p := range [][2]int{{32, 5}, {32, 26}, {12, 16}, {52, 16}} {
			c.set(p[0], p[1], c.col("rivet"))
			c.set(p[0]+1, p[1], shade(c.col("rivet"), .7))
		}
		for k := 0; k < 14; k++ {
			x := 14 + int(hash3(uint32(k), c.seed, 4)%36)
			y := 8 + int(hash3(uint32(k), c.seed, 5)%16)
			if metric(x, y) < .8 {
				c.set(x, y, c.col("scuff"))
				c.set(x+1, y, c.col("scuff"))
			}
		}
	}
}

type region int

const (
	none region = iota
	top
	left
	right
)

// block draws a prism: hw is the half-width of the footprint (32 = full tile).
func (c *canvas) block(spec ArtSprite, pad, variant int) {
	hw := 32
	if spec.Style == "pillar" {
		hw = 17
	}
	if spec.Style == "bunk" || spec.Style == "soil" {
		hw = 30
	}
	H := spec.Height
	hh := hw / 2
	topCY := pad + 16     // center of the top face
	botCY := pad + H + 16 // center of the bottom face (tile footprint)
	for y := 0; y < c.img.Rect.Dy(); y++ {
		for x := 0; x < 64; x++ {
			fx, fy := float64(x)+.5-32, float64(y)+.5
			reg := none
			u, v := 0, 0
			dxn := fx / float64(hw)
			if dxn < 0 {
				dxn = -dxn
			}
			ty := fy - float64(topCY)
			if ty < 0 {
				ty = -ty
			}
			if dxn+ty/float64(hh) <= 1 {
				reg = top
				u, v = x-(32-hw), y-(topCY-hh)
			} else if dxn <= 1 {
				// Side faces: below the top face's lower edges, above the bottom face's lower edges.
				lower := float64(topCY) + float64(hh)*(1-dxn) // lower border of the top face at this x
				bottom := float64(botCY) + float64(hh)*(1-dxn)
				if fy > lower && fy <= bottom {
					if fx < 0 {
						reg = left
						u = x - (32 - hw)
					} else {
						reg = right
						u = x - 32
					}
					v = int(fy - lower)
				}
			}
			if reg == none {
				continue
			}
			c.img.SetNRGBA(x, y, c.face(spec, reg, u, v, hw, H, variant, x, y))
		}
	}
	c.details(spec, pad, hw, H, topCY, variant)
}

func (c *canvas) face(spec ArtSprite, reg region, u, v, hw, H, variant, x, y int) color.NRGBA {
	var base color.NRGBA
	switch reg {
	case top:
		base = c.col("top")
	case left:
		base = c.col("left")
	default:
		base = c.col("right")
	}
	f := 1 + c.noise(x, y, 17)*.08
	col := shade(base, f)
	// Soft vertical gradient on sides: darker toward the floor, so objects sit on the tile.
	if reg != top && H > 0 {
		col = shade(col, 1.08-.22*float64(v)/float64(H))
	}
	switch spec.Style {
	case "wall":
		if reg != top {
			// Panel seams every 11 px, a trim stripe under the top edge.
			if v < 2 {
				col = shade(c.col("trim"), 1+c.noise(x, y, 19)*.1)
			} else if v%11 == 10 {
				col = shade(col, .72)
			}
			if u%16 == 15 {
				col = shade(col, .82)
			}
			// Rust drips from the trim.
			if hash3(uint32(u/3), uint32(reg), c.seed)%11 == 0 && v < 4+int(hash3(uint32(u/3), 5, c.seed)%10) && v >= 2 {
				col = shade(c.col("rust_dark"), 1+c.noise(x, y, 23)*.15)
			}
		} else if (x/2+y)%9 == 0 {
			col = shade(col, .85)
		}
	case "crate":
		// The stable crate glyph is an industrial cargo cassette, never lumber.
		col = c.cargoFace(reg, u, v, hw, H, variant, col)
	case "counter":
		// Varya keeps the warm countertop and seedlings; the reusable housing
		// underneath is a composite panel with locks, not a plank-built stall.
		if reg != top {
			col = c.cargoFace(reg, u, v, hw, H, variant, col)
		}
	case "soil":
		if reg == top {
			col = shade(c.col("top"), 1+c.noise(x, y, 31)*.35)
			if hash3(uint32(x), uint32(y), c.seed)%6 == 0 {
				col = c.col("top_dark")
			}
		} else {
			col = c.cargoFace(reg, u, v, hw, H, variant, col)
		}
	case "pillar":
		// Cylinder look: darker at the edges, a brass band every 14 px.
		edge := float64(u) / float64(hw)
		if reg == right {
			edge = 1 - edge
		}
		col = shade(col, .75+.5*edge)
		if reg != top && v%14 >= 12 {
			col = shade(c.col("trim"), .9+c.noise(x, y, 37)*.2)
		}
	case "pipes":
		if reg != top {
			// Three horizontal pipes with a highlight line.
			band := v % 10
			switch {
			case band == 2:
				col = shade(col, 1.35)
			case band >= 8:
				col = shade(col, .6)
			}
			if u%24 < 2 {
				col = shade(c.col("trim"), 1)
			}
		}
	case "bunk":
		if reg == top {
			// Blanket with sharp folds and a pillow.
			if u > 5 && u < 18 && v > 7 && v < 14 {
				col = shade(c.col("top"), 1.25)
			}
			if (x+2*y)%14 == 0 {
				col = shade(col, .8)
			}
		} else if v < 2 {
			col = c.col("trim")
		}
	case "door":
		if reg != top && reg == left {
			// Inset panel with brass frame on the left face.
			if u >= 4 && u <= 27 && v >= 4 && v <= H-3 {
				col = shade(c.col("panel"), 1+c.noise(x, y, 41)*.1)
				if u == 4 || u == 27 || v == 4 || v == H-3 {
					col = c.col("trim")
				}
				if v == H/2 {
					col = c.col("trim_dark")
				}
			}
		}
	}
	return col
}

// details draws the small bright accents: indicator lights and living things on the cold background.
func (c *canvas) details(spec ArtSprite, pad, hw, H, topCY, variant int) {
	led := func(x, y int, role string) {
		col := c.col(role)
		c.set(x, y, col)
		c.set(x+1, y, col)
		c.set(x, y+1, shade(col, .75))
		c.set(x+1, y+1, shade(col, .75))
	}
	switch spec.Style {
	case "wall":
		// Indicator light on the right face; the second variant carries a scrawl of strokes on the left face.
		led(46, pad+H+2-int(float64(46-32)/2)-4, "led")
		if variant == 1 {
			for k := 0; k < 9; k++ {
				x := 6 + k*2
				y := pad + 16 + x/2 + 12 + int(hash3(uint32(k), c.seed, 43)%5)
				c.set(x, y, c.col("scrawl"))
				c.set(x, y+1, c.col("scrawl"))
			}
			led(14, pad+16+7+8, "led_alt")
		}
	case "door":
		led(42, pad+H-14, "led")
		led(46, pad+H-16, "led_alt")
	case "pillar":
		led(32, pad+H-10, "led")
		led(35, pad+H-8, "led_alt")
	case "bunk":
		led(14, pad+H+2, "led")
	case "soil":
		// The living accent: seedlings stick out above the box.
		for k := 0; k < 7; k++ {
			x := 14 + k*6 + int(hash3(uint32(k), c.seed, 47)%3)
			base := topCY - 3 + int(hash3(uint32(k), c.seed, 49)%7)
			c.sprout(x, base, 5+int(hash3(uint32(k), c.seed, 53)%4))
		}
	case "counter":
		// Two pots with seedlings on the counter: the single warm accent on the cold market.
		c.sprout(23, topCY-1, 10)
		c.sprout(31, topCY+3, 8)
		c.sprout(41, topCY, 9)
		for _, x := range []int{21, 22, 23, 24, 25, 29, 30, 31, 32, 33, 39, 40, 41, 42, 43} {
			c.set(x, topCY+1, c.col("soil"))
			c.set(x, topCY+2, shade(c.col("soil"), .7))
		}
	}
}

func (c *canvas) sprout(x, baseY, h int) {
	stem := c.col("sprout")
	light := c.col("sprout_light")
	for i := 0; i < h; i++ {
		c.setAny(x, baseY-i, shade(stem, .8+.04*float64(i)))
	}
	// Two leaf pairs: a lower one on both sides and a crown on top.
	for _, d := range []int{-3, -2, 2, 3} {
		c.setAny(x+d, baseY-h/2, stem)
	}
	c.setAny(x-2, baseY-h/2-1, light)
	c.setAny(x+2, baseY-h/2-1, light)
	for dx := -2; dx <= 2; dx++ {
		c.setAny(x+dx, baseY-h, light)
		if dx > -2 && dx < 2 {
			c.setAny(x+dx, baseY-h-1, stem)
		}
	}
}

func bytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }
