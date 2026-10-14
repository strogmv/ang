package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// `ang toolchain` works like `go toolchain`: a project pins the generator
// revision in ang.lock, and `ang build` run by any other revision builds the
// pinned one once into a cache and hands the build over to it. A checkout of
// ANG on another branch or commit then never blocks a project, and nobody has
// to keep a worktree at the pinned commit by hand.
//
// The cache holds a bare mirror of ANG and one binary per revision:
//
//	<cache>/ang/git                      bare mirror, filled from ANG_SRC, the
//	                                     checkout this binary was built from,
//	                                     and the repository= URL of ang.lock
//	<cache>/ang/toolchains/<sha>/ang     the binary built from <sha>
//
// <cache> is $XDG_CACHE_HOME, else the user cache directory of the OS.
// A revision is built from a detached worktree of the mirror, so the binary
// carries vcs.revision like one built from a clean checkout (and passes the
// ang.lock check of revisions that predate the toolchain), and also gets the
// revision through -ldflags in buildRevision.
const (
	// toolchainEnv selects the binary: "auto" (default) runs the revision
	// ang.lock pins, "local" always runs the binary that was invoked.
	toolchainEnv = "ANG_TOOLCHAIN"
	// toolchainActiveEnv is set for the binary a build was handed over to, so
	// it never hands over again.
	toolchainActiveEnv = "ANG_TOOLCHAIN_ACTIVE"
	// angSrcEnv names a local ANG clone to take revisions from before the
	// network is tried.
	angSrcEnv = "ANG_SRC"
)

// buildRevision is set by `-ldflags "-X main.buildRevision=<sha>"`; a binary
// built without VCS information (an exported tree, -buildvcs=false) still
// knows its revision through it.
var buildRevision string

// toolchainStaleLock is how old a lock may get before another build takes it
// over: building a revision takes well under a minute.
const toolchainStaleLock = 15 * time.Minute

type toolchainCache struct {
	Root string // <cache>/ang
}

func defaultToolchainCache() (toolchainCache, error) {
	base := strings.TrimSpace(os.Getenv("XDG_CACHE_HOME"))
	if base == "" {
		dir, err := os.UserCacheDir()
		if err != nil {
			return toolchainCache{}, fmt.Errorf("no cache directory for ang toolchains (set XDG_CACHE_HOME): %w", err)
		}
		base = dir
	}
	return toolchainCache{Root: filepath.Join(base, "ang")}, nil
}

func (c toolchainCache) mirror() string { return filepath.Join(c.Root, "git") }

func (c toolchainCache) toolchainsDir() string { return filepath.Join(c.Root, "toolchains") }

func (c toolchainCache) binary(commit string) string {
	name := "ang"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(c.toolchainsDir(), commit, name)
}

// cachedBinary returns the binary of a full revision if it is built.
func (c toolchainCache) cachedBinary(commit string) (string, bool) {
	if !isFullRevision(commit) {
		return "", false
	}
	path := c.binary(commit)
	info, err := os.Stat(path)
	return path, err == nil && info.Mode().IsRegular()
}

// toolchainSource is a place the mirror can fetch a revision from.
type toolchainSource struct {
	Label string // shown in messages
	URL   string // git URL or local path
}

// toolchainSources lists where a revision is looked for, local clones first.
func toolchainSources(pin angLockPin) []toolchainSource {
	var sources []toolchainSource
	seen := map[string]bool{}
	add := func(label, url string) {
		url = strings.TrimSpace(url)
		if url == "" || seen[url] {
			return
		}
		seen[url] = true
		sources = append(sources, toolchainSource{Label: label, URL: url})
	}
	if src := os.Getenv(angSrcEnv); strings.TrimSpace(src) != "" {
		add(angSrcEnv+"="+src, src)
	}
	if src := toolchainSelfCheckout(); src != "" {
		add("the checkout this binary was built from ("+src+")", src)
	}
	add("ang.lock repository "+pin.Repository, pin.Repository)
	return sources
}

// toolchainSelfCheckout is replaced in tests, whose binary was built from the
// ANG checkout itself.
var toolchainSelfCheckout = runningBinarySourceCheckout

