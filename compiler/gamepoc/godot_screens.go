package gamepoc

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler/emitter"
)

// GodotScreens is presentation authoring. Paths refer to the normalized wire
// contract, never a second copy of a gameplay model or an executable expression.
type ScreenChrome struct {
	BottomOrnamentRects [][]float64 `json:"bottom_ornament_rects,omitempty"`
	OrnamentRects       [][]float64 `json:"ornament_rects,omitempty"`
	HeadingGapPX        int         `json:"heading_gap_px"`
	BodyRole            string      `json:"body_role"`
	FrameMargins        []float64   `json:"frame_margins"`
	Padding             []float64   `json:"padding"`
	OrnamentSize        []float64   `json:"ornament_size"`
	InnerBorder         float64     `json:"inner_border_px"`
	HeadingOutside      bool        `json:"heading_outside"`
	HeadingY            float64     `json:"heading_y"`
	HeadingHeight       float64     `json:"heading_height"`
	TitleRole           string      `json:"title_role"`
	TitlePX             int         `json:"title_px"`
	Gap                 int         `json:"gap_px"`
	// A column at one side of the screen, full height (the conversation like Disco Elysium, owner 20.11): the world stays
	// in view beside it. Width is a share of the screen, never below min_px; frame_margins still frame it.
	Column *ScreenColumn `json:"column,omitempty"`
}
type ScreenColumn struct {
	Side     string  `json:"side"`
	Fraction float64 `json:"fraction"`
	MinPX    float64 `json:"min_px"`
}
type ScreenCombatCategory struct {
	ID     string   `json:"id"`
	Key    string   `json:"text_key"`
	Groups []string `json:"groups"`
}
type ScreenHotkey struct {
	Key       string `json:"key"`
	Control   string `json:"control"`
	Index     int    `json:"index,omitempty"`
	Direction int    `json:"direction,omitempty"`
}
type ScreenFilter struct {
	Value ScreenValue `json:"value"`
	In    ScreenValue `json:"in"`
}
type ScreenRegionStyle struct {
	Background string `json:"background"`
	Border     string `json:"border"`
}
type ScreenTarget struct {
	PointerHeightM *float64               `json:"pointer_height_m,omitempty"`
	Action         string                 `json:"action,omitempty"`
	Position       string                 `json:"position"`
	ID             string                 `json:"id"`
	Source         ScreenValue            `json:"source"`
	Bind           map[string]ScreenValue `json:"bind"`
	Screen         string                 `json:"screen,omitempty"`
	List           string                 `json:"list,omitempty"`
	RowKey         string                 `json:"row_key,omitempty"`
	RadiusPX       int                    `json:"radius_px"`
	Priority       int                    `json:"priority"`
	Person         bool                   `json:"person,omitempty"`
}
type ScreenWorldInput struct {
	Peaceful     ScreenShortcut `json:"peaceful"`
	Hostile      ScreenShortcut `json:"hostile"`
	Fallback     string         `json:"fallback"`
	EngageScreen string         `json:"engage_screen"`
	EngageList   string         `json:"engage_list"`
	EngageKey    string         `json:"engage_key"`
}
type GodotScreens struct {
	Targets           []ScreenTarget               `json:"targets,omitempty"`
	WorldInput        *ScreenWorldInput            `json:"world_input,omitempty"`
	RegionStyles      map[string]ScreenRegionStyle `json:"region_styles,omitempty"`
	CombatShortLabels map[string]string            `json:"combat_short_labels,omitempty"`
	CombatCategories  []ScreenCombatCategory       `json:"combat_categories,omitempty"`
	ChromeProfiles    map[string]ScreenChrome      `json:"chrome_profiles,omitempty"`
	Routes            map[string]string            `json:"routes,omitempty"`
	Text              map[string]string            `json:"text,omitempty"`
	Background        string                       `json:"background,omitempty"`
	Response          string                       `json:"response,omitempty"`
	Assets            map[string]string            `json:"assets,omitempty"`
	Themes            map[string]string            `json:"themes,omitempty"`
	ReviewAction      *ScreenAction                `json:"review_action,omitempty"`
	DisciplineSkins   map[string]string            `json:"discipline_skins,omitempty"`
	Launcher          *ScreenLauncher              `json:"launcher,omitempty"`
	PortraitFallback  string                       `json:"portrait_fallback,omitempty"`
	Widgets           map[string]string            `json:"widgets"`
	Screens           []GodotScreen                `json:"screens"`
}
type ScreenLauncher struct {
	Screen string `json:"screen"`
	Key    string `json:"text_key"`
}
type ScreenShortcut struct {
	Key string `json:"key,omitempty"`
	Pad string `json:"pad,omitempty"`
}
type GodotScreen struct {
	// The HUD (routes.hud): its own runtime shows it under every other screen while the game is played; the world keeps
	// its input (only the HUD's buttons take the mouse), its hotkeys act while nothing else is open, Esc and B stay the
	// game's. At most one, layout hud, never opened by the server.
	Persistent      bool            `json:"persistent,omitempty"`
	HideWorldStatus bool            `json:"hide_world_status,omitempty"`
	ClearRect       []float64       `json:"clear_rect,omitempty"`
	Hotkeys         []ScreenHotkey  `json:"hotkeys,omitempty"`
	StatusID        string          `json:"status_id,omitempty"`
	ChromeProfile   string          `json:"chrome_profile,omitempty"`
	KickerKey       string          `json:"kicker_key,omitempty"`
	InitialFocus    string          `json:"initial_focus,omitempty"`
	Heading         *bool           `json:"heading,omitempty"`
	HideClose       bool            `json:"hide_close,omitempty"`
	Backdrop        string          `json:"backdrop,omitempty"`
	OpeningMode     string          `json:"opening_mode,omitempty"`
	PauseWorld      bool            `json:"pause_world,omitempty"`
	Shortcut        *ScreenShortcut `json:"shortcut,omitempty"`
	ID              string          `json:"id"`
	Chrome          string          `json:"chrome"`
	Skin            string          `json:"skin"`
	SkinRule        *ScreenValue    `json:"skin_rule,omitempty"`
	Title           string          `json:"title_key"`
	Layout          string          `json:"layout"`
	Opening         *ScreenValue    `json:"opening,omitempty"`
	Close           ScreenAction    `json:"close"`
	Focus           []string        `json:"focus"`
	Controls        []ScreenControl `json:"controls"`
}
type ScreenControl struct {
	LineSpacingPX *int           `json:"line_spacing_px,omitempty"`
	InsetPX       []int          `json:"inset_px,omitempty"`
	Order         []string       `json:"order,omitempty"`
	PaddingPX     int            `json:"padding_px,omitempty"`
	Columns       int            `json:"columns,omitempty"`
	VisibleRows   int            `json:"visible_rows,omitempty"`
	MinHeight     int            `json:"min_height_px,omitempty"`
	Collapsed     bool           `json:"collapsed,omitempty"`
	Filters       []ScreenFilter `json:"filters,omitempty"`
	RowTitle      bool           `json:"row_title,omitempty"`
	GapPX         int            `json:"gap_px,omitempty"`
	Ordinal       int            `json:"ordinal,omitempty"`
	Align         string         `json:"align,omitempty"`
	ListWidthPX   int            `json:"list_width_px,omitempty"`
	RowKeys       []string       `json:"row_keys,omitempty"`
	Where         *ScreenValue   `json:"where,omitempty"`
	Style         string         `json:"style,omitempty"`
	Formatter     string         `json:"formatter,omitempty"`
	FormatKey     string         `json:"format_key,omitempty"`
	HeightPX      int            `json:"height_px,omitempty"`
	Weight        float64        `json:"weight,omitempty"`
	Passive       bool           `json:"passive,omitempty"`
	// A custom widget whose values change often (a clock, the blood): they stay out of the screen's rebuild signature,
	// and the widget gets them in update_screen(values) on every state instead of being rebuilt.
	Live         bool                   `json:"live,omitempty"`
	ActionSource string                 `json:"action_source,omitempty"`
	Focusable    bool                   `json:"focusable,omitempty"`
	Rect         []float64              `json:"rect,omitempty"`
	RowKey       string                 `json:"row_key,omitempty"`
	Dock         string                 `json:"dock,omitempty"`
	EmptyKey     string                 `json:"empty_key,omitempty"`
	Mode         string                 `json:"mode,omitempty"`
	Join         string                 `json:"join,omitempty"`
	ID           string                 `json:"id"`
	Kind         string                 `json:"kind"`
	Key          string                 `json:"text_key,omitempty"`
	Widget       string                 `json:"widget,omitempty"`
	FontPX       int                    `json:"font_px"`
	WidthPX      int                    `json:"width_px"`
	Lines        int                    `json:"lines"`
	LinePX       []int                  `json:"line_px,omitempty"`
	Bind         map[string]ScreenValue `json:"bind,omitempty"`
	Action       *ScreenAction          `json:"action,omitempty"`
	// A custom widget's further actions by name (a save card's held delete beside its load): each declared and checked
	// like action, on the same row; the widget runs one through the context's act(name, row).
	Actions  map[string]*ScreenAction `json:"actions,omitempty"`
	Children []ScreenControl          `json:"children,omitempty"`
}
type ScreenValue struct {
	Path  string `json:"path,omitempty"`
	Value any    `json:"value,omitempty"`
	Ref   string `json:"ref,omitempty"`
}
type ScreenAction struct {
	Command string                 `json:"command"`
	Params  map[string]ScreenValue `json:"params,omitempty"`
	Ref     string                 `json:"ref,omitempty"`
}
type screenCompiler struct {
	screens  *GodotScreens
	settings *GodotSettings
	def      *normalizer.GRPCDef
	messages map[string]normalizer.GRPCMessage
	selected map[string]string
	bindings strings.Builder
	readers  []string
	actions  strings.Builder
	serial   int
	id       string
}
type screenPath struct {
	typ        string
	repeated   bool
	expression string
	guards     []string
}

