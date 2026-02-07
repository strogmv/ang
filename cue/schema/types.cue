package schema

import "github.com/strogmv/ang/cue/project"

#Operation: {
	description?: string
	service: string // RECOMMENDED: lowercase (e.g., "tender", "user")
	testHints?: {
		happyPath?:  string
		errorCases?: [...string]
	}
	requiresS3?: bool
	input:  _
	output: _
	throws?: [...string]
	publishes?: [...string]
	broadcasts?: [...string]
	subscribes?: [string]: string
	uses?: [...string]
	pagination?: {
		type: "offset" | "cursor"
		default_limit?: int
		max_limit?: int
	}
	sources?: [string]: {
		kind: "sql" | "mongo" | "redis" | "s3"
		entity?: string // RECOMMENDED: PascalCase (e.g., "User", "Tender")
		collection?: string
		by?: [string]: string
		filter?: [string]: string
	}
	impls?: {
		go?: #CodeBlock & { lang: "go" }
		rust?: #CodeBlock & { lang: "rust" }
		ts?: #CodeBlock & { lang: "ts" }
	}
	// Declarative logic flow
	flow?: [...#FlowStep]

	// Resolved implementation for the current target language
	_impl?: #CodeBlock

	if project.state.target.lang == "go" {
		_impl: impls.go
	}
	if project.state.target.lang == "rust" {
		_impl: impls.rust
	}
	if project.state.target.lang == "ts" {
		_impl: impls.ts
	}

	// VALIDATION: If flow uses events, they must be declared in publishes
	// (This is informational - actual validation happens in compiler)
}

#CodeBlock: {
	lang: string
	tx?: bool
	code: string
	imports?: [...string] | string
}

// ============================================================================
// FLOW DSL DEFINITIONS
// ============================================================================
// The Flow DSL provides declarative orchestration for business logic.
// AI AGENTS: Use these definitions to understand valid flow step structures.
// ============================================================================

#FlowStep: #RepoStep | #CheckStep | #MapStep | #EventStep | #CustomStep | #StateStep | #MapActionStep | #IfStep | #ForStep | #BlockStep | #ListStep

// ----------------------------------------------------------------------------
// LIST OPERATIONS
// ----------------------------------------------------------------------------

#ListStep: {
	// list.Append - Add an item to a slice
	action: "list.Append"
	// Target slice (e.g., "resp.Data", "tender.Bids")
	to: string
	// Value to append (e.g., "newItem", "bid")
	item: string
}

// ----------------------------------------------------------------------------
// REPOSITORY OPERATIONS
// ----------------------------------------------------------------------------
// Use these to interact with domain entities through repositories.
// IMPORTANT: Variables from repo.Find/Get are POINTERS (*domain.Entity)
// IMPORTANT: Variables from mapping.Map with entity: are VALUES (domain.Entity)

#RepoStep: {
	// repo.Find   - Find by ID, returns nil if not found (triggers error if configured)
	// repo.Get    - Get by ID, expects entity to exist
	// repo.GetForUpdate - Get with row lock for updates (use inside tx.Block)
	// repo.Save   - Persist entity (create or update)
	// repo.Delete - Remove entity by ID
	// repo.List   - List entities with optional filter
	action: "repo.Save" | "repo.Find" | "repo.Delete" | "repo.List" | "repo.Get" | "repo.GetForUpdate"

	// Entity name from domain (e.g., "Tender", "User", "Company")
	// RECOMMENDED: PascalCase (e.g., "User", "APIKey", "TenderInvite")
	source: string

	// Expression for input (ID for Find/Get/Delete, entity variable for Save)
	// Examples: "req.ID", "key.CompanyID", "newTender", "existing"
	input: string

	// Variable name to store result (required for Find/Get/List)
	// This variable becomes a pointer: *domain.Entity
	// RECOMMENDED: lowercase identifier (e.g., "user", "tender")
	output?: string

	// Custom error message for Find when entity is nil
	// Example: "Invalid API Key", "Tender not found"
	error?: string

	// Method name for List operations (defaults to "FindAll")
	// Example: "FindByCompanyID", "FindActive"
	method?: string
}

// ----------------------------------------------------------------------------
// CONTROL FLOW
// ----------------------------------------------------------------------------

#IfStep: {
	action: "flow.If"
	// Go expression that evaluates to bool
	// Example: "tender.Status == \"draft\"", "user != nil"
	condition: string
	// Steps to execute when condition is true
	then: [...#FlowStep]
	// Optional steps when condition is false
	else?: [...#FlowStep]
}

#ForStep: {
	action: "flow.For"
	// Expression yielding slice to iterate
	// Example: "items", "tender.Bids"
	each: string
	// Loop variable name (will be value, not pointer)
	// Example: "item", "bid"
	as: string
	// Steps to execute for each element
	do: [...#FlowStep]
}

#BlockStep: {
	// tx.Block   - Wrap steps in database transaction (provides txCtx)
	// flow.Block - Group steps without transaction
	action: "tx.Block" | "flow.Block"
	// Steps to execute within the block
	// IMPORTANT for tx.Block: repo operations inside use txCtx automatically
	do: [...#FlowStep]
}

// ----------------------------------------------------------------------------
// VALIDATION & BUSINESS RULES
// ----------------------------------------------------------------------------

#CheckStep: {
	action: "logic.Check"
	// Go boolean expression - step fails if FALSE
	// Example: "company.ActiveTendersCount < company.MaxTendersLimit"
	// Example: "existing.Status == \"draft\""
	condition: string
	// Error message when condition fails (HTTP 400 Bad Request)
	// Example: "Tender limit reached (max 20)"
	throw: string
	// Optional parameters for error message formatting
	params?: [...string]
}

