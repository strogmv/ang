package mcp

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler"
)

type modelSnapshot struct {
	Entities  []map[string]any
	Services  []map[string]any
	Endpoints []map[string]string
}

func buildModelSnapshot(projectPath string, includeFields bool) (modelSnapshot, error) {
	entities, services, endpoints, _, _, _, _, _, err := compiler.RunPipeline(projectPath)
	if err != nil {
		return modelSnapshot{}, err
	}

	sort.Slice(entities, func(i, j int) bool {
		return strings.ToLower(entities[i].Name) < strings.ToLower(entities[j].Name)
	})
	sort.Slice(services, func(i, j int) bool {
		return strings.ToLower(services[i].Name) < strings.ToLower(services[j].Name)
	})
	sort.Slice(endpoints, func(i, j int) bool {
		li := strings.ToUpper(endpoints[i].Method) + " " + endpoints[i].Path + " " + endpoints[i].RPC
		lj := strings.ToUpper(endpoints[j].Method) + " " + endpoints[j].Path + " " + endpoints[j].RPC
		return li < lj
	})

	entityOut := make([]map[string]any, 0, len(entities))
	for _, e := range entities {
		item := map[string]any{
			"name": e.Name,
		}
		if includeFields {
			fields := make([]string, 0, len(e.Fields))
			for _, f := range e.Fields {
				fields = append(fields, f.Name)
			}
			sort.Strings(fields)
			item["fields"] = fields
		}
		entityOut = append(entityOut, item)
	}

	serviceOut := make([]map[string]any, 0, len(services))
	for _, s := range services {
		methods := make([]string, 0, len(s.Methods))
		for _, m := range s.Methods {
			methods = append(methods, m.Name)
		}
		sort.Strings(methods)
		serviceOut = append(serviceOut, map[string]any{
			"name":    s.Name,
			"methods": methods,
		})
	}

	endpointOut := make([]map[string]string, 0, len(endpoints))
	for _, ep := range endpoints {
		endpointOut = append(endpointOut, map[string]string{
			"method": strings.ToUpper(ep.Method),
			"path":   ep.Path,
			"rpc":    ep.RPC,
		})
	}

	return modelSnapshot{
		Entities:  entityOut,
		Services:  serviceOut,
		Endpoints: endpointOut,
	}, nil
}

func filterModelSnapshot(s modelSnapshot, entityFilter, serviceFilter, endpointFilter string) modelSnapshot {
	entityFilter = strings.ToLower(strings.TrimSpace(entityFilter))
	serviceFilter = strings.ToLower(strings.TrimSpace(serviceFilter))
	endpointFilter = strings.ToLower(strings.TrimSpace(endpointFilter))

	out := modelSnapshot{
		Entities:  make([]map[string]any, 0, len(s.Entities)),
		Services:  make([]map[string]any, 0, len(s.Services)),
		Endpoints: make([]map[string]string, 0, len(s.Endpoints)),
	}
	for _, e := range s.Entities {
		name, _ := e["name"].(string)
		if entityFilter != "" && !strings.Contains(strings.ToLower(name), entityFilter) {
			continue
		}
		out.Entities = append(out.Entities, e)
	}
	for _, svc := range s.Services {
		name, _ := svc["name"].(string)
		if serviceFilter != "" && !strings.Contains(strings.ToLower(name), serviceFilter) {
			continue
		}
		out.Services = append(out.Services, svc)
	}
	for _, ep := range s.Endpoints {
		if endpointFilter == "" {
			out.Endpoints = append(out.Endpoints, ep)
			continue
		}
		key := strings.ToLower(ep["method"] + " " + ep["path"] + " " + ep["rpc"])
		if strings.Contains(key, endpointFilter) {
			out.Endpoints = append(out.Endpoints, ep)
		}
	}
	return out
}

func buildModelSnapshotFromGitRef(ref string, includeFields bool) (modelSnapshot, error) {
	tmpDir, err := os.MkdirTemp("", "ang-model-ref-*")
	if err != nil {
		return modelSnapshot{}, err
	}
	defer os.RemoveAll(tmpDir)

	cmd := exec.Command("bash", "-lc", fmt.Sprintf("git archive %q cue cue.mod | tar -x -C %q", ref, tmpDir))
	if out, err := cmd.CombinedOutput(); err != nil {
		return modelSnapshot{}, fmt.Errorf("git archive failed: %s", strings.TrimSpace(string(out)))
	}
	return buildModelSnapshot(tmpDir, includeFields)
}

