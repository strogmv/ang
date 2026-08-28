package schema

// TemplateData contract: CUE #Provider → Go text/template context.
//
// Ang refactor target: compiler/paymentprovider/emit/template_data.go
// Every field here MUST be populated from #Provider (directly or derived).
// Templates reference fields as {{ .FieldName }} or {{ .Nested.Field }}.
//
// Naming: CUE snake_case → Go PascalCase (json tags below document the mapping).

#TemplateAuth: {
	// CUE: auth.type
	Type: "header_token" | "bearer" | "basic" | "custom"
	// CUE: auth.header
	Header: string
	// CUE: auth.secret_key → SecretKeyField (Go struct field name in apiSecrets)
	SecretKeyField: string
	ContentType:    string
	Prefix:         string
	// CUE: auth.masked → use httpcli.AddMasked in authHeaders
	Masked: bool
}

#TemplateCallbackSignature: {
	Algorithm:        string // sha1 | sha256 | hmac-sha256 | hmac-sha512
	SecretKeyField:   string
	UsernameKeyField: string
	Format:           string // sorted_kv_pipe | hmac_body | username_key_form_b64 | custom
	SignatureField:   string // cbData JSON field when format != hmac_body
	CompareEqualFold: bool
	Optional:         bool   // skip ValidateCallback when secret empty
	Header:           string // e.g. X-Signature, X-Notification-Token
}

#TemplateCheckStatusConfig: {
	Enabled:              bool
	SinceCreatedPeriod:   string // Go: "10m" → 10 * time.Minute OR time.ParseDuration
	ByTransactionType:    bool
	PathSuffixForeignID:  bool // GET endpoint + "/" + url.PathEscape(tx.ForeignId)
}

#TemplatePayoutRuntime: {
	ForeignIDOnUnexpectedError: bool // InitPayout: pending + client UUID ForeignId
	ClientUUIDVarName: string // default "clientPayoutID"
}

#TemplateResponseEnvelope: {
	Enabled:        bool
	WrapperField:   string
	WrapperGoField: string
	SuccessField:   string
	SuccessGoField: string
	ErrorField:     string
	ErrorGoField:   string
}

#TemplateRequestSigning: {
	Algorithm:        string // sha256 | hmac-sha1 | hmac-sha256 | md5
	Format:           string // method_url_body | md5_concat | notification_token | username_key_body_b64 | hmac_timestamp_nonce
	Header:           string // X-Signature
	SecretKeyField:   string
	UsernameHeader:   string
	UsernameKeyField: string
	Encoding:         string // base64 | hex
	TimestampHeader:  string // hmac_timestamp_nonce: X-Timestamp
	NonceHeader:      string // hmac_timestamp_nonce: X-Nonce
	// md5_concat: ordered field names for Pacepay-style signatures
	ConcatFields: [...string]
}

#TemplateSecretPart: {
	Name:      string // merchant-facing label
	Key:       string // apiSecrets struct field (PascalCase)
	Optional:  bool
	Type:      "string" | "bool"
	Transform: string
}

#TemplateRequestField: {
	GoName:   string
	GoExpr:   string
	IsCard:   bool
	IsAPM:    bool
	Redacted: bool
}

#TemplateRequestDef: {
	Name:   string
	Fields: [...#TemplateRequestField]
}

#TemplateOperationTransport: {
	Endpoint:              string
	EndpointPath:          string
	RequestType:           string
	ResponseType:          string
	ErrorResponseType:     string
	ResponseLoggingMode:   string
	StatusStrategy:        string
	RetryMaxAttempts:      int
	RetryInitialBackoff:   string
	RetryMaxBackoff:       string
	Timeout:               string
	PendingCallbackAction: string
	StatusField:           string
	StatusDetailsField:    string
	ErrorCodeField:        string
	ContentType:           string // per-operation override
	ResponseFormat:        string // json | xml
	ResponseEnvelope:      *null | #TemplateResponseEnvelope
}

#TemplateOperation: {
	Kind:      string
	Transport: #TemplateOperationTransport
}

