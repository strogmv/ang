package emitter

import (
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

func TestRenderFlowActionMatrix_ControlAndScheduling(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		step  normalizer.FlowStep
		wants []string
		avoid []string
	}{
		{
			name: "flow.Delay",
			step: normalizer.FlowStep{Action: "flow.Delay", Args: map[string]any{"duration": "5 * time.Second"}},
			wants: []string{
				"time.NewTimer(5 * time.Second)",
				"ctx.Done()",
			},
		},
		{
			name: "flow.Schedule",
			step: normalizer.FlowStep{Action: "flow.Schedule", Args: map[string]any{"at": "deadline"}},
			wants: []string{
				"time.Until(deadline)",
				"time.NewTimer(",
			},
		},
		{
			name: "flow.Tag",
			step: normalizer.FlowStep{Action: "flow.Tag", Args: map[string]any{"name": `"stage"`, "value": `"validate"`}},
			wants: []string{
				`slog.Info("flow.tag"`,
				`"name", "stage"`,
			},
		},
		{
			name: "flow.Return",
			step: normalizer.FlowStep{Action: "flow.Return", Args: map[string]any{"set": "resp.Status", "value": `"ok"`}},
			wants: []string{
				`resp.Status = "ok"`,
				"return resp, nil",
			},
		},
		{
			name: "flow.Switch glob",
			step: normalizer.FlowStep{Action: "flow.Switch", Args: map[string]any{
				"value": "req.Kind",
				"match": "glob",
				"_cases": map[string][]normalizer.FlowStep{
					"post.*": {{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"publish"}}}},
				},
			}},
			wants: []string{
				`path.Match("post.*", _switchValue)`,
				`strings.TrimSpace(fmt.Sprint(req.Kind))`,
			},
		},
		{
			name: "flow.Parallel",
			step: normalizer.FlowStep{Action: "flow.Parallel", Args: map[string]any{"_branches": map[string][]normalizer.FlowStep{
				"a": {{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"one"}}}},
				"b": {{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"two"}}}},
			}}},
			wants: []string{
				"context.WithCancel(ctx)",
				"var _fp_",
				"go func() {",
			},
		},
		{
			name: "flow.Join",
			step: normalizer.FlowStep{Action: "flow.Join", Args: map[string]any{"_branches": map[string][]normalizer.FlowStep{
				"a": {{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"one"}}}},
				"b": {{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"two"}}}},
			}}},
			wants: []string{
				"var _fj_",
				"[]error",
				"go func() {",
			},
		},
		{
			name: "flow.Cron",
			step: normalizer.FlowStep{Action: "flow.Cron", Args: map[string]any{"window": "Mon-Fri 09:00-17:00", "_do": []normalizer.FlowStep{{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"inside"}}}}}},
			wants: []string{
				"time.Now().UTC()",
				"_cronMatch",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := renderOneFlowStep(newInfraTestFlowState(), tc.step, 1)
			if strings.TrimSpace(got) == "" {
				t.Fatalf("expected rendered code for %s", tc.step.Action)
			}
			for _, needle := range tc.wants {
				if !strings.Contains(got, needle) {
					t.Fatalf("expected %s to contain %q, got:\n%s", tc.step.Action, needle, got)
				}
			}
			for _, needle := range tc.avoid {
				if strings.Contains(got, needle) {
					t.Fatalf("did not expect %s to contain %q, got:\n%s", tc.step.Action, needle, got)
				}
			}
		})
	}
}

