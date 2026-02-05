// Package ir defines language-agnostic Intermediate Representation.
// This IR knows nothing about Go, Rust, or any specific framework.
// It's a pure architectural description that can be transformed to any target.
package ir

// Schema is the root of the IR tree.
// It contains everything needed to generate a complete application.
type Schema struct {
	Project   Project
	Entities  []Entity
	Services  []Service
	Events    []Event
	Errors    []Error
	Endpoints []Endpoint
	Repos     []Repository
	Config    Config
	Auth      *Auth
	RBAC      *RBAC
	Schedules []Schedule
	Views     []View

	// Metadata for transformers to store computed data
	Metadata map[string]any
}

// Project contains project-level metadata.
type Project struct {
	Name    string
	Version string
	Target  Target
}

// Target describes the generation target.
type Target struct {
	Lang      string // "go", "rust", "typescript"
	Framework string // "chi", "echo", "fiber", "axum"
	DB        string // "postgres", "mysql", "mongodb"
	Cache     string // "redis", "memcached"
	Queue     string // "nats", "kafka", "rabbitmq"
	Storage   string // "s3", "gcs", "minio"
}

// Entity represents a domain entity (aggregate, value object, etc.)
type Entity struct {
	Name        string
	Description string
	Owner       string // Service name that owns this entity
	Fields      []Field
	FSM         *FSM
	Indexes     []Index
	UI          EntityUI
	Metadata    map[string]any
	Source      string
}

type EntityUI struct {
	CRUD *CRUDConfig
}

type CRUDConfig struct {
	Enabled bool
	Custom  bool
	Views   map[string]bool
	Perms   map[string]string
}

// Field represents a field in an entity or DTO.
type Field struct {
	Name        string
	Type        TypeRef
	Optional    bool
	Default     any
	IsSecret    bool
	IsPII       bool
	SkipDomain  bool   // Field is for DTO/UI only, skip in domain models
	ValidateTag string // @validate tag content
	EnvVar      string // @env tag content
	Attributes  []Attribute
	UI          FieldUI
	Metadata    map[string]any
	Source      string
}

type FieldUI struct {
	Type        string
	Label       string
	Placeholder string
	HelperText  string
	Order       int
	Hidden      bool
	Disabled    bool
	FullWidth   bool
	Rows        int      // For textarea
	Min         *float64 // For number/currency
	Max         *float64 // For number/currency
	Step        *float64 // For number
	Currency    string   // For currency
	Source      string   // Data source (e.g. "Service.Method")
	Options     []string // Static options
	Multiple    bool     // For select/autocomplete
	Accept      string   // For file types
	MaxSize     int      // For file size
}

// TypeRef is a language-agnostic type reference.
// It describes what kind of data this is, not how it's implemented.
type TypeRef struct {
	Kind         TypeKind
	Name         string   // For entity references: "User", "Order"
	ItemType     *TypeRef // For List/Map: the element type
	KeyType      *TypeRef // For Map: the key type
	InlineFields []Field  // For inline struct definitions in lists
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
	Name string
	Args map[string]any
}

// FSM describes a Finite State Machine for status fields.
type FSM struct {
	Field       string
	States      []string
	Transitions map[string][]string // from -> []to
}

// Index describes a database index.
type Index struct {
	Fields []string
	Unique bool
	Name   string
}

// Service represents a service/port interface.
type Service struct {
	Name        string
	Description string
	Methods     []Method
	Publishes   []string          // Events this service can publish
	Subscribes  map[string]string // event -> handler method
	Uses        []string          // Service dependencies (by name)
	Metadata    map[string]any
	Source      string

	// Infrastructure dependencies
	RequiresSQL   bool
	RequiresMongo bool
	RequiresRedis bool
	RequiresNats  bool
	RequiresS3    bool
}

// Method represents an RPC/service method.
type Method struct {
	Name        string
	Description string
	Input       *Entity
	Output      *Entity
	Sources     []Source
	CacheTTL    string
	CacheTags   []string
	Throws      []string
	Publishes   []string
	Broadcasts  []string
	Pagination  *Pagination
	Idempotent  bool
	DedupeKey   string
	Outbox      bool
	Impl        *Impl
	Flow        []FlowStep
	Metadata    map[string]any
	Source      string
}

// FlowStep represents a declarative step in a method's logic.
type FlowStep struct {
	Action    string
	Condition string
	Throw     string
	Input     string
	Output    string
	Value     string
	Params    []string
	Args      map[string]any
	Steps     []FlowStep
	Then      []FlowStep
	Else      []FlowStep
}

