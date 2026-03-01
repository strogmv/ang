package flowsem

import (
	"strconv"
	"strings"
)

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
		// "event" is the canonical arg; "message" accepted for backwards compat
		CustomConstraints: func(step Step) *Issue {
			_, hasEvent := step.Args["event"]
			_, hasMessage := step.Args["message"]
			if !hasEvent && !hasMessage {
				return &Issue{
					Code:    "MISSING_EVENT",
					Message: "notification.Dispatch requires 'event' (or 'message') arg",
					Hint:    "{action: \"notification.Dispatch\", event: \"user.registered\", userID: \"user.ID\"}",
				}
			}
			return nil
		},
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
	"flow.Checkpoint": {
		RequiredArgs: []string{"name"},
	},
	"flow.Resume": {
		RequiredArgs:     []string{"name"},
		DeclaresFromArgs: []string{"output"},
	},
	"flow.Validate": {
		RequiredArgs: []string{"condition"},
	},
	"flow.Try": {
		RequiredChildren: []string{"_do"},
	},
	"flow.Catch": {
		RequiredChildren: []string{"_do"},
	},
	"flow.Retry": {
		RequiredChildren: []string{"_do"},
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
	// OS / system operations
	"exec.Run": {
		RequiredArgs:     []string{"cmd"},
		DeclaresFromArgs: []string{"output"},
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
	// Stage 2: Infrastructure actions
	"cache.Get": {
		RequiredArgs:     []string{"key", "output"},
		DeclaresFromArgs: []string{"output"},
	},
	"cache.Set": {
		RequiredArgs: []string{"key", "value"},
	},
	"cache.Del": {
		RequiredArgs: []string{"key"},
	},
	"mail.Send": {
		RequiredArgs: []string{"to", "subject", "body"},
	},
	"storage.Upload": {
		RequiredArgs: []string{"key", "data"},
	},
	"storage.GetURL": {
		RequiredArgs:     []string{"key", "output"},
		DeclaresFromArgs: []string{"output"},
	},
	// Stage 3: New capabilities
	"http.Call": {
		RequiredArgs: []string{"method", "url"},
		CustomConstraints: func(step Step) *Issue {
			if attempts, ok := intArg(step.Args, "attempts"); ok && attempts <= 0 {
				return &Issue{
					Code:    "INVALID_ATTEMPTS",
					Message: "http.Call attempts must be > 0",
					Hint:    "{action: \"http.Call\", method: \"GET\", url: \"...\", attempts: 2}",
				}
			}
			if retries, ok := intArg(step.Args, "retries"); ok && retries < 0 {
				return &Issue{
					Code:    "INVALID_RETRIES",
					Message: "http.Call retries must be >= 0",
					Hint:    "{action: \"http.Call\", method: \"GET\", url: \"...\", retries: 1}",
				}
			}
			if backoff, ok := intArg(step.Args, "backoffMs"); ok && backoff < 0 {
				return &Issue{
					Code:    "INVALID_BACKOFF",
					Message: "http.Call backoffMs must be >= 0",
					Hint:    "{action: \"http.Call\", method: \"GET\", url: \"...\", backoffMs: 150}",
				}
			}
			if timeoutMS, ok := intArg(step.Args, "timeoutMs"); ok && timeoutMS <= 0 {
				return &Issue{
					Code:    "INVALID_TIMEOUT_MS",
					Message: "http.Call timeoutMs must be > 0",
					Hint:    "{action: \"http.Call\", method: \"GET\", url: \"...\", timeoutMs: 5000}",
				}
			}
			return nil
		},
	},
	"rand.Code": {
		RequiredArgs:     []string{"output"},
		DeclaresFromArgs: []string{"output"},
	},
	"rand.Token": {
		RequiredArgs:     []string{"output"},
		DeclaresFromArgs: []string{"output"},
	},
	"str.Format": {
		RequiredArgs:     []string{"template", "output"},
		DeclaresFromArgs: []string{"output"},
	},
	"json.Parse": {
		RequiredArgs:     []string{"input", "into", "output"},
		DeclaresFromArgs: []string{"output"},
	},
	"json.Marshal": {
		RequiredArgs:     []string{"input", "output"},
		DeclaresFromArgs: []string{"output"},
	},
	"parallel.Run": {
		CustomConstraints: func(step Step) *Issue {
			if len(step.Children["_branches"]) == 0 {
				return &Issue{
					Code:    "MISSING_BRANCHES",
					Message: "parallel.Run requires at least one branch",
					Hint:    "{action: \"parallel.Run\", branches: {fetchUser: [ ... ], fetchOrg: [ ... ]}}",
				}
			}
			if maxConc, ok := intArg(step.Args, "maxConcurrency"); ok && maxConc <= 0 {
				return &Issue{
					Code:    "INVALID_MAX_CONCURRENCY",
					Message: "parallel.Run maxConcurrency must be > 0",
					Hint:    "{action: \"parallel.Run\", maxConcurrency: 8, branches: {...}}",
				}
			}
			if maxPar, ok := intArg(step.Args, "maxParallel"); ok && maxPar <= 0 {
				return &Issue{
					Code:    "INVALID_MAX_PARALLEL",
					Message: "parallel.Run maxParallel must be > 0",
					Hint:    "{action: \"parallel.Run\", maxParallel: 8, branches: {...}}",
				}
			}
			return nil
		},
	},
	"pdf.Render": {
		RequiredArgs:     []string{"template", "data", "output"},
		DeclaresFromArgs: []string{"output"},
	},
	// Webhook / queue
	"webhook.Send": {
		RequiredArgs: []string{"url", "payload"},
	},
	"queue.Enqueue": {
		RequiredArgs: []string{"subject", "payload"},
		CustomConstraints: func(step Step) *Issue {
			if timeoutMS, ok := intArg(step.Args, "timeoutMs"); ok && timeoutMS <= 0 {
				return &Issue{
					Code:    "INVALID_TIMEOUT_MS",
					Message: "queue.Enqueue timeoutMs must be > 0",
					Hint:    "{action: \"queue.Enqueue\", subject: \"events\", payload: req, timeoutMs: 3000}",
				}
			}
			return nil
		},
	},
	// notify.Dispatch — short alias for notification.Dispatch
	"notify.Dispatch": {
		CustomConstraints: func(step Step) *Issue {
			_, hasEvent := step.Args["event"]
			_, hasMessage := step.Args["message"]
			if !hasEvent && !hasMessage {
				return &Issue{
					Code:    "MISSING_EVENT",
					Message: "notify.Dispatch requires 'event' arg",
					Hint:    "{action: \"notify.Dispatch\", event: \"user.registered\", userID: \"user.ID\"}",
				}
			}
			return nil
		},
	},
	// storage.Download
	"storage.Download": {
		RequiredArgs:     []string{"key", "output"},
		DeclaresFromArgs: []string{"output"},
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

func intArg(args map[string]any, key string) (int, bool) {
	if args == nil {
		return 0, false
	}
	v, ok := args[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case string:
		s := strings.TrimSpace(n)
		if s == "" {
			return 0, false
		}
		p, err := strconv.Atoi(s)
		if err != nil {
			return 0, false
		}
		return p, true
	default:
		return 0, false
	}
}

func isKnownPrefix(action string) bool {
	if action == "" {
		return true
	}
	prefixes := []string{
		"repo.", "mapping.", "logic.", "event.", "fsm.", "flow.", "tx.",
		"list.", "notification.", "audit.", "auth.", "entity.", "field.",
		"str.", "enum.", "time.", "map.",
		"exec.", "fs.",
		"cache.", "mail.", "storage.",
		"http.", "rand.", "json.", "parallel.",
		"pdf.",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(action, p) {
			return true
		}
	}
	return false
}
