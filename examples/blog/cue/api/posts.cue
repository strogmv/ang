// ============================================================================
// Blog Posts API Operations
// ============================================================================
// Demonstrates: CRUD, FSM transitions, pagination, ownership checks, events
// ============================================================================

package api

import "github.com/strogmv/ang/cue/schema"

// Create a new blog post (draft)
CreatePost: schema.#Operation & {
    service: "blog"
    method: "POST"
    path: "/posts"
    description: "Create a new blog post as draft"
    auth: true
    roles: ["author", "admin"]
    publishes: ["PostCreated"]

    input: {
        title: string @validate("required,min=5,max=200")
        content: string @validate("required")
        excerpt?: string
        tags?: [...string]
    }

    output: {
        id: string
        slug: string
    }

    http: {
        inject: {
            userID: "jwt:sub"
        }
    }

    flow: [
        // Create the post
        {action: "mapping.Map", output: "newPost", entity: "Post"},
        {action: "mapping.Assign", to: "newPost.AuthorID", value: "req.UserID"},
        {action: "mapping.Assign", to: "newPost.Title", value: "req.Title"},
        {action: "logic.Call", func: "slugify", args: "req.Title", output: "slug"},
        {action: "mapping.Assign", to: "newPost.Slug", value: "slug"},
        {action: "mapping.Assign", to: "newPost.Content", value: "req.Content"},
        {action: "mapping.Assign", to: "newPost.Excerpt", value: "req.Excerpt"},
        {action: "mapping.Assign", to: "newPost.Status", value: "\"draft\""},

        // Save in transaction with tags
        {action: "tx.Block", do: [
            {action: "repo.Save", source: "Post", input: "newPost"},

            // Handle tags if provided
            {action: "flow.If", condition: "len(req.Tags) > 0", then: [
                {action: "flow.For", each: "req.Tags", as: "tagName", do: [
                    {action: "repo.Find", source: "Tag", method: "FindBySlug", input: "tagName", output: "tag"},
                    {action: "flow.If", condition: "tag != nil", then: [
                        {action: "mapping.Map", output: "postTag", entity: "PostTag"},
                        {action: "mapping.Assign", to: "postTag.PostID", value: "newPost.ID"},
                        {action: "mapping.Assign", to: "postTag.TagID", value: "tag.ID"},
                        {action: "repo.Save", source: "PostTag", input: "postTag"},
                    ]},
                ]},
            ]},
        ]},

        // Publish event
        {action: "event.Publish", name: "PostCreated", payload: "domain.PostCreated{PostID: newPost.ID, AuthorID: newPost.AuthorID, Title: newPost.Title}"},

        {action: "mapping.Assign", to: "resp.ID", value: "newPost.ID"},
        {action: "mapping.Assign", to: "resp.Slug", value: "newPost.Slug"},
    ]
}

// Get a single post by slug (public)
GetPost: schema.#Operation & {
    service: "blog"
    method: "GET"
    path: "/posts/{slug}"
    description: "Get a published post by slug"

    input: {
        slug: string
    }

    output: {
        id: string
        title: string
        slug: string
        content: string
        excerpt?: string
        status: string
        author: {
            id: string
            name: string
            avatarURL?: string
        }
        viewCount: int
        publishedAt?: string
        tags: [...{
            id: string
            name: string
            slug: string
        }]
    }

    sources: {
        post: {
            kind: "sql"
            entity: "Post"
        }
    }

    flow: [
        {action: "repo.Find", source: "Post", method: "FindBySlug", input: "req.Slug", output: "post", error: "Post not found"},
        {action: "logic.Check", condition: "post.Status == \"published\"", throw: "Post not found"},

        // Increment view count (fire and forget)
        {action: "mapping.Assign", to: "post.ViewCount", value: "post.ViewCount + 1"},
        {action: "repo.Save", source: "Post", input: "post"},

        // Get author info
        {action: "repo.Find", source: "User", input: "post.AuthorID", output: "author"},

        // Get tags
        {action: "repo.List", source: "Tag", method: "ListByPost", input: "post.ID", output: "tags"},

        // Build response
        {action: "mapping.Assign", to: "resp.ID", value: "post.ID"},
        {action: "mapping.Assign", to: "resp.Title", value: "post.Title"},
        {action: "mapping.Assign", to: "resp.Slug", value: "post.Slug"},
        {action: "mapping.Assign", to: "resp.Content", value: "post.Content"},
        {action: "mapping.Assign", to: "resp.Excerpt", value: "post.Excerpt"},
        {action: "mapping.Assign", to: "resp.Status", value: "post.Status"},
        {action: "mapping.Assign", to: "resp.ViewCount", value: "post.ViewCount"},
        {action: "mapping.Assign", to: "resp.PublishedAt", value: "post.PublishedAt"},
        {action: "mapping.Assign", to: "resp.Author.ID", value: "author.ID"},
        {action: "mapping.Assign", to: "resp.Author.Name", value: "author.Name"},
        {action: "mapping.Assign", to: "resp.Author.AvatarURL", value: "author.AvatarURL"},
        {action: "mapping.Assign", to: "resp.Tags", value: "tags"},
    ]
}

