package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"cuelang.org/go/cue"
)

// JourneyScreen is world.loading_screen (owner 23.11, design/loading-screen/v1): the screen of a journey between places.
// The client shows it (presentation.json loading_screen: kinds, stages, places with their names, pictures and route
// nodes); the server reads only what a place's line may name (world.json loading_screen: the knowledge of each place) and
// chooses the line for the view (templates/loading_screen.go.tmpl). The form of a kind is decided by the data from its
// expected duration, never by a timer: a kind expected within the zone-change target is the short form.
type JourneyScreen struct {
	ShortLimitMS int                     `json:"short_limit_ms"`
	Start        JourneyStart            `json:"start"`
	FallbackKind string                  `json:"fallback_kind"`
	Stages       map[string][]string     `json:"stages"`
	Kinds        map[string]JourneyKind  `json:"kinds"`
	Places       map[string]JourneyPlace `json:"places"`
	SourceShown  []string                `json:"source_shown"`
	Server       *loadingScreenServer    `json:"-"`
}

type JourneyStart struct {
	Scenario string `json:"scenario"`
	Level    string `json:"level"`
}

type JourneyKind struct {
	Path       string `json:"path"`
	CoverFrom  string `json:"cover_from"`
	ExpectedMS int    `json:"expected_ms"`
	FromKey    string `json:"from_key,omitempty"`
	ViaKey     string `json:"via_key,omitempty"`
	Form       string `json:"form"`
}

type JourneyPlace struct {
	NameKey   string   `json:"name_key"`
	Painting  string   `json:"painting"`
	Render    string   `json:"render"`
	RouteNode string   `json:"route_node"`
	Knowledge []string `json:"knowledge"`
}

// loadingScreenServer is the server's part (world.json loading_screen): only what a place's line may name.
type loadingScreenServer struct {
	Places map[string]loadingServerPlace `json:"places"`
}

type loadingServerPlace struct {
	Knowledge []string `json:"knowledge"`
}

// loadingStages: the stages the client knows how to detect (client-godot/game/journey.gd), in the only order they can
// happen, by path; a path's data names a subsequence of them ending with ready.
var loadingStages = map[string][]string{
	"entry":  {"world", "scene", "installed", "build", "compile", "ready"},
	"travel": {"trip", "arrival", "scene", "ready"},
}

// loadingTripKinds: the kinds the server names a trip by (templates/loading_screen.go.tmpl loadingTripKind).
var loadingTripKinds = []string{"walk", "rail", "train", "vehicle"}

// loadingEntryKinds: the entries the client starts (game/session.gd): a new game and a load.
var loadingEntryKinds = []string{"new_game", "load"}

var loadingPicture = regexp.MustCompile(`^res://assets/loading/[a-z0-9_.-]+\.(png|jpg)$`)

