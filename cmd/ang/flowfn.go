package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/strogmv/ang/compiler/flowfn"
	anglsp "github.com/strogmv/ang/compiler/lsp"
)

type flowfnOutputStep struct {
	Action string         `json:"action"`
	Args   map[string]any `json:"args,omitempty"`
}

type flowfnPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type flowfnRange struct {
	Start flowfnPosition `json:"start"`
	End   flowfnPosition `json:"end"`
}

type flowfnDiagnostic struct {
	Range    flowfnRange `json:"range"`
	Severity int         `json:"severity"`
	Code     string      `json:"code"`
	Message  string      `json:"message"`
	Source   string      `json:"source"`
}

type flowfnCompletionItem struct {
	Label      string `json:"label"`
	Detail     string `json:"detail"`
	InsertText string `json:"insertText"`
	Deprecated bool   `json:"deprecated"`
	SortText   string `json:"sortText"`
}

type flowfnHover struct {
	Value string       `json:"value"`
	Range *flowfnRange `json:"range,omitempty"`
}

func runFlowfn(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: ang flowfn transpile [--format cue-array|json] [file]")
		os.Exit(1)
	}
	switch args[0] {
	case "transpile":
		runFlowfnTranspile(args[1:])
	case "validate":
		runFlowfnValidate(args[1:])
	case "complete":
		runFlowfnComplete(args[1:])
	case "hover":
		runFlowfnHover(args[1:])
	default:
		fmt.Printf("Unknown flowfn command: %s\n", args[0])
		fmt.Println("Usage: ang flowfn transpile|validate|complete|hover ...")
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

func runFlowfnValidate(args []string) {
	fs := flag.NewFlagSet("flowfn validate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	streaming := fs.Bool("streaming", false, "validate as streaming method")
	if err := fs.Parse(args); err != nil {
		fmt.Printf("flowfn validate FAILED: %v\n", err)
		os.Exit(1)
	}
	src, err := readFlowfnInput(fs.Args())
	if err != nil {
		fmt.Printf("flowfn validate FAILED: %v\n", err)
		os.Exit(1)
	}
	diags := anglsp.FlowDiagnostics(src, *streaming)
	out := make([]flowfnDiagnostic, 0, len(diags))
	for _, d := range diags {
		out = append(out, flowfnDiagnostic{
			Range: flowfnRange{
				Start: flowfnPosition{Line: d.Range.Start.Line, Character: d.Range.Start.Character},
				End:   flowfnPosition{Line: d.Range.End.Line, Character: d.Range.End.Character},
			},
			Severity: d.Severity,
			Code:     d.Code,
			Message:  d.Message,
			Source:   d.Source,
		})
	}
	writeJSON(out)
}

func runFlowfnComplete(args []string) {
	fs := flag.NewFlagSet("flowfn complete", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	line := fs.Int("line", 0, "0-based line")
	character := fs.Int("character", 0, "0-based character")
	if err := fs.Parse(args); err != nil {
		fmt.Printf("flowfn complete FAILED: %v\n", err)
		os.Exit(1)
	}
	src, err := readFlowfnInput(fs.Args())
	if err != nil {
		fmt.Printf("flowfn complete FAILED: %v\n", err)
		os.Exit(1)
	}
	items := anglsp.CompletionItems(src, anglsp.Position{Line: *line, Character: *character})
	out := make([]flowfnCompletionItem, 0, len(items))
	for _, item := range items {
		out = append(out, flowfnCompletionItem{
			Label:      item.Label,
			Detail:     item.Detail,
			InsertText: item.InsertText,
			Deprecated: item.Deprecated,
			SortText:   item.SortText,
		})
	}
	writeJSON(out)
}

func runFlowfnHover(args []string) {
	fs := flag.NewFlagSet("flowfn hover", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	line := fs.Int("line", 0, "0-based line")
	character := fs.Int("character", 0, "0-based character")
	if err := fs.Parse(args); err != nil {
		fmt.Printf("flowfn hover FAILED: %v\n", err)
		os.Exit(1)
	}
	src, err := readFlowfnInput(fs.Args())
	if err != nil {
		fmt.Printf("flowfn hover FAILED: %v\n", err)
		os.Exit(1)
	}
	hover, ok := anglsp.HoverForSource(src, anglsp.Position{Line: *line, Character: *character})
	if !ok {
		writeJSON(struct{}{})
		return
	}
	var out flowfnHover
	out.Value = hover.Value
	if hover.Range != nil {
		out.Range = &flowfnRange{
			Start: flowfnPosition{Line: hover.Range.Start.Line, Character: hover.Range.Start.Character},
			End:   flowfnPosition{Line: hover.Range.End.Line, Character: hover.Range.End.Character},
		}
	}
	writeJSON(out)
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

func writeJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Printf("flowfn FAILED: %v\n", err)
		os.Exit(1)
	}
}
