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
		{name: "parallel_run", step: normalizer.FlowStep{Action: "parallel.Run", Args: map[string]any{"_branches": map[string][]normalizer.FlowStep{"a": []normalizer.FlowStep{}, "b": []normalizer.FlowStep{}}}}},
		{name: "pdf_render", step: normalizer.FlowStep{Action: "pdf.Render", Args: map[string]any{"template": `"t"`, "data": "req.ReportData", "output": "pdfBytes"}}},
		{name: "webhook_send", step: normalizer.FlowStep{Action: "webhook.Send", Args: map[string]any{"url": `"https://hook.example"`, "payload": "req.Payload", "event": `"evt"`}}},
		{name: "queue_enqueue", step: normalizer.FlowStep{Action: "queue.Enqueue", Args: map[string]any{"subject": `"events.test"`, "payload": "req.Payload", "timeout": "2*time.Second"}}},
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