func TestRenderFlowActionMatrix_InfraAndReliability(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		step  normalizer.FlowStep
		wants []string
		avoid []string
	}{
		{
			name: "http.Request",
			step: normalizer.FlowStep{Action: "http.Request", Args: map[string]any{"method": "GET", "url": `"https://example.com"`, "output": "body", "statusVar": "status"}},
			wants: []string{
				"http.NewRequestWithContext",
				"http.DefaultClient.Do",
				"status := _httpRes",
			},
		},
		{
			name: "http.RetryPolicy",
			step: normalizer.FlowStep{Action: "http.RetryPolicy", Args: map[string]any{"method": "POST", "url": `"https://example.com"`, "body": `"{}"`, "output": "body", "attempts": 3}},
			wants: []string{
				"for _httpTry",
				"http.DefaultClient.Do",
				"http.RetryPolicy:",
			},
		},
		{
			name: "http.Paginate",
			step: normalizer.FlowStep{Action: "http.Paginate", Args: map[string]any{"url": `"https://example.com/items"`, "into": "PagedResponse", "as": "page", "cursor_expr": "page.NextCursor", "items_expr": "page.Items", "output": "items", "output_type": "[]Item"}},
			wants: []string{
				"url.Parse(",
				"json.Unmarshal(",
				"items = append(items, page.Items...)",
			},
		},
		{
			name: "url.Build segments",
			step: normalizer.FlowStep{Action: "url.Build", Args: map[string]any{"base": `"https://example.com/base"`, "segments": []string{`"verify"`, "req.Token"}, "output": "link"}},
			wants: []string{
				"url.Parse(",
				"path.Join(",
				"link := _url",
			},
		},
		{
			name: "secret.Get",
			step: normalizer.FlowStep{Action: "secret.Get", Args: map[string]any{"key": `"SMTP_PASSWORD"`, "output": "smtpPassword"}},
			wants: []string{
				`var smtpPassword string = os.Getenv("SMTP_PASSWORD")`,
			},
		},
		{
			name: "config.Get",
			step: normalizer.FlowStep{Action: "config.Get", Args: map[string]any{"key": `"APP_ENV"`, "output": "env", "default": `"dev"`}},
			wants: []string{
				`var env string = os.Getenv("APP_ENV")`,
				`env = "dev"`,
			},
		},
		{
			name: "idem.Check",
			step: normalizer.FlowStep{Action: "idem.Check", Args: map[string]any{"key": "idemKey"}},
			wants: []string{
				"// idem.Check",
				"s.stateStore.Get(ctx, idemKey)",
			},
		},
		{
			name: "idem.SaveResult",
			step: normalizer.FlowStep{Action: "idem.SaveResult", Args: map[string]any{"key": "idemKey", "ttl": "24*time.Hour"}},
			wants: []string{
				"json.Marshal(resp)",
				"s.stateStore.Set(ctx, idemKey",
			},
		},
		{
			name: "dedupe.Once",
			step: normalizer.FlowStep{Action: "dedupe.Once", Args: map[string]any{"key": `"job:1"`, "_do": []normalizer.FlowStep{{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"done"}}}}}},
			wants: []string{
				"// dedupe.Once",
				`s.stateStore.Get(ctx, "job:1")`,
				`s.stateStore.Set(ctx, "job:1", []byte("1")`,
			},
		},
		{
			name: "circuit.Check",
			step: normalizer.FlowStep{Action: "circuit.Check", Args: map[string]any{"name": `"payments"`}},
			wants: []string{
				`"circuit:open:" + "payments"`,
				"circuit.Check",
			},
		},
		{
			name: "circuit.RecordSuccess",
			step: normalizer.FlowStep{Action: "circuit.RecordSuccess", Args: map[string]any{"name": `"payments"`}},
			wants: []string{
				`Delete(ctx, "circuit:count:"+"payments")`,
				`Delete(ctx, "circuit:open:"+"payments")`,
			},
		},
		{
			name: "circuit.RecordFailure",
			step: normalizer.FlowStep{Action: "circuit.RecordFailure", Args: map[string]any{"name": `"payments"`, "threshold": 3}},
			wants: []string{
				`"circuit:count:" + "payments"`,
				"json.Marshal(",
				`Set(ctx, "circuit:open:"+"payments", []byte("1")`,
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := renderOneFlowStep(newInfraTestFlowState(), tc.step, 1)
			if strings.TrimSpace(got) == "" {
				t.Fatalf("expected rendered code for %s", tc.step.Action)
			}
			for _, needle := range tc.wants {
				if !strings.Contains(got, needle) {
					t.Fatalf("expected %s to contain %q, got:\n%s", tc.step.Action, needle, got)
				}
			}
		})
	}
}

