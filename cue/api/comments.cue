// ============================================================================
// Comments API Operations
// ============================================================================
// Demonstrates: Nested resources, ownership checks
// ============================================================================

package api

import "github.com/strogmv/ang/cue/schema"

// Create a new comment on a post
CreateComment: schema.#Operation & {
	service:   "blog"
	description: "Post a comment on a blog post"

	input: {
		postID:   string @validate("required,uuid")
		content:  string @validate("required,min=1,max=1000")
		userId:   string
	}

	output: {
		id: string
	}

	flow: [
		{action: "repo.Find", source: "Post", input: "req.PostID", output: "post", error: "Post not found"},

		// Create comment
		{action: "mapping.Map", output: "newComment", entity: "Comment"},
		{action: "mapping.Assign", to: "newComment.PostID", value: "req.PostID"},
		{action: "mapping.Assign", to: "newComment.AuthorID", value: "req.UserID"},
		{action: "mapping.Assign", to: "newComment.Content", value: "req.Content"},

		{action: "repo.Save", source: "Comment", input: "newComment"},
		{action: "mapping.Assign", to: "resp.ID", value: "newComment.ID"},
	]
}

// List comments for a post
ListComments: schema.#Operation & {
	service:   "blog"
	description: "List comments for a specific post"

	input: {
		postID: string @validate("required,uuid")
		limit:  int | *10
		offset: int | *0
	}

	output: {
		data: [...{
			id:        string
			content:   string
			userId:    string
			createdAt: string
		}]
		total: int
	}

	flow: [
		{action: "repo.List", source: "Comment", method: "ListByPost", input: "req.PostID", output: "comments"},
		{action: "repo.Find", source: "Comment", method: "CountByPost", input: "req.PostID", output: "totalCount"},

		{action: "mapping.Assign", to: "resp.Data", value: "comments"},
		{action: "mapping.Assign", to: "resp.Total", value: "totalCount"},
	]
}

// Update a comment
UpdateComment: schema.#Operation & {
	service:   "blog"
	description: "Update own comment content"

	input: {
		id:      string @validate("required,uuid")
		content: string @validate("required,min=1,max=1000")
		userId:  string
	}

	output: {
		ok: bool
	}

	flow: [
		{action: "repo.Find", source: "Comment", input: "req.ID", output: "comment", error: "Comment not found"},

		// Check ownership
		{action: "logic.Check", condition: "comment.AuthorID == req.UserID", throw: "Not authorized to update this comment"},

		{action: "mapping.Assign", to: "comment.Content", value: "req.Content"},
		{action: "repo.Save", source: "Comment", input: "comment"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// Delete a comment
DeleteComment: schema.#Operation & {
	service:   "blog"
	description: "Delete own comment"

	input: {
		id:     string @validate("required,uuid")
		userId: string
	}

	output: {
		ok: bool
	}

	flow: [
		{action: "repo.Find", source: "Comment", input: "req.ID", output: "comment", error: "Comment not found"},

		// Check ownership or admin
		{action: "logic.Check", condition: "comment.AuthorID == req.UserID", throw: "Not authorized"},

		// Delete comment and its sub-comments (recursive delete simulation)
		{action: "tx.Block", do: [
			{action: "repo.Delete", source: "Comment", method: "DeleteByParent", input: "comment.ID"},
			{action: "repo.Delete", source: "Comment", input: "comment.ID"},
		]},

		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}
