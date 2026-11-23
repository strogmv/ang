package gamepoc

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

func questActFolder(a *storyActs, id string) string {
	if a == nil {
		return ""
	}
	res := storyResidence(a.Scenes, id, a.Default)
	return fmt.Sprintf("акт-%d/", a.order(res.Act))
}

type questPlannedRow struct {
	ID            string           `json:"id"`
	Act           string           `json:"act"`
	Quest         string           `json:"quest_id"`
	Title         questTranslation `json:"title"`
	Notes         string           `json:"notes_author_ru"`
	Prerequisites []string         `json:"prerequisites"`
}

type questPlanned struct {
	Quests map[string]questPlannedRow
	Scenes map[string]questPlannedRow
}

func readQuestPlan(root string) (questPlanned, error) {
	var out questPlanned
	value, err := cuePackage(filepath.Join(root, "content/world/planning/story_arc"))
	if err != nil {
		return out, err
	}
	for path, into := range map[string]any{"catalog.quests": &out.Quests, "catalog.scenes": &out.Scenes} {
		raw, err := questRaw(value, path)
		if err != nil {
			return out, err
		}
		b, err := json.Marshal(raw)
		if err != nil {
			return out, err
		}
		if err := json.Unmarshal(b, into); err != nil {
			return out, err
		}
	}
	return out, nil
}

func questOfficeList(names []string) string {
	if len(names) == 0 {
		return "формального условия нет"
	}
	return strings.Join(names, "; ")
}

func questPolicyParts(a *storyActs, p storyActResidence, name func(string) string) []string {
	at := func(b storyActBoundary) string {
		parts := []string{}
		if b.Act != "" {
			parts = append(parts, a.name(b.Act))
		}
		if b.When != nil {
			if b.When.Fact != "" {
				parts = append(parts, "записан факт «"+name(b.When.Fact)+"»")
			}
			if b.When.Step != nil {
				parts = append(parts, "задание «"+name(b.When.Step.ID)+"» на шаге «"+name(b.When.Step.Step)+"»")
			}
		}
		return questOfficeList(parts)
	}
	closes := "без верхней границы"
	if p.Closes != nil {
		closes = at(*p.Closes)
	}
	fate := map[string]string{"carry": "остаётся доступно", "fail": "провал", "expire": "срок истекает", "unresolved": "не решено"}[p.Fate.Mode]
	if p.Fate.At != "" {
		fate += " при переходе в " + a.name(p.Fate.At)
	}
	back := map[string]string{"same": "как прежде", "changed": "другой вход и реплики", "gone": "исчезает; герой получит объяснение", "unresolved": "не решено"}[p.Return.Mode]
	if p.Return.After != "" {
		back += " · " + a.name(p.Return.After)
	}
	return []string{a.name(p.Act), at(p.Opens), closes, fate, back}
}

func questPolicySentence(a *storyActs, p storyActResidence, name func(string) string) string {
	parts := questPolicyParts(a, p, name)
	return fmt.Sprintf("%s. Открытие — %s. Закрытие — %s. При смене акта — %s. При возвращении — %s.",
		parts[0], parts[1], parts[2], parts[3], parts[4])
}

