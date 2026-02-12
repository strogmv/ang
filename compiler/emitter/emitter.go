package emitter

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
	"github.com/strogmv/ang/templates"
)

type Emitter struct {
	OutputDir        string
	FrontendDir      string
	FrontendAdminDir string
	TemplatesDir     string // Путь к папке с шаблонами
	Version          string
	InputHash        string
	CompilerHash     string
	GoModule         string // Go module path for imports
}

func New(outputDir, frontendDir, templatesDir string) *Emitter {
	if templatesDir == "" {
		templatesDir = "templates"
	}
	return &Emitter{
		OutputDir:    outputDir,
		FrontendDir:  frontendDir,
		TemplatesDir: templatesDir,
	}
}

// ReadTemplate reads a template file from embedded FS or disk.
// Priority: 1) Embedded FS (for installed binary), 2) Disk (for development)
func (e *Emitter) ReadTemplate(name string) ([]byte, error) {
	// First try embedded FS (works after `go install`)
	content, err := templates.FS.ReadFile(name)
	if err == nil {
		return content, nil
	}

	// Fall back to disk (for development with `go run`)
	diskPath := filepath.Join(e.TemplatesDir, name)
	return os.ReadFile(diskPath)
}

// ReadTemplateByPath reads a template using the emitter's templates directory.
// This is a compatibility wrapper for existing code.
func ReadTemplateByPath(tmplPath string) ([]byte, error) {
	// Extract relative path from full path
	// e.g., "templates/domain.tmpl" -> "domain.tmpl"
	// e.g., "templates/frontend/providers/mui/form.tmpl" -> "frontend/providers/mui/form.tmpl"
	name := tmplPath
	if strings.HasPrefix(name, "templates/") {
		name = strings.TrimPrefix(name, "templates/")
	} else if strings.HasPrefix(name, "templates\\") {
		name = strings.TrimPrefix(name, "templates\\")
	}

	candidates := []string{name}
	if !strings.HasPrefix(name, "go/") && !strings.HasPrefix(name, "python/") {
		candidates = append(candidates, filepath.ToSlash(filepath.Join("go", name)))
		candidates = append(candidates, filepath.ToSlash(filepath.Join("python", name)))
	}

	// Try embedded FS first
	for _, candidate := range candidates {
		content, err := templates.FS.ReadFile(candidate)
		if err == nil {
			return content, nil
		}
	}

	// Fall back to disk using template path candidates.
	root := "templates"
	for _, candidate := range candidates {
		path := filepath.Join(root, filepath.FromSlash(candidate))
		content, err := os.ReadFile(path)
		if err == nil {
			return content, nil
		}
	}

	// Final fallback for absolute/custom paths.
	return os.ReadFile(tmplPath)
}

