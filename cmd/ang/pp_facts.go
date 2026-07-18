package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	ppfacts "github.com/strogmv/ang/compiler/paymentprovider/facts"
)

func runPPFacts(args []string) {
	fs := flag.NewFlagSet("pp facts", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	cueRoot := fs.String("cue-root", ".cue", "CUE root directory inside the provider project")
	schemaDir := fs.String("schema-dir", "", "Shared schema directory override")
	jsonOut := fs.Bool("json", false, "Emit canonical JSON (default behavior)")
	projectPath, flagArgs := splitPPProjectPath(args)
	if err := fs.Parse(flagArgs); err != nil {
		os.Exit(1)
	}
	_ = jsonOut

	env, err := ppfacts.Extract(ppfacts.ExtractOptions{
		ProjectPath: projectPath,
		CueRoot:     *cueRoot,
		SchemaDir:   *schemaDir,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "pp facts FAILED: %v\n", err)
		os.Exit(1)
	}
	data, err := json.Marshal(env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pp facts FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(data))
}
