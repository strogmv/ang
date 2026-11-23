package gamepoc

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/strogmv/ang/compiler/questnotation"
)

const questAbout = "О сцене"
const questSettings = "Настройки сцены"
const questOutcomes = "Итоги разговора"

var questTitleByID = map[string]string{
	"intro": "первая встреча", "ask": "про землю", "offer": "цена кактуса", "bought": "кактус куплен",
	"poor": "не хватает денег", "seedlings": "жалоба на сырость", "inspect": "разговор со смотрителем",
	"sniff": "запах земли", "cassette": "запах груза", "open": "разговор", "assembly": "построение",
	"assign": "распределение", "kin": "запись КИН", "released": "линейка закончилась", "missing": "седьмая метка",
	"confirmed": "статус подтверждён", "correct": "замечание", "escort": "сопровождающий", "silver": "серебряный статус",
	"congratulate": "поздравление", "exemplary": "формула службы",
	"end.bye": "прощание", "end.filters": "разговор о фильтрах", "end.gather": "созвать людей", "end.know": "Варя поняла",
	"end.restricted": "стол урезали", "end.shared": "место поделено", "end.tray_fixed": "поддон починен", "end.trade": "торговля",
	"e.first": "первая встреча", "e.usual": "обычная встреча", "e.cassette": "герой везёт груз", "e.seedlings": "жалоба на рассаду",
	"e.inspect": "смотритель на месте", "e.stall_result": "что вышло с рассадой", "e.duty_rumour": "слух об отказе",
	"e.lie_rumour": "слух об обмане", "e.assembly": "начало линейки", "e.day3": "третий день",
}

var questAliases = map[string]string{
	"self": "Собеседник", "hero": "герой", "char.hero": "герой как участник мира", "narrator": "Рассказчик",
	"char.market_granny": "Бабушка Варя", "sq.a1.market_seedlings": "Рассада", "mq.a1.last_delivery": "Последняя доставка",
	"quest.gift_for_maryana": "Подарок Марьяне", "level.c7_market/place.c7_varya_tray": "поддон Вари",
	"level.c7_market/place.c7_varya_second_box":  "второй ящик Вари",
	"level.c7_market/place.c7_varya_soil_packet": "пакет земли Вари",
	"level.c7_market/place.c7_varya_extension":   "приставка к столу Вари",
	"fact.varya.platform_failed":                 "Грузовая платформа сломалась", "fact.platform.failed": "Грузовая платформа сломалась",
	"fact.preference.checked": "Кассету Вари проверили", "fact.preference.illness": "Кассета задержана из-за болезни",
	"fact.preference.violation": "Кассета задержана из-за нарушения", "fact.preference.ignored": "Кассету не приняли вовремя",
	"fact.cassette.noticed": "Кассету заметили", "fact.varya.inspection_requested": "Позвали смотрителя",
	"fact.varya.duty_rumour_said": "Варя заговорила об отказе", "fact.varya.lie_rumour_said": "Варя заговорила об обмане",
	"quest.course_graduation":  "Выпускная линейка",
	"fact.varya.cactus_bought": "Кактус для Марьяны куплен",
	"fact.varya.box_refused":   "Варя отказалась показывать второй ящик",
	"fact.lika.seen_with_hero": "Лику видели с героем",
	"fact.varya.box_put_away":  "Второй ящик убран", "fact.varya.result_said": "Варя рассказала, чем всё кончилось",
	"node.old_pumphouse_market": "Старая насосная", "varya": "от Вари", "grey": "серые куртки", "box": "второй ящик",
	"kin": "Терминал КИН", "observation": "Наблюдательность", "speech": "Речь", "maintenance": "Обслуживание",
}

var questScenePurpose = map[string]string{
	"01": "Герой знакомится с Прохором. Позднейшие встречи показывают, чем оборачивается арест и что остаётся после него.",
	"03": "Герой знакомится с Варей, может купить кактус и помочь решить спор о рассаде. Разговор о накладной связывает рынок с доставкой.",
	"18": "Линейка закрепляет статус героя, знакомит с выпускниками и оставляет зацепки о седьмой метке.",
	"19": "После линейки разговоры показывают порядки транспортного узла, его людей и доступные герою переходы.",
}

var questColors = map[string]string{"вход": "green", "разговор": "blue", "проверка": "orange", "итог": "purple", "пока-вне-игры": "gray", "настройки": "gray", "сведения": "gray"}

func passageID(tags []string, prefix string) string {
	for _, tag := range tags {
		if rest, ok := strings.CutPrefix(tag, prefix); ok {
			return rest
		}
	}
	return ""
}

type questPageWriter struct {
	q          *questExport
	words      questWords
	slots      map[string][]string
	aliasCount map[string]int
}