func (e *Emitter) getSharedFuncMap() template.FuncMap {
	return template.FuncMap{
		"ANGVersion":   func() string { return e.Version },
		"InputHash":    func() string { return e.InputHash },
		"CompilerHash": func() string { return e.CompilerHash },
		"GoModule":     func() string { return e.GoModule },
		"Title":        ToTitle,
		"ExportName":   ExportName,
		"JSONName":     JSONName,
		"DBName":       DBName,
		"ToLower":      strings.ToLower,
		"contains":     strings.Contains,
		"hasPrefix":    strings.HasPrefix,
		"hasSuffix":    strings.HasSuffix,
		"replace":      strings.ReplaceAll,
		"Split":        strings.Split,
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, fmt.Errorf("invalid dict call")
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict keys must be strings")
				}
				dict[key] = values[i+1]
			}
			return dict, nil
		},
		"Indent": func(s string, n int) string {
			indent := strings.Repeat("\t", n)
			return strings.ReplaceAll(s, "\n", "\n"+indent)
		},
		"last": func(i int, list interface{}) bool {
			if list == nil {
				return false
			}
			switch v := list.(type) {
			case []normalizer.Field:
				return i == len(v)-1
			case []string:
				return i == len(v)-1
			case []normalizer.FlowStep:
				return i == len(v)-1
			case []normalizer.Entity:
				return i == len(v)-1
			default:
				return false
			}
		},
		"stringsEqualFold": strings.EqualFold,
		"sortedKeys": func(m interface{}) []string {
			switch v := m.(type) {
			case map[string]string:
				keys := make([]string, 0, len(v))
				for k := range v {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				return keys
			case map[string][]string:
				keys := make([]string, 0, len(v))
				for k := range v {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				return keys
			case map[string]interface{}:
				keys := make([]string, 0, len(v))
				for k := range v {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				return keys
			default:
				return nil
			}
		},
		"mapGet": func(m interface{}, key string) interface{} {
			switch v := m.(type) {
			case map[string]string:
				return v[key]
			case map[string][]string:
				return v[key]
			case map[string]interface{}:
				return v[key]
			default:
				return nil
			}
		},
		"makeMap": func() map[string]bool {
			return make(map[string]bool)
		},
		"mapHas": func(m map[string]bool, key string) bool {
			return m[key]
		},
		"mapSet": func(m map[string]bool, key string, val bool) string {
			m[key] = val
			return ""
		},
		"fail": func(msg string) (string, error) {
			return "", fmt.Errorf("TEMPLATE ASSERTION FAILED: %s", msg)
		},
		"assert": func(condition bool, msg string) (string, error) {
			if !condition {
				return "", fmt.Errorf("TEMPLATE ASSERTION FAILED: %s", msg)
			}
			return "", nil
		},
		"assertNotEmpty": func(s string, fieldName string) (string, error) {
			if s == "" {
				return "", fmt.Errorf("TEMPLATE ASSERTION FAILED: %s is empty", fieldName)
			}
			return "", nil
		},
		"assertHasFields": func(fields []normalizer.Field, entityName string) (string, error) {
			if len(fields) == 0 {
				return "", fmt.Errorf("TEMPLATE ASSERTION FAILED: %s has no fields - check CUE definition", entityName)
			}
			return "", nil
		},
		"FieldValidateTag": func(f normalizer.Field) string {
			tag := ""
			if f.ValidateTag != "" {
				tag = f.ValidateTag
			} else if v, ok := f.Metadata["validate"].(string); ok {
				tag = v
			}

			if tag != "" {
				if strings.HasPrefix(tag, "rule=") {
					tag = strings.TrimPrefix(tag, "rule=")
					tag = strings.Trim(tag, "\"")
				}
				if f.IsOptional && !strings.Contains(tag, "omitempty") {
					tag = "omitempty," + tag
				}
			}
			return tag
		},
		"lowerFirst": func(s string) string {
			if len(s) == 0 {
				return ""
			}
			return strings.ToLower(s[:1]) + s[1:]
		},
		"getRepoEntities": func(s normalizer.Service, entities []normalizer.Entity) []string {
			dtoEntities := make(map[string]bool, len(entities))
			for _, ent := range entities {
				if dto, ok := ent.Metadata["dto"].(bool); ok && dto {
					dtoEntities[ent.Name] = true
				}
			}
			unique := make(map[string]bool)
			var res []string
			var scanSteps func([]normalizer.FlowStep)
			scanSteps = func(steps []normalizer.FlowStep) {
				for _, step := range steps {
					if strings.HasPrefix(step.Action, "repo.") {
						ent := step.Args["source"]
						if entName, ok := ent.(string); ok && entName != "" && !unique[entName] && !dtoEntities[entName] {
							// Check ownership (stored in entities metadata or deduced)
							// Note: We don't have easy access to EntityOwners here, so we skip enforcement
							// in injection for now and rely on LINTER which already has the check.
							// This is safer to avoid breaking legitimate cross-service READS in monolith mode.
							unique[entName] = true
							res = append(res, entName)
						}
					}
					if v, ok := step.Args["_do"].([]normalizer.FlowStep); ok {
						scanSteps(v)
					}
					if v, ok := step.Args["_then"].([]normalizer.FlowStep); ok {
						scanSteps(v)
					}
					if v, ok := step.Args["_else"].([]normalizer.FlowStep); ok {
						scanSteps(v)
					}
				}
			}
			for _, m := range s.Methods {
				for _, src := range m.Sources {
					if src.Entity != "" && !unique[src.Entity] && !dtoEntities[src.Entity] {
						unique[src.Entity] = true
						res = append(res, src.Entity)
					}
				}
				scanSteps(m.Flow)
			}
			sort.Strings(res)
			return res
		},
		"getServiceDeps": func(s normalizer.Service) []string {
			if len(s.Uses) == 0 {
				return nil
			}
			deps := append([]string{}, s.Uses...)
			sort.Strings(deps)
			return deps
		},
		"ServiceNeedsTx": func(s normalizer.Service) bool {
			var scanSteps func([]normalizer.FlowStep) bool
			scanSteps = func(steps []normalizer.FlowStep) bool {
				for _, step := range steps {
					if step.Action == "tx.Block" {
						return true
					}
					if v, ok := step.Args["_do"].([]normalizer.FlowStep); ok {
						if scanSteps(v) {
							return true
						}
					}
					if v, ok := step.Args["_then"].([]normalizer.FlowStep); ok {
						if scanSteps(v) {
							return true
						}
					}
					if v, ok := step.Args["_else"].([]normalizer.FlowStep); ok {
						if scanSteps(v) {
							return true
						}
					}
				}
				return false
			}
			for _, m := range s.Methods {
				if scanSteps(m.Flow) {
					return true
				}
				if m.Impl != nil && m.Impl.RequiresTx {
					return true
				}
			}
			return false
		},
		"ServiceHasOutbox": func(s normalizer.Service) bool {
			for _, m := range s.Methods {
				if m.Outbox {
					return true
				}
			}
			return false
		},
		"ServiceHasIdempotency": func(s normalizer.Service) bool {
			for _, m := range s.Methods {
				if m.Idempotency {
					return true
				}
			}
			return false
		},
		"AnyServiceHasIdempotencyOrOutbox": func(services []normalizer.Service) bool {
			for _, s := range services {
				for _, m := range s.Methods {
					if m.Idempotency || m.Outbox {
						return true
					}
				}
			}
			return false
		},
		"MethodHasIdempotency": func(m normalizer.Method) bool {
			return m.Idempotency
		},
		"MethodHasOutbox": func(m normalizer.Method) bool {
			return m.Outbox
		},
		"ServiceHasPublishes": func(s normalizer.Service) bool {
			var scanSteps func([]normalizer.FlowStep) bool
			scanSteps = func(steps []normalizer.FlowStep) bool {
				for _, step := range steps {
					if step.Action == "event.Publish" {
						return true
					}
					if v, ok := step.Args["_do"].([]normalizer.FlowStep); ok {
						if scanSteps(v) {
							return true
						}
					}
					if v, ok := step.Args["_then"].([]normalizer.FlowStep); ok {
						if scanSteps(v) {
							return true
						}
					}
					if v, ok := step.Args["_else"].([]normalizer.FlowStep); ok {
						if scanSteps(v) {
							return true
						}
					}
				}
				return false
			}
			for _, m := range s.Methods {
				if len(m.Publishes) > 0 {
					return true
				}
				if scanSteps(m.Flow) {
					return true
				}
			}
			return false
		},
		"HasEventMethods": func(s normalizer.Service) bool {
			for _, m := range s.Methods {
				if len(m.Publishes) > 0 && m.Input.Name == "" && m.Output.Name == "" {
					return true
				}
			}
			return false
		},
		"HasDomainTypes": func(s normalizer.Service) bool {
			hasDomain := func(ent normalizer.Entity) bool {
				for _, f := range ent.Fields {
					if strings.HasPrefix(f.Type, "domain.") || strings.HasPrefix(f.ItemTypeName, "domain.") {
						return true
					}
				}
				return false
			}
			for _, m := range s.Methods {
				if (m.Input.Name != "" && hasDomain(m.Input)) || (m.Output.Name != "" && hasDomain(m.Output)) {
					return true
				}
			}
			return false
		},
		"ServiceNestedTypes": func(s normalizer.Service) []normalizer.Entity {
			typeMap := make(map[string]normalizer.Entity)
			addNested := func(ent normalizer.Entity) {
				for _, f := range ent.Fields {
					if f.ItemTypeName == "" || len(f.ItemFields) == 0 {
						continue
					}
					if _, ok := typeMap[f.ItemTypeName]; ok {
						continue
					}
					typeMap[f.ItemTypeName] = normalizer.Entity{Name: f.ItemTypeName, Fields: f.ItemFields}
				}
			}
			for _, m := range s.Methods {
				if m.Input.Name != "" {
					addNested(m.Input)
				}
				if m.Output.Name != "" {
					addNested(m.Output)
				}
			}
			var res []normalizer.Entity
			for _, v := range typeMap {
				res = append(res, v)
			}
			sort.Slice(res, func(i, j int) bool { return res[i].Name < res[j].Name })
			return res
		},
		"EventForMethod": func(s normalizer.Service, m normalizer.Method) string {
			if len(m.Publishes) > 0 && m.Input.Name == "" && m.Output.Name == "" {
				return m.Publishes[0]
			}
			return ""
		},
		"ZeroValue": func(ent normalizer.Entity) string {
			return "port." + ent.Name + "{}"
		},
		"getSteps": func(step normalizer.FlowStep) []normalizer.FlowStep {
			if v, ok := step.Args["_do"].([]normalizer.FlowStep); ok {
				return v
			}
			return nil
		},
		"getThen": func(step normalizer.FlowStep) []normalizer.FlowStep {
			if v, ok := step.Args["_then"].([]normalizer.FlowStep); ok {
				return v
			}
			return nil
		},
		"getElse": func(step normalizer.FlowStep) []normalizer.FlowStep {
			if v, ok := step.Args["_else"].([]normalizer.FlowStep); ok {
				return v
			}
			return nil
		},
		"pointerExpr": func(input string) string {
			if strings.HasPrefix(input, "new") {
				return "&" + input
			}
			return input
		},
		"EndpointType": func(ep normalizer.Endpoint) string {
			method := strings.ToUpper(ep.Method)
			path := ep.Path

			// Action pattern: /api/.../{id}/{action}
			if method == "POST" && strings.HasSuffix(path, "}") == false {
				parts := strings.Split(path, "/")
				if len(parts) >= 2 && strings.HasPrefix(parts[len(parts)-2], "{") {
					return "action"
				}
			}

			if strings.HasSuffix(path, "}") {
				switch method {
				case "GET":
					return "get"
				case "PATCH", "PUT":
					return "update"
				case "DELETE":
					return "delete"
				}
			} else {
				switch method {
				case "GET":
					return "list"
				case "POST":
					return "create"
				}
			}
			return "other"
		},
		"getEntityFields": func(entities []normalizer.Entity, name string) []normalizer.Field {
			for _, e := range entities {
				if strings.EqualFold(e.Name, name) {
					return e.Fields
				}
			}
			return nil
		},
		"hasField": func(fields []normalizer.Field, name string) bool {
			for _, f := range fields {
				if strings.EqualFold(f.Name, name) {
					return true
				}
			}
			return false
		},
		"ServiceImplImports": func(s normalizer.Service) []string {
			goMod := e.GoModule
			if goMod == "" {
				goMod = "github.com/strogmv/ang"
			}
			importsMap := map[string]bool{
				goMod + "/internal/domain":      true,
				goMod + "/internal/pkg/errors":  true,
				goMod + "/internal/pkg/helpers": true,
				goMod + "/internal/port":        true,
				"net/http":                      true,
				"github.com/google/uuid":        true,
				"time":                          true,
			}
			for _, m := range s.Methods {
				if m.Impl != nil {
					for _, imp := range m.Impl.Imports {
						importsMap[imp] = true
					}
				}
			}
			result := make([]string, 0, len(importsMap))
			for imp := range importsMap {
				result = append(result, imp)
			}
			sort.Strings(result)
			return result
		},
		"EntityStorage": func(ent normalizer.Entity) string {
			if ent.Metadata != nil {
				if v, ok := ent.Metadata["storage"].(string); ok && v != "" {
					return v
				}
			}
			return "sql"
		},
		"EntityStorageByName": func(entities []normalizer.Entity, name string) string {
			for _, ent := range entities {
				if ent.Name == name {
					if ent.Metadata != nil {
						if v, ok := ent.Metadata["storage"].(string); ok && v != "" {
							return v
						}
					}
					return "sql"
				}
			}
			return "sql"
		},
		"HasMongoRepoEntities": func(entities []normalizer.Entity) bool {
			for _, ent := range entities {
				if ent.Metadata != nil {
					if v, ok := ent.Metadata["storage"].(string); ok && v == "mongo" {
						return true
					}
				}
			}
			return false
		},
		"InitScope": func() map[string]bool {
			return make(map[string]bool)
		},
		"CloneScope": func(scope map[string]bool) map[string]bool {
			newScope := make(map[string]bool)
			for k, v := range scope {
				newScope[k] = v
			}
			return newScope
		},
		"Assign": func(scope map[string]bool, name string) string {
			if strings.Contains(name, ".") || scope[name] {
				return name + ", err ="
			}
			scope[name] = true
			return name + ", err :="
		},
		"AssignSimple": func(scope map[string]bool, name string) string {
			if strings.Contains(name, ".") || scope[name] {
				return name + " ="
			}
			scope[name] = true
			return name + " :="
		},
		"Declare": func(scope map[string]bool, name string) string {
			if scope[name] {
				return ""
			}
			scope[name] = true
			return "var " + name
		},
		"PrepareCodeBlock": func(code string) string {
			lines := strings.Split(code, "\n")
			var result []string
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				// Remove redundant variable declarations (var x T)
				if strings.HasPrefix(trimmed, "var resp ") ||
					strings.HasPrefix(trimmed, "var err error") {
					result = append(result, "// "+line+" // Removed by ANG Emitter")
					continue
				}
				// Convert short assignments (x := ...) to regular assignments (x = ...)
				// if they target the named return variables 'resp' or 'err'
				if strings.HasPrefix(trimmed, "resp :=") || strings.HasPrefix(trimmed, "resp:=") {
					result = append(result, strings.Replace(line, ":=", "=", 1))
					continue
				}
				if strings.HasPrefix(trimmed, "err :=") || strings.HasPrefix(trimmed, "err:=") {
					result = append(result, strings.Replace(line, ":=", "=", 1))
					continue
				}
				// Fix common "resp, err :=" assignments
				if strings.HasPrefix(trimmed, "resp, err :=") || strings.HasPrefix(trimmed, "resp, err:=") {
					result = append(result, strings.Replace(line, ":=", "=", 1))
					continue
				}
				result = append(result, line)
			}
			return strings.Join(result, "\n")
		},
	}
}

