package lsp

import (
	"strings"
	"testing"
)

func TestCompletionItemsMarkOpenAIUnavailableWithoutPrereqs(t *testing.T) {
	items := CompletionItems("session.Get(output: session)\n", Position{Line: 1, Character: 0})
	found := false
	for _, item := range items {
		if item.Label == "openai.Chat" {
			found = true
			if !item.Deprecated {
				t.Fatalf("expected openai.Chat to be marked unavailable without quota/budget prerequisites")
			}
			if !strings.Contains(item.Detail, "renderer=infrastructure") {
				t.Fatalf("expected renderer group in completion detail, got %q", item.Detail)
			}
		}
	}
	if !found {
		t.Fatalf("expected openai.Chat completion item")
	}
}

func TestCompletionItemsAllowOpenAIWithPrereqs(t *testing.T) {
	src := "session.Get(output: session)\nquota.Check(key: session, limit: 10, window: \"day\")\nbudget.Check(key: session, limit: 1000)\n"
	items := CompletionItems(src, Position{Line: 3, Character: 0})
	found := false
	for _, item := range items {
		if item.Label == "openai.Chat" {
			found = true
			if item.Deprecated {
				t.Fatalf("did not expect openai.Chat to be deprecated after prerequisites")
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected openai.Chat after quota and budget checks")
	}
}

func TestCompletionItemsMarkDBLockUnavailableOutsideTx(t *testing.T) {
	items := CompletionItems("", Position{Line: 0, Character: 0})
	found := false
	for _, item := range items {
		if item.Label == "db.Lock" {
			found = true
			if !item.Deprecated {
				t.Fatalf("expected db.Lock to be marked unavailable outside tx.Block")
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected db.Lock completion item")
	}
}

func TestCompletionItemsAllowDBLockInsideTx(t *testing.T) {
	src := "tx.Block {\n"
	items := CompletionItems(src, Position{Line: 1, Character: 0})
	found := false
	for _, item := range items {
		if item.Label == "db.Lock" {
			found = true
			if item.Deprecated {
				t.Fatalf("did not expect db.Lock to be deprecated inside tx.Block")
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected db.Lock inside tx.Block")
	}
}

func TestHoverForSource(t *testing.T) {
	hover, ok := HoverForSource("openai.Chat(user_message: req.message, output: reply)\n", Position{Line: 0, Character: 3})
	if !ok {
		t.Fatalf("expected hover result")
	}
	if hover == nil || hover.Value == "" {
		t.Fatalf("expected non-empty hover")
	}
	if !strings.Contains(hover.Value, "Renderer group: `infrastructure`") {
		t.Fatalf("expected renderer group in hover, got %q", hover.Value)
	}
}

func TestFlowDiagnostics(t *testing.T) {
	diags := FlowDiagnostics("openai.Chat(user_message: req.message, output: reply)\n", false)
	if len(diags) == 0 {
		t.Fatalf("expected diagnostics")
	}
	if diags[0].Code == "" {
		t.Fatalf("expected diagnostic code")
	}
}
