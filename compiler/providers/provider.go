// Package providers implements swappable template bundles.
// Each provider contains templates for a specific technology stack.
package providers

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"text/template"

	"github.com/strogmv/ang/compiler/ir"
)

// Provider is the interface for template bundles.
// Each provider knows how to generate code for a specific stack.
type Provider interface {
	// Name returns the provider identifier (e.g., "go-chi-postgres").
	Name() string

	// Supports checks if this provider can handle the given target.
	Supports(target ir.Target) bool

	// Templates returns the template filesystem.
	Templates() fs.FS

	// FuncMap returns custom template functions for this provider.
	FuncMap() template.FuncMap

	// TypeMapping returns type conversions for this provider.
	TypeMapping() TypeMap
}

// TypeMap maps IR types to target language types.
type TypeMap struct {
	Mappings map[ir.TypeKind]TypeInfo
	Custom   map[string]TypeInfo // For custom type names
}

// TypeInfo describes a type in the target language.
type TypeInfo struct {
	Type       string // e.g., "string", "time.Time"
	Package    string // e.g., "time" (import path)
	SQLType    string // e.g., "TEXT", "TIMESTAMPTZ"
	NullHelper string // e.g., "sql.NullString"
	ZeroValue  string // e.g., `""`, "0", "nil"
	TSType     string // TypeScript equivalent
	ZodType    string // Zod schema type
}

// Registry holds all registered providers.
type Registry struct {
	providers map[string]Provider
}

// NewRegistry creates a new provider registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// Register adds a provider to the registry.
func (r *Registry) Register(p Provider) {
	r.providers[p.Name()] = p
}

// Get returns a provider by name.
func (r *Registry) Get(name string) (Provider, bool) {
	p, ok := r.providers[name]
	return p, ok
}

// Find returns the first provider that supports the target.
func (r *Registry) Find(target ir.Target) (Provider, bool) {
	for _, p := range r.providers {
		if p.Supports(target) {
			return p, true
		}
	}
	return nil, false
}

// List returns all registered provider names.
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// LoadFromDir loads a provider from a directory.
// The directory should contain templates and a provider.cue config.
func LoadFromDir(path string) (Provider, error) {
	// Check if directory exists
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("provider directory not found: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", path)
	}

	return &DirProvider{
		path: path,
		name: filepath.Base(path),
	}, nil
}

// DirProvider is a provider loaded from a directory.
type DirProvider struct {
	path    string
	name    string
	typeMap TypeMap
	funcMap template.FuncMap
	target  ir.Target
}

func (p *DirProvider) Name() string { return p.name }

func (p *DirProvider) Supports(target ir.Target) bool {
	// For now, check if the name matches the target
	// e.g., "go-chi-postgres" matches {Lang: "go", Framework: "chi", DB: "postgres"}
	expected := fmt.Sprintf("%s-%s-%s", target.Lang, target.Framework, target.DB)
	return p.name == expected
}

func (p *DirProvider) Templates() fs.FS {
	return os.DirFS(p.path)
}

func (p *DirProvider) FuncMap() template.FuncMap {
	return p.funcMap
}

func (p *DirProvider) TypeMapping() TypeMap {
	return p.typeMap
}

// --- Built-in Providers ---

// GoChiPostgresProvider is the default Go provider.
type GoChiPostgresProvider struct {
	templates embed.FS
}

func NewGoChiPostgresProvider(templates embed.FS) *GoChiPostgresProvider {
	return &GoChiPostgresProvider{templates: templates}
}

func (p *GoChiPostgresProvider) Name() string { return "go-chi-postgres" }

func (p *GoChiPostgresProvider) Supports(target ir.Target) bool {
	return target.Lang == "go" &&
		(target.Framework == "chi" || target.Framework == "") &&
		(target.DB == "postgres" || target.DB == "")
}

func (p *GoChiPostgresProvider) Templates() fs.FS {
	return p.templates
}

func (p *GoChiPostgresProvider) FuncMap() template.FuncMap {
	return template.FuncMap{
		"ExportName": exportName,
		"Title":      title,
		"ToLower":    toLower,
		"ToSnake":    toSnake,
		"ToCamel":    toCamel,
		"HasTime":    hasTime,
	}
}

func (p *GoChiPostgresProvider) TypeMapping() TypeMap {
	return TypeMap{
		Mappings: map[ir.TypeKind]TypeInfo{
			ir.KindString: {
				Type:       "string",
				SQLType:    "TEXT",
				NullHelper: "sql.NullString",
				ZeroValue:  `""`,
				TSType:     "string",
				ZodType:    "z.string()",
			},
			ir.KindInt: {
				Type:       "int",
				SQLType:    "INTEGER",
				NullHelper: "sql.NullInt64",
				ZeroValue:  "0",
				TSType:     "number",
				ZodType:    "z.number().int()",
			},
			ir.KindInt64: {
				Type:       "int64",
				SQLType:    "BIGINT",
				NullHelper: "sql.NullInt64",
				ZeroValue:  "0",
				TSType:     "number",
				ZodType:    "z.number().int()",
			},
			ir.KindFloat: {
				Type:       "float64",
				SQLType:    "DOUBLE PRECISION",
				NullHelper: "sql.NullFloat64",
				ZeroValue:  "0.0",
				TSType:     "number",
				ZodType:    "z.number()",
			},
			ir.KindBool: {
				Type:       "bool",
				SQLType:    "BOOLEAN",
				NullHelper: "sql.NullBool",
				ZeroValue:  "false",
				TSType:     "boolean",
				ZodType:    "z.boolean()",
			},
			ir.KindTime: {
				Type:       "time.Time",
				Package:    "time",
				SQLType:    "TIMESTAMPTZ",
				NullHelper: "sql.NullTime",
				ZeroValue:  "time.Time{}",
				TSType:     "string",
				ZodType:    "z.string().datetime()",
			},
			ir.KindUUID: {
				Type:       "string",
				SQLType:    "UUID",
				NullHelper: "sql.NullString",
				ZeroValue:  `""`,
				TSType:     "string",
				ZodType:    "z.string().uuid()",
			},
			ir.KindJSON: {
				Type:       "json.RawMessage",
				Package:    "encoding/json",
				SQLType:    "JSONB",
				NullHelper: "[]byte",
				ZeroValue:  "nil",
				TSType:     "unknown",
				ZodType:    "z.unknown()",
			},
			ir.KindAny: {
				Type:       "any",
				SQLType:    "JSONB",
				NullHelper: "any",
				ZeroValue:  "nil",
				TSType:     "unknown",
				ZodType:    "z.unknown()",
			},
		},
	}
}

// Helper functions for templates
func exportName(s string) string {
	if s == "" {
		return s
	}
	return string([]rune(s)[0]-32) + s[1:]
}

func title(s string) string {
	if s == "" {
		return s
	}
	return string([]rune(s)[0]-32) + s[1:]
}

func toLower(s string) string {
	if s == "" {
		return s
	}
	return string([]rune(s)[0]+32) + s[1:]
}

func toSnake(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		result = append(result, r)
	}
	return string(result)
}

func toCamel(s string) string {
	return s // TODO: implement proper conversion
}

func hasTime(fields []ir.Field) bool {
	for _, f := range fields {
		if f.Type.Kind == ir.KindTime {
			return true
		}
	}
	return false
}
