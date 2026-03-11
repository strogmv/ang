package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	transform "github.com/strogmv/ang-transform/pkg/transform"
)

type importConfidence string

const (
	confidenceHigh   importConfidence = "high"
	confidenceMedium importConfidence = "medium"
	confidenceLow    importConfidence = "low"
)

type importProfile string

const (
	importProfileHexagonal    importProfile = "hexagonal"
	importProfileLayered      importProfile = "layered"
	importProfileLegacyMono   importProfile = "legacy_monolith"
	importProfileMicroservice importProfile = "microservice"
)

type importJavaOptions struct {
	Root              string
	OutDir            string
	Report            bool
	ReportOut         string
	Diff              bool
	Update            bool
	Incremental       bool
	JavaParserBackend string
	Profile           importProfile
	Verify            bool
	VerifyOpenAPI     string
	ContractTestCmd   string
}

type importOpenAPIOptions struct {
	SourcePath   string
	OutAPIDir    string
	OutDomain    string
	Report       bool
	ReportOut    string
	Diff         bool
	Update       bool
	GroupByOwner bool
}

type javaImportIR struct {
	Profile        string              `json:"profile"`
	SourcePath     string              `json:"source_path"`
	GeneratedAtUTC string              `json:"generated_at_utc"`
	Entities       []importEntityIR    `json:"entities"`
	Operations     []importOperationIR `json:"operations"`
	Endpoints      []importEndpointIR  `json:"endpoints"`
	ErrorContracts []FactErrorContract `json:"error_contracts,omitempty"`
	SecurityRules  []FactSecurityRule  `json:"security_rules,omitempty"`
	Events         []FactEvent         `json:"events,omitempty"`
	Mappers        []FactMapper        `json:"mappers,omitempty"`
	Constants      []FactConstant      `json:"constants,omitempty"`
	Enums          []FactEnum          `json:"enums,omitempty"`
}

type importEntityIR struct {
	Name       string           `json:"name"`
	TableHint  string           `json:"table_hint,omitempty"`
	Fields     []FactField      `json:"fields,omitempty"`
	Sources    []string         `json:"sources,omitempty"`
	Confidence importConfidence `json:"confidence"`
}

type importOperationIR struct {
	Name          string           `json:"name"`
	Service       string           `json:"service,omitempty"`
	EntryKind     string           `json:"entry_kind,omitempty"`
	EntryRef      string           `json:"entry_ref,omitempty"`
	HTTPMethod    string           `json:"http_method,omitempty"`
	HTTPPath      string           `json:"http_path,omitempty"`
	AuthExpr      string           `json:"auth_expr,omitempty"`
	Transactional bool             `json:"transactional,omitempty"`
	TxReadOnly    bool             `json:"tx_read_only,omitempty"`
	Inputs        []FactField      `json:"inputs,omitempty"`
	Outputs       []FactField      `json:"outputs,omitempty"`
	Calls         []FactCallRef    `json:"calls,omitempty"`
	ConstantsUsed []string         `json:"constants_used,omitempty"`
	EnumsUsed     []string         `json:"enums_used,omitempty"`
	Sources       []string         `json:"sources,omitempty"`
	Confidence    importConfidence `json:"confidence"`
}

type importEndpointIR struct {
	Operation     string           `json:"operation"`
	EntryKind     string           `json:"entry_kind,omitempty"`
	EntryRef      string           `json:"entry_ref,omitempty"`
	Method        string           `json:"method,omitempty"`
	Path          string           `json:"path,omitempty"`
	AuthExpr      string           `json:"auth_expr,omitempty"`
	Transactional bool             `json:"transactional,omitempty"`
	TxReadOnly    bool             `json:"tx_read_only,omitempty"`
	Confidence    importConfidence `json:"confidence"`
}

type importAdapterStatus struct {
	Name     string `json:"name"`
	Enabled  bool   `json:"enabled"`
	Detected int    `json:"detected"`
	Note     string `json:"note,omitempty"`
}

type importFrameworkStatus struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Note    string `json:"note,omitempty"`
}

type importConflict struct {
	Artifact        string `json:"artifact"`
	Field           string `json:"field"`
	PreferredSource string `json:"preferred_source"`
	PreferredValue  string `json:"preferred_value,omitempty"`
	OtherSource     string `json:"other_source"`
	OtherValue      string `json:"other_value,omitempty"`
	Resolution      string `json:"resolution"`
}

type importTodo struct {
	Artifact string `json:"artifact"`
	Reason   string `json:"reason"`
	Hint     string `json:"hint"`
}

type importDiffItem struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}

type importSummary struct {
	Entities         int `json:"entities"`
	Operations       int `json:"operations"`
	Endpoints        int `json:"endpoints"`
	Constants        int `json:"constants"`
	Enums            int `json:"enums"`
	Conflicts        int `json:"conflicts"`
	Todos            int `json:"todos"`
	HighConfidence   int `json:"high_confidence"`
	MediumConfidence int `json:"medium_confidence"`
	LowConfidence    int `json:"low_confidence"`
}

type importMappingGap struct {
	Artifact string `json:"artifact"`
	Reason   string `json:"reason"`
	Source   string `json:"source,omitempty"`
	FixHint  string `json:"fix_hint,omitempty"`
}

type importMappingReport struct {
	MappedExact int                `json:"mapped_exact"`
	MappedLossy int                `json:"mapped_lossy"`
	Unmapped    []importMappingGap `json:"unmapped,omitempty"`
}

type javaImportReport struct {
	Schema         string                  `json:"schema"`
	SourcePath     string                  `json:"source_path"`
	Profile        string                  `json:"profile"`
	GeneratedAtUTC string                  `json:"generated_at_utc"`
	Adapters       []importAdapterStatus   `json:"adapters"`
	Frameworks     []importFrameworkStatus `json:"frameworks"`
	Summary        importSummary           `json:"summary"`
	Mapping        importMappingReport     `json:"mapping"`
	Verification   importVerification      `json:"verification"`
	Conflicts      []importConflict        `json:"conflicts,omitempty"`
	Todos          []importTodo            `json:"todos,omitempty"`
	Diff           []importDiffItem        `json:"diff,omitempty"`
}

type importVerification struct {
	Enabled bool                      `json:"enabled"`
	Checks  []importVerificationCheck `json:"checks,omitempty"`
}

type importVerificationCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Details string `json:"details,omitempty"`
}

func runImport(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage:")
		fmt.Println("  ang import java [path] [--report] [--report-out report.json] [--diff] [--update] [--out-dir cue/import] [--incremental] [--java-parser auto|regex|treesitter|antlr] [--profile layered|hexagonal|legacy_monolith|microservice]")
		fmt.Println("  ang import openapi [path/to/openapi.yml] [--report] [--report-out report.json] [--diff] [--update] [--out-api-dir cue/api] [--out-domain-file cue/domain/entities.cue]")
		os.Exit(1)
	}
	sub := strings.ToLower(strings.TrimSpace(args[0]))
	switch sub {
	case "java":
		runImportJava(args[1:])
	case "openapi":
		runImportOpenAPI(args[1:])
	default:
		fmt.Printf("import: unknown source %q (supported: java, openapi)\n", sub)
		os.Exit(1)
	}
}

