package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func runStatus(args []string) {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	projectPath := fs.String("project-path", ".", "project root path")
	baseURL := fs.String("base-url", "", "base API url (default: http://localhost:$HTTP_PORT)")
	timeout := fs.Duration("timeout", 2*time.Second, "health probe timeout")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return
		}
		printCommandFailure("Status", err.Error(), "run `ang status --help`")
		os.Exit(1)
	}

	root := filepath.Clean(strings.TrimSpace(*projectPath))
	if root == "" {
		root = "."
	}

	checks, err := collectStartupChecks(root, true)
	if err != nil {
		printCommandFailure("Status", err.Error(), "")
		os.Exit(1)
	}
	okCnt, warnCnt, failCnt := summarizeStartupChecks(checks)

	target := resolveStatusBaseURL(root, *baseURL)
	healthStatus, readyStatus := probeHealthEndpoints(target, *timeout)
	composeStatus := composeRuntimeStatus(root)

	fmt.Println("ANG status")
	fmt.Printf("  Project: %s\n", filepath.ToSlash(filepath.Clean(root)))
	fmt.Printf("  Base URL: %s\n", target)
	fmt.Printf("  Health: %s\n", healthStatus)
	fmt.Printf("  Ready: %s\n", readyStatus)
	fmt.Printf("  Compose: %s\n", composeStatus)
	fmt.Printf("  Checks: ok=%d warn=%d fail=%d\n", okCnt, warnCnt, failCnt)

	if failCnt > 0 {
		fmt.Println("  Action: run `ang doctor start` and then `ang up`.")
		return
	}
	if strings.HasPrefix(healthStatus, "DOWN") || strings.HasPrefix(readyStatus, "DOWN") {
		fmt.Println("  Action: run `ang up` (or inspect app logs) and retry `ang status`.")
		return
	}
	fmt.Println("  Action: environment looks healthy.")
}

func summarizeStartupChecks(checks []startupCheck) (okCnt, warnCnt, failCnt int) {
	for _, c := range checks {
		switch c.Status {
		case startupOK:
			okCnt++
		case startupWarn:
			warnCnt++
		case startupFail:
			failCnt++
		}
	}
	return okCnt, warnCnt, failCnt
}

func resolveStatusBaseURL(projectPath, explicit string) string {
	target := strings.TrimSpace(explicit)
	if target == "" {
		target = "http://localhost:" + resolveHTTPPort(projectPath)
	}
	return strings.TrimRight(target, "/")
}

func probeHealthEndpoints(baseURL string, timeout time.Duration) (healthStatus, readyStatus string) {
	client := &http.Client{Timeout: timeout}
	healthStatus = probeEndpoint(client, baseURL+"/health")
	readyStatus = probeEndpoint(client, baseURL+"/health/ready")
	return healthStatus, readyStatus
}

func probeEndpoint(client *http.Client, url string) string {
	resp, err := client.Get(url)
	if err != nil {
		return "DOWN (" + err.Error() + ")"
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("DOWN (status=%d)", resp.StatusCode)
	}
	return "OK"
}

func composeRuntimeStatus(projectPath string) string {
	composePath := filepath.Join(projectPath, "docker-compose.yml")
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		return "not configured (docker-compose.yml not found)"
	} else if err != nil {
		return "unknown (" + err.Error() + ")"
	}

	composeCmd, err := detectComposeCommand()
	if err != nil {
		return "unavailable (" + err.Error() + ")"
	}
	cmdArgs := append([]string{}, composeCmd[1:]...)
	cmdArgs = append(cmdArgs, "-f", composePath, "ps", "--services", "--status", "running")
	cmd := exec.Command(composeCmd[0], cmdArgs...)
	cmd.Dir = projectPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "unknown (" + strings.TrimSpace(string(out)) + ")"
	}
	services := nonEmptyLines(string(out))
	if len(services) == 0 {
		return "no running services"
	}
	return "running: " + strings.Join(services, ", ")
}

func nonEmptyLines(v string) []string {
	var out []string
	for _, line := range strings.Split(v, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}
