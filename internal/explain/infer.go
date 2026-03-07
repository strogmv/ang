package explain

import (
	"fmt"
	"strings"
)

func inferKnowledge(code string, in Input) knowledge {
	if k, ok := parseCUEErrorMessage(in.Message); ok {
		return k
	}
	if field, ok := missingFieldFromCode(code); ok {
		return knowledge{
			Title:       "Missing required field",
			Description: "Flow step is missing a required argument/child.",
			Hint:        "Add the missing field to the step and keep action contract from `ang actions --json`.",
			DocAnchor:   "docs/flow-semantics.md#validation-model",
			Expected:    []string{field + " must be provided"},
		}
	}
	switch {
	case strings.HasPrefix(code, "INVALID_"), strings.HasPrefix(code, "E_FLOW_INVALID_"):
		return knowledge{
			Title:       "Invalid argument value/type",
			Description: "Value does not satisfy action contract or constraint.",
			Hint:        "Check allowed values and types via `ang actions --json` and fix step args.",
			DocAnchor:   "docs/flow-semantics.md#validation-model",
			Expected:    []string{"argument type/value must match action schema"},
		}
	case code == "UNKNOWN_ACTION" || code == "E_FLOW_UNKNOWN_ACTION":
		return knowledge{
			Title:       "Unknown action",
			Description: "Flow action is not registered in compiler semantics.",
			Hint:        "Use canonical action name from `ang actions --json`.",
			DocAnchor:   "docs/flow-semantics.md#action-catalog",
			Expected:    []string{"known action name"},
		}
	case code == "TX_REQUIRED" || code == "E_FLOW_TX_REQUIRED":
		return knowledge{
			Title:       "Transactional context required",
			Description: "Action must run inside `tx.Block`.",
			Hint:        "Wrap this step into `tx.Block`.",
			DocAnchor:   "docs/flow-semantics.md#validation-model",
			Expected:    []string{"step inside tx.Block"},
		}
	case strings.Contains(code, "_ERROR"):
		return knowledge{
			Title:       "Pipeline stage error",
			Description: "Compilation/build phase failed with a stable stage code.",
			Hint:        "Fix root cause from message details and rerun `ang validate`/`ang build`.",
			DocAnchor:   "docs/architecture.md",
			Expected:    []string{"successful phase execution"},
		}
	default:
		_ = in
		return knowledge{
			Title:       "Unknown diagnostic code",
			Description: "No built-in explanation for this code yet.",
			Hint:        "Run `ang actions --json` and verify flow step contracts.",
			DocAnchor:   "docs/flow-semantics.md",
		}
	}
}

func parseCUEErrorMessage(msg string) (knowledge, bool) {
	m := strings.TrimSpace(msg)
	if m == "" {
		return knowledge{}, false
	}
	lm := strings.ToLower(m)

	switch {
	case strings.Contains(lm, "field") && strings.Contains(lm, "not allowed"):
		field := extractQuotedWord(m)
		desc := "Field is not part of the action schema."
		fix := "Remove/rename the field. Run `ang actions --json` to view valid fields for this action."
		if field != "" {
			desc = fmt.Sprintf("Field %q is not part of the action schema.", field)
			fix = fmt.Sprintf("Remove or rename field %q. Run `ang actions --json` to see valid fields for this action.", field)
		}
		return knowledge{
			Title:       "Typo in flow step field name",
			Description: desc,
			Fix:         fix,
			Hint:        "Common typos: soruce→source, ouput→output, conidtion→condition",
			DocAnchor:   "docs/flow-semantics.md#validation-model",
			Expected:    []string{"only action-supported fields are present"},
		}, true

	case strings.Contains(lm, "cannot unify") && strings.Contains(m, "#RepoGetStep"):
		return knowledge{
			Title:       "Wrong repo step structure",
			Description: "CUE cannot unify the provided step with #RepoGetStep contract.",
			Fix:         "Use: {action: \"repo.Get\", source: \"EntityName\", input: \"req.ID\", output: \"entity\"}",
			ActionRef:   "repo.Get",
			Hint:        "Check step shape and required fields for repo.Get.",
			DocAnchor:   "docs/flow-semantics.md#validation-model",
			Expected:    []string{"repo.Get with source/input/output"},
		}, true

	case strings.Contains(lm, "conflicting values") && strings.Contains(lm, "string"):
		return knowledge{
			Title:       "Wrong value type in flow step",
			Description: "CUE detected conflicting values where string type was expected.",
			Fix:         "Check field type in `ang actions --json`. String fields need proper quoting: value: \"\\\"literal\\\"\" or value: \"req.Field\".",
			Hint:        "Do not pass non-string values into string-typed action arguments.",
			DocAnchor:   "docs/flow-semantics.md#validation-model",
			Expected:    []string{"value type matches action field type"},
		}, true

	case strings.Contains(lm, "does not exist"):
		return knowledge{
			Title:       "Reference to undefined variable",
			Description: "Expression references a variable that was never declared in flow scope.",
			Fix:         "Declare variable before use (e.g. add repo.Find/repo.Get/mapping.Map step earlier in flow).",
			Hint:        "Flow variables must be produced by previous steps (output/declarations).",
			DocAnchor:   "docs/flow-semantics.md#validation-model",
			Expected:    []string{"referenced variables are declared before use"},
		}, true

	case strings.Contains(lm, "invalid interpolation"):
		return knowledge{
			Title:       "Invalid CUE interpolation",
			Description: "String interpolation references invalid/missing symbol or has malformed syntax.",
			Fix:         "Check interpolation placeholders and available scope. Ensure each \\(expr) references a declared symbol and valid path.",
			Hint:        "Example: \"resp.\\(_field)\" requires _field to exist in current CUE struct scope.",
			DocAnchor:   "docs/flow-semantics.md#validation-model",
			Expected:    []string{"all interpolation placeholders resolve to valid symbols"},
		}, true

	case strings.Contains(lm, "incomplete value"):
		return knowledge{
			Title:       "Incomplete CUE value",
			Description: "CUE value is not fully concrete where compiler expects a final value.",
			Fix:         "Provide a concrete value for required fields (or defaults) at this location. Remove unresolved disjunctions/placeholders.",
			Hint:        "Run `ang validate` and fill missing required fields in the referenced operation/entity.",
			DocAnchor:   "docs/flow-semantics.md#validation-model",
			Expected:    []string{"required fields resolved to concrete values"},
		}, true

	case strings.Contains(lm, "incompatible list lengths"),
		strings.Contains(lm, "list length mismatch"),
		strings.Contains(lm, "mismatched list lengths"):
		return knowledge{
			Title:       "List length mismatch",
			Description: "CUE detected conflicting list constraints/values with different lengths.",
			Fix:         "Align list lengths across merged values, or avoid position-based unification by using keyed maps where possible.",
			Hint:        "Check overlays/patches that redefine the same list with different element counts.",
			DocAnchor:   "docs/flow-semantics.md#validation-model",
			Expected:    []string{"unified list constraints have compatible lengths"},
		}, true
	}

	return knowledge{}, false
}

func extractQuotedWord(msg string) string {
	match := quotedWordRe.FindStringSubmatch(msg)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
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