func (g *screenCompiler) field(typ, name string) (normalizer.GRPCField, bool) {
	m := g.messages[typ]
	fields := append([]normalizer.GRPCField(nil), m.Fields...)
	for _, one := range m.Oneofs {
		fields = append(fields, one.Members...)
	}
	for _, f := range fields {
		if f.Name == name {
			return f, true
		}
	}
	return normalizer.GRPCField{}, false
}
func (g *screenCompiler) path(path, row string) (screenPath, error) {
	p := strings.Split(path, ".")
	r := screenPath{}
	switch p[0] {
	case "settings":
		// settings.<id>: the setting's words as the settings screen shows them; settings.<id>.value: its value (a
		// widget's short word for it, the HUD's pace plate)
		if (len(p) != 2 && !(len(p) == 3 && p[2] == "value")) || g.settings == nil {
			return r, fmt.Errorf("unknown settings path %q", path)
		}
		for _, s := range g.settings.Entries {
			if s.ID == p[1] {
				read := "display"
				if len(p) == 3 {
					read = "value"
				}
				return screenPath{typ: "string", expression: "str((selected.get(\"__settings\") as Settings)." + read + "(" + strconv.Quote(s.ID) + "))", guards: []string{"selected.get(\"__settings\") == null"}}, nil
			}
		}
		return r, fmt.Errorf("unknown settings path %q", path)
	case "saves":
		r.typ, r.expression = "PocSaveSlots", "(selected.get(\"__saves\") as C.PocSaveSlots)"
	case "state":
		r.typ, r.expression = "PocGameState", "state"
	case "row":
		r.typ, r.expression = row, "(row as C."+row+")"
	case "selected":
		if len(p) < 2 {
			return r, fmt.Errorf("selected needs a list id")
		}
		r.typ = g.selected[p[1]]
		r.expression = "(selected.get(" + strconv.Quote(p[1]) + ") as C." + r.typ + ")"
		p = p[1:]
	case "request_id":
		if len(p) == 1 {
			return screenPath{typ: "string", expression: "request_id"}, nil
		}
	default:
		return r, fmt.Errorf("unknown contract path %q", path)
	}
	if r.typ == "" {
		return r, fmt.Errorf("unknown row/list in contract path %q", path)
	}
	for _, part := range p[1:] {
		if r.repeated {
			return r, fmt.Errorf("contract path %q traverses a collection without a list", path)
		}
		f, ok := g.field(r.typ, part)
		if !ok {
			return r, fmt.Errorf("unknown contract path %q (%s.%s)", path, r.typ, part)
		}
		r.guards = append(r.guards, r.expression+" == null")
		r.expression += "." + emitter.GDScriptFieldName(part)
		r.typ, r.repeated = f.Type, f.Repeated
	}
	return r, nil
}
func (g *screenCompiler) bind(v *ScreenValue, row, field string) (screenPath, error) {
	if v.Path == "" {
		return screenPath{}, nil
	}
	p, err := g.path(v.Path, row)
	if err != nil {
		return p, fmt.Errorf("screen %s field %s: %w", g.id, field, err)
	}
	g.serial++
	v.Ref = fmt.Sprintf("b%d", g.serial)
	g.readers = append(g.readers, v.Ref)
	fmt.Fprintf(&g.bindings, `
@warning_ignore("unused_parameter")
static func _read_%s(
	state: C.PocGameState, row: C.ProtoMessage,
	selected: Dictionary, request_id: String
) -> Variant:
`, v.Ref)
	if len(p.guards) > 0 {
		fmt.Fprintf(&g.bindings, "\tif %s:\n\t\treturn null\n", strings.Join(p.guards, " or "))
	}
	fmt.Fprintf(&g.bindings, "\treturn %s\n", p.expression)
	return p, nil
}

