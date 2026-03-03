package flowsem

import (
	"math"
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
	OptionalArgKinds  map[string]ArgKind
	RequiresTx        bool
	CustomConstraints func(step Step) *Issue
}

type ArgKind string

const (
	ArgKindString            ArgKind = "string"
	ArgKindInt               ArgKind = "int"
	ArgKindBool              ArgKind = "bool"
	ArgKindStringMap         ArgKind = "map[string]string"
	ArgKindFieldsRuleMap     ArgKind = "map[string]map[string]string"
	ArgKindStringOrStringArr ArgKind = "string|[]string"
	ArgKindStringList        ArgKind = "[]string"
)

var specs = map[string]Spec{
	"logic.Check": {
		RequiredArgs: []string{"condition", "throw"},
	},
	"event.Publish": {
		RequiredArgs: []string{"name"},
	},
	"event.Broadcast": {
		RequiredArgs: []string{"name"},
	},
	"event.Wait": {
		RequiredArgs:     []string{"name"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"timeout": ArgKindString,
			"match":   ArgKindString,
		},
	},
	"event.Subscribe": {
		RequiredArgs:     []string{"name", "match"},
		RequiredChildren: []string{"_do"},
	},
	"event.Match": {
		RequiredArgs: []string{"event", "match"},
		OptionalArgKinds: map[string]ArgKind{
			"throw": ArgKindString,
		},
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
	"http.Request": {
		RequiredArgs: []string{"method", "url"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"auth":      ArgKindString,
			"timeout":   ArgKindString,
			"into":      ArgKindString,
			"statusVar": ArgKindString,
			"output":    ArgKindString,
			"headers":   ArgKindStringMap,
			"query":     ArgKindStringMap,
		},
	},
	"http.RetryPolicy": {
		RequiredArgs: []string{"method", "url"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"auth":      ArgKindString,
			"timeout":   ArgKindString,
			"statusVar": ArgKindString,
			"output":    ArgKindString,
			"headers":   ArgKindStringMap,
			"query":     ArgKindStringMap,
		},
	},
	"http.Paginate": {
		RequiredArgs: []string{"url", "into", "as", "cursor_expr"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"auth":         ArgKindString,
			"method":       ArgKindString,
			"cursor_param": ArgKindString,
			"items_expr":   ArgKindString,
			"output_type":  ArgKindString,
			"headers":      ArgKindStringMap,
		},
	},
	"db.Get": {},
	"db.List": {},
	"db.Query": {
		RequiredArgs: []string{"method"},
	},
	"db.Insert": {
		RequiredArgs: []string{"source", "input"},
	},
	"db.Update": {
		RequiredArgs: []string{"source", "input"},
	},
	"db.Upsert": {
		RequiredArgs: []string{"source", "input"},
	},
	"db.Delete": {},
	"db.Lock": {
		RequiresTx: true,
	},
	"db.SelectForUpdate": {
		RequiresTx: true,
	},
	"state.Get": {
		RequiredArgs:     []string{"key", "output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"default": ArgKindString,
		},
	},
	"state.Set": {
		RequiredArgs: []string{"key", "value"},
		OptionalArgKinds: map[string]ArgKind{
			"ttl": ArgKindString,
		},
	},
	"state.Delete": {
		RequiredArgs: []string{"key"},
	},
	// Idempotency & Deduplication
	"idem.DeriveKey": {
		RequiredArgs:     []string{"output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"prefix": ArgKindString,
			"output": ArgKindString,
			"from":   ArgKindStringList,
		},
		CustomConstraints: func(step Step) *Issue {
			v, ok := step.Args["from"]
			if !ok {
				return &Issue{Code: "MISSING_FROM", Message: "idem.DeriveKey missing 'from'", Hint: "{action: \"idem.DeriveKey\", from: [\"req.UserID\", \"req.OrderID\"], output: \"idemKey\"}"}
			}
			switch arr := v.(type) {
			case []string:
				if len(arr) == 0 {
					return &Issue{Code: "EMPTY_FROM", Message: "idem.DeriveKey 'from' list is empty", Hint: "Provide at least one expression"}
				}
			case []any:
				if len(arr) == 0 {
					return &Issue{Code: "EMPTY_FROM", Message: "idem.DeriveKey 'from' list is empty", Hint: "Provide at least one expression"}
				}
			default:
				return &Issue{Code: "INVALID_FROM_TYPE", Message: "idem.DeriveKey 'from' must be a list of expressions", Hint: "{from: [\"req.UserID\", \"req.OrderID\"]}"}
			}
			return nil
		},
	},
	"idem.Check": {
		RequiredArgs: []string{"key"},
	},
	"idem.SaveResult": {
		RequiredArgs: []string{"key"},
		OptionalArgKinds: map[string]ArgKind{
			"ttl": ArgKindString,
		},
	},
	"dedupe.Once": {
		RequiredArgs:     []string{"key"},
		RequiredChildren: []string{"_do"},
		OptionalArgKinds: map[string]ArgKind{
			"ttl": ArgKindString,
		},
	},
	// Rate limiting & Concurrency
	"ratelimit.Check": {
		RequiredArgs: []string{"key"},
		OptionalArgKinds: map[string]ArgKind{
			"throw": ArgKindString,
			"rps":   ArgKindInt,
		},
		CustomConstraints: func(step Step) *Issue {
			if _, ok := step.Args["rps"]; !ok {
				return &Issue{Code: "MISSING_RPS", Message: "ratelimit.Check missing 'rps'", Hint: "{action: \"ratelimit.Check\", key: \"req.UserID\", rps: 10}"}
			}
			if !isIntLike(step.Args["rps"]) {
				return &Issue{Code: "INVALID_RPS_TYPE", Message: "ratelimit.Check 'rps' must be an integer", Hint: "{rps: 10}"}
			}
			return nil
		},
	},
	"concurrency.Limit": {
		RequiredArgs: []string{"key"},
		OptionalArgKinds: map[string]ArgKind{
			"throw": ArgKindString,
			"max":   ArgKindInt,
		},
		CustomConstraints: func(step Step) *Issue {
			if _, ok := step.Args["max"]; !ok {
				return &Issue{Code: "MISSING_MAX", Message: "concurrency.Limit missing 'max'", Hint: "{action: \"concurrency.Limit\", key: \"\\\"slow-op\\\"\", max: 5}"}
			}
			if !isIntLike(step.Args["max"]) {
				return &Issue{Code: "INVALID_MAX_TYPE", Message: "concurrency.Limit 'max' must be an integer", Hint: "{max: 5}"}
			}
			return nil
		},
	},
	// Circuit breaker & Bulkhead
	"circuit.Check": {
		RequiredArgs: []string{"name"},
		OptionalArgKinds: map[string]ArgKind{
			"throw": ArgKindString,
		},
	},
	"circuit.RecordSuccess": {
		RequiredArgs: []string{"name"},
	},
	"circuit.RecordFailure": {
		RequiredArgs: []string{"name"},
		OptionalArgKinds: map[string]ArgKind{
			"threshold": ArgKindInt,
			"openTTL":   ArgKindString,
		},
	},
	"bulkhead.Acquire": {
		RequiredArgs: []string{"name"},
		OptionalArgKinds: map[string]ArgKind{
			"throw": ArgKindString,
			"max":   ArgKindInt,
		},
		CustomConstraints: func(step Step) *Issue {
			if _, ok := step.Args["max"]; !ok {
				return &Issue{Code: "MISSING_MAX", Message: "bulkhead.Acquire missing 'max'", Hint: "{action: \"bulkhead.Acquire\", name: \"\\\"db-pool\\\"\", max: 20}"}
			}
			if !isIntLike(step.Args["max"]) {
				return &Issue{Code: "INVALID_MAX_TYPE", Message: "bulkhead.Acquire 'max' must be an integer", Hint: "{max: 20}"}
			}
			return nil
		},
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
	"cache.Get": {
		RequiredArgs:     []string{"key", "output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"optional": ArgKindBool,
		},
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
		OptionalArgKinds: map[string]ArgKind{
			"attempts":    ArgKindInt,
			"retries":     ArgKindInt,
			"backoffMs":   ArgKindInt,
			"timeoutMs":   ArgKindInt,
			"failOnError": ArgKindBool,
			"headers":     ArgKindStringMap,
		},
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
		OptionalArgKinds: map[string]ArgKind{
			"maxConcurrency": ArgKindInt,
			"maxParallel":    ArgKindInt,
		},
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
	"flow.Parallel": {
		CustomConstraints: func(step Step) *Issue {
			if len(step.Children["_branches"]) == 0 {
				return &Issue{
					Code:    "MISSING_BRANCHES",
					Message: "flow.Parallel requires branches",
					Hint:    `{action: "flow.Parallel", branches: {loadUser: [...], loadOrg: [...]}}`,
				}
			}
			return nil
		},
	},
	"flow.Join": {
		CustomConstraints: func(step Step) *Issue {
			if len(step.Children["_branches"]) == 0 {
				return &Issue{
					Code:    "MISSING_BRANCHES",
					Message: "flow.Join requires branches",
					Hint:    `{action: "flow.Join", branches: {sendMail: [...], logEvent: [...]}}`,
				}
			}
			return nil
		},
	},
	"flow.Race": {
		CustomConstraints: func(step Step) *Issue {
			if len(step.Children["_branches"]) < 2 {
				return &Issue{
					Code:    "TOO_FEW_BRANCHES",
					Message: "flow.Race requires at least 2 branches",
					Hint:    `{action: "flow.Race", branches: {fromCache: [...], fromDB: [...]}}`,
				}
			}
			return nil
		},
	},
	"flow.Delay": {
		RequiredArgs: []string{"duration"},
	},
	"flow.Schedule": {
		RequiredArgs: []string{"at"},
	},
	"flow.Cron": {
		RequiredArgs: []string{"window"},
		CustomConstraints: func(step Step) *Issue {
			w, _ := step.Args["window"].(string)
			if strings.TrimSpace(w) == "" {
				return &Issue{
					Code:    "INVALID_WINDOW",
					Message: "flow.Cron requires a non-empty window",
					Hint:    `{action: "flow.Cron", window: "Mon-Fri 09:00-17:00", onMismatch: [...]}`,
				}
			}
			return nil
		},
	},
	"flow.Tag": {
		RequiredArgs: []string{"name"},
		OptionalArgKinds: map[string]ArgKind{
			"value": ArgKindString,
		},
	},
	"flow.Saga": {
		RequiredChildren: []string{"_do"},
	},
	"flow.Compensate": {
		RequiredChildren: []string{"_do"},
	},
	"flow.Rollback": {
		OptionalArgKinds: map[string]ArgKind{
			"error": ArgKindString,
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
		OptionalArgKinds: map[string]ArgKind{
			"timeoutMs": ArgKindInt,
		},
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
	"storage.Delete": {
		RequiredArgs: []string{"key"},
	},
	"storage.List": {
		RequiredArgs:     []string{"prefix", "output"},
		DeclaresFromArgs: []string{"output"},
	},
	"secret.Get": {
		RequiredArgs:     []string{"key", "output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"default": ArgKindString,
		},
	},
	"config.Get": {
		RequiredArgs:     []string{"key", "output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"default": ArgKindString,
		},
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
					v, present := step.Args[arg]
					if !present {
						out = append(out, issue(step, i+1, "MISSING_"+strings.ToUpper(arg), step.Action+" missing '"+arg+"'", "See action contract in flow semantics"))
						continue
					}
					if _, ok := nonEmptyString(v); !ok {
						if _, isString := v.(string); isString {
							out = append(out, issue(step, i+1, "MISSING_"+strings.ToUpper(arg), step.Action+" missing '"+arg+"'", "See action contract in flow semantics"))
						} else {
							out = append(out, issue(step, i+1, "INVALID_"+strings.ToUpper(arg)+"_TYPE", step.Action+" arg '"+arg+"' must be string expression", "Pass a CUE string expression"))
						}
					}
				}
				for _, child := range spec.RequiredChildren {
					if len(step.Children[child]) == 0 {
						out = append(out, issue(step, i+1, "MISSING_"+strings.ToUpper(strings.TrimPrefix(child, "_")), step.Action+" missing '"+strings.TrimPrefix(child, "_")+"'", "See action contract in flow semantics"))
					}
				}
				for argName, kind := range spec.OptionalArgKinds {
					v, present := step.Args[argName]
					if !present {
						continue
					}
					if !argMatchesKind(v, kind) {
						out = append(out, issue(step, i+1, "INVALID_"+strings.ToUpper(argName)+"_TYPE", step.Action+" arg '"+argName+"' must be "+string(kind), "Fix arg type in CUE step"))
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
			// Transaction context propagates down the subtree; tx-only actions are validated
			// against this propagated state, not global method position.
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

func nonEmptyString(v any) (string, bool) {
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	return s, true
}

func argMatchesKind(v any, kind ArgKind) bool {
	switch kind {
	case ArgKindString:
		_, ok := nonEmptyString(v)
		return ok
	case ArgKindInt:
		return isIntLike(v)
	case ArgKindBool:
		_, ok := v.(bool)
		return ok
	case ArgKindStringMap:
		m, ok := v.(map[string]string)
		return ok && m != nil
	case ArgKindFieldsRuleMap:
		m, ok := v.(map[string]map[string]string)
		return ok && m != nil
	case ArgKindStringOrStringArr:
		if _, ok := nonEmptyString(v); ok {
			return true
		}
		arr, ok := v.([]string)
		return ok && len(arr) > 0
	case ArgKindStringList:
		switch arr := v.(type) {
		case []string:
			return len(arr) > 0
		case []any:
			return len(arr) > 0
		default:
			return false
		}
	default:
		return true
	}
}

func isIntLike(v any) bool {
	switch n := v.(type) {
	case int, int64:
		return true
	case float64:
		return n == math.Trunc(n)
	default:
		return false
	}
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
		if n != math.Trunc(n) {
			return 0, false
		}
		return int(n), true
	default:
		return 0, false
	}
}

func staticWordLiteral(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", false
	}
	if unquoted, err := strconv.Unquote(s); err == nil {
		s = strings.TrimSpace(unquoted)
	}
	if !isWordToken(s) {
		return "", false
	}
	return strings.ToLower(s), true
}

func isWordToken(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') &&
			(r < 'A' || r > 'Z') &&
			(r < '0' || r > '9') &&
			r != '_' &&
			r != '-' {
			return false
		}
	}
	return true
}

func isKnownPrefix(action string) bool {
	if action == "" {
		return true
	}
	// Prefix allow-list keeps diagnostics useful: unknown actions with known families
	// are handled by emitter-specific validation, while truly foreign actions fail here.
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
