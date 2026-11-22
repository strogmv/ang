package gamepoc

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"

	"cuelang.org/go/cue"
)

type questCanvas struct {
	nodes []map[string]any
	edges []map[string]any
}

func (c *questCanvas) note(id, text, color string) {
	for _, node := range c.nodes {
		if node["id"] == id {
			return
		}
	}
	col := map[string]int{"3": 0, "4": 1, "2": 2, "5": 3, "6": 4, "1": 5}[color]
	row := 0
	for _, node := range c.nodes {
		if node["color"] == color {
			row++
		}
	}
	c.nodes = append(c.nodes, map[string]any{"id": id, "type": "text", "text": text, "color": color,
		"x": col * 510, "y": row * 170, "width": 450, "height": 145})
}
func (c *questCanvas) edge(from, to, label string) {
	id := QuestEdgeID(from, to, label)
	for _, edge := range c.edges {
		if edge["id"] == id {
			return
		}
	}
	c.edges = append(c.edges, map[string]any{"id": id, "fromNode": from, "toNode": to, "label": label, "fromSide": "right", "toSide": "left"})
}
func QuestEdgeID(from, to, label string) string {
	digest := sha256.Sum256([]byte(from + "\n" + to + "\n" + label))
	return fmt.Sprintf("edge.%x", digest[:12])
}
func (c questCanvas) render() string {
	var b strings.Builder
	write := func(rows []map[string]any) {
		slices.SortFunc(rows, func(a, b map[string]any) int { return strings.Compare(a["id"].(string), b["id"].(string)) })
		for i, row := range rows {
			data, _ := json.Marshal(row)
			if i > 0 {
				b.WriteString(",\n")
			}
			fmt.Fprintf(&b, "    %s", data)
		}
	}
	b.WriteString("{\n  \"nodes\": [\n")
	write(c.nodes)
	b.WriteString("\n  ],\n  \"edges\": [\n")
	write(c.edges)
	b.WriteString("\n  ]\n}\n")
	return b.String()
}

const questScriptReading = "Запланированное чтение"
const questScriptWriting = "Запланированная запись"

func (q *questExport) scriptCards(scripts map[string]string, sources questSource) {
	for _, id := range sortedKeys(scripts) {
		status := "Ещё нет в игре"
		switch {
		case sources.count[id] > 0:
			status = "Есть игровые карточки. Полнота переноса описана в сценарии."
		case hasQuestScript(sources.interjections, id):
			status = "Частично в игре: вставки."
		}
		q.canvas.note("script."+id, sources.title[id]+"\n\n"+status+"\n\n[Сценарий](../design/dialogue/scripts/"+scripts[id]+")", "3")
	}
	for _, talk := range sortedKeys(sources.conversations) {
		q.canvas.edge("script."+sources.conversations[talk], talk, "Игровые карточки")
	}
}

func hasQuestScript(sources map[string]string, id string) bool {
	for _, script := range sources {
		if script == id {
			return true
		}
	}
	return false
}

