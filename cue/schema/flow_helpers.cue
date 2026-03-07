package schema

// ============================================================================ 
// FLOW HELPERS - Shorthand definitions for common patterns 
// ============================================================================ 

#FindByID: {
	_entity: string
	_id:     string
	_var:    string
	_error?: string
	action: "repo.Find", source: _entity, input: _id, output: _var
	if _error != _|_ { error: _error }
}

#GetByID: {
	_entity: string, _id: string, _var: string
	action: "repo.Get", source: _entity, input: _id, output: _var
}

#Save: {
	_entity: string, _var: string
	action: "repo.Save", source: _entity, input: _var
}

#Delete: {
	_entity: string, _id: string
	action: "repo.Delete", source: _entity, input: _id
}

#List: {
	_entity: string, _method?: string, _input?: string, _var: string
	action: "repo.List", source: _entity, output: _var
	if _method != _|_ { method: _method }
	if _input != _|_ { input: _input }
}

#ListMap: {
	_from: string, _expr: string, _out: string, _as?: string
	action: "list.Map", from: _from, expr: _expr, output: _out
	if _as != _|_ { as: _as }
}

#ListReduce: {
	_from: string, _expr: string, _out: string, _as?: string, _initial?: string
	action: "list.Reduce", from: _from, expr: _expr, output: _out
	if _as != _|_ { as: _as }
	if _initial != _|_ { initial: _initial }
}

#ListGroupBy: {
	_from: string, _key: string, _out: string, _as?: string
	action: "list.GroupBy", from: _from, key: _key, output: _out
	if _as != _|_ { as: _as }
}

#ListDistinct: {
	_from: string, _out: string, _as?: string, _key?: string
	action: "list.Distinct", from: _from, output: _out
	if _as != _|_ { as: _as }
	if _key != _|_ { key: _key }
}

#ListChunk: {
	_from: string, _size: int | string, _out: string
	action: "list.Chunk", from: _from, size: _size, output: _out
}

#BatchRun: {
	_from: string, _do: [...#FlowStep], _size?: int | string, _as?: string
	action: "batch.Run", from: _from, do: _do
	if _size != _|_ { size: _size }
	if _as != _|_ { as: _as }
}

#Upsert: {
	_entity:   string
	_find:     string
	_input:    string
	_var:      string
	_ifNew?:   [...#FlowStep]
	_ifExists?: [...#FlowStep]
	action: "repo.Upsert", source: _entity, find: _find, input: _input, output: _var
	if _ifNew != _|_ { ifNew: _ifNew }
	if _ifExists != _|_ { ifExists: _ifExists }
}

#NewEntity: {
	_entity: string, _var: string
	action: "mapping.Map", output: _var, entity: _entity
}

#Set: {
	_field: string, _value: string
	action: "mapping.Assign", to: _field, value: _value
}

#SetID: {
	_field: string
	action: "uuid.New", output: _field
}

#SetNow: {
	_field: string
	action: "time.Now", output: _field
}

#SetResponse: {
	_field: string, _value: string
	action: "mapping.Assign", to: "resp.\(_field)", value: _value
}

#Copy: {
	_from:  string
	_to:    string
	action: "mapping.Map"
	input:  _from
	output: _to
}

#Require: {
	_condition: string, _error: string
	action: "logic.Check", condition: _condition, throw: _error
}

#RequireStatus: {
	_entity: string, _status: string, _error: string
	action: "logic.Check", condition: "\(_entity).Status == \"\(_status)\"", throw: _error
}

#RequireOwner: {
	_entity: string, _field: string, _value: string, _error: string
	action: "logic.Check", condition: "\(_entity).\(_field) == \(_value)", throw: _error
}

#InTransaction: {
	_steps: [...#FlowStep]
	action: "tx.Block", do: _steps
}

#Publish: {
	_event: string, _payload: string
	action: "event.Publish", name: _event, payload: _payload
}

#Outbox: {
	_event: string, _payload: string, _id?: string
	action: "event.Outbox", name: _event, payload: _payload
	if _id != _|_ { id: _id }
}

