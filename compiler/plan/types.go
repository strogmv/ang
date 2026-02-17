package plan

type BuildPhase string

const (
	PhasePlan  BuildPhase = "plan"
	PhaseApply BuildPhase = "apply"
)

type PlanStatus string

const (
	StatusOK   PlanStatus = "ok"
	StatusWarn PlanStatus = "warn"
	StatusFail PlanStatus = "fail"
)

type BuildPlan struct {
	SchemaVersion  string        `json:"schemaVersion"`
	PlanVersion    string        `json:"planVersion"`
	GeneratedAtUTC string        `json:"generatedAtUtc"`
	WorkspaceRoot  string        `json:"workspaceRoot"`
	InputHash      string        `json:"inputHash"`
	CompilerHash   string        `json:"compilerHash"`
	BuildArgs      BuildArgs     `json:"buildArgs"`
	Preconditions  Preconditions `json:"preconditions"`
	Status         PlanStatus    `json:"status"`
	Diagnostics    []Diagnostic  `json:"diagnostics"`
	Steps          []PlanStep    `json:"steps"`
	Changes        []FileChange  `json:"changes"`
	Summary        PlanSummary   `json:"summary"`
}

type BuildArgs struct {
	Target     string `json:"target,omitempty"`
	Mode       string `json:"mode,omitempty"`
	BackendDir string `json:"backendDir,omitempty"`
	AutoApply  bool   `json:"autoApply"`
}

type Preconditions struct {
	WorkspaceHash string `json:"workspaceHash,omitempty"`
	GoVersion     string `json:"goVersion,omitempty"`
}

type PlanStep struct {
	Name       string `json:"name"`
	DurationMs int64  `json:"durationMs,omitempty"`
	Status     string `json:"status"`
	Message    string `json:"message,omitempty"`
}

type FileChange struct {
	Op         string `json:"op"`
	Path       string `json:"path"`
	BeforeHash string `json:"beforeHash,omitempty"`
	AfterHash  string `json:"afterHash,omitempty"`
	BeforeMode string `json:"beforeMode,omitempty"`
	AfterMode  string `json:"afterMode,omitempty"`
	ContentB64 string `json:"contentB64,omitempty"`
}

type Diagnostic struct {
	Level   string `json:"level"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
}

type PlanSummary struct {
	Add    int `json:"add"`
	Update int `json:"update"`
	Delete int `json:"delete"`
}
