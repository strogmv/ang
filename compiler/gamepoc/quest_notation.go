package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/strogmv/ang/compiler/questnotation"
)

var questNamePattern = regexp.MustCompile(` «[^»\n]*»`)

func questObject(input any) (map[string]any, error) {
	data, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var obj map[string]any
	err = questJSON(data, &obj)
	return obj, err
}

func questJSON(data []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	return decoder.Decode(value)
}

func (q *questExport) formatter() questnotation.Formatter {
	return func(id string) string { return q.glossary[id].Name }
}

func (q *questExport) phrase(v any) string               { return q.formatter().Phrase(v) }
func (q *questExport) fields(v map[string]any) string    { return q.formatter().Fields(v) }
func (q *questExport) condition(v map[string]any) string { return q.formatter().Condition(v) }

// splitQuest separates only top-level operators, preserving quoted dialogue, lists and groups.
func splitQuest(s, sep string) ([]string, error) {
	var parts []string
	var groups []rune
	quote := rune(0)
	escaped := false
	start := 0
	for i, ch := range s {
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' && quote == '"' {
				escaped = true
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		if len(groups) == 0 && i >= start && strings.HasPrefix(s[i:], sep) {
			parts = append(parts, strings.TrimSpace(s[start:i]))
			start = i + len(sep)
			continue
		}
		switch ch {
		case '"':
			quote = '"'
		case '«':
			quote = '»'
		case '(':
			groups = append(groups, ')')
		case '⟨':
			groups = append(groups, '⟩')
		case ')', '⟩':
			if len(groups) == 0 || groups[len(groups)-1] != ch {
				return nil, fmt.Errorf("лишняя скобка в %q", s)
			}
			groups = groups[:len(groups)-1]
		}
	}
	if len(groups) != 0 || quote != 0 {
		return nil, fmt.Errorf("незакрытая скобка или кавычка в %q", s)
	}
	return append(parts, strings.TrimSpace(s[start:])), nil
}

func parseQuestFields(s, sep string) (map[string]any, error) {
	obj := map[string]any{}
	if s == "" {
		return obj, nil
	}
	parts, err := splitQuest(s, sep)
	if err != nil {
		return nil, err
	}
	for _, part := range parts {
		label, raw, ok := strings.Cut(part, " = ")
		if !ok || label == "" {
			return nil, fmt.Errorf("ожидалось «поле = значение», получено %q", part)
		}
		key := label
		for field, caption := range questnotation.Labels() {
			if label == caption {
				key = field
				break
			}
		}
		if _, exists := obj[key]; exists {
			return nil, fmt.Errorf("повтор поля %q", label)
		}
		val, err := parseQuestValue(raw)
		if err != nil {
			return nil, err
		}
		obj[key] = val
	}
	return obj, nil
}

func parseQuestValue(s string) (any, error) {
	s = strings.TrimSpace(s)
	switch {
	case strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")"):
		return parseQuestFields(s[1:len(s)-1], " · ")
	case strings.HasPrefix(s, "⟨") && strings.HasSuffix(s, "⟩"):
		list := []any{}
		body := strings.TrimSuffix(strings.TrimPrefix(s, "⟨"), "⟩")
		if body == "" {
			return list, nil
		}
		parts, err := splitQuest(body, " · ")
		if err != nil {
			return nil, err
		}
		for _, part := range parts {
			v, err := parseQuestValue(part)
			if err != nil {
				return nil, err
			}
			list = append(list, v)
		}
		return list, nil
	case strings.HasPrefix(s, `"`):
		return strconv.Unquote(s)
	case questnotation.IsNumber(s):
		return json.Number(strings.ReplaceAll(s, ",", ".")), nil
	case s == "да":
		return true, nil
	case s == "нет":
		return false, nil
	case s == "пусто":
		return nil, nil
	}
	id := questNamePattern.ReplaceAllString(s, "")
	if questnotation.IsBare(id) && id != "-" {
		return id, nil
	}
	return nil, fmt.Errorf("непонятное значение %q", s)
}

func parseQuestCondition(s string) (map[string]any, error) {
	s = strings.TrimSpace(s)
	parts, err := splitQuest(s, " И ")
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	for _, part := range parts {
		var values map[string]any
		switch {
		case strings.HasPrefix(part, "НЕ (") && strings.HasSuffix(part, ")"):
			sub, err := parseQuestCondition(strings.TrimPrefix(part, "НЕ (")[:len(part)-len("НЕ (")-1])
			if err != nil {
				return nil, err
			}
			values = map[string]any{"not": sub}
		case strings.HasPrefix(part, "(") && strings.HasSuffix(part, ")"):
			alts, err := splitQuest(part[1:len(part)-1], " ИЛИ ")
			if err != nil {
				return nil, err
			}
			list := []any{}
			for _, alt := range alts {
				if !strings.HasPrefix(alt, "(") || !strings.HasSuffix(alt, ")") {
					return nil, fmt.Errorf("нужны скобки вокруг %q", alt)
				}
				sub, err := parseQuestCondition(alt[1 : len(alt)-1])
				if err != nil {
					return nil, err
				}
				list = append(list, sub)
			}
			values = map[string]any{"any": list}
		default:
			values, err = parseQuestFields(part, " · ")
			if err != nil {
				return nil, err
			}
		}
		for key, val := range values {
			if _, exists := out[key]; exists {
				return nil, fmt.Errorf("повтор условия %q", key)
			}
			out[key] = val
		}
	}
	return out, nil
}
