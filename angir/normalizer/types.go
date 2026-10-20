package normalizer

// AIActPolicy describes EU AI Act compliance metadata for a service method.
// Set via @ai_act(risk="limited", use_case="recommendation", oversight="human_review") on an operation.
type AIActPolicy struct {
	// Risk: "prohibited" | "high" | "limited" | "minimal"
	Risk    string
	UseCase string
	// Oversight: "none" | "human_review" | "human_override"
	Oversight string
	// Logging: whether AI interactions must be logged (default true for high risk)
	Logging bool
}

// GDPRPolicy describes per-entity GDPR compliance requirements.
// Set via @gdpr(erasable=true, exportable=true, retention="2y", owner_field="userId") on an entity.
type GDPRPolicy struct {
	// Erasable: generate ErasePersonalData(ctx, ownerID) that NULLs/anonymizes PII fields.
	Erasable bool
	// Exportable: generate ExportPersonalData(ctx, ownerID) that returns all rows as JSON.
	Exportable bool
	// Retention: Go duration string (e.g. "3y", "90d"). Generate PurgeExpired(ctx, cutoff).
	Retention string
	// OwnerField: field name pointing to the data subject's ID (default: "userId").
	OwnerField string
}

// Entity представляет доменную сущность.
type Entity struct {
	Name           string
	Description    string
	Owner          string
	BoundedContext string
	AggregateRoot  bool
	Owns           []string
	ReadModel      *ReadModelDef
	Fields         []Field
	// Table is the entity's SQL table (MongoDB collection), resolved by
	// ExtractEntities from @db_table(name="…") or target.table_names. Read it
	// through TableName(), which falls back for entities built elsewhere.
	Table string

	FSM     *FSM
	Indexes []IndexDef
	// Checks are named CHECK constraints of the entity's table, declared with
	// @check(name="…", expr="…") on the entity.
	Checks []CheckDef
	// DDL is raw SQL emitted into db/schema/schema.sql after the entity's
	// table, indexes and checks (triggers, trigger functions), from the
	// entity key ddl: """…""". ANG does not parse it.
	DDL []string
	// Queue makes the entity a job queue (@queue(handler="…", …)).
	Queue      *QueueDef
	UI         *EntityUIDef
	GDPRPolicy *GDPRPolicy    // Non-nil when entity has @gdpr annotation
	Metadata   map[string]any // Универсальное хранилище для плагинов
	Source     string
}

// ReadModelDef describes ACL read-model contract for cross-context analytics.
type ReadModelDef struct {
	SourceContext string
	RefreshOn     []string
}

type EntityUIDef struct {
	CRUD *CRUDDef
}

type CRUDDef struct {
	Enabled bool
	Custom  bool
	Views   map[string]bool
	Perms   map[string]string
}

// FSM описывает конечный автомат.
type FSM struct {
	Field       string
	States      []string
	Transitions map[string][]string
}

// Field описывает поле сущности.
type Field struct {
	Name         string
	Type         string
	Default      string
	IsOptional   bool
	IsList       bool
	FileMeta     *FileMeta
	DB           DBMeta
	ValidateTag  string
	Constraints  *Constraints // Structured constraints from CUE
	EnvVar       string
	IsSecret     bool
	IsPII        bool
	SkipDomain   bool // If true, field is excluded from domain.go (DTO/UI only)
	ItemTypeName string
	ItemFields   []Field
	Metadata     map[string]any // Метаданные (sql_type, null_helper и т.д.)
	UI           *UIHints       // UI подсказки для генерации фронтенда
	Source       string
}

// Constraints stores structured validation data from CUE.
type Constraints struct {
	Min    *float64
	Max    *float64
	MinLen *int
	MaxLen *int
	Regex  string
	Enum   []string
}

