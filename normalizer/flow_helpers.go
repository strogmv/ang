package normalizer

func flowUsesObjectStorage(steps []FlowStep) bool {
	for _, step := range steps {
		switch step.Action {
		case "storage.Upload", "storage.Download", "storage.GetURL", "storage.Delete", "storage.List":
			return true
		}
		for _, key := range []string{"_do", "_ifNew", "_ifExists", "_then", "_else", "_default", "_catch", "_fallback", "_onTimeout", "_onMissing", "_onMismatch"} {
			if nested, ok := step.Args[key].([]FlowStep); ok && flowUsesObjectStorage(nested) {
				return true
			}
		}
		if cases, ok := step.Args["_cases"].(map[string][]FlowStep); ok {
			for _, nested := range cases {
				if flowUsesObjectStorage(nested) {
					return true
				}
			}
		}
		if branches, ok := step.Args["_branches"].(map[string][]FlowStep); ok {
			for _, nested := range branches {
				if flowUsesObjectStorage(nested) {
					return true
				}
			}
		}
	}
	return false
}

type FlowWarning struct {
	Op           string
	Step         int
	Action       string
	Message      string
	Code         string
	Severity     string
	Hint         string
	File         string
	Line         int
	Column       int
	CUEPath      string
	SuggestedFix []Fix
}
