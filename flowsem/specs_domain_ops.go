package flowsem

import "strings"

var specsDomainOps = map[string]Spec{
	"audit.Log": {
		RequiredArgs: []string{"actor", "company", "event"},
	},
	"fsm.Transition": {
		RequiredArgs: []string{"entity", "to"},
	},
	"auth.RequireRole": {
		RequiredArgs: []string{"userID", "companyID", "roles"},
	},
	"auth.CheckRole": {
		RequiredArgs: []string{"user", "roles"},
	},
	"rbac.CheckPermission": {
		RequiredArgs: []string{"user", "permission"},
		OptionalArgKinds: map[string]ArgKind{
			"throw":  ArgKindString,
			"code":   ArgKindString,
			"status": ArgKindString,
			"output": ArgKindString,
		},
		DeclaresFromArgs: []string{"output"},
	},
	"entity.PatchNonZero": {
		RequiredArgs: []string{"target", "from", "fields"},
	},
	"entity.PatchValidated": {
		RequiredArgs: []string{"target", "from"},
		OptionalArgKinds: map[string]ArgKind{
			"fields": ArgKindFieldsRuleMap,
		},
		CustomConstraints: func(step Step) *Issue {
			fields, ok := step.Args["fields"].(map[string]map[string]string)
			if !ok || len(fields) == 0 {
				return &Issue{
					Code:    "MISSING_FIELDS",
					Message: "entity.PatchValidated requires non-empty 'fields' map",
					Hint:    "{action: \"entity.PatchValidated\", fields: { Email: { normalize: \"lower\" } }}",
				}
			}
			for fieldName, rules := range fields {
				if strings.TrimSpace(fieldName) == "" {
					return &Issue{
						Code:    "INVALID_FIELD_NAME",
						Message: "entity.PatchValidated contains empty field name",
						Hint:    "{action: \"entity.PatchValidated\", fields: { Email: { ... } }}",
					}
				}
				if normalize := strings.TrimSpace(rules["normalize"]); normalize != "" &&
					normalize != "trim" && normalize != "lower" && normalize != "upper" {
					return &Issue{
						Code:    "INVALID_NORMALIZE",
						Message: "entity.PatchValidated has invalid normalize rule",
						Hint:    "{ normalize: \"trim\" | \"lower\" | \"upper\" }",
					}
				}
				if formatRule := strings.TrimSpace(rules["format"]); formatRule != "" &&
					formatRule != "email" && formatRule != "phone" {
					return &Issue{
						Code:    "INVALID_FORMAT",
						Message: "entity.PatchValidated has invalid format rule",
						Hint:    "{ format: \"email\" | \"phone\" }",
					}
				}
				if uniqueMethod := strings.TrimSpace(rules["unique"]); uniqueMethod != "" &&
					!strings.HasPrefix(uniqueMethod, "FindBy") {
					return &Issue{
						Code:    "INVALID_UNIQUE_METHOD",
						Message: "entity.PatchValidated unique method should start with FindBy",
						Hint:    "{ unique: \"FindByTaxID\" }",
					}
				}
			}
			return nil
		},
	},
	"field.CopyNonEmpty": {
		RequiredArgs: []string{"from", "to"},
	},
	"enum.Validate": {
		RequiredArgs: []string{"value", "allowed", "throw"},
	},
	// OS / system operations
	"exec.Run": {
		RequiredArgs:     []string{"cmd"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"timeout":   ArgKindString,
			"timeoutMs": ArgKindInt,
		},
		CustomConstraints: func(step Step) *Issue {
			if timeoutMS, ok := intArg(step.Args, "timeoutMs"); ok && timeoutMS <= 0 {
				return &Issue{
					Code:    "INVALID_TIMEOUTMS",
					Message: "exec.Run timeoutMs must be > 0",
					Hint:    "{action: \"exec.Run\", cmd: \"...\", timeoutMs: 120000}",
				}
			}
			return nil
		},
	},
	"exec.Stream": {
		RequiredArgs:     []string{"cmd"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"timeout":   ArgKindString,
			"timeoutMs": ArgKindInt,
		},
		CustomConstraints: func(step Step) *Issue {
			if timeoutMS, ok := intArg(step.Args, "timeoutMs"); ok && timeoutMS <= 0 {
				return &Issue{
					Code:    "INVALID_TIMEOUTMS",
					Message: "exec.Stream timeoutMs must be > 0",
					Hint:    "{action: \"exec.Stream\", cmd: \"...\", timeoutMs: 120000}",
				}
			}
			return nil
		},
	},
	"fs.TempDir": {
		RequiredArgs:     []string{"output"},
		DeclaresFromArgs: []string{"output"},
	},
	"fs.WriteFile": {
		RequiredArgs: []string{"path", "data"},
	},
	"fs.ReadFile": {
		RequiredArgs:     []string{"path", "output"},
		DeclaresFromArgs: []string{"output"},
	},
	"fs.Remove": {
		RequiredArgs: []string{"path"},
	},
	"archive.ZipDir": {
		RequiredArgs:     []string{"path", "output"},
		DeclaresFromArgs: []string{"output"},
	},
	"session.Get": {
		RequiredArgs:     []string{"output"},
		DeclaresFromArgs: []string{"output"},
	},
	// Stage 2: Infrastructure actions
}