// UIHints описывает подсказки для генерации UI компонентов.
type UIHints struct {
	Type        string   // text, textarea, number, date, datetime, select, autocomplete, checkbox, switch, file, currency, email, password, phone, url
	Importance  string   // semantic importance: high, normal, low
	InputKind   string   // semantic input kind: sensitive, email, phone, money, etc
	Intent      string   // semantic intent: danger, success, warning, info, neutral
	Density     string   // semantic density: compact, normal, spacious
	LabelMode   string   // semantic label mode: static, floating, hidden
	Surface     string   // semantic surface: paper, flat, raised
	Component   string   // Custom component name for type="custom"
	Section     string   // Logical section/group name
	Columns     int      // Layout columns span hint
	Label       string   // Метка поля
	Placeholder string   // Плейсхолдер
	HelperText  string   // Подсказка под полем
	Order       int      // Порядок отображения
	Hidden      bool     // Скрытое поле
	Disabled    bool     // Отключённое поле
	FullWidth   bool     // На всю ширину
	Rows        int      // Для textarea
	Min         *float64 // Для number/currency
	Max         *float64 // Для number/currency
	Step        *float64 // Для number
	Currency    string   // Для currency (default: "BYN")
	Source      string   // Для select/autocomplete — источник данных
	Options     []string // Для статичного select
	Multiple    bool     // Для select/autocomplete
	Accept      string   // Для file — типы файлов
	MaxSize     int      // Для file — макс размер
}

// DBMeta хранит настройки БД.
type DBMeta struct {
	Type       string
	PrimaryKey bool
	Unique     bool
	Index      bool
}

// IndexDef describes an index of the entity's table, declared in the entity's
// indexes: [...] list. Fields are entity field names; Expr replaces them with a
// raw SQL key list ("lower(email)", "companyid, lower(btrim(externalid))").
// Where, Using, Opclass and Expr are raw SQL and use column names, not field
// names.
type IndexDef struct {
	Fields []string
	Unique bool
	// Name overrides the generated idx_<table>_<columns> name.
	Name string
	// Expr is the raw key list of an expression index; it excludes Fields.
	Expr string
	// Where makes a partial index.
	Where string
	// Using is the access method: btree, hash, gist, spgist, gin, brin.
	Using string
	// Opclass is the operator class applied to every column in Fields.
	Opclass string
	// Concurrently asks migrations to build the index with CREATE INDEX
	// CONCURRENTLY. schema.sql builds an empty database and ignores it.
	Concurrently bool
	// Source is the file:line of the declaration.
	Source string
}

// CheckDef is a named CHECK constraint; Expr is raw SQL over column names.
type CheckDef struct {
	Name   string
	Expr   string
	Source string
}

// Service описывает интерфейс (порт).
type Service struct {
	Name        string
	Description string
	Methods     []Method

	Publishes  []string
	Subscribes map[string]string
	Uses       []string
	Metadata   map[string]any
	Source     string

	// Инфраструктурные зависимости
	RequiresSQL   bool
	RequiresMongo bool
	RequiresRedis bool
	RequiresNats  bool
	RequiresS3    bool
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
	Kind        string
	Channel     string
	Event       string
	Template    string
	TargetField string
}

type PlannerRoute struct {
	Method string
	Path   string
}

type PlannerRepository struct {
	LoadMethod string
	ListMethod string
	ActorField string
	InputField string
}

type PlannerHints struct {
	SourcePack string
	Route      *PlannerRoute
	Repository *PlannerRepository
}

type Method struct {
	Name string

	Description string
	IsStreaming bool
	// Transactional (transactional: true) runs the whole operation — its
	// flow or its impl code — in one database transaction.
	Transactional bool

	Input Entity

	Output               Entity
	Sources              []Source
	CacheTTL             string
	CacheTags            []string
	Throws               []string
	Publishes            []string
	Broadcasts           []string
	Pagination           *PaginationDef
	Idempotency          bool
	DedupeKey            string
	Outbox               bool
	PrimaryOperationKind OperationKind
	Capabilities         []CapabilityKind
	SideEffects          []SideEffect
	ManualRequired       bool
	Planner              *PlannerHints
	Impl                 *MethodImpl
	Flow                 []FlowStep
	Effects              []string
	ImplSteps            []ImplStep
	Attributes           []Attribute
	AIActPolicy          *AIActPolicy
	Metadata             map[string]any
	Source               string
	// Helpers are the shared helpers (Helpers: in cue/api) the operation's Go
	// reaches, directly or through other helpers, sorted.
	Helpers []string
	// HelperCode is the Go of those helpers. It is not emitted with the
	// operation (each helper has its own file); MethodCodeText includes it so
	// that what the operation reaches through a helper (s.XRepo, s.Tx) counts.
	HelperCode string
}

