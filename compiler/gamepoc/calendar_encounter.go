package gamepoc

import (
	"bytes"
	"cuelang.org/go/cue"
	"encoding/json"
	"fmt"
	"strings"
)

// CalendarEncounter is an optional named Life obligation and a finite conversation.
// Texts, dates, poses, approaches and deterministic consequences are authoring data.
type CalendarEncounter struct {
	FlexibleRoutine bool   `json:"flexible_routine"`
	RestActivity    string `json:"rest_activity"`
	RestLabel       string `json:"rest_label"`

	ReplySeconds int64           `json:"reply_seconds"`
	EscortGap    int             `json:"escort_gap_cells"`
	EscortHold   int             `json:"escort_hold_cells"`
	DoorID       string          `json:"door_id"`
	Presentation json.RawMessage `json:"presentation"`

	ID          string            `json:"id"`
	Actor       string            `json:"actor"`
	Guard       string            `json:"guard"`
	Level       string            `json:"level"`
	Shop        string            `json:"shop"`
	FirstDays   int64             `json:"first_days"`
	MissingDays int64             `json:"missing_days"`
	Morning     int64             `json:"morning"`
	Evening     int64             `json:"evening"`
	Reach       int               `json:"reach"`
	Gate        EncounterPlace    `json:"gate"`
	Meet        EncounterPlace    `json:"meet"`
	GuardAt     EncounterPlace    `json:"guard_at"`
	Medical     EncounterPlace    `json:"medical"`
	Exit        EncounterPlace    `json:"exit"`
	MedicalEdge string            `json:"medical_edge"`
	GateEdge    string            `json:"gate_edge"`
	Poses       []EncounterPose   `json:"poses"`
	WalkPose    string            `json:"walk_pose"`
	IdlePose    string            `json:"idle_pose"`
	ReturnWalk  string            `json:"return_walk"`
	ReturnIdle  string            `json:"return_idle"`
	Prompt      string            `json:"prompt"`
	Greeting    string            `json:"greeting"`
	Choices     []EncounterChoice `json:"choices"`
	Duty        struct {
		Law             string  `json:"law"`
		PlayerEligible  bool    `json:"player_eligible"`
		IncludesFriends bool    `json:"includes_friends"`
		TimeoutSeconds  int64   `json:"timeout_seconds"`
		RatingPenalty   int     `json:"rating_penalty"`
		TrustCost       float64 `json:"trust_cost"`
	} `json:"duty"`
}
type EncounterPlace struct {
	Level string `json:"level"`
	At    struct {
		X int `json:"x"`
		Y int `json:"y"`
	} `json:"at"`
}
type EncounterPose struct {
	State   string `json:"state"`
	Seconds int64  `json:"seconds"`
	Bark    string `json:"bark"`
	Item    string `json:"item"`
}
type EncounterChoice struct {
	ReplyUnseen string `json:"reply_unseen,omitempty"`

	ID              string `json:"id"`
	Text            string `json:"text"`
	Reply           string `json:"reply"`
	Approach        string `json:"approach"`
	Hook            string `json:"hook"`
	Condition       string `json:"condition"`
	Risk            string `json:"risk"`
	Consequence     string `json:"consequence"`
	RequiresContact bool   `json:"requires_contact"`
	Outcome         string `json:"outcome"`
}