func (q *questExport) emitActs(a *storyActs, plan questPlanned, m *Memory, convs map[string]*Conversation, sources questSource, scripts map[string]string) {
	if a == nil {
		q.files["act-one-chain.canvas"] = q.canvas.render()
		return
	}
	maps := map[string]*questCanvas{a.Default.Act: &q.canvas}
	overview := questCanvas{}
	for _, act := range a.Acts {
		if maps[act.ID] == nil {
			maps[act.ID] = &questCanvas{}
		}
		folder := fmt.Sprintf("акт-%d", act.Order)
		boundaries := "Границы акта требуют авторского решения."
		if act.Begins != nil || act.Ends != nil {
			boundaries = "Границы заданы фактом или шагом в данных сюжета."
		}
		overview.note(act.ID, act.Name+"\n\n"+boundaries+"\n\n[Цепочки]("+folder+"/Цепочки.canvas)", "4")
		if act.Order > 1 {
			overview.edge(a.Acts[act.Order-2].ID, act.ID, "следующий акт")
		}
	}
	name := func(id string) string {
		if n := q.glossary[id].Name; n != "" {
			return n
		}
		if n := plan.Quests[id].Title.Texts["ru"]; n != "" {
			return n
		}
		return id
	}
	var table strings.Builder
	table.WriteString("# Сводка по актам\n\nОкна действующих сцен и существующие планы из CUE. «Не решено» оставляет привычную работу первого акта.\n\n")
	table.WriteString("| Задание или сцена | Акт | Открывается | Закрывается | При смене акта | При возвращении |\n| --- | --- | --- | --- | --- | --- |\n")
	add := func(title string, res storyActResidence) {
		fmt.Fprintf(&table, "| %s | %s |\n", strings.ReplaceAll(title, "|", "\\|"), strings.Join(questPolicyParts(a, res, name), " | "))
	}
	planAct := func(id string) string {
		if id == "act1" {
			return a.Default.Act
		}
		return id
	}
	planPolicy := func(act, title string) storyActResidence {
		act = planAct(act)
		res := a.Default
		res.Act, res.Opens.Act, res.Name = act, act, title
		if act != a.Default.Act {
			res.Return.After = ""
		}
		return res
	}
	for _, id := range sortedKeys(m.Quests) {
		r := storyResidence(a.Quests, id, a.Default)
		title := r.Name
		if title == "" {
			title = name(id)
		}
		add(title, r)
		if r.Return.After != "" || r.Closes != nil && r.Closes.Act != r.Act {
			overview.note(id, title+"\n\n"+questPolicySentence(a, r, name), "5")
			overview.edge(r.Act, id, "задание")
			if r.Return.After != "" {
				overview.edge(id, r.Return.After, "возвращение")
			}
		}
	}
	for _, id := range sortedKeys(plan.Quests) {
		row := plan.Quests[id]
		title := row.Title.Texts["ru"]
		res := planPolicy(row.Act, title)
		if !slices.Contains(sortedKeys(m.Quests), id) {
			add(title+" · план", res)
		}
		canvas := maps[res.Act]
		if canvas == nil {
			continue
		}
		canvas.note(id, title+"\n\nПо плану. "+questPolicySentence(a, res, name), "6")
		for _, prior := range row.Prerequisites {
			if planAct(plan.Quests[prior].Act) == res.Act {
				canvas.edge(prior, id, "по плану")
			} else {
				overview.edge(planAct(plan.Quests[prior].Act), res.Act, "по плану: "+plan.Quests[prior].Title.Texts["ru"]+" → "+title)
			}
		}
	}
	for _, id := range sortedKeys(plan.Scenes) {
		row := plan.Scenes[id]
		res := planPolicy(plan.Quests[row.Quest].Act, row.Title.Texts["ru"])
		add(row.Title.Texts["ru"]+" · план", res)
		canvas := maps[res.Act]
		if canvas != nil {
			canvas.note(id, row.Title.Texts["ru"]+"\n\nПо плану. "+questPolicySentence(a, res, name), "3")
			canvas.edge(row.Quest, id, "сцена в задании")
		}
	}
	for _, id := range sortedKeys(scripts) {
		if sources.count[id] == 0 {
			add(sources.title[id]+" · сценарий", a.Default)
		}
	}
	for _, id := range sortedKeys(convs) {
		res := storyResidence(a.Scenes, id, a.Default)
		title := res.Name
		if title == "" {
			title = sources.title[sources.conversations[id]]
			if title == "" {
				title = id
			}
		}
		add(title, res)
		for _, entry := range convs[id].Entries {
			if policy, ok := a.Entries[id+"/"+entry.ID]; ok {
				add(title+" · вход «"+entry.ID+"»", policy)
			}
		}
		if res.Return.Mode == "changed" || res.Return.Mode == "same" || res.Return.Mode == "gone" {
			overview.note(id, title+"\n\n"+questPolicySentence(a, res, name), "2")
			overview.edge(res.Act, id, "сцена")
			if res.Return.After != "" {
				overview.edge(id, res.Return.After, "возвращение")
			}
		}
	}
	for _, act := range a.Acts {
		folder := fmt.Sprintf("акт-%d", act.Order)
		canvas := maps[act.ID].render()
		// Existing links were written relative to the root of the workspace.
		canvas = strings.NewReplacer("(../design/", "(../../design/", "(world/", "(../world/", "("+folder+"/", "(").Replace(canvas)
		q.files[folder+"/Цепочки.canvas"] = canvas
	}
	q.files["Акты.canvas"] = overview.render()
	q.files["Сводка.md"] = table.String()
}

func (q *questExport) emitReadableGlossary() {
	groups := map[string][]string{}
	use := map[string][]string{}
	for _, path := range sortedKeys(q.files) {
		if !strings.HasSuffix(path, ".twee") {
			continue
		}
		pages, err := parseQuestStory(q.files[path])
		if err != nil {
			continue
		}
		for _, title := range sortedKeys(pages) {
			if title == "StoryTitle" || title == "StoryData" {
				continue
			}
			for id, term := range q.glossary {
				card := pages[title]
				if strings.Contains(card.body, "«"+questCleanName(term.Name)+"»") ||
					strings.Contains(card.body, strings.ToUpper(term.Name)+":") ||
					strings.Contains(card.body, id+" «") {
					use[id] = append(use[id], fmt.Sprintf("[%s · %s](%s)", strings.TrimSuffix(path, ".twee"), title, path))
				}
			}
		}
	}
	for _, id := range sortedKeys(q.glossary) {
		term := q.glossary[id]
		line := fmt.Sprintf("- **%s** · %s. Код `%s`.", term.Name, strings.TrimRight(term.Meaning, "."), id)
		if rows := use[id]; len(rows) > 0 {
			line += " Встречается в " + strings.Join(rows, ", ") + "."
		}
		groups[term.Kind] = append(groups[term.Kind], line)
	}
	var b strings.Builder
	b.WriteString("# Словарь сцен\n\nНазвания из этого списка можно вставлять в условия и последствия.\n")
	for _, kind := range sortedKeys(groups) {
		rows := groups[kind]
		slices.Sort(rows)
		fmt.Fprintf(&b, "\n## %s\n\n%s\n", kind, strings.Join(rows, "\n"))
	}
	q.files["Словарь.md"] = b.String()
}