type FlowStep struct {
	Action     string
	Params     []string
	Args       map[string]any
	Metadata   map[string]any
	Attributes []Attribute
	File       string
	Line       int
	Column     int
	CUEPath    string
	// ArgText says where the text of each string argument written as one
	// multi-line literal starts: the line after the opening quotes. Go copied
	// from such a literal keeps its lines, so it maps to CUE line by line.
	ArgText map[string]TextPos
}

// TextPos is a line in a CUE file.
type TextPos struct {
	File string
	Line int
}

type Attribute struct {
	Name string
	Args map[string]any
}

type MethodImpl struct {
	Lang       string
	Code       string
	Imports    []string
	RequiresTx bool
	// CodeSource is file:line of `code` in CUE; empty when the code comes
	// from a Go sidecar (funcRef), whose imports the Go parser already read.
	CodeSource string
	// CodeText is where the text of a multi-line `code` string starts.
	CodeText TextPos
	// ImportSources[i] is file:line of Imports[i].
	ImportSources []string
}

// ImplStep is a typed instruction inside impl_steps DSL.
type ImplStep struct {
	Kind         string            // load, assert, call, emit
	LoadTarget   string            // load
	LoadBy       map[string]string // load
	LoadInto     string            // load
	AssertExpr   string            // assert
	AssertError  string            // assert
	CallTarget   string            // call
	CallArgsExpr string            // call (raw expression string)
	CallArgsMap  map[string]any    // call (structured)
	CallInto     string            // call
	EmitEvent    string            // emit
	EmitPayload  map[string]any    // emit
	EmitExpr     string            // emit
	Source       string
}

type Source struct {
	Name       string
	Kind       string
	Entity     string
	Collection string
	By         map[string]string
	Filter     map[string]string
	Metadata   map[string]any
}

// EventDef описывает событие системы.
type EventDef struct {
	Name      string
	Owner     string
	Consumers []string
	Fields    []Field
	Metadata  map[string]any
	Source    string
}

// ErrorDef описывает бизнес-ошибку.
type ErrorDef struct {
	Name       string
	Code       int
	HTTPStatus int
	Message    string
	Source     string
}

// Endpoint описывает HTTP эндпоинт.
type Endpoint struct {
	Method      string
	Path        string
	IsStreaming bool
	ServiceName string
	RPC         string
	Description string
	Messages    []string
	RoomParam   string
	// Live turns a WS room into a server-pushed snapshot room (see LiveRoomDef).
	Live       *LiveRoomDef
	AuthType   string
	Permission string
	AuthRoles  []string
	// AuthRight is auth.right: the right (beyond the role) a role-gated
	// route needs, handed to the project's CheckRoleAccess.
	AuthRight        string
	AuthCheck        string
	AuthInject       []string
	CacheTTL         string
	CacheTags        []string
	Invalidate       []string
	OptimisticUpdate string
	RateLimit        *RateLimitDef  // primary limit, = &RateLimits[0]
	RateLimits       []RateLimitDef // every stacked limit, each on its own counters
	CircuitBreaker   *CircuitBreakerDef
	RetryPolicy      *RetryPolicyDef
	Timeout          string // Request timeout (e.g. "5s", "30s")
	MaxBodySize      int64  // Request body size limit in bytes
	Idempotency      bool
	// Read: a non-GET endpoint that only reads (http read: true).
	Read           bool
	MaxConcurrent  int  // max simultaneous in-flight requests; 0 = unlimited
	Coalesce       bool // deduplicate identical in-flight GET requests via singleflight
	RequiredScopes []string
	DedupeKey      string
	Errors         []string
	Pagination     *PaginationDef
	View           string
	SLO            SLODef
	TestHints      *TestHints
	Metadata       map[string]any
	Source         string
}

