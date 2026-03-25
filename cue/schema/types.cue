package schema

import "github.com/strogmv/ang/cue/project"

#OperationKind: "create" | "get" | "list" | "update" | "delete" | "transition" | "notify" | "auth" | "message" | "upload"

#CapabilityKind: "auth" | "profile" | "media" | "notify" | "messaging" | "moderation" | "search" | "commerce" | "catalog" | "payment"

#SideEffect: {
	// Canonical kinds:
	// - "notify.email"
	// - "notify.sms" (reserved for future lowering)
	// - "publish_event"
	// - "notify_user"
	// - "create_review"
	// - "upload_media"
	// Legacy aliases such as "send_email" and "upload_file" are accepted and normalized by the compiler.
	kind: string
	channel?: string
	event?: string
	template?: string
	target_field?: string
	targetField?: string
}

#Operation: {
	description?: string
	service: string // RECOMMENDED: lowercase (e.g., "tender", "user")
	primary_operation_kind?: #OperationKind
	capabilities?: [...#CapabilityKind]
	side_effects?: [...#SideEffect] | [...string]
	manual_required?: bool
	// Planner-facing canonical hints. Upper-layer generators such as sandbox
	// should populate these explicitly instead of relying on ANG emitter
	// heuristics to invent routes or repository bindings from names.
	planner?: {
		source_pack?: string
		route?:      #RouteBinding
		repository?: #RepositoryBinding
	}
	testHints?: {
		happyPath?:  string
		errorCases?: [...string]
	}
	requiresS3?: bool
	stream?:     bool
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
	// Typed implementation steps (platform-agnostic)
	impl_steps?: [...#ImplStep]
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

// #SubFlow is an internal reusable operation intended to be called from flow.Call.
// It shares the same shape as #Operation but is not required to expose HTTP routes.
#SubFlow: #Operation & {
	internal?: bool | *true
}

#CodeBlock: {
	lang: string
	tx?: bool
	flowFirstBypass?: bool
	code: string
	imports?: [...string] | string
}

// Typed authorization policy registry (place in cue/policy/*.cue).
#Policy: {
	description?: string
	roles?: [...string]
	sameCompany?: bool | *false
	allowAdminOverride?: bool | *true
}

#Policies: [string]: #Policy

// Read-model contract for cross-context analytics (ACL projection).
// Define read models in cue/projections and access them from flow via repo.*.
#ReadModel: {
	owner:           string
	bounded_context: string
	read_model: {
		// source bounded context that owns the original write model
		source_context: string
		// events that refresh this projection
		refreshOn: [...string]
	}
	...
}

// ============================================================================
// FLOW DSL DEFINITIONS
// ============================================================================
// The Flow DSL provides declarative orchestration for business logic.
// AI AGENTS: Use these definitions to understand valid flow step structures.
// ============================================================================

#FlowStep: #RepoStep | #RepoExistsStep | #RepoCountStep | #DbStep | #UpsertStep | #CheckStep | #MapStep | #ValueCoalesceStep | #ErrorNewStep | #ErrorThrowIfStep | #ErrorWrapStep | #ErrorMapStep | #EventStep | #EventEmitIfStep | #EventOutboxStep | #CustomStep | #ServiceCallStep | #FlowCallStep | #StateStep | #ExplicitStateStep | #MapActionStep | #MapGetStep | #MapHasStep | #MapSetStep | #MapMergeStep | #IfStep | #SwitchStep | #ForStep | #WhileStep | #BlockStep | #ListStep | #ListLenStep | #ListNewStep | #ListFindStep | #ListAnyStep | #ListAllStep | #AuditStep | #AuthStep | #CheckRoleStep | #RBACCheckPermissionStep | #PolicyCheckStep | #PolicyEvaluateStep | #PolicyRequireStep | #PolicyDecideStep | #JWTSignStep | #JWTVerifyStep | #TokenGenerateStep | #TokenVerifyStep | #OAuth2TokenStep | #OAuth2RefreshStep | #EncryptStep | #DecryptStep | #HashStep | #PatchStep | #PatchValidatedStep | #CopyNonEmptyStep | #PaginateStep | #NormalizeStep | #EnumValidateStep | #SortStep | #FilterStep | #ListMapStep | #ListReduceStep | #ListGroupByStep | #ListDistinctStep | #ListChunkStep | #BatchRunStep | #EnrichStep | #TimeNowStep | #TimeParseStep | #TimeFormatStep | #TimeInZoneStep | #TimeAddStep | #TimeSubStep | #TimeDiffStep | #TimeCheckExpiryStep | #MapBuildStep | #MapNewStep | #ExecRunStep | #ExecStreamStep | #FSTempDirStep | #FSWriteFileStep | #FSReadFileStep | #FSRemoveStep | #ArchiveZipDirStep | #SessionGetStep | #CacheGetStep | #CacheSetStep | #CacheDelStep | #MailSendStep | #NotifySendStep | #NotifyEmailStep | #NotifyDispatchStep | #ApprovalRequestStep | #ApprovalWaitStep | #ApprovalDecideStep | #StorageUploadStep | #StorageDownloadStep | #StorageDeleteStep | #StorageListStep | #StorageGetURLStep | #WebhookSendStep | #WebhookVerifySignatureStep | #WebhookAckStep | #QueueEnqueueStep | #QueueDequeueStep | #QueueAckStep | #QueueNackStep | #DLQPublishStep | #HTTPCallStep | #HTTPRequestStep | #HTTPRetryPolicyStep | #HTTPPaginateStep | #RandCodeStep | #RandTokenStep | #StrFormatStep | #StrConcatStep | #TemplateRenderStep | #StrStripMarkdownStep | #StrReplaceAllStep | #StrTrimSpaceStep | #CastToStringStep | #NumBinaryStep | #MathExprStep | #MathOpStep | #RegexMatchStep | #RegexReplaceStep | #Base64EncodeStep | #Base64DecodeStep | #URLParseStep | #URLBuildStep | #PathBaseStep | #QueryEncodeStep | #QueryDecodeStep | #HashSumStep | #HashHMACStep | #UUIDNewStep | #ULIDNewStep | #JSONParseStep | #JSONStringifyStep | #JSONMarshalStep | #JSONPathGetStep | #JSONPathSetStep | #ParallelStep | #PDFRenderStep | #FlowParallelStep | #FlowJoinStep | #FlowRaceStep | #FlowDelayStep | #FlowScheduleStep | #FlowCronStep | #SecretGetStep | #ConfigGetStep | #ModelResolveStep | #StreamEmitStep | #EventWaitStep | #EventSubscribeStep | #EventMatchStep | #FlowSagaStep | #FlowCompensateStep | #FlowRollbackStep | #FlowDeferStep | #FlowTagStep | #FlowCheckpointStep | #FlowResumeStep | #FlowRecordEventStep | #FlowReplayStep | #FlowHistoryGetStep | #FlowValidateStep | #FlowTryStep | #FlowCatchStep | #FlowRetryStep | #FlowFallbackStep | #FlowTimeoutStep | #FlowSuggestNextStep | #FlowExplainErrorStep | #IdemDeriveKeyStep | #IdemCheckStep | #IdemSaveResultStep | #IdempotencyDeriveKeyStep | #IdempotencyCheckStep | #IdempotencySaveResultStep | #DedupeOnceStep | #RateLimitCheckStep | #RateLimitLimitStep | #QuotaCheckStep | #BudgetCheckStep | #BudgetConsumeStep | #ContextTrimStep | #ProfileRequireStep | #ConcurrencyLimitStep | #ConcurrencyRunStep | #MutexWithStep | #CircuitCheckStep | #CircuitRecordSuccessStep | #CircuitRecordFailureStep | #CircuitBreakerStep | #BulkheadAcquireStep | #BulkheadRunStep | #LogEmitStep | #MetricEmitStep | #TraceSpanStep | #SLOBudgetStep | #ListSumStep | #ListAvgStep | #FlowReturnStep | #ConvertToFloatStep | #ConvertToIntStep | #ClaudeChatStep | #OpenAIChatStep | #OpenAIEmbedStep | #OpenAIStreamStep | #PlanBuildAutomataStep | #PlanBuildMicroPlanStep | #CueEmitProjectStep | #CueValidateProjectStep | #CueWriteProjectFilesStep | #OAuthGoogleGetURLStep | #OAuthGoogleExchangeStep | #OAuthGoogleUserInfoStep | #LocaleResolveStep

#FlowCheckpointStep: {
	// flow.Checkpoint - Save current flow state for potential resumption
	action: "flow.Checkpoint"
	// Unique name for this checkpoint
	name: string
	// Expression for data to save (default: "req")
	data?: string
}

