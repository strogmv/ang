package emitter

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler/flowir"
)

func renderOpenAIChatForTest(t *testing.T, args map[string]any) string {
	t.Helper()
	services := []normalizer.Service{{
		Name: "Assistant",
		Methods: []normalizer.Method{{
			Name:        "LookupPost",
			Description: "Lookup one post by slug",
			Input:       normalizer.Entity{Name: "LookupPostRequest", Fields: []normalizer.Field{{Name: "Slug", Type: "string"}}},
			Output:      normalizer.Entity{Name: "LookupPostResponse"},
		}},
	}}
	return renderFlowForServiceWithSchemaAndSinkModeWithInfra("Assistant", "Chat", false,
		[]normalizer.FlowStep{{Action: "openai.Chat", Args: args}}, nil, nil, nil,
		map[string]any{flowInfraKeyServicesCatalog: services})
}

// The budget hook wraps every request: Reserve before the request is sent
// (its error is the step's), Settle after it with the usage — cached and
// reasoning tokens included — and the request's error.
func TestRenderFlow_OpenAIChat_BudgetHook(t *testing.T) {
	t.Parallel()
	got := renderOpenAIChatForTest(t, map[string]any{
		"user_message": "req.Message", "model": `"gpt-5-mini"`, "tools": []string{"LookupPost"},
		"max_rounds": 8, "output": "reply", "budget": "meter", "output_rounds": "rounds",
	})
	for _, needle := range []string{
		`var _oaiBudget_0 helpers.LLMBudget = meter`,
		`_oaiCall_0 := helpers.LLMRound{Provider: "openai", Model: strings.TrimSpace(fmt.Sprint("gpt-5-mini")), Round: _oaiRound_0, MaxOutputTokens: 4096, RequestBytes: len(_oaiReqBody_0)}`,
		`_oaiBudget_0.Reserve(ctx, _oaiCall_0)`,
		`_oaiBudget_0.Settle(ctx, _oaiCall_0, _oaiRoundUsage_0, _oaiCallErr_0)`,
		`CachedInputTokens: _oaiParsed_0.Usage.PromptTokensDetails.CachedTokens`,
		`ReasoningTokens: _oaiParsed_0.Usage.CompletionTokensDetails.ReasoningTokens`,
		`var rounds []helpers.LLMRoundUsage`,
		`rounds = append(rounds, helpers.LLMRoundUsage{Round: _oaiCall_0, Usage: _oaiRoundUsage_0})`,
		`helpers.OpenAIURL("/chat/completions")`,
	} {
		if !strings.Contains(got, needle) {
			t.Fatalf("missing %q in:\n%s", needle, got)
		}
	}
	reserve, send, settle := strings.Index(got, ".Reserve("), strings.Index(got, "http.DefaultClient.Do("), strings.Index(got, ".Settle(")
	if !(reserve < send && send < settle) {
		t.Fatalf("reserve (%d) must come before the request (%d) and settle (%d) after it", reserve, send, settle)
	}
	if strings.Contains(got, "api.openai.com") {
		t.Fatalf("the address is OPENAI_BASE_URL's, not hard-coded:\n%s", got)
	}
}

