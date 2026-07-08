package paymentprovider

import (
	"fmt"
	"strings"
)

// VetIssue is a single semantic validation finding for a provider CUE intent.
type VetIssue struct {
	Code     string `json:"code"`
	Severity string `json:"severity"` // error | warning
	Message  string `json:"message"`
	Hint     string `json:"hint,omitempty"`
}

// Vet validates a loaded provider spec beyond CUE structural constraints.
func Vet(spec *ProviderSpec) []VetIssue {
	if spec == nil {
		return []VetIssue{{
			Code:     "PP000",
			Severity: "error",
			Message:  "provider spec is nil",
		}}
	}
	var issues []VetIssue
	secretKeys := secretPartKeys(spec)

	for _, op := range spec.Operations {
		issues = append(issues, vetOperation(spec, op)...)
	}
	issues = append(issues, vetCallback(spec)...)
	issues = append(issues, vetAuth(spec, secretKeys)...)
	issues = append(issues, vetCallbackSignature(spec, secretKeys)...)
	issues = append(issues, vetCapabilities(spec)...)

	return issues
}

// VetProject loads provider intent and runs semantic checks.
func VetProject(projectPath, cueRoot string) ([]VetIssue, error) {
	pc := LoadProjectConfig(projectPath)
	if cueRoot == "" {
		cueRoot = pc.CueRoot
	}
	schemaDir := ""
	if pc.SchemaDir != "" {
		resolved, err := ResolvePath(projectPath, pc.SchemaDir)
		if err != nil {
			return nil, err
		}
		schemaDir = resolved
	}
	spec, err := Load(projectPath, cueRoot, schemaDir)
	if err != nil {
		return nil, err
	}
	return Vet(spec), nil
}

func secretPartKeys(spec *ProviderSpec) map[string]struct{} {
	out := make(map[string]struct{}, len(spec.Secrets.Parts))
	for _, p := range spec.Secrets.Parts {
		k := strings.TrimSpace(p.Key)
		if k != "" {
			out[k] = struct{}{}
		}
	}
	return out
}

func hasSecretKey(keys map[string]struct{}, key string) bool {
	_, ok := keys[strings.TrimSpace(key)]
	return ok
}

func vetOperation(spec *ProviderSpec, op OperationDef) []VetIssue {
	var issues []VetIssue
	kind := strings.TrimSpace(op.Kind)
	t := op.Transport
	epKey := strings.TrimSpace(t.Endpoint)

	if epKey != "" {
		if spec.Endpoints == nil {
			issues = append(issues, VetIssue{
				Code:     "PP001",
				Severity: "error",
				Message:  fmt.Sprintf("operations[%s].transport.endpoint %q: endpoints map is empty", kind, epKey),
				Hint:     "Define provider.endpoints with matching keys.",
			})
		} else if _, ok := spec.Endpoints[epKey]; !ok {
			issues = append(issues, VetIssue{
				Code:     "PP001",
				Severity: "error",
				Message:  fmt.Sprintf("operations[%s].transport.endpoint %q not found in provider.endpoints", kind, epKey),
				Hint:     "Add the endpoint key or fix the operation binding.",
			})
		}
	}

	switch kind {
	case "init_pay", "init_payout", "init_pay_p2p", "init_refund":
		if strings.TrimSpace(t.RequestType) == "" {
			issues = append(issues, VetIssue{
				Code:     "PP002",
				Severity: "error",
				Message:  fmt.Sprintf("operations[%s] requires transport.request_type", kind),
			})
		}
	}

	switch kind {
	case "init_pay", "init_payout", "init_pay_p2p", "init_refund",
		"check_status_payin", "check_status_payout", "cancel_pay", "fetch_balances":
		if strings.TrimSpace(t.ResponseType) == "" {
			issues = append(issues, VetIssue{
				Code:     "PP003",
				Severity: "error",
				Message:  fmt.Sprintf("operations[%s] requires transport.response_type", kind),
			})
		}
	}

	if strings.HasPrefix(kind, "check_status_") {
		strategy := strings.TrimSpace(t.StatusStrategy)
		if strategy == "" || strategy == "inherit" {
			if spec.APICompat == "macan_p2p" {
				strategy = "macan"
			}
		}
		if strategy == "macan" || strategy == "direct" {
			if strings.TrimSpace(t.StatusField) == "" {
				issues = append(issues, VetIssue{
					Code:     "PP004",
					Severity: "error",
					Message:  fmt.Sprintf("operations[%s] with status_strategy %q requires transport.status_field", kind, strategy),
				})
			}
		}
	}

	return issues
}

