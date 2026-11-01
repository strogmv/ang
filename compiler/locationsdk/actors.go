package locationsdk

// LayoutActors is presentation only (demo-hub v3-people): how the people of this layout — the hero and every NPC — are
// drawn. Omitted or "sprite" keeps the character sheets. "humanoid" draws each actor as a rigged 3D Humanoid of the named
// cast; the cast resolves in the Unity visual registry (actor_casts: body prefab, Animator states, measured stride speed,
// tints), so a later artist / Mixamo body replaces the cast there without touching CUE. Go movement, occupancy, sight,
// saves and the structural hash never depend on it: the 3D body follows the interpolated Go position.
type LayoutActors struct {
	Presentation string `json:"presentation"`
	Cast         string `json:"cast,omitempty"`
}

// validateActors: sprite takes no cast; humanoid names a stable cast ID; anything else is rejected.
func validateActors(layout Layout) (*LayoutActors, *ValidationError) {
	a := layout.Actors
	if a == nil {
		return nil, nil
	}
	switch a.Presentation {
	case "sprite":
		if a.Cast != "" {
			return nil, issue("actors_presentation", "layout.actors.cast", "sprite actors take no humanoid cast", layout.ID)
		}
	case "humanoid":
		if !stableID.MatchString(a.Cast) {
			return nil, issue("actors_presentation", "layout.actors.cast", "humanoid actors name a stable cast ID resolved by the visual registry", layout.ID)
		}
	default:
		return nil, issue("actors_presentation", "layout.actors.presentation", "actors are drawn as sprite or humanoid", layout.ID)
	}
	copied := *a
	return &copied, nil
}
