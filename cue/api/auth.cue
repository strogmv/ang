// ============================================================================
// Auth API Operations
// ============================================================================
// Demonstrates: User registration, login, JWT tokens
// ============================================================================

package api

import "github.com/strogmv/ang/cue/schema"

// Register a new user
Register: schema.#Operation & {
	service:   "auth"
	description: "Register a new user account"
	publishes: ["UserRegistered"]

	input: {
		email:    string @validate("required,email")
		password: string @validate("required,min=8,max=64")
		name:     string @validate("required,min=2,max=100")
	}

	output: {
		id:    string
		email: string
		name:  string
	}

	flow: [
		// Check if email already exists
		{action: "repo.Find", source: "User", method: "FindByEmail", input: "req.Email", output: "existing"},
		{action: "logic.Check", condition: "existing == nil", throw: "Email already registered"},

		// Create new user
		{action: "mapping.Map", output: "newUser", entity: "User"},
		{action: "mapping.Assign", to: "newUser.Email", value: "req.Email"},
		{action: "mapping.Assign", to: "newUser.Name", value: "req.Name"},
		{action: "logic.Call", func: "hashPassword", args: "req.Password", output: "hash"},
		{action: "mapping.Assign", to: "newUser.PasswordHash", value: "hash"},
		{action: "mapping.Assign", to: "newUser.Role", value: "\"reader\""},

		// Save and respond
		{action: "repo.Save", source: "User", input: "newUser"},
		{action: "event.Publish", name: "UserRegistered", payload: "domain.UserRegistered{UserID: newUser.ID, Email: newUser.Email}"},

		{action: "mapping.Assign", to: "resp.ID", value: "newUser.ID"},
		{action: "mapping.Assign", to: "resp.Email", value: "newUser.Email"},
		{action: "mapping.Assign", to: "resp.Name", value: "newUser.Name"},
	]
}

// Login and get JWT tokens
Login: schema.#Operation & {
	service:   "auth"
	description: "Authenticate and receive JWT tokens"
	publishes: ["UserLoggedIn"]

	input: {
		email:    string @validate("required,email")
		password: string @validate("required")
	}

	output: {
		accessToken:  string
		refreshToken: string
		user: {
			id:    string
			email: string
			name:  string
			role:  string
		}
	}

	flow: [
		// Find user by email
		{action: "repo.Find", source: "User", method: "FindByEmail", input: "req.Email", output: "user", error: "Invalid credentials"},

		// Verify password
		{action: "logic.Call", func: "checkPassword", args: ["req.Password", "user.PasswordHash"], output: "valid"},
		{action: "logic.Check", condition: "valid", throw: "Invalid credentials"},

		// Generate tokens
		{action: "logic.Call", func: "generateTokens", args: "user", output: "tokens"},

		// Publish event and respond
		{action: "event.Publish", name: "UserLoggedIn", payload: "domain.UserLoggedIn{UserID: user.ID}"},

		{action: "mapping.Assign", to: "resp.AccessToken", value: "tokens.AccessToken"},
		{action: "mapping.Assign", to: "resp.RefreshToken", value: "tokens.RefreshToken"},
		{action: "mapping.Assign", to: "resp.User.ID", value: "user.ID"},
		{action: "mapping.Assign", to: "resp.User.Email", value: "user.Email"},
		{action: "mapping.Assign", to: "resp.User.Name", value: "user.Name"},
		{action: "mapping.Assign", to: "resp.User.Role", value: "user.Role"},
	]
}

// Get current user profile
GetProfile: schema.#Operation & {
	service:   "auth"
	description: "Get current user's profile"

	input: {
		userId: string
	}

	output: {
		id:        string
		email:     string
		name:      string
		role:      string
		avatarURL?: string
		createdAt: string
	}

	flow: [
		{action: "repo.Find", source: "User", input: "req.UserId", output: "user", error: "User not found"},
		{action: "mapping.Assign", to: "resp.ID", value: "user.ID"},
		{action: "mapping.Assign", to: "resp.Email", value: "user.Email"},
		{action: "mapping.Assign", to: "resp.Name", value: "user.Name"},
		{action: "mapping.Assign", to: "resp.Role", value: "user.Role"},
		{action: "mapping.Assign", to: "resp.AvatarURL", value: "user.AvatarURL"},
		{action: "mapping.Assign", to: "resp.CreatedAt", value: "user.CreatedAt"},
	]
}

// Update user profile
UpdateProfile: schema.#Operation & {
	service:   "auth"
	description: "Update current user's profile"

	input: {
		userId:    string
		name?:      string @validate("min=2,max=100")
		avatarURL?: string
	}

	output: {
		ok: bool
	}

	flow: [
		{action: "repo.Find", source: "User", input: "req.UserId", output: "user", error: "User not found"},
		{action: "flow.If", condition: "req.Name != \"\"", then: [
			{action: "mapping.Assign", to: "user.Name", value: "req.Name"},
		]},
		{action: "flow.If", condition: "req.AvatarURL != \"\"", then: [
			{action: "mapping.Assign", to: "user.AvatarURL", value: "req.AvatarURL"},
		]},
		{action: "repo.Save", source: "User", input: "user"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}