// LiveRoomDef makes a WebSocket room "live": on every trigger event one
// replica builds the room state (State op), publishes it to the JetStream
// stream LIVE_ROOMS, and every replica pushes each socket the View op's answer
// for its viewer, replaying missed states on reconnect.
type LiveRoomDef struct {
	State    string   // op: {<room field>} -> {state: string}
	View     string   // op: {state, companyIds, now} -> {snapshots, denied, refreshAt}
	Triggers []string // events that may change the room
}

// ScopeDef represents a scope entry from CUE registry.
type ScopeDef struct {
	Name   string
	Values []string
	Source string
}

type TestHints struct {
	HappyPath  string
	ErrorCases []string
}

type PaginationDef struct {
	Type         string
	DefaultLimit int
	MaxLimit     int
}

type RateLimitDef struct {
	RPS         int
	Burst       int
	Window      string // duration string, e.g. "1h"
	WindowLimit int    // max requests per window (0 = disabled)
	// Key selects whose budget a request spends: "" or "ip" (client address),
	// "user" (authenticated user id, address as fallback) or "company".
	Key string
}

// Rate-limit keys accepted by RateLimitDef.Key.
const (
	RateLimitKeyIP      = "ip"
	RateLimitKeyUser    = "user"
	RateLimitKeyCompany = "company"
)

// KeyOrIP returns the limit's key with the empty default spelled out.
func (rl RateLimitDef) KeyOrIP() string {
	if rl.Key == "" {
		return RateLimitKeyIP
	}
	return rl.Key
}

type CircuitBreakerDef struct {
	Threshold   int    // количество ошибок до открытия
	Timeout     string // "30s" — время в состоянии Open
	HalfOpenMax int    // макс запросов в Half-Open
}

type RetryPolicyDef struct {
	Enabled            bool
	MaxAttempts        int
	BaseDelayMS        int
	RetryOnStatuses    []int
	RetryNetworkErrors bool
}

type SLODef struct {
	Latency string
	Success string
}

// ConfigDef описывает конфигурацию.
type ConfigDef struct {
	Fields []Field
}

// AuthDef описывает настройки JWT.
type AuthDef struct {
	Mode                    string
	BearerFallback          bool
	Alg                     string
	Issuer                  string
	Audience                string
	UserIDClaim             string
	CompanyIDClaim          string
	RolesClaim              string
	PermissionsClaim        string
	AccessTTL               string
	RefreshTTL              string
	Rotation                bool
	RefreshStore            string
	Service                 string
	LoginOp                 string
	LoginAccessField        string
	LoginRefreshField       string
	RegisterOp              string
	RegisterAccessField     string
	RegisterRefreshField    string
	DemoSessionOp           string
	DemoSessionAccessField  string
	DemoSessionRefreshField string
	// MfaVerifyOp finishes a login that stopped at the second factor. It is a
	// session-issuing operation like login and register: without it the
	// cookie is never written and a two-factor login cannot reach the app.
	MfaVerifyOp           string
	MfaVerifyAccessField  string
	MfaVerifyRefreshField string
	RefreshOp             string
	RefreshTokenField     string
	RefreshAccessField    string
	RefreshRefreshField   string
	LogoutOp              string
	LogoutTokenField      string
	// LogoutAllOp ends every session of the calling user.
	LogoutAllOp string
	// PasswordChangeOp changes the password of the signed-in user and hands
	// back a fresh token pair: every other session of the user ends, and the
	// device that made the change keeps working on the new pair.
	PasswordChangeOp           string
	PasswordChangeAccessField  string
	PasswordChangeRefreshField string
	// SessionOps are further operations that end in a signed-in session (a
	// sign-in through an identity provider, the step that finishes such a
	// sign-up): their token pair is written as the session cookie like a
	// login's. CUE: ops.sessions: <name>: {op, access_field, refresh_field}.
	SessionOps []AuthSessionOp
	// SessionClaims are access-token claims an opaque session keeps for its
	// whole life (CUE: jwt.session_claims: [...string]). The operation that
	// issues the session puts them into the access token; the server copies
	// them into the session record, and every request of that session finds
	// them in reqctx.SessionAttrs. Only the server signs the token, so a
	// client cannot add or change one.
	SessionClaims []string
	// SessionExpiryClaim names an access-token claim (unix seconds) that ends
	// the session earlier than its usual absolute lifetime (CUE:
	// jwt.session_expiry_claim). The session is never renewed past it.
	SessionExpiryClaim string
}

