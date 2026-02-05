// ============================================================================
// Blog Service Architecture
// ============================================================================
// Demonstrates: Service definitions, dependencies, event publishing
// ============================================================================

package architecture

#Services: {
    auth: {
        name: "Auth"
        description: "Authentication and authorization"
        entities: ["User"]
        publishes: ["UserRegistered", "UserLoggedIn"]
    }

    blog: {
        name: "Blog"
        description: "Blog post management"
        entities: ["Post", "Tag", "PostTag"]
        depends: ["auth"]
        publishes: ["PostCreated", "PostPublished", "PostUpdated"]
    }

    comment: {
        name: "Comment"
        description: "Comment management"
        entities: ["Comment"]
        depends: ["auth", "blog"]
        publishes: ["CommentCreated"]
    }
}

// Event definitions
#Events: {
    UserRegistered: {
        description: "Fired when a new user registers"
        payload: {
            userID: "uuid"
            email: "string"
        }
    }

    UserLoggedIn: {
        description: "Fired on successful login"
        payload: {
            userID: "uuid"
        }
    }

    PostCreated: {
        description: "Fired when a new post is created"
        payload: {
            postID: "uuid"
            authorID: "uuid"
            title: "string"
        }
    }

    PostPublished: {
        description: "Fired when a post is published"
        payload: {
            postID: "uuid"
            authorID: "uuid"
            title: "string"
            slug: "string"
        }
    }

    PostUpdated: {
        description: "Fired when a post is updated"
        payload: {
            postID: "uuid"
        }
    }

    CommentCreated: {
        description: "Fired when a comment is added"
        payload: {
            commentID: "uuid"
            postID: "uuid"
            authorID: "uuid"
        }
    }
}
