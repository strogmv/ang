package paymentprovider

import (
	"path/filepath"
	"testing"
)

func TestVet_pumpp2pFixture(t *testing.T) {
	root := filepath.Join("testdata", "pumpp2p")
	spec, err := Load(root, ".cue", "")
	if err != nil {
		t.Fatalf("Load pumpp2p: %v", err)
	}
	if spec.APICompat != "macan_p2p" {
		t.Fatalf("api_compat: %q", spec.APICompat)
	}
	if spec.Auth == nil || spec.Auth.Header != "Authorization-Token" {
		t.Fatalf("expected profile auth, got %#v", spec.Auth)
	}
	if spec.CallbackSignature == nil || spec.CallbackSignature.Algorithm != "sha1" {
		t.Fatalf("expected profile callback_signature, got %#v", spec.CallbackSignature)
	}
	if len(spec.Operations) != 6 {
		t.Fatalf("operations: got %d want 6", len(spec.Operations))
	}
	issues := Vet(spec)
	for _, iss := range issues {
		if iss.Severity == "error" {
			t.Errorf("%s: %s", iss.Code, iss.Message)
		}
	}
}

func TestLoad_pumpp2pFixture(t *testing.T) {
	root := filepath.Join("testdata", "pumpp2p")
	if _, err := Load(root, ".cue", ""); err != nil {
		t.Fatalf("Load: %v", err)
	}
}

func TestVet_detectsMissingEndpoint(t *testing.T) {
	spec := &ProviderSpec{
		SID:          "x",
		PackageName:  "x",
		StructName:   "X",
		Endpoints:    map[string]Endpoint{"payin": {Path: "/pay"}},
		APICompat:    "macan_p2p",
		Operations: []OperationDef{{
			Kind: "init_pay_p2p",
			Transport: OperationTransport{
				Endpoint:     "missing",
				RequestType:  "req",
				ResponseType: "resp",
				StatusField:  "payIn.status",
			},
		}},
	}
	issues := Vet(spec)
	if !containsVetCode(issues, "PP001") {
		t.Fatalf("expected PP001, got %#v", issues)
	}
}

func TestVet_detectsAuthSecretMismatch(t *testing.T) {
	spec := &ProviderSpec{
		SID:         "x",
		PackageName: "x",
		StructName:  "X",
		Auth: &AuthConfig{
			Header:    "Authorization-Token",
			SecretKey: "missingKey",
		},
	}
	issues := Vet(spec)
	if !containsVetCode(issues, "PP006") {
		t.Fatalf("expected PP006, got %#v", issues)
	}
}

func containsVetCode(issues []VetIssue, code string) bool {
	for _, iss := range issues {
		if iss.Code == code {
			return true
		}
	}
	return false
}
