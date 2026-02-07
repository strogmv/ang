package api

import "github.com/strogmv/ang/cue/schema"

ScenarioUserSignup: schema.#Scenario & {
    description: "Full user registration and login flow"
    steps: [
        {
            name: "SignUp"
            action: "Auth.Register"
            input: {
                email: "ai@test.com"
                password: "safePassword123"
                name: "AI Agent"
            }
            export: {
                userID: "id"
            }
        },
        {
            name: "Login"
            action: "Auth.Login"
            input: {
                email: "ai@test.com"
                password: "safePassword123"
            }
            expect: {
                status: 200
            }
        }
    ]
}
