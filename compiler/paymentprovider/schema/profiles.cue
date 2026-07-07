package schema

// Reusable provider profile VALUES (not schema intersections — avoids CUE disjunction issues).
// Compose: provider: schema.#Provider & schema.ProfileMacanP2P & { ... deltas ... }

// Shared Macan REST transport defaults.
#MacanTransportBase: #OperationTransport & {
	response_logging_mode: "prefer_parsed"
	status_strategy:       "macan"
	error_response_type:   "macanAPIErrorResponse"
}

#MacanCheckStatusTransport: #MacanTransportBase & {
	retry_max_attempts:    3
	retry_initial_backoff: "300ms"
	retry_max_backoff:     "2s"
	timeout:               "12s"
}

MacanDefaultOps: [
	{
		kind: "init_payout"
		transport: #MacanTransportBase & {
			endpoint:             "payout"
			request_type:         "macanPayOutRequest"
			response_type:        "macanPayOutResponse"
			status_field:         "payOut.status"
			status_details_field: "payOut.statusDetails"
		}
	},
	{
		kind: "check_status_payin"
		transport: #MacanCheckStatusTransport & {
			endpoint:             "payin_status"
			response_type:        "macanPayInResponse"
			status_field:         "payIn.status"
			status_details_field: "payIn.statusDetails"
		}
	},
	{
		kind: "check_status_payout"
		transport: #MacanCheckStatusTransport & {
			endpoint:             "payout_status"
			response_type:        "macanPayOutResponse"
			status_field:         "payOut.status"
			status_details_field: "payOut.statusDetails"
		}
	},
	{
		kind: "init_pay_p2p"
		transport: #MacanTransportBase & {
			endpoint:             "payin"
			request_type:         "macanPayInRequest"
			response_type:        "macanPayInResponse"
			status_field:         "payIn.status"
			status_details_field: "payIn.statusDetails"
		}
	},
	{
		kind: "cancel_pay"
		transport: #MacanTransportBase & {
			endpoint:      "payin"
			request_type:  "httpcli.DummyReq"
			response_type: "macanPayInResponse"
			status_field:  "payIn.status"
		}
	},
	{
		kind: "fetch_balances"
		transport: {
			endpoint:              "balance"
			response_type:         "macanBalanceResponse"
			error_response_type:   "macanAPIErrorResponse"
			status_strategy:       "macan"
			response_logging_mode: "inherit"
		}
	},
]

MacanAuth: #AuthConfig & {
	type:         "header_token"
	header:       "Authorization-Token"
	secret_key:   "apiToken"
	content_type: "application/json"
}

MacanCallbackSignature: #CallbackSignatureConfig & {
	algorithm:  "sha1"
	secret_key: "signatureKey"
	format:     "sorted_kv_pipe"
	compare:    "equal_fold"
	fields: [
		{json: "amount", format: "float_trailing_zero"},
		{json: "external_id"},
		{json: "id"},
		{json: "status"},
		{json: "status_details"},
		{const_key: "merchantSignatureKey", secret_key: "signatureKey"},
	]
}

ProfileMacanP2P: {
	auth_flow:             "p2p"
	api_compat:            "macan_p2p"
	response_logging_mode: "prefer_parsed"
	has_payin:             false
	has_payout:            true
	has_p2p:               true
	has_cancel:            true
	signing: {
		algorithm: "none"
		format:    "custom"
	}
	auth:               MacanAuth
	callback_signature: MacanCallbackSignature
	runtime_policy_config: {
		timeouts: {
			request_timeout:      "30s"
			check_status_timeout: "15s"
		}
		retries: {
			max_attempts:        2
			initial_backoff:     "500ms"
			max_backoff:         "3s"
			retry_on_not_found:  false
			retry_on_5xx:        true
			retry_on_rate_limit: true
		}
		limits: {
			max_callback_body_bytes: 1048576
			max_pending_age:         "4m"
		}
	}
	p2p_request:    null
	payout_request: null
}

PaytechGatewayAuth: #AuthConfig & {
	type:         "bearer"
	header:       "Authorization"
	secret_key:   "apiKey"
	content_type: "application/json"
}

PaytechGatewayCallbackSignature: #CallbackSignatureConfig & {
	algorithm:  "hmac-sha256"
	secret_key: "signingKey"
	format:     "hmac_body"
	compare:    "equal"
	fields: [{json: "referenceId"}]
}

