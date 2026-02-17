package plan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func WritePlan(path string, p *BuildPlan) error {
	if p == nil {
		return fmt.Errorf("plan is nil")
	}
	if err := ValidatePlan(p); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

func ReadPlan(path string) (*BuildPlan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p BuildPlan
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	if err := ValidatePlan(&p); err != nil {
		return nil, err
	}
	return &p, nil
}
