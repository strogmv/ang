package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type startupCheck struct {
	Name   string
	Status string
	Detail string
	Hint   string
}

const (
	startupOK   = "ok"
	startupWarn = "warn"
	startupFail = "fail"
)

func runDoctorStart(args []string) {
	fs := flag.NewFlagSet("doctor start", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	projectPath := fs.String("project-path", ".", "project root path")
	skipConfig := fs.Bool("skip-config", false, "skip config/.env checks")
	strict := fs.Bool("strict", true, "exit with non-zero code on failed checks")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return
		}
		fmt.Printf("Doctor start FAILED: %v\n", err)
		os.Exit(1)
	}

	checks, err := collectStartupChecks(*projectPath, !*skipConfig)
	if err != nil {
		fmt.Printf("Doctor start FAILED: %v\n", err)
		os.Exit(1)
	}

	hasFail := printStartupChecks(checks)
	if hasFail && *strict {
		fmt.Println("Doctor start FAILED")
		os.Exit(1)
	}
	if hasFail {
		fmt.Println("Doctor start completed with failures (strict=false)")
		return
	}
	fmt.Println("Doctor start OK")
}

func runUp(args []string) {
	fs := flag.NewFlagSet("up", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	projectPath := fs.String("project-path", ".", "project root path")
	composeFile := fs.String("compose-file", "docker-compose.yml", "docker compose file path")
	skipDoctor := fs.Bool("skip-doctor", false, "skip preflight doctor start checks")
	doctorStrict := fs.Bool("doctor-strict", true, "fail fast if preflight detects blocking issues")
	skipCompose := fs.Bool("skip-compose", false, "skip docker compose up")
	skipBuild := fs.Bool("skip-build", false, "skip ang build")
	skipSmoke := fs.Bool("skip-smoke", false, "skip health smoke check")
	detach := fs.Bool("detach", true, "run docker compose up in detached mode")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return
		}
		fmt.Printf("Up FAILED: %v\n", err)
		os.Exit(1)
	}

	root := filepath.Clean(strings.TrimSpace(*projectPath))
	if root == "" {
		root = "."
	}

	if !*skipDoctor {
		// Preflight is intentionally first: it surfaces missing env/tools before build noise.
		checks, err := collectStartupChecks(root, true)
		if err != nil {
			fmt.Printf("Up FAILED: %v\n", err)
			os.Exit(1)
		}
		hasFail := printStartupChecks(checks)
		if hasFail && *doctorStrict {
			fmt.Println("Up FAILED: preflight checks failed")
			os.Exit(1)
		}
	}

	if !*skipCompose {
		composePath := filepath.Join(root, strings.TrimSpace(*composeFile))
		if _, err := os.Stat(composePath); err == nil {
			// Detect compose command dynamically to support both modern plugin and legacy binary.
			composeCmd, err := detectComposeCommand()
			if err != nil {
				fmt.Printf("Up FAILED: %v\n", err)
				os.Exit(1)
			}
			cmdArgs := append([]string{}, composeCmd[1:]...)
			cmdArgs = append(cmdArgs, "-f", composePath, "up")
			if *detach {
				cmdArgs = append(cmdArgs, "-d")
			}
			fmt.Printf("==> Running: %s %s\n", composeCmd[0], strings.Join(cmdArgs, " "))
			if err := runCommand(root, composeCmd[0], cmdArgs...); err != nil {
				fmt.Printf("Up FAILED: compose up: %v\n", err)
				os.Exit(1)
			}
		} else if !os.IsNotExist(err) {
			fmt.Printf("Up FAILED: stat compose file: %v\n", err)
			os.Exit(1)
		} else {
			fmt.Printf("==> Skipping compose: %s not found\n", composePath)
		}
	}

	if !*skipBuild {
		fmt.Println("==> Running: ang build")
		if err := runSelf(root, "build"); err != nil {
			fmt.Printf("Up FAILED: build: %v\n", err)
			os.Exit(1)
		}
	}

	if !*skipSmoke {
		fmt.Println("==> Running: ang smoke")
		if err := runSelf(root, "smoke"); err != nil {
			fmt.Printf("Up FAILED: smoke: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println("Up SUCCESSFUL")
}

func runSmoke(args []string) {
	fs := flag.NewFlagSet("smoke", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	baseURL := fs.String("base-url", "", "base API url (default: http://localhost:$HTTP_PORT)")
	timeout := fs.Duration("timeout", 5*time.Second, "request timeout")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return
		}
		fmt.Printf("Smoke FAILED: %v\n", err)
		os.Exit(1)
	}

	target := strings.TrimSpace(*baseURL)
	if target == "" {
		port := strings.TrimSpace(os.Getenv("HTTP_PORT"))
		if port == "" {
			port = "8080"
		}
		target = "http://localhost:" + port
	}
	target = strings.TrimRight(target, "/")

	client := &http.Client{Timeout: *timeout}
	paths := []string{"/health", "/health/ready"}
	for _, p := range paths {
		url := target + p
		resp, err := client.Get(url)
		if err != nil {
			fmt.Printf("Smoke FAILED: GET %s: %v\n", url, err)
			os.Exit(1)
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			fmt.Printf("Smoke FAILED: %s returned %d (%s)\n", url, resp.StatusCode, strings.TrimSpace(string(body)))
			os.Exit(1)
		}
		fmt.Printf("OK %s -> %d\n", url, resp.StatusCode)
	}
	fmt.Println("Smoke SUCCESSFUL")
}

func collectStartupChecks(projectPath string, checkConfig bool) ([]startupCheck, error) {
	root := filepath.Clean(strings.TrimSpace(projectPath))
	if root == "" {
		root = "."
	}
	var checks []startupCheck
	add := func(c startupCheck) { checks = append(checks, c) }

	add(checkTool("go", true))
	add(checkTool("ang", true))
	add(checkFile(filepath.Join(root, "cue", "project", "project.cue"), true, "run `ang init` or restore cue/project/project.cue"))
	add(checkFile(filepath.Join(root, "go.mod"), true, "run `ang init` in a project directory"))

	composePath := filepath.Join(root, "docker-compose.yml")
	if _, err := os.Stat(composePath); err == nil {
		add(checkTool("docker", true))
		if cmd, err := detectComposeCommand(); err != nil {
			add(startupCheck{
				Name:   "docker-compose",
				Status: startupFail,
				Detail: err.Error(),
				Hint:   "install Docker with Compose plugin, or docker-compose binary",
			})
		} else {
			add(startupCheck{
				Name:   "docker-compose",
				Status: startupOK,
				Detail: strings.Join(cmd, " "),
			})
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	} else {
		add(startupCheck{
			Name:   "docker-compose.yml",
			Status: startupWarn,
			Detail: "not found",
			Hint:   "docker services will be skipped by `ang up`",
		})
	}

	frontendPkg := filepath.Join(root, "frontend", "package.json")
	if _, err := os.Stat(frontendPkg); err == nil {
		add(checkTool("node", false))
		add(checkTool("npm", false))
	}

	if checkConfig {
		cfgChecks, err := collectConfigStartupChecks(root)
		if err != nil {
			return nil, err
		}
		checks = append(checks, cfgChecks...)
	}

	port := resolveHTTPPort(root)
	if strings.TrimSpace(port) != "" {
		addr := ":" + port
		// Port conflict is a warning (not failure): an already running local server is valid.
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			add(startupCheck{
				Name:   "http-port",
				Status: startupWarn,
				Detail: fmt.Sprintf("port %s is already in use", port),
				Hint:   "if your server is already running this is fine; otherwise stop conflicting process",
			})
		} else {
			_ = ln.Close()
			add(startupCheck{
				Name:   "http-port",
				Status: startupOK,
				Detail: fmt.Sprintf("port %s is available", port),
			})
		}
	}

	sort.SliceStable(checks, func(i, j int) bool {
		return checks[i].Name < checks[j].Name
	})
	return checks, nil
}

func collectConfigStartupChecks(projectPath string) ([]startupCheck, error) {
	var checks []startupCheck
	configPath := filepath.Join(projectPath, "internal", "config", "config.go")
	fields, err := parseConfigEnvFields(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || strings.Contains(strings.ToLower(err.Error()), "no such file") {
			checks = append(checks, startupCheck{
				Name:   "config-schema",
				Status: startupWarn,
				Detail: "internal/config/config.go not found",
				Hint:   "run `ang build` first",
			})
			return checks, nil
		}
		return nil, err
	}

	envPath := filepath.Join(projectPath, ".env")
	envValues, _, err := readEnvFile(envPath)
	if err != nil {
		return nil, err
	}
	examplePath := filepath.Join(projectPath, ".env.example")
	exampleValues, exampleFound, err := readEnvFile(examplePath)
	if err != nil {
		return nil, err
	}
	exampleKeys := map[string]struct{}{}
	for key := range exampleValues {
		exampleKeys[key] = struct{}{}
	}

	getenv := func(key string) string { return os.Getenv(key) }
	missing, warnings := evaluateConfig(fields, envValues, exampleKeys, getenv)

	if len(missing) > 0 {
		checks = append(checks, startupCheck{
			Name:   "config-env",
			Status: startupFail,
			Detail: strings.Join(missing, "; "),
			Hint:   "fill .env or export env vars, then run `ang config doctor`",
		})
	} else {
		checks = append(checks, startupCheck{
			Name:   "config-env",
			Status: startupOK,
			Detail: "required config is present",
		})
	}

	if !exampleFound {
		checks = append(checks, startupCheck{
			Name:   ".env.example",
			Status: startupWarn,
			Detail: "not found",
			Hint:   "run `ang build` to regenerate .env.example",
		})
	} else if len(warnings) > 0 {
		checks = append(checks, startupCheck{
			Name:   ".env.example",
			Status: startupWarn,
			Detail: strings.Join(warnings, "; "),
			Hint:   "sync config schema and .env.example",
		})
	} else {
		checks = append(checks, startupCheck{
			Name:   ".env.example",
			Status: startupOK,
			Detail: "in sync with config schema",
		})
	}

	return checks, nil
}

func checkTool(name string, required bool) startupCheck {
	path, err := exec.LookPath(name)
	if err != nil {
		status := startupWarn
		if required {
			status = startupFail
		}
		return startupCheck{
			Name:   "tool:" + name,
			Status: status,
			Detail: "not found in PATH",
			Hint:   "install " + name + " and retry",
		}
	}
	return startupCheck{
		Name:   "tool:" + name,
		Status: startupOK,
		Detail: path,
	}
}

func checkFile(path string, required bool, hint string) startupCheck {
	if _, err := os.Stat(path); err != nil {
		status := startupWarn
		if required {
			status = startupFail
		}
		return startupCheck{
			Name:   "file:" + path,
			Status: status,
			Detail: "not found",
			Hint:   hint,
		}
	}
	return startupCheck{
		Name:   "file:" + path,
		Status: startupOK,
		Detail: "present",
	}
}

func printStartupChecks(checks []startupCheck) bool {
	fmt.Println("Doctor start report")
	hasFail := false
	for _, c := range checks {
		prefix := "[OK]"
		switch c.Status {
		case startupWarn:
			prefix = "[WARN]"
		case startupFail:
			prefix = "[FAIL]"
			hasFail = true
		}
		fmt.Printf("  %s %s: %s\n", prefix, c.Name, c.Detail)
		if strings.TrimSpace(c.Hint) != "" {
			fmt.Printf("       hint: %s\n", c.Hint)
		}
	}
	return hasFail
}

func detectComposeCommand() ([]string, error) {
	if dockerPath, err := exec.LookPath("docker"); err == nil {
		cmd := exec.Command(dockerPath, "compose", "version")
		if err := cmd.Run(); err == nil {
			return []string{"docker", "compose"}, nil
		}
	}
	if composePath, err := exec.LookPath("docker-compose"); err == nil {
		return []string{composePath}, nil
	}
	return nil, fmt.Errorf("docker compose is unavailable")
}

func runCommand(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runSelf(projectPath string, args ...string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return runCommand(projectPath, exe, args...)
}

func resolveHTTPPort(projectPath string) string {
	if v := strings.TrimSpace(os.Getenv("HTTP_PORT")); v != "" {
		return v
	}
	envPath := filepath.Join(projectPath, ".env")
	envValues, _, err := readEnvFile(envPath)
	if err == nil {
		if v := strings.TrimSpace(envValues["HTTP_PORT"]); v != "" {
			return v
		}
	}
	return "8080"
}