func (q *questExport) worldCards(world cue.Value, m *Memory, sources questSource) error {
	s := questStory{ID: "world.memory", Title: "Мир · факты, встречи, сроки и журнал"}
	var dispatch []string
	card := func(id, title string, raw any, skip ...string) error {
		var lines []string
		switch raw := raw.(type) {
		case map[string]any:
			for _, field := range skip {
				delete(raw, field)
			}
			lines = q.propertyFields(raw)
		default:
			var err error
			lines, err = q.takeProperties(raw, skip...)
			if err != nil {
				return err
			}
		}
		if len(lines) == 0 {
			lines = []string{"Запись: " + id}
		}
		s.card(id, "property", lines)
		dispatch = append(dispatch, "запись: "+questLink(id))
		q.subjects[id] = id
		q.canvas.note(id, title+"\n\n[Запись мира](world/scene.twee)", "5")
		return nil
	}
	for _, id := range sortedKeys(m.Facts) {
		f := m.Facts[id]
		obj, err := questObject(f)
		if err != nil {
			return err
		}
		filter := func(key string, rows []string, writing bool) {
			var rest []any
			for _, row := range rows {
				num, _, _ := strings.Cut(row, ":")
				if memoryScriptID.MatchString(num) && (writing || row == num) {
					if writing {
						q.canvas.edge("script."+num, id, questScriptWriting+strings.TrimPrefix(row, num))
					} else {
						q.canvas.edge(id, "script."+num, questScriptReading)
					}
				} else {
					rest = append(rest, row)
				}
			}
			obj[key] = rest
		}
		filter("external_readers", f.ExternalReaders, false)
		filter("external_writers", f.ExternalWriters, true)
		if err := card(id, id+"\n"+q.glossary[id].Meaning, obj); err != nil {
			return err
		}
	}
	for _, id := range sortedKeys(m.Topics) {
		obj, err := questObject(m.Topics[id])
		if err != nil {
			return err
		}
		var rest []any
		for _, row := range m.Topics[id].ExternalReaders {
			if memoryScriptID.MatchString(row) {
				q.canvas.edge(id, "script."+row, questScriptReading)
			} else {
				rest = append(rest, row)
			}
		}
		obj["external_readers"] = rest
		if err := card(id, id, obj); err != nil {
			return err
		}
	}
	for _, id := range sortedKeys(m.Quests) {
		if err := card(id, id, map[string]any{"steps": m.Quests[id]}); err != nil {
			return err
		}
	}
	for _, rule := range m.Rules {
		id := "rule." + rule.ID
		if err := card(id, "Правило мира · "+rule.ID, rule, "id"); err != nil {
			return err
		}
		q.subjects["world.memory.rules."+rule.ID] = id
	}
	for _, gathering := range m.Gatherings {
		obj, err := questObject(gathering)
		if err != nil {
			return err
		}
		// Gatherings lack an authored ID. Use the immutable locator (level + people), not the array index or coordinates.
		level := obj["level"].(string)
		digest := sha256.Sum256([]byte(level + "\n" + strings.Join(sortedKeys(obj["positions"].(map[string]any)), "\n")))
		id := fmt.Sprintf("meeting.%x", digest[:8])
		if q.subjects[id] != "" {
			return fmt.Errorf("две встречи %s с теми же участниками требуют устойчивого ID", level)
		}
		if err := card(id, "Встреча · "+level, obj); err != nil {
			return err
		}
	}
	for _, level := range sortedKeys(m.AutoTalks) {
		for _, row := range m.AutoTalks[level] {
			id := "auto." + level + "." + row.Conversation
			if err := card(id, "Вход по месту · "+row.Conversation, row); err != nil {
				return err
			}
			q.canvas.edge(id, row.Conversation, "Открывает разговор")
		}
	}
	for _, edge := range sortedKeys(m.Arrivals) {
		id := "arrival." + edge
		if err := card(id, "Последствия прибытия · "+edge, m.Arrivals[edge]); err != nil {
			return err
		}
		q.subjects["world.memory.arrivals."+edge] = id
	}
	for _, row := range m.Interjections {
		id := row.ID
		obj, err := questObject(row)
		if err != nil {
			return err
		}
		delete(obj, "id")
		first := q.line("Реплика "+q.phrase(row.Speaker), row.Text)
		delete(obj, "text")
		if err := card(id, "Вставка · "+id, obj); err != nil {
			return err
		}
		last := &s.Cards[len(s.Cards)-1]
		last.Lines = append([]string{first}, last.Lines...)
		if script := sources.interjections[id]; script != "" {
			q.canvas.edge("script."+script, id, "Игровая вставка")
		}
	}
	for _, entry := range []struct{ path, id, title string }{
		{"start", "world.start", "Начало новой игры"},
		{"calendar_encounter", "world.calendar_encounter", "Календарь Прохора"},
		{"memory.journal", "world.journal", "Журнал и слухи"},
		{"memory.trade", "world.trade", "Общие реплики торговли"},
		{"memory.interjection_rules", "world.interjections", "Настройки вставок"},
		{"memory.placement_versions", "world.placement_versions", "Версии размещения"},
		{"memory.backfill_placements", "world.backfill_placements", "Добавление объектов в старых сохранениях"},
		{"memory.migrate_placements", "world.migrate_placements", "Перенос объектов в сохранениях"},
	} {
		raw, err := questRaw(world, entry.path)
		if err != nil {
			return err
		}
		if raw == nil {
			continue
		}
		obj, ok := raw.(map[string]any)
		if !ok {
			obj = map[string]any{"value": raw}
		}
		if err := card(entry.id, entry.title, obj); err != nil {
			return err
		}
		if entry.id == "world.start" {
			q.canvas.edge(entry.id, obj["conversation"].(string), "Открывает разговор")
		}
		if entry.id == "world.calendar_encounter" && sources.title["01"] != "" {
			q.canvas.edge("script.01", entry.id, "Игровой календарь")
		}
	}
	s.card("Start", "dispatch", dispatch)
	q.files[path.Join("world", "scene.twee")] = s.render()
	return nil
}

func (q *questExport) storyLinks(links *memoryLinks) {
	source := func(label string) string {
		words := strings.Fields(label)
		for _, word := range words {
			id := strings.Trim(word, "():,")
			if q.subjects[id] != "" {
				return q.subjects[id]
			}
		}
		if strings.HasPrefix(label, "журнал ") {
			return q.subjects["world.journal"]
		}
		return ""
	}
	for _, group := range []struct {
		links map[string][]string
		write bool
	}{
		{links.factWriters, true}, {links.topicWriters, true}, {links.factReaders, false}, {links.topicReaders, false},
	} {
		for _, id := range sortedKeys(group.links) {
			for _, label := range group.links[id] {
				from := source(label)
				if from == "" {
					continue
				}
				if group.write {
					q.canvas.edge(from, id, "Записывает")
				} else {
					q.canvas.edge(id, from, "Читает")
				}
			}
		}
	}
}