func (w *questPageWriter) name(id string) string {
	if questnotation.PlainEnums()[id] != "" {
		return questnotation.PlainEnums()[id]
	}
	if name := w.words.names[id]; name != "" && len(w.words.ids[name]) == 1 && name != id {
		return name
	}
	if id == "" {
		return ""
	}
	// Source data predates editor-facing names for some references. Assign a
	// unique glossary caption so raw identifiers are unnecessary in a script.
	kind := questRefName(id)
	if kind == "ID" {
		kind = "обозначение"
	}
	if term := w.q.glossary[id]; term.Kind != "" {
		kind = term.Kind
	}
	if alias := questAliases[id]; alias != "" && len(w.words.ids[alias]) == 0 {
		w.words.add(id, alias)
		w.q.glossary[id] = questTerm{kind, alias, "Обозначение из данных сцены"}
		return alias
	}
	name := w.words.names[id]
	if name == "" || name == id {
		head := strings.ToUpper(string([]rune(kind)[0])) + string([]rune(kind)[1:])
		w.aliasCount[kind]++
		name = fmt.Sprintf("%s №%d", head, w.aliasCount[kind])
	} else {
		name += " · " + kind
	}
	base := name
	for i := 2; len(w.words.ids[name]) > 0; i++ {
		name = fmt.Sprintf("%s %d", base, i)
	}
	delete(w.words.names, id)
	w.words.add(id, name)
	term := w.q.glossary[id]
	term.Kind, term.Name = kind, name
	if term.Meaning == "" {
		term.Meaning = "Обозначение из данных сцены"
	}
	w.q.glossary[id] = term
	return name
}

func (w *questPageWriter) rewrite(files map[string]string) error {
	for _, name := range sortedKeys(files) {
		if !strings.HasSuffix(name, ".twee") {
			continue
		}
		text, err := w.story(files[name])
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		files[name] = text
	}
	return nil
}

// A role with the interlocutor's display name must stay distinct from an
// explicit person in the same sentence, even in a one-person conversation.
func questLocalSpeaker(w questWords, speaker string) string {
	if speaker == "" {
		return ""
	}
	if ids := w.ids[speaker]; len(ids) == 0 || len(ids) == 1 && ids[0] == "self" {
		return speaker
	}
	first := strings.Fields(speaker)[0]
	if len(w.ids[first]) == 0 {
		return first
	}
	return speaker + " в этом разговоре"
}

func questV2Target(label, title string) string {
	if label == "" {
		return questLink(title)
	}
	return questLink(label + "->" + title)
}

func questHeading(id, body string, index int, prefix string) string {
	if name := questTitleByID[id]; name != "" {
		return prefix + " · " + name
	}
	for _, line := range strings.Split(body, "\n") {
		if m := questText.FindStringSubmatch(line); m != nil {
			words := strings.Fields(strings.Trim(m[2], "«» .,!?:—"))
			if len(words) > 0 {
				title := strings.Join(words[:min(5, len(words))], " ")
				title = strings.NewReplacer("[", "", "]", "", "{", "", "}", "", "|", "").Replace(title)
				return fmt.Sprintf("%s · %s (%d)", prefix, title, index)
			}
		}
	}
	return fmt.Sprintf("%s · эпизод %d", prefix, index)
}