func TestRenderFlowActionMatrix_DomainAndState(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		step  normalizer.FlowStep
		wants []string
		avoid []string
	}{
		{
			name: "auth.RequireRole",
			step: normalizer.FlowStep{Action: "auth.RequireRole", Args: map[string]any{"userID": "req.UserID", "companyID": "req.CompanyID", "roles": "[]string{\"admin\"}", "output": "currentUser"}},
			wants: []string{
				"s.UserRepo.FindByID(ctx, req.UserID)",
				"helpers.HasRole(currentUser.Role",
			},
		},
		{
			name: "auth.CheckRole",
			step: normalizer.FlowStep{Action: "auth.CheckRole", Args: map[string]any{"user": "currentUser", "roles": "[]string{\"admin\"}", "companyID": "req.CompanyID"}},
			wants: []string{
				"helpers.HasRole(currentUser.Role",
				"currentUser.CompanyID != req.CompanyID",
			},
		},
		{
			name: "entity.PatchNonZero",
			step: normalizer.FlowStep{Action: "entity.PatchNonZero", Args: map[string]any{"target": "user", "from": "req", "fields": "Name,Email"}},
			wants: []string{
				`helpers.CopyNonEmptyFields(&user, req, "Name", "Email")`,
			},
		},
		{
			name: "field.CopyNonEmpty",
			step: normalizer.FlowStep{Action: "field.CopyNonEmpty", Args: map[string]any{"from": "req", "to": "user", "fields": "Name,Email"}},
			wants: []string{
				`helpers.CopyNonEmptyFields(&user, req, "Name", "Email")`,
			},
		},
		{
			name: "entity.PatchValidated",
			step: normalizer.FlowStep{Action: "entity.PatchValidated", Args: map[string]any{"target": "user", "from": "req", "source": "User", "fields": map[string]map[string]string{"Email": {"normalize": "lower", "format": "email", "unique": "FindByEmail"}}}},
			wants: []string{
				"user.Email = strings.ToLower(strings.TrimSpace(req.Email))",
				"helpers.IsEmail(user.Email)",
				"s.UserRepo.FindByEmail(ctx, user.Email)",
			},
		},
		{
			name: "time.Parse",
			step: normalizer.FlowStep{Action: "time.Parse", Args: map[string]any{"value": "req.StartAt", "output": "startAt"}},
			wants: []string{
				"time.Parse(",
				"INVALID_DATE",
			},
		},
		{
			name: "time.Add",
			step: normalizer.FlowStep{Action: "time.Add", Args: map[string]any{"input": "expiresAt", "duration": "15*time.Minute", "output": "extendedAt"}},
			wants: []string{
				"extendedAt := expiresAt.Add(15*time.Minute)",
			},
		},
		{
			name: "time.Sub",
			step: normalizer.FlowStep{Action: "time.Sub", Args: map[string]any{"a": "expiresAt", "b": "issuedAt", "output": "ttl"}},
			wants: []string{
				"ttl := expiresAt.Sub(issuedAt)",
			},
		},
		{
			name: "time.Diff",
			step: normalizer.FlowStep{Action: "time.Diff", Args: map[string]any{"from": "issuedAt", "to": "expiresAt", "unit": "minutes", "output": "ttlMinutes"}},
			wants: []string{
				"ttlMinutes := expiresAt.Sub(issuedAt).Minutes()",
			},
		},
		{
			name: "time.CheckExpiry",
			step: normalizer.FlowStep{Action: "time.CheckExpiry", Args: map[string]any{"value": "req.ExpiresAt", "throw": `"expired"`}},
			wants: []string{
				"time.Parse(time.RFC3339, req.ExpiresAt)",
				"EXPIRED",
			},
		},
		{
			name: "map.New",
			step: normalizer.FlowStep{Action: "map.New", Args: map[string]any{"output": "labels", "type": "map[string]string"}},
			wants: []string{
				"labels := make(map[string]string)",
			},
		},
		{
			name: "map.Build",
			step: normalizer.FlowStep{Action: "map.Build", Args: map[string]any{"from": "items", "as": "item", "key": "item.ID", "value": "item.Name", "valueType": "string", "output": "byID"}},
			wants: []string{
				"byID := make(map[string]string, len(items))",
				"byID[item.ID] = item.Name",
			},
		},
		{
			name: "repo.Upsert",
			step: normalizer.FlowStep{Action: "repo.Upsert", Args: map[string]any{"source": "User", "find": "req.ID", "input": "reqUser", "output": "user", "_ifNew": []normalizer.FlowStep{{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"created"}}}}}},
			wants: []string{
				"s.UserRepo.FindByID(ctx, req.ID)",
				"s.UserRepo.Save(ctx, user)",
			},
		},
		{
			name: "notification.Dispatch",
			step: normalizer.FlowStep{Action: "notification.Dispatch", Args: map[string]any{"event": `"invite.sent"`, "userID": "req.UserID", "payload": "req"}},
			wants: []string{
				"s.dispatcher.Dispatch(ctx, port.NotificationMessage{Event: strings.TrimSpace(fmt.Sprint(\"invite.sent\"))",
			},
		},
		{
			name: "state.Set",
			step: normalizer.FlowStep{Action: "state.Set", Args: map[string]any{"key": `"draft:1"`, "value": "req", "ttl": "time.Minute"}},
			wants: []string{
				"json.Marshal(req)",
				`s.stateStore.Set(ctx, "draft:1"`,
			},
		},
		{
			name: "state.Delete",
			step: normalizer.FlowStep{Action: "state.Delete", Args: map[string]any{"key": `"draft:1"`}},
			wants: []string{
				`s.stateStore.Delete(ctx, "draft:1")`,
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := renderOneFlowStep(newInfraTestFlowState(), tc.step, 1)
			if strings.TrimSpace(got) == "" {
				t.Fatalf("expected rendered code for %s", tc.step.Action)
			}
			for _, needle := range tc.wants {
				if !strings.Contains(got, needle) {
					t.Fatalf("expected %s to contain %q, got:\n%s", tc.step.Action, needle, got)
				}
			}
			for _, needle := range tc.avoid {
				if strings.Contains(got, needle) {
					t.Fatalf("did not expect %s to contain %q, got:\n%s", tc.step.Action, needle, got)
				}
			}
		})
	}
}

