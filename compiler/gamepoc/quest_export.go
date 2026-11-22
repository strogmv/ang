package gamepoc

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"cuelang.org/go/cue"
)

type questTranslation struct {
	Key     string
	Context string
	Texts   map[string]string
	Params  map[string]string
}

type questTerm struct{ Kind, Name, Meaning string }

type questExport struct {
	files    map[string]string
	glossary map[string]questTerm
	lex      map[string]questTranslation
	used     map[string]bool
	canvas   questCanvas
	subjects map[string]string
}

func questRead(v cue.Value, path string, into any) error {
	value := v.LookupPath(cue.ParsePath(path))
	if !value.Exists() {
		return nil
	}
	return decodeStrict(value, path, into)
}

func questRaw(v cue.Value, path string) (any, error) {
	value := v.LookupPath(cue.ParsePath(path))
	if !value.Exists() {
		return nil, nil
	}
	data, err := value.MarshalJSON()
	if err != nil {
		return nil, err
	}
	var result any
	err = questJSON(data, &result)
	return result, err
}

// ExportQuestView evaluates the same CUE source as game generation. Files are
// written to a new directory by the command, leaving Git and existing views alone.
func ExportQuestView(gameRoot string) (map[string]string, error) {
	project := filepath.Join(gameRoot, "server")
	config, err := cuePackage(filepath.Join(project, "cue", "poc"))
	if err != nil {
		return nil, err
	}
	dir, err := config.LookupPath(cue.ParsePath("GamePOC.world_dir")).String()
	if err != nil {
		return nil, err
	}
	cueRoot, err := cuePackage(filepath.Join(project, dir))
	if err != nil {
		return nil, err
	}
	world := cueRoot.LookupPath(cue.ParsePath("world"))
	memory, convs, err := readMemory(world)
	if err != nil {
		return nil, err
	}
	if memory == nil {
		return nil, fmt.Errorf("в CUE отсутствует world.memory")
	}
	scripts, err := readMemoryScripts(gameRoot)
	if err != nil {
		return nil, err
	}
	sources, err := questConversationSources(world, gameRoot, scripts, convs)
	if err != nil {
		return nil, err
	}
	compiled, _, err := readCompiled(project)
	if err != nil {
		return nil, err
	}
	if err := compileBarks(project, &compiled); err != nil {
		return nil, err
	}
	compiled.memoryScripts = scripts
	if err := checkMemory(&compiled); err != nil {
		return nil, err
	}

	q := questExport{files: map[string]string{}, glossary: map[string]questTerm{}, lex: map[string]questTranslation{},
		used: map[string]bool{}, subjects: map[string]string{}}
	var translations []questTranslation
	if err := questRead(world, "localization.messages", &translations); err != nil {
		return nil, err
	}
	for _, t := range translations {
		q.lex[t.Key] = t
	}
	if err := q.loadTerms(world, memory, compiled.Life); err != nil {
		return nil, err
	}
	q.scriptCards(scripts, sources)
	for _, id := range sortedKeys(convs) {
		path := sources.path(id)
		if err := q.conversation(path, convs[id], memory); err != nil {
			return nil, fmt.Errorf("%s: %w", id, err)
		}
	}
	if err := q.worldCards(world, memory, sources); err != nil {
		return nil, err
	}
	q.storyLinks(compiled.memoryLinks)

	var glossary, table strings.Builder
	g, t := csv.NewWriter(&glossary), csv.NewWriter(&table)
	g.Comma, t.Comma = '\t', '\t'
	_ = g.Write([]string{"ID", "Вид", "Название", "Что означает"})
	_ = t.Write([]string{"Ключ", "Контекст", "Английский"})
	for _, id := range sortedKeys(q.glossary) {
		term := q.glossary[id]
		_ = g.Write([]string{id, term.Kind, term.Name, term.Meaning})
	}
	for _, key := range sortedKeys(q.used) {
		row := q.lex[key]
		if strings.TrimSpace(row.Texts["ru"]) == "" || strings.TrimSpace(row.Texts["en"]) == "" || strings.TrimSpace(row.Context) == "" {
			return nil, fmt.Errorf("%s: требуются русский, английский и контекст", key)
		}
		_ = t.Write([]string{key, row.Context, row.Texts["en"]})
	}
	g.Flush()
	t.Flush()
	q.files["glossary.tsv"] = glossary.String()
	q.files["translations.tsv"] = table.String()
	q.files["act-one-chain.canvas"] = q.canvas.render()
	return q.files, nil
}

