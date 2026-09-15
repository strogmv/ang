package emitter

import (
	"strings"

	"github.com/strogmv/ang-ir/ir"
)

// UsageCapabilities names the infrastructure a project actually needs, as
// capability strings the build adds to the target's set. Each rule matches the
// condition under which the generated runtime imports that output, widened
// where missing code would not compile while an extra client is harmless.
func UsageCapabilities(ctx MainContext, schema *ir.Schema) []string {
	var services []ir.Service
	var entities []ir.Entity
	if schema != nil {
		services = schema.Services
		entities = schema.Entities
	}
	store := strings.ToLower(strings.TrimSpace(ctx.AuthRefreshStore))
	hasAuth := strings.TrimSpace(ctx.AuthService) != ""

	var out []string
	add := func(ok bool, name string) {
		if ok {
			out = append(out, name)
		}
	}
	add(ctx.HasCache || store == "redis" || store == "hybrid" || anyServiceHasStateActionsIR(services), "uses_redis_client")
	add(ctx.HasMongo || hasMongoRepoEntitiesIR(entities), "uses_mongo")
	add(ctx.HasS3 || anyServiceHasStorageIR(services), "uses_s3")
	add(hasAuth && store == "memory", "uses_refresh_store_memory")
	add(hasAuth && (store == "postgres" || store == "hybrid"), "uses_refresh_store_postgres")
	add(hasAuth && store == "hybrid", "uses_refresh_store_hybrid")
	return out
}
