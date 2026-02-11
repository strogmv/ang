package compiler

import (
	"fmt"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/ir"
)

// ValidateIRSemantics performs fail-fast semantic validation on IR before emitters run.
func ValidateIRSemantics(schema *ir.Schema) error {
	if schema == nil {
		return fmt.Errorf("ir schema is nil")
	}

	var errs []string
	entityByName := make(map[string]ir.Entity, len(schema.Entities))
	for _, ent := range schema.Entities {
		entityByName[ent.Name] = ent
	}
	serviceByName := make(map[string]ir.Service, len(schema.Services))
	for _, svc := range schema.Services {
		serviceByName[svc.Name] = svc
	}

	// 1) Entity type references and type map integrity.
	for _, ent := range schema.Entities {
		for _, f := range ent.Fields {
			validateTypeRef(&errs, entityByName, fmt.Sprintf("entity %s field %s", ent.Name, f.Name), f.Type)
		}
	}

	// 2) Service/method semantic references.
	for _, svc := range schema.Services {
		for _, m := range svc.Methods {
			if m.Input != nil {
				for _, f := range m.Input.Fields {
					validateTypeRef(&errs, entityByName, fmt.Sprintf("service %s method %s input field %s", svc.Name, m.Name, f.Name), f.Type)
				}
			}
			if m.Output != nil {
				for _, f := range m.Output.Fields {
					validateTypeRef(&errs, entityByName, fmt.Sprintf("service %s method %s output field %s", svc.Name, m.Name, f.Name), f.Type)
				}
			}
			for _, src := range m.Sources {
				if strings.TrimSpace(src.Entity) == "" {
					continue
				}
				if _, ok := entityByName[src.Entity]; !ok {
					errs = append(errs, fmt.Sprintf("service %s method %s source references unknown entity %q", svc.Name, m.Name, src.Entity))
				}
			}
		}
	}

	// 3) Repository/finder semantic references.
	for _, repo := range schema.Repos {
		ent, ok := entityByName[repo.Entity]
		if !ok {
			errs = append(errs, fmt.Sprintf("repository %s references unknown entity %q", repo.Name, repo.Entity))
			continue
		}
		fieldSet := make(map[string]bool, len(ent.Fields))
		for _, f := range ent.Fields {
			fieldSet[strings.ToLower(f.Name)] = true
		}
		for _, finder := range repo.Finders {
			for _, w := range finder.Where {
				if strings.TrimSpace(w.Field) == "" {
					continue
				}
				if !fieldSet[strings.ToLower(w.Field)] {
					errs = append(errs, fmt.Sprintf("repository %s finder %s where field %q does not exist on entity %s", repo.Name, finder.Name, w.Field, repo.Entity))
				}
			}
			for _, col := range finder.Select {
				if strings.TrimSpace(col) == "" {
					continue
				}
				if !fieldSet[strings.ToLower(col)] {
					errs = append(errs, fmt.Sprintf("repository %s finder %s select field %q does not exist on entity %s", repo.Name, finder.Name, col, repo.Entity))
				}
			}
		}
	}

	// 4) Endpoint RPC/service references.
	for _, ep := range schema.Endpoints {
		svc, ok := serviceByName[ep.Service]
		if !ok {
			errs = append(errs, fmt.Sprintf("endpoint %s %s references unknown service %q", ep.Method, ep.Path, ep.Service))
			continue
		}
		foundMethod := false
		for _, m := range svc.Methods {
			if m.Name == ep.RPC {
				foundMethod = true
				break
			}
		}
		if !foundMethod {
			errs = append(errs, fmt.Sprintf("endpoint %s %s references unknown RPC %q on service %s", ep.Method, ep.Path, ep.RPC, ep.Service))
		}
	}

	// 5) Cycle detection in service dependencies.
	if cycleErr := validateServiceDependencyCycles(schema.Services); cycleErr != nil {
		errs = append(errs, cycleErr.Error())
	}

	if len(errs) > 0 {
		sort.Strings(errs)
		return fmt.Errorf("ir semantic validation failed:\n - %s", strings.Join(errs, "\n - "))
	}
	return nil
}

func validateTypeRef(errs *[]string, entities map[string]ir.Entity, where string, ref ir.TypeRef) {
	switch ref.Kind {
	case ir.KindString, ir.KindInt, ir.KindInt64, ir.KindFloat, ir.KindBool, ir.KindTime, ir.KindUUID, ir.KindJSON, ir.KindEnum, ir.KindFile, ir.KindAny:
		return
	case ir.KindEntity:
		if strings.TrimSpace(ref.Name) == "" {
			*errs = append(*errs, fmt.Sprintf("%s has entity type without name", where))
			return
		}
		if _, ok := entities[ref.Name]; !ok {
			*errs = append(*errs, fmt.Sprintf("%s references unknown entity type %q", where, ref.Name))
		}
	case ir.KindList:
		if ref.ItemType == nil {
			*errs = append(*errs, fmt.Sprintf("%s list type has nil item type", where))
			return
		}
		validateTypeRef(errs, entities, where+"[]", *ref.ItemType)
	case ir.KindMap:
		if ref.KeyType == nil || ref.ItemType == nil {
			*errs = append(*errs, fmt.Sprintf("%s map type has nil key/item type", where))
			return
		}
		validateTypeRef(errs, entities, where+"<key>", *ref.KeyType)
		validateTypeRef(errs, entities, where+"<value>", *ref.ItemType)
	default:
		*errs = append(*errs, fmt.Sprintf("%s has unsupported type kind %q", where, ref.Kind))
	}
}

func validateServiceDependencyCycles(services []ir.Service) error {
	byName := make(map[string]ir.Service, len(services))
	for _, s := range services {
		byName[s.Name] = s
	}
	visiting := make(map[string]bool)
	visited := make(map[string]bool)
	var path []string

	var dfs func(string) error
	dfs = func(name string) error {
		if visiting[name] {
			idx := 0
			for i, p := range path {
				if p == name {
					idx = i
					break
				}
			}
			cycle := append(path[idx:], name)
			return fmt.Errorf("service dependency cycle detected: %s", strings.Join(cycle, " -> "))
		}
		if visited[name] {
			return nil
		}
		visited[name] = true
		visiting[name] = true
		path = append(path, name)

		svc, ok := byName[name]
		if ok {
			for _, dep := range svc.Uses {
				if _, exists := byName[dep]; !exists {
					continue
				}
				if err := dfs(dep); err != nil {
					return err
				}
			}
		}

		path = path[:len(path)-1]
		visiting[name] = false
		return nil
	}

	for _, s := range services {
		if err := dfs(s.Name); err != nil {
			return err
		}
	}
	return nil
}
