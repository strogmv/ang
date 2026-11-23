world: story_acts: {
	acts: [
		{id: "a1", name: "Первый", order: 1, origins: ["home"], ends: fact: "fact.left"},
		{id: "a2", name: "Второй", order: 2, origins: ["away"], begins: fact: "fact.left", ends: quest: {id: "test.main", step: "done"}},
		{id: "return", name: "Возвращение", order: 3, origins: ["away"], begins: quest: {id: "test.main", step: "done"}},
	]
	default: {act: "a1", opens: act: "a1", fate: mode: "carry", return: mode: "same"}
	quests: "test.side": {
		act: "a1"
		opens: act: "a1"
		closes: act: "a2"
		fate: {mode: "expire", at: "a2", step: "expired", fact: "fact.expired"}
		return: mode: "same"
		completions: [{step: "done", place: "home"}]
		finished: ["done", "expired"]
	}
	scenes: "dlg.fixture": {
		act: "a1"
		place: "home"
		opens: act: "a1"
		closes: act: "a2"
		fate: mode: "carry"
		return: {mode: "changed", after: "return", entry: "e.back", when: fact: "fact.expired"}
	}
	places: home: {act: "a1", opens: act: "a1", fate: mode: "carry", return: {mode: "same", after: "return"}}
	entries: {}
	routes: [{from: "away", to: "home", opens: "return"}]
}