#FlowResumeStep: {
	// flow.Resume - Restore state from a previously saved checkpoint
	action: "flow.Resume"
	// Checkpoint name to restore from
	name: string
	// Variable name to store restored data
	output: string
	// Optional Go type for restored payload (e.g. "map[string]any", "port.EventPayload")
	into?: string
	// Steps to execute if checkpoint is missing
	onMissing?: [...#FlowStep]
}

#FlowRecordEventStep: {
	// flow.RecordEvent - append structured event to deterministic in-flow history
	action: "flow.RecordEvent"
	// Event name/key (for replay/debug/audit)
	name: string
	// Optional payload expression
	payload?: string
	// Optional variable for recorded event object
	output?: string
}

#FlowReplayStep: {
	// flow.Replay - load pre-recorded history and optionally execute steps in replay mode
	action: "flow.Replay"
	// Expression returning []map[string]any or []any
	history: string
	// Optional output variable for loaded history
	output?: string
	// Optional steps to execute while replay mode is enabled
	do?: [...#FlowStep]
	// Optional fallback when history shape is invalid
	onMismatch?: [...#FlowStep]
}

#FlowHistoryGetStep: {
	// flow.History.Get - read current in-flow execution history
	action: "flow.History.Get"
	// Optional filter by event name expression
	name?: string
	// Optional limit expression/int; keeps last N items
	limit?: int | string
	// Output variable for []map[string]any history records
	output: string
}

#FlowValidateStep: {
	// flow.Validate - Perform semantic validation, returns error if condition is false
	action: "flow.Validate"
	// Go boolean expression
	condition: string
	// Custom error message
	throw?: string
	// Optional hint for the user/AI
	hint?: string
}

#FlowTryStep: {
	// flow.Try - Execute steps with error handling and optional retries
	action: "flow.Try"
	do: [...#FlowStep]
	catch?: [...#FlowStep]
	retries?:   int
	backoffMs?: int
}

#FlowCatchStep: {
	// flow.Catch - Execute handler steps when _flowLastError is set
	action: "flow.Catch"
	do: [...#FlowStep]
}

#FlowRetryStep: {
	// flow.Retry - Simple retry block for transient operations
	action: "flow.Retry"
	attempts: int
	backoffMs?: int
	do: [...#FlowStep]
}

#FlowFallbackStep: {
	// flow.Fallback - Try primary path, execute fallback if it fails
	action: "flow.Fallback"
	do: [...#FlowStep]
	fallback: [...#FlowStep]
}

#FlowTimeoutStep: {
	// flow.Timeout - Limit execution time for a block of steps
	action: "flow.Timeout"
	// Duration expression (e.g. "2*time.Second")
	duration: string
	do: [...#FlowStep]
	// Steps to run if timeout occurs
	onTimeout?: [...#FlowStep]
}

#FlowSuggestNextStep: {
	// flow.SuggestNext - Provide user-facing options for next steps (AI-friendly)
	action: "flow.SuggestNext"
	options: [...string]
	output: string
}

#FlowExplainErrorStep: {
	// flow.ExplainError - Generate natural language explanation for an error (AI-friendly)
	action: "flow.ExplainError"
	error?: string
	output: string
	hint?:  string
}

#FlowTagStep: {
	// flow.Tag - Add execution metadata/tags for observability (tracing, logs)
	action: "flow.Tag"
	// Tag name/key (e.g. "order_id", "priority")
	name: string
	// Optional tag value expression (e.g. "req.ID", "\"high\"")
	value?: string
}

#ExplicitStateStep: {
	// state.Get - Retrieve a value from explicit state storage (KV store)
	action: "state.Get"
	// Key to lookup
	key: string
	// Variable name to store the result
	output: string
	// Optional default value if key not found
	"default"?: string
} | {
	// state.Set - Store a value in explicit state storage
	action: "state.Set"
	// Key to store under
	key: string
	// Value expression to store
	value: string
	// Optional TTL expression (e.g. "24*time.Hour")
	ttl?: string
} | {
	// state.Delete - Remove a value from explicit state storage
	action: "state.Delete"
	// Key to remove
	key: string
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

// ============================================================================
// IMPL STEPS (typed business logic inside impl)
// ============================================================================

#ImplStep: #ImplLoadStep | #ImplAssertStep | #ImplCallStep | #ImplEmitStep

#ImplLoadStep: {
	load: string        // e.g. "repo.Tender" or "repo.User"
	by: string | { [string]: string } // lookup criteria: "req.TenderID" or {id: "req.TenderID"}
	into: string        // variable name to bind (e.g. "tender")
}

#ImplAssertStep: {
	assert: string      // boolean expression, e.g. "tender.Status == 'active'"
	error:  string      // error code/identifier, e.g. "ErrTenderNotActive"
}

#ImplCallStep: {
	call: string        // e.g. "repo.Bid.Create" or "service.Notification.Send"
	with?: string | { [string]: _ } // arguments map or expression
	into?: string       // optional variable to store result
}

#ImplEmitStep: {
	emit: string        // event name, e.g. "BidPlaced"
	payload?: string | { [string]: _ } // payload expression or struct literal
}

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

#ListLenStep: {
	// list.Len - Compute len(input) into output
	action: "list.Len"
	input:  string
	output: string
}

#ListNewStep: {
	// list.New - Initialize empty slice with explicit type
	action: "list.New"
	output: string
	type:   string
	cap?:   int | string
}

#ListFindStep: {
	action: "list.Find"
	from:   string
	as?:    string
	condition: string
	output: string
	into?:  string
	found?: string
}

#ListAnyStep: {
	action: "list.Any"
	from:   string
	as?:    string
	condition: string
	output: string
}

#ListAllStep: {
	action: "list.All"
	from:   string
	as?:    string
	condition: string
	output: string
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

#CheckRoleStep: {
	// auth.CheckRole - Role-based guard for already loaded user variable
	action: "auth.CheckRole"
	// Expression of already loaded user variable (e.g., "currentUser", "user")
	user: string
	// Allowed roles as Go string literals (e.g., "\"owner\", \"admin\"")
	roles: string
	// Optional company scope expression (e.g., "req.CompanyID")
	// When provided: user.CompanyID must match, admin is allowed to bypass
	companyID?: string
}

#RBACCheckPermissionStep: {
	// rbac.CheckPermission - Permission/scope check (beyond role name checks)
	action: "rbac.CheckPermission"
	// Expression of loaded user variable (e.g., "currentUser")
	user: string
	// Permission expression (e.g., "\"project.create\"" or "req.Permission")
	permission: string
	// Optional output bool variable for check result
	output?: string
	// Optional error message when permission is denied
	throw?: string
	// Optional error code (default FORBIDDEN)
	code?: string
	// Optional HTTP status expression (default http.StatusForbidden)
	status?: string
}

#PolicyCheckStep: {
	// policy.Check - enforce typed policy from cue/policy registry.
	action: "policy.Check"
	// Policy key from #Policies map (e.g. "CompanyAdminOnly").
	policy: string
	// Expression of loaded user variable (e.g., "currentUser")
	user: string
	// Optional company scope expression (required for sameCompany=true policies)
	companyID?: string
	// Optional output bool variable with check result
	output?: string
	// Optional error message when denied
	throw?: string
	// Optional error code (default FORBIDDEN)
	code?: string
	// Optional HTTP status expression (default http.StatusForbidden)
	status?: string
}

#PolicyEvaluateStep: {
	// policy.Evaluate - evaluate policy and return decision context without enforcing.
	action: "policy.Evaluate"
	policyKey: string
	subject?: string
	resource?: string
	operation?: string
	tenant?: string
	attrs?: string
	context?: string
	decision?: string
	reason?: string
	effects?: string
	output?: string
}

#PolicyRequireStep: {
	// policy.Require - evaluate policy and fail request when decision is deny.
	action: "policy.Require"
	policyKey: string
	subject?: string
	resource?: string
	operation?: string
	tenant?: string
	attrs?: string
	context?: string
	throw?: string
	code?: string
	status?: string
	decision?: string
	reason?: string
	effects?: string
	output?: string
}

#PolicyDecideStep: {
	// policy.Decide - evaluate and return full decision object for orchestration/UI.
	action: "policy.Decide"
	policyKey: string
	subject?: string
	resource?: string
	operation?: string
	tenant?: string
	attrs?: string
	context?: string
	decision?: string
	reason?: string
	effects?: string
	output: string
}

#JWTSignStep: {
	// jwt.Sign - Sign claims into JWT token (currently HS256)
	action: "jwt.Sign"
	// Claims expression (must evaluate to map[string]any)
	claims: string
	// Optional secret expression. If omitted, runtime uses APP_TOKEN_SECRET or JWT_PRIVATE_KEY.
	secret?: string
	// Optional algorithm expression (default HS256)
	alg?: string
	// Optional token TTL duration expression (e.g. "\"15m\"")
	ttl?: string
	// Output token variable
	output: string
}