func (w *questPageWriter) story(raw string) (string, error) {
	old, err := parseQuestStory(raw)
	if err != nil {
		return "", err
	}
	data, err := questStoryData(old["StoryData"].body)
	if err != nil {
		return "", err
	}
	storyID, speaker, _ := strings.Cut(strings.TrimSpace(old["StoryTitle"].body), " · ")
	world := storyID == "Мир"
	if world {
		storyID, speaker = "world.memory", "Мир"
	}
	if speaker == "" {
		switch {
		case strings.HasPrefix(storyID, "dlg.prokhor"):
			speaker = "Прохор"
		case storyID == "dlg.lineup":
			speaker = "Линейка"
		default:
			speaker = "Разговор"
		}
	}
	if speaker == "Бабушка Варя" {
		speaker = "Варя"
	}
	titles := map[string]string{"Start": questAbout}
	outcomes := map[string]string{}
	counter := 0
	for _, id := range sortedKeys(old) {
		switch {
		case slices.Contains(old[id].tags, "beat"), slices.Contains(old[id].tags, "event"):
			counter++
			titles[id] = questHeading(id, old[id].body, counter, speaker)
		case slices.Contains(old[id].tags, "property"):
			counter++
			title := w.q.glossary[id].Name
			if title == "" || title == id {
				title = "запись мира"
			}
			titles[id] = fmt.Sprintf("Мир · %s (%d)", questCleanName(title), counter)
		case strings.HasPrefix(id, "end."):
			title := questTitleByID[id]
			if title == "" {
				title = fmt.Sprintf("исход %d", len(outcomes)+1)
			}
			outcomes[id] = title
		}
	}
	links := func(line string) string {
		return questMarkup.ReplaceAllStringFunc(line, func(ref string) string {
			id := ref[2 : len(ref)-2]
			if title := titles[id]; title != "" {
				return questV2Target("", title)
			}
			if title := outcomes[id]; title != "" {
				return questV2Target(title, questOutcomes)
			}
			return ref
		})
	}
	ifid := data["ifid"].(string)
	format := questnotation.Formatter(func(id string) string {
		if id == "self" && !world && speaker != "Разговор" && speaker != "Линейка" {
			return questLocalSpeaker(w.words, speaker)
		}
		return w.name(id)
	})
	readFacts, writtenFacts, quests := map[string]bool{}, map[string]bool{}, map[string]bool{}
	var references func(any, bool)
	references = func(v any, writing bool) {
		switch obj := v.(type) {
		case map[string]any:
			for k, v := range obj {
				switch k {
				case "learn", "fact", "not_fact", "quest":
					key, _ := v.(string)
					if one, ok := v.(map[string]any); ok {
						key, _ = one["id"].(string)
					}
					if key != "" && k == "quest" && writing {
						quests[key] = true
					}
					if key != "" && k != "quest" {
						if writing && k == "fact" {
							writtenFacts[key] = true
						} else {
							readFacts[key] = true
						}
					}
				}
				references(v, writing && k != "when")
			}
		case []any:
			for _, v := range obj {
				references(v, writing)
			}
		}
	}
	body := func(id string, lines []string) ([]string, error) {
		out := []string{}
		slot := 0
		for _, line := range lines {
			trim := strings.TrimLeft(line, " ")
			indent := line[:len(line)-len(trim)]
			label, value, _ := strings.Cut(trim, ": ")
			switch {
			case questText.MatchString(trim):
				m := questText.FindStringSubmatch(trim)
				slot++
				w.slots[m[3]] = append(w.slots[m[3]], fmt.Sprintf("%s/%s/%d~%s", ifid, id, slot, m[1]))
				role := m[1]
				if who, ok := strings.CutPrefix(role, "Реплика "); ok {
					who = questNamePattern.ReplaceAllString(who, "")
					name := w.name(who)
					if who == "self" && !world && speaker != "Разговор" && speaker != "Линейка" {
						name = speaker
					}
					role = strings.ToUpper(name)
				} else if strings.HasPrefix(role, "Голос ") {
					role = "МЫСЛЬ ГЕРОЯ"
				}
				trim = role + ": " + m[2]
			case label == "если":
				condition, err := parseQuestCondition(value)
				if err != nil {
					return nil, err
				}
				references(condition, false)
				trim = "видно, если: " + format.Sentence(condition)
			case label == "эффект":
				effect, err := parseQuestFields(value, " · ")
				if err != nil {
					return nil, err
				}
				references(effect, true)
				trim = "Что меняется: " + format.Effect(effect)
			case label == "поле":
				fields, err := parseQuestFields(value, " · ")
				if err != nil {
					return nil, err
				}
				if fields["open"] == "always" || fields["event"] == false || fields["others"] == false ||
					id == "Start" && (fmt.Sprint(fields["reach"]) == "6" || fmt.Sprint(fields["linger_seconds"]) == "12") {
					continue
				}
				if arr, ok := fields["speakers"].([]any); ok && len(arr) == 0 {
					continue
				}
				if people, ok := fields["people"].([]any); id == "Start" && ok && len(people) == 1 {
					trim = "Разговор начинает " + format.Plain(people[0]) + "."
				} else {
					trim = "Уточнение: " + format.PlainFields(fields)
				}
			case label == "проверка":
				m := questCheck.FindStringSubmatch(value)
				if m == nil {
					return nil, fmt.Errorf("непонятная проверка %s", value)
				}
				skill := w.name(questNamePattern.ReplaceAllString(m[1], ""))
				difficulty := w.name(questNamePattern.ReplaceAllString(m[2], ""))
				trim = fmt.Sprintf("проверка: %s, %s, %s — успех → %s / провал → %s", skill, difficulty, w.name(m[3]), links(m[4]), links(m[5]))
			default:
				trim = links(trim)
			}
			out = append(out, indent+trim)
		}
		return out, nil
	}

	type card struct {
		id, title, kind string
		lines           []string
	}
	cards := []card{}
	startLines := strings.Split(old["Start"].body, "\n")
	settings, about, entryIDs := []string{}, []string{}, []string{}
	for _, line := range startLines {
		switch {
		case strings.HasPrefix(line, "вход: "):
			id := singleQuestLink(strings.TrimPrefix(line, "вход: "))
			entryIDs = append(entryIDs, id)
			caption := questTitleByID[id]
			if caption == "" {
				caption = "особое начало"
			}
			rows, err := body(id, strings.Split(old[id].body, "\n"))
			if err != nil {
				return "", err
			}
			about = append(about, fmt.Sprintf("Вход %d · %s", len(entryIDs), caption))
			about = append(about, indentQuest(rows)...)
		case strings.HasPrefix(line, "событие: "), strings.HasPrefix(line, "запись: "):
			about = append(about, links(line))
		default:
			settings = append(settings, line)
		}
	}
	set, err := body("Start", settings)
	if err != nil {
		return "", err
	}
	if len(set) > 0 {
		cards = append(cards, card{"settings", questSettings, "настройки", set})
	}
	if len(about) > 0 && len(set) > 0 {
		about = append([]string{"Как устроен разговор: " + questLink(questSettings), ""}, about...)
	}
	if len(entryIDs) > 0 {
		about = append(about, "", "коды: "+strings.Join(entryIDs, " · "))
	}
	if brief := w.q.briefs[storyID]; len(brief) > 0 {
		about = append(append(slices.Clone(brief), ""), about...)
	}
	cards = append([]card{{"Start", questAbout, "вход", about}}, cards...)
	if len(outcomes) > 0 {
		rows, codes := []string{}, []string{}
		for _, id := range sortedKeys(outcomes) {
			rows = append(rows, "Итог · "+outcomes[id])
			codes = append(codes, id)
		}
		rows = append(rows, "", "коды: "+strings.Join(codes, " · "))
		cards = append(cards, card{"outcomes", questOutcomes, "итог", rows})
	}
	for _, id := range sortedKeys(titles) {
		if id == "Start" {
			continue
		}
		var rows, codes, effects []string
		kind := "разговор"
		if world {
			kind = "сведения"
		}
		if strings.HasPrefix(id, "event.") {
			kind = "вход"
		}
		number := 0
		var nodeLines []string
		for _, line := range strings.Split(old[id].body, "\n") {
			if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "ответ: ") {
				answerID := singleQuestLink(strings.TrimPrefix(trimmed, "ответ: "))
				codes = append(codes, strings.TrimPrefix(answerID, "answer."))
				continue
			}
			if strings.HasPrefix(line, "эффект: ") {
				effects = append(effects, line)
			} else {
				nodeLines = append(nodeLines, line)
			}
		}
		rows, err = body(id, nodeLines)
		if err != nil {
			return "", err
		}
		for _, code := range codes {
			number++
			answerID := "answer." + code
			answer, err := body(answerID, strings.Split(old[answerID].body, "\n"))
			if err != nil {
				return "", err
			}
			first := fmt.Sprintf("%d. %s", number, answer[0])
			var detail []string
			for _, line := range answer[1:] {
				prefix, value, _ := strings.Cut(line, ": ")
				switch prefix {
				case "переход":
					first += " → " + value
				case "к вопросам":
					first += " → снова к вопросам " + value
				case "в торговлю":
					first += " → к торговле " + value
				case "проверка":
					kind = "проверка"
					detail = append(detail, line)
				case "заменяет":
					target := strings.TrimPrefix(singleQuestLink(value), "answer.")
					index := slices.Index(codes, target)
					if index < 0 {
						return "", fmt.Errorf("ответ %s заменяет ответ вне карточки", code)
					}
					detail = append(detail, fmt.Sprintf("заменяет ответ %d", index+1))
				default:
					detail = append(detail, line)
				}
			}
			rows = append(rows, "", first)
			rows = append(rows, indentQuest(detail)...)
		}
		if len(effects) > 0 {
			tail, err := body(id, effects)
			if err != nil {
				return "", err
			}
			rows = append(rows, "")
			rows = append(rows, tail...)
		}
		if len(codes) > 0 {
			rows = append(rows, "", "коды: "+strings.Join(codes, " · "))
		}
		cards = append(cards, card{id, titles[id], kind, rows})
	}
	var synopsis []string
	if speaker != "Разговор" {
		synopsis = append(synopsis, "В центре сцены — "+speaker+".")
	}
	for _, section := range []struct {
		label  string
		values map[string]bool
	}{
		{"Продвигает задания", quests}, {"Сверяется с фактами", readFacts}, {"Записывает факты", writtenFacts},
	} {
		if len(section.values) == 0 {
			continue
		}
		list := []string{}
		for _, key := range sortedKeys(section.values) {
			list = append(list, "«"+w.name(key)+"»")
		}
		synopsis = append(synopsis, section.label+" — "+strings.Join(list, ", ")+".")
	}
	if len(synopsis) > 0 {
		cards[0].lines = append(append(synopsis, ""), cards[0].lines...)
	}
	data["start"], data["tag-colors"] = questAbout, questColors
	// Match the converter's canonical Twee encoding so an untouched HTML
	// round trip preserves the exported file byte for byte.
	keys := []string{"ifid", "format", "format-version", "start"}
	for _, key := range sortedKeys(data) {
		if !slices.Contains(keys, key) {
			keys = append(keys, key)
		}
	}
	fields := make([]string, 0, len(keys))
	for _, key := range keys {
		value, err := json.Marshal(data[key])
		if err != nil {
			return "", err
		}
		fields = append(fields, fmt.Sprintf("%q:%s", key, value))
	}
	var out strings.Builder
	fmt.Fprintf(&out, ":: StoryTitle\n%s\n\n:: StoryData\n{%s}\n\n",
		strings.TrimSpace(old["StoryTitle"].body), strings.Join(fields, ","))
	rank := func(id string) int {
		switch id {
		case "Start":
			return 0
		case "settings":
			return 1
		default:
			return 2
		}
	}
	slices.SortFunc(cards, func(a, b card) int {
		if order := rank(a.id) - rank(b.id); order != 0 {
			return order
		}
		return strings.Compare(a.title, b.title)
	})
	columns := map[string]int{"вход": 0, "настройки": 0, "разговор": 1, "проверка": 2, "итог": 3, "сведения": 1}
	counts := [7]int{}
	for _, c := range cards {
		column := columns[c.kind]
		if c.kind == "сведения" {
			column = 1 + (counts[0]+counts[1]+counts[2]+counts[3]+counts[4]+counts[5])%5
		}
		tags := []string{c.kind, "id." + c.id}
		if c.id == "Start" {
			tags = append(tags, "story."+storyID)
		}
		slices.Sort(tags)
		fmt.Fprintf(&out, ":: %s [%s] {\"position\":\"%d,%d\",\"size\":\"100,100\"}\n%s\n\n", c.title,
			strings.Join(tags, " "), 80+175*column, 80+140*counts[column], strings.Join(c.lines, "\n"))
		counts[column]++
	}
	return strings.TrimSuffix(out.String(), "\n"), nil
}

