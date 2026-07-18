// Package facts defines the stable, importable ang/facts/v1 data contract.
//
// It deliberately contains data only: extraction, validation, and inference
// stay in their respective packages. Keeping the JSON model outside cmd/ang
// lets compiler packages consume facts without importing a command package.
package facts

const SchemaV1 = "ang/facts/v1"

// Envelope is the root ang/facts/v1 document.
type Envelope struct {
	Schema         string          `json:"schema"`
	SourceType     string          `json:"source_type"`
	SourcePath     string          `json:"source_path"`
	Entities       []Entity        `json:"entities"`
	Operations     []Operation     `json:"operations"`
	Repositories   []Repository    `json:"repositories"`
	Events         []Event         `json:"events"`
	Constants      []Constant      `json:"constants,omitempty"`
	Enums          []Enum          `json:"enums,omitempty"`
	Endpoints      []Endpoint      `json:"endpoints,omitempty"`
	Calls          []CallEdge      `json:"calls,omitempty"`
	Mappers        []Mapper        `json:"mappers,omitempty"`
	ErrorContracts []ErrorContract `json:"error_contracts,omitempty"`
	SecurityRules  []SecurityRule  `json:"security_rules,omitempty"`
}

// Entity represents a domain entity or struct.
type Entity struct {
	Name               string  `json:"name"`
	TableHint          string  `json:"table_hint,omitempty"`
	Source             string  `json:"source,omitempty"`
	Fields             []Field `json:"fields"`
	CompositeKey       string  `json:"composite_key,omitempty"`
	SoftDelete         bool    `json:"soft_delete,omitempty"`
	SoftDeleteStrategy string  `json:"soft_delete_strategy,omitempty"`
	SoftDeleteClause   string  `json:"soft_delete_clause,omitempty"`
	WhereClause        string  `json:"where_clause,omitempty"`
}

// Field is one field of an entity, operation, or event.
type Field struct {
	Name            string   `json:"name"`
	GoType          string   `json:"go_type,omitempty"`
	ResolvedType    string   `json:"resolved_type,omitempty"`
	CueTypeHint     string   `json:"cue_type_hint,omitempty"`
	JSONTag         string   `json:"json_tag,omitempty"`
	DBTag           string   `json:"db_tag,omitempty"`
	SourceLine      int      `json:"source_line,omitempty"`
	Extractor       string   `json:"extractor,omitempty"`
	Evidence        []string `json:"evidence,omitempty"`
	Validate        string   `json:"validate,omitempty"`
	ValidationRules []string `json:"validation_rules,omitempty"`
	Required        bool     `json:"required,omitempty"`
	RelationKind    string   `json:"relation_kind,omitempty"`
	RelationTarget  string   `json:"relation_target,omitempty"`
	MappedBy        string   `json:"mapped_by,omitempty"`
	Cascade         []string `json:"cascade,omitempty"`
	Fetch           string   `json:"fetch,omitempty"`
	OrphanRemoval   bool     `json:"orphan_removal,omitempty"`
	JoinColumn      string   `json:"join_column,omitempty"`
	JoinTable       string   `json:"join_table,omitempty"`
	EnumType        string   `json:"enum_type,omitempty"`
	Persistence     []string `json:"persistence,omitempty"`
}

// Operation represents a service operation from source methods or OpenAPI paths.
type Operation struct {
	Name          string    `json:"name"`
	ServiceHint   string    `json:"service_hint,omitempty"`
	Source        string    `json:"source,omitempty"`
	SourceLine    int       `json:"source_line,omitempty"`
	ClassName     string    `json:"class_name,omitempty"`
	Kind          string    `json:"kind,omitempty"`
	Extractor     string    `json:"extractor,omitempty"`
	Evidence      []string  `json:"evidence,omitempty"`
	EntryKind     string    `json:"entry_kind,omitempty"`
	EntryRef      string    `json:"entry_ref,omitempty"`
	HTTPMethod    string    `json:"http_method,omitempty"`
	HTTPPath      string    `json:"http_path,omitempty"`
	AuthExpr      string    `json:"auth_expr,omitempty"`
	Transactional bool      `json:"transactional,omitempty"`
	TxReadOnly    bool      `json:"tx_read_only,omitempty"`
	InputFields   []Field   `json:"input_fields,omitempty"`
	OutputFields  []Field   `json:"output_fields,omitempty"`
	Calls         []CallRef `json:"calls,omitempty"`
	ConstantsUsed []string  `json:"constants_used,omitempty"`
	EnumsUsed     []string  `json:"enums_used,omitempty"`
}

// Repository describes repository interface methods.
type Repository struct {
	Entity  string             `json:"entity"`
	Source  string             `json:"source,omitempty"`
	Methods []RepositoryMethod `json:"methods"`
}

// RepositoryMethod is one method of a repository.
type RepositoryMethod struct {
	Name           string   `json:"name"`
	Returns        string   `json:"returns"`
	QueryKind      string   `json:"query_kind,omitempty"`
	CriteriaFields []string `json:"criteria_fields,omitempty"`
}

// Event represents an event or notification struct.
type Event struct {
	Name          string  `json:"name"`
	Source        string  `json:"source,omitempty"`
	PayloadFields []Field `json:"payload_fields,omitempty"`
}

type Constant struct {
	Name   string `json:"name"`
	Type   string `json:"type,omitempty"`
	Value  string `json:"value,omitempty"`
	Scope  string `json:"scope,omitempty"`
	Source string `json:"source,omitempty"`
}

type Enum struct {
	Name   string   `json:"name"`
	Values []string `json:"values,omitempty"`
	Source string   `json:"source,omitempty"`
}

type Endpoint struct {
	Operation     string   `json:"operation"`
	HTTPMethod    string   `json:"http_method,omitempty"`
	HTTPPath      string   `json:"http_path,omitempty"`
	ServiceHint   string   `json:"service_hint,omitempty"`
	Source        string   `json:"source,omitempty"`
	SourceLine    int      `json:"source_line,omitempty"`
	Extractor     string   `json:"extractor,omitempty"`
	Evidence      []string `json:"evidence,omitempty"`
	AuthExpr      string   `json:"auth_expr,omitempty"`
	Transactional bool     `json:"transactional,omitempty"`
	TxReadOnly    bool     `json:"tx_read_only,omitempty"`
}

type CallRef struct {
	Target         string   `json:"target"`
	ResolvedTarget string   `json:"resolved_target,omitempty"`
	Kind           string   `json:"kind,omitempty"`
	Evidence       []string `json:"evidence,omitempty"`
}

type CallEdge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Kind   string `json:"kind,omitempty"`
	Source string `json:"source,omitempty"`
}

type Mapper struct {
	Name    string         `json:"name"`
	Source  string         `json:"source,omitempty"`
	Uses    []string       `json:"uses,omitempty"`
	Methods []MapperMethod `json:"methods,omitempty"`
}

type MapperMethod struct {
	Name       string `json:"name"`
	SourceType string `json:"source_type,omitempty"`
	TargetType string `json:"target_type,omitempty"`
	Many       bool   `json:"many,omitempty"`
}

type ErrorContract struct {
	Exception string `json:"exception"`
	Status    string `json:"status,omitempty"`
	HTTPCode  int    `json:"http_code,omitempty"`
	Handler   string `json:"handler,omitempty"`
	Source    string `json:"source,omitempty"`
}

type SecurityRule struct {
	Scope       string `json:"scope,omitempty"`
	Pattern     string `json:"pattern,omitempty"`
	Requirement string `json:"requirement,omitempty"`
	Source      string `json:"source,omitempty"`
}
