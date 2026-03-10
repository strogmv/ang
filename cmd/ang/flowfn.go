package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/strogmv/ang/compiler/flowfn"
)

type flowfnOutputStep struct {
	Action string         `json:"action"`
	Args   map[string]any `json:"args,omitempty"`
}

func runFlowfn(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: ang flowfn transpile [--format cue-array|json] [file]")
		os.Exit(1)
	}
	switch args[0] {
	case "transpile":
		runFlowfnTranspile(args[1:])
	default:
		fmt.Printf("Unknown flowfn command: %s\n", args[0])
		fmt.Println("Usage: ang flowfn transpile [--format cue-array|json] [file]")
		os.Exit(1)
	}
}

func runFlowfnTranspile(args []string) {
	fs := flag.NewFlagSet("flowfn transpile", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	format := fs.String("format", "cue-array", "output format: cue-array|json")
	if err := fs.Parse(args); err != nil {
		fmt.Printf("flowfn transpile FAILED: %v\n", err)
		os.Exit(1)
	}

	src, err := readFlowfnInput(fs.Args())
	if err != nil {
		fmt.Printf("flowfn transpile FAILED: %v\n", err)
		os.Exit(1)
	}

	steps, err := flowfn.ParseTranspile(src)
	if err != nil {
		fmt.Printf("flowfn transpile FAILED: %v\n", err)
		os.Exit(1)
	}

	rendered, err := renderFlowfnOutput(*format, steps)
	if err != nil {
		fmt.Printf("flowfn transpile FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(rendered)
}

func readFlowfnInput(args []string) (string, error) {
	if len(args) > 0 {
		data, err := os.ReadFile(args[0])
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func renderFlowfnOutput(format string, steps []flowfn.Step) (string, error) {
	items := make([]flowfnOutputStep, 0, len(steps))
	for _, step := range steps {
		args := make(map[string]any, len(step.Args)+len(step.Children))
		for k, v := range step.Args {
			args[k] = v
		}
		for k, child := range step.Children {
			args[k] = convertFlowfnOutputSteps(child)
		}
		items = append(items, flowfnOutputStep{
			Action: step.Action,
			Args:   args,
		})
	}

	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "cue-array", "json":
		data, err := json.MarshalIndent(items, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data), nil
	default:
		return "", fmt.Errorf("unsupported format %q", format)
	}
}

func convertFlowfnOutputSteps(steps []flowfn.Step) []flowfnOutputStep {
	items := make([]flowfnOutputStep, 0, len(steps))
	for _, step := range steps {
		args := make(map[string]any, len(step.Args)+len(step.Children))
		for k, v := range step.Args {
			args[k] = v
		}
		for k, child := range step.Children {
			args[k] = convertFlowfnOutputSteps(child)
		}
		items = append(items, flowfnOutputStep{
			Action: step.Action,
			Args:   args,
		})
	}
	return items
}
