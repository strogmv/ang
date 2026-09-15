package normalizer

import (
	"strings"

	"cuelang.org/go/cue"
)

// ExtractModels parses #Models alias registry from infra.
func (n *Normalizer) ExtractModels(val cue.Value) (*ModelsDef, error) {
	modelsVal := val.LookupPath(cue.ParsePath("#Models"))
	if !modelsVal.Exists() {
		return nil, nil
	}

	iter, err := modelsVal.Fields()
	if err != nil {
		return nil, err
	}

	out := &ModelsDef{Aliases: map[string]string{}}
	for iter.Next() {
		name := strings.Trim(iter.Selector().String(), "\"")
		model, _ := iter.Value().String()
		model = strings.TrimSpace(model)
		if name == "" || model == "" {
			continue
		}
		out.Aliases[name] = model
	}
	if len(out.Aliases) == 0 {
		return nil, nil
	}
	return out, nil
}