func TestRenderFlowActionMatrix_InvalidConfigPaths(t *testing.T) {
	t.Parallel()

	cases := []struct {
		action string
		step   normalizer.FlowStep
		want   string
	}{
		{action: "config.Get", step: normalizer.FlowStep{Action: "config.Get", Args: map[string]any{"output": "cfg"}}, want: `config.Get: config.Get requires key and output`},
		{action: "http.Request", step: normalizer.FlowStep{Action: "http.Request", Args: map[string]any{"url": `"https://example.com"`}}, want: `http.Request: http.Request requires method and url`},
		{action: "http.RetryPolicy", step: normalizer.FlowStep{Action: "http.RetryPolicy", Args: map[string]any{"method": "GET"}}, want: `http.RetryPolicy: http.RetryPolicy requires method and url`},
		{action: "http.Paginate", step: normalizer.FlowStep{Action: "http.Paginate", Args: map[string]any{"url": `"https://example.com"`}}, want: `http.Paginate: http.Paginate requires url, into, as, and cursor_expr`},
		{action: "flow.Tag", step: normalizer.FlowStep{Action: "flow.Tag", Args: map[string]any{}}, want: `flow.Tag: flow.Tag requires name`},
		{action: "flow.Call", step: normalizer.FlowStep{Action: "flow.Call", Args: map[string]any{}}, want: `flow.Call: flow.Call requires op`},
		{action: "auth.RequireRole", step: normalizer.FlowStep{Action: "auth.RequireRole", Args: map[string]any{"userID": "req.UserID"}}, want: `auth.RequireRole: auth.RequireRole requires userID, companyID, and roles`},
		{action: "audit.Log", step: normalizer.FlowStep{Action: "audit.Log", Args: map[string]any{"actor": "req.UserID"}}, want: `audit.Log: audit.Log requires actor, company, and event`},
		{action: "list.Sum", step: normalizer.FlowStep{Action: "list.Sum", Args: map[string]any{"input": "items"}}, want: `list.Sum: list.Sum requires input and output`},
		{action: "dedupe.Once", step: normalizer.FlowStep{Action: "dedupe.Once", Args: map[string]any{}}, want: `dedupe.Once: dedupe.Once requires key`},
		{action: "circuit.RecordSuccess", step: normalizer.FlowStep{Action: "circuit.RecordSuccess", Args: map[string]any{}}, want: `circuit.RecordSuccess: circuit.RecordSuccess requires name`},
		{action: "flow.Cron", step: normalizer.FlowStep{Action: "flow.Cron", Args: map[string]any{}}, want: `flow.Cron: flow.Cron requires window`},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.action, func(t *testing.T) {
			t.Parallel()
			got := renderOneFlowStep(newInfraTestFlowState(), tc.step, 1)
			if !strings.Contains(got, tc.want) {
				t.Fatalf("expected invalid-config path for %s to contain %q, got:\n%s", tc.action, tc.want, got)
			}
		})
	}
}

