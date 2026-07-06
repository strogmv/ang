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
	"strconv"
	"strings"
	"time"

	"github.com/strogmv/ang/compiler"
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
		printCommandFailure("Doctor start", err.Error(), "run `ang doctor start --help`")
		os.Exit(1)
	}

	checks, err := collectStartupChecks(*projectPath, !*skipConfig)
	if err != nil {
		printCommandFailure("Doctor start", err.Error(), "")
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
	watch := fs.Bool("watch", false, "watch cue/ changes and auto-run validate/build/smoke loop")
	watchInterval := fs.Duration("watch-interval", 2*time.Second, "poll interval for --watch mode")
	fun := fs.Bool("fun", false, "show launch banner and celebratory ready marker")
	frontend := fs.Bool("frontend", false, "start frontend dev server in background when package.json is present")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return
		}
		printCommandFailure("Up", err.Error(), "run `ang up --help`")
		os.Exit(1)
	}

	root := filepath.Clean(strings.TrimSpace(*projectPath))
	if root == "" {
		root = "."
	}
	funMode := isFunEnabled(*fun)
	if funMode {
		printFunRocket()
	}
	if usedPort, altPort := suggestPortConflict(root); usedPort != "" && altPort != "" {
		fmt.Printf("Port hint: HTTP_PORT=%s is busy. Try HTTP_PORT=%s (for smoke: `ang smoke --base-url http://localhost:%s`).\n", usedPort, altPort, altPort)
	}
	steps := 4
	if *frontend {
		steps++
	}
	progress := newUpProgress(steps)

	progress.step("Preflight checks")
	if !*skipDoctor {
		// Preflight is intentionally first: it surfaces missing env/tools before build noise.
		checks, err := collectStartupChecks(root, true)
		if err != nil {
			printCommandFailure("Up", err.Error(), "run `ang doctor start`")
			os.Exit(1)
		}
		hasFail := printStartupChecks(checks)
		if hasFail && *doctorStrict {
			printCommandFailure("Up", "preflight checks failed", "run `ang doctor start --strict=false` to inspect all checks")
			os.Exit(1)
		}
	} else {
		fmt.Println("     skipped (--skip-doctor)")
	}

	progress.step("Dependencies (docker compose)")
	if !*skipCompose {
		composePath := filepath.Join(root, strings.TrimSpace(*composeFile))
		if _, err := os.Stat(composePath); err == nil {
			// Detect compose command dynamically to support both modern plugin and legacy binary.
			composeCmd, err := detectComposeCommand()
			if err != nil {
				printCommandFailure("Up", err.Error(), "install Docker Compose or pass `--skip-compose`")
				os.Exit(1)
			}
			cmdArgs := append([]string{}, composeCmd[1:]...)
			cmdArgs = append(cmdArgs, "-f", composePath, "up")
			if *detach {
				cmdArgs = append(cmdArgs, "-d")
			}
			fmt.Printf("==> Running: %s %s\n", composeCmd[0], strings.Join(cmdArgs, " "))
			if err := runCommand(root, composeCmd[0], cmdArgs...); err != nil {
				printCommandFailure("Up", fmt.Sprintf("compose up: %v", err), "run `docker compose logs` and retry")
				os.Exit(1)
			}
		} else if !os.IsNotExist(err) {
			printCommandFailure("Up", fmt.Sprintf("stat compose file: %v", err), "")
			os.Exit(1)
		} else {
			fmt.Printf("==> Skipping compose: %s not found\n", composePath)
		}
	} else {
		fmt.Println("     skipped (--skip-compose)")
	}

	progress.step("Code generation (ang build)")
	if !*skipBuild {
		fmt.Println("==> Running: ang build")
		if err := runSelf(root, "build"); err != nil {
			printCommandFailure("Up", fmt.Sprintf("build: %v", err), "run `ang build` directly to inspect emitter errors")
			os.Exit(1)
		}
	} else {
		fmt.Println("     skipped (--skip-build)")
	}

	if *frontend {
		progress.step("Frontend dev server")
		if err := startFrontendDev(root); err != nil {
			printCommandFailure("Up", fmt.Sprintf("frontend: %v", err), "check frontend/package.json and npm scripts")
			os.Exit(1)
		}
	}

	progress.step("Readiness smoke")
	if !*skipSmoke {
		fmt.Println("==> Running: ang smoke")
		if err := runSelf(root, "smoke"); err != nil {
			printCommandFailure("Up", fmt.Sprintf("smoke: %v", err), "check server logs, then run `ang smoke --base-url http://localhost:8080`")
			os.Exit(1)
		}
	} else {
		fmt.Println("     skipped (--skip-smoke)")
	}

	fmt.Println("READY: local bootstrap completed.")
	fmt.Println("Tip: run `ang tips` for quick next commands.")
	if *watch {
		runUpWatchLoop(root, *skipSmoke, *watchInterval)
	}
}

