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

// GenerateTenderReport creates a PDF report from report-like data.
// Keep signature generic so runtime compiles even when tender-specific DTO is absent.
func (g *Generator) GenerateTenderReport(data any) ([]byte, error) {
	_ = data
	return nil, fmt.Errorf("report rendering is not configured for this project")
}
