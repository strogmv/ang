package gamepoc

import (
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/questnotation"
)

func TestQuestStoryDeterministicAndInsertionKeepsIDs(t *testing.T) {
	s := questStory{ID: "dlg.sample", Title: "Проверка"}
	s.card("Start", "dispatch", []string{"вход: [[e.usual]]"})
	s.card("main", "beat", []string{"ответ: [[answer.leave]]"})
	s.card("e.usual", "entry", []string{"войти: [[main]]"})
	s.card("answer.leave", "answer", []string{"Герой: До свидания.  ‹line.bye›", "переход: [[end.bye]]"})
	s.card("end.bye", "close", []string{"Итог разговора: bye"})
	first := s.render()
	if first != s.render() {
		t.Fatal("same graph produced different bytes")
	}
	s.card("another", "beat", []string{"ответ: [[answer.leave]]"})
	previous, err := parseQuestStory(first)
	if err != nil {
		t.Fatal(err)
	}
	next, err := parseQuestStory(s.render())
	if err != nil {
		t.Fatal(err)
	}
	for id, page := range previous {
		if page.body != next[id].body {
			t.Errorf("inserting another node changed the identity or contents of %s", id)
		}
	}
	if questIFID("dlg.sample") == questIFID("dlg.other") {
		t.Fatal("different conversation has the same IFID")
	}
}

func TestQuestCompareLayoutAndSignificantFields(t *testing.T) {
	s := questStory{ID: "dlg.sample", Title: "Проверка"}
	s.card("Start", "dispatch", []string{"если: ((факт = fact.a.b) ИЛИ (нет факта = fact.a.c))", "Герой: Слово  ‹line.bye›"})
	before := map[string]string{"sample.twee": s.render(), "act-one-chain.canvas": `{"nodes":[{"id":"a","type":"text","text":"A","x":12}],"edges":[]}`}
	edited := map[string]string{
		"sample.twee":          strings.ReplaceAll(before["sample.twee"], "80,80", "12,42"),
		"act-one-chain.canvas": strings.ReplaceAll(before["act-one-chain.canvas"], `"x":12`, `"x":42`),
	}
	delta, err := CompareQuestViews(before, edited)
	if err != nil || len(delta) != 0 {
		t.Fatalf("layout change: %v %v", delta, err)
	}
	edited["sample.twee"] = strings.ReplaceAll(edited["sample.twee"], "fact.a.c", "fact.a.d")
	delta, err = CompareQuestViews(before, edited)
	if err != nil || len(delta) != 1 || !strings.Contains(delta[0], "Изменено условие") {
		t.Fatalf("condition change: %v %v", delta, err)
	}
	edited["sample.twee"] = strings.ReplaceAll(before["sample.twee"], "Слово", "Другое слово")
	delta, err = CompareQuestViews(before, edited)
	if err != nil || len(delta) != 1 || !strings.Contains(delta[0], "Изменена строка") {
		t.Fatalf("line change: %v %v", delta, err)
	}
}

func TestQuestConditionsAndEffectsRoundTrip(t *testing.T) {
	q := questExport{glossary: map[string]questTerm{"char.varya": {Name: "Варя"}}}
	source := `{"knows":{"who":"char.varya","fact":"fact.varya.met","at_least":0.5},"any":[{"fact":"fact.varya.met"},{"not":{"not_topic":"topic.goods"}}]}`
	var before map[string]any
	if err := questJSON([]byte(source), &before); err != nil {
		t.Fatal(err)
	}
	readable := q.condition(before)
	after, err := parseQuestCondition(readable)
	if err != nil || !questRawEqual(before, after) {
		t.Fatalf("%s: %v %v", readable, after, err)
	}
	var effect map[string]any
	if err := questJSON([]byte(`{"trust":{"who":"char.varya","delta":-0.1},"when":{"fact":"fact.varya.met"}}`), &effect); err != nil {
		t.Fatal(err)
	}
	readable = q.fields(effect)
	after, err = parseQuestFields(readable, " · ")
	if err != nil || !questRawEqual(effect, after) {
		t.Fatalf("%s: %v %v", readable, after, err)
	}
}

func TestQuestSentencesAreReversible(t *testing.T) {
	terms := map[string][]string{
		"self": {"роль", "Варя"}, "sq.demo": {"квест", "Рассада"}, "fact.first": {"факт", "У лотка пахнет землёй."},
		"fact.other": {"факт", "Кассета опломбирована."}, "item.box": {"предмет", "опломбированная кассета"},
		"place.demo": {"место", "пакет земли"},
	}
	w := newQuestWords(terms)
	format := questnotation.Formatter(func(id string) string { return w.names[id] })
	for _, tc := range []struct {
		source string
		effect bool
	}{
		{`{"any":[{"quest":{"id":"sq.demo","step":"done"}},{"trust":{"who":"self","at_least":0.1}}]}`, false},
		{`{"not":{"not_fact":"fact.first","item":{"item":"item.box","held":true,"owner":"hero"}}}`, false},
		{`{"fact":{"id":"fact.first","args":{"by":"someone"}}}`, false},
		{`{"transfer":{"to":"hero","placement":"place.demo","pay_crd_minor":300},"when":{"fact":{"id":"fact.other"}}}`, true},
		{`{"trust":{"who":"self","delta":0.1}}`, true},
		{`{"trust":{"who":"self","delta":-0.1}}`, true},
	} {
		var source map[string]any
		if err := questJSON([]byte(tc.source), &source); err != nil {
			t.Fatal(err)
		}
		var phrase string
		read := w.condition
		if tc.effect {
			phrase, read = format.Effect(source), w.effect
		} else {
			phrase = format.Sentence(source)
		}
		got, err := read(phrase)
		if err != nil || !questRawEqual(source, got) {
			t.Fatalf("%s: got %v (%v), want %s", phrase, got, err, tc.source)
		}
	}
	w.add("fact.ambiguous", "У лотка пахнет землёй.")
	if _, err := w.condition("известно: «У лотка пахнет землёй.»"); err == nil || !strings.Contains(err.Error(), "Уточните название") {
		t.Fatalf("ambiguous name was accepted: %v", err)
	}
}

func TestQuestStoryRequiresItsDeclaredStart(t *testing.T) {
	story := questStory{ID: "dlg.proof", Title: "Проверка"}
	story.card("Start", "dispatch", []string{"Начало"})
	legacy := story.render()
	missing := strings.Replace(legacy, ":: Start ", ":: Lost ", 1)
	if _, err := parseQuestStory(missing); err == nil {
		t.Fatal("accepted a story whose initial passage was removed")
	}
	renamed := strings.Replace(legacy, ":: Start [dispatch]", ":: О сцене [id.Start вход]", 1)
	renamed = strings.Replace(renamed, `"start":"Start"`, `"start":"О сцене"`, 1)
	pages, err := parseQuestStory(renamed)
	if err != nil || passageID(pages["О сцене"].tags, "id.") != "Start" {
		t.Fatalf("renamed v2 start lost its identity: %v", err)
	}
	missing = strings.Replace(renamed, ":: О сцене ", ":: Потеряна ", 1)
	if _, err := parseQuestStory(missing); err == nil {
		t.Fatal("accepted a v2 story whose initial passage was removed")
	}
}
