package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	IntegrationBriefSchemaV1 = "ang/expert-knowledge/integration-brief/v1"
)

// integrationBriefJSON is the Expert knowledge/data/* integration brief envelope.
type integrationBriefJSON struct {
	Schema         string   `json:"schema"`
	ID             string   `json:"id"`
	Version        int      `json:"version"`
	Provider       briefProviderJSON `json:"provider"`
	Integration    briefIntegrationJSON `json:"integration"`
	Methods        []string `json:"methods"`
	Operations     []string `json:"operations"`
	Currencies     []string `json:"currencies"`
	Implementation string   `json:"implementation"`
	Status         string   `json:"status"`
	References     struct {
		Ticket   string   `json:"ticket"`
		DocURLs  []string `json:"doc_urls"`
	} `json:"references"`
}

type briefProviderJSON struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Label       string `json:"label"`
	PackageName string `json:"package_name"`
}

type briefIntegrationJSON struct {
	Type string `json:"type"`
	Flow string `json:"flow"`
}

// ResolveBrief loads integration brief from legacy integration/brief.yaml or Expert knowledge/data.
func ResolveBrief(projectPath, expertRoot, expertKnowledgeID string) (Brief, error) {
	legacy := filepath.Join(projectPath, "integration", "brief.yaml")
	if _, err := os.Stat(legacy); err == nil {
		return LoadBrief(legacy)
	}
	id := strings.TrimSpace(expertKnowledgeID)
	if id == "" {
		return Brief{}, fmt.Errorf("no integration/brief.yaml and expert_knowledge_id is empty in ang.yaml")
	}
	root := strings.TrimSpace(expertRoot)
	if root == "" {
		root = strings.TrimSpace(os.Getenv("ANG_EXPERT_ROOT"))
	}
	if root == "" {
		return Brief{}, fmt.Errorf("no integration/brief.yaml: set expert_knowledge_id in ang.yaml and ANG_EXPERT_ROOT to deal/expert")
	}
	return LoadBriefFromExpert(root, id)
}

// LoadBriefFromExpert reads knowledge/data/<id>.json from Expert repo.
func LoadBriefFromExpert(expertRoot, knowledgeID string) (Brief, error) {
	knowledgeID = strings.TrimSpace(knowledgeID)
	if knowledgeID == "" {
		return Brief{}, fmt.Errorf("expert knowledge id is required")
	}
	path := filepath.Join(expertRoot, "knowledge", "data", knowledgeID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return Brief{}, fmt.Errorf("read expert knowledge %s: %w", path, err)
	}
	var doc integrationBriefJSON
	if err := json.Unmarshal(data, &doc); err != nil {
		return Brief{}, fmt.Errorf("parse expert knowledge %s: %w", path, err)
	}
	if doc.Schema != IntegrationBriefSchemaV1 {
		return Brief{}, fmt.Errorf("expert knowledge %s: schema must be %s", path, IntegrationBriefSchemaV1)
	}
	var brief Brief
	brief.Version = BriefVersion
	if doc.Version > 0 {
		brief.Version = doc.Version
	}
	brief.Provider.Code = doc.Provider.Code
	brief.Provider.Name = doc.Provider.Name
	brief.Provider.Label = doc.Provider.Label
	brief.Provider.PackageName = doc.Provider.PackageName
	brief.Integration.Type = doc.Integration.Type
	brief.Integration.Flow = doc.Integration.Flow
	brief.Methods = doc.Methods
	brief.Operations = doc.Operations
	brief.Currencies = doc.Currencies
	brief.Implementation = doc.Implementation
	brief.Status = doc.Status
	brief.References.Ticket = doc.References.Ticket
	brief.References.ExpertKnowledge = doc.ID
	brief.References.URLs = doc.References.DocURLs
	if err := brief.Validate(); err != nil {
		return Brief{}, err
	}
	return brief, nil
}