func parseImportProfile(raw string) (importProfile, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", string(importProfileLayered):
		return importProfileLayered, nil
	case string(importProfileHexagonal):
		return importProfileHexagonal, nil
	case string(importProfileLegacyMono):
		return importProfileLegacyMono, nil
	case string(importProfileMicroservice):
		return importProfileMicroservice, nil
	default:
		return "", fmt.Errorf("unknown profile %q", raw)
	}
}

func runImportJava(args []string) {
	parseArgs := append([]string(nil), args...)
	fs := flag.NewFlagSet("import java", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	report := fs.Bool("report", false, "print import report JSON to stdout")
	reportOut := fs.String("report-out", "", "write report JSON to file")
	diff := fs.Bool("diff", false, "show contract-layer diff against existing generated import CUE")
	update := fs.Bool("update", false, "write generated import contract CUE into out-dir")
	outDir := fs.String("out-dir", "cue/import", "directory for generated import contract CUE")
	incremental := fs.Bool("incremental", false, "import only changed source files in git working tree")
	javaParser := fs.String("java-parser", "auto", "java parser backend: auto|regex|treesitter|antlr")
	profileRaw := fs.String("profile", string(importProfileLayered), "project profile: layered|hexagonal|legacy_monolith|microservice")
	verify := fs.Bool("verify", false, "run verification loop (snapshot parity + optional contract tests)")
	verifyOpenAPI := fs.String("verify-openapi", "", "explicit OpenAPI snapshot path for verification parity check")
	contractTestCmd := fs.String("contract-test-cmd", "", "command to run contract tests in source root")

	var positional []string
	filtered := parseArgs[:0]
	for _, a := range parseArgs {
		if !strings.HasPrefix(a, "-") && len(filtered) == 0 && !strings.Contains(a, "=") {
			positional = append(positional, a)
		} else {
			filtered = append(filtered, a)
		}
	}
	if err := fs.Parse(filtered); err != nil {
		fmt.Fprintf(os.Stderr, "import java: %v\n", err)
		os.Exit(1)
	}

	sourcePath := "."
	if len(positional) > 0 {
		sourcePath = positional[0]
	} else if fs.NArg() > 0 {
		sourcePath = fs.Arg(0)
	}
	sourcePath = filepath.Clean(sourcePath)

	profile, err := parseImportProfile(*profileRaw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "import java: %v\n", err)
		os.Exit(1)
	}

	opts := importJavaOptions{
		Root:              sourcePath,
		OutDir:            filepath.Clean(*outDir),
		Report:            *report,
		ReportOut:         strings.TrimSpace(*reportOut),
		Diff:              *diff,
		Update:            *update,
		Incremental:       *incremental,
		JavaParserBackend: strings.TrimSpace(*javaParser),
		Profile:           profile,
		Verify:            *verify,
		VerifyOpenAPI:     strings.TrimSpace(*verifyOpenAPI),
		ContractTestCmd:   strings.TrimSpace(*contractTestCmd),
	}

	topts, err := localJavaImportOptionsToTransform(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "import java FAILED (convert opts): %v\n", err)
		os.Exit(1)
	}
	tir, treport, files, err := transform.BuildJavaImportPipeline(topts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "import java FAILED: %v\n", err)
		os.Exit(1)
	}
	ir, reportDoc, err := transformImportArtifactsToLocal(tir, treport)
	if err != nil {
		fmt.Fprintf(os.Stderr, "import java FAILED (convert): %v\n", err)
		os.Exit(1)
	}

	diffItems, err := computeImportDiff(opts.OutDir, files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "import java FAILED (diff): %v\n", err)
		os.Exit(1)
	}
	reportDoc.Diff = diffItems
	if opts.Diff {
		printImportDiff(diffItems)
	}

	if opts.Update {
		if err := writeImportCueFiles(opts.OutDir, files); err != nil {
			fmt.Fprintf(os.Stderr, "import java FAILED (update): %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Updated import contract layer in %s (%d files).\n", opts.OutDir, len(files))
	}

	if opts.ReportOut != "" {
		if err := writeImportReport(opts.ReportOut, reportDoc); err != nil {
			fmt.Fprintf(os.Stderr, "import java FAILED (report-out): %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Wrote report: %s\n", opts.ReportOut)
	}

	if opts.Report {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(reportDoc)
	}

	if !opts.Report && opts.ReportOut == "" && !opts.Diff && !opts.Update {
		fmt.Printf("Imported Java -> IR (%d entities, %d operations, %d endpoints).\n", len(ir.Entities), len(ir.Operations), len(ir.Endpoints))
		fmt.Println("Tip: run `ang import java --report --diff --update` to inspect and apply contract-layer updates.")
	}
}

func runImportOpenAPI(args []string) {
	parseArgs := append([]string(nil), args...)
	fs := flag.NewFlagSet("import openapi", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	report := fs.Bool("report", false, "print import report JSON to stdout")
	reportOut := fs.String("report-out", "", "write report JSON to file")
	diff := fs.Bool("diff", false, "show schema/http layer diff against existing CUE files")
	update := fs.Bool("update", false, "write generated schema/http CUE files")
	outAPIDir := fs.String("out-api-dir", "cue/api", "directory for generated API CUE files")
	outDomain := fs.String("out-domain-file", "cue/domain/entities.cue", "generated entities CUE file path")
	groupByOwner := fs.Bool("group-by-owner", true, "split operations into operations_<owner>.cue files")

	var positional []string
	filtered := parseArgs[:0]
	for _, a := range parseArgs {
		if !strings.HasPrefix(a, "-") && len(filtered) == 0 && !strings.Contains(a, "=") {
			positional = append(positional, a)
		} else {
			filtered = append(filtered, a)
		}
	}
	if err := fs.Parse(filtered); err != nil {
		fmt.Fprintf(os.Stderr, "import openapi: %v\n", err)
		os.Exit(1)
	}

	sourcePath := "."
	if len(positional) > 0 {
		sourcePath = positional[0]
	} else if fs.NArg() > 0 {
		sourcePath = fs.Arg(0)
	}
	sourcePath = filepath.Clean(sourcePath)

	opts := importOpenAPIOptions{
		SourcePath:   sourcePath,
		OutAPIDir:    filepath.Clean(*outAPIDir),
		OutDomain:    filepath.Clean(*outDomain),
		Report:       *report,
		ReportOut:    strings.TrimSpace(*reportOut),
		Diff:         *diff,
		Update:       *update,
		GroupByOwner: *groupByOwner,
	}

	topts, err := localOpenAPIImportOptionsToTransform(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "import openapi FAILED (convert opts): %v\n", err)
		os.Exit(1)
	}
	tir, treport, files, err := transform.BuildOpenAPIImportPipeline(topts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "import openapi FAILED: %v\n", err)
		os.Exit(1)
	}
	ir, reportDoc, err := transformImportArtifactsToLocal(tir, treport)
	if err != nil {
		fmt.Fprintf(os.Stderr, "import openapi FAILED (convert): %v\n", err)
		os.Exit(1)
	}

	diffItems, err := computeImportDiff("", files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "import openapi FAILED (diff): %v\n", err)
		os.Exit(1)
	}
	reportDoc.Diff = diffItems
	if opts.Diff {
		printImportDiff(diffItems)
	}

	if opts.Update {
		skipped, werr := writeOpenAPIImportFilesSafe(files)
		if werr != nil {
			fmt.Fprintf(os.Stderr, "import openapi FAILED (update): %v\n", werr)
			os.Exit(1)
		}
		for _, s := range skipped {
			reportDoc.Todos = append(reportDoc.Todos, importTodo{
				Artifact: s,
				Reason:   "skipped to preserve manual flow/impl",
				Hint:     "Move manual flow/impl into separate file or add generated marker before forcing overwrite.",
			})
		}
		reportDoc.Summary = summarizeImportIR(ir, reportDoc.Conflicts, reportDoc.Todos)
		fmt.Printf("Updated OpenAPI schema/http layer (%d files, skipped %d).\n", len(files)-len(skipped), len(skipped))
	}

	if opts.ReportOut != "" {
		if err := writeImportReport(opts.ReportOut, reportDoc); err != nil {
			fmt.Fprintf(os.Stderr, "import openapi FAILED (report-out): %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Wrote report: %s\n", opts.ReportOut)
	}

	if opts.Report {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(reportDoc)
	}

	if !opts.Report && opts.ReportOut == "" && !opts.Diff && !opts.Update {
		fmt.Printf("Imported OpenAPI -> CUE contract (%d entities, %d operations, %d endpoints).\n", len(ir.Entities), len(ir.Operations), len(ir.Endpoints))
		fmt.Println("Tip: run `ang import openapi --report --diff --update` to inspect and apply contract-layer updates.")
	}
}

