package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/normalizer"
)

type flowTestCase struct {
	ID             string `json:"id"`
	Service        string `json:"service"`
	Method         string `json:"method"`
	EndpointMethod string `json:"endpoint_method,omitempty"`
	EndpointPath   string `json:"endpoint_path,omitempty"`
	Kind           string `json:"kind"`
	Title          string `json:"title"`
	Condition      string `json:"condition,omitempty"`
	Throw          string `json:"throw,omitempty"`
	ExpectedStatus int    `json:"expected_status,omitempty"`
	StepFile       string `json:"step_file,omitempty"`
	StepLine       int    `json:"step_line,omitempty"`
	StepColumn     int    `json:"step_column,omitempty"`
	CUEPath        string `json:"cue_path,omitempty"`
}

type flowTestManifest struct {
	Status       string         `json:"status"`
	ProjectPath  string         `json:"project_path"`
	OutputPath   string         `json:"output_path"`
	Services     int            `json:"services"`
	Methods      int            `json:"methods"`
	Cases        int            `json:"cases"`
	CasesByKind  map[string]int `json:"cases_by_kind"`
	GeneratedAt  string         `json:"generated_at,omitempty"`
	Generator    string         `json:"generator"`
	WarningCount int            `json:"warning_count,omitempty"`
}

func runTest(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: ang test <gen>")
		return
	}
	switch args[0] {
	case "gen":
		runTestGen(args[1:])
	default:
		fmt.Printf("Unknown test command: %s\n", args[0])
		fmt.Println("Usage: ang test <gen>")
	}
}

