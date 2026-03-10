package emitter

import (
	"bytes"
	"fmt"
	"go/format"
	"path/filepath"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

// EmitTestContainerFromIR generates an in-memory bootstrap container with auto-wired mocks.
func (e *Emitter) EmitTestContainerFromIR(ctx MainContext, schema *ir.Schema, auth *normalizer.AuthDef, values map[string]any) error {
	services := append([]normalizer.Service(nil), ctx.Services...)
	entities := append([]normalizer.Entity(nil), ctx.Entities...)
	if len(services) == 0 && schema != nil {
		tmp := e.AnalyzeContextFromIR(schema)
		services = tmp.Services
		entities = tmp.Entities
	}
	if len(services) == 0 {
		return nil
	}

	repoNames := collectTestContainerRepoInterfaces(services, entities)
	serviceNames := make([]string, 0, len(services))
	for _, svc := range services {
		serviceNames = append(serviceNames, svc.Name)
	}
	sort.Strings(repoNames)

	flags := collectTestContainerFlags(services, auth)

	var buf bytes.Buffer
	buf.WriteString("package bootstrap\n\n")
	buf.WriteString("import (\n")
	buf.WriteString("\t")
	buf.WriteString(strconvQuote(e.GoModule + "/internal/adapter/mock"))
	buf.WriteString("\n")
	buf.WriteString("\t")
	buf.WriteString(strconvQuote(e.GoModule + "/internal/config"))
	buf.WriteString("\n")
	buf.WriteString("\t")
	buf.WriteString(strconvQuote(e.GoModule + "/internal/port"))
	buf.WriteString("\n")
	buf.WriteString("\t")
	buf.WriteString(strconvQuote(e.GoModule + "/internal/service"))
	buf.WriteString("\n")
	if flags.needsRedis {
		buf.WriteString("\tredis ")
		buf.WriteString(strconvQuote("github.com/redis/go-redis/v9"))
		buf.WriteString("\n")
	}
	buf.WriteString(")\n\n")

	buf.WriteString("type TestOption func(*TestContainer)\n\n")
	buf.WriteString("type TestContainer struct {\n")
	buf.WriteString("\tConfig *config.Config\n")
	buf.WriteString("\tEffects *EffectRegistry\n")
	if flags.needsRedis {
		buf.WriteString("\tCache *redis.Client\n")
	}
	for _, repoName := range repoNames {
		buf.WriteString("\t")
		buf.WriteString(repoName)
		buf.WriteString(" *mock.Mock")
		buf.WriteString(repoName)
		buf.WriteString("\n")
		buf.WriteString("\t")
		buf.WriteString(lowerFirst(repoName))
		buf.WriteString("Impl port.")
		buf.WriteString(repoName)
		buf.WriteString("\n")
	}
	for _, dep := range flags.extraDeps {
		buf.WriteString("\t")
		buf.WriteString(dep.FieldName)
		buf.WriteString(" *mock.Mock")
		buf.WriteString(dep.InterfaceName)
		buf.WriteString("\n")
		buf.WriteString("\t")
		buf.WriteString(dep.implField)
		buf.WriteString(" port.")
		buf.WriteString(dep.InterfaceName)
		buf.WriteString("\n")
	}
	for _, svcName := range serviceNames {
		buf.WriteString("\tSvc")
		buf.WriteString(svcName)
		buf.WriteString(" port.")
		buf.WriteString(svcName)
		buf.WriteString("\n")
		buf.WriteString("\tsvc")
		buf.WriteString(svcName)
		buf.WriteString("Override port.")
		buf.WriteString(svcName)
		buf.WriteString("\n")
	}
	buf.WriteString("}\n\n")

	buf.WriteString("// NewTestContainer creates a mock-first bootstrap container for unit tests.\n")
	buf.WriteString("func NewTestContainer(opts ...TestOption) *TestContainer {\n")
	buf.WriteString("\tc := &TestContainer{Config: &config.Config{}}\n")
	for _, repoName := range repoNames {
		field := repoName
		impl := lowerFirst(repoName) + "Impl"
		buf.WriteString("\tc.")
		buf.WriteString(field)
		buf.WriteString(" = mock.New")
		buf.WriteString(repoName)
		buf.WriteString("()\n")
		buf.WriteString("\tc.")
		buf.WriteString(impl)
		buf.WriteString(" = c.")
		buf.WriteString(field)
		buf.WriteString("\n")
	}
	for _, dep := range flags.extraDeps {
		buf.WriteString("\tc.")
		buf.WriteString(dep.FieldName)
		buf.WriteString(" = mock.New")
		buf.WriteString(dep.InterfaceName)
		buf.WriteString("()\n")
		buf.WriteString("\tc.")
		buf.WriteString(dep.implField)
		buf.WriteString(" = c.")
		buf.WriteString(dep.FieldName)
		buf.WriteString("\n")
	}
	buf.WriteString("\tfor _, opt := range opts {\n")
	buf.WriteString("\t\tif opt != nil {\n")
	buf.WriteString("\t\t\topt(c)\n")
	buf.WriteString("\t\t}\n")
	buf.WriteString("\t}\n")
	buf.WriteString("\tc.Effects = NewTestEffectRegistry(")
	buf.WriteString(flags.lookupImpl("Publisher"))
	buf.WriteString(", ")
	buf.WriteString(flags.lookupImpl("FileStorage"))
	buf.WriteString(", ")
	buf.WriteString(flags.lookupImpl("StateStore"))
	buf.WriteString(")\n")
	for _, svc := range services {
		buf.WriteString("\tif c.svc")
		buf.WriteString(svc.Name)
		buf.WriteString("Override != nil {\n")
		buf.WriteString("\t\tc.Svc")
		buf.WriteString(svc.Name)
		buf.WriteString(" = c.svc")
		buf.WriteString(svc.Name)
		buf.WriteString("Override\n")
		buf.WriteString("\t} else {\n")
		buf.WriteString("\t\tc.Svc")
		buf.WriteString(svc.Name)
		buf.WriteString(" = service.New")
		buf.WriteString(svc.Name)
		buf.WriteString("Impl(\n")
		for _, repoEntity := range serviceImplRepoEntities(svc, entities) {
			repoName := ExportName(repoEntity) + "Repository"
			buf.WriteString("\t\t\tc.")
			buf.WriteString(lowerFirst(repoName))
			buf.WriteString("Impl,\n")
		}
		for _, depSvc := range serviceImplServiceDeps(svc) {
			buf.WriteString("\t\t\tc.Svc")
			buf.WriteString(ExportName(depSvc))
			buf.WriteString(",\n")
		}
		if serviceImplNeedsTx(svc) {
			buf.WriteString("\t\t\t")
			buf.WriteString(flags.lookupImpl("TxManager"))
			buf.WriteString(",\n")
		}
		if auth != nil && auth.Service == svc.Name {
			buf.WriteString("\t\t\tc.Config,\n")
			buf.WriteString("\t\t\t")
			buf.WriteString(flags.lookupImpl("RefreshTokenStore"))
			buf.WriteString(",\n")
		}
		if serviceImplHasPublishes(svc) {
			buf.WriteString("\t\t\tc.Effects.Publisher,\n")
		}
		if serviceImplHasIdempotency(svc) {
			buf.WriteString("\t\t\t")
			buf.WriteString(flags.lookupImpl("IdempotencyStore"))
			buf.WriteString(",\n")
		}
		if serviceImplHasOutbox(svc) {
			buf.WriteString("\t\t\t")
			buf.WriteString(flags.lookupImpl("OutboxRepository"))
			buf.WriteString(",\n")
		}
		if svc.RequiresS3 || serviceImplHasStorageActions(svc) {
			buf.WriteString("\t\t\tc.Effects.Storage,\n")
		}
		if serviceImplHasNotificationDispatch(svc) {
			buf.WriteString("\t\t\t")
			buf.WriteString(flags.lookupImpl("NotificationDispatcher"))
			buf.WriteString(",\n")
		}
		if serviceImplHasCacheActions(svc) {
			buf.WriteString("\t\t\tc.Cache,\n")
		}
		if serviceImplHasMailSend(svc) {
			buf.WriteString("\t\t\t")
			buf.WriteString(flags.lookupImpl("Mailer"))
			buf.WriteString(",\n")
		}
		if serviceImplHasQueueDeliveryActions(svc) {
			buf.WriteString("\t\t\t")
			buf.WriteString(flags.lookupImpl("QueuePublisher"))
			buf.WriteString(",\n")
		}
		if serviceImplHasStateActions(svc) {
			buf.WriteString("\t\t\tc.Effects.StateStore,\n")
		}
		if serviceImplHasPolicyActions(svc) {
			buf.WriteString("\t\t\t")
			buf.WriteString(flags.lookupImpl("PolicyEngine"))
			buf.WriteString(",\n")
		}
		buf.WriteString("\t\t)\n")
		buf.WriteString("\t}\n")
	}
	buf.WriteString("\treturn c\n")
	buf.WriteString("}\n\n")

	buf.WriteString("// NewTestContainerWith applies partial overrides on top of NewTestContainer.\n")
	buf.WriteString("func NewTestContainerWith(opts ...TestOption) *TestContainer {\n")
	buf.WriteString("\treturn NewTestContainer(opts...)\n")
	buf.WriteString("}\n\n")

	buf.WriteString("func WithConfig(cfg *config.Config) TestOption {\n")
	buf.WriteString("\treturn func(c *TestContainer) {\n")
	buf.WriteString("\t\tif cfg != nil {\n")
	buf.WriteString("\t\t\tc.Config = cfg\n")
	buf.WriteString("\t\t}\n")
	buf.WriteString("\t}\n")
	buf.WriteString("}\n\n")

	for _, repoName := range repoNames {
		buf.WriteString("func With")
		buf.WriteString(repoName)
		buf.WriteString("(v port.")
		buf.WriteString(repoName)
		buf.WriteString(") TestOption {\n")
		buf.WriteString("\treturn func(c *TestContainer) {\n")
		buf.WriteString("\t\tif v != nil {\n")
		buf.WriteString("\t\t\tc.")
		buf.WriteString(lowerFirst(repoName))
		buf.WriteString("Impl = v\n")
		buf.WriteString("\t\t}\n")
		buf.WriteString("\t}\n")
		buf.WriteString("}\n\n")
	}
	for _, dep := range flags.extraDeps {
		buf.WriteString("func With")
		buf.WriteString(dep.InterfaceName)
		buf.WriteString("(v port.")
		buf.WriteString(dep.InterfaceName)
		buf.WriteString(") TestOption {\n")
		buf.WriteString("\treturn func(c *TestContainer) {\n")
		buf.WriteString("\t\tif v != nil {\n")
		buf.WriteString("\t\t\tc.")
		buf.WriteString(dep.implField)
		buf.WriteString(" = v\n")
		if dep.InterfaceName == "FileStorage" || dep.InterfaceName == "StateStore" || dep.InterfaceName == "Publisher" {
			buf.WriteString("\t\t\tif c.Effects != nil {\n")
			switch dep.InterfaceName {
			case "FileStorage":
				buf.WriteString("\t\t\t\tc.Effects.Storage = v\n")
			case "StateStore":
				buf.WriteString("\t\t\t\tc.Effects.StateStore = v\n")
			case "Publisher":
				buf.WriteString("\t\t\t\tc.Effects.Publisher = v\n")
			}
			buf.WriteString("\t\t\t}\n")
		}
		buf.WriteString("\t\t}\n")
		buf.WriteString("\t}\n")
		buf.WriteString("}\n\n")
	}
	for _, svcName := range serviceNames {
		buf.WriteString("func With")
		buf.WriteString(svcName)
		buf.WriteString("Service(v port.")
		buf.WriteString(svcName)
		buf.WriteString(") TestOption {\n")
		buf.WriteString("\treturn func(c *TestContainer) {\n")
		buf.WriteString("\t\tif v != nil {\n")
		buf.WriteString("\t\t\tc.svc")
		buf.WriteString(svcName)
		buf.WriteString("Override = v\n")
		buf.WriteString("\t\t\tc.Svc")
		buf.WriteString(svcName)
		buf.WriteString(" = v\n")
		buf.WriteString("\t\t}\n")
		buf.WriteString("\t}\n")
		buf.WriteString("}\n\n")
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("format test container: %w", err)
	}
	path := filepath.Join(e.OutputDir, "internal", "bootstrap", "test_container.gen.go")
	return WriteFileIfChanged(path, formatted, 0644)
}

type testContainerDep struct {
	InterfaceName string
	FieldName     string
	implField     string
}

type testContainerFlags struct {
	needsRedis bool
	extraDeps  []testContainerDep
}

func (f testContainerFlags) lookupImpl(interfaceName string) string {
	for _, dep := range f.extraDeps {
		if dep.InterfaceName == interfaceName {
			return "c." + dep.implField
		}
	}
	return "nil"
}

func collectTestContainerRepoInterfaces(services []normalizer.Service, entities []normalizer.Entity) []string {
	seen := map[string]struct{}{}
	for _, svc := range services {
		for _, repoEntity := range serviceImplRepoEntities(svc, entities) {
			seen[ExportName(repoEntity)+"Repository"] = struct{}{}
		}
	}
	var out []string
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func collectTestContainerFlags(services []normalizer.Service, auth *normalizer.AuthDef) testContainerFlags {
	seen := map[string]struct{}{}
	order := []string{
		"TxManager",
		"RefreshTokenStore",
		"Publisher",
		"IdempotencyStore",
		"OutboxRepository",
		"FileStorage",
		"NotificationDispatcher",
		"Mailer",
		"QueuePublisher",
		"StateStore",
		"PolicyEngine",
	}
	flags := testContainerFlags{}
	for _, svc := range services {
		if serviceImplNeedsTx(svc) {
			seen["TxManager"] = struct{}{}
		}
		if auth != nil && auth.Service == svc.Name {
			seen["RefreshTokenStore"] = struct{}{}
		}
		if serviceImplHasPublishes(svc) {
			seen["Publisher"] = struct{}{}
		}
		if serviceImplHasIdempotency(svc) {
			seen["IdempotencyStore"] = struct{}{}
		}
		if serviceImplHasOutbox(svc) {
			seen["OutboxRepository"] = struct{}{}
		}
		if svc.RequiresS3 || serviceImplHasStorageActions(svc) {
			seen["FileStorage"] = struct{}{}
		}
		if serviceImplHasNotificationDispatch(svc) {
			seen["NotificationDispatcher"] = struct{}{}
		}
		if serviceImplHasCacheActions(svc) {
			flags.needsRedis = true
		}
		if serviceImplHasMailSend(svc) {
			seen["Mailer"] = struct{}{}
		}
		if serviceImplHasQueueDeliveryActions(svc) {
			seen["QueuePublisher"] = struct{}{}
		}
		if serviceImplHasStateActions(svc) {
			seen["StateStore"] = struct{}{}
		}
		if serviceImplHasPolicyActions(svc) {
			seen["PolicyEngine"] = struct{}{}
		}
	}
	for _, name := range order {
		if _, ok := seen[name]; !ok {
			continue
		}
		flags.extraDeps = append(flags.extraDeps, testContainerDep{
			InterfaceName: name,
			FieldName:     name,
			implField:     lowerFirst(name) + "Impl",
		})
	}
	return flags
}

func strconvQuote(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}
