package godot

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The game's CUE definition is the source: this integration never reimplements
// the rail platform or teaches the generator a second location name.
func TestRailPlatformSecondLayoutByData(t *testing.T) {
	root := os.Getenv("ANG_GAME_ROOT")
	if root == "" {
		t.Skip("set ANG_GAME_ROOT for the game CUE rail-platform integration")
	}
	cue, err := exec.LookPath("cue")
	if err != nil {
		t.Fatal(err)
	}
	for _, span := range []int{64, 128} {
		expression := fmt.Sprintf(`#RailPlatform & {id:"other.station",x_m:20,length_m:%d,edge_y_m:100,floor_m:7,track_center_y_m:101.5,beam:{model_ref:"same.beam",length_m:64,top_below_floor_m:1.2},tunnel_model_ref:"same.tunnel",wall_role:"concrete.bronze",safety_role:"ochre"}`, span)
		cmd := exec.Command(cue, "export", ".", "-e", expression)
		cmd.Dir = filepath.Join(root, "content/world/godot_finishing")
		raw, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("second layout CUE: %v\n%s", err, raw)
		}
		var result struct {
			Extras []struct {
				InstanceID string     `json:"instance_id"`
				Position   [3]float64 `json:"position_plan_m"`
			}
			Draws []struct {
				Size [3]float64 `json:"size_m"`
			}
		}
		if err = json.Unmarshal(raw, &result); err != nil {
			t.Fatal(err)
		}
		if len(result.Extras) != span/64+2 || len(result.Draws) != 3 {
			t.Fatalf("data declaration lost rail components: %s", raw)
		}
		if result.Extras[len(result.Extras)-1].Position != ([3]float64{20 + float64(span), 7, 101.5}) || result.Draws[0].Size[0] != float64(span) {
			t.Fatal("second layout retained hub pose or size")
		}
	}
}
