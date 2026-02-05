// Package transformers provides attribute hooks.
// Hooks are triggered when specific attributes are found in the IR.
package transformers

import (
	"fmt"
	"sync"

	"github.com/strogmv/ang/compiler/ir"
)

// Hook is called when an attribute is found.
type Hook interface {
	// Attribute returns the attribute name this hook handles.
	Attribute() string

	// OnField is called when the attribute is found on a field.
	OnField(schema *ir.Schema, entity *ir.Entity, field *ir.Field, attr ir.Attribute) error

	// OnEntity is called when the attribute is found on an entity.
	OnEntity(schema *ir.Schema, entity *ir.Entity, attr ir.Attribute) error

	// OnService is called when the attribute is found on a service.
	OnService(schema *ir.Schema, service *ir.Service, attr ir.Attribute) error

	// OnMethod is called when the attribute is found on a method.
	OnMethod(schema *ir.Schema, service *ir.Service, method *ir.Method, attr ir.Attribute) error
}

// HookRegistry manages attribute hooks.
type HookRegistry struct {
	mu    sync.RWMutex
	hooks map[string][]Hook
}

// NewHookRegistry creates a new hook registry.
func NewHookRegistry() *HookRegistry {
	return &HookRegistry{
		hooks: make(map[string][]Hook),
	}
}

// Register adds a hook to the registry.
func (r *HookRegistry) Register(h Hook) {
	r.mu.Lock()
	defer r.mu.Unlock()

	attr := h.Attribute()
	r.hooks[attr] = append(r.hooks[attr], h)
}

// Process scans the schema and triggers hooks for all attributes.
func (r *HookRegistry) Process(schema *ir.Schema) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Process entities
	for i := range schema.Entities {
		entity := &schema.Entities[i]

		// Entity-level attributes
		for _, attr := range getEntityAttrs(entity) {
			if hooks, ok := r.hooks[attr.Name]; ok {
				for _, h := range hooks {
					if err := h.OnEntity(schema, entity, attr); err != nil {
						return fmt.Errorf("hook %s on entity %s: %w", attr.Name, entity.Name, err)
					}
				}
			}
		}

		// Field-level attributes
		for j := range entity.Fields {
			field := &entity.Fields[j]
			for _, attr := range field.Attributes {
				if hooks, ok := r.hooks[attr.Name]; ok {
					for _, h := range hooks {
						if err := h.OnField(schema, entity, field, attr); err != nil {
							return fmt.Errorf("hook %s on field %s.%s: %w", attr.Name, entity.Name, field.Name, err)
						}
					}
				}
			}
		}
	}

	// Process services
	for i := range schema.Services {
		service := &schema.Services[i]

		// Service-level attributes
		for _, attr := range getServiceAttrs(service) {
			if hooks, ok := r.hooks[attr.Name]; ok {
				for _, h := range hooks {
					if err := h.OnService(schema, service, attr); err != nil {
						return fmt.Errorf("hook %s on service %s: %w", attr.Name, service.Name, err)
					}
				}
			}
		}

		// Method-level attributes
		for j := range service.Methods {
			method := &service.Methods[j]
			for _, attr := range getMethodAttrs(method) {
				if hooks, ok := r.hooks[attr.Name]; ok {
					for _, h := range hooks {
						if err := h.OnMethod(schema, service, method, attr); err != nil {
							return fmt.Errorf("hook %s on method %s.%s: %w", attr.Name, service.Name, method.Name, err)
						}
					}
				}
			}
		}
	}

	return nil
}

// Helper functions to extract attributes from metadata
func getEntityAttrs(e *ir.Entity) []ir.Attribute {
	if e.Metadata == nil {
		return nil
	}
	if attrs, ok := e.Metadata["attributes"].([]ir.Attribute); ok {
		return attrs
	}
	return nil
}

func getServiceAttrs(s *ir.Service) []ir.Attribute {
	if s.Metadata == nil {
		return nil
	}
	if attrs, ok := s.Metadata["attributes"].([]ir.Attribute); ok {
		return attrs
	}
	return nil
}

func getMethodAttrs(m *ir.Method) []ir.Attribute {
	if m.Metadata == nil {
		return nil
	}
	if attrs, ok := m.Metadata["attributes"].([]ir.Attribute); ok {
		return attrs
	}
	return nil
}

// DefaultHookRegistry returns a registry with built-in hooks.
func DefaultHookRegistry() *HookRegistry {
	r := NewHookRegistry()
	r.Register(&DBHook{})
	r.Register(&ValidateHook{})
	r.Register(&ImageHook{})
	r.Register(&FileHook{})
	r.Register(&EnvHook{})
	r.Register(&CacheHook{})
	r.Register(&StripePaymentHook{})
	return r
}

// --- Built-in Hooks ---

// BaseHook provides default no-op implementations.
type BaseHook struct{}

func (h *BaseHook) OnField(schema *ir.Schema, entity *ir.Entity, field *ir.Field, attr ir.Attribute) error {
	return nil
}
func (h *BaseHook) OnEntity(schema *ir.Schema, entity *ir.Entity, attr ir.Attribute) error {
	return nil
}
func (h *BaseHook) OnService(schema *ir.Schema, service *ir.Service, attr ir.Attribute) error {
	return nil
}
func (h *BaseHook) OnMethod(schema *ir.Schema, service *ir.Service, method *ir.Method, attr ir.Attribute) error {
	return nil
}

// DBHook processes @db attributes.
type DBHook struct{ BaseHook }

func (h *DBHook) Attribute() string { return "db" }