// AuthSessionOp is one operation that issues a session.
type AuthSessionOp struct {
	Op           string
	AccessField  string
	RefreshField string
	// Credential "header": the session is not written as the cookie. Its
	// opaque id comes back in SessionField and the client presents it in the
	// X-Auth-Session header (a WebSocket handshake: ?auth_session=), which
	// the server reads before the cookie. One browser tab can then hold a
	// session of its own without replacing the cookie session every other
	// tab of the same API host shares. The response carries neither token.
	Credential   string
	SessionField string
}

// HeaderCredential reports whether the operation issues a header session.
func (o AuthSessionOp) HeaderCredential() bool {
	return o.Credential == "header"
}

// SessionDef describes anonymous cookie-session configuration.
type SessionDef struct {
	CookieName string // e.g. "sendbox_session"
	TTLSeconds int    // cookie max-age in seconds; default 365 days
}

// RBACDef описывает матрицу доступа.
type RBACDef struct {
	Roles       map[string][]string
	Permissions map[string]string
}

// PolicyDef describes typed authorization policy constraints.
type PolicyDef struct {
	Name               string
	Description        string
	Roles              []string
	SameCompany        bool
	AllowAdminOverride bool
	Source             string
}

// Repository описывает интерфейс доступа к данным.
type Repository struct {
	Name    string
	Entity  string
	Finders []RepositoryFinder
	Source  string
}

type RepositoryFinder struct {
	Name       string
	Action     string
	Returns    string
	ReturnType string // Explicit return type like "*domain.TenderReportInfo" or "[]domain.PriceSnapshot"
	Select     []string
	ScanFields []string // Field names to scan from SQL result (for custom entity types)
	Where      []FinderWhere
	OrderBy    string
	Limit      int
	ForUpdate  bool
	CustomSQL  string // New field for hand-written complex SQL
	SumField   string // field to use in SUM() when Returns == "sum"
	Source     string
	// Params are the method parameters of a sql: finder, bound to $1..$n in
	// this order (params: [{name, type}]). Where and Params never both.
	Params []FinderParam
	// Paged is the explicit paged: true/false. nil keeps the legacy rule:
	// limit: > 0 or a name containing "paginate" (FinderPaged).
	Paged *bool
	// Kind "upsert" makes the finder an INSERT … ON CONFLICT (OnConflict)
	// DO UPDATE of a batch of entities.
	Kind       string
	OnConflict []string
	// SelectAuto (select: "auto" on a sql: finder): ANG replaces the * of the
	// SELECT with the columns the method scans, in scan order.
	SelectAuto bool
}

// FinderParam is one declared parameter of a sql: finder.
type FinderParam struct {
	Name string
	Type string // Go type
	Spec string // the type as written in CUE (uuid, []uuid, time …)
}

type FinderWhere struct {
	Field     string
	Op        string
	Param     string
	ParamType string
	// ParamTypeExplicit is set when the CUE wrote param_type; only an
	// unwritten type takes the type of the entity field.
	ParamTypeExplicit bool
}

type ViewDef struct {
	Name  string
	Roles map[string][]string
}

