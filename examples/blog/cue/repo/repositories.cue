// ============================================================================
// Repository Finders
// ============================================================================
// Demonstrates: Custom query methods for repositories
// ============================================================================

package repo

Repositories: {
    User: {
        finders: [
            {
                name: "FindByEmail"
                returns: "one"
                where: [{field: "email", op: "=", param: "email"}]
            },
        ]
    }

    Post: {
        finders: [
            {
                name: "FindBySlug"
                returns: "one"
                where: [{field: "slug", op: "=", param: "slug"}]
            },
            {
                name: "ListPublished"
                returns: "many"
                where: [{field: "status", op: "=", value: "'published'"}]
                order_by: "published_at DESC"
            },
            {
                name: "CountPublished"
                returns: "count"
                where: [{field: "status", op: "=", value: "'published'"}]
            },
            {
                name: "ListPublishedByTag"
                returns: "many"
                where: [
                    {field: "status", op: "=", value: "'published'"},
                    {field: "id", op: "IN", subquery: "SELECT post_id FROM post_tags pt JOIN tags t ON pt.tag_id = t.id WHERE t.slug = $tag"},
                ]
                order_by: "published_at DESC"
            },
            {
                name: "CountPublishedByTag"
                returns: "count"
                where: [
                    {field: "status", op: "=", value: "'published'"},
                    {field: "id", op: "IN", subquery: "SELECT post_id FROM post_tags pt JOIN tags t ON pt.tag_id = t.id WHERE t.slug = $tag"},
                ]
            },
            {
                name: "ListByAuthor"
                returns: "many"
                where: [{field: "author_id", op: "=", param: "authorID"}]
                order_by: "created_at DESC"
            },
            {
                name: "ListByAuthorAndStatus"
                returns: "many"
                where: [
                    {field: "author_id", op: "=", param: "authorID"},
                    {field: "status", op: "=", param: "status"},
                ]
                order_by: "created_at DESC"
            },
        ]
    }

    Tag: {
        finders: [
            {
                name: "FindBySlug"
                returns: "one"
                where: [{field: "slug", op: "=", param: "slug"}]
            },
            {
                name: "ListAll"
                returns: "many"
                order_by: "name ASC"
            },
            {
                name: "ListByPost"
                returns: "many"
                where: [{field: "id", op: "IN", subquery: "SELECT tag_id FROM post_tags WHERE post_id = $postID"}]
            },
        ]
    }

    PostTag: {
        finders: [
            {
                name: "DeleteByPost"
                action: "delete"
                where: [{field: "post_id", op: "=", param: "postID"}]
            },
            {
                name: "DeleteByTag"
                action: "delete"
                where: [{field: "tag_id", op: "=", param: "tagID"}]
            },
        ]
    }

    Comment: {
        finders: [
            {
                name: "ListByPost"
                returns: "many"
                where: [{field: "post_id", op: "=", param: "postID"}]
                order_by: "created_at ASC"
            },
            {
                name: "CountByPost"
                returns: "count"
                where: [{field: "post_id", op: "=", param: "postID"}]
            },
            {
                name: "DeleteByParent"
                action: "delete"
                where: [{field: "parent_id", op: "=", param: "parentID"}]
            },
            {
                name: "DeleteByPost"
                action: "delete"
                where: [{field: "post_id", op: "=", param: "postID"}]
            },
        ]
    }
}
