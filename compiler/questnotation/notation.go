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