type questSource struct {
	conversations, interjections map[string]string
	count                        map[string]int
	title                        map[string]string
}

func (s questSource) path(id string) string {
	script := s.conversations[id]
	if script == "" {
		return id + "/scene.twee"
	}
	if s.count[script] == 1 {
		return "scene-" + script + "/scene.twee"
	}
	return "scene-" + script + "/" + strings.TrimPrefix(id, "dlg.") + ".twee"
}

func questConversationSources(world cue.Value, gameRoot string, scripts map[string]string, convs map[string]*Conversation) (questSource, error) {
	sources, err := readQuestSources(gameRoot, scripts, convs)
	if err != nil {
		return sources, err
	}

	calendar, err := questRaw(world, "calendar_encounter.talk")
	if err != nil {
		return sources, err
	}
	if values, ok := calendar.(map[string]any); ok && scripts["01"] != "" {
		for _, role := range sortedKeys(values) {
			id := values[role].(string)
			if convs[id] != nil && sources.conversations[id] == "" {
				sources.conversations[id] = "01"
				sources.count["01"]++
			}
		}
	}
	return sources, nil
}

func readQuestSources(root string, scripts map[string]string, convs map[string]*Conversation) (questSource, error) {
	out := questSource{map[string]string{}, map[string]string{}, map[string]int{}, map[string]string{}}
	folder := filepath.Join(root, "design", "dialogue", "scripts")
	raw, err := os.ReadFile(filepath.Join(folder, "usage_sources.json"))
	if err != nil {
		return out, err
	}
	var entries map[string]struct{ Conversations, Interjections []string }
	if err := json.Unmarshal(raw, &entries); err != nil {
		return out, err
	}
	for id, filename := range scripts {
		data, err := os.ReadFile(filepath.Join(folder, filename))
		if err != nil {
			return out, err
		}
		title, _, _ := strings.Cut(string(data), "\n")
		out.title[id] = strings.TrimPrefix(title, "# ")
	}
	for _, id := range sortedKeys(entries) {
		row := entries[id]
		for _, talk := range row.Conversations {
			if convs[talk] == nil || out.conversations[talk] != "" {
				return out, fmt.Errorf("сценарий %s: неверная ссылка на разговор %s", id, talk)
			}
			out.conversations[talk] = id
			out.count[id]++
		}
		for _, interjection := range row.Interjections {
			out.interjections[interjection] = id
		}
	}
	return out, nil
}

func firstQuestLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

