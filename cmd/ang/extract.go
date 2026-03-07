package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	transform "github.com/strogmv/ang-transform/pkg/transform"
)

// FactsEnvelope is the root ang/facts/v1 document.
type FactsEnvelope struct {
	Schema         string              `json:"schema"`
	SourceType     string              `json:"source_type"`
	SourcePath     string              `json:"source_path"`
	Entities       []FactEntity        `json:"entities"`
	Operations     []FactOp            `json:"operations"`
	Repositories   []FactRepo          `json:"repositories"`
	Events         []FactEvent         `json:"events"`
	Constants      []FactConstant      `json:"constants,omitempty"`
	Enums          []FactEnum          `json:"enums,omitempty"`
	Endpoints      []FactEndpoint      `json:"endpoints,omitempty"`
	Calls          []FactCallEdge      `json:"calls,omitempty"`
	Mappers        []FactMapper        `json:"mappers,omitempty"`
	ErrorContracts []FactErrorContract `json:"error_contracts,omitempty"`
	SecurityRules  []FactSecurityRule  `json:"security_rules,omitempty"`
}

// FactEntity represents a domain entity/struct.
type FactEntity struct {
	Name               string      `json:"name"`
	TableHint          string      `json:"table_hint,omitempty"`
	Source             string      `json:"source,omitempty"`
	Fields             []FactField `json:"fields"`
	CompositeKey       string      `json:"composite_key,omitempty"`        // embedded_id|id_class:<Type>|multiple_ids
	SoftDelete         bool        `json:"soft_delete,omitempty"`          // inferred from SQLDelete/Where/soft-delete field
	SoftDeleteStrategy string      `json:"soft_delete_strategy,omitempty"` // sql_delete|where_clause|field
	SoftDeleteClause   string      `json:"soft_delete_clause,omitempty"`
	WhereClause        string      `json:"where_clause,omitempty"`
}

// FactField is one field of an entity, operation, or event.
type FactField struct {
	Name            string   `json:"name"`
	GoType          string   `json:"go_type,omitempty"`
	ResolvedType    string   `json:"resolved_type,omitempty"`
	CueTypeHint     string   `json:"cue_type_hint,omitempty"`
	JSONTag         string   `json:"json_tag,omitempty"`
	DBTag           string   `json:"db_tag,omitempty"`
	SourceLine      int      `json:"source_line,omitempty"`
	Extractor       string   `json:"extractor,omitempty"`
	Evidence        []string `json:"evidence,omitempty"`
	Validate        string   `json:"validate,omitempty"`
	ValidationRules []string `json:"validation_rules,omitempty"`
	Required        bool     `json:"required,omitempty"`
	RelationKind    string   `json:"relation_kind,omitempty"`   // one_to_many|many_to_one|one_to_one|many_to_many|element_collection
	RelationTarget  string   `json:"relation_target,omitempty"` // inferred from generic/field type
	MappedBy        string   `json:"mapped_by,omitempty"`
	Cascade         []string `json:"cascade,omitempty"`
	Fetch           string   `json:"fetch,omitempty"` // lazy|eager
	OrphanRemoval   bool     `json:"orphan_removal,omitempty"`
	JoinColumn      string   `json:"join_column,omitempty"`
	JoinTable       string   `json:"join_table,omitempty"`
	EnumType        string   `json:"enum_type,omitempty"` // string|ordinal
	Persistence     []string `json:"persistence,omitempty"`
}

// FactOp represents a service operation (from interface methods or OpenAPI paths).
type FactOp struct {
	Name          string        `json:"name"`
	ServiceHint   string        `json:"service_hint,omitempty"`
	Source        string        `json:"source,omitempty"`
	SourceLine    int           `json:"source_line,omitempty"`
	ClassName     string        `json:"class_name,omitempty"`
	Kind          string        `json:"kind,omitempty"` // controller|service|usecase|other
	Extractor     string        `json:"extractor,omitempty"`
	Evidence      []string      `json:"evidence,omitempty"`
	EntryKind     string        `json:"entry_kind,omitempty"`
	EntryRef      string        `json:"entry_ref,omitempty"`
	HTTPMethod    string        `json:"http_method,omitempty"`
	HTTPPath      string        `json:"http_path,omitempty"`
	AuthExpr      string        `json:"auth_expr,omitempty"`
	Transactional bool          `json:"transactional,omitempty"`
	TxReadOnly    bool          `json:"tx_read_only,omitempty"`
	InputFields   []FactField   `json:"input_fields,omitempty"`
	OutputFields  []FactField   `json:"output_fields,omitempty"`
	Calls         []FactCallRef `json:"calls,omitempty"`
	ConstantsUsed []string      `json:"constants_used,omitempty"`
	EnumsUsed     []string      `json:"enums_used,omitempty"`
}

