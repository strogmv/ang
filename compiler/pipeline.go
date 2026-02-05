// Package compiler provides the main compilation pipeline.
// This is the new architecture: CUE -> Parser -> Normalizer -> IR -> Transformers -> Emitter
package compiler

import (
	"fmt"

	"github.com/strogmv/ang/compiler/ir"
	"github.com/strogmv/ang/compiler/normalizer"
	"github.com/strogmv/ang/compiler/providers"
	"github.com/strogmv/ang/compiler/transformers"
)

// Pipeline orchestrates the compilation process.
// It's designed to be extensible and provider-agnostic.
type Pipeline struct {
	// Parser loads CUE files
	parser *normalizer.Normalizer // Using existing normalizer as parser for now

	// Transformers enrich the IR
	transformers *transformers.Registry

	// Hooks process attributes
	hooks *transformers.HookRegistry

	// Providers supply templates
	providers *providers.Registry

	// Config
	templatesDir string
	outputDir    string
	frontendDir  string
}

// PipelineConfig configures the pipeline.
type PipelineConfig struct {
	TemplatesDir string
	OutputDir    string
	FrontendDir  string
}

// NewPipeline creates a new compilation pipeline.
func NewPipeline(cfg PipelineConfig) *Pipeline {
	return &Pipeline{
		parser:       normalizer.New(),
		transformers: transformers.DefaultRegistry(),
		hooks:        transformers.DefaultHookRegistry(),
		providers:    providers.NewRegistry(),
		templatesDir: cfg.TemplatesDir,
		outputDir:    cfg.OutputDir,
		frontendDir:  cfg.FrontendDir,
	}
}

// RegisterTransformer adds a custom transformer.
func (p *Pipeline) RegisterTransformer(t transformers.Transformer) {
	p.transformers.Register(t)
}

// RegisterHook adds a custom attribute hook.
func (p *Pipeline) RegisterHook(h transformers.Hook) {
	p.hooks.Register(h)
}

// RegisterProvider adds a custom template provider.
func (p *Pipeline) RegisterProvider(prov providers.Provider) {
	p.providers.Register(prov)
}

// CompileResult contains the compilation output.
type CompileResult struct {
	Schema   *ir.Schema
	Provider providers.Provider
	Errors   []error
	Warnings []string
}

// Compile runs the full compilation pipeline.
// Steps:
// 1. Parse CUE files
// 2. Normalize to legacy types
// 3. Convert to universal IR
// 4. Apply transformers
// 5. Process attribute hooks
// 6. Select provider based on target
// 7. Emit code using provider templates
func (p *Pipeline) Compile(cuePaths []string) (*CompileResult, error) {
	result := &CompileResult{}

	// Step 1-2: Parse and normalize CUE (using existing normalizer)
	// This would be replaced with direct IR parsing in the future
	legacyEntities, legacyServices, err := p.parseAndNormalize(cuePaths)
	if err != nil {
		return nil, fmt.Errorf("parse/normalize: %w", err)
	}

	// Step 3: Convert to universal IR
	// In the future, this conversion won't be needed - we'll parse directly to IR
	schema := ir.ConvertFromNormalizer(
		legacyEntities,
		legacyServices,
		nil, // events
		nil, // errors
		nil, // endpoints
		nil, // repos
		normalizer.ConfigDef{},
		nil, // auth
		nil, // rbac
		nil, // schedules
		nil, // views
		normalizer.ProjectDef{},
	)
	result.Schema = schema

	// Step 4: Apply transformers
	if err := p.transformers.Apply(schema); err != nil {
		return nil, fmt.Errorf("transformers: %w", err)
	}

	// Step 5: Process attribute hooks
	if err := p.hooks.Process(schema); err != nil {
		return nil, fmt.Errorf("hooks: %w", err)
	}

	// Step 6: Select provider
	provider, found := p.providers.Find(schema.Project.Target)
	if !found {
		// Fall back to default provider
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("No provider found for target %+v, using default", schema.Project.Target))
		// Use the built-in go-chi-postgres provider
		// provider = providers.NewGoChiPostgresProvider(templates.FS)
	}
	result.Provider = provider

	// Step 7: Emit code
	// This would call the emitter with the provider's templates
	// For now, the existing emitter handles this

	return result, nil
}

// parseAndNormalize uses the existing normalizer.
// This is a temporary bridge until we parse directly to IR.
func (p *Pipeline) parseAndNormalize(cuePaths []string) ([]normalizer.Entity, []normalizer.Service, error) {
	// TODO: Use the existing normalizer to parse CUE
	// This is a placeholder showing the integration point
	return nil, nil, nil
}

// --- Example: How to use the new architecture ---

/*
Usage example:

	// Create pipeline with config
	pipeline := compiler.NewPipeline(compiler.PipelineConfig{
		TemplatesDir: "templates",
		OutputDir:    ".",
		FrontendDir:  "sdk",
	})

	// Register custom transformer for domain-specific logic
	pipeline.RegisterTransformer(&MySearchTransformer{})

	// Register custom hook for @my_attribute
	pipeline.RegisterHook(&MyAttributeHook{})

	// Register custom provider for Rust/Axum
	pipeline.RegisterProvider(&RustAxumProvider{})

	// Compile
	result, err := pipeline.Compile([]string{"cue/domain", "cue/api"})
	if err != nil {
		log.Fatal(err)
	}

	// The schema is now enriched and ready for code generation
	fmt.Printf("Compiled %d entities, %d services\n",
		len(result.Schema.Entities),
		len(result.Schema.Services))
*/

// --- Custom Transformer Example ---

/*
type SearchTransformer struct{}

func (t *SearchTransformer) Name() string { return "search" }

func (t *SearchTransformer) Transform(schema *ir.Schema) error {
	// Find entities with @searchable attribute
	for i := range schema.Entities {
		entity := &schema.Entities[i]
		if entity.Metadata["searchable"] == true {
			// Add to search index
			// Generate search methods in service
		}
	}
	return nil
}
*/

// --- Custom Provider Example ---

/*
type RustAxumProvider struct{}

func (p *RustAxumProvider) Name() string { return "rust-axum-postgres" }

func (p *RustAxumProvider) Supports(target ir.Target) bool {
	return target.Lang == "rust" && target.Framework == "axum"
}

func (p *RustAxumProvider) Templates() fs.FS {
	return os.DirFS("templates/rust-axum")
}

func (p *RustAxumProvider) FuncMap() template.FuncMap {
	return template.FuncMap{
		"RustType": func(t ir.TypeRef) string { ... },
	}
}

func (p *RustAxumProvider) TypeMapping() providers.TypeMap {
	return providers.TypeMap{
		Mappings: map[ir.TypeKind]providers.TypeInfo{
			ir.KindString: {Type: "String", ...},
			ir.KindInt:    {Type: "i32", ...},
		},
	}
}
*/
