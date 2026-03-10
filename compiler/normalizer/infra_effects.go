package normalizer

import (
	"encoding/json"
	"strings"

	"cuelang.org/go/cue"
)

// ExtractEffectHandlers parses infra handler bindings from Handlers or TestHandlers.
func (n *Normalizer) ExtractEffectHandlers(val cue.Value, root string) (*EffectHandlersDef, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		root = "Handlers"
	}
	handlersVal := val.LookupPath(cue.ParsePath(root))
	if !handlersVal.Exists() {
		return nil, nil
	}

	iter, err := handlersVal.Fields()
	if err != nil {
		return nil, err
	}

	out := &EffectHandlersDef{Bindings: map[string]EffectHandlerBinding{}}
	for iter.Next() {
		kind := strings.Trim(iter.Selector().String(), "\"")
		binding := parseEffectHandlerBinding(kind, iter.Value())
		out.Bindings[kind] = binding
	}
	return out, nil
}

// ExtractEffectMiddleware parses per-effect middleware chains from infra Middleware.
func (n *Normalizer) ExtractEffectMiddleware(val cue.Value) (*EffectMiddlewareCatalogDef, error) {
	mwVal := val.LookupPath(cue.ParsePath("Middleware"))
	if !mwVal.Exists() {
		return nil, nil
	}

	iter, err := mwVal.Fields()
	if err != nil {
		return nil, err
	}

	out := &EffectMiddlewareCatalogDef{Chains: map[string][]EffectMiddlewareDef{}}
	for iter.Next() {
		kind := strings.Trim(iter.Selector().String(), "\"")
		list, err := parseEffectMiddlewareChain(iter.Value())
		if err != nil {
			return nil, err
		}
		out.Chains[kind] = list
	}
	return out, nil
}

func parseEffectHandlerBinding(kind string, v cue.Value) EffectHandlerBinding {
	binding := EffectHandlerBinding{
		Kind:    strings.TrimSpace(kind),
		Driver:  strings.TrimSpace(getString(v, "driver")),
		Source:  formatPos(v),
		Options: map[string]any{},
	}
	if provider := strings.TrimSpace(getString(v, "provider")); provider != "" {
		binding.Provider = provider
	}

	iter, err := v.Fields()
	if err != nil {
		return binding
	}
	for iter.Next() {
		key := strings.Trim(iter.Selector().String(), "\"")
		if key == "driver" || key == "provider" {
			continue
		}
		binding.Options[key] = cueValueToJSONCompatible(iter.Value())
	}
	if len(binding.Options) == 0 {
		binding.Options = nil
	}
	return binding
}

func parseEffectMiddlewareChain(v cue.Value) ([]EffectMiddlewareDef, error) {
	var out []EffectMiddlewareDef
	list, err := v.List()
	if err != nil {
		return nil, err
	}
	for list.Next() {
		item := list.Value()
		step := EffectMiddlewareDef{
			Type:     strings.TrimSpace(getString(item, "type")),
			Backoff:  strings.TrimSpace(getString(item, "backoff")),
			TTL:      strings.TrimSpace(getString(item, "ttl")),
			Key:      strings.TrimSpace(getString(item, "key")),
			Duration: strings.TrimSpace(getString(item, "duration")),
			Level:    strings.TrimSpace(getString(item, "level")),
			Source:   formatPos(item),
			Options:  map[string]any{},
		}
		if v, err := item.LookupPath(cue.ParsePath("attempts")).Int64(); err == nil {
			step.Attempts = int(v)
		}
		if onVal := item.LookupPath(cue.ParsePath("on")); onVal.Exists() {
			onList, err := onVal.List()
			if err == nil {
				for onList.Next() {
					if code, err := onList.Value().Int64(); err == nil {
						step.On = append(step.On, int(code))
					}
				}
			}
		}

		iter, err := item.Fields()
		if err == nil {
			for iter.Next() {
				key := strings.Trim(iter.Selector().String(), "\"")
				switch key {
				case "type", "attempts", "backoff", "on", "ttl", "key", "duration", "level":
					continue
				default:
					step.Options[key] = cueValueToJSONCompatible(iter.Value())
				}
			}
		}
		if len(step.Options) == 0 {
			step.Options = nil
		}
		out = append(out, step)
	}
	return out, nil
}

func cueValueToJSONCompatible(v cue.Value) any {
	data, err := v.MarshalJSON()
	if err == nil {
		var out any
		if json.Unmarshal(data, &out) == nil {
			return out
		}
	}
	return cueValueToInterface(v)
}
