package emitter

import (
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/ir"
	"github.com/strogmv/ang/angir/normalizer"
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

// transactional: true and s.Tx in raw Go both need the TxManager; s.Tx also
// gets the public field, set from the same constructor argument.
func TestTransactionalAndTxFieldInjectTxManager(t *testing.T) {
	tx := normalizer.Service{Name: "Stock", Methods: []normalizer.Method{{Name: "Move", Transactional: true, Flow: []normalizer.FlowStep{{Action: "mapping.Assign"}}}}}
	if !serviceImplNeedsTx(tx) || serviceImplUsesTx(tx) {
		t.Fatalf("transactional: needsTx=%v usesTx=%v", serviceImplNeedsTx(tx), serviceImplUsesTx(tx))
	}
	code := "txErr := s.Tx.WithTx(ctx, func(ctx context.Context) error { return nil })\nreturn resp, txErr"
	public := normalizer.Service{Name: "Stock", Methods: []normalizer.Method{{Name: "Move", Impl: &normalizer.MethodImpl{Lang: "go", Code: code}}}}
	if !serviceImplNeedsTx(public) || !serviceImplUsesTx(public) {
		t.Fatalf("s.Tx: needsTx=%v usesTx=%v", serviceImplNeedsTx(public), serviceImplUsesTx(public))
	}
	decl, err := renderServiceImplTypeDecl(public, nil, nil)
	if err != nil || !strings.Contains(decl, "Tx        port.TxManager") {
		t.Fatalf("type declaration lacks Tx (err %v):\n%s", err, decl)
	}
	ctor, err := renderServiceImplConstructorDecl(public, nil, nil)
	if err != nil || !strings.Contains(ctor, "txManager: txManager, Tx: txManager}") || strings.Count(ctor, "port.TxManager") != 1 {
		t.Fatalf("constructor must take one TxManager and set both fields (err %v):\n%s", err, ctor)
	}
	needsTx := New(t.TempDir(), "", "templates").getAppFuncMap()["ServiceNeedsTxIR"].(func(ir.Service) bool)
	if !needsTx(ir.Service{Name: "Stock", Methods: []ir.Method{{Name: "Move", Transactional: true}}}) {
		t.Fatal("the runtime container must pass a TxManager to a transactional operation's service")
	}
	for _, other := range []string{"s.TxManager.Do()", "s.Txn.WithTx(ctx, nil)"} {
		svc := normalizer.Service{Name: "Stock", Methods: []normalizer.Method{{Name: "Move", Impl: &normalizer.MethodImpl{Lang: "go", Code: other}}}}
		if serviceImplUsesTx(svc) {
			t.Errorf("%q is not s.Tx", other)
		}
	}
}

func TestWrapTransactionalOperations(t *testing.T) {
	flow := []normalizer.FlowStep{{Action: "repo.Find", File: "cue/api/x.cue", Line: 7}, {Action: "repo.Save"}}
	services := []normalizer.Service{{Name: "Stock", Methods: []normalizer.Method{
		{Name: "Move", Transactional: true, Flow: flow},
		{Name: "Keep", Flow: flow},
	}}}
	if err := wrapTransactionalOperations(services); err != nil {
		t.Fatal(err)
	}
	moved := services[0].Methods[0].Flow
	if len(moved) != 1 || moved[0].Action != "tx.Block" || moved[0].Line != 7 || len(moved[0].Args["_do"].([]normalizer.FlowStep)) != 2 {
		t.Fatalf("transactional flow not wrapped in one tx.Block: %+v", moved)
	}
	if len(services[0].Methods[1].Flow) != 2 {
		t.Fatal("a flow without transactional: true must stay as written")
	}
	bad := []normalizer.Service{{Name: "Stock", Methods: []normalizer.Method{{Name: "Stream", Transactional: true, IsStreaming: true, Flow: flow}}}}
	if err := wrapTransactionalOperations(bad); err == nil || !strings.Contains(err.Error(), "streaming") {
		t.Fatalf("streaming + transactional: %v", err)
	}
}
