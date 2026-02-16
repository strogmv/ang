package flowsem

import "strings"

type Step struct {
	Action   string
	Args     map[string]any
	Children map[string][]Step
	File     string
	Line     int
	Column   int
	CUEPath  string
}

type Issue struct {
	Step     int
	Action   string
	Code     string
	Message  string
	Hint     string
	File     string
	Line     int
	Column   int
	CUEPath  string
	Severity string
}

type Spec struct {
	RequiredArgs      []string
	RequiredChildren  []string
	DeclaresFromArgs  []string
	RequiresTx        bool
	CustomConstraints func(step Step) *Issue
}

var specs = map[string]Spec{
	"logic.Check": {
		RequiredArgs: []string{"condition", "throw"},
	},
	"event.Publish": {
		RequiredArgs: []string{"name"},
	},
	"notification.Dispatch": {
		RequiredArgs: []string{"message"},
	},
	"repo.GetForUpdate": {
		RequiresTx: true,
	},
	"repo.Query": {
		RequiredArgs: []string{"method"},
	},
	"repo.Upsert": {
		RequiredArgs:     []string{"source", "find", "input", "output"},
		DeclaresFromArgs: []string{"output"},
		CustomConstraints: func(step Step) *Issue {
			if len(step.Children["_ifNew"]) == 0 && len(step.Children["_ifExists"]) == 0 {
				return &Issue{
					Code:    "MISSING_BRANCHES",
					Message: "repo.Upsert requires at least one branch: ifNew or ifExists",
					Hint:    "{action: \"repo.Upsert\", ..., ifNew: [ ... ]}",
				}
			}
			return nil
		},
	},
	"flow.If": {
		RequiredArgs:     []string{"condition"},
		RequiredChildren: []string{"_then"},
	},
	"flow.Switch": {
		RequiredArgs: []string{"value"},
		CustomConstraints: func(step Step) *Issue {
			if len(step.Children["_cases"]) == 0 {
				return &Issue{
					Code:    "MISSING_CASES",
					Message: "flow.Switch requires at least one case",
					Hint:    "{action: \"flow.Switch\", value: \"req.Role\", cases: {owner: [ ... ]}}",
				}
			}
			return nil
		},
	},
	"flow.For": {
		RequiredArgs:     []string{"each", "as"},
		RequiredChildren: []string{"_do"},
	},
	"flow.Block": {
		RequiredChildren: []string{"_do"},
	},
	"tx.Block": {
		RequiredChildren: []string{"_do"},
	},
	"list.Filter": {
		RequiredArgs:     []string{"from", "condition", "output"},
		DeclaresFromArgs: []string{"output"},
	},
	"list.Sort": {
		RequiredArgs: []string{"items", "by"},
	},
	"list.Append": {
		RequiredArgs: []string{"to", "item"},
	},
	"list.Enrich": {
		RequiredArgs: []string{"items", "lookupSource", "lookupInput", "set"},
		CustomConstraints: func(step Step) *Issue {
			setRaw, _ := step.Args["set"].(string)
			if strings.TrimSpace(setRaw) == "" {
				return &Issue{
					Code:    "MISSING_SET",
					Message: "list.Enrich missing 'set'",
					Hint:    "{action: \"list.Enrich\", ..., set: \"AuthorName=Name,AuthorLogo=Logo\"}",
				}
			}
			pairs := strings.Split(setRaw, ",")
			for _, pair := range pairs {
				kv := strings.Split(strings.TrimSpace(pair), "=")
				if len(kv) != 2 || strings.TrimSpace(kv[0]) == "" || strings.TrimSpace(kv[1]) == "" || kv[0] != strings.TrimSpace(kv[0]) || kv[1] != strings.TrimSpace(kv[1]) {
					return &Issue{
						Code:    "INVALID_SET_FORMAT",
						Message: "list.Enrich 'set' must be comma-separated TargetField=LookupField pairs without spaces around '='",
						Hint:    "{action: \"list.Enrich\", ..., set: \"AuthorName=Name,AuthorLogo=Logo\"}",
					}
				}
			}
			return nil
		},
	},
	"list.Paginate": {
		RequiredArgs:     []string{"input", "offset", "limit", "output"},
		DeclaresFromArgs: []string{"output"},
	},
	"str.Normalize": {
		RequiredArgs:     []string{"input", "output"},
		DeclaresFromArgs: []string{"output"},
	},
	"time.CheckExpiry": {
		RequiredArgs: []string{"value", "throw"},
	},
	"time.Parse": {
		RequiredArgs:     []string{"value", "output"},
		DeclaresFromArgs: []string{"output"},
	},
	"map.Build": {
		RequiredArgs:     []string{"from", "key", "value", "output"},
		DeclaresFromArgs: []string{"output"},
	},
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
	"entity.PatchNonZero": {
		RequiredArgs: []string{"target", "from", "fields"},
	},
	"entity.PatchValidated": {
		RequiredArgs: []string{"target", "from"},
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
}

func Validate(steps []Step) []Issue {
	var out []Issue
	var walk func(items []Step, inTx bool)
	walk = func(items []Step, inTx bool) {
		for i := range items {
			step := items[i]
			spec, ok := specs[step.Action]
			if !ok {
				if !isKnownPrefix(step.Action) {
					out = append(out, issue(step, i+1, "UNKNOWN_ACTION", "unknown action '"+step.Action+"'", "{action: \"repo.Find\" | \"mapping.Assign\" | \"flow.If\" ...}"))
				}
			} else {
				for _, arg := range spec.RequiredArgs {
					if !hasNonEmptyString(step.Args, arg) {
						out = append(out, issue(step, i+1, "MISSING_"+strings.ToUpper(arg), step.Action+" missing '"+arg+"'", "See action contract in flow semantics"))
					}
				}
				for _, child := range spec.RequiredChildren {
					if len(step.Children[child]) == 0 {
						out = append(out, issue(step, i+1, "MISSING_"+strings.ToUpper(strings.TrimPrefix(child, "_")), step.Action+" missing '"+strings.TrimPrefix(child, "_")+"'", "See action contract in flow semantics"))
					}
				}
				if spec.RequiresTx && !inTx {
					out = append(out, issue(step, i+1, "TX_REQUIRED", step.Action+" outside tx.Block", "{action: \"tx.Block\", do: [ ... ]}"))
				}
				if spec.CustomConstraints != nil {
					if extra := spec.CustomConstraints(step); extra != nil {
						extra.Step = i + 1
						extra.Action = step.Action
						extra.File = step.File
						extra.Line = step.Line
						extra.Column = step.Column
						extra.CUEPath = step.CUEPath
						if extra.Severity == "" {
							extra.Severity = "error"
						}
						out = append(out, *extra)
					}
				}
			}
			nextTx := inTx || step.Action == "tx.Block"
			for _, children := range step.Children {
				if len(children) > 0 {
					walk(children, nextTx)
				}
			}
		}
	}
	walk(steps, false)
	return out
}

func issue(step Step, idx int, code, message, hint string) Issue {
	return Issue{
		Step:     idx,
		Action:   step.Action,
		Code:     code,
		Message:  message,
		Hint:     hint,
		File:     step.File,
		Line:     step.Line,
		Column:   step.Column,
		CUEPath:  step.CUEPath,
		Severity: "error",
	}
}

func hasNonEmptyString(args map[string]any, key string) bool {
	if args == nil {
		return false
	}
	v, ok := args[key]
	if !ok {
		return false
	}
	s, ok := v.(string)
	return ok && strings.TrimSpace(s) != ""
}

func isKnownPrefix(action string) bool {
	if action == "" {
		return true
	}
	prefixes := []string{
		"repo.", "mapping.", "logic.", "event.", "fsm.", "flow.", "tx.",
		"list.", "notification.", "audit.", "auth.", "entity.", "field.",
		"str.", "enum.", "time.", "map.",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(action, p) {
			return true
		}
	}
	return false
}
