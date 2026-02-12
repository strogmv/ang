package emitter

import (
	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

func (e *Emitter) EmitServiceFromIR(schema *ir.Schema) error {
	return e.EmitService(schema.Services)
}

func (e *Emitter) EmitHTTPFromIR(schema *ir.Schema, auth *normalizer.AuthDef) error {
	return e.EmitHTTP(
		schema.Endpoints,
		schema.Services,
		schema.Events,
		auth,
	)
}

func (e *Emitter) EmitRepositoryFromIR(schema *ir.Schema) error {
	return e.EmitRepository(schema.Repos, schema.Entities)
}

func (e *Emitter) EmitPostgresRepoFromIR(schema *ir.Schema) error {
	return e.EmitPostgresRepo(schema.Repos, schema.Entities)
}

func (e *Emitter) EmitMongoRepoFromIR(schema *ir.Schema) error {
	return e.EmitMongoRepo(schema.Repos, schema.Entities)
}

func (e *Emitter) EmitMongoCommonFromIR(schema *ir.Schema) error {
	return e.EmitMongoCommon(schema.Entities)
}

func (e *Emitter) EmitSQLFromIR(schema *ir.Schema) error {
	return e.EmitSQL(schema.Entities)
}

func (e *Emitter) EmitSQLQueriesFromIR(schema *ir.Schema) error {
	return e.EmitSQLQueries(schema.Entities)
}

func (e *Emitter) EmitMongoSchemaFromIR(schema *ir.Schema) error {
	return e.EmitMongoSchema(schema.Entities)
}

func (e *Emitter) EmitStubRepoFromIR(schema *ir.Schema) error {
	return e.EmitStubRepo(schema.Repos, schema.Entities)
}

func (e *Emitter) EmitEventsFromIR(schema *ir.Schema) error {
	return e.EmitEvents(schema.Events)
}

func (e *Emitter) EmitSchedulerFromIR(schema *ir.Schema) error {
	return e.EmitScheduler(schema.Schedules)
}

func (e *Emitter) EmitPublisherInterfaceFromIR(schema *ir.Schema) error {
	return e.EmitPublisherInterface(schema.Services, schema.Schedules)
}

func (e *Emitter) EmitNatsAdapterFromIR(schema *ir.Schema) error {
	return e.EmitNatsAdapter(schema.Services, schema.Schedules)
}

func (e *Emitter) EmitErrorsFromIR(schema *ir.Schema) error {
	return e.EmitErrors(schema.Errors)
}

func (e *Emitter) EmitContractTestsFromIR(schema *ir.Schema) error {
	return e.EmitContractTests(schema.Endpoints, schema.Services)
}

func (e *Emitter) EmitFrontendSDKFromIR(schema *ir.Schema, rbac *normalizer.RBACDef) error {
	return e.EmitFrontendSDK(
		schema.Entities,
		schema.Services,
		schema.Endpoints,
		schema.Events,
		schema.Errors,
		rbac,
	)
}

func (e *Emitter) EmitFrontendComponentsFromIR(schema *ir.Schema) error {
	return e.EmitFrontendComponents(
		schema.Services,
		schema.Endpoints,
		schema.Entities,
	)
}

func (e *Emitter) EmitFrontendAdminFromIR(schema *ir.Schema) error {
	return e.EmitFrontendAdmin(
		schema.Entities,
		schema.Services,
	)
}

func (e *Emitter) EmitServiceImplFromIR(schema *ir.Schema, auth *normalizer.AuthDef) error {
	return e.EmitServiceImpl(schema.Services, schema.Entities, auth)
}

func (e *Emitter) EmitCachedServiceFromIR(schema *ir.Schema) error {
	return e.EmitCachedService(schema.Services)
}

func (e *Emitter) EmitK8sFromIR(schema *ir.Schema, isMicroservice bool) error {
	return e.EmitK8s(schema.Services, isMicroservice)
}

func (e *Emitter) EmitMicroservicesFromIR(schema *ir.Schema, wsServices map[string]bool, auth *normalizer.AuthDef) error {
	return e.EmitMicroservices(schema.Services, wsServices, auth)
}

func (e *Emitter) EmitOpenAPIFromIR(schema *ir.Schema, project *normalizer.ProjectDef) error {
	return e.EmitOpenAPI(
		schema.Endpoints,
		schema.Services,
		schema.Errors,
		project,
	)
}

func (e *Emitter) EmitAsyncAPIFromIR(schema *ir.Schema, project *normalizer.ProjectDef) error {
	return e.EmitAsyncAPI(schema.Events, project)
}

func (e *Emitter) EmitViewsFromIR(schema *ir.Schema) error {
	return e.EmitViews(schema.Views)
}

func (e *Emitter) EmitNotificationDispatchPortsFromIR(schema *ir.Schema) error {
	if schema == nil {
		return nil
	}
	return e.EmitNotificationDispatchPorts(schema.Notifications)
}

func (e *Emitter) EmitNotificationDispatcherRuntimeFromIR(schema *ir.Schema) error {
	if schema == nil {
		return nil
	}
	return e.EmitNotificationDispatcherRuntime(schema.Notifications)
}
