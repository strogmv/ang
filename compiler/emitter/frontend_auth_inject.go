package emitter

import (
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
)

func frontendEndpointKey(serviceName, rpc string) string {
	return strings.ToLower(strings.TrimSpace(serviceName)) + "|" + strings.ToLower(strings.TrimSpace(rpc))
}

func frontendAuthInjectIndex(endpoints []normalizer.Endpoint) map[string]map[string]struct{} {
	out := make(map[string]map[string]struct{}, len(endpoints))
	for _, ep := range endpoints {
		if len(ep.AuthInject) == 0 {
			continue
		}
		key := frontendEndpointKey(ep.ServiceName, ep.RPC)
		fields := make(map[string]struct{}, len(ep.AuthInject))
		for _, field := range ep.AuthInject {
			name := strings.ToLower(strings.TrimSpace(field))
			if name == "" {
				continue
			}
			fields[name] = struct{}{}
		}
		if len(fields) > 0 {
			out[key] = fields
		}
	}
	return out
}

func frontendFilterEntityFields(fields []normalizer.Field, injected map[string]struct{}) []normalizer.Field {
	if len(fields) == 0 || len(injected) == 0 {
		return append([]normalizer.Field(nil), fields...)
	}
	out := make([]normalizer.Field, 0, len(fields))
	for _, field := range fields {
		fieldName := strings.ToLower(strings.TrimSpace(field.Name))
		jsonName := strings.ToLower(JSONName(field.Name))
		if _, skip := injected[fieldName]; skip {
			continue
		}
		if _, skip := injected[jsonName]; skip {
			continue
		}
		out = append(out, field)
	}
	return out
}

func frontendFilterEntity(ent normalizer.Entity, injected map[string]struct{}) normalizer.Entity {
	filtered := ent
	filtered.Fields = frontendFilterEntityFields(ent.Fields, injected)
	return filtered
}

func applyFrontendAuthInjectFilters(services []normalizer.Service, entities []normalizer.Entity, endpoints []normalizer.Endpoint) ([]normalizer.Service, []normalizer.Entity) {
	injectByMethod := frontendAuthInjectIndex(endpoints)
	if len(injectByMethod) == 0 {
		return services, entities
	}

	filteredServices := make([]normalizer.Service, len(services))
	for i, svc := range services {
		filteredSvc := svc
		filteredSvc.Methods = make([]normalizer.Method, len(svc.Methods))
		for j, method := range svc.Methods {
			filteredMethod := method
			if injected := injectByMethod[frontendEndpointKey(svc.Name, method.Name)]; len(injected) > 0 {
				filteredMethod.Input = frontendFilterEntity(method.Input, injected)
			}
			filteredSvc.Methods[j] = filteredMethod
		}
		filteredServices[i] = filteredSvc
	}

	filteredEntities := make([]normalizer.Entity, len(entities))
	copy(filteredEntities, entities)
	entityIndex := make(map[string]int, len(filteredEntities))
	for i, ent := range filteredEntities {
		entityIndex[ent.Name] = i
	}

	for _, svc := range filteredServices {
		for _, method := range svc.Methods {
			if strings.TrimSpace(method.Input.Name) == "" {
				continue
			}
			if idx, ok := entityIndex[method.Input.Name]; ok {
				filteredEntities[idx] = method.Input
				continue
			}
			entityIndex[method.Input.Name] = len(filteredEntities)
			filteredEntities = append(filteredEntities, method.Input)
		}
	}

	for _, ep := range endpoints {
		injected := injectByMethod[frontendEndpointKey(ep.ServiceName, ep.RPC)]
		if len(injected) == 0 {
			continue
		}
		requestName := strings.TrimSpace(ep.RPC) + "Request"
		if requestName == "Request" {
			continue
		}
		idx, ok := entityIndex[requestName]
		if !ok {
			continue
		}
		filteredEntities[idx] = frontendFilterEntity(filteredEntities[idx], injected)
	}

	return filteredServices, filteredEntities
}