#JWTVerifyStep: {
	// jwt.Verify - Verify JWT signature and decode claims
	action: "jwt.Verify"
	// Token expression
	token: string
	// Optional secret expression. If omitted, runtime uses JWT_PRIVATE_KEY/JWT_PUBLIC_KEY env.
	secret?: string
	// Output claims variable (map[string]any)
	output: string
}

#TokenGenerateStep: {
	// token.Generate - Sign a compact app token with subject, purpose, and optional claims
	action: "token.Generate"
	subject: string
	output:  string
	purpose?: string
	claims?:  string
	// Optional secret expression. If omitted, runtime uses APP_TOKEN_SECRET or JWT_PRIVATE_KEY.
	secret?:  string
	ttl?:     string
}

#TokenVerifyStep: {
	// token.Verify - Verify token signature, expiry, and optional purpose
	action: "token.Verify"
	token: string
	output: string
	purpose?: string
	// Optional secret expression. If omitted, runtime uses APP_TOKEN_SECRET or JWT_PRIVATE_KEY.
	secret?:  string
}

#OAuth2TokenStep: {
	// oauth2.Token - Fetch access token from OAuth2 provider
	action: "oauth2.Token"
	// Token endpoint URL
	tokenURL: string
	clientID?: string
	clientSecret?: string
	scope?: string
	audience?: string
	grantType?: string
	username?: string
	password?: string
	code?: string
	redirectURI?: string
	refreshToken?: string
	// Output token response map
	output: string
}

#OAuth2RefreshStep: {
	// oauth2.Refresh - Refresh access token via refresh token
	action: "oauth2.Refresh"
	tokenURL: string
	refreshToken: string
	clientID?: string
	clientSecret?: string
	scope?: string
	audience?: string
	// Output token response map
	output: string
}

#EncryptStep: {
	// crypto.Encrypt - Encrypt plaintext (AES-GCM with derived key)
	action: "crypto.Encrypt"
	// Plaintext expression
	input: string
	// Optional encryption key expression (fallback APP_ENCRYPTION_KEY env)
	key?: string
	// Optional additional authenticated data (AAD)
	aad?: string
	// Output ciphertext (base64 string)
	output: string
}

#DecryptStep: {
	// crypto.Decrypt - Decrypt ciphertext produced by crypto.Encrypt
	action: "crypto.Decrypt"
	// Ciphertext expression (base64 string)
	input: string
	// Optional encryption key expression (fallback APP_ENCRYPTION_KEY env)
	key?: string
	// Optional additional authenticated data (AAD)
	aad?: string
	// Output plaintext string
	output: string
}

#HashStep: {
	// crypto.Hash - hash input (sha256 default, bcrypt when algo == "bcrypt")
	action: "crypto.Hash"
	input: string
	algo?: string
	output: string
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

#CopyNonEmptyStep: {
	// field.CopyNonEmpty - Type-aware copy of non-zero fields from request/model to target
	action: "field.CopyNonEmpty"
	// Source variable (e.g., "req")
	from: string
	// Target variable (e.g., "existing", "item")
	to: string
	// Optional comma-separated field names. If omitted, helper copies all non-zero fields.
	fields?: string
}

#PatchValidatedStep: {
	// entity.PatchValidated - Patch with per-field normalization/validation/uniqueness checks
	action: "entity.PatchValidated"
	// Target variable (e.g., "company")
	target: string
	// Source variable with patch payload (e.g., "req")
	from: string
	// Optional repository entity for uniqueness checks (e.g., "Company")
	source?: string
	// Per-field rules
	fields: [string]: {
		// Optional normalization mode
		normalize?: "trim" | "lower" | "upper"
		// Optional format validation
		format?: "email" | "phone"
		// Optional repository method name for uniqueness check (e.g., "FindByTaxID")
		unique?: string
	}
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
	// repo.Query  - Query entities via explicit repository method (recommended for filtered lists)
	action: "repo.Save" | "repo.Find" | "repo.Delete" | "repo.List" | "repo.Query" | "repo.Get" | "repo.GetForUpdate"

	// Entity name from domain (e.g., "Tender", "User", "Company")
	// RECOMMENDED: PascalCase (e.g., "User", "APIKey", "TenderInvite")
	source: string

	// Expression for input (ID for Find/Get/Delete, entity variable for Save)
	// SAFE expression subset in strict validator (no function calls)
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

	// required: true → auto nil-check after Query/Find; returns 404 if result is nil
	required?: bool
}

#RepoExistsStep: {
	action: "repo.Exists"
	source: string
	input:  string
	output: string
	method?: string
}

#RepoCountStep: {
	action: "repo.Count"
	source: string
	output: string
	input?: string
	method: string
}

#DbStep: {
	// db.* - Explicit Database Primitives
	action: "db.Get" | "db.List" | "db.Query" | "db.Insert" | "db.Update" | "db.Upsert" | "db.Delete" | "db.Lock" | "db.SelectForUpdate"

	// Entity name from domain (e.g., "Tender", "User", "Company")
	source: string

	// Expression for input (ID for Get/Lock/Delete, entity variable for Insert/Update/Upsert)
	input: string

	// Variable name to store result (required for Get/List/Lock/SelectForUpdate)
	output?: string

	// Method name for db.Query (e.g., "FindByStatus")
	method?: string
}

#UpsertStep: {
	action: "repo.Upsert"
	source: string
	find:   string
	input:  string
	output: string
	ifNew?: [...#FlowStep]
	ifExists?: [...#FlowStep]
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

#SwitchStep: {
	action: "flow.Switch"
	// Go expression yielding branch selector value (e.g., "req.Role", "tender.Status")
	value: string
	// Matching mode for branch keys.
	match?: "exact" | "prefix" | "suffix" | "contains" | "glob" | *"exact"
	// Branches keyed by selector literal values (e.g., owner, admin, draft)
	cases: [string]: [...#FlowStep]
	// Optional fallback branch
	default?: [...#FlowStep]
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
	// SAFE subset only: refs/literals with boolean comparisons (no function calls)
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
	// SAFE value expression only (no function calls/operators)
	// Examples:
	//   "req.Title"                                     - Copy from request
	//   "item.Owner.ID"                                 - Dot-path
	//   "\"draft\""                                     - String literal (escape quotes!)
	//   "true"                                          - Boolean literal
	// For generated values use typed actions:
	//   {action: "uuid.New", output: "..."}
	//   {action: "time.Now", output: "..."}
	//   {action: "rand.Token", output: "..."}
	value: string
	// Set to "true" to declare new variable (var x = value)
	// Use when creating new local variables, not when assigning to fields
	declare?: string | bool
	// Optional type hint for complex declarations
	type?: string
	// Internal: marks step as auto-generated by compiler
	generated?: string | bool
}

#ValueCoalesceStep: {
	action: "value.Coalesce"
	values: [...string]
	output: string
	mode?: "non_empty" | "non_nil" | "non_zero" | *"non_empty"
	into?: string
}

#ErrorNewStep: {
	action: "errors.New"
	message: string
	status?: int | string
	code?:   string
	output?: string
	throw?:  bool | *false
}

#ErrorThrowIfStep: {
	action: "errors.ThrowIf"
	condition: string
	throw:     string
	status?:   int | string
	code?:     string
}

#ErrorWrapStep: {
	action: "errors.Wrap"
	err:     string
	message: string
	output?: string
}

#ErrorMapStep: {
	action: "errors.Map"
	input:  string
	cases:  [string]: [string]: string
	mode?:  "contains" | "equals" | *"contains"
	output?: string
	defaultMessage?: string
	defaultCode?:    string
	defaultStatus?:  string
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
	// LEGACY raw payload expression (discouraged; validated as unsafe in strict flow checks)
	payload?: string
	// Typed payload mapping (preferred):
	// ANG generates domain.<Event>{Field: Expr, ...}
	// Values must be safe refs/literals (no function calls).
	payloadMap?: [string]: string
	// For FileUploaded events specifically:
	fileID?: string
	url?: string
	kind?: string
}

#EventEmitIfStep: {
	action: "event.EmitIf"
	condition: string
	name:      string
	payload?:  string
	payloadMap?: [string]: string
}

#EventOutboxStep: {
	// event.Outbox - Persist event to outbox table in current tx for reliable delivery
	action: "event.Outbox"
	// Event topic/name (usually domain event name)
	name: string
	// Payload expression to serialize
	payload: string
	// Optional explicit event id (default generated in runtime)
	id?: string
}

#EventWaitStep: {
	// event.Wait - Block execution until a specific event arrives
	action: "event.Wait"
	// Event name to wait for
	name: string
	// Optional timeout duration (e.g. "5*time.Minute")
	timeout?: string
	// Optional correlation mapping (e.g. "OrderID=req.ID")
	match?: string
	// Variable name to store received event payload
	output?: string
	// Optional Go type for received payload (e.g. "map[string]any", "domain.OrderCreated")
	into?: string
}