type MainContext struct {
	IRSchema                *ir.Schema
	ServicesIR              []ir.Service
	EntitiesIR              []ir.Entity
	EndpointsIR             []ir.Endpoint
	Services                []normalizer.Service
	Entities                []normalizer.Entity
	Endpoints               []normalizer.Endpoint
	HasCache                bool
	HasSQL                  bool
	HasMongo                bool
	HasNats                 bool
	HasS3                   bool
	WebSocketServices       map[string]bool
	HasScheduler            bool
	WSEventMap              map[string]map[string]bool
	EventPayloads           map[string]normalizer.Entity
	EventPayloadsIR         map[string]ir.Entity
	WSRoomField             map[string]string
	AuthService             string
	AuthRefreshStore        string
	InputHash               string
	CompilerHash            string
	ANGVersion              string
	EntityOwners            map[string]string
	GoModule                string // Go module path for imports (e.g., "github.com/strog/dealingi-back")
	NotificationMuting      bool   // Enable notification muting decorator
	HasNotificationsService bool
}

// AnalyzeContext checks which infrastructure dependencies are required.
func (e *Emitter) AnalyzeContext(services []normalizer.Service, entities []normalizer.Entity, endpoints []normalizer.Endpoint) MainContext {
	ctx := MainContext{
		Services:          services,
		Entities:          entities,
		Endpoints:         endpoints,
		WebSocketServices: make(map[string]bool),
		WSEventMap:        make(map[string]map[string]bool),
		EventPayloads:     make(map[string]normalizer.Entity),
		WSRoomField:       make(map[string]string),
		EntityOwners:      make(map[string]string),
	}
	for _, ent := range entities {
		ctx.EntityOwners[ent.Name] = ent.Owner
	}
	for _, s := range services {
		if s.Name == "Notifications" {
			ctx.HasNotificationsService = true
		}
		for _, m := range s.Methods {
			if m.CacheTTL != "" {
				ctx.HasCache = true
			}
		}
		if len(s.Publishes) > 0 || len(s.Subscribes) > 0 {
			ctx.HasNats = true
		}
		if s.RequiresS3 {
			ctx.HasS3 = true
		}
	}
	for _, ent := range entities {
		if v, ok := ent.Metadata["storage"].(string); ok && v == "mongo" {
			ctx.HasMongo = true
		}
		for _, f := range ent.Fields {
			if strings.EqualFold(f.DB.Type, "ObjectId") {
				ctx.HasMongo = true
			} else if f.DB.Type != "" {
				ctx.HasSQL = true
			}
		}
	}
	ctx.Services = OrderServicesByDependencies(ctx.Services)
	return ctx
}

