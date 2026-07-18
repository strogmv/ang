// Package expert defines deterministic, reviewable contracts for ANG's
// engineering expert layer. It contains no rule execution, CUE loading, LLM
// integration, or filesystem mutation.
package expert

import "encoding/json"

const SchemaV1 = "ang/expert-report/v1"

type TruthState string

const (
	TruthKnown    TruthState = "known"
	TruthUnknown  TruthState = "unknown"
	TruthConflict TruthState = "conflict"
)

type FindingStatus string

const (
	FindingConfirmed  FindingStatus = "confirmed"
	FindingHypothesis FindingStatus = "hypothesis"
	FindingUnknown    FindingStatus = "unknown"
	FindingConflict   FindingStatus = "conflict"
)

type ReportStatus string

const (
	ReportNoChange ReportStatus = "no_change"
	ReportAdvice   ReportStatus = "advice"
	ReportBlocked  ReportStatus = "blocked"
	ReportVerified ReportStatus = "verified"
	ReportFailed   ReportStatus = "failed"
)

type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

type ChangeOp string

const (
	ChangeInsert  ChangeOp = "insert"
	ChangeMerge   ChangeOp = "merge"
	ChangeReplace ChangeOp = "replace"
	ChangeDelete  ChangeOp = "delete"
)

// Evidence identifies a concrete source observation. ContentHash must be
// calculated without machine-specific absolute paths or timestamps.
type Evidence struct {
	ID          string   `json:"id"`
	SourceType  string   `json:"source_type"`
	SourcePath  string   `json:"source_path"`
	Line        int      `json:"line,omitempty"`
	Extractor   string   `json:"extractor"`
	Snippets    []string `json:"snippets,omitempty"`
	ContentHash string   `json:"content_hash"`
}

// Fact is a normalized observation or derivation over evidence.
type Fact struct {
	ID          string          `json:"id"`
	Kind        string          `json:"kind"`
	Subject     string          `json:"subject"`
	Predicate   string          `json:"predicate"`
	Value       json.RawMessage `json:"value,omitempty"`
	State       TruthState      `json:"state"`
	Confidence  float64         `json:"confidence"`
	EvidenceIDs []string        `json:"evidence_ids"`
}

type Finding struct {
	ID          string        `json:"id"`
	Code        string        `json:"code"`
	Severity    string        `json:"severity"`
	Summary     string        `json:"summary"`
	Origin      string        `json:"origin"`
	RuleID      string        `json:"rule_id,omitempty"`
	FactIDs     []string      `json:"fact_ids,omitempty"`
	EvidenceIDs []string      `json:"evidence_ids,omitempty"`
	Confidence  float64       `json:"confidence"`
	Status      FindingStatus `json:"status"`
}

type Assertion struct {
	Kind      string          `json:"kind"`
	Subject   string          `json:"subject"`
	Predicate string          `json:"predicate"`
	Expected  json.RawMessage `json:"expected,omitempty"`
	Message   string          `json:"message,omitempty"`
}

// Change is a typed intent-only patch. Applying it is deliberately outside
// this package and requires path, hash, validation, and approval gates.
type Change struct {
	Op         ChangeOp        `json:"op"`
	File       string          `json:"file"`
	CUEPath    string          `json:"cue_path"`
	Value      json.RawMessage `json:"value,omitempty"`
	BeforeHash string          `json:"before_hash,omitempty"`
	Rationale  string          `json:"rationale"`
}

type Proposal struct {
	ID               string      `json:"id"`
	Goal             string      `json:"goal"`
	RuleIDs          []string    `json:"rule_ids,omitempty"`
	FindingIDs       []string    `json:"finding_ids,omitempty"`
	Changes          []Change    `json:"changes"`
	Preconditions    []Assertion `json:"preconditions,omitempty"`
	Postconditions   []Assertion `json:"postconditions,omitempty"`
	Risk             RiskLevel   `json:"risk"`
	RequiresApproval bool        `json:"requires_approval"`
}

type RuleTrace struct {
	Origin         string   `json:"origin"`
	RuleID         string   `json:"rule_id"`
	MatchedFacts   []string `json:"matched_facts"`
	MissingFacts   []string `json:"missing_facts,omitempty"`
	Result         string   `json:"result"`
	ProducedIDs    []string `json:"produced_ids,omitempty"`
	RejectedReason string   `json:"rejected_reason,omitempty"`
}

type VerificationResult struct {
	Check   string `json:"check"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type Diagnostic struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Origin   string `json:"origin"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	CUEPath  string `json:"cue_path,omitempty"`
}

// Report is the public, versioned result returned by an expert decision run.
type Report struct {
	Schema            string               `json:"schema"`
	Goal              string               `json:"goal"`
	Status            ReportStatus         `json:"status"`
	CompilerVersion   string               `json:"compiler_version"`
	FactsHash         string               `json:"facts_hash"`
	KnowledgeVersions []string             `json:"knowledge_versions"`
	Findings          []Finding            `json:"findings"`
	Proposals         []Proposal           `json:"proposals"`
	Trace             []RuleTrace          `json:"trace"`
	Verification      []VerificationResult `json:"verification"`
	Diagnostics       []Diagnostic         `json:"diagnostics"`
}

// ReconcileReportStatus derives a safe status after findings from all sources
// have been assembled. A conflict is never downgraded to ordinary advice.
func ReconcileReportStatus(status ReportStatus, findings []Finding) ReportStatus {
	if status == ReportFailed || status == ReportBlocked || status == ReportVerified {
		return status
	}
	for _, finding := range findings {
		if finding.Status == FindingConflict {
			return ReportBlocked
		}
	}
	if len(findings) > 0 {
		return ReportAdvice
	}
	return ReportNoChange
}
