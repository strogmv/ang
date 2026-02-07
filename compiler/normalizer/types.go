package normalizer

// Entity представляет доменную сущность.
type Entity struct {
	Name        string
	Description string
	Owner       string
	Fields      []Field

	FSM      *FSM
	Indexes  []IndexDef
	UI       *EntityUIDef
	Metadata map[string]any // Универсальное хранилище для плагинов
	Source   string
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

// UIHints описывает подсказки для генерации UI компонентов.
type UIHints struct {
	Type        string   // text, textarea, number, date, datetime, select, autocomplete, checkbox, switch, file, currency, email, password, phone, url
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

// IndexDef описывает составной индекс.
type IndexDef struct {
	Fields []string
	Unique bool
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

type Method struct {
	Name string

	Description string

	Input Entity

	Output      Entity
	Sources     []Source
	CacheTTL    string
	CacheTags   []string
	Throws      []string
	Publishes   []string
	Broadcasts  []string
	Pagination  *PaginationDef
	Idempotency bool
	DedupeKey   string
	Outbox      bool
	Impl        *MethodImpl
	Flow        []FlowStep
	Attributes  []Attribute
	Metadata    map[string]any
	Source      string
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
	Name     string
	Fields   []Field
	Metadata map[string]any
	Source   string
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
	Method           string
	Path             string
	ServiceName      string
	RPC              string
	Description      string
	Messages         []string
	RoomParam        string
	AuthType         string
	Permission       string
	AuthRoles        []string
	AuthCheck        string
	AuthInject       []string
	CacheTTL         string
	CacheTags        []string
	Invalidate       []string
	OptimisticUpdate string
	RateLimit        *RateLimitDef
	CircuitBreaker   *CircuitBreakerDef
	Timeout          string // Request timeout (e.g. "5s", "30s")
	MaxBodySize      int64  // Request body size limit in bytes
	Idempotency      bool
	DedupeKey        string
	Errors           []string
	Pagination       *PaginationDef
	View             string
	SLO              SLODef
	TestHints        *TestHints
	Metadata         map[string]any
	Source           string
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
	RPS   int
	Burst int
}

type CircuitBreakerDef struct {
	Threshold   int    // количество ошибок до открытия
	Timeout     string // "30s" — время в состоянии Open
	HalfOpenMax int    // макс запросов в Half-Open
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
	Alg                 string
	Issuer              string
	Audience            string
	UserIDClaim         string
	CompanyIDClaim      string
	RolesClaim          string
	PermissionsClaim    string
	AccessTTL           string
	RefreshTTL          string
	Rotation            bool
	RefreshStore        string
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

// RBACDef описывает матрицу доступа.
type RBACDef struct {
	Roles       map[string][]string
	Permissions map[string]string
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
	ReturnType string   // Explicit return type like "*domain.TenderReportInfo" or "[]domain.PriceSnapshot"
	Select     []string
	ScanFields []string // Field names to scan from SQL result (for custom entity types)
	Where      []FinderWhere
	OrderBy    string
	Limit      int
	ForUpdate  bool
	CustomSQL  string // New field for hand-written complex SQL
	Source     string
}

type FinderWhere struct {
	Field     string
	Op        string
	Param     string
	ParamType string
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
	Payload []SchedulePayloadField
}

type SchedulePayloadField struct {
	Name  string
	Type  string
	Value string
}

type ProjectDef struct {
	Name    string
	Version string
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
	Name    string
	Subject string
	Text    string
	HTML    string
}

type FileMeta struct {
	Kind      string
	Thumbnail bool
}

// TargetDef describes the code generation target stack.
type TargetDef struct {
	Lang      string // "go", "rust", "typescript"
	Framework string // "chi", "echo", "fiber", "axum"
	DB        string // "postgres", "mysql", "mongodb"
	Cache     string // "redis", "memcached", "none"
	Queue     string // "nats", "kafka", "rabbitmq"
	Storage   string // "s3", "gcs", "minio"
}

// TransformersConfig describes which transformers are enabled.
type TransformersConfig struct {
	Timestamps  bool
	SoftDelete  bool
	Image       bool
	ThumbSuffix string
	Validation  bool
}
