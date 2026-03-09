package compiler

import (
	"fmt"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

// RenderContextMarkdown generates a compact domain snapshot for AI context loading.
// filterService: empty = all services; "tender" = only tender-owned entities/repos/methods.
// showAll: include HTTP endpoints listing.
func RenderContextMarkdown(
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
			// Add flow DSL usage hints for special types
			switch f.Type {
			case "time.Time":
				b.WriteString(" // compare: .IsZero(), .Before(); parse: time.Parse; assign via temp var")
			case "float64", "float32":
				b.WriteString(" // convert: convert.ToFloat; arithmetic: math.Expr")
			case "int64", "int32", "int":
				b.WriteString(" // convert: convert.ToInt; arithmetic: math.Expr")
			}
			b.WriteString("\n")
		}
		if e.FSM != nil && len(e.FSM.States) > 0 {
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
			b.WriteString(RenderFinderSignature(r.Entity, f))
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
		if filterService != "" && !showAll {
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
			if !ownerMatch {
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

// RenderFinderSignature builds a human-readable repo finder method signature.
func RenderFinderSignature(entity string, f normalizer.RepositoryFinder) string {
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