// FactRepo describes repository interface methods.
type FactRepo struct {
	Entity  string           `json:"entity"`
	Source  string           `json:"source,omitempty"`
	Methods []FactRepoMethod `json:"methods"`
}

// FactRepoMethod is one method of a repository.
type FactRepoMethod struct {
	Name           string   `json:"name"`
	Returns        string   `json:"returns"` // one|many|count|none
	QueryKind      string   `json:"query_kind,omitempty"`
	CriteriaFields []string `json:"criteria_fields,omitempty"`
}

// FactEvent represents an event/notification struct.
type FactEvent struct {
	Name          string      `json:"name"`
	Source        string      `json:"source,omitempty"`
	PayloadFields []FactField `json:"payload_fields,omitempty"`
}

type FactConstant struct {
	Name   string `json:"name"`
	Type   string `json:"type,omitempty"`
	Value  string `json:"value,omitempty"`
	Scope  string `json:"scope,omitempty"`
	Source string `json:"source,omitempty"`
}

type FactEnum struct {
	Name   string   `json:"name"`
	Values []string `json:"values,omitempty"`
	Source string   `json:"source,omitempty"`
}

type FactEndpoint struct {
	Operation     string   `json:"operation"`
	HTTPMethod    string   `json:"http_method,omitempty"`
	HTTPPath      string   `json:"http_path,omitempty"`
	ServiceHint   string   `json:"service_hint,omitempty"`
	Source        string   `json:"source,omitempty"`
	SourceLine    int      `json:"source_line,omitempty"`
	Extractor     string   `json:"extractor,omitempty"`
	Evidence      []string `json:"evidence,omitempty"`
	AuthExpr      string   `json:"auth_expr,omitempty"`
	Transactional bool     `json:"transactional,omitempty"`
	TxReadOnly    bool     `json:"tx_read_only,omitempty"`
}

type FactCallRef struct {
	Target         string   `json:"target"`
	ResolvedTarget string   `json:"resolved_target,omitempty"`
	Kind           string   `json:"kind,omitempty"`
	Evidence       []string `json:"evidence,omitempty"`
}

type FactCallEdge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Kind   string `json:"kind,omitempty"`
	Source string `json:"source,omitempty"`
}

type FactMapper struct {
	Name    string             `json:"name"`
	Source  string             `json:"source,omitempty"`
	Uses    []string           `json:"uses,omitempty"`
	Methods []FactMapperMethod `json:"methods,omitempty"`
}

type FactMapperMethod struct {
	Name       string `json:"name"`
	SourceType string `json:"source_type,omitempty"`
	TargetType string `json:"target_type,omitempty"`
	Many       bool   `json:"many,omitempty"`
}

type FactErrorContract struct {
	Exception string `json:"exception"`
	Status    string `json:"status,omitempty"`
	HTTPCode  int    `json:"http_code,omitempty"`
	Handler   string `json:"handler,omitempty"`
	Source    string `json:"source,omitempty"`
}

type FactSecurityRule struct {
	Scope       string `json:"scope,omitempty"` // global|method|endpoint
	Pattern     string `json:"pattern,omitempty"`
	Requirement string `json:"requirement,omitempty"`
	Source      string `json:"source,omitempty"`
}

func runExtract(args []string) {
	fs := flag.NewFlagSet("extract", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	from := fs.String("from", "auto", "source type: go|java|proto|grpc|bytecode|openapi|sql|auto")
	javaParser := fs.String("java-parser", "auto", "java parser backend for --from java: auto|regex|treesitter|antlr")
	out := fs.String("out", "", "write JSON to file (default: stdout)")

	var positional []string
	filtered := args[:0]
	for _, a := range args {
		if !strings.HasPrefix(a, "-") && len(filtered) == 0 && !strings.Contains(a, "=") {
			positional = append(positional, a)
		} else {
			filtered = append(filtered, a)
		}
	}
	if err := fs.Parse(filtered); err != nil {
		fmt.Fprintf(os.Stderr, "extract: %v\n", err)
		os.Exit(1)
	}

	srcPath := "."
	if len(positional) > 0 {
		srcPath = positional[0]
	} else if fs.NArg() > 0 {
		srcPath = fs.Arg(0)
	}
	srcPath = filepath.Clean(srcPath)

	sourceType := strings.ToLower(strings.TrimSpace(*from))
	if sourceType == "auto" || sourceType == "" {
		sourceType = detectSourceType(srcPath)
	}

	var env FactsEnvelope
	var err error
	env, err = extractViaTransformWithOptions(sourceType, srcPath, extractTransformOptions{
		JavaParserBackend: strings.TrimSpace(*javaParser),
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "extract FAILED: %v\n", err)
		os.Exit(1)
	}

	env.Schema = "ang/facts/v1"
	env.SourceType = sourceType
	env.SourcePath = srcPath

	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "extract: marshal json: %v\n", err)
		os.Exit(1)
	}
	data = append(data, '\n')

	if *out != "" {
		if err := os.WriteFile(*out, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "extract: write %s: %v\n", *out, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "wrote %s\n", *out)
		return
	}
	os.Stdout.Write(data)
}

