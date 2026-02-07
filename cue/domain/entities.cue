// ============================================================================
// Blog Domain Entities
// ============================================================================
// Demonstrates: Entity definitions, relationships, FSM, validation, timestamps
// ============================================================================

package domain

import "github.com/strogmv/ang/cue/schema"

// User entity - blog authors and readers
#User: schema.#Entity & {
    name: "User"
    description: "Blog user account"

    fields: {
        id: {
            type: "uuid"
            description: "Unique identifier"
        }
        email: {
            type: "string"
            description: "User email address"
            validate: "required,email"
        }
        phoneNumber?: {
            type: "string"
            description: "User phone number"
        }
        passwordHash: {
            type: "string"
            description: "Bcrypt password hash"
        }
        name: {
            type: "string"
            description: "Display name"
            validate: "required,min=2,max=100"
        }
        role: {
            type: "string"
            description: "User role: reader, author, admin"
            default: "reader"
        }
        avatarURL: {
            type: "string"
            description: "Profile picture URL"
            optional: true
        }
        createdAt: {
            type: "time"
            description: "Account creation timestamp"
        }
        updatedAt: {
            type: "time"
            description: "Last update timestamp"
        }
    }
}

// Post entity - blog articles with state machine
#Post: schema.#Entity & {
    name: "Post"
    description: "Blog post/article"

    fields: {
        id: {
            type: "uuid"
            description: "Unique identifier"
        }
        authorID: {
            type: "uuid"
            description: "Reference to User who created the post"
            ref: "User"
        }
        title: {
            type: "string"
            description: "Post title"
            validate: "required,min=5,max=200"
        }
        slug: {
            type: "string"
            description: "URL-friendly identifier"
            validate: "required"
        }
        content: {
            type: "string"
            description: "Post content (markdown)"
            validate: "required"
        }
        excerpt: {
            type: "string"
            description: "Short preview text"
            optional: true
        }
        status: {
            type: "string"
            description: "Publication status"
            default: "draft"
        }
        publishedAt: {
            type: "*time"
            description: "When post was published"
            optional: true
        }
        viewCount: {
            type: "int"
            description: "Number of views"
            default: "0"
        }
        createdAt: {
            type: "time"
            description: "Creation timestamp"
        }
        updatedAt: {
            type: "time"
            description: "Last update timestamp"
        }
    }

    // State machine for post lifecycle
    fsm: {
        field: "status"
        initial: "draft"
        states: ["draft", "review", "published", "archived"]
        transitions: [
            {from: "draft", to: "review"},
            {from: "review", to: "draft"},
            {from: "review", to: "published"},
            {from: "published", to: "archived"},
            {from: "archived", to: "draft"},
        ]
    }
}

// Comment entity - user comments on posts
#Comment: schema.#Entity & {
    name: "Comment"
    description: "Comment on a blog post"

    fields: {
        id: {
            type: "uuid"
            description: "Unique identifier"
        }
        postID: {
            type: "uuid"
            description: "Reference to Post"
            ref: "Post"
        }
        authorID: {
            type: "uuid"
            description: "Reference to User who wrote the comment"
            ref: "User"
        }
        parentID: {
            type: "*uuid"
            description: "Parent comment for nested replies"
            optional: true
            ref: "Comment"
        }
        content: {
            type: "string"
            description: "Comment text"
            validate: "required,min=1,max=2000"
        }
        createdAt: {
            type: "time"
            description: "Creation timestamp"
        }
        updatedAt: {
            type: "time"
            description: "Last update timestamp"
        }
    }
}

// Tag entity - categorization for posts
#Tag: schema.#Entity & {
    name: "Tag"
    description: "Post categorization tag"

    fields: {
        id: {
            type: "uuid"
            description: "Unique identifier"
        }
        name: {
            type: "string"
            description: "Tag name"
            validate: "required,min=2,max=50"
        }
        slug: {
            type: "string"
            description: "URL-friendly identifier"
            validate: "required"
        }
        description: {
            type: "string"
            description: "Tag description"
            optional: true
        }
    }
}

// PostTag - many-to-many relationship
#PostTag: schema.#Entity & {
    name: "PostTag"
    description: "Post-Tag relationship"

    fields: {
        postID: {
            type: "uuid"
            ref: "Post"
        }
        tagID: {
            type: "uuid"
            ref: "Tag"
        }
    }
}
