package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"cuelang.org/go/cue"
)

// InputActions is world.input_actions (controls v1, step 1: the game's design/controls/v1/BRIEF.md): every input action
// the player can press, in one table. ANG checks it and writes client-godot/generated/input_actions.gd, which registers
// the actions with Godot's InputMap (the defaults, the player's keys from the settings file's Section on top); the
// client, the generated screens and the hints name actions, never keys. Presentation only: never in the server's world.
type InputActions struct {
	Section   string                   `json:"section"`
	Pads      map[string]InputPad      `json:"pads"`
	KeyLabels map[string]InputKeyLabel `json:"key_labels"`
	// Screens: a screen's input context by its id (a screen not listed is "menu").
	Screens map[string]string `json:"screens"`
	// Precedence: in a context, on one device, these actions win an input they share with another action of it.
	Precedence []InputPrecedence `json:"precedence"`
	Actions    []InputAction     `json:"actions"`
	// Groups: the groups in the order the controls screen of the settings lists them (step 2), each with its heading
	// (a ui.controls.* text); Listed false: not on that screen (a developer's keys). Every action's group is here once.
	Groups []InputGroup `json:"groups,omitempty"`
	// Summary: the settings card's line about the player's keys: Unchanged when none differs from the defaults,
	// Changed («{n}»: how many actions differ) otherwise.
	Summary *InputSummary `json:"summary,omitempty"`
	// Hinted: the public texts that name a key as {key:<id>} or a button as {pad:<id>} (filled by the check); the client
	// puts the current binding into exactly these.
	Hinted []string `json:"hinted,omitempty"`
}

// InputGroup is one group of actions on the controls screen: its heading, and whether the screen lists it.
type InputGroup struct {
	ID     string        `json:"id"`
	Label  InputKeyLabel `json:"label"`
	Listed bool          `json:"listed"`
}

// InputSummary: the settings card's line about the player's keys (public texts of the catalog).
type InputSummary struct {
	Unchanged InputKeyLabel `json:"unchanged"`
	Changed   InputKeyLabel `json:"changed"`
}

// InputPad is one gamepad button: its Godot JoyButton index and the label a hint shows.
type InputPad struct {
	Button int    `json:"button"`
	Label  string `json:"label"`
}

// InputKeyLabel names a key whose label is not its Godot name (Space → «Пробел»); a public text of the catalog.
// Platforms: the label on one OS (OS.get_name()) in place of the text (Meta: Cmd on macOS, Win on Windows).
type InputKeyLabel struct {
	Key       string            `json:"key"`
	Context   string            `json:"context"`
	RU        string            `json:"ru"`
	EN        string            `json:"en"`
	Platforms map[string]string `json:"platforms,omitempty"`
}

// InputPrecedence: in Context, on Device ("key" or "pad"), the actions Wins take an input they share with another
// action of the context; the other's binding is idle there (its reader asks with the context and steps aside).
type InputPrecedence struct {
	Context string   `json:"context"`
	Device  string   `json:"device"`
	Wins    []string `json:"wins"`
	Reason  string   `json:"reason"`
}

// InputAction: Keys are Godot key names of the US layout (matched by the key's place, physical), modifiers first with
// «+»; Pad are names of Pads; the first of each is the one a hint shows. Contexts say where the action is read: two
// actions on one key conflict only when they share one. Remap false: fixed (Esc, Enter, the arrows, a developer's keys).
type InputAction struct {
	ID       string        `json:"id"`
	Group    string        `json:"group"`
	Contexts []string      `json:"contexts"`
	Keys     []string      `json:"keys"`
	Pad      []string      `json:"pad"`
	Remap    bool          `json:"remap"`
	Hold     bool          `json:"hold"`
	Label    InputKeyLabel `json:"label"`
}

var inputActionID = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// inputHintToken is how a public text names a binding: {key:<action>} or {pad:<action>}.
var inputHintToken = regexp.MustCompile(`\{(key|pad):([a-z0-9_]+)\}`)

