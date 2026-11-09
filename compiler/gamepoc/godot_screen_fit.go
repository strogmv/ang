package gamepoc

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// Font advance bounds are read from the approved SFNT, without a tool download.
// A 15% shaping margin and the larger 125% setting are included. Native review
// additionally checks actual Godot shaping and scrolling of server-owned text.
type screenFont struct {
	raw     []byte
	tables  map[string][]byte
	units   int
	metrics int
}

func readScreenFont(path string) (screenFont, error) {
	f := screenFont{tables: map[string][]byte{}}
	b, err := os.ReadFile(path)
	if err != nil {
		return f, err
	}
	f.raw = b
	if len(b) < 12 {
		return f, fmt.Errorf("short SFNT font")
	}
	u16 := binary.BigEndian.Uint16
	u32 := binary.BigEndian.Uint32
	for i := 0; i < int(u16(b[4:6])); i++ {
		at := 12 + 16*i
		if at+16 > len(b) {
			return f, fmt.Errorf("short SFNT directory")
		}
		o, n := uint64(u32(b[at+8:at+12])), uint64(u32(b[at+12:at+16]))
		if o+n > uint64(len(b)) {
			return f, fmt.Errorf("bad SFNT table")
		}
		f.tables[string(b[at:at+4])] = b[o : o+n]
	}
	if len(f.tables["head"]) < 20 || len(f.tables["hhea"]) < 36 || len(f.tables["cmap"]) < 4 {
		return f, fmt.Errorf("font lacks metrics/cmap")
	}
	f.units = int(u16(f.tables["head"][18:20]))
	f.metrics = int(u16(f.tables["hhea"][34:36]))
	if f.units < 1 || f.metrics < 1 || len(f.tables["hmtx"]) < 4*f.metrics {
		return f, fmt.Errorf("invalid font metrics")
	}
	return f, nil
}
func (f screenFont) advance(r rune) (float64, error) {
	u16 := binary.BigEndian.Uint16
	u32 := binary.BigEndian.Uint32
	b := f.tables["cmap"]
	glyph := 0
	for i := 0; i < int(u16(b[2:4])); i++ {
		at := 4 + i*8
		if at+8 > len(b) {
			break
		}
		off := int(u32(b[at+4 : at+8]))
		if off < 0 || off+2 > len(b) {
			continue
		}
		sub := b[off:]
		switch u16(sub[:2]) {
		case 12:
			if len(sub) < 16 {
				continue
			}
			for j := 0; j < int(u32(sub[12:16])); j++ {
				p := 16 + j*12
				if p+12 > len(sub) {
					break
				}
				lo, hi := u32(sub[p:p+4]), u32(sub[p+4:p+8])
				if uint32(r) >= lo && uint32(r) <= hi {
					glyph = int(u32(sub[p+8:p+12]) + uint32(r) - lo)
					break
				}
			}
		case 4:
			if r > 65535 || len(sub) < 16 {
				continue
			}
			count := int(u16(sub[6:8])) / 2
			for j := 0; j < count; j++ {
				endAt := 14 + j*2
				startAt := 16 + count*2 + j*2
				deltaAt := 16 + count*4 + j*2
				rangeAt := 16 + count*6 + j*2
				if rangeAt+2 > len(sub) {
					break
				}
				lo, hi := u16(sub[startAt:startAt+2]), u16(sub[endAt:endAt+2])
				if uint16(r) < lo || uint16(r) > hi {
					continue
				}
				delta := u16(sub[deltaAt : deltaAt+2])
				offset := int(u16(sub[rangeAt : rangeAt+2]))
				if offset == 0 {
					glyph = int(uint16(uint32(r) + uint32(delta)))
				} else {
					p := rangeAt + offset + 2*(int(r)-int(lo))
					if p+2 <= len(sub) {
						glyph = int(u16(sub[p : p+2]))
						if glyph != 0 {
							glyph = int(uint16(glyph + int(delta)))
						}
					}
				}
				break
			}
		}
		if glyph != 0 {
			break
		}
	}
	if glyph == 0 {
		return 0, fmt.Errorf("missing glyph %U", r)
	}
	if glyph >= f.metrics {
		glyph = f.metrics - 1
	}
	return float64(u16(f.tables["hmtx"][glyph*4:glyph*4+2])) / float64(f.units), nil
}
func screenText(c Compiled, key string, px, width, lines int, root string) error {
	return screenLocalizedText(c, key, px, width, lines, root, true)
}

// Item names arrive translated by the server and are never exported to public PO.
func screenLocalizedText(c Compiled, key string, px, width, lines int, root string, uiOnly bool) error {
	if px < 12 {
		return fmt.Errorf("effective text %d px below 12 px", px)
	}
	if width < 48 || width > 1280 || lines < 1 {
		return fmt.Errorf("invalid text box at 1280x800: width %d, lines %d", width, lines)
	}
	if key == "" {
		return nil
	}
	var message *LocalizationMessage
	for i := range c.Localization.Messages {
		if c.Localization.Messages[i].Key == key {
			message = &c.Localization.Messages[i]
			break
		}
	}
	if message == nil || (uiOnly && !strings.HasPrefix(key, "ui.")) {
		return fmt.Errorf("missing public text key %q", key)
	}
	var fonts []screenFont
	for _, a := range c.GodotTheme.Fonts {
		f, err := readScreenFont(filepath.Join(root, strings.TrimPrefix(a.File, "res://")))
		if err != nil {
			return err
		}
		fonts = append(fonts, f)
	}
	for _, locale := range c.Localization.Locales {
		copy := message.Texts[locale]
		if strings.TrimSpace(copy) == "" {
			return fmt.Errorf("missing text %s locale %s", key, locale)
		}
		for _, f := range fonts {
			count, used := 1, 0.0
			for _, word := range strings.Fields(strings.ReplaceAll(copy, "\n", " \x00 ")) {
				if word == "\x00" {
					count++
					used = 0
					continue
				}
				w := 0.0
				for _, r := range word {
					adv, err := f.advance(r)
					// Dynamic server item names also use Godot's font fallback. Bound an
					// absent SFNT glyph by 1.25 em; actual shaping is checked natively.
					if err != nil && !uiOnly {
						adv, err = 1.25, nil
					}
					if err != nil {
						return fmt.Errorf("%s %s: %w", key, locale, err)
					}
					w += adv * float64(px) * 1.25 * 1.15
				}
				if math.Ceil(w) > float64(width) {
					return fmt.Errorf("text %s locale %s cannot fit word at 1280x800/125%% (%g > %d px)", key, locale, w, width)
				}
				if used+w > float64(width) {
					count++
					used = 0
				}
				used += w + float64(px)*.45
			}
			if count > lines {
				return fmt.Errorf("text %s locale %s cannot fit at 1280x800/125%% (%d > %d lines)", key, locale, count, lines)
			}
		}
	}
	return nil
}
