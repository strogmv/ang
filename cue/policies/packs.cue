package policies

import "github.com/strogmv/ang/cue/schema"

// Pack rules are planner-facing declarations. They describe how an upper-layer
// generator such as sandbox may recognize and lower domain intent into explicit
// canonical metadata before handing it to ANG.
#Packs: schema.#PackRegistry & {
	conflict_strategy: "highest_priority"
	active: ["auth_profile", "messaging_community", "commerce"]

	policies: {
		auth_profile:       schema.#AuthProfilePackExample
		messaging_community: schema.#MessagingCommunityPackExample
		commerce:           schema.#CommercePackExample
	}
}
