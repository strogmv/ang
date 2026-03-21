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

SendNoticeEmail: schema.#Operation & {
    service: "notifications"
    description: "Send a templated email using the universal notification module"
    primary_operation_kind: "notify"
    capabilities: ["notify"]
    side_effects: [{kind: "notify.email", channel: "email", template: "generic_notice"}]

    input: {
        email: string @validate("required,email")
        name?: string
        title: string @validate("required,min=3,max=120")
        body: string @validate("required,min=5,max=1000")
    }

    output: {
        ok: bool
    }

    flow: [
        {action: "notify.Email", to: "req.Email", template: "\"generic_notice\"", data: "map[string]any{\"Name\": req.Name, \"Title\": req.Title, \"Body\": req.Body, \"AppName\": \"ANG Minimal Example\", \"SupportEmail\": \"minimal-support@ang.local\"}"},
        {action: "mapping.Assign", to: "resp.Ok", value: "true"},
    ]
}

SendPasswordResetEmail: schema.#Operation & {
    service: "notifications"
    description: "Send a password reset email using the universal notification module"
    primary_operation_kind: "notify"
    capabilities: ["notify"]
    side_effects: [{kind: "notify.email", channel: "email", template: "password_reset"}]

    input: {
        email: string @validate("required,email")
        name?: string
        resetURL: string @validate("required,min=12,max=500")
    }

    output: {
        ok: bool
    }

    flow: [
        {action: "notify.Email", to: "req.Email", template: "\"password_reset\"", data: "map[string]any{\"Name\": req.Name, \"ResetURL\": req.ResetURL, \"AppName\": \"ANG Minimal Example\", \"SupportEmail\": \"minimal-support@ang.local\"}"},
        {action: "mapping.Assign", to: "resp.Ok", value: "true"},
    ]
}

SendInvitationEmail: schema.#Operation & {
    service: "notifications"
    description: "Send an invitation email using the universal notification module"
    primary_operation_kind: "notify"
    capabilities: ["notify"]
    side_effects: [{kind: "notify.email", channel: "email", template: "invitation_email"}]

    input: {
        email: string @validate("required,email")
        name?: string
        inviterName: string @validate("required,min=2,max=120")
        inviteURL: string @validate("required,min=12,max=500")
    }

    output: {
        ok: bool
    }

    flow: [
        {action: "notify.Email", to: "req.Email", template: "\"invitation_email\"", data: "map[string]any{\"Name\": req.Name, \"InviterName\": req.InviterName, \"InviteURL\": req.InviteURL, \"AppName\": \"ANG Minimal Example\", \"SupportEmail\": \"minimal-support@ang.local\"}"},
        {action: "mapping.Assign", to: "resp.Ok", value: "true"},
    ]
}