// inputContexts: where the code reads an action; "global" is read everywhere and meets every other.
var inputContexts = map[string]bool{"world": true, "fight": true, "menu": true, "inventory": true, "inventory_column": true,
	"trade": true, "saves": true, "hero_file": true, "conversation": true, "global": true}

var inputGroups = map[string]bool{"move": true, "world": true, "screens": true, "system": true, "choices": true,
	"combat": true, "menu": true, "inventory": true, "trade": true, "saves": true, "hero_file": true, "developer": true}

// inputModifiers in the one order a key name writes them.
var inputModifiers = []string{"Shift", "Ctrl", "Alt", "Meta"}

// inputKeyNames: the Godot names (OS.get_keycode_string, US layout) a binding may use; the modifiers are written as
// themselves on every platform (Godot calls Alt «Option» and Meta «Command» on a Mac).
var inputKeyNames = func() map[string]bool {
	names := map[string]bool{}
	for _, n := range strings.Fields("Escape Tab Backspace Enter Insert Delete Pause Home End Left Up Right Down PageUp PageDown " +
		"Shift Ctrl Alt Meta CapsLock Space Apostrophe Comma Minus Period Slash Semicolon Equal BracketLeft BackSlash " +
		"BracketRight QuoteLeft Section") {
		names[n] = true
	}
	for _, n := range []string{"Kp Enter", "Kp Multiply", "Kp Divide", "Kp Subtract", "Kp Period", "Kp Add"} {
		names[n] = true
	}
	for c := 'A'; c <= 'Z'; c++ {
		names[string(c)] = true
	}
	for d := 0; d <= 9; d++ {
		names[strconv.Itoa(d)] = true
		names["Kp "+strconv.Itoa(d)] = true
	}
	for f := 1; f <= 12; f++ {
		names["F"+strconv.Itoa(f)] = true
	}
	return names
}()

// inputKeyValid: a known base key, its modifiers known, each once, in the one order, never a modifier on itself.
func inputKeyValid(name string) bool {
	parts := strings.Split(name, "+")
	base := parts[len(parts)-1]
	if !inputKeyNames[base] {
		return false
	}
	at := -1
	for _, m := range parts[:len(parts)-1] {
		index := -1
		for i, known := range inputModifiers {
			if m == known {
				index = i
			}
		}
		if index <= at || m == base {
			return false
		}
		at = index
	}
	return true
}

