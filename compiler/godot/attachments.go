package godot

import (
	"fmt"
	"math"
	"strings"

	"github.com/strogmv/ang/compiler/placement"
)

// Attachment is source-art metadata in the model's own numeric basis, the same
// local frame as its mesh. The enclosing placement supplies the world transform.
type Attachment struct {
	Name       string         `json:"name"`
	Position   placement.Vec3 `json:"position"`
	SourcePath string         `json:"source_path"`
}
type AttachmentCatalog struct {
	Schema      string                  `json:"schema"`
	SourceFrame string                  `json:"source_frame"`
	Sources     map[string]string       `json:"sources"`
	Models      map[string][]Attachment `json:"models"`
	Instances   map[string][]Attachment `json:"instances,omitempty"`
}
type attachmentNode struct{ Name, Parent, Position, SourcePath string }

func (c AttachmentCatalog) Validate() error {
	if c.Schema != "ang.model-attachments.v1" || c.SourceFrame != "model-local" {
		return fmt.Errorf("godot: invalid attachment schema/frame")
	}
	groups := map[string][]Attachment{}
	for model, nodes := range c.Models {
		groups["model "+model] = nodes
	}
	for id, nodes := range c.Instances {
		groups["instance "+id] = nodes
	}
	for model, nodes := range groups {
		seen := map[string]bool{}
		for _, n := range nodes {
			if n.Name == "" || strings.ContainsAny(n.Name, "/\n\r\x00") || seen[n.Name] || n.SourcePath == "" {
				return fmt.Errorf("godot: invalid/duplicate attachment in %s", model)
			}
			seen[n.Name] = true
			for _, v := range n.Position {
				if math.IsNaN(v) || math.IsInf(v, 0) {
					return fmt.Errorf("godot: non-finite attachment %s", n.Name)
				}
			}
		}
	}
	return nil
}
