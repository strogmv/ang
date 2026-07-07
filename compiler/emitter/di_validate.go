package emitter

import (
	"fmt"
	"os"
	"path/filepath"
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
		requireMain("authstore.NewMemoryStore()", "memory refresh-store capability requires store construction")
	case "redis":
		requireMain("authredis.NewStore(redisClient)", "Redis refresh-store capability requires store construction")
	case "postgres":
		requireMain("authpg.NewStore(pgPool)", "PostgreSQL refresh-store capability requires store construction")
	case "hybrid":
		requireMain("authhybrid.NewStore(authpg.NewStore(pgPool), authredis.NewStore(redisClient))", "hybrid refresh-store capability requires both backing stores")
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
