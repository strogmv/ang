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
	"github.com/strogmv/ang/templates"
)

// TemplateContext - единая структура данных для всех Go-шаблонов
type TemplateContext struct {
	Service          *normalizer.Service
	Method           *normalizer.Method
	Entities         []normalizer.Entity
	Entity           *normalizer.Entity
	Auth             *normalizer.AuthDef
	Imports          []string
	Metadata         map[string]interface{}
	Overrides        map[string]bool
	GoModule         string
	ProvenanceHeader string
	MissingImpls     []MissingImpl
}

type MissingImpl struct {
	Service string
	Method  string
	Source  string
}

type Emitter struct {
	IRSchema                 *ir.Schema
	WarningSink              func(normalizer.Warning)
	OutputDir                string
	FrontendDir              string
	FrontendAdminDir         string
	TemplatesDir             string // Путь к папке с шаблонами
	UIProviderPath           string
	Version                  string
	InputHash                string
	CompilerHash             string
	GoModule                 string // Go module path for imports
	NatsWorkers              int    // max concurrent NATS handlers per subscriber (from target.nats_workers)
	NatsPublishRetryAttempts int    // retry attempts on publish failure (from target.nats_publish_retry_attempts)
	NatsPublishRetryDelayMS  int    // initial backoff ms for publish retry (from target.nats_publish_retry_delay_ms)
	MissingImpls             []MissingImpl
	missingImplIndex         map[string]struct{}
}

const DefaultUIProviderPath = "@/components/ui/forms"

func New(outputDir, frontendDir, templatesDir string) *Emitter {
	if templatesDir == "" {
		templatesDir = "templates"
	}
	return &Emitter{
		OutputDir:      outputDir,
		FrontendDir:    frontendDir,
		TemplatesDir:   templatesDir,
		UIProviderPath: DefaultUIProviderPath,
	}
}

func (e *Emitter) resolvedUIProviderPath() string {
	p := strings.TrimSpace(e.UIProviderPath)
	if p == "" {
		return DefaultUIProviderPath
	}
	return p
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
	// 🚀 QUICK WIN: Dynamic template loading for faster development
	if customDir := os.Getenv("ANG_TEMPLATES_DIR"); customDir != "" {
		// Try relative name first
		name := tmplPath
		if strings.HasPrefix(name, "templates/") {
			name = strings.TrimPrefix(name, "templates/")
		}
		fullPath := filepath.Join(customDir, name)
		if data, err := os.ReadFile(fullPath); err == nil {
			return data, nil
		}
	}
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
		var hasDispatch func([]normalizer.FlowStep) bool
		hasDispatch = func(steps []normalizer.FlowStep) bool {
			for _, step := range steps {
				if step.Action == "notification.Dispatch" || step.Action == "notify.Dispatch" || step.Action == "notify.Send" {
					return true
				}
				if v, ok := step.Args["_do"].([]normalizer.FlowStep); ok && hasDispatch(v) {
					return true
				}
				if v, ok := step.Args["_then"].([]normalizer.FlowStep); ok && hasDispatch(v) {
					return true
				}
				if v, ok := step.Args["_else"].([]normalizer.FlowStep); ok && hasDispatch(v) {
					return true
				}
				if v, ok := step.Args["_default"].([]normalizer.FlowStep); ok && hasDispatch(v) {
					return true
				}
				if cases, ok := step.Args["_cases"].(map[string][]normalizer.FlowStep); ok {
					for _, branch := range cases {
						if hasDispatch(branch) {
							return true
						}
					}
				}
			}
			return false
		}
		for _, m := range s.Methods {
			if m.CacheTTL != "" {
				ctx.HasCache = true
			}
			if hasDispatch(m.Flow) {
				ctx.HasNotificationDispatch = true
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
		var hasDispatch func([]ir.FlowStep) bool
		hasDispatch = func(steps []ir.FlowStep) bool {
			for _, step := range steps {
				if step.Action == "notification.Dispatch" || step.Action == "notify.Dispatch" || step.Action == "notify.Send" {
					return true
				}
				if hasDispatch(step.Steps) || hasDispatch(step.Then) || hasDispatch(step.Else) {
					return true
				}
			}
			return false
		}
		for _, m := range s.Methods {
			if m.CacheTTL != "" {
				ctx.HasCache = true
			}
			if hasDispatch(m.Flow) {
				ctx.HasNotificationDispatch = true
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
			MaxConcurrent:    ep.MaxConcurrent,
			Coalesce:         ep.Coalesce,
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
		if ep.Retry != nil {
			n.RetryPolicy = &normalizer.RetryPolicyDef{
				Enabled:            ep.Retry.Enabled,
				MaxAttempts:        ep.Retry.MaxAttempts,
				BaseDelayMS:        ep.Retry.BaseDelayMS,
				RetryOnStatuses:    initializeIntSlice(ep.Retry.RetryOnStatuses),
				RetryNetworkErrors: ep.Retry.RetryNetworkErrors,
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
func WriteFileIfChanged(filename string, data []byte, perm os.FileMode) error {
	existing, err := os.ReadFile(filename)
	if err == nil && bytes.Equal(existing, data) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return err
	}
	return writeFileAtomic(filename, data, perm)
}

func (e *Emitter) RenderTemplate(name, text string, data interface{}) (string, error) {
	tmpl, err := template.New(name).Funcs(e.getSharedFuncMap()).Parse(text)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (e *Emitter) FormatGo(src []byte) ([]byte, error) {
	return src, nil
}
