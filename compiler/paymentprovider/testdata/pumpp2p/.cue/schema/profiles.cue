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
