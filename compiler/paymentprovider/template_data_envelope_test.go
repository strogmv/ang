package paymentprovider

import "testing"

func TestNestUnderEnvelope(t *testing.T) {
	env := &ResponseEnvelopeTemplate{Enabled: true, WrapperGoField: "Data"}
	if got := nestUnderEnvelope(env, "Status"); got != "Data.Status" {
		t.Fatalf("got %q", got)
	}
	if got := nestUnderEnvelope(nil, "Status"); got != "Status" {
		t.Fatalf("nil env: %q", got)
	}
	if got := nestUnderEnvelope(env, "Data.UUID"); got != "Data.UUID" {
		t.Fatalf("already nested: %q", got)
	}
}

func TestLooksLikeEnvelope(t *testing.T) {
	env := &ResponseEnvelopeTemplate{Enabled: true, SuccessGoField: "Success", WrapperGoField: "Data"}
	shell := ResponseType{Name: "payinResponse", Fields: []StructField{
		{Name: "Success", Type: "bool"},
		{Name: "Data", Type: "payinData"},
	}}
	payload := ResponseType{Name: "payinData", Fields: []StructField{
		{Name: "UUID", Type: "string"},
		{Name: "Status", Type: "string"},
	}}
	if !looksLikeEnvelope(shell, env) {
		t.Fatal("expected envelope shell")
	}
	if looksLikeEnvelope(payload, env) {
		t.Fatal("payload is not an envelope")
	}
}

func TestInferResponseSkipsEnvelopeShell(t *testing.T) {
	env := &ResponseEnvelopeTemplate{Enabled: true, SuccessGoField: "Success", WrapperGoField: "Data"}
	types := []ResponseType{
		{Name: "payinResponse", Fields: []StructField{
			{Name: "Success", Type: "bool"},
			{Name: "Data", Type: "payinData"},
		}},
		{Name: "payinData", Fields: []StructField{
			{Name: "UUID", Type: "string"},
			{Name: "Status", Type: "string"},
		}},
	}
	name, foreign := inferResponse(types, "payin", "payinProfile", "PaymentID", env)
	if name != "payinData" || foreign != "UUID" {
		t.Fatalf("got %s %s", name, foreign)
	}
}
