package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Maps are conjunctions in conditions. Lists and all repeated statements keep their order.
var questLabels = map[string]string{
	"when": "если", "any": "любое из", "not": "не выполняется", "knows": "знает", "fact": "факт",
	"not_fact": "нет факта", "topic": "тема", "not_topic": "нет темы", "who": "кто",
	"args": "подробности", "at_least": "не меньше", "below": "меньше", "id": "ID",
	"today": "сегодня", "before_today": "до сегодняшнего дня", "hours_since": "прошло часов",
	"within_hours": "за последние часы", "calendar_days_since": "прошло календарных дней",
	"train_berth": "остановка поезда", "deck_phase": "этап платформы", "deck": "платформа", "phases": "этапы",
	"file_trait": "черта из личного дела", "file_strength": "сильная сторона из личного дела",
	"quest": "квест", "step": "шаг", "hour": "час", "day": "день", "from": "от", "to": "в",
	"present": "встреча", "hero_at": "герой на месте", "level": "место", "at": "клетка", "reach": "радиус",
	"people": "люди", "crd_minor_at_least": "денег не меньше (сотых CRD)", "placement": "объект на месте",
	"trust": "доверие", "skill": "навык", "asked": "выбран ответ", "not_asked": "не выбран ответ",
	"voiced": "прозвучал голос", "entry": "вход", "interjected": "была вставка",
	"speaker": "говорит", "caste": "каста", "item": "вещь", "owner": "у владельца", "held": "при себе",
	"condition_below": "состояние ниже", "condition_at_least": "состояние от", "world": "состояние мира",
	"seen_by": "опознан", "not_seen_by": "не опознан", "resume_routine": "вернуть к распорядку",
	"deck_ride": "поездка на платформе", "deck_with": "едут вместе", "rating_loss": "потеря рейтинга",
	"transfer": "передать", "also": "а также", "ground": "на земле", "pay_crd_minor": "плата (сотые CRD)",
	"remove_placement": "убрать с места", "placement_condition": "состояние объекта на месте",
	"reveal_node": "открыть на карте", "learn": "сообщить факт герою", "delta": "изменение",
	"give_item": "дать вещь", "item_condition": "состояние вещи", "set": "станет",
	"visibility": "видимость", "recorded": "записан", "known_by": "сразу знают", "memory_days": "помнят дней",
	"sight_cells": "радиус зрения", "hearing_cells": "радиус слуха", "rumour": "слух",
	"journal": "журнал", "mention": "упоминание", "record_access": "доступ к записи",
	"replace_args": "заменять подробности", "legacy_participant": "прежний участник",
	"refutes": "опровергает факт", "from_barks": "из выкриков", "external_readers": "внешние читатели",
	"external_writers": "внешние писатели", "on_trade": "при покупке", "seller": "продавец",
	"conversation": "разговор", "positions": "точки людей", "open": "способ открытия", "others": "остальные люди",
	"stage": "позиции участников", "keep_stage": "сохранить позиции", "after_decision": "после выбора",
	"no_interjection": "без вставки", "answer_seconds": "секунд на ответ", "linger_seconds": "секунд на чтение итога",
	"stage_level": "место группы", "speakers": "другие говорящие", "named_speakers": "имена говорящих",
	"event": "разговор по событию", "waits": "ждёт реализации", "actors": "имена в подстановках",
	"belief": "уверенность", "difficulty": "сложность", "kind": "тип", "text": "ключ текста",
	"status": "статус", "scenario": "режим новой игры", "hero_status": "статус героя",
	"status_fact": "факт статуса", "ability_fact": "факт способности", "marks": "отметки", "promotions": "повышения",
	"fresh_days": "дней свежести", "heard_belief": "вера в слух",
}

var questBare = regexp.MustCompile(`^[a-zA-Z0-9_./-]+$`)
var questNumber = regexp.MustCompile(`^-?[0-9]+(?:,[0-9]+)?(?:[eE][+-]?[0-9]+)?$`)
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

func (q *questExport) phrase(v any) string {
	switch v := v.(type) {
	case map[string]any:
		return "(" + q.fields(v) + ")"
	case []any:
		values := make([]string, 0, len(v))
		for _, member := range v {
			values = append(values, q.phrase(member))
		}
		return "⟨" + strings.Join(values, " · ") + "⟩"
	case json.Number:
		return strings.ReplaceAll(v.String(), ".", ",")
	case bool:
		if v {
			return "да"
		}
		return "нет"
	case string:
		if questBare.MatchString(v) && !questNumber.MatchString(v) && v != "-" {
			if row := q.glossary[v]; row.Name != "" && row.Name != v {
				return v + " «" + strings.ReplaceAll(strings.ReplaceAll(row.Name, "»", "›"), "\n", " ") + "»"
			}
			return v
		}
		return strconv.Quote(v)
	default:
		return "пусто"
	}
}

func (q *questExport) fields(obj map[string]any) string {
	fields := make([]string, 0, len(obj))
	for _, key := range sortedKeys(obj) {
		label := questLabels[key]
		if label == "" {
			label = key
		}
		fields = append(fields, label+" = "+q.phrase(obj[key]))
	}
	return strings.Join(fields, " · ")
}

func (q *questExport) condition(obj map[string]any) string {
	var clauses []string
	for _, key := range sortedKeys(obj) {
		value := obj[key]
		switch key {
		case "any":
			var alternatives []string
			for _, option := range value.([]any) {
				alternatives = append(alternatives, "("+q.condition(option.(map[string]any))+")")
			}
			clauses = append(clauses, "("+strings.Join(alternatives, " ИЛИ ")+")")
		case "not":
			clauses = append(clauses, "НЕ ("+q.condition(value.(map[string]any))+")")
		default:
			clauses = append(clauses, q.fields(map[string]any{key: value}))
		}
	}
	return strings.Join(clauses, " И ")
}

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
		for field, caption := range questLabels {
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
	case questNumber.MatchString(s):
		return json.Number(strings.ReplaceAll(s, ",", ".")), nil
	case s == "да":
		return true, nil
	case s == "нет":
		return false, nil
	case s == "пусто":
		return nil, nil
	}
	id := questNamePattern.ReplaceAllString(s, "")
	if questBare.MatchString(id) && id != "-" {
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
