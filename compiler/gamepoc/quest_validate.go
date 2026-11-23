package gamepoc

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

var questWord = regexp.MustCompile(`^[a-z_]+$`)
var questEntry = regexp.MustCompile(`^e\.[a-z0-9_]+$`)
var questCheck = regexp.MustCompile(`^(.+?) · (.+?) · (once|retry|daily) → успех (.+?) / провал (.+)$`)

type questValidation struct {
	terms        map[string][]string
	translations map[string][]string
	ru           map[string]string
	ifids        map[string]string
}

func ValidateQuestView(view, trusted map[string]string) error {
	normalized, _, err := questV2View(view, trusted)
	if err != nil {
		return err
	}
	view = normalized
	glossary, err := questTable(trusted["glossary.tsv"], 4)
	if err != nil {
		return fmt.Errorf("словарь CUE: %w", err)
	}
	if _, err := questTable(view["glossary.tsv"], 4); err != nil {
		return fmt.Errorf("словарь дизайнера: %w", err)
	}
	trans, err := questTable(view["translations.tsv"], 3)
	if err != nil {
		return fmt.Errorf("переводы: %w", err)
	}
	for _, key := range sortedKeys(trans) {
		row := trans[key]
		if key != "" && (strings.TrimSpace(row[0]) == "" || strings.TrimSpace(row[1]) == "") {
			return fmt.Errorf("%s: отсутствует английский текст или контекст", key)
		}
	}
	v := questValidation{glossary, trans, map[string]string{}, map[string]string{}}
	for _, name := range sortedKeys(view) {
		switch {
		case strings.HasSuffix(name, ".twee"):
			if err := v.validateStory(name, view[name]); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
		case strings.HasSuffix(name, ".canvas"):
			if _, err := questMap(view[name]); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
		}
	}
	if len(v.ifids) == 0 {
		return fmt.Errorf("в папке нет историй Twee")
	}
	if view["act-one-chain.canvas"] == "" && view["акт-1/Цепочки.canvas"] == "" {
		return fmt.Errorf("нет карты цепочек первого акта")
	}
	return nil
}

func (v questValidation) skill(id string) error {
	row := v.terms[id]
	if len(row) == 0 || row[0] != "навык" {
		return fmt.Errorf("неизвестный навык %s", id)
	}
	return nil
}

func (v questValidation) references(key string, value any) error {
	switch val := value.(type) {
	case string:
		if val == "" {
			return nil
		}
		id := strings.SplitN(strings.TrimPrefix(val, "actor/"), "/", 2)[0]
		required := questReferenceKind[key]
		term := v.terms[id]
		if key == "skill" {
			return v.skill(id)
		}
		if required != "" && (len(term) == 0 || (term[0] != required && (required != "человек" || term[0] != "роль"))) {
			return fmt.Errorf("%s %s", questUnknown(required), id)
		}
		if questRef.MatchString(id) && term == nil && v.translations[id] == nil {
			return fmt.Errorf("%s %s", questUnknown(questRefName(id)), id)
		}
	case map[string]any:
		for _, field := range sortedKeys(val) {
			if (key == "positions" || key == "stage") && v.terms[field] == nil {
				return fmt.Errorf("неизвестный человек %s", field)
			}
			if field == "id" && key == "skill" {
				if err := v.skill(fmt.Sprint(val[field])); err != nil {
					return err
				}
			}
			kind := field
			if field == "id" && questReferenceKind[key] != "" {
				kind = key
			}
			if err := v.references(kind, val[field]); err != nil {
				return err
			}
		}
	case []any:
		for _, part := range val {
			if err := v.references(key, part); err != nil {
				return err
			}
		}
	}
	return nil
}

var questReferenceKind = map[string]string{
	"fact": "факт", "not_fact": "факт", "learn": "факт", "refutes": "факт",
	"status_fact": "факт", "ability_fact": "факт",
	"topic": "тема", "not_topic": "тема", "item": "предмет",
	"who": "человек", "seller": "человек", "owner": "человек", "known_by": "человек",
	"resume_routine": "человек", "seen_by": "человек", "not_seen_by": "человек", "deck_with": "человек",
	"witness": "человек", "people": "человек", "speakers": "человек",
}

func questUnknown(kind string) string {
	switch kind {
	case "тема":
		return "неизвестная тема"
	case "место":
		return "неизвестное место"
	default:
		return "неизвестный " + kind
	}
}

