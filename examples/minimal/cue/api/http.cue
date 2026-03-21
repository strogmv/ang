package api

import "github.com/strogmv/ang/cue/schema"

HTTP: schema.#HTTP & {
	ListUsers: {
		method: "GET"
		path:   "/users"
	}
	SendNoticeEmail: {
		method: "POST"
		path:   "/notifications/email"
	}
	SendPasswordResetEmail: {
		method: "POST"
		path:   "/notifications/password-reset"
	}
	SendInvitationEmail: {
		method: "POST"
		path:   "/notifications/invitation"
	}
}
