package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type explainKnowledge struct {
	Title       string
	Description string
	// Fix is a concrete, actionable instruction the AI can execute immediately.
	Fix string
	// ActionRef names the primary flow action involved (e.g. "http.Request").
	// AI can look it up in 'ang actions --json'.
	ActionRef string
	// SchemaRef is the ops-schema rule code violated (e.g. "R_OP_MODE_XOR").
	// AI can look it up in 'ang ops schema --json'.
	SchemaRef string
	Hint      string
	DocAnchor string
	Expected  []string
}

type explainInput struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
	Path    string `json:"path,omitempty"`
	Action  string `json:"action,omitempty"`
	Hint    string `json:"hint,omitempty"`
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
	Column  int    `json:"column,omitempty"`
	Stage   string `json:"stage,omitempty"`
}

type explainItem struct {
	Code        string   `json:"code"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Path        string   `json:"path,omitempty"`
	Expected    []string `json:"expected,omitempty"`
	Found       []string `json:"found,omitempty"`
	Hint        string   `json:"hint,omitempty"`
	// Fix is a concrete, AI-executable instruction (e.g. "Add timeout: \"5s\" to step args").
	Fix string `json:"fix,omitempty"`
	// ActionRef is the primary flow action involved — look up in 'ang actions --json'.
	ActionRef string `json:"action_ref,omitempty"`
	// SchemaRef is the ops-schema rule code violated — look up in 'ang ops schema --json'.
	SchemaRef string `json:"schema_ref,omitempty"`
	DocAnchor string `json:"doc_anchor,omitempty"`
}

type explainEnvelope struct {
	Schema string        `json:"schema"`
	Items  []explainItem `json:"items"`
}

var contractErrRe = regexp.MustCompile(`\[(?P<stage>[A-Z_]+):(?P<code>[A-Z0-9_]+)\]\s*(?P<rest>.*)`)

func runExplain(args []string) {
	jsonOut := false
	var positional []string
	for _, a := range args {
		if strings.TrimSpace(a) == "--json" {
			jsonOut = true
			continue
		}
		positional = append(positional, a)
	}

	if len(positional) == 0 {
		fmt.Println("Usage: ang explain <CODE|error-json|path-to-json> [--json]")
		os.Exit(1)
	}

	rawInput := strings.TrimSpace(positional[0])
	items, err := explainAnyInput(rawInput)
	if err != nil {
		fmt.Printf("Explain FAILED: %v\n", err)
		os.Exit(1)
	}
	if len(items) == 0 {
		fmt.Println("Explain FAILED: no diagnostics found in input")
		os.Exit(1)
	}

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(explainEnvelope{
			Schema: "ang/explain/v2",
			Items:  items,
		})
		return
	}
	for i, it := range items {
		fmt.Printf("[%d] %s — %s\n", i+1, it.Code, it.Title)
		if it.Path != "" {
			fmt.Printf("Path: %s\n", it.Path)
		}
		fmt.Printf("What: %s\n", it.Description)
		if len(it.Expected) > 0 {
			fmt.Printf("Expected: %s\n", strings.Join(it.Expected, "; "))
		}
		if len(it.Found) > 0 {
			fmt.Printf("Found: %s\n", strings.Join(it.Found, "; "))
		}
		if it.Fix != "" {
			fmt.Printf("Fix: %s\n", it.Fix)
		}
		if it.Hint != "" {
			fmt.Printf("Hint: %s\n", it.Hint)
		}
		if it.ActionRef != "" {
			fmt.Printf("Action: %s  (see: ang actions --json | jq '.[] | select(.name==\"%s\")')\n", it.ActionRef, it.ActionRef)
		}
		if it.SchemaRef != "" {
			fmt.Printf("Rule: %s  (see: ang ops schema --json | jq '.rules[] | select(.code==\"%s\")')\n", it.SchemaRef, it.SchemaRef)
		}
		if it.DocAnchor != "" {
			fmt.Printf("Docs: %s\n", it.DocAnchor)
		}
		if i != len(items)-1 {
			fmt.Println()
		}
	}
}

func explainAnyInput(input string) ([]explainItem, error) {
	if input == "" {
		return nil, fmt.Errorf("empty input")
	}
	if raw, ok, err := readMaybeJSON(input); err != nil {
		return nil, err
	} else if ok {
		diags, derr := decodeDiagnostics(raw)
		if derr != nil {
			return nil, derr
		}
		items := make([]explainItem, 0, len(diags))
		for _, d := range diags {
			items = append(items, explainFromInput(d))
		}
		return items, nil
	}
	return []explainItem{explainFromInput(explainInput{Code: strings.ToUpper(strings.TrimSpace(input))})}, nil
}

func readMaybeJSON(input string) ([]byte, bool, error) {
	if strings.HasPrefix(input, "{") || strings.HasPrefix(input, "[") {
		return []byte(input), true, nil
	}
	if input == "-" {
		raw, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, false, fmt.Errorf("read stdin: %w", err)
		}
		return raw, true, nil
	}
	clean := filepath.Clean(input)
	raw, err := os.ReadFile(clean)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read %s: %w", clean, err)
	}
	trim := strings.TrimSpace(string(raw))
	if strings.HasPrefix(trim, "{") || strings.HasPrefix(trim, "[") {
		return []byte(trim), true, nil
	}
	return nil, false, nil
}

func decodeDiagnostics(raw []byte) ([]explainInput, error) {
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("parse json diagnostics: %w", err)
	}
	items := flattenDiagnostics(payload)
	uniq := make(map[string]explainInput, len(items))
	for _, it := range items {
		key := strings.Join([]string{
			strings.TrimSpace(it.Code),
			strings.TrimSpace(it.Path),
			strings.TrimSpace(it.Message),
			strconvI(it.Line),
			strconvI(it.Column),
		}, "|")
		uniq[key] = it
	}
	keys := make([]string, 0, len(uniq))
	for key := range uniq {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]explainInput, 0, len(keys))
	for _, key := range keys {
		out = append(out, uniq[key])
	}
	return out, nil
}

func flattenDiagnostics(v any) []explainInput {
	switch x := v.(type) {
	case []any:
		out := make([]explainInput, 0, len(x))
		for _, item := range x {
			out = append(out, flattenDiagnostics(item)...)
		}
		return out
	case map[string]any:
		// ang/diags/v1 format: {"schema":"ang/diags/v1","valid":...,"diagnostics":[...]}
		if diagnostics, ok := x["diagnostics"].([]any); ok {
			var out []explainInput
			for _, d := range diagnostics {
				if dm, ok := d.(map[string]any); ok {
					out = append(out, diagFromMap(dm))
				}
			}
			return out
		}
		if warnings, ok := x["warnings"].([]any); ok {
			var out []explainInput
			for _, w := range warnings {
				if wm, okMap := w.(map[string]any); okMap {
					out = append(out, diagFromMap(wm))
				}
			}
			if errorsList, okErr := x["errors"].([]any); okErr {
				for _, e := range errorsList {
					if em, okMap := e.(map[string]any); okMap {
						msg := asString(em["message"])
						if parsed, okParsed := parseContractMessage(msg); okParsed {
							out = append(out, parsed)
						}
					}
				}
			}
			return out
		}
		if code := asString(x["code"]); code != "" || asString(x["message"]) != "" {
			return []explainInput{diagFromMap(x)}
		}
		if msg := asString(x["message"]); msg != "" {
			if parsed, ok := parseContractMessage(msg); ok {
				return []explainInput{parsed}
			}
		}
	}
	return nil
}

func diagFromMap(m map[string]any) explainInput {
	it := explainInput{
		Code:    strings.ToUpper(strings.TrimSpace(asString(m["code"]))),
		Message: asString(m["message"]),
		Path:    asString(m["cue_path"]),
		Action:  asString(m["action"]),
		Hint:    asString(m["hint"]),
		File:    asString(m["file"]),
		Line:    asInt(m["line"]),
		Column:  asInt(m["column"]),
	}
	if it.Path == "" {
		step := asInt(m["step"])
		if step > 0 {
			if it.Action != "" {
				it.Path = fmt.Sprintf("flow[%d] (%s)", step, it.Action)
			} else {
				it.Path = fmt.Sprintf("flow[%d]", step)
			}
		}
	}
	if it.Code == "" && it.Message != "" {
		if parsed, ok := parseContractMessage(it.Message); ok {
			if parsed.Code != "" {
				it.Code = parsed.Code
			}
			if parsed.Stage != "" {
				it.Stage = parsed.Stage
			}
		}
	}
	return it
}

func parseContractMessage(msg string) (explainInput, bool) {
	m := contractErrRe.FindStringSubmatch(strings.TrimSpace(msg))
	if len(m) < 4 {
		return explainInput{}, false
	}
	return explainInput{
		Stage:   strings.TrimSpace(m[1]),
		Code:    strings.TrimSpace(m[2]),
		Message: strings.TrimSpace(m[3]),
	}, true
}

func explainFromInput(in explainInput) explainItem {
	code := strings.ToUpper(strings.TrimSpace(in.Code))
	knowledge, ok := explainDB[code]
	if !ok {
		knowledge = inferKnowledge(code, in)
	}

	item := explainItem{
		Code:        code,
		Title:       knowledge.Title,
		Description: knowledge.Description,
		Path:        in.Path,
		Expected:    append([]string(nil), knowledge.Expected...),
		Fix:         knowledge.Fix,
		ActionRef:   knowledge.ActionRef,
		SchemaRef:   knowledge.SchemaRef,
		DocAnchor:   knowledge.DocAnchor,
	}
	found := buildFound(in)
	if len(found) > 0 {
		item.Found = found
	}
	if strings.TrimSpace(in.Hint) != "" {
		item.Hint = in.Hint
	} else {
		item.Hint = knowledge.Hint
	}
	if item.Code == "" {
		item.Code = "UNKNOWN_ERROR"
	}
	if item.Title == "" {
		item.Title = "Unknown diagnostic code"
	}
	if item.Description == "" {
		item.Description = "No built-in explanation yet. Check message details and source location."
	}
	return item
}

func buildFound(in explainInput) []string {
	var out []string
	if in.Action != "" {
		out = append(out, "action="+in.Action)
	}
	if strings.TrimSpace(in.Message) != "" {
		out = append(out, in.Message)
	}
	if in.File != "" && in.Line > 0 {
		out = append(out, fmt.Sprintf("%s:%d:%d", in.File, in.Line, in.Column))
	}
	return out
}

func inferKnowledge(code string, in explainInput) explainKnowledge {
	if field, ok := missingFieldFromCode(code); ok {
		return explainKnowledge{
			Title:       "Missing required field",
			Description: "Flow step is missing a required argument/child.",
			Hint:        "Add the missing field to the step and keep action contract from `ang actions --json`.",
			DocAnchor:   "docs/flow-semantics.md#validation-model",
			Expected:    []string{field + " must be provided"},
		}
	}
	switch {
	case strings.HasPrefix(code, "INVALID_"), strings.HasPrefix(code, "E_FLOW_INVALID_"):
		return explainKnowledge{
			Title:       "Invalid argument value/type",
			Description: "Value does not satisfy action contract or constraint.",
			Hint:        "Check allowed values and types via `ang actions --json` and fix step args.",
			DocAnchor:   "docs/flow-semantics.md#validation-model",
			Expected:    []string{"argument type/value must match action schema"},
		}
	case code == "UNKNOWN_ACTION" || code == "E_FLOW_UNKNOWN_ACTION":
		return explainKnowledge{
			Title:       "Unknown action",
			Description: "Flow action is not registered in compiler semantics.",
			Hint:        "Use canonical action name from `ang actions --json`.",
			DocAnchor:   "docs/flow-semantics.md#action-catalog",
			Expected:    []string{"known action name"},
		}
	case code == "TX_REQUIRED" || code == "E_FLOW_TX_REQUIRED":
		return explainKnowledge{
			Title:       "Transactional context required",
			Description: "Action must run inside `tx.Block`.",
			Hint:        "Wrap this step into `tx.Block`.",
			DocAnchor:   "docs/flow-semantics.md#validation-model",
			Expected:    []string{"step inside tx.Block"},
		}
	case strings.Contains(code, "_ERROR"):
		return explainKnowledge{
			Title:       "Pipeline stage error",
			Description: "Compilation/build phase failed with a stable stage code.",
			Hint:        "Fix root cause from message details and rerun `ang validate`/`ang build`.",
			DocAnchor:   "docs/architecture.md",
			Expected:    []string{"successful phase execution"},
		}
	default:
		_ = in
		return explainKnowledge{
			Title:       "Unknown diagnostic code",
			Description: "No built-in explanation for this code yet.",
			Hint:        "Run `ang actions --json` and verify flow step contracts.",
			DocAnchor:   "docs/flow-semantics.md",
		}
	}
}

func missingFieldFromCode(code string) (string, bool) {
	candidates := []string{
		"MISSING_",
		"E_FLOW_MISSING_",
	}
	for _, p := range candidates {
		if strings.HasPrefix(code, p) {
			field := strings.ToLower(strings.TrimPrefix(code, p))
			if field != "" {
				return field, true
			}
		}
	}
	return "", false
}

func asString(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	default:
		return ""
	}
}

func asInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	default:
		return 0
	}
}

func strconvI(v int) string {
	return fmt.Sprintf("%d", v)
}

var explainDB = map[string]explainKnowledge{
	// ── Semantic lint codes (Layer 2) ──────────────────────────────────────────
	"E_FLOW_TOO_LARGE": {
		Title:       "Flow exceeds step limit",
		Description: "Flow has more than 20 steps, making it hard to maintain, test, and reason about.",
		Fix:         "Split the flow into smaller sub-operations. Extract cohesive groups of steps into separate CUE operations and call them via {action: \"flow.Call\", op: \"<service>.<MethodName>\"}. Alternatively, extract repeated patterns into a shared preset in cue/schema/flow_helpers.cue.",
		SchemaRef:   "R_PREFER_FLOW",
		Hint:        "Split into sub-operations using 'flow.Call', or extract repeated patterns into a shared preset in cue/schema/flow_helpers.cue.",
		DocAnchor:   "docs/flow-semantics.md#flow-size",
		Expected:    []string{"flow with ≤20 steps"},
	},
	"W_FLOW_HTTP_NO_TIMEOUT": {
		Title:       "HTTP action missing timeout",
		Description: "An http.* step has no timeout — external HTTP calls can hang indefinitely, blocking the handler goroutine.",
		Fix:         "Add timeout: \"5s\" (or an appropriate value) to the step args. Example: {action: \"http.Request\", url: \"...\", timeout: \"5s\"}",
		ActionRef:   "http.Request",
		SchemaRef:   "R_FLOW_ACTION_KNOWN",
		Hint:        "Add timeout to the http step. Recommended: 3–30 s depending on SLA.",
		DocAnchor:   "docs/flow-semantics.md#http",
		Expected:    []string{"timeout field present in http.* step args"},
	},
	"W_FLOW_OUTBOX_PREFERRED": {
		Title:       "Event publish without transactional outbox",
		Description: "The flow writes to DB (repo.Save/Upsert/Update) and publishes an event (event.Publish/Broadcast) without using the outbox pattern. If the process crashes after the DB commit but before the publish, the event is silently lost.",
		Fix:         "Replace event.Publish with event.Outbox inside a tx.Block. Pattern:\n  {action: \"tx.Block\", do: [\n    {action: \"repo.Save\", ...},\n    {action: \"event.Outbox\", event: \"<EventName>\", payload: \"...\"},\n  ]}\nThe outbox ensures the event is stored atomically and delivered at-least-once.",
		ActionRef:   "event.Outbox",
		SchemaRef:   "R_PREFER_FLOW",
		Hint:        "Use event.Outbox inside tx.Block instead of event.Publish.",
		DocAnchor:   "docs/flow-semantics.md#transactional-outbox",
		Expected:    []string{"event.Outbox inside tx.Block when combining DB write and event publish"},
	},
	"E_FLOW_INVALID_NESTING_KEYS": {
		Title:       "Invalid nested block for action",
		Description: "A flow step uses structural nested keys (_then/_do/_catch/...) that are not supported by this action.",
		Fix:         "Remove unsupported nested keys from the step. Use only nested keys listed in diagnostics.allowed for this action.",
		SchemaRef:   "R_FLOW_ACTION_KNOWN",
		Hint:        "Keep only allowed structural keys for the action (see diagnostics.allowed).",
		DocAnchor:   "docs/flow-semantics.md#control-flow-actions",
		Expected:    []string{"only action-supported nested keys are used"},
	},
	"E_DB_WRITE_WITHOUT_TX_WHEN_REQUIRED": {
		Title:       "DB mutation requires tx.Block",
		Description: "The flow has multiple DB writes (or lock + write) outside transaction boundary, risking partial commit and data races.",
		Fix:         "Wrap related DB writes/locks into a single tx.Block with do: [...].",
		ActionRef:   "tx.Block",
		Hint:        "Use {action: \"tx.Block\", do: [...]} for write bundles and row locks.",
		DocAnchor:   "docs/flow-semantics.md#authoring-rules-recommended",
		Expected:    []string{"multi-write and lock+write sequences are inside tx.Block"},
	},
	"E_MUTATING_ENDPOINT_WITHOUT_IDEMPOTENCY": {
		Title:       "Mutating endpoint lacks idempotency",
		Description: "A mutating HTTP operation writes to DB without idempotency/dedup protection, so retries can duplicate side effects.",
		Fix:         "Add idempotency.DeriveKey + idempotency.Check + idempotency.SaveResult (or dedupe.Once) before DB write steps.",
		ActionRef:   "idempotency.DeriveKey",
		Hint:        "Use idempotency guard for POST/PUT/PATCH/DELETE handlers with DB writes.",
		DocAnchor:   "docs/flow-semantics.md#authoring-rules-recommended",
		Expected:    []string{"idempotency guard around mutating operation"},
	},
	"E_PROTECTED_OPERATION_WITHOUT_AUTHZ": {
		Title:       "Missing auth/authz guard",
		Description: "Mutating external operation has no endpoint auth policy and no authz guard in flow.",
		Fix:         "Add endpoint auth (jwt/roles/permission) or explicit guard step at flow start: auth.RequireRole / rbac.CheckPermission.",
		ActionRef:   "auth.RequireRole",
		Hint:        "Canonical order: auth -> validate -> rate/idem -> business.",
		DocAnchor:   "docs/flow-semantics.md#authoring-rules-recommended",
		Expected:    []string{"auth or permission gate is present for protected mutating operation"},
	},
	"E_MIGRATION_FACTS_REQUIRED": {
		Title:       "Migration mode requires facts file",
		Description: "Facts-driven migration lint was enabled, but no `ang/facts/v1` JSON was provided.",
		Fix:         "Run `ang extract <source-path> --from auto --out migration.facts.json`, then rerun `ang ops vet --migration-mode --facts migration.facts.json`.",
		SchemaRef:   "R_PREFER_FLOW",
		Hint:        "Provide `--facts <path>` when using `--migration-mode`.",
		DocAnchor:   "docs/ai-playbook.md#mcp-first",
		Expected:    []string{"--facts points to a valid ang/facts/v1 JSON"},
	},
	"E_MIGRATION_FACTS_LOAD_FAILED": {
		Title:       "Facts file cannot be loaded",
		Description: "The file passed to `--facts` is missing, invalid JSON, or has an unsupported schema.",
		Fix:         "Regenerate facts with `ang extract ... --out migration.facts.json` and verify `schema` is `ang/facts/v1`.",
		SchemaRef:   "R_PREFER_FLOW",
		Hint:        "Use a valid JSON file generated by `ang extract`.",
		DocAnchor:   "docs/commands.md#ang-extract",
		Expected:    []string{"readable JSON with schema `ang/facts/v1`"},
	},
	"E_MIGRATION_GAP_WITHOUT_OPEN_QUESTION": {
		Title:       "Fact gap without unresolved marker",
		Description: "Flow references data not present in extracted facts, but does not mark uncertainty (`unknown.*`/`openQuestion`). This indicates likely hallucinated business logic.",
		Fix:         "Replace uncertain expressions with explicit placeholders (`unknown.<name>` or `openQuestion: \"...\"`) and keep operation in draft until facts are completed.",
		SchemaRef:   "R_PREFER_FLOW",
		Hint:        "Never invent missing inputs; mark gaps explicitly.",
		DocAnchor:   "docs/ai-playbook.md#prompting-pattern-for-agents",
		Expected:    []string{"all unresolved business facts are marked as unknown/openQuestion"},
	},
	"W_MIGRATION_GAP_MARKED": {
		Title:       "Fact gap is explicitly marked",
		Description: "Flow has unresolved data gaps, but they are properly marked with `unknown.*` or `openQuestion`.",
		Fix:         "Complete source facts and replace placeholders with validated fields from `ang/facts/v1`.",
		SchemaRef:   "R_PREFER_FLOW",
		Hint:        "Keep markers until source facts are confirmed.",
		DocAnchor:   "docs/ai-playbook.md#prompting-pattern-for-agents",
		Expected:    []string{"placeholder markers are eventually replaced by confirmed facts"},
	},

	// ── Compiler / flow codes ──────────────────────────────────────────────────
	"MISSING_ID": {
		Title:       "Missing ID assignment before repo.Save",
		Description: "New entity must have deterministic ID before save.",
		Fix:         "Add {action: \"mapping.Assign\", to: \"<entity>.ID\", value: \"uuid.NewString()\"} before the repo.Save step.",
		ActionRef:   "mapping.Assign",
		Hint:        `Add mapping.Assign: {to: "newEntity.ID", value: "uuid.NewString()"}`,
		DocAnchor:   "docs/flow-semantics.md#authoring-rules-recommended",
		Expected:    []string{"mapping.Assign for <entity>.ID before repo.Save"},
	},
	"MISSING_CREATED_AT": {
		Title:       "Missing CreatedAt assignment before repo.Save",
		Description: "CreatedAt should be explicit and normalized for sorting and audit.",
		Fix:         "Add {action: \"mapping.Assign\", to: \"<entity>.CreatedAt\", value: \"time.Now().UTC().Format(time.RFC3339)\"} before the repo.Save step.",
		ActionRef:   "mapping.Assign",
		Hint:        `Set CreatedAt using RFC3339 UTC before save`,
		DocAnchor:   "docs/flow-semantics.md#authoring-rules-recommended",
		Expected:    []string{"mapping.Assign for <entity>.CreatedAt"},
	},
	"MISSING_OUTPUT": {
		Title:       "Missing output variable",
		Description: "Step must declare output variable to store produced value.",
		Fix:         "Add output: \"<varName>\" to the step args. The variable can then be referenced in subsequent steps.",
		Hint:        "Add output field in step args.",
		DocAnchor:   "docs/flow-semantics.md#validation-model",
		Expected:    []string{"output field is present"},
	},
	"MISSING_INPUT": {
		Title:       "Missing input argument",
		Description: "Repository/mapping step requires input expression.",
		Fix:         "Add input: \"<varName>\" to the step args referencing the variable holding the entity/value.",
		ActionRef:   "repo.Save",
		Hint:        "Provide `input` field in step args.",
		DocAnchor:   "docs/flow-semantics.md#validation-model",
		Expected:    []string{"input field is present"},
	},
	"MISSING_SOURCE": {
		Title:       "Missing source argument",
		Description: "Repository step requires source entity/repository.",
		Fix:         "Add source: \"<EntityName>\" to the step args (e.g. source: \"User\").",
		ActionRef:   "repo.Save",
		Hint:        "Provide `source` field with entity name.",
		DocAnchor:   "docs/flow-semantics.md#validation-model",
		Expected:    []string{"source field is present"},
	},
	"MISSING_ENTITY": {
		Title:       "Missing entity in mapping.Map",
		Description: "mapping.Map requires entity for typed object creation.",
		Fix:         "Add entity: \"<EntityName>\" to the mapping.Map step args (e.g. entity: \"User\").",
		ActionRef:   "mapping.Map",
		Hint:        "Set `entity` in mapping.Map step.",
		DocAnchor:   "docs/flow-semantics.md#validation-model",
		Expected:    []string{"entity field is present"},
	},
	"UNKNOWN_ACTION": {
		Title:       "Unknown flow action",
		Description: "Action is not part of registered stdlib.",
		Fix:         "Run 'ang actions --json' to see all registered actions and replace the typo/unknown name with the correct canonical action.",
		SchemaRef:   "R_FLOW_ACTION_KNOWN",
		Hint:        "Use `ang actions --json` to pick canonical action.",
		DocAnchor:   "docs/flow-semantics.md#action-catalog",
		Expected:    []string{"known flow action"},
	},
	"E_FLOW_UNKNOWN_ACTION": {
		Title:       "Unknown flow action",
		Description: "Action is not part of registered stdlib.",
		Fix:         "Run 'ang actions --json' to see all registered actions and replace the typo/unknown name with the correct canonical action. The diagnostic's 'allowed' field lists same-namespace suggestions.",
		SchemaRef:   "R_FLOW_ACTION_KNOWN",
		Hint:        "Use `ang actions --json` to pick canonical action. Check 'allowed' field for suggestions.",
		DocAnchor:   "docs/flow-semantics.md#action-catalog",
		Expected:    []string{"known flow action"},
	},
	"TX_REQUIRED": {
		Title:       "Action requires transaction",
		Description: "This action can only execute inside `tx.Block`.",
		Fix:         "Wrap the step inside tx.Block: {action: \"tx.Block\", do: [{action: \"<this-action>\", ...}]}",
		ActionRef:   "tx.Block",
		Hint:        "Wrap step in `tx.Block { do: [...] }`.",
		DocAnchor:   "docs/flow-semantics.md#validation-model",
		Expected:    []string{"step located inside tx.Block"},
	},
	"MISSING_CONTENT": {
		Title:       "notify.Send needs template or text",
		Description: "notify.Send step requires either a template reference or inline text content.",
		Fix:         "Add text: \"Your message here\" or template: \"<template-name>\" to the notify.Send step args.",
		ActionRef:   "notify.Send",
		Hint:        `Add text: "..." or template: "..." to the notify.Send step args.`,
		DocAnchor:   "docs/flow-semantics.md#notify",
		Expected:    []string{"text or template field present in notify.Send args"},
	},
	"MISSING_APPROVERS": {
		Title:       "Approval requires non-empty approvers",
		Description: "approval.Request step must specify at least one approver.",
		Fix:         "Add approvers: [\"user@example.com\"] to the approval.Request step args.",
		ActionRef:   "approval.Request",
		Hint:        `Add approvers: ["user@example.com"] to the approval step.`,
		DocAnchor:   "docs/flow-semantics.md#approval",
		Expected:    []string{"non-empty approvers list"},
	},
	"INVALID_APPROVERS_TYPE": {
		Title:       "Approvers must be string or list",
		Description: "The approvers field accepts a single string or a list of strings.",
		Fix:         "Change approvers to a string or a list of strings: approvers: \"user@example.com\" or approvers: [\"a@b.com\"].",
		ActionRef:   "approval.Request",
		Hint:        "Use approvers: \"user@example.com\" or approvers: [\"a@b.com\", \"c@d.com\"].",
		DocAnchor:   "docs/flow-semantics.md#approval",
		Expected:    []string{"string or []string for approvers"},
	},
	"INVALID_POLICY": {
		Title:       "Approval policy must be any/all/quorum",
		Description: "The policy field on approval steps must be one of: any, all, quorum.",
		Fix:         "Set policy to one of the allowed values: policy: \"any\", policy: \"all\", or policy: \"quorum\".",
		ActionRef:   "approval.Request",
		SchemaRef:   "R_OP_MODE_XOR",
		Hint:        `Set policy: "any" or policy: "all" or policy: "quorum".`,
		DocAnchor:   "docs/flow-semantics.md#approval",
		Expected:    []string{"policy is one of: any, all, quorum"},
	},
	"MISSING_BLOCK": {
		Title:       "Required child block missing",
		Description: "A compound step is missing a required child block (e.g. _do, _then, _else).",
		Fix:         "Add the missing child block. Run 'ang actions --json' to see the contract for this compound step.",
		Hint:        "Add the missing block. Check `ang actions --json` for step contract.",
		DocAnchor:   "docs/flow-semantics.md#compound-steps",
		Expected:    []string{"required child block present (_do, _then, etc.)"},
	},
	"MISSING_FIELD": {
		Title:       "Required step argument missing",
		Description: "A flow step is missing a required argument from its action contract.",
		Fix:         "Run 'ang actions --json | jq \".[] | select(.name==\\\"<action>\\\")\"' to find required fields and add the missing one to the step.",
		Hint:        "Check the step contract via `ang actions --json` and add the missing field.",
		DocAnchor:   "docs/flow-semantics.md#validation-model",
		Expected:    []string{"all required action arguments present"},
	},
}
