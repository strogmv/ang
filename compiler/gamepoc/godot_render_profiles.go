package gamepoc

import "fmt"

// applyGodotRenderProfiles changes only GodotSettings (excluded from world.json).
// Keeping the projection outside world preserves server/save identities when a
// player-facing rendering preset changes. CUE owns the values and constraints.
func applyGodotRenderProfiles(settings *GodotSettings, overrides map[string]any) error {
	if settings == nil {
		return fmt.Errorf("godot_render_profiles requires godot_settings")
	}
	profiles := make(map[string]any, len(settings.Profiles))
	for id, original := range settings.Profiles {
		profiles[id] = original
	}
	for id, value := range overrides {
		original, ok := settings.Profiles[id].(map[string]any)
		if !ok {
			return fmt.Errorf("godot_render_profiles: unknown profile %q", id)
		}
		patch, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("godot_render_profiles: profile %q must be an object", id)
		}
		profiles[id] = mergeRenderProfile(original, patch)
	}
	settings.Profiles = profiles
	return nil
}

func mergeRenderProfile(original, patch map[string]any) map[string]any {
	merged := make(map[string]any, len(original)+len(patch))
	for key, value := range original {
		merged[key] = value
	}
	for key, value := range patch {
		left, leftOK := original[key].(map[string]any)
		right, rightOK := value.(map[string]any)
		if leftOK && rightOK {
			merged[key] = mergeRenderProfile(left, right)
		} else {
			merged[key] = value
		}
	}
	return merged
}
