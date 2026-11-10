package godot

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler/locationsdk"
)

// ValidateInteriorAuthoring links visual wall policy to the SDK floor check.
// An open concourse may explicitly explain why it has no enclosing wall ring;
// a closed dome hall cannot accidentally omit the coverage validation.
func ValidateInteriorAuthoring(f *Finishing, layout locationsdk.Layout) error {
	if f == nil || f.Architecture == nil || f.Architecture.Setting != "dome_interior" {
		return nil
	}
	if layout.Setting == "dome_interior" && layout.Interior != nil {
		return nil
	}
	if strings.TrimSpace(f.Architecture.OpenPlanReason) != "" {
		return nil
	}
	return fmt.Errorf("layout %s: dome interior requires SDK setting dome_interior and interior wall envelope/declared voids; an open concourse needs architecture.open_plan_reason", layout.ID)
}