func readLoadingScreen(world cue.Value) (*JourneyScreen, error) {
	v := world.LookupPath(cue.ParsePath("loading_screen"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.loading_screen: %w", err)
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var out JourneyScreen
	if err := d.Decode(&out); err != nil {
		return nil, fmt.Errorf("world.loading_screen: %w", err)
	}
	out.Server = &loadingScreenServer{Places: map[string]loadingServerPlace{}}
	for level, p := range out.Places {
		out.Server.Places[level] = loadingServerPlace{Knowledge: append([]string{}, p.Knowledge...)}
	}
	return &out, nil
}

// validateLoadingScreen: every Life location is a place with a name in every locale that fits the screen at 1280x800,
// pictures that exist when named (a painting, a render of the place), a route node of the chip's map bound to it; every journal
// line a place may show exists, fits, and belongs to a fact or topic of the hero's journal; every kind the client and
// the server name exists with a form that follows from its expected duration; the stages are the client's, in order.
func validateLoadingScreen(project string, c Compiled) error {
	l := c.LoadingScreen
	if l == nil {
		return nil
	}
	fail := func(format string, args ...any) error {
		return fmt.Errorf("world.loading_screen: "+format, args...)
	}
	if l.ShortLimitMS <= 0 {
		return fail("short_limit_ms: expected a positive target")
	}
	// stages: the client's, in order, ending with ready
	for _, path := range []string{"entry", "travel"} {
		known := loadingStages[path]
		got := l.Stages[path]
		if len(got) == 0 || got[len(got)-1] != "ready" {
			return fail("stages.%s: expected stages ending with ready", path)
		}
		at := 0
		for _, stage := range got {
			for at < len(known) && known[at] != stage {
				at++
			}
			if at == len(known) {
				return fail("stages.%s: %q is not a stage the client detects, or out of order (%s)", path, stage, strings.Join(known, ", "))
			}
			at++
		}
	}
	for path := range l.Stages {
		if _, ok := loadingStages[path]; !ok {
			return fail("stages.%s: unknown path", path)
		}
	}
	// kinds: the ones the client and the server name, each with its form from its expected duration
	for _, name := range append(append([]string{}, loadingEntryKinds...), loadingTripKinds...) {
		if _, ok := l.Kinds[name]; !ok {
			return fail("kinds.%s: missing (the client or the server names it)", name)
		}
	}
	if _, ok := l.Kinds[l.FallbackKind]; !ok || l.Kinds[l.FallbackKind].Path != "travel" {
		return fail("fallback_kind: %q is no travel kind", l.FallbackKind)
	}
	locales := c.Localization.Locales
	texts := map[string]map[string]string{}
	for _, m := range c.Localization.Messages {
		texts[m.Key] = m.Texts
	}
	root := ""
	if c.GodotTheme != nil {
		root = filepath.Join(project, c.GodotTheme.Target)
	}
	// a public UI key in every locale; with the client's fonts at hand, fitting at the Deck size (1280x800: x0.74)
	fit := func(field, key string, px, width, lines int, ui bool) error {
		if key == "" {
			return fail("%s: empty text key", field)
		}
		if ui && !strings.HasPrefix(key, "ui.") {
			return fail("%s: %q is not a public interface key", field, key)
		}
		for _, locale := range locales {
			if strings.TrimSpace(texts[key][locale]) == "" {
				return fail("%s: no %s text for %s", field, locale, key)
			}
		}
		if root == "" {
			return nil
		}
		if err := screenLocalizedText(c, key, px, width, lines, root, ui); err != nil {
			return fail("%s: %w", field, err)
		}
		return nil
	}
	var deck = 0.74
	scaled := func(px float64) int { return max(12, int(px*deck+0.5)) }
	for _, name := range sortedScreenKeys(l.Kinds) {
		k := l.Kinds[name]
		switch k.Path {
		case "entry":
			if k.CoverFrom != "request" {
				return fail("kinds.%s.cover_from: an entry is covered from its request", name)
			}
		case "travel":
			if k.CoverFrom != "trip" && k.CoverFrom != "arrival" {
				return fail("kinds.%s.cover_from: a journey is covered from its trip or its arrival", name)
			}
		default:
			return fail("kinds.%s.path: unknown path %q", name, k.Path)
		}
		want := "short"
		if k.ExpectedMS > l.ShortLimitMS {
			want = "full"
		}
		if k.ExpectedMS <= 0 || k.Form != want {
			return fail("kinds.%s.form: %q, but its expected %d ms against the %d ms target makes it %q", name, k.Form, k.ExpectedMS, l.ShortLimitMS, want)
		}
		if k.FromKey != "" {
			if err := fit("kinds."+name+".from_key", k.FromKey, 12, 300, 1, true); err != nil {
				return err
			}
		}
		if k.ViaKey != "" {
			if err := fit("kinds."+name+".via_key", k.ViaKey, 12, 300, 1, true); err != nil {
				return err
			}
		}
	}
	for _, name := range loadingEntryKinds {
		if l.Kinds[name].Path != "entry" {
			return fail("kinds.%s.path: the client starts it as an entry", name)
		}
	}
	for _, name := range loadingTripKinds {
		if l.Kinds[name].Path != "travel" {
			return fail("kinds.%s.path: the server names it for a trip", name)
		}
	}
	for _, s := range l.SourceShown {
		switch s {
		case "saw", "told", "record", "rumour":
		default:
			return fail("source_shown: unknown journal source %q", s)
		}
	}
	// places: exactly the Life locations
	var life struct {
		Locations map[string]json.RawMessage `json:"locations"`
	}
	if err := json.Unmarshal(c.Life, &life); err != nil {
		return fail("life: %w", err)
	}
	for _, level := range sortedScreenKeys(life.Locations) {
		if _, ok := l.Places[level]; !ok {
			return fail("places: Life location %s has no place (its name, picture and route node)", level)
		}
	}
	if _, ok := life.Locations[l.Start.Level]; !ok {
		return fail("start.level: %q is no Life location", l.Start.Level)
	}
	nodes := map[string]RegionNode{}
	for _, n := range c.Region.Nodes {
		nodes[n.ID] = n
	}
	for _, level := range sortedScreenKeys(l.Places) {
		p := l.Places[level]
		field := "places." + level
		if _, ok := life.Locations[level]; !ok {
			return fail("%s: no Life location of that id", field)
		}
		// the name stands large on the full screen (52 px, two lines of 1080 px) and in one line on the short form
		if err := fit(field+".name_key", p.NameKey, scaled(52), int(1080*deck), 2, true); err != nil {
			return err
		}
		if err := fit(field+".name_key", p.NameKey, scaled(34), int(1000*deck), 1, true); err != nil {
			return err
		}
		for _, picture := range []struct{ name, path string }{{"render", p.Render}, {"painting", p.Painting}} {
			// no picture yet: the full screen stands on its dark ground with the name, the line and the route
			if picture.path == "" {
				continue
			}
			if !loadingPicture.MatchString(picture.path) {
				return fail("%s.%s: %q is not a picture under res://assets/loading/", field, picture.name, picture.path)
			}
			if root != "" {
				if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(picture.path, "res://")))); err != nil {
					return fail("%s.%s: %s is missing in the client", field, picture.name, picture.path)
				}
			}
		}
		if p.RouteNode != "" {
			n, ok := nodes[p.RouteNode]
			if !ok {
				return fail("%s.route_node: %q is no node of the chip's map (region_map)", field, p.RouteNode)
			}
			if n.LevelID != "" && n.LevelID != level {
				return fail("%s.route_node: %q is the node of %s", field, p.RouteNode, n.LevelID)
			}
		}
		seen := map[string]bool{}
		for _, id := range p.Knowledge {
			if seen[id] {
				return fail("%s.knowledge: %s twice", field, id)
			}
			seen[id] = true
			line := ""
			if c.Memory != nil {
				if f := c.Memory.Facts[id]; f != nil {
					line = f.Journal
				} else if t := c.Memory.Topics[id]; t != nil {
					line = t.Journal
				} else {
					return fail("%s.knowledge: %s is no fact or topic of the story memory", field, id)
				}
			}
			if line == "" {
				return fail("%s.knowledge: %s has no journal line, the hero can never know it as a line", field, id)
			}
			// the line: three lines of 25 px in 1080 px on the full screen, two of 21 px in 1000 px on the short form
			if err := fit(field+".knowledge."+id, line, scaled(25), int(1080*deck), 3, false); err != nil {
				return err
			}
			if err := fit(field+".knowledge."+id, line, scaled(21), int(1000*deck), 2, false); err != nil {
				return err
			}
		}
	}
	return nil
}

// clientLoadingScreen is the client's projection (presentation.json loading_screen): everything but the knowledge.
func clientLoadingScreen(l *JourneyScreen) map[string]any {
	places := map[string]any{}
	for level, p := range l.Places {
		places[level] = map[string]any{"name_key": p.NameKey, "painting": p.Painting, "render": p.Render, "route_node": p.RouteNode}
	}
	return map[string]any{"short_limit_ms": l.ShortLimitMS, "start": l.Start, "fallback_kind": l.FallbackKind,
		"stages": l.Stages, "kinds": l.Kinds, "places": places, "source_shown": l.SourceShown}
}
