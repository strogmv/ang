package emitter

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

type MainServerHeaderContext struct {
	ServicesIR   []ir.Service
	ANGVersion   string
	InputHash    string
	CompilerHash string
}

type MainServerImportsContext struct {
	HasNats                 bool
	HasSQL                  bool
	HasMongo                bool
	HasCache                bool
	AuthRefreshStore        string
	GoModule                string
	HasS3                   bool
	HasScheduler            bool
	AuthService             string
	NotificationMuting      bool
	HasNotificationsService bool
	HasNotificationDispatch bool
	ServicesIR              []ir.Service
	EntitiesIR              []ir.Entity
}

type MainServerInfrastructureContext struct {
	HasSQL                  bool
	HasMongo                bool
	HasCache                bool
	AuthRefreshStore        string
	HasNats                 bool
	HasS3                   bool
	HasScheduler            bool
	HasNotificationsService bool
	HasNotificationDispatch bool
}

type MainServerRepositoriesContext struct {
	AuthService             string
	AuthRefreshStore        string
	NotificationMuting      bool
	HasNotificationsService bool
	HasNotificationDispatch bool
	HasSQL                  bool
	ServicesIR              []ir.Service
	EntitiesIR              []ir.Entity
}

type MainServerServicesContext struct {
	ServicesIR              []ir.Service
	EntitiesIR              []ir.Entity
	AuthService             string
	AuthRefreshStore        string
	HasSQL                  bool
	NotificationMuting      bool
	HasNotificationsService bool
	HasNotificationDispatch bool
	WebSocketServices       map[string]bool
}

type MainServerHTTPRouterContext struct{}

type MainServerWebSocketsContext struct {
	HasNats       bool
	WSEventMap    map[string]map[string]bool
	EndpointsIR   []ir.Endpoint
	ServicesIR    []ir.Service
	EventPayloads map[string]ir.Entity
}

type MainServerGracefulShutdownContext struct {
	WebSocketServices map[string]bool
}

type MainServerTemplateData struct {
	Header           MainServerHeaderContext
	Imports          MainServerImportsContext
	Infrastructure   MainServerInfrastructureContext
	Repositories     MainServerRepositoriesContext
	Services         MainServerServicesContext
	HTTPRouter       MainServerHTTPRouterContext
	WebSockets       MainServerWebSocketsContext
	GracefulShutdown MainServerGracefulShutdownContext
}

func buildMainServerTemplateData(ctx MainContext) MainServerTemplateData {
	return MainServerTemplateData{
		Header: MainServerHeaderContext{
			ServicesIR:   ctx.ServicesIR,
			ANGVersion:   ctx.ANGVersion,
			InputHash:    ctx.InputHash,
			CompilerHash: ctx.CompilerHash,
		},
		Imports: MainServerImportsContext{
			HasNats:                 ctx.HasNats,
			HasSQL:                  ctx.HasSQL,
			HasMongo:                ctx.HasMongo,
			HasCache:                ctx.HasCache,
			AuthRefreshStore:        ctx.AuthRefreshStore,
			GoModule:                ctx.GoModule,
			HasS3:                   ctx.HasS3,
			HasScheduler:            ctx.HasScheduler,
			AuthService:             ctx.AuthService,
			NotificationMuting:      ctx.NotificationMuting,
			HasNotificationsService: ctx.HasNotificationsService,
			HasNotificationDispatch: ctx.HasNotificationDispatch,
			ServicesIR:              ctx.ServicesIR,
			EntitiesIR:              ctx.EntitiesIR,
		},
		Infrastructure: MainServerInfrastructureContext{
			HasSQL:                  ctx.HasSQL,
			HasMongo:                ctx.HasMongo,
			HasCache:                ctx.HasCache,
			AuthRefreshStore:        ctx.AuthRefreshStore,
			HasNats:                 ctx.HasNats,
			HasS3:                   ctx.HasS3,
			HasScheduler:            ctx.HasScheduler,
			HasNotificationsService: ctx.HasNotificationsService,
			HasNotificationDispatch: ctx.HasNotificationDispatch,
		},
		Repositories: MainServerRepositoriesContext{
			AuthService:             ctx.AuthService,
			AuthRefreshStore:        ctx.AuthRefreshStore,
			NotificationMuting:      ctx.NotificationMuting,
			HasNotificationsService: ctx.HasNotificationsService,
			HasNotificationDispatch: ctx.HasNotificationDispatch,
			HasSQL:                  ctx.HasSQL,
			ServicesIR:              ctx.ServicesIR,
			EntitiesIR:              ctx.EntitiesIR,
		},
		Services: MainServerServicesContext{
			ServicesIR:              ctx.ServicesIR,
			EntitiesIR:              ctx.EntitiesIR,
			AuthService:             ctx.AuthService,
			AuthRefreshStore:        ctx.AuthRefreshStore,
			HasSQL:                  ctx.HasSQL,
			NotificationMuting:      ctx.NotificationMuting,
			HasNotificationsService: ctx.HasNotificationsService,
			HasNotificationDispatch: ctx.HasNotificationDispatch,
			WebSocketServices:       ctx.WebSocketServices,
		},
		HTTPRouter: MainServerHTTPRouterContext{},
		WebSockets: MainServerWebSocketsContext{
			HasNats:       ctx.HasNats,
			WSEventMap:    ctx.WSEventMap,
			EndpointsIR:   ctx.EndpointsIR,
			ServicesIR:    ctx.ServicesIR,
			EventPayloads: ctx.EventPayloadsIR,
		},
		GracefulShutdown: MainServerGracefulShutdownContext{
			WebSocketServices: ctx.WebSocketServices,
		},
	}
}

