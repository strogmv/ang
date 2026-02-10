package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/strogmv/ang/compiler/doctor"
)

func runDoctor(args []string) {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	logFile := fs.String("log-file", "ang-build.log", "path to build log file")
	inlineLog := fs.String("log", "", "inline log text to analyze")
	fromStdin := fs.Bool("stdin", false, "read log from stdin")
	if err := fs.Parse(args); err != nil {
		fmt.Printf("Doctor FAILED: %v\n", err)
		os.Exit(1)
	}

	logText := strings.TrimSpace(*inlineLog)
	if logText == "" {
		useStdin := *fromStdin || isPipedStdin()
		if useStdin {
			b, err := io.ReadAll(os.Stdin)
			if err != nil {
				fmt.Printf("Doctor FAILED: read stdin: %v\n", err)
				os.Exit(1)
			}
			logText = string(b)
		}
	}
	if strings.TrimSpace(logText) == "" {
		b, err := os.ReadFile(*logFile)
		if err != nil {
			fmt.Printf("Doctor FAILED: cannot read %s (%v)\n", *logFile, err)
			fmt.Println("Provide --log, --stdin, or run with an existing ang-build.log")
			os.Exit(1)
		}
		logText = string(b)
	}

	resp := doctor.NewAnalyzer(".").Analyze(logText)
	b, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		fmt.Printf("Doctor FAILED: marshal response: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(b))
}

func isPipedStdin() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) == 0
}
