package godot

import (
	"github.com/strogmv/ang/compiler/locationsdk"
	"testing"
)

func TestDomeWallPolicyCannotOmitFloorEnvelope(t *testing.T) {
	f := &Finishing{Architecture: &Architecture{Setting: "dome_interior"}}
	l := locationsdk.Layout{ID: "next.hall"}
	if ValidateInteriorAuthoring(f, l) == nil {
		t.Fatal("next dome hall omitted its interior check")
	}
	l.Setting = "dome_interior"
	l.Interior = &locationsdk.InteriorEnvelope{}
	if err := ValidateInteriorAuthoring(f, l); err != nil {
		t.Fatal(err)
	}
	l.Interior = nil
	f.Architecture.OpenPlanReason = "Open station concourse faces an authored canyon, with no enclosing wall ring."
	if err := ValidateInteriorAuthoring(f, l); err != nil {
		t.Fatal(err)
	}
}
