package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/normalizer"
)

// vetDiagsEnvelope is the ang/diags/v1 JSON envelope used by ops vet and validate --json.
type vetDiagsEnvelope struct {
	Schema      string               `json:"schema"`
	Valid       bool                 `json:"valid"`
	Diagnostics []normalizer.Warning `json:"diagnostics"`
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
}

func runOps(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: ang ops <schema|vet> [flags]")
		fmt.Println("  ang ops schema [--json|--cue]   Machine-readable #Operation schema for AI")
		fmt.Println("  ang ops vet [path] [--json]     Semantic validation of ops CUE files")
		os.Exit(1)
	}
	switch args[0] {
	case "schema":
		runOpsSchema(args[1:])
	case "vet":
		runOpsVet(args[1:])
	default:
		fmt.Printf("Unknown ops command: %s\n", args[0])
		fmt.Println("Usage: ang ops <schema|vet>")
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

func runOpsVet(args []string) {
	fs := flag.NewFlagSet("ops vet", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	asJSON := fs.Bool("json", false, "output diagnostics as JSON (ang/diags/v1)")
	withExplain := fs.Bool("explain", false, "inline AI explanations in JSON output (ang/diags/v2, implies --json)")
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

	semantic, pipelineErr := compiler.RunSemanticPhases(projectPath)
	diags := append([]normalizer.Warning(nil), compiler.LatestDiagnostics...)

	// Layer 2: semantic lints (flow size, http timeout, outbox pattern)
	diags = append(diags, runSemanticLints(semantic.Services, semantic.Endpoints, profile)...)
	if *migrationMode {
		if strings.TrimSpace(*factsPath) == "" {
			diags = append(diags, normalizer.Warning{
				Kind:     "migration",
				Code:     codeMigrationFactsRequired,
				Severity: "error",
				Message:  "--migration-mode requires --facts <path-to-ang-facts-v1.json>",
				Hint:     "Generate facts via `ang extract <src> --from auto --out migration.facts.json` and rerun `ang ops vet --migration-mode --facts migration.facts.json`.",
			})
		} else if facts, err := loadFactsEnvelope(*factsPath); err != nil {
			diags = append(diags, normalizer.Warning{
				Kind:     "migration",
				Code:     codeMigrationFactsLoadFailed,
				Severity: "error",
				Message:  fmt.Sprintf("failed to load facts file %q: %v", *factsPath, err),
				Hint:     "Ensure file exists, is valid JSON, and has schema ang/facts/v1.",
			})
		} else {
			diags = append(diags, runMigrationLints(semantic.Services, facts, profile)...)
		}
	}

	// Enrich unknown-action warnings with Allowed list + did-you-mean hint
	diags = enrichUnknownActionDiags(diags)

	// Compute human-readable path for every diagnostic
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

	hasErrors := pipelineErr != nil
	for _, d := range diags {
		if strings.ToLower(d.Severity) == "error" {
			hasErrors = true
			break
		}
	}

	if *asJSON {
		if diags == nil {
			diags = []normalizer.Warning{}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(vetDiagsEnvelope{
			Schema:      "ang/diags/v1",
			Valid:       !hasErrors,
			Diagnostics: diags,
		})
		if hasErrors {
			os.Exit(1)
		}
		return
	}

	emitDiagnostics(os.Stderr, diags)
	if hasErrors {
		fmt.Println("Vet FAILED.")
		os.Exit(1)
	}
	fmt.Println("Vet OK.")
}
