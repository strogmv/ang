package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler/emitter"
	"math"
	"path/filepath"
	"strconv"
	"strings"
)

type GodotSettings struct {
	Entries  []GodotSetting            `json:"entries"`
	Appliers map[string]SettingApplier `json:"appliers"`
	Profiles map[string]any            `json:"profiles"`
	// Foreign: sections of the settings file another table owns (the input actions' player keys, controls v1): read and
	// written by it, never warned about, kept as they are when the settings are saved.
	Foreign []string `json:"foreign,omitempty"`
}
type SettingApplier struct {
	Script string `json:"script"`
	Method string `json:"method"`
}
type GodotSetting struct {
	ID           string               `json:"id"`
	Section      string               `json:"section"`
	Key          string               `json:"key"`
	Kind         string               `json:"kind"`
	Options      []SettingOption      `json:"options,omitempty"`
	Min          float64              `json:"min"`
	Max          float64              `json:"max"`
	Step         float64              `json:"step"`
	Default      any                  `json:"default"`
	Label        string               `json:"label_key"`
	ValueKey     string               `json:"value_key,omitempty"`
	DisplayScale float64              `json:"display_scale"`
	Applier      string               `json:"applier"`
	Params       map[string]any       `json:"params"`
	Notification *SettingNotification `json:"notification,omitempty"`
}
type SettingOption struct {
	Value any    `json:"value"`
	Key   string `json:"text_key"`
}
type SettingNotification struct {
	Message string `json:"message"`
	Field   string `json:"field"`
}

