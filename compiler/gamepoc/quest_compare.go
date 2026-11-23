package gamepoc

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var questHeader = regexp.MustCompile(`^:: (.+?)(?:\s+\[([^\]]*)\])?(?:\s+(\{.*\}))?$`)
var questID = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)
var questText = regexp.MustCompile(`^(.*?): (.*)  ‹([a-zA-Z0-9_.-]+)›$`)
var questRef = regexp.MustCompile(`^(fact\.|topic\.|item\.|char\.|level\.|quest\.|sq\.|mq\.)`)

type questPassage struct {
	tags []string
	meta map[string]any
	body string
}

func questLF(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.TrimPrefix(s, "\ufeff"), "\r\n", "\n"), "\r", "\n")
}
func parseQuestStory(s string) (map[string]questPassage, error) {
	result := map[string]questPassage{}
	var id string
	var passage questPassage
	var body []string
	save := func() {
		if id == "" {
			return
		}
		passage.body = strings.TrimRight(strings.Join(body, "\n"), "\n")
		result[id] = passage
	}
	for _, line := range strings.Split(questLF(s), "\n") {
		if strings.HasPrefix(line, "::") {
			save()
			m := questHeader.FindStringSubmatch(line)
			if m == nil || strings.TrimSpace(m[1]) == "" {
				return nil, fmt.Errorf("неверный или пустой ID карточки: %s", line)
			}
			id = m[1]
			if _, exists := result[id]; exists {
				return nil, fmt.Errorf("повтор ID карточки %s", id)
			}
			passage = questPassage{tags: strings.Fields(m[2]), meta: map[string]any{}}
			slices.Sort(passage.tags)
			passage.tags = slices.Compact(passage.tags)
			if m[3] != "" {
				if err := questJSON([]byte(m[3]), &passage.meta); err != nil {
					return nil, err
				}
			}
			delete(passage.meta, "position")
			delete(passage.meta, "size")
			body = nil
			continue
		}
		if id == "" && line != "" {
			return nil, fmt.Errorf("текст перед первой карточкой")
		}
		body = append(body, line)
	}
	save()
	for _, name := range []string{"StoryTitle", "StoryData"} {
		if _, ok := result[name]; !ok {
			return nil, fmt.Errorf("отсутствует карточка %s", name)
		}
	}
	data, err := questStoryData(result["StoryData"].body)
	if err != nil {
		return nil, err
	}
	start, _ := data["start"].(string)
	if _, exists := result[start]; start == "" || !exists {
		return nil, fmt.Errorf("отсутствует начальная карточка %q", start)
	}
	// V2 stores stable ASCII identifiers in tags and gives passages author
	// names. Legacy stories still use their passage names as identifiers.
	if passageID(result[start].tags, "id.") != "Start" {
		if _, exists := result["Start"]; !exists {
			return nil, fmt.Errorf("отсутствует карточка Start")
		}
		for name := range result {
			if !questID.MatchString(name) {
				return nil, fmt.Errorf("неверный ID карточки: %s", name)
			}
		}
	}
	return result, nil
}

func questStoryData(body string) (map[string]any, error) {
	var data map[string]any
	if err := questJSON([]byte(body), &data); err != nil {
		return nil, err
	}
	for _, key := range []string{"zoom", "tag-colors"} {
		delete(data, key)
	}
	return data, nil
}

func IsQuestViewFile(name string) bool {
	switch filepath.Base(name) {
	case "Словарь.md", "Сводка.md", "Решения автора.md":
		return true
	}
	switch filepath.Ext(name) {
	case ".twee", ".canvas", ".tsv":
		return true
	default:
		return false
	}
}
func ReadQuestView(dir string) (map[string]string, error) {
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("нет папки %s", dir)
	}
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(file string, ent fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ent.IsDir() {
			if file != dir && strings.HasPrefix(ent.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !IsQuestViewFile(file) {
			return nil
		}
		if !ent.Type().IsRegular() {
			return fmt.Errorf("ожидался обычный файл: %s", file)
		}
		relative, err := filepath.Rel(dir, file)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(relative)] = string(data)
		return nil
	})
	return out, err
}

func questTable(s string, cols int) (map[string][]string, error) {
	r := csv.NewReader(strings.NewReader(questLF(s)))
	r.Comma, r.FieldsPerRecord = '\t', cols
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("пустая таблица")
	}
	out := map[string][]string{}
	for i, row := range rows {
		if i == 0 {
			out[""] = row
			continue
		}
		if row[0] == "" {
			return nil, fmt.Errorf("строка %d: отсутствует ID", i+1)
		}
		if _, exists := out[row[0]]; exists {
			return nil, fmt.Errorf("повтор ID %s", row[0])
		}
		out[row[0]] = row[1:]
	}
	return out, nil
}

