// Package ir defines language-agnostic Intermediate Representation.
// This IR knows nothing about Go, Rust, or any specific framework.
// It's a pure architectural description that can be transformed to any target.
package ir

// Schema is the root of the IR tree.
// It contains everything needed to generate a complete application.
type Schema struct {
	IRVersion string `json:"ir_version"`

	Project    Project      `json:"project"`
	Entities   []Entity     `json:"entities"`
	Services   []Service    `json:"services"`
	Events     []Event      `json:"events"`
	Errors     []Error      `json:"errors"`
	Endpoints  []Endpoint   `json:"endpoints"`
	Scopes     []Scope      `json:"scopes"`
	Repos      []Repository `json:"repos"`
	Config     Config       `json:"config"`
	Auth       *Auth        `json:"auth,omitempty"`
	RBAC       *RBAC        `json:"rbac,omitempty"`
	Schedules  []Schedule   `json:"schedules"`
	Views      []View       `json:"views"`
	Templates  []Template   `json:"templates,omitempty"`
	Provenance *Provenance  `json:"provenance,omitempty"`
	// Notifications stores declarative notification routing configuration
	// normalized from cue/infra blocks.
	Notifications *NotificationsConfig `json:"notifications,omitempty"`

	// Dependency Graph for Impact Analysis
	Graph *DependencyGraph `json:"graph,omitempty"`

	// Metadata for transformers to store computed data
	Metadata map[string]any `json:"metadata"`
}

type OriginKind string

const (
	OriginExplicit  OriginKind = "explicit"
	OriginMigrated  OriginKind = "migrated"
	OriginDefaulted OriginKind = "defaulted"
	OriginEnriched  OriginKind = "enriched"
)

type Provenance struct {
	Origin OriginKind `json:"origin"`
	From   string     `json:"from,omitempty"`
	Detail string     `json:"detail,omitempty"`
}

// NotificationsConfig describes multi-channel routing and policy rules.
type NotificationsConfig struct {
	Channels   *NotificationChannels `json:"channels,omitempty"`
	Policies   *NotificationPolicies `json:"policies,omitempty"`
	Provenance *Provenance           `json:"provenance,omitempty"`
}

type NotificationChannels struct {
	Enabled         bool                               `json:"enabled"`
	DefaultChannels []string                           `json:"default_channels,omitempty"`
	Channels        map[string]NotificationChannelSpec `json:"channels,omitempty"`
}

type NotificationChannelSpec struct {
	Enabled    bool   `json:"enabled"`
	Driver     string `json:"driver,omitempty"`
	Topic      string `json:"topic,omitempty"`
	Subject    string `json:"subject,omitempty"`
	Template   string `json:"template,omitempty"`
	DSNEnv     string `json:"dsn_env,omitempty"`
	BrokersEnv string `json:"brokers_env,omitempty"`
}

type NotificationPolicies struct {
	Enabled bool                     `json:"enabled"`
	Rules   []NotificationPolicyRule `json:"rules,omitempty"`
}

type NotificationPolicyRule struct {
	Enabled  bool     `json:"enabled"`
	Event    string   `json:"event,omitempty"`
	Type     string   `json:"type,omitempty"`
	Audience string   `json:"audience,omitempty"`
	Channels []string `json:"channels,omitempty"`
	Template string   `json:"template,omitempty"`
	MuteKey  string   `json:"mute_key,omitempty"`
}

// Template is a channel-agnostic template catalog item.
type Template struct {
	ID           string   `json:"id"`
	Kind         string   `json:"kind,omitempty"`
	Channel      string   `json:"channel,omitempty"`
	Locale       string   `json:"locale,omitempty"`
	Version      string   `json:"version,omitempty"`
	Engine       string   `json:"engine,omitempty"`
	Subject      string   `json:"subject,omitempty"`
	Text         string   `json:"text,omitempty"`
	HTML         string   `json:"html,omitempty"`
	Body         string   `json:"body,omitempty"`
	RequiredVars []string `json:"required_vars,omitempty"`
	OptionalVars []string `json:"optional_vars,omitempty"`
	Provenance   *Provenance `json:"provenance,omitempty"`
}

type DependencyGraph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