func readInputActions(world cue.Value) (*InputActions, error) {
	v := world.LookupPath(cue.ParsePath("input_actions"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.input_actions: %w", err)
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var out InputActions
	if err := d.Decode(&out); err != nil {
		return nil, fmt.Errorf("world.input_actions: %w", err)
	}
	if err := out.validate(); err != nil {
		return nil, fmt.Errorf("world.input_actions: %w", err)
	}
	return &out, nil
}

// action finds an action by id; a world without the table has none.
func (a *InputActions) action(id string) (InputAction, bool) {
	if a == nil {
		return InputAction{}, false
	}
	for _, one := range a.Actions {
		if one.ID == id {
			return one, true
		}
	}
	return InputAction{}, false
}

// validate checks the table alone: ids, groups, contexts, key and button names, and that no two defaults meet on one
// key or button in a context (the player's own keys are checked by the client: conflicts()).
func (a *InputActions) validate() error {
	if !inputActionID.MatchString(a.Section) {
		return fmt.Errorf("section %q: a settings file section name", a.Section)
	}
	if len(a.Actions) == 0 {
		return fmt.Errorf("actions: none")
	}
	buttons := map[int]string{}
	for _, name := range sortedScreenKeys(a.Pads) {
		pad := a.Pads[name]
		if !godotResourceID.MatchString(name) || pad.Button < 0 || pad.Button > 20 || pad.Label == "" || buttons[pad.Button] != "" {
			return fmt.Errorf("pads %s: a name, a JoyButton index 0..20 of its own and a label", name)
		}
		buttons[pad.Button] = name
	}
	for _, name := range sortedScreenKeys(a.KeyLabels) {
		if !inputKeyNames[name] || !strings.HasPrefix(a.KeyLabels[name].Key, "ui.key.") {
			return fmt.Errorf("key_labels %s: a Godot key name and a ui.key.* text", name)
		}
	}
	seen := map[string]bool{}
	for _, one := range a.Actions {
		field := "actions." + one.ID
		if !inputActionID.MatchString(one.ID) || seen[one.ID] {
			return fmt.Errorf("%s.id: invalid or duplicate", field)
		}
		seen[one.ID] = true
		if !inputGroups[one.Group] {
			return fmt.Errorf("%s.group: unknown group %q", field, one.Group)
		}
		if len(one.Contexts) == 0 {
			return fmt.Errorf("%s.contexts: none", field)
		}
		contexts := map[string]bool{}
		for _, c := range one.Contexts {
			if !inputContexts[c] || contexts[c] {
				return fmt.Errorf("%s.contexts: unknown or repeated context %q", field, c)
			}
			contexts[c] = true
		}
		if len(one.Keys) == 0 && len(one.Pad) == 0 {
			return fmt.Errorf("%s: neither a key nor a button", field)
		}
		if !strings.HasPrefix(one.Label.Key, "ui.controls.") || one.Label.RU == "" || one.Label.EN == "" || one.Label.Context == "" {
			return fmt.Errorf("%s.label: a ui.controls.* text with ru, en and context", field)
		}
		own := map[string]bool{}
		for _, key := range one.Keys {
			if !inputKeyValid(key) {
				return fmt.Errorf("%s.keys: unknown key %q (Godot names, modifiers Shift+Ctrl+Alt+Meta+ first)", field, key)
			}
			if own["key:"+key] {
				return fmt.Errorf("%s: key:%s twice", field, key)
			}
			own["key:"+key] = true
		}
		for _, pad := range one.Pad {
			if _, ok := a.Pads[pad]; !ok {
				return fmt.Errorf("%s.pad: unknown button %q", field, pad)
			}
			if own["pad:"+pad] {
				return fmt.Errorf("%s: pad:%s twice", field, pad)
			}
			own["pad:"+pad] = true
		}
	}
	if len(a.Groups) > 0 {
		listed := map[string]bool{}
		for i, g := range a.Groups {
			field := fmt.Sprintf("groups.%d", i)
			if !inputGroups[g.ID] || listed[g.ID] {
				return fmt.Errorf("%s.id: unknown or repeated group %q", field, g.ID)
			}
			listed[g.ID] = true
			if !strings.HasPrefix(g.Label.Key, "ui.controls.") || g.Label.RU == "" || g.Label.EN == "" || g.Label.Context == "" {
				return fmt.Errorf("%s.label: a ui.controls.* text with ru, en and context", field)
			}
		}
		for _, one := range a.Actions {
			if !listed[one.Group] {
				return fmt.Errorf("actions.%s.group: %q is not in groups", one.ID, one.Group)
			}
		}
	}
	if m := a.Summary; m != nil {
		for field, l := range map[string]InputKeyLabel{"unchanged": m.Unchanged, "changed": m.Changed} {
			if !strings.HasPrefix(l.Key, "ui.controls.") || l.RU == "" || l.EN == "" || l.Context == "" {
				return fmt.Errorf("summary.%s: a ui.controls.* text with ru, en and context", field)
			}
		}
		if !strings.Contains(m.Changed.RU, "{n}") || !strings.Contains(m.Changed.EN, "{n}") {
			return fmt.Errorf("summary.changed: the count {n} in ru and en")
		}
	}
	for i, rule := range a.Precedence {
		field := fmt.Sprintf("precedence.%d", i)
		if !inputContexts[rule.Context] || (rule.Device != "key" && rule.Device != "pad") || rule.Reason == "" || len(rule.Wins) == 0 {
			return fmt.Errorf("%s: a context, the device key or pad, the winners and the reason", field)
		}
		for _, id := range rule.Wins {
			one, ok := a.action(id)
			if !ok || !inputHasContext(one, rule.Context) {
				return fmt.Errorf("%s.wins: %q is no action of the context %s", field, id, rule.Context)
			}
		}
	}
	for _, id := range sortedScreenKeys(a.Screens) {
		if !inputContexts[a.Screens[id]] || a.Screens[id] == "global" {
			return fmt.Errorf("screens.%s: unknown context %q", id, a.Screens[id])
		}
	}
	if found := a.conflicts(); len(found) > 0 {
		return fmt.Errorf("defaults conflict: %s", strings.Join(found, "; "))
	}
	return nil
}

func inputHasContext(one InputAction, context string) bool {
	for _, c := range one.Contexts {
		if c == context {
			return true
		}
	}
	return false
}

// freeModifiers: the modifiers bound alone to a held action, by the contexts of that action (Shift: the run's world).
func (a *InputActions) freeModifiers() map[string]map[string]bool {
	free := map[string]map[string]bool{}
	for _, one := range a.Actions {
		if !one.Hold {
			continue
		}
		for _, key := range one.Keys {
			for _, m := range inputModifiers {
				if key != m {
					continue
				}
				for _, c := range one.Contexts {
					if free[c] == nil {
						free[c] = map[string]bool{}
					}
					free[c][m] = true
				}
			}
		}
	}
	return free
}

// effectiveKeys: a key binding and the combinations it takes with the free modifiers of the action's contexts held
// (W of the walk: W, Shift+W, Alt+W, Shift+Alt+W), in the one order of the names.
func (a *InputActions) effectiveKeys(one InputAction, key string, free map[string]map[string]bool) []string {
	parts := strings.Split(key, "+")
	base, held := parts[len(parts)-1], map[string]bool{}
	for _, m := range parts[:len(parts)-1] {
		held[m] = true
	}
	extra := []string{}
	for _, m := range inputModifiers {
		tolerated := false
		for _, c := range one.Contexts {
			tolerated = tolerated || free[c][m]
		}
		if tolerated && !held[m] && m != base {
			extra = append(extra, m)
		}
	}
	out := []string{}
	for mask := 0; mask < 1<<len(extra); mask++ {
		mods := map[string]bool{}
		for m := range held {
			mods[m] = true
		}
		for i, m := range extra {
			if mask&(1<<i) != 0 {
				mods[m] = true
			}
		}
		name := ""
		for _, m := range inputModifiers {
			if mods[m] {
				name += m + "+"
			}
		}
		out = append(out, name+base)
	}
	return out
}

// conflicts: every key or button two actions take in one context (a free modifier's combinations counted), unless a
// precedence rule of that context and device names exactly one of them; a global action meets every context.
func (a *InputActions) conflicts() []string {
	free := a.freeModifiers()
	holders := map[string][]string{}
	asIs := map[string]bool{}
	slots := []string{}
	add := func(one InputAction, input string, held bool) {
		contexts := one.Contexts
		if inputHasContext(one, "global") {
			contexts = sortedScreenKeys(inputContexts)
		}
		for _, c := range contexts {
			slot := c + " " + input
			taken := false
			for _, other := range holders[slot] {
				taken = taken || other == one.ID
			}
			if taken {
				continue
			}
			if holders[slot] == nil {
				slots = append(slots, slot)
			}
			holders[slot] = append(holders[slot], one.ID)
			asIs[slot] = asIs[slot] || held
		}
	}
	for _, one := range a.Actions {
		for _, key := range one.Keys {
			for _, input := range a.effectiveKeys(one, key, free) {
				add(one, "key:"+input, input == key)
			}
		}
		for _, pad := range one.Pad {
			add(one, "pad:"+pad, true)
		}
	}
	out := []string{}
	for _, slot := range slots {
		ids := holders[slot]
		// reported on a binding one of them holds as it is: the combinations both take only by a free modifier
		// repeat their own keys' conflict
		if len(ids) < 2 || !asIs[slot] {
			continue
		}
		context, input, _ := strings.Cut(slot, " ")
		device, _, _ := strings.Cut(input, ":")
		winners := 0
		for _, rule := range a.Precedence {
			if rule.Context != context || rule.Device != device {
				continue
			}
			for _, id := range ids {
				for _, w := range rule.Wins {
					if w == id {
						winners++
					}
				}
			}
		}
		if winners != 1 {
			out = append(out, fmt.Sprintf("%s in the context %s: %s", input, context, strings.Join(ids, ", ")))
		}
	}
	return out
}

// inputLabel: the label a hint shows for a key name by default, in a locale (Ctrl+Enter, «Пробел», ←).
func (a *InputActions) inputLabel(name, locale string) string {
	parts := strings.Split(name, "+")
	for i, part := range parts {
		if l, ok := a.KeyLabels[part]; ok {
			if locale == "en" {
				parts[i] = l.EN
			} else {
				parts[i] = l.RU
			}
		}
	}
	return strings.Join(parts, "+")
}

// expandInputHints puts the default binding into a public text, as the client does with the current one (the fit
// checks measure this).
func expandInputHints(c Compiled, text, locale string) string {
	a := c.InputActions
	if a == nil || !strings.Contains(text, "{") {
		return text
	}
	return inputHintToken.ReplaceAllStringFunc(text, func(token string) string {
		m := inputHintToken.FindStringSubmatch(token)
		one, ok := a.action(m[2])
		if !ok {
			return token
		}
		if m[1] == "key" && len(one.Keys) > 0 {
			return a.inputLabel(one.Keys[0], locale)
		}
		if m[1] == "pad" && len(one.Pad) > 0 {
			return a.Pads[one.Pad[0]].Label
		}
		return token
	})
}

// checkInputActions checks the table against the rest of the world: its texts are in the catalog, every hint names a
// real action with that kind of binding, and everything that names an action (screens' shortcuts and hotkeys, the
// world's peaceful and hostile action, the combat pace, the developer's toggle) names one in the table.
func checkInputActions(c *Compiled) error {
	a := c.InputActions
	texts := map[string]map[string]string{}
	for _, m := range c.Localization.Messages {
		texts[m.Key] = m.Texts
	}
	if a == nil {
		for _, m := range c.Localization.Messages {
			for _, text := range m.Texts {
				if inputHintToken.MatchString(text) {
					return fmt.Errorf("localization %s: a key hint with no world.input_actions", m.Key)
				}
			}
		}
		return nil
	}
	for _, one := range a.Actions {
		for _, locale := range c.Localization.Locales {
			if texts[one.Label.Key][locale] == "" {
				return fmt.Errorf("world.input_actions actions.%s.label: no %s text %s", one.ID, locale, one.Label.Key)
			}
		}
	}
	for _, name := range sortedScreenKeys(a.KeyLabels) {
		for _, locale := range c.Localization.Locales {
			if texts[a.KeyLabels[name].Key][locale] == "" {
				return fmt.Errorf("world.input_actions key_labels.%s: no %s text %s", name, locale, a.KeyLabels[name].Key)
			}
		}
	}
	more := map[string]string{}
	for _, g := range a.Groups {
		more["groups."+g.ID] = g.Label.Key
	}
	if a.Summary != nil {
		more["summary.unchanged"], more["summary.changed"] = a.Summary.Unchanged.Key, a.Summary.Changed.Key
	}
	for _, field := range sortedScreenKeys(more) {
		for _, locale := range c.Localization.Locales {
			if texts[more[field]][locale] == "" {
				return fmt.Errorf("world.input_actions %s: no %s text %s", field, locale, more[field])
			}
		}
	}
	a.Hinted = nil
	for _, m := range c.Localization.Messages {
		hinted := false
		for _, locale := range sortedScreenKeys(m.Texts) {
			for _, token := range inputHintToken.FindAllStringSubmatch(m.Texts[locale], -1) {
				one, ok := a.action(token[2])
				if !ok {
					return fmt.Errorf("localization %s %s: {%s:%s} names no input action", m.Key, locale, token[1], token[2])
				}
				if (token[1] == "key" && len(one.Keys) == 0) || (token[1] == "pad" && len(one.Pad) == 0) {
					return fmt.Errorf("localization %s %s: input action %s has no default %s", m.Key, locale, one.ID, token[1])
				}
				hinted = true
			}
		}
		if hinted {
			if !strings.HasPrefix(m.Key, "ui.") {
				return fmt.Errorf("localization %s: a key hint in a text the client never receives (only ui.* texts)", m.Key)
			}
			a.Hinted = append(a.Hinted, m.Key)
		}
	}
	sort.Strings(a.Hinted)
	known := func(field, id string) error {
		if _, ok := a.action(id); !ok {
			return fmt.Errorf("%s: unknown input action %q", field, id)
		}
		return nil
	}
	if s := c.GodotScreens; s != nil {
		ids := map[string]bool{}
		for _, screen := range s.Screens {
			ids[screen.ID] = true
		}
		for _, id := range sortedScreenKeys(a.Screens) {
			if !ids[id] {
				return fmt.Errorf("world.input_actions screens.%s: no such screen", id)
			}
		}
		for _, screen := range s.Screens {
			if screen.Shortcut != nil {
				if err := known(fmt.Sprintf("screen %s field shortcut.action", screen.ID), screen.Shortcut.Action); err != nil {
					return err
				}
			}
			for i, h := range screen.Hotkeys {
				if err := known(fmt.Sprintf("screen %s field hotkeys.%d.action", screen.ID, i), h.Action); err != nil {
					return err
				}
			}
		}
		if w := s.WorldInput; w != nil {
			for field, id := range map[string]string{"peaceful": w.Peaceful.Action, "hostile": w.Hostile.Action} {
				if err := known("screen world field world_input."+field+".action", id); err != nil {
					return err
				}
			}
		}
	}
	if p := c.CombatPace; p != nil {
		if err := known("world.combat_pace.action", p.Action); err != nil {
			return err
		}
	}
	if c.dialogueDev.ToggleAction != "" {
		if err := known("developerInput.toggle_action", c.dialogueDev.ToggleAction); err != nil {
			return err
		}
	}
	return nil
}

// compileGodotInput writes generated/input_actions.gd: the table as data, a constant per action id (a renamed or
// removed action fails the client's parse, not its play) and the shared runtime (templates/godot_input_runtime.gd.tmpl).
func compileGodotInput(c Compiled) ([]byte, error) {
	a := c.InputActions
	if a == nil {
		return nil, nil
	}
	raw, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	fmt.Fprintln(&out, "# Generated by ANG from CUE (world.input_actions). DO NOT EDIT.\nextends RefCounted\n## The game's input actions (controls v1): registered with the InputMap, the player's keys from the settings file on top.")
	fmt.Fprintf(&out, "const SECTION := %q\n", a.Section)
	for _, one := range a.Actions {
		fmt.Fprintf(&out, "const %s := &%q\n", strings.ToUpper(one.ID), one.ID)
	}
	fmt.Fprintf(&out, "static var _data: Dictionary = {}\nstatic func specification() -> Dictionary:\n\tif _data.is_empty(): _data = JSON.parse_string(%s)\n\treturn _data\n", strconv.Quote(string(raw)))
	runtime, err := sources.ReadFile("templates/godot_input_runtime.gd.tmpl")
	if err != nil {
		return nil, err
	}
	out.Write(runtime)
	return out.Bytes(), nil
}