type questNormalMap struct {
	nodes, edges map[string]map[string]any
	extra        map[string]any
}

func questMap(s string) (questNormalMap, error) {
	var data map[string]json.RawMessage
	m := questNormalMap{extra: map[string]any{}}
	if err := questJSON([]byte(questLF(s)), &data); err != nil {
		return m, err
	}
	for _, spec := range []struct {
		key    string
		out    *map[string]map[string]any
		layout []string
	}{
		{"nodes", &m.nodes, []string{"x", "y", "width", "height", "color"}},
		{"edges", &m.edges, []string{"fromSide", "toSide", "fromEnd", "toEnd", "color"}},
	} {
		var list []map[string]any
		if len(data[spec.key]) == 0 {
			return m, fmt.Errorf("Canvas: нет поля %s", spec.key)
		}
		if err := questJSON(data[spec.key], &list); err != nil {
			return m, err
		}
		*spec.out = map[string]map[string]any{}
		for _, row := range list {
			id, _ := row["id"].(string)
			if id == "" {
				return m, fmt.Errorf("Canvas: нет ID в %s", spec.key)
			}
			if _, exists := (*spec.out)[id]; exists {
				return m, fmt.Errorf("Canvas: повтор ID %s", id)
			}
			for _, key := range spec.layout {
				delete(row, key)
			}
			(*spec.out)[id] = row
		}
		delete(data, spec.key)
	}
	for id, row := range m.edges {
		for _, key := range []string{"fromNode", "toNode"} {
			node, _ := row[key].(string)
			if m.nodes[node] == nil {
				return m, fmt.Errorf("Canvas %s: неизвестная карточка %q", id, node)
			}
		}
	}
	for _, key := range sortedKeys(data) {
		var value any
		if err := questJSON(data[key], &value); err != nil {
			return m, err
		}
		m.extra[key] = value
	}
	return m, nil
}

func questRawEqual(a, b any) bool {
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	return string(left) == string(right)
}

func normalizeQuestBody(body string) string {
	lines := strings.Split(body, "\n")
	q := questExport{}
	for i, line := range lines {
		trim := strings.TrimLeft(line, " ")
		prefix, val, _ := strings.Cut(trim, ": ")
		switch prefix {
		case "если", "эффект":
			var obj map[string]any
			var err error
			if prefix == "если" {
				obj, err = parseQuestCondition(val)
			} else {
				obj, err = parseQuestFields(val, " · ")
			}
			if err != nil {
				continue
			}
			if prefix == "если" {
				val = q.condition(obj)
			} else {
				val = q.fields(obj)
			}
			// The authored IDs and values matter. Friendly captions following an ID are derived from the glossary.
			lines[i] = line[:len(line)-len(trim)] + prefix + ": " + val
		}
	}
	return strings.Join(lines, "\n")
}

