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