// Host commands are presentation/lifecycle operations, deliberately closed. host.delete_save deletes the save the
// action's row names (its slot id): the host sends it to the server, which answers with the new list.
var screenHostCommands = map[string]bool{"host.close": true, "host.open": true, "host.back": true, "host.main": true, "host.quit": true, "host.setting": true, "host.toggle": true, "host.delete_save": true, "host.world": true}

// The verbs of host.world: the HUD's touches of the world, through the world targets the client already has (doc 09,
// «Касание мира»): use = the peaceful action on the pointed or nearest target (E/X), talk = the nearest person's,
// approach = walk to the pointed or nearest target without using it, inspect = point at the next target near the hero
// (its server inspection text shows), hostile = the choice of whom to engage (F/R1; never an ordinary click).
var screenWorldVerbs = map[string]bool{"use": true, "talk": true, "approach": true, "inspect": true, "hostile": true}

func (g *screenCompiler) action(a *ScreenAction, row, field string) error {
	if a.Command == "" {
		return fmt.Errorf("screen %s field %s.command: action has no command", g.id, field)
	}
	g.serial++
	a.Ref = fmt.Sprintf("a%d", g.serial)
	if screenHostCommands[a.Command] {
		allowed := map[string]string{}
		switch a.Command {
		case "host.open":
			allowed["screen"] = "screen"
		case "host.setting":
			allowed["id"] = "setting"
		case "host.toggle":
			allowed["control"] = "control"
		case "host.delete_save":
			allowed["slot"] = "row string"
		case "host.world":
			allowed["verb"] = "verb"
		}
		for _, key := range sortedScreenKeys(a.Params) {
			if _, ok := allowed[key]; !ok {
				return fmt.Errorf("screen %s field %s.params.%s: unknown host parameter", g.id, field, key)
			}
		}
		for key := range allowed {
			if _, ok := a.Params[key]; !ok {
				return fmt.Errorf("screen %s field %s.params.%s: required", g.id, field, key)
			}
		}
		for _, key := range sortedScreenKeys(a.Params) {
			kind, ok := allowed[key]
			if !ok {
				return fmt.Errorf("screen %s field %s.params.%s: unknown host parameter", g.id, field, key)
			}
			v := a.Params[key]
			if kind == "row string" {
				// read from the row the action is on (a save card's slot id), never a literal
				if v.Path == "" {
					return fmt.Errorf("screen %s field %s.params.%s: expected a binding to a row's string", g.id, field, key)
				}
				p, err := g.bind(&v, row, field+".params."+key)
				if err != nil {
					return err
				}
				if p.typ != "string" || p.repeated {
					return fmt.Errorf("screen %s field %s.params.%s: %s cannot bind a string", g.id, field, key, p.typ)
				}
				a.Params[key] = v
				continue
			}
			id, literal := v.Value.(string)
			if v.Path != "" || !literal {
				return fmt.Errorf("screen %s field %s.params.%s: expected declared %s", g.id, field, key, kind)
			}
			found := false
			switch kind {
			case "verb":
				found = screenWorldVerbs[id]
			case "screen":
				for _, screen := range g.screens.Screens {
					if screen.ID == id {
						found = true
					}
				}
			case "setting":
				if g.settings != nil {
					for _, setting := range g.settings.Entries {
						if setting.ID == id {
							found = true
						}
					}
				}
			case "control":
				for _, screen := range g.screens.Screens {
					if screen.ID == g.id && findScreenControl(screen.Controls, id) != nil {
						found = true
					}
				}
			}
			if !found {
				return fmt.Errorf("screen %s field %s.params.%s: unknown %s", g.id, field, key, kind)
			}
			if _, err := g.bind(&v, row, field+".params."+key); err != nil {
				return err
			}
			a.Params[key] = v
		}
		return nil
	}
	f, ok := g.field("ClientMessage", a.Command)
	if !ok || g.messages[f.Type].Name == "" {
		return fmt.Errorf("screen %s field %s.command: unknown contract command %q", g.id, field, a.Command)
	}
	fmt.Fprintf(&g.actions, "\t\t%q:\n\t\t\tvar q := C.%s.new()\n", a.Ref, f.Type)
	for _, key := range sortedScreenKeys(a.Params) {
		v := a.Params[key]
		param, ok := g.field(f.Type, key)
		if !ok {
			return fmt.Errorf("screen %s field %s.params.%s: unknown command field", g.id, field, key)
		}
		p, err := g.bind(&v, row, field+".params."+key)
		if err != nil {
			return err
		}
		a.Params[key] = v
		expr := ""
		if v.Path != "" {
			if p.typ != param.Type && !(screenNumber(p.typ) && screenNumber(param.Type)) {
				return fmt.Errorf("screen %s field %s.params.%s: %s cannot bind %s", g.id, field, key, p.typ, param.Type)
			}
			expr = fmt.Sprintf("read(%q,state,row,selected,request_id)", v.Ref)
		} else {
			b, _ := json.Marshal(v.Value)
			expr = string(b)
			if !screenLiteral(v.Value, param.Type) {
				return fmt.Errorf("screen %s field %s.params.%s: invalid literal for %s", g.id, field, key, param.Type)
			}
		}
		fmt.Fprintf(&g.actions, "\t\t\tq.%s = %s\n", emitter.GDScriptFieldName(key), expr)
	}
	fmt.Fprintf(&g.actions, "\t\t\tmessage.%s = q\n", emitter.GDScriptFieldName(a.Command))
	return nil
}