// Root template context (ang: paymentprovider.TemplateData).
#TemplateData: {
	// Identity
	PackageName:      string
	StructName:       string
	ConstructorName:  string
	ConstructorParams: string
	SID:              string
	Label:            string
	Source:           string
	MIDPrefix:        string

	// Capability flags (derived from has_* and endpoints)
	HasPayin:            bool
	HasPayout:           bool
	HasP2P:              bool
	HasRefund:           bool
	HasCancel:           bool
	HasSubscription:     bool
	HasStatusEndpoints:  bool
	HasBalanceFetcher:   bool
	HasMobileProcessor:  bool
	HasCustomerRandomization: bool
	HasCheckStatusConfig:     bool
	HasMultiCurrency:         bool

	// api_compat → boolean shortcuts for bundled compatibility layers (macan, paytech, fluxsgate)
	APICompat:          string
	UseMacanP2P:        bool
	UseTransfertyH2H:   bool
	UsePaytechGateway:  bool
	UseFluxsgate:       bool
	UseOperationRuntime: bool

	PaymentSource: string // card | apm | both
	AuthFlow:      string
	StatusCodeType: string // int | string

	// Secrets (CUE: secrets.*)
	SecretFormat:     string
	SecretSeparator:  string
	SecretParts:      [...#TemplateSecretPart]
	SecretPartsCount:         int
	SecretPartsNeedTransform: bool
	SecretPartsSimple:        bool // all required string parts, no transforms
	SecretUseLabels:  bool // secrets.use_labels → creds_macan.go.tmpl + secretLabels.GetLabels()

	// HTTP
	HTTPAuth:        *null | #TemplateAuth
	UseBasicAuth:    bool
	PubKeyField:     string
	SecretKeyField:  string
	SigningAlgorithm: string
	SigningFormat:    string
	SigningSecretField: string

	// Outbound request signing (Ikra X-Signature, Pacepay hash in body)
	RequestSigning: *null | #TemplateRequestSigning

	// Runtime policies (from payout_runtime, check_status_*, init_payout_policy)
	PayoutRuntime:               *null | #TemplatePayoutRuntime
	InitPayoutPolicy:            *null | {
		MapStatusFromResponse: bool
		ForeignIDStrategy:     string
		ClientUUIDField:       string
	}
	CheckStatusForeignIDEmpty:   string // error | declined | error_status
	CheckStatusConfig:           *null | #TemplateCheckStatusConfig
	CheckStatusPeriod:           string
	CheckStatusByTxType:         bool

	// Wire formats
	ResponseFormat: string // json | xml
	CallbackFormat: string // json | form-urlencoded
	ResponseLoggingMode: string

	// Endpoints (const names + paths emitted to datatypes.go)
	Endpoints: [...{
		ConstName:      string
		Path:           string
		Method:         string
		ContentType:    string
		PathSecretKey:  string // secret key substituted into %s (Pacepay)
	}]

	PayinEndpointConst:        string
	PayoutEndpointConst:       string
	PayinStatusEndpointConst:  string
	PayoutStatusEndpointConst: string
	RefundEndpointConst:       string

	// Auto requests
	PayinRequest:  *null | #TemplateRequestDef
	PayoutRequest: *null | #TemplateRequestDef
	P2PRequest:    *null | #TemplateRequestDef
	RefundRequest: *null | #TemplateRequestDef

	// Legacy / manual types
	PayinRequestType:    string
	PayoutRequestType:   string
	RefundRequestType:   string
	PayinResponseType:   string
	PayoutResponseType:  string
	RefundResponseType:  string
	PayinForeignIDField:   string
	PayoutForeignIDField:  string
	RefundForeignIDField:  string

	// CheckStatus response parsing
	PayinStatusResponseType:  string
	PayoutStatusResponseType: string
	PayinStatusField:         string
	PayoutStatusField:        string

	// Status maps
	PayinStatuses:       [...]
	PayoutStatuses:      [...]
	PayoutStatusesExtra: [...]
	StatusDetails:       [...]
	ErrorCodes:          [...]
	ErrorMappingMatrix:  [...]
	TnxStatusVars:       [...]

	// Callback
	CallbackSignature:     *null | #TemplateCallbackSignature
	CallbackTxIDField:     string
	CallbackForeignIDField: string
	CallbackStatusField:   string
	CallbackErrorCodeField: string
	CallbackMessageField:  string
	CallbackReturnCodeField: string
	CallbackFields:        [...]
	CallbackReturnQueryTxIDParam: string
	CallbackReturnQueryStatusValue: string
	CallbackReturnQueryInfoCallback: bool

	// Macan / P2P extras
	PaymentMethodMap: [...]
	SupportedMethods:   [...]
	SupportedCurrencies: [...]
	CurrencyCode:       string

	// Operations DSL
	Operations: [...#TemplateOperation]

	// Optional interface deps
	HasTDSRedirector:         bool
	HasOTPFormRedirector:     bool
	HasPaymentMethodSelector: bool
	ConstructorDeps:          [...]

	// Policy configs (pass-through for advanced templates)
	RuntimePolicyConfig:          _
	Async3DSConfig:               _
	CrossInstanceStateConfig:     _
	StatusConfirmationConfig:     _
	CheckStatusThrottleConfig:    _
	ExtendedCallbackConfig:       _
	OTPConfig:                    _
	OTPHandlesExternally:         bool

	ExtraImports: [...string]
}

// Live field-source mapping is the switch in resolve.go, kept in lockstep with
// #CatalogFieldSource by TestCatalogFieldSourcesMatchResolve. Do not revive a
// parallel go_expr catalog here — it drifted (duplicate keys, incomplete coverage).

// --- Ang implementation checklist (compiler/paymentprovider/emit) ---
//
// 1. Extend TemplateData struct per #TemplateData above (json tags = template dot paths).
// 2. Map #Provider → TemplateData in buildTemplateData():
//    - api_compat → UseMacanP2P / UsePaytechGateway / UseFluxsgate only (bundled alternate template sets)
//    - secrets.use_labels → SecretUseLabels
//    - nullable blocks: keep nil when CUE value is null (templates use {{ if .PayoutRuntime }})
// 3. Field sources: #CatalogFieldSource ↔ resolve.go (TestCatalogFieldSourcesMatchResolve).
// 4. Durations: check_status_config.since_created_period "10m" → CheckStatusPeriod "10 * time.Minute".
// 5. Response types: set PayoutStatusField from response_types status field name.
// 6. Pick creds template: SecretUseLabels → creds_macan.go.tmpl else creds.go.tmpl.
// 7. Templates branch on schema fields (RequestSigning.Format, ResponseFormat, PayoutRuntime, …) — not per-provider flags.
// 8. SecretPart.type → TemplateSecretPart.Type for bool parts (Ikra returnRecipientDetails).
