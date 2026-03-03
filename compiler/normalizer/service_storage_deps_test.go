package normalizer

import "testing"

func TestFlowUsesObjectStorage(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		steps []FlowStep
		want  bool
	}{
		{
			name:  "direct_storage_list",
			steps: []FlowStep{{Action: "storage.List", Args: map[string]any{"prefix": `"p/"`, "output": "keys"}}},
			want:  true,
		},
		{
			name: "nested_in_flow_if",
			steps: []FlowStep{{
				Action: "flow.If",
				Args: map[string]any{
					"_then": []FlowStep{{Action: "storage.Upload", Args: map[string]any{"key": `"k"`, "data": `"v"`}}},
				},
			}},
			want: true,
		},
		{
			name: "nested_in_cases",
			steps: []FlowStep{{
				Action: "flow.Switch",
				Args: map[string]any{
					"_cases": map[string][]FlowStep{
						"ok": {{Action: "storage.Delete", Args: map[string]any{"key": `"k"`}}},
					},
				},
			}},
			want: true,
		},
		{
			name:  "no_storage",
			steps: []FlowStep{{Action: "repo.Find", Args: map[string]any{"source": "Project", "input": "req.ID", "output": "project"}}},
			want:  false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := flowUsesObjectStorage(tc.steps)
			if got != tc.want {
				t.Fatalf("flowUsesObjectStorage()=%v want %v", got, tc.want)
			}
		})
	}
}
