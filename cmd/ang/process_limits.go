package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

const defaultBuildMaxProcs = 2

func applyBuildProcessLimitsIfNeeded(cmd string) {
	if !isBuildLikeCommand(cmd) {
		return
	}
	limit := resolvedBuildMaxProcs()
	if limit <= 0 {
		return
	}
	runtime.GOMAXPROCS(limit)
	_ = os.Setenv("GOMAXPROCS", strconv.Itoa(limit))
	_ = os.Setenv("GOFLAGS", withGoParallelism(os.Getenv("GOFLAGS"), limit))
}

func isBuildLikeCommand(cmd string) bool {
	switch strings.TrimSpace(cmd) {
	case "build", "up", "first-run":
		return true
	default:
		return false
	}
}

func resolvedBuildMaxProcs() int {
	limit := defaultBuildMaxProcs
	if raw := strings.TrimSpace(os.Getenv("ANG_MAX_PROCS")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if raw := strings.TrimSpace(os.Getenv("GOMAXPROCS")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed < limit {
			limit = parsed
		}
	}
	if cpu := runtime.NumCPU(); cpu > 0 && limit > cpu {
		limit = cpu
	}
	if limit < 1 {
		limit = 1
	}
	return limit
}

func configureBuildSubprocess(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	limit := resolvedBuildMaxProcs()
	if limit <= 0 {
		return
	}
	env := cmd.Env
	if len(env) == 0 {
		env = os.Environ()
	} else {
		env = append([]string{}, env...)
	}
	env = upsertEnv(env, "GOMAXPROCS", strconv.Itoa(limit))
	env = upsertEnv(env, "GOFLAGS", withGoParallelism(envValue(env, "GOFLAGS"), limit))
	cmd.Env = env
}

func withGoParallelism(flags string, limit int) string {
	tokens := strings.Fields(flags)
	filtered := make([]string, 0, len(tokens)+1)
	skipNext := false
	for _, token := range tokens {
		if skipNext {
			skipNext = false
			continue
		}
		if token == "-p" {
			skipNext = true
			continue
		}
		if strings.HasPrefix(token, "-p=") {
			continue
		}
		filtered = append(filtered, token)
	}
	filtered = append(filtered, fmt.Sprintf("-p=%d", limit))
	return strings.TrimSpace(strings.Join(filtered, " "))
}

func upsertEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, item := range env {
		if strings.HasPrefix(item, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func envValue(env []string, key string) string {
	prefix := key + "="
	for _, item := range env {
		if strings.HasPrefix(item, prefix) {
			return strings.TrimPrefix(item, prefix)
		}
	}
	return ""
}
