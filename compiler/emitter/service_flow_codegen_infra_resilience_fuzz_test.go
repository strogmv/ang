package emitter

import (
	"fmt"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
)

// FuzzRenderFlow_InfraResilience_NoPanic ensures infra/resilience actions
// don't panic under arbitrary input payloads.
func FuzzRenderFlow_InfraResilience_NoPanic(f *testing.F) {
	f.Add("https://api.test", "events.core", "cache:key", "it")
	f.Add("bad expr(", "events:bad", "cache", "_")

	f.Fuzz(func(t *testing.T, urlRaw, subjectRaw, keyRaw, aliasRaw string) {
		urlExpr := trimInfra(urlRaw, 160)
		subjectExpr := trimInfra(subjectRaw, 80)
		keyExpr := trimInfra(keyRaw, 80)
		alias := fallbackInfra(safeIdentInfra(aliasRaw), "item")
		if strings.TrimSpace(urlExpr) == "" {
			urlExpr = `"https://api.test"`
		}
		if strings.TrimSpace(subjectExpr) == "" {
			subjectExpr = `"events.core"`
		}
		if strings.TrimSpace(keyExpr) == "" {
			keyExpr = `"cache:key"`
		}

		steps := []normalizer.FlowStep{
			{Action: "flow.Try", Args: map[string]any{
				"retries":   1,
				"backoffMs": 10,
				"_do": []normalizer.FlowStep{
					{Action: "http.Call", Args: map[string]any{"method": "GET", "url": urlExpr, "output": "body"}},
					{Action: "cache.Get", Args: map[string]any{"key": keyExpr, "output": "cacheVal", "optional": true}},
				},
				"_catch": []normalizer.FlowStep{
					{Action: "flow.ExplainError", Args: map[string]any{"output": "tryErr"}},
				},
			}},
			{Action: "parallel.Run", Args: map[string]any{
				"maxConcurrency": 2,
				"_branches": map[string][]normalizer.FlowStep{
					"a": {
						{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}},
					},
					alias: {
						{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}},
					},
				},
			}},
			{Action: "queue.Enqueue", Args: map[string]any{"subject": subjectExpr, "payload": "req", "timeoutMs": 1200}},
			{Action: "queue.Dequeue", Args: map[string]any{"subject": subjectExpr, "output": "msg", "ackToken": "msgID", "timeoutMs": 1200}},
			{Action: "queue.Ack", Args: map[string]any{"subject": subjectExpr, "messageID": "msgID"}},
			{Action: "queue.Nack", Args: map[string]any{"subject": subjectExpr, "messageID": "msgID", "reason": `"decode failed"`}},
			{Action: "dlq.Publish", Args: map[string]any{"subject": subjectExpr, "payload": "msg", "reason": `"decode failed"`}},
			{Action: "webhook.VerifySignature", Args: map[string]any{"payload": "req", "signature": `"sha256=deadbeef"`, "output": "sigOK", "strict": false}},
			{Action: "event.Outbox", Args: map[string]any{"name": `"BuildCompleted"`, "payload": "req"}},
			{Action: "flow.Timeout", Args: map[string]any{
				"duration": "2*time.Second",
				"_do": []normalizer.FlowStep{
					{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}},
				},
			}},
		}

		_ = renderFlow(steps)
	})
}

// FuzzRenderFlow_InfraResilience_SyntaxForSafeInputs validates syntax for
// generated code when safe expressions are provided.
func FuzzRenderFlow_InfraResilience_SyntaxForSafeInputs(f *testing.F) {
	f.Add("alpha", "events.core", "k1")
	f.Add("beta", "events.audit", "k2")

	f.Fuzz(func(t *testing.T, urlSeed, subjectSeed, keySeed string) {
		urlExpr := fmt.Sprintf("%q", safeURLInfra(urlSeed))
		subjectExpr := fmt.Sprintf("%q", fallbackInfra(trimInfra(subjectSeed, 48), "events.core"))
		keyExpr := fmt.Sprintf("%q", fallbackInfra(trimInfra(keySeed, 48), "cache:key"))
		maxConc := (hashInfra(urlSeed)%4 + 1)

		steps := []normalizer.FlowStep{
			{Action: "http.Call", Args: map[string]any{
				"method":      "GET",
				"url":         urlExpr,
				"output":      "respBody",
				"attempts":    2,
				"backoffMs":   5,
				"timeoutMs":   5000,
				"failOnError": true,
			}},
			{Action: "cache.Get", Args: map[string]any{"key": keyExpr, "output": "cached", "optional": true}},
			{Action: "parallel.Run", Args: map[string]any{
				"maxConcurrency": maxConc,
				"_branches": map[string][]normalizer.FlowStep{
					"left": {
						{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}},
					},
					"right": {
						{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}},
					},
				},
			}},
			{Action: "queue.Enqueue", Args: map[string]any{"subject": subjectExpr, "payload": "req", "timeoutMs": 1000}},
			{Action: "queue.Dequeue", Args: map[string]any{"subject": subjectExpr, "output": "msg", "ackToken": "msgID", "timeoutMs": 1000}},
			{Action: "queue.Ack", Args: map[string]any{"subject": subjectExpr, "messageID": "msgID"}},
			{Action: "queue.Nack", Args: map[string]any{"subject": subjectExpr, "messageID": "msgID", "reason": `"decode failed"`}},
			{Action: "dlq.Publish", Args: map[string]any{"subject": subjectExpr, "payload": "msg", "reason": `"decode failed"`}},
			{Action: "webhook.VerifySignature", Args: map[string]any{"payload": "req", "signature": `"sha256=deadbeef"`, "output": "sigOK", "strict": false}},
			{Action: "event.Outbox", Args: map[string]any{"name": `"BuildCompleted"`, "payload": "req"}},
			{Action: "flow.Retry", Args: map[string]any{
				"attempts":  2,
				"backoffMs": 5,
				"_do": []normalizer.FlowStep{
					{Action: "logic.Check", Args: map[string]any{"condition": "true", "throw": "ok"}},
				},
			}},
		}

		code := renderFlow(steps)
		if _, err := parseFlowStmtList(code); err != nil {
			t.Fatalf("generated infra/resilience code must be syntactically valid: %v\n\n%s", err, code)
		}
	})
}

func safeURLInfra(seed string) string {
	switch hashInfra(seed) % 4 {
	case 0:
		return "https://api.test/v1/resource"
	case 1:
		return "https://service.local/health"
	case 2:
		return "https://example.org/events"
	default:
		return "https://api.test/default"
	}
}

func safeIdentInfra(v string) string {
	v = trimInfra(v, 64)
	var b strings.Builder
	for i, r := range v {
		isLetter := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		isDigit := r >= '0' && r <= '9'
		if i == 0 {
			if isLetter || r == '_' {
				b.WriteRune(r)
			}
			continue
		}
		if isLetter || isDigit || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func trimInfra(s string, max int) string {
	if len(s) > max {
		return s[:max]
	}
	return s
}

func fallbackInfra(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

func hashInfra(s string) int {
	h := 19
	for _, r := range s {
		h = h*33 + int(r)
	}
	if h < 0 {
		h = -h
	}
	return h
}