func buildJavaImportPipeline(opts importJavaOptions) (javaImportIR, javaImportReport, map[string]string, error) {
	var ir javaImportIR
	report := javaImportReport{
		Schema:         "ang/import-report/v1",
		SourcePath:     opts.Root,
		Profile:        string(opts.Profile),
		GeneratedAtUTC: time.Now().UTC().Format(time.RFC3339),
		Frameworks: []importFrameworkStatus{
			{Name: "spring-mvc", Enabled: true, Note: "detected via Java annotations"},
			{Name: "spring-webflux", Enabled: false, Note: "plugin stub"},
			{Name: "quarkus", Enabled: false, Note: "plugin stub"},
			{Name: "micronaut", Enabled: false, Note: "plugin stub"},
			{Name: "jax-rs", Enabled: false, Note: "plugin stub"},
			{Name: "grpc", Enabled: false, Note: "plugin stub"},
		},
	}

	javaFacts, err := extractJavaFactsWithTransform(opts.Root, false)
	if err != nil {
		return ir, report, nil, err
	}
	report.Adapters = append(report.Adapters, importAdapterStatus{
		Name:     "spring-annotations",
		Enabled:  true,
		Detected: len(javaFacts.Entities) + len(javaFacts.Operations) + len(javaFacts.Endpoints) + len(javaFacts.SecurityRules) + len(javaFacts.ErrorContracts),
	})
	report.Adapters = append(report.Adapters, importAdapterStatus{
		Name:     "mapstruct",
		Enabled:  true,
		Detected: len(javaFacts.Mappers),
	})
	report.Adapters = append(report.Adapters, importAdapterStatus{
		Name:     "jpa-hibernate",
		Enabled:  true,
		Detected: len(javaFacts.Entities),
		Note:     "entity extraction includes table/column hints; deep relation semantics still heuristic",
	})

	var openapiFacts FactsEnvelope
	if p, ok := transform.FindJavaProjectOpenAPI(opts.Root); ok {
		op, oerr := extractOpenAPIFactsWithTransform(p)
		if oerr == nil {
			openapiFacts = op
			report.Adapters = append(report.Adapters, importAdapterStatus{Name: "openapi", Enabled: true, Detected: len(op.Operations) + len(op.Entities)})
		} else {
			report.Adapters = append(report.Adapters, importAdapterStatus{Name: "openapi", Enabled: false, Note: oerr.Error()})
		}
	} else {
		report.Adapters = append(report.Adapters, importAdapterStatus{Name: "openapi", Enabled: false, Note: "openapi file not found"})
	}

	var sqlFacts FactsEnvelope
	sqlFiles, serr := transform.CollectSQLFiles(opts.Root)
	if serr == nil && len(sqlFiles) > 0 {
		sf, err := extractSQLFactsWithTransform(opts.Root)
		if err == nil {
			sqlFacts = sf
			report.Adapters = append(report.Adapters, importAdapterStatus{Name: "flyway-liquibase-sql", Enabled: true, Detected: len(sf.Entities)})
		} else {
			report.Adapters = append(report.Adapters, importAdapterStatus{Name: "flyway-liquibase-sql", Enabled: false, Note: err.Error()})
		}
	} else {
		report.Adapters = append(report.Adapters, importAdapterStatus{Name: "flyway-liquibase-sql", Enabled: false, Note: "no .sql migrations detected"})
	}

	if opts.Incremental {
		changed, cerr := gitChangedFiles(opts.Root)
		if cerr != nil {
			report.Todos = append(report.Todos, importTodo{Artifact: "incremental", Reason: "cannot detect changed files", Hint: cerr.Error()})
		} else if len(changed) > 0 {
			javaFacts = filterFactsByChanged(javaFacts, changed)
			openapiFacts = filterFactsByChanged(openapiFacts, changed)
			sqlFacts = filterFactsByChanged(sqlFacts, changed)
		} else {
			report.Todos = append(report.Todos, importTodo{Artifact: "incremental", Reason: "no changes detected", Hint: "falling back to full snapshot"})
		}
	}

	conflicts := make([]importConflict, 0)
	todos := make([]importTodo, 0)

	ir = buildImportIRFromSources(opts, javaFacts, openapiFacts, sqlFacts, &conflicts, &todos)
	ir.GeneratedAtUTC = report.GeneratedAtUTC
	report.Conflicts = conflicts
	report.Todos = append(report.Todos, todos...)
	report.Summary = summarizeImportIR(ir, report.Conflicts, report.Todos)

	files := renderImportCueFiles(opts.OutDir, ir, report)
	return ir, report, files, nil
}

func buildOpenAPIImportPipeline(opts importOpenAPIOptions) (javaImportIR, javaImportReport, map[string]string, error) {
	var ir javaImportIR
	report := javaImportReport{
		Schema:         "ang/import-report/v1",
		SourcePath:     opts.SourcePath,
		Profile:        "openapi",
		GeneratedAtUTC: time.Now().UTC().Format(time.RFC3339),
		Adapters: []importAdapterStatus{
			{Name: "openapi", Enabled: true},
			{Name: "spring-annotations", Enabled: false, Note: "not used in import openapi mode"},
			{Name: "flyway-liquibase-sql", Enabled: false, Note: "not used in import openapi mode"},
		},
		Frameworks: []importFrameworkStatus{
			{Name: "openapi-spec", Enabled: true, Note: "source of truth for HTTP contract"},
		},
	}

	openapiFacts, err := extractOpenAPIFactsWithTransform(opts.SourcePath)
	if err != nil {
		return ir, report, nil, err
	}
	report.Adapters[0].Detected = len(openapiFacts.Entities) + len(openapiFacts.Operations)

	conflicts := make([]importConflict, 0)
	todos := make([]importTodo, 0)
	ir = buildImportIRFromSources(importJavaOptions{
		Root:    opts.SourcePath,
		Profile: importProfileLayered,
	}, FactsEnvelope{}, openapiFacts, FactsEnvelope{}, &conflicts, &todos)
	ir.GeneratedAtUTC = report.GeneratedAtUTC
	ir.Profile = "openapi"
	report.Conflicts = conflicts
	report.Todos = append(report.Todos, todos...)
	if len(ir.Operations) == 0 {
		report.Todos = append(report.Todos, importTodo{
			Artifact: "openapi.paths",
			Reason:   "no operations extracted",
			Hint:     "Ensure paths/operationId exist in OpenAPI and are valid OpenAPI 3.x.",
		})
	}
	report.Summary = summarizeImportIR(ir, report.Conflicts, report.Todos)
	files := renderOpenAPICanonicalCueFiles(opts, ir)
	return ir, report, files, nil
}

