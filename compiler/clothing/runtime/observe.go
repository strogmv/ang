package clothing

import "fmt"

// Evidence is constructed by authoritative perception, never from a client
// claim. Covered inner layers are not automatically visible or smell-reachable.
type Evidence struct {
	VisibleLayers  []string `json:"visible_layers"`
	SmellLayers    []string `json:"smell_layers"`
	SmellReachable bool     `json:"smell_reachable"`
}
type Observation struct {
	ItemID          string `json:"item_id,omitempty"`
	Name            string `json:"name,omitempty"`
	AppearanceStyle string `json:"appearance_style,omitempty"`
	Visible         bool   `json:"visible"`
	Dirt            int    `json:"dirt"`
	Blood           int    `json:"blood"`
	Sanitary        int    `json:"sanitary"`
	Wetness         int    `json:"wetness"`
	Wear            int    `json:"wear"`
	SanitaryOdor    int    `json:"sanitary_odor"`
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func Observe(m Model, s State, e Evidence, now int64) (Observation, error) {
	if err := s.Validate(m, now); err != nil {
		return Observation{}, err
	}
	var out Observation
	for _, id := range e.VisibleLayers {
		layer, ok := s.Layers[id]
		if !ok {
			return Observation{}, fmt.Errorf("unknown visible layer %q", id)
		}
		out.Visible = true
		out.Dirt = max(out.Dirt, layer.Dirt)
		out.Blood = max(out.Blood, layer.Blood)
		out.Sanitary = max(out.Sanitary, layer.Sanitary)
		out.Wetness = max(out.Wetness, layer.Wetness)
		out.Wear = max(out.Wear, layer.Wear)
	}
	if out.Visible {
		out.ItemID = s.ItemID
		out.Name = m.Garments[s.Definition].Name
		out.AppearanceStyle = m.Garments[s.Definition].AppearanceStyle
	}
	for _, id := range e.SmellLayers {
		layer, ok := s.Layers[id]
		if !ok {
			return Observation{}, fmt.Errorf("unknown smell layer %q", id)
		}
		if e.SmellReachable {
			out.SanitaryOdor = max(out.SanitaryOdor, layer.Sanitary)
		}
	}
	// Provenance, stain source, concealed layers and precise stain age are omitted.
	return out, nil
}

// Context contains only facts this observer actually knows. An absent fact is
// unknown, not false. Witnessing an attack is separate from noticing blood.
type Context struct {
	Role  string          `json:"role"`
	Known map[string]bool `json:"known"`
}
type Signal struct {
	ID     string   `json:"id"`
	Weight int      `json:"weight"`
	Rules  []string `json:"rules"`
}

func Assess(m Model, o Observation, c Context) ([]Signal, error) {
	if !bounded(o.Dirt) || !bounded(o.Blood) || !bounded(o.Sanitary) || !bounded(o.Wetness) || !bounded(o.Wear) || !bounded(o.SanitaryOdor) {
		return nil, fmt.Errorf("invalid observation")
	}
	if !o.Visible && (o.ItemID != "" || o.Name != "" || o.AppearanceStyle != "" || o.Dirt != 0 || o.Blood != 0 || o.Sanitary != 0 || o.Wetness != 0 || o.Wear != 0) {
		return nil, fmt.Errorf("non-visible observation contains visual identity or measurements")
	}
	out := []Signal{}
	for _, rule := range m.Reactions {
		if rule.Role != "" && rule.Role != c.Role || rule.AppearanceStyle != "" && rule.AppearanceStyle != o.AppearanceStyle {
			continue
		}
		matched := true
		for key, want := range rule.When {
			got, known := c.Known[key]
			if !known || got != want {
				matched = false
				break
			}
		}
		if !matched {
			continue
		}
		value := 0
		if rule.Metric == "sanitary_odor" {
			value = o.SanitaryOdor
		} else if o.Visible {
			switch rule.Metric {
			case "dirt":
				value = o.Dirt
			case "blood":
				value = o.Blood
			case "sanitary":
				value = o.Sanitary
			case "wetness":
				value = o.Wetness
			case "wear":
				value = o.Wear
			}
		}
		if value < rule.Threshold {
			continue
		}
		index := -1
		for i := range out {
			if out[i].ID == rule.Signal {
				index = i
				break
			}
		}
		if index < 0 {
			out = append(out, Signal{ID: rule.Signal})
			index = len(out) - 1
		}
		out[index].Weight = clamp(out[index].Weight + rule.Weight)
		out[index].Rules = append(out[index].Rules, rule.ID)
	}
	return out, nil
}
