package doctor

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

type AutoFix struct {
	Code  string         `json:"code"`
	Fix   string         `json:"fix"`
	Patch map[string]any `json:"patch,omitempty"`
}

type Suggestion struct {
	Code         string         `json:"code"`
	Fix          string         `json:"fix"`
	Patch        map[string]any `json:"patch,omitempty"`
	CanAutoApply bool           `json:"can_auto_apply"`
}

type State struct {
	Iteration int      `json:"iteration"`
	OpenCodes []string `json:"open_codes"`
}

type Response struct {
	Status          string       `json:"status"`
	Iteration       int          `json:"iteration"`
	ErrorsFixed     int          `json:"errors_fixed"`
	ErrorsRemaining int          `json:"errors_remaining"`
	DetectedCodes   []string     `json:"detected_codes"`
	KnownCodesTotal int          `json:"known_codes_total"`
	AutoFixable     []AutoFix    `json:"auto_fixable"`
	Suggestions     []Suggestion `json:"suggestions"`
	CatalogTotal    int          `json:"catalog_total"`
	Catalog         []Suggestion `json:"catalog"`
	LegacyHint      string       `json:"legacy_hint,omitempty"`
}

type Analyzer struct {
	statePath string
}

func NewAnalyzer(projectRoot string) *Analyzer {
	root := strings.TrimSpace(projectRoot)
	if root == "" {
		root = "."
	}
	return &Analyzer{
		statePath: filepath.Join(root, ".ang", "doctor_state.json"),
	}
}

func DetectErrorCodes(log string) []string {
	re := regexp.MustCompile(`\b(E_[A-Z0-9_]+|[A-Z]+_[A-Z0-9_]*_ERROR)\b`)
	matches := re.FindAllString(log, -1)
	if len(matches) == 0 {
		return nil
	}

	known := map[string]struct{}{
		"E_FSM_UNDEFINED_STATE": {},
	}
	for _, c := range compiler.StableErrorCodes {
		known[c] = struct{}{}
	}

	uniq := map[string]struct{}{}
	for _, m := range matches {
		if _, ok := known[m]; ok {
			uniq[m] = struct{}{}
		}
	}
	if len(uniq) == 0 {
		return nil
	}

	out := make([]string, 0, len(uniq))
	for c := range uniq {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

func BuildSuggestionCatalog(log string) []Suggestion {
	all := make([]string, 0, len(compiler.StableErrorCodes)+1)
	all = append(all, "E_FSM_UNDEFINED_STATE")
	all = append(all, compiler.StableErrorCodes...)
	all = uniqueSorted(all)

	out := make([]Suggestion, 0, len(all))
	for _, code := range all {
		out = append(out, suggestionForCode(code, log))
	}
	return out
}

func (a *Analyzer) Analyze(log string) Response {
	codes := DetectErrorCodes(log)
	codes = uniqueSorted(codes)
	catalog := BuildSuggestionCatalog(log)

	prev := a.loadState()
	next := computeIteration(prev, codes)
	fixed := countFixed(prev.OpenCodes, codes)
	remaining := len(codes)
	a.saveState(next)

	suggestions := make([]Suggestion, 0, len(codes))
	autoFixable := make([]AutoFix, 0, len(codes))
	for _, code := range codes {
		s := suggestionForCode(code, log)
		suggestions = append(suggestions, s)
		if s.CanAutoApply {
			autoFixable = append(autoFixable, AutoFix{
				Code:  s.Code,
				Fix:   s.Fix,
				Patch: s.Patch,
			})
		}
	}

	resp := Response{
		Status:          "Analyzed",
		Iteration:       next.Iteration,
		ErrorsFixed:     fixed,
		ErrorsRemaining: remaining,
		DetectedCodes:   codes,
		KnownCodesTotal: len(catalog),
		AutoFixable:     autoFixable,
		Suggestions:     suggestions,
		CatalogTotal:    len(catalog),
		Catalog:         catalog,
	}
	if strings.Contains(log, "range can't iterate over") {
		resp.LegacyHint = "logic.Call args must be a list"
	}
	return resp
}

func (a *Analyzer) loadState() State {
	data, err := os.ReadFile(a.statePath)
	if err != nil {
		return State{}
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return State{}
	}
	if st.Iteration < 0 {
		st.Iteration = 0
	}
	st.OpenCodes = uniqueSorted(st.OpenCodes)
	return st
}

func (a *Analyzer) saveState(st State) {
	st.OpenCodes = uniqueSorted(st.OpenCodes)
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(a.statePath), 0o755)
	_ = os.WriteFile(a.statePath, b, 0o644)
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

func computeIteration(prev State, current []string) State {
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

func suggestionForCode(code, log string) Suggestion {
	switch code {
	case "E_FSM_UNDEFINED_STATE":
		path, _, entity, state := parseFSMLocation(log)
		return Suggestion{
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
		return Suggestion{
			Code:         code,
			Fix:          "Fix CUE syntax or type conflicts in domain models.",
			CanAutoApply: false,
			Patch:        defaultPatchTemplate(code),
		}
	case compiler.ErrCodeCUEArchLoad:
		return Suggestion{
			Code:         code,
			Fix:          "Fix CUE syntax in architecture definitions.",
			CanAutoApply: false,
			Patch:        defaultPatchTemplate(code),
		}
	case compiler.ErrCodeCUEAPILoad:
		return Suggestion{
			Code:         code,
			Fix:          "Fix CUE syntax in API operations/endpoints.",
			CanAutoApply: false,
			Patch:        defaultPatchTemplate(code),
		}
	case compiler.ErrCodeCUERepoNormalize:
		return Suggestion{
			Code:         code,
			Fix:          "Fix repository schema: finder fields, returns/select compatibility.",
			CanAutoApply: false,
			Patch:        defaultPatchTemplate(code),
		}
	case compiler.ErrCodeCUETargetsParse, compiler.ErrCodeCUEProjectParse, compiler.ErrCodeCUEProjectLoad:
		return Suggestion{
			Code:         code,
			Fix:          "Fix target/project schema in cue/project.",
			CanAutoApply: false,
			Patch:        defaultPatchTemplate(code),
		}
	case compiler.ErrCodeCUEViewsLoad, compiler.ErrCodeCUEViewsParse:
		return Suggestion{
			Code:         code,
			Fix:          "Fix view definitions and referenced entities/fields.",
			CanAutoApply: false,
			Patch:        defaultPatchTemplate(code),
		}
	case compiler.ErrCodeCUEPolicyLoad, compiler.ErrCodeCUEPolicyValidate:
		return Suggestion{
			Code:         code,
			Fix:          "Fix policy file syntax/constraints under cue/policies.",
			CanAutoApply: false,
			Patch:        defaultPatchTemplate(code),
		}
	case compiler.ErrCodeEmitterCapabilityResolve:
		return Suggestion{
			Code:         code,
			Fix:          "Adjust target capabilities (lang/framework/db) in cue/project.",
			CanAutoApply: false,
			Patch:        defaultPatchTemplate(compiler.ErrCodeCUEProjectLoad),
		}
	case compiler.ErrCodeEmitterStep:
		return Suggestion{
			Code:         code,
			Fix:          "Inspect failing emitter step and fix upstream CUE intent causing invalid generation context.",
			CanAutoApply: false,
			Patch:        defaultPatchTemplate(compiler.ErrCodeCUEProjectLoad),
		}
	default:
		return Suggestion{
			Code:         code,
			Fix:          "Inspect error details and patch related CUE source; then re-run build.",
			CanAutoApply: false,
			Patch:        defaultPatchTemplate(code),
		}
	}
}