#WebhookVerify: {
	_payload: string, _signature: string, _out?: string, _secret?: string, _strict?: bool
	action: "webhook.VerifySignature", payload: _payload, signature: _signature
	if _out != _|_ { output: _out }
	if _secret != _|_ { secret: _secret }
	if _strict != _|_ { strict: _strict }
}

#QueueEnqueue: {
	_subject: string, _payload: string, _timeout?: string, _timeoutMs?: int
	action: "queue.Enqueue", subject: _subject, payload: _payload
	if _timeout != _|_ { timeout: _timeout }
	if _timeoutMs != _|_ { timeoutMs: _timeoutMs }
}

#QueueDequeue: {
	_subject: string, _out: string, _ackToken?: string, _timeout?: string, _timeoutMs?: int, _attempts?: int, _retries?: int, _backoffMs?: int, _jitterMs?: int
	action: "queue.Dequeue", subject: _subject, output: _out
	if _ackToken != _|_ { ackToken: _ackToken }
	if _timeout != _|_ { timeout: _timeout }
	if _timeoutMs != _|_ { timeoutMs: _timeoutMs }
	if _attempts != _|_ { attempts: _attempts }
	if _retries != _|_ { retries: _retries }
	if _backoffMs != _|_ { backoffMs: _backoffMs }
	if _jitterMs != _|_ { jitterMs: _jitterMs }
}

#QueueAck: {
	_subject: string, _messageID: string
	action: "queue.Ack", subject: _subject, messageID: _messageID
}

#QueueNack: {
	_subject: string, _messageID: string, _reason?: string
	action: "queue.Nack", subject: _subject, messageID: _messageID
	if _reason != _|_ { reason: _reason }
}

#DLQPublish: {
	_subject: string, _payload: string, _reason?: string
	action: "dlq.Publish", subject: _subject, payload: _payload
	if _reason != _|_ { reason: _reason }
}

#NotifySend: {
	_channel: string, _to: string, _template?: string, _text?: string, _subject?: string, _html?: string, _data?: string, _out?: string
	action: "notify.Send", channel: _channel, to: _to
	if _template != _|_ { template: _template }
	if _text != _|_ { text: _text }
	if _subject != _|_ { subject: _subject }
	if _html != _|_ { html: _html }
	if _data != _|_ { data: _data }
	if _out != _|_ { output: _out }
}

#ApprovalRequest: {
	_key: string, _title: string, _requestedBy: string, _approvers: string | [...string], _policy: string, _payload: string, _description?: string, _deadline?: string, _ttl?: string, _approvalId?: string, _status?: string
	action: "approval.Request", approvalKey: _key, title: _title, requestedBy: _requestedBy, approvers: _approvers, policy: _policy, payload: _payload
	if _description != _|_ { description: _description }
	if _deadline != _|_ { deadline: _deadline }
	if _ttl != _|_ { ttl: _ttl }
	if _approvalId != _|_ { approvalId: _approvalId }
	if _status != _|_ { status: _status }
}

#ApprovalWait: {
	_approvalId: string, _timeout?: string, _onTimeoutMode?: string, _onTimeout?: [...#FlowStep], _decision?: string, _status?: string, _decidedBy?: string, _decidedAt?: string, _reason?: string
	action: "approval.Wait", approvalId: _approvalId
	if _timeout != _|_ { timeout: _timeout }
	if _onTimeoutMode != _|_ { onTimeout: _onTimeoutMode }
	if _onTimeout != _|_ { onTimeout: _onTimeout }
	if _decision != _|_ { decision: _decision }
	if _status != _|_ { status: _status }
	if _decidedBy != _|_ { decidedBy: _decidedBy }
	if _decidedAt != _|_ { decidedAt: _decidedAt }
	if _reason != _|_ { reason: _reason }
}

#ApprovalDecide: {
	_approvalId: string, _decision: string, _actor: string, _reason?: string, _status?: string
	action: "approval.Decide", approvalId: _approvalId, decision: _decision, actor: _actor
	if _reason != _|_ { reason: _reason }
	if _status != _|_ { status: _status }
}

