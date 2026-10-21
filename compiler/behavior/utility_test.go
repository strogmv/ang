package behavior

import (
	"strings"
	"testing"
)

const validUtilityCatalog = `{
 "facts":{"danger.known":{"id":"danger.known"},"danger.immediate":{"id":"danger.immediate"}},
 "signals":{"signal.near":{"id":"signal.near"}},
 "curves":{"curve.linear":{"id":"curve.linear","points":[{"x":0,"y":0},{"x":1,"y":1}]}},
 "tasks":{"task.root":{"id":"task.root","methods":[{"id":"method.done","steps":[]}]}},
 "goals":{
  "goal.flee":{"id":"goal.flee","intent":"flee","action":"flee","applicable":[{"fact":"danger.known","is":true}],"considerations":[{"input":"signal.near","curve":"curve.linear","weight":1,"need":"safety","emotion":"fear"}],"base":1,"hold_ticks":10,"stress_responses":["flight"]},
  "goal.safety":{"id":"goal.safety","intent":"withdraw","action":"flee","applicable":[{"fact":"danger.immediate","is":true}],"considerations":[{"input":"signal.near","curve":"curve.linear","weight":1}],"base":2,"hold_ticks":2,"interrupt":true},
  "goal.watch":{"id":"goal.watch","intent":"gather_watch","action":"investigate","plan":"task.root","applicable":[{"fact":"danger.known","is":true}],"considerations":[{"input":"signal.near","curve":"curve.linear","weight":0.5}],"base":1,"hold_ticks":10}
 },
 "profiles":{"profile.a":{"id":"profile.a","scene_goals":["goal.flee","goal.watch","goal.safety"],"forbid_actions":["flee"]}},
 "policy":{
  "utility":{"min_hold_ticks":5,"max_hold_ticks":100,"switch_margin":0.1,"max_goals_per_pass":8,"explain_top":3,"interrupt_facts":["danger.immediate"],"unknown_signal_value":0,"stress":{"signal":"signal.near","emotion":"fear"},"emotion_proxies":{"fear":"signal.near"}},
  "memory":{"capacity":4,"merge_window_ticks":5,"default_ttl_ticks":50,"priorities":{"incident":3},"unknown_threat_value":0.5}
 }
}`

func TestUtilityValidatorAcceptsCatalogAndRejectsEachBrokenReference(t *testing.T) {
	vocab := &Vocabulary{Needs: []string{"safety"}, Emotions: []string{"fear"}, Actions: []string{"flee", "investigate"}}
	if errs := ValidateUtility([]byte(validUtilityCatalog), vocab); len(errs) != 0 {
		t.Fatalf("valid catalog rejected: %v", errs)
	}
	cases := []struct{ name, from, to, path string }{
		{"unknown fact", `"fact":"danger.known","is":true}],"considerations":[{"input":"signal.near","curve":"curve.linear","weight":1,"need"`, `"fact":"danger.unknown","is":true}],"considerations":[{"input":"signal.near","curve":"curve.linear","weight":1,"need"`, `applicable[0].fact`},
		{"unknown signal", `"input":"signal.near","curve":"curve.linear","weight":0.5`, `"input":"signal.missing","curve":"curve.linear","weight":0.5`, `considerations[0].input`},
		{"unknown curve", `"curve":"curve.linear","weight":0.5`, `"curve":"curve.none","weight":0.5`, `considerations[0].curve`},
		{"missing weight", `"curve":"curve.linear","weight":0.5`, `"curve":"curve.linear"`, `considerations[0].weight`},
		{"unknown need", `"need":"safety"`, `"need":"glory"`, `.need`},
		{"unknown emotion", `"emotion":"fear"}`, `"emotion":"rage"}`, `.emotion`},
		{"unknown action", `"action":"investigate"`, `"action":"gossip"`, `.action`},
		{"unknown plan", `"plan":"task.root"`, `"plan":"task.none"`, `.plan`},
		{"unknown intent", `"intent":"gather_watch"`, `"intent":"wander"`, `.intent`},
		{"unknown profile goal", `"goal.flee","goal.watch","goal.safety"`, `"goal.flee","goal.lost","goal.safety"`, `scene_goals[1]`},
		{"unknown forbid action", `"forbid_actions":["flee"]`, `"forbid_actions":["dance"]`, `forbid_actions[0]`},
		{"profile forbids everything", `"forbid_actions":["flee"]`, `"forbid_actions":["flee","investigate"]`, `forbid_actions`},
		{"negative min hold", `"min_hold_ticks":5`, `"min_hold_ticks":-1`, `min_hold_ticks`},
		{"max hold below min", `"max_hold_ticks":100`, `"max_hold_ticks":4`, `max_hold_ticks`},
		{"goal hold above max", `"hold_ticks":10,"stress_responses"`, `"hold_ticks":1000,"stress_responses"`, `hold_ticks`},
		{"margin forbids switching", `"switch_margin":0.1`, `"switch_margin":1`, `switch_margin`},
		{"zero goals per pass", `"max_goals_per_pass":8`, `"max_goals_per_pass":0`, `max_goals_per_pass`},
		{"interrupt fact unknown", `"interrupt_facts":["danger.immediate"]`, `"interrupt_facts":["danger.vanished"]`, `interrupt_facts[0]`},
		{"always-on interrupt", `"applicable":[{"fact":"danger.immediate","is":true}]`, `"applicable":[]`, `.interrupt`},
		{"stress signal unknown", `"stress":{"signal":"signal.near"`, `"stress":{"signal":"signal.ghost"`, `stress.signal`},
		{"memory capacity", `"capacity":4`, `"capacity":0`, `memory.capacity`},
		{"memory ttl", `"default_ttl_ticks":50`, `"default_ttl_ticks":0`, `default_ttl_ticks`},
		{"unknown stress response", `"stress_responses":["flight"]`, `"stress_responses":["panic"]`, `stress_responses[0]`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			broken := strings.Replace(validUtilityCatalog, c.from, c.to, 1)
			if broken == validUtilityCatalog {
				t.Fatal("test case did not modify the catalog")
			}
			errs := ValidateUtility([]byte(broken), vocab)
			if len(errs) == 0 {
				t.Fatal("accepted invalid catalog")
			}
			found := false
			for _, e := range errs {
				if strings.Contains(e.Path, c.path) {
					found = true
				}
			}
			if !found {
				t.Fatalf("error does not point at %s: %v", c.path, errs)
			}
		})
	}
	if err := JoinErrors(ValidateUtility([]byte(strings.Replace(validUtilityCatalog, `"capacity":4`, `"capacity":0`, 1)), nil), func(path string) string { return "memory.cue:3:2" }); err == nil || !strings.Contains(err.Error(), "memory.cue:3:2: policy.memory.capacity") {
		t.Fatalf("positions are not attached to catalog errors: %v", err)
	}
}

func TestCatalogWithoutUtilityLayerStillValidates(t *testing.T) {
	if err := ValidateCatalog([]byte(`{"actions":{},"tasks":{}}`)); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCatalog([]byte(`{"actions":{},"tasks":{},"goals":{"goal.x":{"id":"goal.x","intent":"flee","action":"flee","considerations":[]}}}`)); err == nil {
		t.Fatal("goals without policy or considerations accepted")
	}
}
