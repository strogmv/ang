package goemitter

import (
	"fmt"

	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/emitter"
	"github.com/strogmv/ang/compiler/generator"
	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

type RegisterInput struct {
	Em             *emitter.Emitter
	IRSchema       *ir.Schema
	Ctx            emitter.MainContext
	Entities       []normalizer.Entity
	Services       []normalizer.Service
	Endpoints      []normalizer.Endpoint
	Repos          []normalizer.Repository
	Events         []normalizer.EventDef
	BizErrors      []normalizer.ErrorDef
	Schedules      []normalizer.ScheduleDef
	Scenarios      []normalizer.ScenarioDef
	Views          []normalizer.ViewDef
	CfgDef         *normalizer.ConfigDef
	AuthDef        *normalizer.AuthDef
	RBACDef        *normalizer.RBACDef
	IsMicroservice bool

	TestStubsEnabled        bool
	ResolveMissingTestStubs func() ([]normalizer.Endpoint, error)
	CopyFrontendSDK         func() error
	CopyFrontendAdmin       func() error
	WriteFrontendEnvExample func() error
}

func Register(registry *generator.StepRegistry, in RegisterInput) {
	goOnly := []compiler.Capability{compiler.CapabilityProfileGoLegacy}
	goHTTP := []compiler.Capability{compiler.CapabilityProfileGoLegacy, compiler.CapabilityHTTP}
	goSQL := []compiler.Capability{compiler.CapabilityProfileGoLegacy, compiler.CapabilitySQLRepo}
	goEvents := []compiler.Capability{compiler.CapabilityProfileGoLegacy, compiler.CapabilityEvents}

	registry.Register(generator.Step{Name: "Config", Requires: goOnly, Run: func() error { return in.Em.EmitConfig(in.CfgDef) }})
	registry.Register(generator.Step{Name: "Logger", Requires: goOnly, Run: func() error { return in.Em.EmitLogger() }})
	registry.Register(generator.Step{Name: "RBAC", Requires: goOnly, Run: func() error { return in.Em.EmitRBAC(in.RBACDef) }})
	registry.Register(generator.Step{Name: "Domain Entities", Requires: goOnly, Run: func() error { return in.Em.EmitDomain(in.IRSchema.Entities) }})
	registry.Register(generator.Step{Name: "DTOs", Requires: goOnly, Run: func() error { return in.Em.EmitDTO(in.IRSchema.Entities) }})
	registry.Register(generator.Step{Name: "Service Ports", Requires: goOnly, Run: func() error { return in.Em.EmitService(in.Services) }})
	registry.Register(generator.Step{Name: "HTTP Handlers", Requires: goHTTP, Run: func() error { return in.Em.EmitHTTP(in.Endpoints, in.Services, in.Events, in.AuthDef) }})
	registry.Register(generator.Step{Name: "Health Probes", Requires: goHTTP, Run: func() error { return in.Em.EmitHealth() }})
	registry.Register(generator.Step{Name: "Repository Ports", Requires: goOnly, Run: func() error { return in.Em.EmitRepository(in.Repos, in.Entities) }})
	registry.Register(generator.Step{Name: "Transaction Port", Requires: goOnly, Run: func() error { return in.Em.EmitTransactionPort() }})
	registry.Register(generator.Step{Name: "Storage Port", Requires: goOnly, Run: func() error { return in.Em.EmitStoragePort() }})
	registry.Register(generator.Step{Name: "S3 Client", Requires: goOnly, Run: func() error { return in.Em.EmitS3Client() }})
	registry.Register(generator.Step{Name: "Postgres Repos", Requires: goSQL, Run: func() error { return in.Em.EmitPostgresRepo(in.Repos, in.Entities) }})
	registry.Register(generator.Step{Name: "Postgres Common", Requires: goSQL, Run: func() error { return in.Em.EmitPostgresCommon() }})
	registry.Register(generator.Step{Name: "Mongo Repos", Requires: goOnly, Run: func() error { return in.Em.EmitMongoRepo(in.Repos, in.Entities) }})
	registry.Register(generator.Step{Name: "Mongo Common", Requires: goOnly, Run: func() error { return in.Em.EmitMongoCommon(in.Entities) }})
	registry.Register(generator.Step{Name: "SQL Schema", Requires: goSQL, Run: func() error { return in.Em.EmitSQL(in.Entities) }})
	registry.Register(generator.Step{Name: "Infra Configs", Requires: goOnly, Run: func() error { return in.Em.EmitInfraConfigs() }})
	registry.Register(generator.Step{Name: "SQL Queries", Requires: goSQL, Run: func() error { return in.Em.EmitSQLQueries(in.Entities) }})
	registry.Register(generator.Step{Name: "Mongo Schemas", Requires: goOnly, Run: func() error { return in.Em.EmitMongoSchema(in.Entities) }})
	registry.Register(generator.Step{Name: "Repo Stubs", Requires: goOnly, Run: func() error { return in.Em.EmitStubRepo(in.Repos, in.Entities) }})
	registry.Register(generator.Step{Name: "Redis Client", Requires: goOnly, Run: func() error { return in.Em.EmitRedisClient() }})
	registry.Register(generator.Step{Name: "Auth Package", Requires: []compiler.Capability{compiler.CapabilityProfileGoLegacy, compiler.CapabilityAuth}, Run: func() error { return in.Em.EmitAuthPackage(in.AuthDef) }})
	registry.Register(generator.Step{Name: "Refresh Store Port", Requires: []compiler.Capability{compiler.CapabilityProfileGoLegacy, compiler.CapabilityAuth}, Run: func() error { return in.Em.EmitRefreshTokenStorePort() }})
	registry.Register(generator.Step{Name: "Refresh Store Memory", Requires: []compiler.Capability{compiler.CapabilityProfileGoLegacy, compiler.CapabilityAuth}, Run: func() error { return in.Em.EmitRefreshTokenStoreMemory() }})
	registry.Register(generator.Step{Name: "Refresh Store Redis", Requires: []compiler.Capability{compiler.CapabilityProfileGoLegacy, compiler.CapabilityAuth}, Run: func() error { return in.Em.EmitRefreshTokenStoreRedis() }})
	registry.Register(generator.Step{Name: "Refresh Store Postgres", Requires: []compiler.Capability{compiler.CapabilityProfileGoLegacy, compiler.CapabilityAuth, compiler.CapabilitySQLRepo}, Run: func() error { return in.Em.EmitRefreshTokenStorePostgres() }})
	registry.Register(generator.Step{Name: "Refresh Store Hybrid", Requires: []compiler.Capability{compiler.CapabilityProfileGoLegacy, compiler.CapabilityAuth, compiler.CapabilitySQLRepo}, Run: func() error { return in.Em.EmitRefreshTokenStoreHybrid() }})
	registry.Register(generator.Step{Name: "Mailer Port", Requires: goOnly, Run: func() error { return in.Em.EmitMailerPort() }})
	registry.Register(generator.Step{Name: "SMTP Client", Requires: goOnly, Run: func() error { return in.Em.EmitMailerAdapter() }})
	registry.Register(generator.Step{Name: "Events", Requires: goEvents, Run: func() error { return in.Em.EmitEvents(in.Events) }})
	registry.Register(generator.Step{Name: "Scheduler", Requires: goOnly, Run: func() error { return in.Em.EmitScheduler(in.Schedules) }})
	registry.Register(generator.Step{Name: "Publisher Interface", Requires: goEvents, Run: func() error { return in.Em.EmitPublisherInterface(in.Services, in.Schedules) }})
	registry.Register(generator.Step{Name: "NATS Adapter", Requires: goEvents, Run: func() error { return in.Em.EmitNatsAdapter(in.Services, in.Schedules) }})
	registry.Register(generator.Step{Name: "Metrics Middleware", Requires: goHTTP, Run: func() error { return in.Em.EmitMetrics() }})
	registry.Register(generator.Step{Name: "Logging Middleware", Requires: goHTTP, Run: func() error { return in.Em.EmitLoggingMiddleware() }})
	registry.Register(generator.Step{Name: "Errors", Requires: goOnly, Run: func() error { return in.Em.EmitErrors(in.BizErrors) }})
	registry.Register(generator.Step{Name: "Views", Requires: goOnly, Run: func() error { return in.Em.EmitViews(in.Views) }})
	registry.Register(generator.Step{Name: "Contract Tests", Requires: goHTTP, Run: func() error { return in.Em.EmitContractTests(in.Endpoints, in.Services) }})
	registry.Register(generator.Step{Name: "E2E Behavioral Tests", Requires: goHTTP, Run: func() error { return in.Em.EmitE2ETests(in.Scenarios) }})
	registry.Register(generator.Step{Name: "Test Stubs", Requires: goHTTP, Run: func() error {
		if !in.TestStubsEnabled {
			return nil
		}
		if in.ResolveMissingTestStubs == nil {
			return fmt.Errorf("ResolveMissingTestStubs callback is nil")
		}
		missing, err := in.ResolveMissingTestStubs()
		if err != nil {
			return err
		}
		if len(missing) == 0 {
			fmt.Println("No missing tests found. Skipping stub generation.")
			return nil
		}
		return in.Em.EmitTestStubs(missing, "NEW-endpoint-stubs.test.ts")
	}})
	registry.Register(generator.Step{Name: "Frontend SDK", Requires: goOnly, Run: func() error {
		return in.Em.EmitFrontendSDK(in.Entities, in.Services, in.Endpoints, in.Events, in.BizErrors, in.RBACDef)
	}})
	registry.Register(generator.Step{Name: "Frontend Components", Requires: goOnly, Run: func() error { return in.Em.EmitFrontendComponents(in.Services, in.Endpoints, in.Entities) }})
	registry.Register(generator.Step{Name: "Frontend Admin", Requires: goOnly, Run: func() error { return in.Em.EmitFrontendAdmin(in.Entities, in.Services) }})
	registry.Register(generator.Step{Name: "Frontend SDK Copy", Requires: goOnly, Run: func() error {
		if in.CopyFrontendSDK == nil {
			return nil
		}
		return in.CopyFrontendSDK()
	}})
	registry.Register(generator.Step{Name: "Frontend Admin Copy", Requires: goOnly, Run: func() error {
		if in.CopyFrontendAdmin == nil {
			return nil
		}
		return in.CopyFrontendAdmin()
	}})
	registry.Register(generator.Step{Name: "Frontend Env Example", Requires: goOnly, Run: func() error {
		if in.WriteFrontendEnvExample == nil {
			return nil
		}
		return in.WriteFrontendEnvExample()
	}})
	registry.Register(generator.Step{Name: "Tracing", Requires: goOnly, Run: func() error { return in.Em.EmitTracing() }})
	registry.Register(generator.Step{Name: "Service Impls", Requires: goOnly, Run: func() error { return in.Em.EmitServiceImpl(in.Services, in.Entities, in.AuthDef) }})
	registry.Register(generator.Step{Name: "Cached Services", Requires: goOnly, Run: func() error { return in.Em.EmitCachedService(in.Services) }})
	registry.Register(generator.Step{Name: "K8s Manifests", Requires: goOnly, Run: func() error { return in.Em.EmitK8s(in.Services, in.IsMicroservice) }})
	registry.Register(generator.Step{Name: "Server Main", Requires: goOnly, Run: func() error {
		if in.IsMicroservice {
			return in.Em.EmitMicroservices(in.Services, in.Ctx.WebSocketServices, in.AuthDef)
		}
		return in.Em.EmitMain(in.Ctx)
	}})
}
