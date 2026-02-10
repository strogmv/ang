package planner

// ScanVariable describes one scan destination and conversion mapping.
type ScanVariable struct {
	Name       string
	GoPath     string
	IsOptional bool
	MappingFn  string
	TmpVar     string
	TmpType    string
	Guard      string
	AssignCode string
}

// ScanPlan is an ordered DB-to-struct mapping contract.
type ScanPlan struct {
	Columns   []string
	ColList   string
	Variables []ScanVariable
}

// RenderPlan is a generic template rendering envelope.
// Templates should consume precomputed plan data instead of embedding logic.
type RenderPlan struct {
	Name string
	Data map[string]any
}

type FieldPlan struct {
	Name       string
	Type       string
	IsOptional bool
}

type ModelPlan struct {
	Name   string
	Fields []FieldPlan
}

type RoutePlan struct {
	ServiceName string
	HandlerName string
	Method      string
	Path        string
	InputType   string
	OutputType  string
	PathParams  []string
	NeedsAuth   bool

	Decorator  string
	Signature  string
	ReturnType string
	CallExpr   string
}

type MethodPlan struct {
	Name      string
	Body      string
	CustomKey string
}

type ServicePlan struct {
	Name        string
	ModuleName  string
	ClassName   string
	GetService  string
	Methods     []MethodPlan
	Imports     []string
	Routes      []RoutePlan
	ServiceName string
}

type RepoFinderPlan struct {
	Name      string
	CustomKey string
}

type RepoPlan struct {
	RepoName      string
	ModuleName    string
	PortClassName string
	PGClassName   string
	Finders       []RepoFinderPlan
}

type FastAPIPlan struct {
	ProjectName   string
	Version       string
	Models        []ModelPlan
	Routers       []ServicePlan
	ServiceStubs  []ServicePlan
	RepoStubs     []RepoPlan
	RouterModules []string
}
