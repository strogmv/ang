package schema

// Canonical payment-provider CUE schema bundled with ang.
// Sync into provider projects: ang pp schema sync [path]

#Field: {
	name:      string
	type:      string
	json:      string
	omitempty: *false | bool
	comment:   *"" | string
}

// Field sources for auto-population of request structs.
// Each source maps to a Go expression in the generated code.
#FieldSource: #CatalogFieldSource

#OwnerInfoKey: #CatalogOwnerInfoKey

#OwnerInfoFrom: "card" | "apm"

#RequestFieldMapping: {
	name:       string        // Go struct field name
	json:       string        // JSON tag
	type:       *"string" | "int" | "int64" | "float64" | "decimal.Decimal" | "bool"
	source:     #FieldSource  // what populates this field
	secret_key: *"" | string  // when source is "secret", which key from apiSecrets
	owner_key:  *"" | #OwnerInfoKey // when source is "owner_info"
	owner_from: *"" | #OwnerInfoFrom
	default:    *"" | string  // default value for MapGet/GetBrowserData
	const_val:  *"" | string  // literal value when source is "const"
	format:     *"" | string  // optional format string
	omitempty:  *false | bool
	redacted:   *false | bool // hide in logs (PAN, CVV, tokens)
}

#RequestDef: {
	name:   string
	fields: [...#RequestFieldMapping]
}

// Deprecated: use payin_request / payout_request (#RequestDef) instead.
#RequestType: {
	name:   string
	fields: [...#Field]
	has_marshal:  *true | bool
	has_redacted: *false | bool
	redacted_fields: [...string]
}

#ResponseType: {
	name:   string
	fields: [...#Field]
}

// --- Transport operation DSL (step-by-step migration target) ---
// Declarative binding between provider operations and transport contracts.
#OperationKind:
	"init_pay" |
	"init_payout" |
	"check_status_payin" |
	"check_status_payout" |
	"init_pay_p2p" |
	"cancel_pay" |
	"fetch_balances" |
	"init_refund" |
	"parse_callback" |
	"finish_callback"

#OperationTransport: {
	// Key from provider.endpoints map
	endpoint: string
	// Optional explicit path override (when endpoint-key lookup is not enough)
	endpoint_path: *"" | string

	// Struct names for request/response contracts
	request_type:        *"" | string
	response_type:       *"" | string
	error_response_type: *"" | string

	// Optional operation-level logging override
	response_logging_mode: *"inherit" | "raw" | "prefer_parsed"

	// Optional status extraction strategy for runtime guards/parsers.
	// - inherit: use provider-level/default strategy
	// - macan: payIn/payOut status + statusDetails contract
	// - direct: flat "status" field contract
	status_strategy: *"inherit" | "macan" | "direct"

	// Optional operation-level runtime policy overrides.
	// Empty values mean "inherit from runtime_policy_config".
	retry_max_attempts:   *0 | int
	retry_initial_backoff:*"" | string
	retry_max_backoff:    *"" | string
	timeout:              *"" | string
	// Operation behaviour when callback returns pending/intermediate status.
	pending_callback_action: *"none" | "check_status"

	// Optional response status extraction hints
	status_field:         *"" | string // e.g. "status", "ticket.status"
	status_details_field: *"" | string // e.g. "status_details"
	error_code_field:     *"" | string
}

#OperationDef: {
	kind:      #OperationKind
	transport: #OperationTransport
}

#Endpoint: {
	path:   string
	method: *"POST" | "GET" | "PUT" | "DELETE" | "PATCH"
}

// --- Status mapping ---

#StatusMapping: {
	code:        int | string
	status:      "success" | "pending" | "declined" | "error"
	status_code: #StatusCode
	message:     *"" | string
}

#ErrorMapping: {
	code:        string
	status:      "success" | "pending" | "declined" | "error"
	status_code: #StatusCode
}

// Multi-dimensional error mapping for providers returning major/minor codes.
#ErrorMatrixEntry: {
	major:       string | int
	minor:       *"" | string | int
	status:      "success" | "pending" | "declined" | "error"
	status_code: #StatusCode
	message:     *"" | string
}

// Macan: maps internal payment method brand → API payment_method.name
#PaymentMethodMapEntry: {
	brand: string
	match: *"literal" |
		"types_cardp2p" |
		"types_sbp" |
		"types_click" |
		"types_qrcode" |
		"types_trans_card2card" |
		"types_trans_sbp"
	api_name: string
	currency_overrides: *[...{currency: string, api_name: string}] | [...{currency: string, api_name: string}]
}

#StatusCode: #CatalogStatusCode

// --- Secrets ---

#SecretPart: {
	name: string
	key:  string
	optional?: bool
	// Optional transform applied after parsing the part.
	// query_unescape is useful for URL values entered percent-encoded.
	transform?: *"none" | "query_unescape" | "trim_space"
}

