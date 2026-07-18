package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/expert"
	"github.com/strogmv/ang/compiler/facts"
)

const (
	expertRequestSchema  = "ang/expert-request/v1"
	expertResponseSchema = "ang/expert-response/v1"
	expertRequestID      = "ang.advise.v1"
)

type expertRuntimeRequest = ExpertAnalyzeRequest

// buildExternalAdviceReport calls a separately installed Expert Runtime over
// one-request/one-response stdio. It intentionally does not provide project
// paths or write authority to that process.
func buildExternalAdviceReport(projectPath, goal, factsPath, command string, commandArgs, packIDs []string) (expert.Report, error) {
	request, factsHash, err := buildExpertRuntimeRequest(projectPath, goal, factsPath, packIDs)
	if err != nil {
		return expert.Report{}, err
	}
	responseJSON, err := executeExpertRuntime(command, commandArgs, request)
	if err != nil {
		return expert.Report{}, err
	}
	return validateExpertRuntimeResponse(responseJSON, request, factsHash)
}

func buildHTTPAdviceReport(projectPath, goal, factsPath, endpoint string, packIDs []string) (expert.Report, error) {
	request, factsHash, err := buildExpertRuntimeRequest(projectPath, goal, factsPath, packIDs)
	if err != nil {
		return expert.Report{}, err
	}
	responseJSON, err := executeExpertHTTP(endpoint, request)
	if err != nil {
		return expert.Report{}, err
	}
	return validateExpertRuntimeResponse(responseJSON, request, factsHash)
}

func buildExpertRuntimeRequest(projectPath, goal, factsPath string, packIDs []string) (expertRuntimeRequest, string, error) {
	if strings.TrimSpace(factsPath) == "" {
		return expertRuntimeRequest{}, "", fmt.Errorf("Expert Runtime requires --facts")
	}
	envelope, err := loadFactsEnvelope(factsPath)
	if err != nil {
		return expertRuntimeRequest{}, "", fmt.Errorf("load facts: %w", err)
	}
	factJSON, err := facts.CanonicalJSON(*envelope)
	if err != nil {
		return expertRuntimeRequest{}, "", fmt.Errorf("canonicalize facts: %w", err)
	}
	sum := sha256.Sum256(factJSON)
	factsHash := hex.EncodeToString(sum[:])

	_, _ = compiler.RunSemanticPhases(projectPath)
	diagnostics, err := json.Marshal(compiler.LatestDiagnostics)
	if err != nil {
		return expertRuntimeRequest{}, "", fmt.Errorf("encode compiler diagnostics: %w", err)
	}
	request := expertRuntimeRequest{
		Schema: expertRequestSchema, RequestID: expertRequestID, Goal: goal,
		CompilerVersion: compiler.Version, Facts: factJSON, Diagnostics: diagnostics,
		PackIDs: append([]string(nil), packIDs...),
	}
	return request, factsHash, nil
}

func executeExpertRuntime(command string, commandArgs []string, request expertRuntimeRequest) ([]byte, error) {
	input, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("encode Expert Runtime request: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	process := exec.CommandContext(ctx, command, commandArgs...)
	process.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	process.Stdout = &stdout
	process.Stderr = &stderr
	if err := process.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("Expert Runtime timed out: %w", ctx.Err())
		}
		return nil, fmt.Errorf("Expert Runtime failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

func executeExpertHTTP(endpoint string, request expertRuntimeRequest) ([]byte, error) {
	input, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("encode Expert Runtime request: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(input))
	if err != nil {
		return nil, fmt.Errorf("build Expert Runtime HTTP request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("call Expert Runtime HTTP endpoint: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("read Expert Runtime HTTP response: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Expert Runtime HTTP endpoint returned %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func validateExpertRuntimeResponse(data []byte, request expertRuntimeRequest, expectedFactsHash string) (expert.Report, error) {
	validated, err := validateExpertAnalyzeResponse(data, request, expectedFactsHash, nil)
	if err != nil {
		return expert.Report{}, err
	}
	if validated.ReportSchema == expert.SchemaV2 {
		return expert.Report{
			Schema:            validated.ReportSchema,
			Goal:              validated.ReportV2.Goal,
			Status:            validated.ReportV2.Status,
			CompilerVersion:   validated.ReportV2.CompilerVersion,
			FactsHash:         validated.ReportV2.FactsHash,
			KnowledgeVersions: validated.ReportV2.KnowledgeVersions,
			Findings:          validated.ReportV2.Findings,
			Trace:             validated.ReportV2.Trace,
			Verification:      validated.ReportV2.Verification,
			Diagnostics:       validated.ReportV2.Diagnostics,
		}, nil
	}
	return validated.Report, nil
}