type Node struct {
	ID   string `json:"id"`   // e.g. "cue://#User", "go://internal/domain/user.go"
	Kind string `json:"kind"` // cue_def, service, method, file, table, etc.
	Name string `json:"name"`
}

type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"` // generates, uses, calls, writes, reads
}

// Project contains project-level metadata.
type Project struct {
	Name       string      `json:"name"`
	Version    string      `json:"version"`
	Target     Target      `json:"target"`
	Provenance *Provenance `json:"provenance,omitempty"`
}

// Target describes the generation target.
type Target struct {
	Lang      string `json:"lang"`      // "go", "rust", "typescript"
	Framework string `json:"framework"` // "chi", "echo", "fiber", "axum"
	DB        string `json:"db"`        // "postgres", "mysql", "mongodb"
	Cache     string `json:"cache"`     // "redis", "memcached"
	Queue     string `json:"queue"`     // "nats", "kafka", "rabbitmq"
	Storage   string `json:"storage"`   // "s3", "gcs", "minio"
	Provenance *Provenance `json:"provenance,omitempty"`
}

// Entity represents a domain entity (aggregate, value object, etc.)
type Entity struct {
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	Owner          string         `json:"owner"` // Service name that owns this entity
	BoundedContext string         `json:"bounded_context"`
	AggregateRoot  bool           `json:"aggregate_root"`
	Owns           []string       `json:"owns"`
	ReadModel      *ReadModel     `json:"read_model,omitempty"`
	Fields         []Field        `json:"fields"`
	FSM            *FSM           `json:"fsm,omitempty"`
	Indexes        []Index        `json:"indexes"`
	UI             EntityUI       `json:"ui"`
	Metadata       map[string]any `json:"metadata"`
	Source         string         `json:"source"`
	Provenance     *Provenance    `json:"provenance,omitempty"`
}

type ReadModel struct {
	SourceContext string   `json:"source_context"`
	RefreshOn     []string `json:"refresh_on"`
}

type EntityUI struct {
	CRUD *CRUDConfig `json:"crud,omitempty"`
}

type CRUDConfig struct {
	Enabled bool              `json:"enabled"`
	Custom  bool              `json:"custom"`
	Views   map[string]bool   `json:"views"`
	Perms   map[string]string `json:"perms"`
}

// Field represents a field in an entity or DTO.
type Field struct {
	Name        string         `json:"name"`
	Type        TypeRef        `json:"type"`
	Optional    bool           `json:"optional"`
	Default     any            `json:"default,omitempty"`
	IsSecret    bool           `json:"is_secret"`
	IsPII       bool           `json:"is_pii"`
	SkipDomain  bool           `json:"skip_domain"`           // Field is for DTO/UI only, skip in domain models
	ValidateTag string         `json:"validate_tag"`          // @validate tag content
	Constraints *Constraints   `json:"constraints,omitempty"` // Structured constraints from CUE
	EnvVar      string         `json:"env_var"`               // @env tag content
	Attributes  []Attribute    `json:"attributes"`
	UI          FieldUI        `json:"ui"`
	Metadata    map[string]any `json:"metadata"`
	Source      string         `json:"source"`
	Provenance  *Provenance    `json:"provenance,omitempty"`
}

type Constraints struct {
	Min    *float64 `json:"min,omitempty"`
	Max    *float64 `json:"max,omitempty"`
	MinLen *int     `json:"min_len,omitempty"`
	MaxLen *int     `json:"max_len,omitempty"`
	Regex  string   `json:"regex,omitempty"`
	Enum   []string `json:"enum,omitempty"`
}

type FieldUI struct {
	Type        string   `json:"type"`
	Importance  string   `json:"importance"`
	InputKind   string   `json:"input_kind"`
	Intent      string   `json:"intent"`
	Density     string   `json:"density"`
	LabelMode   string   `json:"label_mode"`
	Surface     string   `json:"surface"`
	Component   string   `json:"component"`
	Section     string   `json:"section"`
	Columns     int      `json:"columns"`
	Label       string   `json:"label"`
	Placeholder string   `json:"placeholder"`
	HelperText  string   `json:"helper_text"`
	Order       int      `json:"order"`
	Hidden      bool     `json:"hidden"`
	Disabled    bool     `json:"disabled"`
	FullWidth   bool     `json:"full_width"`
	Rows        int      `json:"rows"`           // For textarea
	Min         *float64 `json:"min,omitempty"`  // For number/currency
	Max         *float64 `json:"max,omitempty"`  // For number/currency
	Step        *float64 `json:"step,omitempty"` // For number
	Currency    string   `json:"currency"`       // For currency
	Source      string   `json:"source"`         // Data source (e.g. "Service.Method")
	Options     []string `json:"options"`        // Static options
	Multiple    bool     `json:"multiple"`       // For select/autocomplete
	Accept      string   `json:"accept"`         // For file types
	MaxSize     int      `json:"max_size"`       // For file size
}

