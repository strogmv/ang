// ============================================================================
// Posts API Operations
// ============================================================================
// Demonstrates: Complex flows, transactions, many-to-many associations
// ============================================================================

package api

import "github.com/strogmv/ang/cue/schema"

// Create a new post (Author only)
CreatePost: schema.#Operation & {
	service:   "blog"
	description: "Create a new blog post in draft status"

	input: {
		title:   string @validate("required,min=5,max=200")
		content: string @validate("required")
		userId:  string
		tags?: [...string]
	}

	output: {
		id:   string
		slug: string
	}

	flow: [
		// Generate slug and check availability
		{action: "logic.Call", func: "slugify", args: "req.Title", output: "slug"},
		{action: "repo.Find", source: "Post", method: "FindBySlug", input: "slug", output: "existing"},
		{action: "flow.If", condition: "existing != nil", then: [
			{action: "logic.Call", func: "appendRandom", args: "slug", output: "slug"},
		]},

		{action: "tx.Block", do: [
			// Create post
			{action: "mapping.Map", output: "newPost", entity: "Post"},
			{action: "mapping.Assign", to: "newPost.Title", value: "req.Title"},
			{action: "mapping.Assign", to: "newPost.Content", value: "req.Content"},
			{action: "mapping.Assign", to: "newPost.Slug", value: "slug"},
			{action: "mapping.Assign", to: "newPost.AuthorID", value: "req.UserId"},
			{action: "mapping.Assign", to: "newPost.Status", value: "\"draft\""},

			{action: "repo.Save", source: "Post", input: "newPost"},

			// Handle tags
			{action: "flow.For", each: "req.Tags", as: "tagName", do: [
				{action: "repo.Find", source: "Tag", method: "FindBySlug", input: "tagName", output: "tag"},
				{action: "flow.If", condition: "tag != nil", then: [
					{action: "mapping.Map", output: "assoc", entity: "PostTag"},
					{action: "mapping.Assign", to: "assoc.PostID", value: "newPost.ID"},
					{action: "mapping.Assign", to: "assoc.TagID", value: "tag.ID"},
					{action: "repo.Save", source: "PostTag", input: "assoc"},
				]},
			]},
		]},

		{action: "mapping.Assign", to: "resp.ID", value: "newPost.ID"},
		{action: "mapping.Assign", to: "resp.Slug", value: "newPost.Slug"},
	]
}

// Get a post by slug
GetPost: schema.#Operation & {
	service:   "blog"
	description: "Get post details by slug"

	input: {
		slug: string @validate("required")
	}

	output: {
		id:        string
		title:     string
		content:   string
		authorId:  string
		createdAt: string
		tags: [...{
			id:   string
			name: string
			slug: string
		}]
	}

	flow: [
		{action: "repo.Find", source: "Post", method: "FindBySlug", input: "req.Slug", output: "post", error: "Post not found"},
		{action: "repo.List", source: "Tag", method: "ListByPost", input: "post.ID", output: "tags"},

		{action: "mapping.Assign", to: "resp.ID", value: "post.ID"},
		{action: "mapping.Assign", to: "resp.Title", value: "post.Title"},
		{action: "mapping.Assign", to: "resp.Content", value: "post.Content"},
		{action: "mapping.Assign", to: "resp.AuthorID", value: "post.AuthorID"},
		{action: "mapping.Assign", to: "resp.CreatedAt", value: "post.CreatedAt"},
		{action: "mapping.Assign", to: "resp.Tags", value: "tags"},
	]
}

// List posts with pagination and optional tag filter
ListPosts: schema.#Operation & {
	service:   "blog"
	description: "List all published posts"

	input: {
		tag?:   string
		limit:  int | *10
		offset: int | *0
	}

	output: {
		data: [...{
			id:        string
			title:     string
			slug:      string
			authorId:  string
			createdAt: string
		}]
		total: int
	}

	flow: [
		{action: "flow.If", condition: "req.Tag != \"\"", then: [
			{action: "repo.List", source: "Post", method: "ListPublishedByTag", input: "req.Tag", output: "posts"},
			{action: "repo.Find", source: "Post", method: "CountPublishedByTag", input: "req.Tag", output: "totalCount"},
		], else: [
			{action: "repo.List", source: "Post", method: "ListPublished", output: "posts"},
			{action: "repo.Find", source: "Post", method: "CountPublished", output: "totalCount"},
		]},

		{action: "mapping.Assign", to: "resp.Data", value: "posts"},
		{action: "mapping.Assign", to: "resp.Total", value: "totalCount"},
	]
}

// List my posts
ListMyPosts: schema.#Operation & {
	service:   "blog"
	description: "List all posts by the current author"

	input: {
		userId: string
		status?: string
		limit:  int | *10
		offset: int | *0
	}

	output: {
		data: [...{
			id:        string
			title:     string
			status:    string
			createdAt: string
		}]
	}

	flow: [
		{action: "flow.If", condition: "req.Status != \"\"", then: [
			{action: "repo.List", source: "Post", method: "ListByAuthorAndStatus", input: "req.UserId", output: "posts"},
		], else: [
			{action: "repo.List", source: "Post", method: "ListByAuthor", input: "req.UserId", output: "posts"},
		]},
		{action: "mapping.Assign", to: "resp.Data", value: "posts"},
	]
}

// Update a post
UpdatePost: schema.#Operation & {
	service:   "blog"
	description: "Update an existing post"

	input: {
		id:       string @validate("required,uuid")
		title?:    string @validate("min=5,max=200")
		content?:  string
	}

	output: {
		ok: bool
	}

	flow: [
		{action: "repo.Find", source: "Post", input: "req.ID", output: "post", error: "Post not found"},

		{action: "flow.If", condition: "req.Title != \"\"", then: [
			{action: "mapping.Assign", to: "post.Title", value: "req.Title"},
		]},
		{action: "flow.If", condition: "req.Content != \"\"", then: [
			{action: "mapping.Assign", to: "post.Content", value: "req.Content"},
		]},

		{action: "repo.Save", source: "Post", input: "post"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// Submit post for review
SubmitPost: schema.#Operation & {
	service:   "blog"
	description: "Change post status to pending review"

	input: {
		id: string @validate("required,uuid")
	}

	output: {
		ok: bool
	}

	flow: [
		{action: "repo.Find", source: "Post", input: "req.ID", output: "post", error: "Post not found"},
		{action: "fsm.Transition", entity: "post", to: "pending"},
		{action: "repo.Save", source: "Post", input: "post"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// Publish post (Admin only)
PublishPost: schema.#Operation & {
	service:   "blog"
	description: "Approve and publish a post"

	input: {
		id: string @validate("required,uuid")
	}

	output: {
		ok: bool
	}

	flow: [
		{action: "repo.Find", source: "Post", input: "req.ID", output: "post", error: "Post not found"},
		{action: "fsm.Transition", entity: "post", to: "published"},
		{action: "repo.Save", source: "Post", input: "post"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// Archive post
ArchivePost: schema.#Operation & {
	service:   "blog"
	description: "Archive a published post"

	input: {
		id: string @validate("required,uuid")
	}

	output: {
		ok: bool
	}

	flow: [
		{action: "repo.Find", source: "Post", input: "req.ID", output: "post", error: "Post not found"},
		{action: "fsm.Transition", entity: "post", to: "archived"},
		{action: "repo.Save", source: "Post", input: "post"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}

// Delete post
DeletePost: schema.#Operation & {
	service:   "blog"
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