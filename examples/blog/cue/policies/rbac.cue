// ============================================================================
// RBAC Policies
// ============================================================================
// Demonstrates: Role-based access control
// ============================================================================

package policies

#RBAC: {
    roles: {
        reader: {
            description: "Can read posts and comments, write comments"
            permissions: [
                "post:read",
                "comment:read",
                "comment:create",
                "comment:update:own",
                "comment:delete:own",
            ]
        }

        author: {
            description: "Can create and manage own posts"
            inherits: ["reader"]
            permissions: [
                "post:create",
                "post:update:own",
                "post:delete:own",
                "post:submit:own",
            ]
        }

        admin: {
            description: "Full access to all resources"
            permissions: ["*"]
        }
    }

    resources: {
        post: {
            actions: ["create", "read", "update", "delete", "submit", "publish", "archive"]
        }
        comment: {
            actions: ["create", "read", "update", "delete"]
        }
        user: {
            actions: ["read", "update", "delete"]
        }
    }
}
