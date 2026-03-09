package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/flowsem"
	"github.com/strogmv/ang/compiler/normalizer"
)

// vetDiagsEnvelope is the ang/diags/v1 JSON envelope used by ops vet and validate --json.
type vetDiagsEnvelope struct {
	Schema      string               `json:"schema"`
	Valid       bool                 `json:"valid"`
	Diagnostics []normalizer.Warning `json:"diagnostics"`
	FixReport   []string             `json:"fix_report,omitempty"`
}

// vetDiagWithExplain is one entry in the ang/diags/v2 envelope (--explain flag).
type vetDiagWithExplain struct {
	normalizer.Warning
	Explain *explainItem `json:"explain,omitempty"`
}

// vetDiagsV2Envelope is the ang/diags/v2 JSON envelope (ops vet --json --explain).
type vetDiagsV2Envelope struct {
	Schema      string               `json:"schema"`
	Valid       bool                 `json:"valid"`
	Diagnostics []vetDiagWithExplain `json:"diagnostics"`
	FixReport   []string             `json:"fix_report,omitempty"`
}

type vetRunResult struct {
	Semantic  compiler.NormalizePhaseOutput
	Diags     []normalizer.Warning
	HasErrors bool
}

type opsContextEnvelope struct {
	Schema      string                       `json:"schema"`
	Valid       bool                         `json:"valid"`
	ProjectPath string                       `json:"project_path"`
	Profile     string                       `json:"profile"`
	OpsSchema   opsSchemaEnvelope            `json:"ops_schema"`
	Actions     []flowsem.ActionCatalogEntry `json:"actions"`
	Diagnostics []vetDiagWithExplain         `json:"diagnostics"`
	Proof       ProofReport                  `json:"proof"`
}

func runOps(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: ang ops <schema|vet|context> [flags]")
		fmt.Println("  ang ops schema [--json|--cue]   Machine-readable #Operation schema for AI")
		fmt.Println("  ang ops vet [path] [--json]     Semantic validation of ops CUE files")
		fmt.Println("  ang ops context [path] --json   Unified AI context (schema + actions + diagnostics + proof)")
		os.Exit(1)
	}
	switch args[0] {
	case "schema":
		runOpsSchema(args[1:])
	case "vet":
		runOpsVet(args[1:])
	case "context":
		runOpsContext(args[1:])
	default:
		fmt.Printf("Unknown ops command: %s\n", args[0])
		fmt.Println("Usage: ang ops <schema|vet|context>")
		os.Exit(1)
	}
}

// ── Schema ────────────────────────────────────────────────────────────────────

type opsFieldDef struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

// opsRule is a machine-readable constraint with a stable code.
// AI agents should validate generated CUE against these rules before writing.
type opsRule struct {
	Code    string `json:"code"`
	Desc    string `json:"desc"`
	Enforce string `json:"enforce"` // "error"|"warning"
}

// opsInvalidExample shows a known-bad CUE snippet and why it fails.
type opsInvalidExample struct {
	Label     string `json:"label"`
	CUE       string `json:"cue"`
	Violation string `json:"violation"` // rule code
	Reason    string `json:"reason"`
}

// opsExamples contains concrete CUE strings for AI ground-truth.
type opsExamples struct {
	Minimal string              `json:"minimal"` // smallest valid op
	Full    string              `json:"full"`    // real-world complete op
	Invalid []opsInvalidExample `json:"invalid"` // common mistakes
}

type opsSchemaOperation struct {
	Description    string        `json:"description"`
	RequiredFields []opsFieldDef `json:"required_fields"`
	OptionalFields []opsFieldDef `json:"optional_fields"`
}

type opsSchemaEnvelope struct {
	Schema        string             `json:"schema"`
	Version       string             `json:"version"`
	OpFileGlobs   []string           `json:"op_file_globs"`
	OperationType string             `json:"operation_type"`
	Modes         []string           `json:"modes"`
	Operation     opsSchemaOperation `json:"operation"`
	Rules         []opsRule          `json:"rules"`
	Examples      opsExamples        `json:"examples"`
	ActionsRef    string             `json:"actions_ref"`
}

