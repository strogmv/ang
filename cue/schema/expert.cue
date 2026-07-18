package schema

// Expert knowledge is declarative input for compiler/expert. It is distinct
// from #PackRegistry: planner packs shape generated application structure,
// while expert packs derive auditable findings from normalized facts.

#ExpertTruthState: "known" | "unknown" | "conflict"
#ExpertFindingStatus: "confirmed" | "hypothesis" | "unknown" | "conflict"
#ExpertRisk: "low" | "medium" | "high" | "critical"

// #ExpertCondition is intentionally a small data DSL. A rule engine must not
// evaluate arbitrary Go, shell, or project-provided CUE expressions.
#ExpertCondition: {
	op: "fact_exists" | "fact_state" | "string_equals" | "string_in"

	fact_kind?: string
	subject?:   string
	predicate?: string
	state?:     #ExpertTruthState
	value?:     string
	values?:    [...string]

	if op == "fact_state" {
		state: #ExpertTruthState
		value?: _|_
		values?: _|_
	}
	if op == "string_equals" {
		value: string
		state?: _|_
		values?: _|_
	}
	if op == "string_in" {
		values: [...string]
		state?: _|_
		value?: _|_
	}
	if op == "fact_exists" {
		state?: _|_
		value?: _|_
		values?: _|_
	}
}

// V1 conclusions only produce findings. Changes/proposals are a later phase
// and must not be smuggled into an inference pack before sandbox verification.
#ExpertConclusion: {
	kind:       "finding"
	code:       string
	severity:   "info" | "warning" | "error"
	summary:    string
	status:     #ExpertFindingStatus | *"confirmed"
	risk:       #ExpertRisk | *"low"
}

#ExpertRule: {
	// Stable ID, for example security.auth.endpoint_requires_actor.
	id:          string
	version:     string
	description?: string
	priority:     int | *100

	required_kinds?:  [...string]
	conditions:       [...#ExpertCondition]
	conclusions:      [...#ExpertConclusion]
	conflict_keys?:   [...string]
	base_confidence:  number & >=0 & <=1 | *0.5
	auto_apply:       false | *false
	risk:             #ExpertRisk | *"low"
}

#ExpertKnowledgePack: {
	schema:      "ang/knowledge-pack/v1"
	name:        string
	version:     string
	description?: string
	rules:       [...#ExpertRule]
}

#ExpertKnowledgeRegistry: {
	packs:  [string]: #ExpertKnowledgePack
	active?: [...string]
}
