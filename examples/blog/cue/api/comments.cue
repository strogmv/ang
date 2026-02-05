// ============================================================================
// Comments API Operations
// ============================================================================
// Demonstrates: Nested resources, parent references, moderation
// ============================================================================

package api

import "github.com/strogmv/ang/cue/schema"

// Add a comment to a post
CreateComment: schema.#Operation & {
    service: "comment"
    method: "POST"
    path: "/posts/{postID}/comments"
    description: "Add a comment to a post"
    auth: true
    publishes: ["CommentCreated"]

    input: {
        postID: string
        content: string @validate("required,min=1,max=2000")
        parentID?: string
    }

    output: {
        id: string
        content: string
        createdAt: string
    }

    http: {
        inject: {
            userID: "jwt:sub"
        }
    }

    flow: [
        // Verify post exists and is published
        {action: "repo.Find", source: "Post", input: "req.PostID", output: "post", error: "Post not found"},
        {action: "logic.Check", condition: "post.Status == \"published\"", throw: "Cannot comment on unpublished posts"},

        // Verify parent comment exists if replying
        {action: "flow.If", condition: "req.ParentID != \"\"", then: [
            {action: "repo.Find", source: "Comment", input: "req.ParentID", output: "parent", error: "Parent comment not found"},
            {action: "logic.Check", condition: "parent.PostID == req.PostID", throw: "Parent comment belongs to different post"},
        ]},

        // Create comment
        {action: "mapping.Map", output: "newComment", entity: "Comment"},
        {action: "mapping.Assign", to: "newComment.PostID", value: "req.PostID"},
        {action: "mapping.Assign", to: "newComment.AuthorID", value: "req.UserID"},
        {action: "mapping.Assign", to: "newComment.Content", value: "req.Content"},
        {action: "flow.If", condition: "req.ParentID != \"\"", then: [
            {action: "mapping.Assign", to: "newComment.ParentID", value: "&req.ParentID"},
        ]},

        {action: "repo.Save", source: "Comment", input: "newComment"},
        {action: "event.Publish", name: "CommentCreated", payload: "domain.CommentCreated{CommentID: newComment.ID, PostID: newComment.PostID, AuthorID: newComment.AuthorID}"},

        {action: "mapping.Assign", to: "resp.ID", value: "newComment.ID"},
        {action: "mapping.Assign", to: "resp.Content", value: "newComment.Content"},
        {action: "mapping.Assign", to: "resp.CreatedAt", value: "newComment.CreatedAt"},
    ]
}

// List comments for a post
ListComments: schema.#Operation & {
    service: "comment"
    method: "GET"
    path: "/posts/{postID}/comments"
    description: "List comments on a post"

    pagination: {
        type: "offset"
        default_limit: 20
        max_limit: 100
    }

    input: {
        postID: string
    }

    output: {
        data: [...{
            id: string
            content: string
            parentID?: string
            author: {
                id: string
                name: string
                avatarURL?: string
            }
            createdAt: string
        }]
        total: int
    }

    flow: [
        // Verify post exists
        {action: "repo.Find", source: "Post", input: "req.PostID", output: "post", error: "Post not found"},

        // Get comments
        {action: "repo.List", source: "Comment", method: "ListByPost", input: "req.PostID, req.Offset, req.Limit", output: "comments"},
        {action: "repo.Find", source: "Comment", method: "CountByPost", input: "req.PostID", output: "total"},

        {action: "mapping.Assign", to: "resp.Data", value: "comments"},
        {action: "mapping.Assign", to: "resp.Total", value: "total"},
    ]
}

// Update own comment
UpdateComment: schema.#Operation & {
    service: "comment"
    method: "PUT"
    path: "/comments/{id}"
    description: "Update your own comment"
    auth: true

    input: {
        id: string
        content: string @validate("required,min=1,max=2000")
    }

    output: {
        ok: bool
    }

    http: {
        inject: {
            userID: "jwt:sub"
        }
    }

    flow: [
        {action: "repo.Find", source: "Comment", input: "req.ID", output: "comment", error: "Comment not found"},
        {action: "logic.Check", condition: "comment.AuthorID == req.UserID", throw: "Access denied"},

        {action: "mapping.Assign", to: "comment.Content", value: "req.Content"},
        {action: "repo.Save", source: "Comment", input: "comment"},

        {action: "mapping.Assign", to: "resp.Ok", value: "true"},
    ]
}

// Delete own comment (or admin can delete any)
DeleteComment: schema.#Operation & {
    service: "comment"
    method: "DELETE"
    path: "/comments/{id}"
    description: "Delete a comment"
    auth: true

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
        {action: "repo.Find", source: "Comment", input: "req.ID", output: "comment", error: "Comment not found"},
        {action: "flow.If", condition: "req.Role != \"admin\"", then: [
            {action: "logic.Check", condition: "comment.AuthorID == req.UserID", throw: "Access denied"},
        ]},

        // Delete child comments first, then the comment
        {action: "tx.Block", do: [
            {action: "repo.Delete", source: "Comment", method: "DeleteByParent", input: "comment.ID"},
            {action: "repo.Delete", source: "Comment", input: "comment.ID"},
        ]},

        {action: "mapping.Assign", to: "resp.Ok", value: "true"},
    ]
}
