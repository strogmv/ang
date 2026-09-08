package emitter

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
)

type generatedDIRequirement struct {
	file     string
	contains string
	reason   string
}

// ValidateGeneratedDI checks capability wiring in the generated bootstrap.
// It runs before the build transaction commits, so a missing setter cannot
// leave a partially generated application behind.
func ValidateGeneratedDI(backendDir string, ctx MainContext, auth *normalizer.AuthDef) error {
	var requirements []generatedDIRequirement
	requireMain := func(contains, reason string) {
		requirements = append(requirements, generatedDIRequirement{"cmd/server/main.go", contains, reason})
	}
	requireEffectRegistry := func(contains, reason string) {
		requirements = append(requirements, generatedDIRequirement{"internal/bootstrap/effect_registry.gen.go", contains, reason})
	}
	if ctx.HasSQL {
		requireMain("pgxpool.NewWithConfig(ctx, poolCfg)", "SQL capability requires a PostgreSQL pool")
		requireMain("bootstrap.NewRuntimeContainer(", "SQL capability requires runtime-container wiring")
	}
	if ctx.HasMongo {
		requireMain("mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURL))", "Mongo capability requires a Mongo client")
		requireMain("mongoClient,", "Mongo capability requires runtime-container wiring")
	}
	if ctx.HasNats {
		requireMain("nats.NewClient(cfg.NatsURL)", "NATS capability requires a NATS client")
		requireMain("publisher = natsClient", "NATS capability requires publisher wiring")
	}
	if ctx.HasS3 {
		requireMain("s3.New(ctx, cfg.AWSRegion, cfg.S3Bucket, cfg.S3Endpoint)", "S3 capability requires a storage client")
		requireMain("s3Client,", "S3 capability requires runtime-container wiring")
	}
	if ctx.HasScheduler {
		requireMain("scheduler.New(publisher, scheduler.DefaultSchedules)", "scheduler capability requires scheduler construction")
		requireMain("sched.Start(ctx)", "scheduler capability requires startup wiring")
	}
	if ctx.HasSession {
		requireMain("r.Use(transport.SessionMiddleware)", "session capability requires HTTP middleware wiring")
	}
	if ctx.HasNotificationsService || ctx.HasNotificationDispatch {
		requireMain("notifications.NewDispatcher(cfg)", "notification capability requires dispatcher construction")
		requireMain("notificationDispatcher,", "notification capability requires runtime-container wiring")
	}
	refreshStore := strings.ToLower(strings.TrimSpace(ctx.AuthRefreshStore))
	if refreshStore == "" && auth != nil {
		refreshStore = strings.ToLower(strings.TrimSpace(auth.RefreshStore))
	}
	switch refreshStore {
	case "memory":
		requireEffectRegistry("reg.RefreshStore = authstore.NewMemoryStore()", "memory refresh-store capability requires store construction")
	case "redis":
		requireEffectRegistry("reg.RefreshStore = authredis.NewStore(redisClient)", "Redis refresh-store capability requires store construction")
	case "postgres":
		requireEffectRegistry("reg.RefreshStore = authpg.NewStore(pgPool)", "PostgreSQL refresh-store capability requires store construction")
	case "hybrid":
		requireEffectRegistry("reg.RefreshStore = authhybrid.NewStore(authpg.NewStore(pgPool), authredis.NewStore(redisClient))", "hybrid refresh-store capability requires both backing stores")
	}
	if auth != nil && strings.EqualFold(strings.TrimSpace(auth.Mode), "opaque_session_cookie") {
		requirements = append(requirements,
			generatedDIRequirement{"cmd/server/main.go", "transport.SetRedisClient(redisClient)", "opaque session auth requires Redis bootstrap wiring"},
			generatedDIRequirement{"internal/transport/http/common.go", "authSessionStore = statestoreredis.New(c)", "opaque session auth requires a session store"},
			generatedDIRequirement{"internal/transport/http/common.go", "authRefreshStore = authredis.NewStore(c)", "opaque session auth requires a refresh store"},
		)
	}
	if ctx.HasCache {
		requirements = append(requirements, generatedDIRequirement{
			"cmd/server/main.go", "transport.SetRedisClient(redisClient)", "cache capability requires Redis transport wiring",
		})
	}
	if redisIsBootstrapped(ctx, refreshStore) {
		requirements = append(requirements, redisClientRequirements(backendDir)...)
	}
	seen := map[string]struct{}{}
	for _, requirement := range requirements {
		key := requirement.file + "|" + requirement.contains
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		path := filepath.Join(backendDir, filepath.FromSlash(requirement.file))
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("%s: read %s: %w", requirement.reason, requirement.file, err)
		}
		if !strings.Contains(string(data), requirement.contains) {
			return fmt.Errorf("%s: %s does not contain %q", requirement.reason, requirement.file, requirement.contains)
		}
	}
	return nil
}

// redisIsBootstrapped mirrors the condition under which the bootstrap template
// constructs redisClient. Without it there is nothing to hand to a package
// setter, so the wiring requirements below do not apply.
func redisIsBootstrapped(ctx MainContext, refreshStore string) bool {
	return ctx.HasCache || refreshStore == "redis" || refreshStore == "hybrid"
}

// redisClientRequirements demands bootstrap wiring for every generated package
// under internal/pkg that exposes SetRedisClient.
//
// Emitting a package is not the same as connecting it: presence shipped for
// months with its setter never called, so online status silently lived in
// process memory and never expired, and the session store had the same defect
// before it (cookie logins 401'd in production). The setter exists precisely
// because the package cannot work without the client, so a generated package
// that declares one and is never handed a client is a build error, not a
// runtime surprise.
func redisClientRequirements(backendDir string) []generatedDIRequirement {
	pkgRoot := filepath.Join(backendDir, "internal", "pkg")
	entries, err := os.ReadDir(pkgRoot)
	if err != nil {
		return nil
	}
	var out []generatedDIRequirement
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pkg := entry.Name()
		if !declaresRedisSetter(filepath.Join(pkgRoot, pkg)) {
			continue
		}
		out = append(out, generatedDIRequirement{
			file:     "cmd/server/main.go",
			contains: pkg + ".SetRedisClient(redisClient)",
			reason:   fmt.Sprintf("package internal/pkg/%s exposes SetRedisClient and requires Redis bootstrap wiring", pkg),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].contains < out[j].contains })
	return out
}

// declaresRedisSetter reports whether any Go file directly in dir declares
// SetRedisClient.
func declaresRedisSetter(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		if strings.Contains(string(data), "func SetRedisClient(") {
			return true
		}
	}
	return false
}