#EventSubscribeStep: {
	// event.Subscribe - Define an async handler for events within a flow
	action: "event.Subscribe"
	// Event name to subscribe to
	name: string
	// Correlation ID mapping
	match: string
	// Steps to execute when event arrives
	do: [...#FlowStep]
}

#EventMatchStep: {
	// event.Match - Validate correlation criteria for a received event
	action: "event.Match"
	// Variable holding received event
	event: string
	// Correlation criteria (e.g. "ID=req.OrderID")
	match: string
	// Error to throw if not matched (default: 400 Bad Request)
	throw?: string
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
	// SAFE expression subset in strict validator (no function calls)
	// Example: "ctx, req" or ["ctx", "req", "options"]
	args?: string | [...string]
	// Variable to store result (if function returns value)
	output?: string
	// Ignore returned error explicitly. Use only for deliberate fire-and-forget behavior.
	ignoreErr?: bool
	// Optional reason documenting why ignoring the error is acceptable here.
	ignoreErrReason?: string
}

#ServiceCallStep: {
	action: "service.Call"
	// Target service dependency (must be declared in operation/service uses)
	service: string
	// Target service method (operation name)
	method: string
	// Arguments as single expression or array of expressions.
	// SAFE expression subset in strict validator (no function calls).
	// If ctx is omitted, emitter prepends ctx automatically.
	args?: string | [...string]
	// Variable to store result
	output?: string
	// Ignore returned error explicitly. Use only for deliberate fire-and-forget behavior.
	ignoreErr?: bool
	// Optional reason documenting why ignoring the error is acceptable here.
	ignoreErrReason?: string
}

#FlowCallStep: {
	action: "flow.Call"
	// Operation reference:
	//   "MethodName" for current service
	//   "ServiceName.MethodName" for declared dependency service
	op: string
	// Request field mapping for generated port.<Method>Request{...}
	args?: [string]: string
	// Variable to store port.<Method>Response result
	output?: string
	// Ignore returned error explicitly. Use only for deliberate fire-and-forget behavior.
	ignoreErr?: bool
	// Optional reason documenting why ignoring the error is acceptable here.
	ignoreErrReason?: string
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
// LIST MAP / REDUCE / GROUP / DISTINCT / CHUNK
// ----------------------------------------------------------------------------

#ListMapStep: {
	// list.Map - Transform list items into a new list
	action: "list.Map"
	// Source slice expression (e.g., "items")
	from: string
	// Loop variable name (default "item")
	as?: string
	// Go expression to append into output (e.g., "item.ID")
	expr: string
	// Variable name for transformed output
	output: string
}

#ListReduceStep: {
	// list.Reduce - Fold a list into a single accumulator value
	action: "list.Reduce"
	// Source slice expression (e.g., "items")
	from: string
	// Loop variable name (default "item")
	as?: string
	// Accumulator expression (e.g., "total + item.Price")
	expr: string
	// Optional initial accumulator expression
	initial?: string
	// Variable name for accumulator result
	output: string
}

#ListGroupByStep: {
	// list.GroupBy - Group list items by key into map[string][]any
	action: "list.GroupBy"
	// Source slice expression
	from: string
	// Loop variable name (default "item")
	as?: string
	// Key expression (e.g., "item.Status")
	key: string
	// Variable name for grouped result map
	output: string
}

#ListDistinctStep: {
	// list.Distinct - Deduplicate list items by key (or full item)
	action: "list.Distinct"
	// Source slice expression
	from: string
	// Loop variable name (default "item")
	as?: string
	// Optional key expression (e.g., "item.ID")
	key?: string
	// Variable name for distinct result list
	output: string
}

#ListChunkStep: {
	// list.Chunk - Split list into chunks of fixed size
	action: "list.Chunk"
	// Source slice expression
	from: string
	// Chunk size (int literal or expression string)
	size: int | string
	// Variable name for chunked output ([][]any)
	output: string
}

#BatchRunStep: {
	// batch.Run - Execute nested flow for each batch chunk
	action: "batch.Run"
	// Source slice expression
	from: string
	// Optional chunk size (int literal or expression string)
	size?: int | string
	// Batch variable name inside do (default "batch")
	as?: string
	// Nested steps executed for each batch
	do: [...#FlowStep]
}

// ----------------------------------------------------------------------------
// LIST ENRICH
// ----------------------------------------------------------------------------

#EnrichStep: {
	// list.Enrich - Enrich each list item using related entity lookup
	action: "list.Enrich"
	// Source slice expression (e.g., "reviews", "items")
	items: string
	// Loop variable name (default "item")
	as?: string
	// Related entity for repository lookup (e.g., "Company")
	lookupSource: string
	// Go expression for lookup ID (e.g., "item.AuthorCompanyID")
	lookupInput: string
	// Mapping pairs "TargetField=LookupField" separated by commas
	// Example: "AuthorName=Name,AuthorLogo=Logo"
	set: string
}

// ----------------------------------------------------------------------------
// TIME PARSING
// ----------------------------------------------------------------------------

#TimeNowStep: {
	// time.Now - assign current UTC time into output target
	action: "time.Now"
	// Target field or variable
	output: string
	// Optional string format (for string targets), e.g. "time.RFC3339"
	format?: string
}

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

#TimeFormatStep: {
	// time.Format - Format time.Time into string
	action: "time.Format"
	// Source time expression (e.g., "bid.CreatedAt")
	input: string
	// Target variable/field for formatted string
	output: string
	// Go time format constant (default "time.RFC3339")
	format?: string
	// IANA timezone to convert to before formatting (e.g. "user.Timezone" or "\"Europe/Moscow\"")
	timezone?: string
}

#TimeInZoneStep: {
	// time.InZone - Convert time.Time to a specific IANA timezone
	action:   "time.InZone"
	input:    string
	// IANA timezone expression (e.g. "user.Timezone" or "\"Europe/Moscow\"")
	timezone: string
	output:   string
}

#TimeAddStep: {
	action: "time.Add"
	input:  string
	duration: string
	output: string
}

#TimeSubStep: {
	action: "time.Sub"
	a:      string
	b:      string
	output: string
}

#TimeDiffStep: {
	action: "time.Diff"
	from:   string
	to:     string
	output: string
	unit?:  "duration" | "seconds" | "minutes" | "hours" | "days" | "milliseconds" | *"duration"
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

#MapNewStep: {
	// map.New - Initialize empty map with explicit type
	action: "map.New"
	output: string
	type:   string
}

#MapGetStep: {
	action: "map.Get"
	input:  string
	key:    string
	output: string
	into?:  string
	default?: string
	found?: string
}

#MapHasStep: {
	action: "map.Has"
	input:  string
	key:    string
	output: string
}

#MapSetStep: {
	action: "map.Set"
	input:  string
	key:    string
	value:  string
	output?: string
}

#MapMergeStep: {
	action: "map.Merge"
	left:   string
	right:  string
	output: string
}

// --- HTTP & INFRA ---

#RateLimitDef: {
	rps?:    int
	burst?:  int
	window?: string // duration, e.g. "1h", "30m" — enables fixed-window quota
	limit?:  int    // max requests per window (requires window)
}

#CircuitBreaker: {
	threshold:     int | *5      // сбоев до открытия
	timeout:       string | *"30s" // время в состоянии Open
	half_open_max: int | *3      // запросов в Half-Open
}

#RetryPolicy: {
	enabled?:              bool | *true
	max_attempts?:         int | *3
	base_delay_ms?:        int | *200
	retry_on_statuses?:    [...int]
	retry_network_errors?: bool | *true
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
		retry?:           #RetryPolicy
		timeout?:         string // Request timeout (e.g. "5s", "30s", "1m")
		max_body_size?:   string // Max request body size (e.g. "10mb", "100kb")
		// Omit = JSON body. "multipart_form" = multipart/form-data with field "file" (+ optional ownerType, ownerId, folderId, name, mime) merged into the same DTO as fileBase64.
		request_body?: string
		idempotency?: bool
                auth?: {
                        type:    "jwt"
                        action?: string
                        check?:  string
                        roles?: [...string]
                        inject?: [...string] | string
                        scope?:  string | [...string]
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

// ============================================================================
// STAGE 2: INFRASTRUCTURE ACTIONS (cache.*, mail.*, storage.*)
// ============================================================================

#CacheGetStep: {
	action: "cache.Get"
	// Redis key expression (e.g., "\"session:\" + req.Token")
	key: string
	// Variable name to receive cached value as string
	output: string
	// If true, missing key is not an error (default true)
	optional?: bool | *true
}