// AnalyzeContextFromIR computes runtime context from IR while preserving legacy MainContext shape.
func (e *Emitter) AnalyzeContextFromIR(schema *ir.Schema) MainContext {
	if schema == nil {
		return e.AnalyzeContext(nil, nil, nil)
	}
	ctx := MainContext{
		IRSchema:          schema,
		ServicesIR:        append([]ir.Service{}, schema.Services...),
		EntitiesIR:        append([]ir.Entity{}, schema.Entities...),
		EndpointsIR:       append([]ir.Endpoint{}, schema.Endpoints...),
		WebSocketServices: make(map[string]bool),
		WSEventMap:        make(map[string]map[string]bool),
		EventPayloads:     make(map[string]normalizer.Entity),
		EventPayloadsIR:   make(map[string]ir.Entity),
		WSRoomField:       make(map[string]string),
		EntityOwners:      make(map[string]string),
	}
	for _, s := range schema.Services {
		if s.Name == "Notifications" {
			ctx.HasNotificationsService = true
		}
		for _, m := range s.Methods {
			if m.CacheTTL != "" {
				ctx.HasCache = true
			}
		}
		if len(s.Publishes) > 0 || len(s.Subscribes) > 0 || s.RequiresNats {
			ctx.HasNats = true
		}
		if s.RequiresS3 {
			ctx.HasS3 = true
		}
		if s.RequiresSQL {
			ctx.HasSQL = true
		}
		if s.RequiresMongo {
			ctx.HasMongo = true
		}
		if s.RequiresRedis {
			ctx.HasCache = true
		}
	}
	for _, ent := range schema.Entities {
		ctx.EntityOwners[ent.Name] = ent.Owner
		if v, ok := ent.Metadata["storage"].(string); ok && v == "mongo" {
			ctx.HasMongo = true
		}
	}

	services := make([]normalizer.Service, 0, len(schema.Services))
	for _, s := range schema.Services {
		services = append(services, contextServiceFromIR(s))
	}
	entities := make([]normalizer.Entity, 0, len(schema.Entities))
	for _, ent := range schema.Entities {
		entities = append(entities, contextEntityFromIR(ent))
	}
	endpoints := make([]normalizer.Endpoint, 0, len(schema.Endpoints))
	for _, ep := range schema.Endpoints {
		n := normalizer.Endpoint{
			Method:           ep.Method,
			Path:             ep.Path,
			ServiceName:      ep.Service,
			RPC:              ep.RPC,
			Description:      ep.Description,
			Messages:         append([]string{}, ep.Messages...),
			RoomParam:        ep.RoomParam,
			CacheTTL:         ep.Cache,
			CacheTags:        initializeSlice(ep.CacheTags),
			Invalidate:       initializeSlice(ep.Invalidate),
			OptimisticUpdate: ep.OptimisticUpdate,
			Timeout:          ep.Timeout,
			MaxBodySize:      ep.MaxBodySize,
			Idempotency:      ep.Idempotent,
			DedupeKey:        ep.DedupeKey,
			Errors:           append([]string{}, ep.Errors...),
			View:             ep.View,
			Metadata:         ep.Metadata,
			Source:           ep.Source,
		}
		if ep.Auth != nil {
			n.AuthType = ep.Auth.Type
			n.Permission = ep.Auth.Permission
			n.AuthRoles = append([]string{}, ep.Auth.Roles...)
			n.AuthCheck = ep.Auth.Check
			n.AuthInject = append([]string{}, ep.Auth.Inject...)
		}
		if ep.RateLimit != nil {
			n.RateLimit = &normalizer.RateLimitDef{
				RPS:   ep.RateLimit.RPS,
				Burst: ep.RateLimit.Burst,
			}
		}
		if ep.CircuitBreaker != nil {
			n.CircuitBreaker = &normalizer.CircuitBreakerDef{
				Threshold:   ep.CircuitBreaker.Threshold,
				Timeout:     ep.CircuitBreaker.Timeout,
				HalfOpenMax: ep.CircuitBreaker.HalfOpenMax,
			}
		}
		if ep.Pagination != nil {
			n.Pagination = &normalizer.PaginationDef{
				Type:         ep.Pagination.Type,
				DefaultLimit: ep.Pagination.DefaultLimit,
				MaxLimit:     ep.Pagination.MaxLimit,
			}
		}
		if ep.SLO != nil {
			n.SLO = normalizer.SLODef{
				Latency: ep.SLO.Latency,
				Success: ep.SLO.Success,
			}
		}
		if ep.TestHints != nil {
			n.TestHints = &normalizer.TestHints{
				HappyPath:  ep.TestHints.HappyPath,
				ErrorCases: initializeSlice(ep.TestHints.ErrorCases),
			}
		}
		endpoints = append(endpoints, n)
	}
	ctx.Services = OrderServicesByDependencies(services)
	ctx.Entities = entities
	ctx.Endpoints = endpoints
	return ctx
}

