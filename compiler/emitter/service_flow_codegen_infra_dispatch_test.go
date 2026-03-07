package emitter

import (
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
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
		{name: "rand_code", step: normalizer.FlowStep{Action: "rand.Code", Args: map[string]any{"output": "otp", "length": 6}}},
		{name: "rand_token", step: normalizer.FlowStep{Action: "rand.Token", Args: map[string]any{"output": "token", "bytes": 16}}},
		{name: "str_format", step: normalizer.FlowStep{Action: "str.Format", Args: map[string]any{"template": `"u:%s"`, "args": []string{"req.UserID"}, "output": "line"}}},
		{name: "json_parse", step: normalizer.FlowStep{Action: "json.Parse", Args: map[string]any{"input": "req.Raw", "into": "map[string]any", "output": "parsed"}}},
		{name: "json_marshal", step: normalizer.FlowStep{Action: "json.Marshal", Args: map[string]any{"input": "req.Payload", "output": "rawJSON"}}},
		{name: "regex_match", step: normalizer.FlowStep{Action: "regex.Match", Args: map[string]any{"input": "req.Email", "pattern": `"^[^@]+@[^@]+$"`, "output": "ok"}}},
		{name: "regex_replace", step: normalizer.FlowStep{Action: "regex.Replace", Args: map[string]any{"input": "req.Name", "pattern": `"\\s+"`, "repl": `"-"`, "output": "slug"}}},
		{name: "base64_encode", step: normalizer.FlowStep{Action: "base64.Encode", Args: map[string]any{"input": "req.Payload", "output": "b64"}}},
		{name: "base64_decode", step: normalizer.FlowStep{Action: "base64.Decode", Args: map[string]any{"input": "req.Encoded", "output": "raw"}}},
		{name: "url_parse", step: normalizer.FlowStep{Action: "url.Parse", Args: map[string]any{"input": "req.URL", "output": "u"}}},
		{name: "url_build", step: normalizer.FlowStep{Action: "url.Build", Args: map[string]any{"base": `"https://api.test"`, "path": `"/v1/items"`, "query": map[string]string{"q": "req.Query"}, "output": "builtURL"}}},
		{name: "query_encode", step: normalizer.FlowStep{Action: "query.Encode", Args: map[string]any{"input": "req.QueryMap", "output": "rawQuery"}}},
		{name: "query_decode", step: normalizer.FlowStep{Action: "query.Decode", Args: map[string]any{"input": "req.RawQuery", "output": "queryVals"}}},
		{name: "hash_sum", step: normalizer.FlowStep{Action: "hash.Sum", Args: map[string]any{"algorithm": `"sha256"`, "input": "req.Payload", "output": "digest"}}},
		{name: "hash_hmac", step: normalizer.FlowStep{Action: "hash.HMAC", Args: map[string]any{"algorithm": `"sha256"`, "key": "req.Secret", "input": "req.Payload", "output": "signature"}}},
		{name: "uuid_new", step: normalizer.FlowStep{Action: "uuid.New", Args: map[string]any{"output": "id"}}},
		{name: "ulid_new", step: normalizer.FlowStep{Action: "ulid.New", Args: map[string]any{"output": "ulid"}}},
		{name: "math_op", step: normalizer.FlowStep{Action: "math.Op", Args: map[string]any{"op": `"clamp"`, "value": "req.Score", "min": "0", "max": "100", "output": "clamped"}}},
		{name: "jsonpath_get", step: normalizer.FlowStep{Action: "jsonpath.Get", Args: map[string]any{"input": "req.Payload", "path": `"$.user.email"`, "output": "email"}}},
		{name: "jsonpath_set", step: normalizer.FlowStep{Action: "jsonpath.Set", Args: map[string]any{"input": "req.Payload", "path": `"$.user.role"`, "value": `"admin"`, "output": "patched"}}},
		{name: "jwt_sign", step: normalizer.FlowStep{Action: "jwt.Sign", Args: map[string]any{"claims": "map[string]any{\"sub\": req.UserID}", "secret": `"secret"`, "ttl": `"1h"`, "output": "token"}}},
		{name: "jwt_verify", step: normalizer.FlowStep{Action: "jwt.Verify", Args: map[string]any{"token": "req.Token", "secret": `"secret"`, "output": "claims"}}},
		{name: "oauth2_token", step: normalizer.FlowStep{Action: "oauth2.Token", Args: map[string]any{"tokenURL": `"https://oauth.example/token"`, "clientID": `"id"`, "clientSecret": `"secret"`, "output": "tokenResp"}}},
		{name: "oauth2_refresh", step: normalizer.FlowStep{Action: "oauth2.Refresh", Args: map[string]any{"tokenURL": `"https://oauth.example/token"`, "refreshToken": "req.RefreshToken", "output": "tokenResp"}}},
		{name: "crypto_encrypt", step: normalizer.FlowStep{Action: "crypto.Encrypt", Args: map[string]any{"input": "req.Payload", "key": `"enc-key"`, "output": "cipher"}}},
		{name: "crypto_decrypt", step: normalizer.FlowStep{Action: "crypto.Decrypt", Args: map[string]any{"input": "req.Cipher", "key": `"enc-key"`, "output": "plain"}}},
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
		{name: "approval_request", step: normalizer.FlowStep{Action: "approval.Request", Args: map[string]any{"approvalKey": `"refund:123"`, "title": `"Refund approval"`, "requestedBy": "req.UserID", "approvers": []string{"manager@acme.io"}, "policy": `"any"`, "payload": "req", "approvalId": "approvalID", "status": "approvalStatus"}}},
		{name: "approval_wait", step: normalizer.FlowStep{Action: "approval.Wait", Args: map[string]any{"approvalId": "approvalID", "timeout": "10*time.Minute", "decision": "approvalDecision", "status": "approvalStatus"}}},
		{name: "approval_decide", step: normalizer.FlowStep{Action: "approval.Decide", Args: map[string]any{"approvalId": "approvalID", "decision": `"approved"`, "actor": "req.UserID", "status": "approvalStatus"}}},
		{name: "policy_evaluate", step: normalizer.FlowStep{Action: "policy.Evaluate", Args: map[string]any{"policyKey": `"project.create"`, "subject": "req.UserID", "operation": `"create"`, "decision": "policyDecision", "reason": "policyReason", "effects": "policyEffects", "output": "policyResult"}}},
		{name: "policy_require", step: normalizer.FlowStep{Action: "policy.Require", Args: map[string]any{"policyKey": `"project.create"`, "subject": "req.UserID", "operation": `"create"`, "throw": `"forbidden"`}}},
		{name: "policy_decide", step: normalizer.FlowStep{Action: "policy.Decide", Args: map[string]any{"policyKey": `"project.create"`, "subject": "req.UserID", "operation": `"create"`, "output": "decisionObj"}}},
		{name: "idempotency_derive", step: normalizer.FlowStep{Action: "idempotency.DeriveKey", Args: map[string]any{"from": []string{"req.UserID", "req.OrderID"}, "output": "idemKey"}}},
		{name: "idempotency_check", step: normalizer.FlowStep{Action: "idempotency.Check", Args: map[string]any{"key": "idemKey"}}},
		{name: "idempotency_save", step: normalizer.FlowStep{Action: "idempotency.SaveResult", Args: map[string]any{"key": "idemKey", "ttl": "24*time.Hour"}}},
		{name: "ratelimit_limit", step: normalizer.FlowStep{Action: "ratelimit.Limit", Args: map[string]any{"key": "req.UserID", "rps": 20}}},
		{name: "concurrency_run", step: normalizer.FlowStep{Action: "concurrency.Run", Args: map[string]any{"key": `"build"`, "max": 8, "_do": []normalizer.FlowStep{{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}}}}}},
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

			got := renderFlowStepInfra(stNew, tc.step, 1, "_x", arg, child)
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
