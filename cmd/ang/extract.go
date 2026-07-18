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
	"github.com/strogmv/ang/compiler/facts"
)

// These aliases keep the cmd/ang internal API source-compatible while facts
// becomes the importable owner of the ang/facts/v1 model.
type FactsEnvelope = facts.Envelope
type FactEntity = facts.Entity
type FactField = facts.Field
type FactOp = facts.Operation
type FactRepo = facts.Repository
type FactRepoMethod = facts.RepositoryMethod
type FactEvent = facts.Event
type FactConstant = facts.Constant
type FactEnum = facts.Enum
type FactEndpoint = facts.Endpoint
type FactCallRef = facts.CallRef
type FactCallEdge = facts.CallEdge
type FactMapper = facts.Mapper
type FactMapperMethod = facts.MapperMethod
type FactErrorContract = facts.ErrorContract
type FactSecurityRule = facts.SecurityRule

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