// detectSourceType scans a path for clues about what source type it is.
func detectSourceType(path string) string {
	if v := strings.TrimSpace(transform.DetectSourceType(path)); v != "" {
		return v
	}
	// 0. Look for Java project markers
	for _, marker := range []string{"pom.xml", "build.gradle", "build.gradle.kts"} {
		if _, err := os.Stat(filepath.Join(path, marker)); err == nil {
			return "java"
		}
	}

	// 1. Look for openapi/swagger files at top level
	openapiNames := []string{"openapi.json", "openapi.yaml", "openapi.yml", "swagger.json", "swagger.yaml", "swagger.yml"}
	for _, name := range openapiNames {
		if _, err := os.Stat(filepath.Join(path, name)); err == nil {
			return "openapi"
		}
	}
	// If path itself is a file, check extension
	if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".yaml", ".yml", ".json":
			return "openapi"
		case ".sql":
			return "sql"
		case ".go":
			return "go"
		case ".java":
			return "java"
		}
	}
	// 2. Check for *.sql in top-level dir
	entries, err := os.ReadDir(path)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".sql") {
				return "sql"
			}
		}
	}
	// 3. Walk subdirs for .sql or .go
	foundSQL := false
	foundGo := false
	foundJava := false
	filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		lower := strings.ToLower(d.Name())
		if strings.HasSuffix(lower, ".sql") {
			foundSQL = true
		}
		if strings.HasSuffix(lower, ".go") {
			foundGo = true
		}
		if strings.HasSuffix(lower, ".java") {
			foundJava = true
		}
		if foundSQL || foundGo || foundJava {
			return filepath.SkipAll
		}
		return nil
	})
	if foundSQL {
		return "sql"
	}
	if foundJava {
		return "java"
	}
	if foundGo {
		return "go"
	}
	return "go"
}

type extractTransformOptions struct {
	JavaParserBackend string
}

func extractViaTransform(sourceType, srcPath string) (FactsEnvelope, error) {
	return extractViaTransformWithOptions(sourceType, srcPath, extractTransformOptions{})
}

func extractViaTransformWithOptions(sourceType, srcPath string, opts extractTransformOptions) (FactsEnvelope, error) {
	tf, err := transform.ExtractFactsWithJavaParser(sourceType, srcPath, opts.JavaParserBackend)
	if err != nil {
		return FactsEnvelope{}, err
	}
	data, err := json.Marshal(tf)
	if err != nil {
		return FactsEnvelope{}, err
	}
	var env FactsEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return FactsEnvelope{}, err
	}
	return env, nil
}

// toPascalCase converts snake_case or kebab-case to PascalCase.
func toPascalCase(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})
	var b strings.Builder
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		runes := []rune(p)
		runes[0] = unicode.ToUpper(runes[0])
		b.WriteString(string(runes))
	}
	return b.String()
}

// toSnakePlural converts PascalCase entity name to snake_case plural (table hint).
func toSnakePlural(name string) string {
	var b strings.Builder
	runes := []rune(name)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			b.WriteRune('_')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	base := b.String()
	// simple pluralization
	switch {
	case strings.HasSuffix(base, "y"):
		return base[:len(base)-1] + "ies"
	case strings.HasSuffix(base, "s") || strings.HasSuffix(base, "x") || strings.HasSuffix(base, "z"):
		return base + "es"
	default:
		return base + "s"
	}
}

// singularPascal converts a snake_case plural table name to a PascalCase singular entity name.
func singularPascal(snake string) string {
	lower := strings.ToLower(strings.TrimSpace(snake))
	// de-pluralize
	switch {
	case strings.HasSuffix(lower, "ies"):
		lower = lower[:len(lower)-3] + "y"
	case strings.HasSuffix(lower, "sses") || strings.HasSuffix(lower, "xes") || strings.HasSuffix(lower, "zes"):
		lower = lower[:len(lower)-2]
	case strings.HasSuffix(lower, "s") && !strings.HasSuffix(lower, "ss") && len(lower) > 1:
		lower = lower[:len(lower)-1]
	}
	return toPascalCase(lower)
}