func buildImportIRFromSources(opts importJavaOptions, javaFacts, openapiFacts, sqlFacts FactsEnvelope, conflicts *[]importConflict, todos *[]importTodo) javaImportIR {
	ir := javaImportIR{
		Profile:        string(opts.Profile),
		SourcePath:     opts.Root,
		Entities:       mergeImportEntities(javaFacts.Entities, openapiFacts.Entities, sqlFacts.Entities, conflicts),
		Operations:     mergeImportOperations(javaFacts.Operations, openapiFacts.Operations, conflicts, todos),
		ErrorContracts: append([]FactErrorContract(nil), javaFacts.ErrorContracts...),
		SecurityRules:  append([]FactSecurityRule(nil), javaFacts.SecurityRules...),
		Events:         append([]FactEvent(nil), javaFacts.Events...),
		Mappers:        append([]FactMapper(nil), javaFacts.Mappers...),
	}
	for _, op := range ir.Operations {
		ir.Endpoints = append(ir.Endpoints, importEndpointIR{
			Operation:     op.Name,
			Method:        op.HTTPMethod,
			Path:          op.HTTPPath,
			AuthExpr:      op.AuthExpr,
			Transactional: op.Transactional,
			TxReadOnly:    op.TxReadOnly,
			Confidence:    op.Confidence,
		})
		if op.HTTPMethod == "" || op.HTTPPath == "" {
			*todos = append(*todos, importTodo{
				Artifact: "operation:" + op.Name,
				Reason:   "missing endpoint method/path",
				Hint:     "Provide OpenAPI operationId mapping or explicit Spring/JAX-RS annotations.",
			})
		}
		if isMutatingHTTPMethod(op.HTTPMethod) && strings.TrimSpace(op.AuthExpr) == "" {
			*todos = append(*todos, importTodo{
				Artifact: "operation:" + op.Name,
				Reason:   "mutating endpoint has no explicit auth rule",
				Hint:     "Add @PreAuthorize/@Secured or endpoint-level auth contract before migration to flow.",
			})
		}
	}
	if len(ir.SecurityRules) == 0 {
		*todos = append(*todos, importTodo{Artifact: "security", Reason: "no global security rules extracted", Hint: "Check security config adapter and method security annotations."})
	}
	*todos = append(*todos,
		importTodo{Artifact: "framework-plugin:jax-rs", Reason: "adapter not enabled", Hint: "Implement JAX-RS extractor plugin to raise coverage."},
		importTodo{Artifact: "framework-plugin:bytecode", Reason: "adapter not enabled", Hint: "Add bytecode fallback extractor for closed-source/service jars."},
	)
	return ir
}

func mergeImportOperations(javaOps, openapiOps []FactOp, conflicts *[]importConflict, todos *[]importTodo) []importOperationIR {
	javaByKey := map[string]FactOp{}
	for _, op := range javaOps {
		k := normalizeFactID(op.Name)
		if k == "" {
			continue
		}
		javaByKey[k] = op
	}
	openapiByKey := map[string]FactOp{}
	for _, op := range openapiOps {
		k := normalizeFactID(op.Name)
		if k == "" {
			continue
		}
		openapiByKey[k] = op
	}
	keySet := map[string]struct{}{}
	for k := range javaByKey {
		keySet[k] = struct{}{}
	}
	for k := range openapiByKey {
		keySet[k] = struct{}{}
	}
	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]importOperationIR, 0, len(keys))
	for _, k := range keys {
		j, hasJ := javaByKey[k]
		o, hasO := openapiByKey[k]

		name := firstNonEmpty(j.Name, o.Name)
		service := firstNonEmpty(j.ServiceHint, o.ServiceHint)
		method := strings.ToUpper(strings.TrimSpace(j.HTTPMethod))
		path := strings.TrimSpace(j.HTTPPath)
		auth := strings.TrimSpace(j.AuthExpr)
		tx := j.Transactional
		txRO := j.TxReadOnly
		inputs := mergeFactFields(nil, j.InputFields)
		outputs := mergeFactFields(nil, j.OutputFields)
		calls := mergeCallRefs(nil, j.Calls)
		sources := []string{}
		if hasJ {
			sources = append(sources, "spring-annotations", "code-inference")
		}
		if hasO {
			sources = append(sources, "openapi")
		}

		localConflict := false
		if hasJ && hasO {
			if strings.TrimSpace(o.HTTPMethod) != "" && method != "" && !strings.EqualFold(method, o.HTTPMethod) {
				*conflicts = append(*conflicts, importConflict{
					Artifact:        "operation:" + name,
					Field:           "http_method",
					PreferredSource: "openapi",
					PreferredValue:  strings.ToUpper(strings.TrimSpace(o.HTTPMethod)),
					OtherSource:     "spring-annotations",
					OtherValue:      method,
					Resolution:      "OpenAPI > annotations > code inference > DB",
				})
				localConflict = true
			}
			if strings.TrimSpace(o.HTTPPath) != "" && path != "" && normalizeHTTPPath(path) != normalizeHTTPPath(o.HTTPPath) {
				*conflicts = append(*conflicts, importConflict{
					Artifact:        "operation:" + name,
					Field:           "http_path",
					PreferredSource: "openapi",
					PreferredValue:  normalizeHTTPPath(o.HTTPPath),
					OtherSource:     "spring-annotations",
					OtherValue:      normalizeHTTPPath(path),
					Resolution:      "OpenAPI > annotations > code inference > DB",
				})
				localConflict = true
			}
		}

		// Priority: OpenAPI > annotations > inference
		if hasO {
			if strings.TrimSpace(o.HTTPMethod) != "" {
				method = strings.ToUpper(strings.TrimSpace(o.HTTPMethod))
			}
			if strings.TrimSpace(o.HTTPPath) != "" {
				path = normalizeHTTPPath(o.HTTPPath)
			}
			if len(o.InputFields) > 0 {
				inputs = mergeFactFields(nil, o.InputFields)
			}
			if len(o.OutputFields) > 0 {
				outputs = mergeFactFields(nil, o.OutputFields)
			}
		}

		confidence := confidenceLow
		switch {
		case hasJ && hasO && !localConflict:
			confidence = confidenceHigh
		case hasO || auth != "" || method != "" || path != "":
			confidence = confidenceMedium
		default:
			confidence = confidenceLow
		}
		if confidence == confidenceLow {
			*todos = append(*todos, importTodo{Artifact: "operation:" + name, Reason: "low-confidence inference", Hint: "Cross-check with OpenAPI/spec tests before updating CUE contract."})
		}

		out = append(out, importOperationIR{
			Name:          name,
			Service:       service,
			HTTPMethod:    method,
			HTTPPath:      normalizeHTTPPath(path),
			AuthExpr:      auth,
			Transactional: tx,
			TxReadOnly:    txRO,
			Inputs:        inputs,
			Outputs:       outputs,
			Calls:         calls,
			Sources:       uniqueStrings(sources),
			Confidence:    confidence,
		})
	}
	return out
}