func buildOpsSchema() opsSchemaEnvelope {
	return opsSchemaEnvelope{
		Schema:        "ang/ops-schema/v2",
		Version:       "2.0",
		OpFileGlobs:   []string{"cue/api/*.cue"},
		OperationType: "schema.#Operation",
		Modes:         []string{"flow", "impl_steps", "impl", "none"},
		Operation: opsSchemaOperation{
			Description: "#Operation defines a single business operation (API method) in CUE intent. " +
				"Each operation belongs to a service and describes its inputs, outputs, and execution flow. " +
				"Operations live in cue/api/*.cue and are compiled to Go handlers by 'ang build'.",
			RequiredFields: []opsFieldDef{
				{
					Name:        "service",
					Type:        "string",
					Required:    true,
					Description: "Target service name — must be lowercase (e.g. \"users\"). Must match an entry in cue/architecture/services.cue.",
				},
				{
					Name:        "input",
					Type:        "object",
					Required:    true,
					Description: "Input parameters as a CUE struct literal. Keys are field names, values are CUE type expressions (string, int, bool, ?, [...string], etc.).",
				},
				{
					Name:        "output",
					Type:        "object",
					Required:    true,
					Description: "Output fields as a CUE struct literal. Same type expressions as input.",
				},
			},
			OptionalFields: []opsFieldDef{
				{
					Name:        "flow",
					Type:        "list[step]",
					Required:    false,
					Description: "Declarative execution flow — ordered list of steps. Each step has 'action' (string) and action-specific args. Preferred over impl_steps. Mutually exclusive with impl and impl_steps.",
				},
				{
					Name:        "impl_steps",
					Type:        "list[step]",
					Required:    false,
					Description: "Deprecated alias for 'flow'. Mutually exclusive with flow and impl. Use 'flow' for all new operations.",
				},
				{
					Name:        "impl",
					Type:        "string",
					Required:    false,
					Description: "Inline Go snippet for custom logic not expressible in flow. Mutually exclusive with flow and impl_steps.",
				},
				{
					Name:        "description",
					Type:        "string",
					Required:    false,
					Description: "Human-readable description of this operation. Included in generated OpenAPI docs.",
				},
				{
					Name:        "throws",
					Type:        "list[string]",
					Required:    false,
					Description: "Error codes this operation can throw. Must match entries in cue/errors/.",
				},
				{
					Name:        "publishes",
					Type:        "list[string]",
					Required:    false,
					Description: "Events published by this operation. Must be defined in cue/events/.",
				},
				{
					Name:        "broadcasts",
					Type:        "list[string]",
					Required:    false,
					Description: "Fire-and-forget events (no subscriber required).",
				},
				{
					Name:        "subscribes",
					Type:        "string",
					Required:    false,
					Description: "Event this operation subscribes to (event-driven handlers only).",
				},
				{
					Name:        "uses",
					Type:        "list[string]",
					Required:    false,
					Description: "External service dependencies (infra adapters, third-party clients).",
				},
				{
					Name:        "pagination",
					Type:        "object",
					Required:    false,
					Description: "Pagination config: {type: \"offset\"} or {type: \"cursor\"}.",
				},
			},
		},
		Rules: []opsRule{
			{
				Code:    "R_REQUIRED_FIELD",
				Desc:    "Fields 'service', 'input', and 'output' must all be present in every operation.",
				Enforce: "error",
			},
			{
				Code:    "R_SERVICE_LOWERCASE",
				Desc:    "service field must be lowercase (e.g. \"users\", not \"Users\" or \"USERS\").",
				Enforce: "error",
			},
			{
				Code:    "R_OP_MODE_XOR",
				Desc:    "At most one of: flow, impl, impl_steps may be present. Omitting all is valid (stub).",
				Enforce: "error",
			},
			{
				Code:    "R_INPUT_OUTPUT_STRUCT",
				Desc:    "input and output must be CUE struct literals (keys: CUE types), not primitives or lists.",
				Enforce: "error",
			},
			{
				Code:    "R_FLOW_ACTION_KNOWN",
				Desc:    "Each step in flow/impl_steps must have a known 'action' value. Run 'ang actions --json' for the full catalog.",
				Enforce: "error",
			},
			{
				Code:    "R_PREFER_FLOW",
				Desc:    "Prefer 'flow' over 'impl_steps' (deprecated alias). Prefer 'flow' over 'impl' when logic is expressible declaratively.",
				Enforce: "warning",
			},
		},
		Examples: opsExamples{
			Minimal: `package api

// Minimal valid operation — no flow, just shape declaration (stub mode).
CreateUser: {
	service: "users"
	input: {
		email: string
		name:  string
	}
	output: {
		id: string
	}
}`,
			Full: `package api

// Full operation with declarative flow.
CreateUser: {
	service:     "users"
	description: "Register a new user account"
	input: {
		email: string
		name:  string
	}
	output: {
		id:        string
		createdAt: string
	}
	flow: [
		{action: "mapping.Map",    entity: "User",   output: "newUser"},
		{action: "mapping.Assign", to: "newUser.ID",        value: "uuid.NewString()"},
		{action: "mapping.Assign", to: "newUser.Email",     value: "req.Email"},
		{action: "mapping.Assign", to: "newUser.Name",      value: "req.Name"},
		{action: "mapping.Assign", to: "newUser.CreatedAt", value: "time.Now().UTC().Format(time.RFC3339)"},
		{action: "repo.Save",      source: "User",   input: "newUser"},
		{action: "mapping.Assign", to: "resp.ID",        value: "newUser.ID"},
		{action: "mapping.Assign", to: "resp.CreatedAt", value: "newUser.CreatedAt"},
	]
	publishes: ["UserCreated"]
	throws:    ["EMAIL_ALREADY_EXISTS"]
}`,
			Invalid: []opsInvalidExample{
				{
					Label:     "missing_required_fields",
					Violation: "R_REQUIRED_FIELD",
					Reason:    "'service' and 'output' are missing; ANG cannot determine where to attach the operation or what to return.",
					CUE: `// WRONG — violates R_REQUIRED_FIELD
CreateUser: {
	input: {
		email: string
	}
}`,
				},
				{
					Label:     "service_not_lowercase",
					Violation: "R_SERVICE_LOWERCASE",
					Reason:    "service value \"Users\" is PascalCase; ANG service names must be lowercase.",
					CUE: `// WRONG — violates R_SERVICE_LOWERCASE
CreateUser: {
	service: "Users"
	input:  {email: string}
	output: {id: string}
}`,
				},
				{
					Label:     "flow_and_impl_steps_both_set",
					Violation: "R_OP_MODE_XOR",
					Reason:    "Both 'flow' and 'impl_steps' are present. Only one execution mode is allowed per operation.",
					CUE: `// WRONG — violates R_OP_MODE_XOR
CreateUser: {
	service: "users"
	input:  {email: string}
	output: {id: string}
	flow:       [{action: "repo.Save", source: "User", input: "newUser"}]
	impl_steps: [{action: "repo.Save", source: "User", input: "newUser"}]
}`,
				},
				{
					Label:     "unknown_flow_action",
					Violation: "R_FLOW_ACTION_KNOWN",
					Reason:    "'database.Insert' is not a registered ANG flow action. Use 'ang actions --json' to find the correct action name.",
					CUE: `// WRONG — violates R_FLOW_ACTION_KNOWN
CreateUser: {
	service: "users"
	input:  {email: string}
	output: {id: string}
	flow: [
		{action: "database.Insert", table: "users", data: "req"}
	]
}`,
				},
			},
		},
		ActionsRef: "ang actions --json",
	}
}

