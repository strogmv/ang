package gamepoc

import (
	"fmt"
	"math"
	"sort"
)

// Distance steps (MetaHuman people, art-source/characters/metahuman-v1; owner 15.11, doc 09: more triangles on the face and
// hands where the eye looks, the crowd at 12 m lighter). A look's model may carry one skinned mesh per step on its one skeleton
// and atlas: near (head LOD2, hands with full fingers) and crowd (head LOD3, simple hands); the record (look.json) names each
// step's mesh, its triangles and the build's budget. The camera is orthographic, so the step follows the frame's height: the
// client draws near while the frame is at most NearUpToFrameM high and crowd beyond, with HysteresisM of slack on the way
// back (a wheel turn at the border does not flicker the step).

// LookSteps: when the near step is drawn (world.art.looks[].steps).
type LookSteps struct {
	NearUpToFrameM float64 `json:"near_up_to_frame_m"`
	HysteresisM    float64 `json:"hysteresis_m"`
}

// lookRecordStep: one step of a model in its record: the mesh (a node of the GLB), its triangles and the build's budget.
type lookRecordStep struct {
	Mesh      string `json:"mesh"`
	Triangles int64  `json:"triangles"`
	Budget    int64  `json:"budget"`
}

var (
	// The steps a model with distance steps has, nearest first.
	lookStepNames = []string{"near", "crowd"}
	// The frame (orthographic camera height, metres) the near step may be drawn up to: the game's wheel is 9–16 m, close-ups 1–5 m.
	lookStepMinFrameM, lookStepMaxFrameM = 1.0, 16.0
	lookStepMaxHysteresisM               = 1.0
)

// checkLookSteps: a model with steps in its record is drawn with them (the look names when near is drawn) and a look names
// steps only for such a model; the record has exactly near and crowd, each a named mesh within its budget, near the richer.
func checkLookSteps(key string, l Look, steps map[string]lookRecordStep, record string) error {
	if len(steps) == 0 {
		if l.Steps != nil {
			return fmt.Errorf("art.looks %s: steps need a model with distance steps (near, crowd) in its record %s", key, record)
		}
		return nil
	}
	if l.Steps == nil {
		return fmt.Errorf("art.looks %s: the model has distance steps in %s; name the frame the near step is drawn up to (steps)", key, record)
	}
	names := make([]string, 0, len(steps))
	for n := range steps {
		names = append(names, n)
	}
	sort.Strings(names)
	if len(names) != len(lookStepNames) || steps["near"] == (lookRecordStep{}) || steps["crowd"] == (lookRecordStep{}) {
		return fmt.Errorf("art.looks %s: record %s: the steps must be exactly %v, not %v", key, record, lookStepNames, names)
	}
	meshes := map[string]bool{}
	for _, n := range lookStepNames {
		s := steps[n]
		if !lookModel.MatchString(lowerASCII(s.Mesh)) || meshes[s.Mesh] {
			return fmt.Errorf("art.looks %s: record %s: step %s needs its own mesh name, not %q", key, record, n, s.Mesh)
		}
		meshes[s.Mesh] = true
		if s.Triangles <= 0 || s.Budget <= 0 || s.Triangles > s.Budget {
			return fmt.Errorf("art.looks %s: record %s: step %s has %d triangles for a budget of %d", key, record, n, s.Triangles, s.Budget)
		}
	}
	if steps["near"].Triangles <= steps["crowd"].Triangles {
		return fmt.Errorf("art.looks %s: record %s: the near step (%d triangles) must be richer than the crowd step (%d)", key, record,
			steps["near"].Triangles, steps["crowd"].Triangles)
	}
	f, h := l.Steps.NearUpToFrameM, l.Steps.HysteresisM
	if math.IsNaN(f) || f < lookStepMinFrameM || f > lookStepMaxFrameM {
		return fmt.Errorf("art.looks %s: steps: near_up_to_frame_m %g must be within %g..%g m", key, f, lookStepMinFrameM, lookStepMaxFrameM)
	}
	if math.IsNaN(h) || h < 0 || h >= lookStepMaxHysteresisM {
		return fmt.Errorf("art.looks %s: steps: hysteresis_m %g must be within 0..%g m", key, h, lookStepMaxHysteresisM)
	}
	return nil
}

// lowerASCII: a mesh name of the GLB (Bronze_polina_mh) checked as a plain name.
func lowerASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}