// TypeRef is a language-agnostic type reference.
// It describes what kind of data this is, not how it's implemented.
type TypeRef struct {
	Kind         TypeKind `json:"kind"`
	Name         string   `json:"name"`                    // For entity references: "User", "Order"
	ItemType     *TypeRef `json:"item_type,omitempty"`     // For List/Map: the element type
	KeyType      *TypeRef `json:"key_type,omitempty"`      // For Map: the key type
	InlineFields []Field  `json:"inline_fields,omitempty"` // For inline struct definitions in lists
}

// TypeKind represents the fundamental data kinds.
type TypeKind string

const (
	KindString TypeKind = "string"
	KindInt    TypeKind = "int"
	KindInt64  TypeKind = "int64"
	KindFloat  TypeKind = "float"
	KindBool   TypeKind = "bool"
	KindTime   TypeKind = "time"
	KindUUID   TypeKind = "uuid"
	KindJSON   TypeKind = "json"
	KindList   TypeKind = "list"
	KindMap    TypeKind = "map"
	KindEntity TypeKind = "entity" // Reference to another entity
	KindEnum   TypeKind = "enum"
	KindFile   TypeKind = "file"
	KindAny    TypeKind = "any"
)

// Attribute represents a CUE attribute like @db, @validate, @image.
type Attribute struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

// FSM describes a Finite State Machine for status fields.
type FSM struct {
	Field       string              `json:"field"`
	States      []string            `json:"states"`
	Transitions map[string][]string `json:"transitions"` // from -> []to
}

// Index describes a database index.
type Index struct {
	Fields []string `json:"fields"`
	Unique bool     `json:"unique"`
	Name   string   `json:"name"`
}

// Service represents a service/port interface.
type Service struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Methods     []Method          `json:"methods"`
	Publishes   []string          `json:"publishes"`  // Events this service can publish
	Subscribes  map[string]string `json:"subscribes"` // event -> handler method
	Uses        []string          `json:"uses"`       // Service dependencies (by name)
	Metadata    map[string]any    `json:"metadata"`
	Source      string            `json:"source"`
	Provenance  *Provenance       `json:"provenance,omitempty"`

	// Infrastructure dependencies
	RequiresSQL   bool `json:"requires_sql"`
	RequiresMongo bool `json:"requires_mongo"`
	RequiresRedis bool `json:"requires_redis"`
	RequiresNats  bool `json:"requires_nats"`
	RequiresS3    bool `json:"requires_s3"`
}

type OperationKind string

const (
	OperationKindCreate     OperationKind = "create"
	OperationKindGet        OperationKind = "get"
	OperationKindUpdate     OperationKind = "update"
	OperationKindDelete     OperationKind = "delete"
	OperationKindList       OperationKind = "list"
	OperationKindTransition OperationKind = "transition"
	OperationKindNotify     OperationKind = "notify"
	OperationKindAuth       OperationKind = "auth"
	OperationKindMessage    OperationKind = "message"
	OperationKindUpload     OperationKind = "upload"
)

type CapabilityKind string

const (
	CapabilityAuth       CapabilityKind = "auth"
	CapabilityProfile    CapabilityKind = "profile"
	CapabilityMedia      CapabilityKind = "media"
	CapabilityNotify     CapabilityKind = "notify"
	CapabilityMessaging  CapabilityKind = "messaging"
	CapabilityModeration CapabilityKind = "moderation"
	CapabilitySearch     CapabilityKind = "search"
)

type SideEffect struct {
	Kind        string `json:"kind"`
	Channel     string `json:"channel,omitempty"`
	Event       string `json:"event,omitempty"`
	Template    string `json:"template,omitempty"`
	TargetField string `json:"target_field,omitempty"`
}