// CompareQuestViews compares workspace text only. Its normal form has no dependency
// on the CUE source or on the version of an editor installed on this machine.
func CompareQuestViews(before, after map[string]string) ([]string, error) {
	left, _, err := questV2View(before, before)
	if err != nil {
		return nil, err
	}
	vocabulary := before
	if !strings.Contains(before["translations.tsv"], "\tСтроки") {
		vocabulary = after
	}
	right, _, err := questV2View(after, vocabulary)
	if err != nil {
		return nil, err
	}
	before, after = left, right
	var changes []string
	for _, name := range questUnion(before, after) {
		a, aok := before[name]
		b, bok := after[name]
		if !aok {
			changes = append(changes, "Добавлен файл "+name)
			continue
		}
		if !bok {
			changes = append(changes, "Удалён файл "+name)
			continue
		}
		if a == b {
			continue
		}
		switch filepath.Ext(name) {
		case ".md":
			if name != "Словарь.md" && questLF(a) != questLF(b) {
				changes = append(changes, "Изменён "+name)
			}
		case ".twee":
			x, err := parseQuestStory(a)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			y, err := parseQuestStory(b)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			for _, id := range questUnion(x, y) {
				left, lok := x[id]
				right, rok := y[id]
				place := name + " / " + id
				added, removed := "Добавлена карточка", "Удалена карточка"
				if strings.HasPrefix(id, "answer.") {
					added, removed = "Добавлен ответ", "Удалён ответ"
				}
				if !lok {
					changes = append(changes, added+" "+place)
					continue
				}
				if !rok {
					changes = append(changes, removed+" "+place)
					continue
				}
				if !questRawEqual(left.meta, right.meta) || !slices.Equal(left.tags, right.tags) {
					changes = append(changes, "Изменены свойства карточки "+place)
				}
				if id == "StoryData" {
					l, err := questStoryData(left.body)
					if err != nil {
						return nil, err
					}
					r, err := questStoryData(right.body)
					if err != nil {
						return nil, err
					}
					if !questRawEqual(l, r) {
						changes = append(changes, "Изменены данные истории "+name)
					}
				} else if normalizeQuestBody(left.body) != normalizeQuestBody(right.body) {
					changes = append(changes, questLineDiff(place, left.body, right.body)...)
				}
			}
		case ".canvas":
			x, err := questMap(a)
			if err != nil {
				return nil, err
			}
			y, err := questMap(b)
			if err != nil {
				return nil, err
			}
			if !questRawEqual(x.extra, y.extra) {
				changes = append(changes, "Изменены данные карты "+name)
			}
			for _, id := range questUnion(x.nodes, y.nodes) {
				if !questRawEqual(x.nodes[id], y.nodes[id]) {
					changes = append(changes, "Изменена карточка карты "+id)
				}
			}
			for _, id := range questUnion(x.edges, y.edges) {
				xr, yr := x.edges[id], y.edges[id]
				switch {
				case xr == nil:
					changes = append(changes, "Добавлена связь "+questEdgeLabel(yr))
				case yr == nil:
					changes = append(changes, "Удалена связь "+questEdgeLabel(xr))
				case !questRawEqual(xr, yr):
					changes = append(changes, "Изменена связь "+questEdgeLabel(xr)+" → "+questEdgeLabel(yr))
				}
			}
		case ".tsv":
			count := 3
			if name == "glossary.tsv" {
				count = 4
			}
			x, err := questTable(a, count)
			if err != nil {
				return nil, err
			}
			y, err := questTable(b, count)
			if err != nil {
				return nil, err
			}
			for _, id := range questUnion(x, y) {
				if !slices.Equal(x[id], y[id]) {
					changes = append(changes, fmt.Sprintf("%s / %s: %q → %q", name, id, x[id], y[id]))
				}
			}
		}
	}
	return changes, nil
}

func questUnion[V any](a, b map[string]V) []string {
	keys := make(map[string]bool, len(a)+len(b))
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	return sortedKeys(keys)
}
func questEdgeLabel(row map[string]any) string {
	return fmt.Sprintf("%v → %v (%v, %v)", row["fromNode"], row["toNode"], row["label"], row["id"])
}
func questLineDiff(where, a, b string) []string {
	var changes []string
	left, right := strings.Split(a, "\n"), strings.Split(b, "\n")
	shared := make([][]int, len(left)+1)
	for i := range shared {
		shared[i] = make([]int, len(right)+1)
	}
	equal := func(x, y string) bool { return x == y || normalizeQuestBody(x) == normalizeQuestBody(y) }
	for i := len(left) - 1; i >= 0; i-- {
		for j := len(right) - 1; j >= 0; j-- {
			shared[i][j] = max(shared[i+1][j], shared[i][j+1])
			if equal(left[i], right[j]) {
				shared[i][j] = 1 + shared[i+1][j+1]
			}
		}
	}
	for i, j := 0, 0; i < len(left) || j < len(right); {
		switch {
		case i < len(left) && j < len(right) && equal(left[i], right[j]):
			i++
			j++
		case i < len(left) && j < len(right) && shared[i][j] == shared[i+1][j+1]:
			label := "Изменена строка"
			if strings.HasPrefix(strings.TrimSpace(left[i]), "если:") {
				label = "Изменено условие"
			}
			if strings.HasPrefix(strings.TrimSpace(left[i]), "эффект:") {
				label = "Изменён эффект"
			}
			changes = append(changes, fmt.Sprintf("%s %s: %s → %s", label, where, strings.TrimSpace(left[i]), strings.TrimSpace(right[j])))
			i++
			j++
		case i < len(left) && (j == len(right) || shared[i+1][j] >= shared[i][j+1]):
			changes = append(changes, "Убрано "+where+": "+strings.TrimSpace(left[i]))
			i++
		default:
			changes = append(changes, "Добавлено "+where+": "+strings.TrimSpace(right[j]))
			j++
		}
	}
	return changes
}