func runTestGen(args []string) {
	projectPath := "."
	parseArgs := args
	if len(parseArgs) > 0 && !strings.HasPrefix(parseArgs[0], "-") {
		projectPath = parseArgs[0]
		parseArgs = parseArgs[1:]
	}

	fs := flag.NewFlagSet("test gen", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	outPath := fs.String("out", "tests/generated/flow_cases.json", "output path for generated flow test cases")
	if err := fs.Parse(parseArgs); err != nil {
		fmt.Printf("Test generation FAILED: %v\n", err)
		os.Exit(1)
	}

	_, services, endpoints, _, _, _, _, _, err := compiler.RunPipeline(projectPath)
	if err != nil {
		printStageFailure("Test generation FAILED", compiler.StageCUE, compiler.ErrCodeCUEPipeline, "run pipeline", err)
		os.Exit(1)
	}
	if emitDiagnostics(os.Stderr, compiler.LatestDiagnostics) {
		fmt.Println("Test generation FAILED due to diagnostic errors.")
		os.Exit(1)
	}

	endpointByRPC := make(map[string]normalizer.Endpoint)
	for _, ep := range endpoints {
		key := strings.ToLower(ep.ServiceName + "::" + ep.RPC)
		if _, ok := endpointByRPC[key]; !ok {
			endpointByRPC[key] = ep
		}
	}

	var allCases []flowTestCase
	methodCount := 0
	for _, svc := range services {
		for _, m := range svc.Methods {
			methodCount++
			ep := endpointByRPC[strings.ToLower(svc.Name+"::"+m.Name)]
			allCases = append(allCases, deriveFlowCases(svc.Name, m, ep)...)
		}
	}

	sort.Slice(allCases, func(i, j int) bool {
		if allCases[i].Service != allCases[j].Service {
			return allCases[i].Service < allCases[j].Service
		}
		if allCases[i].Method != allCases[j].Method {
			return allCases[i].Method < allCases[j].Method
		}
		return allCases[i].ID < allCases[j].ID
	})

	if err := os.MkdirAll(filepath.Dir(*outPath), 0o755); err != nil {
		fmt.Printf("Test generation FAILED: mkdir %s: %v\n", filepath.Dir(*outPath), err)
		os.Exit(1)
	}
	payload := map[string]any{
		"manifest": flowTestManifest{
			Status:      "generated",
			ProjectPath: filepath.Clean(projectPath),
			OutputPath:  filepath.ToSlash(filepath.Clean(*outPath)),
			Services:    len(services),
			Methods:     methodCount,
			Cases:       len(allCases),
			CasesByKind: summarizeCaseKinds(allCases),
			Generator:   "ang test gen",
		},
		"cases": allCases,
	}
	b, _ := json.MarshalIndent(payload, "", "  ")
	if err := os.WriteFile(*outPath, b, 0o644); err != nil {
		fmt.Printf("Test generation FAILED: write %s: %v\n", *outPath, err)
		os.Exit(1)
	}

	fmt.Printf("Generated flow test cases: %s\n", *outPath)
	fmt.Printf("Services: %d, Methods: %d, Cases: %d\n", len(services), methodCount, len(allCases))
	for kind, n := range summarizeCaseKinds(allCases) {
		fmt.Printf("  - %s: %d\n", kind, n)
	}
}

func summarizeCaseKinds(cases []flowTestCase) map[string]int {
	out := map[string]int{}
	for _, c := range cases {
		out[c.Kind]++
	}
	return out
}

func deriveFlowCases(service string, method normalizer.Method, ep normalizer.Endpoint) []flowTestCase {
	var out []flowTestCase
	var visit func([]normalizer.FlowStep)
	visit = func(steps []normalizer.FlowStep) {
		for idx, step := range steps {
			stepNo := idx + 1
			switch step.Action {
			case "logic.Check":
				cond, _ := step.Args["condition"].(string)
				throwMsg := fmt.Sprintf("%v", step.Args["throw"])
				c := flowTestCase{
					ID:             fmt.Sprintf("%s.%s.logic_check.%d", service, method.Name, stepNo),
					Service:        service,
					Method:         method.Name,
					EndpointMethod: ep.Method,
					EndpointPath:   ep.Path,
					Kind:           "logic_check",
					Title:          "logic.Check guard failure should return error",
					Condition:      cond,
					Throw:          throwMsg,
					ExpectedStatus: inferCheckStatus(throwMsg),
					StepFile:       step.File,
					StepLine:       step.Line,
					StepColumn:     step.Column,
					CUEPath:        step.CUEPath,
				}
				out = append(out, c)
			case "flow.If":
				cond, _ := step.Args["condition"].(string)
				base := flowTestCase{
					Service:        service,
					Method:         method.Name,
					EndpointMethod: ep.Method,
					EndpointPath:   ep.Path,
					Kind:           "flow_if",
					Condition:      cond,
					StepFile:       step.File,
					StepLine:       step.Line,
					StepColumn:     step.Column,
					CUEPath:        step.CUEPath,
				}
				thenCase := base
				thenCase.ID = fmt.Sprintf("%s.%s.flow_if_then.%d", service, method.Name, stepNo)
				thenCase.Title = "flow.If then branch should be covered"
				out = append(out, thenCase)

				elseCase := base
				elseCase.ID = fmt.Sprintf("%s.%s.flow_if_else.%d", service, method.Name, stepNo)
				elseCase.Title = "flow.If else branch should be covered"
				out = append(out, elseCase)
			case "repo.Find", "repo.Get":
				if errMsg, ok := step.Args["error"].(string); ok && strings.TrimSpace(errMsg) != "" {
					c := flowTestCase{
						ID:             fmt.Sprintf("%s.%s.repo_not_found.%d", service, method.Name, stepNo),
						Service:        service,
						Method:         method.Name,
						EndpointMethod: ep.Method,
						EndpointPath:   ep.Path,
						Kind:           "repo_not_found",
						Title:          "repo.Find/Get not-found branch should return 404",
						Throw:          errMsg,
						ExpectedStatus: 404,
						StepFile:       step.File,
						StepLine:       step.Line,
						StepColumn:     step.Column,
						CUEPath:        step.CUEPath,
					}
					out = append(out, c)
				}
			}
			if nested, ok := step.Args["_do"].([]normalizer.FlowStep); ok {
				visit(nested)
			}
			if nested, ok := step.Args["_then"].([]normalizer.FlowStep); ok {
				visit(nested)
			}
			if nested, ok := step.Args["_else"].([]normalizer.FlowStep); ok {
				visit(nested)
			}
		}
	}
	visit(method.Flow)
	return out
}

func inferCheckStatus(throwMsg string) int {
	m := strings.ToLower(strings.TrimSpace(throwMsg))
	if strings.Contains(m, "access denied") || strings.Contains(m, "forbidden") {
		return 403
	}
	return 400
}
