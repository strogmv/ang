package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/strogmv/ang-ir/flowsem"
)

type actionCatalogEnvelope struct {
	Schema  string                       `json:"schema"`
	Actions []flowsem.ActionCatalogEntry `json:"actions"`
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

	catalog := flowsem.ActionCatalog()
	if *asCUE {
		fmt.Print(renderActionCatalogCUE(catalog))
		return
	}

	env := actionCatalogEnvelope{
		Schema:  "ang/actions/v1",
		Actions: catalog,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(env); err != nil {
		fmt.Printf("Actions FAILED: %v\n", err)
		os.Exit(1)
	}
}

func renderActionCatalogCUE(entries []flowsem.ActionCatalogEntry) string {
	var b strings.Builder
	b.WriteString("package actions\n\n")
	b.WriteString("#Catalog: {\n")
	b.WriteString("\tschema: \"ang/actions/v1\"\n")
	b.WriteString("\tactions: [\n")
	for _, e := range entries {
		b.WriteString("\t\t{\n")
		b.WriteString("\t\t\tname: " + strconv.Quote(e.Name) + "\n")
		b.WriteString("\t\t\tdescription: " + strconv.Quote(e.Description) + "\n")
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
