package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/emitter"
)

func runOpenAPI(args []string) {
	fs := flag.NewFlagSet("openapi", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	out := fs.String("out", "", "output file (default: api/openapi.yaml)")
	stdout := fs.Bool("stdout", false, "print to stdout instead of writing to file")
	projectPath := fs.String("path", ".", "project root")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "openapi: %v\n", err)
		os.Exit(1)
	}
	if fs.NArg() > 0 {
		*projectPath = fs.Arg(0)
	}

	semantic, err := compiler.RunSemanticPhases(*projectPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "openapi: parse error: %v\n", err)
		os.Exit(1)
	}

	project := &semantic.Project

	outPath := strings.TrimSpace(*out)
	if outPath == "" && !*stdout {
		outPath = filepath.Join(*projectPath, "api", "openapi.yaml")
	}

	if *stdout {
		outPath = filepath.Join(os.TempDir(), "ang-openapi-tmp.yaml")
	}

	em := &emitter.Emitter{
		OutputDir: filepath.Dir(outPath),
		Version:   compiler.Version,
	}

	if err := em.EmitOpenAPIFromNormalizerTypes(
		semantic.Endpoints,
		semantic.Services,
		semantic.Errors,
		project,
		outPath,
	); err != nil {
		fmt.Fprintf(os.Stderr, "openapi: %v\n", err)
		os.Exit(1)
	}

	if *stdout {
		data, err := os.ReadFile(outPath)
		if err == nil {
			fmt.Print(string(data))
			os.Remove(outPath)
		}
	}
}
