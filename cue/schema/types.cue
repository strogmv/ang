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
		python?: #CodeBlock & { lang: "python" }
		rust?: #CodeBlock & { lang: "rust" }
		ts?: #CodeBlock & { lang: "ts" }
	}
	// Explicit implementation block (used when a target-specific resolver is not available)
	impl?: #CodeBlock
	// Declarative logic flow
	flow?: [...#FlowStep]

	// Resolved implementation for the current target language
	_impl?: #CodeBlock

	if project.state.target.lang == "go" {
		_impl: impls.go
	}
	if project.state.target.lang == "python" {
		_impl: impls.python
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

#FlowStep: #RepoStep | #CheckStep | #MapStep | #EventStep | #CustomStep | #StateStep | #MapActionStep | #IfStep | #ForStep | #WhileStep | #BlockStep | #ListStep | #AuditStep | #AuthStep | #PatchStep | #PaginateStep | #NormalizeStep | #EnumValidateStep | #SortStep | #FilterStep | #TimeParseStep | #TimeCheckExpiryStep | #MapBuildStep

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
// AUDIT LOGGING
// ----------------------------------------------------------------------------

#AuditStep: {
	// audit.Log - Create audit trail entry
	action: "audit.Log"
	// Expression for actor user ID (e.g., "req.UserID")
	actor: string
	// Expression for company ID (e.g., "req.CompanyID")
	company: string
	// Event name string (e.g., "company.category.added")
	event: string
}

// ----------------------------------------------------------------------------
// AUTHORIZATION
// ----------------------------------------------------------------------------

#AuthStep: {
	// auth.RequireRole - Role-based authorization guard
	action: "auth.RequireRole"
	// Expression for user ID to authorize (e.g., "req.UserID")
	userID: string
	// Expression for company ID to check (e.g., "req.CompanyID")
	companyID: string
	// Allowed roles as Go string literals (e.g., "\"owner\", \"admin\"")
	roles: string
	// Variable name for loaded user (default: "currentUser")
	output?: string
	// If true (default), admin can access any company
	adminBypass?: bool | *true
}

// ----------------------------------------------------------------------------
// PARTIAL UPDATE
// ----------------------------------------------------------------------------

#PatchStep: {
	// entity.PatchNonZero - Apply non-zero field values (PATCH semantics)
	action: "entity.PatchNonZero"
	// Target variable to patch (e.g., "tender")
	target: string
	// Source variable with new values (e.g., "req")
	from: string
	// Comma-separated field names (e.g., "Title, Description, Status")
	fields: string
}

// ----------------------------------------------------------------------------
// PAGINATION
// ----------------------------------------------------------------------------

#PaginateStep: {
	// list.Paginate - In-memory pagination with bounds checking
	action: "list.Paginate"
	// Source slice expression (e.g., "filtered")
	input: string
	// Offset expression (e.g., "req.Offset")
	offset: string
	// Limit expression (e.g., "req.Limit")
	limit: string
	// Default limit when <= 0 (default: 50)
	defaultLimit?: int | *50
	// Variable name for paginated result slice
	output: string
	// Optional: assign total count (e.g., "resp.Total")
	total?: string
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

#WhileStep: {
	action: "flow.While"
	// Go condition string
	condition: string
	// Steps to execute while true
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

// ----------------------------------------------------------------------------
// STRING OPERATIONS
// ----------------------------------------------------------------------------

#NormalizeStep: {
	// str.Normalize - Normalize string (lower/upper/trim)
	action: "str.Normalize"
	// Go expression for input string (e.g., "req.Slug", "req.Email")
	input: string
	// Normalization mode: lower = ToLower+TrimSpace, upper = ToUpper+TrimSpace, trim = TrimSpace only
	mode?: "lower" | "upper" | "trim" | *"lower"
	// Variable name to store result (e.g., "slug", "email")
	output: string
}

// ----------------------------------------------------------------------------
// ENUM VALIDATION
// ----------------------------------------------------------------------------

#EnumValidateStep: {
	// enum.Validate - Validate value against allowed set
	action: "enum.Validate"
	// Go expression for value to check (e.g., "role", "req.Status")
	value: string
	// Comma-separated allowed values (e.g., "owner, admin, employee")
	allowed: string
	// Error message when value is not in allowed set
	throw: string
}

// ----------------------------------------------------------------------------
// SORTING
// ----------------------------------------------------------------------------

#SortStep: {
	// list.Sort - Sort a slice by field
	action: "list.Sort"
	// Slice expression (e.g., "items", "resp.Data")
	items: string
	// Field name to sort by (e.g., "CreatedAt", "Name")
	by: string
	// Sort descending (default true)
	desc?: bool | *true
}

// ----------------------------------------------------------------------------
// FILTERING
// ----------------------------------------------------------------------------

#FilterStep: {
	// list.Filter - Filter a slice by condition
	action: "list.Filter"
	// Source slice expression (e.g., "items", "notifications")
	from: string
	// Loop variable name (default "item")
	as?: string
	// Go boolean expression for filter (e.g., "item.Status != \"deleted\"")
	condition: string
	// Variable name for filtered result (e.g., "filtered")
	output: string
}

// ----------------------------------------------------------------------------
// TIME PARSING
// ----------------------------------------------------------------------------

#TimeParseStep: {
	// time.Parse - Parse time string into time.Time
	action: "time.Parse"
	// String expression to parse (e.g., "evt.ExpiresAt", "req.StartDate")
	value: string
	// Variable name for parsed time (e.g., "expiresAt", "startDate")
	output: string
	// Go time format constant (default "time.RFC3339")
	format?: string
}

// ----------------------------------------------------------------------------
// EXPIRY CHECK
// ----------------------------------------------------------------------------

#TimeCheckExpiryStep: {
	// time.CheckExpiry - Parse time and check if it's in the future or past
	action: "time.CheckExpiry"
	// String expression to parse (e.g., "token.ExpiresAt")
	value: string
	// Error message when check fails
	throw: string
	// "future" = value must be in the future, "past" = value must be in the past
	mustBe?: "future" | "past" | *"future"
}

// ----------------------------------------------------------------------------
// MAP BUILDING
// ----------------------------------------------------------------------------

#MapBuildStep: {
	// map.Build - Build a map[string]string from a slice
	action: "map.Build"
	// Source slice expression (e.g., "users", "categories")
	from: string
	// Loop variable name (default "item")
	as?: string
	// Go expression for map key (e.g., "item.ID", "item.Slug")
	key: string
	// Go expression for map value (e.g., "item.Name", "item")
	value: string
	// Variable name for result map (e.g., "userByID", "nameBySlug")
	output: string
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
