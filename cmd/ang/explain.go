package main

import (
	"fmt"
	"os"
	"strings"
)

type explainEntry struct {
	Code        string
	Title       string
	Description string
	Example     string
}

func runExplain(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: ang explain <CODE>")
		os.Exit(1)
	}
	code := strings.ToUpper(strings.TrimSpace(args[0]))
	explanations := map[string]explainEntry{
		"MISSING_ID": {
			Code:        "MISSING_ID",
			Title:       "Missing ID assignment before repo.Save",
			Description: "When creating a new entity, an ID must be set before saving. ANG expects an explicit mapping.Assign to ensure deterministic identifiers.",
			Example:     "{action: \"mapping.Assign\", to: \"newItem.ID\", value: \"uuid.NewString()\"}",
		},
		"MISSING_CREATED_AT": {
			Code:        "MISSING_CREATED_AT",
			Title:       "Missing CreatedAt assignment before repo.Save",
			Description: "New entities should set CreatedAt to RFC3339 in UTC so storage is consistent and sortable.",
			Example:     "{action: \"mapping.Assign\", to: \"newItem.CreatedAt\", value: \"time.Now().UTC().Format(time.RFC3339)\"}",
		},
		"MISSING_OUTPUT": {
			Code:        "MISSING_OUTPUT",
			Title:       "Missing output variable in repo.Find/repo.List",
			Description: "Repo operations must declare an output variable to store the fetched entity or list.",
			Example:     "{action: \"repo.Find\", source: \"Item\", input: \"req.ItemID\", output: \"item\"}",
		},
		"MISSING_INPUT": {
			Code:        "MISSING_INPUT",
			Title:       "Missing input in repo or mapping step",
			Description: "Repo operations need an input (ID or filter) to identify what to fetch or save.",
			Example:     "{action: \"repo.Find\", source: \"Item\", input: \"req.ItemID\", output: \"item\"}",
		},
		"MISSING_SOURCE": {
			Code:        "MISSING_SOURCE",
			Title:       "Missing source entity in repo step",
			Description: "Repo operations must specify the source entity to select the correct repository.",
			Example:     "{action: \"repo.Save\", source: \"Item\", input: \"item\"}",
		},
		"MISSING_ENTITY": {
			Code:        "MISSING_ENTITY",
			Title:       "Missing entity in mapping.Map",
			Description: "mapping.Map requires an entity when creating new domain objects to infer the correct type.",
			Example:     "{action: \"mapping.Map\", output: \"newItem\", entity: \"Item\"}",
		},
	}

	if entry, ok := explanations[code]; ok {
		fmt.Printf("%s\n%s\n\nExample:\n%s\n", entry.Title, entry.Description, entry.Example)
		return
	}

	fmt.Printf("Unknown code: %s\n", code)
	os.Exit(1)
}
