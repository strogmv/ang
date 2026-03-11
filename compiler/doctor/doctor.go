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
	"github.com/strogmv/ang/compiler/ir"
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
	Summary         []string     `json:"summary"`
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

	known := map[string]struct{}{
		"E_FSM_UNDEFINED_STATE":                  {},
		"MISSING_ID":                             {},
		"MISSING_CREATED_AT":                     {},
		"UNKNOWN_ACTION":                         {},
		"E_FLOW_UNKNOWN_ACTION":                  {},
		"E_FLOW_TOO_LARGE":                       {},
		"W_FLOW_OUTBOX_PREFERRED":                {},
		"GO_UNDEFINED_SELECTOR":                  {},
		"W_PACK_AUTH_MISSING_SELF_PROFILE_ROUTE": {},
		"E_PACK_MODERATION_MISSING_TRANSITIONS":  {},
		"E_PACK_NOTIFY_MISSING_RECIPIENT_SOURCE": {},
		"W_PACK_MISSING_PLANNER_HINTS":           {},
		"W_PACK_MISSING_PLANNER_ROUTE_PATH":      {},
		"W_IR_CANONICAL_PACK_MISMATCH":           {},
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
	for extra := range known {
		if strings.Contains(log, extra) {
			uniq[extra] = struct{}{}
		}
	}
	if strings.Contains(log, "undefined: req.") {
		uniq["GO_UNDEFINED_SELECTOR"] = struct{}{}
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
	all := make([]string, 0, len(compiler.StableErrorCodes)+6)
	all = append(all, "E_FSM_UNDEFINED_STATE")
	all = append(all,
		"W_PACK_AUTH_MISSING_SELF_PROFILE_ROUTE",
		"E_PACK_MODERATION_MISSING_TRANSITIONS",
		"E_PACK_NOTIFY_MISSING_RECIPIENT_SOURCE",
		"W_PACK_MISSING_PLANNER_HINTS",
		"W_PACK_MISSING_PLANNER_ROUTE_PATH",
		"W_IR_CANONICAL_PACK_MISMATCH",
	)
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
		Summary:         buildSummary(log, codes, fixed, remaining, suggestions),
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

func buildSummary(log string, codes []string, fixed int, remaining int, suggestions []Suggestion) []string {
	out := []string{}
	if len(codes) == 0 {
		if strings.TrimSpace(log) == "" {
			return []string{"No build log input was provided."}
		}
		return []string{
			"No structured ANG error codes detected in log.",
			"Run `ang validate` to get structured diagnostics and error codes.",
		}
	}
	out = append(out, fmt.Sprintf("Detected %d structured error code(s).", len(codes)))
	out = append(out, fmt.Sprintf("Errors fixed since previous run: %d.", fixed))
	out = append(out, fmt.Sprintf("Errors remaining: %d.", remaining))
	if len(suggestions) > 0 {
		out = append(out, "Top fix: "+suggestions[0].Fix)
	}
	for _, code := range codes {
		if code == compiler.ErrCodeIRVersionMigration {
			out = append(out, fmt.Sprintf("IR migration hint: ANG auto-migrates legacy IR via %s.", strings.Join(ir.RegisteredMigrations(), ", ")))
			out = append(out, fmt.Sprintf("Expected canonical IR version: %s.", ir.CurrentVersion()))
			break
		}
	}
	return out
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
	case "MISSING_ID":
		return Suggestion{
			Code:         code,
			Fix:          "Add mapping.Assign before repo.Save: {action: \"mapping.Assign\", to: \"<entityVar>.ID\", value: \"uuid.NewString()\"}.",
			CanAutoApply: true,
			Patch: map[string]any{
				"op":    "insert_before_repo_save",
				"field": "ID",
				"value": "uuid.NewString()",
			},
		}
	case "MISSING_CREATED_AT":
		return Suggestion{
			Code:         code,
			Fix:          "Add mapping.Assign before repo.Save: {action: \"mapping.Assign\", to: \"<entityVar>.CreatedAt\", value: \"time.Now().UTC().Format(time.RFC3339)\"}.",
			CanAutoApply: true,
			Patch: map[string]any{
				"op":    "insert_before_repo_save",
				"field": "CreatedAt",
				"value": "time.Now().UTC().Format(time.RFC3339)",
			},
		}
	case "UNKNOWN_ACTION", "E_FLOW_UNKNOWN_ACTION":
		return Suggestion{
			Code:         code,
			Fix:          "Fix typo in action name. Run `ang actions --json` and replace with canonical action (doctor --fix can auto-correct obvious typos with Levenshtein<=2).",
			CanAutoApply: true,
			Patch: map[string]any{
				"op": "rename_unknown_action",
			},
		}
	case "E_FLOW_TOO_LARGE":
		return Suggestion{
			Code:         code,
			Fix:          "Split large flow into sub-operations and call them via flow.Call. Suggested scaffold:\n1) Extract validation block to <Service>.Validate...\n2) Extract persistence block to <Service>.Persist...\n3) Keep orchestration op with flow.Call steps only.",
			CanAutoApply: false,
			Patch: map[string]any{
				"op": "split_flow_scaffold",
			},
		}
	case "W_FLOW_OUTBOX_PREFERRED":
		return Suggestion{
			Code:         code,
			Fix:          "Replace event.Publish with event.Outbox when flow writes DB state in same operation.",
			CanAutoApply: true,
			Patch: map[string]any{
				"op":   "replace_action",
				"from": "event.Publish",
				"to":   "event.Outbox",
			},
		}
	case "W_PACK_AUTH_MISSING_SELF_PROFILE_ROUTE":
		return Suggestion{
			Code:         code,
			Fix:          "Add a canonical self-profile route. Recommended contract: GET /auth/profile -> GetProfile (and keep PUT /auth/profile for UpdateProfile).",
			CanAutoApply: false,
			Patch: map[string]any{
				"op":     "insert_endpoint_scaffold",
				"method": "GET",
				"path":   "/auth/profile",
				"rpc":    "GetProfile",
			},
		}
	case "E_PACK_MODERATION_MISSING_TRANSITIONS":
		return Suggestion{
			Code:         code,
			Fix:          "Add canonical moderation FSM states and transitions: pending -> approved|rejected.",
			CanAutoApply: false,
			Patch: map[string]any{
				"op":          "merge_fsm",
				"states":      []string{"pending", "approved", "rejected"},
				"transitions": map[string]any{"pending": []string{"approved", "rejected"}},
			},
		}
	case "E_PACK_NOTIFY_MISSING_RECIPIENT_SOURCE":
		return Suggestion{
			Code:         code,
			Fix:          "Add explicit recipient source to the notify pack, for example req.Email or notify.Email { to: \"req.Email\" }.",
			CanAutoApply: false,
			Patch: map[string]any{
				"op": "merge_notify_recipient",
				"side_effect": map[string]any{
					"kind": "notify.email",
					"to":   "req.Email",
				},
			},
		}
	case "W_PACK_MISSING_PLANNER_HINTS":
		return Suggestion{
			Code:         code,
			Fix:          "Add planner.source_pack and explicit planner.route / planner.repository bindings so ANG does not rely on fallback pack heuristics.",
			CanAutoApply: false,
			Patch: map[string]any{
				"op": "merge_planner_hints",
				"planner": map[string]any{
					"source_pack": "custom",
					"route": map[string]any{
						"method": "POST",
					},
				},
			},
		}
	case "W_PACK_MISSING_PLANNER_ROUTE_PATH":
		return Suggestion{
			Code:         code,
			Fix:          "Add planner.route.path explicitly so canonical routing is fully declared in planner metadata.",
			CanAutoApply: false,
			Patch: map[string]any{
				"op": "merge_planner_route_path",
				"planner": map[string]any{
					"route": map[string]any{
						"path": "/...",
					},
				},
			},
		}
	case "W_IR_CANONICAL_PACK_MISMATCH":
		return Suggestion{
			Code:         code,
			Fix:          "Align canonical IR metadata: primary_operation_kind, capabilities, side_effects, and route/path should describe the same pack.",
			CanAutoApply: false,
			Patch: map[string]any{
				"op": "align_canonical_pack_metadata",
			},
		}
	case "GO_UNDEFINED_SELECTOR":
		return Suggestion{
			Code:         code,
			Fix:          "Go selector mismatch in generated code (e.g. req.TenderId vs req.TenderID). Align field names to ANG canonical CamelCase/ID style in CUE expressions.",
			CanAutoApply: false,
			Patch: map[string]any{
				"op": "suggest_selector_fix",
			},
		}
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
	case compiler.ErrCodeIRVersionMigration:
		return Suggestion{
			Code:         code,
			Fix:          fmt.Sprintf("Regenerate canonical IR (v%s) from CUE; legacy IR is migrated automatically through %s.", ir.CurrentVersion(), strings.Join(ir.RegisteredMigrations(), ", ")),
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