// screenValueKeys walks literal data handed to a custom widget: a string under a key named text_key or ending in _key is
// a UI text key and must exist in every locale (its fit is the widget's, measured by the real Godot proofs).
func screenValueKeys(c Compiled, v any, root string) error {
	switch x := v.(type) {
	case map[string]any:
		for _, k := range sortedScreenKeys(x) {
			if key, ok := x[k].(string); ok && (k == "text_key" || strings.HasSuffix(k, "_key")) && key != "" {
				if err := screenText(c, key, 12, 1280, 8, root); err != nil {
					return fmt.Errorf("%s: %w", k, err)
				}
				continue
			}
			if err := screenValueKeys(c, x[k], root); err != nil {
				return fmt.Errorf("%s.%w", k, err)
			}
		}
	case []any:
		for i, item := range x {
			if err := screenValueKeys(c, item, root); err != nil {
				return fmt.Errorf("%d.%w", i, err)
			}
		}
	}
	return nil
}
func screenNumber(t string) bool {
	return strings.Contains(" int32 int64 uint32 uint64 float double ", " "+t+" ")
}
func screenLiteral(v any, t string) bool {
	switch v.(type) {
	case string:
		return t == "string"
	case bool:
		return t == "bool"
	case float64, float32, int, int32, int64, uint, uint32, uint64, json.Number:
		return screenNumber(t)
	}
	return false
}
func sortedScreenKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// preselect learns the row type of every list (and every custom widget choosing among rows) of a screen before its
// controls are checked; a source that reads another list's choice resolves on a later round.
func (g *screenCompiler) preselect(nodes []ScreenControl, row string) {
	for round := 0; round < 4; round++ {
		var walk func([]ScreenControl, string)
		walk = func(nodes []ScreenControl, row string) {
			for i := range nodes {
				n := &nodes[i]
				childRow := row
				if n.Kind == "list" || (n.Kind == "custom" && n.ActionSource == "source") {
					if v, ok := n.Bind["source"]; ok && g.selected[n.ID] == "" {
						if p, err := g.path(v.Path, row); err == nil && p.repeated {
							g.selected[n.ID] = p.typ
						}
					}
					if n.Kind == "list" {
						childRow = g.selected[n.ID]
					}
				}
				walk(n.Children, childRow)
			}
		}
		walk(nodes, row)
	}
}

