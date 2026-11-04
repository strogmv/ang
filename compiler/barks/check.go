// Package barks checks the lines spoken over the head (doc 94) against the compiled world and emits their engine.
// A violation fails `generate`: the content rules live here, next to the generator, not in a script one can forget.
package barks

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"

	runtime "github.com/strogmv/ang/compiler/barks/runtime"
)

// Font is the advance table of one face (WorldText metrics) for the fit check.
type Font struct {
	Advance  map[rune]float64
	Tracking float64
}

// Hidden is a person's secret (the `secret` section of the persons catalog, the meaning of the clues about him) and his
// public cover; the no-spoiler rule is derived from them, never from a hand list of words.
type Hidden struct {
	Person string
	Secret []string
	Public []string
}

// World is what the content checks need from the compiled world.
type World struct {
	Texts  func(key string) map[string]string // locale -> text, nil when the key is missing
	Fonts  map[string]Font                    // face -> advances
	Hidden []Hidden
	// Corpus: every public text of the world outside the barks themselves; a word the world already says openly is not
	// a secret word (the secret's distinctive vocabulary stays forbidden).
	Corpus   []string
	Walkable func(level string, at runtime.Cell) bool
	Fillers  []string
	// Warnings are collected here (not failures): long lines, stubs, undecided castes, a line without its register.
	Warn func(string)
	// Registers: caste -> locale -> the speech register of the caste (barks v2, owner 02.11: «на английском медь — это
	// реднеки»). A line of a caste whose register has markers and that carries none of them is a warning, never a failure.
	Registers map[string]map[string]Register
}

// Register is how a caste speaks in one locale: an id for the report, the note for the writer and the words that mark it.
type Register struct {
	ID      string   `json:"id"`
	Note    string   `json:"note"`
	Markers []string `json:"markers"`
}

// RegisterMarker returns the first marker of the register found in the text, "" when none: "-in'" is a word ending, a
// marker with a space is a phrase, anything else a whole word (lowercase; the apostrophe belongs to the word).
func RegisterMarker(text string, markers []string) string {
	lower := strings.ToLower(strings.ReplaceAll(text, "’", "'"))
	ws := strings.FieldsFunc(lower, func(r rune) bool { return !unicode.IsLetter(r) && r != '\'' })
	padded := " " + strings.Join(ws, " ") + " "
	for _, m := range markers {
		m = strings.ToLower(m)
		switch {
		case strings.HasPrefix(m, "-"):
			for _, w := range ws {
				if strings.HasSuffix(w, m[1:]) && len(w) > len(m)-1 {
					return m
				}
			}
		case strings.Contains(m, " "):
			if strings.Contains(padded, " "+m+" ") {
				return m
			}
		default:
			for _, w := range ws {
				if w == m {
					return m
				}
			}
		}
	}
	return ""
}

var punctuation = ".,;:!?"

func words(text string) int { return runtime.MeasureText(text).Words }

// stems: the first five letters of every word of six letters or more (Russian inflects at the end).
func stems(text string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) }) {
		rs := []rune(w)
		if len(rs) >= 6 {
			out[string(rs[:5])] = true
		}
	}
	return out
}

// HiddenStems: words of the secrets that never occur in any public text of the same people (the cover).
func HiddenStems(h []Hidden, corpus ...string) map[string]string {
	public := map[string]bool{}
	for _, t := range corpus {
		for s := range stems(t) {
			public[s] = true
		}
	}
	for _, p := range h {
		for _, t := range p.Public {
			for s := range stems(t) {
				public[s] = true
			}
		}
	}
	out := map[string]string{}
	for _, p := range h {
		for _, t := range p.Secret {
			for s := range stems(t) {
				if !public[s] {
					out[s] = p.Person
				}
			}
		}
	}
	return out
}