func questAnswerCode(text string) string {
	hash := sha256.Sum256([]byte(text))
	// CUE option IDs use lowercase letters and underscores.
	var id strings.Builder
	id.WriteString("answer_")
	for _, b := range hash[:5] {
		id.WriteByte('a' + b>>4)
		id.WriteByte('a' + b&15)
	}
	return id.String()
}

func questTranslationsV2(view string, slots map[string][]string) (string, error) {
	rows, err := questTable(view, 3)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	csv := csv.NewWriter(&b)
	csv.Comma = '\t'
	_ = csv.Write(append(rows[""], "Строки"))
	for _, key := range sortedKeys(rows) {
		if key != "" {
			_ = csv.Write([]string{key, rows[key][0], rows[key][1], strings.Join(slots[key], " | ")})
		}
	}
	csv.Flush()
	return b.String(), csv.Error()
}

type questSlot struct{ key, role string }

var questV2Header = regexp.MustCompile(`(?m)^::[^\n]*[\[ ]id\.Start[ \]]`)
var questNumbered = regexp.MustCompile(`^([0-9]+)\. (.+)$`)
var questEntryHeading = regexp.MustCompile(`^Вход ([0-9]+) · (.+)$`)
var questCheckV2 = regexp.MustCompile(`^([^,]+), ([^,]+), (.+?) — успех → (.+) / провал → (.+)$`)