#CacheSetStep: {
	action: "cache.Set"
	// Redis key expression
	key: string
	// Value expression to cache (will be stored as string)
	value: string
	// TTL expression (e.g., "24*time.Hour", "time.Hour")
	ttl?: string
}

#CacheDelStep: {
	action: "cache.Del"
	// Redis key expression
	key: string
}

#MailSendStep: {
	action: "mail.Send"
	// Recipient email expression (e.g., "user.Email", "req.Email")
	to: string
	// Subject expression (e.g., "\"Welcome to Sendbox\"")
	subject: string
	// Body text or template expression
	body: string
	// Optional HTML body
	html?: string
}

#NotifySendStep: {
	// notify.Send - Multi-channel notification dispatch through runtime dispatcher
	action: "notify.Send"
	// Channel expression (e.g. "\"email\"", "\"slack\"")
	channel: string
	// Recipient expression (email/user/slack ID)
	to: string
	// Template ID expression (optional, but either template or text is required)
	template?: string
	// Plain text message expression (optional, but either template or text is required)
	text?: string
	// Optional subject expression (used by email channel)
	subject?: string
	// Optional HTML expression (used by email channel)
	html?: string
	// Optional payload/data expression for templating
	data?: string
	// Optional output variable (message id marker)
	output?: string
}

#NotifyEmailStep: {
	// notify.Email - First-class email notification dispatch through runtime dispatcher
	action: "notify.Email"
	to: string
	template?: string
	text?: string
	subject?: string
	html?: string
	data?: string
	locale?: string
	output?: string
}

#NotifyDispatchStep: {
	// notification.Dispatch / notify.Dispatch - dispatch domain notification event
	action: "notification.Dispatch" | "notify.Dispatch"
	event?: string
	message?: string
	type?: string
	userID?: string
	entityID?: string
	payload?: string
	template?: string
}

#ApprovalRequestStep: {
	// approval.Request - Create (idempotent) approval task
	action: "approval.Request"
	approvalKey: string
	title: string
	description?: string
	requestedBy: string
	approvers: string | [...string]
	policy: string
	payload: string
	deadline?: string
	ttl?:      string
	approvalId?: string
	status?:     string
}

#ApprovalWaitStep: {
	// approval.Wait - Durable wait for manual decision
	action: "approval.Wait"
	approvalId: string
	timeout?: string
	// Timeout mode ("reject" | "auto-approve" | "fallback") or fallback steps
	onTimeout?: string | [...#FlowStep]
	decision?:  string
	status?:    string
	decidedBy?: string
	decidedAt?: string
	reason?:    string
}

#ApprovalDecideStep: {
	// approval.Decide - Apply decision for existing approval task
	action: "approval.Decide"
	approvalId: string
	decision:   string
	actor:      string
	reason?: string
	status?: string
}

#StorageUploadStep: {
	action: "storage.Upload"
	// Object key/path (e.g., "\"avatars/\" + user.ID + \".jpg\"")
	key: string
	// Data expression (bytes or string)
	data: string
	// Content type (e.g., "\"image/jpeg\"")
	contentType?: string
	// Variable name to receive public URL
	output?: string
}

#StorageDownloadStep: {
	action: "storage.Download"
	// Object key expression to download
	key: string
	// Variable name for downloaded []byte payload
	output: string
}

#StorageDeleteStep: {
	action: "storage.Delete"
	// Object key expression to delete
	key: string
}

#StorageListStep: {
	action: "storage.List"
	// Prefix expression to list by
	prefix: string
	// Variable name for []string keys
	output: string
}

#StorageGetURLStep: {
	action: "storage.GetURL"
	// Object key expression
	key: string
	// Variable name for URL string
	output: string
}

#WebhookSendStep: {
	// webhook.Send - Send JSON payload to external webhook URL
	action: "webhook.Send"
	url: string
	payload: string
	event?: string
	retries?: int | *3
}

#WebhookVerifySignatureStep: {
	// webhook.VerifySignature - Verify incoming webhook signature (HMAC SHA-256)
	action: "webhook.VerifySignature"
	payload: string
	signature: string
	secret?: string
	algorithm?: string | *"sha256"
	// strict=true => mismatch returns error, strict=false => only fills output when provided
	strict?: bool | *true
	output?: string
	throw?:  string
}

#WebhookAckStep: {
	// webhook.Ack - Marker step for transport-level webhook acknowledgement
	action: "webhook.Ack"
	status?: int | *200
	body?:   string | *"\"ok\""
}

#QueueEnqueueStep: {
	// queue.Enqueue - Publish payload to queue/topic
	action: "queue.Enqueue"
	subject: string
	payload: string
	timeout?: string
	timeoutMs?: int
}

#QueueDequeueStep: {
	// queue.Dequeue - Receive payload from queue/topic
	action: "queue.Dequeue"
	subject: string
	output: string
	ackToken?: string
	attempts?: int
	retries?:  int
	backoffMs?: int
	jitterMs?: int
	timeout?: string
	timeoutMs?: int
}

#QueueAckStep: {
	// queue.Ack - Acknowledge consumed message
	action: "queue.Ack"
	subject: string
	messageID: string
}

#QueueNackStep: {
	// queue.Nack - Negative acknowledge for failed processing
	action: "queue.Nack"
	subject: string
	messageID: string
	reason?: string
}

#DLQPublishStep: {
	// dlq.Publish - Send payload to dead-letter stream/topic
	action: "dlq.Publish"
	subject: string
	payload: string
	reason?: string
}

// ============================================================================
// STAGE 3: NEW CAPABILITIES
// ============================================================================

#SecretGetStep: {
	action: "secret.Get"
	// Secret key name (e.g. "STRIPE_SECRET_KEY")
	key: string
	// Variable name to store the secret value
	output: string
	// Optional default value if secret is not set (not recommended for actual secrets)
	default?: string
}

#ConfigGetStep: {
	action: "config.Get"
	// Config key name (e.g. "MAX_ITEMS")
	key: string
	// Variable name to store the config value
	output: string
	// Optional default value if config is not set
	default?: string
}

#ModelResolveStep: {
	action: "model.Resolve"
	// Alias from infra #Models registry (e.g. "Cheap", "Fast", "Smart")
	name: string
	// Variable name to store the resolved model value
	output: string
	// Optional fallback when alias is missing from the registry
	default?: string
}

#StreamEmitStep: {
	action: "stream.Emit"
	// JSON/text payload to push into SSE chunks.
	data: string
}

#HTTPCallStep: {
	action: "http.Call"
	method: "GET" | "POST" | "PUT" | "DELETE" | "PATCH"
	url: string
	body?: string
	headers?: [string]: string
	output?: string
	statusVar?: string
	failOnError?: bool | *true
}

#HTTPRequestStep: {
	// http.Request - Enriched HTTP call with auth, query params, and typed JSON decode.
	action: "http.Request"
	method: "GET" | "POST" | "PUT" | "DELETE" | "PATCH"
	// URL expression (Go expr, e.g. "\"https://api.example.com/items\"" or req.URL)
	url: string
	// Request body expression (passed to strings.NewReader)
	body?: string
	// Static/dynamic request headers (value is a Go expression)
	headers?: [string]: string
	// URL query parameters (value is a Go expression)
	query?: [string]: string
	// Auth: "bearer:TOKEN_EXPR" or "basic:USER_EXPR:PASS_EXPR"
	auth?: string
	// Per-request timeout duration expression (default "10*time.Second")
	timeout?: string
	// Go type name for JSON unmarshal of response body (e.g. "domain.Item")
	into?: string
	// Variable name for result (string body when no 'into', typed struct when 'into' set)
	output?: string
	// Variable name to store HTTP status code
	statusVar?: string
	// Return error on 4xx/5xx responses (default true)
	failOnError?: bool | *true
}

#OAuthGoogleGetURLStep: {
	// oauth.Google.GetURL — builds Google OAuth2 authorization URL
	action:      "oauth.Google.GetURL"
	clientID:    string
	redirectURL: string
	output:      string
	state?:      string
	scopes?:     string
}

#OAuthGoogleExchangeStep: {
	// oauth.Google.Exchange — exchanges authorization code for *oauth2.Token
	action:       "oauth.Google.Exchange"
	clientID:     string
	clientSecret: string
	redirectURL:  string
	code:         string
	output:       string
	scopes?:      string
}

#OAuthGoogleUserInfoStep: {
	// oauth.Google.UserInfo — fetches Google profile using an *oauth2.Token
	action: "oauth.Google.UserInfo"
	token:  string
	output: string
}

#LocaleResolveStep: {
	// locale.Resolve — resolves user locale via fallback chain: sources → reqctx → default
	action:   "locale.Resolve"
	output?:  string
	// Comma-separated Go expressions to try in order (e.g. "req.Locale, user.Locale")
	sources?: string
	// Final fallback locale code (default: "en")
	default?: string
}