type ScheduleDef struct {
	Name    string
	Service string
	Action  string
	At      string
	Publish string
	Every   string
	// Cron is a five-field cron expression (minute hour day-of-month month
	// day-of-week) evaluated in SCHEDULER_TIMEZONE.
	Cron    string
	Payload []SchedulePayloadField
	// Generated is set by ExpandSchedules on a one-declaration schedule
	// (no publish:): its trigger event and subscription were created for it.
	Generated bool
	// Source is the file:line of the declaration.
	Source string
}

type SchedulePayloadField struct {
	Name  string
	Type  string
	Value string
}

type CrossServiceRule struct {
	Service string
	Entity  string
}

type ProjectDef struct {
	Name              string
	Version           string
	Plugins           []string
	UIProvider        string
	ArchitectureMode  string
	AllowCrossService []CrossServiceRule
}

// ScenarioDef represents a behavioral E2E scenario.
type ScenarioDef struct {
	Name        string
	Description string
	Steps       []ScenarioStep
	Source      string
}

type ScenarioStep struct {
	Name   string
	Action string
	Input  map[string]any
	Expect ScenarioExpect
	Export map[string]string
}

type ScenarioExpect struct {
	Status int
	Body   map[string]any
}

type EmailTemplateDef struct {
	Name     string
	Subject  string
	Text     string
	HTML     string
	TextFile string
	HTMLFile string
}

// TemplateDef describes universal channel-agnostic template catalog item.
type TemplateDef struct {
	ID           string
	Kind         string
	Channel      string
	Locale       string
	Version      string
	Engine       string
	Subject      string
	Text         string
	HTML         string
	Body         string
	SubjectFile  string
	TextFile     string
	HTMLFile     string
	BodyFile     string
	RequiredVars []string
	OptionalVars []string
}

type FileMeta struct {
	Kind      string
	Thumbnail bool
}

// TargetDef describes the code generation target stack.
type TargetDef struct {
	Name           string // "go-core"
	Lang           string // "go" (the only language ANG generates)
	Framework      string // "chi" (HTTP) or "grpc"
	DB             string // "postgres"
	Cache          string // "redis", "none"
	Queue          string // "nats", "none"
	Storage        string // "s3", "none"
	OutputDir      string // generated backend output root for this target
	FrontendAppDir string // optional: copy generated SDK into this frontend directory
	// FrontendDir is where the frontend SDK is generated: relative to the
	// project or absolute. Empty means the default, "sdk" in the backend.
	FrontendDir string
	// FrontendSDKSkip names frontend SDK modules this project does not use;
	// ANG skips generating them (see docs/ang/conventions.md of a project).
	FrontendSDKSkip          []string
	NatsWorkers              int // max concurrent NATS handlers per subscriber; 0 = default (20)
	NatsPublishRetryAttempts int // retry attempts on publish failure; default 3
	NatsPublishRetryDelayMS  int // initial backoff in ms; doubles each attempt; default 100
	// SQLIndexNames is how a generated index name longer than Postgres' 63
	// bytes is emitted: "" / "truncate" as is (Postgres keeps 63 bytes),
	// "hashed" as its first 54 bytes + "_" + 8 hex digits of its SHA-256.
	SQLIndexNames string
	// TableNames is target.table_names: "" / "legacy" names a table
	// lower(Entity) + "s" (companys), "plural" by English rules (companies).
	TableNames string
	// PostgresVersion is the major version of the Atlas dev database used to
	// plan migrations (atlas.hcl, ang migrate diff, ang db); "" means 18.
	PostgresVersion string
	// AuthDefault is target.auth_default: "" or "allow" keeps an endpoint
	// without auth public; "deny" makes it require a signed-in caller unless
	// it says auth: {public: true}.
	AuthDefault string
	// ServiceConfig is target.service_config: every <Service>Impl gets the
	// loaded configuration as s.Cfg, and config.Default() is generated.
	ServiceConfig bool
	// OperationMetrics is target.operation_metrics: Prometheus metrics and
	// OpenTelemetry spans per operation and per repository query.
	OperationMetrics bool
	// ServerRoles (target.server_roles: true) makes the generated
	// cmd/server read SERVER_ROLE=api|worker|all: HTTP routes only for
	// api|all, subscriptions, schedules and queue workers only for
	// worker|all. Off, the server runs everything as before.
	ServerRoles bool
	// ShutdownTimeout (target.shutdown_timeout, e.g. "30s") bounds the
	// graceful drain on SIGTERM; SHUTDOWN_TIMEOUT overrides it at run time.
	ShutdownTimeout string
	// SecurityHeaders is target.security_headers: the API's
	// Content-Security-Policy (csp), Cross-Origin-Opener-Policy (coop) and
	// Permissions-Policy (permissions_policy). nil keeps the defaults.
	SecurityHeaders *SecurityHeadersDef
	// MetricsPort is target.metrics_port: /metrics is served on this port
	// only, not on the API port (METRICS_PORT overrides it at run time).
	MetricsPort string
	// RouteRights is target.route_rights: routes gated by one of its roles
	// must declare auth.right, one of its rights.
	RouteRights *RouteRightsDef
}

