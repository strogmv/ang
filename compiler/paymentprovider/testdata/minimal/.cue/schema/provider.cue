package schema

#FieldSource:
	"tx_id" | "tx_amount_fmt" | "currency_iso_num" | "secret" | "salt" | "const" | "owner_info"

#RequestFieldMapping: {
	name:       string
	json:       string
	type:       *"string" | "int"
	source:     #FieldSource
	secret_key: *"" | string
	const_val:  *"" | string
	owner_key:  *"" | string
	owner_from: *"" | "card" | "apm"
	omitempty:  *false | bool
}

#RequestDef: {
	name:   string
	fields: [...#RequestFieldMapping]
}

#StatusMapping: {
	code:        int | string
	status:      "success" | "pending" | "declined" | "error"
	status_code: string
	message:     *"" | string
}

#SigningScheme: {
	algorithm: "sha256" | "hmac-sha256" | "md5" | "none"
	format:    *"key_concat_sorted_values" | string
}

#Provider: {
	package_name: string
	sid:          string
	source:       string
	label:        string
	mid_prefix:   string
	struct_name:      string
	constructor_name: string
	payment_source:   *"apm" | "card" | "both"
	secrets: {
		format:    string
		separator: *":" | string
		parts: [{name: string, key: string}, ...]
	}
	currency: {
		code:    string
		iso_num: *0 | int
	}
	endpoints: [string]: {path: string, method: *"POST" | string}
	signing: #SigningScheme
	payin_statuses: [...#StatusMapping]
	payout_statuses: [...#StatusMapping]
	payin_request:  *null | #RequestDef
	payout_request: *null | #RequestDef
	response_types: [...{name: string, fields: [...{name: string, type: string, json: string}]}]
	has_payin:  *true | bool
	has_payout: *false | bool
	interfaces: {
		balance_fetcher: *false | bool
		mobile_processor: *false | bool
		tds_redirector: *false | bool
	}
	constructor_deps: [...{name: string, type: string}]
}
