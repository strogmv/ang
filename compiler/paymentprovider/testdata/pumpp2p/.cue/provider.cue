package provider

import "transferty.local/pumpp2p/schema"

// PumpP2P (Macan) REST API — see swagger.yaml in this package.
// Staging: https://api.macanp2p.xyz  Production: https://api.xforge.io

provider: schema.#Provider & schema.ProfileMacanP2P & {
	package_name: "pumpp2p"
	sid:          "pumpp2p"
	source:       "PPPumpP2P"
	label:        "PumpP2P"
	mid_prefix:   "PUMP"

	struct_name:      "PPPumpP2P"
	constructor_name: "NewPPPumpP2P"

	operations: schema.MacanDefaultOps

	secrets: {
		format:    "AccessToken:SignatureKey[:ReturnRecipientDetails]"
		separator: ":"
		parts: [
			{name: "Access Token", key: "apiToken"},
			{name: "Signature Key", key: "signatureKey"},
			{name: "Return Recipient Details (true/false, optional)", key: "returnRecipientDetails"},
		]
	}

	currency: {
		code:    "RUB"
		iso_num: 643
		country: "RU"
	}

	supported_currencies: ["RUB", "KGS", "UZS"]

	endpoints: {
		payin:         {path: "/api/v1/pay_ins", method: "POST"}
		payout:        {path: "/api/v1/pay_outs", method: "POST"}
		payin_status:  {path: "/api/v1/pay_ins", method: "GET"}
		payout_status: {path: "/api/v1/pay_outs", method: "GET"}
		balance:       {path: "/api/v1/merchants/balance", method: "GET"}
	}

	interfaces: {
		balance_fetcher:         true
		mobile_processor:        false
		otp_form_redirector:     false
		tds_redirector:          true
		payment_method_selector: false
		customer_randomization:  true
	}

	constructor_deps: [
		{name: "tdsRedirector", type: "model.TDSRedirector", pkg: "gitlab.q-tech.host/transferty/backend/tnx_processor/model"},
		{name: "txPathLogger", type: "model.TxPathLogger", pkg:  "gitlab.q-tech.host/transferty/backend/tnx_processor/model"},
	]

	supported_methods: [
		"cardp2p",
		"sbp",
		"trans_sbp",
		"trans_card2card",
		"click",
		"qrcode",
	]

	payin_statuses: [
		{code: "created", status: "pending", status_code: "SCodeOk"},
		{code: "requisites", status: "pending", status_code: "SCodeOk"},
		{code: "customer_confirm", status: "pending", status_code: "SCodeOk"},
		{code: "trader_confirm", status: "pending", status_code: "SCodeOk"},
		{code: "appellation", status: "pending", status_code: "SCodeOk"},
		{code: "completed", status: "success", status_code: "SCodeOk"},
		{code: "cancelled", status: "declined", status_code: "SCodeDeclinedByBank"},
	]

	payout_statuses: [
		{code: "created", status: "pending", status_code: "SCodeOk"},
		{code: "in_progress", status: "pending", status_code: "SCodeOk"},
		{code: "pending_approval", status: "pending", status_code: "SCodeOk"},
		{code: "appellation", status: "pending", status_code: "SCodeOk"},
		{code: "completed", status: "success", status_code: "SCodeOk"},
		{code: "cancelled", status: "declined", status_code: "SCodeDeclinedByBank"},
	]

	error_codes: [
		{code: "M10000", status: "declined", status_code: "SCodeSuspended"},
		{code: "M10001", status: "declined", status_code: "SCodeDeclinedByBank"},
		{code: "M10003", status: "declined", status_code: "SCodeDeclinedByBank"},
		{code: "M10004", status: "declined", status_code: "SCodeLimitReached"},
		{code: "P10000", status: "declined", status_code: "SCodeDeclinedByBank"},
		{code: "P10005", status: "declined", status_code: "SCodeRequisiteNotAvailable"},
		{code: "P10011", status: "declined", status_code: "SCodeDeclinedByBank"},
	]

	status_details: [
		{code: "connection_is_lost", status: "error", status_code: "SCodeInternalError"},
		{code: "no_payment", status: "declined", status_code: "SCodeDeclinedByBank"},
		{code: "different_amount", status: "declined", status_code: "SCodeDeclinedByBank"},
		{code: "trader_confirm_timeout", status: "error", status_code: "SCodeTimeouted"},
		{code: "customer_confirm_timeout", status: "error", status_code: "SCodeTimeouted"},
		{code: "requisites_timeout", status: "declined", status_code: "SCodeRequisiteNotAvailable"},
		{code: "new_timeout", status: "error", status_code: "SCodeTimeouted"},
		{code: "not_in_progress", status: "declined", status_code: "SCodeDeclinedByBank"},
		{code: "not_pending_approval", status: "declined", status_code: "SCodeDeclinedByBank"},
		{code: "merchant", status: "pending", status_code: "SCodeOk"},
		{code: "admin", status: "pending", status_code: "SCodeOk"},
		{code: "trader", status: "pending", status_code: "SCodeOk"},
		{code: "customer", status: "pending", status_code: "SCodeOk"},
		{code: "operator", status: "pending", status_code: "SCodeOk"},
		{code: "support", status: "pending", status_code: "SCodeOk"},
		{code: "admin_created", status: "pending", status_code: "SCodeOk"},
		{code: "revert_cancelled", status: "declined", status_code: "SCodeDeclinedByBank"},
	]

	payment_method_map: [
		{brand: "cardp2p", match: "types_cardp2p", api_name: "card2card"},
		{brand: "sbp", match: "types_sbp", api_name: "sbp"},
		{brand: "trans_card2card", match: "types_trans_card2card", api_name: "trans_card2card"},
		{brand: "trans_sbp", match: "types_trans_sbp", api_name: "trans_sbp"},
		{brand: "click", match: "types_click", api_name: "nspk", currency_overrides: [{currency: "KGS", api_name: "elqr"}]},
		{brand: "qrcode", match: "types_qrcode", api_name: "nspk", currency_overrides: [{currency: "KGS", api_name: "elqr"}]},
	]

	callback: {
		tx_id_field:      "ExternalID"
		foreign_id_field: "ID"
		status_field:     "Status"
		status_type:      "string"
		fields: [
			{name: "ID", type: "string", json: "id"},
			{name: "ExternalID", type: "string", json: "external_id"},
			{name: "Status", type: "string", json: "status"},
			{name: "StatusDetails", type: "string", json: "status_details"},
			{name: "Amount", type: "float64", json: "amount"},
			{name: "Signature", type: "string", json: "signature"},
		]
	}

	response_types: []
	request_types:  []
}