func TestRenderFlow_OpenAIChat_ContextBeforeUser(t *testing.T) {
	t.Parallel()
	got := renderOpenAIChatForTest(t, map[string]any{
		"user_message": "req.Message", "system": `"You help."`, "system_context": "sysCtx", "history": "req.Messages",
		"system_context_position": "before_user", "prompt_cache_key": `"assistant-v1"`, "output": "reply",
	})
	if strings.Contains(got, `"\n\n== Context ==\n" + sysCtx`) {
		t.Fatalf("the context must not be part of the system prompt:\n%s", got)
	}
	system, history, context, user := strings.Index(got, `_oaiSystem_0 := "You help."`), strings.Index(got, "range req.Messages"), strings.Index(got, "_oaiContext_0 := sysCtx"), strings.Index(got, `"role": "user"`)
	if system < 0 || !(system < history && history < context && context < user) {
		t.Fatalf("order system %d < history %d < context %d < user %d broken:\n%s", system, history, context, user, got)
	}
	if !strings.Contains(got, `["prompt_cache_key"] = _oaiCacheKey_0`) {
		t.Fatalf("prompt_cache_key missing:\n%s", got)
	}

	legacy := renderOpenAIChatForTest(t, map[string]any{"user_message": "req.Message", "system": `"You help."`, "system_context": "sysCtx"})
	if !strings.Contains(legacy, `"You help." + "\n\n== Context ==\n" + sysCtx`) {
		t.Fatalf("the default keeps the context in the system prompt:\n%s", legacy)
	}
}

func TestDecodeOpenAIChat_RejectsUnknownContextPosition(t *testing.T) {
	t.Parallel()
	_, err := flowir.Decode(normalizer.FlowStep{Action: "openai.Chat", Args: map[string]any{"user_message": "req.Message", "system_context_position": "after"}})
	if err == nil || !strings.Contains(err.Error(), "system_context_position") {
		t.Fatalf("want a system_context_position error, got %v", err)
	}
}

// The generated tool loop, compiled and run against a fake provider: every
// round reserves and settles, usage adds up, a refused reservation stops the
// step before the request is sent, and two calls with different contexts
// share a byte-identical prefix (system prompt, tools, history).
func TestOpenAIChat_BudgetHook_Runs(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("runs go test in a temporary module")
	}
	tools := renderOpenAIChatForTest(t, map[string]any{
		"user_message": "req.Message", "model": `"gpt-5-mini"`, "system": `"You help with posts."`, "system_context": "req.Context",
		"system_context_position": "before_user", "history": "req.History", "tools": []string{"LookupPost"}, "max_rounds": 8,
		"max_tokens": 1000, "output": "reply", "output_usage": "usage", "budget": "req.Budget", "output_rounds": "rounds",
	})
	plain := renderOpenAIChatForTest(t, map[string]any{
		"user_message": "req.Message", "model": `"gpt-5-mini"`, "output": "reply", "budget": "req.Budget",
	})
	llm := helpersLLMSourceForTest(t)
	files := map[string]string{
		"go.mod":             "module llmrun\n\ngo 1.22\n",
		"helpers/helpers.go": llm,
		"port/port.go":       "package port\n\ntype LookupPostRequest struct{ Slug string }\ntype LookupPostResponse struct{ Title string }\n",
		"svc/svc.go": `package svc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"llmrun/helpers"
	"llmrun/port"
)

var _ = bytes.NewReader
var _ = json.Marshal
var _ = fmt.Sprint
var _ = io.ReadAll
var _ = http.NewRequest
var _ = os.Getenv
var _ = strings.TrimSpace
var _ = time.Second

type Msg struct{ Role, Content string }

type Req struct {
	Message, Context string
	History          []Msg
	Budget           helpers.LLMBudget
}

type Resp struct {
	Content  string
	Rounds   []helpers.LLMRoundUsage
	Cached   int
	Reasoned int
	Tools    int
}

type Svc struct{ Looked []string }

func (s *Svc) LookupPost(ctx context.Context, req port.LookupPostRequest) (port.LookupPostResponse, error) {
	s.Looked = append(s.Looked, req.Slug)
	return port.LookupPostResponse{Title: "Hello " + req.Slug}, nil
}

func (s *Svc) Chat(ctx context.Context, req Req) (resp Resp, err error) {
` + tools + `
	resp.Content, resp.Rounds, resp.Cached, resp.Reasoned, resp.Tools = reply.Content, rounds, usage.CachedTokens, usage.ReasoningTokens, reply.ToolCalls
	return resp, nil
}

func (s *Svc) Plain(ctx context.Context, req Req) (resp Resp, err error) {
` + plain + `
	resp.Content = reply
	return resp, nil
}
`,
		"svc/svc_test.go": openAIBudgetRunTest,
	}
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "test", "-count=1", "-v", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated openai.Chat: %v\n%s", err, out)
	}
	for _, name := range []string{"TestRounds", "TestRefusedReservationSendsNothing", "TestFailedCallIsSettled", "TestPrefixIsStable"} {
		if !strings.Contains(string(out), "--- PASS: "+name) {
			t.Fatalf("%s did not run:\n%s", name, out)
		}
	}
}

