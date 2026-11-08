package blender

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBlendPoseAuthorityRejectsHandAndStaleSources(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "author.blend")
	raw := []byte("source Blender bytes")
	if err := os.WriteFile(source, raw, 0600); err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(raw)
	geometry := filepath.Join(dir, "generated.cue")
	text := fmt.Sprintf("// Generated from the ID-tagged Blender source by ANG; do not hand-edit.\npackage location\ngenerated_geometry:{source_kind:\"blend\",source_sha256:%q,floor:[]}\n", hex.EncodeToString(h[:]))
	if err := os.WriteFile(geometry, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	profile := PoseAuthority{PoseSource: "blend", GeneratedCUE: "generated.cue", Blend: "author.blend"}
	if err := ValidatePoseAuthority(profile, dir, ""); err != nil {
		t.Fatal(err)
	}
	hand := filepath.Join(dir, "poses.cue")
	if err := os.WriteFile(hand, []byte("package location\ngenerated_geometry:{floor:[]}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePoseAuthority(profile, dir, ""); err == nil || !strings.Contains(err.Error(), "poses.cue") {
		t.Fatalf("hand pose owner accepted: %v", err)
	}
	if err := os.Rename(hand, hand+".preserved"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("changed geometry"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePoseAuthority(profile, dir, ""); err == nil || !strings.Contains(err.Error(), "SHA changed") {
		t.Fatalf("stale generated geometry accepted: %v", err)
	}
	if err := ValidatePoseAuthority(PoseAuthority{PoseSource: "unexpected"}, dir, ""); err == nil {
		t.Fatal("unknown authority accepted")
	}
}

func TestGeneratedGeometryRenderingOwnershipIsReadOnlyForBlend(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "geometry.cue")
	data := "package geometry\ngenerated_geometry:{replacements:[{surface_id:\"prop.cell.1.2.0\",instance_id:\"prop\",reason:\"Same source mesh renders this collider\"}]}\n"
	if err := os.WriteFile(file, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	replacements, err := GeneratedReplacements(PoseAuthority{PoseSource: "blend", GeneratedCUE: "geometry.cue"}, dir)
	if err != nil || len(replacements) != 1 || replacements[0].InstanceID != "prop" {
		t.Fatalf("generated render owner lost: %+v %v", replacements, err)
	}
	if list, err := GeneratedReplacements(PoseAuthority{PoseSource: "cue", GeneratedCUE: "missing.cue"}, dir); err != nil || len(list) != 0 {
		t.Fatal("CUE authority consumed Blender geometry metadata")
	}
}
