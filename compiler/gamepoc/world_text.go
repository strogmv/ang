package gamepoc

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/strogmv/ang/compiler/locationsdk"
)

// World text (demo-hub v11-text, owner rule 31.10.2026, document 09): every in-world inscription is a slot on a
// surface (locationsdk.TextSlot) + a localization key or a price list; Unity draws it as its own layer in the
// player's language. This compiler resolves each slot in EVERY locale of the catalog (key, language-neutral
// arguments, store prices of the inventory = the server's prices), measures it with the same font metrics Unity lays
// it out with (Assets/Resources/<metrics>.json, made from OFL fonts by the book's atlas tool) and rejects a text that
// does not fit, needs a glyph the font lacks, or falls under the slot's minimum glyph height. Unity receives the
// resolved lines, the layout mode and the cap height per locale: what was checked is what is drawn.

// WorldTextCatalog is world.world_text: the lettering of each style.
type WorldTextCatalog struct {
	Fonts []WorldTextFontRef `json:"fonts"`
	// Plates of corridor vistas with a sign_key (presentation of layout vistas): style and limits.
	VistaSign *WorldTextVistaSign `json:"vista_sign,omitempty"`
}

type WorldTextFontRef struct {
	Style      string  `json:"style"`
	Metrics    string  `json:"metrics"` // Resources path of the metrics JSON (without .json)
	TrackingEM float64 `json:"tracking_em"`
	LinePitch  float64 `json:"line_pitch"` // baseline step in cap heights
}

type WorldTextVistaSign struct {
	Style      string `json:"style"`
	Surface    string `json:"surface"`
	Ink        string `json:"ink"`
	Wear       int    `json:"wear"`
	WidthMM    int    `json:"width_mm"`
	HeightMM   int    `json:"height_mm"`
	MarginMM   int    `json:"margin_mm"`
	MinGlyphMM int    `json:"min_glyph_mm"`
}

// WorldTextFontMetrics is the font file written by art-source/world-text (schema ang.world_text.font.v1).
type WorldTextFontMetrics struct {
	Schema  string `json:"schema"`
	Style   string `json:"style"`
	Family  string `json:"family"`
	Metrics struct {
		CapHeight float64 `json:"cap_height"`
	} `json:"metrics"`
	Glyphs []struct {
		C   string  `json:"c"`
		Adv float64 `json:"adv"`
	} `json:"glyphs"`
}

// WorldTextRuntime goes to Unity (PocConfig.world_text).
type WorldTextRuntime struct {
	Fonts []WorldTextFontRef   `json:"fonts"`
	Slots []WorldTextSlotDrawn `json:"slots"`
}

type WorldTextSlotDrawn struct {
	LevelID    string                  `json:"level_id"`
	ID         string                  `json:"id"`
	Instance   string                  `json:"instance,omitempty"`
	Anchor     string                  `json:"anchor,omitempty"`
	Vista      string                  `json:"vista,omitempty"`
	CenterMM   locationsdk.PointMM     `json:"center_mm"`
	Face       string                  `json:"face"`
	WidthMM    int                     `json:"width_mm"`
	HeightMM   int                     `json:"height_mm"`
	MarginMM   int                     `json:"margin_mm"`
	Style      string                  `json:"style"`
	Surface    string                  `json:"surface"`
	Ink        string                  `json:"ink"`
	Wear       int                     `json:"wear"`
	Layout     string                  `json:"layout"`
	Strips     *locationsdk.TextStrips `json:"strips,omitempty"`
	HasStrips  bool                    `json:"has_strips"`
	Plate      *locationsdk.TextPlate  `json:"plate,omitempty"`
	HasPlate   bool                    `json:"has_plate"`
	Texts      []WorldTextLocaleDrawn  `json:"texts"`
	MinGlyphMM int                     `json:"min_glyph_mm"`
}

// WorldTextLocaleDrawn: Mode per_strip|flow|rows, GlyphMM = cap height, Lines = resolved text (per_strip: one line,
// its non-space glyphs go one per strip from FirstStrip in reading order).
type WorldTextLocaleDrawn struct {
	Locale     string   `json:"locale"`
	Mode       string   `json:"mode"`
	GlyphMM    int      `json:"glyph_mm"`
	FirstStrip int      `json:"first_strip"`
	Lines      []string `json:"lines"`
}

