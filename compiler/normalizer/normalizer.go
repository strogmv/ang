package normalizer

import (
	"fmt"
	"os"
	"strings"
)

type Warning struct {
	Kind         string `json:"kind"`
	Code         string `json:"code,omitempty"`
	Severity     string `json:"severity,omitempty"` // error, warn, info
	Message      string `json:"message"`
	Op           string `json:"op,omitempty"`
	Step         int    `json:"step,omitempty"`
	Action       string `json:"action,omitempty"`
	File         string `json:"file,omitempty"`
	Line         int    `json:"line,omitempty"`
	Column       int    `json:"column,omitempty"`
	CUEPath      string `json:"cue_path,omitempty"`
	Hint         string `json:"hint,omitempty"`
	DocsURL      string `json:"docs_url,omitempty"`
	CanAutoApply bool   `json:"can_auto_apply"`
	SuggestedFix []Fix  `json:"suggested_fix,omitempty"`
}

type Fix struct {
	Kind      string `json:"kind"` // replace, insert, delete, create
	File      string `json:"file,omitempty"`
	CUEPath   string `json:"cue_path,omitempty"`
	Text      string `json:"text"`
	Rationale string `json:"rationale,omitempty"`
}

type Normalizer struct {
	TypeMapping map[string]TypeConfig
	WarningSink func(Warning)
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
	}
}

func (n *Normalizer) Warn(w Warning) {
	if n.WarningSink != nil {
		n.WarningSink(w)
	}
}
