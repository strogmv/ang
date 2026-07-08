package paymentprovider

// ProviderSpec is the decoded CUE value at provider: schema.#Provider & {...}.
type ProviderSpec struct {
	PackageName     string `json:"package_name"`
	SID             string `json:"sid"`
	Source          string `json:"source"`
	Label           string `json:"label"`
	MIDPrefix       string `json:"mid_prefix"`
	StructName      string `json:"struct_name"`
	ConstructorName string `json:"constructor_name"`
	PaymentSource   string `json:"payment_source"`

	Secrets struct {
		Format    string       `json:"format"`
		Separator string       `json:"separator"`
		Parts     []SecretPart `json:"parts"`
		UseLabels bool         `json:"use_labels"`
	} `json:"secrets"`

	RequestSigning *RequestSigningConfig `json:"request_signing"`

	InitPayoutPolicy *InitPayoutPolicy `json:"init_payout_policy"`

	PayoutRuntime   *PayoutRuntimeConfig   `json:"payout_runtime"`
	CallbackRuntime *CallbackRuntimeConfig `json:"callback_runtime"`

	CheckStatusForeignIDEmpty string `json:"check_status_foreign_id_empty"`

	ResponseFormat string `json:"response_format"`
	CallbackFormat string `json:"callback_format"`

	ResponseEnvelope *ResponseEnvelopeConfig `json:"response_envelope"`

	Currency struct {
		Code    string `json:"code"`
		ISONum  int    `json:"iso_num"`
		Country string `json:"country"`
	} `json:"currency"`

	Endpoints map[string]Endpoint `json:"endpoints"`

	Signing struct {
		Algorithm string `json:"algorithm"`
		Format    string `json:"format"`
	} `json:"signing"`

	Auth *AuthConfig `json:"auth"`

	CallbackSignature *CallbackSignatureConfig `json:"callback_signature"`

	PayinStatuses      []StatusMapping         `json:"payin_statuses"`
	PayoutStatuses     []StatusMapping         `json:"payout_statuses"`
	ErrorCodes         []ErrorMapping          `json:"error_codes"`
	ErrorMappingMatrix []ErrorMatrixEntry      `json:"error_mapping_matrix"`
	StatusDetails      []ErrorMapping          `json:"status_details"`
	PaymentMethodMap   []PaymentMethodMapEntry `json:"payment_method_map"`

	PayinRequest        *RequestDef `json:"payin_request"`
	PayoutRequest       *RequestDef `json:"payout_request"`
	PayinStatusRequest  *RequestDef `json:"payin_status_request"`
	PayoutStatusRequest *RequestDef `json:"payout_status_request"`
	P2PRequest          *RequestDef `json:"p2p_request"`
	RefundRequest       *RequestDef `json:"refund_request"`

	ResponseTypes []ResponseType `json:"response_types"`

	SupportedMethods    []string `json:"supported_methods"`
	SupportedCurrencies []string `json:"supported_currencies"`

	HasPayin        bool `json:"has_payin"`
	HasPayout       bool `json:"has_payout"`
	HasP2P          bool `json:"has_p2p"`
	HasSubscription bool `json:"has_subscription"`
	HasRefund       bool `json:"has_refund"`
	HasCancel       bool `json:"has_cancel"`

	AuthFlow  string `json:"auth_flow"`
	APICompat string `json:"api_compat"`

	ResponseLoggingMode string `json:"response_logging_mode"`

	Interfaces struct {
		BalanceFetcher        bool `json:"balance_fetcher"`
		MobileProcessor       bool `json:"mobile_processor"`
		OTPFormRedirector     bool `json:"otp_form_redirector"`
		TDSRedirector         bool `json:"tds_redirector"`
		PaymentMethodSelector bool `json:"payment_method_selector"`
		CustomerRandomization bool `json:"customer_randomization"`
	} `json:"interfaces"`

	ConstructorDeps []ConstructorDep `json:"constructor_deps"`

	Async3DSConfig *struct {
		Enabled                   bool   `json:"enabled"`
		AutoChargeAfter           string `json:"auto_charge_after"`
		FinishCallbackWaitTimeout string `json:"finish_callback_wait_timeout"`
		StalePendingGrace         string `json:"stale_pending_grace"`
	} `json:"async_3ds_config"`

	CrossInstanceStateConfig *struct {
		Enabled bool   `json:"enabled"`
		Backend string `json:"backend"`
	} `json:"cross_instance_state_config"`

	StatusConfirmationConfig *struct {
		Enabled          bool     `json:"enabled"`
		Strategy         string   `json:"strategy"`
		RetryNotReady    bool     `json:"retry_not_ready"`
		NotReadyPatterns []string `json:"not_ready_patterns"`
	} `json:"status_confirmation_config"`

	CheckStatusThrottleConfig *struct {
		Enabled bool   `json:"enabled"`
		Period  string `json:"period"`
	} `json:"check_status_throttle_config"`

	ExtendedCallbackConfig *struct {
		Enabled bool     `json:"enabled"`
		Fields  []string `json:"fields"`
	} `json:"extended_callback_config"`

	RuntimePolicyConfig *struct {
		Timeouts struct {
			RequestTimeout      string `json:"request_timeout"`
			CheckStatusTimeout  string `json:"check_status_timeout"`
			CallbackWaitTimeout string `json:"callback_wait_timeout"`
		} `json:"timeouts"`
		Retries struct {
			MaxAttempts      int    `json:"max_attempts"`
			InitialBackoff   string `json:"initial_backoff"`
			MaxBackoff       string `json:"max_backoff"`
			RetryOnNotFound  bool   `json:"retry_on_not_found"`
			RetryOn5xx       bool   `json:"retry_on_5xx"`
			RetryOnRateLimit bool   `json:"retry_on_rate_limit"`
		} `json:"retries"`
		Limits struct {
			MaxCallbackBodyBytes int    `json:"max_callback_body_bytes"`
			MaxPendingAge        string `json:"max_pending_age"`
		} `json:"limits"`
	} `json:"runtime_policy_config"`

	Operations []OperationDef `json:"operations"`

	OTPConfig *struct {
		HandlesExternally bool `json:"handles_externally"`
	} `json:"otp_config"`

	CheckStatusConfig *CheckStatusConfigSpec `json:"check_status_config"`

	Callback *CallbackConfig `json:"callback"`

	ExtraImports []string `json:"extra_imports"`
}