func questRefName(id string) string {
	prefix, _, _ := strings.Cut(id, ".")
	if caption := map[string]string{"fact": "факт", "topic": "тема", "char": "человек", "item": "предмет", "level": "место"}[prefix]; caption != "" {
		return caption
	}
	return "ID"
}
func singleQuestLink(s string) string {
	m := questMarkup.FindStringSubmatch(s)
	if m == nil || m[0] != s {
		return ""
	}
	return m[1]
}

func (v questValidation) validateStory(path, text string) error {
	pages, err := parseQuestStory(text)
	if err != nil {
		return err
	}
	data, err := questStoryData(pages["StoryData"].body)
	if err != nil {
		return err
	}
	ifid, _ := data["ifid"].(string)
	if ifid == "" || data["start"] != "Start" || data["format"] != "Paperthin" || data["format-version"] != "1.0.0" {
		return fmt.Errorf("проверьте IFID, Start и формат Paperthin 1.0.0")
	}
	if other := v.ifids[ifid]; other != "" {
		return fmt.Errorf("IFID уже есть в %s", other)
	}
	v.ifids[ifid] = path
	graph := map[string][]string{}
	types := map[string]string{}
	options := map[string]ConvOption{}
	optionsAt := map[string][]string{}
	voiceIDs := map[string]bool{}
	var exits []string
	for _, id := range sortedKeys(pages) {
		if id == "StoryTitle" || id == "StoryData" {
			continue
		}
		page := pages[id]
		if len(page.tags) != 1 {
			return fmt.Errorf("%s: у карточки должен быть один тег вида", id)
		}
		kind := page.tags[0]
		valid := false
		switch kind {
		case "dispatch":
			valid = id == "Start"
		case "entry":
			valid = questEntry.MatchString(id)
		case "beat":
			valid = questWord.MatchString(id)
		case "answer":
			valid = strings.HasPrefix(id, "answer.") && questWord.MatchString(strings.TrimPrefix(id, "answer."))
		case "event", "close":
			prefix := map[string]string{"event": "event.", "close": "end."}[kind]
			valid = strings.HasPrefix(id, prefix) && questWord.MatchString(strings.TrimPrefix(id, prefix))
		case "property":
			valid = questID.MatchString(id)
		}
		if !valid {
			return fmt.Errorf("%s: неверный ID или вид карточки %s", id, kind)
		}
		types[id] = kind
	}
	for _, id := range sortedKeys(types) {
		kind := types[id]
		page := pages[id]
		option := ConvOption{ID: strings.TrimPrefix(id, "answer.")}
		toTransition := 0
		for _, raw := range strings.Split(page.body, "\n") {
			line := strings.TrimLeft(raw, " ")
			prefix, val, hasValue := strings.Cut(line, ": ")
			add := func(target string, tags ...string) error {
				if dest := types[target]; !slices.Contains(tags, dest) || dest == "" {
					return fmt.Errorf("%s: неверный переход в %q", id, target)
				}
				graph[id] = append(graph[id], target)
				toTransition++
				return nil
			}
			if m := questText.FindStringSubmatch(line); m != nil {
				key, ru := m[3], m[2]
				if strings.TrimSpace(ru) == "" || v.translations[key] == nil {
					return fmt.Errorf("%s / %s: добавьте русский текст и строку с английским текстом и контекстом в translations.tsv", id, key)
				}
				if questMarkup.MatchString(ru) || strings.Contains(ru, "<") {
					return fmt.Errorf("%s / %s: экранируйте разметку в реплике", id, key)
				}
				if old, exists := v.ru[key]; exists && old != ru {
					return fmt.Errorf("%s: русский текст под одним ключом различается", key)
				}
				v.ru[key] = ru
				if voice, ok := strings.CutPrefix(m[1], "Голос "); ok {
					if !questWord.MatchString(voice) || voiceIDs[voice] {
						return fmt.Errorf("%s: пустой или повторный ID голоса %q", id, voice)
					}
					voiceIDs[voice] = true
				}
				if speaker, ok := strings.CutPrefix(m[1], "Реплика "); ok {
					if err := v.references("speaker", questNamePattern.ReplaceAllString(speaker, "")); err != nil {
						return fmt.Errorf("%s: %w", id, err)
					}
				}
				continue
			}
			if !hasValue {
				if slices.Contains([]string{"", "Кто открывает:", "Ответ собеседника:", "При провале:", "Реплики после голосов:"}, line) {
					continue
				}
				return fmt.Errorf("%s: непонятная строка %q (для заявки используйте нужно-новое.md)", id, line)
			}
			var lineErr error
			switch prefix {
			case "если", "эффект", "поле":
				var obj map[string]any
				if prefix == "если" {
					obj, lineErr = parseQuestCondition(val)
				} else {
					obj, lineErr = parseQuestFields(val, " · ")
				}
				if lineErr != nil {
					break
				}
				if lineErr = v.references("", obj); lineErr != nil {
					break
				}
				data, err := json.Marshal(obj)
				if err != nil {
					return err
				}
				switch prefix {
				case "если":
					var when When
					lineErr = questDecodeStrict(data, &when)
					if lineErr == nil && kind == "answer" && raw == line {
						option.When = &when
					}
				case "эффект":
					var effect Do
					lineErr = questDecodeStrict(data, &effect)
				}
			case "ответ":
				dest := singleQuestLink(val)
				lineErr = add(dest, "answer")
				optionsAt[id] = append(optionsAt[id], dest)
			case "вход":
				lineErr = add(singleQuestLink(val), "entry")
			case "войти":
				lineErr = add(singleQuestLink(val), "beat")
			case "событие":
				lineErr = add(singleQuestLink(val), "event")
			case "запись":
				lineErr = add(singleQuestLink(val), "property")
			case "переход", "после решения", "исход", "к вопросам", "в торговлю":
				lineErr = add(singleQuestLink(val), "beat", "close")
			case "заменяет":
				dest := singleQuestLink(val)
				if types[dest] != "answer" {
					lineErr = fmt.Errorf("неизвестный заменяемый ответ %q", dest)
				}
				option.Replaces = strings.TrimPrefix(dest, "answer.")
			case "проверка":
				m := questCheck.FindStringSubmatch(val)
				if m == nil {
					lineErr = fmt.Errorf("нужны навык, сложность, вид попытки, успех и провал")
					break
				}
				lineErr = v.skill(questNamePattern.ReplaceAllString(m[1], ""))
				if lineErr == nil {
					for _, dest := range m[4:] {
						target := questMarkup.FindStringSubmatch(dest)
						if target == nil {
							lineErr = fmt.Errorf("нужна ссылка в исходе проверки")
							break
						}
						if lineErr = add(target[1], "beat", "close"); lineErr != nil {
							break
						}
					}
				}
			case "один раз", "Запись", "Итог разговора":
			default:
				lineErr = fmt.Errorf("неизвестное поле %q", prefix)
			}
			if lineErr != nil {
				return fmt.Errorf("%s: %w", id, lineErr)
			}
		}
		switch kind {
		case "close", "property":
			exits = append(exits, id)
		default:
			if toTransition == 0 {
				return fmt.Errorf("%s: добавьте ссылку на следующую карточку или итог", id)
			}
		}
		if kind == "answer" {
			m := questText.FindStringSubmatch(strings.SplitN(page.body, "\n", 2)[0])
			if m == nil || (m[1] != "Герой" && m[1] != "Продолжить") {
				return fmt.Errorf("%s: ответ должен начинаться строкой героя с ключом", id)
			}
			options[id] = option
		}
	}
	owners := map[string]string{}
	for _, node := range sortedKeys(optionsAt) {
		var local []ConvOption
		localNames := map[string]bool{}
		for _, id := range optionsAt[node] {
			if other := owners[id]; other != "" {
				return fmt.Errorf("%s: ответ %s уже указан в %s", node, id, other)
			}
			owners[id] = node
			local = append(local, options[id])
			localNames[options[id].ID] = true
		}
		for _, opt := range local {
			if opt.Replaces != "" && (opt.Replaces == opt.ID || !localNames[opt.Replaces]) {
				return fmt.Errorf("%s: ответ %s заменяет ответ вне этого узла", node, opt.ID)
			}
		}
		if count, _ := maxAnswersShown(local); count > 9 {
			return fmt.Errorf("%s: одновременно видно %d ответов при пределе 9", node, count)
		}
	}
	reverse := map[string][]string{}
	for src, targets := range graph {
		for _, dest := range targets {
			reverse[dest] = append(reverse[dest], src)
		}
	}
	reachable := questWalk(graph, []string{"Start"})
	canClose := questWalk(reverse, exits)
	for _, id := range sortedKeys(types) {
		if !reachable[id] {
			return fmt.Errorf("%s: карточка недостижима из Start", id)
		}
		if !canClose[id] {
			return fmt.Errorf("%s: цикл без выхода к исходу", id)
		}
	}
	return nil
}
func questWalk(graph map[string][]string, todo []string) map[string]bool {
	seen := map[string]bool{}
	stack := slices.Clone(todo)
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[id] {
			continue
		}
		seen[id] = true
		stack = append(stack, graph[id]...)
	}
	return seen
}

func questDecodeStrict(data []byte, into any) error {
	d := json.NewDecoder(strings.NewReader(string(data)))
	d.DisallowUnknownFields()
	return d.Decode(into)
}