func questV2View(view, trusted map[string]string) (map[string]string, []string, error) {
	out := map[string]string{}
	for k, v := range view {
		out[k] = v
	}
	if !strings.Contains(view["translations.tsv"], "\tСтроки") {
		return out, nil, nil
	}
	rows, err := questTable(view["translations.tsv"], 4)
	if err != nil {
		return nil, nil, err
	}
	terms, err := questTable(trusted["glossary.tsv"], 4)
	if err != nil {
		return nil, nil, err
	}
	words := newQuestWords(terms)
	slots := map[string]questSlot{}
	var table strings.Builder
	writer := csv.NewWriter(&table)
	writer.Comma = '\t'
	_ = writer.Write(rows[""][:3])
	for _, key := range sortedKeys(rows) {
		if key == "" {
			continue
		}
		row := rows[key]
		_ = writer.Write([]string{key, row[0], row[1]})
		if row[2] == "" {
			continue
		}
		for _, entry := range strings.Split(row[2], " | ") {
			address, role, ok := strings.Cut(entry, "~")
			if !ok || address == "" || role == "" || slots[address].key != "" {
				return nil, nil, fmt.Errorf("переводы, %s: повтор или ошибка адреса строки %q", key, address)
			}
			slots[address] = questSlot{key, role}
		}
	}
	writer.Flush()
	out["translations.tsv"] = table.String()
	var requests []string
	for _, name := range sortedKeys(view) {
		if !strings.HasSuffix(name, ".twee") || !questV2Header.MatchString(view[name]) {
			continue
		}
		text, asked, err := words.readV2(view[name], slots)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", name, err)
		}
		out[name] = text
		for _, req := range asked {
			requests = append(requests, name+" / "+req)
		}
	}
	return out, requests, nil
}