ProfilePaytechGateway: {
	auth_flow:             "h2h"
	api_compat:            "paytech_gateway"
	response_logging_mode: "prefer_parsed"
	has_payin:             true
	has_refund:            true
	has_payout:            false
	has_p2p:               false
	signing: {
		algorithm: "none"
		format:    "custom"
	}
	auth:               PaytechGatewayAuth
	callback_signature: PaytechGatewayCallbackSignature
	interfaces: {
		tds_redirector: true
	}
	endpoints: {
		payin: {path: "/api/v1/payments", method: "POST"}
	}
	callback: {
		tx_id_field:      "ReferenceID"
		foreign_id_field: "ID"
		status_field:     "State"
		status_type:      "string"
		fields: [
			{name: "ID", type: "string", json: "id"},
			{name: "ReferenceID", type: "string", json: "referenceId"},
			{name: "State", type: "string", json: "state"},
			{name: "ErrorCode", type: "string", json: "errorCode"},
			{name: "RRN", type: "string", json: "rrn"},
		]
	}
}

FluxsgateAuth: #AuthConfig & {
	type:         "bearer"
	header:       "Authorization"
	secret_key:   "merchantPrivateKey"
	content_type: "application/json"
}

ProfileFluxsgate: {
	auth_flow:             "h2h"
	api_compat:            "fluxsgate"
	response_logging_mode: "prefer_parsed"
	payment_source:        "card"
	has_payin:             true
	has_refund:            true
	has_payout:            false
	has_p2p:               false
	signing: {
		algorithm: "none"
		format:    "custom"
	}
	auth: FluxsgateAuth
	interfaces: {
		tds_redirector: true
	}
	endpoints: {
		payin:        {path: "/api/v1/payments", method: "POST"}
		payin_status: {path: "/api/v1/payments", method: "GET"}
		refund:       {path: "/api/v1/refunds", method: "POST"}
	}
	callback: {
		tx_id_field:      "OrderNumber"
		foreign_id_field: "Token"
		status_field:     "Status"
		status_type:      "string"
		fields: [
			{name: "Token", type: "string", json: "token"},
			{name: "Type", type: "string", json: "type"},
			{name: "Status", type: "string", json: "status"},
			{name: "ExtraReturnParam", type: "string", json: "extraReturnParam"},
			{name: "OrderNumber", type: "string", json: "orderNumber"},
			{name: "WalletToken", type: "string", json: "walletToken"},
			{name: "RecurringToken", type: "string", json: "recurringToken"},
			{name: "SanitizedMask", type: "string", json: "sanitizedMask"},
			{name: "Amount", type: "string", json: "amount"},
			{name: "Currency", type: "string", json: "currency"},
			{name: "GatewayAmount", type: "string", json: "gatewayAmount"},
			{name: "GatewayCurrency", type: "string", json: "gatewayCurrency"},
			{name: "InitialAmount", type: "string", json: "initAmount"},
			{name: "RRN", type: "string", json: "rrn"},
			{name: "ARN", type: "string", json: "arn"},
		]
	}
	payin_statuses: [
		{code: "init", status: "pending", status_code: "SCodeOk"},
		{code: "pending", status: "pending", status_code: "SCodeOk"},
		{code: "approved", status: "success", status_code: "SCodeOk"},
		{code: "declined", status: "declined", status_code: "SCodeDeclinedByBank"},
		{code: "expired", status: "declined", status_code: "SCodeTimeouted"},
	]
	// Fluxsgate customer.email is required — use providers.GetParameter in InitPay
	// (see buildCustomerRequest in templates/fluxsgate/provider.go.tmpl).
	payout_statuses:  []
	error_codes:      []
	supported_methods: ["card"]
}

// --- Nebeus MX-7 (card payout, JSON, optional HMAC-SHA512 webhook) ---
// Hand-written: payment_providers/nebeus_mx7/

NebeusAuth: #AuthConfig & {
	type:         "header_token"
	header:       "x-api-key"
	secret_key:   "apiKey"
	content_type: "application/json"
	masked:       true
}

NebeusCallbackSignature: #CallbackSignatureConfig & {
	algorithm:  "hmac-sha512"
	secret_key: "webhookSecret"
	format:     "hmac_body"
	compare:    "equal"
	header:     "X-Signature"
	optional:   true
	fields:     []
}