// EnrichContextFromIR fills runtime routing/event maps from IR.
func (e *Emitter) EnrichContextFromIR(ctx *MainContext, schema *ir.Schema) {
	if ctx == nil || schema == nil {
		return
	}
	if ctx.EventPayloads == nil {
		ctx.EventPayloads = make(map[string]normalizer.Entity)
	}
	if ctx.EventPayloadsIR == nil {
		ctx.EventPayloadsIR = make(map[string]ir.Entity)
	}
	if ctx.WebSocketServices == nil {
		ctx.WebSocketServices = make(map[string]bool)
	}
	if ctx.WSEventMap == nil {
		ctx.WSEventMap = make(map[string]map[string]bool)
	}
	if ctx.WSRoomField == nil {
		ctx.WSRoomField = make(map[string]string)
	}

	for _, ev := range schema.Events {
		fields := make([]normalizer.Field, 0, len(ev.Fields))
		for _, f := range ev.Fields {
			fields = append(fields, normalizer.Field{Name: f.Name})
		}
		ctx.EventPayloadsIR[ev.Name] = ir.Entity{
			Name:   ev.Name,
			Fields: append([]ir.Field{}, ev.Fields...),
		}
		ctx.EventPayloads[ev.Name] = normalizer.Entity{
			Name:   ev.Name,
			Fields: fields,
		}
	}
	for _, ep := range schema.Endpoints {
		if strings.ToUpper(ep.Method) != "WS" {
			continue
		}
		ctx.WebSocketServices[ep.Service] = true
		if ctx.WSEventMap[ep.Service] == nil {
			ctx.WSEventMap[ep.Service] = make(map[string]bool)
		}
		for _, msg := range ep.Messages {
			if msg != "" {
				ctx.WSEventMap[ep.Service][msg] = true
			}
		}
		if ctx.WSRoomField[ep.Service] != "" {
			continue
		}
		param := ep.RoomParam
		if param == "" {
			param = firstPathParam(ep.Path)
		}
		if param != "" {
			ctx.WSRoomField[ep.Service] = ExportName(param)
		}
	}
}

