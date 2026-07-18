package expert

import "encoding/json"

const SchemaV2 = "ang/expert-report/v2"

const IntentTargetProjectCUERoot = "project_cue_root"

// IntentTarget addresses project-local CUE intent relative to a caller-resolved root.
type IntentTarget struct {
	Kind         string `json:"kind"`
	RelativePath string `json:"relative_path"`
}

// ChangeV2 is a typed intent-only patch for payment-provider and other project-rooted targets.
type ChangeV2 struct {
	Op         ChangeOp        `json:"op"`
	Target     IntentTarget    `json:"target"`
	CUEPath    string          `json:"cue_path"`
	Value      json.RawMessage `json:"value,omitempty"`
	BeforeHash string          `json:"before_hash,omitempty"`
	Rationale  string          `json:"rationale"`
}

// ProposalV2 carries review-required intent changes addressed through typed targets.
type ProposalV2 struct {
	ID               string      `json:"id"`
	Goal             string      `json:"goal"`
	RuleIDs          []string    `json:"rule_ids,omitempty"`
	FindingIDs       []string    `json:"finding_ids,omitempty"`
	Changes          []ChangeV2  `json:"changes"`
	Preconditions    []Assertion `json:"preconditions,omitempty"`
	Postconditions   []Assertion `json:"postconditions,omitempty"`
	Risk             RiskLevel   `json:"risk"`
	RequiresApproval bool        `json:"requires_approval"`
}

// ReportV2 is the payment-provider-safe expert report contract.
type ReportV2 struct {
	Schema            string               `json:"schema"`
	Goal              string               `json:"goal"`
	Status            ReportStatus         `json:"status"`
	CompilerVersion   string               `json:"compiler_version"`
	FactsHash         string               `json:"facts_hash"`
	KnowledgeVersions []string             `json:"knowledge_versions"`
	Findings          []Finding            `json:"findings"`
	Proposals         []ProposalV2         `json:"proposals"`
	Trace             []RuleTrace          `json:"trace"`
	Verification      []VerificationResult `json:"verification"`
	Diagnostics       []Diagnostic         `json:"diagnostics"`
}
