package schema

// TypeMapping defines how CUE types map to target language types.
// This is the single source of truth for type conversions.
// The compiler reads this instead of hardcoding type logic.

#TypeMapping: {
	// Core scalar types
	string: #TypeInfo & {
		go: {
			type:        "string"
			zero:        "\"\""
			null_helper: "sql.NullString"
		}
		ts: {
			type: "string"
			zod:  "z.string()"
		}
		sql: {
			type: "TEXT"
		}
	}

	int: #TypeInfo & {
		go: {
			type:        "int"
			zero:        "0"
			null_helper: "sql.NullInt64"
		}
		ts: {
			type: "number"
			zod:  "z.number().int()"
		}
		sql: {
			type: "INTEGER"
		}
	}

	int64: #TypeInfo & {
		go: {
			type:        "int64"
			zero:        "0"
			null_helper: "sql.NullInt64"
		}
		ts: {
			type: "number"
			zod:  "z.number().int()"
		}
		sql: {
			type: "BIGINT"
		}
	}

	float64: #TypeInfo & {
		go: {
			type:        "float64"
			zero:        "0.0"
			null_helper: "sql.NullFloat64"
		}
		ts: {
			type: "number"
			zod:  "z.number()"
		}
		sql: {
			type: "DOUBLE PRECISION"
		}
	}

	bool: #TypeInfo & {
		go: {
			type:        "bool"
			zero:        "false"
			null_helper: "sql.NullBool"
		}
		ts: {
			type: "boolean"
			zod:  "z.boolean()"
		}
		sql: {
			type: "BOOLEAN"
		}
	}

	// Special types
	"time.Time": #TypeInfo & {
		go: {
			type:        "time.Time"
			pkg:         "time"
			zero:        "time.Time{}"
			null_helper: "sql.NullTime"
		}
		ts: {
			type: "string"
			zod:  "z.string().datetime()"
		}
		sql: {
			type: "TIMESTAMPTZ"
		}
	}

	UUID: #TypeInfo & {
		go: {
			type:        "string"
			zero:        "\"\""
			null_helper: "sql.NullString"
		}
		ts: {
			type: "string"
			zod:  "z.string().uuid()"
		}
		sql: {
			type: "UUID"
		}
	}

	email: #TypeInfo & {
		go: {
			type:        "string"
			zero:        "\"\""
			null_helper: "sql.NullString"
			validate:    "email"
		}
		ts: {
			type: "string"
			zod:  "z.string().email()"
		}
		sql: {
			type: "TEXT"
		}
	}

	url: #TypeInfo & {
		go: {
			type:        "string"
			zero:        "\"\""
			null_helper: "sql.NullString"
			validate:    "url"
		}
		ts: {
			type: "string"
			zod:  "z.string().url()"
		}
		sql: {
			type: "TEXT"
		}
	}

	json: #TypeInfo & {
		go: {
			type: "json.RawMessage"
			pkg:  "encoding/json"
			zero: "nil"
		}
		ts: {
			type: "unknown"
			zod:  "z.unknown()"
		}
		sql: {
			type: "JSONB"
		}
	}

	// Currency/Money (example of domain-specific type)
	money: #TypeInfo & {
		go: {
			type:        "int64"
			zero:        "0"
			null_helper: "sql.NullInt64"
			comment:     "Amount in cents"
		}
		ts: {
			type: "number"
			zod:  "z.number().int().nonnegative()"
		}
		sql: {
			type: "BIGINT"
		}
	}
}

// TypeInfo schema for each target language
#TypeInfo: {
	go?: {
		type:         string
		pkg?:         string
		zero?:        string
		null_helper?: string
		validate?:    string
		comment?:     string
	}
	ts?: {
		type: string
		zod:  string
	}
	sql?: {
		type: string
	}
}

// ListMapping defines how list types are constructed
#ListMapping: {
	go:   "[]{{.ItemType}}"
	ts:   "{{.ItemType}}[]"
	sql:  "{{.ItemType}}[]" // Postgres array syntax
}

// MapMapping defines how map/dict types are constructed
#MapMapping: {
	go:   "map[{{.KeyType}}]{{.ValueType}}"
	ts:   "Record<{{.KeyType}}, {{.ValueType}}>"
	sql:  "JSONB" // Maps are stored as JSONB in SQL
}

// OptionalMapping defines how optional types are handled
#OptionalMapping: {
	go:   "*{{.Type}}" // Go uses pointers for optional
	ts:   "{{.Type}} | null"
}

// EntityRefMapping defines how entity references are written
#EntityRefMapping: {
	go:   "domain.{{.EntityName}}"
	ts:   "{{.EntityName}}"
}