func contextServiceFromIR(s ir.Service) normalizer.Service {
	out := normalizer.Service{
		Name:          s.Name,
		Publishes:     append([]string{}, s.Publishes...),
		Subscribes:    s.Subscribes,
		Uses:          append([]string{}, s.Uses...),
		RequiresSQL:   s.RequiresSQL,
		RequiresMongo: s.RequiresMongo,
		RequiresRedis: s.RequiresRedis,
		RequiresNats:  s.RequiresNats,
		RequiresS3:    s.RequiresS3,
	}
	out.Methods = make([]normalizer.Method, 0, len(s.Methods))
	for _, m := range s.Methods {
		nm := normalizer.Method{
			Name:        m.Name,
			CacheTTL:    m.CacheTTL,
			Publishes:   append([]string{}, m.Publishes...),
			Idempotency: m.Idempotent,
			Outbox:      m.Outbox,
		}
		if m.Impl != nil {
			nm.Impl = &normalizer.MethodImpl{RequiresTx: m.Impl.RequiresTx}
		}
		nm.Flow = irFlowStepsToNormalizer(m.Flow)
		if len(m.Sources) > 0 {
			nm.Sources = make([]normalizer.Source, 0, len(m.Sources))
			for _, src := range m.Sources {
				nm.Sources = append(nm.Sources, normalizer.Source{Entity: src.Entity})
			}
		}
		out.Methods = append(out.Methods, nm)
	}
	return out
}

