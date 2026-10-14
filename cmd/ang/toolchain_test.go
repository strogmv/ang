package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeANGMain is the cmd/ang of a stand-in ANG repository: it prints the
// revision it was built from (ldflags and VCS) and a marker of its commit.
const fakeANGMain = `package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"
)

var buildRevision string

const marker = "MARKER"

func main() {
	vcs := ""
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" {
				vcs = s.Value
			}
		}
	}
	fmt.Printf("marker=%s ldflags=%s vcs=%s args=%s\n", marker, buildRevision, vcs, strings.Join(os.Args[1:], " "))
}
`

type fakeANGRepo struct {
	Dir     string
	Commits []string // oldest first
}

func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Env = toolchainGitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// newFakeANGRepo makes a git repository with two commits of a tiny Go program
// at cmd/ang, the first marked "one", the second "two", and leaves the
// checkout on a branch at the second.
func newFakeANGRepo(t *testing.T) fakeANGRepo {
	t.Helper()
	// The test binary is built from the ANG checkout; it is not a source here.
	self := toolchainSelfCheckout
	toolchainSelfCheckout = func() string { return "" }
	t.Cleanup(func() { toolchainSelfCheckout = self })
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not installed")
	}
	dir := filepath.Join(t.TempDir(), "ang")
	if err := os.MkdirAll(filepath.Join(dir, "cmd", "ang"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitT(t, dir, "init", "--quiet", "-b", "main")
	writeTestFile(t, filepath.Join(dir, "go.mod"), "module example.com/fakeang\n\ngo 1.21\n")
	repo := fakeANGRepo{Dir: dir}
	for _, marker := range []string{"one", "two"} {
		writeTestFile(t, filepath.Join(dir, "cmd", "ang", "main.go"), strings.Replace(fakeANGMain, "MARKER", marker, 1))
		gitT(t, dir, "add", "-A")
		gitT(t, dir, "commit", "--quiet", "-m", marker)
		repo.Commits = append(repo.Commits, gitT(t, dir, "rev-parse", "HEAD"))
	}
	// The pinned commit is on main only through history; the checkout moves on.
	gitT(t, dir, "checkout", "--quiet", "-b", "feature")
	return repo
}

func runToolchainBinary(t *testing.T, bin string, args ...string) string {
	t.Helper()
	out, err := exec.Command(bin, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v\n%s", bin, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestEnsureToolchainBuildsThePinnedRevisionFromALocalClone(t *testing.T) {
	repo := newFakeANGRepo(t)
	cache := toolchainCache{Root: filepath.Join(t.TempDir(), "cache", "ang")}
	t.Setenv(angSrcEnv, repo.Dir)
	pinned := repo.Commits[0]
	pin := angLockPin{Repository: "https://invalid.example/ang.git", Commit: pinned}

	var log bytes.Buffer
	bin, err := ensureToolchain(cache, pin, &log)
	if err != nil {
		t.Fatalf("ensureToolchain: %v\n%s", err, log.String())
	}
	if want := filepath.Join(cache.Root, "toolchains", pinned, "ang"); bin != want {
		t.Fatalf("binary at %s, want %s", bin, want)
	}
	out := runToolchainBinary(t, bin, "build", ".")
	for _, want := range []string{"marker=one", "ldflags=" + pinned, "vcs=" + pinned, "args=build ."} {
		if !strings.Contains(out, want) {
			t.Errorf("toolchain output %q lacks %q", out, want)
		}
	}
	if !strings.Contains(log.String(), "building "+shortRevision(pinned)) {
		t.Errorf("log does not say it builds the revision:\n%s", log.String())
	}

	// The second call uses the cache: no fetch, no build.
	stat, _ := os.Stat(bin)
	log.Reset()
	again, err := ensureToolchain(cache, pin, &log)
	if err != nil || again != bin {
		t.Fatalf("second ensureToolchain = %q, %v", again, err)
	}
	if log.Len() != 0 {
		t.Errorf("a cached toolchain must not be fetched or built again:\n%s", log.String())
	}
	if now, _ := os.Stat(bin); !now.ModTime().Equal(stat.ModTime()) {
		t.Error("the cached binary was rebuilt")
	}
	// Nothing of the build is left next to the binaries, and the source
	// checkout was not touched.
	entries, _ := os.ReadDir(filepath.Join(cache.Root, "toolchains"))
	for _, e := range entries {
		if e.Name() != pinned {
			t.Errorf("left behind in the cache: %s", e.Name())
		}
	}
	if branch := gitT(t, repo.Dir, "rev-parse", "--abbrev-ref", "HEAD"); branch != "feature" {
		t.Errorf("the local clone moved to %s", branch)
	}
	if worktrees := gitT(t, repo.Dir, "worktree", "list", "--porcelain"); strings.Count(worktrees, "worktree ") != 1 {
		t.Errorf("the local clone got worktrees:\n%s", worktrees)
	}
}

func TestEnsureToolchainFetchesFromTheLockRepository(t *testing.T) {
	repo := newFakeANGRepo(t)
	cache := toolchainCache{Root: filepath.Join(t.TempDir(), "cache", "ang")}
	t.Setenv(angSrcEnv, "")
	// An abbreviated pin, the repository as a URL.
	pin := angLockPin{Repository: "file://" + repo.Dir, Commit: repo.Commits[1][:10]}
	var log bytes.Buffer
	bin, err := ensureToolchain(cache, pin, &log)
	if err != nil {
		t.Fatalf("ensureToolchain: %v\n%s", err, log.String())
	}
	if !strings.Contains(bin, repo.Commits[1]) {
		t.Errorf("an abbreviated pin must be cached under the full hash: %s", bin)
	}
	if out := runToolchainBinary(t, bin); !strings.Contains(out, "marker=two") {
		t.Errorf("built the wrong revision: %s", out)
	}
}

func TestEnsureToolchainNamesWhereItLookedForAMissingCommit(t *testing.T) {
	repo := newFakeANGRepo(t)
	cache := toolchainCache{Root: filepath.Join(t.TempDir(), "cache", "ang")}
	t.Setenv(angSrcEnv, repo.Dir)
	missing := strings.Repeat("ab", 20)
	_, err := ensureToolchain(cache, angLockPin{Repository: filepath.Join(t.TempDir(), "nowhere"), Commit: missing}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("a commit no source has must fail")
	}
	for _, want := range []string{missing, angSrcEnv + "=" + repo.Dir, "nowhere"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %q:\n%v", want, err)
		}
	}
}

func TestEnsureToolchainWaitsForAnotherBuild(t *testing.T) {
	cache := toolchainCache{Root: t.TempDir()}
	unlock, err := lockToolchainCache(cache, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	released := make(chan struct{})
	go func() {
		time.Sleep(300 * time.Millisecond)
		close(released)
		unlock()
	}()
	var log bytes.Buffer
	unlock2, err := lockToolchainCache(cache, &log)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock2()
	select {
	case <-released:
	default:
		t.Fatal("took the lock while another build held it")
	}
	if !strings.Contains(log.String(), "waiting") {
		t.Errorf("waiting is not reported: %q", log.String())
	}
}

func TestDecideToolchain(t *testing.T) {
	project := t.TempDir()
	writeTestFile(t, filepath.Join(project, angLockFileName), "repository=https://example.com/ang.git\ncommit="+pinnedRevision+"\n")
	unpinned := t.TempDir()
	env := func(kv ...string) func(string) string {
		m := map[string]string{}
		for i := 0; i+1 < len(kv); i += 2 {
			m[kv[i]] = kv[i+1]
		}
		return func(k string) string { return m[k] }
	}
	other := compilerRevision{Revision: otherRevision}
	cases := []struct {
		name    string
		args    []string
		env     func(string) string
		running compilerRevision
		want    bool
		wantErr bool
		reason  string
	}{
		{name: "other revision switches", args: []string{project}, env: env(), running: other, want: true},
		{name: "project flag", args: []string{"--skip-go-verify", "--project", project}, env: env(), running: other, want: true},
		{name: "project flag with =", args: []string{"--project=" + project}, env: env(), running: other, want: true},
		{name: "auto", args: []string{project}, env: env(toolchainEnv, "auto"), running: other, want: true},
		{name: "same revision", args: []string{project}, env: env(), running: compilerRevision{Revision: pinnedRevision}, reason: "pinned revision"},
		{name: "local", args: []string{project}, env: env(toolchainEnv, "local"), running: other, reason: "local"},
		{name: "already handed over", args: []string{project}, env: env(toolchainActiveEnv, pinnedRevision), running: other, reason: "already"},
		{name: "allow mismatch", args: []string{project, "--allow-lock-mismatch"}, env: env(), running: other, reason: "--allow-lock-mismatch"},
		{name: "no lock", args: []string{unpinned}, env: env(), running: other, reason: angLockFileName},
		{name: "unknown revision", args: []string{project}, env: env(), running: compilerRevision{}, reason: "no revision"},
		{name: "bad mode", args: []string{project}, env: env(toolchainEnv, "newest"), running: other, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, err := decideToolchain(tc.args, tc.env, tc.running)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if h.Switch != tc.want {
				t.Fatalf("switch = %v (%s), want %v", h.Switch, h.Reason, tc.want)
			}
			if h.Switch && h.Pin.Commit != pinnedRevision {
				t.Errorf("pin = %+v", h.Pin)
			}
			if tc.reason != "" && !strings.Contains(h.Reason, tc.reason) {
				t.Errorf("reason %q does not mention %q", h.Reason, tc.reason)
			}
		})
	}
}

func TestBuildArgsProject(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{nil, "."},
		{[]string{"app", "--check"}, "app"},
		{[]string{"--check", "--project", "p1"}, "p1"},
		{[]string{"-project=p2", "x"}, "p2"},
		{[]string{"--check"}, "."},
		{[]string{"--log-format", "json"}, "."},
	}
	for _, tc := range cases {
		if got := buildArgsProject(tc.args); got != tc.want {
			t.Errorf("buildArgsProject(%q) = %q, want %q", tc.args, got, tc.want)
		}
	}
}