func startFrontendDev(root string) error {
	frontendDir := ""
	for _, candidate := range []string{filepath.Join(root, "frontend"), root} {
		if _, err := os.Stat(filepath.Join(candidate, "package.json")); err == nil {
			frontendDir = candidate
			break
		}
	}
	if frontendDir == "" {
		return fmt.Errorf("package.json not found in frontend/ or project root")
	}
	npm, err := exec.LookPath("npm")
	if err != nil {
		return fmt.Errorf("npm not found in PATH")
	}
	stateDir := filepath.Join(root, ".ang")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	logPath := filepath.Join(stateDir, "frontend.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	cmd := exec.Command(npm, "run", "dev", "--", "--host", "0.0.0.0")
	cmd.Dir = frontendDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return err
	}
	_ = logFile.Close()
	pidPath := filepath.Join(stateDir, "frontend.pid")
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0o644); err != nil {
		_ = cmd.Process.Kill()
		return err
	}
	fmt.Printf("==> Frontend started: pid=%d log=%s\n", cmd.Process.Pid, logPath)
	return nil
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
	if _, err := os.Stat(filepath.Join(root, "atlas.hcl")); err == nil {
		add(checkTool("atlas", false))
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
			hint := "if your server is already running this is fine; otherwise stop conflicting process"
			if alt := findFreePortNear(port); alt != "" {
				hint = fmt.Sprintf("%s; or set HTTP_PORT=%s and retry", hint, alt)
			}
			add(startupCheck{
				Name:   "http-port",
				Status: startupWarn,
				Detail: fmt.Sprintf("port %s is already in use", port),
				Hint:   hint,
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
	autofixDetails, err := bootstrapEnvFromExample(projectPath)
	if err != nil {
		return nil, err
	}
	if len(autofixDetails) > 0 {
		checks = append(checks, startupCheck{
			Name:   ".env.bootstrap",
			Status: startupOK,
			Detail: strings.Join(autofixDetails, "; "),
		})
	}
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

func bootstrapEnvFromExample(projectPath string) ([]string, error) {
	examplePath := filepath.Join(projectPath, ".env.example")
	exampleValues, exampleFound, err := readEnvFile(examplePath)
	if err != nil {
		return nil, err
	}
	if !exampleFound {
		return nil, nil
	}

	envPath := filepath.Join(projectPath, ".env")
	envValues, envFound, err := readEnvFile(envPath)
	if err != nil {
		return nil, err
	}

	var actions []string
	if !envFound {
		raw, err := os.ReadFile(examplePath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", examplePath, err)
		}
		if err := os.WriteFile(envPath, raw, 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w", envPath, err)
		}
		actions = append(actions, "created .env from .env.example")
		envValues, _, err = readEnvFile(envPath)
		if err != nil {
			return nil, err
		}
	}

	keys := make([]string, 0, len(exampleValues))
	for key := range exampleValues {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var lines []string
	for _, key := range keys {
		exampleVal := strings.TrimSpace(exampleValues[key])
		if exampleVal == "" {
			continue
		}
		if strings.TrimSpace(envValues[key]) != "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s=%s", key, exampleValues[key]))
		actions = append(actions, fmt.Sprintf("filled %s from .env.example", key))
		envValues[key] = exampleValues[key]
	}
	if len(lines) == 0 {
		return actions, nil
	}

	f, err := os.OpenFile(envPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", envPath, err)
	}
	defer f.Close()
	if stat, err := f.Stat(); err == nil && stat.Size() > 0 {
		if _, err := f.WriteString("\n"); err != nil {
			return nil, fmt.Errorf("append newline to %s: %w", envPath, err)
		}
	}
	for _, line := range lines {
		if _, err := f.WriteString(line + "\n"); err != nil {
			return nil, fmt.Errorf("append %s to %s: %w", line, envPath, err)
		}
	}
	return actions, nil
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

func suggestPortConflict(projectPath string) (usedPort, suggestedPort string) {
	port := resolveHTTPPort(projectPath)
	if strings.TrimSpace(port) == "" {
		return "", ""
	}
	ln, err := net.Listen("tcp", ":"+port)
	if err == nil {
		_ = ln.Close()
		return "", ""
	}
	alt := findFreePortNear(port)
	if alt == "" {
		return port, ""
	}
	return port, alt
}

func findFreePortNear(port string) string {
	base := strings.TrimSpace(port)
	if base == "" {
		base = "8080"
	}
	start := 8080
	if parsed, err := strconv.Atoi(base); err == nil && parsed > 0 {
		start = parsed + 1
	}
	for p := start; p < start+100; p++ {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", p))
		if err != nil {
			continue
		}
		_ = ln.Close()
		return fmt.Sprintf("%d", p)
	}
	return ""
}

func runUpWatchLoop(projectPath string, skipSmoke bool, interval time.Duration) {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	lastHash, err := compiler.ComputeProjectHash(projectPath)
	if err != nil {
		fmt.Printf("Watch mode: unable to compute initial cue hash (%v). Continuing with best effort.\n", err)
		lastHash = ""
	}
	fmt.Printf("Watch mode: polling cue/ every %s. Press Ctrl+C to stop.\n", interval)
	for {
		time.Sleep(interval)
		nextHash, err := compiler.ComputeProjectHash(projectPath)
		if err != nil {
			fmt.Printf("Watch mode: hash error: %v\n", err)
			continue
		}
		if nextHash == lastHash {
			continue
		}
		lastHash = nextHash

		fmt.Println("Watch mode: change detected. Running validate -> build -> smoke ...")
		if err := runSelf(projectPath, "validate"); err != nil {
			printCommandFailure("Watch", fmt.Sprintf("validate: %v", err), "fix CUE errors and save again")
			continue
		}
		if err := runSelf(projectPath, "build"); err != nil {
			printCommandFailure("Watch", fmt.Sprintf("build: %v", err), "fix generation/runtime issues and save again")
			continue
		}
		if !skipSmoke {
			if err := runSelf(projectPath, "smoke"); err != nil {
				printCommandFailure("Watch", fmt.Sprintf("smoke: %v", err), "check server health and retry")
				continue
			}
		}
		fmt.Println("Watch mode: cycle OK.")
	}
}

type upProgress struct {
	total int
	stepN int
}

func newUpProgress(total int) *upProgress {
	return &upProgress{total: total}
}

func (p *upProgress) step(label string) {
	p.stepN++
	fmt.Printf("[%d/%d] %s\n", p.stepN, p.total, label)
}
