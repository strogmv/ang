// ============================================================================
// Post Lifecycle API Operations
// ============================================================================
// Split from posts.cue to keep files compact and purpose-focused
// ============================================================================

package api

import "github.com/strogmv/ang/cue/schema"

// Archive post
ArchivePost: schema.#Operation & {
	service:    "blog"
	description: "Archive a published post"

	input: {
		id: string @validate("required,uuid")
	}

	output: {
		ok: bool
	}

	flow: [
		{action: "repo.Find", source: "Post", input: "req.ID", output: "post", error: "Post not found"},
		{action: "fsm.Transition", entity: "post", to: "\"archived\""},
		{action: "repo.Save", source: "Post", input: "post"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// Delete post
DeletePost: schema.#Operation & {
	service:    "blog"
	description: "Delete a post and its associations"

	input: {
		id: string @validate("required,uuid")
	}

	output: {
		ok: bool
	}

	flow: [
		{action: "repo.Find", source: "Post", input: "req.ID", output: "post", error: "Post not found"},

		{action: "tx.Block", do: [
			{action: "repo.Delete", source: "PostTag", method: "DeleteByPost", input: "post.ID"},
			{action: "repo.Delete", source: "Post", input: "post.ID"},
		]},

		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}
