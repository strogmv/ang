package emitter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/ir"
)

func OrderIRServicesByDependencies(services []ir.Service) []ir.Service {
	if len(services) == 0 {
		return services
	}
	byName := make(map[string]ir.Service, len(services))
	inDegree := make(map[string]int, len(services))
	graph := make(map[string][]string, len(services))
	for _, svc := range services {
		byName[svc.Name] = svc
		inDegree[svc.Name] = 0
	}
	for _, svc := range services {
		for _, dep := range svc.Uses {
			if _, ok := byName[dep]; !ok {
				continue
			}
			graph[dep] = append(graph[dep], svc.Name)
			inDegree[svc.Name]++
		}
	}

	queue := make([]string, 0, len(services))
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}
	sort.Strings(queue)

	result := make([]ir.Service, 0, len(services))
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if svc, ok := byName[name]; ok {
			result = append(result, svc)
		}
		for _, next := range graph[name] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
				sort.Strings(queue)
			}
		}
	}

	if len(result) != len(services) {
		seen := make(map[string]bool, len(result))
		for _, svc := range result {
			seen[svc.Name] = true
		}
		for _, svc := range services {
			if !seen[svc.Name] {
				result = append(result, svc)
			}
		}
	}

	return result
}

func ValidateIRServiceDependencies(services []ir.Service) error {
	if len(services) == 0 {
		return nil
	}
	byName := make(map[string]ir.Service, len(services))
	inDegree := make(map[string]int, len(services))
	graph := make(map[string][]string, len(services))
	for _, svc := range services {
		byName[svc.Name] = svc
		inDegree[svc.Name] = 0
	}

	var missing []string
	for _, svc := range services {
		for _, dep := range svc.Uses {
			if _, ok := byName[dep]; !ok {
				missing = append(missing, svc.Name+" -> "+dep)
				continue
			}
			graph[dep] = append(graph[dep], svc.Name)
			inDegree[svc.Name]++
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("unknown service dependencies: %s", strings.Join(missing, ", "))
	}

	queue := make([]string, 0, len(services))
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		for _, next := range graph[name] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	var cycle []string
	for name, deg := range inDegree {
		if deg > 0 {
			cycle = append(cycle, name)
		}
	}
	if len(cycle) > 0 {
		sort.Strings(cycle)
		return fmt.Errorf("cycle detected among services: %s", strings.Join(cycle, ", "))
	}
	return nil
}

