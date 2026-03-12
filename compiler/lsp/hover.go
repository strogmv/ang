package lsp

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang-ir/flowsem"
)

func HoverForSource(text string, pos Position) (*Hover, bool) {
	word, rng := wordRangeAt(text, pos)
	if strings.TrimSpace(word) == "" {
		return nil, false
	}
	entry, ok := catalogEntry(word)
	if !ok {
		return nil, false
	}
	var parts []string
	parts = append(parts, "```text")
	parts = append(parts, entry.Name)
	parts = append(parts, "```")
	if entry.Description != "" {
		parts = append(parts, entry.Description)
	}
	if entry.Effect != "" {
		parts = append(parts, fmt.Sprintf("Effect: `%s`", entry.Effect))
	}
	if len(entry.RequiresTags) > 0 {
		parts = append(parts, "Requires: `"+strings.Join(entry.RequiresTags, "`, `")+"`")
	}
	if entry.RequiresTx {
		parts = append(parts, "Requires transaction: `true`")
	}
	if !entry.TxCompatible {
		parts = append(parts, "Allowed in `tx.Block`: `false`")
	}
	if len(entry.Args) > 0 {
		args := make([]string, 0, len(entry.Args))
		for _, arg := range entry.Args {
			if arg.Required {
				args = append(args, fmt.Sprintf("`%s`", arg.Name))
			}
		}
		if len(args) > 0 {
			parts = append(parts, "Required args: "+strings.Join(args, ", "))
		}
	}
	if len(entry.NestedKeys) > 0 {
		parts = append(parts, "Nested blocks: `"+strings.Join(entry.NestedKeys, "`, `")+"`")
	}
	if entry.Example != "" {
		parts = append(parts, "Example:")
		parts = append(parts, "```cue")
		parts = append(parts, entry.Example)
		parts = append(parts, "```")
	}
	return &Hover{
		Value: strings.Join(parts, "\n\n"),
		Range: rng,
	}, true
}

func catalogEntry(name string) (flowsem.ActionCatalogEntry, bool) {
	for _, entry := range flowsem.ActionCatalog() {
		if entry.Name == name {
			return entry, true
		}
	}
	return flowsem.ActionCatalogEntry{}, false
}
