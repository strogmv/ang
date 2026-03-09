package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/normalizer"
)

// runContext generates a compact domain snapshot for AI context loading.
// Usage: ang context [service] [--format=md|yaml] [--all]
func runContext(args []string) {
	format := "md"
	filterService := ""
	showAll := false
	projectPath := "."

	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--format="):
			format = strings.TrimPrefix(a, "--format=")
		case strings.HasPrefix(a, "--project="):
			projectPath = strings.TrimPrefix(a, "--project=")
		case a == "--all":
			showAll = true
		case !strings.HasPrefix(a, "--"):
			filterService = strings.ToLower(a)
		}
	}

	entities, services, endpoints, repos, events, _, _, _, _, err := compiler.RunPipeline(projectPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ang context: failed to parse project: %v\n", err)
		os.Exit(1)
	}

	switch format {
	case "md", "markdown":
		fmt.Print(compiler.RenderContextMarkdown(filterService, showAll, entities, services, endpoints, repos, events))
	default:
		fmt.Fprintf(os.Stderr, "ang context: unknown format %q (supported: md)\n", format)
		os.Exit(1)
	}
}

func renderContextMarkdown_UNUSED(
	filterService string,
	showAll bool,
	entities []normalizer.Entity,
	services []normalizer.Service,
	endpoints []normalizer.Endpoint,
	repos []normalizer.Repository,
	events []normalizer.EventDef,
) string {
	var b strings.Builder

	b.WriteString("# ANG Domain Context\n\n")

	// --- Entities ---
	b.WriteString("## Entities\n\n")
	sort.Slice(entities, func(i, j int) bool { return entities[i].Name < entities[j].Name })
	for _, e := range entities {
		if filterService != "" && !strings.EqualFold(e.Owner, filterService) && !strings.EqualFold(e.BoundedContext, filterService) {
			continue
		}
		b.WriteString("### ")
		b.WriteString(e.Name)
		if e.Owner != "" {
			b.WriteString(" (owner: ")
			b.WriteString(e.Owner)
			b.WriteString(")")
		}
		b.WriteString("\n")
		for _, f := range e.Fields {
			opt := ""
			if f.IsOptional {
				opt = "?"
			}
			list := ""
			if f.IsList {
				list = "[]"
			}
			b.WriteString("- ")
			b.WriteString(f.Name)
			b.WriteString(opt)
			b.WriteString(": ")
			b.WriteString(list)
			b.WriteString(f.Type)
			if f.DB.Type != "" && showAll {
				b.WriteString(" @db(")
				b.WriteString(f.DB.Type)
				b.WriteString(")")
			}
			b.WriteString("\n")
		}
		if e.FSM != nil {
			b.WriteString("- FSM field: ")
			b.WriteString(e.FSM.Field)
			b.WriteString(" [")
			b.WriteString(strings.Join(e.FSM.States, " | "))
			b.WriteString("]\n")
		}
		b.WriteString("\n")
	}

	// --- Repositories ---
	b.WriteString("## Repositories\n\n")
	sort.Slice(repos, func(i, j int) bool { return repos[i].Entity < repos[j].Entity })
	for _, r := range repos {
		if filterService != "" {
			// match repo to service by entity owner
			matched := false
			for _, e := range entities {
				if e.Name == r.Entity && (strings.EqualFold(e.Owner, filterService) || strings.EqualFold(e.BoundedContext, filterService)) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		if len(r.Finders) == 0 {
			continue
		}
		b.WriteString("### ")
		b.WriteString(r.Entity)
		b.WriteString("Repository\n")
		for _, f := range r.Finders {
			b.WriteString("- ")
			b.WriteString(renderFinderSignature(r.Entity, f))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// --- Services & Methods ---
	b.WriteString("## Services\n\n")
	sort.Slice(services, func(i, j int) bool { return services[i].Name < services[j].Name })
	for _, svc := range services {
		if filterService != "" && !strings.EqualFold(svc.Name, filterService) {
			continue
		}
		b.WriteString("### ")
		b.WriteString(svc.Name)
		b.WriteString("Service\n")
		for _, m := range svc.Methods {
			b.WriteString("- ")
			b.WriteString(m.Name)
			b.WriteString("(")
			b.WriteString(m.Input.Name)
			b.WriteString(") → ")
			b.WriteString(m.Output.Name)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// --- HTTP Endpoints ---
	if showAll || filterService != "" {
		b.WriteString("## Endpoints\n\n")
		for _, ep := range endpoints {
			if filterService != "" && !strings.EqualFold(ep.ServiceName, filterService) {
				continue
			}
			b.WriteString("- ")
			b.WriteString(ep.Method)
			b.WriteString(" ")
			b.WriteString(ep.Path)
			b.WriteString(" → ")
			b.WriteString(ep.RPC)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// --- Events ---
	b.WriteString("## Events\n\n")
	sort.Slice(events, func(i, j int) bool { return events[i].Name < events[j].Name })
	for _, ev := range events {
		if filterService != "" {
			ownerMatch := false
			for _, svc := range services {
				if strings.EqualFold(svc.Name, filterService) {
					for _, pub := range svc.Publishes {
						if strings.EqualFold(pub, ev.Name) {
							ownerMatch = true
						}
					}
				}
			}
			if !ownerMatch && !showAll {
				continue
			}
		}
		b.WriteString("### ")
		b.WriteString(ev.Name)
		b.WriteString(" (owner: ")
		b.WriteString(ev.Owner)
		b.WriteString(")\n")
		for _, f := range ev.Fields {
			b.WriteString("- ")
			b.WriteString(f.Name)
			b.WriteString(": ")
			b.WriteString(f.Type)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}

func renderFinderSignature(entity string, f normalizer.RepositoryFinder) string {
	// Build param list from Where clauses
	var params []string
	params = append(params, "ctx context.Context")
	for _, w := range f.Where {
		paramType := w.ParamType
		if paramType == "" {
			paramType = "string"
		}
		params = append(params, w.Param+" "+paramType)
	}

	ret := "*domain." + entity
	switch f.Returns {
	case "list":
		ret = "[]domain." + entity
	case "count":
		ret = "int64"
	case "bool":
		ret = "bool"
	case "sum":
		ret = "float64"
	}
	if f.ReturnType != "" {
		ret = f.ReturnType
	}

	return fmt.Sprintf("%s(%s) (%s, error)", f.Name, strings.Join(params, ", "), ret)
}