#When: {
	_if: string, _then: [...#FlowStep], _else?: [...#FlowStep]
	action: "flow.If", condition: _if, then: _then
	if _else != _|_ { "else": _else }
}

#FlowRecordEvent: {
	_name: string
	_payload?: string
	_out?: string
	action: "flow.RecordEvent", name: _name
	if _payload != _|_ { payload: _payload }
	if _out != _|_ { output: _out }
}

#FlowReplay: {
	_history: string
	_do?: [...#FlowStep]
	_onMismatch?: [...#FlowStep]
	_out?: string
	action: "flow.Replay", history: _history
	if _do != _|_ { do: _do }
	if _onMismatch != _|_ { onMismatch: _onMismatch }
	if _out != _|_ { output: _out }
}

#FlowHistoryGet: {
	_out: string
	_name?: string
	_limit?: int | string
	action: "flow.History.Get", output: _out
	if _name != _|_ { name: _name }
	if _limit != _|_ { limit: _limit }
}

#Switch: {
	_value:    string
	_cases:    [string]: [...#FlowStep]
	_default?: [...#FlowStep]
	action: "flow.Switch", value: _value, cases: _cases
	if _default != _|_ { default: _default }
}

#ForEach: {
	_items: string, _as: string, _do: [...#FlowStep]
	action: "flow.For", each: _items, as: _as, do: _do
}

#While: {
	_condition: string, _do: [...#FlowStep]
	action: "flow.While", condition: _condition, do: _do
}

#TransitionTo: {
	_entity: string, _state: string
	action: "fsm.Transition", entity: _entity, to: _state
}

// ============================================================================
// UNIVERSAL FLOW ACTIONS - Common cross-cutting concerns
// ============================================================================

#AuditLog: {
	_actor:   string
	_company: string
	_event:   string
	action: "audit.Log", actor: _actor, company: _company, event: _event
}

#RequireRole: {
	_userID:       string
	_companyID:    string
	_roles:        string
	_output?:      string
	_adminBypass?: bool
	action: "auth.RequireRole", userID: _userID, companyID: _companyID, roles: _roles
	if _output != _|_ { output: _output }
	if _adminBypass != _|_ { adminBypass: _adminBypass }
}

#CheckRole: {
	_user:      string
	_roles:     string
	_companyID?: string
	action: "auth.CheckRole", user: _user, roles: _roles
	if _companyID != _|_ { companyID: _companyID }
}

#CheckPermission: {
	_user:       string
	_permission: string
	_output?:    string
	_throw?:     string
	_code?:      string
	_status?:    string
	action: "rbac.CheckPermission", user: _user, permission: _permission
	if _output != _|_ { output: _output }
	if _throw != _|_ { throw: _throw }
	if _code != _|_ { code: _code }
	if _status != _|_ { status: _status }
}

#PolicyCheck: {
	_policy: string
	_user:   string
	_companyID?: string
	_output?:    string
	_throw?:     string
	_code?:      string
	_status?:    string
	action: "policy.Check", policy: _policy, user: _user
	if _companyID != _|_ { companyID: _companyID }
	if _output != _|_ { output: _output }
	if _throw != _|_ { throw: _throw }
	if _code != _|_ { code: _code }
	if _status != _|_ { status: _status }
}

#PolicyEvaluate: {
	_key: string
	_decision?: string
	_reason?:   string
	_effects?:  string
	_output?:   string
	_subject?:   string
	_resource?:  string
	_operation?: string
	_tenant?:    string
	_attrs?:     string
	_context?:   string
	action: "policy.Evaluate", policyKey: _key
	if _decision != _|_ { decision: _decision }
	if _reason != _|_ { reason: _reason }
	if _effects != _|_ { effects: _effects }
	if _output != _|_ { output: _output }
	if _subject != _|_ { subject: _subject }
	if _resource != _|_ { resource: _resource }
	if _operation != _|_ { operation: _operation }
	if _tenant != _|_ { tenant: _tenant }
	if _attrs != _|_ { attrs: _attrs }
	if _context != _|_ { context: _context }
}

