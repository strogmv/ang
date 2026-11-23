package gamepoc

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"cuelang.org/go/cue"
)

type storyActTrigger struct {
	Fact string          `json:"fact,omitempty"`
	Step *storyQuestStep `json:"quest,omitempty"`
}

type storyAct struct {
	ID      string           `json:"id"`
	Name    string           `json:"name"`
	Order   int              `json:"order"`
	Begins  *storyActTrigger `json:"begins,omitempty"`
	Ends    *storyActTrigger `json:"ends,omitempty"`
	Origins []string         `json:"origins,omitempty"`
}

type storyActBoundary struct {
	Act  string           `json:"act,omitempty"`
	When *storyActTrigger `json:"when,omitempty"`
}

type storyActReturn struct {
	After   string           `json:"after,omitempty"`
	Mode    string           `json:"mode"`
	Scene   string           `json:"scene,omitempty"`
	Entry   string           `json:"entry,omitempty"`
	When    *storyActTrigger `json:"when,omitempty"`
	Message string           `json:"message,omitempty"`
}

type storyActFate struct {
	Mode string `json:"mode"`
	At   string `json:"at,omitempty"`
	Step string `json:"step,omitempty"`
	Fact string `json:"fact,omitempty"`
}

type storyActCompletion struct {
	Step  string `json:"step"`
	Place string `json:"place"`
}

type storyActResidence struct {
	Act         string               `json:"act"`
	Name        string               `json:"name,omitempty"`
	Place       string               `json:"place,omitempty"`
	Opens       storyActBoundary     `json:"opens"`
	Closes      *storyActBoundary    `json:"closes,omitempty"`
	Fate        storyActFate         `json:"fate"`
	Return      storyActReturn       `json:"return"`
	Completions []storyActCompletion `json:"completions,omitempty"`
	Finished    []string             `json:"finished,omitempty"`
}

type storyActRoute struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Opens  string `json:"opens,omitempty"`
	Closes string `json:"closes,omitempty"`
}

type storyActs struct {
	Acts    []storyAct                   `json:"acts"`
	Default storyActResidence            `json:"default"`
	Quests  map[string]storyActResidence `json:"quests"`
	Scenes  map[string]storyActResidence `json:"scenes"`
	Entries map[string]storyActResidence `json:"entries"`
	Places  map[string]storyActResidence `json:"places"`
	Routes  []storyActRoute              `json:"routes,omitempty"`
}

func (a *storyActs) order(id string) int {
	for _, act := range a.Acts {
		if act.ID == id {
			return act.Order
		}
	}
	return 0
}

func (a *storyActs) name(id string) string {
	for _, act := range a.Acts {
		if act.ID == id {
			return act.Name
		}
	}
	return id
}

func storyResidence(rows map[string]storyActResidence, id string, def storyActResidence) storyActResidence {
	if row, ok := rows[id]; ok {
		return row
	}
	return def
}

func readStoryActs(world cue.Value, c *Compiled) error {
	value := world.LookupPath(cue.ParsePath("story_acts"))
	if !value.Exists() {
		return nil
	}
	var a storyActs
	if err := decodeStrict(value, "world.story_acts", &a); err != nil {
		return err
	}
	slices.SortFunc(a.Acts, func(x, y storyAct) int { return x.Order - y.Order })
	c.StoryActs = &a
	return nil
}

