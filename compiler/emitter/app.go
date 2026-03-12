package emitter

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/strogmv/ang-ir/ir"
	"github.com/strogmv/ang-ir/normalizer"
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
	ServicesIR              []ir.Service
	EntitiesIR              []ir.Entity
}

type MainServerRepositoriesContext struct {
	AuthService             string
	AuthRefreshStore        string
	NotificationMuting      bool
	HasNotificationsService bool
	HasNotificationDispatch bool
	HasSQL                  bool
	HasMongo                bool
	HasCache                bool
	HasS3                   bool
	ServicesIR              []ir.Service
	EntitiesIR              []ir.Entity
}

type MainServerRuntimeContainerContext struct {
	AuthService             string
	AuthRefreshStore        string
	NotificationMuting      bool
	HasNotificationsService bool
	HasNotificationDispatch bool
	HasSQL                  bool
	HasMongo                bool
	HasCache                bool
	HasS3                   bool
	GoModule                string
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

type MainServerHTTPRouterContext struct {
	HasSession bool
}

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
	RuntimeContainer MainServerRuntimeContainerContext
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
			ServicesIR:              ctx.ServicesIR,
			EntitiesIR:              ctx.EntitiesIR,
		},
		Repositories: MainServerRepositoriesContext{
			AuthService:             ctx.AuthService,
			AuthRefreshStore:        ctx.AuthRefreshStore,
			NotificationMuting:      ctx.NotificationMuting,
			HasNotificationsService: ctx.HasNotificationsService,
			HasNotificationDispatch: ctx.HasNotificationDispatch,
			HasSQL:                  ctx.HasSQL,
			HasMongo:                ctx.HasMongo,
			HasCache:                ctx.HasCache,
			HasS3:                   ctx.HasS3,
			ServicesIR:              ctx.ServicesIR,
			EntitiesIR:              ctx.EntitiesIR,
		},
		RuntimeContainer: MainServerRuntimeContainerContext{
			AuthService:             ctx.AuthService,
			AuthRefreshStore:        ctx.AuthRefreshStore,
			NotificationMuting:      ctx.NotificationMuting,
			HasNotificationsService: ctx.HasNotificationsService,
			HasNotificationDispatch: ctx.HasNotificationDispatch,
			HasSQL:                  ctx.HasSQL,
			HasMongo:                ctx.HasMongo,
			HasCache:                ctx.HasCache,
			HasS3:                   ctx.HasS3,
			GoModule:                ctx.GoModule,
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
		HTTPRouter: MainServerHTTPRouterContext{
			HasSession: ctx.HasSession,
		},
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
		"templates/main_server/runtime_container.tmpl",
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
	return WriteFileIfChanged(filepath.Join(targetDir, "main.go"), formatted, 0644)
}

// EmitMain generates cmd/server/main.go (monolith).
func (e *Emitter) EmitMain(ctx MainContext) error {
	t, err := e.parseMainServerTemplate()
	if err != nil {
		return err
	}
	if err := e.EmitRuntimeContainer(ctx); err != nil {
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
	return WriteFileIfChanged(filepath.Join(targetDir, "main.go"), formatted, 0644)
}

func (e *Emitter) EmitRuntimeContainer(ctx MainContext) error {
	t, err := e.parseMainServerTemplate()
	if err != nil {
		return err
	}

	targetDir := filepath.Join(e.OutputDir, "internal", "bootstrap")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	data := buildMainServerTemplateData(ctx)
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "main_server_runtime_container", data.RuntimeContainer); err != nil {
		return err
	}
	formatted, err := formatGoStrict(buf.Bytes(), "internal/bootstrap/runtime_container.go")
	if err != nil {
		return err
	}
	path := filepath.Join(targetDir, "runtime_container.go")
	if shouldPreserveGoCustomBlocks("internal/bootstrap/runtime_container.go") {
		if prev, err := os.ReadFile(path); err == nil {
			formatted = []byte(mergeGoCustomBlocks(string(formatted), string(prev)))
		}
	}
	return WriteFileIfChanged(path, formatted, 0644)
}