// List published posts (public, paginated)
ListPosts: schema.#Operation & {
    service: "blog"
    method: "GET"
    path: "/posts"
    description: "List published posts with pagination"

    pagination: {
        type: "offset"
        default_limit: 10
        max_limit: 50
    }

    input: {
        tag?: string
    }

    output: {
        data: [...{
            id: string
            title: string
            slug: string
            excerpt?: string
            author: {
                id: string
                name: string
            }
            publishedAt: string
            viewCount: int
        }]
        total: int
    }

    flow: [
        {action: "flow.If", condition: "req.Tag != \"\"", then: [
            {action: "repo.List", source: "Post", method: "ListPublishedByTag", input: "req.Tag, req.Offset, req.Limit", output: "posts"},
            {action: "repo.Find", source: "Post", method: "CountPublishedByTag", input: "req.Tag", output: "total"},
        ], else: [
            {action: "repo.List", source: "Post", method: "ListPublished", input: "req.Offset, req.Limit", output: "posts"},
            {action: "repo.Find", source: "Post", method: "CountPublished", output: "total"},
        ]},
        {action: "mapping.Assign", to: "resp.Data", value: "posts"},
        {action: "mapping.Assign", to: "resp.Total", value: "total"},
    ]
}

// List my posts (author's dashboard)
ListMyPosts: schema.#Operation & {
    service: "blog"
    method: "GET"
    path: "/my/posts"
    description: "List current user's posts (all statuses)"
    auth: true
    roles: ["author", "admin"]

    pagination: {
        type: "offset"
        default_limit: 20
        max_limit: 100
    }

    input: {
        status?: string
    }

    output: {
        data: [...{
            id: string
            title: string
            slug: string
            status: string
            viewCount: int
            createdAt: string
            updatedAt: string
        }]
        total: int
    }

    http: {
        inject: {
            userID: "jwt:sub"
        }
    }

    flow: [
        {action: "flow.If", condition: "req.Status != \"\"", then: [
            {action: "repo.List", source: "Post", method: "ListByAuthorAndStatus", input: "req.UserID, req.Status, req.Offset, req.Limit", output: "posts"},
        ], else: [
            {action: "repo.List", source: "Post", method: "ListByAuthor", input: "req.UserID, req.Offset, req.Limit", output: "posts"},
        ]},
        {action: "mapping.Assign", to: "resp.Data", value: "posts"},
        {action: "mapping.Assign", to: "resp.Total", value: "len(posts)"},
    ]
}

// Update a post
UpdatePost: schema.#Operation & {
    service: "blog"
    method: "PUT"
    path: "/posts/{id}"
    description: "Update a post (only drafts can be fully edited)"
    auth: true
    roles: ["author", "admin"]
    publishes: ["PostUpdated"]

    input: {
        id: string
        title?: string @validate("min=5,max=200")
        content?: string
        excerpt?: string
    }

    output: {
        ok: bool
    }

    http: {
        inject: {
            userID: "jwt:sub"
            role: "jwt:role"
        }
    }

    flow: [
        {action: "repo.Find", source: "Post", input: "req.ID", output: "post", error: "Post not found"},

        // Check ownership (unless admin)
        {action: "flow.If", condition: "req.Role != \"admin\"", then: [
            {action: "logic.Check", condition: "post.AuthorID == req.UserID", throw: "Access denied"},
        ]},

        // Only drafts can be fully edited
        {action: "logic.Check", condition: "post.Status == \"draft\" || post.Status == \"review\"", throw: "Published posts cannot be edited"},

        // Update fields
        {action: "flow.If", condition: "req.Title != \"\"", then: [
            {action: "mapping.Assign", to: "post.Title", value: "req.Title"},
            {action: "logic.Call", func: "slugify", args: "req.Title", output: "slug"},
            {action: "mapping.Assign", to: "post.Slug", value: "slug"},
        ]},
        {action: "flow.If", condition: "req.Content != \"\"", then: [
            {action: "mapping.Assign", to: "post.Content", value: "req.Content"},
        ]},
        {action: "flow.If", condition: "req.Excerpt != \"\"", then: [
            {action: "mapping.Assign", to: "post.Excerpt", value: "req.Excerpt"},
        ]},

        {action: "repo.Save", source: "Post", input: "post"},
        {action: "event.Publish", name: "PostUpdated", payload: "domain.PostUpdated{PostID: post.ID}"},
        {action: "mapping.Assign", to: "resp.Ok", value: "true"},
    ]
}