// runningBinarySourceCheckout is the ANG checkout the running binary was built
// from, when it was built without -trimpath and the checkout still exists.
func runningBinarySourceCheckout() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok || !filepath.IsAbs(file) {
		return ""
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(file))) // <root>/cmd/ang/toolchain.go
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return ""
	}
	if _, err := os.Stat(filepath.Join(root, "cmd", "ang")); err != nil {
		return ""
	}
	return root
}

// ensureToolchain returns the binary of the pinned revision, fetching and
// building it first if the cache does not have it. Progress goes to log.
func ensureToolchain(cache toolchainCache, pin angLockPin, log io.Writer) (string, error) {
	if path, ok := cache.cachedBinary(pin.Commit); ok {
		return path, nil
	}
	if err := os.MkdirAll(cache.toolchainsDir(), 0o755); err != nil {
		return "", err
	}
	unlock, err := lockToolchainCache(cache, log)
	if err != nil {
		return "", err
	}
	defer unlock()

	commit, err := resolveToolchainCommit(cache, pin, log)
	if err != nil {
		return "", err
	}
	if path, ok := cache.cachedBinary(commit); ok {
		return path, nil // built while this build waited for the lock
	}
	fmt.Fprintf(log, "ang toolchain: building %s into %s (once per revision)\n", shortRevision(commit), filepath.Dir(cache.binary(commit)))
	started := time.Now()
	if err := buildToolchain(cache, commit); err != nil {
		return "", err
	}
	fmt.Fprintf(log, "ang toolchain: built %s in %s\n", shortRevision(commit), time.Since(started).Round(100*time.Millisecond))
	return cache.binary(commit), nil
}

// resolveToolchainCommit makes sure the mirror has the pinned commit and
// returns its full hash.
func resolveToolchainCommit(cache toolchainCache, pin angLockPin, log io.Writer) (string, error) {
	mirror := cache.mirror()
	if _, err := os.Stat(filepath.Join(mirror, "HEAD")); err != nil {
		if err := os.MkdirAll(filepath.Dir(mirror), 0o755); err != nil {
			return "", err
		}
		if _, err := runToolchainGit("", "init", "--quiet", "--bare", mirror); err != nil {
			return "", fmt.Errorf("create the ang mirror at %s: %w", mirror, err)
		}
	}
	if commit, ok := mirrorCommit(mirror, pin.Commit); ok {
		return commit, nil
	}
	var tried []string
	for _, src := range toolchainSources(pin) {
		fmt.Fprintf(log, "ang toolchain: fetching %s from %s\n", shortRevision(pin.Commit), src.Label)
		err := fetchIntoMirror(mirror, src, pin.Commit)
		if commit, ok := mirrorCommit(mirror, pin.Commit); ok {
			return commit, nil
		}
		reason := "commit not found there"
		if err != nil {
			reason = firstLine(err.Error())
		}
		tried = append(tried, fmt.Sprintf("%s: %s", src.Label, reason))
	}
	if len(tried) == 0 {
		tried = append(tried, "no source: ang.lock has no repository= and "+angSrcEnv+" is not set")
	}
	return "", fmt.Errorf("commit %s pinned in %s is in none of the places tried:\n  - %s\nSet %s to an ANG clone that has it, or check the repository= line and your network",
		pin.Commit, angLockFileName, strings.Join(tried, "\n  - "), angSrcEnv)
}

func mirrorCommit(mirror, commit string) (string, bool) {
	out, err := runToolchainGit(mirror, "rev-parse", "--verify", "--quiet", commit+"^{commit}")
	full := strings.TrimSpace(out)
	if err != nil || !isFullRevision(full) {
		return "", false
	}
	return full, true
}

