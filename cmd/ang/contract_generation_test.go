//go:build contract

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContract_UserMutedTypesAndTenderMuteFlow(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "saas-contract")
	if err := initFromTemplate(initTemplateOptions{
		TemplateName: "saas",
		TargetDir:    projectDir,
		ProjectName:  "saas-contract",
		Lang:         "go",
		DB:           "postgres",
		ModulePath:   "github.com/example/saas-contract",
		Force:        true,
	}); err != nil {
		t.Fatalf("initFromTemplate: %v", err)
	}

	entitiesPath := filepath.Join(projectDir, "cue", "domain", "entities.cue")
	if err := insertAfter(
		entitiesPath,
		`		role: {type: "string"}`,
		`
        notificationMutedTypes: {
            type: "[]string"
            description: "Muted notification channels"
            optional: true
        }`,
	); err != nil {
		t.Fatalf("inject notificationMutedTypes: %v", err)
	}

	opsPath := filepath.Join(projectDir, "cue", "api", "operations.cue")
	flowBlock := `

// Tender notification mute guard
ProcessTenderMute: {
	service:   "auth"
	description: "Guard tender notification delivery by user mute settings"

	input: {
		userId: string @validate("required,uuid")
	}

	output: {
		ok: bool
	}

	flow: [
		{action: "repo.Find", source: "User", input: "req.UserId", output: "user", error: "User not found"},
		{action: "logic.Check", condition: "len(user.NotificationMutedTypes) == 0", throw: "Tender notifications are muted"},
		{action: "mapping.Assign", to: "resp.Ok", value: "true"},
	]
}
`
	if err := appendText(opsPath, flowBlock); err != nil {
		t.Fatalf("append tender mute operation: %v", err)
	}

	runBuild([]string{projectDir, "--mode=release"})

	releaseRoot := filepath.Join(projectDir, "dist", "release", "go-service")
	domainUser := filepath.Join(releaseRoot, "internal", "domain", "user.go")
	dtoUser := filepath.Join(releaseRoot, "internal", "dto", "user.go")
	repoUser := filepath.Join(releaseRoot, "internal", "adapter", "repository", "postgres", "userrepository.go")
	svcAuth := filepath.Join(releaseRoot, "internal", "service", "auth.go")

	checkContains(t, domainUser, "NotificationMutedTypes []string")
	checkContains(t, dtoUser, "NotificationMutedTypes []string")
	checkContains(t, svcAuth, "len(user.NotificationMutedTypes) == 0")
	checkContains(t, svcAuth, "ProcessTenderMute")

	checkNotContains(t, domainUser, "[]any")
	checkNotContains(t, dtoUser, "[]any")
	checkNotContains(t, repoUser, "[]any")
	checkNotContains(t, svcAuth, "[]any")
	checkNotContains(t, domainUser, "[]interface{}")
	checkNotContains(t, dtoUser, "[]interface{}")
	checkNotContains(t, repoUser, "[]interface{}")
	checkNotContains(t, svcAuth, "[]interface{}")

}

func insertAfter(path, anchor, payload string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(data)
	if !strings.Contains(text, anchor) {
		return fmt.Errorf("anchor not found in %s", path)
	}
	out := strings.Replace(text, anchor, anchor+payload, 1)
	return os.WriteFile(path, []byte(out), 0o644)
}

func appendText(path, payload string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(payload)
	return err
}

func checkContains(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(data), want) {
		t.Fatalf("%s missing %q", path, want)
	}
}

func checkNotContains(t *testing.T, path, bad string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if strings.Contains(string(data), bad) {
		t.Fatalf("%s contains forbidden %q", path, bad)
	}
}
