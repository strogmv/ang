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
	        subscribes: {
	            // String form defaults to queue delivery across replicas.
	            UserRegistered: "OnUserRegisteredAudit"
	            // Object form can opt into broadcast explicitly.
	            UserLoggedIn: {op: "OnUserLoggedInAudit", delivery: "broadcast"}
	        }
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

    notifications: {
        name: "Notifications"
        description: "Transactional email delivery and templated outbound notifications"
        depends: ["auth"]
    }

    assistant: {
        name: "Assistant"
        description: "AI assistant operations with explicit tool allowlists"
        entities: ["Post"]
        depends: ["auth", "blog"]
    }
}

// Event definitions
#Events: {
	    UserRegistered: {
	        owner: "auth"
	        description: "Fired when a new user registers"
	        payload: {
	            userID: "uuid"
            email: "string"
        }
    }

	    UserLoggedIn: {
	        owner: "auth"
	        description: "Fired on successful login"
        payload: {
            userID: "uuid"
        }
    }

	    PostCreated: {
	        owner: "blog"
	        description: "Fired when a new post is created"
        payload: {
            postID: "uuid"
            authorID: "uuid"
            title: "string"
        }
    }

	    PostPublished: {
	        owner: "blog"
	        description: "Fired when a post is published"
        payload: {
            postID: "uuid"
            authorID: "uuid"
            title: "string"
            slug: "string"
        }
    }

	    PostUpdated: {
	        owner: "blog"
	        description: "Fired when a post is updated"
        payload: {
            postID: "uuid"
        }
    }

	    CommentCreated: {
	        owner: "blog"
	        description: "Fired when a comment is added"
        payload: {
            commentID: "uuid"
            postID: "uuid"
            authorID: "uuid"
        }
    }
}
