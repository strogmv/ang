package api

import "github.com/strogmv/ang/cue/schema"

SendPasswordResetEmail: schema.#Operation & {
	service:   "notifications"
	description: "Send a password reset email using the universal notification module"
	primary_operation_kind: "notify"
	capabilities: ["notify"]
	side_effects: [{kind: "notify.email", channel: "email", template: "password_reset"}]

	input: {
		email:    string @validate("required,email")
		name?:    string
		resetURL: string @validate("required,min=12,max=500")
	}

	output: {
		ok: bool
	}

	flow: [
		{action: "notify.Email", to: "req.Email", template: "\"password_reset\"", data: "map[string]any{\"Name\": req.Name, \"ResetURL\": req.ResetURL, \"AppName\": \"ANG Blog Example\", \"SupportEmail\": \"blog-support@ang.local\"}"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

SendEmailVerification: schema.#Operation & {
	service:   "notifications"
	description: "Send an email verification message using the universal notification module"
	primary_operation_kind: "notify"
	capabilities: ["notify"]
	side_effects: [{kind: "notify.email", channel: "email", template: "email_verification"}]

	input: {
		email:     string @validate("required,email")
		name?:     string
		verifyURL: string @validate("required,min=12,max=500")
	}

	output: {
		ok: bool
	}

	flow: [
		{action: "notify.Email", to: "req.Email", template: "\"email_verification\"", data: "map[string]any{\"Name\": req.Name, \"VerifyURL\": req.VerifyURL, \"AppName\": \"ANG Blog Example\", \"SupportEmail\": \"blog-support@ang.local\"}"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}