type SecretPart struct {
	Name      string `json:"name"`
	Key       string `json:"key"`
	Optional  bool   `json:"optional"`
	Transform string `json:"transform"`
	Type      string `json:"type"`
}

type CheckStatusConfigSpec struct {
	SinceCreatedPeriod  string `json:"since_created_period"`
	ByTransactionType   bool   `json:"by_transaction_type"`
	PathSuffixForeignID bool   `json:"path_suffix_foreign_id"`
}

type PayoutRuntimeConfig struct {
	ForeignIDOnUnexpectedError bool `json:"foreign_id_on_unexpected_error"`
}

type CallbackRuntimeConfig struct {
	FinishViaCheckStatus bool `json:"finish_via_check_status"`
}

type InitPayoutPolicy struct {
	MapStatusFromResponse bool   `json:"map_status_from_response"`
	ForeignIDStrategy     string `json:"foreign_id_strategy"`
	ClientUUIDField       string `json:"client_uuid_field"`
}

type ResponseEnvelopeConfig struct {
	Enabled      bool   `json:"enabled"`
	WrapperField string `json:"wrapper_field"`
	SuccessField string `json:"success_field"`
	ErrorField   string `json:"error_field"`
}

type RequestSigningConfig struct {
	Algorithm      string   `json:"algorithm"`
	Format         string   `json:"format"`
	Header         string   `json:"header"`
	SecretKey      string   `json:"secret_key"`
	UsernameHeader string   `json:"username_header"`
	UsernameKey    string   `json:"username_key"`
	Encoding       string   `json:"encoding"`
	ConcatFields   []string `json:"concat_fields"`
}

type Endpoint struct {
	Path          string `json:"path"`
	Method        string `json:"method"`
	ContentType   string `json:"content_type"`
	PathSecretKey string `json:"path_secret_key"`
}

type StatusMapping struct {
	Code       any    `json:"code"` // int or string
	Status     string `json:"status"`
	StatusCode string `json:"status_code"`
	Message    string `json:"message"`
}