func (h *DBHook) OnField(schema *ir.Schema, entity *ir.Entity, field *ir.Field, attr ir.Attribute) error {
	if field.Metadata == nil {
		field.Metadata = make(map[string]any)
	}

	if sqlType, ok := attr.Args["type"].(string); ok {
		field.Metadata["sql_type"] = sqlType
	}
	if pk, ok := attr.Args["primary_key"].(bool); ok && pk {
		field.Metadata["primary_key"] = true
	}
	if unique, ok := attr.Args["unique"].(bool); ok && unique {
		field.Metadata["unique"] = true
	}
	if index, ok := attr.Args["index"].(bool); ok && index {
		field.Metadata["index"] = true
	}

	return nil
}

// ValidateHook processes @validate attributes.
type ValidateHook struct{ BaseHook }

func (h *ValidateHook) Attribute() string { return "validate" }

func (h *ValidateHook) OnField(schema *ir.Schema, entity *ir.Entity, field *ir.Field, attr ir.Attribute) error {
	if field.Metadata == nil {
		field.Metadata = make(map[string]any)
	}

	if rule, ok := attr.Args["rule"].(string); ok {
		field.Metadata["validate_rule"] = rule
	} else if len(attr.Args) == 0 {
		// @validate("email") style - first positional arg
		for k, v := range attr.Args {
			field.Metadata["validate_rule"] = k
			_ = v
			break
		}
	}

	return nil
}

// ImageHook processes @image attributes.
type ImageHook struct{ BaseHook }

func (h *ImageHook) Attribute() string { return "image" }

func (h *ImageHook) OnField(schema *ir.Schema, entity *ir.Entity, field *ir.Field, attr ir.Attribute) error {
	if field.Metadata == nil {
		field.Metadata = make(map[string]any)
	}

	field.Metadata["file_kind"] = "image"
	field.Metadata["generate_thumbnail"] = true

	// Check for custom thumbnail suffix
	if suffix, ok := attr.Args["thumb_suffix"].(string); ok {
		field.Metadata["thumb_suffix"] = suffix
	} else {
		field.Metadata["thumb_suffix"] = "_thumb"
	}

	return nil
}

// FileHook processes @file attributes.
type FileHook struct{ BaseHook }

func (h *FileHook) Attribute() string { return "file" }

func (h *FileHook) OnField(schema *ir.Schema, entity *ir.Entity, field *ir.Field, attr ir.Attribute) error {
	if field.Metadata == nil {
		field.Metadata = make(map[string]any)
	}

	if kind, ok := attr.Args["kind"].(string); ok {
		field.Metadata["file_kind"] = kind
	} else {
		field.Metadata["file_kind"] = "auto"
	}

	if thumb, ok := attr.Args["thumbnail"].(bool); ok {
		field.Metadata["generate_thumbnail"] = thumb
	}

	return nil
}

// EnvHook processes @env attributes.
type EnvHook struct{ BaseHook }

func (h *EnvHook) Attribute() string { return "env" }

func (h *EnvHook) OnField(schema *ir.Schema, entity *ir.Entity, field *ir.Field, attr ir.Attribute) error {
	if field.Metadata == nil {
		field.Metadata = make(map[string]any)
	}

	// @env("VAR_NAME") - first positional argument
	for k := range attr.Args {
		field.Metadata["env_var"] = k
		break
	}

	return nil
}

// CacheHook processes @cache attributes on methods.
type CacheHook struct{ BaseHook }

func (h *CacheHook) Attribute() string { return "cache" }

func (h *CacheHook) OnMethod(schema *ir.Schema, service *ir.Service, method *ir.Method, attr ir.Attribute) error {
	if method.Metadata == nil {
		method.Metadata = make(map[string]any)
	}

	if ttl, ok := attr.Args["ttl"].(string); ok {
		method.CacheTTL = ttl
		method.Metadata["cache_enabled"] = true
	}

	if key, ok := attr.Args["key"].(string); ok {
		method.Metadata["cache_key_template"] = key
	}

	return nil
}

// StripePaymentHook processes @stripe_payment attributes.
// This is an example of a domain-specific hook that could be in a plugin.
type StripePaymentHook struct{ BaseHook }

func (h *StripePaymentHook) Attribute() string { return "stripe_payment" }

func (h *StripePaymentHook) OnService(schema *ir.Schema, service *ir.Service, attr ir.Attribute) error {
	if service.Metadata == nil {
		service.Metadata = make(map[string]any)
	}

	service.Metadata["stripe_enabled"] = true

	// Add payment methods to the service
	amountField := "amount"
	if f, ok := attr.Args["field"].(string); ok {
		amountField = f
	}
	service.Metadata["stripe_amount_field"] = amountField

	// Add CreatePaymentIntent method
	service.Methods = append(service.Methods, ir.Method{
		Name: "CreatePaymentIntent",
		Input: &ir.Entity{
			Name: "CreatePaymentIntentInput",
			Fields: []ir.Field{
				{Name: "amount", Type: ir.TypeRef{Kind: ir.KindInt64}},
				{Name: "currency", Type: ir.TypeRef{Kind: ir.KindString}},
				{Name: "customer_id", Type: ir.TypeRef{Kind: ir.KindString}, Optional: true},
			},
		},
		Output: &ir.Entity{
			Name: "PaymentIntent",
			Fields: []ir.Field{
				{Name: "id", Type: ir.TypeRef{Kind: ir.KindString}},
				{Name: "client_secret", Type: ir.TypeRef{Kind: ir.KindString}},
				{Name: "status", Type: ir.TypeRef{Kind: ir.KindString}},
			},
		},
		Metadata: map[string]any{
			"generated_by": "stripe_payment_hook",
		},
	})

	return nil
}
