package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"golang.org/x/mod/semver"
)

// rawGoScopeCUE is Go that the generated method used to break: names the
// template declared next to the code (err, the metrics' observeDone, the
// transaction's txErr) and the rewrites it applied to the code (`err :=` to
// `err =`, a variable l to slog, .ExternalId to .ExternalID, inside strings
// too).
const rawGoScopeCUE = `package api

RenameInvoice: {
	service: "billing"
	input: {id: string}
	output: {label: string, path: string}
	impls: go: {
		lang: "go"
		imports: ["strconv"]
		code: """
			// err of another type than the method's result
			err := strconv.Itoa(len(req.ID))
			type line struct {
				ExternalId string
				Name       string
			}
			l := line{ExternalId: "x-" + err, Name: "first"}
			observeDone := l.Name + "/" + l.ExternalId
			resp := port.RenameInvoiceResponse{Label: observeDone, Path: "$.data.userId"}
			return resp, nil
			"""
	}
}

ReissueInvoice: {
	service:       "billing"
	transactional: true
	input: {id: string}
	output: {ok: bool}
	impls: go: {
		lang: "go"
		imports: ["strconv"]
		code: """
			err := strconv.Itoa(len(req.ID))
			txErr := err != ""
			var resp port.ReissueInvoiceResponse
			resp.Ok = txErr
			return resp, nil
			"""
	}
}
`

// Item 11 of docs/tech-debt-backlog.md: impls: go: code is a block of its own
// and is not rewritten, so any Go name is legal in it. The generated service
// package must compile with operation metrics on (the template used to
// declare observeDone next to the code).
func TestRawGoCodeRunsInItsOwnScope(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH")
	}
	projectDir := filepath.Join(t.TempDir(), "rawgoapp")
	if err := initFromTemplate(initTemplateOptions{
		TemplateName: "saas",
		TargetDir:    projectDir,
		ProjectName:  "rawgoapp",
		Lang:         "go",
		DB:           "postgres",
		ModulePath:   "github.com/example/rawgoapp",
		Force:        true,
	}); err != nil {
		t.Fatal(err)
	}
	projectCUE := filepath.Join(projectDir, "cue", "project", "project.cue")
	data, err := os.ReadFile(projectCUE)
	if err != nil {
		t.Fatal(err)
	}
	withMetrics := strings.Replace(string(data), "target: {", "target: {\n\t\toperation_metrics: true", 1)
	withMetrics = strings.Replace(withMetrics, "targets: [{", "targets: [{\n\t\toperation_metrics: true", 1)
	writeTestFile(t, projectCUE, withMetrics)
	writeTestFile(t, filepath.Join(projectDir, "cue", "api", "rawgo.cue"), rawGoScopeCUE)
	if err := runBuild([]string{projectDir, "--mode=in_place", "--backend-dir=.", "--skip-go-verify", "--skip-frontend"}); err != nil {
		t.Fatalf("build: %v", err)
	}
	read := func(rel string) string {
		data, err := os.ReadFile(filepath.Join(projectDir, rel))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	rename := read("internal/service/billing__rename_invoice.gen.go")
	for _, want := range []string{
		`defer observeOperation(&ctx, "Billing", "RenameInvoice")(&err)`,
		"err := strconv.Itoa(len(req.ID))",
		`l := line{ExternalId: "x-" + err, Name: "first"}`,
		`Path: "$.data.userId"`,
		"//line ../../cue/api/rawgo.cue:12\n",
	} {
		if !strings.Contains(rename, want) {
			t.Errorf("billing__rename_invoice.gen.go lacks %q:\n%s", want, rename)
		}
	}
	if strings.Contains(rename, "slog.") || strings.Contains(rename, "ExternalID") {
		t.Errorf("the code was rewritten:\n%s", rename)
	}
	reissue := read("internal/service/billing__reissue_invoice.gen.go")
	if !strings.Contains(reissue, "err := strconv.Itoa(len(req.ID))") || !strings.Contains(reissue, "s.txManager.WithTx") {
		t.Errorf("billing__reissue_invoice.gen.go:\n%s", reissue)
	}

	vetGeneratedPackagesOffline(t, projectDir, "github.com/example/rawgoapp", "./internal/service/")
}

