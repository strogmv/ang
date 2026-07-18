package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/strogmv/ang/compiler/expert"
)

const (
	expertAnalyzePath  = "/v1/analyze"
	expertOutcomesPath = "/v1/outcomes"
)

// ExpertClientConfig configures HTTP calls to a local or hosted Expert Runtime.
type ExpertClientConfig struct {
	BaseURL string
	Timeout time.Duration
}

// ExpertAnalyzeRequest is the versioned analyze envelope sent to Expert Runtime.
type ExpertAnalyzeRequest struct {
	Schema          string          `json:"schema"`
	RequestID       string          `json:"request_id"`
	Goal            string          `json:"goal"`
	CompilerVersion string          `json:"compiler_version,omitempty"`
	Facts           json.RawMessage `json:"facts"`
	Diagnostics     json.RawMessage `json:"diagnostics,omitempty"`
	PackIDs         []string        `json:"pack_ids,omitempty"`
}

// ValidatedExpertReport is a locally verified Expert Runtime report.
type ValidatedExpertReport struct {
	ReportSchema   string
	Report         expert.Report
	ReportV2       expert.ReportV2
	RuntimeVersion string
	RequestID      string
	FactsHash      string
	ReportHash     string
}

// ExpertAnalyzeScope optionally validates v2 proposal targets against a project cue root.
type ExpertAnalyzeScope struct {
	ProjectPath string
	CueRoot     string
}

// ExpertOutcomeRequest mirrors ang/expert-outcome/v1 for POST /v1/outcomes.
type ExpertOutcomeRequest struct {
	Schema             string                   `json:"schema"`
	RunID              string                   `json:"run_id"`
	ScopeID            string                   `json:"scope_id"`
	Goal               string                   `json:"goal"`
	CompilerVersion    string                   `json:"compiler_version"`
	FactsBeforeHash    string                   `json:"facts_before_hash"`
	ReportHash         string                   `json:"report_hash"`
	KnowledgeVersions  []string                 `json:"knowledge_versions"`
	ProposalDecisions  []ExpertProposalDecision `json:"proposal_decisions"`
	Verification       []ExpertVerification     `json:"verification"`
	FactsAfterHash         string                   `json:"facts_after_hash,omitempty"`
	OutputManifestHash     string                   `json:"output_manifest_hash,omitempty"`
	BlockingFindingCodes   []string                 `json:"blocking_finding_codes,omitempty"`
	UnresolvedFindingCodes []string                 `json:"unresolved_finding_codes,omitempty"`
	FinalStatus            string                   `json:"final_status"`
}

type ExpertProposalDecision struct {
	ProposalID string `json:"proposal_id"`
	Decision   string `json:"decision"`
	ReasonCode string `json:"reason_code,omitempty"`
}

type ExpertVerification struct {
	Check  string   `json:"check"`
	Status string   `json:"status"`
	Codes  []string `json:"codes,omitempty"`
}

type expertRuntimeResponse struct {
	Schema         string          `json:"schema"`
	RequestID      string          `json:"request_id"`
	RuntimeVersion string          `json:"runtime_version"`
	Report         json.RawMessage `json:"report"`
}

type expertOutcomeResponse struct {
	Schema   string `json:"schema"`
	RunID    string `json:"run_id"`
	Accepted bool   `json:"accepted"`
}

