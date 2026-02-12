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