// vetGeneratedPackagesOffline runs go vet on packages of a generated project
// with ANG's own dependency versions, offline: a module the generated code
// needs beyond those is pinned to the newest version the local module cache
// holds, and the cache is the GOPROXY. The test is skipped when the cache
// lacks a module.
func vetGeneratedPackagesOffline(t *testing.T, projectDir, module string, pkgs ...string) {
	t.Helper()
	runGoOffline(t, projectDir, module, append([]string{"vet"}, pkgs...)...)
}

// runGoOffline runs a go command (vet, test …) in a generated project with
// ANG's own requirements and the module cache as the only proxy, pinning a
// module ANG's go.mod lacks to its newest cached version.
func runGoOffline(t *testing.T, projectDir, module string, goArgs ...string) {
	t.Helper()
	repoRoot, _ := filepath.Abs("../..")
	angMod, err := os.ReadFile(filepath.Join(repoRoot, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	requires := string(angMod)[strings.Index(string(angMod), "require"):]
	writeTestFile(t, filepath.Join(projectDir, "go.mod"), "module "+module+"\n\ngo 1.27.0\n\n"+requires)
	sum, err := os.ReadFile(filepath.Join(repoRoot, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(projectDir, "go.sum"), string(sum))
	modCache, err := exec.Command("go", "env", "GOMODCACHE").Output()
	if err != nil || strings.TrimSpace(string(modCache)) == "" {
		t.Skip("no module cache")
	}
	download := filepath.Join(strings.TrimSpace(string(modCache)), "cache", "download")
	env := append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod", "GOPROXY=file://"+filepath.ToSlash(download), "GOSUMDB=off")
	missingPackage := regexp.MustCompile(`cannot find module providing package ([^\s:]+)`)
	missingZip := regexp.MustCompile(`([^\s:@]+)@(v[^\s:]+): reading file:[^\s]*\.zip: no such file`)
	for attempt := 0; ; attempt++ {
		cmd := exec.Command("go", goArgs...)
		cmd.Dir = projectDir
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err == nil {
			return
		}
		pins := map[string]string{}
		for _, m := range missingPackage.FindAllStringSubmatch(string(out), -1) {
			if mod, version := newestCachedModule(download, m[1]); mod != "" {
				pins[mod] = version
			} else {
				t.Skipf("module cache lacks a dependency: %s", out)
			}
		}
		for _, m := range missingZip.FindAllStringSubmatch(string(out), -1) {
			if _, pinned := pins[m[1]]; pinned {
				continue // the package's module, pinned above
			}
			if mod, version := newestCachedModule(download, m[1]); mod == m[1] && semver.Compare(version, m[2]) > 0 {
				pins[mod] = version
			} else {
				t.Skipf("module cache lacks a dependency: %s", out)
			}
		}
		if len(pins) == 0 {
			t.Fatalf("go %s in the generated project: %v\n%s", strings.Join(goArgs, " "), err, out)
		}
		if attempt > 20 {
			t.Skipf("module cache cannot satisfy the dependencies: %s", out)
		}
		for mod, version := range pins {
			edit := exec.Command("go", "mod", "edit", "-require="+mod+"@"+version)
			edit.Dir = projectDir
			edit.Env = env
			if editOut, err := edit.CombinedOutput(); err != nil {
				t.Fatalf("go mod edit: %v\n%s", err, editOut)
			}
		}
	}
}

// newestCachedModule is the module of pkg the module cache can serve (its
// zip is there) and the newest such version.
func newestCachedModule(download, pkg string) (string, string) {
	escape := func(path string) string {
		var b strings.Builder
		for _, r := range path {
			if r >= 'A' && r <= 'Z' {
				b.WriteByte('!')
				r += 'a' - 'A'
			}
			b.WriteRune(r)
		}
		return b.String()
	}
	for mod := pkg; mod != "." && mod != ""; mod = filepath.ToSlash(filepath.Dir(mod)) {
		zips, _ := filepath.Glob(filepath.Join(download, filepath.FromSlash(escape(mod)), "@v", "*.zip"))
		best := ""
		for _, zip := range zips {
			v := strings.TrimSuffix(filepath.Base(zip), ".zip")
			if semver.IsValid(v) && (best == "" || semver.Compare(v, best) > 0) {
				best = v
			}
		}
		if best != "" {
			return mod, best
		}
	}
	return "", ""
}