func (g *screenCompiler) controls(nodes []ScreenControl, row string, ids, interactive map[string]bool, c Compiled, root string) error {
	for i := range nodes {
		n := &nodes[i]
		field := "controls." + n.ID
		if !godotResourceID.MatchString(n.ID) || ids[n.ID] {
			return fmt.Errorf("screen %s field %s.id: invalid/duplicate id", g.id, field)
		}
		ids[n.ID] = true
		if n.Kind == "region" {
			if n.Style != "" {
				if _, ok := c.GodotScreens.RegionStyles[n.Style]; !ok {
					return fmt.Errorf("screen %s field %s.style: unknown tactical panel style", g.id, field)
				}
			}
			if len(n.Rect) != 4 || n.Rect[0] < 0 || n.Rect[1] < 0 || n.Rect[2] <= 0 || n.Rect[3] <= 0 || n.Rect[0]+n.Rect[2] > 1280 || n.Rect[1]+n.Rect[3] > 800 {
				return fmt.Errorf("screen %s field %s.rect: region outside 1280x800 safe area", g.id, field)
			}
		}
		if n.LineSpacingPX != nil && (*n.LineSpacingPX < 0 || *n.LineSpacingPX > 48) {
			return fmt.Errorf("screen %s field %s.line_spacing_px: expected 0..48", g.id, field)
		}
		if len(n.InsetPX) != 0 {
			if len(n.InsetPX) != 4 {
				return fmt.Errorf("screen %s field %s.inset_px: expected four margins", g.id, field)
			}
			for _, margin := range n.InsetPX {
				if margin < 0 {
					return fmt.Errorf("screen %s field %s.inset_px: negative margin", g.id, field)
				}
			}
		}
		if n.VisibleRows < 0 || n.VisibleRows > 9 || (n.VisibleRows > 0 && (n.Kind != "list" || n.Columns < 1 || (n.Mode != "choices" && n.Mode != "commands"))) {
			return fmt.Errorf("screen %s field %s.visible_rows: expected 0..9 and a grid list", g.id, field)
		}
		if n.Columns < 0 || n.Columns > 9 {
			return fmt.Errorf("screen %s field %s.columns: expected 0..9", g.id, field)
		}
		if n.Dock != "" && n.Dock != "body" && n.Dock != "top" && n.Dock != "bottom" {
			return fmt.Errorf("screen %s field %s.dock: unknown dock", g.id, field)
		}
		switch n.Kind {
		case "divider", "spacer", "image", "region", "stack", "row", "detail", "label", "list", "button", "custom":
		default:
			return fmt.Errorf("screen %s field %s.kind: unknown kind %q", g.id, field, n.Kind)
		}
		if (n.Kind == "list" && n.Mode != "cards") || n.Kind == "button" || (n.Kind == "custom" && !n.Passive) || n.Focusable {
			interactive[n.ID] = true
		}
		if (n.Kind == "button" || (n.Kind == "list" && n.Mode == "commands")) && n.Action == nil {
			return fmt.Errorf("screen %s field %s.action: action has no command", g.id, field)
		}
		if n.Kind == "list" && (n.Mode == "choices" || n.Mode == "commands") {
			if len(n.Children) == 0 {
				return fmt.Errorf("screen %s field %s.children: choices need a label", g.id, field)
			}
			for _, child := range n.Children {
				if child.Kind != "label" {
					return fmt.Errorf("screen %s field %s.children: choices/commands show labels only", g.id, field)
				}
			}
		}
		if n.Live && n.Kind != "custom" {
			return fmt.Errorf("screen %s field %s.live: only a custom widget updates in place", g.id, field)
		}
		if n.Kind == "custom" {
			// the text keys a custom widget is handed in literal data (its look, a HUD deck's slots): every key named
			// text_key or ending in _key must be a public UI text in every locale, as a control's own text_key
			for _, k := range sortedScreenKeys(n.Bind) {
				value := n.Bind[k].Value
				if key, ok := value.(string); ok && strings.HasSuffix(k, "_key") && key != "" {
					// a bound key itself (hint_key: value: "ui.…")
					value = map[string]any{k: key}
				}
				if err := screenValueKeys(c, value, root); err != nil {
					return fmt.Errorf("screen %s field %s.bind.%s: %w", g.id, field, k, err)
				}
			}
			path := c.GodotScreens.Widgets[n.Widget]
			if !godotScenePath(path) && (!strings.HasPrefix(path, "res://") || !strings.HasSuffix(path, ".gd") || strings.Contains(path, "..")) {
				return fmt.Errorf("screen %s field %s.widget: unregistered custom widget %q", g.id, field, n.Widget)
			}
		}
		if n.Kind == "image" {
			if _, ok := c.GodotScreens.Assets[n.Widget]; !ok {
				return fmt.Errorf("screen %s field %s.widget: unregistered image", g.id, field)
			}
		}
		if n.Formatter != "" && n.Formatter != "datetime" && n.Formatter != "format" {
			return fmt.Errorf("screen %s field %s.formatter: unknown formatter", g.id, field)
		}

		textWidth, textLines := n.WidthPX, n.Lines
		if n.Kind == "button" {
			textWidth -= 32
			if n.Mode != "response" && n.Mode != "tile" {
				textLines = 1
			}
		}
		if err := screenTextSized(c, n.Key, n.FontPX, n.LinePX, textWidth, textLines, root); err != nil {
			return fmt.Errorf("screen %s field %s.text_key: %w", g.id, field, err)
		}
		if err := screenText(c, n.FormatKey, n.FontPX, n.WidthPX, 2, root); err != nil {
			return fmt.Errorf("screen %s field %s.format_key: %w", g.id, field, err)
		}
		if err := screenText(c, n.EmptyKey, 18, n.WidthPX, 8, root); err != nil {
			return fmt.Errorf("screen %s field %s.empty_key: %w", g.id, field, err)
		}
		if n.Kind == "list" || (n.Kind == "custom" && n.ActionSource == "source") {
			if n.Mode != "" && n.Mode != "select" && n.Mode != "cards" && n.Mode != "choices" && n.Mode != "commands" {
				return fmt.Errorf("screen %s field %s.mode: unknown list mode", g.id, field)
			}
			v, ok := n.Bind["source"]
			if !ok {
				return fmt.Errorf("screen %s field %s.bind.source: list needs collection", g.id, field)
			}
			p, err := g.path(v.Path, row)
			if err != nil || !p.repeated || g.messages[p.typ].Name == "" {
				return fmt.Errorf("screen %s field %s.bind.source: expected contract message collection %s: %v", g.id, field, v.Path, err)
			}
			if n.RowKey != "" {
				if _, err := g.path("row."+n.RowKey, p.typ); err != nil {
					return fmt.Errorf("screen %s field %s.row_key: %w", g.id, field, err)
				}
			}
			for _, key := range n.RowKeys {
				if _, err := g.path("row."+key, p.typ); err != nil {
					return fmt.Errorf("screen %s field %s.row_keys: %w", g.id, field, err)
				}
			}
			if n.Where != nil {
				if where, err := g.bind(n.Where, p.typ, field+".where"); err != nil {
					return err
				} else if where.typ != "bool" {
					return fmt.Errorf("screen %s field %s.where: expected server boolean", g.id, field)
				}
			}
			g.selected[n.ID] = p.typ
		}
		for index := range n.Filters {
			filter := &n.Filters[index]
			filterRow := row
			if n.Kind == "list" || (n.Kind == "custom" && n.ActionSource == "source") {
				filterRow = g.selected[n.ID]
			}
			field := fmt.Sprintf("%s.filters.%d", field, index)
			value, err := g.bind(&filter.Value, filterRow, field+".value")
			if err != nil {
				return err
			}
			values, err := g.bind(&filter.In, filterRow, field+".in")
			if err != nil {
				return err
			}
			if value.repeated || !values.repeated || value.typ != values.typ {
				return fmt.Errorf("screen %s field %s.in: expected collection of %s", g.id, field, value.typ)
			}
		}
		for _, k := range sortedScreenKeys(n.Bind) {
			v := n.Bind[k]
			bindRow := row
			if n.Kind == "list" && n.Mode == "commands" && (k == "enabled" || k == "reason") {
				bindRow = g.selected[n.ID]
			}
			if _, err := g.bind(&v, bindRow, field+".bind."+k); err != nil {
				return err
			}
			n.Bind[k] = v
		}
		if n.Action != nil {
			actionRow := row
			if n.Kind == "list" {
				actionRow = g.selected[n.ID]
			}
			if n.Kind == "custom" && n.ActionSource != "" {
				p, err := g.path(n.Bind[n.ActionSource].Path, row)
				if err != nil || !p.repeated {
					return fmt.Errorf("screen %s field %s.action_source: expected a contract collection", g.id, field)
				}
				actionRow = p.typ
			}
			if err := g.action(n.Action, actionRow, field+".action"); err != nil {
				return err
			}
		}
		if len(n.Actions) > 0 {
			if n.Kind != "custom" {
				return fmt.Errorf("screen %s field %s.actions: only a custom widget takes named actions", g.id, field)
			}
			actionRow := row
			if n.ActionSource != "" {
				p, err := g.path(n.Bind[n.ActionSource].Path, row)
				if err != nil || !p.repeated {
					return fmt.Errorf("screen %s field %s.action_source: expected a contract collection", g.id, field)
				}
				actionRow = p.typ
			}
			for _, name := range sortedScreenKeys(n.Actions) {
				if !godotResourceID.MatchString(name) || n.Actions[name] == nil {
					return fmt.Errorf("screen %s field %s.actions.%s: invalid action name or empty action", g.id, field, name)
				}
				if err := g.action(n.Actions[name], actionRow, field+".actions."+name); err != nil {
					return err
				}
			}
		}
		childRow := row
		if n.Kind == "list" {
			childRow = g.selected[n.ID]
		}
		if err := g.controls(n.Children, childRow, ids, interactive, c, root); err != nil {
			return err
		}
	}
	return nil
}

