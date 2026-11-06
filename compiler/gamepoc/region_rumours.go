package gamepoc

import (
	"encoding/json"
	"fmt"
)

// RegionRumour binds testimony to a real place privately. Public testimony has no geometry.
type RegionRumour struct {
	ID            string `json:"id"`
	SubjectID     string `json:"subject_id"`
	NodeID        string `json:"node_id"`
	SourceActorID string `json:"source_actor_id"`
	TextKey       string `json:"text_key"`
	ContextKey    string `json:"context_key"`
	ImageVariant  string `json:"image_variant"`
}

func (r *RegionCatalog) validateRumours(life json.RawMessage) error {
	if len(r.Rumours) > 4096 {
		return fmt.Errorf("too many region rumours")
	}
	var l struct {
		Actors []struct {
			ID string `json:"id"`
		} `json:"actors"`
	}
	if err := json.Unmarshal(life, &l); err != nil {
		return err
	}
	actors := map[string]bool{}
	for _, a := range l.Actors {
		actors[a.ID] = true
	}
	nodes := map[string]RegionNode{}
	for _, n := range r.Nodes {
		nodes[n.ID] = n
	}
	ids := map[string]bool{}
	subjects := map[string]string{}
	for _, q := range r.Rumours {
		n := nodes[q.NodeID]
		if q.ID == "" || ids[q.ID] || q.SubjectID == "" || q.SubjectID == q.NodeID || q.SubjectID == n.LevelID || n.ID == "" || n.LevelID == "" || !actors[q.SourceActorID] || q.TextKey == "" || q.ContextKey == "" || (q.ImageVariant != "narrow_stair" && q.ImageVariant != "great_descent") {
			return fmt.Errorf("invalid region rumour %s", q.ID)
		}
		if other := subjects[q.SubjectID]; other != "" && other != q.NodeID {
			return fmt.Errorf("ambiguous region rumour subject %s", q.SubjectID)
		}
		ids[q.ID] = true
		subjects[q.SubjectID] = q.NodeID
	}
	return nil
}
