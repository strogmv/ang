package domain

import "github.com/strogmv/ang/cue/schema"

User: schema.#Entity & {
	description: "User account"
	fields: {
		id: {
			type: "uuid"
			required: true
		}
		email: {
			type: "string"
			required: true
			unique: true
		}
		name: {
			type: "string"
			required: true
		}
	}
}