#PolicyRequire: {
	_key: string
	_throw?: string
	_code?:  string
	_status?: string
	_decision?: string
	_reason?:   string
	_effects?:  string
	_output?:   string
	_subject?:   string
	_resource?:  string
	_operation?: string
	_tenant?:    string
	_attrs?:     string
	_context?:   string
	action: "policy.Require", policyKey: _key
	if _throw != _|_ { throw: _throw }
	if _code != _|_ { code: _code }
	if _status != _|_ { status: _status }
	if _decision != _|_ { decision: _decision }
	if _reason != _|_ { reason: _reason }
	if _effects != _|_ { effects: _effects }
	if _output != _|_ { output: _output }
	if _subject != _|_ { subject: _subject }
	if _resource != _|_ { resource: _resource }
	if _operation != _|_ { operation: _operation }
	if _tenant != _|_ { tenant: _tenant }
	if _attrs != _|_ { attrs: _attrs }
	if _context != _|_ { context: _context }
}

#PolicyDecide: {
	_key: string, _out: string
	_decision?: string
	_reason?:   string
	_effects?:  string
	_subject?:   string
	_resource?:  string
	_operation?: string
	_tenant?:    string
	_attrs?:     string
	_context?:   string
	action: "policy.Decide", policyKey: _key, output: _out
	if _decision != _|_ { decision: _decision }
	if _reason != _|_ { reason: _reason }
	if _effects != _|_ { effects: _effects }
	if _subject != _|_ { subject: _subject }
	if _resource != _|_ { resource: _resource }
	if _operation != _|_ { operation: _operation }
	if _tenant != _|_ { tenant: _tenant }
	if _attrs != _|_ { attrs: _attrs }
	if _context != _|_ { context: _context }
}

#JWTSign: {
	_claims: string
	_out:    string
	_secret?: string
	_alg?:    string
	_ttl?:    string
	action: "jwt.Sign", claims: _claims, output: _out
	if _secret != _|_ { secret: _secret }
	if _alg != _|_ { alg: _alg }
	if _ttl != _|_ { ttl: _ttl }
}

#JWTVerify: {
	_token: string
	_out:   string
	_secret?: string
	action: "jwt.Verify", token: _token, output: _out
	if _secret != _|_ { secret: _secret }
}

#OAuth2Token: {
	_url: string
	_out: string
	_clientID?:     string
	_clientSecret?: string
	_scope?:        string
	_audience?:     string
	_grantType?:    string
	_username?:     string
	_password?:     string
	_code?:         string
	_redirectURI?:  string
	_refreshToken?: string
	action: "oauth2.Token", tokenURL: _url, output: _out
	if _clientID != _|_ { clientID: _clientID }
	if _clientSecret != _|_ { clientSecret: _clientSecret }
	if _scope != _|_ { scope: _scope }
	if _audience != _|_ { audience: _audience }
	if _grantType != _|_ { grantType: _grantType }
	if _username != _|_ { username: _username }
	if _password != _|_ { password: _password }
	if _code != _|_ { code: _code }
	if _redirectURI != _|_ { redirectURI: _redirectURI }
	if _refreshToken != _|_ { refreshToken: _refreshToken }
}

#OAuth2Refresh: {
	_url:          string
	_refreshToken: string
	_out:          string
	_clientID?:     string
	_clientSecret?: string
	_scope?:        string
	_audience?:     string
	action: "oauth2.Refresh", tokenURL: _url, refreshToken: _refreshToken, output: _out
	if _clientID != _|_ { clientID: _clientID }
	if _clientSecret != _|_ { clientSecret: _clientSecret }
	if _scope != _|_ { scope: _scope }
	if _audience != _|_ { audience: _audience }
}

#Encrypt: {
	_input: string
	_out:   string
	_key?:  string
	_aad?:  string
	action: "crypto.Encrypt", input: _input, output: _out
	if _key != _|_ { key: _key }
	if _aad != _|_ { aad: _aad }
}

#Decrypt: {
	_input: string
	_out:   string
	_key?:  string
	_aad?:  string
	action: "crypto.Decrypt", input: _input, output: _out
	if _key != _|_ { key: _key }
	if _aad != _|_ { aad: _aad }
}

