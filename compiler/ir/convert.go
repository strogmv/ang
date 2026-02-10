// Package ir provides conversion from legacy normalizer types to new IR.
// This allows gradual migration without breaking existing code.
package ir

import (
	"fmt"
	"github.com/strogmv/ang/compiler/normalizer"
	"strings"
)

// ConvertFromNormalizer converts legacy normalizer types to new IR.
// This is a bridge for gradual migration.
func ConvertFromNormalizer(
	entities []normalizer.Entity,
	services []normalizer.Service,
	events []normalizer.EventDef,
	errors []normalizer.ErrorDef,
	endpoints []normalizer.Endpoint,
	repos []normalizer.Repository,
	config normalizer.ConfigDef,
	auth *normalizer.AuthDef,
	rbac *normalizer.RBACDef,
	schedules []normalizer.ScheduleDef,
	views []normalizer.ViewDef,
	project normalizer.ProjectDef,
) *Schema {
	schema := &Schema{
		IRVersion: IRVersionV1,
		Project: Project{
			Name:    project.Name,
			Version: project.Version,
			Target: Target{
				Lang:      "go",       // Default, should come from CUE
				Framework: "chi",      // Default
				DB:        "postgres", // Default
			},
		},
		Metadata: make(map[string]any),
	}

	// Convert entities
	for _, e := range entities {
		schema.Entities = append(schema.Entities, ConvertEntity(e))
	}

	// Convert services
	for _, s := range services {
		schema.Services = append(schema.Services, ConvertService(s))
	}

	// Convert events
	for _, ev := range events {
		schema.Events = append(schema.Events, ConvertEvent(ev))
	}

	// Convert errors
	for _, er := range errors {
		schema.Errors = append(schema.Errors, Error{
			Name:       er.Name,
			Code:       er.Code,
			HTTPStatus: er.HTTPStatus,
			Message:    er.Message,
			Source:     er.Source,
		})
	}

	// Convert endpoints
	for _, ep := range endpoints {
		schema.Endpoints = append(schema.Endpoints, ConvertEndpoint(ep))
	}

	// Convert repositories
	for _, r := range repos {
		schema.Repos = append(schema.Repos, ConvertRepository(r))
	}

	// Convert config
	schema.Config = Config{
		Fields: ConvertFields(config.Fields),
	}

	// Convert auth
	if auth != nil {
		schema.Auth = ConvertAuth(auth)
	}

	// Convert RBAC
	if rbac != nil {
		schema.RBAC = &RBAC{
			Roles:       rbac.Roles,
			Permissions: rbac.Permissions,
		}
	}

	// Convert schedules
	for _, s := range schedules {
		schema.Schedules = append(schema.Schedules, ConvertSchedule(s))
	}

	// Convert views
	for _, v := range views {
		schema.Views = append(schema.Views, View{
			Name:  v.Name,
			Roles: v.Roles,
		})
	}

	schema.Graph = BuildDependencyGraph(schema)

	return schema
}

func BuildDependencyGraph(s *Schema) *DependencyGraph {
	g := &DependencyGraph{}

	// Add Nodes
	for _, e := range s.Entities {
		g.Nodes = append(g.Nodes, Node{ID: "cue://#" + e.Name, Kind: "entity", Name: e.Name})
	}
	for _, svc := range s.Services {
		g.Nodes = append(g.Nodes, Node{ID: "svc://" + svc.Name, Kind: "service", Name: svc.Name})
		for _, m := range svc.Methods {
			mID := fmt.Sprintf("method://%s.%s", svc.Name, m.Name)
			g.Nodes = append(g.Nodes, Node{ID: mID, Kind: "method", Name: m.Name})
			g.Edges = append(g.Edges, Edge{From: "svc://" + svc.Name, To: mID, Type: "has"})

			// Links to entities
			if m.Input != nil {
				g.Edges = append(g.Edges, Edge{From: mID, To: "cue://#" + m.Input.Name, Type: "reads"})
			}
			if m.Output != nil {
				g.Edges = append(g.Edges, Edge{From: mID, To: "cue://#" + m.Output.Name, Type: "writes"})
			}
		}
	}

	return g
}