func vetCallback(spec *ProviderSpec) []VetIssue {
	if spec.Callback == nil {
		return nil
	}
	if strings.TrimSpace(spec.Callback.TxIDField) == "" {
		return []VetIssue{{
			Code:     "PP005",
			Severity: "error",
			Message:  "callback is defined but callback.tx_id_field is empty",
			Hint:     "Set tx_id_field to the JSON field carrying the internal transaction id.",
		}}
	}
	return nil
}

func vetAuth(spec *ProviderSpec, secretKeys map[string]struct{}) []VetIssue {
	if spec.Auth == nil {
		return nil
	}
	key := strings.TrimSpace(spec.Auth.SecretKey)
	if key == "" {
		return []VetIssue{{
			Code:     "PP006",
			Severity: "error",
			Message:  "auth.secret_key is required when auth is configured",
		}}
	}
	if !hasSecretKey(secretKeys, key) {
		return []VetIssue{{
			Code:     "PP006",
			Severity: "error",
			Message:  fmt.Sprintf("auth.secret_key %q not found in secrets.parts", key),
			Hint:     "Add a matching secrets.parts entry or fix auth.secret_key.",
		}}
	}
	if strings.TrimSpace(spec.Auth.Header) == "" {
		return []VetIssue{{
			Code:     "PP006",
			Severity: "error",
			Message:  "auth.header is required when auth is configured",
		}}
	}
	return nil
}

func vetCallbackSignature(spec *ProviderSpec, secretKeys map[string]struct{}) []VetIssue {
	if spec.CallbackSignature == nil {
		return nil
	}
	key := strings.TrimSpace(spec.CallbackSignature.SecretKey)
	if key == "" {
		return []VetIssue{{
			Code:     "PP007",
			Severity: "error",
			Message:  "callback_signature.secret_key is required when callback_signature is configured",
		}}
	}
	if !hasSecretKey(secretKeys, key) {
		return []VetIssue{{
			Code:     "PP007",
			Severity: "error",
			Message:  fmt.Sprintf("callback_signature.secret_key %q not found in secrets.parts", key),
			Hint:     "Add signature key to secrets.parts (e.g. signatureKey).",
		}}
	}
	format := strings.TrimSpace(spec.CallbackSignature.Format)
	if format == "username_key_form_b64" {
		userKey := strings.TrimSpace(spec.CallbackSignature.UsernameKey)
		if userKey == "" {
			return []VetIssue{{
				Code:     "PP007",
				Severity: "error",
				Message:  "callback_signature.username_key is required for username_key_form_b64",
			}}
		}
		if !hasSecretKey(secretKeys, userKey) {
			return []VetIssue{{
				Code:     "PP007",
				Severity: "error",
				Message:  fmt.Sprintf("callback_signature.username_key %q not found in secrets.parts", userKey),
			}}
		}
		return nil
	}
	if len(spec.CallbackSignature.Fields) == 0 {
		return []VetIssue{{
			Code:     "PP007",
			Severity: "error",
			Message:  "callback_signature.fields must not be empty",
		}}
	}
	return nil
}

func vetCapabilities(spec *ProviderSpec) []VetIssue {
	var issues []VetIssue
	if spec.HasP2P && !hasOperationKind(spec.Operations, "init_pay_p2p") {
		issues = append(issues, VetIssue{
			Code:     "PP008",
			Severity: "error",
			Message:  "has_p2p is true but no init_pay_p2p operation is defined",
			Hint:     "Add init_pay_p2p to operations or use schema.#ProfileMacanP2P.",
		})
	}
	if spec.HasCancel && !hasOperationKind(spec.Operations, "cancel_pay") {
		issues = append(issues, VetIssue{
			Code:     "PP009",
			Severity: "error",
			Message:  "has_cancel is true but no cancel_pay operation is defined",
		})
	}
	if spec.HasPayout && !hasOperationKind(spec.Operations, "init_payout") {
		issues = append(issues, VetIssue{
			Code:     "PP009",
			Severity: "warning",
			Message:  "has_payout is true but no init_payout operation is defined",
		})
	}
	if len(spec.SupportedCurrencies) == 0 && len(spec.PaymentMethodMap) > 0 {
		for _, m := range spec.PaymentMethodMap {
			if len(m.CurrencyOverrides) > 0 {
				issues = append(issues, VetIssue{
					Code:     "PP010",
					Severity: "warning",
					Message:  "payment_method_map has currency_overrides but supported_currencies is empty",
					Hint:     "Set supported_currencies for multi-currency validation.",
				})
				break
			}
		}
	}
	return issues
}

func hasOperationKind(ops []OperationDef, kind string) bool {
	for _, op := range ops {
		if op.Kind == kind {
			return true
		}
	}
	return false
}
