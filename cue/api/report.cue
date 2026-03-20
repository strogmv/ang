// ============================================================================
// Event Audit + Projection Handlers
// ============================================================================
//
// These operations are used as subscribers for event-driven audit/projection
// flows. `cmd/server/main.go` expects corresponding request/response DTOs and
// service methods to exist under `internal/port`.
//
// The operations are intentionally minimal: they validate event payload shape
// (via typed inputs) and return `ok=true`. Real projection/audit logic can
// be added later in flow steps.
//

package api

import "github.com/strogmv/ang/cue/schema"

// --- Auth audits (owned by `auth` service) ---

OnUserLoggedInAudit: schema.#Operation & {
	service: "auth"
	description: "Audit user login event"

	input: {
		UserID: string
	}

	output: {
		ok: bool
	}

	flow: [
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

OnUserRegisteredAudit: schema.#Operation & {
	service: "auth"
	description: "Audit user registration event"

	input: {
		UserID: string
		Email:  string
	}

	output: {
		ok: bool
	}

	flow: [
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// --- Blog projections (owned by `blog` service) ---

OnCommentCreatedProjection: schema.#Operation & {
	service: "blog"
	description: "Project comment created event"

	input: {
		CommentID: string
		PostID:    string
		AuthorID:  string
	}

	output: {
		ok: bool
	}

	flow: [
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

OnPostCreatedProjection: schema.#Operation & {
	service: "blog"
	description: "Project post created event"

	input: {
		PostID:   string
		AuthorID: string
		Title:    string
	}

	output: {
		ok: bool
	}

	flow: [
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

OnPostPublishedProjection: schema.#Operation & {
	service: "blog"
	description: "Project post published event"

	input: {
		PostID:   string
		AuthorID: string
		Title:    string
		Slug:     string
	}

	output: {
		ok: bool
	}

	flow: [
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

OnPostUpdatedProjection: schema.#Operation & {
	service: "blog"
	description: "Project post updated event"

	input: {
		PostID: string
	}

	output: {
		ok: bool
	}

	flow: [
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