type PlannerRoute struct {
	Method string `json:"method,omitempty"`
	Path   string `json:"path,omitempty"`
}

type PlannerRepository struct {
	LoadMethod string `json:"load_method,omitempty"`
	ListMethod string `json:"list_method,omitempty"`
	ActorField string `json:"actor_field,omitempty"`
	InputField string `json:"input_field,omitempty"`
}

type PlannerHints struct {
	SourcePack string             `json:"source_pack,omitempty"`
	Route      *PlannerRoute      `json:"route,omitempty"`
	Repository *PlannerRepository `json:"repository,omitempty"`
}

// Method represents an RPC/service method.
type Method struct {
	Name                 string           `json:"name"`
	Description          string           `json:"description"`
	IsStreaming          bool             `json:"is_streaming,omitempty"`
	Input                *Entity          `json:"input,omitempty"`
	Output               *Entity          `json:"output,omitempty"`
	Sources              []Source         `json:"sources"`
	CacheTTL             string           `json:"cache_ttl"`
	CacheTags            []string         `json:"cache_tags"`
	Throws               []string         `json:"throws"`
	Publishes            []string         `json:"publishes"`
	Broadcasts           []string         `json:"broadcasts"`
	Pagination           *Pagination      `json:"pagination,omitempty"`
	Idempotent           bool             `json:"idempotent"`
	DedupeKey            string           `json:"dedupe_key"`
	Outbox               bool             `json:"outbox"`
	PrimaryOperationKind OperationKind    `json:"primary_operation_kind,omitempty"`
	Capabilities         []CapabilityKind `json:"capabilities,omitempty"`
	SideEffects          []SideEffect     `json:"side_effects,omitempty"`
	ManualRequired       bool             `json:"manual_required,omitempty"`
	Planner              *PlannerHints    `json:"planner,omitempty"`
	Impl                 *Impl            `json:"impl,omitempty"`
	ImplSteps            []ImplStep       `json:"impl_steps"`
	Flow                 []FlowStep       `json:"flow"`
	Attributes           []Attribute      `json:"attributes"`
	Metadata             map[string]any   `json:"metadata"`
	Source               string           `json:"source"`
	Provenance           *Provenance      `json:"provenance,omitempty"`
}

// FlowStep represents a declarative step in a method's logic.
type FlowStep struct {
	Action     string                `json:"action"`
	Condition  string                `json:"condition,omitempty"`
	Throw      string                `json:"throw,omitempty"`
	Input      string                `json:"input,omitempty"`
	Output     string                `json:"output,omitempty"`
	Value      string                `json:"value,omitempty"`
	Params     []string              `json:"params,omitempty"`
	Args       map[string]any        `json:"args,omitempty"`
	Steps      []FlowStep            `json:"steps,omitempty"`
	IfNew      []FlowStep            `json:"if_new,omitempty"`
	IfExists   []FlowStep            `json:"if_exists,omitempty"`
	Then       []FlowStep            `json:"then,omitempty"`
	Else       []FlowStep            `json:"else,omitempty"`
	Cases      map[string][]FlowStep `json:"cases,omitempty"`
	Default    []FlowStep            `json:"default,omitempty"`
	Attributes []Attribute           `json:"attributes,omitempty"`
}

// Source describes where method data comes from.
type Source struct {
	Name       string            `json:"name"`
	Kind       string            `json:"kind"` // "sql", "mongo", "cache", "external"
	Entity     string            `json:"entity"`
	Collection string            `json:"collection"`
	Query      map[string]string `json:"query"`
	Metadata   map[string]any    `json:"metadata"`
}

// Pagination describes pagination settings.
type Pagination struct {
	Type         string `json:"type"` // "cursor", "offset"
	DefaultLimit int    `json:"default_limit"`
	MaxLimit     int    `json:"max_limit"`
}

// Impl holds implementation code (for inline implementations).
type Impl struct {
	Lang       string   `json:"lang"`
	Code       string   `json:"code"`
	Imports    []string `json:"imports"`
	RequiresTx bool     `json:"requires_tx"`
}