func ConvertEntity(e normalizer.Entity) Entity {
	entity := Entity{
		Name:        e.Name,
		Description: e.Description,
		Owner:       e.Owner,
		Fields:      ConvertFields(e.Fields),
		Metadata:    e.Metadata,
		Source:      e.Source,
	}

	if e.UI != nil && e.UI.CRUD != nil {
		entity.UI.CRUD = &CRUDConfig{
			Enabled: e.UI.CRUD.Enabled,
			Custom:  e.UI.CRUD.Custom,
			Views:   e.UI.CRUD.Views,
			Perms:   e.UI.CRUD.Perms,
		}
	}

	if e.FSM != nil {
		entity.FSM = &FSM{
			Field:       e.FSM.Field,
			States:      e.FSM.States,
			Transitions: e.FSM.Transitions,
		}
	}

	for _, idx := range e.Indexes {
		entity.Indexes = append(entity.Indexes, Index{
			Fields: idx.Fields,
			Unique: idx.Unique,
		})
	}

	return entity
}

func ConvertFields(fields []normalizer.Field) []Field {
	var result []Field
	for _, f := range fields {
		result = append(result, ConvertField(f))
	}
	return result
}

func ConvertField(f normalizer.Field) Field {
	field := Field{
		Name:        f.Name,
		Type:        inferTypeRef(f),
		Optional:    f.IsOptional,
		IsSecret:    f.IsSecret,
		IsPII:       f.IsPII,
		SkipDomain:  f.SkipDomain,
		ValidateTag: f.ValidateTag,
		Constraints: convertConstraints(f.Constraints),
		EnvVar:      f.EnvVar,
		Metadata:    f.Metadata,
		Source:      f.Source,
	}

	if f.UI != nil {
		field.UI = FieldUI{
			Type:        f.UI.Type,
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

	if f.Default != "" {
		field.Default = f.Default
	}

	// Convert DB attributes
	if f.DB.Type != "" || f.DB.PrimaryKey || f.DB.Unique || f.DB.Index {
		field.Attributes = append(field.Attributes, Attribute{
			Name: "db",
			Args: map[string]any{
				"type":        f.DB.Type,
				"primary_key": f.DB.PrimaryKey,
				"unique":      f.DB.Unique,
				"index":       f.DB.Index,
			},
		})
	}

	// Convert validation
	if f.ValidateTag != "" {
		field.Attributes = append(field.Attributes, Attribute{
			Name: "validate",
			Args: map[string]any{
				"rule": f.ValidateTag,
			},
		})
	}

	// Convert env
	if f.EnvVar != "" {
		field.Attributes = append(field.Attributes, Attribute{
			Name: "env",
			Args: map[string]any{
				f.EnvVar: true,
			},
		})
	}

	// Convert file metadata
	if f.FileMeta != nil {
		attrName := "file"
		if f.FileMeta.Kind == "image" {
			attrName = "image"
		}
		field.Attributes = append(field.Attributes, Attribute{
			Name: attrName,
			Args: map[string]any{
				"kind":      f.FileMeta.Kind,
				"thumbnail": f.FileMeta.Thumbnail,
			},
		})
	}

	return field
}

func inferTypeRef(f normalizer.Field) TypeRef {
	// Parse the Go type string and convert to TypeRef
	goType := strings.TrimSpace(f.Type)

	// Handle list types
	if f.IsList || strings.HasPrefix(goType, "[]") {
		itemType := goType
		if len(goType) > 2 && goType[:2] == "[]" {
			itemType = goType[2:]
		}
		ref := TypeRef{
			Kind:     KindList,
			ItemType: &TypeRef{Kind: inferKind(itemType), Name: cleanTypeName(itemType)},
		}

		// Preserve item type info for lists
		if f.ItemTypeName != "" {
			ref.ItemType.Name = f.ItemTypeName
			if len(f.ItemFields) > 0 {
				ref.InlineFields = ConvertFields(f.ItemFields)
			}
		}

		return ref
	}

	kind := inferKind(goType)
	ref := TypeRef{Kind: kind}

	if kind == KindEntity {
		ref.Name = cleanTypeName(goType)
	}

	return ref
}

func inferKind(goType string) TypeKind {
	switch goType {
	case "string":
		return KindString
	case "int":
		return KindInt
	case "int64":
		return KindInt64
	case "float64":
		return KindFloat
	case "bool":
		return KindBool
	case "time.Time":
		return KindTime
	case "json.RawMessage":
		return KindJSON
	case "any", "interface{}":
		return KindAny
	default:
		// If starts with "domain." it's an entity reference
		if len(goType) > 7 && goType[:7] == "domain." {
			return KindEntity
		}
		// If starts with "[]domain." it's a list of entities
		if len(goType) > 9 && goType[:9] == "[]domain." {
			return KindList
		}
		return KindAny
	}
}

func cleanTypeName(goType string) string {
	// Remove "domain." prefix
	if len(goType) > 7 && goType[:7] == "domain." {
		return goType[7:]
	}
	// Remove "[]domain." prefix
	if len(goType) > 9 && goType[:9] == "[]domain." {
		return goType[9:]
	}
	return goType
}

func ConvertService(s normalizer.Service) Service {
	svc := Service{
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
		svc.Methods = append(svc.Methods, ConvertMethod(m))
	}

	return svc
}

func ConvertMethod(m normalizer.Method) Method {
	method := Method{
		Name:        m.Name,
		Description: m.Description,
		CacheTTL:    m.CacheTTL,
		CacheTags:   initializeSlice(m.CacheTags),
		Throws:      m.Throws,
		Publishes:   m.Publishes,
		Broadcasts:  m.Broadcasts,
		Idempotent:  m.Idempotency,
		DedupeKey:   m.DedupeKey,
		Outbox:      m.Outbox,
		Metadata:    m.Metadata,
		Source:      m.Source,
	}

	// Convert input
	inputEntity := ConvertEntity(m.Input)
	method.Input = &inputEntity

	// Convert output
	outputEntity := ConvertEntity(m.Output)
	method.Output = &outputEntity

	// Convert sources
	for _, src := range m.Sources {
		method.Sources = append(method.Sources, Source{
			Name:       src.Name,
			Kind:       src.Kind,
			Entity:     src.Entity,
			Collection: src.Collection,
			Query:      src.By,
			Metadata:   src.Metadata,
		})
	}

	// Convert pagination
	if m.Pagination != nil {
		method.Pagination = &Pagination{
			Type:         m.Pagination.Type,
			DefaultLimit: m.Pagination.DefaultLimit,
			MaxLimit:     m.Pagination.MaxLimit,
		}
	}

	// Convert impl
	if m.Impl != nil {
		method.Impl = &Impl{
			Lang:       m.Impl.Lang,
			Code:       m.Impl.Code,
			Imports:    m.Impl.Imports,
			RequiresTx: m.Impl.RequiresTx,
		}
	}

	// RECURSIVE FLOW CONVERSION
	method.Flow = ConvertFlowSteps(m.Flow)
	method.Attributes = ConvertAttributes(m.Attributes)

	return method
}

func ConvertEvent(e normalizer.EventDef) Event {
	return Event{
		Name:     e.Name,
		Fields:   ConvertFields(e.Fields),
		Metadata: e.Metadata,
		Source:   e.Source,
	}
}

func ConvertEndpoint(ep normalizer.Endpoint) Endpoint {
	endpoint := Endpoint{
		Method:           ep.Method,
		Path:             ep.Path,
		Service:          ep.ServiceName,
		RPC:              ep.RPC,
		Description:      ep.Description,
		Messages:         ep.Messages,
		RoomParam:        ep.RoomParam,
		Cache:            ep.CacheTTL,
		CacheTags:        initializeSlice(ep.CacheTags),
		Invalidate:       initializeSlice(ep.Invalidate),
		OptimisticUpdate: ep.OptimisticUpdate,
		Timeout:          ep.Timeout,
		MaxBodySize:      ep.MaxBodySize,
		Idempotent:       ep.Idempotency,
		DedupeKey:        ep.DedupeKey,
		Errors:           ep.Errors,
		View:             ep.View,
		Metadata:         ep.Metadata,
		Source:           ep.Source,
	}

	// Convert auth
	if ep.AuthType != "" || ep.Permission != "" || len(ep.AuthRoles) > 0 {
		endpoint.Auth = &EndpointAuth{
			Type:       ep.AuthType,
			Permission: ep.Permission,
			Roles:      ep.AuthRoles,
			Check:      ep.AuthCheck,
			Inject:     ep.AuthInject,
		}
	}

	// Convert rate limit
	if ep.RateLimit != nil {
		endpoint.RateLimit = &RateLimit{
			RPS:   ep.RateLimit.RPS,
			Burst: ep.RateLimit.Burst,
		}
	}

	// Convert circuit breaker
	if ep.CircuitBreaker != nil {
		endpoint.CircuitBreaker = &CircuitBreaker{
			Threshold:   ep.CircuitBreaker.Threshold,
			Timeout:     ep.CircuitBreaker.Timeout,
			HalfOpenMax: ep.CircuitBreaker.HalfOpenMax,
		}
	}

	// Convert pagination
	if ep.Pagination != nil {
		endpoint.Pagination = &Pagination{
			Type:         ep.Pagination.Type,
			DefaultLimit: ep.Pagination.DefaultLimit,
			MaxLimit:     ep.Pagination.MaxLimit,
		}
	}

	// Convert SLO
	if ep.SLO.Latency != "" || ep.SLO.Success != "" {
		endpoint.SLO = &SLO{
			Latency: ep.SLO.Latency,
			Success: ep.SLO.Success,
		}
	}

	if ep.TestHints != nil {
		endpoint.TestHints = &TestHints{
			HappyPath:  ep.TestHints.HappyPath,
			ErrorCases: initializeSlice(ep.TestHints.ErrorCases),
		}
	}

	return endpoint
}

func ConvertRepository(r normalizer.Repository) Repository {
	repo := Repository{
		Name:   r.Name,
		Entity: r.Entity,
		Source: r.Source,
	}

	for _, f := range r.Finders {
		repo.Finders = append(repo.Finders, Finder{
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
			Where:      ConvertWhereClauses(f.Where),
			Source:     f.Source,
		})
	}

	return repo
}

func ConvertWhereClauses(clauses []normalizer.FinderWhere) []WhereClause {
	var result []WhereClause
	for _, c := range clauses {
		result = append(result, WhereClause{
			Field:     c.Field,
			Op:        c.Op,
			Param:     c.Param,
			ParamType: c.ParamType,
		})
	}
	return result
}

func ConvertAuth(a *normalizer.AuthDef) *Auth {
	return &Auth{
		Algorithm:    a.Alg,
		Issuer:       a.Issuer,
		Audience:     a.Audience,
		AccessTTL:    a.AccessTTL,
		RefreshTTL:   a.RefreshTTL,
		Rotation:     a.Rotation,
		RefreshStore: a.RefreshStore,
		Claims: AuthClaims{
			UserID:      a.UserIDClaim,
			CompanyID:   a.CompanyIDClaim,
			Roles:       a.RolesClaim,
			Permissions: a.PermissionsClaim,
		},
		Operations: AuthOps{
			Service:             a.Service,
			LoginOp:             a.LoginOp,
			LoginAccessField:    a.LoginAccessField,
			LoginRefreshField:   a.LoginRefreshField,
			RefreshOp:           a.RefreshOp,
			RefreshTokenField:   a.RefreshTokenField,
			RefreshAccessField:  a.RefreshAccessField,
			RefreshRefreshField: a.RefreshRefreshField,
			LogoutOp:            a.LogoutOp,
			LogoutTokenField:    a.LogoutTokenField,
		},
	}
}

func ConvertSchedule(s normalizer.ScheduleDef) Schedule {
	schedule := Schedule{
		Name:    s.Name,
		Service: s.Service,
		Action:  s.Action,
		At:      s.At,
		Every:   s.Every,
		Publish: s.Publish,
	}

	for _, p := range s.Payload {
		schedule.Payload = append(schedule.Payload, Field{
			Name:    p.Name,
			Type:    TypeRef{Kind: inferKind(p.Type)},
			Default: p.Value,
		})
	}

	return schedule
}

func ConvertFlowSteps(source []normalizer.FlowStep) []FlowStep {
	var steps []FlowStep
	for _, step := range source {
		args := make(map[string]any)
		var thenSteps, elseSteps, doSteps []FlowStep
		for k, v := range step.Args {
			if k == "_then" {
				if nested, ok := v.([]normalizer.FlowStep); ok {
					thenSteps = ConvertFlowSteps(nested)
				}
				continue
			}
			if k == "_else" {
				if nested, ok := v.([]normalizer.FlowStep); ok {
					elseSteps = ConvertFlowSteps(nested)
				}
				continue
			}
			if k == "_do" {
				if nested, ok := v.([]normalizer.FlowStep); ok {
					doSteps = ConvertFlowSteps(nested)
				}
				continue
			}
			args[k] = v
		}
		steps = append(steps, FlowStep{
			Action:     step.Action,
			Params:     step.Params,
			Args:       args,
			Steps:      doSteps,
			Then:       thenSteps,
			Else:       elseSteps,
			Attributes: ConvertAttributes(step.Attributes),
		})
	}
	return steps
}

func ConvertAttributes(attrs []normalizer.Attribute) []Attribute {
	var result []Attribute
	for _, a := range attrs {
		result = append(result, Attribute{
			Name: a.Name,
			Args: a.Args,
		})
	}
	return result
}

func initializeSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func convertConstraints(c *normalizer.Constraints) *Constraints {
	if c == nil {
		return nil
	}
	return &Constraints{
		Min:    c.Min,
		Max:    c.Max,
		MinLen: c.MinLen,
		MaxLen: c.MaxLen,
		Regex:  c.Regex,
		Enum:   c.Enum,
	}
}
