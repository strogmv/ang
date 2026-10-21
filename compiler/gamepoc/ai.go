package gamepoc

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func canonicalJSONBytes(data []byte) ([]byte, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

func clientResource(c Compiled) ([]byte, error) {
	cfg := c.Config
	return json.MarshalIndent(map[string]any{
		"content_hash": c.ContentHash,
		"config": map[string]any{
			"title": cfg.Title, "subtitle": cfg.Subtitle, "seed": cfg.Seed, "tick_hz": cfg.TickHz,
			"queue_capacity": cfg.QueueCapacity, "input_ack_budget_ms": cfg.InputAckBudgetMS,
			"min_client_fps": cfg.MinClientFPS, "presentation_lead_ms": cfg.PresentationLeadMS,
			"presentation_horizon_ms": cfg.PresentationHorizonMS, "labels": cfg.Labels,
		},
	}, "", "  ")
}

func validateAIFixtureFacts(data []byte, facts map[string]bool) error {
	var c struct {
		Facts map[string]json.RawMessage `json:"facts"`
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return err
	}
	for key := range facts {
		if _, ok := c.Facts[key]; !ok {
			return fmt.Errorf("unknown AI fixture fact %s", key)
		}
		switch key {
		case "target.alive", "target.dead", "group.formed", "group.in_position", "custody.captured", "custody.held", "custody.at_medblock", "custody.handed_over":
			return fmt.Errorf("AI fixture cannot override live state %s", key)
		}
	}
	return nil
}