// ----------------------------------------------------------------------------
// MAPPING & ASSIGNMENT
// ----------------------------------------------------------------------------
// CRITICAL: These steps control variable creation and field assignment

#MapStep: {
	action: "mapping.Assign"
	// Target field or variable
	// Examples: "newTender.ID", "resp.TenderID", "company.ActiveTendersCount"
	to: string
	// Go expression for the value
	// Examples:
	//   "uuid.NewString()"                              - Generate UUID
	//   "time.Now().UTC().Format(time.RFC3339)"         - Current timestamp
	//   "req.Title"                                     - Copy from request
	//   "company.ActiveTendersCount + 1"               - Increment
	//   "\"draft\""                                     - String literal (escape quotes!)
	//   "true"                                          - Boolean literal
	value: string
	// Set to "true" to declare new variable (var x = value)
	// Use when creating new local variables, not when assigning to fields
	declare?: string | bool
	// Optional type hint for complex declarations
	type?: string
	// Internal: marks step as auto-generated by compiler
	generated?: string | bool
}

#MapActionStep: {
	action: "mapping.Map"
	// Source variable to map from
	// Aliases: input or from
	input?: string
	from?: string
	// Target variable to map to
	// Aliases: output or to
	// RECOMMENDED: Use "new" prefix for new entities (e.g., "newTender")
	output?: string
	to?: string
	// IMPORTANT: When entity is set and output starts with "new":
	// Declares a new VALUE variable: var newTender domain.Tender
	// Without entity, maps fields from source to target
	// RECOMMENDED: PascalCase (e.g., "Tender", "User")
	entity?: string
}

// ----------------------------------------------------------------------------
// EVENTS & MESSAGING
// ----------------------------------------------------------------------------

#EventStep: {
	action: "event.Publish" | "event.Broadcast"
	// Event name - must be declared in publishes: or broadcasts:
	// RECOMMENDED: PascalCase (e.g., "TenderCreatedByAI", "BidPlaced")
	name: string
	// Go expression constructing event payload
	// Example: "domain.TenderCreatedByAI{TenderID: newTender.ID, CompanyID: company.ID}"
	// RECOMMENDED: Start with "domain." for type safety
	payload?: string
	// For FileUploaded events specifically:
	fileID?: string
	url?: string
	kind?: string
}

// ----------------------------------------------------------------------------
// FUNCTION CALLS
// ----------------------------------------------------------------------------

#CustomStep: {
	action: "logic.Call"
	// Function to call (can be method or package function)
	// Example: "validateInput", "pkg.ProcessData"
	func: string
	// Arguments as single string or array
	// Example: "ctx, req" or ["ctx", "req", "options"]
	args?: string | [...string]
	// Variable to store result (if function returns value)
	output?: string
}

// ----------------------------------------------------------------------------
// STATE MACHINE
// ----------------------------------------------------------------------------

#StateStep: {
	action: "fsm.Transition"
	// Variable holding entity with FSM state
	// Example: "tender", "order"
	entity: string
	// Target state name
	// Example: "published", "closed", "cancelled"
	to: string
}

// --- HTTP & INFRA ---

#RateLimitDef: {
	rps:   int
	burst: int
}

#CircuitBreaker: {
	threshold:     int | *5      // сбоев до открытия
	timeout:       string | *"30s" // время в состоянии Open
	half_open_max: int | *3      // запросов в Half-Open
}

#SecurityProfile: "baseline" | "strict" | "pci"

#HTTP: {
	// Default rate limit applied to all endpoints without explicit rate_limit
	default_rate_limit?: #RateLimitDef
	// Default timeout applied to all endpoints without explicit timeout (e.g. "30s", "1m")
	default_timeout?: string
	// Default max body size (e.g. "1mb", "512kb"). Default is 1mb.
	default_max_body_size?: string | *"1mb"

	// Endpoints (keys starting with uppercase letter)
	[=~"^[A-Z]"]: {
		method: "GET" | "POST" | "PUT" | "DELETE" | "PATCH" | "WS"
		path:   string
		room?:  string
		description?: string
		view?:        string
		messages?: [...string] | {[string]: _}
		cache?: {
			ttl: string
			tags?: [...string]
		}
		invalidate?: [...string] // RPC methods to invalidate after this mutation
		optimistic_update?: string // GET RPC method to update optimistically
		rate_limit?:      #RateLimitDef
		circuit_breaker?: #CircuitBreaker
		timeout?:         string // Request timeout (e.g. "5s", "30s", "1m")
		max_body_size?:   string // Max request body size (e.g. "10mb", "100kb")
		idempotency?: bool
		auth?: {
			type:    "jwt"
			action?: string
			check?:  string
			roles?: [...string]
			inject?: [...string] | string
		}
	}
}

#Services: {
	[string]: {
		description?: string
		storage: "sql" | "mongo" | "redis"
		owns: [...string]
	}
}

#Schedules: {
	[string]: {
		service: string
		action: string
		at: string
		publish?: string
		every?: string
		payload?: [string]: _
	}
}

#AppConfig: {
	[string]: _
	security_profile?: #SecurityProfile | *"baseline"
	image_proxy?: {
		enabled: bool
		url:     string
		key:     string
		salt:    string
	}
}

// ============================================================================
// BEHAVIORAL SCENARIOS (Stage 31)
// ============================================================================

#Scenario: {
	description?: string
	steps: [...#ScenarioStep]
}

#ScenarioStep: {
	name: string
	action: string // e.g. "Auth.Register", "Blog.CreatePost"
	input: _
	expect?: {
		status?: int | *200
		body?: _ // partial matching of response body
	}
	export?: [string]: string // map response fields to scenario variables
}
