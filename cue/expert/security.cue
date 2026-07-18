package expert

import "github.com/strogmv/ang/cue/schema"

// pack is opt-in. It is loaded only when a caller explicitly passes this
// directory to `ang advise --pack`; it never changes generated application
// behavior.
pack: schema.#ExpertKnowledgePack & {
	schema:      "ang/knowledge-pack/v1"
	name:        "security"
	version:     "v1"
	description: "Conservative security findings from explicit extracted evidence."
	rules: [
		{
			id:          "security.auth.permit_all_rule"
			version:     "v1"
			description: "Flags an explicitly extracted permit-all security rule for review."
			priority:    200
			required_kinds: ["security_rule"]
			conditions: [{
				op:        "string_in"
				fact_kind: "security_rule"
				values:    ["permitAll", "permitAll()"]
			}]
			conclusions: [{
				kind:     "finding"
				code:     "EXPERT_SECURITY_PERMIT_ALL"
				severity: "warning"
				summary:  "An extracted security rule permits unauthenticated access; verify that the scope is intentionally public."
				risk:     "high"
			}]
			conflict_keys:   ["security.auth.access_policy"]
			base_confidence: 0.9
			risk:            "high"
		},
	]
}