func indentAs(line string) string {
	return line[:len(line)-len(strings.TrimLeft(line, " "))]
}

func questWithCodes(body string) ([]string, []string) {
	lines := strings.Split(strings.TrimSpace(body), "\n")
	codes := []string{}
	if last, ok := strings.CutPrefix(lines[len(lines)-1], "коды: "); ok {
		codes = strings.Split(last, " · ")
		lines = lines[:len(lines)-1]
	}
	return lines, codes
}

func (w questWords) readV2(raw string, slots map[string]questSlot) (string, []string, error) {
	pages, err := parseQuestStory(raw)
	if err != nil {
		return "", nil, err
	}
	data, err := questStoryData(pages["StoryData"].body)
	if err != nil {
		return "", nil, err
	}
	start, _ := data["start"].(string)
	startPage := pages[start]
	if passageID(startPage.tags, "id.") != "Start" {
		return "", nil, fmt.Errorf("начальная карточка должна иметь тег id.Start")
	}
	storyID := passageID(startPage.tags, "story.")
	if storyID == "" {
		return "", nil, fmt.Errorf("у начальной карточки потерян тег истории")
	}
	story := questStory{ID: storyID, Title: strings.TrimSpace(pages["StoryTitle"].body)}
	_, mainSpeaker, _ := strings.Cut(story.Title, " · ")
	if storyID == "world.memory" {
		mainSpeaker = ""
	}
	if mainSpeaker == "Бабушка Варя" {
		mainSpeaker = "Варя"
	}
	if mainSpeaker == "" && strings.HasPrefix(storyID, "dlg.prokhor") {
		mainSpeaker = "Прохор"
	}
	if mainSpeaker != "" {
		w.ids = maps.Clone(w.ids)
		w.ids[questLocalSpeaker(w, mainSpeaker)] = []string{"self"}
	}
	ifid := data["ifid"]
	if ifid != questIFID(storyID) || data["format"] != "Paperthin" || data["format-version"] != "1.0.0" {
		return "", nil, fmt.Errorf("проверьте IFID и формат Paperthin 1.0.0")
	}
	known := map[string]string{}
	byID := map[string]string{}
	for _, title := range sortedKeys(pages) {
		if title == "StoryTitle" || title == "StoryData" {
			continue
		}
		id := passageID(pages[title].tags, "id.")
		tagCount := 0
		for _, tag := range pages[title].tags {
			if strings.HasPrefix(tag, "id.") {
				tagCount++
			}
		}
		if tagCount != 1 || !questID.MatchString(id) || byID[id] != "" {
			return "", nil, fmt.Errorf("«%s»: укажите один уникальный тег id. для карточки", title)
		}
		known[title], byID[id] = id, title
	}
	outcomes := map[string]string{}
	if outcome, ok := byID["outcomes"]; ok {
		lines, codes := questWithCodes(pages[outcome].body)
		i := 0
		for _, line := range lines {
			label, ok := strings.CutPrefix(line, "Итог · ")
			if strings.TrimSpace(line) == "" {
				continue
			}
			if !ok || i >= len(codes) || !strings.HasPrefix(codes[i], "end.") || outcomes[label] != "" {
				return "", nil, fmt.Errorf("«%s»: каждому итогу нужен свой код и подпись", outcome)
			}
			id := codes[i]
			outcomes[label] = id
			story.card(id, "close", []string{"Итог разговора: " + strings.TrimPrefix(id, "end.")})
			i++
		}
		if i != len(codes) {
			return "", nil, fmt.Errorf("«%s»: потерян итог", outcome)
		}
	}
	links := func(line string) (string, error) {
		var linkErr error
		out := questMarkup.ReplaceAllStringFunc(line, func(ref string) string {
			text := ref[2 : len(ref)-2]
			label, title, arrow := strings.Cut(text, "->")
			if reverse, reverseLabel, ok := strings.Cut(text, "<-"); ok {
				title, label, arrow = reverse, reverseLabel, true
			}
			if !arrow {
				title = label
				label = ""
			}
			id := known[title]
			if id == "outcomes" {
				id = outcomes[label]
			}
			if id == "" {
				linkErr = fmt.Errorf("неизвестная карточка или итог %q. Проверьте ссылку [[название]]", text)
			}
			return questLink(id)
		})
		return out, linkErr
	}
	var asked []string
	normalize := func(title, id string, lines []string) ([]string, error) {
		var out []string
		slot := 0
		q := questExport{}
		for _, line := range lines {
			trim := strings.TrimSpace(line)
			if trim == "" {
				continue
			}
			if who, ok := strings.CutPrefix(trim, "Разговор начинает "); ok {
				person, err := w.value(strings.TrimSuffix(who, "."))
				if err != nil {
					return nil, fmt.Errorf("«%s»: %w", title, err)
				}
				out = append(out, indentAs(line)+"поле: "+q.fields(map[string]any{"people": []any{person}}))
				continue
			}
			indent := indentAs(line)
			label, value, _ := strings.Cut(trim, ": ")
			var err error
			var obj map[string]any
			switch label {
			case "нужно новое":
				if value == "" {
					return nil, fmt.Errorf("«%s»: опишите заявку после «нужно новое:»", title)
				}
				asked = append(asked, title+" — "+value)
				continue
			case "видно, если":
				obj, err = w.condition(value)
				if err == nil {
					trim = "если: " + q.condition(obj)
				}
			case "Что меняется":
				obj, err = w.effect(value)
				if err == nil {
					trim = "эффект: " + q.fields(obj)
				}
			case "Уточнение":
				obj, err = w.fields(value)
				if err == nil {
					trim = "поле: " + q.fields(obj)
				}
			case "проверка":
				m := questCheckV2.FindStringSubmatch(value)
				if m == nil {
					return nil, fmt.Errorf("«%s»: напишите навык, сложность, число попыток и ссылки для успеха и провала", title)
				}
				check := []string{}
				for _, caption := range m[1:4] {
					code, err := w.code(caption)
					if err != nil {
						return nil, fmt.Errorf("«%s»: %w", title, err)
					}
					check = append(check, code)
				}
				trim = fmt.Sprintf("проверка: %s → успех %s / провал %s", strings.Join(check, " · "), m[4], m[5])
				trim, err = links(trim)
			case "войти", "исход", "событие", "запись", "переход", "к вопросам", "в торговлю", "после решения":
				trim, err = links(trim)
			case "заменяет", "один раз":
				// References to a local answer were restored from its ordinal.
			default:
				if value != "" && (label == strings.ToUpper(label) || label == "Герой" || label == "Вариант" || label == "Продолжить" || label == "Текст") {
					slot++
					info := slots[fmt.Sprintf("%s/%s/%d", ifid, id, slot)]
					key, role := info.key, info.role
					if label == "ГЕРОЙ" {
						label = "Герой"
					}
					if key == "" {
						key = fmt.Sprintf("%s.%s.%d", storyID, strings.TrimPrefix(id, "answer."), slot)
						role = label
					}
					if strings.HasPrefix(role, "Реплика ") || (role == label && role == strings.ToUpper(role) && role != "МЫСЛЬ ГЕРОЯ") {
						who := questNamePattern.ReplaceAllString(strings.TrimPrefix(role, "Реплика "), "")
						name := w.names[who]
						if who == "self" && mainSpeaker != "" {
							name = mainSpeaker
						}
						if label != strings.ToUpper(name) {
							if mainSpeaker != "" && label == strings.ToUpper(mainSpeaker) {
								who = "self"
							} else {
								who = ""
								for _, candidate := range sortedKeys(w.names) {
									if (strings.HasPrefix(candidate, "char.") || candidate == "narrator") && label == strings.ToUpper(w.names[candidate]) {
										if who != "" {
											return nil, fmt.Errorf("«%s»: двое говорящих с именем %s", title, label)
										}
										who = candidate
									}
								}
								if who == "" {
									return nil, fmt.Errorf("«%s»: говорящий %s отсутствует в словаре", title, label)
								}
							}
						}
						role = "Реплика " + who
					} else if strings.HasPrefix(role, "Голос ") && label != "МЫСЛЬ ГЕРОЯ" {
						return nil, fmt.Errorf("«%s»: у внутренней реплики оставьте подпись «МЫСЛЬ ГЕРОЯ»", title)
					} else if !strings.HasPrefix(role, "Голос ") {
						role = label
					}
					trim = role + ": " + value + "  ‹" + key + "›"
				}
			}
			if err != nil {
				return nil, fmt.Errorf("«%s»: %w", title, err)
			}
			out = append(out, indent+trim)
		}
		return out, nil
	}

	var dispatch []string
	if title := byID["settings"]; title != "" {
		dispatch, err = normalize(title, "Start", strings.Split(pages[title].body, "\n"))
		if err != nil {
			return "", nil, err
		}
	}
	lines, entryIDs := questWithCodes(startPage.body)
	var overview []string
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		m := questEntryHeading.FindStringSubmatch(line)
		if m == nil {
			if strings.HasPrefix(line, "событие: ") || strings.HasPrefix(line, "запись: ") || strings.HasPrefix(line, "нужно новое: ") {
				rows, err := normalize(start, "Start", []string{line})
				if err != nil {
					return "", nil, err
				}
				dispatch = append(dispatch, rows...)
			} else if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "Как устроен разговор: ") {
				overview = append(overview, line)
			}
			continue
		}
		n, _ := strconv.Atoi(m[1])
		if n < 1 || n > len(entryIDs) {
			return "", nil, fmt.Errorf("«%s»: у входа %d потерян код", start, n)
		}
		id := entryIDs[n-1]
		end := i + 1
		rows := []string{}
		for end < len(lines) && strings.HasPrefix(lines[end], "  ") {
			rows = append(rows, lines[end][2:])
			end++
		}
		if rows, err = normalize(start, id, rows); err != nil {
			return "", nil, err
		}
		story.card(id, "entry", rows)
		dispatch = append(dispatch, "вход: "+questLink(id))
		i = end - 1
	}
	story.card("Start", "dispatch", dispatch)
	for _, id := range sortedKeys(byID) {
		title := byID[id]
		if id == "Start" || id == "settings" || id == "outcomes" {
			continue
		}
		lines, codes := questWithCodes(pages[title].body)
		node, answers := []string{}, []string{}
		usedCodes := map[string]bool{}
		for i := 0; i < len(lines); i++ {
			line := lines[i]
			m := questNumbered.FindStringSubmatch(line)
			if m == nil {
				node = append(node, line)
				continue
			}
			n, _ := strconv.Atoi(m[1])
			if n != len(answers)+1 {
				return "", nil, fmt.Errorf("«%s»: пронумеруйте ответы подряд, начиная с 1", title)
			}
			text, branch, hasBranch := strings.Cut(m[2], " → ")
			if !hasBranch && strings.HasSuffix(text, "]]") {
				if last := strings.LastIndex(text, " [["); last >= 0 {
					text, branch, hasBranch = text[:last], text[last+1:], true
				}
			}
			code := questAnswerCode(id + "\n" + strings.TrimPrefix(text, "Герой: "))
			if n <= len(codes) {
				code = codes[n-1]
			}
			if !questWord.MatchString(code) || usedCodes[code] {
				return "", nil, fmt.Errorf("«%s»: повтор или ошибка кода ответа %d", title, n)
			}
			usedCodes[code] = true
			answerID := "answer." + code
			answers = append(answers, "ответ: "+questLink(answerID))
			if !strings.HasPrefix(text, "Герой: ") && !strings.HasPrefix(text, "Продолжить: ") {
				text = "Герой: " + text
			}
			rows := []string{text}
			end := i + 1
			for end < len(lines) && strings.HasPrefix(lines[end], "  ") {
				detail := lines[end][2:]
				if after, ok := strings.CutPrefix(detail, "заменяет ответ "); ok {
					ref, _ := strconv.Atoi(after)
					if ref < 1 || ref > len(codes) {
						return "", nil, fmt.Errorf("«%s»: заменяемый ответ %s отсутствует", title, after)
					}
					detail = "заменяет: " + questLink("answer."+codes[ref-1])
				}
				rows = append(rows, detail)
				end++
			}
			if hasBranch {
				label := "переход"
				if rest, ok := strings.CutPrefix(branch, "снова к вопросам "); ok {
					label, branch = "к вопросам", rest
				}
				if rest, ok := strings.CutPrefix(branch, "к торговле "); ok {
					label, branch = "в торговлю", rest
				}
				rows = append(rows, label+": "+branch)
			}
			i = end - 1
			rows, err = normalize(title, answerID, rows)
			if err != nil {
				return "", nil, err
			}
			story.card(answerID, "answer", rows)
		}
		if len(codes) > len(answers) {
			return "", nil, fmt.Errorf("«%s»: удалите код убранного ответа из строки «коды»", title)
		}
		node, err = normalize(title, id, node)
		if err != nil {
			return "", nil, err
		}
		kind := "beat"
		if storyID == "world.memory" {
			kind = "property"
		}
		if strings.HasPrefix(id, "event.") {
			kind = "event"
		}
		story.card(id, kind, append(node, answers...))
	}
	normalized := story.render()
	if len(overview) > 0 {
		parsed, err := parseQuestStory(normalized)
		if err != nil {
			return "", nil, err
		}
		body := parsed["StoryData"].body
		data, err := questStoryData(body)
		if err != nil {
			return "", nil, err
		}
		data["описание сцены"] = overview
		fields, _ := json.Marshal(data)
		normalized = strings.Replace(normalized, body, string(fields), 1)
	}
	return normalized, asked, nil
}

// QuestViewRequests returns in-card requests as review notes, never as game data.
func QuestViewRequests(view, trusted map[string]string) ([]string, error) {
	_, requests, err := questV2View(view, trusted)
	return requests, err
}
