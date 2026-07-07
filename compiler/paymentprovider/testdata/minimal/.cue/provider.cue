package provider

import "ang.test/minimal/schema"

provider: schema.#Provider & {
	package_name: "testpp"
	sid:          "testpp"
	source:       "PPTestPP"
	label:        "Test"
	mid_prefix:   "TST"

	struct_name:      "PPTest"
	constructor_name: "New"
	payment_source:   "apm"

	secrets: {
		format:    "key:sign"
		separator: ":"
		parts: [
			{name: "Key", key: "apiKey"},
			{name: "Sign", key: "signingKey"},
		]
	}

	currency: {code: "USD", iso_num: 840}

	endpoints: {
		payin: {path: "/payin", method: "POST"}
	}

	signing: {algorithm: "hmac-sha256", format: "hmac_body"}

	payin_request: {
		name: "payinRequest"
		fields: [
			{name: "OrderID", json: "orderId", source: "tx_id"},
			{name: "Amount", json: "amount", source: "tx_amount_fmt"},
			{name: "Currency", json: "currency", type: "int", source: "currency_iso_num"},
			{name: "ApiKey", json: "apiKey", source: "secret", secret_key: "apiKey"},
			{name: "Salt", json: "salt", source: "salt", omitempty: true},
		]
	}

	payin_statuses: [
		{code: 1, status: "pending", status_code: "SCodeOk"},
		{code: 10, status: "success", status_code: "SCodeOk"},
	]

	response_types: [
		{
			name: "payinResponse"
			fields: [{name: "PaymentID", type: "string", json: "paymentId"}]
		},
	]

	has_payin: true

	constructor_deps: [
		{name: "txPathLogger", type: "model.TxPathLogger"},
	]
}
