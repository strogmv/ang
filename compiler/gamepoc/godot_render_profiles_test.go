package gamepoc

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestGodotRenderProfilesDoNotChangeServerIdentity(t *testing.T) {
	original := map[string]any{"render": map[string]any{"msaa_3d": float64(2), "scaling_3d_scale": 1.0}, "actor_mode": "near"}
	c := Compiled{ContentHash: "unchanged-server-identity", GodotSettings: &GodotSettings{Profiles: map[string]any{"full": original}}}
	before, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	patch := map[string]any{"full": map[string]any{"render": map[string]any{"scaling_3d_scale": 0.67}}}
	if err := applyGodotRenderProfiles(c.GodotSettings, patch); err != nil {
		t.Fatal(err)
	}
	after, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("Rendering changed server serialization")
	}
	got := c.GodotSettings.Profiles["full"].(map[string]any)
	render := got["render"].(map[string]any)
	if render["scaling_3d_scale"] != 0.67 || render["msaa_3d"] != float64(2) || got["actor_mode"] != "near" {
		t.Fatalf("Partial override lost inherited settings: %#v", got)
	}
	if original["render"].(map[string]any)["scaling_3d_scale"] != 1.0 {
		t.Fatal("Source profile mutated")
	}
}

func TestGodotRenderProfilesRejectUnknownOrMalformedProfiles(t *testing.T) {
	if err := applyGodotRenderProfiles(nil, map[string]any{}); err == nil {
		t.Fatal("Accepted absent settings")
	}
	for _, patch := range []map[string]any{{"unknown": map[string]any{}}, {"full": true}} {
		settings := &GodotSettings{Profiles: map[string]any{"full": map[string]any{}}}
		if err := applyGodotRenderProfiles(settings, patch); err == nil {
			t.Fatalf("Accepted %#v", patch)
		}
	}
}