func (q *questExport) loadTerms(world cue.Value, m *Memory, rawLife json.RawMessage) error {
	for _, catalog := range []struct{ path, kind string }{
		{"persons", "человек"}, {"items", "предмет"}, {"levels", "место"}, {"sites", "место"},
		{"dialogue_rules.skills", "навык"},
	} {
		var rows map[string]json.RawMessage
		if err := questRead(world, catalog.path, &rows); err != nil {
			return err
		}
		for id, data := range rows {
			var row struct {
				Name, ShortName, Title, Description string
				Tags                                []string
			}
			_ = json.Unmarshal(data, &row)
			name := row.Name
			if name == "" {
				name = row.Title
			}
			if message := q.lex[name].Texts["ru"]; message != "" {
				name = message
			}
			if message := q.lex["actor."+id+".name"].Texts["ru"]; message != "" {
				name = message
			}
			meaning := row.Description
			if meaning == "" {
				meaning = strings.Join(row.Tags, ", ")
			}
			if meaning == "" {
				meaning = name
			}
			if name == "" {
				name = id
			}
			if meaning == "" {
				meaning = "В CUE задан идентификатор " + id
			}
			q.glossary[id] = questTerm{catalog.kind, name, firstQuestLine(meaning)}
		}
	}
	var life struct {
		Actors    []struct{ ID, Name string }
		Locations map[string]struct{ Name string }
	}
	if len(rawLife) > 0 {
		if err := json.Unmarshal(rawLife, &life); err != nil {
			return err
		}
	}
	for _, actor := range life.Actors {
		if _, ok := q.glossary[actor.ID]; !ok {
			name := q.lex["actor."+actor.ID+".name"].Texts["ru"]
			if name == "" {
				name = actor.Name
			}
			if name == "" {
				name = actor.ID
			}
			q.glossary[actor.ID] = questTerm{"человек", name, "Участник мира"}
		}
	}
	for id, place := range life.Locations {
		if _, ok := q.glossary[id]; !ok {
			name := place.Name
			if name == "" {
				name = id
			}
			q.glossary[id] = questTerm{"место", name, "Место в игровом мире"}
		}
	}
	var levels []struct{ ID, Text string }
	if err := questRead(world, "dialogue_rules.difficulties", &levels); err != nil {
		// Difficulty records contain the numeric value as well as their localized label.
		raw, err := questRaw(world, "dialogue_rules.difficulties")
		if err != nil {
			return err
		}
		if known, ok := raw.([]any); ok {
			for _, v := range known {
				row := v.(map[string]any)
				id := row["id"].(string)
				q.glossary[id] = questTerm{"сложность", q.lex[row["text"].(string)].Texts["ru"], "Уровень проверки"}
			}
		}
	}

	for _, key := range sortedKeys(q.lex) {
		if strings.HasPrefix(key, "actor.char.") && strings.HasSuffix(key, ".name") {
			id := strings.TrimSuffix(strings.TrimPrefix(key, "actor."), ".name")
			row := q.lex[key]
			if _, exists := q.glossary[id]; !exists {
				q.glossary[id] = questTerm{"человек", row.Texts["ru"], firstQuestLine(row.Context)}
			}
		}
	}
	for _, id := range []string{"hero", "self", "char.hero"} {
		name := "Герой"
		if id == "self" {
			name = "Собеседник"
		}
		q.glossary[id] = questTerm{"роль", name, name + " в текущем разговоре"}
	}
	for id, fact := range m.Facts {
		name := q.lex[fact.Journal].Texts["ru"]
		if name == "" {
			name = q.lex[fact.Mention].Texts["ru"]
		}
		if name == "" {
			name = id
		}
		q.glossary[id] = questTerm{"факт", firstQuestLine(name), fmt.Sprintf("%s. Хранится %d дн.", visibilityName(fact.Visibility), fact.MemoryDays)}
	}
	for id, topic := range m.Topics {
		name := firstQuestLine(q.lex[topic.Journal].Texts["ru"])
		if name == "" {
			name = id
		}
		q.glossary[id] = questTerm{"тема", name, "Сведение, услышанное героем"}
	}
	for id, steps := range m.Quests {
		q.glossary[id] = questTerm{"квест", id, strings.Join(steps, " → ")}
	}
	return nil
}

func visibilityName(id string) string {
	return map[string]string{"private": "Личный", "witnessed": "Известен свидетелям", "recorded": "В журнале купола", "public": "Публичный"}[id]
}

type questCard struct {
	ID, Tag string
	Lines   []string
}
type questStory struct {
	ID, Title string
	Cards     []questCard
}

func (s *questStory) card(id, tag string, lines []string) {
	s.Cards = append(s.Cards, questCard{id, tag, lines})
}
func questIFID(id string) string {
	h := sha256.Sum256([]byte("ang-quest-view/v1/" + id))
	h[6] = h[6]&0xf | 0x50
	h[8] = h[8]&0x3f | 0x80
	return strings.ToUpper(fmt.Sprintf("%x-%x-%x-%x-%x", h[:4], h[4:6], h[6:8], h[8:10], h[10:16]))
}
func (s *questStory) render() string {
	var b strings.Builder
	fmt.Fprintf(&b, ":: StoryTitle\n%s\n\n:: StoryData\n{\"ifid\":%q,\"format\":\"Paperthin\",\"format-version\":\"1.0.0\",\"start\":\"Start\"}\n\n", s.Title, questIFID(s.ID))
	slices.SortFunc(s.Cards, func(a, b questCard) int { return strings.Compare(a.ID, b.ID) })
	types := map[string]int{"dispatch": 0, "entry": 1, "beat": 2, "answer": 3, "event": 4, "close": 5, "property": 6}
	var rows [7]int
	for _, card := range s.Cards {
		column := types[card.Tag]
		fmt.Fprintf(&b, ":: %s [%s] {\"position\":\"%d,%d\",\"size\":\"100,100\"}\n%s\n\n", card.ID, card.Tag, 80+170*column, 80+140*rows[column], strings.Join(card.Lines, "\n"))
		rows[column]++
	}
	return strings.TrimSuffix(b.String(), "\n")
}

