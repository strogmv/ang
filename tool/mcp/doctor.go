package mcp

import (
	"encoding/json"

	"github.com/strogmv/ang/compiler/doctor"
)

func buildDoctorResponse(log string) map[string]any {
	resp := doctor.NewAnalyzer(".").Analyze(log)
	out := make(map[string]any)
	b, err := json.Marshal(resp)
	if err != nil {
		return map[string]any{"status": "Analyze FAILED", "error": err.Error()}
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return map[string]any{"status": "Analyze FAILED", "error": err.Error()}
	}
	return out
}