var worldTextPlaceholder = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)\}`)

type worldTextFont struct {
	ref     WorldTextFontRef
	cap     float64
	advance map[rune]float64
}

func (f worldTextFont) width(line string) (float64, error) {
	w, n := 0.0, 0
	for _, r := range line {
		a, ok := f.advance[r]
		if !ok {
			return 0, fmt.Errorf("glyph %q (U+%04X) is not in the %s font", string(r), r, f.ref.Style)
		}
		w += a
		n++
	}
	if n > 1 {
		w += f.ref.TrackingEM * float64(n-1)
	}
	return w, nil
}

func loadWorldTextFonts(unity string, catalog WorldTextCatalog) (map[string]worldTextFont, error) {
	fonts := map[string]worldTextFont{}
	for _, ref := range catalog.Fonts {
		if ref.Style != "stencil" && ref.Style != "chalk" && ref.Style != "matrix" || fonts[ref.Style].advance != nil {
			return nil, fmt.Errorf("world text: unknown or duplicate font style %q", ref.Style)
		}
		if !strings.HasPrefix(ref.Metrics, "WorldText/") || strings.Contains(ref.Metrics, "..") || ref.TrackingEM < 0 || ref.TrackingEM > 1 || ref.LinePitch < 1 || ref.LinePitch > 3 {
			return nil, fmt.Errorf("world text: font %s needs metrics WorldText/*, tracking 0..1 em, line pitch 1..3", ref.Style)
		}
		raw, err := os.ReadFile(filepath.Join(unity, "Assets", "Resources", filepath.FromSlash(ref.Metrics)+".json"))
		if err != nil {
			return nil, fmt.Errorf("world text: font metrics of %s: %w", ref.Style, err)
		}
		var m WorldTextFontMetrics
		if err := json.Unmarshal(raw, &m); err != nil || m.Schema != "ang.world_text.font.v1" || m.Style != ref.Style || !(m.Metrics.CapHeight > 0.2 && m.Metrics.CapHeight < 1.2) || len(m.Glyphs) == 0 {
			return nil, fmt.Errorf("world text: invalid metrics %s for style %s", ref.Metrics, ref.Style)
		}
		f := worldTextFont{ref: ref, cap: m.Metrics.CapHeight, advance: map[rune]float64{}}
		for _, g := range m.Glyphs {
			rs := []rune(g.C)
			if len(rs) != 1 || !(g.Adv >= 0 && g.Adv < 4) {
				return nil, fmt.Errorf("world text: invalid glyph in %s", ref.Metrics)
			}
			f.advance[rs[0]] = g.Adv
		}
		if _, ok := f.advance[' ']; !ok {
			return nil, fmt.Errorf("world text: font %s has no space", ref.Style)
		}
		fonts[ref.Style] = f
	}
	return fonts, nil
}

// worldTextFit returns the layout of resolved lines in a slot: mode, cap height (mm, floor) and the first strip.
func worldTextFit(s locationsdk.TextSlot, f worldTextFont, lines []string) (string, int, int, error) {
	aw := float64(s.WidthMM - 2*s.MarginMM)
	ah := float64(s.HeightMM - 2*s.MarginMM)
	if len(lines) == 0 || len(lines) > s.MaxLines {
		return "", 0, 0, fmt.Errorf("%d lines, the slot takes 1..%d", len(lines), s.MaxLines)
	}
	rows := func() (int, error) {
		capMM := ah / (1 + float64(len(lines)-1)*f.ref.LinePitch)
		for _, line := range lines {
			w, err := f.width(line)
			if err != nil {
				return 0, err
			}
			if w > 0 {
				capMM = math.Min(capMM, aw*f.cap/w)
			}
		}
		return int(math.Floor(capMM)), nil
	}
	perStrip := func() (int, int, bool, error) {
		var glyphs []rune
		for _, r := range lines[0] {
			if r != ' ' {
				glyphs = append(glyphs, r)
			}
		}
		if s.Strips == nil || len(glyphs) == 0 || len(glyphs) > s.Strips.Count {
			return 0, 0, false, nil
		}
		sw := float64(s.Strips.WidthMM - 2*min(s.MarginMM, s.Strips.WidthMM/5))
		capMM := ah
		for _, r := range glyphs {
			a, ok := f.advance[r]
			if !ok {
				return 0, 0, false, fmt.Errorf("glyph %q (U+%04X) is not in the %s font", string(r), r, f.ref.Style)
			}
			if a > 0 {
				capMM = math.Min(capMM, sw*f.cap/a)
			}
		}
		return int(math.Floor(capMM)), (s.Strips.Count - len(glyphs)) / 2, true, nil
	}
	mode := s.Layout
	glyph, first := 0, 0
	switch mode {
	case "rows", "flow":
		g, err := rows()
		if err != nil {
			return "", 0, 0, err
		}
		glyph = g
	case "per_strip", "strips_auto":
		g, k, ok, err := perStrip()
		if err != nil {
			return "", 0, 0, err
		}
		if ok && g >= s.MinGlyphMM {
			mode, glyph, first = "per_strip", g, k
		} else if s.Layout == "per_strip" {
			return "", 0, 0, fmt.Errorf("per_strip needs at most %d glyphs at cap >= %d mm", s.Strips.Count, s.MinGlyphMM)
		} else {
			mode = "flow"
			g, err := rows()
			if err != nil {
				return "", 0, 0, err
			}
			glyph = g
		}
	}
	if glyph < s.MinGlyphMM {
		return "", 0, 0, fmt.Errorf("cap height %d mm < minimum %d mm in %s", glyph, s.MinGlyphMM, mode)
	}
	return mode, glyph, first, nil
}

func worldTextFill(template string, values map[string]string) (string, error) {
	var missing string
	out := worldTextPlaceholder.ReplaceAllStringFunc(template, func(m string) string {
		name := m[1 : len(m)-1]
		v, ok := values[name]
		if !ok {
			missing = name
			return m
		}
		return v
	})
	if missing != "" {
		return "", fmt.Errorf("placeholder {%s} has no value", missing)
	}
	return out, nil
}

func worldTextPrice(d struct{ num, den int64 }, locale string) string {
	if d.den <= 0 {
		return "?"
	}
	if d.num%d.den == 0 {
		return strconv.FormatInt(d.num/d.den, 10)
	}
	// demo-hub v14-traders: a fractional price keeps both decimals (0,60 CRD, not 0,6)
	text := strconv.FormatFloat(float64(d.num)/float64(d.den), 'f', 2, 64)
	if locale == "ru" {
		text = strings.ReplaceAll(text, ".", ",")
	}
	return text
}

// compileWorldText resolves and checks every slot of every SDK level in every locale.
func compileWorldText(unity string, c Compiled) (*WorldTextRuntime, error) {
	if c.LocationSDK == nil {
		return nil, nil
	}
	var slots []struct {
		level string
		slot  locationsdk.TextSlot
		vista string
	}
	for _, level := range c.LocationSDK.Levels {
		for _, s := range level.TextSlots {
			slots = append(slots, struct {
				level string
				slot  locationsdk.TextSlot
				vista string
			}{level.ID, s, ""})
		}
		for _, v := range level.Vistas {
			if v.SignKey == "" || c.WorldText.VistaSign == nil {
				continue
			}
			p := c.WorldText.VistaSign
			slots = append(slots, struct {
				level string
				slot  locationsdk.TextSlot
				vista string
			}{level.ID, locationsdk.TextSlot{ID: v.ID + ".sign", Style: p.Style, Surface: p.Surface, Ink: p.Ink, Wear: p.Wear, WidthMM: p.WidthMM, HeightMM: p.HeightMM, MarginMM: p.MarginMM, MinGlyphMM: p.MinGlyphMM, MaxLines: 1, Layout: "rows", Key: v.SignKey, Face: locationsdk.North, Reason: "plate of vista " + v.ID}, v.ID})
		}
	}
	if len(slots) == 0 {
		return nil, nil
	}
	fonts, err := loadWorldTextFonts(unity, c.WorldText)
	if err != nil {
		return nil, err
	}
	locales := c.Localization.Locales
	if len(locales) == 0 {
		return nil, fmt.Errorf("world text needs the localization catalog")
	}
	messages := map[string]map[string]string{}
	for _, m := range c.Localization.Messages {
		messages[m.Key] = m.Texts
	}
	text := func(key, locale string) (string, error) {
		texts, ok := messages[key]
		if !ok {
			return "", fmt.Errorf("key %s is not in the localization catalog", key)
		}
		t, ok := texts[locale]
		if !ok || strings.TrimSpace(t) == "" {
			return "", fmt.Errorf("key %s has no %s text", key, locale)
		}
		return t, nil
	}
	out := &WorldTextRuntime{Fonts: append([]WorldTextFontRef(nil), c.WorldText.Fonts...)}
	for _, entry := range slots {
		s := entry.slot
		f, ok := fonts[s.Style]
		if !ok {
			return nil, fmt.Errorf("world text %s/%s: no font for style %s", entry.level, s.ID, s.Style)
		}
		drawn := WorldTextSlotDrawn{LevelID: entry.level, ID: s.ID, Instance: s.Instance, Anchor: s.Anchor, Vista: entry.vista, CenterMM: s.CenterMM, Face: string(s.Face), WidthMM: s.WidthMM, HeightMM: s.HeightMM, MarginMM: s.MarginMM, Style: s.Style, Surface: s.Surface, Ink: s.Ink, Wear: s.Wear, Layout: s.Layout, Strips: s.Strips, HasStrips: s.Strips != nil, Plate: s.Plate, HasPlate: s.Plate != nil, MinGlyphMM: s.MinGlyphMM}
		for _, locale := range locales {
			var lines []string
			fail := func(err error) error {
				return fmt.Errorf("world text %s/%s [%s]: %w", entry.level, s.ID, locale, err)
			}
			if s.Key != "" {
				t, err := text(s.Key, locale)
				if err != nil {
					return nil, fail(err)
				}
				t, err = worldTextFill(t, s.Args)
				if err != nil {
					return nil, fail(fmt.Errorf("%s: %w", s.Key, err))
				}
				lines = strings.Split(t, "\n")
			} else {
				p := s.PriceList
				for _, row := range p.Rows {
					d, ok := c.Inventory.Definitions[row.Item]
					if !ok {
						return nil, fail(fmt.Errorf("price row %s is not an item definition", row.Item))
					}
					if d.PriceUnit != p.Currency {
						return nil, fail(fmt.Errorf("price row %s is priced in %s, the board shows %s", row.Item, d.PriceUnit, p.Currency))
					}
					sold := false
					for _, src := range c.Inventory.Sources {
						if src.Definition == row.Item && src.ForSale && src.Location.Kind == "store" && src.Location.Store == p.Store {
							sold = true
						}
					}
					if !sold {
						return nil, fail(fmt.Errorf("price row %s is not for sale in store %s", row.Item, p.Store))
					}
					name, err := text(row.NameKey, locale)
					if err != nil {
						return nil, fail(err)
					}
					format, err := text(p.LineKey, locale)
					if err != nil {
						return nil, fail(err)
					}
					scale := int64(1)
					if cur, ok := c.Inventory.Currencies[d.PriceUnit]; ok && cur.Scale > 0 {
						scale = cur.Scale
					}
					line, err := worldTextFill(format, map[string]string{"item": name, "price": worldTextPrice(struct{ num, den int64 }{d.PriceNumerator, d.PriceDenominator * scale}, locale)})
					if err != nil {
						return nil, fail(err)
					}
					lines = append(lines, line)
				}
				footer, err := text(p.FooterKey, locale)
				if err != nil {
					return nil, fail(err)
				}
				footer, err = worldTextFill(footer, map[string]string{"currency": p.Currency})
				if err != nil {
					return nil, fail(err)
				}
				lines = append(lines, footer)
			}
			mode, glyph, first, err := worldTextFit(s, f, lines)
			if err != nil {
				return nil, fail(err)
			}
			drawn.Texts = append(drawn.Texts, WorldTextLocaleDrawn{Locale: locale, Mode: mode, GlyphMM: glyph, FirstStrip: first, Lines: lines})
		}
		out.Slots = append(out.Slots, drawn)
	}
	sort.SliceStable(out.Slots, func(i, j int) bool {
		if out.Slots[i].LevelID != out.Slots[j].LevelID {
			return out.Slots[i].LevelID < out.Slots[j].LevelID
		}
		return out.Slots[i].ID < out.Slots[j].ID
	})
	return out, nil
}
