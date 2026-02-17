// Package emitter provides IR-based code generation.
// This file contains methods that work directly with the universal IR.
package emitter

import (
	"fmt"
	"sort"

	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
	"github.com/strogmv/ang/compiler/providers"
)

// IRContext holds the IR schema and provider for generation.
type IRContext struct {
	Schema   *ir.Schema
	Provider providers.Provider
	TypeMap  providers.TypeMap
}

// EmitFromIR generates all code from the IR schema.
// This is the main entry point for IR-based generation.
func (e *Emitter) EmitFromIR(schema *ir.Schema) error {
	if err := ir.MigrateToCurrent(schema); err != nil {
		return fmt.Errorf("ir version migration: %w", err)
	}

	// Use existing emit methods with converted data
	if err := e.EmitDomain(schema.Entities); err != nil {
		return err
	}
	if err := e.EmitService(schema.Services); err != nil {
		return err
	}

	return nil
}

// IREntitiesToNormalizer converts IR entities to normalizer entities.
func IREntitiesToNormalizer(irEntities []ir.Entity) []normalizer.Entity {
	result := make([]normalizer.Entity, 0, len(irEntities))
	for _, e := range irEntities {
		result = append(result, IREntityToNormalizer(e))
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

// IREntityToNormalizer converts a single IR entity.
func IREntityToNormalizer(e ir.Entity) normalizer.Entity {
	entity := normalizer.Entity{
		Name:        e.Name,
		Description: e.Description,
		Owner:       e.Owner,
		Fields:      IRFieldsToNormalizer(e.Fields),
		Metadata:    e.Metadata,
		Source:      e.Source,
	}

	if e.UI.CRUD != nil {
		entity.UI = &normalizer.EntityUIDef{
			CRUD: &normalizer.CRUDDef{
				Enabled: e.UI.CRUD.Enabled,
				Custom:  e.UI.CRUD.Custom,
				Views:   e.UI.CRUD.Views,
				Perms:   e.UI.CRUD.Perms,
			},
		}
	}

	if e.FSM != nil {
		entity.FSM = &normalizer.FSM{
			Field:       e.FSM.Field,
			States:      e.FSM.States,
			Transitions: e.FSM.Transitions,
		}
	}

	for _, idx := range e.Indexes {
		entity.Indexes = append(entity.Indexes, normalizer.IndexDef{
			Fields: idx.Fields,
			Unique: idx.Unique,
		})
	}

	return entity
}

// IRFieldsToNormalizer converts IR fields to normalizer fields.
func IRFieldsToNormalizer(irFields []ir.Field) []normalizer.Field {
	result := make([]normalizer.Field, 0, len(irFields))
	for _, f := range irFields {
		result = append(result, IRFieldToNormalizer(f))
	}
	return result
}

// IRFieldToNormalizer converts a single IR field.
func IRFieldToNormalizer(f ir.Field) normalizer.Field {
	field := normalizer.Field{
		Name:        f.Name,
		Type:        IRTypeRefToGoType(f.Type),
		IsOptional:  f.Optional,
		IsList:      f.Type.Kind == ir.KindList,
		IsSecret:    f.IsSecret,
		IsPII:       f.IsPII,
		SkipDomain:  f.SkipDomain,
		ValidateTag: f.ValidateTag,
		EnvVar:      f.EnvVar,
		Metadata:    f.Metadata,
		Source:      f.Source,
	}
	if f.Constraints != nil {
		var enum []string
		if f.Constraints.Enum != nil {
			enum = append([]string{}, f.Constraints.Enum...)
		}
		field.Constraints = &normalizer.Constraints{
			Min:    f.Constraints.Min,
			Max:    f.Constraints.Max,
			MinLen: f.Constraints.MinLen,
			MaxLen: f.Constraints.MaxLen,
			Regex:  f.Constraints.Regex,
			Enum:   enum,
		}
	}

	if f.UI.Type != "" || f.UI.Label != "" || f.UI.Order != 0 || f.UI.Component != "" || f.UI.Section != "" || f.UI.Columns != 0 || f.UI.Importance != "" || f.UI.InputKind != "" || f.UI.Intent != "" || f.UI.Density != "" || f.UI.LabelMode != "" || f.UI.Surface != "" {
		field.UI = &normalizer.UIHints{
			Type:        f.UI.Type,
			Importance:  f.UI.Importance,
			InputKind:   f.UI.InputKind,
			Intent:      f.UI.Intent,
			Density:     f.UI.Density,
			LabelMode:   f.UI.LabelMode,
			Surface:     f.UI.Surface,
			Component:   f.UI.Component,
			Section:     f.UI.Section,
			Columns:     f.UI.Columns,
			Label:       f.UI.Label,
			Placeholder: f.UI.Placeholder,
			HelperText:  f.UI.HelperText,
			Order:       f.UI.Order,
			Hidden:      f.UI.Hidden,
			Disabled:    f.UI.Disabled,
			FullWidth:   f.UI.FullWidth,
			Rows:        f.UI.Rows,
			Min:         f.UI.Min,
			Max:         f.UI.Max,
			Step:        f.UI.Step,
			Currency:    f.UI.Currency,
			Source:      f.UI.Source,
			Options:     f.UI.Options,
			Multiple:    f.UI.Multiple,
			Accept:      f.UI.Accept,
			MaxSize:     f.UI.MaxSize,
		}
	}

	// Restore item type info for lists
	if f.Type.Kind == ir.KindList && f.Type.ItemType != nil {
		field.ItemTypeName = f.Type.ItemType.Name
		if len(f.Type.InlineFields) > 0 {
			field.ItemFields = IRFieldsToNormalizer(f.Type.InlineFields)
		}
	}

	if f.Default != nil {
		switch v := f.Default.(type) {
		case string:
			field.Default = v
		}
	}

	// Extract DB metadata from attributes
	for _, attr := range f.Attributes {
		switch attr.Name {
		case "db":
			field.DB = normalizer.DBMeta{
				Type:       getStringArg(attr.Args, "type"),
				PrimaryKey: getBoolArg(attr.Args, "primary_key"),
				Unique:     getBoolArg(attr.Args, "unique"),
				Index:      getBoolArg(attr.Args, "index"),
			}
		case "validate":
			field.ValidateTag = getStringArg(attr.Args, "rule")
		case "env":
			for k := range attr.Args {
				field.EnvVar = k
				break
			}
		case "image", "file":
			field.FileMeta = &normalizer.FileMeta{
				Kind:      getStringArg(attr.Args, "kind"),
				Thumbnail: getBoolArg(attr.Args, "thumbnail"),
			}
			if attr.Name == "image" {
				field.FileMeta.Kind = "image"
				field.FileMeta.Thumbnail = true
			}
		}
	}

	// Also check metadata for processed attributes
	if field.Metadata != nil {
		if v, ok := field.Metadata["validate_tag"].(string); ok && field.ValidateTag == "" {
			field.ValidateTag = v
		}
		if v, ok := field.Metadata["sql_type"].(string); ok && field.DB.Type == "" {
			field.DB.Type = v
		}
		if v, ok := field.Metadata["primary_key"].(bool); ok && v {
			field.DB.PrimaryKey = true
		}
		if v, ok := field.Metadata["file_kind"].(string); ok {
			if field.FileMeta == nil {
				field.FileMeta = &normalizer.FileMeta{}
			}
			field.FileMeta.Kind = v
		}
		if v, ok := field.Metadata["generate_thumbnail"].(bool); ok {
			if field.FileMeta == nil {
				field.FileMeta = &normalizer.FileMeta{}
			}
			field.FileMeta.Thumbnail = v
		}
	}

	return field
}

// IRTypeRefToGoType converts IR TypeRef to Go type string.
func IRTypeRefToGoType(t ir.TypeRef) string {
	switch t.Kind {
	case ir.KindString:
		return "string"
	case ir.KindInt:
		return "int"
	case ir.KindInt64:
		return "int64"
	case ir.KindFloat:
		return "float64"
	case ir.KindBool:
		return "bool"
	case ir.KindTime:
		return "time.Time"
	case ir.KindUUID:
		return "string"
	case ir.KindJSON:
		return "json.RawMessage"
	case ir.KindAny:
		return "any"
	case ir.KindList:
		if t.ItemType != nil {
			// For inline struct types, use the name directly
			if t.ItemType.Name != "" && len(t.InlineFields) > 0 {
				return "[]" + t.ItemType.Name
			}
			return "[]" + IRTypeRefToGoType(*t.ItemType)
		}
		return "[]any"
	case ir.KindMap:
		keyType := "string"
		valType := "any"
		if t.KeyType != nil {
			keyType = IRTypeRefToGoType(*t.KeyType)
		}
		if t.ItemType != nil {
			valType = IRTypeRefToGoType(*t.ItemType)
		}
		return "map[" + keyType + "]" + valType
	case ir.KindEntity:
		if t.Name != "" {
			return "domain." + t.Name
		}
		return "any"
	case ir.KindEnum:
		return "string"
	case ir.KindFile:
		return "string"
	default:
		return "any"
	}
}

// IRServicesToNormalizer converts IR services to normalizer services.
func IRServicesToNormalizer(irServices []ir.Service) []normalizer.Service {
	result := make([]normalizer.Service, 0, len(irServices))
	for _, s := range irServices {
		svc := IRServiceToNormalizer(s)
		sort.Slice(svc.Methods, func(i, j int) bool { return svc.Methods[i].Name < svc.Methods[j].Name })
		result = append(result, svc)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

// IRServiceToNormalizer converts a single IR service.
func IRServiceToNormalizer(s ir.Service) normalizer.Service {
	svc := normalizer.Service{
		Name:          s.Name,
		Description:   s.Description,
		Publishes:     s.Publishes,
		Subscribes:    s.Subscribes,
		Uses:          s.Uses,
		Metadata:      s.Metadata,
		Source:        s.Source,
		RequiresSQL:   s.RequiresSQL,
		RequiresMongo: s.RequiresMongo,
		RequiresRedis: s.RequiresRedis,
		RequiresNats:  s.RequiresNats,
		RequiresS3:    s.RequiresS3,
	}

	for _, m := range s.Methods {
		svc.Methods = append(svc.Methods, IRMethodToNormalizer(m))
	}

	return svc
}

// IRMethodToNormalizer converts a single IR method.
func IRMethodToNormalizer(m ir.Method) normalizer.Method {
	method := normalizer.Method{
		Name:        m.Name,
		Description: m.Description,
		CacheTTL:    m.CacheTTL,
		CacheTags:   initializeSlice(m.CacheTags),
		Throws:      m.Throws,
		Publishes:   m.Publishes,
		Broadcasts:  m.Broadcasts,
		Idempotency: m.Idempotent,
		DedupeKey:   m.DedupeKey,
		Outbox:      m.Outbox,
		Metadata:    m.Metadata,
		Source:      m.Source,
	}

	if m.Input != nil {
		input := IREntityToNormalizer(*m.Input)
		method.Input = input
	}

	if m.Output != nil {
		output := IREntityToNormalizer(*m.Output)
		method.Output = output
	}

	for _, src := range m.Sources {
		method.Sources = append(method.Sources, normalizer.Source{
			Name:       src.Name,
			Kind:       src.Kind,
			Entity:     src.Entity,
			Collection: src.Collection,
			By:         src.Query,
			Metadata:   src.Metadata,
		})
	}

	if m.Pagination != nil {
		method.Pagination = &normalizer.PaginationDef{
			Type:         m.Pagination.Type,
			DefaultLimit: m.Pagination.DefaultLimit,
			MaxLimit:     m.Pagination.MaxLimit,
		}
	}

	if m.Impl != nil {
		method.Impl = &normalizer.MethodImpl{
			Lang:       m.Impl.Lang,
			Code:       m.Impl.Code,
			Imports:    m.Impl.Imports,
			RequiresTx: m.Impl.RequiresTx,
		}
	}

	method.Flow = irFlowStepsToNormalizer(m.Flow)

	return method
}

// irFlowStepsToNormalizer recursively converts IR flow steps to normalizer flow steps
func irFlowStepsToNormalizer(irSteps []ir.FlowStep) []normalizer.FlowStep {
	var result []normalizer.FlowStep
	for _, step := range irSteps {
		args := make(map[string]any)
		for k, v := range step.Args {
			args[k] = v
		}
		// Convert nested steps back to _do, _ifNew, _ifExists, _then, _else, _cases, _default
		if len(step.Steps) > 0 {
			args["_do"] = irFlowStepsToNormalizer(step.Steps)
		}
		if len(step.IfNew) > 0 {
			args["_ifNew"] = irFlowStepsToNormalizer(step.IfNew)
		}
		if len(step.IfExists) > 0 {
			args["_ifExists"] = irFlowStepsToNormalizer(step.IfExists)
		}
		if len(step.Then) > 0 {
			args["_then"] = irFlowStepsToNormalizer(step.Then)
		}
		if len(step.Else) > 0 {
			args["_else"] = irFlowStepsToNormalizer(step.Else)
		}
		if len(step.Default) > 0 {
			args["_default"] = irFlowStepsToNormalizer(step.Default)
		}
		if len(step.Cases) > 0 {
			cases := make(map[string][]normalizer.FlowStep, len(step.Cases))
			for label, branch := range step.Cases {
				cases[label] = irFlowStepsToNormalizer(branch)
			}
			args["_cases"] = cases
		}
		result = append(result, normalizer.FlowStep{
			Action: step.Action,
			Params: step.Params,
			Args:   args,
		})
	}
	return result
}

// IREndpointsToNormalizer converts IR endpoints.
func IREndpointsToNormalizer(irEndpoints []ir.Endpoint) []normalizer.Endpoint {
	result := make([]normalizer.Endpoint, 0, len(irEndpoints))
	for _, ep := range irEndpoints {
		endpoint := normalizer.Endpoint{
			Method:           ep.Method,
			Path:             ep.Path,
			ServiceName:      ep.Service,
			RPC:              ep.RPC,
			Description:      ep.Description,
			Messages:         ep.Messages,
			RoomParam:        ep.RoomParam,
			CacheTTL:         ep.Cache,
			CacheTags:        initializeSlice(ep.CacheTags),
			Invalidate:       initializeSlice(ep.Invalidate),
			OptimisticUpdate: ep.OptimisticUpdate,
			Timeout:          ep.Timeout,
			MaxBodySize:      ep.MaxBodySize,
			Idempotency:      ep.Idempotent,
			DedupeKey:        ep.DedupeKey,
			Errors:           ep.Errors,
			View:             ep.View,
			Metadata:         ep.Metadata,
			Source:           ep.Source,
		}

		if ep.Auth != nil {
			endpoint.AuthType = ep.Auth.Type
			endpoint.Permission = ep.Auth.Permission
			endpoint.AuthRoles = ep.Auth.Roles
			endpoint.AuthCheck = ep.Auth.Check
			endpoint.AuthInject = ep.Auth.Inject
		}

		if ep.RateLimit != nil {
			endpoint.RateLimit = &normalizer.RateLimitDef{
				RPS:   ep.RateLimit.RPS,
				Burst: ep.RateLimit.Burst,
			}
		}

		if ep.CircuitBreaker != nil {
			endpoint.CircuitBreaker = &normalizer.CircuitBreakerDef{
				Threshold:   ep.CircuitBreaker.Threshold,
				Timeout:     ep.CircuitBreaker.Timeout,
				HalfOpenMax: ep.CircuitBreaker.HalfOpenMax,
			}
		}
		if ep.Retry != nil {
			endpoint.RetryPolicy = &normalizer.RetryPolicyDef{
				Enabled:            ep.Retry.Enabled,
				MaxAttempts:        ep.Retry.MaxAttempts,
				BaseDelayMS:        ep.Retry.BaseDelayMS,
				RetryOnStatuses:    initializeIntSlice(ep.Retry.RetryOnStatuses),
				RetryNetworkErrors: ep.Retry.RetryNetworkErrors,
			}
		}

		if ep.Pagination != nil {
			endpoint.Pagination = &normalizer.PaginationDef{
				Type:         ep.Pagination.Type,
				DefaultLimit: ep.Pagination.DefaultLimit,
				MaxLimit:     ep.Pagination.MaxLimit,
			}
		}

		if ep.SLO != nil {
			endpoint.SLO = normalizer.SLODef{
				Latency: ep.SLO.Latency,
				Success: ep.SLO.Success,
			}
		}

		if ep.TestHints != nil {
			endpoint.TestHints = &normalizer.TestHints{
				HappyPath:  ep.TestHints.HappyPath,
				ErrorCases: initializeSlice(ep.TestHints.ErrorCases),
			}
		}

		result = append(result, endpoint)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ServiceName != result[j].ServiceName {
			return result[i].ServiceName < result[j].ServiceName
		}
		if result[i].Path != result[j].Path {
			return result[i].Path < result[j].Path
		}
		if result[i].Method != result[j].Method {
			return result[i].Method < result[j].Method
		}
		return result[i].RPC < result[j].RPC
	})
	return result
}

// IRReposToNormalizer converts IR repositories.
func IRReposToNormalizer(irRepos []ir.Repository) []normalizer.Repository {
	result := make([]normalizer.Repository, 0, len(irRepos))
	for _, r := range irRepos {
		repo := normalizer.Repository{
			Name:   r.Name,
			Entity: r.Entity,
			Source: r.Source,
		}

		for _, f := range r.Finders {
			finder := normalizer.RepositoryFinder{
				Name:       f.Name,
				Action:     f.Action,
				Returns:    f.Returns,
				ReturnType: f.ReturnType,
				Select:     f.Select,
				ScanFields: f.ScanFields,
				OrderBy:    f.OrderBy,
				Limit:      f.Limit,
				ForUpdate:  f.ForUpdate,
				CustomSQL:  f.CustomSQL,
				Source:     f.Source,
			}

			for _, w := range f.Where {
				finder.Where = append(finder.Where, normalizer.FinderWhere{
					Field:     w.Field,
					Op:        w.Op,
					Param:     w.Param,
					ParamType: w.ParamType,
				})
			}

			repo.Finders = append(repo.Finders, finder)
		}
		sort.Slice(repo.Finders, func(i, j int) bool { return repo.Finders[i].Name < repo.Finders[j].Name })

		result = append(result, repo)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

// IREventsToNormalizer converts IR events.
func IREventsToNormalizer(irEvents []ir.Event) []normalizer.EventDef {
	result := make([]normalizer.EventDef, 0, len(irEvents))
	for _, ev := range irEvents {
		result = append(result, normalizer.EventDef{
			Name:     ev.Name,
			Fields:   IRFieldsToNormalizer(ev.Fields),
			Metadata: ev.Metadata,
			Source:   ev.Source,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

// IRErrorsToNormalizer converts IR errors.
func IRErrorsToNormalizer(irErrors []ir.Error) []normalizer.ErrorDef {
	result := make([]normalizer.ErrorDef, 0, len(irErrors))
	for _, e := range irErrors {
		result = append(result, normalizer.ErrorDef{
			Name:       e.Name,
			Code:       e.Code,
			HTTPStatus: e.HTTPStatus,
			Message:    e.Message,
			Source:     e.Source,
		})
	}
	return result
}

// IRSchedulesToNormalizer converts IR schedules.
func IRSchedulesToNormalizer(irSchedules []ir.Schedule) []normalizer.ScheduleDef {
	result := make([]normalizer.ScheduleDef, 0, len(irSchedules))
	for _, s := range irSchedules {
		sched := normalizer.ScheduleDef{
			Name:    s.Name,
			Service: s.Service,
			Action:  s.Action,
			At:      s.At,
			Every:   s.Every,
			Publish: s.Publish,
		}

		for _, p := range s.Payload {
			sched.Payload = append(sched.Payload, normalizer.SchedulePayloadField{
				Name:  p.Name,
				Type:  IRTypeRefToGoType(p.Type),
				Value: "",
			})
		}

		result = append(result, sched)
	}
	return result
}

// Helper functions
func getStringArg(args map[string]any, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

func getBoolArg(args map[string]any, key string) bool {
	if v, ok := args[key].(bool); ok {
		return v
	}
	return false
}

func initializeSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func initializeIntSlice(s []int) []int {
	if s == nil {
		return []int{}
	}
	return s
}