type ErrorMapping struct {
	Code       string `json:"code"`
	Status     string `json:"status"`
	StatusCode string `json:"status_code"`
}

type ErrorMatrixEntry struct {
	Major      any    `json:"major"`
	Minor      any    `json:"minor"`
	Status     string `json:"status"`
	StatusCode string `json:"status_code"`
	Message    string `json:"message"`
}

type PaymentMethodMapEntry struct {
	Brand             string `json:"brand"`
	Match             string `json:"match"`
	APIName           string `json:"api_name"`
	CurrencyOverrides []struct {
		Currency string `json:"currency"`
		APIName  string `json:"api_name"`
	} `json:"currency_overrides"`
}

type RequestDef struct {
	Name   string         `json:"name"`
	Fields []RequestField `json:"fields"`
}

type RequestField struct {
	Name      string `json:"name"`
	JSON      string `json:"json"`
	Type      string `json:"type"`
	Source    string `json:"source"`
	SecretKey string `json:"secret_key"`
	Default   string `json:"default"`
	ConstVal  string `json:"const_val"`
	ConstName string `json:"const_name"`
	OmitEmpty bool   `json:"omitempty"`
	Redacted  bool   `json:"redacted"`
	OwnerKey  string `json:"owner_key"`
	OwnerFrom string `json:"owner_from"`
}

type ResponseType struct {
	Name   string        `json:"name"`
	Fields []StructField `json:"fields"`
}

type StructField struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	JSON      string `json:"json"`
	OmitEmpty bool   `json:"omitempty"`
	Omitempty bool   // template alias (datatypes.go.tmpl uses .Omitempty)
}

type CallbackConfig struct {
	TxIDField               string        `json:"tx_id_field"`
	ForeignIDField          string        `json:"foreign_id_field"`
	StatusField             string        `json:"status_field"`
	StatusType              string        `json:"status_type"`
	ErrorCodeField          string        `json:"error_code_field"`
	ReturnQueryTxIDParam    string        `json:"return_query_txid_param"`
	ReturnQueryStatusValue  string        `json:"return_query_status_value"`
	ReturnQueryInfoCallback bool          `json:"return_query_info_callback"`
	Fields                  []StructField `json:"fields"`
}

type AuthConfig struct {
	Type        string `json:"type"`
	Header      string `json:"header"`
	SecretKey   string `json:"secret_key"`
	ContentType string `json:"content_type"`
	Prefix      string `json:"prefix"`
	Masked      bool   `json:"masked"`
}

type CallbackSignatureField struct {
	JSON        string `json:"json"`
	ConstKey    string `json:"const_key"`
	SecretKey   string `json:"secret_key"`
	OmitIfEmpty bool   `json:"omit_if_empty"`
	Format      string `json:"format"`
}

type CallbackSignatureConfig struct {
	Algorithm     string                   `json:"algorithm"`
	SecretKey     string                   `json:"secret_key"`
	Format        string                   `json:"format"`
	Fields        []CallbackSignatureField `json:"fields"`
	Compare       string                   `json:"compare"`
	Optional      bool                     `json:"optional"`
	Header        string                   `json:"header"`
	UsernameKey   string                   `json:"username_key"`
	SignatureJSON string                   `json:"signature_json"`
}

type ConstructorDep struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Pkg  string `json:"pkg"`
}

type OperationDef struct {
	Kind      string             `json:"kind"`
	Transport OperationTransport `json:"transport"`
}

type OperationTransport struct {
	Endpoint              string `json:"endpoint"`
	EndpointPath          string `json:"endpoint_path"`
	RequestType           string `json:"request_type"`
	ResponseType          string `json:"response_type"`
	ErrorResponseType     string `json:"error_response_type"`
	ResponseLoggingMode   string `json:"response_logging_mode"`
	StatusStrategy        string `json:"status_strategy"`
	RetryMaxAttempts      int    `json:"retry_max_attempts"`
	RetryInitialBackoff   string `json:"retry_initial_backoff"`
	RetryMaxBackoff       string `json:"retry_max_backoff"`
	Timeout               string `json:"timeout"`
	PendingCallbackAction string `json:"pending_callback_action"`
	StatusField           string `json:"status_field"`
	StatusDetailsField    string `json:"status_details_field"`
	ErrorCodeField        string `json:"error_code_field"`
}
