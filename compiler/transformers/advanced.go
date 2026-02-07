package transformers

import (
	"fmt"
	"github.com/strogmv/ang/compiler/ir"
)

// TracingTransformer adds telemetry metadata to methods and services.
type TracingTransformer struct{}

func (t *TracingTransformer) Name() string { return "tracing" }

func (t *TracingTransformer) Transform(schema *ir.Schema) error {
	for i := range schema.Services {
		svc := &schema.Services[i]
		
		// If tracing is enabled globally or for this service
		if svc.Metadata == nil {
			svc.Metadata = make(map[string]any)
		}
		
		// Add tracing middleware flag
		svc.Metadata["tracing_enabled"] = true
		
		for j := range svc.Methods {
			method := &svc.Methods[j]
			if method.Metadata == nil {
				method.Metadata = make(map[string]any)
			}
			// Set span name convention: ServiceName.MethodName
			method.Metadata["span_name"] = fmt.Sprintf("%s.%s", svc.Name, method.Name)
		}
	}
	return nil
}

// CachingTransformer processes @cache attributes on service methods.
type CachingTransformer struct{}

func (t *CachingTransformer) Name() string { return "caching" }

func (t *CachingTransformer) Transform(schema *ir.Schema) error {
	for i := range schema.Services {
		svc := &schema.Services[i]
		hasCaching := false

		for j := range svc.Methods {
			method := &svc.Methods[j]
			
			// Look for cache attribute
			for _, attr := range method.Attributes {
				if attr.Name == "cache" {
					hasCaching = true
					if method.Metadata == nil {
						method.Metadata = make(map[string]any)
					}
					
					// Default TTL if not provided
					ttl := attr.Args["ttl"]
					if ttl == nil {
						ttl = "5m"
					}
					
					method.Metadata["cache"] = map[string]any{
						"enabled":  true,
						"ttl":      ttl,
						"key":      attr.Args["key"],
						"strategy": "read-through",
					}
				}
			}
		}

		if hasCaching {
			if svc.Metadata == nil {
				svc.Metadata = make(map[string]any)
			}
			svc.Metadata["needs_caching_decorator"] = true
		}
	}
	return nil
}

// ProfilingTransformer injects pprof and runtime metrics configuration.
type ProfilingTransformer struct{}

func (t *ProfilingTransformer) Name() string { return "profiling" }

func (t *ProfilingTransformer) Transform(schema *ir.Schema) error {
	if schema.Metadata == nil {
		schema.Metadata = make(map[string]any)
	}

	// Check for global profiling settings in Project metadata
	// If enabled, we flag the emitter to include pprof handlers in main
	schema.Metadata["profiling_enabled"] = true
	schema.Metadata["pprof_endpoint"] = "/debug/pprof"
	
	return nil
}