// Source describes where method data comes from.
type Source struct {
	Name       string
	Kind       string // "sql", "mongo", "cache", "external"
	Entity     string
	Collection string
	Query      map[string]string
	Metadata   map[string]any
}

// Pagination describes pagination settings.
type Pagination struct {
	Type         string // "cursor", "offset"
	DefaultLimit int
	MaxLimit     int
}

// Impl holds implementation code (for inline implementations).
type Impl struct {
	Lang       string
	Code       string
	Imports    []string
	RequiresTx bool
}

// Event represents a domain event.
type Event struct {
	Name     string
	Fields   []Field
	Metadata map[string]any
	Source   string
}

// Error represents a business error.
type Error struct {
	Name       string
	Code       int
	HTTPStatus int
	Message    string
	Source     string
}

// Endpoint represents an HTTP endpoint.
type Endpoint struct {
	Method           string // GET, POST, PUT, DELETE, WS
	Path             string
	Service          string
	RPC              string
	Description      string
	Messages         []string
	RoomParam        string
	Auth             *EndpointAuth
	Cache            string
	CacheTags        []string
	Invalidate       []string
	OptimisticUpdate string
	RateLimit        *RateLimit
	CircuitBreaker   *CircuitBreaker
	Timeout          string // Request timeout (e.g. "5s", "30s")
	MaxBodySize      int64  // Request body size limit in bytes
	Idempotent       bool
	DedupeKey        string
	Errors           []string
	Pagination       *Pagination
	View             string
	SLO              *SLO
	TestHints        *TestHints
	Metadata         map[string]any
	Source           string
}

type TestHints struct {
	HappyPath  string
	ErrorCases []string
}

// EndpointAuth describes authentication requirements.
type EndpointAuth struct {
	Type       string // "jwt", "api_key", "none"
	Permission string
	Roles      []string
	Check      string   // Custom auth check expression
	Inject     []string // Fields to inject from token
}

// RateLimit describes rate limiting.
type RateLimit struct {
	RPS   int
	Burst int
}

type CircuitBreaker struct {
	Threshold   int    // количество ошибок до открытия
	Timeout     string // "30s" — время в состоянии Open
	HalfOpenMax int    // макс запросов в Half-Open
}

// SLO describes service level objectives.
type SLO struct {
	Latency string
	Success string
}

// Repository describes a data access interface.
type Repository struct {
	Name    string
	Entity  string
	Finders []Finder
	Source  string
}

// Finder describes a repository query method.
type Finder struct {
	Name       string
	Action     string // "find", "find_one", "count", "exists"
	Returns    string
	ReturnType string // Explicit return type like "*domain.TenderReportInfo"
	Select     []string
	ScanFields []string // Field names to scan from SQL result (for custom entity types)
	Where      []WhereClause
	OrderBy    string
	Limit      int
	ForUpdate  bool
	CustomSQL  string // New field
	Source     string
}

// WhereClause describes a query condition.
type WhereClause struct {
	Field     string
	Op        string // "eq", "ne", "gt", "lt", "in", "like"
	Param     string
	ParamType string
}

// Config describes application configuration.
type Config struct {
	Fields []Field
}

// Auth describes authentication settings.
type Auth struct {
	Algorithm    string
	Issuer       string
	Audience     string
	AccessTTL    string
	RefreshTTL   string
	Rotation     bool
	RefreshStore string // "redis", "postgres", "memory"
	Claims       AuthClaims
	Operations   AuthOps
}

// AuthClaims describes JWT claim mappings.
type AuthClaims struct {
	UserID      string
	CompanyID   string
	Roles       string
	Permissions string
}

// AuthOps describes auth service operations.
type AuthOps struct {
	Service             string
	LoginOp             string
	LoginAccessField    string
	LoginRefreshField   string
	RefreshOp           string
	RefreshTokenField   string
	RefreshAccessField  string
	RefreshRefreshField string
	LogoutOp            string
	LogoutTokenField    string
}

// RBAC describes role-based access control.
type RBAC struct {
	Roles       map[string][]string // role -> permissions
	Permissions map[string]string   // permission -> description
}

// Schedule describes a scheduled job.
type Schedule struct {
	Name    string
	Service string
	Action  string
	At      string // cron expression
	Every   string // duration
	Publish string // event to publish
	Payload []Field
}

// View describes field visibility rules.
type View struct {
	Name  string
	Roles map[string][]string // role -> visible fields
}