func compileGodotScreens(project string, c Compiled, def *normalizer.GRPCDef) ([]byte, error) {
	if c.GodotScreens == nil {
		return nil, nil
	}
	if c.GodotTheme == nil {
		id := "unknown"
		if len(c.GodotScreens.Screens) > 0 {
			id = c.GodotScreens.Screens[0].ID
		}
		return nil, fmt.Errorf("screen %s field skin: no godot_theme skins declared", id)
	}
	g := &screenCompiler{screens: c.GodotScreens, settings: c.GodotSettings, def: def, messages: map[string]normalizer.GRPCMessage{}, selected: map[string]string{}}
	for _, m := range def.Messages {
		g.messages[m.Name] = m
	}
	if c.GodotSettings != nil {
		if _, err := compileGodotSettings(project, c, def); err != nil {
			return nil, err
		}
	}
	seen := map[string]bool{}
	persistent := ""
	skins := map[string]bool{}
	if c.GodotTheme != nil {
		for _, s := range c.GodotTheme.Skins {
			skins[s.ID] = true
		}
	}
	c.GodotScreens.Themes = map[string]string{}
	for _, skin := range c.GodotTheme.Skins {
		folder := c.GodotTheme.OutputDir
		if folder == "" {
			folder = "generated/theme"
		}
		c.GodotScreens.Themes[skin.ID] = "res://" + folder + "/" + skin.ID + ".tres"
	}
	root := filepath.Join(project, c.GodotTheme.Target)
	for discipline, skin := range c.GodotScreens.DisciplineSkins {
		if !skins[skin] {
			return nil, fmt.Errorf("screen crafting field discipline_skins.%s: unknown skin %s", discipline, skin)
		}
	}
	for action, key := range c.GodotScreens.CombatShortLabels {
		if err := screenText(c, key, 14, 196, 2, root); err != nil {
			return nil, fmt.Errorf("screen combat field combat_short_labels.%s: %w", action, err)
		}
	}
	for _, category := range c.GodotScreens.CombatCategories {
		if err := screenText(c, category.Key, 12, 138, 2, root); err != nil {
			return nil, fmt.Errorf("screen combat field combat_categories.%s.text_key: %w", category.ID, err)
		}
	}
	for _, id := range sortedScreenKeys(c.Inventory.Definitions) {
		definition := c.Inventory.Definitions[id]
		if err := screenLocalizedText(c, definition.Name, 24, 620, 8, root, false); err != nil {
			return nil, fmt.Errorf("screen inventory item %s field name: %w", id, err)
		}
	}
	for i := range c.GodotScreens.Screens {
		s := &c.GodotScreens.Screens[i]
		g.id = s.ID
		g.selected = map[string]string{}
		if !godotResourceID.MatchString(s.ID) || seen[s.ID] {
			return nil, fmt.Errorf("screen %s field id: invalid/duplicate", s.ID)
		}
		seen[s.ID] = true
		if s.Shortcut != nil {
			if s.Shortcut.Key != "" && (len(s.Shortcut.Key) != 1 || s.Shortcut.Key[0] < 'A' || s.Shortcut.Key[0] > 'Z') {
				return nil, fmt.Errorf("screen %s field shortcut.key: expected A-Z", s.ID)
			}
			if s.Shortcut.Pad != "" && !strings.Contains(" X Y Start Back L1 R1 ", " "+s.Shortcut.Pad+" ") {
				return nil, fmt.Errorf("screen %s field shortcut.pad: unknown button", s.ID)
			}
		}
		if s.Chrome != "folk" && s.Chrome != "tactical" {
			return nil, fmt.Errorf("screen %s field chrome: unknown chrome %q", s.ID, s.Chrome)
		}
		if !skins[s.Skin] {
			return nil, fmt.Errorf("screen %s field skin: unknown skin %q", s.ID, s.Skin)
		}
		if s.Layout != "page" && s.Layout != "split" && s.Layout != "overlay" && s.Layout != "hud" {
			return nil, fmt.Errorf("screen %s field layout: unknown layout %q", s.ID, s.Layout)
		}
		if s.ChromeProfile != "" {
			profile, ok := c.GodotScreens.ChromeProfiles[s.ChromeProfile]
			if !ok {
				return nil, fmt.Errorf("screen %s field chrome_profile: unknown chrome geometry", s.ID)
			}
			if err := validateScreenChrome(profile); err != nil {
				return nil, fmt.Errorf("screen %s field chrome_profile.%s: %w", s.ID, s.ChromeProfile, err)
			}
		}
		if err := screenText(c, s.KickerKey, 14, 1000, 2, root); err != nil {
			return nil, fmt.Errorf("screen %s field kicker_key: %w", s.ID, err)
		}
		titlePX, titleWidth, titleLines := 26, 1000, 2
		if profile, ok := c.GodotScreens.ChromeProfiles[s.ChromeProfile]; ok && profile.HeadingOutside {
			titlePX = profile.TitlePX
			titleWidth = int(1280-profile.FrameMargins[0]-profile.FrameMargins[2]) - 170
			titleLines = 1
		}
		if err := screenText(c, s.Title, titlePX, titleWidth, titleLines, root); err != nil {
			return nil, fmt.Errorf("screen %s field title_key: %w", s.ID, err)
		}
		if s.Opening != nil {
			if _, err := g.bind(s.Opening, "", "opening"); err != nil {
				return nil, err
			}
		}
		if s.Persistent {
			if s.Layout != "hud" || s.Opening != nil || s.Shortcut != nil || persistent != "" {
				return nil, fmt.Errorf("screen %s field persistent: one HUD, layout hud, never opened by the server or a shortcut", s.ID)
			}
			persistent = s.ID
		}
		// a control may read the chosen row of any list of its screen (selected.<id>), wherever that list stands: the
		// HUD's body-zone card shows the chance of the command chosen below it, whose cells are filtered by the zone
		g.preselect(s.Controls, "")
		ids, interactive := map[string]bool{}, map[string]bool{}
		if err := g.controls(s.Controls, "", ids, interactive, c, root); err != nil {
			return nil, err
		}
		if s.SkinRule != nil {
			if _, err := g.bind(s.SkinRule, "", "skin_rule"); err != nil {
				return nil, err
			}
		}
		if err := g.action(&s.Close, "", "close"); err != nil {
			return nil, err
		}
		reachable := map[string]bool{}
		for _, id := range s.Focus {
			if !interactive[id] || reachable[id] {
				return nil, fmt.Errorf("screen %s field focus: unknown/duplicate control %q", s.ID, id)
			}
			reachable[id] = true
		}
		if len(s.ClearRect) > 0 {
			if len(s.ClearRect) != 4 || s.ClearRect[0] < 0 || s.ClearRect[1] < 0 ||
				s.ClearRect[2] <= 0 || s.ClearRect[3] <= 0 ||
				s.ClearRect[0]+s.ClearRect[2] > 1280 || s.ClearRect[1]+s.ClearRect[3] > 800 {
				return nil, fmt.Errorf("screen %s field clear_rect: outside viewport", s.ID)
			}
			var check func([]ScreenControl) error
			check = func(nodes []ScreenControl) error {
				for _, node := range nodes {
					if node.Kind == "region" && !node.Collapsed && screenRectsOverlap(s.ClearRect, node.Rect) {
						return fmt.Errorf("screen %s field controls.%s.rect: overlaps clear_rect", s.ID, node.ID)
					}
					if err := check(node.Children); err != nil {
						return err
					}
				}
				return nil
			}
			if err := check(s.Controls); err != nil {
				return nil, err
			}
		}
		if s.InitialFocus != "" && !interactive[s.InitialFocus] {
			return nil, fmt.Errorf("screen %s field initial_focus: unknown control", s.ID)
		}
		if s.StatusID != "" && !ids[s.StatusID] {
			return nil, fmt.Errorf("screen %s field status_id: unknown control", s.ID)
		}
		keys := map[string]bool{}
		for index, key := range s.Hotkeys {
			field := fmt.Sprintf("hotkeys.%d", index)
			valid := len(key.Key) == 1 && strings.Contains("123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ", key.Key)
			valid = valid || key.Key == "Space" || key.Key == "Left" || key.Key == "Right" || key.Key == "Tab"
			if !valid || keys[key.Key] {
				return nil, fmt.Errorf("screen %s field %s.key: unknown/duplicate key", s.ID, field)
			}
			keys[key.Key] = true
			if !interactive[key.Control] {
				return nil, fmt.Errorf("screen %s field %s.control: unknown control", s.ID, field)
			}
			if key.Index < 0 || key.Direction < -1 || key.Direction > 1 {
				return nil, fmt.Errorf("screen %s field %s: invalid index/direction", s.ID, field)
			}
		}
		for _, id := range sortedScreenKeys(interactive) {
			if !reachable[id] {
				return nil, fmt.Errorf("screen %s field focus: unreachable control %s", s.ID, id)
			}
		}
	}
	for route, id := range c.GodotScreens.Routes {
		if !seen[id] {
			return nil, fmt.Errorf("screen %s field routes.%s: unknown screen", id, route)
		}
	}
	if c.GodotScreens.Routes["hud"] != persistent {
		return nil, fmt.Errorf("screen %s field routes.hud: the route names the one persistent screen (%q)", c.GodotScreens.Routes["hud"], persistent)
	}
	for name, key := range c.GodotScreens.Text {
		if err := screenText(c, key, 19, 300, 2, root); err != nil {
			return nil, fmt.Errorf("screen chrome field text.%s: %w", name, err)
		}
	}
	if action := c.GodotScreens.ReviewAction; action != nil {
		g.id = "review"
		if err := g.action(action, "", "review_action"); err != nil {
			return nil, err
		}
	}
	if l := c.GodotScreens.Launcher; l != nil {
		if !seen[l.Screen] {
			return nil, fmt.Errorf("screen %s field launcher.screen: unknown screen", l.Screen)
		}
		if err := screenText(c, l.Key, 19, 300, 2, root); err != nil {
			return nil, fmt.Errorf("screen %s field launcher.text_key: %w", l.Screen, err)
		}
	}
	if err := g.targets(c.GodotScreens); err != nil {
		return nil, err
	}
	spec, err := json.Marshal(c.GodotScreens)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	fmt.Fprintln(&out, "# Generated by ANG from CUE. DO NOT EDIT.\nextends RefCounted\nconst C = preload(\"res://generated/game_contract.gd\")")
	if c.GodotSettings != nil {
		fmt.Fprintln(&out, "const Settings=preload(\"res://generated/settings.gd\")")
	}
	fmt.Fprintf(&out, "static var _specification: Dictionary = {}\nstatic func specification() -> Dictionary:\n\tif _specification.is_empty(): _specification = JSON.parse_string(%s)\n\treturn _specification\n", strconv.Quote(string(spec)))
	fmt.Fprintln(&out, "static func theme(id: String) -> Theme:\n\tmatch id:")
	for _, id := range sortedScreenKeys(c.GodotScreens.Themes) {
		fmt.Fprintf(&out, "\t\t%q:return preload(%q)\n", id, c.GodotScreens.Themes[id])
	}
	fmt.Fprintln(&out, "\treturn null")
	if c.GodotScreens.Response != "" {
		fmt.Fprintf(&out, "static func response() -> Button:\n\treturn preload(%q).new()\n", c.GodotScreens.Response)
	} else {
		fmt.Fprintln(&out, "static func response() -> Button:\n\treturn Button.new()")
	}
	fmt.Fprintln(&out, "static func image(id: String) -> Texture2D:\n\tmatch id:")
	for _, id := range sortedScreenKeys(c.GodotScreens.Assets) {
		fmt.Fprintf(&out, "\t\t%q:return preload(%q)\n", id, c.GodotScreens.Assets[id])
	}
	fmt.Fprintln(&out, "\t\t_:return null")

	fmt.Fprintln(&out, "static func widget(id: String) -> Control:\n\tmatch id:")
	for _, id := range sortedScreenKeys(c.GodotScreens.Widgets) {
		path := c.GodotScreens.Widgets[id]
		method := "new"
		if strings.HasSuffix(path, ".tscn") {
			method = "instantiate"
		}
		fmt.Fprintf(&out, "\t\t%q: return preload(%q).%s()\n", id, path, method)
	}
	fmt.Fprintln(&out, "\t\t_: push_error(\"unregistered screen widget: \"+id)\n\treturn Control.new()")
	// Hover reads many bindings on every state. Dispatch must not scan all of
	// the unrelated screens before reaching a target's typed property getter.
	fmt.Fprintln(&out, "static var _readers: Dictionary = {")
	for _, id := range g.readers {
		fmt.Fprintf(&out, "\t%q: _read_%s,\n", id, id)
	}
	fmt.Fprintln(&out, `}
static func read(
	id: String, state: C.PocGameState, row: C.ProtoMessage,
	selected: Dictionary, request_id := ""
) -> Variant:
	var reader: Callable = _readers.get(id, Callable())
	if not reader.is_valid():
		return null
	return reader.call(state, row, selected, request_id)`)
	fmt.Fprint(&out, g.bindings.String())
	fmt.Fprintf(&out, "static func command(id: String, state: C.PocGameState, row: C.ProtoMessage, selected: Dictionary, request_id: String) -> C.ClientMessage:\n\tvar message := C.ClientMessage.new()\n\tmatch id:\n%s\t\t_: pass\n\treturn message\n", g.actions.String())
	return out.Bytes(), nil
}
func writeGodotScreens(project string, c Compiled, def *normalizer.GRPCDef) error {
	if c.GodotScreens == nil {
		return nil
	}
	if c.GodotTheme == nil {
		return fmt.Errorf("godot_screens requires godot_theme")
	}
	raw, err := compileGodotScreens(project, c, def)
	if err != nil {
		return err
	}
	outputs := map[string][]byte{"catalog.gd": raw}
	for _, name := range []string{"runtime", "routing", "world", "chrome", "elements", "navigation", "checks", "review", "capture"} {
		data, err := sources.ReadFile("templates/godot_screen_" + name + ".gd.tmpl")
		if err != nil {
			return err
		}
		outputs[name+".gd"] = data
	}
	settings, err := compileGodotSettings(project, c, def)
	if err != nil {
		return err
	}
	for _, target := range c.GodotTargets {
		if settings != nil {
			if err := os.WriteFile(filepath.Join(project, target, "generated/settings.gd"), settings, 0644); err != nil {
				return err
			}
		}
		folder := filepath.Join(project, target, "generated/screens")
		if err := os.MkdirAll(folder, 0755); err != nil {
			return err
		}
		old := map[string]string{}
		if data, err := os.ReadFile(filepath.Join(folder, "manifest.json")); err == nil {
			if err = json.Unmarshal(data, &old); err != nil {
				return err
			}
		}
		hashes := map[string]string{}
		for _, name := range sortedScreenKeys(outputs) {
			data := outputs[name]
			if err := os.WriteFile(filepath.Join(folder, name), data, 0644); err != nil {
				return err
			}
			sum := sha256.Sum256(data)
			hashes[name] = hex.EncodeToString(sum[:])
		}
		for _, name := range sortedScreenKeys(old) {
			if _, ok := outputs[name]; ok {
				continue
			}
			if filepath.Base(name) != name {
				return fmt.Errorf("invalid screen manifest name %s", name)
			}
			path := filepath.Join(folder, name)
			data, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return err
			}
			sum := sha256.Sum256(data)
			if hex.EncodeToString(sum[:]) != old[name] {
				return fmt.Errorf("obsolete screen %s was modified; refusing deletion", name)
			}
			if err := os.Remove(path); err != nil {
				return err
			}
		}
		manifest, _ := json.MarshalIndent(hashes, "", "  ")
		if err := os.WriteFile(filepath.Join(folder, "manifest.json"), append(manifest, '\n'), 0644); err != nil {
			return err
		}
	}
	return nil
}