// fetchIntoMirror fetches every branch and tag of src (and HEAD, which may be
// a detached commit) into a namespace of its own, and the commit itself when
// the pin is a full hash and src is remote (GitHub serves any reachable one).
func fetchIntoMirror(mirror string, src toolchainSource, commit string) error {
	sum := sha256.Sum256([]byte(src.URL))
	ns := "refs/ang-sources/" + hex.EncodeToString(sum[:6])
	if isFullRevision(commit) && !isLocalGitSource(src.URL) {
		if _, err := runToolchainGit(mirror, "fetch", "--quiet", "--no-tags", src.URL, "+"+commit+":"+ns+"/pinned/"+commit); err == nil {
			return nil
		}
	}
	_, err := runToolchainGit(mirror, "fetch", "--quiet", "--no-tags", src.URL,
		"+HEAD:"+ns+"/HEAD",
		"+refs/heads/*:"+ns+"/heads/*",
		"+refs/tags/*:"+ns+"/tags/*",
	)
	return err
}

func isLocalGitSource(url string) bool {
	if strings.HasPrefix(url, "file://") {
		return true
	}
	if strings.Contains(url, "://") || strings.Contains(url, "@") {
		return false
	}
	_, err := os.Stat(url)
	return err == nil
}

// buildToolchain builds one revision from a detached worktree of the mirror
// and moves the binary into place in one rename, so a reader never sees a
// partial file.
func buildToolchain(cache toolchainCache, commit string) error {
	goBin, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("building ang %s needs the go command: %w", shortRevision(commit), err)
	}
	work, err := os.MkdirTemp(cache.toolchainsDir(), "build-"+shortRevision(commit)+"-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	src := filepath.Join(work, "src")
	mirror := cache.mirror()
	if _, err := runToolchainGit(mirror, "worktree", "add", "--quiet", "--detach", src, commit); err != nil {
		return fmt.Errorf("check out %s: %w", shortRevision(commit), err)
	}
	defer func() {
		if _, err := runToolchainGit(mirror, "worktree", "remove", "--force", src); err != nil {
			_ = os.RemoveAll(src)
			_, _ = runToolchainGit(mirror, "worktree", "prune") // the cache's own mirror only
		}
	}()

	tmpBin := filepath.Join(work, filepath.Base(cache.binary(commit)))
	cmd := exec.Command(goBin, "build", "-o", tmpBin, "-ldflags", "-X main.buildRevision="+commit, "./cmd/ang")
	cmd.Dir = src
	cmd.Env = append(toolchainGitEnv(), "GOWORK=off")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build of ang %s failed: %w\n%s", shortRevision(commit), err, strings.TrimSpace(out.String()))
	}
	final := cache.binary(commit)
	if err := os.MkdirAll(filepath.Dir(final), 0o755); err != nil {
		return err
	}
	return os.Rename(tmpBin, final)
}

