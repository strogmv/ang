package flowsem

import (
	"fmt"
	"sort"
	"strings"

	sharedeffects "github.com/strogmv/ang/compiler/effects"
)

// ActionArg describes one action argument for machine-readable catalogs.
type ActionArg struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

// ActionCatalogEntry is a normalized schema row for one flow action.
type ActionCatalogEntry struct {
	Name         string      `json:"name"`
	Description  string      `json:"description"`
	Args         []ActionArg `json:"args"`
	Outputs      []string    `json:"outputs"`
	Errors       []string    `json:"errors"`
	NestedKeys   []string    `json:"nested_keys"`
	Example      string      `json:"example"`
	Effect       string      `json:"effect"`
	RequiresTags []string    `json:"requires_tags,omitempty"`
	ProducesTags []string    `json:"produces_tags,omitempty"`
	RequiresVars []string    `json:"requires_vars,omitempty"`
	ProducesVar  string      `json:"produces_var,omitempty"`
	ChildTags    []string    `json:"child_tags,omitempty"`
	TxCompatible bool        `json:"tx_compatible"`
	RequiresTx   bool        `json:"requires_tx,omitempty"`
	KnownBy      string      `json:"known_by"`
}

// ActionCatalog builds a deterministic machine-readable catalog from flowsem specs.
func ActionCatalog() []ActionCatalogEntry {
	names := make([]string, 0, len(specs))
	for name := range specs {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]ActionCatalogEntry, 0, len(names))
	for _, name := range names {
		spec := specs[name]
		entry := ActionCatalogEntry{
			Name:        name,
			Description: defaultActionDescription(name),
			Args:        buildCatalogArgs(spec),
			Outputs:     sortedStringsCopy(spec.DeclaresFromArgs),
			Errors:      buildCatalogErrors(spec),
			NestedKeys:  buildCatalogNestedKeys(name, spec),
			Example:     buildCatalogExample(name, spec),
		}
		applyCatalogSemantics(&entry, name, spec)
		out = append(out, entry)
	}
	return out
}

func buildCatalogArgs(spec Spec) []ActionArg {
	args := make([]ActionArg, 0, len(spec.RequiredArgs)+len(spec.OptionalArgKinds))
	for _, key := range sortedStringsCopy(spec.RequiredArgs) {
		args = append(args, ActionArg{
			Name:     key,
			Type:     "any",
			Required: true,
		})
	}
	if len(spec.OptionalArgKinds) > 0 {
		keys := make([]string, 0, len(spec.OptionalArgKinds))
		for key := range spec.OptionalArgKinds {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			args = append(args, ActionArg{
				Name:     key,
				Type:     string(spec.OptionalArgKinds[key]),
				Required: false,
			})
		}
	}
	return args
}

func buildCatalogErrors(spec Spec) []string {
	set := map[string]struct{}{}
	for _, req := range spec.RequiredArgs {
		set["E_FLOW_MISSING_"+strings.ToUpper(req)] = struct{}{}
	}
	for _, child := range spec.RequiredChildren {
		set["E_FLOW_MISSING_"+strings.ToUpper(strings.TrimPrefix(child, "_"))] = struct{}{}
	}
	if spec.RequiresTx {
		set["E_FLOW_TX_REQUIRED"] = struct{}{}
	}
	if spec.CustomConstraints != nil {
		set["E_FLOW_CONSTRAINT"] = struct{}{}
	}
	return mapKeysSorted(set)
}

func buildCatalogNestedKeys(name string, spec Spec) []string {
	set := map[string]struct{}{}
	for _, child := range spec.RequiredChildren {
		set[child] = struct{}{}
	}
	for _, child := range actionOptionalChildren[name] {
		set[child] = struct{}{}
	}
	return mapKeysSorted(set)
}

func buildCatalogExample(name string, spec Spec) string {
	parts := []string{fmt.Sprintf(`action: %q`, name)}
	for _, req := range sortedStringsCopy(spec.RequiredArgs) {
		parts = append(parts, fmt.Sprintf(`%s: "<expr>"`, req))
	}
	for _, child := range sortedStringsCopy(spec.RequiredChildren) {
		plain := strings.TrimPrefix(child, "_")
		parts = append(parts, fmt.Sprintf(`%s: [{action: "logic.Check", condition: "true", throw: "ok"}]`, plain))
	}
	return "{ " + strings.Join(parts, ", ") + " }"
}

func defaultActionDescription(name string) string {
	return "Flow action contract generated from compiler semantics."
}

func applyCatalogSemantics(entry *ActionCatalogEntry, name string, spec Spec) {
	if entry == nil {
		return
	}
	logos, ok := LookupLogos(name)
	if !ok {
		entry.Effect = string(sharedeffects.EffectPure)
		entry.TxCompatible = !spec.RequiresTx
		entry.RequiresTx = spec.RequiresTx
		entry.KnownBy = "missing"
		return
	}
	entry.Effect = string(logos.Effect)
	entry.RequiresTags = safetyTagsToStrings(logos.RequiresTags)
	entry.ProducesTags = safetyTagsToStrings(logos.ProducesTags)
	entry.RequiresVars = sortedStringsCopy(logos.RequiresVars)
	entry.ProducesVar = strings.TrimSpace(logos.ProducesVar)
	entry.ChildTags = safetyTagsToStrings(logos.ChildTags)
	entry.TxCompatible = logos.TxCompatible
	entry.RequiresTx = spec.RequiresTx || logos.RequiresTx
	if _, explicit := sharedeffects.Registry[name]; explicit {
		entry.KnownBy = "explicit"
	} else {
		entry.KnownBy = "prefix"
	}
}

func safetyTagsToStrings(tags []sharedeffects.SafetyTag) []string {
	if len(tags) == 0 {
		return nil
	}
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		out = append(out, string(tag))
	}
	sort.Strings(out)
	return out
}

func sortedStringsCopy(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func mapKeysSorted[K ~string, V any](m map[K]V) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)
	return keys
}

var actionOptionalChildren = map[string][]string{
	"flow.If":         {"_else"},
	"flow.Switch":     {"_default", "_cases"},
	"repo.Upsert":     {"_ifNew", "_ifExists"},
	"flow.Try":        {"_catch"},
	"flow.Timeout":    {"_onTimeout"},
	"flow.Resume":     {"_onMissing"},
	"flow.Replay":     {"_do", "_onMismatch"},
	"flow.Fallback":   {"_fallback"},
	"flow.Saga":       {"_do"},
	"flow.Compensate": {"_do"},
	"event.Subscribe": {"_do"},
	"parallel.Run":    {"_branches"},
	"flow.Parallel":   {"_branches"},
	"flow.Join":       {"_branches"},
	"flow.Race":       {"_branches"},
	"batch.Run":       {"_do"},
	"concurrency.Run": {"_do"},
	"bulkhead.Run":    {"_do"},
	"circuit.Breaker": {"_do"},
	"trace.Span":      {"_do"},
	"slo.Budget":      {"_do"},
	"approval.Wait":   {"_onTimeout"},
}
