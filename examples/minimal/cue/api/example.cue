package api

import "github.com/strogmv/ang/cue/schema"

ListUsers: schema.#Operation & {
    service: "user"
    input: {
        limit?: int
        offset?: int
    }
    output: {
        data: [...{
            id: string
            email: string
            name: string
        }]
    }
    sources: {
        users: {
            kind: "sql"
            entity: "User"
        }
    }
}
