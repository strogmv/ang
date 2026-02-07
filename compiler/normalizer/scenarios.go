package normalizer

import (
	"encoding/json"
	"strings"

	"cuelang.org/go/cue"
)

func (n *Normalizer) ExtractScenarios(val cue.Value) ([]ScenarioDef, error) {
	if !val.Exists() || val.IncompleteKind() == cue.BottomKind {
		return nil, nil
	}

	var scenarios []ScenarioDef
	iter, err := val.Fields(cue.All())
	if err != nil {
		return nil, err
	}

	for iter.Next() {
		label := iter.Selector().String()
		v := iter.Value()

		// Scenarios should start with 'Scenario' prefix or be in a specific block
		if !strings.HasPrefix(label, "Scenario") {
			continue
		}

		scen, err := n.parseScenario(label, v)
		if err != nil {
			return nil, err
		}
		scenarios = append(scenarios, scen)
	}

	return scenarios, nil
}

func (n *Normalizer) parseScenario(name string, val cue.Value) (ScenarioDef, error) {
	scen := ScenarioDef{
		Name:        name,
		Description: getString(val, "description"),
		Source:      formatPos(val),
	}

	stepsVal := val.LookupPath(cue.ParsePath("steps"))
	if stepsVal.Exists() && stepsVal.Kind() == cue.ListKind {
		list, _ := stepsVal.List()
		for list.Next() {
			sv := list.Value()
			step := ScenarioStep{
				Name:   getString(sv, "name"),
				Action: getString(sv, "action"),
				Input:  make(map[string]any),
				Export: make(map[string]string),
			}

			// Parse Input
			inVal := sv.LookupPath(cue.ParsePath("input"))
			if inVal.Exists() {
				temp, _ := inVal.MarshalJSON()
				json.Unmarshal(temp, &step.Input)
			}

			// Parse Expect
			expVal := sv.LookupPath(cue.ParsePath("expect"))
			if expVal.Exists() {
				status, _ := expVal.LookupPath(cue.ParsePath("status")).Int64()
				step.Expect.Status = int(status)
				
				bodyVal := expVal.LookupPath(cue.ParsePath("body"))
				if bodyVal.Exists() {
					temp, _ := bodyVal.MarshalJSON()
					json.Unmarshal(temp, &step.Expect.Body)
				}
			} else {
				step.Expect.Status = 200 // Default
			}

			// Parse Export
			exVal := sv.LookupPath(cue.ParsePath("export"))
			if exVal.Exists() {
				iter, _ := exVal.Fields()
				for iter.Next() {
					s, _ := iter.Value().String()
					step.Export[iter.Selector().String()] = s
				}
			}

			scen.Steps = append(scen.Steps, step)
		}
	}

	return scen, nil
}