// lockToolchainCache serializes fetches and builds of all processes sharing
// the cache (parallel `make generate` runs). The lock is a directory; one left
// behind by a killed process is taken over once it is stale.
func lockToolchainCache(cache toolchainCache, log io.Writer) (func(), error) {
	lock := filepath.Join(cache.Root, "lock")
	deadline := time.Now().Add(toolchainStaleLock + time.Minute)
	announced := false
	for {
		err := os.Mkdir(lock, 0o755)
		if err == nil {
			return func() { _ = os.Remove(lock) }, nil
		}
		if !os.IsExist(err) {
			return nil, fmt.Errorf("lock the ang toolchain cache: %w", err)
		}
		if info, statErr := os.Stat(lock); statErr == nil && time.Since(info.ModTime()) > toolchainStaleLock {
			_ = os.Remove(lock)
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("the ang toolchain cache stayed locked (%s); remove it if no build is running", lock)
		}
		if !announced {
			fmt.Fprintf(log, "ang toolchain: waiting for another build of a toolchain (%s)\n", lock)
			announced = true
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// toolchainGitEnv is the environment for git and go run on the cache: git
// hooks export GIT_DIR and friends, which would point every git command at the
// repository whose hook is running.
func toolchainGitEnv() []string {
	drop := map[string]bool{
		"GIT_DIR": true, "GIT_WORK_TREE": true, "GIT_INDEX_FILE": true, "GIT_COMMON_DIR": true,
		"GIT_OBJECT_DIRECTORY": true, "GIT_PREFIX": true, "GIT_NAMESPACE": true,
	}
	var env []string
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		if drop[key] {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "GIT_TERMINAL_PROMPT=0")
}

func runToolchainGit(dir string, args ...string) (string, error) {
	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}
	cmd := exec.Command("git", args...)
	cmd.Env = toolchainGitEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return stdout.String(), fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return stdout.String(), nil
}

func isFullRevision(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}

// toolchainHandover is what `ang build` does about ang.lock before it starts.
type toolchainHandover struct {
	Switch  bool
	Pin     angLockPin
	Reason  string // why the invoked binary runs, when it does
	Project string
}

// decideToolchain decides whether a build hands over to the pinned toolchain.
// It never does when ANG_TOOLCHAIN=local, when this binary already is a
// handed-over toolchain, when --allow-lock-mismatch asks for this binary, when
// the project has no ang.lock (or a broken one, which the build reports), or
// when this binary's revision is unknown or already the pinned one.
func decideToolchain(args []string, getenv func(string) string, running compilerRevision) (toolchainHandover, error) {
	var h toolchainHandover
	switch mode := strings.ToLower(strings.TrimSpace(getenv(toolchainEnv))); mode {
	case "", "auto":
	case "local":
		h.Reason = toolchainEnv + "=local"
		return h, nil
	default:
		return h, fmt.Errorf("%s=%q: expected auto or local", toolchainEnv, getenv(toolchainEnv))
	}
	if active := getenv(toolchainActiveEnv); active != "" {
		h.Reason = "already the pinned toolchain " + shortRevision(active)
		return h, nil
	}
	if buildArgsAllowLockMismatch(args) {
		h.Reason = "--allow-lock-mismatch"
		return h, nil
	}
	h.Project = buildArgsProject(args)
	pin, pinned, err := readANGLock(h.Project)
	if err != nil || !pinned {
		h.Reason = "no usable " + angLockFileName
		return h, nil
	}
	h.Pin = pin
	if running.Revision == "" {
		h.Reason = "this binary has no revision to compare"
		return h, nil
	}
	if sameRevision(pin.Commit, running.Revision) {
		h.Reason = "this binary is the pinned revision"
		return h, nil
	}
	h.Switch = true
	return h, nil
}

func buildArgsAllowLockMismatch(args []string) bool {
	for _, arg := range args {
		name, value, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		if strings.HasPrefix(arg, "-") && name == "allow-lock-mismatch" && (!hasValue || value == "true" || value == "1") {
			return true
		}
	}
	return false
}

// buildArgsProject finds the project of `ang build` the way runBuild does: the
// --project flag, else the first argument when it is not a flag, else ".".
func buildArgsProject(args []string) string {
	for i, arg := range args {
		name, value, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		if !strings.HasPrefix(arg, "-") || name != "project" {
			continue
		}
		if hasValue {
			if strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		} else if i+1 < len(args) && strings.TrimSpace(args[i+1]) != "" {
			return strings.TrimSpace(args[i+1])
		}
	}
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return args[0]
	}
	return "."
}

// handOverToPinnedToolchain runs before `ang build`: when the project pins
// another revision, it builds that one if needed and replaces this process
// with it. It returns only when this binary should do the build itself.
func handOverToPinnedToolchain(args []string) {
	h, err := decideToolchain(args, os.Getenv, runningCompilerRevision())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Build FAILED: %v\n", err)
		os.Exit(1)
	}
	if !h.Switch {
		return
	}
	running := runningCompilerRevision()
	fmt.Fprintf(os.Stderr, "ang: %s pins %s, this binary is %s; running the pinned toolchain (%s=local keeps this binary)\n",
		filepath.Join(h.Project, angLockFileName), shortRevision(h.Pin.Commit), shortRevision(running.Revision), toolchainEnv)
	cache, err := defaultToolchainCache()
	if err == nil {
		var bin string
		bin, err = ensureToolchain(cache, h.Pin, os.Stderr)
		if err == nil {
			env := append(os.Environ(), toolchainActiveEnv+"="+h.Pin.Commit)
			err = execToolchain(bin, append([]string{"build"}, args...), env)
		}
	}
	fmt.Fprintf(os.Stderr, "Build FAILED: cannot run the ang toolchain pinned in %s: %v\n", angLockFileName, err)
	fmt.Fprintf(os.Stderr, "To build with this binary anyway: %s=local ang build --allow-lock-mismatch (do not commit that output without updating %s)\n", toolchainEnv, angLockFileName)
	os.Exit(1)
}