#HTTPRetryPolicyStep: {
	// http.RetryPolicy - HTTP call with status-code-aware retry logic.
	action: "http.RetryPolicy"
	method: "GET" | "POST" | "PUT" | "DELETE" | "PATCH"
	// URL expression
	url: string
	body?: string
	headers?: [string]: string
	query?: [string]: string
	auth?: string
	// Per-attempt timeout (default "10*time.Second")
	timeout?: string
	// Maximum number of attempts (default 3)
	attempts?: int | *3
	// Backoff between retries in ms (default 500)
	backoffMs?: int | *500
	// HTTP status codes that trigger a retry (default [429, 503])
	retryOn?: [...int] | *[429, 503]
	output?: string
	statusVar?: string
	failOnError?: bool | *true
}

#HTTPPaginateStep: {
	// http.Paginate - Cursor-based pagination loop over an external API.
	// Calls the URL repeatedly, advancing the cursor each time, until cursor is empty.
	action: "http.Paginate"
	// Base URL expression (cursor added as a query param each iteration)
	url: string
	// HTTP method (default "GET")
	method?: string | *"GET"
	// Go type for each page response (e.g. "PagedResponse" or "struct{Items []any; NextCursor string}")
	into: string
	// Variable name for each page in scope (used in cursor_expr and items_expr)
	as: string
	// Go expression to extract next cursor from page var (empty string = stop)
	cursor_expr: string
	// Go expression for items slice to accumulate from each page (e.g. "page.Items")
	items_expr?: string
	// Query parameter name for cursor (default "cursor")
	cursor_param?: string | *"cursor"
	// Variable name for the accumulated items slice
	output?: string
	// Go type for the accumulated output slice (default "[]any")
	output_type?: string | *"[]any"
	// Safety limit on page iterations (default 100)
	max_pages?: int | *100
	headers?: [string]: string
	auth?: string
}

#RandCodeStep: {
	action: "rand.Code"
	// Length of numeric code (default 6)
	length?: int | *6
	// Variable name to receive the code string
	output: string
}

#RandTokenStep: {
	action: "rand.Token"
	// Length in bytes (default 32, produces 64-char hex)
	bytes?: int | *32
	// Variable name to receive hex token
	output: string
}

#StrFormatStep: {
	action: "str.Format"
	// Format string as Go expression (e.g., "\"Hello, %s! Code: %d\"")
	template: string
	// Arguments list as Go expressions
	args: [...string]
	// Variable name for result
	output: string
}

#StrConcatStep: {
	action: "str.Concat"
	// Parts to concatenate (Go expressions)
	parts: string | [...string]
	// Optional separator between parts
	sep?: string | *""
	// Variable name for result
	output: string
}

#StrStripMarkdownStep: {
	action: "str.StripMarkdown"
	// Go expression to strip markdown fences from
	input: string
	// Optional output variable; defaults to input (in-place)
	output?: string
}

// str.ReplaceAll — strings.ReplaceAll(input, old, new); old/new are Go expressions (e.g. "\"\\\\\"" and "\"/\"")
#StrReplaceAllStep: {
	action: "str.ReplaceAll"
	input: string
	old:   string
	new:   string
	output: string
}

// str.TrimSpace — strings.TrimSpace(input)
#StrTrimSpaceStep: {
	action: "str.TrimSpace"
	input:  string
	output: string
}

#CastToStringStep: {
	action: "cast.ToString"
	// Source expression
	input: string
	// Optional printf-style format, e.g. "%d"
	format?: string
	// Variable name for result
	output: string
}

#NumBinaryStep: {
	action: "num.Add" | "num.Sub" | "num.Mul" | "num.Div"
	a: string
	b: string
	output: string
}

#MathExprStep: {
	action: "math.Expr"
	expr: string
	output: string
	declare?: bool | string
}

#MathOpStep: {
	action: "math.Op"
	op: string
	output: string
	a?: string
	b?: string
	value?: string
	min?: string
	max?: string
	precision?: int
}

#RegexMatchStep: {
	action: "regex.Match"
	input:  string
	pattern: string
	output: string
}

#RegexReplaceStep: {
	action: "regex.Replace"
	input:  string
	pattern: string
	repl:   string
	output: string
}

#Base64EncodeStep: {
	action: "base64.Encode"
	input:  string
	output: string
}

#Base64DecodeStep: {
	action: "base64.Decode"
	input:  string
	output: string
}

#URLParseStep: {
	action: "url.Parse"
	input:  string
	output: string
}

#URLBuildStep: {
	action: "url.Build"
	base:   string
	path?:  string
	segments?: [...string]
	query?: [string]: string
	output: string
}

// path.Base — last path segment; normalizes backslashes to '/' first (portable filenames)
#PathBaseStep: {
	action: "path.Base"
	input:  string
	output: string
}

#QueryEncodeStep: {
	action: "query.Encode"
	// Source map/url.Values expression
	input: string
	// Variable name for encoded query string
	output: string
}

#QueryDecodeStep: {
	action: "query.Decode"
	// Raw query string expression
	input: string
	// Variable name for decoded url.Values
	output: string
}

#HashSumStep: {
	action: "hash.Sum"
	input:  string
	algorithm?: string
	algo?: string
	output: string
}

#HashHMACStep: {
	action: "hash.HMAC"
	input:  string
	key:    string
	algorithm?: string
	algo?: string
	output: string
}

#UUIDNewStep: {
	action: "uuid.New"
	// Variable or field path to receive generated UUID string
	output: string
}

#ULIDNewStep: {
	action: "ulid.New"
	// Variable or field path to receive generated ULID string
	output: string
}

#JSONParseStep: {
	action: "json.Parse"
	// JSON string expression to parse
	input: string
	// Type to unmarshal into as Go type expression (e.g., "map[string]interface{}")
	into: string
	// Variable name for result
	output: string
}

#JSONStringifyStep: {
	action: "json.Stringify"
	input:  string
	output: string
}

#JSONMarshalStep: {
	action: "json.Marshal"
	// Value expression to marshal
	input: string
	// Variable name for result JSON string
	output: string
}

#TemplateRenderStep: {
	action: "template.Render"
	template: string
	data:     string
	output:   string
}

#JSONPathGetStep: {
	action: "jsonpath.Get"
	input: string
	path: string
	output: string
}

#JSONPathSetStep: {
	action: "jsonpath.Set"
	input: string
	path: string
	value: string
	output: string
}

#ParallelStep: {
	action: "parallel.Run"
	// Named parallel branches, each is a list of flow steps
	branches: [string]: [...#FlowStep]
}

#PDFRenderStep: {
	action: "pdf.Render"
	template: string
	data: string
	output: string
}

// flow.Parallel - concurrent fan-out, ALL branches must succeed (fail-fast)
// First error cancels all remaining branches and is returned immediately.
#FlowParallelStep: {
	action: "flow.Parallel"
	branches: [string]: [...#FlowStep]
}

// flow.Join - concurrent fan-out, wait ALL branches (lenient, no fail-fast)
// All branches run to completion; if any fail, the first error is returned at the end.
#FlowJoinStep: {
	action: "flow.Join"
	branches: [string]: [...#FlowStep]
}

// flow.Race - concurrent race, FIRST branch to succeed wins, others cancelled
// If all branches fail, an error is returned.
#FlowRaceStep: {
	action: "flow.Race"
	branches: [string]: [...#FlowStep]
}

// flow.Delay - context-aware pause (interrupted by ctx cancel)
#FlowDelayStep: {
	action: "flow.Delay"
	// Go duration expression, e.g. "2*time.Second", "time.Minute"
	duration: string
}

// flow.Schedule - block until absolute time.Time arrives (context-aware)
#FlowScheduleStep: {
	action: "flow.Schedule"
	// Go expression evaluating to time.Time, e.g. "auction.StartsAt", "req.ScheduledAt"
	at: string
}

// flow.Cron - time-window gate: only proceed if current time matches window
// window formats: "Mon-Fri 09:00-17:00" | "Mon-Fri" | "09:00-17:00" | "Mon,Wed,Fri 09:00-17:00"
// Weekday names: Mon Tue Wed Thu Fri Sat Sun. End hour is exclusive.
#FlowCronStep: {
	action:   "flow.Cron"
	window:   string
	timezone?: string // IANA tz name, default "UTC". E.g. "Europe/Moscow", "America/New_York"
	onMismatch?: [...#FlowStep]
}

// flow.Saga - Define a saga block that tracks compensations for completed steps.
// If any step within 'do' fails, the compensation chain is executed in reverse order.
#FlowSagaStep: {
	action: "flow.Saga"
	do: [...#FlowStep]
}

