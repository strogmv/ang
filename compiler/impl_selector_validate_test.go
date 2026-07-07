package compiler

import (
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

func TestValidateMethodImplDTOSelectorsSuggestsCanonicalGoField(t *testing.T) {
	method := normalizer.Method{
		Name:   "GetCustomer",
		Source: "cue/api/impl_customer.cue:73:5",
		Input:  normalizer.Entity{Name: "GetCustomerRequest", Fields: []normalizer.Field{{Name: "customer_id"}}},
		Output: normalizer.Entity{Name: "GetCustomerResponse", Fields: []normalizer.Field{{Name: "telegram_user_id"}}},
		Impl:   &normalizer.MethodImpl{Code: "out.TelegramUserId = req.CustomerID"},
	}

	err := validateMethodImplDTOSelectors("Customer", method)
	if err == nil {
		t.Fatal("expected unknown DTO field error")
	}
	message := err.Error()
	for _, want := range []string{"cue/api/impl_customer.cue:73", `field "TelegramUserId"`, "GetCustomerResponse", `did you mean "TelegramUserID"?`} {
		if !strings.Contains(message, want) {
			t.Fatalf("error %q does not contain %q", message, want)
		}
	}
}

func TestValidateMethodImplDTOSelectorsAcceptsKnownFieldsAndLocalOut(t *testing.T) {
	tests := []string{
		"resp.TelegramUserID = req.CustomerID",
		"out := customResult{}\nout.Value = req.CustomerID",
	}
	for _, code := range tests {
		method := normalizer.Method{
			Input:  normalizer.Entity{Name: "Request", Fields: []normalizer.Field{{Name: "customer_id"}}},
			Output: normalizer.Entity{Name: "Response", Fields: []normalizer.Field{{Name: "telegram_user_id"}}},
			Impl:   &normalizer.MethodImpl{Code: code},
		}
		if err := validateMethodImplDTOSelectors("Customer", method); err != nil {
			t.Fatalf("code %q rejected: %v", code, err)
		}
	}
}

func TestValidateMethodImplDTOSelectorsChecksKnownLocalDTO(t *testing.T) {
	method := normalizer.Method{
		Source: "cue/api/customer.cue:20:1",
		Output: normalizer.Entity{Name: "GetCustomerResponse", Fields: []normalizer.Field{{Name: "telegram_user_id"}}},
		Impl:   &normalizer.MethodImpl{Code: "var local GetCustomerResponse\nlocal.TelegramUserId = 1"},
	}
	err := validateMethodImplDTOSelectors("Customer", method)
	if err == nil || !strings.Contains(err.Error(), `did you mean "TelegramUserID"?`) {
		t.Fatalf("expected local DTO selector suggestion, got %v", err)
	}
}

func TestValidateFlowLogicCallDTOSelectorsInNestedBranch(t *testing.T) {
	method := normalizer.Method{
		Input:  normalizer.Entity{Name: "Request", Fields: []normalizer.Field{{Name: "customer_id"}}},
		Output: normalizer.Entity{Name: "Response", Fields: []normalizer.Field{{Name: "telegram_user_id"}}},
		Flow: []normalizer.FlowStep{{
			Action: "flow.If",
			Args: map[string]any{"_then": []normalizer.FlowStep{{
				Action: "logic.Call", File: "cue/api/customer.cue", Line: 73,
				Args: map[string]any{"func": "func() (string, error) { return out.TelegramUserId, nil }"},
			}}},
		}},
	}
	err := validateImplDTOSelectors([]normalizer.Service{{Name: "Customer", Methods: []normalizer.Method{method}}})
	if err == nil {
		t.Fatal("expected unknown flow lambda DTO field error")
	}
	for _, want := range []string{"cue/api/customer.cue:73", `field "TelegramUserId"`, `did you mean "TelegramUserID"?`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not contain %q", err, want)
		}
	}
}