// helpersLLMSourceForTest is the model-call part of templates/helpers.tmpl,
// as its own package (the rest of the template needs the validator module).
func helpersLLMSourceForTest(t *testing.T) string {
	t.Helper()
	raw, err := ReadTemplateByPath("templates/helpers.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	i := strings.Index(src, "// ── Model calls (openai.Chat)")
	if i < 0 {
		t.Fatal("helpers.tmpl has no model-call section")
	}
	return "package helpers\n\nimport (\n\t\"context\"\n\t\"os\"\n\t\"strings\"\n)\n\n" + src[i:]
}

const openAIBudgetRunTest = `package svc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"llmrun/helpers"
)

type meter struct {
	mu       sync.Mutex
	reserved []helpers.LLMRound
	settled  []helpers.LLMUsage
	errs     []error
	refuseAt int
}

var errEmpty = errors.New("AI_BALANCE_EMPTY")

func (m *meter) Reserve(ctx context.Context, r helpers.LLMRound) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.refuseAt >= 0 && r.Round == m.refuseAt {
		return errEmpty
	}
	m.reserved = append(m.reserved, r)
	return nil
}

func (m *meter) Settle(ctx context.Context, r helpers.LLMRound, u helpers.LLMUsage, callErr error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.settled = append(m.settled, u)
	m.errs = append(m.errs, callErr)
	return nil
}

type fake struct {
	mu     sync.Mutex
	bodies []map[string]any
	fail   bool
}

func (f *fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	f.mu.Lock()
	f.bodies = append(f.bodies, body)
	f.mu.Unlock()
	if f.fail {
		w.Write([]byte(` + "`" + `{"error":{"message":"quota"}}` + "`" + `))
		return
	}
	msgs, _ := body["messages"].([]any)
	last, _ := msgs[len(msgs)-1].(map[string]any)
	usage := ` + "`" + `"usage":{"prompt_tokens":1200,"completion_tokens":50,"total_tokens":1250,"prompt_tokens_details":{"cached_tokens":1024},"completion_tokens_details":{"reasoning_tokens":30}}` + "`" + `
	if body["tools"] != nil && last["role"] == "user" {
		w.Write([]byte(` + "`" + `{"choices":[{"finish_reason":"tool_calls","message":{"content":"","tool_calls":[{"id":"c1","type":"function","function":{"name":"LookupPost","arguments":"{\"Slug\":\"go\"}"}}]}}],` + "`" + ` + usage + "}"))
		return
	}
	w.Write([]byte(` + "`" + `{"choices":[{"finish_reason":"stop","message":{"content":"done"}}],` + "`" + ` + usage + "}"))
}

func start(t *testing.T, f *fake) {
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	os.Setenv("OPENAI_BASE_URL", srv.URL+"/v1")
}

func TestRounds(t *testing.T) {
	f := &fake{}
	start(t, f)
	m := &meter{refuseAt: -1}
	s := &Svc{}
	resp, err := s.Chat(context.Background(), Req{Message: "find go", Context: "now: 10:00", Budget: m})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "done" || len(s.Looked) != 1 || resp.Tools != 1 {
		t.Fatalf("resp %+v looked %v", resp, s.Looked)
	}
	if len(m.reserved) != 2 || len(m.settled) != 2 || m.reserved[0].Round != 0 || m.reserved[1].Round != 1 {
		t.Fatalf("every round reserves and settles: %+v %+v", m.reserved, m.settled)
	}
	for _, r := range m.reserved {
		if r.Model != "gpt-5-mini" || r.MaxOutputTokens != 1000 || r.RequestBytes <= 0 || r.Provider != "openai" {
			t.Fatalf("round %+v", r)
		}
	}
	if m.settled[1].CachedInputTokens != 1024 || m.settled[1].ReasoningTokens != 30 || m.settled[1].InputTokens != 1200 || m.settled[1].OutputTokens != 50 || m.errs[1] != nil {
		t.Fatalf("settled %+v %v", m.settled, m.errs)
	}
	if len(resp.Rounds) != 2 || resp.Cached != 2048 || resp.Reasoned != 60 {
		t.Fatalf("usage per round %+v cached %d reasoned %d", resp.Rounds, resp.Cached, resp.Reasoned)
	}
}

func TestRefusedReservationSendsNothing(t *testing.T) {
	f := &fake{}
	start(t, f)
	m := &meter{refuseAt: 1}
	_, err := (&Svc{}).Chat(context.Background(), Req{Message: "find go", Budget: m})
	if !errors.Is(err, errEmpty) {
		t.Fatalf("want the reservation's error, got %v", err)
	}
	if len(f.bodies) != 1 || len(m.settled) != 1 {
		t.Fatalf("round 1 must not be sent: %d requests, %d settled", len(f.bodies), len(m.settled))
	}

	f2 := &fake{}
	start(t, f2)
	_, err = (&Svc{}).Plain(context.Background(), Req{Message: "hi", Budget: &meter{refuseAt: 0}})
	if !errors.Is(err, errEmpty) || len(f2.bodies) != 0 {
		t.Fatalf("plain: err %v, %d requests", err, len(f2.bodies))
	}
}

func TestFailedCallIsSettled(t *testing.T) {
	f := &fake{fail: true}
	start(t, f)
	m := &meter{refuseAt: -1}
	_, err := (&Svc{}).Plain(context.Background(), Req{Message: "hi", Budget: m})
	if err == nil || !strings.Contains(err.Error(), "openai.Chat: API error: quota") {
		t.Fatalf("err %v", err)
	}
	if len(m.settled) != 1 || m.errs[0] == nil {
		t.Fatalf("the failed request is settled with its error: %+v %v", m.settled, m.errs)
	}
}

func TestPrefixIsStable(t *testing.T) {
	f := &fake{}
	start(t, f)
	hist := []Msg{{Role: "user", Content: "earlier"}, {Role: "assistant", Content: "answer"}}
	for _, c := range []string{"now: 10:00:01, chat a", "now: 10:00:02, chat b"} {
		if _, err := (&Svc{}).Chat(context.Background(), Req{Message: "find go", Context: c, History: hist, Budget: &meter{refuseAt: -1}}); err != nil {
			t.Fatal(err)
		}
	}
	first, second := f.bodies[0], f.bodies[2]
	prefix := func(b map[string]any) string {
		msgs := b["messages"].([]any)
		raw, _ := json.Marshal(map[string]any{"tools": b["tools"], "head": msgs[:3]})
		return string(raw)
	}
	if prefix(first) != prefix(second) {
		t.Fatalf("system prompt, tools and history differ between calls:\n%s\n%s", prefix(first), prefix(second))
	}
	msgs := first["messages"].([]any)
	ctxMsg := msgs[3].(map[string]any)
	if ctxMsg["role"] != "system" || !strings.Contains(ctxMsg["content"].(string), "chat a") || msgs[4].(map[string]any)["role"] != "user" {
		t.Fatalf("the context goes right before the user's message: %v", msgs)
	}
	if strings.Contains(msgs[0].(map[string]any)["content"].(string), "now:") {
		t.Fatalf("the system prompt carries no context: %v", msgs[0])
	}
}
`