// --- Signing ---

#SigningScheme: {
	algorithm: #SigningAlgorithm
	format:    *"key_concat_sorted_values" | "hmac" | "hmac_body" | "md5_concat" | "basic_auth" | "custom"
}

#SigningAlgorithm: "sha256" | "hmac-sha256" | "md5" | "none" | "basic"

// --- HTTP auth (request headers) ---

#AuthType: "header_token" | "bearer" | "basic" | "custom"

#AuthConfig: {
	type: *"header_token" | #AuthType
	header: string
	secret_key: string
	content_type: *"application/json" | string
	prefix: *"" | string // e.g. "Bearer "
}

// --- Callback signature verification ---

#CallbackSignatureFormat: "sorted_kv_pipe" | "hmac_body" | "custom"
#CallbackSignatureCompare: "equal_fold" | "equal"
#CallbackSignatureFieldFormat: "plain" | "float_trailing_zero"

#CallbackSignatureField: {
	json?:         string
	const_key?:    string
	secret_key?:   string
	omit_if_empty?: *false | bool
	format?:       *"plain" | #CallbackSignatureFieldFormat
}

#CallbackSignatureConfig: {
	algorithm:  "sha1" | "sha256" | "md5" | "hmac-sha256"
	secret_key: string
	format:     *"sorted_kv_pipe" | #CallbackSignatureFormat
	fields:     [...#CallbackSignatureField]
	compare:    *"equal_fold" | #CallbackSignatureCompare
}

// --- Payment methods (from utils/types) ---

#PaymentMethodType: "card" | "digitalwallet" | "crypto" | "banktransfer" | "other"

#PaymentMethodSID: #CatalogPaymentMethodSID

#AccountIDType: "phone" | "email" | "iban" | "none" | "walletid"

#PaymentMethod: {
	sid:            #PaymentMethodSID
	method_type:    #PaymentMethodType
	account_id_type: *"none" | #AccountIDType
}

// --- Transaction types ---

#TransactionType: "pay" | "payout" | "refund"

// --- Auth flow types ---

#AuthFlowType: #CatalogAuthFlowType

// --- Optional interfaces ---

#OptionalInterfaces: {
	balance_fetcher:          *false | bool
	mobile_processor:         *false | bool
	otp_form_redirector:      *false | bool
	tds_redirector:           *false | bool
	payment_method_selector:  *false | bool
	customer_randomization:   *false | bool
}

// --- OTP config ---

#OTPConfig: {
	handles_externally: *false | bool
}

// --- Check status config ---

#CheckStatusConfig: {
	since_created_period: *"" | string
	by_transaction_type:  *false | bool
}

// --- Async 3DS orchestration config ---
// Describes callback-triggered async charge flows (as used by Blumon-like providers).
#Async3DSConfig: {
	enabled:                     *false | bool
	auto_charge_after:           *"" | string // e.g. "2m"
	finish_callback_wait_timeout:*"" | string // e.g. "90s"
	stale_pending_grace:         *"" | string // e.g. "30s"
}

// --- Cross-instance state config ---
// Shared state backend for multi-instance pending/completion coordination.
#CrossInstanceStateConfig: {
	enabled: *false | bool
	backend: *"none" | "redis"
}

// --- Confirmation via list/check endpoint ---
// For providers where initial charge can be non-final and requires list polling.
#StatusConfirmationConfig: {
	enabled:            *false | bool
	strategy:           *"none" | "transaction_list"
	retry_not_ready:    *false | bool
	not_ready_patterns: [...string]
}

// --- CheckStatus throttling config ---
// Rate limits expensive provider status/list calls.
#CheckStatusThrottleConfig: {
	enabled: *false | bool
	period:  *"" | string // e.g. "2m"
}

// --- Extended callback output config ---
// Ensures non-empty callback output fields where downstream systems require them.
#ExtendedCallbackConfig: {
	enabled: *false | bool
	fields:  [...("auth_code" | "md" | "rrn")]
}

// --- Runtime quality policies (cross-operation) ---
// These policies are intentionally operation-agnostic and should be reused
// across generated flows to keep reliability behaviour consistent.
#RuntimePolicyConfig: {
	timeouts: {
		request_timeout:       *"" | string // e.g. "30s"
		check_status_timeout:  *"" | string // e.g. "15s"
		callback_wait_timeout: *"" | string // e.g. "90s"
	}
	retries: {
		max_attempts:        *0 | int
		initial_backoff:     *"" | string // e.g. "500ms"
		max_backoff:         *"" | string // e.g. "10s"
		retry_on_not_found:  *false | bool
		retry_on_5xx:        *false | bool
		retry_on_rate_limit: *false | bool
	}
	limits: {
		max_callback_body_bytes: *0 | int
		max_pending_age:         *"" | string
	}
}

