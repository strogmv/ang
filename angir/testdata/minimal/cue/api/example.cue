package api

ListUsers: {
    service: "user"
    input: {
        limit?: int
    }
    output: {
        data: [...{
            id: string
            email: string
        }]
    }
    sources: {
        users: {
            kind: "sql"
            entity: "User"
        }
    }
}
