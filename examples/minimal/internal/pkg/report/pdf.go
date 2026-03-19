package report

import (
	"fmt"
)

// Generator generates PDF reports.
type Generator struct{}

// NewGenerator creates a new report generator.
func NewGenerator() *Generator {
	return &Generator{}
}

// GenerateReport creates a PDF report from report-like data.
// Keep signature generic so runtime compiles even when a project-specific DTO is absent.
func (g *Generator) GenerateReport(data any) ([]byte, error) {
	_ = data
	return nil, fmt.Errorf("report rendering is not configured for this project")
}
