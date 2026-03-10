package schema

#EffectKind:
	"db" |
	"ai" |
	"storage" |
	"session" |
	"events" |
	"http" |
	"cache" |
	"state" |
	"config" |
	"time" |
	"id" |
	"crypto"

#DBHandler:      "postgres" | "sqlite" | "stub"
#AIHandler:      "openai" | "anthropic" | "mock"
#StorageHandler: "s3" | "gcs" | "local" | "memory"
#SessionHandler: "cookie" | "memory"
#EventsHandler:  "nats" | "redis" | "memory" | "noop"
#HTTPHandler:    "default" | "mock"
#CacheHandler:   "redis" | "memory" | "noop"
#StateHandler:   "redis" | "memory"

#MiddlewareKind: "retry" | "cache" | "trace" | "metrics" | "timeout" | "log"

#Middleware: {
	type: #MiddlewareKind

	attempts?: int
	backoff?:  string
	on?:       [...int]

	ttl?: string
	key?: string

	duration?: string
	level?:    "debug" | "info" | "warn"
}
