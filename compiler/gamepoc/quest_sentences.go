package gamepoc

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/strogmv/ang/compiler/questnotation"
)

type questWords struct {
	names map[string]string
	ids   map[string][]string
}

func newQuestWords(terms map[string][]string) questWords {
	w := questWords{map[string]string{}, map[string][]string{}}
	for id, name := range questnotation.PlainEnums() {
		w.add(id, name)
	}
	for _, id := range sortedKeys(terms) {
		if id != "" {
			w.add(id, terms[id][1])
		}
	}
	return w
}

func (w questWords) add(id, name string) {
	name = questCleanName(name)
	if name == "" || w.names[id] != "" {
		return
	}
	w.names[id] = name
	w.ids[name] = append(w.ids[name], id)
}

func questCleanName(name string) string {
	return strings.NewReplacer("«", "‹", "»", "›", "\n", " ").Replace(name)
}

func (w questWords) code(name string) (string, error) {
	ids := w.ids[name]
	switch len(ids) {
	case 0:
		return "", fmt.Errorf("не найдено «%s» в словаре. Проверьте название или опишите нужное в строке «нужно новое»", name)
	case 1:
		return ids[0], nil
	default:
		return "", fmt.Errorf("у «%s» несколько значений. Уточните название по словарю (%s)", name, strings.Join(ids, ", "))
	}
}

func (w questWords) value(s string) (any, error) {
	switch {
	case strings.HasPrefix(s, "«") && strings.HasSuffix(s, "»"):
		return w.code(strings.TrimSuffix(strings.TrimPrefix(s, "«"), "»"))
	case strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")"):
		return w.fields(s[1 : len(s)-1])
	case strings.HasPrefix(s, "⟨") && strings.HasSuffix(s, "⟩"):
		parts, err := splitQuest(s[len("⟨"):len(s)-len("⟩")], " · ")
		if err != nil {
			return nil, err
		}
		values := []any{}
		for _, p := range parts {
			if p == "" {
				continue
			}
			v, err := w.value(p)
			if err != nil {
				return nil, err
			}
			values = append(values, v)
		}
		return values, nil
	default:
		if len(w.ids[s]) > 0 {
			return w.code(s)
		}
		return parseQuestValue(s)
	}
}

func (w questWords) fields(s string) (map[string]any, error) {
	parts, err := splitQuest(s, " · ")
	if err != nil {
		return nil, err
	}
	result := map[string]any{}
	for _, p := range parts {
		if p == "" {
			continue
		}
		label, raw, ok := strings.Cut(p, " — ")
		if !ok {
			return nil, fmt.Errorf("непонятная фраза «%s». Сверьтесь с примером в README", p)
		}
		key := strings.ReplaceAll(label, " ", "_")
		for code := range questnotation.Labels() {
			if questnotation.PlainLabel(code) == label {
				key = code
				break
			}
		}
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("повтор «%s»", label)
		}
		v, err := w.value(raw)
		if err != nil {
			return nil, err
		}
		result[key] = v
	}
	return result, nil
}

type questSentencePattern struct {
	match *regexp.Regexp
	questnotation.ClausePattern
}

func questSentencePatterns(patterns []questnotation.ClausePattern) []questSentencePattern {
	out := make([]questSentencePattern, 0, len(patterns))
	for _, p := range patterns {
		pattern := strings.ReplaceAll(regexp.QuoteMeta(p.Sentence), "%s", "(.+?)")
		out = append(out, questSentencePattern{regexp.MustCompile("^" + pattern + "$"), p})
	}
	return out
}

var questConditions = questSentencePatterns(questnotation.ConditionPatterns())
var questEffects = questSentencePatterns(questnotation.EffectPatterns())

func (w questWords) match(s string, patterns []questSentencePattern) (map[string]any, error) {
	for _, p := range patterns {
		parts := p.match.FindStringSubmatch(s)
		if parts == nil {
			continue
		}
		fields := map[string]any{}
		for i, path := range p.Paths {
			part := parts[i+1]
			if p.Sentence == "задание «%s» выполнено" {
				part = "«" + part + "»"
			}
			val, err := w.value(part)
			if err != nil {
				return nil, err
			}
			obj := fields
			keys := strings.Split(path, ".")
			for _, key := range keys[:len(keys)-1] {
				sub, exists := obj[key].(map[string]any)
				if !exists {
					sub = map[string]any{}
					obj[key] = sub
				}
				obj = sub
			}
			obj[keys[len(keys)-1]] = val
		}
		switch p.Sentence {
		case "задание «%s» выполнено":
			fields["quest"].(map[string]any)["step"] = "done"
		case "герой забирает %s", "герой забирает %s (плата %s сотых CRD)":
			fields["transfer"].(map[string]any)["to"] = "hero"
		case "при себе: %s":
			item := fields["item"].(map[string]any)
			item["owner"], item["held"] = "hero", true
		}
		return fields, nil
	}
	return w.fields(s)
}

func (w questWords) effect(s string) (map[string]any, error) {
	pieces, err := splitQuest(s, "; если ")
	if err != nil || len(pieces) > 2 {
		return nil, fmt.Errorf("после изменения допускается одно «; если …»")
	}
	out, err := w.match(pieces[0], questEffects)
	if err != nil {
		return nil, err
	}
	if len(pieces) == 2 {
		out["when"], err = w.condition(pieces[1])
	}
	return out, err
}

func (w questWords) condition(s string) (map[string]any, error) {
	s = strings.TrimSpace(s)
	alternatives, err := splitQuest(s, " или ")
	if err != nil {
		return nil, err
	}
	if len(alternatives) > 1 {
		choices := []any{}
		for _, alt := range alternatives {
			if strings.HasPrefix(alt, "(") && strings.HasSuffix(alt, ")") {
				alt = alt[1 : len(alt)-1]
			}
			one, err := w.condition(alt)
			if err != nil {
				return nil, err
			}
			choices = append(choices, one)
		}
		return map[string]any{"any": choices}, nil
	}
	conjunction, err := splitQuest(s, " и ")
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	for _, atom := range conjunction {
		var step map[string]any
		if rest, ok := strings.CutPrefix(atom, "неверно, что ("); ok && strings.HasSuffix(rest, ")") {
			nested, err := w.condition(rest[:len(rest)-1])
			if err != nil {
				return nil, err
			}
			step = map[string]any{"not": nested}
		} else if strings.HasPrefix(atom, "(") && strings.HasSuffix(atom, ")") {
			step, err = w.condition(atom[1 : len(atom)-1])
			if err == nil && step["any"] == nil {
				step = map[string]any{"any": []any{step}}
			}
		} else {
			step, err = w.match(atom, questConditions)
		}
		if err != nil {
			return nil, err
		}
		for k, v := range step {
			if _, exists := out[k]; exists {
				return nil, fmt.Errorf("условие «%s» повторяется. Для вариантов напишите «или»", questnotation.PlainLabel(k))
			}
			out[k] = v
		}
	}
	return out, nil
}