// ImplStep represents typed implementation step from impl_steps DSL.
type ImplStep struct {
	Kind        string            `json:"kind"`
	LoadTarget  string            `json:"load_target,omitempty"`
	LoadBy      map[string]string `json:"load_by,omitempty"`
	LoadInto    string            `json:"load_into,omitempty"`
	AssertExpr  string            `json:"assert_expr,omitempty"`
	AssertError string            `json:"assert_error,omitempty"`
	CallTarget  string            `json:"call_target,omitempty"`
	CallArgs    map[string]any    `json:"call_args,omitempty"`
	CallInto    string            `json:"call_into,omitempty"`
	EmitEvent   string            `json:"emit_event,omitempty"`
	EmitPayload map[string]any    `json:"emit_payload,omitempty"`
	Source      string            `json:"source,omitempty"`
}

// Event represents a domain event.
type Event struct {
	Name      string         `json:"name"`
	Owner     string         `json:"owner"`
	Consumers []string       `json:"consumers"`
	Fields    []Field        `json:"fields"`
	Metadata  map[string]any `json:"metadata"`
	Source    string         `json:"source"`
	Provenance *Provenance   `json:"provenance,omitempty"`
}

// Error represents a business error.
type Error struct {
	Name       string `json:"name"`
	Code       int    `json:"code"`
	HTTPStatus int    `json:"http_status"`
	Message    string `json:"message"`
	Source     string `json:"source"`
}

// Endpoint represents an HTTP endpoint.
type Endpoint struct {
	Method           string          `json:"method"` // GET, POST, PUT, DELETE, WS
	Path             string          `json:"path"`
	IsStreaming      bool            `json:"is_streaming,omitempty"`
	Service          string          `json:"service"`
	RPC              string          `json:"rpc"`
	Description      string          `json:"description"`
	Messages         []string        `json:"messages"`
	RoomParam        string          `json:"room_param"`
	Auth             *EndpointAuth   `json:"auth,omitempty"`
	RequiredScopes   []string        `json:"required_scopes,omitempty"`
	Cache            string          `json:"cache"`
	CacheTags        []string        `json:"cache_tags"`
	Invalidate       []string        `json:"invalidate"`
	OptimisticUpdate string          `json:"optimistic_update"`
	RateLimit        *RateLimit      `json:"rate_limit,omitempty"`
	CircuitBreaker   *CircuitBreaker `json:"circuit_breaker,omitempty"`
	Retry            *RetryPolicy    `json:"retry,omitempty"`
	Timeout          string          `json:"timeout"`        // Request timeout (e.g. "5s", "30s")
	MaxBodySize      int64           `json:"max_body_size"`  // Request body size limit in bytes
	MaxConcurrent    int             `json:"max_concurrent"` // max simultaneous in-flight requests; 0 = unlimited
	Coalesce         bool            `json:"coalesce"`       // deduplicate identical in-flight GET requests via singleflight
	Idempotent       bool            `json:"idempotent"`
	DedupeKey        string          `json:"dedupe_key"`
	Errors           []string        `json:"errors"`
	Pagination       *Pagination     `json:"pagination,omitempty"`
	View             string          `json:"view"`
	SLO              *SLO            `json:"slo,omitempty"`
	TestHints        *TestHints      `json:"test_hints,omitempty"`
	Metadata         map[string]any  `json:"metadata"`
	Source           string          `json:"source"`
	Provenance       *Provenance     `json:"provenance,omitempty"`
}

type TestHints struct {
	HappyPath  string   `json:"happy_path"`
	ErrorCases []string `json:"error_cases"`
}

// EndpointAuth describes authentication requirements.
type EndpointAuth struct {
	Type       string   `json:"type"` // "jwt", "api_key", "none"
	Permission string   `json:"permission"`
	Roles      []string `json:"roles"`
	Check      string   `json:"check"`  // Custom auth check expression
	Inject     []string `json:"inject"` // Fields to inject from token
}

// Scope represents an API scope entry defined in CUE registry.
type Scope struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
	Source string   `json:"source,omitempty"`
}

// RateLimit describes rate limiting.
type RateLimit struct {
	RPS   int `json:"rps"`
	Burst int `json:"burst"`
}

type CircuitBreaker struct {
	Threshold   int    `json:"threshold"`     // количество ошибок до открытия
	Timeout     string `json:"timeout"`       // "30s" — время в состоянии Open
	HalfOpenMax int    `json:"half_open_max"` // макс запросов в Half-Open
}