func mergeImportEntities(javaEntities, openapiEntities, sqlEntities []FactEntity, conflicts *[]importConflict) []importEntityIR {
	javaByKey := map[string]FactEntity{}
	for _, e := range javaEntities {
		k := normalizeFactID(e.Name)
		if k == "" {
			continue
		}
		javaByKey[k] = e
	}
	openapiByKey := map[string]FactEntity{}
	for _, e := range openapiEntities {
		k := normalizeFactID(e.Name)
		if k == "" {
			continue
		}
		openapiByKey[k] = e
	}
	sqlByKey := map[string]FactEntity{}
	for _, e := range sqlEntities {
		k := normalizeFactID(e.Name)
		if k == "" {
			continue
		}
		sqlByKey[k] = e
	}
	keysMap := map[string]struct{}{}
	for k := range javaByKey {
		keysMap[k] = struct{}{}
	}
	for k := range openapiByKey {
		keysMap[k] = struct{}{}
	}
	for k := range sqlByKey {
		keysMap[k] = struct{}{}
	}
	keys := make([]string, 0, len(keysMap))
	for k := range keysMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]importEntityIR, 0, len(keys))
	for _, k := range keys {
		j, hasJ := javaByKey[k]
		o, hasO := openapiByKey[k]
		s, hasS := sqlByKey[k]
		name := firstNonEmpty(o.Name, j.Name, s.Name)
		tableHint := firstNonEmpty(o.TableHint, j.TableHint, s.TableHint)
		fields := mergeFactFields(nil, j.Fields)
		sources := []string{}
		if hasJ {
			sources = append(sources, "spring-annotations", "jpa-hibernate")
		}
		if hasO {
			sources = append(sources, "openapi")
		}
		if hasS {
			sources = append(sources, "db-schema")
		}

		// Priority: OpenAPI > annotations > code inference > DB
		if hasO && len(o.Fields) > 0 {
			fields = mergeFactFields(fields, o.Fields)
		}
		if hasS && len(s.Fields) > 0 {
			fields = mergeFactFields(fields, s.Fields)
		}

		if hasJ && hasO {
			jByField := map[string]FactField{}
			for _, f := range j.Fields {
				jByField[normalizeFactID(f.Name)] = f
			}
			for _, f := range o.Fields {
				jf, ok := jByField[normalizeFactID(f.Name)]
				if !ok {
					continue
				}
				jType := strings.TrimSpace(jf.CueTypeHint)
				oType := strings.TrimSpace(f.CueTypeHint)
				if jType != "" && oType != "" && jType != oType {
					*conflicts = append(*conflicts, importConflict{
						Artifact:        "entity:" + name,
						Field:           jf.Name + ".type",
						PreferredSource: "openapi",
						PreferredValue:  oType,
						OtherSource:     "spring-annotations",
						OtherValue:      jType,
						Resolution:      "OpenAPI > annotations > code inference > DB",
					})
				}
			}
		}
		if hasO && hasS {
			oByField := map[string]FactField{}
			for _, f := range o.Fields {
				oByField[normalizeFactID(f.Name)] = f
			}
			for _, f := range s.Fields {
				of, ok := oByField[normalizeFactID(f.Name)]
				if !ok {
					continue
				}
				oType := strings.TrimSpace(of.CueTypeHint)
				sType := strings.TrimSpace(f.CueTypeHint)
				if oType != "" && sType != "" && oType != sType {
					*conflicts = append(*conflicts, importConflict{
						Artifact:        "entity:" + name,
						Field:           of.Name + ".type",
						PreferredSource: "openapi",
						PreferredValue:  oType,
						OtherSource:     "db-schema",
						OtherValue:      sType,
						Resolution:      "OpenAPI > annotations > code inference > DB",
					})
				}
			}
		}
		if hasJ && hasS && !hasO {
			jByField := map[string]FactField{}
			for _, f := range j.Fields {
				jByField[normalizeFactID(f.Name)] = f
			}
			for _, f := range s.Fields {
				jf, ok := jByField[normalizeFactID(f.Name)]
				if !ok {
					continue
				}
				jType := strings.TrimSpace(jf.CueTypeHint)
				sType := strings.TrimSpace(f.CueTypeHint)
				if jType != "" && sType != "" && jType != sType {
					*conflicts = append(*conflicts, importConflict{
						Artifact:        "entity:" + name,
						Field:           jf.Name + ".type",
						PreferredSource: "spring-annotations",
						PreferredValue:  jType,
						OtherSource:     "db-schema",
						OtherValue:      sType,
						Resolution:      "OpenAPI > annotations > code inference > DB",
					})
				}
			}
		}

		confidence := confidenceLow
		switch {
		case hasO && hasJ:
			confidence = confidenceHigh
		case hasO:
			confidence = confidenceMedium
		case hasJ && hasS:
			confidence = confidenceHigh
		case hasJ:
			confidence = confidenceMedium
		case hasS:
			confidence = confidenceLow
		}
		out = append(out, importEntityIR{
			Name:       name,
			TableHint:  tableHint,
			Fields:     fields,
			Sources:    uniqueStrings(sources),
			Confidence: confidence,
		})
	}
	return out
}

func summarizeImportIR(ir javaImportIR, conflicts []importConflict, todos []importTodo) importSummary {
	s := importSummary{
		Entities:   len(ir.Entities),
		Operations: len(ir.Operations),
		Endpoints:  len(ir.Endpoints),
		Conflicts:  len(conflicts),
		Todos:      len(todos),
	}
	for _, e := range ir.Entities {
		switch e.Confidence {
		case confidenceHigh:
			s.HighConfidence++
		case confidenceMedium:
			s.MediumConfidence++
		default:
			s.LowConfidence++
		}
	}
	for _, o := range ir.Operations {
		switch o.Confidence {
		case confidenceHigh:
			s.HighConfidence++
		case confidenceMedium:
			s.MediumConfidence++
		default:
			s.LowConfidence++
		}
	}
	return s
}

func renderImportCueFiles(outDir string, ir javaImportIR, report javaImportReport) map[string]string {
	files := map[string]string{}
	files[filepath.Join(outDir, "project.cue")] = renderImportProjectCue(ir, report)
	files[filepath.Join(outDir, "entities.cue")] = renderImportEntitiesCue(ir.Entities)
	files[filepath.Join(outDir, "operations.cue")] = renderImportOperationsCue(ir.Operations)
	files[filepath.Join(outDir, "contracts.cue")] = renderImportContractsCue(ir)
	return files
}

