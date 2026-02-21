package normalizer

import (
	"fmt"
	"strings"

	"cuelang.org/go/cue"
)

// parseImplSteps converts CUE impl_steps into typed ImplStep list.
func (n *Normalizer) parseImplSteps(val cue.Value) ([]ImplStep, error) {
	if val.Kind() != cue.ListKind {
		return nil, fmt.Errorf("impl_steps must be list")
	}
	var steps []ImplStep
	it, _ := val.List()
	idx := 0
	for it.Next() {
		idx++
		stepVal := it.Value()
		k := stepKind(stepVal)
		switch k {
		case "load":
			loadTarget := getString(stepVal, "load")
			if strings.TrimSpace(loadTarget) == "" {
				return nil, fmt.Errorf("impl_steps[%d]: load target is empty", idx-1)
			}
			if !n.repoExists(loadTarget) {
				n.Warn(Warning{Kind: "impl", Severity: "error", Code: "IMPL_REPO_NOT_FOUND", Message: fmt.Sprintf("impl_steps[%d]: repo %s not found", idx-1, loadTarget), CUEPath: stepVal.Path().String()})
			}
			into := getString(stepVal, "into")
			byMap := map[string]string{}
			byVal := stepVal.LookupPath(cue.ParsePath("by"))
			if byVal.Exists() {
				if byVal.IncompleteKind() == cue.StructKind {
					fields, _ := byVal.Fields(cue.All())
					for fields.Next() {
						s, _ := fields.Value().String()
						byMap[fields.Selector().String()] = s
					}
				} else if s, err := byVal.String(); err == nil {
					byMap["id"] = s
				}
			}
			steps = append(steps, ImplStep{
				Kind:       "load",
				LoadTarget: loadTarget,
				LoadBy:     byMap,
				LoadInto:   into,
				Source:     formatPos(stepVal),
			})
		case "assert":
			expr := getString(stepVal, "assert")
			if strings.TrimSpace(expr) == "" {
				return nil, fmt.Errorf("impl_steps[%d]: assert expression is empty", idx-1)
			}
			errCode := getString(stepVal, "error")
			steps = append(steps, ImplStep{
				Kind:        "assert",
				AssertExpr:  expr,
				AssertError: errCode,
				Source:      formatPos(stepVal),
			})
		case "call":
			target := getString(stepVal, "call")
			if strings.TrimSpace(target) == "" {
				return nil, fmt.Errorf("impl_steps[%d]: call target is empty", idx-1)
			}
			if !n.repoExists(target) && !strings.Contains(target, ".") {
				n.Warn(Warning{Kind: "impl", Severity: "warn", Code: "IMPL_CALL_UNKNOWN", Message: fmt.Sprintf("impl_steps[%d]: call target %s not recognized", idx-1, target), CUEPath: stepVal.Path().String()})
			}
			callArgsMap := map[string]any{}
			if withVal := stepVal.LookupPath(cue.ParsePath("with")); withVal.Exists() {
				if withVal.IncompleteKind() == cue.StructKind {
					fields, _ := withVal.Fields(cue.All())
					for fields.Next() {
						callArgsMap[fields.Selector().String()] = cueValueToInterface(fields.Value())
					}
				} else if s, err := withVal.String(); err == nil {
					callArgsMap["expr"] = s
				}
			}
			steps = append(steps, ImplStep{
				Kind:         "call",
				CallTarget:   target,
				CallArgsMap:  callArgsMap,
				CallArgsExpr: getString(stepVal, "with"),
				CallInto:     getString(stepVal, "into"),
				Source:       formatPos(stepVal),
			})
		case "emit":
			evt := getString(stepVal, "emit")
			if strings.TrimSpace(evt) == "" {
				return nil, fmt.Errorf("impl_steps[%d]: emit event is empty", idx-1)
			}
			if !n.eventExists(evt) {
				n.Warn(Warning{Kind: "impl", Severity: "warn", Code: "IMPL_EVENT_NOT_FOUND", Message: fmt.Sprintf("impl_steps[%d]: event %s not found", idx-1, evt), CUEPath: stepVal.Path().String()})
			}
			payloadMap := map[string]any{}
			if payloadVal := stepVal.LookupPath(cue.ParsePath("payload")); payloadVal.Exists() && payloadVal.IncompleteKind() == cue.StructKind {
				fields, _ := payloadVal.Fields(cue.All())
				for fields.Next() {
					payloadMap[fields.Selector().String()] = cueValueToInterface(fields.Value())
				}
			}
			steps = append(steps, ImplStep{
				Kind:        "emit",
				EmitEvent:   evt,
				EmitPayload: payloadMap,
				EmitExpr:    getString(stepVal, "payload"),
				Source:      formatPos(stepVal),
			})
		default:
			return nil, fmt.Errorf("impl_steps[%d]: unknown step kind", idx-1)
		}
	}
	return steps, nil
}

func stepKind(v cue.Value) string {
	for _, key := range []string{"load", "assert", "call", "emit"} {
		if v.LookupPath(cue.ParsePath(key)).Exists() {
			return key
		}
	}
	return ""
}

func (n *Normalizer) repoExists(name string) bool {
	if n.RepoNames == nil {
		return false
	}
	repo := name
	if parts := strings.Split(name, "."); len(parts) > 0 {
		repo = parts[0]
	}
	_, ok := n.RepoNames[repo]
	return ok
}

func (n *Normalizer) eventExists(name string) bool {
	if n.EventNames == nil {
		return false
	}
	_, ok := n.EventNames[name]
	return ok
}

// cueValueToInterface best-effort conversion for diagnostics; strings for non-basic kinds.
func cueValueToInterface(v cue.Value) any {
	switch v.IncompleteKind() {
	case cue.StringKind:
		s, _ := v.String()
		return s
	case cue.IntKind:
		i, _ := v.Int64()
		return i
	case cue.BoolKind:
		b, _ := v.Bool()
		return b
	case cue.FloatKind:
		f, _ := v.Float64()
		return f
	case cue.StructKind, cue.ListKind:
		return fmt.Sprintf("%v", v.Syntax())
	default:
		return fmt.Sprintf("%v", v.Syntax())
	}
}