ProfileNebeusPayout: {
	auth_flow:             "h2h"
	api_compat:            "nebeus_payout"
	response_logging_mode: "prefer_parsed"
	payment_source:        "card"
	has_payin:             false
	has_payout:            true
	has_p2p:               false
	signing: {
		algorithm: "none"
		format:    "custom"
	}
	auth:               NebeusAuth
	callback_signature: NebeusCallbackSignature
	check_status_config: {
		since_created_period:   "10m"
		path_suffix_foreign_id: true
	}
	payout_runtime: {
		foreign_id_on_unexpected_error: true
	}
	callback_runtime: {
		finish_via_check_status: true
	}
	check_status_foreign_id_empty: "error_status"
	init_payout_policy: {
		map_status_from_response: true
		foreign_id_strategy:      "coalesce"
		client_uuid_field:        "payoutId"
	}
	endpoints: {
		payout:        {path: "/api/beta/payouts/b2p", method: "POST"}
		payout_status: {path: "/api/beta/payouts/status", method: "GET"}
	}
	payout_request: {
		name: "createPayoutRequest"
		fields: [
			{name: "PayoutID", json: "payoutId", source: "client_payout_id"},
			{name: "PAN", json: "pan", source: "card_pan", redacted: true},
			{name: "CardHolderName", json: "cardHolderName", source: "card_holder_name"},
			{name: "ExpirationDate", json: "expirationDate", source: "card_exp_last_day", omitempty: true},
			{name: "Currency", json: "currency", source: "tx_currency"},
			{name: "Amount", json: "amount", type: "float64", source: "tx_amount_float"},
			{name: "Details", json: "details", source: "description_payout"},
			{name: "FirstName", json: "firstName", source: "first_name", omitempty: true},
			{name: "LastName", json: "lastName", source: "last_name", omitempty: true},
			{name: "Email", json: "email", source: "owner_info", owner_key: "email", owner_from: "card", omitempty: true},
			{name: "BirthDate", json: "birthDate", source: "owner_info", owner_key: "birth_date", owner_from: "card", omitempty: true},
			{name: "Country", json: "country", source: "owner_info", owner_key: "country", owner_from: "card", omitempty: true},
			{name: "IPAddress", json: "ipAddress", source: "tx_ip", omitempty: true},
			{name: "ExternalClientID", json: "externalClientId", source: "tx_id", omitempty: true},
			{name: "WebhookUrl", json: "webhookUrl", source: "tx_callback_url", omitempty: true},
		]
	}
	payout_statuses: [
		{code: "OrderAccepted", status: "success", status_code: "SCodeOk"},
		{code: "AMLCheckFailed", status: "declined", status_code: "SCodeDeclinedByBank"},
		{code: "TransactionCheckFailed", status: "declined", status_code: "SCodeDeclinedByBank"},
		{code: "CurrencyCheckFailed", status: "declined", status_code: "SCodeDeclinedByBank"},
		{code: "AvailableAmountCheckFailed", status: "declined", status_code: "SCodeDeclinedByBank"},
		{code: "LimitsCheckFailed", status: "declined", status_code: "SCodeDeclinedByBank"},
		{code: "OtherFinancialCheckFailed", status: "declined", status_code: "SCodeDeclinedByBank"},
		{code: "CreatingOrderFailed", status: "declined", status_code: "SCodeDeclinedByBank"},
		{code: "GatewayExecutionFailed", status: "declined", status_code: "SCodeDeclinedByBank"},
		{code: "GatewayCallbackFailed", status: "declined", status_code: "SCodeDeclinedByBank"},
		{code: "ExecutingOrderFailed", status: "declined", status_code: "SCodeDeclinedByBank"},
		{code: "CheckCardBinFailed", status: "declined", status_code: "SCodeDeclinedByBank"},
		{code: "CountryCheckFailed", status: "declined", status_code: "SCodeDeclinedByBank"},
	]
	payin_statuses: []
	error_codes:    []
}

// --- Ikra N-146 (invoice API: P2P pay-in + card payout, request signing) ---
// Universal generic templates: request_signing method_url_body, notification token callback.
// Hand-written reference: payment_providers/ikra/

IkraAuthIdentity: #AuthConfig & {
	type:         "header_token"
	header:       "X-Identity"
	secret_key:   "apiKey"
	content_type: "application/json"
	masked:       true
}