type RetryPolicy struct {
	Enabled            bool  `json:"enabled"`
	MaxAttempts        int   `json:"max_attempts"`
	BaseDelayMS        int   `json:"base_delay_ms"`
	RetryOnStatuses    []int `json:"retry_on_statuses"`
	RetryNetworkErrors bool  `json:"retry_network_errors"`
}

// SLO describes service level objectives.
type SLO struct {
	Latency string `json:"latency"`
	Success string `json:"success"`
}

// Repository describes a data access interface.
type Repository struct {
	Name    string   `json:"name"`
	Entity  string   `json:"entity"`
	Finders []Finder `json:"finders"`
	Source  string   `json:"source"`
	Provenance *Provenance `json:"provenance,omitempty"`
}

// Finder describes a repository query method.
type Finder struct {
	Name       string        `json:"name"`
	Action     string        `json:"action"` // "find", "find_one", "count", "exists"
	Returns    string        `json:"returns"`
	ReturnType string        `json:"return_type"` // Explicit return type like "*domain.TenderReportInfo"
	Select     []string      `json:"select"`
	ScanFields []string      `json:"scan_fields"` // Field names to scan from SQL result (for custom entity types)
	Where      []WhereClause `json:"where"`
	OrderBy    string        `json:"order_by"`
	Limit      int           `json:"limit"`
	ForUpdate  bool          `json:"for_update"`
	CustomSQL  string        `json:"custom_sql"` // New field
	Source     string        `json:"source"`
	Provenance *Provenance   `json:"provenance,omitempty"`
}

// WhereClause describes a query condition.
type WhereClause struct {
	Field     string `json:"field"`
	Op        string `json:"op"` // "eq", "ne", "gt", "lt", "in", "like"
	Param     string `json:"param"`
	ParamType string `json:"param_type"`
}

// Config describes application configuration.
type Config struct {
	Fields     []Field     `json:"fields"`
	Provenance *Provenance `json:"provenance,omitempty"`
}

// Auth describes authentication settings.
type Auth struct {
	Algorithm    string     `json:"algorithm"`
	Issuer       string     `json:"issuer"`
	Audience     string     `json:"audience"`
	AccessTTL    string     `json:"access_ttl"`
	RefreshTTL   string     `json:"refresh_ttl"`
	Rotation     bool       `json:"rotation"`
	RefreshStore string     `json:"refresh_store"` // "redis", "postgres", "memory"
	Claims       AuthClaims `json:"claims"`
	Operations   AuthOps    `json:"operations"`
	Provenance   *Provenance `json:"provenance,omitempty"`
}

// AuthClaims describes JWT claim mappings.
type AuthClaims struct {
	UserID      string `json:"user_id"`
	CompanyID   string `json:"company_id"`
	Roles       string `json:"roles"`
	Permissions string `json:"permissions"`
}

// AuthOps describes auth service operations.
type AuthOps struct {
	Service             string `json:"service"`
	LoginOp             string `json:"login_op"`
	LoginAccessField    string `json:"login_access_field"`
	LoginRefreshField   string `json:"login_refresh_field"`
	RefreshOp           string `json:"refresh_op"`
	RefreshTokenField   string `json:"refresh_token_field"`
	RefreshAccessField  string `json:"refresh_access_field"`
	RefreshRefreshField string `json:"refresh_refresh_field"`
	LogoutOp            string `json:"logout_op"`
	LogoutTokenField    string `json:"logout_token_field"`
}

// RBAC describes role-based access control.
type RBAC struct {
	Roles       map[string][]string `json:"roles"`       // role -> permissions
	Permissions map[string]string   `json:"permissions"` // permission -> description
}

// Schedule describes a scheduled job.
type Schedule struct {
	Name    string  `json:"name"`
	Service string  `json:"service"`
	Action  string  `json:"action"`
	At      string  `json:"at"`      // cron expression
	Every   string  `json:"every"`   // duration
	Publish string  `json:"publish"` // event to publish
	Payload []Field `json:"payload"`
}

// View describes field visibility rules.
type View struct {
	Name  string              `json:"name"`
	Roles map[string][]string `json:"roles"` // role -> visible fields
}