func renderOpenAPICanonicalCueFiles(opts importOpenAPIOptions, ir javaImportIR) map[string]string {
	files := map[string]string{}
	files[filepath.Join(opts.OutAPIDir, "http.cue")] = renderCanonicalHTTPCue(ir.Operations)
	files[opts.OutDomain] = renderCanonicalEntitiesCue(ir.Entities)
	for path, content := range renderCanonicalOperationsCueByGroup(opts.OutAPIDir, ir.Operations, opts.GroupByOwner) {
		files[path] = content
	}
	return files
}

func renderCanonicalHTTPCue(ops []importOperationIR) string {
	var b strings.Builder
	b.WriteString("// Code generated by ang import openapi. DO NOT EDIT.\n")
	b.WriteString("package api\n\n")
	b.WriteString("import \"github.com/strogmv/ang/cue/schema\"\n\n")
	b.WriteString("HTTP: schema.#HTTP & {\n")
	sort.Slice(ops, func(i, j int) bool { return ops[i].Name < ops[j].Name })
	for _, op := range ops {
		if strings.TrimSpace(op.Name) == "" {
			continue
		}
		b.WriteString("  " + cueIdent(op.Name) + ": {\n")
		if strings.TrimSpace(op.HTTPMethod) != "" {
			b.WriteString("    method: " + cueString(strings.ToUpper(strings.TrimSpace(op.HTTPMethod))) + "\n")
		}
		if strings.TrimSpace(op.HTTPPath) != "" {
			b.WriteString("    path:   " + cueString(normalizeHTTPPath(op.HTTPPath)) + "\n")
		}
		if strings.TrimSpace(op.AuthExpr) != "" {
			b.WriteString("    authExpr: " + cueString(op.AuthExpr) + "\n")
		}
		b.WriteString("  }\n")
	}
	b.WriteString("}\n")
	return b.String()
}

func renderCanonicalOperationsCueByGroup(outAPIDir string, ops []importOperationIR, groupByOwner bool) map[string]string {
	byGroup := map[string][]importOperationIR{}
	for _, op := range ops {
		group := strings.TrimSpace(op.Service)
		if group == "" {
			group = guessOwnerFromPath(op.HTTPPath)
		}
		group = sanitizeCueFileStem(group)
		if group == "" || !groupByOwner {
			group = "default"
		}
		byGroup[group] = append(byGroup[group], op)
	}
	groups := make([]string, 0, len(byGroup))
	for g := range byGroup {
		groups = append(groups, g)
	}
	sort.Strings(groups)
	files := make(map[string]string, len(groups))
	for _, g := range groups {
		opsGroup := byGroup[g]
		sort.Slice(opsGroup, func(i, j int) bool { return opsGroup[i].Name < opsGroup[j].Name })
		var b strings.Builder
		b.WriteString("// Code generated by ang import openapi. DO NOT EDIT.\n")
		b.WriteString("package api\n\n")
		b.WriteString("import \"github.com/strogmv/ang/cue/schema\"\n\n")
		for _, op := range opsGroup {
			if strings.TrimSpace(op.Name) == "" {
				continue
			}
			b.WriteString(cueIdent(op.Name) + ": schema.#Operation & {\n")
			service := strings.TrimSpace(op.Service)
			if service == "" {
				service = g
			}
			b.WriteString("  service: " + cueString(service) + "\n")
			b.WriteString("  input: {\n")
			for _, f := range op.Inputs {
				b.WriteString(renderOperationFieldLine(f))
			}
			b.WriteString("  }\n")
			b.WriteString("  output: {\n")
			for _, f := range op.Outputs {
				b.WriteString(renderOperationFieldLine(f))
			}
			b.WriteString("  }\n")
			b.WriteString("}\n\n")
		}
		files[filepath.Join(outAPIDir, "operations_"+g+".cue")] = b.String()
	}
	return files
}

func renderCanonicalEntitiesCue(entities []importEntityIR) string {
	var b strings.Builder
	b.WriteString("// Code generated by ang import openapi. DO NOT EDIT.\n")
	b.WriteString("package domain\n\n")
	sort.Slice(entities, func(i, j int) bool { return entities[i].Name < entities[j].Name })
	for _, e := range entities {
		if strings.TrimSpace(e.Name) == "" {
			continue
		}
		b.WriteString("#" + cueIdent(e.Name) + ": {\n")
		b.WriteString("  fields: {\n")
		for _, f := range e.Fields {
			b.WriteString(renderEntityFieldLine(f))
		}
		b.WriteString("  }\n")
		b.WriteString("}\n\n")
	}
	return b.String()
}

func renderOperationFieldLine(f FactField) string {
	name := cueFieldName(f.Name, f.JSONTag)
	if name == "" {
		name = "field"
	}
	fieldType := cueTypeExpr(f.CueTypeHint)
	if fieldType == "" {
		fieldType = "_"
	}
	optional := " "
	if !f.Required {
		optional = "?"
	}
	validate := strings.TrimSpace(f.Validate)
	if validate == "" && len(f.ValidationRules) > 0 {
		validate = strings.Join(f.ValidationRules, ",")
	}
	line := "    " + name + optional + ": " + fieldType
	if validate != "" {
		line += " @validate(" + cueString(validate) + ")"
	}
	line += "\n"
	return line
}

func renderEntityFieldLine(f FactField) string {
	name := cueFieldName(f.Name, f.JSONTag)
	if name == "" {
		name = "field"
	}
	fieldType := cueTypeExpr(f.CueTypeHint)
	if fieldType == "" {
		fieldType = "_"
	}
	optional := " "
	if !f.Required {
		optional = "?"
	}
	validate := strings.TrimSpace(f.Validate)
	if validate == "" && len(f.ValidationRules) > 0 {
		validate = strings.Join(f.ValidationRules, ",")
	}
	line := "    " + name + optional + ": " + fieldType
	if validate != "" {
		line += " @validate(" + cueString(validate) + ")"
	}
	if strings.TrimSpace(f.RelationKind) != "" {
		line += " @rel(kind=" + cueString(f.RelationKind)
		if strings.TrimSpace(f.RelationTarget) != "" {
			line += ",target=" + cueString(f.RelationTarget)
		}
		line += ")"
	}
	if strings.TrimSpace(f.Fetch) != "" {
		line += " @fetch(mode=" + cueString(f.Fetch) + ")"
	}
	if len(f.Cascade) > 0 {
		line += " @cascade(values=" + cueString(strings.Join(f.Cascade, ",")) + ")"
	}
	line += "\n"
	return line
}

func cueFieldName(name, jsonTag string) string {
	raw := strings.TrimSpace(jsonTag)
	if raw == "" {
		raw = strings.TrimSpace(name)
	}
	if raw == "" {
		return ""
	}
	raw = strings.Trim(raw, "\"`")
	var b strings.Builder
	for i, r := range raw {
		isAlpha := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		isDigit := r >= '0' && r <= '9'
		if i == 0 {
			if isAlpha || r == '_' {
				b.WriteRune(r)
			} else if isDigit {
				b.WriteRune('_')
				b.WriteRune(r)
			} else {
				b.WriteRune('_')
			}
			continue
		}
		if isAlpha || isDigit || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return ""
	}
	if strings.ToUpper(out[:1]) == out[:1] {
		out = lowerFirst(out)
	}
	return out
}