// Wrap lays a text out in lines of at most width em with the face's advances; it fails on a glyph the face lacks.
func Wrap(f Font, text string, width float64) ([]string, error) {
	var lines []string
	cur := ""
	for _, w := range strings.Fields(text) {
		if _, err := measureWidth(f, w); err != nil {
			return nil, fmt.Errorf("glyph missing in %q: %v", w, err)
		}
		if cur == "" {
			cur = w
			continue
		}
		if cw, _ := measureWidth(f, cur+" "+w); cw <= width {
			cur += " " + w
			continue
		}
		lines = append(lines, cur)
		cur = w
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	for _, l := range lines {
		if lw, _ := measureWidth(f, l); lw > width {
			return lines, fmt.Errorf("the word %q is wider than the line", l)
		}
	}
	return lines, nil
}
func measureWidth(f Font, s string) (float64, error) {
	w, n := 0.0, 0
	for _, r := range s {
		a, ok := f.Advance[r]
		if !ok {
			return 0, fmt.Errorf("glyph %q", string(r))
		}
		w += a
		n++
	}
	if n > 1 {
		w += f.Tracking * float64(n-1)
	}
	return w, nil
}

// Check enforces doc 94: identity and references, the caste ladder (silent castes, the literacy of copper, no fix for
// silver), the density of Disco Elysium (3..8 words, 10 for silver and system; 2..5 in a fight), no empty greetings, the
// line fits in every locale with its face, every key is translated, no line touches a person's secret.
func Check(m runtime.Model, w World) error {
	var errs []string
	fail := func(format string, args ...any) { errs = append(errs, fmt.Sprintf(format, args...)) }
	warn := func(format string, args ...any) {
		if w.Warn != nil {
			w.Warn(fmt.Sprintf(format, args...))
		}
	}
	if err := m.Validate(); err != nil {
		return err
	}
	for _, c := range []string{"copper", "bronze", "silver", "silver_demoted", "gold", "platinum", "freeman", "none"} {
		if _, ok := m.Face(c); !ok {
			fail("caste %s has no face", c)
		}
	}
	for _, f := range m.Faces {
		if f.Face != "stencil" && f.Face != "chalk" && f.Face != "matrix" || f.Reveal != "written" && f.Reveal != "stamped" && f.Reveal != "set" {
			fail("caste %s: unknown face %q or reveal %q", f.Caste, f.Face, f.Reveal)
		}
		if f.Undecided && f.Speaks {
			fail("caste %s is undecided and may not speak (owner 02.11)", f.Caste)
		}
		if f.Undecided {
			warn("UNDECIDED caste %s (%s, %s) — silent in act one", f.Caste, f.Face, f.Reveal)
		}
		if len(f.Colour) != 7 || f.Colour[0] != '#' {
			fail("caste %s: colour %q is not #rrggbb", f.Caste, f.Colour)
		}
	}
	hidden := HiddenStems(w.Hidden, w.Corpus...)
	texts := func(owner, key string) map[string]string {
		if key == "" {
			return nil
		}
		t := w.Texts(key)
		for _, loc := range m.Locales {
			if t == nil || strings.TrimSpace(t[loc]) == "" {
				fail("%s: localization key %s has no %s text", owner, key, loc)
			}
		}
		return t
	}
	// a spoken text in every locale: fits, density, literacy, fillers, spoilers
	spoken := func(owner, caste, key string, minWords, maxWords int, warnFrom int) map[string]string {
		t := texts(owner, key)
		face, ok := m.Face(caste)
		if !ok {
			fail("%s: speaker caste %q has no face", owner, caste)
			return t
		}
		if !face.Speaks {
			fail("%s: caste %s is silent in act one (owner 02.11)", owner, caste)
		}
		for _, loc := range m.Locales {
			text := t[loc]
			if text == "" {
				continue
			}
			n := words(text)
			if n < minWords || n > maxWords {
				fail("%s: %s has %d words (%d..%d for %s)", owner, loc, n, minWords, maxWords, caste)
			} else if warnFrom > 0 && n >= warnFrom {
				warn("WARN %s: %s has %d words (a warning from %d, the limit %d)", owner, loc, n, warnFrom, maxWords)
			}
			if r := len([]rune(text)); r > m.Layout.LineLimitRunes {
				fail("%s: %s is %d characters (limit %d)", owner, loc, r, m.Layout.LineLimitRunes)
			}
			if font, ok := w.Fonts[face.Face]; ok {
				lines, err := Wrap(font, text, m.Layout.MaxWidthEM)
				if err != nil {
					fail("%s: %s does not fit the %s face: %v", owner, loc, face.Face, err)
				} else if len(lines) > m.Layout.MaxLines {
					fail("%s: %s takes %d lines of %.1f em (max %d)", owner, loc, len(lines), m.Layout.MaxWidthEM, m.Layout.MaxLines)
				}
			} else {
				fail("%s: no font for face %s", owner, face.Face)
			}
			lower := strings.ToLower(text)
			for _, f := range w.Fillers {
				if strings.Contains(lower, f) {
					fail("%s: %s is an empty greeting (%q)", owner, loc, f)
				}
			}
			if !face.Capitals && strings.IndexFunc(text, unicode.IsUpper) >= 0 {
				fail("%s: %s has capitals, %s writes without them", owner, loc, caste)
			}
			if !face.Punctuation && strings.Count(text, ".")+strings.Count(text, ",")+strings.Count(text, "!")+strings.Count(text, "?")+strings.Count(text, ";")+strings.Count(text, ":") > 1 {
				fail("%s: %s has more than one stop, %s writes almost without them", owner, loc, caste)
			}
			for s := range stems(text) {
				if who, bad := hidden[s]; bad {
					fail("%s: %s touches the secret of %s (%q)", owner, loc, who, s)
				}
			}
			if reg, ok := w.Registers[caste][loc]; ok && len(reg.Markers) > 0 && RegisterMarker(text, reg.Markers) == "" {
				warn("REGISTER %s: %s has no marker of %s (%s)", owner, loc, reg.ID, caste)
			}
		}
		return t
	}
	known := func(id string) bool { _, ok := m.Castes[id]; return ok }
	stubs := map[string]bool{}
	for _, e := range m.Events {
		stubs[e.ID] = e.Source == "stub"
	}
	for _, p := range m.Places {
		if w.Walkable == nil {
			break
		}
		cells := 0
		for _, c := range p.Spots {
			if w.Walkable(p.Level, c) {
				cells++
			}
		}
		for _, r := range p.Rects {
			for y := r.Y0; y <= r.Y1 && cells == 0; y++ {
				for x := r.X0; x <= r.X1 && cells == 0; x++ {
					if w.Walkable(p.Level, runtime.Cell{X: x, Y: y}) {
						cells++
					}
				}
			}
		}
		if cells == 0 {
			fail("place %s: no walkable cell on %s — nobody can stand there", p.ID, p.Level)
		}
	}
	for _, p := range m.Packs {
		place, _ := m.Place(p.Place)
		used := map[string]bool{}
		for _, l := range p.Lines {
			owner := "line " + l.ID
			if l.Speaker == runtime.Anyone {
				if l.Caste == "" {
					fail("%s: anyone must name the caste", owner)
				}
			} else if !known(l.Speaker) {
				fail("%s: unknown speaker %s", owner, l.Speaker)
			} else if m.Castes[l.Speaker] != l.Caste {
				fail("%s: caste %s differs from the speaker's %s", owner, l.Caste, m.Castes[l.Speaker])
			}
			if strings.HasPrefix(l.To, "char.") && !known(l.To) {
				fail("%s: speaks to unknown %s", owner, l.To)
			}
			target := l.Requires.Target
			if target == "" {
				target = l.To
			}
			if (l.Requires.TargetOnPost || l.Requires.TargetWithinCells > 0) && !known(target) {
				fail("%s: a target condition needs a known person (requires.target or `to`)", owner)
			}
			maxWords := 8
			if f, ok := m.Face(l.Caste); ok {
				maxWords = f.WordsMax
			}
			if l.Voice == "system" {
				maxWords = 10
			}
			t := spoken(owner, l.Caste, l.ID, 3, maxWords, 7)
			for loc, fx := range l.Fix {
				face, _ := m.Face(l.Caste)
				if face.Correction == "reprint" || face.Correction == "none" {
					fail("%s: %s does not correct in place (%s)", owner, l.Caste, face.Correction)
				}
				rs := []rune(t[loc])
				if fx.From < 0 || fx.To <= fx.From || fx.To > len(rs) {
					fail("%s: fix %s range %d..%d outside the text", owner, loc, fx.From, fx.To)
					continue
				}
				if strings.ContainsAny(string(rs[fx.From:fx.To]), punctuation) {
					fail("%s: fix %s falls on punctuation", owner, loc)
				}
				ft := texts(owner+" fix", fx.Key)
				if f, ok := w.Fonts[face.Face]; ok && ft[loc] != "" {
					if _, err := measureWidth(f, ft[loc]); err != nil {
						fail("%s: fix %s does not fit the face: %v", owner, loc, err)
					}
				}
			}
			for _, loc := range m.Locales {
				if _, ok := l.Fix[loc]; len(l.Fix) > 0 && !ok {
					fail("%s: fix has no %s range", owner, loc)
				}
			}
			for _, r := range l.React {
				if stubs[r.Event] {
					warn("STUB %s: %s (system not built yet, never fires)", l.ID, r.Event)
				}
				if e, _ := m.Event(r.Event); e.Scope == "place" && r.Subject == "" && len(place.Subjects) == 0 {
					fail("%s: place event %s at a place without subjects", owner, r.Event)
				}
				if r.WithinSeconds < 30 {
					fail("%s: react window under 30 s", owner)
				}
			}
			used[l.Reply] = true
		}
		for _, r := range p.Replies {
			owner := "reply " + r.ID
			if !known(r.Speaker) || m.Castes[r.Speaker] != r.Caste {
				fail("%s: unknown speaker %s or caste mismatch", owner, r.Speaker)
			}
			if !used[r.ID] {
				fail("%s: nobody answers with it", owner)
			}
			if r.Kind == "gesture" {
				if r.Key != "" || r.Gesture == "" {
					fail("%s: a gesture carries no text and names its gesture", owner)
				}
				continue
			}
			spoken(owner, r.Caste, r.Key, 1, 8, 0)
		}
	}
	// exchanges (demo-hub v26-alive): known people or anyone of a speaking caste; every turn is a spoken text with all its rules
	for _, e := range m.Exchanges {
		owner := "exchange " + e.ID
		for _, p := range []runtime.Party{e.A, e.B} {
			if p.Who == runtime.Anyone {
				if p.Caste == "" {
					fail("%s: anyone must name the caste", owner)
				}
				continue
			}
			if !known(p.Who) {
				fail("%s: unknown person %s", owner, p.Who)
			} else if m.Castes[p.Who] != p.Caste {
				fail("%s: caste %s differs from the person's %s", owner, p.Caste, m.Castes[p.Who])
			}
			if p.Posture != "" && p.Posture != "post" && p.Posture != "seated" && p.Posture != "standing" {
				fail("%s: unknown posture %q", owner, p.Posture)
			}
		}
		for _, t := range e.Turns {
			caste := e.A.Caste
			if t.By == 1 {
				caste = e.B.Caste
			}
			maxWords := 8
			if f, ok := m.Face(caste); ok {
				maxWords = f.WordsMax
			}
			spoken(owner+" turn "+t.ID, caste, t.ID, 1, maxWords, 0)
		}
	}
	// combat (owner 02.11): voices, intents, short shouts, beasts without words, deceptions with tells
	intents := map[string]bool{}
	for _, i := range m.Combat.Intents {
		intents[i] = true
	}
	voices := map[string]runtime.Voice{}
	for _, v := range m.Combat.Voices {
		voices[v.ID] = v
	}
	for id, v := range m.Combat.Speakers {
		if _, ok := voices[v]; !ok {
			fail("combat speaker %s: unknown voice %s", id, v)
		}
	}
	for id, v := range m.Combat.Archetypes {
		if _, ok := voices[v]; !ok {
			fail("combat archetype %s: unknown voice %s", id, v)
		}
	}
	for _, l := range m.Combat.Lines {
		owner := "combat line " + l.ID
		v := voices[l.Voice]
		if !intents[l.Intent] {
			fail("%s: unknown intent %s", owner, l.Intent)
		}
		if v.Beast {
			if l.Sign == "" || l.Sfx == "" || w.Texts(l.ID) != nil {
				fail("%s: a beast has a sound and a sign, never words", owner)
			}
			continue
		}
		if l.Sign != "" {
			fail("%s: a person speaks, the sign is for beasts", owner)
		}
		spoken(owner, v.Caste, l.ID, 2, 5, 0)
	}
	if len(m.Combat.Lines) > 0 && len(m.Combat.CunningRoles) == 0 {
		fail("combat: nobody may lie (cunning_roles empty)")
	}
	for _, d := range m.Combat.Deceptions {
		owner := "deception " + d.Declared + "->" + d.Truth
		if !intents[d.Declared] || !intents[d.Truth] || d.Declared == d.Truth || len(d.Tells) == 0 {
			fail("%s: needs two different known intents and a tell", owner)
		}
		for _, t := range d.Tells {
			if t.Skill == "" || t.Threshold < 1 || t.Threshold > 100 {
				fail("%s: tell %s needs a skill and a threshold 1..100", owner, t.Key)
			}
			spoken(owner+" tell", "none", t.Key, 2, 6, 0)
		}
	}
	if len(errs) > 0 {
		sort.Strings(errs)
		return errors.New("barks content:\n  " + strings.Join(errs, "\n  "))
	}
	return nil
}
