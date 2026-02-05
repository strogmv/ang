// ============================================================================
// Tags API Operations
// ============================================================================
// Demonstrates: Simple CRUD, admin-only operations
// ============================================================================

package api

import "github.com/strogmv/ang/cue/schema"

// List all tags (public)
ListTags: schema.#Operation & {
    service: "blog"
    method: "GET"
    path: "/tags"
    description: "List all available tags"

    input: {}

    output: {
        data: [...{
            id: string
            name: string
            slug: string
            description?: string
        }]
    }

    flow: [
        {action: "repo.List", source: "Tag", method: "ListAll", output: "tags"},
        {action: "mapping.Assign", to: "resp.Data", value: "tags"},
    ]
}

// Create a new tag (admin only)
CreateTag: schema.#Operation & {
    service: "blog"
    method: "POST"
    path: "/tags"
    description: "Create a new tag"
    auth: true
    roles: ["admin"]

    input: {
        name: string @validate("required,min=2,max=50")
        description?: string
    }

    output: {
        id: string
        slug: string
    }

    flow: [
        // Generate slug from name
        {action: "logic.Call", func: "slugify", args: "req.Name", output: "slug"},

        // Check if tag already exists
        {action: "repo.Find", source: "Tag", method: "FindBySlug", input: "slug", output: "existing"},
        {action: "logic.Check", condition: "existing == nil", throw: "Tag already exists"},

        // Create tag
        {action: "mapping.Map", output: "newTag", entity: "Tag"},
        {action: "mapping.Assign", to: "newTag.Name", value: "req.Name"},
        {action: "mapping.Assign", to: "newTag.Slug", value: "slug"},
        {action: "mapping.Assign", to: "newTag.Description", value: "req.Description"},

        {action: "repo.Save", source: "Tag", input: "newTag"},

        {action: "mapping.Assign", to: "resp.ID", value: "newTag.ID"},
        {action: "mapping.Assign", to: "resp.Slug", value: "newTag.Slug"},
    ]
}

// Update a tag (admin only)
UpdateTag: schema.#Operation & {
    service: "blog"
    method: "PUT"
    path: "/tags/{id}"
    description: "Update a tag"
    auth: true
    roles: ["admin"]

    input: {
        id: string
        name?: string @validate("min=2,max=50")
        description?: string
    }

    output: {
        ok: bool
    }

    flow: [
        {action: "repo.Find", source: "Tag", input: "req.ID", output: "tag", error: "Tag not found"},

        {action: "flow.If", condition: "req.Name != \"\"", then: [
            {action: "mapping.Assign", to: "tag.Name", value: "req.Name"},
            {action: "logic.Call", func: "slugify", args: "req.Name", output: "slug"},
            {action: "mapping.Assign", to: "tag.Slug", value: "slug"},
        ]},
        {action: "flow.If", condition: "req.Description != \"\"", then: [
            {action: "mapping.Assign", to: "tag.Description", value: "req.Description"},
        ]},

        {action: "repo.Save", source: "Tag", input: "tag"},
        {action: "mapping.Assign", to: "resp.Ok", value: "true"},
    ]
}

// Delete a tag (admin only)
DeleteTag: schema.#Operation & {
    service: "blog"
    method: "DELETE"
    path: "/tags/{id}"
    description: "Delete a tag"
    auth: true
    roles: ["admin"]

    input: {
        id: string
    }

    output: {
        ok: bool
    }

    flow: [
        {action: "repo.Find", source: "Tag", input: "req.ID", output: "tag", error: "Tag not found"},

        // Remove tag associations and delete
        {action: "tx.Block", do: [
            {action: "repo.Delete", source: "PostTag", method: "DeleteByTag", input: "tag.ID"},
            {action: "repo.Delete", source: "Tag", input: "tag.ID"},
        ]},

        {action: "mapping.Assign", to: "resp.Ok", value: "true"},
    ]
}
