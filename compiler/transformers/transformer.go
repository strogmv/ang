// Package transformers provides a plugin system for enriching the IR.
// Each transformer can inspect and modify the schema based on attributes,
// conventions, or external configuration.
package transformers

import (
	"strings"

	"github.com/strogmv/ang/compiler/ir"
)

// Transformer is the interface that all transformers must implement.
// Transformers modify the IR in place, adding computed fields, methods,
// or metadata based on attributes and conventions.
type Transformer interface {
	// Name returns the transformer's identifier.
	Name() string

	// Transform modifies the schema in place.
	// It should be idempotent: calling it twice should have no effect.
	Transform(schema *ir.Schema) error
}

// Registry holds all registered transformers.
type Registry struct {
	transformers []Transformer
}

// NewRegistry creates a new transformer registry.
func NewRegistry() *Registry {
	return &Registry{
		transformers: make([]Transformer, 0),
	}
}

// Register adds a transformer to the registry.
func (r *Registry) Register(t Transformer) {
	r.transformers = append(r.transformers, t)
}

// Apply runs all transformers on the schema in order.
func (r *Registry) Apply(schema *ir.Schema) error {
	for _, t := range r.transformers {
		if err := t.Transform(schema); err != nil {
			return err
		}
	}
	return nil
}

// DefaultRegistry returns a registry with all built-in transformers.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(&ImageTransformer{})
	r.Register(&ValidationTransformer{})
	r.Register(&TimestampTransformer{})
	r.Register(&SoftDeleteTransformer{})
	r.Register(&TracingTransformer{})
	r.Register(&CachingTransformer{})
	return r
}

// --- Built-in Transformers ---

// ImageTransformer processes @image attributes.
// When it finds a field with @image, it adds a thumbnail field.
type ImageTransformer struct {
	ThumbSuffix string
}

func (t *ImageTransformer) Name() string { return "image" }

func (t *ImageTransformer) Transform(schema *ir.Schema) error {
	suffix := t.ThumbSuffix
	if suffix == "" {
		suffix = "_thumb"
	}

	for i := range schema.Entities {
		entity := &schema.Entities[i]
		var newFields []ir.Field

		for _, field := range entity.Fields {
			newFields = append(newFields, field)

			// Check for @image attribute
			for _, attr := range field.Attributes {
				if attr.Name == "image" {
					// Add thumbnail field
					thumbField := ir.Field{
						Name:     field.Name + suffix,
						Type:     ir.TypeRef{Kind: ir.KindString},
						Optional: true,
						Metadata: map[string]any{
							"generated_by": "image_transformer",
							"source_field": field.Name,
						},
					}
					newFields = append(newFields, thumbField)

					// Add metadata to original field
					if field.Metadata == nil {
						field.Metadata = make(map[string]any)
					}
					field.Metadata["has_thumbnail"] = true
					field.Metadata["thumbnail_field"] = thumbField.Name
				}
			}
		}

		entity.Fields = newFields
	}

	return nil
}

// ValidationTransformer processes @validate attributes.
type ValidationTransformer struct{}

func (t *ValidationTransformer) Name() string { return "validation" }

func (t *ValidationTransformer) Transform(schema *ir.Schema) error {
	for i := range schema.Entities {
		entity := &schema.Entities[i]
		for j := range entity.Fields {
			field := &entity.Fields[j]

			for _, attr := range field.Attributes {
				if attr.Name == "validate" {
					if field.Metadata == nil {
						field.Metadata = make(map[string]any)
					}
					rule, _ := attr.Args["rule"].(string)
					if field.Optional && rule != "" && !strings.Contains(rule, "omitempty") {
						rule += ",omitempty"
					}
					field.Metadata["validate_tag"] = rule
				}
			}
		}
	}
	return nil
}

// TimestampTransformer adds created_at/updated_at fields.
type TimestampTransformer struct{}

func (t *TimestampTransformer) Name() string { return "timestamps" }

func (t *TimestampTransformer) Transform(schema *ir.Schema) error {
	for i := range schema.Entities {
		entity := &schema.Entities[i]

		// Check if entity has @timestamps attribute (in metadata)
		if entity.Metadata == nil {
			continue
		}
		if _, ok := entity.Metadata["timestamps"]; !ok {
			continue
		}

		hasCreatedAt := false
		hasUpdatedAt := false

		for _, f := range entity.Fields {
			if f.Name == "created_at" {
				hasCreatedAt = true
			}
			if f.Name == "updated_at" {
				hasUpdatedAt = true
			}
		}

		if !hasCreatedAt {
			entity.Fields = append(entity.Fields, ir.Field{
				Name: "created_at",
				Type: ir.TypeRef{Kind: ir.KindTime},
				Metadata: map[string]any{
					"generated_by": "timestamp_transformer",
					"auto_set":     "on_create",
				},
			})
		}

		if !hasUpdatedAt {
			entity.Fields = append(entity.Fields, ir.Field{
				Name: "updated_at",
				Type: ir.TypeRef{Kind: ir.KindTime},
				Metadata: map[string]any{
					"generated_by": "timestamp_transformer",
					"auto_set":     "on_update",
				},
			})
		}
	}
	return nil
}

// SoftDeleteTransformer adds deleted_at field for soft deletes.
type SoftDeleteTransformer struct{}

func (t *SoftDeleteTransformer) Name() string { return "soft_delete" }

func (t *SoftDeleteTransformer) Transform(schema *ir.Schema) error {
	for i := range schema.Entities {
		entity := &schema.Entities[i]

		if entity.Metadata == nil {
			continue
		}
		if _, ok := entity.Metadata["soft_delete"]; !ok {
			continue
		}

		hasDeletedAt := false
		for _, f := range entity.Fields {
			if f.Name == "deleted_at" {
				hasDeletedAt = true
				break
			}
		}

		if !hasDeletedAt {
			entity.Fields = append(entity.Fields, ir.Field{
				Name:     "deleted_at",
				Type:     ir.TypeRef{Kind: ir.KindTime},
				Optional: true,
				Metadata: map[string]any{
					"generated_by": "soft_delete_transformer",
				},
			})
		}
	}
	return nil
}
