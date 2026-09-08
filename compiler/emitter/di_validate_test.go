package emitter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang-ir/normalizer"
)

func TestValidateGeneratedDIOpaqueSessionRequiresCompleteWiring(t *testing.T) {
	root := t.TempDir()
	write := func(path, content string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("cmd/server/main.go", "transport.SetRedisClient(redisClient)")
	write("internal/transport/http/common.go", "authSessionStore = statestoreredis.New(c)\nauthRefreshStore = authredis.NewStore(c)")
	auth := &normalizer.AuthDef{Mode: "opaque_session_cookie"}
	if err := ValidateGeneratedDI(root, MainContext{}, auth); err != nil {
		t.Fatalf("complete wiring rejected: %v", err)
	}
	write("internal/transport/http/common.go", "authSessionStore = statestoreredis.New(c)")
	if err := ValidateGeneratedDI(root, MainContext{}, auth); err == nil {
		t.Fatal("expected missing refresh-store wiring error")
	}
}

func TestValidateGeneratedDICapabilityMatrix(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "cmd", "server", "main.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	complete := `
pgxpool.NewWithConfig(ctx, poolCfg)
mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURL))
nats.NewClient(cfg.NatsURL)
publisher = natsClient
s3.New(ctx, cfg.AWSRegion, cfg.S3Bucket, cfg.S3Endpoint)
s3Client,
mongoClient,
bootstrap.NewRuntimeContainer(
scheduler.New(publisher, scheduler.DefaultSchedules)
sched.Start(ctx)
r.Use(transport.SessionMiddleware)
notifications.NewDispatcher(cfg)
notificationDispatcher,
`
	if err := os.WriteFile(path, []byte(complete), 0o644); err != nil {
		t.Fatal(err)
	}
	effectRegistryPath := filepath.Join(root, "internal", "bootstrap", "effect_registry.gen.go")
	if err := os.MkdirAll(filepath.Dir(effectRegistryPath), 0o755); err != nil {
		t.Fatal(err)
	}
	const effectRegistry = "reg.RefreshStore = authhybrid.NewStore(authpg.NewStore(pgPool), authredis.NewStore(redisClient))"
	if err := os.WriteFile(effectRegistryPath, []byte(effectRegistry), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := MainContext{HasSQL: true, HasMongo: true, HasNats: true, HasS3: true, HasScheduler: true, HasSession: true, HasNotificationDispatch: true, AuthRefreshStore: "hybrid"}
	if err := ValidateGeneratedDI(root, ctx, nil); err != nil {
		t.Fatalf("complete capability wiring rejected: %v", err)
	}
	if err := os.WriteFile(path, []byte(strings.ReplaceAll(complete, "publisher = natsClient", "")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateGeneratedDI(root, ctx, nil); err == nil || !strings.Contains(err.Error(), "publisher wiring") {
		t.Fatalf("expected missing NATS publisher error, got %v", err)
	}
	if err := os.WriteFile(path, []byte(complete), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(effectRegistryPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateGeneratedDI(root, ctx, nil); err == nil || !strings.Contains(err.Error(), "hybrid refresh-store") {
		t.Fatalf("expected missing hybrid refresh-store error, got %v", err)
	}
}

func TestValidateGeneratedDIRequiresRedisSetterWiring(t *testing.T) {
	root := t.TempDir()
	write := func(path, content string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("internal/pkg/presence/store.go", "package presence\n\nfunc SetRedisClient(client *redis.Client) {}\n")
	write("internal/pkg/logger/logger.go", "package logger\n")
	ctx := MainContext{HasCache: true}

	write("cmd/server/main.go", "transport.SetRedisClient(redisClient)")
	err := ValidateGeneratedDI(root, ctx, nil)
	if err == nil {
		t.Fatal("expected an unwired presence setter to fail validation")
	}
	if !strings.Contains(err.Error(), "presence.SetRedisClient(redisClient)") {
		t.Fatalf("expected the message to name the missing call, got %v", err)
	}

	write("cmd/server/main.go", "transport.SetRedisClient(redisClient)\npresence.SetRedisClient(redisClient)")
	if err := ValidateGeneratedDI(root, ctx, nil); err != nil {
		t.Fatalf("complete wiring rejected: %v", err)
	}
}

func TestValidateGeneratedDISkipsRedisSettersWithoutRedis(t *testing.T) {
	root := t.TempDir()
	presence := filepath.Join(root, "internal", "pkg", "presence")
	if err := os.MkdirAll(presence, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(presence, "store.go"), []byte("package presence\n\nfunc SetRedisClient(client *redis.Client) {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	main := filepath.Join(root, "cmd", "server", "main.go")
	if err := os.MkdirAll(filepath.Dir(main), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(main, []byte("func main() {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	// No cache and no Redis refresh store: the bootstrap builds no client, so
	// the setter cannot be called and must not be demanded.
	if err := ValidateGeneratedDI(root, MainContext{}, nil); err != nil {
		t.Fatalf("expected no Redis wiring requirement, got %v", err)
	}
}
