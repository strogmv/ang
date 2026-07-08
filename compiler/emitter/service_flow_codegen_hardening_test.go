package emitter

import (
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

func TestInferFlowCapturedVarType_OpenAIChatWithToolsReturnsStructuredReply(t *testing.T) {
	t.Parallel()

	got := inferFlowCapturedVarType("reply", []normalizer.FlowStep{
		{
			Action: "openai.Chat",
			Args: map[string]any{
				"output": "reply",
				"tools":  []string{"LookupPostForAssistant"},
			},
		},
	})
	want := "struct{ Content string; FinishReason string; ToolCalls int; PromptTokens int; CompletionTokens int; TotalTokens int }"
	if got != want {
		t.Fatalf("expected structured captured type %q, got %q", want, got)
	}
}

func TestRenderFlowStepState_StateGetUsesIntoType(t *testing.T) {
	t.Parallel()

	step := normalizer.FlowStep{
		Action: "state.Get",
		Args: map[string]any{
			"key":     `"draft:1"`,
			"output":  "result",
			"into":    "map[string]any",
			"default": "map[string]any{}",
		},
	}

	got := renderOneFlowStep(newInfraTestFlowState(), step, 1)
	if !strings.Contains(got, "var result map[string]any") {
		t.Fatalf("expected typed declaration, got:\n%s", got)
	}
	if strings.Contains(got, "var result any") {
		t.Fatalf("unexpected any declaration, got:\n%s", got)
	}
}

func TestRenderFlowStepState_StateGetDoesNotRedeclareExistingOutput(t *testing.T) {
	t.Parallel()

	step := normalizer.FlowStep{
		Action: "state.Get",
		Args: map[string]any{
			"key":    `"draft:1"`,
			"output": "result",
		},
	}
	st := newInfraTestFlowState()
	st.declared["result"] = true
	st.types["result"] = "map[string]any"

	got := renderOneFlowStep(st, step, 1)
	if strings.Contains(got, "var result ") {
		t.Fatalf("unexpected redeclaration, got:\n%s", got)
	}
}

func TestTypedServiceCallMissingMethodReturnsError(t *testing.T) {
	t.Parallel()

	step := normalizer.FlowStep{
		Action: "service.Call",
		Args: map[string]any{
			"service": "Blog",
		},
	}

	got := renderOneFlowStep(newInfraTestFlowState(), step, 1)
	if !strings.Contains(got, `service.Call: method is required`) {
		t.Fatalf("expected deterministic invalid-config error, got:\n%s", got)
	}
}

func TestRenderFlowStepEventOrchestration_NotifyEmailMissingToReturnsError(t *testing.T) {
	t.Parallel()

	step := normalizer.FlowStep{
		Action: "notify.Email",
		Args: map[string]any{
			"text": `"Hello"`,
		},
	}

	got, ok := renderFlowStepEventOrchestration(newInfraTestFlowState(), step, 1, "_x", infraTestArg(step), infraTestChild(step))
	if !ok {
		t.Fatal("expected notify.Email to be handled")
	}
	if !strings.Contains(got, `notify.Email: notify.Email requires to`) {
		t.Fatalf("expected deterministic invalid-config error, got:\n%s", got)
	}
}

func TestRenderFlow_EventPublishPropagatesPublisherError(t *testing.T) {
	t.Parallel()

	code := renderFlow([]normalizer.FlowStep{
		{
			Action: "event.Publish",
			Args: map[string]any{
				"name": "TenderClosed",
			},
		},
	})

	mustContain := []string{
		`if s.publisher == nil {`,
		`event.Publish: publisher wiring is not configured`,
		`if err := s.publisher.PublishTenderClosed(ctx, domain.TenderClosed{}); err != nil {`,
		`fmt.Errorf("event.Publish TenderClosed: %w", err)`,
	}
	for _, needle := range mustContain {
		if !strings.Contains(code, needle) {
			t.Fatalf("expected event.Publish hardening snippet %q, got:\n%s", needle, code)
		}
	}
	if strings.Contains(code, `_ = s.publisher.PublishTenderClosed`) {
		t.Fatalf("unexpected swallowed publish error, got:\n%s", code)
	}
}

func TestRenderFlow_NoCodegenStepReturnsDeterministicRuntimeError(t *testing.T) {
	t.Parallel()

	code := renderFlow([]normalizer.FlowStep{
		{
			Action: "cache.Set",
			Args: map[string]any{
				"ttl": "60",
			},
		},
	})

	for _, needle := range []string{
		`// WARNING: step 1 (cache.Set) produced no code; check required fields`,
		`cache.Set: step produced no code; check required fields`,
	} {
		if !strings.Contains(code, needle) {
			t.Fatalf("expected no-codegen runtime guard %q, got:\n%s", needle, code)
		}
	}
}

func TestRenderFlow_CacheDelPropagatesDeleteError(t *testing.T) {
	t.Parallel()

	got := renderOneFlowStep(newInfraTestFlowState(), normalizer.FlowStep{
		Action: "cache.Del",
		Args: map[string]any{
			"key": `"k"`,
		},
	}, 1)

	if !strings.Contains(got, `if _cErr := s.cache.Del(ctx, "k").Err(); _cErr != nil {`) {
		t.Fatalf("expected cache.Del to propagate delete error, got:\n%s", got)
	}
}

func TestRenderFlow_AuditLogPropagatesRepositoryErrors(t *testing.T) {
	t.Parallel()

	got := renderFlow([]normalizer.FlowStep{
		{
			Action: "audit.Log",
			Args: map[string]any{
				"actor":   "req.UserID",
				"company": "req.CompanyID",
				"event":   `"user.logged_in"`,
			},
		},
	})

	for _, needle := range []string{
		`if s.AuditLogRepo == nil {`,
		`audit.Log: audit repository wiring is not configured`,
		`if _auditErr := s.AuditLogRepo.Save(ctx, _auditRec); _auditErr != nil {`,
		`fmt.Errorf("audit.Log: %w", _auditErr)`,
	} {
		if !strings.Contains(got, needle) {
			t.Fatalf("expected audit.Log hardening snippet %q, got:\n%s", needle, got)
		}
	}
}

func TestRenderFlow_NotifyDispatchPropagatesDispatcherErrors(t *testing.T) {
	t.Parallel()

	got := renderFlow([]normalizer.FlowStep{
		{
			Action: "notify.Dispatch",
			Args: map[string]any{
				"event":   `"invite.sent"`,
				"userID":  "req.UserID",
				"payload": "req",
			},
		},
	})

	for _, needle := range []string{
		`if s.dispatcher == nil {`,
		`notify.Dispatch: notification dispatcher wiring is not configured`,
		`if _dispatchErr := s.dispatcher.Dispatch(ctx, port.NotificationMessage{Event: strings.TrimSpace(fmt.Sprint("invite.sent"))`,
		`fmt.Errorf("notify.Dispatch: %w", _dispatchErr)`,
	} {
		if !strings.Contains(got, needle) {
			t.Fatalf("expected notify.Dispatch hardening snippet %q, got:\n%s", needle, got)
		}
	}
}

func TestRenderFlowResumeLegacy_UsesIntoTypeAssertion(t *testing.T) {
	t.Parallel()

	step := normalizer.FlowStep{
		Action: "flow.Resume",
		Args: map[string]any{
			"name":   `"draft"`,
			"output": "draft",
			"into":   "map[string]any",
		},
	}

	got := renderOneFlowStep(newInfraTestFlowState(), step, 1)
	for _, needle := range []string{
		`var draft map[string]any`,
		`_ckptCast`,
		`_ckptVal`,
		`.(map[string]any)`,
		`checkpoint "draft" not found`,
	} {
		if !strings.Contains(got, needle) {
			t.Fatalf("expected typed flow.Resume snippet %q, got:\n%s", needle, got)
		}
	}
}

func TestRenderFlowStepEventOrchestration_EventWaitUsesIntoTypeAssertion(t *testing.T) {
	t.Parallel()

	step := normalizer.FlowStep{
		Action: "event.Wait",
		Args: map[string]any{
			"name":   `"OrderCreated"`,
			"output": "evt",
			"into":   "map[string]any",
		},
	}

	got, ok := renderFlowStepEventOrchestration(newInfraTestFlowState(), step, 1, "_x", infraTestArg(step), infraTestChild(step))
	if !ok {
		t.Fatal("expected event.Wait to be handled")
	}
	for _, needle := range []string{
		`var evt map[string]any`,
		`_waitName := strings.TrimSpace(fmt.Sprint("OrderCreated"))`,
		`event.Wait: publisher wiring is not configured`,
		`_waitCast, _waitCastOK := _evt.(map[string]any)`,
		`fmt.Errorf("event.Wait(%s): payload is not map[string]any", _waitName)`,
	} {
		if !strings.Contains(got, needle) {
			t.Fatalf("expected typed event.Wait snippet %q, got:\n%s", needle, got)
		}
	}
}

func TestRenderFlow_FlowCallIgnoreErrWithReasonEmitsCommentWithoutWarning(t *testing.T) {
	t.Parallel()

	var diags []normalizer.Warning
	got := renderFlowForServiceWithSchemaAndSink("Blog", "PublishPost", []normalizer.FlowStep{
		{
			Action: "flow.Call",
			Args: map[string]any{
				"op":              "Notifications.SendInvitationEmail",
				"ignoreErr":       true,
				"ignoreErrReason": `"best effort side effect"`,
			},
		},
	}, nil, nil, func(w normalizer.Warning) {
		diags = append(diags, w)
	})

	if !strings.Contains(got, `// explicit ignoreErr=true: "best effort side effect"`) {
		t.Fatalf("expected explicit ignoreErr comment, got:\n%s", got)
	}
	if len(diags) != 0 {
		t.Fatalf("expected no FLOW_IGNORE_ERR warning when ignoreErrReason is present, got %#v", diags)
	}
}
