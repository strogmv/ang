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

func TestResolveSource_foreignID(t *testing.T) {
	fields := []RequestField{{Name: "ReferenceID", JSON: "reference_id", Source: "foreign_id"}}
	resolved, err := ResolveRequestFields(fields, "card", 840)
	if err != nil {
		t.Fatal(err)
	}
	if resolved[0].GoExpr != `tx.ForeignId` {
		t.Fatalf("foreign_id: got %q", resolved[0].GoExpr)
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

func TestResolveSource_ownerInfoGetParameterEvenWhenOmitEmpty(t *testing.T) {
	fields := []RequestField{
		{Name: "Email", JSON: "email", Source: "owner_info", OwnerKey: "email", OwnerFrom: "card", OmitEmpty: true},
	}
	resolved, err := ResolveRequestFields(fields, "card", 840)
	if err != nil {
		t.Fatal(err)
	}
	want := `providers.GetParameter(card.OwnerInfo, providers.Email)`
	if resolved[0].GoExpr != want {
		t.Fatalf("got %q want %q", resolved[0].GoExpr, want)
	}
}

func TestResolveSource_ownerInfoWithoutParameterUsesMapIndex(t *testing.T) {
	fields := []RequestField{
		{Name: "IBAN", JSON: "iban", Source: "owner_info", OwnerKey: "receiver_iban", OwnerFrom: "card", OmitEmpty: true},
	}
	resolved, err := ResolveRequestFields(fields, "card", 840)
	if err != nil {
		t.Fatal(err)
	}
	want := `card.OwnerInfo[providers.ReceiverIbanKey]`
	if resolved[0].GoExpr != want {
		t.Fatalf("got %q want %q", resolved[0].GoExpr, want)
	}
}

func TestResolveSource_ownerInfoWithoutParameterKeepsMapGetSentinel(t *testing.T) {
	fields := []RequestField{
		{Name: "IBAN", JSON: "iban", Source: "owner_info", OwnerKey: "receiver_iban", OwnerFrom: "card", Default: "GB00"},
	}
	resolved, err := ResolveRequestFields(fields, "card", 840)
	if err != nil {
		t.Fatal(err)
	}
	want := `helpers.MapGet(card.OwnerInfo, providers.ReceiverIbanKey, "GB00")`
	if resolved[0].GoExpr != want {
		t.Fatalf("got %q want %q", resolved[0].GoExpr, want)
	}
}

func TestResolveSource_externalCustomerIDCard(t *testing.T) {
	fields := []RequestField{
		{Name: "ExternalID", JSON: "external_id", Source: "external_customer_id", OmitEmpty: true},
	}
	resolved, err := ResolveRequestFields(fields, "card", 840)
	if err != nil {
		t.Fatal(err)
	}
	want := `card.OwnerInfo[providers.ExternalCustomerIDKey]`
	if resolved[0].GoExpr != want {
		t.Fatalf("got %q want %q", resolved[0].GoExpr, want)
	}
	if !resolved[0].IsCard {
		t.Fatal("expected IsCard so the constructor declares card")
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

func TestResolveCardExpYearFmt(t *testing.T) {
	fields, err := ResolveRequestFields([]RequestField{
		{Name: "Year", JSON: "year", Source: "card_exp_year_fmt"},
	}, "card", 840)
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 1 || fields[0].GoExpr != `fmt.Sprintf("%d", card.ExpDateYear)` {
		t.Fatalf("got %#v", fields)
	}
}

func TestFillPerMethodObjects_localsStayOnTheMethod(t *testing.T) {
	fields := []ResolvedField{{
		GoName:    "Details",
		Type:      "payinDetails",
		PerMethod: true,
	}}
	methods := []Method{
		{Sid: "cards", Destination: []RequestField{{Name: "PAN", JSON: "card_number", Source: "card_pan"}}},
		{Sid: "upi", Destination: []RequestField{{Name: "UPI", JSON: "upi_id", Source: "apm_id"}}},
	}
	if err := FillPerMethodObjects(fields, methods, "both", 840); err != nil {
		t.Fatal(err)
	}
	slot := fields[0]
	if len(slot.Locals) != 0 {
		t.Fatalf("slot constructor must not union locals, got %#v", slot.Locals)
	}
	if len(slot.Methods) != 2 {
		t.Fatalf("methods: %d", len(slot.Methods))
	}
	if got := localNames(slot.Methods[0].Locals); len(got) != 1 || got[0] != "card" {
		t.Fatalf("cards locals: %v", got)
	}
	if got := localNames(slot.Methods[1].Locals); len(got) != 0 {
		t.Fatalf("upi locals should be empty (ps.APM.ID), got %v", got)
	}
	if slot.Methods[0].GoConst != "CardsMethod" || slot.Methods[1].GoConst != "UPIMethod" {
		t.Fatalf("go consts: %q %q", slot.Methods[0].GoConst, slot.Methods[1].GoConst)
	}
}

func TestRequestLocals_skipsPerMethodSlot(t *testing.T) {
	fields := []ResolvedField{
		{GoName: "Amount", GoExpr: "tx.Amount", Source: "tx_amount"},
		{GoName: "Details", PerMethod: true, Nested: []ResolvedField{
			{GoName: "PAN", GoExpr: "card.PAN", Source: "card_pan"},
		}},
	}
	locals := RequestLocals(fields)
	if len(locals) != 0 {
		t.Fatalf("outer ctor must not hoist slot locals, got %#v", locals)
	}
}

func TestResolveRequestDef_rejectsMixedEnvelopeSources(t *testing.T) {
	def := &RequestDef{
		Name: "payinRequest",
		Fields: []RequestField{
			{Name: "PAN", JSON: "pan", Source: "card_pan"},
			{Name: "Email", JSON: "email", Source: "owner_info", OwnerKey: "email", OwnerFrom: "apm"},
		},
	}
	_, err := resolveRequestDef(def, nil, "both", 840)
	if err == nil {
		t.Fatal("expected mixed-source error")
	}
}

func TestCollectObjectTypes_threeLevels(t *testing.T) {
	fields := []ResolvedField{{
		GoName: "Outer",
		Type:   "payinOuter",
		Nested: []ResolvedField{{
			GoName: "Mid",
			Type:   "payinOuterMid",
			Nested: []ResolvedField{{
				GoName: "Inner",
				Type:   "payinOuterMidInner",
				Nested: []ResolvedField{
					{GoName: "Amount", GoExpr: "tx.Amount", Source: "tx_amount"},
				},
			}},
		}},
	}}
	types := collectObjectTypes(fields)
	if len(types) != 3 {
		t.Fatalf("object types: %d %#v", len(types), types)
	}
	leaves := collectMaskLeaves(fields, "r")
	if len(leaves) != 1 || leaves[0].Selector != "r.Outer.Mid.Inner.Amount" {
		t.Fatalf("mask leaves: %#v", leaves)
	}
}

func TestFillPerMethodObjects_requiredKeepsEmpty(t *testing.T) {
	fields := []ResolvedField{{GoName: "Details", Type: "details", PerMethod: true}}
	methods := []Method{{
		Sid: "cards",
		Destination: []RequestField{
			{Name: "PAN", JSON: "card_number", Source: "card_pan"},
			{Name: "Token", JSON: "token", Source: "const", ConstVal: "", Required: true},
		},
	}}
	// const with empty const_val fails resolve — use tx_id for required empty-able string
	methods[0].Destination[1] = RequestField{Name: "Note", JSON: "note", Source: "tx_description", Required: true}
	if err := FillPerMethodObjects(fields, methods, "card", 840); err != nil {
		t.Fatal(err)
	}
	var note ResolvedField
	for _, f := range fields[0].Nested {
		if f.GoName == "Note" {
			note = f
		}
		if f.GoName == "PAN" && !f.OmitEmpty {
			t.Fatal("union PAN should be omitempty")
		}
	}
	if note.GoName == "" {
		t.Fatal("missing Note")
	}
	if note.OmitEmpty || !note.Required {
		t.Fatalf("required note: omitempty=%v required=%v", note.OmitEmpty, note.Required)
	}
}

func localNames(locals []RequestLocal) []string {
	out := make([]string, 0, len(locals))
	for _, l := range locals {
		out = append(out, l.Name)
	}
	return out
}
