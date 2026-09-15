package compiler

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
)

// shared_arch lifts the "entity of another bounded context" check from flow
// steps (ang-ir normalizer.validateFlowSteps). A mark is therefore needed
// whenever a foreign context reaches the entity — even a single one — and is
// unneeded only when every access comes from the entity's own context.
//
// Accesses are counted from flow steps (repo.*, db.*, list.Enrich) and from Go
// written in CUE (func and code strings, impl code): s.<Entity>Repo.<Method>.
// The boundary check itself only sees flow steps, but a repository call in Go
// crosses the same boundary.

var goRepoAccess = regexp.MustCompile(`\bs\.([A-Z][A-Za-z0-9_]*)Repo\.`)

// SharedArchEntry describes one entity marked shared_arch: the architecture
// debt `ang vet` lists.
type SharedArchEntry struct {
	Entity           string
	Context          string
	Reason           string
	Ticket           string
	File             string
	Line             int
	ForeignFlowUsers []string
	ForeignGoUsers   []string
}

func emitSharedArchDiagnostics(entities []normalizer.Entity, services []normalizer.Service, opts PipelineOptions) {
	for _, entry := range SharedArchRegister(entities, services) {
		if entry.Reason == "" {
			recordPipelineDiagnostic(normalizer.Warning{
				Kind:     "architecture",
				Code:     "SHARED_ARCH_REASON_REQUIRED",
				Severity: "error",
				Message:  fmt.Sprintf("Entity '%s' uses shared_arch but has no reason", entry.Entity),
				File:     entry.File,
				Line:     entry.Line,
				Hint:     `Add explicit rationale: @shared_arch(reason="...") or shared_arch_reason: "..."`,
			}, opts)
		}
		if entry.Context != "" && len(entry.ForeignFlowUsers) == 0 && len(entry.ForeignGoUsers) == 0 {
			recordPipelineDiagnostic(normalizer.Warning{
				Kind:     "architecture",
				Code:     "SHARED_ARCH_UNDERUSED",
				Severity: "warn",
				Message:  fmt.Sprintf("Entity '%s' is shared_arch but every access comes from its own context '%s'", entry.Entity, entry.Context),
				File:     entry.File,
				Line:     entry.Line,
				Hint:     "Remove shared_arch: no flow step or Go block reaches this entity from another bounded context.",
			}, opts)
		}
	}
}

// SharedArchRegister lists every shared_arch entity with the foreign contexts
// that reach it, split by flow steps and Go blocks. admin and audit are left
// out, as the boundary check leaves them out.
func SharedArchRegister(entities []normalizer.Entity, services []normalizer.Service) []SharedArchEntry {
	flowUsers, goUsers := collectEntityServiceAccess(services)
	var out []SharedArchEntry
	for _, ent := range entities {
		if !isSharedArchEntity(ent) {
			continue
		}
		file, line := parseSourcePos(ent.Source)
		ctx := entityBoundedContext(ent)
		out = append(out, SharedArchEntry{
			Entity:           ent.Name,
			Context:          ctx,
			Reason:           strings.TrimSpace(toMetadataString(ent.Metadata["shared_arch_reason"])),
			Ticket:           strings.TrimSpace(toMetadataString(ent.Metadata["shared_arch_ticket"])),
			File:             file,
			Line:             line,
			ForeignFlowUsers: foreignContexts(flowUsers[ent.Name], ctx),
			ForeignGoUsers:   foreignContexts(goUsers[ent.Name], ctx),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Entity < out[j].Entity })
	return out
}

// collectEntityServiceAccess maps entity name → bounded contexts of the
// services that access it, once for flow steps and once for Go blocks.
func collectEntityServiceAccess(services []normalizer.Service) (flow, goBlocks map[string]map[string]struct{}) {
	flow = map[string]map[string]struct{}{}
	goBlocks = map[string]map[string]struct{}{}
	add := func(target map[string]map[string]struct{}, entity, ctx string) {
		if target[entity] == nil {
			target[entity] = map[string]struct{}{}
		}
		target[entity][ctx] = struct{}{}
	}
	addGo := func(code, ctx string) {
		for _, match := range goRepoAccess.FindAllStringSubmatch(code, -1) {
			add(goBlocks, match[1], ctx)
		}
	}
	for _, svc := range services {
		ctx := serviceBoundedContext(svc.Name)
		if ctx == "admin" || ctx == "audit" {
			continue
		}
		for _, method := range svc.Methods {
			walkFlowSteps(method.Flow, func(step normalizer.FlowStep) {
				if entity := strings.TrimSpace(stepEntityAccess(step)); entity != "" {
					add(flow, entity, ctx)
				}
				for _, key := range []string{"func", "code"} {
					if code, ok := step.Args[key].(string); ok {
						addGo(code, ctx)
					}
				}
			})
			if method.Impl != nil {
				addGo(method.Impl.Code, ctx)
			}
		}
	}
	return flow, goBlocks
}

func foreignContexts(users map[string]struct{}, own string) []string {
	var out []string
	for ctx := range users {
		if ctx != own {
			out = append(out, ctx)
		}
	}
	sort.Strings(out)
	return out
}

func walkFlowSteps(steps []normalizer.FlowStep, fn func(step normalizer.FlowStep)) {
	for _, step := range steps {
		fn(step)
		for _, key := range []string{"_do", "_then", "_else", "_ifNew", "_ifExists", "_default", "_catch", "_fallback", "_onTimeout", "_onMissing", "_onMismatch"} {
			if nested, ok := step.Args[key].([]normalizer.FlowStep); ok && len(nested) > 0 {
				walkFlowSteps(nested, fn)
			}
		}
		if cases, ok := step.Args["_cases"].(map[string][]normalizer.FlowStep); ok && len(cases) > 0 {
			for _, nested := range cases {
				walkFlowSteps(nested, fn)
			}
		}
		if branches, ok := step.Args["_branches"].(map[string][]normalizer.FlowStep); ok && len(branches) > 0 {
			for _, nested := range branches {
				walkFlowSteps(nested, fn)
			}
		}
	}
}

func stepEntityAccess(step normalizer.FlowStep) string {
	switch {
	case strings.HasPrefix(step.Action, "repo.") || strings.HasPrefix(step.Action, "db."):
		if source, _ := step.Args["source"].(string); strings.TrimSpace(source) != "" {
			return source
		}
	case step.Action == "list.Enrich":
		if source, _ := step.Args["lookupSource"].(string); strings.TrimSpace(source) != "" {
			return source
		}
	}
	return ""
}

// entityBoundedContext and serviceBoundedContext follow ang-ir's
// inferBoundedContext, which the boundary check uses: the explicit
// bounded_context, else the owner or service name up to the first _, - or .
func entityBoundedContext(ent normalizer.Entity) string {
	if ctx := strings.TrimSpace(strings.ToLower(ent.BoundedContext)); ctx != "" {
		return ctx
	}
	return boundedContextPrefix(ent.Owner)
}

func serviceBoundedContext(service string) string {
	return boundedContextPrefix(service)
}

func boundedContextPrefix(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	for _, sep := range []string{"_", "-", "."} {
		if i := strings.Index(name, sep); i > 0 {
			return name[:i]
		}
	}
	return name
}

func isSharedArchEntity(ent normalizer.Entity) bool {
	if ent.Metadata == nil {
		return false
	}
	v, ok := ent.Metadata["shared_arch"].(bool)
	return ok && v
}

func toMetadataString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	default:
		return ""
	}
}

func sortedSetKeys(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