// flow.Compensate - Register a compensating action for the preceding step.
// This action is only executed if a subsequent step in the same saga fails.
#FlowCompensateStep: {
	action: "flow.Compensate"
	do: [...#FlowStep]
}

// flow.Rollback - Manually trigger a rollback of the current saga or transaction.
#FlowRollbackStep: {
	action: "flow.Rollback"
	// Optional error to return
	error?: string
}

// flow.Defer - Register cleanup/finalization steps that always run on function exit.
// Similar to Go defer; use for temp files, locks, and best-effort cleanup.
#FlowDeferStep: {
	action: "flow.Defer"
	do: [...#FlowStep]
}

// ============================================================================
// SYSTEM / OS OPERATIONS (exec.*, fs.*)
// ============================================================================

#ExecRunStep: {
	// exec.Run - Execute external command and capture combined stdout+stderr.
	// Generated Go: _execCmd := exec.CommandContext(ctx, cmd, args...); out, err := cmd.CombinedOutput()
	action: "exec.Run"
	// Command to execute as a Go expression (e.g. "os.Getenv(\"ANG_BIN\")")
	cmd: string
	// Arguments list. Each element is a Go expression.
	// Use escaped quotes for string literals: "\"build\"", "\"--flag\"".
	// Use bare identifiers for variables: "workDir", "req.ID".
	args?: [...string]
	// Optional: pipe this expression as stdin (e.g. "req.CueContent")
	stdin?: string
	// Optional command timeout (default: "120 * time.Second")
	timeout?: string
	// Optional timeout in milliseconds (used when timeout is not set)
	timeoutMs?: int
	// Variable name to receive combined output as string (e.g. "buildOutput")
	output?: string
	// Variable name to receive exit code as int (e.g. "exitCode")
	exitCodeVar?: string
	// If false, non-zero exit code is captured instead of throwing (default: true)
	failOnError?: bool | *true
	// Error message thrown on non-zero exit code; only used when failOnError: true
	throw?: string
}

#ExecStreamStep: {
	// exec.Stream - Execute external command and stream output line-by-line while running.
	// Generated Go:
	//   - context.WithTimeout
	//   - io.Pipe + Scanner over merged stdout/stderr
	//   - line logs via slog.Info("exec.stream", ...)
	//   - optional aggregated output string
	action: "exec.Stream"
	cmd: string
	args?: [...string]
	stdin?: string
	timeout?: string
	timeoutMs?: int
	output?: string
	exitCodeVar?: string
	failOnError?: bool | *true
	throw?: string
}

#FSTempDirStep: {
	// fs.TempDir - Create a temporary directory.
	// Generated Go: dir, err := os.MkdirTemp("", pattern)
	action: "fs.TempDir"
	// Variable name to receive the temp dir path (e.g. "workDir")
	output: string
	// Optional glob pattern (e.g. "\"sendbox-*\""). Defaults to "\"ang-tmp-*\""
	pattern?: string
}

#FSWriteFileStep: {
	// fs.WriteFile - Write string content to a file, creating parent dirs.
	// Generated Go: os.MkdirAll(dir); os.WriteFile(path, []byte(data), 0644)
	action: "fs.WriteFile"
	// File path expression (e.g. "filepath.Join(workDir, \"cue\", \"main.cue\")")
	path: string
	// Content expression (e.g. "req.CueContent")
	data: string
}

#FSReadFileStep: {
	// fs.ReadFile - Read file contents into a string variable.
	// Generated Go: b, err := os.ReadFile(path); output = string(b)
	action: "fs.ReadFile"
	// File path expression (e.g. "filepath.Join(workDir, \"api\", \"openapi.yaml\")")
	path: string
	// Variable name to receive file contents as string
	output: string
	// If true, skip error when file does not exist (output = "")
	optional?: bool
}

#FSRemoveStep: {
	// fs.Remove - Remove a file or directory (recursive).
	// Generated Go: os.RemoveAll(path)
	action: "fs.Remove"
	// Path expression to remove
	path: string
}

#ArchiveZipDirStep: {
	// archive.ZipDir - Zip local directory into []byte payload
	action: "archive.ZipDir"
	path: string
	output: string
}

#SessionGetStep: {
	// session.Get - Read session id from request context
	action: "session.Get"
	output: string
}

// ============================================================================
// RELIABILITY PRIMITIVES (idem.*, dedupe.*, ratelimit.*, concurrency.*, circuit.*, bulkhead.*)
// ============================================================================

#IdemDeriveKeyStep: {
	// idem.DeriveKey - Compute a stable idempotency key from a set of expressions.
	// Generated Go: sha256(fmt.Sprintf("%v|%v|...", expr1, expr2, ...))[:16]
	action: "idem.DeriveKey"
	// Go expressions to hash together (e.g. ["req.UserID", "req.OrderID"])
	from: [...string]
	// Variable name to store the derived key (e.g. "idemKey")
	output: string
	// Optional string prefix for the key (default: "idem:")
	prefix?: string
}

#IdemCheckStep: {
	// idem.Check - Return a cached response if the idempotency key was already processed.
	// If key is found in stateStore, unmarshals stored bytes into resp and returns early.
	action: "idem.Check"
	// Key expression (e.g. "idemKey")
	key: string
}

#IdemSaveResultStep: {
	// idem.SaveResult - Persist the current response under the idempotency key.
	action: "idem.SaveResult"
	// Key expression (e.g. "idemKey")
	key: string
	// TTL as a Go duration expression (default: "24 * time.Hour")
	ttl?: string
}

#DedupeOnceStep: {
	// dedupe.Once - Execute child steps only once per key+TTL window.
	// If key exists in stateStore, the block is skipped entirely.
	action: "dedupe.Once"
	// Key expression (e.g. "\"notify:\"+req.UserID")
	key: string
	// TTL as a Go duration expression (default: "24 * time.Hour")
	ttl?: string
	// Steps to execute on first call only
	do: [...#FlowStep]
}

#RateLimitCheckStep: {
	// ratelimit.Check - Per-key per-second token counter. Returns 429 when limit exceeded.
	action: "ratelimit.Check"
	// Key expression used for bucketing (e.g. "req.UserID", "\"global\"")
	key: string
	// Requests per second allowed (e.g. 10)
	rps: int
	// Optional error message (default: "rate limit exceeded")
	throw?: string
}

#ConcurrencyLimitStep: {
	// concurrency.Limit - State-store semaphore. Returns 503 when pool is full.
	// Automatically releases slot on function return via defer.
	action: "concurrency.Limit"
	// Key expression to namespace the semaphore (e.g. "\"process\"", "req.TenantID")
	key: string
	// Maximum concurrent executions
	max: int
	// Optional error message (default: "concurrency limit exceeded")
	throw?: string
}

#CircuitCheckStep: {
	// circuit.Check - Returns 503 if the named circuit is open.
	// Pair with circuit.RecordSuccess / circuit.RecordFailure around the external call.
	action: "circuit.Check"
	// Circuit name string literal (e.g. "\"payment-svc\"")
	name: string
	// Optional error message (default: "circuit breaker open: <name>")
	throw?: string
}

#CircuitRecordSuccessStep: {
	// circuit.RecordSuccess - Reset failure counter and close the circuit.
	action: "circuit.RecordSuccess"
	// Circuit name string literal (e.g. "\"payment-svc\"")
	name: string
}

#CircuitRecordFailureStep: {
	// circuit.RecordFailure - Increment failure counter; open circuit at threshold.
	action: "circuit.RecordFailure"
	// Circuit name string literal (e.g. "\"payment-svc\"")
	name: string
	// Number of failures before the circuit opens (default: 5)
	threshold?: int
	// Duration the circuit stays open (Go duration expr, default: "60 * time.Second")
	openTTL?: string
}

#BulkheadAcquireStep: {
	// bulkhead.Acquire - Named resource pool. Returns 503 when pool is full.
	// Automatically releases slot on function return via defer.
	action: "bulkhead.Acquire"
	// Pool name string literal (e.g. "\"db-pool\"", "\"payment-api\"")
	name: string
	// Maximum pool size
	max: int
	// Optional error message (default: "bulkhead full: <name>")
	throw?: string
}

#IdempotencyDeriveKeyStep: {
	// idempotency.DeriveKey - Alias for idem.DeriveKey with explicit naming.
	action: "idempotency.DeriveKey"
	from: [...string]
	output: string
	prefix?: string
}

#IdempotencyCheckStep: {
	// idempotency.Check - Alias for idem.Check.
	action: "idempotency.Check"
	key: string
}

#IdempotencySaveResultStep: {
	// idempotency.SaveResult - Alias for idem.SaveResult.
	action: "idempotency.SaveResult"
	key: string
	ttl?: string
}

#RateLimitLimitStep: {
	// ratelimit.Limit - Alias for ratelimit.Check.
	action: "ratelimit.Limit"
	key: string
	rps: int
	throw?: string
}

