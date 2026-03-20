package compiler

import (
	"crypto/sha256"
	"encoding/hex"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/strogmv/ang-ir/ir"
)

const buildFingerprintSchema = "compiler-fingerprint/v1"

// BuildFingerprint returns a stable hash for the currently running ANG compiler binary.
// It is used to invalidate stale artifact manifests when compiler internals change.
func BuildFingerprint() string {
	parts := []string{
		"schema=" + buildFingerprintSchema,
		"ang_version=" + strings.TrimSpace(Version),
		"schema_version=" + strings.TrimSpace(SchemaVersion),
		"ir_canonical_version=" + strings.TrimSpace(ir.CurrentVersion()),
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		parts = append(parts,
			"main_path="+strings.TrimSpace(bi.Main.Path),
		)
		// We intentionally exclude vcs.* and GOOS/GOARCH to keep the fingerprint
		// stable across developer machines and CI, ensuring that generated artifacts
		// (which include this hash) don't trigger unnecessary git diffs.
	} else {
		parts = append(parts, "build_info=unavailable")
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])
}