#PatchFields: {
	_target: string
	_from:   string
	_fields: string
	action: "entity.PatchNonZero", target: _target, from: _from, fields: _fields
}

#CopyNonEmpty: {
	_from:    string
	_to:      string
	_fields?: string
	action: "field.CopyNonEmpty", from: _from, to: _to
	if _fields != _|_ { fields: _fields }
}

#PatchValidated: {
	_target: string
	_from:   string
	_fields: [string]: {
		normalize?: "trim" | "lower" | "upper"
		format?:    "email" | "phone"
		unique?:    string
	}
	_source?: string
	action: "entity.PatchValidated", target: _target, from: _from, fields: _fields
	if _source != _|_ { source: _source }
}

#Paginate: {
	_input:         string
	_offset:        string
	_limit:         string
	_output:        string
	_total?:        string
	_defaultLimit?: int
	action: "list.Paginate", input: _input, offset: _offset, limit: _limit, output: _output
	if _total != _|_ { total: _total }
	if _defaultLimit != _|_ { defaultLimit: _defaultLimit }
}

// ============================================================================
// BATCH 2: STRING, ENUM, LIST, TIME, MAP HELPERS
// ============================================================================

#Normalize: {
	_input:   string
	_output:  string
	_mode?:   "lower" | "upper" | "trim"
	action: "str.Normalize", input: _input, output: _output
	if _mode != _|_ { mode: _mode }
}

#ValidateEnum: {
	_value:   string
	_allowed: string
	_throw:   string
	action: "enum.Validate", value: _value, allowed: _allowed, throw: _throw
}

#SortBy: {
	_items: string
	_by:    string
	_desc?: bool
	action: "list.Sort", items: _items, by: _by
	if _desc != _|_ { desc: _desc }
}

#Filter: {
	_from:      string
	_condition: string
	_output:    string
	_as?:       string
	action: "list.Filter", from: _from, condition: _condition, output: _output
	if _as != _|_ { as: _as }
}

#Enrich: {
	_items:        string
	_lookupSource: string
	_lookupInput:  string
	_set:          string
	_as?:          string
	action: "list.Enrich", items: _items, lookupSource: _lookupSource, lookupInput: _lookupInput, set: _set
	if _as != _|_ { as: _as }
}

#ParseTime: {
	_value:   string
	_output:  string
	_format?: string
	action: "time.Parse", value: _value, output: _output
	if _format != _|_ { format: _format }
}

#CheckExpiry: {
	_value:   string
	_throw:   string
	_mustBe?: "future" | "past"
	action: "time.CheckExpiry", value: _value, throw: _throw
	if _mustBe != _|_ { mustBe: _mustBe }
}

#BuildMap: {
	_from:   string
	_key:    string
	_value:  string
	_output: string
	_as?:    string
	action: "map.Build", from: _from, key: _key, value: _value, output: _output
	if _as != _|_ { as: _as }
}

// ============================================================================
// CRUD PATTERNS - Standard operations
// ============================================================================

#CRUDCreate: {
	_entity:     string
	_var:        string | *"new\( _entity )"
	_ownerField: string | *"CompanyID"
	_ownerValue: string | *"req.CompanyID"
	_event?:     string
	
	_out: [
		#NewEntity & { _entity: _entity, _var: _var },
		#SetID & { _field: "\(_var).ID" },
		#SetNow & { _field: "\(_var).CreatedAt" },
		#SetNow & { _field: "\(_var).UpdatedAt" },
		#Set & { _field: "\(_var).\(_ownerField)", _value: _ownerValue },
		#Set & { _field: "\(_var).Status", _value: "\"draft\"" },
		#InTransaction & {
			_steps: [
				#Save & { _entity: _entity, _var: _var },
				if _event != _|_ {
					#Publish & { _event: _event, _payload: "domain.\(_event){ID: \(_var).ID}" }
				}
			]
		},
		#SetResponse & { _field: "ID", _value: "\(_var).ID" }
	]
}

