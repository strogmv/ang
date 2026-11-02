package locationsdk

import (
	"fmt"
	"regexp"
	"strings"
)

// TextSlot is an in-world inscription (owner rule 31.10.2026, document 09): words are never baked into geometry,
// textures or video loops. A slot is an area on a surface plus a localization key (or a price list built from the
// inventory); Unity draws the text as its own layer in the player's language. Presentation only: a slot has no cells,
// blocks nothing and is not seen by the server. The fit of every locale is checked by the game compiler (gamepoc),
// which knows the localization catalog and the font metrics.
type TextSlot struct {
	ID string `json:"id"`
	// Host: an instance of the layout (its registry prefab may carry the locator node Anchor, "textslot.*"); without an
	// instance the slot stands in the layout plan.
	Instance string `json:"instance,omitempty"`
	Anchor   string `json:"anchor,omitempty"`
	// Centre of the slot area in mm: instance-local plan frame (x east, y south, z up from the instance floor, the
	// instance at rotation 0) or the layout plan. A prefab locator, when found, replaces this point.
	CenterMM PointMM `json:"center_mm"`
	// Outward normal of the surface in that frame; it turns with the instance. Text reads left to right for a viewer
	// in front of the face (the reading direction follows the face, so any camera in front reads it correctly).
	Face     Side `json:"face"`
	WidthMM  int  `json:"width_mm"`
	HeightMM int  `json:"height_mm"`
	MarginMM int  `json:"margin_mm"`
	// Lettering: stencil (sprayed through a stencil), chalk (handwritten), matrix (dot-matrix screen, emissive).
	Style string `json:"style"`
	// Surface the paint lies on: its wear mask (pvc strips flex and crack, steel chips and rusts, concrete is porous,
	// a board is wiped, a screen does not wear but glows).
	Surface string `json:"surface"`
	Ink     string `json:"ink"`  // #RRGGBB
	Wear    int    `json:"wear"` // per mille, 0 = fresh paint
	// Smallest cap height in mm the slot accepts in any locale, at most this many lines.
	MinGlyphMM int `json:"min_glyph_mm"`
	MaxLines   int `json:"max_lines"`
	// per_strip: one glyph per strip (noren); flow: one line across the whole area, cut by the strip gaps; rows: lines
	// centred with one cap height; strips_auto: per_strip when the glyphs (without spaces) fit the strips, else flow.
	Layout string      `json:"layout"`
	Strips *TextStrips `json:"strips,omitempty"`
	// The text: a localization key with language-neutral arguments (digits, codes), or a price list.
	Key       string            `json:"key,omitempty"`
	Args      map[string]string `json:"args,omitempty"`
	PriceList *TextPriceList    `json:"price_list,omitempty"`
	// Plate (demo-hub v15): a steel plate the text is painted on, mounted at the slot (a sign clamped to a railing): the
	// client draws a box of the slot area (+ rim) and depth_mm behind the face; color #RRGGBB.
	Plate  *TextPlate `json:"plate,omitempty"`
	Reason string     `json:"reason"`
}

// TextPlate is the backing plate of a slot (mm).
type TextPlate struct {
	DepthMM int    `json:"depth_mm"`
	RimMM   int    `json:"rim_mm,omitempty"`
	Color   string `json:"color"`
}

// PointMM is a point in millimetres.
type PointMM struct {
	X int `json:"x"`
	Y int `json:"y"`
	Z int `json:"z"`
}

// TextStrips divides the slot area along the reading direction into strips (a strip curtain): strip k covers
// [k*pitch, k*pitch+width] from the area's start edge. Nodes name the prefab strips the glyphs ride on (they swing).
type TextStrips struct {
	Count   int      `json:"count"`
	PitchMM int      `json:"pitch_mm"`
	WidthMM int      `json:"width_mm"`
	Nodes   []string `json:"nodes,omitempty"`
}

// TextPriceList: real goods of a store with the server's prices; only the row format and short names are keys.
type TextPriceList struct {
	Store     string         `json:"store"`
	LineKey   string         `json:"line_key"`
	FooterKey string         `json:"footer_key"`
	Currency  string         `json:"currency"`
	Rows      []TextPriceRow `json:"rows"`
}

type TextPriceRow struct {
	Item    string `json:"item"`
	NameKey string `json:"name_key"`
}

var textKey = regexp.MustCompile(`^[a-z][a-z0-9_-]*(\.[a-z0-9_-]+)+$`)
var textArgName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

// TextArgValue: language-neutral values only - digits, upper-case codes and signs (a word would escape translation).
var TextArgValue = regexp.MustCompile(`^[0-9A-Z][0-9A-Z.,:/ ×%-]{0,15}$`)
var textInk = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
var textCurrency = regexp.MustCompile(`^[A-Z]{3}$`)
var textStyles = map[string]bool{"stencil": true, "chalk": true, "matrix": true}
var textSurfaces = map[string]bool{"pvc": true, "steel": true, "concrete": true, "board": true, "screen": true}
var textLayouts = map[string]bool{"per_strip": true, "flow": true, "rows": true, "strips_auto": true}