func validateScreenChrome(p ScreenChrome) error {
	if p.HeadingGapPX < 0 || p.HeadingGapPX > 64 {
		return fmt.Errorf("heading_gap_px outside 0..64")
	}
	for field, rectangles := range map[string][][]float64{
		"ornament_rects":        p.OrnamentRects,
		"bottom_ornament_rects": p.BottomOrnamentRects,
	} {
		for _, rect := range rectangles {
			if len(rect) != 4 || rect[0] < 0 || rect[1] < 0 || rect[2] <= 0 || rect[3] <= 0 {
				return fmt.Errorf("%s: invalid rectangle", field)
			}
			if len(p.OrnamentSize) != 2 || rect[0]+rect[2] > p.OrnamentSize[0] || rect[1]+rect[3] > p.OrnamentSize[1] {
				return fmt.Errorf("%s: outside corner bounds", field)
			}
		}
	}
	for _, field := range []struct {
		name   string
		values []float64
		count  int
	}{
		{"frame_margins", p.FrameMargins, 4},
		{"padding", p.Padding, 4},
		{"ornament_size", p.OrnamentSize, 2},
	} {
		if len(field.values) != field.count {
			return fmt.Errorf("%s: expected %d coordinates", field.name, field.count)
		}
		for _, value := range field.values {
			if value < 0 {
				return fmt.Errorf("%s: negative coordinate", field.name)
			}
		}
	}
	if p.FrameMargins[0]+p.FrameMargins[2] >= 1280 ||
		p.FrameMargins[1]+p.FrameMargins[3] >= 800 {
		return fmt.Errorf("frame_margins: no content area at 1280x800")
	}
	if p.TitlePX < 12 {
		return fmt.Errorf("title_px: effective text below 12 px")
	}
	if c := p.Column; c != nil {
		if c.Side != "left" && c.Side != "right" {
			return fmt.Errorf("column.side: left or right")
		}
		// a column leaves the world in view and still holds a line of text at 1280x800
		if c.Fraction < 0.2 || c.Fraction > 0.6 || c.MinPX < 320 || c.MinPX > 1280*0.6 {
			return fmt.Errorf("column: fraction 0.2..0.6 of the screen, min_px 320..768")
		}
	}
	return nil
}

func screenRectsOverlap(a, b []float64) bool {
	return len(a) == 4 && len(b) == 4 && a[0] < b[0]+b[2] && a[0]+a[2] > b[0] &&
		a[1] < b[1]+b[3] && a[1]+a[3] > b[1]
}