func checkStoryActs(c *Compiled) ([]string, error) {
	a, m := c.StoryActs, c.Memory
	if a == nil {
		return nil, nil
	}
	if m == nil || len(a.Acts) == 0 {
		return nil, fmt.Errorf("Актам нужны память сюжета и первый акт")
	}
	var locations struct{ Locations map[string]json.RawMessage }
	if len(c.Life) > 0 {
		if err := json.Unmarshal(c.Life, &locations); err != nil {
			return nil, err
		}
	}
	knownAct := func(id string) bool { return id != "" && a.order(id) > 0 }
	isStep := func(id, step string) bool { return slices.Contains(m.Quests[id], step) }
	trigger := func(p *storyActTrigger) error {
		if p == nil {
			return nil
		}
		switch {
		case p.Fact != "" && p.Step == nil:
			if m.Facts[p.Fact] == nil {
				return fmt.Errorf("неизвестный факт %s", p.Fact)
			}
			if c.memoryLinks != nil && len(c.memoryLinks.factWriters[p.Fact]) == 0 {
				return fmt.Errorf("факт %s никто не записывает", p.Fact)
			}
		case p.Fact == "" && p.Step != nil:
			if !isStep(p.Step.ID, p.Step.Step) {
				return fmt.Errorf("нет шага %s у задания %s", p.Step.Step, p.Step.ID)
			}
		default:
			return fmt.Errorf("укажите один факт или один шаг задания")
		}
		return nil
	}
	for id := range m.Quests {
		if strings.HasPrefix(id, "sq.") && a.Quests[id].Fate.Mode == "" {
			return nil, fmt.Errorf("задание %s: укажите судьбу при смене акта", id)
		}
	}
	seen := map[string]bool{}
	for i, act := range a.Acts {
		if act.ID == "" || strings.TrimSpace(act.Name) == "" || act.Order != i+1 || seen[act.ID] {
			return nil, fmt.Errorf("Акт %q: нужны уникальные название и номер акта без пропусков", act.ID)
		}
		seen[act.ID] = true
		for _, p := range []*storyActTrigger{act.Begins, act.Ends} {
			if err := trigger(p); err != nil {
				return nil, fmt.Errorf("%s: %w", act.Name, err)
			}
		}
		for _, place := range act.Origins {
			if locations.Locations[place] == nil {
				return nil, fmt.Errorf("%s: неизвестная точка входа %s", act.Name, place)
			}
		}
	}
	for _, route := range a.Routes {
		if locations.Locations[route.From] == nil || locations.Locations[route.To] == nil {
			return nil, fmt.Errorf("Путь %s → %s: неизвестное место", route.From, route.To)
		}
		if route.Opens != "" && !knownAct(route.Opens) || route.Closes != "" && !knownAct(route.Closes) {
			return nil, fmt.Errorf("Путь %s → %s: неизвестный акт", route.From, route.To)
		}
		if route.Closes != "" && a.order(route.Closes) <= a.order(route.Opens) {
			return nil, fmt.Errorf("Путь %s → %s закрыт всё окно", route.From, route.To)
		}
	}
	var pending []string
	for _, spec := range []struct {
		label string
		rows  map[string]storyActResidence
	}{
		{"по умолчанию", map[string]storyActResidence{"*": a.Default}}, {"задание", a.Quests},
		{"сцена", a.Scenes}, {"вход", a.Entries}, {"место", a.Places},
	} {
		for _, id := range sortedKeys(spec.rows) {
			r := spec.rows[id]
			label := spec.label + " " + id
			if !knownAct(r.Act) || r.Opens.Act != "" && !knownAct(r.Opens.Act) {
				return nil, fmt.Errorf("%s: укажите существующий акт и окно", label)
			}
			if err := trigger(r.Opens.When); err != nil {
				return nil, fmt.Errorf("%s открывается: %w", label, err)
			}
			if r.Closes != nil {
				if r.Closes.Act != "" && !knownAct(r.Closes.Act) || r.Closes.Act == "" && r.Closes.When == nil {
					return nil, fmt.Errorf("%s: у закрытия нет акта или условия", label)
				}
				if err := trigger(r.Closes.When); err != nil {
					return nil, fmt.Errorf("%s закрывается: %w", label, err)
				}
				if r.Closes.Act != "" && a.order(r.Closes.Act) <= a.order(r.Opens.Act) && r.Closes.When == nil {
					return nil, fmt.Errorf("%s: окно закрывается до открытия", label)
				}
			}
			f := r.Fate
			switch f.Mode {
			case "carry":
			case "unresolved":
				pending = append(pending, label+" — судьба при смене акта не решена")
			case "fail", "expire":
				if spec.label != "задание" || !knownAct(f.At) || !isStep(id, f.Step) || len(r.Finished) == 0 {
					return nil, fmt.Errorf("%s: для завершения на границе нужны акт, шаг исхода и список завершённых шагов", label)
				}
				if fact := m.Facts[f.Fact]; fact == nil || fact.Journal == "" || m.Journal == nil || len(fact.Args) != 0 {
					return nil, fmt.Errorf("%s: для истечения или провала нужен факт с текстом в журнале", label)
				}
			default:
				return nil, fmt.Errorf("%s: укажите судьбу при смене акта", label)
			}
			for _, terminal := range r.Finished {
				if !isStep(id, terminal) {
					return nil, fmt.Errorf("%s: нет завершённого шага %s", label, terminal)
				}
			}
			back := r.Return
			if back.After != "" && !knownAct(back.After) {
				return nil, fmt.Errorf("%s: неизвестный акт возвращения", label)
			}
			if err := trigger(back.When); err != nil {
				return nil, fmt.Errorf("%s на возвращении: %w", label, err)
			}
			switch back.Mode {
			case "same":
			case "unresolved":
				pending = append(pending, label+" — возвращение не решено")
			case "changed":
				dialogue, _, _ := strings.Cut(id, "/")
				if back.Scene != "" {
					dialogue = back.Scene
				}
				conversation := c.Conversations[dialogue]
				if back.After == "" || back.Entry == "" || conversation == nil || !slices.ContainsFunc(conversation.Entries, func(e ConvEntry) bool { return e.ID == back.Entry }) {
					return nil, fmt.Errorf("%s: у изменённого возвращения нужен вход с репликами после указанного акта", label)
				}
			case "gone":
				if back.After == "" || !slices.ContainsFunc(c.Localization.Messages, func(msg LocalizationMessage) bool {
					return msg.Key == back.Message && msg.Texts["ru"] != "" && msg.Texts["en"] != "" && msg.Context != ""
				}) {
					return nil, fmt.Errorf("%s: для исчезновения нужна локализованная подсказка игроку", label)
				}
			default:
				return nil, fmt.Errorf("%s: укажите, что герой увидит при возвращении", label)
			}
			afterReturn := back.After
			if afterReturn == "" && len(a.Acts) >= 3 {
				afterReturn = a.Acts[2].ID
			}
			if back.Mode == "unresolved" && spec.label == "сцена" && r.Act == a.Acts[0].ID && r.Place != "" &&
				afterReturn != "" && a.canReachAfter(afterReturn, r.Place) {
				return nil, fmt.Errorf("%s: до сцены можно вернуться. Укажите «как прежде», новый вариант или исчезновение", label)
			}
			if r.Place != "" && locations.Locations[r.Place] == nil {
				return nil, fmt.Errorf("%s: неизвестное место %s", label, r.Place)
			}
			for _, step := range r.Completions {
				if !isStep(id, step.Step) || locations.Locations[step.Place] == nil {
					return nil, fmt.Errorf("%s: неизвестный шаг или место завершения", label)
				}
				if !a.canComplete(r, step.Place) {
					return nil, fmt.Errorf("%s: шаг «%s» можно завершить только в %s, куда герой не попадёт в это окно", label, step.Step, step.Place)
				}
			}
		}
	}
	return pending, nil
}

func (a *storyActs) canComplete(r storyActResidence, place string) bool {
	for _, act := range a.Acts {
		if r.Opens.Act != "" && a.order(r.Opens.Act) > act.Order ||
			r.Closes != nil && r.Closes.Act != "" && r.Closes.When == nil && a.order(r.Closes.Act) <= act.Order {
			continue
		}
		graph := map[string][]string{}
		for _, e := range a.Routes {
			if (e.Opens == "" || a.order(e.Opens) <= act.Order) && (e.Closes == "" || a.order(e.Closes) > act.Order) {
				graph[e.From] = append(graph[e.From], e.To)
			}
		}
		if questWalk(graph, act.Origins)[place] {
			return true
		}
	}
	return false
}

func (a *storyActs) canReachAfter(from, place string) bool {
	return a.canComplete(storyActResidence{Opens: storyActBoundary{Act: from}}, place)
}