// Submit post for review (FSM transition: draft -> review)
SubmitForReview: schema.#Operation & {
    service: "blog"
    method: "POST"
    path: "/posts/{id}/submit"
    description: "Submit a draft post for review"
    auth: true
    roles: ["author", "admin"]

    input: {
        id: string
    }

    output: {
        ok: bool
        status: string
    }

    http: {
        inject: {
            userID: "jwt:sub"
        }
    }

    flow: [
        {action: "repo.Find", source: "Post", input: "req.ID", output: "post", error: "Post not found"},
        {action: "logic.Check", condition: "post.AuthorID == req.UserID", throw: "Access denied"},
        {action: "logic.Check", condition: "post.Status == \"draft\"", throw: "Only drafts can be submitted for review"},

        {action: "fsm.Transition", entity: "post", to: "review"},
        {action: "repo.Save", source: "Post", input: "post"},

        {action: "mapping.Assign", to: "resp.Ok", value: "true"},
        {action: "mapping.Assign", to: "resp.Status", value: "post.Status"},
    ]
}

// Publish a post (FSM transition: review -> published)
PublishPost: schema.#Operation & {
    service: "blog"
    method: "POST"
    path: "/posts/{id}/publish"
    description: "Publish a post (admin only)"
    auth: true
    roles: ["admin"]
    publishes: ["PostPublished"]

    input: {
        id: string
    }

    output: {
        ok: bool
        status: string
        publishedAt: string
    }

    flow: [
        {action: "repo.Find", source: "Post", input: "req.ID", output: "post", error: "Post not found"},
        {action: "logic.Check", condition: "post.Status == \"review\"", throw: "Only posts in review can be published"},

        {action: "fsm.Transition", entity: "post", to: "published"},
        {action: "mapping.Assign", to: "post.PublishedAt", value: "time.Now().UTC().Format(time.RFC3339)"},
        {action: "repo.Save", source: "Post", input: "post"},

        {action: "event.Publish", name: "PostPublished", payload: "domain.PostPublished{PostID: post.ID, AuthorID: post.AuthorID, Title: post.Title, Slug: post.Slug}"},

        {action: "mapping.Assign", to: "resp.Ok", value: "true"},
        {action: "mapping.Assign", to: "resp.Status", value: "post.Status"},
        {action: "mapping.Assign", to: "resp.PublishedAt", value: "post.PublishedAt"},
    ]
}

// Archive a post (FSM transition: published -> archived)
ArchivePost: schema.#Operation & {
    service: "blog"
    method: "POST"
    path: "/posts/{id}/archive"
    description: "Archive a published post"
    auth: true
    roles: ["author", "admin"]

    input: {
        id: string
    }

    output: {
        ok: bool
    }

    http: {
        inject: {
            userID: "jwt:sub"
            role: "jwt:role"
        }
    }

    flow: [
        {action: "repo.Find", source: "Post", input: "req.ID", output: "post", error: "Post not found"},
        {action: "flow.If", condition: "req.Role != \"admin\"", then: [
            {action: "logic.Check", condition: "post.AuthorID == req.UserID", throw: "Access denied"},
        ]},
        {action: "logic.Check", condition: "post.Status == \"published\"", throw: "Only published posts can be archived"},

        {action: "fsm.Transition", entity: "post", to: "archived"},
        {action: "repo.Save", source: "Post", input: "post"},

        {action: "mapping.Assign", to: "resp.Ok", value: "true"},
    ]
}

// Delete a post (only drafts)
DeletePost: schema.#Operation & {
    service: "blog"
    method: "DELETE"
    path: "/posts/{id}"
    description: "Delete a draft post"
    auth: true
    roles: ["author", "admin"]

    input: {
        id: string
    }

    output: {
        ok: bool
    }

    http: {
        inject: {
            userID: "jwt:sub"
            role: "jwt:role"
        }
    }

    flow: [
        {action: "repo.Find", source: "Post", input: "req.ID", output: "post", error: "Post not found"},
        {action: "flow.If", condition: "req.Role != \"admin\"", then: [
            {action: "logic.Check", condition: "post.AuthorID == req.UserID", throw: "Access denied"},
        ]},
        {action: "logic.Check", condition: "post.Status == \"draft\"", throw: "Only drafts can be deleted"},

        // Delete post tags first
        {action: "tx.Block", do: [
            {action: "repo.Delete", source: "PostTag", method: "DeleteByPost", input: "post.ID"},
            {action: "repo.Delete", source: "Post", input: "post.ID"},
        ]},

        {action: "mapping.Assign", to: "resp.Ok", value: "true"},
    ]
}
