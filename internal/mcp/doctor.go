package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/strogmv/ang/compiler"
)

type doctorAutoFix struct {
	Code  string         `json:"code"`
	Fix   string         `json:"fix"`
	Patch map[string]any `json:"patch,omitempty"`
}

type doctorSuggestion struct {
	Code         string         `json:"code"`
	Fix          string         `json:"fix"`
	Patch        map[string]any `json:"patch,omitempty"`
	CanAutoApply bool           `json:"can_auto_apply"`
}

type doctorState struct {
	Iteration int      `json:"iteration"`
	OpenCodes []string `json:"open_codes"`
}

func stateFilePath() string {
	return filepath.Join(".ang", "doctor_state.json")
}

func loadDoctorState() doctorState {
	data, err := os.ReadFile(stateFilePath())
	if err != nil {
		return doctorState{}
	}
	var st doctorState
	if err := json.Unmarshal(data, &st); err != nil {
		return doctorState{}
	}
	if st.Iteration < 0 {
		st.Iteration = 0
	}
	st.OpenCodes = uniqueSorted(st.OpenCodes)
	return st
}

func saveDoctorState(st doctorState) {
	st.OpenCodes = uniqueSorted(st.OpenCodes)
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(stateFilePath()), 0o755)
	_ = os.WriteFile(stateFilePath(), b, 0o644)
}

