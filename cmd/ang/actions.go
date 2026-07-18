package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/strogmv/ang-ir/flowsem"
	"github.com/strogmv/ang/compiler/flowir"
)

type actionCatalogEnvelope struct {
	Schema  string                  `json:"schema"`
	Actions []documentedActionEntry `json:"actions"`
}

// documentedActionEntry combines semantic documentation from flowsem with
// the target-neutral renderer routing published by the Typed Flow IR registry.
// The embedded entry keeps the existing ang/actions/v1 fields stable.
type documentedActionEntry struct {
	flowsem.ActionCatalogEntry
	RendererGroup flowir.RendererGroup `json:"renderer_group"`
}

func runActions(args []string) {
	fs := flag.NewFlagSet("actions", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	asJSON := fs.Bool("json", false, "print catalog as JSON")
	asCUE := fs.Bool("cue", false, "print catalog as CUE")
	if err := fs.Parse(args); err != nil {
		fmt.Printf("Actions FAILED: %v\n", err)
		os.Exit(1)
	}
	if *asJSON && *asCUE {
		fmt.Println("Actions FAILED: use only one of --json or --cue")
		os.Exit(1)
	}

	baseCatalog := flowsem.ActionCatalog()
	if err := flowir.ValidateSchemaContract(baseCatalog); err != nil {
		fmt.Printf("Actions FAILED: %v\n", err)
		return
	}
	catalog := mergedActionCatalogFrom(baseCatalog)
	documented := documentedActionCatalog(catalog)
	if *asCUE {
		fmt.Print(renderActionCatalogCUE(documented))
		return
	}

	env := actionCatalogEnvelope{
		Schema:  "ang/actions/v1",
		Actions: documented,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(env); err != nil {
		fmt.Printf("Actions FAILED: %v\n", err)
		os.Exit(1)
	}
}

func mergedActionCatalog() []flowsem.ActionCatalogEntry {
	return mergedActionCatalogFrom(flowsem.ActionCatalog())
}

func mergedActionCatalogFrom(catalog []flowsem.ActionCatalogEntry) []flowsem.ActionCatalogEntry {
	index := make(map[string]int, len(catalog))
	for i := range catalog {
		index[catalog[i].Name] = i
	}
	for _, typed := range flowir.All() {
		args := make([]flowsem.ActionArg, 0, len(typed.Args))
		for _, arg := range typed.Args {
			args = append(args, flowsem.ActionArg{Name: arg.Name, Type: string(arg.Kind), Required: arg.Required})
		}
		if position, ok := index[typed.Name]; ok {
			catalog[position].Description = typed.Description
			catalog[position].Args = args
			catalog[position].KnownBy = "typed-flowir"
			continue
		}
		catalog = append(catalog, flowsem.ActionCatalogEntry{Name: typed.Name, Description: typed.Description, Args: args, KnownBy: "typed-flowir"})
	}
	sort.Slice(catalog, func(i, j int) bool { return catalog[i].Name < catalog[j].Name })
	return catalog
}

func documentedActionCatalog(catalog []flowsem.ActionCatalogEntry) []documentedActionEntry {
	groups := make(map[string]flowir.RendererGroup, len(flowir.All()))
	for _, spec := range flowir.All() {
		groups[spec.Name] = spec.RendererGroup
	}
	out := make([]documentedActionEntry, 0, len(catalog))
	for _, entry := range catalog {
		out = append(out, documentedActionEntry{
			ActionCatalogEntry: entry,
			RendererGroup:      groups[entry.Name],
		})
	}
	return out
}

func renderActionCatalogCUE(entries []documentedActionEntry) string {
	var b strings.Builder
	b.WriteString("package actions\n\n")
	b.WriteString("#Catalog: {\n")
	b.WriteString("\tschema: \"ang/actions/v1\"\n")
	b.WriteString("\tactions: [\n")
	for _, e := range entries {
		b.WriteString("\t\t{\n")
		b.WriteString("\t\t\tname: " + strconv.Quote(e.Name) + "\n")
		b.WriteString("\t\t\tdescription: " + strconv.Quote(e.Description) + "\n")
		b.WriteString("\t\t\trenderer_group: " + strconv.Quote(string(e.RendererGroup)) + "\n")
		b.WriteString("\t\t\targs: [\n")
		for _, a := range e.Args {
			b.WriteString("\t\t\t\t{")
			b.WriteString("name: " + strconv.Quote(a.Name))
			b.WriteString(", type: " + strconv.Quote(a.Type))
			if a.Required {
				b.WriteString(", required: true")
			} else {
				b.WriteString(", required: false")
			}
			b.WriteString("}\n")
		}
		b.WriteString("\t\t\t]\n")
		b.WriteString("\t\t\toutputs: " + renderStringListCUE(e.Outputs) + "\n")
		b.WriteString("\t\t\terrors: " + renderStringListCUE(e.Errors) + "\n")
		b.WriteString("\t\t\tnested_keys: " + renderStringListCUE(e.NestedKeys) + "\n")
		b.WriteString("\t\t\texample: " + strconv.Quote(e.Example) + "\n")
		b.WriteString("\t\t}\n")
	}
	b.WriteString("\t]\n")
	b.WriteString("}\n")
	return b.String()
}

func renderStringListCUE(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteString("[")
	for i, item := range items {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.Quote(item))
	}
	b.WriteString("]")
	return b.String()
}
