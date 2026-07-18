package emitter

import (
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

func TestRenderFlowStepInfra_DispatchHandlesKnownActions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		step normalizer.FlowStep
	}{
		{name: "cache_get", step: normalizer.FlowStep{Action: "cache.Get", Args: map[string]any{"key": `"k"`, "output": "cached"}}},
		{name: "cache_set", step: normalizer.FlowStep{Action: "cache.Set", Args: map[string]any{"key": `"k"`, "value": `"v"`, "ttl": "time.Minute"}}},
		{name: "cache_del", step: normalizer.FlowStep{Action: "cache.Del", Args: map[string]any{"key": `"k"`}}},
		{name: "mail_send", step: normalizer.FlowStep{Action: "mail.Send", Args: map[string]any{"to": "req.Email", "subject": `"Hello"`, "body": `"Body"`}}},
		{name: "storage_upload", step: normalizer.FlowStep{Action: "storage.Upload", Args: map[string]any{"key": `"path/file"`, "data": "req.Payload", "output": "url"}}},
		{name: "storage_download", step: normalizer.FlowStep{Action: "storage.Download", Args: map[string]any{"key": `"path/file"`, "output": "blob"}}},
		{name: "storage_geturl", step: normalizer.FlowStep{Action: "storage.GetURL", Args: map[string]any{"key": `"path/file"`, "output": "publicURL"}}},
		{name: "storage_delete", step: normalizer.FlowStep{Action: "storage.Delete", Args: map[string]any{"key": `"path/file"`}}},
		{name: "storage_list", step: normalizer.FlowStep{Action: "storage.List", Args: map[string]any{"prefix": `"path/"`, "output": "keys"}}},
		{name: "http_call", step: normalizer.FlowStep{Action: "http.Call", Args: map[string]any{"method": "POST", "url": `"https://example.com"`, "body": `"{}"`, "output": "httpBody", "statusVar": "httpStatus"}}},
		{name: "http_soap", step: normalizer.FlowStep{Action: "http.SOAP", Args: map[string]any{"url": `"https://example.com/soap"`, "namespace": `"urn:test"`, "operation": `"CheckVat"`, "request": map[string]string{"countryCode": `"DE"`, "vatNumber": "req.VAT"}, "into": "VIESResponse", "output": "soapResp", "statusVar": "soapStatus"}}},
		{name: "rand_code", step: normalizer.FlowStep{Action: "rand.Code", Args: map[string]any{"output": "otp", "length": 6}}},
		{name: "rand_token", step: normalizer.FlowStep{Action: "rand.Token", Args: map[string]any{"output": "token", "bytes": 16}}},
		{name: "str_format", step: normalizer.FlowStep{Action: "str.Format", Args: map[string]any{"template": `"u:%s"`, "args": []string{"req.UserID"}, "output": "line"}}},
		{name: "str_replace_all", step: normalizer.FlowStep{Action: "str.ReplaceAll", Args: map[string]any{"input": "req.Path", "old": `"\\"`, "new": `"/"`, "output": "normPath"}}},
		{name: "str_trim_space", step: normalizer.FlowStep{Action: "str.TrimSpace", Args: map[string]any{"input": "req.Name", "output": "trimName"}}},
		{name: "json_parse", step: normalizer.FlowStep{Action: "json.Parse", Args: map[string]any{"input": "req.Raw", "into": "map[string]any", "output": "parsed"}}},
		{name: "json_marshal", step: normalizer.FlowStep{Action: "json.Marshal", Args: map[string]any{"input": "req.Payload", "output": "rawJSON"}}},
		{name: "parallel_run", step: normalizer.FlowStep{Action: "parallel.Run", Args: map[string]any{"_branches": map[string][]normalizer.FlowStep{"a": []normalizer.FlowStep{}, "b": []normalizer.FlowStep{}}}}},
		{name: "pdf_render", step: normalizer.FlowStep{Action: "pdf.Render", Args: map[string]any{"template": `"t"`, "data": "req.ReportData", "output": "pdfBytes"}}},
		{name: "webhook_send", step: normalizer.FlowStep{Action: "webhook.Send", Args: map[string]any{"url": `"https://hook.example"`, "payload": "req.Payload", "event": `"evt"`}}},
		{name: "queue_enqueue", step: normalizer.FlowStep{Action: "queue.Enqueue", Args: map[string]any{"subject": `"events.test"`, "payload": "req.Payload", "timeout": "2*time.Second"}}},
		{name: "webhook_verify", step: normalizer.FlowStep{Action: "webhook.VerifySignature", Args: map[string]any{"payload": "req.Body", "signature": "req.Signature", "output": "sigOK"}}},
		{name: "webhook_ack", step: normalizer.FlowStep{Action: "webhook.Ack", Args: map[string]any{"status": 202, "body": `"accepted"`}}},
		{name: "queue_dequeue", step: normalizer.FlowStep{Action: "queue.Dequeue", Args: map[string]any{"subject": `"events.test"`, "output": "msg", "ackToken": "msgID", "timeout": "2*time.Second", "attempts": 3, "backoffMs": 50, "jitterMs": 10}}},
		{name: "queue_ack", step: normalizer.FlowStep{Action: "queue.Ack", Args: map[string]any{"subject": `"events.test"`, "messageID": "msgID"}}},
		{name: "queue_nack", step: normalizer.FlowStep{Action: "queue.Nack", Args: map[string]any{"subject": `"events.test"`, "messageID": "msgID", "reason": `"decode failed"`}}},
		{name: "dlq_publish", step: normalizer.FlowStep{Action: "dlq.Publish", Args: map[string]any{"subject": `"events.test"`, "payload": "msg", "reason": `"decode failed"`}}},
		{name: "event_outbox", step: normalizer.FlowStep{Action: "event.Outbox", Args: map[string]any{"name": `"ProjectCreated"`, "payload": "domain.ProjectCreated{ID: req.ID}"}}},
		{name: "notify_send", step: normalizer.FlowStep{Action: "notify.Send", Args: map[string]any{"channel": `"email"`, "to": "req.Email", "text": `"Hello"`}}},
		{name: "notify_email", step: normalizer.FlowStep{Action: "notify.Email", Args: map[string]any{"to": "req.Email", "text": `"Hello"`}}},
		{name: "approval_request", step: normalizer.FlowStep{Action: "approval.Request", Args: map[string]any{"approvalKey": `"refund:123"`, "title": `"Refund approval"`, "requestedBy": "req.UserID", "approvers": []string{"manager@acme.io"}, "policy": `"any"`, "payload": "req", "approvalId": "approvalID", "status": "approvalStatus"}}},
		{name: "approval_wait", step: normalizer.FlowStep{Action: "approval.Wait", Args: map[string]any{"approvalId": "approvalID", "timeout": "10*time.Minute", "decision": "approvalDecision", "status": "approvalStatus"}}},
		{name: "approval_decide", step: normalizer.FlowStep{Action: "approval.Decide", Args: map[string]any{"approvalId": "approvalID", "decision": `"approved"`, "actor": "req.UserID", "status": "approvalStatus"}}},
		{name: "policy_evaluate", step: normalizer.FlowStep{Action: "policy.Evaluate", Args: map[string]any{"policyKey": `"project.create"`, "subject": "req.UserID", "operation": `"create"`, "decision": "policyDecision", "reason": "policyReason", "effects": "policyEffects", "output": "policyResult"}}},
		{name: "policy_check", step: normalizer.FlowStep{Action: "policy.Check", Args: map[string]any{"policy": `"CompanyAdminOnly"`, "user": "currentUser", "companyID": "req.CompanyID", "_policyResolved": true, "_policyRoles": []string{"owner", "admin"}, "_policySameCompany": true, "_policyAllowAdminOverride": true}}},
		{name: "policy_require", step: normalizer.FlowStep{Action: "policy.Require", Args: map[string]any{"policyKey": `"project.create"`, "subject": "req.UserID", "operation": `"create"`, "throw": `"forbidden"`}}},
		{name: "policy_decide", step: normalizer.FlowStep{Action: "policy.Decide", Args: map[string]any{"policyKey": `"project.create"`, "subject": "req.UserID", "operation": `"create"`, "output": "decisionObj"}}},
		{name: "idempotency_derive", step: normalizer.FlowStep{Action: "idempotency.DeriveKey", Args: map[string]any{"from": []string{"req.UserID", "req.OrderID"}, "output": "idemKey"}}},
		{name: "idempotency_check", step: normalizer.FlowStep{Action: "idempotency.Check", Args: map[string]any{"key": "idemKey"}}},
		{name: "idempotency_save", step: normalizer.FlowStep{Action: "idempotency.SaveResult", Args: map[string]any{"key": "idemKey", "ttl": "24*time.Hour"}}},
		{name: "ratelimit_limit", step: normalizer.FlowStep{Action: "ratelimit.Limit", Args: map[string]any{"key": "req.UserID", "rps": 20}}},
		{name: "quota_check", step: normalizer.FlowStep{Action: "quota.Check", Args: map[string]any{"key": "req.UserID", "limit": 100, "window": `"day"`}}},
		{name: "budget_check", step: normalizer.FlowStep{Action: "budget.Check", Args: map[string]any{"key": "req.UserID", "limit": 5000}}},
		{name: "budget_consume", step: normalizer.FlowStep{Action: "budget.Consume", Args: map[string]any{"key": "req.UserID", "tokens": "reply.TokensUsed"}}},
		{name: "context_trim", step: normalizer.FlowStep{Action: "context.Trim", Args: map[string]any{"input": "project.CueContent", "output": "trimmedCue", "max_bytes": 12000}}},
		{name: "profile_require", step: normalizer.FlowStep{Action: "profile.Require", Args: map[string]any{"key": "req.UserID", "tier": `"ops"`}}},
		{name: "plan_build_automata", step: normalizer.FlowStep{Action: "plan.BuildAutomata", Args: map[string]any{"input": "usecasesDoc", "output": "automataDoc"}}},
		{name: "plan_build_micro_plan", step: normalizer.FlowStep{Action: "plan.BuildMicroPlan", Args: map[string]any{"usecases": "usecasesDoc", "automata": "automataDoc", "output": "microPlanDoc"}}},
		{name: "cue_emit_project", step: normalizer.FlowStep{Action: "cue.EmitProject", Args: map[string]any{"usecases": "usecasesDoc", "micro_plan": "microPlanDoc", "output": "projectFiles"}}},
		{name: "cue_validate_project", step: normalizer.FlowStep{Action: "cue.ValidateProject", Args: map[string]any{"files": "projectFiles", "output": "validation"}}},
		{name: "cue_write_project_files", step: normalizer.FlowStep{Action: "cue.WriteProjectFiles", Args: map[string]any{"root": "\"/tmp/project\"", "files": "projectFiles", "output": "writeResult"}}},
		{name: "concurrency_run", step: normalizer.FlowStep{Action: "concurrency.Run", Args: map[string]any{"key": `"build"`, "max": 8, "_do": []normalizer.FlowStep{{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}}}}}},
		{name: "mutex_with", step: normalizer.FlowStep{Action: "mutex.With", Args: map[string]any{"key": `"build"`, "_do": []normalizer.FlowStep{{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}}}}}},
		{name: "circuit_breaker", step: normalizer.FlowStep{Action: "circuit.Breaker", Args: map[string]any{"name": `"external-api"`, "threshold": 3, "openTTL": "30*time.Second", "_do": []normalizer.FlowStep{{Action: "http.Call", Args: map[string]any{"method": "GET", "url": `"https://api.test"`, "output": "body"}}}}}},
		{name: "bulkhead_run", step: normalizer.FlowStep{Action: "bulkhead.Run", Args: map[string]any{"name": `"s3-upload"`, "max": 12, "_do": []normalizer.FlowStep{{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}}}}}},
		{name: "log_emit", step: normalizer.FlowStep{Action: "log.Emit", Args: map[string]any{"level": `"info"`, "message": `"created project"`}}},
		{name: "metric_emit", step: normalizer.FlowStep{Action: "metric.Emit", Args: map[string]any{"name": `"project.created"`, "kind": `"counter"`, "value": "1"}}},
		{name: "trace_span", step: normalizer.FlowStep{Action: "trace.Span", Args: map[string]any{"name": `"BuildProject"`, "_do": []normalizer.FlowStep{{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}}}}}},
		{name: "slo_budget", step: normalizer.FlowStep{Action: "slo.Budget", Args: map[string]any{"name": `"build"`, "duration": "2*time.Second", "_do": []normalizer.FlowStep{{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}}}}}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			stNew := newInfraTestFlowState()
			arg := infraTestArg(tc.step)
			child := infraTestChild(tc.step)

			got := renderOneFlowStep(stNew, tc.step, 1)
			if strings.TrimSpace(got) == "" {
				t.Fatalf("dispatcher returned empty render for known infra action %q", tc.step.Action)
			}

			stLegacy := newInfraTestFlowState()
			legacy := renderFlowStepInfraLegacy(stLegacy, tc.step, 1, "_x", arg, child)
			if legacy != "" {
				t.Fatalf("legacy fallback must stay empty; got:\n%s", legacy)
			}
		})
	}
}

