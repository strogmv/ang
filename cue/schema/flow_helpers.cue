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
	action: "mapping.Assign", to: _field, value: "uuid.NewString()"
}

#SetNow: {
	_field: string
	action: "mapping.Assign", to: _field, value: "time.Now().UTC().Format(time.RFC3339)"
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

#When: {
	_if: string, _then: [...#FlowStep], _else?: [...#FlowStep]
	action: "flow.If", condition: _if, then: _then
	if _else != _|_ { "else": _else }
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
