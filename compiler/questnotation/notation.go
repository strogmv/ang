// Package questnotation is shared by the workspace export and the game's designer diagnostics.
package questnotation

import (
	"encoding/json"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// Formatter resolves human names alongside stable identifiers.
type Formatter func(string) string

// Maps are conjunctions in conditions. Lists and all repeated statements keep their order.
var grammar struct {
	once         sync.Once
	labels       map[string]string
	bare, number *regexp.Regexp
}

func initGrammar() {
	grammar.once.Do(func() {
		grammar.labels = map[string]string{
			"when": "если", "any": "любое из", "not": "не выполняется", "knows": "знает", "fact": "факт",
			"not_fact": "нет факта", "topic": "тема", "not_topic": "нет темы", "who": "кто",
			"args": "подробности", "by": "кто заметил", "answer": "ответ на линейке", "at_least": "не меньше", "below": "меньше", "id": "ID",
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
		grammar.bare = regexp.MustCompile(`^[a-zA-Z0-9_./-]+$`)
		grammar.number = regexp.MustCompile(`^-?[0-9]+(?:,[0-9]+)?(?:[eE][+-]?[0-9]+)?$`)
	})
}

// Labels returns the grammar's field captions.
func Labels() map[string]string {
	initGrammar()
	return grammar.labels
}

func IsBare(value string) bool {
	initGrammar()
	return grammar.bare.MatchString(value)
}

func IsNumber(value string) bool {
	initGrammar()
	return grammar.number.MatchString(value)
}

func (q Formatter) Phrase(v any) string {
	switch v := v.(type) {
	case map[string]any:
		return "(" + q.Fields(v) + ")"
	case []any:
		values := make([]string, 0, len(v))
		for _, member := range v {
			values = append(values, q.Phrase(member))
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
		if IsBare(v) && !IsNumber(v) && v != "-" {
			if name := q(v); name != "" && name != v {
				return v + " «" + strings.ReplaceAll(strings.ReplaceAll(name, "»", "›"), "\n", " ") + "»"
			}
			return v
		}
		return strconv.Quote(v)
	default:
		return "пусто"
	}
}

func (q Formatter) Fields(obj map[string]any) string {
	labels := Labels()
	fields := make([]string, 0, len(obj))
	for _, key := range slices.Sorted(maps.Keys(obj)) {
		label := labels[key]
		if label == "" {
			label = key
		}
		fields = append(fields, label+" = "+q.Phrase(obj[key]))
	}
	return strings.Join(fields, " · ")
}

func (q Formatter) Condition(obj map[string]any) string {
	var clauses []string
	for _, key := range slices.Sorted(maps.Keys(obj)) {
		value := obj[key]
		switch key {
		case "any":
			var alternatives []string
			for _, option := range value.([]any) {
				alternatives = append(alternatives, "("+q.Condition(option.(map[string]any))+")")
			}
			clauses = append(clauses, "("+strings.Join(alternatives, " ИЛИ ")+")")
		case "not":
			clauses = append(clauses, "НЕ ("+q.Condition(value.(map[string]any))+")")
		default:
			clauses = append(clauses, q.Fields(map[string]any{key: value}))
		}
	}
	return strings.Join(clauses, " И ")
}

// ClausePattern is a reversible sentence. Holes name paths in the original object.
// Each pattern describes one complete atom, including all of its optional fields.
type ClausePattern struct {
	Sentence string
	Paths    []string
}

func pattern(sentence string, paths ...string) ClausePattern {
	return ClausePattern{Sentence: sentence, Paths: paths}
}

func ConditionPatterns() []ClausePattern {
	return []ClausePattern{
		pattern("задание «%s» выполнено", "quest.id"), // done is supplied by the sentence reader
		pattern("задание %s на шаге %s", "quest.id", "quest.step"),
		pattern("%s доверяет герою (не меньше %s)", "trust.who", "trust.at_least"),
		pattern("при себе: %s", "item.item"),
		pattern("у %s есть %s с состоянием ниже %s", "item.owner", "item.item", "item.condition_below"),
		pattern("у %s есть %s", "item.owner", "item.item"),
		pattern("известно: %s (подробности: %s)", "fact.id", "fact.args"),
		pattern("известно: %s", "fact.id"),
		pattern("ещё не случилось: %s", "not_fact"),
		pattern("%s знает о том, что %s (уверенность не меньше %s)", "knows.who", "knows.fact", "knows.at_least"),
		pattern("%s знает о том, что %s", "knows.who", "knows.fact"),
		pattern("герой слышал о том, что %s", "topic"),
		pattern("герой ещё не слышал о том, что %s", "not_topic"),
		pattern("с %s до %s часов", "hour.from", "hour.to"),
		pattern("начиная с %s-го дня", "day.from"),
		pattern("с %s-го до %s-го дня", "day.from", "day.to"),
		pattern("рядом %s на площадке %s, в точке %s, %s (расстояние до %s клеток)", "present.people", "present.level", "present.at.x", "present.at.y", "present.reach"),
		pattern("на месте стоит %s", "placement"),
		pattern("у героя не меньше %s сотых CRD", "crd_minor_at_least"),
		pattern("прозвучала мысль %s", "voiced"),
	}
}

func EffectPatterns() []ClausePattern {
	return []ClausePattern{
		pattern("герой запоминает: %s (знают: %s)", "fact", "known_by"),
		pattern("записывается факт: %s (подробности: %s)", "fact", "args"),
		pattern("записывается факт: %s", "fact"),
		pattern("герой запоминает: %s", "learn"),
		pattern("герой узнаёт: %s", "topic"),
		pattern("%s доверяет больше (+%s)", "trust.who", "trust.delta"),
		pattern("%s доверяет меньше (%s)", "trust.who", "trust.delta"),
		pattern("герой забирает %s (плата %s сотых CRD)", "transfer.placement", "transfer.pay_crd_minor"),
		pattern("герой забирает %s", "transfer.placement"),
		pattern("задание %s переходит на шаг %s", "quest.id", "quest.step"),
		pattern("с места убирают %s", "remove_placement"),
		pattern("состояние объекта %s меняется на %s", "placement_condition.placement", "placement_condition.set"),
		pattern("состояние вещи %s у %s меняется на %s", "item_condition.item", "item_condition.owner", "item_condition.set"),
		pattern("герой получает %s от %s", "give_item.item", "give_item.from"),
		pattern("на карте появляется %s", "reveal_node"),
	}
}

// Sentence writes a condition using names without IDs. The authoring reader
// resolves names through the same glossary and restores the exact object.
func (q Formatter) Sentence(obj map[string]any) string {
	var clauses []string
	for _, key := range slices.Sorted(maps.Keys(obj)) {
		v := obj[key]
		switch key {
		case "any":
			var alternatives []string
			for _, choice := range v.([]any) {
				body := q.Sentence(choice.(map[string]any))
				if len(choice.(map[string]any)) > 1 {
					body = "(" + body + ")"
				}
				alternatives = append(alternatives, body)
			}
			clauses = append(clauses, "("+strings.Join(alternatives, " или ")+")")
		case "not":
			clauses = append(clauses, "неверно, что ("+q.Sentence(v.(map[string]any))+")")
		default:
			atom := map[string]any{key: v}
			if line, ok := q.sentencePattern(atom, ConditionPatterns()); ok {
				clauses = append(clauses, line)
			} else {
				clauses = append(clauses, q.PlainFields(atom))
			}
		}
	}
	return strings.Join(clauses, " и ")
}

func (q Formatter) Effect(obj map[string]any) string {
	if when, ok := obj["when"].(map[string]any); ok {
		copy := maps.Clone(obj)
		delete(copy, "when")
		return q.Effect(copy) + "; если " + q.Sentence(when)
	}
	if text, ok := q.sentencePattern(obj, EffectPatterns()); ok {
		return text
	}
	return q.PlainFields(obj)
}

func (q Formatter) sentencePattern(obj map[string]any, patterns []ClausePattern) (string, bool) {
	for _, p := range patterns {
		fields := flattenClause(obj, "")
		if p.Sentence == "задание «%s» выполнено" {
			if fields["quest.step"] != "done" {
				continue
			}
			delete(fields, "quest.step")
		}
		if p.Sentence == "при себе: %s" {
			if fields["item.owner"] != "hero" || fields["item.held"] != true {
				continue
			}
			delete(fields, "item.owner")
			delete(fields, "item.held")
		}
		if strings.HasPrefix(p.Sentence, "герой забирает ") {
			if fields["transfer.to"] != "hero" {
				continue
			}
			delete(fields, "transfer.to")
		}
		if delta, ok := fields["trust.delta"].(json.Number); ok {
			if strings.Contains(p.Sentence, "доверяет больше") && (strings.HasPrefix(delta.String(), "-") || delta.String() == "0") {
				continue
			}
			if strings.Contains(p.Sentence, "доверяет меньше") && !strings.HasPrefix(delta.String(), "-") {
				continue
			}
		}
		if len(fields) != len(p.Paths) {
			continue
		}
		values := make([]string, 0, len(p.Paths))
		for _, path := range p.Paths {
			v, ok := fields[path]
			if !ok {
				break
			}
			value := q.Plain(v)
			if path == "trust.who" && strings.HasPrefix(value, "«") && !strings.Contains(value, " ") {
				value = strings.Trim(value, "«»")
			}
			if p.Sentence == "задание «%s» выполнено" {
				value = strings.TrimSuffix(strings.TrimPrefix(value, "«"), "»")
			}
			values = append(values, value)
		}
		if len(values) == len(p.Paths) {
			var b strings.Builder
			parts := strings.Split(p.Sentence, "%s")
			for i, value := range values {
				b.WriteString(parts[i])
				b.WriteString(value)
			}
			b.WriteString(parts[len(parts)-1])
			return b.String(), true
		}
	}
	return "", false
}

func flattenClause(obj map[string]any, prefix string) map[string]any {
	out := map[string]any{}
	for k, v := range obj {
		switch sub := v.(type) {
		case map[string]any:
			if k != "args" {
				maps.Copy(out, flattenClause(sub, prefix+k+"."))
				continue
			}
		}
		out[prefix+k] = v
	}
	return out
}

func PlainEnums() map[string]string {
	return map[string]string{
		"always": "всегда", "news": "при новости", "once": "одна попытка", "daily": "раз в день", "retry": "можно повторять",
		"hard": "сложная", "medium": "средняя", "easy": "лёгкая", "done": "выполнено", "active": "в работе",
		"bronze": "бронза", "copper": "медь", "silver": "серебро", "gold": "золото", "hauling": "везёт груз",
		"private": "личный", "public": "публичный", "witnessed": "известен свидетелям", "recorded": "записанный",
	}
}

func (q Formatter) Plain(v any) string {
	switch value := v.(type) {
	case map[string]any:
		return "(" + q.PlainFields(value) + ")"
	case []any:
		parts := make([]string, 0, len(value))
		for _, one := range value {
			parts = append(parts, q.Plain(one))
		}
		return "⟨" + strings.Join(parts, " · ") + "⟩"
	case string:
		if caption := PlainEnums()[value]; caption != "" {
			return "«" + caption + "»"
		}
		if caption := q(value); caption != "" {
			return "«" + strings.NewReplacer("«", "‹", "»", "›", "\n", " ").Replace(caption) + "»"
		}
		return strconv.Quote(value)
	default:
		return q.Phrase(v)
	}
}

func PlainLabel(key string) string {
	if label := Labels()[key]; label != "" && label != "ID" {
		return label
	}
	if key == "id" {
		return "название"
	}
	return strings.ReplaceAll(key, "_", " ")
}

func (q Formatter) PlainFields(obj map[string]any) string {
	var clauses []string
	for _, key := range slices.Sorted(maps.Keys(obj)) {
		clauses = append(clauses, PlainLabel(key)+" — "+q.Plain(obj[key]))
	}
	return strings.Join(clauses, " · ")
}