// runToolchain implements `ang toolchain`.
func runToolchain(args []string) {
	sub := "status"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		switch args[0] {
		case "status", "install", "path", "list":
			sub, args = args[0], args[1:]
		}
	}
	fs := flag.NewFlagSet("toolchain "+sub, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  ang toolchain [status] [project]   the pinned revision, this binary's, and what ang build runs")
		fmt.Fprintln(os.Stderr, "  ang toolchain install [project]    fetch and build the revision ang.lock pins; prints the binary")
		fmt.Fprintln(os.Stderr, "  ang toolchain path [project]       as install, printing only the binary path (for scripts)")
		fmt.Fprintln(os.Stderr, "  ang toolchain list                 revisions built in the cache")
		fmt.Fprintln(os.Stderr, "Environment: ANG_TOOLCHAIN=auto|local, ANG_SRC=<local ANG clone>, XDG_CACHE_HOME")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		os.Exit(2)
	}
	project := "."
	if fs.NArg() > 0 {
		project = fs.Arg(0)
	}
	cache, err := defaultToolchainCache()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Toolchain FAILED: %v\n", err)
		os.Exit(1)
	}
	if sub == "list" {
		for _, line := range listToolchains(cache) {
			fmt.Println(line)
		}
		return
	}
	pin, pinned, err := readANGLock(project)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Toolchain FAILED: %v\n", err)
		os.Exit(1)
	}
	if !pinned {
		if sub == "status" {
			fmt.Printf("%s has no %s: ang build runs this binary.\n", project, angLockFileName)
			return
		}
		fmt.Fprintf(os.Stderr, "Toolchain FAILED: %s has no %s to take the revision from\n", project, angLockFileName)
		os.Exit(1)
	}
	switch sub {
	case "status":
		printToolchainStatus(cache, project, pin)
	case "install", "path":
		log := io.Writer(os.Stderr)
		bin, err := ensureToolchain(cache, pin, log)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Toolchain FAILED: %v\n", err)
			os.Exit(1)
		}
		if sub == "install" {
			fmt.Printf("ang %s: %s\n", shortRevision(pin.Commit), bin)
		} else {
			fmt.Println(bin)
		}
	}
}

func printToolchainStatus(cache toolchainCache, project string, pin angLockPin) {
	running := runningCompilerRevision()
	fmt.Printf("pinned:   %s (%s)\n", pin.Commit, filepath.Join(project, angLockFileName))
	if pin.Repository != "" {
		fmt.Printf("from:     %s\n", pin.Repository)
	}
	this := running.Revision
	if this == "" {
		this = "unknown (built without VCS information)"
	} else if running.Modified {
		this += " with uncommitted changes"
	}
	fmt.Printf("this:     %s\n", this)
	if path, ok := cache.cachedBinary(pin.Commit); ok {
		fmt.Printf("cached:   %s\n", path)
	} else {
		fmt.Printf("cached:   no (ang toolchain install builds it into %s)\n", cache.toolchainsDir())
	}
	h, err := decideToolchain([]string{project}, os.Getenv, running)
	switch {
	case err != nil:
		fmt.Printf("ang build: fails: %v\n", err)
	case h.Switch:
		fmt.Printf("ang build: runs the pinned toolchain (%s=local keeps this binary)\n", toolchainEnv)
	default:
		fmt.Printf("ang build: runs this binary (%s)\n", h.Reason)
	}
}

func listToolchains(cache toolchainCache) []string {
	entries, err := os.ReadDir(cache.toolchainsDir())
	if err != nil {
		return nil
	}
	var lines []string
	for _, entry := range entries {
		if !entry.IsDir() || !isFullRevision(entry.Name()) {
			continue
		}
		if path, ok := cache.cachedBinary(entry.Name()); ok {
			lines = append(lines, entry.Name()+"  "+path)
		}
	}
	sort.Strings(lines)
	return lines
}
