package emitter

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

func TestRenderServiceImplASTSkeleton(t *testing.T) {
	t.Parallel()

	svc := normalizer.Service{
		Name: "Orders",
		Methods: []normalizer.Method{
			{
				Name: "CreateOrder",
				Sources: []normalizer.Source{
					{Entity: "Order"},
				},
				Idempotency: true,
				Flow: []normalizer.FlowStep{
					{Action: "event.Outbox", Args: map[string]any{"name": `"OrderCreated"`, "payload": "req"}},
					{Action: "queue.Dequeue", Args: map[string]any{"subject": `"orders"`, "output": "msg"}},
					{Action: "notify.Email", Args: map[string]any{"to": "req.Email", "text": `"hi"`}},
					{Action: "approval.Wait", Args: map[string]any{"approvalId": "req.ApprovalID"}},
					{Action: "policy.Require", Args: map[string]any{"policyKey": `"order.create"`, "subject": "req.UserID"}},
				},
			},
		},
		Uses: []string{"Audit"},
	}
	entities := []normalizer.Entity{{Name: "Order"}}
	auth := &normalizer.AuthDef{Service: "Orders"}

	typeDecl, err := renderServiceImplTypeDecl(svc, entities, auth)
	if err != nil {
		t.Fatalf("renderServiceImplTypeDecl failed: %v", err)
	}
	ctorDecl, err := renderServiceImplConstructorDecl(svc, entities, auth)
	if err != nil {
		t.Fatalf("renderServiceImplConstructorDecl failed: %v", err)
	}
	src := "package service\nimport (\n\"github.com/acme/project/internal/config\"\n\"github.com/acme/project/internal/port\"\n)\n" + typeDecl + "\n" + ctorDecl

	if !strings.Contains(src, "type OrdersImpl struct") {
		t.Fatalf("expected OrdersImpl struct, got:\n%s", src)
	}
	if !strings.Contains(src, "func NewOrdersImpl(") {
		t.Fatalf("expected constructor, got:\n%s", src)
	}
	if !strings.Contains(src, "idempotency port.IdempotencyStore") {
		t.Fatalf("expected idempotency dependency in constructor, got:\n%s", src)
	}
	if !strings.Contains(src, "outbox port.OutboxRepository") {
		t.Fatalf("expected outbox dependency in constructor, got:\n%s", src)
	}
	if !strings.Contains(src, "queuePublisher port.QueuePublisher") {
		t.Fatalf("expected queuePublisher dependency in constructor, got:\n%s", src)
	}
	if !strings.Contains(src, "dispatcher port.NotificationDispatcher") {
		t.Fatalf("expected notification dispatcher dependency in constructor, got:\n%s", src)
	}
	if !strings.Contains(src, "stateStore port.StateStore") {
		t.Fatalf("expected stateStore dependency in constructor, got:\n%s", src)
	}
	if !strings.Contains(src, "policyEngine port.PolicyEngine") {
		t.Fatalf("expected policyEngine dependency in constructor, got:\n%s", src)
	}

	if _, err := parser.ParseFile(token.NewFileSet(), "orders_impl.go", src, parser.AllErrors); err != nil {
		t.Fatalf("generated skeleton should be valid Go: %v\n%s", err, src)
	}
}

func TestServiceImplNeedsTxForInlineLogicCallThatOwnsTransaction(t *testing.T) {
	t.Parallel()

	svc := normalizer.Service{
		Name: "Orders",
		Methods: []normalizer.Method{{
			Name: "Cancel",
			Flow: []normalizer.FlowStep{{
				Action: "flow.Block",
				Args: map[string]any{"_do": []normalizer.FlowStep{{
					Action: "logic.Call",
					Args:   map[string]any{"func": "(func() error { return s.txManager.WithTx(ctx, nil) })"},
				}}},
			}},
		}},
	}

	if !serviceImplNeedsTx(svc) {
		t.Fatal("expected raw logic.Call that uses s.txManager to inject TxManager")
	}
}