func settingFieldName(id string) string {
	field := emitter.GDScriptFieldName(id)
	if strings.Contains(" file warnings changed value definition valid load_file save_file set_value cycle display make_notification apply snapshot _data _assign _warn ", " "+field+" ") || strings.HasPrefix(field, "setting_") {
		return "setting_" + field
	}
	return field
}
func settingType(s GodotSetting) string {
	switch s.Kind {
	case "toggle":
		return "bool"
	case "choice":
		return "String"
	default:
		return "float"
	}
}
func settingNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint64:
		return float64(n), true
	}
	return 0, false
}
func settingValid(s GodotSetting, v any) bool {
	if s.Kind == "toggle" {
		_, ok := v.(bool)
		return ok
	}
	if s.Kind == "choice" {
		text, ok := v.(string)
		if !ok {
			return false
		}
		for _, o := range s.Options {
			if option, ok := o.Value.(string); ok && option == text {
				return true
			}
		}
		return false
	}
	n, ok := settingNumber(v)
	return ok && !math.IsNaN(n) && !math.IsInf(n, 0) && n >= s.Min && n <= s.Max && s.Step > 0 && math.Abs((n-s.Min)/s.Step-math.Round((n-s.Min)/s.Step)) < 1e-6
}
func compileGodotSettings(project string, c Compiled, d *normalizer.GRPCDef) ([]byte, error) {
	if c.GodotSettings == nil {
		return nil, nil
	}
	settings := c.GodotSettings
	ids, storage := map[string]bool{}, map[string]bool{}
	root := filepath.Join(project, c.GodotTheme.Target)
	fieldError := func(s GodotSetting, field, reason string) error {
		return fmt.Errorf("setting %s field %s: %s", s.ID, field, reason)
	}
	for _, s := range settings.Entries {
		if !godotResourceID.MatchString(s.ID) || (len(s.ID) > 0 && s.ID[0] >= '0' && s.ID[0] <= '9') || ids[s.ID] {
			return nil, fieldError(s, "id", "invalid or duplicate")
		}
		ids[s.ID] = true
		if s.Section == "" || s.Key == "" || storage[s.Section+"/"+s.Key] {
			return nil, fieldError(s, "key", "empty or duplicate storage key")
		}
		storage[s.Section+"/"+s.Key] = true
		if s.Kind != "toggle" && s.Kind != "choice" && s.Kind != "stepped_number" {
			return nil, fieldError(s, "kind", "unknown setting kind")
		}
		if s.Kind == "stepped_number" && (s.Step <= 0 || s.Max < s.Min || (s.Max-s.Min)/s.Step > 1000) {
			return nil, fieldError(s, "step", "invalid or excessive range")
		}
		if !settingValid(s, s.Default) {
			return nil, fieldError(s, "default", "outside options or range")
		}
		a, ok := settings.Appliers[s.Applier]
		if !ok || !strings.HasPrefix(a.Script, "res://") || strings.Contains(a.Script, "..") || !strings.HasSuffix(a.Script, ".gd") || a.Method == "" {
			return nil, fieldError(s, "applier", "unknown registered applier")
		}
		if err := screenText(c, s.Label, 19, 700, 2, root); err != nil {
			return nil, fieldError(s, "label_key", err.Error())
		}
		keys := []string{}
		for _, o := range s.Options {
			keys = append(keys, o.Key)
		}
		if s.ValueKey != "" {
			keys = append(keys, s.ValueKey)
		}
		if len(keys) == 0 {
			return nil, fieldError(s, "value_key", "missing value text")
		}
		for _, key := range keys {
			copyC := c
			copyC.Localization.Messages = append([]LocalizationMessage(nil), c.Localization.Messages...)
			for i, m := range copyC.Localization.Messages {
				if m.Key != key {
					continue
				}
				m.Texts = map[string]string{}
				for locale, text := range c.Localization.Messages[i].Texts {
					scale := s.DisplayScale
					if scale == 0 {
						scale = 1
					}
					number := strconv.FormatFloat(s.Max*scale, 'f', 2, 64)
					m.Texts[locale] = strings.ReplaceAll(text, "{value}", number)
				}
				copyC.Localization.Messages[i] = m
			}
			if err := screenText(copyC, key, 19, 700, 2, root); err != nil {
				return nil, fieldError(s, "value_key", err.Error())
			}
		}
		for _, key := range keys {
			copyC := c
			copyC.Localization.Messages = append([]LocalizationMessage(nil), c.Localization.Messages...)
			texts := map[string]string{}
			for _, locale := range c.Localization.Locales {
				label, value := "", ""
				for _, m := range c.Localization.Messages {
					if m.Key == s.Label {
						label = m.Texts[locale]
					}
					if m.Key == key {
						value = m.Texts[locale]
					}
				}
				scale := s.DisplayScale
				if scale == 0 {
					scale = 1
				}
				number := strconv.FormatFloat(s.Max*scale, 'f', 2, 64)
				texts[locale] = label + " · " + strings.ReplaceAll(value, "{value}", number)
			}
			copyC.Localization.Messages = append(copyC.Localization.Messages, LocalizationMessage{Key: "ui.setting.combined", Texts: texts})
			if err := screenText(copyC, "ui.setting.combined", 19, 1028, 1, root); err != nil {
				return nil, fieldError(s, "value_key", err.Error())
			}
		}
		if n := s.Notification; n != nil {
			var message *normalizer.GRPCMessage
			var command string
			for i, m := range d.Messages {
				if m.Name == n.Message {
					message = &d.Messages[i]
				}
				if m.Name == "ClientMessage" {
					for _, one := range m.Oneofs {
						for _, f := range one.Members {
							if f.Type == n.Message {
								command = f.Name
							}
						}
					}
				}
			}
			if message == nil || command == "" {
				return nil, fieldError(s, "notification.message", "unknown contract message")
			}
			found := false
			for _, f := range message.Fields {
				if f.Name == n.Field {
					found = true
					if !screenLiteral(s.Default, f.Type) {
						return nil, fieldError(s, "notification.field", "incompatible field type")
					}
				}
			}
			if !found {
				return nil, fieldError(s, "notification.field", "unknown contract field")
			}
		}
	}
	var out bytes.Buffer
	fmt.Fprintln(&out, "# Generated by ANG from CUE. DO NOT EDIT.\nextends RefCounted\nconst C=preload(\"res://generated/game_contract.gd\")\nsignal changed(id: String, value: Variant)\nvar file:=ConfigFile.new()\nvar warnings: Array[String]=[]")
	described := *settings
	if c.InputActions != nil {
		for _, e := range settings.Entries {
			if e.Section == c.InputActions.Section {
				return nil, fmt.Errorf("setting %s field section: %s is the input actions' section", e.ID, e.Section)
			}
		}
		described.Foreign = []string{c.InputActions.Section}
	}
	raw, _ := json.Marshal(described)
	fmt.Fprintf(&out, "static var _data: Dictionary={}\nstatic func specification() -> Dictionary:\n\tif _data.is_empty():_data=JSON.parse_string(%s)\n\treturn _data\n", strconv.Quote(string(raw)))
	for _, s := range settings.Entries {
		v, _ := json.Marshal(s.Default)
		fmt.Fprintf(&out, "var %s: %s = %s\n", settingFieldName(s.ID), settingType(s), v)
	}
	fmt.Fprintln(&out, "func value(id: String) -> Variant:\n\tmatch id:")
	for _, s := range settings.Entries {
		fmt.Fprintf(&out, "\t\t%q:return %s\n", s.ID, settingFieldName(s.ID))
	}
	fmt.Fprintln(&out, "\treturn null\nfunc _assign(id: String,v: Variant) -> void:\n\tmatch id:")
	for _, s := range settings.Entries {
		fmt.Fprintf(&out, "\t\t%q:%s=%s(v)\n", s.ID, settingFieldName(s.ID), settingType(s))
	}
	fmt.Fprintln(&out, "\t\t_:pass")
	runtime, err := sources.ReadFile("templates/godot_settings_runtime.gd.tmpl")
	if err != nil {
		return nil, err
	}
	out.Write(runtime)
	fmt.Fprintln(&out, "\nfunc make_notification(id: String) -> C.ClientMessage:\n\tvar result:=C.ClientMessage.new()\n\tmatch id:")
	for _, s := range settings.Entries {
		if s.Notification == nil {
			continue
		}
		n := s.Notification
		command := ""
		for _, m := range d.Messages {
			if m.Name == "ClientMessage" {
				for _, one := range m.Oneofs {
					for _, f := range one.Members {
						if f.Type == n.Message {
							command = f.Name
						}
					}
				}
			}
		}
		fmt.Fprintf(&out, "\t\t%q:\n\t\t\tvar q:=C.%s.new()\n\t\t\tq.%s=%s\n\t\t\tresult.%s=q\n", s.ID, n.Message, emitter.GDScriptFieldName(n.Field), settingFieldName(s.ID), emitter.GDScriptFieldName(command))
	}
	fmt.Fprintln(&out, "\t\t_:return null\n\treturn result")
	fmt.Fprintln(&out, "\nfunc apply(id: String,context: Object) -> void:\n\tvar entry:=definition(id)\n\tmatch str(entry.applier):")
	for _, id := range sortedScreenKeys(settings.Appliers) {
		a := settings.Appliers[id]
		fmt.Fprintf(&out, "\t\t%q:preload(%q).%s(context,value(id),entry.get(\"params\",{}),specification().profiles)\n", id, a.Script, a.Method)
	}
	fmt.Fprintln(&out, "\t\t_:push_error(\"unknown applier: \"+str(entry.applier))")
	return out.Bytes(), nil
}