func TestRenderFlowStepInfra_StrFormatAssignsToStructField(t *testing.T) {
	t.Parallel()

	step := normalizer.FlowStep{
		Action: "str.Format",
		Args: map[string]any{
			"template": `"u:%s/%s"`,
			"args":     []string{"req.UserID", "req.CompanyID"},
			"output":   "resp.RedirectURL",
		},
	}

	st := newInfraTestFlowState()
	got := renderOneFlowStep(st, step, 1)
	if strings.TrimSpace(got) == "" {
		t.Fatal("dispatcher returned empty render for str.Format with dotted output")
	}
	if !strings.Contains(got, "_fmt_0 := fmt.Sprintf(") {
		t.Fatalf("expected temp fmt var, got:\n%s", got)
	}
	if !strings.Contains(got, "helpers.Assign(&resp.RedirectURL, _fmt_0)") {
		t.Fatalf("expected helper-based assignment into struct field, got:\n%s", got)
	}
}

func newInfraTestFlowState() *flowRenderState {
	n := 0
	return &flowRenderState{
		declared: map[string]bool{"resp": true, "err": true},
		pointers: map[string]bool{},
		types:    map[string]string{},
		stepN:    &n,
	}
}

func infraTestArg(step normalizer.FlowStep) func(string) string {
	return func(name string) string {
		v, ok := step.Args[name]
		if !ok {
			return ""
		}
		s, ok := v.(string)
		if !ok {
			return ""
		}
		return normalizeFlowExpr(strings.TrimSpace(s))
	}
}

func infraTestChild(step normalizer.FlowStep) func(string) []normalizer.FlowStep {
	return func(name string) []normalizer.FlowStep {
		v, ok := step.Args[name]
		if !ok {
			return nil
		}
		steps, ok := v.([]normalizer.FlowStep)
		if !ok {
			return nil
		}
		return steps
	}
}
