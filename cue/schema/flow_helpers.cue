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

#ForEach: {
	_items: string, _as: string, _do: [...#FlowStep]
	action: "flow.For", each: _items, as: _as, do: _do
}

#TransitionTo: {
	_entity: string, _state: string
	action: "fsm.Transition", entity: _entity, to: _state
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