func (e *Emitter) parseMainServerTemplate() (*template.Template, error) {
	paths := []string{
		"templates/main_server/root.tmpl",
		"templates/main_server/imports.tmpl",
		"templates/main_server/infrastructure.tmpl",
		"templates/main_server/repositories.tmpl",
		"templates/main_server/services.tmpl",
		"templates/main_server/http_router.tmpl",
		"templates/main_server/websockets.tmpl",
		"templates/main_server/graceful_shutdown.tmpl",
	}
	t := template.New("main_server").Funcs(e.getAppFuncMap())
	for _, p := range paths {
		content, err := ReadTemplateByPath(p)
		if err != nil {
			return nil, fmt.Errorf("read template %s: %w", p, err)
		}
		if _, err := t.Parse(string(content)); err != nil {
			return nil, fmt.Errorf("parse template %s: %w", p, err)
		}
	}
	return t, nil
}

// EmitMicroservices generates separate binaries for each service.
func (e *Emitter) EmitMicroservices(services []ir.Service, wsServices map[string]bool, auth *normalizer.AuthDef) error {
	for _, svc := range services {
		svcNorm := IRServiceToNormalizer(svc)
		ctx := MainContext{
			ServicesIR: []ir.Service{svc},
			Services:   []normalizer.Service{svcNorm},
			HasCache:   svc.RequiresRedis,
			HasSQL:     svc.RequiresSQL,
			HasMongo:   svc.RequiresMongo,
			HasNats:    svc.RequiresNats,
			WebSocketServices: map[string]bool{
				svc.Name: wsServices[svc.Name],
			},
			WSEventMap: map[string]map[string]bool{
				svc.Name: {},
			},
			EventPayloads:    make(map[string]normalizer.Entity),
			WSRoomField:      make(map[string]string),
			AuthService:      "",
			AuthRefreshStore: "",
			ANGVersion:       e.Version,
			InputHash:        e.InputHash,
			CompilerHash:     e.CompilerHash,
		}
		if auth != nil && auth.Service == svc.Name {
			ctx.AuthService = auth.Service
			ctx.AuthRefreshStore = auth.RefreshStore
		}

		if err := e.EmitServiceMain(svc.Name, ctx); err != nil {
			return err
		}
	}
	return nil
}

