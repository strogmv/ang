// Package behavior compiles reusable CUE behavior catalogs independently of game hosts and clients.
package behavior

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// CatalogError points at a catalog path (CUE-like, e.g. goals."goal.x".considerations[0].input).
// The caller that still holds the CUE value can map the path to file:line.
type CatalogError struct{ Path, Message string }

func (e CatalogError) Error() string { return e.Path + ": " + e.Message }

// Vocabulary is the world's enumerations the AI catalog may reference. A nil list skips that check.
type Vocabulary struct{ Needs, Emotions, Actions []string }

// ValidateCatalog checks graph references and optional mission roots.
// CUE validates types and references. The compiler also rejects recursive task
// graphs: this first bounded interpreter intentionally supports acyclic HTNs.
func ValidateCatalog(data []byte, roots ...string) error {
	var c struct {
		Actions map[string]struct {
			ID string `json:"id"`
		} `json:"actions"`
		Tasks map[string]struct {
			ID      string `json:"id"`
			Methods []struct {
				ID    string   `json:"id"`
				Steps []string `json:"steps"`
			} `json:"methods"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return err
	}
	for _, root := range roots {
		if _, ok := c.Tasks[root]; !ok {
			return fmt.Errorf("AI mission root %q is not a declared task", root)
		}
	}
	for id := range c.Actions {
		if _, exists := c.Tasks[id]; exists {
			return fmt.Errorf("AI id %s is both action and task", id)
		}
	}
	keys := make([]string, 0, len(c.Tasks))
	for id := range c.Tasks {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	visited := map[string]int{}
	var visit func(string) error
	visit = func(id string) error {
		if visited[id] == 1 {
			return fmt.Errorf("AI task cycle at %s", id)
		}
		if visited[id] == 2 {
			return nil
		}
		visited[id] = 1
		task := c.Tasks[id]
		if task.ID != id || len(task.Methods) == 0 {
			return fmt.Errorf("invalid AI task %s", id)
		}
		methods := map[string]bool{}
		for _, m := range task.Methods {
			if m.ID == "" || methods[m.ID] {
				return fmt.Errorf("duplicate or missing method in %s", id)
			}
			methods[m.ID] = true
			for _, step := range m.Steps {
				if _, ok := c.Actions[step]; ok {
					continue
				}
				if _, ok := c.Tasks[step]; !ok {
					return fmt.Errorf("unknown AI step %s in %s", step, id)
				}
				if err := visit(step); err != nil {
					return err
				}
			}
		}
		visited[id] = 2
		return nil
	}
	for _, id := range keys {
		if err := visit(id); err != nil {
			return err
		}
	}
	if errs := ValidateUtility(data, nil); len(errs) > 0 {
		return errs[0]
	}
	return nil
}

var utilityIntents = []string{"routine", "withdraw", "gather_watch", "group_order", "flee", "assist"}
var stressResponses = []string{"fight", "flight", "freeze", "appease", "analyze", "withdraw"}

func member(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// ValidateUtility checks the personal-goal layer: references between goals, facts,
// signals, curves, tasks and profiles, the world vocabulary, and hysteresis settings
// that could never work (a hold longer than the maximum, a margin that forbids every
// switch, an interrupt goal without any applicability condition). It returns every
// problem found so the caller can print them all with source positions.
func ValidateUtility(data []byte, vocab *Vocabulary) []CatalogError {
	var c struct {
		Facts   map[string]json.RawMessage `json:"facts"`
		Signals map[string]json.RawMessage `json:"signals"`
		Curves  map[string]json.RawMessage `json:"curves"`
		Tasks   map[string]json.RawMessage `json:"tasks"`
		Goals   map[string]struct {
			ID         string `json:"id"`
			Intent     string `json:"intent"`
			Action     string `json:"action"`
			Plan       string `json:"plan"`
			Applicable []struct {
				Fact string `json:"fact"`
			} `json:"applicable"`
			Considerations []struct {
				Input   string  `json:"input"`
				Curve   string  `json:"curve"`
				Weight  float64 `json:"weight"`
				Need    string  `json:"need"`
				Emotion string  `json:"emotion"`
			} `json:"considerations"`
			Base            float64  `json:"base"`
			HoldTicks       int64    `json:"hold_ticks"`
			Interrupt       bool     `json:"interrupt"`
			StressResponses []string `json:"stress_responses"`
		} `json:"goals"`
		Profiles map[string]struct {
			ID            string   `json:"id"`
			SceneGoals    []string `json:"scene_goals"`
			ForbidActions []string `json:"forbid_actions"`
		} `json:"profiles"`
		Policy struct {
			Utility *struct {
				MinHoldTicks       int64             `json:"min_hold_ticks"`
				MaxHoldTicks       int64             `json:"max_hold_ticks"`
				SwitchMargin       float64           `json:"switch_margin"`
				MaxGoalsPerPass    int               `json:"max_goals_per_pass"`
				ExplainTop         int               `json:"explain_top"`
				InterruptFacts     []string          `json:"interrupt_facts"`
				UnknownSignalValue float64           `json:"unknown_signal_value"`
				Stress             map[string]string `json:"stress"`
				EmotionProxies     map[string]string `json:"emotion_proxies"`
			} `json:"utility"`
			Memory *struct {
				Capacity           int            `json:"capacity"`
				MergeWindowTicks   int64          `json:"merge_window_ticks"`
				DefaultTTLTicks    int64          `json:"default_ttl_ticks"`
				Priorities         map[string]int `json:"priorities"`
				UnknownThreatValue float64        `json:"unknown_threat_value"`
			} `json:"memory"`
		} `json:"policy"`
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return []CatalogError{{Path: "catalog", Message: err.Error()}}
	}
	var errs []CatalogError
	fail := func(path, format string, args ...any) {
		errs = append(errs, CatalogError{Path: path, Message: fmt.Sprintf(format, args...)})
	}
	if len(c.Goals) == 0 && c.Policy.Utility == nil && c.Policy.Memory == nil {
		return nil
	}
	var maxHold int64 = -1
	if u := c.Policy.Utility; u == nil {
		fail("policy.utility", "missing utility policy")
	} else {
		maxHold = u.MaxHoldTicks
		if u.MinHoldTicks < 0 {
			fail("policy.utility.min_hold_ticks", "must not be negative")
		}
		if u.MaxHoldTicks < u.MinHoldTicks {
			fail("policy.utility.max_hold_ticks", "below min_hold_ticks: no goal could ever be held")
		}
		if u.SwitchMargin < 0 || u.SwitchMargin >= 1 {
			fail("policy.utility.switch_margin", "must be in [0,1): a margin of 1 or more forbids every switch")
		}
		if u.MaxGoalsPerPass < 1 {
			fail("policy.utility.max_goals_per_pass", "must evaluate at least one goal per pass")
		}
		if u.ExplainTop < 1 {
			fail("policy.utility.explain_top", "must keep at least one scored goal")
		}
		if u.UnknownSignalValue < 0 || u.UnknownSignalValue > 1 {
			fail("policy.utility.unknown_signal_value", "must be in [0,1]")
		}
		for i, f := range u.InterruptFacts {
			if _, ok := c.Facts[f]; !ok {
				fail(fmt.Sprintf("policy.utility.interrupt_facts[%d]", i), "unknown fact %s", f)
			}
		}
		if _, ok := c.Signals[u.Stress["signal"]]; !ok {
			fail("policy.utility.stress.signal", "unknown signal %s", u.Stress["signal"])
		}
		if vocab != nil && vocab.Emotions != nil && !member(vocab.Emotions, u.Stress["emotion"]) {
			fail("policy.utility.stress.emotion", "unknown emotion %s", u.Stress["emotion"])
		}
		for e, s := range u.EmotionProxies {
			if _, ok := c.Signals[s]; !ok {
				fail("policy.utility.emotion_proxies."+e, "unknown signal %s", s)
			}
			if vocab != nil && vocab.Emotions != nil && !member(vocab.Emotions, e) {
				fail("policy.utility.emotion_proxies."+e, "unknown emotion %s", e)
			}
		}
	}
	if m := c.Policy.Memory; m == nil {
		fail("policy.memory", "missing memory policy")
	} else {
		if m.Capacity < 1 {
			fail("policy.memory.capacity", "must hold at least one fact")
		}
		if m.DefaultTTLTicks < 1 {
			fail("policy.memory.default_ttl_ticks", "facts must live at least one tick")
		}
		if m.MergeWindowTicks < 0 {
			fail("policy.memory.merge_window_ticks", "must not be negative")
		}
		if m.UnknownThreatValue < 0 || m.UnknownThreatValue > 1 {
			fail("policy.memory.unknown_threat_value", "must be in [0,1]")
		}
		for kind, p := range m.Priorities {
			if p < 1 {
				fail("policy.memory.priorities."+kind, "priority must be positive")
			}
		}
	}
	goalIDs := make([]string, 0, len(c.Goals))
	for id := range c.Goals {
		goalIDs = append(goalIDs, id)
	}
	sort.Strings(goalIDs)
	for _, id := range goalIDs {
		g := c.Goals[id]
		path := "goals." + strconvQuote(id)
		if g.ID != id {
			fail(path+".id", "id %q does not match key", g.ID)
		}
		if !member(utilityIntents, g.Intent) {
			fail(path+".intent", "unknown intent %q", g.Intent)
		}
		if g.Action == "" {
			fail(path+".action", "missing action tag")
		} else if vocab != nil && vocab.Actions != nil && !member(vocab.Actions, g.Action) {
			fail(path+".action", "unknown action tag %q", g.Action)
		}
		if g.Plan != "" {
			if _, ok := c.Tasks[g.Plan]; !ok {
				fail(path+".plan", "unknown task %s", g.Plan)
			}
		}
		for i, a := range g.Applicable {
			if _, ok := c.Facts[a.Fact]; !ok {
				fail(fmt.Sprintf("%s.applicable[%d].fact", path, i), "unknown fact %s", a.Fact)
			}
		}
		if len(g.Considerations) == 0 {
			fail(path+".considerations", "goal needs at least one consideration")
		}
		for i, con := range g.Considerations {
			cp := fmt.Sprintf("%s.considerations[%d]", path, i)
			if _, ok := c.Signals[con.Input]; !ok {
				fail(cp+".input", "unknown signal %s", con.Input)
			}
			if c.Curves != nil {
				if _, ok := c.Curves[con.Curve]; !ok {
					fail(cp+".curve", "unknown curve %s", con.Curve)
				}
			}
			if con.Weight <= 0 {
				fail(cp+".weight", "missing or non-positive weight")
			}
			if con.Need != "" && vocab != nil && vocab.Needs != nil && !member(vocab.Needs, con.Need) {
				fail(cp+".need", "unknown need %q", con.Need)
			}
			if con.Emotion != "" && vocab != nil && vocab.Emotions != nil && !member(vocab.Emotions, con.Emotion) {
				fail(cp+".emotion", "unknown emotion %q", con.Emotion)
			}
		}
		if g.Base <= 0 {
			fail(path+".base", "missing or non-positive base")
		}
		if g.HoldTicks < 0 {
			fail(path+".hold_ticks", "must not be negative")
		}
		if maxHold >= 0 && g.HoldTicks > maxHold {
			fail(path+".hold_ticks", "%d exceeds policy max_hold_ticks %d", g.HoldTicks, maxHold)
		}
		if g.Interrupt && len(g.Applicable) == 0 {
			fail(path+".interrupt", "an always-applicable interrupt goal would break every hold")
		}
		for i, s := range g.StressResponses {
			if !member(stressResponses, s) {
				fail(fmt.Sprintf("%s.stress_responses[%d]", path, i), "unknown stress response %q", s)
			}
		}
	}
	profileIDs := make([]string, 0, len(c.Profiles))
	for id := range c.Profiles {
		profileIDs = append(profileIDs, id)
	}
	sort.Strings(profileIDs)
	for _, id := range profileIDs {
		p := c.Profiles[id]
		path := "profiles." + strconvQuote(id)
		if len(c.Goals) > 0 && len(p.SceneGoals) == 0 {
			fail(path+".scene_goals", "profile has no personal goals")
		}
		allForbidden := len(p.SceneGoals) > 0
		for i, g := range p.SceneGoals {
			goal, ok := c.Goals[g]
			if !ok {
				fail(fmt.Sprintf("%s.scene_goals[%d]", path, i), "unknown goal %s", g)
				allForbidden = false
				continue
			}
			if !member(p.ForbidActions, goal.Action) {
				allForbidden = false
			}
		}
		if allForbidden {
			fail(path+".forbid_actions", "role duty forbids every personal goal of the profile")
		}
		for i, a := range p.ForbidActions {
			if vocab != nil && vocab.Actions != nil && !member(vocab.Actions, a) {
				fail(fmt.Sprintf("%s.forbid_actions[%d]", path, i), "unknown action tag %q", a)
			}
		}
	}
	return errs
}

func strconvQuote(id string) string {
	if strings.ContainsAny(id, ".-") {
		return `"` + id + `"`
	}
	return id
}

// JoinErrors formats catalog errors one per line, optionally prefixed with positions.
func JoinErrors(errs []CatalogError, position func(path string) string) error {
	if len(errs) == 0 {
		return nil
	}
	lines := make([]string, 0, len(errs))
	for _, e := range errs {
		prefix := ""
		if position != nil {
			prefix = position(e.Path)
		}
		if prefix != "" {
			lines = append(lines, prefix+": "+e.Error())
		} else {
			lines = append(lines, e.Error())
		}
	}
	return fmt.Errorf("AI catalog:\n%s", strings.Join(lines, "\n"))
}
