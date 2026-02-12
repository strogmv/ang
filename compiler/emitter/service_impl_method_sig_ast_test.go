package emitter

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
)

func TestRenderServiceImplMethodSignature(t *testing.T) {
	t.Parallel()

	sig, err := renderServiceImplMethodSignature("Orders", normalizer.Method{
		Name:   "CreateOrder",
		Input:  normalizer.Entity{Name: "CreateOrderRequest"},
		Output: normalizer.Entity{Name: "CreateOrderResponse"},
	})
	if err != nil {
		t.Fatalf("renderServiceImplMethodSignature failed: %v", err)
	}
	if !strings.Contains(sig, "func (s *OrdersImpl) CreateOrder(") {
		t.Fatalf("unexpected signature: %s", sig)
	}
	if !strings.Contains(sig, "(resp port.CreateOrderResponse, err error)") {
		t.Fatalf("expected named response returns: %s", sig)
	}

	fileSrc := "package service\nimport (\n\"context\"\n\"github.com/acme/project/internal/domain\"\n\"github.com/acme/project/internal/port\"\n)\n" + sig + " { return resp, err }"
	if _, err := parser.ParseFile(token.NewFileSet(), "orders_method.go", fileSrc, parser.AllErrors); err != nil {
		t.Fatalf("signature should be valid in function declaration: %v\n%s", err, fileSrc)
	}
}
