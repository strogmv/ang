package emitter

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/strogmv/ang/angir/ir"
	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler/flowir"
)

func serviceConfigTestDef() *normalizer.ConfigDef {
	return &normalizer.ConfigDef{Fields: []normalizer.Field{
		{Name: "AppName", Type: "string", EnvVar: "APP_NAME", Default: `"shop"`},
		{Name: "PageSize", Type: "int", EnvVar: "PAGE_SIZE", Default: "50"},
		{Name: "MailEnabled", Type: "bool", EnvVar: "MAIL_ENABLED", Default: "true"},
		{Name: "APIToken", Type: "string", EnvVar: "API_TOKEN", IsOptional: true},
	}}
}

func emitConfigSource(t *testing.T, serviceConfig bool) string {
	t.Helper()
	root := t.TempDir()
	em := New(root, filepath.Join(root, "sdk"), "templates")
	em.ServiceConfig = serviceConfig
	if err := em.EmitConfig(serviceConfigTestDef()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "internal", "config", "config.go"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "config.go", data, 0); err != nil {
		t.Fatalf("generated config does not parse: %v\n%s", err, data)
	}
	return string(data)
}

// target.service_config: config.Default() holds the CUE defaults (and the
// runtime defaults ANG adds) without reading the environment.
func TestEmitConfigDefaultFromCUE(t *testing.T) {
	t.Parallel()

	src := emitConfigSource(t, true)
	for _, want := range []string{
		"func Default() *Config {",
		`AppName:     "shop",`,
		"PageSize:    50,",
		"MailEnabled: true,",
		`PGMaxConns:`,
		"func OrDefault(cfg *Config) *Config {",
	} {
		if !strings.Contains(strings.Join(strings.Fields(src), " "), strings.Join(strings.Fields(want), " ")) {
			t.Fatalf("config.go lacks %q:\n%s", want, src)
		}
	}
	defaultBody := src[strings.Index(src, "func Default()"):strings.Index(src, "func OrDefault")]
	if strings.Contains(defaultBody, "APIToken") || strings.Contains(defaultBody, "Getenv") {
		t.Fatalf("Default() sets only fields with a default and reads no environment:\n%s", defaultBody)
	}
	off := emitConfigSource(t, false)
	if strings.Contains(off, "func Default()") || strings.Contains(off, "OrDefault") {
		t.Fatal("without service_config the config is generated as before")
	}
	if strings.Contains(off, "}// dotEnvMu") {
		t.Fatal("the disabled block must not glue the next comment to Load")
	}
}

func TestServiceImplGetsCfgUnderServiceConfig(t *testing.T) {
	t.Parallel()

	svc := normalizer.Service{Name: "Orders", Methods: []normalizer.Method{{Name: "List"}}}
	auth := &normalizer.AuthDef{Service: "Auth"}
	authSvc := normalizer.Service{Name: "Auth", Methods: []normalizer.Method{{Name: "Login"}}}
	opts := serviceImplOptions{ServiceConfig: true}

	typeDecl, err := renderServiceImplTypeDecl(svc, nil, auth, opts)
	if err != nil {
		t.Fatal(err)
	}
	ctor, err := renderServiceImplConstructorDecl(svc, nil, auth, opts)
	if err != nil {
		t.Fatal(err)
	}
	authCtor, err := renderServiceImplConstructorDecl(authSvc, nil, auth, opts)
	if err != nil {
		t.Fatal(err)
	}
	src := "package service\n" + typeDecl + "\n" + ctor + "\n" + authCtor
	if _, err := parser.ParseFile(token.NewFileSet(), "impl.go", src, 0); err != nil {
		t.Fatalf("does not parse: %v\n%s", err, src)
	}
	if !strings.Contains(typeDecl, "Cfg *config.Config") {
		t.Fatalf("every service gets Cfg:\n%s", typeDecl)
	}
	if !strings.Contains(ctor, "func NewOrdersImpl() *OrdersImpl") || !strings.Contains(ctor, "Cfg: config.Default()") {
		t.Fatalf("the constructor keeps its parameters and starts from the defaults:\n%s", ctor)
	}
	if !strings.Contains(authCtor, "Cfg:          config.OrDefault(cfg)") && !strings.Contains(authCtor, "config.OrDefault(cfg)") {
		t.Fatalf("the auth service takes Cfg from its cfg argument:\n%s", authCtor)
	}
	plain, _ := renderServiceImplTypeDecl(svc, nil, auth)
	plainCtor, _ := renderServiceImplConstructorDecl(svc, nil, auth)
	if strings.Contains(plain+plainCtor, "Cfg") {
		t.Fatalf("without service_config nothing changes:\n%s\n%s", plain, plainCtor)
	}
}

