package emitter

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/strogmv/ang/compiler/normalizer"
)

func TestRenderServiceInterfaceDecl(t *testing.T) {
	t.Parallel()

	svc := normalizer.Service{
		Name:        "Orders",
		Description: "handles order operations",
		Methods: []normalizer.Method{
			{
				Name:        "CreateOrder",
				Description: "creates a new order",
				Input:       normalizer.Entity{Name: "CreateOrderRequest"},
				Output:      normalizer.Entity{Name: "CreateOrderResponse"},
			},
			{
				Name:      "OrderPaid",
				Publishes: []string{"OrderPaid"},
			},
		},
	}

	src, err := renderServiceInterfaceDecl(svc)
	if err != nil {
		t.Fatalf("renderServiceInterfaceDecl failed: %v", err)
	}
	if !strings.Contains(src, "type Orders interface") {
		t.Fatalf("expected interface declaration, got:\n%s", src)
	}
	if !strings.Contains(src, "CreateOrder(ctx context.Context, req CreateOrderRequest) (CreateOrderResponse, error)") {
		t.Fatalf("expected method signature in source, got:\n%s", src)
	}
	if !strings.Contains(src, "OrderPaid(ctx context.Context, req domain.OrderPaid) error") {
		t.Fatalf("expected event method signature in source, got:\n%s", src)
	}

	fileSrc := "package port\nimport (\n\"context\"\n\"github.com/acme/project/internal/domain\"\n)\n" + src
	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "orders.go", fileSrc, parser.AllErrors); err != nil {
		t.Fatalf("service interface snippet should be valid Go in file context: %v\n%s", err, fileSrc)
	}
}
