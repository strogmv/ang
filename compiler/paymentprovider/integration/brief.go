package integration

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const BriefVersion = 1

// Brief is the structured PM/integration ticket consumed by agents and tooling.
type Brief struct {
	Version int `yaml:"version"`
	Provider struct {
		Code        string `yaml:"code"`
		Name        string `yaml:"name"`
		Label       string `yaml:"label"`
		PackageName string `yaml:"package_name"`
	} `yaml:"provider"`
	Integration struct {
		Type string `yaml:"type"` // checkout, h2h, p2p, payout
		Flow string `yaml:"flow"` // redirect, direct, callback
	} `yaml:"integration"`
	Methods    []string `yaml:"methods"`
	Operations []string `yaml:"operations"`
	Currencies []string `yaml:"currencies"`
	Customer   struct {
		MandatoryFields []string `yaml:"mandatory_fields"`
		Notes           string   `yaml:"notes"`
	} `yaml:"customer"`
	Implementation string `yaml:"implementation"` // investigation, hand_written, generated
	Estimates      struct {
		DevHours  int `yaml:"dev_hours"`
		TestHours int `yaml:"test_hours"`
	} `yaml:"estimates"`
	References struct {
		Ticket          string `yaml:"ticket"`
		APIDoc          string `yaml:"api_doc"` // deprecated: use expert_knowledge in ang.yaml
		ExpertKnowledge string `yaml:"expert_knowledge"`
		Chat            string `yaml:"chat"`
		URLs            []string `yaml:"urls"`
	} `yaml:"references"`
	Status string `yaml:"status"` // investigation, in_progress, done
	Notes  string `yaml:"notes"`
}

func LoadBrief(path string) (Brief, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Brief{}, err
	}
	var brief Brief
	if err := yaml.Unmarshal(data, &brief); err != nil {
		return Brief{}, fmt.Errorf("parse brief: %w", err)
	}
	if err := brief.Validate(); err != nil {
		return Brief{}, err
	}
	return brief, nil
}

func (b Brief) Validate() error {
	if b.Version == 0 {
		b.Version = BriefVersion
	}
	if b.Version != BriefVersion {
		return fmt.Errorf("brief.version must be %d", BriefVersion)
	}
	if strings.TrimSpace(b.Provider.Code) == "" {
		return fmt.Errorf("brief.provider.code is required")
	}
	if strings.TrimSpace(b.Provider.Label) == "" {
		return fmt.Errorf("brief.provider.label is required")
	}
	switch strings.TrimSpace(b.Implementation) {
	case "", "investigation", "hand_written", "generated":
	default:
		return fmt.Errorf("brief.implementation must be investigation, hand_written, or generated")
	}
	return nil
}

func DefaultBrief(opts InitOptions) Brief {
	var brief Brief
	brief.Version = BriefVersion
	brief.Provider.Code = opts.SID
	brief.Provider.Name = opts.Name
	brief.Provider.Label = opts.Label
	brief.Provider.PackageName = opts.PackageName
	brief.Integration.Type = "checkout"
	brief.Integration.Flow = "redirect"
	brief.Operations = []string{"payin"}
	brief.Currencies = []string{"EUR"}
	brief.Implementation = "investigation"
	brief.Status = "investigation"
	brief.References.ExpertKnowledge = opts.SID
	if opts.KnowledgeID != "" {
		brief.References.ExpertKnowledge = opts.KnowledgeID
	}
	brief.References.Ticket = opts.TicketSummary
	return brief
}
