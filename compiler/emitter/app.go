package emitter

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
)

// EmitMicroservices generates separate binaries for each service.
func (e *Emitter) EmitMicroservices(services []ir.Service, wsServices map[string]bool, auth *normalizer.AuthDef) error {
	for _, svc := range services {
		svcNorm := IRServiceToNormalizer(svc)
		ctx := MainContext{
			Services: []normalizer.Service{svcNorm},
			HasCache: svc.RequiresRedis,
			HasSQL:   svc.RequiresSQL,
			HasMongo: svc.RequiresMongo,
			HasNats:  svc.RequiresNats,
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
	appFuncs["HasRepoEntities"] = func(services []normalizer.Service) bool {
		for _, svc := range services {
			unique := make(map[string]bool)
			var count int
			for _, m := range svc.Methods {
				for _, src := range m.Sources {
					if src.Entity != "" && !unique[src.Entity] {
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
	tmplPath := "templates/main_server.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	t, err := template.New("main_server").Funcs(e.getAppFuncMap()).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "cmd", "services", strings.ToLower(svcName))
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, ctx); err != nil {
		return err
	}
	formatted, _ := format.Source(buf.Bytes())
	if formatted == nil {
		formatted = buf.Bytes()
	}
	return os.WriteFile(filepath.Join(targetDir, "main.go"), formatted, 0644)
}

// EmitMain generates cmd/server/main.go (monolith).
func (e *Emitter) EmitMain(ctx MainContext) error {
	tmplPath := "templates/main_server.tmpl"
	tmplContent, err := ReadTemplateByPath(tmplPath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	t, err := template.New("main_server").Funcs(e.getAppFuncMap()).Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	targetDir := filepath.Join(e.OutputDir, "cmd", "server")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, ctx); err != nil {
		return err
	}
	formatted, _ := format.Source(buf.Bytes())
	if formatted == nil {
		formatted = buf.Bytes()
	}
	return os.WriteFile(filepath.Join(targetDir, "main.go"), formatted, 0644)
}