func runOpsSchema(args []string) {
	fs := flag.NewFlagSet("ops schema", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	asJSON := fs.Bool("json", false, "output as JSON (default)")
	asCUE := fs.Bool("cue", false, "output as CUE definition")
	if err := fs.Parse(args); err != nil {
		fmt.Printf("Ops schema FAILED: %v\n", err)
		os.Exit(1)
	}
	if *asJSON && *asCUE {
		fmt.Println("Ops schema FAILED: use only one of --json or --cue")
		os.Exit(1)
	}

	schema := buildOpsSchema()
	if *asCUE {
		fmt.Print(renderOpsSchemaCUE(schema))
		return
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(schema); err != nil {
		fmt.Printf("Ops schema FAILED: %v\n", err)
		os.Exit(1)
	}
}

func renderOpsSchemaCUE(s opsSchemaEnvelope) string {
	var b strings.Builder
	b.WriteString("package ops\n\n")
	b.WriteString("// #Operation — CUE schema for ANG operation definitions\n")
	b.WriteString("// Generated by: ang ops schema --cue  (version " + s.Version + ")\n")
	b.WriteString("// Write operations to: " + strings.Join(s.OpFileGlobs, ", ") + "\n\n")
	b.WriteString("#Operation: {\n")
	b.WriteString("\t// Required fields\n")
	for _, f := range s.Operation.RequiredFields {
		b.WriteString("\t" + f.Name + ": " + cueTypeForOps(f.Type) + "\n")
		b.WriteString("\t// " + f.Description + "\n\n")
	}
	b.WriteString("\t// Optional fields\n")
	for _, f := range s.Operation.OptionalFields {
		b.WriteString("\t" + f.Name + "?: " + cueTypeForOps(f.Type) + "\n")
		b.WriteString("\t// " + f.Description + "\n\n")
	}
	b.WriteString("\t// Rules\n")
	for _, r := range s.Rules {
		b.WriteString("\t// [" + r.Code + "] " + r.Desc + "\n")
	}
	b.WriteString("}\n\n")
	b.WriteString("// Execution modes: " + strings.Join(s.Modes, " | ") + "\n")
	b.WriteString("// Flow actions catalog: run " + strconv.Quote(s.ActionsRef) + "\n\n")
	b.WriteString("// Minimal example:\n")
	for _, line := range strings.Split(s.Examples.Minimal, "\n") {
		b.WriteString("// " + line + "\n")
	}
	return b.String()
}

func cueTypeForOps(t string) string {
	switch t {
	case "string":
		return "string"
	case "object":
		return "{...}"
	case "list[step]":
		return "[...{action: string, ...}]"
	case "list[string]":
		return "[...string]"
	default:
		return "string"
	}
}

// ── Vet ───────────────────────────────────────────────────────────────────────

func runOpsContext(args []string) {
	fs := flag.NewFlagSet("ops context", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	asJSON := fs.Bool("json", false, "output unified AI context as JSON")
	profileRaw := fs.String("profile", string(lintProfileMini), "lint profile: mini|saas|prod")
	migrationMode := fs.Bool("migration-mode", false, "enable facts-driven migration checks (requires --facts)")
	factsPath := fs.String("facts", "", "path to ang/facts/v1 JSON (from `ang extract ...`)")
	if err := fs.Parse(args); err != nil {
		fmt.Printf("Ops context FAILED: %v\n", err)
		os.Exit(1)
	}
	if !*asJSON {
		fmt.Println("Ops context FAILED: --json is required for machine-readable output")
		os.Exit(1)
	}

	profile, err := parseLintProfile(*profileRaw)
	if err != nil {
		fmt.Printf("Ops context FAILED: %v\n", err)
		os.Exit(1)
	}

	projectPath := "."
	if fs.NArg() > 0 {
		projectPath = fs.Arg(0)
	}

	vetRes := collectVetDiagnostics(projectPath, profile, *migrationMode, *factsPath)
	diagsV2 := make([]vetDiagWithExplain, 0, len(vetRes.Diags))
	for _, d := range vetRes.Diags {
		diagsV2 = append(diagsV2, vetDiagWithExplain{
			Warning: d,
			Explain: explainPtrForDiag(d),
		})
	}

	report := BuildProofReport(vetRes.Semantic.Services, vetRes.Semantic.Endpoints)
	payload := opsContextEnvelope{
		Schema:      "ang/ops-context/v1",
		Valid:       !vetRes.HasErrors,
		ProjectPath: projectPath,
		Profile:     string(profile),
		OpsSchema:   buildOpsSchema(),
		Actions:     flowsem.ActionCatalog(),
		Diagnostics: diagsV2,
		Proof:       report,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(payload)
	if vetRes.HasErrors {
		os.Exit(1)
	}
}

func runOpsVet(args []string) {
	fs := flag.NewFlagSet("ops vet", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	asJSON := fs.Bool("json", false, "output diagnostics as JSON (ang/diags/v1)")
	withExplain := fs.Bool("explain", false, "inline AI explanations per diagnostic (ang/diags/v2, implies --json)")
	withProof := fs.Bool("proof", false, "output per-operation proof report (ang/proof/v1)")
	withFix := fs.Bool("fix", false, "apply safe structured suggested fixes and re-run vet")
	profileRaw := fs.String("profile", string(lintProfileMini), "lint profile: mini|saas|prod")
	migrationMode := fs.Bool("migration-mode", false, "enable facts-driven migration checks (requires --facts)")
	factsPath := fs.String("facts", "", "path to ang/facts/v1 JSON (from `ang extract ...`)")
	if err := fs.Parse(args); err != nil {
		fmt.Printf("Ops vet FAILED: %v\n", err)
		os.Exit(1)
	}
	if *withExplain {
		*asJSON = true
	}
	profile, err := parseLintProfile(*profileRaw)
	if err != nil {
		fmt.Printf("Ops vet FAILED: %v\n", err)
		os.Exit(1)
	}

	projectPath := "."
	if fs.NArg() > 0 {
		projectPath = fs.Arg(0)
	}

	vetRes := collectVetDiagnostics(projectPath, profile, *migrationMode, *factsPath)
	fixReport := []string(nil)
	if *withFix {
		report, err := applyVetFixes(projectPath, vetRes.Diags)
		if err != nil {
			fmt.Printf("Ops vet FAILED: apply fix: %v\n", err)
			os.Exit(1)
		}
		fixReport = report
		vetRes = collectVetDiagnostics(projectPath, profile, *migrationMode, *factsPath)
	}

	if *withProof {
		report := BuildProofReport(vetRes.Semantic.Services, vetRes.Semantic.Endpoints)
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
		if vetRes.HasErrors {
			os.Exit(1)
		}
		return
	}

	if *asJSON {
		if vetRes.Diags == nil {
			vetRes.Diags = []normalizer.Warning{}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if *withExplain {
			// ang/diags/v2: each diagnostic carries an inline explain object.
			dv2 := make([]vetDiagWithExplain, 0, len(vetRes.Diags))
			for _, d := range vetRes.Diags {
				dv2 = append(dv2, vetDiagWithExplain{
					Warning: d,
					Explain: explainPtrForDiag(d),
				})
			}
			_ = enc.Encode(vetDiagsV2Envelope{
				Schema:      "ang/diags/v2",
				Valid:       !vetRes.HasErrors,
				Diagnostics: dv2,
				FixReport:   fixReport,
			})
		} else {
			_ = enc.Encode(vetDiagsEnvelope{
				Schema:      "ang/diags/v1",
				Valid:       !vetRes.HasErrors,
				Diagnostics: vetRes.Diags,
				FixReport:   fixReport,
			})
		}
		if vetRes.HasErrors {
			os.Exit(1)
		}
		return
	}

	if len(fixReport) > 0 {
		for _, line := range fixReport {
			fmt.Fprintln(os.Stderr, line)
		}
	}
	emitDiagnostics(os.Stderr, vetRes.Diags)
	if vetRes.HasErrors {
		fmt.Println("Vet FAILED.")
		os.Exit(1)
	}
	fmt.Println("Vet OK.")
}

func collectVetDiagnostics(projectPath string, profile lintProfile, migrationMode bool, factsPath string) vetRunResult {
	semantic, pipelineErr := compiler.RunSemanticPhases(projectPath)
	diags := append([]normalizer.Warning(nil), compiler.LatestDiagnostics...)

	diags = append(diags, runSemanticLints(semantic.Services, semantic.Endpoints, profile)...)
	if migrationMode {
		if strings.TrimSpace(factsPath) == "" {
			diags = append(diags, normalizer.Warning{
				Kind:     "migration",
				Code:     codeMigrationFactsRequired,
				Severity: "error",
				Message:  "--migration-mode requires --facts <path-to-ang-facts-v1.json>",
				Hint:     "Generate facts via `ang extract <src> --from auto --out migration.facts.json` and rerun `ang ops vet --migration-mode --facts migration.facts.json`.",
			})
		} else if facts, err := loadFactsEnvelope(factsPath); err != nil {
			diags = append(diags, normalizer.Warning{
				Kind:     "migration",
				Code:     codeMigrationFactsLoadFailed,
				Severity: "error",
				Message:  fmt.Sprintf("failed to load facts file %q: %v", factsPath, err),
				Hint:     "Ensure file exists, is valid JSON, and has schema ang/facts/v1.",
			})
		} else {
			diags = append(diags, runMigrationLints(semantic.Services, facts, profile)...)
		}
	}

	diags = rewriteDiagnosticSources(diags, semantic.Services)
	diags = enrichUnknownActionDiags(diags)
	diags = enrichSuggestedFixDiags(diags)
	for i := range diags {
		if diags[i].Path == "" {
			diags[i].Path = computeVetPath(diags[i])
		}
	}
	if pipelineErr != nil {
		diags = append(diags, normalizer.Warning{
			Kind:     "pipeline",
			Code:     "PIPELINE_ERROR",
			Severity: "error",
			Message:  pipelineErr.Error(),
		})
	}
	return vetRunResult{
		Semantic:  semantic,
		Diags:     diags,
		HasErrors: hasErrorDiagnostics(diags) || pipelineErr != nil,
	}
}

func hasErrorDiagnostics(diags []normalizer.Warning) bool {
	for _, d := range diags {
		if strings.EqualFold(strings.TrimSpace(d.Severity), "error") {
			return true
		}
	}
	return false
}

func explainPtrForDiag(d normalizer.Warning) *explainItem {
	ex := explainFromInput(explainInput{
		Code:    d.Code,
		Message: d.Message,
		Path:    d.Path,
		Action:  d.Action,
		Hint:    d.Hint,
		File:    d.File,
		Line:    d.Line,
		Column:  d.Column,
	})
	return &ex
}

func enrichSuggestedFixDiags(diags []normalizer.Warning) []normalizer.Warning {
	for i := range diags {
		d := &diags[i]
		for fi := range d.SuggestedFix {
			fx := &d.SuggestedFix[fi]
			if strings.TrimSpace(fx.File) == "" {
				fx.File = d.File
			}
			if strings.TrimSpace(fx.CUEPath) == "" {
				fx.CUEPath = d.CUEPath
			}
			if strings.TrimSpace(fx.Op) == "" {
				if strings.TrimSpace(fx.Kind) != "" {
					fx.Op = fx.Kind
				} else if fx.Value != nil {
					fx.Op = "merge"
				}
			}
		}
		d.CanAutoApply = d.CanAutoApply && isSafeAutoFixCode(d.Code) && strings.TrimSpace(d.File) != "" && d.Line > 0
	}
	return diags
}

func isSafeAutoFixCode(code string) bool {
	switch strings.TrimSpace(code) {
	case "UNKNOWN_ACTION", "E_FLOW_UNKNOWN_ACTION", "W_FLOW_HTTP_NO_TIMEOUT", "NEEDS_QUOTES":
		return true
	default:
		return false
	}
}

func rewriteDiagnosticSources(diags []normalizer.Warning, services []normalizer.Service) []normalizer.Warning {
	sources := buildOperationSourceIndex(services)
	for i := range diags {
		d := &diags[i]
		src, ok := sources[strings.ToLower(strings.TrimSpace(d.Op))]
		if !ok {
			continue
		}
		if shouldOverrideDiagnosticFile(d.File) {
			d.File = src.File
			if src.Line > 0 {
				d.Line = src.Line
			}
			if src.Column > 0 {
				d.Column = src.Column
			}
		}
		for fi := range d.SuggestedFix {
			fx := &d.SuggestedFix[fi]
			if shouldOverrideDiagnosticFile(fx.File) {
				fx.File = src.File
			}
		}
	}
	return diags
}

func applyVetFixes(projectPath string, diags []normalizer.Warning) ([]string, error) {
	root := strings.TrimSpace(projectPath)
	if root == "" {
		root = "."
	}
	attempted := 0
	applied := 0
	seen := make(map[string]struct{})
	for _, d := range diags {
		if !d.CanAutoApply || len(d.SuggestedFix) == 0 {
			continue
		}
		for _, fx := range d.SuggestedFix {
			op := strings.ToLower(strings.TrimSpace(fx.Op))
			if op == "" {
				op = strings.ToLower(strings.TrimSpace(fx.Kind))
			}
			if op != "merge" && op != "replace" {
				continue
			}
			key := fmt.Sprintf("%s|%d|%s|%s|%s", strings.TrimSpace(d.File), d.Line, d.Code, d.Action, op)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			attempted++
			changed, err := applyStructuredFix(root, d, fx)
			if err != nil {
				return nil, err
			}
			if changed {
				applied++
			}
		}
	}
	if attempted == 0 {
		return []string{"Ops vet --fix: no auto-fixable diagnostics detected."}, nil
	}
	return []string{fmt.Sprintf("Ops vet --fix: applied %d/%d suggested fix(es).", applied, attempted)}, nil
}

func applyStructuredFix(root string, d normalizer.Warning, fx normalizer.Fix) (bool, error) {
	path := resolveDoctorPath(root, firstNonEmptyString(fx.File, d.File))
	if strings.TrimSpace(path) == "" {
		return false, nil
	}
	if !isSafeVetFixPath(path) {
		return false, nil
	}
	aroundLine := d.Line
	if aroundLine <= 0 {
		return false, nil
	}
	valueMap, _ := fx.Value.(map[string]any)
	if len(valueMap) == 0 {
		return false, nil
	}

	if toAction, ok := valueMap["action"].(string); ok && strings.TrimSpace(toAction) != "" {
		from := strings.TrimSpace(d.Action)
		if from == "" {
			from = extractActionFromMessage(d.Message)
		}
		if from == "" {
			before := strings.TrimSpace(fx.Before)
			re := regexp.MustCompile(`action:\s*"([^"]+)"`)
			if m := re.FindStringSubmatch(before); len(m) == 2 {
				from = strings.TrimSpace(m[1])
			}
		}
		if from == "" {
			return false, nil
		}
		return replaceActionNearLine(path, aroundLine, from, strings.TrimSpace(toAction))
	}

	if timeout, ok := valueMap["timeout"].(string); ok {
		return addOrReplaceStepFieldNearLine(path, aroundLine, d.Action, "timeout", timeout)
	}

	if assignValue, ok := valueMap["value"].(string); ok && strings.EqualFold(strings.TrimSpace(d.Action), "mapping.Assign") {
		return replaceAssignValueNearLine(path, aroundLine, assignValue)
	}
	return false, nil
}

func addOrReplaceStepFieldNearLine(path string, aroundLine int, action, field, value string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	lines := strings.Split(string(raw), "\n")
	idx := nearestActionLine(lines, aroundLine, action)
	if idx < 0 {
		return false, nil
	}
	if strings.Contains(lines[idx], field+":") {
		return false, nil
	}
	pos := strings.LastIndex(lines[idx], "}")
	quoted := strconv.Quote(value)
	if pos >= 0 {
		lines[idx] = lines[idx][:pos] + ", " + field + ": " + quoted + lines[idx][pos:]
		return true, os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
	}

	indent := leadingWhitespace(lines[idx])
	inserted := indent + "\t" + field + ": " + quoted
	lines = append(lines[:idx+1], append([]string{inserted}, lines[idx+1:]...)...)
	return true, os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

func replaceAssignValueNearLine(path string, aroundLine int, newValue string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	lines := strings.Split(string(raw), "\n")
	idx := nearestActionLine(lines, aroundLine, "mapping.Assign")
	if idx < 0 {
		return false, nil
	}
	reValue := regexp.MustCompile(`value:\s*"([^"\\]|\\.)*"`)
	replaced := false
	quoted := strconv.Quote(newValue)
	end := idx + 6
	if end >= len(lines) {
		end = len(lines) - 1
	}
	for i := idx; i <= end; i++ {
		if !strings.Contains(lines[i], "value:") {
			continue
		}
		next := reValue.ReplaceAllString(lines[i], "value: "+quoted)
		if next == lines[i] {
			continue
		}
		lines[i] = next
		replaced = true
		break
	}
	if !replaced {
		return false, nil
	}
	return true, os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func isSafeVetFixPath(path string) bool {
	clean := filepath.ToSlash(strings.TrimSpace(path))
	if clean == "" {
		return false
	}
	return strings.Contains(clean, "/cue/api/") || strings.HasPrefix(clean, "cue/api/")
}

type diagSource struct {
	File   string
	Line   int
	Column int
}

func buildOperationSourceIndex(services []normalizer.Service) map[string]diagSource {
	out := make(map[string]diagSource)
	for _, svc := range services {
		for _, method := range svc.Methods {
			key := strings.ToLower(strings.TrimSpace(svc.Name) + "." + strings.TrimSpace(method.Name))
			file, line, column := parseSourceLocation(method.Source)
			if file == "" {
				continue
			}
			out[key] = diagSource{File: file, Line: line, Column: column}
		}
	}
	return out
}

func parseSourceLocation(src string) (string, int, int) {
	raw := strings.TrimSpace(src)
	if raw == "" {
		return "", 0, 0
	}
	parts := strings.Split(raw, ":")
	if len(parts) < 2 {
		return raw, 0, 0
	}
	line, _ := strconv.Atoi(strings.TrimSpace(parts[len(parts)-1]))
	file := strings.Join(parts[:len(parts)-1], ":")
	return file, line, 0
}

func shouldOverrideDiagnosticFile(path string) bool {
	clean := filepath.ToSlash(strings.TrimSpace(path))
	if clean == "" {
		return true
	}
	return strings.Contains(clean, "/cue/schema/") || strings.HasPrefix(clean, "cue/schema/")
}