func readCalendarEncounter(world cue.Value) (*CalendarEncounter, error) {
	v := world.LookupPath(cue.ParsePath("calendar_encounter"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("world.calendar_encounter: %w", err)
	}
	var e CalendarEncounter
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err = d.Decode(&e); err != nil {
		return nil, fmt.Errorf("world.calendar_encounter: %w", err)
	}
	if err = e.validate(); err != nil {
		return nil, err
	}
	return &e, nil
}
func (e CalendarEncounter) validate() error {
	fail := func(s string) error { return fmt.Errorf("world.calendar_encounter: %s", s) }
	if !e.FlexibleRoutine || e.RestActivity != "sleep" || e.RestLabel == "" {
		return fail("invalid public trading and private night routine")
	}
	if e.ReplySeconds < 10 || e.ReplySeconds > 300 {
		return fail("invalid reply duration")
	}
	if e.EscortGap < 1 || e.EscortGap > 4 || e.EscortHold < e.EscortGap || e.EscortHold > 6 {
		return fail("invalid physical escort spacing")
	}
	if e.ID == "" || e.Actor == "" || e.Guard == e.Actor || e.Guard == "" || e.FirstDays < 1 || e.FirstDays > 30 || e.MissingDays < 1 || e.MissingDays > 30 || e.Morning < 0 || e.Evening <= e.Morning || e.Evening >= 86400 || e.Reach < 1 || e.Reach > 24 {
		return fail("invalid identity, dates, clock or reach")
	}
	if !e.Duty.PlayerEligible || !e.Duty.IncludesFriends || e.Duty.Law == "" || e.Duty.TimeoutSeconds < 1 || e.Duty.RatingPenalty < 1 || e.Duty.RatingPenalty > 1000 || e.Duty.TrustCost > 0 || e.Duty.TrustCost < -1 {
		return fail("duty must include the player and friends and use a bounded law penalty")
	}
	keys := map[string]bool{}
	outcomes := map[string]bool{}
	for _, c := range e.Choices {
		if c.ID == "" || keys[c.ID] || c.Text == "" || c.Reply == "" || c.Approach == "" || c.Hook == "" || c.Condition == "" || c.Risk == "" || c.Consequence == "" {
			return fail("incomplete or duplicate conversation choice")
		}
		keys[c.ID] = true
		switch c.Outcome {
		case "willing", "forced", "refuse", "escape":
		default:
			return fail("unknown deterministic consequence")
		}
		outcomes[c.Outcome] = true
	}
	if len(outcomes) != 4 {
		return fail("conversation must include willing, forced, refusal and escape")
	}
	if len(e.Poses) < 1 || e.WalkPose == "" || e.IdlePose == "" || e.ReturnWalk == "" || e.ReturnIdle == "" || e.Prompt == "" || e.Greeting == "" {
		return fail("missing poses or speech")
	}
	for _, p := range e.Poses {
		if p.State == "" || p.Seconds < 1 || p.Seconds > 120 {
			return fail("invalid pose duration")
		}
	}
	return nil
}
func checkCalendarEncounter(c *Compiled) error {
	e := c.CalendarEncounter
	if e == nil {
		return nil
	}
	var life struct {
		Actors []struct {
			ID string `json:"id"`
		} `json:"actors"`
		Locations map[string]struct {
			Terrain []string `json:"terrain"`
			Gates   []struct {
				ID string `json:"id"`
			} `json:"gates"`
		} `json:"locations"`
		Edges []struct{ ID, From, To string } `json:"edges"`
	}
	if err := json.Unmarshal(c.Life, &life); err != nil {
		return err
	}
	foundDoor := false
	for _, g := range life.Locations[e.Level].Gates {
		foundDoor = foundDoor || g.ID == e.DoorID
	}
	if !foundDoor {
		return fmt.Errorf("calendar encounter: missing physical door %s", e.DoorID)
	}
	actors := map[string]bool{}
	for _, a := range life.Actors {
		actors[a.ID] = true
	}
	if !actors[e.Actor] || !actors[e.Guard] {
		return fmt.Errorf("calendar encounter: unknown Life participant")
	}
	for _, p := range []EncounterPlace{e.Gate, e.Meet, e.GuardAt, e.Medical, e.Exit} {
		l := life.Locations[p.Level]
		x, y := p.At.X, p.At.Y
		if y < 0 || y >= len(l.Terrain) || x < 0 || x >= len(l.Terrain[y]) || !c.Walkable[string(l.Terrain[y][x])] {
			return fmt.Errorf("calendar encounter: blocked approach %s (%d,%d)", p.Level, x, y)
		}
	}
	for _, id := range []string{e.GateEdge, e.MedicalEdge} {
		found := false
		for _, edge := range life.Edges {
			found = found || edge.ID == id
		}
		if !found {
			return fmt.Errorf("calendar encounter: missing Life edge %s", id)
		}
	}
	for _, pose := range e.Poses {
		if pose.Item != "" {
			if _, ok := c.Inventory.Definitions[pose.Item]; !ok {
				return fmt.Errorf("calendar encounter: unknown shown item %s", pose.Item)
			}
		}
	}
	messages := map[string]bool{}
	for _, m := range c.Localization.Messages {
		messages[m.Key] = true
	}
	texts := []string{e.Prompt, e.Greeting, e.RestLabel}
	for _, q := range e.Choices {
		texts = append(texts, q.Text, q.Reply)
		if q.ReplyUnseen != "" {
			texts = append(texts, q.ReplyUnseen)
		}
	}
	for _, p := range e.Poses {
		if p.Bark != "" {
			texts = append(texts, p.Bark)
		}
	}
	for _, key := range texts {
		if !messages[key] || strings.TrimSpace(key) == "" {
			return fmt.Errorf("calendar encounter: missing localization %s", key)
		}
	}
	return nil
}