// SecurityHeadersDef is target.security_headers; a nil field keeps the
// default, a pointer to "" switches the header off.
type SecurityHeadersDef struct {
	CSP               *string
	COOP              *string
	PermissionsPolicy *string
}

// RouteRightsDef is target.route_rights: {roles: ["admin"], rights: [...]}.
// Every HTTP route whose auth.roles names one of Roles must say which right
// it needs (auth.right); with Rights given, the right must be one of them.
type RouteRightsDef struct {
	Roles  []string
	Rights []string
}

// NotificationMutingDef describes automatic notification muting via repository decorator.
type NotificationMutingDef struct {
	Enabled         bool
	UserEntity      string
	MuteAllField    string
	MutedTypesField string
}

// NotificationChannelConfig describes one delivery channel backend (email/in_app/nats/kafka).
type NotificationChannelConfig struct {
	Enabled    bool
	Driver     string
	Topic      string
	Subject    string
	Template   string
	DSNEnv     string
	BrokersEnv string
}

// NotificationChannelsDef describes multi-channel notification routing settings.
type NotificationChannelsDef struct {
	Enabled         bool
	DefaultChannels []string
	Channels        map[string]NotificationChannelConfig
}

// NotificationPolicyRule describes one routing rule from domain event to channels/templates.
type NotificationPolicyRule struct {
	Enabled  bool
	Event    string
	Type     string
	Audience string
	Channels []string
	Template string
	MuteKey  string
}

// NotificationPoliciesDef describes policy-based notification routing configuration.
type NotificationPoliciesDef struct {
	Enabled bool
	Rules   []NotificationPolicyRule
}

// EffectHandlerBinding describes one infra handler binding for an effect kind.
type EffectHandlerBinding struct {
	Kind     string
	Driver   string
	Provider string
	Options  map[string]any
	Source   string
}

// EffectHandlersDef groups effect handler bindings by effect kind.
type EffectHandlersDef struct {
	Bindings map[string]EffectHandlerBinding
}

// EffectMiddlewareDef describes one middleware instance bound to an effect kind.
type EffectMiddlewareDef struct {
	Type     string
	Attempts int
	Backoff  string
	On       []int
	TTL      string
	Key      string
	Duration string
	Level    string
	Options  map[string]any
	Source   string
}

// EffectMiddlewareCatalogDef stores middleware chains per effect kind.
type EffectMiddlewareCatalogDef struct {
	Chains map[string][]EffectMiddlewareDef
}

// ModelsDef stores named AI/model aliases from infra.
type ModelsDef struct {
	Aliases map[string]string
}

// TransformersConfig describes which transformers are enabled.
type TransformersConfig struct {
	Timestamps  bool
	SoftDelete  bool
	Image       bool
	ThumbSuffix string
	Validation  bool
}