// Analyze posts canonical facts bytes to POST {baseURL}/v1/analyze and validates the response.
func Analyze(ctx context.Context, cfg ExpertClientConfig, request ExpertAnalyzeRequest, factsHash string, scope ...ExpertAnalyzeScope) (ValidatedExpertReport, error) {
	baseURL, err := validateExpertBaseURL(cfg.BaseURL)
	if err != nil {
		return ValidatedExpertReport{}, err
	}
	if strings.TrimSpace(request.RequestID) == "" {
		return ValidatedExpertReport{}, fmt.Errorf("expert analyze request_id must not be empty")
	}
	if len(request.Facts) == 0 || !json.Valid(request.Facts) {
		return ValidatedExpertReport{}, fmt.Errorf("expert analyze facts must be valid JSON")
	}
	if strings.TrimSpace(factsHash) == "" {
		sum := sha256.Sum256(request.Facts)
		factsHash = hex.EncodeToString(sum[:])
	}
	endpoint := strings.TrimRight(baseURL.String(), "/") + expertAnalyzePath
	responseJSON, err := postExpertJSON(ctx, cfg, endpoint, request)
	if err != nil {
		return ValidatedExpertReport{}, err
	}
	var analyzeScope *ExpertAnalyzeScope
	if len(scope) > 0 {
		analyzeScope = &scope[0]
	}
	return validateExpertAnalyzeResponse(responseJSON, request, factsHash, analyzeScope)
}

// RecordOutcome posts one verified outcome to POST {baseURL}/v1/outcomes.
func RecordOutcome(ctx context.Context, cfg ExpertClientConfig, outcome ExpertOutcomeRequest) error {
	baseURL, err := validateExpertBaseURL(cfg.BaseURL)
	if err != nil {
		return err
	}
	endpoint := strings.TrimRight(baseURL.String(), "/") + expertOutcomesPath
	responseJSON, err := postExpertJSON(ctx, cfg, endpoint, outcome)
	if err != nil {
		return err
	}
	var response expertOutcomeResponse
	if err := json.Unmarshal(responseJSON, &response); err != nil {
		return fmt.Errorf("decode expert outcome response: %w", err)
	}
	if response.Schema != "ang/expert-outcome-response/v1" {
		return fmt.Errorf("invalid expert outcome response schema %q", response.Schema)
	}
	if response.RunID != outcome.RunID {
		return fmt.Errorf("expert outcome response run_id %q does not match request", response.RunID)
	}
	if !response.Accepted {
		return fmt.Errorf("expert outcome was not accepted")
	}
	return nil
}

func validateExpertBaseURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("expert base URL must not be empty")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse expert base URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("expert base URL must use http or https")
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return nil, fmt.Errorf("expert base URL must include a host")
	}
	host := strings.ToLower(parsed.Hostname())
	loopback := host == "127.0.0.1" || host == "localhost" || host == "::1"
	if !loopback && parsed.Scheme != "https" {
		return nil, fmt.Errorf("non-loopback expert base URL must use https")
	}
	return parsed, nil
}

func postExpertJSON(ctx context.Context, cfg ExpertClientConfig, endpoint string, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode expert request: %w", err)
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build expert HTTP request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return fmt.Errorf("expert HTTP redirects are not allowed")
		},
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call expert HTTP endpoint: %w", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("read expert HTTP response: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("expert HTTP endpoint returned %s: %s", response.Status, strings.TrimSpace(string(data)))
	}
	return data, nil
}

