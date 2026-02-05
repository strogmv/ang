package emitter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/strogmv/ang/compiler/ir"
)

// SystemManifest represents the compact architectural map of the system.
type SystemManifest struct {
	Project    ir.Project       `json:"project"`
	Services   []ServiceSummary `json:"services"`
	Events     []EventSummary   `json:"events"`
	Entities   []string         `json:"entities"`
}

type ServiceSummary struct {
	Name       string            `json:"name"`
	Methods    []string          `json:"methods"`
	Publishes  []string          `json:"publishes"`
	Subscribes map[string]string `json:"subscribes"`
	DependsOn  []string          `json:"depends_on"`
}

type EventSummary struct {
	Name   string   `json:"name"`
	Fields []string `json:"fields"`
}

// EmitManifest generates a compact JSON manifest of the whole system.
func (e *Emitter) EmitManifest(schema *ir.Schema) error {
	manifest := SystemManifest{
		Project:  schema.Project,
		Entities: make([]string, 0),
	}

	for _, ent := range schema.Entities {
		manifest.Entities = append(manifest.Entities, ent.Name)
	}

	for _, svc := range schema.Services {
		sSummary := ServiceSummary{
			Name:       svc.Name,
			Methods:    make([]string, 0),
			Publishes:  svc.Publishes,
			Subscribes: svc.Subscribes,
			DependsOn:  make([]string, 0),
		}

		for _, m := range svc.Methods {
			sSummary.Methods = append(sSummary.Methods, m.Name)
		}

		if svc.RequiresSQL { sSummary.DependsOn = append(sSummary.DependsOn, "Postgres") }
		if svc.RequiresMongo { sSummary.DependsOn = append(sSummary.DependsOn, "MongoDB") }
		if svc.RequiresRedis { sSummary.DependsOn = append(sSummary.DependsOn, "Redis") }
		if svc.RequiresNats { sSummary.DependsOn = append(sSummary.DependsOn, "NATS") }
		if svc.RequiresS3 { sSummary.DependsOn = append(sSummary.DependsOn, "S3") }

		manifest.Services = append(manifest.Services, sSummary)
	}

	for _, ev := range schema.Events {
		eSummary := EventSummary{
			Name:   ev.Name,
			Fields: make([]string, 0),
		}
		for _, f := range ev.Fields {
			eSummary.Fields = append(eSummary.Fields, f.Name)
		}
		manifest.Events = append(manifest.Events, eSummary)
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	path := filepath.Join(e.OutputDir, "ang-manifest.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	fmt.Printf("Generated System Manifest: %s\n", path)
	return nil
}