// --- Constructor dependencies ---

#ConstructorDep: {
	name: string
	type: string
	pkg:  *"" | string
}

// --- Callback payload ---

#CallbackField: {
	name:      string
	type:      string
	json:      string
	omitempty: *false | bool
}

#CallbackConfig: {
	tx_id_field:      *"" | string
	foreign_id_field: *"" | string
	status_field:     string
	status_type:      *"int" | "string"
	error_code_field: *"" | string
	// Optional return-url callback channel via query parameter (e.g. ?txid=...).
	return_query_txid_param:   *"" | string
	// Optional status value injected for return-url query callbacks.
	return_query_status_value: *"" | string
	// Optional flag for generated CallbackData.InfoCallback on return-url query callbacks.
	return_query_info_callback:*false | bool
	fields: [...#CallbackField]
}

// --- Payment source type ---

#PaymentSourceType: "card" | "apm" | "both"

// --- Main Provider definition ---

#Provider: {
	package_name: string
	sid:          string
	source:       string
	label:        string
	mid_prefix:   string

	struct_name:      string
	constructor_name: string

	// What payment source this provider accepts
	payment_source: *"apm" | #PaymentSourceType

	secrets: {
		format:    string
		separator: *":" | string
		parts: [...#SecretPart]
	}

	currency: {
		code:     string
		iso_num:  *0 | int
		country:  *"" | string
	}

	// Multi-currency support: if empty, single currency from `currency.code` is used
	supported_currencies: [...string]

	endpoints: {
		[string]: #Endpoint
	}

	signing: #SigningScheme

	// HTTP auth headers for outbound API calls (optional; profile may set defaults).
	auth: *null | #AuthConfig

	// Callback webhook signature verification (optional; separate from outbound signing).
	callback_signature: *null | #CallbackSignatureConfig

	payin_statuses: [...#StatusMapping]
	payout_statuses: [...#StatusMapping]
	error_codes: [...#ErrorMapping]
	error_mapping_matrix: *[...#ErrorMatrixEntry] | [...#ErrorMatrixEntry]
	// Provider status_details / detail strings (e.g. Macan status_details) → internal status
	status_details: [...#ErrorMapping]
	// Macan P2P: brand → API payment method name (see payment_method_map)
	payment_method_map: *[...#PaymentMethodMapEntry] | [...#PaymentMethodMapEntry]

	// Auto-populated request definitions (generate both struct + population code)
	payin_request:  *null | #RequestDef
	payout_request: *null | #RequestDef
	p2p_request:    *null | #RequestDef
	refund_request: *null | #RequestDef

	// Declarative operation→transport bindings.
	operations: [...#OperationDef]

	// Deprecated: manual struct definitions — use payin_request / payout_request.
	request_types: [...#RequestType]
	response_types: [...#ResponseType]

	supported_methods: [...#PaymentMethodSID]

	// Transaction type support
	has_payin:        *true | bool
	has_payout:       *true | bool
	has_p2p:          *false | bool
	has_subscription: *false | bool
	has_refund:       *false | bool
	has_cancel:       *false | bool

	// Auth flow
	auth_flow: *"h2h" | #AuthFlowType

	// Merchant API compatibility layer (e.g. Transferty H2H JSON shape)
	api_compat: *"" | "transferty_h2h" | "macan_p2p"
	// macan_p2p: use has_p2p + InitPayP2P; classic has_payin should stay false

	// Response logging strategy for raw response bodies:
	// - raw: keep raw response details logging
	// - prefer_parsed: try response.Unmarshal+String (or JSON marshal) before raw fallback
	response_logging_mode: *"raw" | "prefer_parsed"

	// Optional interfaces
	interfaces: *#OptionalInterfaces | #OptionalInterfaces

	// Constructor dependencies
	constructor_deps: [...#ConstructorDep]

	// OTP configuration
	otp_config: *null | #OTPConfig

	// Check status configuration
	check_status_config: *null | #CheckStatusConfig

	// Async 3DS orchestration (callback-triggered charge flow)
	async_3ds_config: *null | #Async3DSConfig

	// Cross-instance shared state for async flows
	cross_instance_state_config: *null | #CrossInstanceStateConfig

	// Non-final charge confirmation strategy (e.g. transaction list lookup)
	status_confirmation_config: *null | #StatusConfirmationConfig

	// CheckStatus throttling policy
	check_status_throttle_config: *null | #CheckStatusThrottleConfig

	// Extended callback output policy
	extended_callback_config: *null | #ExtendedCallbackConfig

	// Shared runtime reliability policy (timeouts/retries/limits)
	runtime_policy_config: *null | #RuntimePolicyConfig

	// Callback configuration
	callback: *null | #CallbackConfig

	// Custom imports
	extra_imports: [...string]
}