func (e *Emitter) getAppFuncMap() template.FuncMap {
	appFuncs := e.getSharedFuncMap()

	// Add app-specific functions
	appFuncs["HasRepoEntitiesIR"] = func(services []ir.Service, entities []ir.Entity) bool {
		dtoEntities := make(map[string]bool, len(entities))
		mongoEntities := make(map[string]bool, len(entities))
		for _, ent := range entities {
			if dto, ok := ent.Metadata["dto"].(bool); ok && dto {
				dtoEntities[ent.Name] = true
			}
			if storage, ok := ent.Metadata["storage"].(string); ok && strings.EqualFold(storage, "mongo") {
				mongoEntities[ent.Name] = true
			}
		}
		for _, svc := range services {
			unique := make(map[string]bool)
			var count int
			for _, m := range svc.Methods {
				for _, src := range m.Sources {
					if src.Entity != "" && !unique[src.Entity] && !dtoEntities[src.Entity] && !mongoEntities[src.Entity] {
						unique[src.Entity] = true
						count++
					}
				}
			}
			if count > 0 {
				return true
			}
		}
		return false
	}
	appFuncs["EntityStorageByNameIR"] = func(entities []ir.Entity, name string) string {
		for _, ent := range entities {
			if ent.Name != name {
				continue
			}
			if ent.Metadata != nil {
				if v, ok := ent.Metadata["storage"].(string); ok && v != "" {
					return v
				}
			}
			return "sql"
		}
		return "sql"
	}
	appFuncs["HasMongoRepoEntitiesIR"] = func(entities []ir.Entity) bool {
		for _, ent := range entities {
			if v, ok := ent.Metadata["storage"].(string); ok && strings.EqualFold(v, "mongo") {
				return true
			}
		}
		return false
	}
	appFuncs["AllRepoEntitiesIR"] = func(entities []ir.Entity) []string {
		var res []string
		for _, ent := range entities {
			if isDTO, ok := ent.Metadata["dto"].(bool); ok && isDTO {
				continue
			}
			res = append(res, ent.Name)
		}
		sort.Strings(res)
		return res
	}
	appFuncs["HasEntityByNameIR"] = func(entities []ir.Entity, name string) bool {
		for _, ent := range entities {
			if strings.EqualFold(ent.Name, name) {
				return true
			}
		}
		return false
	}
	appFuncs["HasTxServicesIR"] = func(services []ir.Service) bool {
		var hasTx func([]ir.FlowStep) bool
		hasTx = func(steps []ir.FlowStep) bool {
			for _, step := range steps {
				if step.Action == "tx.Block" {
					return true
				}
				if hasTx(step.Steps) || hasTx(step.IfNew) || hasTx(step.IfExists) || hasTx(step.Then) || hasTx(step.Else) || hasTx(step.Default) {
					return true
				}
				for _, branch := range step.Cases {
					if hasTx(branch) {
						return true
					}
				}
			}
			return false
		}
		for _, svc := range services {
			for _, m := range svc.Methods {
				if (m.Impl != nil && m.Impl.RequiresTx) || hasTx(m.Flow) {
					return true
				}
			}
		}
		return false
	}
	appFuncs["AnyServiceHasIdempotencyOrOutboxIR"] = func(services []ir.Service) bool {
		for _, svc := range services {
			for _, m := range svc.Methods {
				if m.Idempotent || m.Outbox {
					return true
				}
			}
		}
		return false
	}
	appFuncs["getServiceDepsIR"] = func(s ir.Service) []string {
		if len(s.Uses) == 0 {
			return nil
		}
		deps := append([]string{}, s.Uses...)
		sort.Strings(deps)
		return deps
	}
	appFuncs["ServiceNeedsTxIR"] = func(s ir.Service) bool {
		var hasTx func([]ir.FlowStep) bool
		hasTx = func(steps []ir.FlowStep) bool {
			for _, step := range steps {
				if step.Action == "tx.Block" {
					return true
				}
				if hasTx(step.Steps) || hasTx(step.IfNew) || hasTx(step.IfExists) || hasTx(step.Then) || hasTx(step.Else) || hasTx(step.Default) {
					return true
				}
				for _, branch := range step.Cases {
					if hasTx(branch) {
						return true
					}
				}
			}
			return false
		}
		for _, m := range s.Methods {
			if (m.Impl != nil && m.Impl.RequiresTx) || hasTx(m.Flow) {
				return true
			}
		}
		return false
	}
	appFuncs["ServiceHasPublishesIR"] = func(s ir.Service) bool {
		var hasPublish func([]ir.FlowStep) bool
		hasPublish = func(steps []ir.FlowStep) bool {
			for _, step := range steps {
				if step.Action == "event.Publish" {
					return true
				}
				if hasPublish(step.Steps) || hasPublish(step.IfNew) || hasPublish(step.IfExists) || hasPublish(step.Then) || hasPublish(step.Else) || hasPublish(step.Default) {
					return true
				}
				for _, branch := range step.Cases {
					if hasPublish(branch) {
						return true
					}
				}
			}
			return false
		}
		for _, m := range s.Methods {
			if len(m.Publishes) > 0 || hasPublish(m.Flow) {
				return true
			}
		}
		return false
	}
	appFuncs["ServiceHasNotificationDispatchIR"] = func(s ir.Service) bool {
		var hasDispatch func([]ir.FlowStep) bool
		hasDispatch = func(steps []ir.FlowStep) bool {
			for _, step := range steps {
				if step.Action == "notification.Dispatch" {
					return true
				}
				if hasDispatch(step.Steps) || hasDispatch(step.IfNew) || hasDispatch(step.IfExists) || hasDispatch(step.Then) || hasDispatch(step.Else) || hasDispatch(step.Default) {
					return true
				}
				for _, branch := range step.Cases {
					if hasDispatch(branch) {
						return true
					}
				}
			}
			return false
		}
		for _, m := range s.Methods {
			if hasDispatch(m.Flow) {
				return true
			}
		}
		return false
	}
	appFuncs["AnyServiceHasNotificationDispatchIR"] = func(services []ir.Service) bool {
		var hasDispatch func([]ir.FlowStep) bool
		hasDispatch = func(steps []ir.FlowStep) bool {
			for _, step := range steps {
				if step.Action == "notification.Dispatch" {
					return true
				}
				if hasDispatch(step.Steps) || hasDispatch(step.IfNew) || hasDispatch(step.IfExists) || hasDispatch(step.Then) || hasDispatch(step.Else) || hasDispatch(step.Default) {
					return true
				}
				for _, branch := range step.Cases {
					if hasDispatch(branch) {
						return true
					}
				}
			}
			return false
		}
		for _, svc := range services {
			for _, m := range svc.Methods {
				if hasDispatch(m.Flow) {
					return true
				}
			}
		}
		return false
	}
	appFuncs["ServiceHasIdempotencyIR"] = func(s ir.Service) bool {
		for _, m := range s.Methods {
			if m.Idempotent {
				return true
			}
		}
		return false
	}
	appFuncs["ServiceHasOutboxIR"] = func(s ir.Service) bool {
		for _, m := range s.Methods {
			if m.Outbox {
				return true
			}
		}
		return false
	}
	appFuncs["getRepoEntitiesIR"] = func(s ir.Service, entities []ir.Entity) []string {
		dtoEntities := make(map[string]bool, len(entities))
		for _, ent := range entities {
			if dto, ok := ent.Metadata["dto"].(bool); ok && dto {
				dtoEntities[ent.Name] = true
			}
		}
		seen := make(map[string]bool)
		var out []string

		var scanSteps func([]ir.FlowStep)
		scanSteps = func(steps []ir.FlowStep) {
			for _, step := range steps {
				if strings.HasPrefix(step.Action, "repo.") {
					if src, ok := step.Args["source"].(string); ok && src != "" && !seen[src] && !dtoEntities[src] {
						seen[src] = true
						out = append(out, src)
					}
				}
				if step.Action == "list.Enrich" {
					if src, ok := step.Args["lookupSource"].(string); ok && src != "" && !seen[src] && !dtoEntities[src] {
						seen[src] = true
						out = append(out, src)
					}
				}
				if step.Action == "entity.PatchValidated" {
					hasUnique := false
					if fields, ok := step.Args["fields"].(map[string]map[string]string); ok {
						for _, cfg := range fields {
							if strings.TrimSpace(cfg["unique"]) != "" {
								hasUnique = true
								break
							}
						}
					}
					if hasUnique {
						repoEntity := ""
						if src, ok := step.Args["source"].(string); ok {
							repoEntity = strings.TrimSpace(src)
						}
						if repoEntity != "" && !seen[repoEntity] && !dtoEntities[repoEntity] {
							seen[repoEntity] = true
							out = append(out, repoEntity)
						}
					}
				}
				scanSteps(step.Steps)
				scanSteps(step.IfNew)
				scanSteps(step.IfExists)
				scanSteps(step.Then)
				scanSteps(step.Else)
				scanSteps(step.Default)
				for _, branch := range step.Cases {
					scanSteps(branch)
				}
			}
		}

		for _, m := range s.Methods {
			for _, src := range m.Sources {
				if src.Entity == "" || seen[src.Entity] || dtoEntities[src.Entity] {
					continue
				}
				seen[src.Entity] = true
				out = append(out, src.Entity)
			}
			scanSteps(m.Flow)
		}

		sort.Strings(out)
		return out
	}
	appFuncs["HasEventFieldIR"] = func(evtPayloads map[string]ir.Entity, evtName, fieldName string) bool {
		if fieldName == "" {
			return false
		}
		if entity, ok := evtPayloads[evtName]; ok {
			for _, f := range entity.Fields {
				if strings.EqualFold(f.Name, fieldName) {
					return true
				}
			}
		}
		return false
	}
	appFuncs["RoomFieldForEventIR"] = func(endpoints []ir.Endpoint, services []ir.Service, serviceName, eventName string) string {
		var methods map[string]ir.Method
		for _, svc := range services {
			if svc.Name != serviceName {
				continue
			}
			methods = make(map[string]ir.Method)
			for _, m := range svc.Methods {
				methods[m.Name] = m
			}
			break
		}
		firstPathParam := func(path string) string {
			start := strings.Index(path, "{")
			if start == -1 {
				return ""
			}
			end := strings.Index(path[start:], "}")
			if end == -1 {
				return ""
			}
			return path[start+1 : start+end]
		}
		for _, ep := range endpoints {
			if strings.ToUpper(ep.Method) != "WS" || ep.Service != serviceName {
				continue
			}
			found := false
			for _, msg := range ep.Messages {
				if msg == eventName {
					found = true
					break
				}
			}
			if !found {
				continue
			}
			roomParam := ep.RoomParam
			if roomParam == "" {
				roomParam = firstPathParam(ep.Path)
			}
			if roomParam != "" {
				return ExportName(roomParam)
			}
			if m, ok := methods[ep.RPC]; ok && m.Input != nil {
				for _, f := range m.Input.Fields {
					if strings.EqualFold(f.Name, "userId") {
						return "UserID"
					}
				}
			}
		}
		return ""
	}

	appFuncs["HasService"] = func(services []normalizer.Service, name string) bool {
		for _, svc := range services {
			if svc.Name == name {
				return true
			}
		}
		return false
	}
	appFuncs["HasTxServices"] = func(services []normalizer.Service) bool {
		for _, svc := range services {
			var hasTx func([]normalizer.FlowStep) bool
			hasTx = func(steps []normalizer.FlowStep) bool {
				for _, step := range steps {
					if step.Action == "tx.Block" {
						return true
					}
					if v, ok := step.Args["_do"].([]normalizer.FlowStep); ok && hasTx(v) {
						return true
					}
					if v, ok := step.Args["_then"].([]normalizer.FlowStep); ok && hasTx(v) {
						return true
					}
					if v, ok := step.Args["_else"].([]normalizer.FlowStep); ok && hasTx(v) {
						return true
					}
					if v, ok := step.Args["_default"].([]normalizer.FlowStep); ok && hasTx(v) {
						return true
					}
					if cases, ok := step.Args["_cases"].(map[string][]normalizer.FlowStep); ok {
						for _, branch := range cases {
							if hasTx(branch) {
								return true
							}
						}
					}
				}
				return false
			}
			for _, m := range svc.Methods {
				if (m.Impl != nil && m.Impl.RequiresTx) || hasTx(m.Flow) {
					return true
				}
			}
		}
		return false
	}
	appFuncs["HasMethod"] = func(s normalizer.Service, name string) bool {
		for _, m := range s.Methods {
			if m.Name == name {
				return true
			}
		}
		return false
	}
	appFuncs["HasRepoEntities"] = func(services []normalizer.Service, entities []normalizer.Entity) bool {
		dtoEntities := make(map[string]bool, len(entities))
		mongoEntities := make(map[string]bool, len(entities))
		for _, ent := range entities {
			if dto, ok := ent.Metadata["dto"].(bool); ok && dto {
				dtoEntities[ent.Name] = true
			}
			if storage, ok := ent.Metadata["storage"].(string); ok && strings.EqualFold(storage, "mongo") {
				mongoEntities[ent.Name] = true
			}
		}
		for _, svc := range services {
			unique := make(map[string]bool)
			var count int
			for _, m := range svc.Methods {
				for _, src := range m.Sources {
					if src.Entity != "" && !unique[src.Entity] && !dtoEntities[src.Entity] && !mongoEntities[src.Entity] {
						unique[src.Entity] = true
						count++
					}
				}
			}
			if count > 0 {
				return true
			}
		}
		return false
	}
	appFuncs["HasEntity"] = func(services []normalizer.Service, entity string) bool {
		for _, svc := range services {
			for _, m := range svc.Methods {
				for _, src := range m.Sources {
					if src.Entity == entity {
						return true
					}
				}
			}
		}
		return false
	}
	appFuncs["HasNonUserEntities"] = func(services []normalizer.Service) bool {
		for _, svc := range services {
			for _, m := range svc.Methods {
				for _, src := range m.Sources {
					if src.Entity != "" && src.Entity != "User" {
						return true
					}
				}
			}
		}
		return false
	}
	appFuncs["AllRepoEntities"] = func(entities []normalizer.Entity) []string {
		var res []string
		for _, ent := range entities {
			// Skip DTO-only entities
			if isDTO, ok := ent.Metadata["dto"].(bool); ok && isDTO {
				continue
			}
			res = append(res, ent.Name)
		}
		sort.Strings(res)
		return res
	}
	appFuncs["UniqueRepoEntities"] = func(services []normalizer.Service) []string {
		seen := make(map[string]bool)
		var res []string
		for _, svc := range services {
			for _, m := range svc.Methods {
				for _, src := range m.Sources {
					if src.Entity != "" && !seen[src.Entity] {
						seen[src.Entity] = true
						res = append(res, src.Entity)
					}
				}
			}
		}
		sort.Strings(res)
		return res
	}
	appFuncs["ToSnake"] = ToSnakeCase
	appFuncs["ToTitle"] = ToTitle
	appFuncs["HasEventField"] = func(evtPayloads map[string]normalizer.Entity, evtName, fieldName string) bool {
		if fieldName == "" {
			return false
		}
		if entity, ok := evtPayloads[evtName]; ok {
			for _, f := range entity.Fields {
				if strings.EqualFold(f.Name, fieldName) {
					return true
				}
			}
		}
		return false
	}
	appFuncs["RoomFieldForEvent"] = func(endpoints []normalizer.Endpoint, services []normalizer.Service, serviceName, eventName string) string {
		var methods map[string]normalizer.Method
		for _, svc := range services {
			if svc.Name != serviceName {
				continue
			}
			methods = make(map[string]normalizer.Method)
			for _, m := range svc.Methods {
				methods[m.Name] = m
			}
			break
		}
		firstPathParam := func(path string) string {
			start := strings.Index(path, "{")
			if start == -1 {
				return ""
			}
			end := strings.Index(path[start:], "}")
			if end == -1 {
				return ""
			}
			return path[start+1 : start+end]
		}
		for _, ep := range endpoints {
			if strings.ToUpper(ep.Method) != "WS" || ep.ServiceName != serviceName {
				continue
			}
			found := false
			for _, msg := range ep.Messages {
				if msg == eventName {
					found = true
					break
				}
			}
			if !found {
				continue
			}
			roomParam := ep.RoomParam
			if roomParam == "" {
				roomParam = firstPathParam(ep.Path)
			}
			if roomParam != "" {
				return ExportName(roomParam)
			}
			if m, ok := methods[ep.RPC]; ok {
				for _, f := range m.Input.Fields {
					if strings.EqualFold(f.Name, "userId") {
						return "UserID"
					}
				}
			}
		}
		return ""
	}

	return appFuncs
}

// EmitServiceMain generates main.go for a specific service.
func (e *Emitter) EmitServiceMain(svcName string, ctx MainContext) error {
	t, err := e.parseMainServerTemplate()
	if err != nil {
		return err
	}

	targetDir := filepath.Join(e.OutputDir, "cmd", "services", strings.ToLower(svcName))
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "main_server_root", buildMainServerTemplateData(ctx)); err != nil {
		return err
	}
	formatted, err := formatGoStrict(buf.Bytes(), "cmd/services/"+strings.ToLower(svcName)+"/main.go")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(targetDir, "main.go"), formatted, 0644)
}

// EmitMain generates cmd/server/main.go (monolith).
func (e *Emitter) EmitMain(ctx MainContext) error {
	t, err := e.parseMainServerTemplate()
	if err != nil {
		return err
	}

	targetDir := filepath.Join(e.OutputDir, "cmd", "server")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "main_server_root", buildMainServerTemplateData(ctx)); err != nil {
		return err
	}
	formatted, err := formatGoStrict(buf.Bytes(), "cmd/server/main.go")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(targetDir, "main.go"), formatted, 0644)
}
