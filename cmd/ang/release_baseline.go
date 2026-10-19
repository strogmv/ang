package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
)

// releaseRefNone turns the release baseline off: the gates compare with the
// previous generation only.
const releaseRefNone = "none"

// releaseBaseline is the generation callers already have: the generated files
// as committed at a git revision (default HEAD; --release-ref or
// ANG_RELEASE_REF name another, e.g. main or a release tag). The build gates
// refuse only what changes something that revision has; an operation, field,
// or finder added since then and changed or dropped again was never released.
type releaseBaseline struct {
	ref string // "" = off
}

func resolveReleaseBaseline(flagValue string) releaseBaseline {
	ref := strings.TrimSpace(flagValue)
	if ref == "" {
		ref = strings.TrimSpace(os.Getenv("ANG_RELEASE_REF"))
	}
	if ref == "" {
		ref = "HEAD"
	}
	if ref == releaseRefNone {
		return releaseBaseline{}
	}
	return releaseBaseline{ref: ref}
}

// git runs git in dir with the environment of a plain shell (no GIT_DIR a
// hook exported).
func (b releaseBaseline) git(dir string, stdin io.Reader, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = toolchainGitEnv()
	cmd.Stdin = stdin
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return out, fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// prefix is dir's path inside its repository at the baseline revision, or
// ok=false when there is no baseline (off, not a git work tree, unknown ref).
func (b releaseBaseline) prefix(dir string) (string, bool) {
	if b.ref == "" {
		return "", false
	}
	if _, err := os.Stat(dir); err != nil {
		return "", false
	}
	if _, err := b.git(dir, nil, "rev-parse", "--verify", "--quiet", b.ref+"^{commit}"); err != nil {
		return "", false
	}
	out, err := b.git(dir, nil, "rev-parse", "--show-prefix")
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

// file reads dir/rel at the baseline revision; present=false when the
// revision does not have it. ok=false when there is no baseline.
func (b releaseBaseline) file(dir, rel string) (data []byte, present, ok bool) {
	prefix, ok := b.prefix(dir)
	if !ok {
		return nil, false, false
	}
	spec := b.ref + ":" + path.Join(prefix, rel)
	if _, err := b.git(dir, nil, "cat-file", "-e", spec); err != nil {
		return nil, false, true
	}
	data, err := b.git(dir, nil, "cat-file", "blob", spec)
	if err != nil {
		return nil, false, false
	}
	return data, true, true
}

// tree reads the files directly under dir/sub at the baseline revision, by
// base name. ok=false when there is no baseline.
func (b releaseBaseline) tree(dir, sub string) (map[string][]byte, bool) {
	prefix, ok := b.prefix(dir)
	if !ok {
		return nil, false
	}
	treePath := path.Join(prefix, sub)
	out, err := b.git(dir, nil, "ls-tree", "-z", "--full-tree", "--name-only", b.ref, treePath+"/")
	if err != nil {
		return nil, false
	}
	var paths []string
	for _, name := range strings.Split(string(out), "\x00") {
		if name != "" {
			paths = append(paths, name)
		}
	}
	files := map[string][]byte{}
	if len(paths) == 0 {
		return files, true
	}
	var request bytes.Buffer
	for _, p := range paths {
		request.WriteString(b.ref + ":" + p + "\n")
	}
	out, err = b.git(dir, &request, "cat-file", "--batch")
	if err != nil {
		return nil, false
	}
	reader := bufio.NewReader(bytes.NewReader(out))
	for _, p := range paths {
		header, err := reader.ReadString('\n')
		if err != nil {
			return nil, false
		}
		fields := strings.Fields(header)
		if len(fields) != 3 {
			continue // "<spec> missing"
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, false
		}
		content := make([]byte, size+1) // the content and its trailing newline
		if _, err := io.ReadFull(reader, content); err != nil {
			return nil, false
		}
		if fields[1] == "blob" {
			files[path.Base(p)] = content[:size]
		}
	}
	return files, true
}

// contractChangesSinceRelease keeps the changes of this build (sinceLast:
// previous generation → this one) that touch something the release has, and
// reports them as the release sees them (sinceRelease: release → this one).
func contractChangesSinceRelease(sinceLast, sinceRelease []contractChange) []contractChange {
	touched := map[string]bool{}
	for _, change := range sinceLast {
		touched[change.At] = true
	}
	var kept []contractChange
	for _, change := range sinceRelease {
		if touched[change.At] {
			kept = append(kept, change)
		}
	}
	return kept
}

// removalsSinceRelease keeps the removed repository methods (as listed by
// diffRepositoryRemovals) the release has.
func removalsSinceRelease(removed []string, released repositoryMethods) []string {
	var kept []string
	for _, item := range removed {
		name := strings.TrimPrefix(item, "port.")
		if iface, whole := strings.CutSuffix(name, " (the whole interface)"); whole {
			if _, ok := released[iface]; ok {
				kept = append(kept, item)
			}
			continue
		}
		iface, method, _ := strings.Cut(name, ".")
		if released[iface][method] {
			kept = append(kept, item)
		}
	}
	return kept
}