func contextEntityFromIR(ent ir.Entity) normalizer.Entity {
	fields := make([]normalizer.Field, 0, len(ent.Fields))
	for _, f := range ent.Fields {
		fields = append(fields, normalizer.Field{
			Name: f.Name,
			DB: normalizer.DBMeta{
				Type: func() string {
					for _, a := range f.Attributes {
						if a.Name == "db" {
							if t, ok := a.Args["type"].(string); ok {
								return t
							}
						}
					}
					return ""
				}(),
			},
		})
	}
	return normalizer.Entity{
		Name:     ent.Name,
		Owner:    ent.Owner,
		Fields:   fields,
		Metadata: ent.Metadata,
	}
}

func OrderServicesByDependencies(services []normalizer.Service) []normalizer.Service {
	if len(services) == 0 {
		return services
	}
	byName := make(map[string]normalizer.Service, len(services))
	inDegree := make(map[string]int, len(services))
	graph := make(map[string][]string, len(services))
	for _, svc := range services {
		byName[svc.Name] = svc
		inDegree[svc.Name] = 0
	}
	for _, svc := range services {
		for _, dep := range svc.Uses {
			if _, ok := byName[dep]; !ok {
				continue
			}
			graph[dep] = append(graph[dep], svc.Name)
			inDegree[svc.Name]++
		}
	}

	queue := make([]string, 0, len(services))
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}
	sort.Strings(queue)

	result := make([]normalizer.Service, 0, len(services))
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if svc, ok := byName[name]; ok {
			result = append(result, svc)
		}
		for _, next := range graph[name] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
				sort.Strings(queue)
			}
		}
	}

	if len(result) != len(services) {
		seen := make(map[string]bool, len(result))
		for _, svc := range result {
			seen[svc.Name] = true
		}
		for _, svc := range services {
			if !seen[svc.Name] {
				result = append(result, svc)
			}
		}
	}

	return result
}

func ValidateServiceDependencies(services []normalizer.Service) error {
	if len(services) == 0 {
		return nil
	}
	byName := make(map[string]normalizer.Service, len(services))
	inDegree := make(map[string]int, len(services))
	graph := make(map[string][]string, len(services))
	for _, svc := range services {
		byName[svc.Name] = svc
		inDegree[svc.Name] = 0
	}

	var missing []string
	for _, svc := range services {
		for _, dep := range svc.Uses {
			if _, ok := byName[dep]; !ok {
				missing = append(missing, svc.Name+" -> "+dep)
				continue
			}
			graph[dep] = append(graph[dep], svc.Name)
			inDegree[svc.Name]++
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("unknown service dependencies: %s", strings.Join(missing, ", "))
	}

	queue := make([]string, 0, len(services))
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		for _, next := range graph[name] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	var cycle []string
	for name, deg := range inDegree {
		if deg > 0 {
			cycle = append(cycle, name)
		}
	}
	if len(cycle) > 0 {
		sort.Strings(cycle)
		return fmt.Errorf("cycle detected among services: %s", strings.Join(cycle, ", "))
	}
	return nil
}
