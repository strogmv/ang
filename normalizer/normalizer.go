package normalizer

import (
	"fmt"
	"os"
	"strings"
)

type Warning struct {
	Kind     string `json:"kind"`
	Code     string `json:"code,omitempty"`
	Severity string `json:"severity,omitempty"` // error, warn, info
	Message  string `json:"message"`
	Op       string `json:"op,omitempty"`
	Step     int    `json:"step,omitempty"`
	Action   string `json:"action,omitempty"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Column   int    `json:"column,omitempty"`
	CUEPath  string `json:"cue_path,omitempty"`
	// Path is the human-readable source location (file:flow[N]) for AI output.
	Path string `json:"path,omitempty"`
	Hint string `json:"hint,omitempty"`
	// Allowed is the list of permitted values when a constraint is violated.
	Allowed      []string `json:"allowed,omitempty"`
	DocsURL      string   `json:"docs_url,omitempty"`
	CanAutoApply bool     `json:"can_auto_apply"`
	SuggestedFix []Fix    `json:"suggested_fix,omitempty"`
}

type Fix struct {
	Kind string `json:"kind,omitempty"` // legacy alias for Op
	// Op is machine-applicable edit intent: merge|replace|insert|delete.
	Op string `json:"op,omitempty"`
	// File/CUEPath pin the patch target in source.
	File    string `json:"file,omitempty"`
	CUEPath string `json:"cue_path,omitempty"`
	// Value is the patch payload (usually an object for merge).
	Value any `json:"value,omitempty"`
	// Text keeps backward compatibility with older fix consumers.
	Text string `json:"text,omitempty"`
	// Before/After provide explicit transformation hints for AI agents.
	Before    string `json:"before,omitempty"`
	After     string `json:"after,omitempty"`
	Rationale string `json:"rationale,omitempty"`
}

type Normalizer struct {
	TypeMapping map[string]TypeConfig
	WarningSink func(Warning)
	Scopes      []ScopeDef
	scopeIndex  map[string][]string
	Policies    map[string]PolicyDef
	RepoNames   map[string]struct{}
	EventNames  map[string]struct{}

	ArchitectureMode       string
	ArchitectureAllowCross map[string]map[string]struct{}
}

type TypeConfig struct {
	GoType     string
	Package    string
	SQLType    string
	NullHelper string
}

func New() *Normalizer {
	return &Normalizer{
		TypeMapping: make(map[string]TypeConfig),
		WarningSink: func(w Warning) {
			label := strings.ToUpper(w.Kind)
			if label == "" {
				label = "WARNING"
			}
			if w.Op != "" && w.Step > 0 {
				fmt.Fprintf(os.Stderr, "⚠️  %s WARNING: [%s step %d] %s\n", label, w.Op, w.Step, w.Message)
				return
			}
			fmt.Fprintf(os.Stderr, "⚠️  %s WARNING: %s\n", label, w.Message)
		},
		Scopes:                 nil,
		scopeIndex:             make(map[string][]string),
		Policies:               make(map[string]PolicyDef),
		ArchitectureMode:       "strict",
		ArchitectureAllowCross: make(map[string]map[string]struct{}),
	}
}

func (n *Normalizer) Warn(w Warning) {
	if n.WarningSink != nil {
		n.WarningSink(w)
	}
}