// validateTextSlots: stable unique IDs, a known host, a bounded area on a known face, a known lettering, layout and
// surface, strips where the layout needs them, exactly one of key / price list with well-formed keys.
func validateTextSlots(layout Layout) *ValidationError {
	if len(layout.TextSlots) > 256 {
		return issue("text_slot_budget", "layout.text_slots", "a layout supports at most 256 text slots")
	}
	instances := map[string]bool{}
	for _, instance := range layout.Instances {
		instances[instance.ID] = true
	}
	seen := map[string]bool{}
	for i, s := range layout.TextSlots {
		path := fmt.Sprintf("layout.text_slots[%d]", i)
		if !stableID.MatchString(s.ID) || seen[s.ID] || strings.TrimSpace(s.Reason) == "" {
			return issue("text_slot_definition", path, "a text slot needs a unique stable ID and a reason", s.ID)
		}
		seen[s.ID] = true
		if s.Instance != "" && !instances[s.Instance] {
			return issue("text_slot_host", path, "the host instance is not in the layout", s.ID, s.Instance)
		}
		if s.Anchor != "" && (s.Instance == "" || !strings.HasPrefix(s.Anchor, "textslot.") || !stableID.MatchString(s.Anchor)) {
			return issue("text_slot_host", path, "an anchor is a prefab locator textslot.* of the host instance", s.ID)
		}
		if s.Face != North && s.Face != South && s.Face != East && s.Face != West {
			return issue("text_slot_face", path, "face is north, south, east or west", s.ID)
		}
		if s.WidthMM < 50 || s.WidthMM > 20000 || s.HeightMM < 20 || s.HeightMM > 5000 || s.MarginMM < 0 || 2*s.MarginMM >= s.WidthMM || 2*s.MarginMM >= s.HeightMM {
			return issue("text_slot_area", path, "area 50..20000 x 20..5000 mm with margins inside it", s.ID)
		}
		if s.CenterMM.Z < -1000 || s.CenterMM.Z > 30000 {
			return issue("text_slot_area", path, "slot height -1..30 m", s.ID)
		}
		if !textStyles[s.Style] || !textSurfaces[s.Surface] || !textInk.MatchString(s.Ink) || s.Wear < 0 || s.Wear > 1000 {
			return issue("text_slot_style", path, "style stencil|chalk|matrix, surface pvc|steel|concrete|board|screen, ink #RRGGBB, wear 0..1000", s.ID)
		}
		if pl := s.Plate; pl != nil && (pl.DepthMM < 2 || pl.DepthMM > 120 || pl.RimMM < 0 || pl.RimMM > 200 || !textInk.MatchString(pl.Color) || s.Style == "matrix") {
			return issue("text_slot_plate", path, "a plate is 2..120 mm deep with a rim 0..200 mm and a #RRGGBB colour; a matrix screen has none", s.ID)
		}
		if s.MinGlyphMM < 10 || s.MinGlyphMM > s.HeightMM-2*s.MarginMM || s.MaxLines < 1 || s.MaxLines > 16 || !textLayouts[s.Layout] {
			return issue("text_slot_layout", path, "min glyph 10 mm..area height, 1..16 lines, layout per_strip|flow|rows|strips_auto", s.ID)
		}
		strips := s.Layout == "per_strip" || s.Layout == "strips_auto"
		if strips && s.Strips == nil || s.Strips != nil && (s.Layout == "rows" || s.MaxLines != 1) {
			return issue("text_slot_strips", path, "per_strip / strips_auto need strips and one line; rows has none", s.ID)
		}
		if st := s.Strips; st != nil {
			if st.Count < 2 || st.Count > 64 || st.WidthMM < 20 || st.PitchMM < st.WidthMM || (st.Count-1)*st.PitchMM+st.WidthMM > s.WidthMM || len(st.Nodes) != 0 && len(st.Nodes) != st.Count {
				return issue("text_slot_strips", path, "2..64 strips of width <= pitch inside the area; nodes, when given, one per strip", s.ID)
			}
			nodes := map[string]bool{}
			for _, n := range st.Nodes {
				if !stableID.MatchString(n) || nodes[n] || s.Instance == "" {
					return issue("text_slot_strips", path, "strip nodes are unique prefab node names of the host instance", s.ID, n)
				}
				nodes[n] = true
			}
		}
		if (s.Key == "") == (s.PriceList == nil) {
			return issue("text_slot_text", path, "a slot shows exactly one of key or price_list", s.ID)
		}
		if s.Key != "" && !textKey.MatchString(s.Key) {
			return issue("text_slot_text", path, "invalid localization key", s.ID, s.Key)
		}
		for name, value := range s.Args {
			if !textArgName.MatchString(name) || !TextArgValue.MatchString(value) || s.Key == "" {
				return issue("text_slot_text", path, "arguments go with a key; values are language-neutral (digits, upper-case codes)", s.ID, name)
			}
		}
		if p := s.PriceList; p != nil {
			if !stableID.MatchString(p.Store) || !textKey.MatchString(p.LineKey) || !textKey.MatchString(p.FooterKey) || !textCurrency.MatchString(p.Currency) || len(p.Rows) < 1 || len(p.Rows) > s.MaxLines-1 || s.Layout != "rows" {
				return issue("text_slot_price", path, "a price list: store, line and footer keys, a currency code, 1..max_lines-1 rows, layout rows", s.ID)
			}
			items := map[string]bool{}
			for _, row := range p.Rows {
				if !stableID.MatchString(row.Item) || items[row.Item] || !textKey.MatchString(row.NameKey) {
					return issue("text_slot_price", path, "price rows name unique items with a short-name key", s.ID, row.Item)
				}
				items[row.Item] = true
			}
		}
	}
	return nil
}
