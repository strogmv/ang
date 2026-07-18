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
