package flowsem

import "strings"

var specsInfra = map[string]Spec{
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
	"openai.Stream": {
		RequiredArgs:      []string{"user_message"},
		DeclaresFromArgs:  []string{"output"},
		RequiresStreaming: true,
		OptionalArgKinds: map[string]ArgKind{
			"system":         ArgKindString,
			"system_context": ArgKindString,
			"history":        ArgKindString,
			"output":         ArgKindString,
			"model":          ArgKindString,
			"max_tokens":     ArgKindInt,
		},
	},
	"plan.BuildAutomata": {
		RequiredArgs:     []string{"input", "output"},
		DeclaresFromArgs: []string{"output"},
	},
	"plan.BuildMicroPlan": {
		RequiredArgs:     []string{"usecases", "automata", "output"},
		DeclaresFromArgs: []string{"output"},
	},
	"cue.EmitProject": {
		RequiredArgs:     []string{"usecases", "micro_plan", "output"},
		DeclaresFromArgs: []string{"output"},
	},
	"cue.ValidateProject": {
		RequiredArgs:     []string{"files", "output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"binary": ArgKindString,
		},
	},
	"cue.WriteProjectFiles": {
		RequiredArgs:     []string{"root", "files", "output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"mode":     ArgKindString,
			"prefixes": ArgKindStringList,
		},
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
	"str.Concat": {
		RequiredArgs:     []string{"parts", "output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"parts": ArgKindStringOrStringArr,
			"sep":   ArgKindString,
		},
	},
	"str.StripMarkdown": {OptionalArgKinds: map[string]ArgKind{"input": ArgKindString, "output": ArgKindString}},
	"cast.ToString": {
		RequiredArgs:     []string{"input", "output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"format": ArgKindString,
		},
	},
	"convert.ToFloat": {
		RequiredArgs:     []string{"input", "output"},
		DeclaresFromArgs: []string{"output"},
	},
	"convert.ToInt": {
		RequiredArgs:     []string{"input", "output"},
		DeclaresFromArgs: []string{"output"},
	},
	"num.Add": {
		RequiredArgs:     []string{"a", "b", "output"},
		DeclaresFromArgs: []string{"output"},
	},
	"num.Sub": {
		RequiredArgs:     []string{"a", "b", "output"},
		DeclaresFromArgs: []string{"output"},
	},
	"num.Mul": {
		RequiredArgs:     []string{"a", "b", "output"},
		DeclaresFromArgs: []string{"output"},
	},
	"num.Div": {
		RequiredArgs:     []string{"a", "b", "output"},
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
	"regex.Match": {
		RequiredArgs:     []string{"input", "pattern", "output"},
		DeclaresFromArgs: []string{"output"},
	},
	"regex.Replace": {
		RequiredArgs:     []string{"input", "pattern", "repl", "output"},
		DeclaresFromArgs: []string{"output"},
	},
	"base64.Encode": {
		RequiredArgs:     []string{"input", "output"},
		DeclaresFromArgs: []string{"output"},
	},
	"base64.Decode": {
		RequiredArgs:     []string{"input", "output"},
		DeclaresFromArgs: []string{"output"},
	},
	"url.Parse": {
		RequiredArgs:     []string{"input", "output"},
		DeclaresFromArgs: []string{"output"},
	},
	"url.Build": {
		RequiredArgs:     []string{"base", "output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"path":  ArgKindString,
			"query": ArgKindStringMap,
		},
	},
	"query.Encode": {
		RequiredArgs:     []string{"input", "output"},
		DeclaresFromArgs: []string{"output"},
	},
	"query.Decode": {
		RequiredArgs:     []string{"input", "output"},
		DeclaresFromArgs: []string{"output"},
	},
	"hash.Sum": {
		RequiredArgs:     []string{"input", "output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"algorithm": ArgKindString,
			"algo":      ArgKindString,
		},
	},
	"hash.HMAC": {
		RequiredArgs:     []string{"input", "key", "output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"algorithm": ArgKindString,
			"algo":      ArgKindString,
		},
	},
	"uuid.New": {
		RequiredArgs:     []string{"output"},
		DeclaresFromArgs: []string{"output"},
	},
	"ulid.New": {
		RequiredArgs:     []string{"output"},
		DeclaresFromArgs: []string{"output"},
	},
	"math.Op": {
		RequiredArgs:     []string{"op", "output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"precision": ArgKindInt,
		},
		CustomConstraints: func(step Step) *Issue {
			opRaw, ok := step.Args["op"].(string)
			if !ok || strings.TrimSpace(opRaw) == "" {
				return &Issue{
					Code:    "MISSING_OP",
					Message: "math.Op missing 'op'",
					Hint:    "Use {action: \"math.Op\", op: \"min\", a: \"x\", b: \"y\", output: \"z\"}",
				}
			}
			op, isLiteral := staticWordLiteral(opRaw)
			if !isLiteral {
				return nil
			}
			require := func(name, hint string) *Issue {
				v, ok := step.Args[name]
				if !ok {
					return &Issue{Code: "MISSING_" + strings.ToUpper(name), Message: "math.Op missing '" + name + "'", Hint: hint}
				}
				if _, ok := nonEmptyString(v); !ok {
					return &Issue{Code: "MISSING_" + strings.ToUpper(name), Message: "math.Op missing '" + name + "'", Hint: hint}
				}
				return nil
			}
			switch op {
			case "min", "max":
				if is := require("a", "Provide both 'a' and 'b'"); is != nil {
					return is
				}
				if is := require("b", "Provide both 'a' and 'b'"); is != nil {
					return is
				}
			case "clamp":
				if is := require("value", "Provide value/min/max"); is != nil {
					return is
				}
				if is := require("min", "Provide value/min/max"); is != nil {
					return is
				}
				if is := require("max", "Provide value/min/max"); is != nil {
					return is
				}
			case "round":
				if is := require("value", "Provide value for round"); is != nil {
					return is
				}
			}
			return nil
		},
	},
	"jsonpath.Get": {
		RequiredArgs:     []string{"input", "path", "output"},
		DeclaresFromArgs: []string{"output"},
	},
	"jsonpath.Set": {
		RequiredArgs:     []string{"input", "path", "value", "output"},
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
	// Webhook / queue / async delivery
	"webhook.Send": {
		RequiredArgs: []string{"url", "payload"},
		OptionalArgKinds: map[string]ArgKind{
			"retries": ArgKindInt,
			"event":   ArgKindString,
		},
	},
	"webhook.VerifySignature": {
		RequiredArgs:     []string{"payload", "signature"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"secret":    ArgKindString,
			"algorithm": ArgKindString,
			"throw":     ArgKindString,
			"output":    ArgKindString,
			"strict":    ArgKindBool,
		},
	},
	"webhook.Ack": {
		OptionalArgKinds: map[string]ArgKind{
			"status": ArgKindInt,
			"body":   ArgKindString,
		},
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
	"queue.Dequeue": {
		RequiredArgs:     []string{"subject", "output"},
		DeclaresFromArgs: []string{"output", "ackToken"},
		OptionalArgKinds: map[string]ArgKind{
			"ackToken":  ArgKindString,
			"attempts":  ArgKindInt,
			"retries":   ArgKindInt,
			"backoffMs": ArgKindInt,
			"jitterMs":  ArgKindInt,
			"timeoutMs": ArgKindInt,
		},
		CustomConstraints: func(step Step) *Issue {
			if timeoutMS, ok := intArg(step.Args, "timeoutMs"); ok && timeoutMS <= 0 {
				return &Issue{
					Code:    "INVALID_TIMEOUT_MS",
					Message: "queue.Dequeue timeoutMs must be > 0",
					Hint:    "{action: \"queue.Dequeue\", subject: \"events\", output: \"msg\", timeoutMs: 3000}",
				}
			}
			if attempts, ok := intArg(step.Args, "attempts"); ok && attempts <= 0 {
				return &Issue{
					Code:    "INVALID_ATTEMPTS",
					Message: "queue.Dequeue attempts must be > 0",
					Hint:    "{action: \"queue.Dequeue\", subject: \"events\", output: \"msg\", attempts: 3}",
				}
			}
			if retries, ok := intArg(step.Args, "retries"); ok && retries < 0 {
				return &Issue{
					Code:    "INVALID_RETRIES",
					Message: "queue.Dequeue retries must be >= 0",
					Hint:    "{action: \"queue.Dequeue\", subject: \"events\", output: \"msg\", retries: 2}",
				}
			}
			if backoff, ok := intArg(step.Args, "backoffMs"); ok && backoff < 0 {
				return &Issue{
					Code:    "INVALID_BACKOFF",
					Message: "queue.Dequeue backoffMs must be >= 0",
					Hint:    "{action: \"queue.Dequeue\", subject: \"events\", output: \"msg\", backoffMs: 150}",
				}
			}
			if jitter, ok := intArg(step.Args, "jitterMs"); ok && jitter < 0 {
				return &Issue{
					Code:    "INVALID_JITTER",
					Message: "queue.Dequeue jitterMs must be >= 0",
					Hint:    "{action: \"queue.Dequeue\", subject: \"events\", output: \"msg\", jitterMs: 50}",
				}
			}
			return nil
		},
	},
	"queue.Ack": {
		RequiredArgs: []string{"subject", "messageID"},
	},
	"queue.Nack": {
		RequiredArgs: []string{"subject", "messageID"},
		OptionalArgKinds: map[string]ArgKind{
			"reason": ArgKindString,
		},
	},
	"dlq.Publish": {
		RequiredArgs: []string{"subject", "payload"},
		OptionalArgKinds: map[string]ArgKind{
			"reason": ArgKindString,
		},
	},
	"event.Outbox": {
		RequiredArgs: []string{"name", "payload"},
		OptionalArgKinds: map[string]ArgKind{
			"id": ArgKindString,
		},
		RequiresTx: true,
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
	"jwt.Sign": {
		RequiredArgs:     []string{"claims", "output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"secret": ArgKindString,
			"alg":    ArgKindString,
			"ttl":    ArgKindString,
		},
	},
	"jwt.Verify": {
		RequiredArgs:     []string{"token", "output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"secret": ArgKindString,
		},
	},
	"oauth2.Token": {
		RequiredArgs:     []string{"tokenURL", "output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"clientID":     ArgKindString,
			"clientSecret": ArgKindString,
			"scope":        ArgKindString,
			"audience":     ArgKindString,
			"grantType":    ArgKindString,
			"username":     ArgKindString,
			"password":     ArgKindString,
			"code":         ArgKindString,
			"redirectURI":  ArgKindString,
			"refreshToken": ArgKindString,
		},
	},
	"oauth2.Refresh": {
		RequiredArgs:     []string{"tokenURL", "refreshToken", "output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"clientID":     ArgKindString,
			"clientSecret": ArgKindString,
			"scope":        ArgKindString,
			"audience":     ArgKindString,
		},
	},
	"crypto.Encrypt": {
		RequiredArgs:     []string{"input", "output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"key": ArgKindString,
			"aad": ArgKindString,
		},
	},
	"crypto.Decrypt": {
		RequiredArgs:     []string{"input", "output"},
		DeclaresFromArgs: []string{"output"},
		OptionalArgKinds: map[string]ArgKind{
			"key": ArgKindString,
			"aad": ArgKindString,
		},
	},
}