func TestRenderFlowActionMatrix_RemainingCoverage(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		step  normalizer.FlowStep
		wants []string
		avoid []string
	}{
		{
			name: "archive.ZipDir",
			step: normalizer.FlowStep{Action: "archive.ZipDir", Args: map[string]any{"path": `"./tmp"`, "output": "zipBytes"}},
			wants: []string{
				"zip.NewWriter(",
				"filepath.Walk(",
			},
		},
		{
			name: "cast.ToString",
			step: normalizer.FlowStep{Action: "cast.ToString", Args: map[string]any{"input": "req.UserID", "output": "userIDString"}},
			wants: []string{
				"fmt.Sprint(req.UserID)",
			},
		},
		{
			name: "claude.Chat",
			step: normalizer.FlowStep{Action: "claude.Chat", Args: map[string]any{"user_message": `"hello"`, "output": "reply"}},
			wants: []string{
				"https://api.anthropic.com/v1/messages",
				"ANTHROPIC_API_KEY",
			},
		},
		{
			name: "convert.ToFloat",
			step: normalizer.FlowStep{Action: "convert.ToFloat", Args: map[string]any{"input": "req.Count", "output": "countFloat"}},
			wants: []string{
				"countFloat := float64(req.Count)",
			},
		},
		{
			name: "convert.ToInt",
			step: normalizer.FlowStep{Action: "convert.ToInt", Args: map[string]any{"input": "req.Count", "output": "countInt"}},
			wants: []string{
				"countInt := int64(req.Count)",
			},
		},
		{
			name: "crypto.Hash",
			step: normalizer.FlowStep{Action: "crypto.Hash", Args: map[string]any{"input": "req.Password", "output": "passwordHash"}},
			wants: []string{
				"sha256.Sum256([]byte(req.Password))",
				"hex.EncodeToString",
			},
		},
		{
			name: "db.Get",
			step: normalizer.FlowStep{Action: "db.Get", Args: map[string]any{"source": "User", "input": "req.UserID", "output": "user"}},
			wants: []string{
				"s.UserRepo.FindByID(ctx, req.UserID)",
			},
		},
		{
			name: "enum.Validate",
			step: normalizer.FlowStep{Action: "enum.Validate", Args: map[string]any{"value": "req.Status", "allowed": "draft,published", "throw": "invalid status"}},
			wants: []string{
				"helpers.IsOneOf(req.Status, []string{\"draft\", \"published\"})",
			},
		},
		{
			name: "event.Match",
			step: normalizer.FlowStep{Action: "event.Match", Args: map[string]any{"event": "evt", "match": `"order.created"`}},
			wants: []string{
				"_eventMatch_",
				"helpers.MatchEvent(evt, _eventMatch_",
			},
		},
		{
			name: "event.Subscribe",
			step: normalizer.FlowStep{Action: "event.Subscribe", Args: map[string]any{"name": `"OrderCreated"`, "match": `"tenant=acme"`, "_do": []normalizer.FlowStep{{Action: "flow.SuggestNext", Args: map[string]any{"options": []string{"seen"}}}}}},
			wants: []string{
				"_subName_",
				"s.publisher.Subscribe(ctx, _subName_",
				"func(ctx context.Context, evt any)",
			},
		},
		{
			name: "fs.WriteFile",
			step: normalizer.FlowStep{Action: "fs.WriteFile", Args: map[string]any{"path": `"/tmp/out.txt"`, "data": `"hello"`}},
			wants: []string{
				"os.MkdirAll(filepath.Dir(",
				"os.WriteFile(",
			},
		},
		{
			name: "list.Avg",
			step: normalizer.FlowStep{Action: "list.Avg", Args: map[string]any{"input": "items", "field": "Price", "output": "avgPrice"}},
			wants: []string{
				"var _avgSum",
				"avgPrice := 0.0",
			},
		},
		{
			name: "list.Enrich",
			step: normalizer.FlowStep{Action: "list.Enrich", Args: map[string]any{"items": "items", "lookupSource": "User", "lookupInput": "_item.UserID", "set": "AuthorName=Name"}},
			wants: []string{
				"s.UserRepo.FindByID(ctx, _item.UserID)",
				"_item.AuthorName = _enriched.Name",
			},
		},
		{
			name: "list.Len",
			step: normalizer.FlowStep{Action: "list.Len", Args: map[string]any{"input": "items", "output": "itemCount"}},
			wants: []string{
				"itemCount := len(items)",
			},
		},
		{
			name: "list.New",
			step: normalizer.FlowStep{Action: "list.New", Args: map[string]any{"output": "ids", "type": "[]string", "cap": "16"}},
			wants: []string{
				"ids := make([]string, 0, 16)",
			},
		},
		{
			name: "logic.Call",
			step: normalizer.FlowStep{Action: "logic.Call", Args: map[string]any{"func": "normalizeName", "args": []string{"req.Name"}, "output": "normalized"}},
			wants: []string{
				"normalized, err := normalizeName(req.Name)",
			},
		},
		{
			name: "mapping.Map",
			step: normalizer.FlowStep{Action: "mapping.Map", Args: map[string]any{"from": "req", "to": "user", "entity": "User"}},
			wants: []string{
				"var user domain.User",
				"helpers.Assign(&user, req)",
			},
		},
		{
			name: "math.Expr",
			step: normalizer.FlowStep{Action: "math.Expr", Args: map[string]any{"expr": "req.Price * req.Qty", "output": "total", "declare": true}},
			wants: []string{
				"total := req.Price * req.Qty",
			},
		},
		{
			name: "num.Add",
			step: normalizer.FlowStep{Action: "num.Add", Args: map[string]any{"a": "req.A", "b": "req.B", "output": "sum"}},
			wants: []string{
				"sum := float64(req.A) + float64(req.B)",
			},
		},
		{
			name: "session.Get",
			step: normalizer.FlowStep{Action: "session.Get", Args: map[string]any{"output": "sessionID"}},
			wants: []string{
				"sessionID := reqctx.SessionID(ctx)",
			},
		},
		{
			name: "str.StripMarkdown",
			step: normalizer.FlowStep{Action: "str.StripMarkdown", Args: map[string]any{"input": "req.Content", "output": "plain"}},
			wants: []string{
				"// str.StripMarkdown",
				"strings.TrimSpace(req.Content)",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := renderOneFlowStep(newInfraTestFlowState(), tc.step, 1)
			if strings.TrimSpace(got) == "" {
				t.Fatalf("expected rendered code for %s", tc.step.Action)
			}
			for _, needle := range tc.wants {
				if !strings.Contains(got, needle) {
					t.Fatalf("expected %s to contain %q, got:\n%s", tc.step.Action, needle, got)
				}
			}
		})
	}
}