ProfileIkraInvoice: {
	auth_flow:             "p2p"
	api_compat:            "ikra_invoice"
	response_logging_mode: "prefer_parsed"
	payment_source:        "both"
	has_payin:             false
	has_payout:            true
	has_p2p:               true
	signing: {
		algorithm: "none"
		format:    "custom"
	}
	auth: IkraAuthIdentity
	request_signing: {
		algorithm:  "hmac-sha1"
		format:     "method_url_body"
		header:     "X-Signature"
		secret_key: "secret"
		encoding:   "base64"
	}
	callback_signature: #CallbackSignatureConfig & {
		algorithm:  "hmac-sha256"
		secret_key: "secret"
		format:     "notification_token"
		header:     "X-Notification-Token"
		compare:    "equal"
		fields:     []
	}
	check_status_config: {
		path_suffix_foreign_id: true
	}
	check_status_foreign_id_empty: "declined"
	endpoints: {
		payin:           {path: "/api/merchant/invoices", method: "POST"}
		payout:          {path: "/api/merchant/invoices", method: "POST"}
		payin_status:    {path: "/api/merchant/invoices", method: "GET"}
		payout_status:   {path: "/api/merchant/invoices", method: "GET"}
		payment_methods: {path: "/api/merchant/payment-methods", method: "GET"}
	}
	secrets: {
		format:      "API Key:Secret:Return Recipient Details (optional)"
		separator:   ":"
		use_labels:  true
		parts: [
			{name: "API Key", key: "apiKey"},
			{name: "Secret", key: "secret"},
			{name: "Return Recipient Details", key: "returnRecipientDetails", optional: true, type: "bool"},
		]
	}
	interfaces: {
		tds_redirector: true
	}
	payin_statuses: [
		{code: "new", status: "pending", status_code: "SCodeOk"},
		{code: "transfer_waiting", status: "pending", status_code: "SCodeOk"},
		{code: "paid", status: "success", status_code: "SCodeOk"},
		{code: "canceled", status: "declined", status_code: "SCodeDeclinedByBank"},
		{code: "expired", status: "declined", status_code: "SCodeTimeouted"},
		{code: "dispute", status: "declined", status_code: "SCodeDeclinedByBank"},
	]
	payout_statuses: [
		{code: "paid", status: "success", status_code: "SCodeOk"},
		{code: "canceled", status: "declined", status_code: "SCodeDeclinedByBank"},
		{code: "expired", status: "declined", status_code: "SCodeDeclinedByBank"},
		{code: "dispute", status: "declined", status_code: "SCodeDeclinedByBank"},
	]
	error_codes: []
}

// --- Pacepay MX-757 (card payout, XML responses, MD5 request hash, form callbacks) ---
// Universal generic templates: response_format xml, request_signing md5_fields_concat, path_secret_key.
// Hand-written reference: payment_providers/pacepay/

ProfilePacepay: {
	auth_flow:             "h2h"
	api_compat:            "pacepay"
	response_logging_mode: "raw"
	response_format:       "xml"
	callback_format:       "form-urlencoded"
	payment_source:        "card"
	has_payin:             false
	has_payout:            true
	has_p2p:               false
	signing: {
		algorithm: "md5"
		format:    "md5_fields_concat"
	}
	request_signing: {
		algorithm:  "md5"
		format:     "md5_fields_concat"
		secret_key: "secretKey"
		encoding:   "hex"
		concat_fields: ["id", "dt", "amount", "phone", "client", "destination"]
	}
	secrets: {
		format:     "Service ID:Secret Key"
		separator:  ":"
		use_labels: true
		parts: [
			{name: "Service ID", key: "serviceID"},
			{name: "Secret Key", key: "secretKey"},
		]
	}
	interfaces: {
		balance_fetcher: true
	}
	check_status_foreign_id_empty: "error_status"
	endpoints: {
		payout:  {path: "/partner/%s/payout", method: "POST", content_type: "application/json", path_secret_key: "serviceID"}
		check:   {path: "/partner/%s/check", method: "POST", content_type: "application/x-www-form-urlencoded", path_secret_key: "serviceID"}
		balance: {path: "/partner/%s/balance", method: "POST", content_type: "application/x-www-form-urlencoded", path_secret_key: "serviceID"}
	}
	payout_statuses: [
		{code: "PAID", status: "success", status_code: "SCodeOk"},
		{code: "NOT_PAID", status: "declined", status_code: "SCodeDeclinedByBank"},
		{code: "ERROR", status: "error", status_code: "SCodeInternalError"},
		{code: "NOT FOUND", status: "pending", status_code: "SCodeOk"},
		{code: "PROCESSING", status: "pending", status_code: "SCodeOk"},
	]
	payin_statuses: []
	error_codes:    []
}
