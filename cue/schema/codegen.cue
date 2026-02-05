package schema

// Глобальные правила генерации кода
#Codegen: {
	lang: "go"
	
	// Маппинг типов: CUE Path -> Go Type
	type_mapping: {
		"time.Time": {
			type: "time.Time"
			pkg:  "time"
			sql:  "TIMESTAMPTZ"
			null: "nullTime"
		}
		"UUID": {
			type: "string"
			sql:  "UUID"
			null: "nullString"
		}
		"int": {
			type: "int"
			sql:  "BIGINT"
			null: "nullInt"
		}
		"string": {
			type: "string"
			sql:  "TEXT"
			null: "nullString"
		}
		"bool": {
			type: "bool"
			sql:  "BOOLEAN"
			null: "nullBool"
		}
	}
	
	// Настройки модулей (Transformers)
	features: {
		images: {
			enabled: bool | *false
			thumb_suffix: string | *"_thumb"
		}
		auth: {
			enabled: bool | *false
		}
	}
}