func TestRenderFlowActionMatrix_NewPrimitives(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		step  normalizer.FlowStep
		wants []string
		avoid []string
	}{
		{
			name: "repo.Exists",
			step: normalizer.FlowStep{Action: "repo.Exists", Args: map[string]any{"source": "User", "input": "req.UserID", "output": "exists"}},
			wants: []string{
				"s.UserRepo.FindByID(ctx, req.UserID)",
				"exists := _repoExists",
				"!= nil",
			},
		},
		{
			name: "repo.Exists bool method",
			step: normalizer.FlowStep{Action: "repo.Exists", Args: map[string]any{"source": "User", "method": "ExistsByEmail", "input": "req.Email", "output": "exists"}},
			wants: []string{
				"exists, err := s.UserRepo.ExistsByEmail(ctx, req.Email)",
			},
			avoid: []string{
				"_repoExists",
				"!= nil",
			},
		},
		{
			name: "repo.Count",
			step: normalizer.FlowStep{Action: "repo.Count", Args: map[string]any{"source": "Post", "method": "CountByAuthorID", "input": "req.AuthorID", "output": "count"}},
			wants: []string{
				"count, err := s.PostRepo.CountByAuthorID(ctx, req.AuthorID)",
			},
		},
		{
			name: "map.Get",
			step: normalizer.FlowStep{Action: "map.Get", Args: map[string]any{"input": "byID", "key": "req.ID", "output": "name", "into": "string", "default": `"unknown"`, "found": "ok"}},
			wants: []string{
				"_mapVal",
				"_mapFound",
				"name = _typedVal",
				`name = "unknown"`,
				"ok :=",
			},
		},
		{
			name: "map.Has",
			step: normalizer.FlowStep{Action: "map.Has", Args: map[string]any{"input": "byID", "key": "req.ID", "output": "exists"}},
			wants: []string{
				"exists := func() bool { _, _ok := byID[req.ID]; return _ok }()",
			},
		},
		{
			name: "map.Set",
			step: normalizer.FlowStep{Action: "map.Set", Args: map[string]any{"input": "labels", "key": `"status"`, "value": `"ready"`, "output": "nextLabels"}},
			wants: []string{
				"nextLabels := maps.Clone(labels)",
				`nextLabels["status"] = "ready"`,
			},
		},
		{
			name: "map.Merge",
			step: normalizer.FlowStep{Action: "map.Merge", Args: map[string]any{"left": "baseLabels", "right": "extraLabels", "output": "mergedLabels"}},
			wants: []string{
				"mergedLabels := maps.Clone(baseLabels)",
				"maps.Copy(mergedLabels, extraLabels)",
			},
		},
		{
			name: "value.Coalesce",
			step: normalizer.FlowStep{Action: "value.Coalesce", Args: map[string]any{"values": []string{"req.DisplayName", "req.Email", `"anonymous"`}, "output": "displayName", "into": "string"}},
			wants: []string{
				"_coalesceFound",
				"[]any{req.DisplayName, req.Email, \"anonymous\"}",
				"displayName = _coalesceTyped",
			},
		},
		{
			name: "errors.New",
			step: normalizer.FlowStep{Action: "errors.New", Args: map[string]any{"message": `"boom"`, "status": "404", "code": `"NOT_FOUND"`, "output": "errObj"}},
			wants: []string{
				"var errObj error",
				`errObj = errors.New(http.StatusNotFound, "NOT_FOUND", "boom")`,
			},
		},
		{
			name: "errors.ThrowIf",
			step: normalizer.FlowStep{Action: "errors.ThrowIf", Args: map[string]any{"condition": "user == nil", "throw": "user missing", "status": "404", "code": "NOT_FOUND"}},
			wants: []string{
				"if user == nil {",
				`errors.New(http.StatusNotFound, "NOT_FOUND", "user missing")`,
			},
		},
		{
			name: "errors.Map",
			step: normalizer.FlowStep{Action: "errors.Map", Args: map[string]any{
				"input": "err",
				"cases": map[string]map[string]string{
					"no rows": {"status": "http.StatusNotFound", "code": "NOT_FOUND", "message": "resource missing"},
				},
				"defaultMessage": "unexpected upstream failure",
				"defaultCode":    "UPSTREAM_ERROR",
			}},
			wants: []string{
				"strings.Contains(_mappedErrMsg",
				`errors.New(http.StatusNotFound, "NOT_FOUND", "resource missing")`,
				`errors.New(http.StatusInternalServerError, "UPSTREAM_ERROR", "unexpected upstream failure")`,
			},
		},
		{
			name: "errors.Wrap",
			step: normalizer.FlowStep{Action: "errors.Wrap", Args: map[string]any{"err": "err", "message": `"save failed"`}},
			wants: []string{
				`fmt.Errorf("%s: %w", "save failed", err)`,
			},
		},
		{
			name: "event.EmitIf",
			step: normalizer.FlowStep{Action: "event.EmitIf", Args: map[string]any{"condition": "req.Notify", "name": "UserRegistered", "payloadMap": map[string]string{"UserID": "user.ID"}}},
			wants: []string{
				"if req.Notify {",
				"s.publisher.PublishUserRegistered",
			},
		},
		{
			name: "json.Stringify",
			step: normalizer.FlowStep{Action: "json.Stringify", Args: map[string]any{"input": "resp", "output": "raw"}},
			wants: []string{
				"json.Marshal(resp)",
				"raw := string(",
			},
		},
		{
			name: "template.Render",
			step: normalizer.FlowStep{Action: "template.Render", Args: map[string]any{"template": `"Hello {{.Name}}"`, "data": "map[string]any{\"Name\": req.Name}", "output": "body"}},
			wants: []string{
				`template.New("flow").Parse("Hello {{.Name}}")`,
				"bytes.Buffer",
				"body := _tmplBuf",
			},
		},
		{
			name: "openai.Embed",
			step: normalizer.FlowStep{Action: "openai.Embed", Args: map[string]any{"input": "req.Query", "output": "embedding", "output_usage": "usage", "dimensions": 256}},
			wants: []string{
				`"https://api.openai.com/v1/embeddings"`,
				`"dimensions"] = 256`,
				"embedding := _oaiEmbedParsed",
				"usage.PromptTokens = _oaiEmbedParsed",
			},
		},
		{
			name: "token.Generate",
			step: normalizer.FlowStep{Action: "token.Generate", Args: map[string]any{"subject": "user.ID", "purpose": `"verify_email"`, "ttl": `"30m"`, "claims": `map[string]any{"email": user.Email}`, "secret": `"secret"`, "output": "token"}},
			wants: []string{
				`_tokenPayload`,
				`_tokenPayload`,
				`["sub"] = user.ID`,
				`["purpose"] = "verify_email"`,
				`token := _tokenUnsigned`,
			},
		},
		{
			name: "token.Verify",
			step: normalizer.FlowStep{Action: "token.Verify", Args: map[string]any{"token": "req.Token", "purpose": `"verify_email"`, "secret": `"secret"`, "output": "claims"}},
			wants: []string{
				"strings.Split(req.Token, \".\")",
				`TOKEN_INVALID`,
				`TOKEN_PURPOSE_MISMATCH`,
				"claims := _tokenClaims",
			},
		},
		{
			name: "list.Find",
			step: normalizer.FlowStep{Action: "list.Find", Args: map[string]any{"from": "items", "as": "item", "condition": `item.ID == req.ID`, "output": "match", "into": "Item", "found": "matchFound"}},
			wants: []string{
				"matchFound := false",
				"for _, item := range items",
				"match = item",
			},
		},
		{
			name: "list.Any",
			step: normalizer.FlowStep{Action: "list.Any", Args: map[string]any{"from": "items", "as": "item", "condition": `item.Active`, "output": "hasActive"}},
			wants: []string{
				"hasActive := false",
				"if item.Active {",
				"hasActive = true",
			},
		},
		{
			name: "list.All",
			step: normalizer.FlowStep{Action: "list.All", Args: map[string]any{"from": "items", "as": "item", "condition": `item.Active`, "output": "allActive"}},
			wants: []string{
				"allActive := true",
				"if !(item.Active) {",
				"allActive = false",
			},
		},
		{
			name: "mutex.With",
			step: normalizer.FlowStep{Action: "mutex.With", Args: map[string]any{"key": `"jobs:sync"`, "wait": "2 * time.Second", "poll": "25 * time.Millisecond", "_do": []normalizer.FlowStep{{Action: "log.Emit", Args: map[string]any{"message": `"inside lock"`}}}}},
			wants: []string{
				`_FlowMutexForKey(fmt.Sprint("jobs:sync"))`,
				"TryLock()",
				"time.Sleep(_mutexPoll",
				"defer _mutex",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := renderOneFlowStep(newInfraTestFlowState(), tc.step, 1)
			if strings.TrimSpace(got) == "" {
				t.Fatalf("expected rendered code for %s", tc.step.Action)
			}
			for _, needle := range tc.wants {
				if !strings.Contains(got, needle) {
					t.Fatalf("expected %s to contain %q, got:\n%s", tc.step.Action, needle, got)
				}
			}
		})
	}
}