#CRUDUpdate: {
	_entity:     string
	_id:         string | *"req.\(_entity)ID"
	_var:        string | *"existing"
	_ownerField: string | *"CompanyID"
	_ownerValue: string | *"req.CompanyID"
	_event?:     string

	_out: [
		#FindByID & { _entity: _entity, _id: _id, _var: _var },
		#RequireOwner & { _entity: _var, _field: _ownerField, _value: _ownerValue, _error: "Access denied" },
		#InTransaction & {
			_steps: [
				#Copy & { _from: "req", _to: _var },
				#SetNow & { _field: "\(_var).UpdatedAt" },
				#Save & { _entity: _entity, _var: _var },
				if _event != _|_ {
					#Publish & { _event: _event, _payload: "domain.\(_event){ID: \(_var).ID}" }
				}
			]
		},
		#SetResponse & { _field: "Ok", _value: "true" }
	]
}

#CRUDDelete: {
	_entity:     string
	_id:         string | *"req.\(_entity)ID"
	_var:        string | *"existing"
	_ownerField: string | *"CompanyID"
	_ownerValue: string | *"req.CompanyID"

	_out: [
		#FindByID & { _entity: _entity, _id: _id, _var: _var },
		#RequireOwner & { _entity: _var, _field: _ownerField, _value: _ownerValue, _error: "Access denied" },
		#InTransaction & {
			_steps: [
				#Delete & { _entity: _entity, _id: _id }
			]
		},
		#SetResponse & { _field: "Ok", _value: "true" }
	]
}

#Query: {
	_entity: string, _method: string, _input?: string, _var: string
	action: "repo.Query", source: _entity, method: _method, output: _var
	if _input != _|_ { input: _input }
}

#IdempotencyDeriveKey: {
	_from: [...string], _out: string, _prefix?: string
	action: "idempotency.DeriveKey", from: _from, output: _out
	if _prefix != _|_ { prefix: _prefix }
}

#IdempotencyCheck: {
	_key: string
	action: "idempotency.Check", key: _key
}

#IdempotencySaveResult: {
	_key: string, _ttl?: string
	action: "idempotency.SaveResult", key: _key
	if _ttl != _|_ { ttl: _ttl }
}

#RateLimit: {
	_key: string, _rps: int, _throw?: string
	action: "ratelimit.Limit", key: _key, rps: _rps
	if _throw != _|_ { throw: _throw }
}

#ConcurrencyRun: {
	_key: string, _max: int, _do: [...#FlowStep], _throw?: string
	action: "concurrency.Run", key: _key, max: _max, do: _do
	if _throw != _|_ { throw: _throw }
}

#CircuitBreaker: {
	_name: string, _do: [...#FlowStep], _threshold?: int, _openTTL?: string, _throw?: string
	action: "circuit.Breaker", name: _name, do: _do
	if _threshold != _|_ { threshold: _threshold }
	if _openTTL != _|_ { openTTL: _openTTL }
	if _throw != _|_ { throw: _throw }
}

#BulkheadRun: {
	_name: string, _max: int, _do: [...#FlowStep], _throw?: string
	action: "bulkhead.Run", name: _name, max: _max, do: _do
	if _throw != _|_ { throw: _throw }
}

#LogEmit: {
	_message: string, _level?: string, _fields?: [string]: string
	action: "log.Emit", message: _message
	if _level != _|_ { level: _level }
	if _fields != _|_ { fields: _fields }
}

#MetricEmit: {
	_name: string, _kind?: string, _value?: string, _labels?: [string]: string
	action: "metric.Emit", name: _name
	if _kind != _|_ { kind: _kind }
	if _value != _|_ { value: _value }
	if _labels != _|_ { labels: _labels }
}

#TraceSpan: {
	_name: string, _do: [...#FlowStep], _attrs?: [string]: string
	action: "trace.Span", name: _name, do: _do
	if _attrs != _|_ { attrs: _attrs }
}

#SLOBudget: {
	_duration: string, _do: [...#FlowStep], _name?: string
	action: "slo.Budget", duration: _duration, do: _do
	if _name != _|_ { name: _name }
}