func TestRuntimeAndTestContainersSetServiceCfg(t *testing.T) {
	t.Parallel()

	schema := &ir.Schema{Services: []ir.Service{{
		Name:    "Orders",
		Methods: []ir.Method{{Name: "List", Output: &ir.Entity{Name: "ListResponse"}}},
	}}}
	emit := func(on bool) (string, string) {
		root := t.TempDir()
		em := New(root, filepath.Join(root, "sdk"), "templates")
		em.GoModule = "example.com/shop"
		em.ServiceConfig = on
		em.IRSchema = schema
		ctx := em.AnalyzeContextFromIR(schema)
		ctx.GoModule = em.GoModule
		if err := em.EmitRuntimeContainer(ctx); err != nil {
			t.Fatal(err)
		}
		if err := em.EmitTestContainerFromIR(ctx, schema, nil, nil); err != nil {
			t.Fatal(err)
		}
		rc, err := os.ReadFile(filepath.Join(root, "internal", "bootstrap", "runtime_container.go"))
		if err != nil {
			t.Fatal(err)
		}
		tc, err := os.ReadFile(filepath.Join(root, "internal", "bootstrap", "test_container.gen.go"))
		if err != nil {
			t.Fatal(err)
		}
		return string(rc), string(tc)
	}
	rc, tc := emit(true)
	if !strings.Contains(rc, "if impl, ok := c.SvcOrders.(*service.OrdersImpl); ok {") || !strings.Contains(rc, "impl.Cfg = cfg") {
		t.Fatalf("the runtime container gives each service the loaded config:\n%s", rc)
	}
	if !strings.Contains(tc, "Config: config.Default()") || !strings.Contains(tc, "impl.Cfg = c.Config") {
		t.Fatalf("the test container starts from config.Default() and hands it on:\n%s", tc)
	}
	rc, tc = emit(false)
	if strings.Contains(rc, "impl.Cfg") || strings.Contains(tc, "config.Default()") || strings.Contains(tc, "impl.Cfg") {
		t.Fatal("without service_config the containers are generated as before")
	}
}

func TestConfigGetReadsServiceCfg(t *testing.T) {
	t.Parallel()

	render := func(infra map[string]any) string {
		st := &flowRenderState{declared: map[string]bool{}, pointers: map[string]bool{}, types: map[string]string{}, infraValues: infra}
		step := flowir.TypedStep{Name: "config.Get", Action: flowir.ConfigGet{Key: flowir.Expression{Source: `"APP_NAME"`}, Default: flowir.Expression{Source: `"shop"`}, Output: "name"}}
		out, ok := renderTypedStepSecretConfig(st, step, 1)
		if !ok {
			t.Fatal("config.Get not rendered")
		}
		return out
	}
	on := render(map[string]any{flowInfraKeyServiceConfig: map[string]string{"APP_NAME": "AppName"}})
	if !strings.Contains(on, "var name string = s.Cfg.AppName") || !strings.Contains(on, `name = "shop"`) {
		t.Fatalf("config.Get of a declared key reads s.Cfg:\n%s", on)
	}
	if off := render(nil); !strings.Contains(off, `os.Getenv("APP_NAME")`) {
		t.Fatalf("without service_config config.Get reads the environment:\n%s", off)
	}
	unknown := render(map[string]any{flowInfraKeyServiceConfig: map[string]string{"OTHER": "Other"}})
	if !strings.Contains(unknown, `os.Getenv("APP_NAME")`) {
		t.Fatalf("a key the config does not declare keeps reading the environment:\n%s", unknown)
	}
}