// The real binary, run in a project that pins another revision, hands the
// build over to that revision with the same arguments; ANG_TOOLCHAIN=local
// keeps it and the lock check refuses.
func TestBuildHandsOverToThePinnedToolchain(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the ang binary")
	}
	repo := newFakeANGRepo(t)
	ang := filepath.Join(t.TempDir(), "ang")
	build := exec.Command("go", "build", "-o", ang, ".")
	build.Env = append(toolchainGitEnv(), "GOWORK=off")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	project := t.TempDir()
	writeTestFile(t, filepath.Join(project, angLockFileName), "repository=https://invalid.example/ang.git\ncommit="+repo.Commits[0]+"\n")
	if err := os.MkdirAll(filepath.Join(project, "cue"), 0o755); err != nil {
		t.Fatal(err)
	}

	run := func(extraEnv ...string) (string, string, error) {
		cmd := exec.Command(ang, "build", project, "--skip-go-verify")
		cmd.Env = append(toolchainGitEnv(), "XDG_CACHE_HOME="+filepath.Join(project, ".cache"), angSrcEnv+"="+repo.Dir, toolchainEnv+"=", toolchainActiveEnv+"=")
		cmd.Env = append(cmd.Env, extraEnv...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		return stdout.String(), stderr.String(), err
	}

	stdout, stderr, err := run()
	if err != nil {
		t.Fatalf("handover failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	if want := "marker=one ldflags=" + repo.Commits[0]; !strings.Contains(stdout, want) || !strings.Contains(stdout, "args=build "+project+" --skip-go-verify") {
		t.Errorf("the pinned toolchain did not get the build: %q", stdout)
	}
	if !strings.Contains(stderr, "running the pinned toolchain") || !strings.Contains(stderr, toolchainEnv+"=local") {
		t.Errorf("the handover is not explained on stderr: %q", stderr)
	}

	stdout, stderr, err = run(toolchainEnv + "=local")
	if err == nil {
		t.Fatalf("ANG_TOOLCHAIN=local must keep this binary, whose revision is not the pin\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	if strings.Contains(stdout, "marker=") {
		t.Errorf("ANG_TOOLCHAIN=local still ran the toolchain: %q", stdout)
	}
	if out := stdout + stderr; !strings.Contains(out, "pins ang at "+shortRevision(repo.Commits[0])) {
		t.Errorf("the lock check does not explain the refusal:\n%s", out)
	}
}