func uniqueSorted(in []string) []string {
	set := make(map[string]struct{}, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		set[v] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func toSet(in []string) map[string]struct{} {
	out := make(map[string]struct{}, len(in))
	for _, c := range in {
		out[c] = struct{}{}
	}
	return out
}

func countFixed(prev, current []string) int {
	cur := toSet(current)
	n := 0
	for _, p := range uniqueSorted(prev) {
		if _, ok := cur[p]; !ok {
			n++
		}
	}
	return n
}

func signature(codes []string) string {
	return strings.Join(uniqueSorted(codes), ",")
}

func computeIteration(prev doctorState, current []string) doctorState {
	current = uniqueSorted(current)
	prevCodes := uniqueSorted(prev.OpenCodes)
	prevSig := signature(prevCodes)
	curSig := signature(current)

	next := prev
	if prev.Iteration == 0 && len(current) > 0 {
		next.Iteration = 1
	} else if curSig != prevSig {
		next.Iteration++
	}
	next.OpenCodes = current
	return next
}

func parseFSMLocation(log string) (path string, line int, entity string, state string) {
	if m := regexp.MustCompile(`Entity '([^']+)' FSM transition '[^']*' references undefined state '([^']+)'`).FindStringSubmatch(log); len(m) == 3 {
		entity = m[1]
		state = m[2]
	}
	if m := regexp.MustCompile(`at (cue/[^:\s]+):(\d+):\d+`).FindStringSubmatch(log); len(m) == 3 {
		path = m[1]
		line, _ = strconv.Atoi(m[2])
	}
	if state == "" {
		if m := regexp.MustCompile(`undefined state '([^']+)'`).FindStringSubmatch(log); len(m) == 2 {
			state = m[1]
		}
	}
	if entity == "" {
		entity = "Order"
	}
	if state == "" {
		state = "paid"
	}
	if path == "" {
		path = "cue/domain/order.cue"
	}
	return path, line, entity, state
}

func defaultPatchTemplate(code string) map[string]any {
	pathByCode := map[string]string{
		compiler.ErrCodeCUEDomainLoad:        "cue/domain/entities.cue",
		compiler.ErrCodeCUEArchLoad:          "cue/architecture/services.cue",
		compiler.ErrCodeCUEAPILoad:           "cue/api/http.cue",
		compiler.ErrCodeCUERepoNormalize:     "cue/repo/repositories.cue",
		compiler.ErrCodeCUEScheduleNormalize: "cue/api/schedules.cue",
		compiler.ErrCodeCUEViewsLoad:         "cue/views/views.cue",
		compiler.ErrCodeCUEProjectLoad:       "cue/project/project.cue",
	}
	path := pathByCode[code]
	if path == "" {
		path = "cue/domain/entities.cue"
	}
	return map[string]any{
		"path":         path,
		"selector":     "",
		"forced_merge": false,
		"content": fmt.Sprintf(
			"// TODO: fix %s in this CUE file and re-run build.\n",
			code,
		),
	}
}

func suggestionForCode(code, log string) doctorSuggestion {
	switch code {
	case "E_FSM_UNDEFINED_STATE":
		path, _, entity, state := parseFSMLocation(log)
		return doctorSuggestion{
			Code:         code,
			Fix:          fmt.Sprintf("Add '%s' to %s.fsm.states.", state, entity),
			CanAutoApply: true,
			Patch: map[string]any{
				"path":         path,
				"selector":     "",
				"forced_merge": false,
				"content":      fmt.Sprintf("// Add state '%s' to fsm.states for entity %s.\n", state, entity),
			},
		}
	case compiler.ErrCodeCUEDomainLoad:
		return doctorSuggestion{
			Code:         code,
			Fix:          "Fix CUE syntax or type conflicts in domain models.",
			CanAutoApply: false,
			Patch:        defaultPatchTemplate(code),
		}
	case compiler.ErrCodeCUEArchLoad:
		return doctorSuggestion{
			Code:         code,
			Fix:          "Fix CUE syntax in architecture definitions.",
			CanAutoApply: false,
			Patch:        defaultPatchTemplate(code),
		}
	case compiler.ErrCodeCUEAPILoad:
		return doctorSuggestion{
			Code:         code,
			Fix:          "Fix CUE syntax in API operations/endpoints.",
			CanAutoApply: false,
			Patch:        defaultPatchTemplate(code),
		}
	case compiler.ErrCodeCUERepoNormalize:
		return doctorSuggestion{
			Code:         code,
			Fix:          "Fix repository schema: finder fields, returns/select compatibility.",
			CanAutoApply: false,
			Patch:        defaultPatchTemplate(code),
		}
	case compiler.ErrCodeCUETargetsParse, compiler.ErrCodeCUEProjectParse, compiler.ErrCodeCUEProjectLoad:
		return doctorSuggestion{
			Code:         code,
			Fix:          "Fix target/project schema in cue/project.",
			CanAutoApply: false,
			Patch:        defaultPatchTemplate(code),
		}
	case compiler.ErrCodeCUEViewsLoad, compiler.ErrCodeCUEViewsParse:
		return doctorSuggestion{
			Code:         code,
			Fix:          "Fix view definitions and referenced entities/fields.",
			CanAutoApply: false,
			Patch:        defaultPatchTemplate(code),
		}
	case compiler.ErrCodeCUEPolicyLoad, compiler.ErrCodeCUEPolicyValidate:
		return doctorSuggestion{
			Code:         code,
			Fix:          "Fix policy file syntax/constraints under cue/policies.",
			CanAutoApply: false,
			Patch:        defaultPatchTemplate(code),
		}
	case compiler.ErrCodeEmitterCapabilityResolve:
		return doctorSuggestion{
			Code:         code,
			Fix:          "Adjust target capabilities (lang/framework/db) in cue/project.",
			CanAutoApply: false,
			Patch:        defaultPatchTemplate(compiler.ErrCodeCUEProjectLoad),
		}
	case compiler.ErrCodeEmitterStep:
		return doctorSuggestion{
			Code:         code,
			Fix:          "Inspect failing emitter step and fix upstream CUE intent causing invalid generation context.",
			CanAutoApply: false,
			Patch:        defaultPatchTemplate(compiler.ErrCodeCUEProjectLoad),
		}
	default:
		return doctorSuggestion{
			Code:         code,
			Fix:          "Inspect error details and patch related CUE source; then re-run build.",
			CanAutoApply: false,
			Patch:        defaultPatchTemplate(code),
		}
	}
}

func buildDoctorResponse(log string) map[string]any {
	codes := detectErrorCodes(log)
	codes = uniqueSorted(codes)
	catalog := buildSuggestionCatalog(log)

	prev := loadDoctorState()
	next := computeIteration(prev, codes)
	fixed := countFixed(prev.OpenCodes, codes)
	remaining := len(codes)
	saveDoctorState(next)

	suggestions := make([]doctorSuggestion, 0, len(codes))
	autoFixable := make([]doctorAutoFix, 0, len(codes))
	for _, code := range codes {
		s := suggestionForCode(code, log)
		suggestions = append(suggestions, s)
		if s.CanAutoApply {
			autoFixable = append(autoFixable, doctorAutoFix{
				Code:  s.Code,
				Fix:   s.Fix,
				Patch: s.Patch,
			})
		}
	}

	knownCodesTotal := len(catalog)
	resp := map[string]any{
		"status":            "Analyzed",
		"iteration":         next.Iteration,
		"errors_fixed":      fixed,
		"errors_remaining":  remaining,
		"detected_codes":    codes,
		"known_codes_total": knownCodesTotal,
		"auto_fixable":      autoFixable,
		"suggestions":       suggestions,
		"catalog_total":     len(catalog),
		"catalog":           catalog,
	}

	if strings.Contains(log, "range can't iterate over") {
		resp["legacy_hint"] = "logic.Call args must be a list"
	}

	return resp
}

func buildSuggestionCatalog(log string) []doctorSuggestion {
	all := make([]string, 0, len(compiler.StableErrorCodes)+1)
	all = append(all, "E_FSM_UNDEFINED_STATE")
	all = append(all, compiler.StableErrorCodes...)
	all = uniqueSorted(all)

	out := make([]doctorSuggestion, 0, len(all))
	for _, code := range all {
		out = append(out, suggestionForCode(code, log))
	}
	return out
}