func cueIdent(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Unknown"
	}
	var b strings.Builder
	for i, r := range name {
		isAlpha := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		isDigit := r >= '0' && r <= '9'
		if i == 0 {
			if isAlpha || r == '_' {
				b.WriteRune(r)
			} else if isDigit {
				b.WriteRune('_')
				b.WriteRune(r)
			} else {
				b.WriteRune('_')
			}
			continue
		}
		if isAlpha || isDigit || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "Unknown"
	}
	return out
}

func cueTypeExpr(hint string) string {
	t := strings.TrimSpace(hint)
	if t == "" {
		return "_"
	}
	switch t {
	case "string", "int", "float", "bool", "bytes", "_", "{...}":
		return t
	default:
		if strings.HasPrefix(t, "[...") && strings.HasSuffix(t, "]") {
			return t
		}
		return "string"
	}
}

func guessOwnerFromPath(p string) string {
	p = normalizeHTTPPath(p)
	p = strings.Trim(p, "/")
	if p == "" {
		return "default"
	}
	parts := strings.Split(p, "/")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || strings.HasPrefix(part, "{") {
			continue
		}
		return sanitizeCueFileStem(part)
	}
	return "default"
}

func sanitizeCueFileStem(in string) string {
	in = strings.ToLower(strings.TrimSpace(in))
	if in == "" {
		return ""
	}
	var b strings.Builder
	lastUnderscore := false
	for _, r := range in {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAlphaNum {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteRune('_')
			lastUnderscore = true
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "default"
	}
	return out
}

func renderImportProjectCue(ir javaImportIR, report javaImportReport) string {
	var b strings.Builder
	b.WriteString("package importcontract\n\n")
	b.WriteString("meta: {\n")
	b.WriteString("  sourcePath: " + cueString(ir.SourcePath) + "\n")
	b.WriteString("  profile: " + cueString(ir.Profile) + "\n")
	b.WriteString("  generatedAt: " + cueString(ir.GeneratedAtUTC) + "\n")
	b.WriteString("  adapters: [\n")
	for _, a := range report.Adapters {
		b.WriteString("    {name: " + cueString(a.Name) + " enabled: " + cueBool(a.Enabled) + " detected: " + fmt.Sprintf("%d", a.Detected))
		if strings.TrimSpace(a.Note) != "" {
			b.WriteString(" note: " + cueString(a.Note))
		}
		b.WriteString("}\n")
	}
	b.WriteString("  ]\n")
	b.WriteString("}\n")
	return b.String()
}

func renderImportEntitiesCue(entities []importEntityIR) string {
	var b strings.Builder
	b.WriteString("package importcontract\n\n")
	b.WriteString("entities: [\n")
	for _, e := range entities {
		b.WriteString("  {\n")
		b.WriteString("    name: " + cueString(e.Name) + "\n")
		if strings.TrimSpace(e.TableHint) != "" {
			b.WriteString("    tableHint: " + cueString(e.TableHint) + "\n")
		}
		b.WriteString("    confidence: " + cueString(string(e.Confidence)) + "\n")
		b.WriteString("    sources: [")
		for i, s := range e.Sources {
			if i > 0 {
				b.WriteString(" ")
			}
			b.WriteString(cueString(s))
		}
		b.WriteString("]\n")
		b.WriteString("    fields: [\n")
		for _, f := range e.Fields {
			b.WriteString("      {name: " + cueString(f.Name))
			if strings.TrimSpace(f.CueTypeHint) != "" {
				b.WriteString(" type: " + cueString(f.CueTypeHint))
			}
			if strings.TrimSpace(f.DBTag) != "" {
				b.WriteString(" db: " + cueString(f.DBTag))
			}
			if strings.TrimSpace(f.JSONTag) != "" {
				b.WriteString(" json: " + cueString(f.JSONTag))
			}
			b.WriteString("}\n")
		}
		b.WriteString("    ]\n")
		b.WriteString("  }\n")
	}
	b.WriteString("]\n")
	return b.String()
}

func renderImportOperationsCue(ops []importOperationIR) string {
	var b strings.Builder
	b.WriteString("package importcontract\n\n")
	b.WriteString("operations: [\n")
	for _, op := range ops {
		b.WriteString("  {\n")
		b.WriteString("    name: " + cueString(op.Name) + "\n")
		if strings.TrimSpace(op.Service) != "" {
			b.WriteString("    service: " + cueString(op.Service) + "\n")
		}
		if strings.TrimSpace(op.HTTPMethod) != "" || strings.TrimSpace(op.HTTPPath) != "" {
			b.WriteString("    http: {\n")
			if strings.TrimSpace(op.HTTPMethod) != "" {
				b.WriteString("      method: " + cueString(op.HTTPMethod) + "\n")
			}
			if strings.TrimSpace(op.HTTPPath) != "" {
				b.WriteString("      path: " + cueString(op.HTTPPath) + "\n")
			}
			b.WriteString("    }\n")
		}
		if strings.TrimSpace(op.AuthExpr) != "" {
			b.WriteString("    authExpr: " + cueString(op.AuthExpr) + "\n")
		}
		b.WriteString("    transactional: " + cueBool(op.Transactional) + "\n")
		b.WriteString("    txReadOnly: " + cueBool(op.TxReadOnly) + "\n")
		b.WriteString("    confidence: " + cueString(string(op.Confidence)) + "\n")
		b.WriteString("    sources: [")
		for i, s := range op.Sources {
			if i > 0 {
				b.WriteString(" ")
			}
			b.WriteString(cueString(s))
		}
		b.WriteString("]\n")
		b.WriteString("  }\n")
	}
	b.WriteString("]\n")
	return b.String()
}

func renderImportContractsCue(ir javaImportIR) string {
	var b strings.Builder
	b.WriteString("package importcontract\n\n")
	b.WriteString("errorContracts: [\n")
	for _, e := range ir.ErrorContracts {
		b.WriteString("  {exception: " + cueString(e.Exception))
		if strings.TrimSpace(e.Status) != "" {
			b.WriteString(" status: " + cueString(e.Status))
		}
		if e.HTTPCode > 0 {
			b.WriteString(" httpCode: " + fmt.Sprintf("%d", e.HTTPCode))
		}
		if strings.TrimSpace(e.Handler) != "" {
			b.WriteString(" handler: " + cueString(e.Handler))
		}
		b.WriteString("}\n")
	}
	b.WriteString("]\n\n")
	b.WriteString("securityRules: [\n")
	for _, s := range ir.SecurityRules {
		b.WriteString("  {scope: " + cueString(s.Scope))
		if strings.TrimSpace(s.Pattern) != "" {
			b.WriteString(" pattern: " + cueString(s.Pattern))
		}
		if strings.TrimSpace(s.Requirement) != "" {
			b.WriteString(" requirement: " + cueString(s.Requirement))
		}
		b.WriteString("}\n")
	}
	b.WriteString("]\n")
	return b.String()
}

func cueString(s string) string {
	bs, _ := json.Marshal(s)
	return string(bs)
}

func cueBool(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func printImportDiff(items []importDiffItem) {
	fmt.Println("Import diff (contract layer):")
	for _, it := range items {
		fmt.Printf("  %s  %s\n", it.Status, it.Path)
	}
}

func computeImportDiff(outDir string, files map[string]string) ([]importDiffItem, error) {
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	items := make([]importDiffItem, 0, len(paths))
	for _, p := range paths {
		newContent := files[p]
		oldBytes, err := os.ReadFile(p)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				items = append(items, importDiffItem{Path: p, Status: "added"})
				continue
			}
			return nil, err
		}
		if string(oldBytes) == newContent {
			items = append(items, importDiffItem{Path: p, Status: "unchanged"})
		} else {
			items = append(items, importDiffItem{Path: p, Status: "updated"})
		}
	}
	_ = outDir
	return items, nil
}