func diffModelSnapshots(base, curr modelSnapshot) map[string]any {
	baseEntities := map[string][]string{}
	currEntities := map[string][]string{}
	for _, e := range base.Entities {
		name, _ := e["name"].(string)
		baseEntities[name] = toStringSlice(e["fields"])
	}
	for _, e := range curr.Entities {
		name, _ := e["name"].(string)
		currEntities[name] = toStringSlice(e["fields"])
	}

	entityAdded := []string{}
	entityRemoved := []string{}
	fieldAdded := []map[string]string{}
	fieldRemoved := []map[string]string{}

	for name := range currEntities {
		if _, ok := baseEntities[name]; !ok {
			entityAdded = append(entityAdded, name)
		}
	}
	for name := range baseEntities {
		if _, ok := currEntities[name]; !ok {
			entityRemoved = append(entityRemoved, name)
		}
	}
	for name, currFields := range currEntities {
		baseFields, ok := baseEntities[name]
		if !ok {
			continue
		}
		baseSet := map[string]bool{}
		currSet := map[string]bool{}
		for _, f := range baseFields {
			baseSet[f] = true
		}
		for _, f := range currFields {
			currSet[f] = true
		}
		for _, f := range currFields {
			if !baseSet[f] {
				fieldAdded = append(fieldAdded, map[string]string{"entity": name, "field": f})
			}
		}
		for _, f := range baseFields {
			if !currSet[f] {
				fieldRemoved = append(fieldRemoved, map[string]string{"entity": name, "field": f})
			}
		}
	}

	baseServices := map[string][]string{}
	currServices := map[string][]string{}
	for _, s := range base.Services {
		name, _ := s["name"].(string)
		baseServices[name] = toStringSlice(s["methods"])
	}
	for _, s := range curr.Services {
		name, _ := s["name"].(string)
		currServices[name] = toStringSlice(s["methods"])
	}
	serviceAdded := []string{}
	serviceRemoved := []string{}
	methodAdded := []map[string]string{}
	methodRemoved := []map[string]string{}
	for name := range currServices {
		if _, ok := baseServices[name]; !ok {
			serviceAdded = append(serviceAdded, name)
		}
	}
	for name := range baseServices {
		if _, ok := currServices[name]; !ok {
			serviceRemoved = append(serviceRemoved, name)
		}
	}
	for name, currMethods := range currServices {
		baseMethods, ok := baseServices[name]
		if !ok {
			continue
		}
		baseSet := map[string]bool{}
		currSet := map[string]bool{}
		for _, m := range baseMethods {
			baseSet[m] = true
		}
		for _, m := range currMethods {
			currSet[m] = true
		}
		for _, m := range currMethods {
			if !baseSet[m] {
				methodAdded = append(methodAdded, map[string]string{"service": name, "method": m})
			}
		}
		for _, m := range baseMethods {
			if !currSet[m] {
				methodRemoved = append(methodRemoved, map[string]string{"service": name, "method": m})
			}
		}
	}

	baseEndpoints := map[string]map[string]string{}
	currEndpoints := map[string]map[string]string{}
	for _, ep := range base.Endpoints {
		k := endpointKey(ep)
		baseEndpoints[k] = ep
	}
	for _, ep := range curr.Endpoints {
		k := endpointKey(ep)
		currEndpoints[k] = ep
	}
	endpointAdded := []map[string]string{}
	endpointRemoved := []map[string]string{}
	for k, ep := range currEndpoints {
		if _, ok := baseEndpoints[k]; !ok {
			endpointAdded = append(endpointAdded, ep)
		}
	}
	for k, ep := range baseEndpoints {
		if _, ok := currEndpoints[k]; !ok {
			endpointRemoved = append(endpointRemoved, ep)
		}
	}

	sort.Strings(entityAdded)
	sort.Strings(entityRemoved)
	sort.Strings(serviceAdded)
	sort.Strings(serviceRemoved)
	sort.Slice(fieldAdded, func(i, j int) bool {
		return fieldAdded[i]["entity"]+":"+fieldAdded[i]["field"] < fieldAdded[j]["entity"]+":"+fieldAdded[j]["field"]
	})
	sort.Slice(fieldRemoved, func(i, j int) bool {
		return fieldRemoved[i]["entity"]+":"+fieldRemoved[i]["field"] < fieldRemoved[j]["entity"]+":"+fieldRemoved[j]["field"]
	})
	sort.Slice(methodAdded, func(i, j int) bool {
		return methodAdded[i]["service"]+":"+methodAdded[i]["method"] < methodAdded[j]["service"]+":"+methodAdded[j]["method"]
	})
	sort.Slice(methodRemoved, func(i, j int) bool {
		return methodRemoved[i]["service"]+":"+methodRemoved[i]["method"] < methodRemoved[j]["service"]+":"+methodRemoved[j]["method"]
	})
	sort.Slice(endpointAdded, func(i, j int) bool { return endpointKey(endpointAdded[i]) < endpointKey(endpointAdded[j]) })
	sort.Slice(endpointRemoved, func(i, j int) bool { return endpointKey(endpointRemoved[i]) < endpointKey(endpointRemoved[j]) })

	return map[string]any{
		"entities": map[string]any{
			"added":          entityAdded,
			"removed":        entityRemoved,
			"fields_added":   fieldAdded,
			"fields_removed": fieldRemoved,
		},
		"services": map[string]any{
			"added":           serviceAdded,
			"removed":         serviceRemoved,
			"methods_added":   methodAdded,
			"methods_removed": methodRemoved,
		},
		"endpoints": map[string]any{
			"added":   endpointAdded,
			"removed": endpointRemoved,
		},
	}
}

func toStringSlice(v any) []string {
	switch vv := v.(type) {
	case []string:
		return vv
	case []any:
		out := make([]string, 0, len(vv))
		for _, it := range vv {
			s, _ := it.(string)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func endpointKey(ep map[string]string) string {
	return strings.ToUpper(ep["method"]) + " " + ep["path"] + " " + ep["rpc"]
}
