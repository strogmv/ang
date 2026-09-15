package emitter

import (
	"testing"

	"github.com/strogmv/ang-ir/ir"
	"github.com/strogmv/ang-ir/normalizer"
)

// An impl code block that uses s.txManager needs the field as much as a flow
// step does; without it the service does not compile.
func TestImplCodeUsingTxManagerInjectsIt(t *testing.T) {
	code := "return resp, s.txManager.WithTx(ctx, func(ctx context.Context) error { return nil })"
	svc := normalizer.Service{Name: "Stock", Methods: []normalizer.Method{{Name: "Move", Impl: &normalizer.MethodImpl{Lang: "go", Code: code}}}}
	if !serviceImplNeedsTx(svc) {
		t.Fatal("serviceImplNeedsTx must see s.txManager in impl code")
	}
	irSvc := ir.Service{Name: "Stock", Methods: []ir.Method{{Name: "Move", Impl: &ir.Impl{Lang: "go", Code: code}}}}
	needsTx, ok := New(t.TempDir(), "", "templates").getAppFuncMap()["ServiceNeedsTxIR"].(func(ir.Service) bool)
	if !ok {
		t.Fatal("ServiceNeedsTxIR is not registered")
	}
	if !needsTx(irSvc) {
		t.Fatal("ServiceNeedsTxIR must see s.txManager in impl code")
	}
	plain := normalizer.Service{Name: "Stock", Methods: []normalizer.Method{{Name: "Get", Impl: &normalizer.MethodImpl{Lang: "go", Code: "return resp, nil"}}}}
	if serviceImplNeedsTx(plain) {
		t.Fatal("impl code without s.txManager must not inject it")
	}
}