func writeImportCueFiles(_ string, files map[string]string) error {
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte(files[p]), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func writeOpenAPIImportFilesSafe(files map[string]string) ([]string, error) {
	const generatedHeader = "// Code generated by ang import openapi. DO NOT EDIT."
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	skipped := make([]string, 0)
	for _, p := range paths {
		newContent := files[p]
		old, err := os.ReadFile(p)
		if err == nil {
			oldStr := string(old)
			if !strings.HasPrefix(oldStr, generatedHeader) {
				// Preserve manual files with flow/impl logic.
				if strings.Contains(oldStr, "flow:") || strings.Contains(oldStr, "impl_steps:") || strings.Contains(oldStr, "impl:") {
					skipped = append(skipped, p)
					continue
				}
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return skipped, err
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return skipped, err
		}
		if err := os.WriteFile(p, []byte(newContent), 0o644); err != nil {
			return skipped, err
		}
	}
	return skipped, nil
}

func writeImportReport(path string, report javaImportReport) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func gitChangedFiles(root string) (map[string]struct{}, error) {
	cmd := exec.Command("git", "-C", root, "diff", "--name-only", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	set := map[string]struct{}{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		abs := filepath.Clean(filepath.Join(root, line))
		set[abs] = struct{}{}
	}
	return set, nil
}

func filterFactsByChanged(env FactsEnvelope, changed map[string]struct{}) FactsEnvelope {
	isChanged := func(src string) bool {
		src = filepath.Clean(strings.TrimSpace(src))
		if src == "" {
			return false
		}
		_, ok := changed[src]
		return ok
	}
	f := env
	f.Entities = f.Entities[:0]
	for _, e := range env.Entities {
		if isChanged(e.Source) {
			f.Entities = append(f.Entities, e)
		}
	}
	f.Operations = f.Operations[:0]
	for _, o := range env.Operations {
		if isChanged(o.Source) {
			f.Operations = append(f.Operations, o)
		}
	}
	f.Repositories = f.Repositories[:0]
	for _, r := range env.Repositories {
		if isChanged(r.Source) {
			f.Repositories = append(f.Repositories, r)
		}
	}
	f.Events = f.Events[:0]
	for _, e := range env.Events {
		if isChanged(e.Source) {
			f.Events = append(f.Events, e)
		}
	}
	f.Endpoints = f.Endpoints[:0]
	for _, e := range env.Endpoints {
		if isChanged(e.Source) {
			f.Endpoints = append(f.Endpoints, e)
		}
	}
	f.Calls = f.Calls[:0]
	for _, c := range env.Calls {
		if isChanged(c.Source) {
			f.Calls = append(f.Calls, c)
		}
	}
	f.Mappers = f.Mappers[:0]
	for _, m := range env.Mappers {
		if isChanged(m.Source) {
			f.Mappers = append(f.Mappers, m)
		}
	}
	f.ErrorContracts = f.ErrorContracts[:0]
	for _, e := range env.ErrorContracts {
		if isChanged(e.Source) {
			f.ErrorContracts = append(f.ErrorContracts, e)
		}
	}
	f.SecurityRules = f.SecurityRules[:0]
	for _, s := range env.SecurityRules {
		if isChanged(s.Source) {
			f.SecurityRules = append(f.SecurityRules, s)
		}
	}
	return f
}

func extractJavaFactsWithTransform(root string, mergeOpenAPI bool) (FactsEnvelope, error) {
	tf, err := transform.ExtractJavaFactsWithOptions(root, mergeOpenAPI)
	if err != nil {
		return FactsEnvelope{}, err
	}
	return transformFactsToLocal(tf)
}

func extractOpenAPIFactsWithTransform(path string) (FactsEnvelope, error) {
	tf, err := transform.ExtractOpenAPIFacts(path)
	if err != nil {
		return FactsEnvelope{}, err
	}
	return transformFactsToLocal(tf)
}

func extractSQLFactsWithTransform(path string) (FactsEnvelope, error) {
	tf, err := transform.ExtractSQLFacts(path)
	if err != nil {
		return FactsEnvelope{}, err
	}
	return transformFactsToLocal(tf)
}

func transformFactsToLocal(tf transform.FactsEnvelope) (FactsEnvelope, error) {
	data, err := json.Marshal(tf)
	if err != nil {
		return FactsEnvelope{}, err
	}
	var env FactsEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return FactsEnvelope{}, err
	}
	canonicalizeFactsEnvelopeFields(&env)
	return env, nil
}

func localJavaImportOptionsToTransform(opts importJavaOptions) (transform.ImportJavaOptions, error) {
	data, err := json.Marshal(opts)
	if err != nil {
		return transform.ImportJavaOptions{}, err
	}
	var out transform.ImportJavaOptions
	if err := json.Unmarshal(data, &out); err != nil {
		return transform.ImportJavaOptions{}, err
	}
	return out, nil
}

func localOpenAPIImportOptionsToTransform(opts importOpenAPIOptions) (transform.ImportOpenAPIOptions, error) {
	data, err := json.Marshal(opts)
	if err != nil {
		return transform.ImportOpenAPIOptions{}, err
	}
	var out transform.ImportOpenAPIOptions
	if err := json.Unmarshal(data, &out); err != nil {
		return transform.ImportOpenAPIOptions{}, err
	}
	return out, nil
}

func transformImportArtifactsToLocal(ir transform.JavaImportIR, report transform.JavaImportReport) (javaImportIR, javaImportReport, error) {
	pair := struct {
		IR     transform.JavaImportIR     `json:"ir"`
		Report transform.JavaImportReport `json:"report"`
	}{IR: ir, Report: report}
	data, err := json.Marshal(pair)
	if err != nil {
		return javaImportIR{}, javaImportReport{}, err
	}
	var out struct {
		IR     javaImportIR     `json:"ir"`
		Report javaImportReport `json:"report"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return javaImportIR{}, javaImportReport{}, err
	}
	for i := range out.IR.Entities {
		out.IR.Entities[i].Fields = canonicalizeFactFields(out.IR.Entities[i].Fields)
	}
	for i := range out.IR.Operations {
		out.IR.Operations[i].Inputs = canonicalizeFactFields(out.IR.Operations[i].Inputs)
		out.IR.Operations[i].Outputs = canonicalizeFactFields(out.IR.Operations[i].Outputs)
	}
	return out.IR, out.Report, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
