package flowsem

import "strings"

var specsCoreFlow = map[string]Spec{
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
	"flow.While": {
		RequiredArgs:     []string{"condition"},
		RequiredChildren: []string{"_do"},
		CustomConstraints: func(step Step) *Issue {
			return validateGoExprArg(step, "condition")
		},
	},
	"flow.Checkpoint": {
		RequiredArgs: []string{"name"},
	},
	"flow.Resume": {
		RequiredArgs:     []string{"name"},
		DeclaresFromArgs: []string{"output"},
	},
	"flow.RecordEvent": {
		RequiredArgs:     []string{"name"},
		DeclaresFromArgs: []string{"output"},
	},
	"flow.Replay": {
		RequiredArgs:     []string{"history"},
		DeclaresFromArgs: []string{"output"},
	},
	"flow.History.Get": {
		RequiredArgs:     []string{"output"},
		DeclaresFromArgs: []string{"output"},
	},
	"flow.Validate": {
		RequiredArgs: []string{"condition"},
	},
	"flow.Try": {
		RequiredChildren: []string{"_do"},
		OptionalArgKinds: map[string]ArgKind{
			"retries":   ArgKindInt,
			"backoffMs": ArgKindInt,
		},
	},
	"flow.Catch": {
		RequiredChildren: []string{"_do"},
	},
	"flow.Retry": {
		RequiredChildren: []string{"_do"},
		OptionalArgKinds: map[string]ArgKind{
			"attempts":  ArgKindInt,
			"retries":   ArgKindInt,
			"backoffMs": ArgKindInt,
		},
	},
	"flow.Fallback": {
		RequiredChildren: []string{"_do", "_fallback"},
	},
	"flow.Timeout": {
		RequiredArgs:      []string{"duration"},
		RequiredChildren:  []string{"_do"},
		CustomConstraints: nil,
	},
	"flow.SuggestNext": {
		DeclaresFromArgs: []string{"output"},
		CustomConstraints: func(step Step) *Issue {
			opts, ok := step.Args["options"]
			if !ok {
				return &Issue{
					Code:    "MISSING_OPTIONS",
					Message: "flow.SuggestNext requires non-empty 'options'",
					Hint:    "{action: \"flow.SuggestNext\", options: [\"retry\", \"open editor\"]}",
				}
			}
			switch x := opts.(type) {
			case []string:
				if len(x) == 0 {
					return &Issue{
						Code:    "MISSING_OPTIONS",
						Message: "flow.SuggestNext requires non-empty 'options'",
						Hint:    "{action: \"flow.SuggestNext\", options: [\"retry\", \"open editor\"]}",
					}
				}
			case string:
				if strings.TrimSpace(x) == "" {
					return &Issue{
						Code:    "MISSING_OPTIONS",
						Message: "flow.SuggestNext requires non-empty 'options'",
						Hint:    "{action: \"flow.SuggestNext\", options: [\"retry\", \"open editor\"]}",
					}
				}
			default:
				return &Issue{
					Code:    "MISSING_OPTIONS",
					Message: "flow.SuggestNext requires 'options' as string or []string",
					Hint:    "{action: \"flow.SuggestNext\", options: [\"retry\", \"open editor\"]}",
				}
			}
			return nil
		},
	},
	"flow.ExplainError": {
		DeclaresFromArgs: []string{"output"},
	},
	"flow.Block": {
		RequiredChildren: []string{"_do"},
	},
	"flow.Return": {},
	"flow.Defer": {
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
		OptionalArgKinds: map[string]ArgKind{
			"order": ArgKindString,
		},
		CustomConstraints: func(step Step) *Issue {
			raw, ok := nonEmptyString(step.Args["order"])
			if !ok {
				return nil
			}
			order, isStatic := staticWordLiteral(raw)
			if !isStatic {
				// Dynamic CUE/Go expression (for example req.SortOrder) is allowed.
				return nil
			}
			if order != "asc" && order != "desc" {
				return &Issue{
					Code:    "INVALID_ORDER",
					Message: "list.Sort order must be asc or desc when literal value is used",
					Hint:    "{action: \"list.Sort\", items: \"items\", by: \"CreatedAt\", order: \"desc\"}",
				}
			}
			return nil
		},
	},
	"list.Append": {
		RequiredArgs: []string{"to", "item"},
	},
	"list.Map": {
		RequiredArgs:     []string{"from", "expr", "output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"as": ArgKindString,
		},
	},
	"list.Reduce": {
		RequiredArgs:     []string{"from", "expr", "output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"as": ArgKindString,
		},
	},
	"list.GroupBy": {
		RequiredArgs:     []string{"from", "key", "output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"as": ArgKindString,
		},
	},
	"list.Distinct": {
		RequiredArgs:     []string{"from", "output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"as":  ArgKindString,
			"key": ArgKindString,
		},
	},
	"list.Chunk": {
		RequiredArgs:     []string{"from", "output"},
		DeclaresFromArgs: []string{"output"},
		CustomConstraints: func(step Step) *Issue {
			sizeVal, ok := step.Args["size"]
			if !ok {
				return &Issue{Code: "MISSING_SIZE", Message: "list.Chunk missing 'size'", Hint: "{action: \"list.Chunk\", from: \"items\", size: 100, output: \"batches\"}"}
			}
			switch v := sizeVal.(type) {
			case string:
				if strings.TrimSpace(v) == "" {
					return &Issue{Code: "MISSING_SIZE", Message: "list.Chunk missing 'size'", Hint: "{action: \"list.Chunk\", from: \"items\", size: 100, output: \"batches\"}"}
				}
			case int, int64:
				if n, ok := intArg(step.Args, "size"); ok && n <= 0 {
					return &Issue{Code: "INVALID_SIZE", Message: "list.Chunk 'size' must be > 0", Hint: "{action: \"list.Chunk\", from: \"items\", size: 100, output: \"batches\"}"}
				}
			case float64:
				if !isIntLike(v) || v <= 0 {
					return &Issue{Code: "INVALID_SIZE", Message: "list.Chunk 'size' must be a positive integer", Hint: "{action: \"list.Chunk\", from: \"items\", size: 100, output: \"batches\"}"}
				}
			default:
				return &Issue{Code: "INVALID_SIZE_TYPE", Message: "list.Chunk 'size' must be int or expression string", Hint: "{action: \"list.Chunk\", from: \"items\", size: 100, output: \"batches\"}"}
			}
			return nil
		},
	},
	"batch.Run": {
		RequiredArgs:     []string{"from"},
		RequiredChildren: []string{"_do"},
		OptionalArgKinds: map[string]ArgKind{
			"as": ArgKindString,
		},
		CustomConstraints: func(step Step) *Issue {
			sizeVal, ok := step.Args["size"]
			if !ok {
				return nil
			}
			switch v := sizeVal.(type) {
			case string:
				if strings.TrimSpace(v) == "" {
					return &Issue{Code: "MISSING_SIZE", Message: "batch.Run has empty 'size'", Hint: "{action: \"batch.Run\", from: \"items\", size: 100, as: \"batch\", do: [...]}"}
				}
			case int, int64:
				if n, ok := intArg(step.Args, "size"); ok && n <= 0 {
					return &Issue{Code: "INVALID_SIZE", Message: "batch.Run 'size' must be > 0", Hint: "{action: \"batch.Run\", from: \"items\", size: 100, as: \"batch\", do: [...]}"}
				}
			case float64:
				if !isIntLike(v) || v <= 0 {
					return &Issue{Code: "INVALID_SIZE", Message: "batch.Run 'size' must be a positive integer", Hint: "{action: \"batch.Run\", from: \"items\", size: 100, as: \"batch\", do: [...]}"}
				}
			default:
				return &Issue{Code: "INVALID_SIZE_TYPE", Message: "batch.Run 'size' must be int or expression string", Hint: "{action: \"batch.Run\", from: \"items\", size: 100, as: \"batch\", do: [...]}"}
			}
			return nil
		},
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
		OptionalArgKinds: map[string]ArgKind{
			"defaultLimit": ArgKindInt,
		},
	},
	"list.Len": {
		RequiredArgs:     []string{"input", "output"},
		DeclaresFromArgs: []string{"output"},
	},
	"list.New": {
		RequiredArgs:     []string{"output", "type"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"cap": ArgKindString,
		},
	},
	"list.Sum": {
		RequiredArgs:     []string{"input", "output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"field": ArgKindString,
		},
	},
	"list.Avg": {
		RequiredArgs:     []string{"input", "output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"field": ArgKindString,
		},
	},
	"map.New": {
		RequiredArgs:     []string{"output", "type"},
		DeclaresFromArgs: []string{"output"},
	},
	"str.Normalize": {
		RequiredArgs:     []string{"input", "output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"mode": ArgKindString,
		},
		CustomConstraints: func(step Step) *Issue {
			raw, ok := nonEmptyString(step.Args["mode"])
			if !ok {
				return nil
			}
			mode, isStatic := staticWordLiteral(raw)
			if !isStatic {
				// Dynamic CUE/Go expression (for example req.NormalizeMode) is allowed.
				return nil
			}
			if mode != "trim" && mode != "lower" && mode != "upper" {
				return &Issue{
					Code:    "INVALID_MODE",
					Message: "str.Normalize mode must be trim, lower, or upper when literal value is used",
					Hint:    "{action: \"str.Normalize\", input: \"req.Name\", output: \"name\", mode: \"lower\"}",
				}
			}
			return nil
		},
	},
	"time.CheckExpiry": {
		RequiredArgs: []string{"value", "throw"},
	},
	"time.Now": {
		RequiredArgs:     []string{"output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"format": ArgKindString,
		},
	},
	"time.Parse": {
		RequiredArgs:     []string{"value", "output"},
		DeclaresFromArgs: []string{"output"},
	},
	"time.Format": {
		RequiredArgs:     []string{"input", "output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"format": ArgKindString,
		},
	},
	"map.Build": {
		RequiredArgs:     []string{"from", "key", "value", "output"},
		DeclaresFromArgs: []string{"output"},
	},
}