#QuotaCheckStep: {
	// quota.Check - Per-key fixed-window quota guard (hour/day/month).
	action: "quota.Check"
	key: string
	limit: int
	window: "hour" | "day" | "month" | string
	throw?: string
}

#BudgetCheckStep: {
	// budget.Check - Ensure cumulative token budget for key is below limit.
	action: "budget.Check"
	key: string
	limit: int
	throw?: string
}

#BudgetConsumeStep: {
	// budget.Consume - Increase cumulative token budget for key.
	action: "budget.Consume"
	key: string
	tokens: string
	ttl?: string
}

#ContextTrimStep: {
	// context.Trim - Trim large text context to keep prompts bounded.
	action: "context.Trim"
	input: string
	output: string
	max_bytes?: int
	strategy?: "lines" | "chars" | "sentences" | string
}

#ProfileRequireStep: {
	// profile.Require - Enforce minimum profile tier (free < ops < enterprise).
	action: "profile.Require"
	key: string
	tier: "free" | "ops" | "enterprise" | string
	throw?: string
}

#ConcurrencyRunStep: {
	// concurrency.Run - Acquire concurrency slot and run child steps.
	action: "concurrency.Run"
	key: string
	max: int
	throw?: string
	do: [...#FlowStep]
}

#MutexWithStep: {
	// mutex.With - Acquire a process-local mutex by key and run child steps.
	action: "mutex.With"
	key: string
	throw?: string
	wait?: string
	poll?: string
	do: [...#FlowStep]
}

#CircuitBreakerStep: {
	// circuit.Breaker - Open-check gate plus success/failure recording around child steps.
	action: "circuit.Breaker"
	name: string
	threshold?: int
	openTTL?: string
	throw?: string
	do: [...#FlowStep]
}

#BulkheadRunStep: {
	// bulkhead.Run - Acquire bulkhead slot and run child steps.
	action: "bulkhead.Run"
	name: string
	max: int
	throw?: string
	do: [...#FlowStep]
}

#LogEmitStep: {
	// log.Emit - Structured log entry from flow.
	action: "log.Emit"
	message: string
	level?: "debug" | "info" | "warn" | "error" | string
	fields?: [string]: string
}

#MetricEmitStep: {
	// metric.Emit - Structured metric event from flow.
	// Current runtime emits this as structured log for portability.
	action: "metric.Emit"
	name: string
	kind?: "counter" | "gauge" | "histogram" | string
	value?: string
	labels?: [string]: string
}

#TraceSpanStep: {
	// trace.Span - Start trace span around child steps.
	action: "trace.Span"
	name: string
	attrs?: [string]: string
	do: [...#FlowStep]
}

#SLOBudgetStep: {
	// slo.Budget - Apply deadline budget for child steps and warn on overrun.
	action: "slo.Budget"
	name?: string
	duration: string
	do: [...#FlowStep]
}

#ListSumStep: {
	action: "list.Sum"
	input:  string
	output: string
	field?: string  // optional: sum a specific struct field
}

#ListAvgStep: {
	action: "list.Avg"
	input:  string
	output: string
	field?: string  // optional: average a specific struct field
}

#FlowReturnStep: {
	action: "flow.Return"
	set?:   string  // optional: field to set before returning
	value?: string  // value to assign to set field
}

#ConvertToFloatStep: {
	action: "convert.ToFloat"
	input:  string
	output: string
}

#ConvertToIntStep: {
	action: "convert.ToInt"
	input:  string
	output: string
}

#ClaudeChatStep: {
	// claude.Chat - Call Claude API and get a text reply.
	// Uses ANTHROPIC_API_KEY env var.
	action: "claude.Chat"
	// Current user message Go expression (e.g. "req.Message")
	user_message: string
	// System prompt Go expression (e.g. "systemPrompt")
	system?: string
	// History slice Go expression - must have .Role and .Content fields (e.g. "history")
	history?: string
	// Variable name for the reply text (default: "claudeReply")
	output?: string
	// Claude model to use (default: "claude-sonnet-4-6")
	model?: string
	// Max tokens in response (default: 4096)
	max_tokens?: int
	// Locale Go expression to inject into system prompt prefix (e.g. "locale")
	locale?: string
	// Timezone Go expression to inject into system prompt prefix (e.g. "tz")
	timezone?: string
}

#OpenAIChatStep: {
	// openai.Chat - Call OpenAI Chat Completions API and get a text reply.
	// Uses OPENAI_API_KEY env var.
	action: "openai.Chat"
	// Current user message Go expression (e.g. "req.Message")
	user_message: string
	// System prompt Go expression (e.g. `"You are a helpful assistant"`)
	system?: string
	// Go expression appended to system prompt (e.g. "project.CueContent")
	system_context?: string
	// History slice Go expression - must have .Role and .Content fields (e.g. "history")
	history?: string
	// Variable name for the reply text (default: "openaiReply")
	output?: string
	// OpenAI model to use (default: "gpt-4o-mini")
	model?: string
	// Max tokens in response (default: 4096)
	max_tokens?: int
	// Explicit allowlist of tool-call methods.
	// Unqualified names resolve to the current service, qualified names use
	// "Service.Method" and require that Service is listed in architecture.depends.
	tools?: [...string]
	// OpenAI tool choice mode: "auto", "none", "required", or exact tool name.
	tool_choice?: string
	// Maximum tool-calling rounds before aborting (default: 6).
	max_rounds?: int
	// Optional variable receiving usage summary with PromptTokens / CompletionTokens / TotalTokens.
	output_usage?: string
	// Optional variable receiving tool call count.
	output_tool_calls?: string
	// Optional raw JSON Schema string expression for strict structured output.
	response_json_schema?: string
	// Optional response_format json_schema.name (default: "structured_response").
	response_json_name?: string
	// Optional response_format json_schema.strict (default: true).
	response_json_strict?: bool
	// Optional variable receiving parsed final JSON as map[string]any.
	output_json?: string
}

#OpenAIEmbedStep: {
	action: "openai.Embed"
	input:  string
	output: string
	model?: string
	dimensions?: int
	output_usage?: string
}

#OpenAIStreamStep: {
	// openai.Stream - Call OpenAI Chat Completions API in SSE mode and emit text chunks.
	// Uses OPENAI_API_KEY env var.
	action: "openai.Stream"
	// Current user message Go expression (e.g. "req.Message")
	user_message: string
	// System prompt Go expression (e.g. `"You are a helpful assistant"`)
	system?: string
	// Go expression appended to system prompt (e.g. "project.CueContent")
	system_context?: string
	// History slice Go expression - must have .Role and .Content fields (e.g. "history")
	history?: string
	// Optional accumulated output variable in addition to streaming chunks
	output?: string
	// OpenAI model to use (default: "gpt-4o")
	model?: string
	// Max tokens in response (default: 4096)
	max_tokens?: int
}

#PlanBuildAutomataStep: {
	// plan.BuildAutomata - Build automata.json from structured use cases.
	action: "plan.BuildAutomata"
	// Input expression containing usecases JSON/string/map/struct.
	input: string
	// Variable name for resulting automata document.
	output: string
}

#PlanBuildMicroPlanStep: {
	// plan.BuildMicroPlan - Build micro_plan.json from structured use cases + automata.
	action: "plan.BuildMicroPlan"
	// Input expression containing usecases JSON/string/map/struct.
	usecases: string
	// Input expression containing automata JSON/string/map/struct.
	automata: string
	// Variable name for resulting micro plan document.
	output: string
}

#CueEmitProjectStep: {
	// cue.EmitProject - Emit CUE project files from usecases + micro plan.
	action: "cue.EmitProject"
	// Input expression containing usecases JSON/string/map/struct.
	usecases: string
	// Input expression containing micro plan JSON/string/map/struct.
	micro_plan: string
	// Optional layout: "split" (default) or "single_file" for legacy cue/main.cue.
	layout?: string
	// Variable name for resulting file map (path -> content).
	output: string
}

#CueValidateProjectStep: {
	// cue.ValidateProject - Validate emitted project files via `ang validate --json`.
	action: "cue.ValidateProject"
	// Input expression containing file map (path -> content).
	files: string
	// Optional ANG binary path/expression. Default: "ang".
	binary?: string
	// Variable name for resulting diagnostics envelope.
	output: string
}

#CueWriteProjectFilesStep: {
	// cue.WriteProjectFiles - Write emitted project file map to disk.
	action: "cue.WriteProjectFiles"
	// Target root directory.
	root: string
	// Input expression containing file map (path -> content).
	files: string
	// Optional write mode: "upsert" or "replace_prefix". Default: "upsert".
	mode?: string
	// Optional prefixes to clean first in replace_prefix mode.
	prefixes?: [...string]
	// Variable name for write result.
	output: string
}