var questTextEscape = strings.NewReplacer("\\", "\\\\", "\r", "\\r", "\n", "\\n", "[[", `\u005b[`, "<", `\u003c`)
var questMarkup = regexp.MustCompile(`\[\[([^\[\]\r\n]+)\]\]`)

func (q *questExport) line(role, key string) string {
	row := q.lex[key]
	q.used[key] = true
	return role + ": " + questTextEscape.Replace(row.Texts["ru"]) + "  ‹" + key + "›"
}
func questLink(id string) string { return "[[" + id + "]]" }

func (q *questExport) propertyFields(v map[string]any) []string {
	var lines []string
	for _, key := range sortedKeys(v) {
		val := v[key]
		switch key {
		case "when":
			lines = append(lines, "если: "+q.condition(val.(map[string]any)))
		case "do":
			for _, effect := range val.([]any) {
				lines = append(lines, "эффект: "+q.fields(effect.(map[string]any)))
			}
		default:
			lines = append(lines, "поле: "+q.fields(map[string]any{key: val}))
			q.localizedProperties(val, &lines)
		}
	}
	return lines
}
func (q *questExport) localizedProperties(value any, lines *[]string) {
	switch v := value.(type) {
	case map[string]any:
		for _, key := range sortedKeys(v) {
			q.localizedProperties(v[key], lines)
		}
	case []any:
		for _, part := range v {
			q.localizedProperties(part, lines)
		}
	case string:
		if _, exists := q.lex[v]; exists {
			*lines = append(*lines, q.line("Текст", v))
		}
	}
}
func (q *questExport) takeProperties(input any, skip ...string) ([]string, error) {
	obj, err := questObject(input)
	if err != nil {
		return nil, err
	}
	for _, field := range skip {
		delete(obj, field)
	}
	return q.propertyFields(obj), nil
}
func (q *questExport) lines(rows []ConvLine) ([]string, error) {
	var out []string
	for _, row := range rows {
		fields, err := q.takeProperties(row, "speaker", "text", "variants")
		if err != nil {
			return nil, err
		}
		out = append(out, q.line("Реплика "+q.phrase(row.Speaker), row.Text))
		out = append(out, indentQuest(fields)...)
		for _, variant := range row.Variants {
			v, err := q.takeProperties(&variant, "text")
			if err != nil {
				return nil, err
			}
			out = append(out, "  "+q.line("Вариант", variant.Text))
			out = append(out, indentQuest(indentQuest(v))...)
		}
	}
	return out, nil
}
func indentQuest(lines []string) []string {
	result := make([]string, len(lines))
	for i, line := range lines {
		result[i] = "  " + line
	}
	return result
}
func (q *questExport) conversation(path string, c *Conversation, m *Memory) error {
	id := c.ID
	s := questStory{ID: id, Title: id}
	var speakerName string
	for _, row := range m.Talks {
		if row.Conversation == id && len(row.People) == 1 {
			if name := q.glossary[row.People[0]].Name; name != "" {
				speakerName = name
				break
			}
		}
	}
	if speakerName != "" {
		s.Title += " · " + speakerName
	}
	ends := map[string]bool{}
	branch := func(current, next, close string, trade bool) string {
		switch {
		case close != "":
			ends[close] = true
			return questLink("end." + close)
		case trade:
			ends["trade"] = true
			return "торговля " + questLink("end.trade")
		case next != "":
			return questLink(next)
		default:
			return "к вопросам " + questLink(current)
		}
	}
	var err error
	dispatch, err := q.takeProperties(c, "id", "entries", "nodes", "events")
	if err != nil {
		return err
	}
	for _, entry := range c.Entries {
		body, err := q.lines(entry.Lines)
		if err != nil {
			return err
		}
		fields, err := q.takeProperties(entry, "id", "lines", "node")
		if err != nil {
			return err
		}
		body = append(body, fields...)
		s.card(entry.ID, "entry", append(body, "войти: "+questLink(entry.Node)))
		dispatch = append(dispatch, "вход: "+questLink(entry.ID))
	}
	for _, talk := range m.Talks {
		if talk.Conversation != id {
			continue
		}
		fields, err := q.takeProperties(talk, "conversation")
		if err != nil {
			return err
		}
		dispatch = append(dispatch, "Кто открывает:")
		dispatch = append(dispatch, indentQuest(fields)...)
	}
	for _, name := range sortedKeys(c.Events) {
		event := c.Events[name]
		body, err := q.lines(event.Lines)
		if err != nil {
			return err
		}
		fields, err := q.takeProperties(event, "lines", "close")
		if err != nil {
			return err
		}
		body = append(body, fields...)
		s.card("event."+name, "event", append(body, "исход: "+branch("", "", event.Close, false)))
		dispatch = append(dispatch, "событие: "+questLink("event."+name))
	}
	s.card("Start", "dispatch", dispatch)
	for _, name := range sortedKeys(c.Nodes) {
		node := c.Nodes[name]
		body, err := q.lines(node.Lines)
		if err != nil {
			return err
		}
		fields, err := q.takeProperties(node, "lines", "tail_lines", "voices", "options")
		if err != nil {
			return err
		}
		body = append(body, fields...)
		for _, voice := range node.Voices {
			line := q.line("Голос "+voice.ID, voice.Text)
			fields, err := q.takeProperties(voice, "id", "text", "variants")
			if err != nil {
				return err
			}
			body = append(body, line)
			body = append(body, indentQuest(fields)...)
			for _, variant := range voice.Variants {
				fields, err := q.takeProperties(variant, "text")
				if err != nil {
					return err
				}
				body = append(body, "  "+q.line("Вариант", variant.Text))
				body = append(body, indentQuest(indentQuest(fields))...)
			}
		}
		if len(node.TailLines) > 0 {
			lines, err := q.lines(node.TailLines)
			if err != nil {
				return err
			}
			body = append(body, "Реплики после голосов:")
			body = append(body, indentQuest(lines)...)
		}
		for _, option := range node.Options {
			fields, err := q.takeProperties(option, "id", "text", "variants", "reply", "next", "close", "trade", "follow_up", "check", "fail", "continue", "once", "replaces")
			if err != nil {
				return err
			}
			answer := []string{q.line("Герой", option.Text)}
			if option.Continue {
				answer[0] = q.line("Продолжить", option.Text)
			}
			answer = append(answer, fields...)
			for _, variant := range option.Variants {
				v, err := q.takeProperties(variant, "text")
				if err != nil {
					return err
				}
				answer = append(answer, q.line("Вариант", variant.Text))
				answer = append(answer, indentQuest(v)...)
			}
			if option.Once {
				answer = append(answer, "один раз: да")
			}
			if option.Replaces != "" {
				answer = append(answer, "заменяет: "+questLink("answer."+option.Replaces))
			}
			if len(option.Reply) > 0 {
				answer = append(answer, "Ответ собеседника:")
				lines, err := q.lines(option.Reply)
				if err != nil {
					return err
				}
				answer = append(answer, indentQuest(lines)...)
			}
			dest := branch(name, option.Next, option.Close, option.Trade)
			if option.Check != nil && option.Fail != nil {
				check := option.Check
				answer = append(answer, fmt.Sprintf("проверка: %s · %s · %s → успех %s / провал %s", q.phrase(check.Skill), q.phrase(check.Difficulty), q.phrase(check.Kind), dest, branch(name, option.Fail.Next, option.Fail.Close, false)))
				answer = append(answer, "При провале:")
				lines, err := q.lines(option.Fail.Reply)
				if err != nil {
					return err
				}
				fields, err := q.takeProperties(option.Fail, "reply", "next", "close")
				if err != nil {
					return err
				}
				answer = append(answer, indentQuest(append(lines, fields...))...)
			} else {
				switch {
				case option.Trade:
					answer = append(answer, "в торговлю: "+questLink("end.trade"))
				case option.Next == "" && option.Close == "":
					answer = append(answer, "к вопросам: "+questLink(name))
				default:
					answer = append(answer, "переход: "+dest)
				}
			}
			if option.FollowUp != "" {
				answer = append(answer, "после решения: "+questLink(option.FollowUp))
			}
			s.card("answer."+option.ID, "answer", answer)
			body = append(body, "ответ: "+questLink("answer."+option.ID))
		}
		s.card(name, "beat", body)
	}
	for _, name := range sortedKeys(ends) {
		s.card("end."+name, "close", []string{"Итог разговора: " + name})
	}
	q.files[path] = s.render()
	q.subjects[id] = id
	shape := sortedKeys(ends)
	q.canvas.note(id, id+"\n\n[Карточки]("+path+")\n\nИсходы: "+strings.Join(shape, ", "), "4")
	return nil
}