func validateExpertAnalyzeResponse(data []byte, request ExpertAnalyzeRequest, expectedFactsHash string, scope *ExpertAnalyzeScope) (ValidatedExpertReport, error) {
	var response expertRuntimeResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return ValidatedExpertReport{}, fmt.Errorf("decode expert runtime response: %w", err)
	}
	if response.Schema != expertResponseSchema {
		return ValidatedExpertReport{}, fmt.Errorf("invalid expert runtime response schema %q", response.Schema)
	}
	if response.RequestID != request.RequestID {
		return ValidatedExpertReport{}, fmt.Errorf("expert runtime response request_id %q does not match request", response.RequestID)
	}
	if strings.TrimSpace(response.RuntimeVersion) == "" {
		return ValidatedExpertReport{}, fmt.Errorf("expert runtime response runtime_version is empty")
	}
	schema, err := expert.ReportSchema(response.Report)
	if err != nil {
		return ValidatedExpertReport{}, fmt.Errorf("expert runtime report: %w", err)
	}
	validated := ValidatedExpertReport{
		ReportSchema:   schema,
		RuntimeVersion: response.RuntimeVersion,
		RequestID:      request.RequestID,
		FactsHash:      expectedFactsHash,
	}
	switch schema {
	case expert.SchemaV1:
		var report expert.Report
		if err := json.Unmarshal(response.Report, &report); err != nil {
			return ValidatedExpertReport{}, fmt.Errorf("decode expert runtime report: %w", err)
		}
		if report.Goal != request.Goal {
			return ValidatedExpertReport{}, fmt.Errorf("expert runtime report goal %q does not match request", report.Goal)
		}
		if report.CompilerVersion != request.CompilerVersion {
			return ValidatedExpertReport{}, fmt.Errorf("expert runtime report compiler_version %q does not match request", report.CompilerVersion)
		}
		if report.FactsHash != expectedFactsHash {
			return ValidatedExpertReport{}, fmt.Errorf("expert runtime report facts_hash does not match supplied facts")
		}
		canonical, err := expert.Canonicalize(report)
		if err != nil {
			return ValidatedExpertReport{}, fmt.Errorf("canonicalize expert runtime report: %w", err)
		}
		if err := expert.ValidateReport(canonical); err != nil {
			return ValidatedExpertReport{}, fmt.Errorf("validate expert runtime report: %w", err)
		}
		reportHash, err := expert.Hash(canonical)
		if err != nil {
			return ValidatedExpertReport{}, fmt.Errorf("hash expert runtime report: %w", err)
		}
		validated.Report = canonical
		validated.ReportHash = reportHash
	case expert.SchemaV2:
		var report expert.ReportV2
		if err := json.Unmarshal(response.Report, &report); err != nil {
			return ValidatedExpertReport{}, fmt.Errorf("decode expert runtime report: %w", err)
		}
		if report.Goal != request.Goal {
			return ValidatedExpertReport{}, fmt.Errorf("expert runtime report goal %q does not match request", report.Goal)
		}
		if report.CompilerVersion != request.CompilerVersion {
			return ValidatedExpertReport{}, fmt.Errorf("expert runtime report compiler_version %q does not match request", report.CompilerVersion)
		}
		if report.FactsHash != expectedFactsHash {
			return ValidatedExpertReport{}, fmt.Errorf("expert runtime report facts_hash does not match supplied facts")
		}
		canonical, err := expert.CanonicalizeV2(report)
		if err != nil {
			return ValidatedExpertReport{}, fmt.Errorf("canonicalize expert runtime report: %w", err)
		}
		if err := expert.ValidateReportV2(canonical); err != nil {
			return ValidatedExpertReport{}, fmt.Errorf("validate expert runtime report: %w", err)
		}
		if scope != nil && strings.TrimSpace(scope.ProjectPath) != "" {
			if err := expert.ValidateReportV2Scope(scope.ProjectPath, scope.CueRoot, canonical); err != nil {
				return ValidatedExpertReport{}, fmt.Errorf("validate expert proposal scope: %w", err)
			}
		}
		reportHash, err := expert.HashV2(canonical)
		if err != nil {
			return ValidatedExpertReport{}, fmt.Errorf("hash expert runtime report: %w", err)
		}
		validated.ReportV2 = canonical
		validated.ReportHash = reportHash
	default:
		return ValidatedExpertReport{}, fmt.Errorf("unsupported expert runtime report schema %q", schema)
	}
	return validated, nil
}

func newExpertRequestID(prefix string) (string, error) {
	if strings.TrimSpace(prefix) == "" {
		prefix = "ang.expert"
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate expert request id: %w", err)
	}
	return prefix + "." + hex.EncodeToString(buf), nil
}

func validateExpertMode(mode string) error {
	switch strings.TrimSpace(mode) {
	case "off", "shadow", "advise", "gate":
		return nil
	default:
		return fmt.Errorf("unsupported expert mode %q", mode)
	}
}
