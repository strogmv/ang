package paymentprovider

import "testing"

func TestResolveSource_txFields(t *testing.T) {
	fields := []RequestField{
		{Name: "PayInID", JSON: "payInId", Source: "tx_id"},
		{Name: "Amount", JSON: "amount", Source: "tx_amount_fmt"},
		{Name: "Currency", JSON: "currency", Source: "currency_iso_num", Type: "int"},
		{Name: "Method", JSON: "method", Source: "secret", SecretKey: "method"},
		{Name: "Salt", JSON: "salt", Source: "salt"},
	}
	resolved, err := ResolveRequestFields(fields, "apm", 643)
	if err != nil {
		t.Fatal(err)
	}
	if resolved[0].GoExpr != `tx.Id` {
		t.Fatalf("tx_id: got %q", resolved[0].GoExpr)
	}
	if resolved[3].GoExpr != "secrets.method" {
		t.Fatalf("secret: got %q", resolved[3].GoExpr)
	}
}

func TestResolveSource_ownerInfoAPM(t *testing.T) {
	fields := []RequestField{
		{Name: "Phone", JSON: "phone", Source: "owner_info", OwnerKey: "phone", OwnerFrom: "apm"},
	}
	resolved, err := ResolveRequestFields(fields, "apm", 643)
	if err != nil {
		t.Fatal(err)
	}
	want := `providers.GetParameter(info, providers.Phone)`
	if resolved[0].GoExpr != want {
		t.Fatalf("got %q want %q", resolved[0].GoExpr, want)
	}
	if !resolved[0].IsAPM {
		t.Fatal("expected IsAPM")
	}
}

func TestResolveSource_guardClassification(t *testing.T) {
	fields := []RequestField{
		{Name: "PAN", Source: "card_pan"},
		{Name: "ID", Source: "apm_id"},
	}
	resolved, err := ResolveRequestFields(fields, "both", 643)
	if err != nil {
		t.Fatal(err)
	}
	if !resolved[0].IsCard || resolved[0].IsAPM {
		t.Fatalf("card_pan: IsCard=%v IsAPM=%v", resolved[0].IsCard, resolved[0].IsAPM)
	}
	if resolved[1].IsCard || !resolved[1].IsAPM {
		t.Fatalf("apm_id: IsCard=%v IsAPM=%v", resolved[1].IsCard, resolved[1].IsAPM)
	}
}

func TestResolveSource_constUsesNamedConstant(t *testing.T) {
	fields := []RequestField{
		{Name: "TransactionType", JSON: "transaction_type", Source: "const", ConstVal: "CREDIT"},
	}
	resolved, err := ResolveRequestFields(fields, "card", 840)
	if err != nil {
		t.Fatal(err)
	}
	if resolved[0].GoExpr != "transactionType" {
		t.Fatalf("const: got %q want transactionType", resolved[0].GoExpr)
	}
}

func TestBuildRequestLiteralConsts(t *testing.T) {
	spec := &ProviderSpec{
		PayoutRequest: &RequestDef{
			Name: "payoutRequest",
			Fields: []RequestField{
				{Name: "TransactionType", Source: "const", ConstVal: "CREDIT"},
			},
		},
	}
	consts, err := BuildRequestLiteralConsts(spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(consts) != 1 || consts[0].Name != "transactionType" || consts[0].Value != "CREDIT" {
		t.Fatalf("unexpected consts: %#v", consts)
	}
}
