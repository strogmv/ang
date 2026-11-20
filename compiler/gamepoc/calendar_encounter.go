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
	Greeting    string            `json:"greeting"`
	Choices     []EncounterChoice `json:"choices"`
	// The custody conversation like Disco Elysium (owner 20.11): its opening lines with their speakers, the hero's inner
	// voices at the opening, and questions answered before the decision (the shared conversation types, dialogue.go).
	Prompt    []DialogueLine      `json:"prompt"`
	Voices    []DialogueVoice     `json:"voices"`
	Questions []EncounterQuestion `json:"questions"`
	Duty      struct {
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
	ReplyUnseen []DialogueLine `json:"reply_unseen,omitempty"`
	Reply       []DialogueLine `json:"reply"`
	// A skill check of this answer and what follows its failure (another outcome, another reply).
	Check *DialogueCheck `json:"check,omitempty"`
	Fail  *EncounterFail `json:"fail,omitempty"`

	ID              string `json:"id"`
	Text            string `json:"text"`
	Approach        string `json:"approach"`
	Hook            string `json:"hook"`
	Condition       string `json:"condition"`
	Risk            string `json:"risk"`
	Consequence     string `json:"consequence"`
	RequiresContact bool   `json:"requires_contact"`
	Outcome         string `json:"outcome"`
}

// EncounterFail is the outcome and the reply of a failed skill check.
type EncounterFail struct {
	Outcome     string `json:"outcome"`
	Consequence string `json:"consequence"`
	// TrustCost: the person heard it fail; his trust in the hero drops by this much more (≤ 0), whatever the outcome
	TrustCost float64        `json:"trust_cost"`
	Reply     []DialogueLine `json:"reply"`
}

// EncounterQuestion is asked before the decision: the reply is written to the log and the same answers come back. A
// question may carry a skill check (once or retry); a failed one is answered with FailReply.
type EncounterQuestion struct {
	ID        string         `json:"id"`
	Text      string         `json:"text"`
	Reply     []DialogueLine `json:"reply"`
	Check     *DialogueCheck `json:"check,omitempty"`
	FailReply []DialogueLine `json:"fail_reply,omitempty"`
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
	known := func(outcome string) bool {
		switch outcome {
		case "willing", "forced", "refuse", "escape":
			return true
		}
		return false
	}
	for _, c := range e.Choices {
		if c.ID == "" || c.ID == "timeout" || keys[c.ID] || c.Text == "" || len(c.Reply) == 0 || c.Approach == "" || c.Hook == "" || c.Condition == "" || c.Risk == "" || c.Consequence == "" {
			return fail("incomplete or duplicate conversation choice")
		}
		keys[c.ID] = true
		if !known(c.Outcome) {
			return fail("unknown deterministic consequence")
		}
		outcomes[c.Outcome] = true
		// a check decides between two outcomes: the failure has its own, and a failure without a check is meaningless
		if (c.Check == nil) != (c.Fail == nil) || c.Fail != nil && (!known(c.Fail.Outcome) || c.Fail.Consequence == "" || len(c.Fail.Reply) == 0 || c.Fail.TrustCost > 0 || c.Fail.TrustCost < -1) {
			return fail("choice " + c.ID + ": a skill check needs its failure (outcome, consequence, reply) and only a check has one")
		}
		// a decisive answer ends the conversation either way: it has one try
		if c.Check != nil && c.Check.Kind != "once" {
			return fail("choice " + c.ID + ": the check of a decisive answer is once")
		}
	}
	for _, q := range e.Questions {
		if q.ID == "" || q.ID == "timeout" || keys[q.ID] || q.Text == "" || len(q.Reply) == 0 || (q.Check == nil) != (len(q.FailReply) == 0) {
			return fail("incomplete or duplicate conversation question, or a check without its failure reply")
		}
		keys[q.ID] = true
	}
	if len(outcomes) != 4 {
		return fail("conversation must include willing, forced, refusal and escape")
	}
	if len(e.Poses) < 1 || e.WalkPose == "" || e.IdlePose == "" || e.ReturnWalk == "" || e.ReturnIdle == "" || len(e.Prompt) == 0 || e.Greeting == "" {
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
	// the conversation's lines are spoken by the encounter's own two people; checks and voices follow the shared rules
	speakers := map[string]bool{e.Actor: true, e.Guard: true}
	rules := c.DialogueRules
	if err := rules.lines(c, "calendar encounter prompt", e.Prompt, speakers); err != nil {
		return err
	}
	if err := rules.voices(c, "calendar encounter voices", e.Voices); err != nil {
		return err
	}
	texts := []string{e.Greeting, e.RestLabel}
	for _, q := range e.Choices {
		texts = append(texts, q.Text)
		for _, set := range [][]DialogueLine{q.Reply, q.ReplyUnseen} {
			if len(set) > 0 {
				if err := rules.lines(c, "calendar encounter choice "+q.ID, set, speakers); err != nil {
					return err
				}
			}
		}
		if q.Check != nil {
			if err := rules.skillCheck("calendar encounter choice "+q.ID, *q.Check); err != nil {
				return err
			}
			if err := rules.lines(c, "calendar encounter choice "+q.ID+" failure", q.Fail.Reply, speakers); err != nil {
				return err
			}
		}
	}
	for _, q := range e.Questions {
		texts = append(texts, q.Text)
		if err := rules.lines(c, "calendar encounter question "+q.ID, q.Reply, speakers); err != nil {
			return err
		}
		if q.Check != nil {
			if err := rules.skillCheck("calendar encounter question "+q.ID, *q.Check); err != nil {
				return err
			}
			if err := rules.lines(c, "calendar encounter question "+q.ID+" failure", q.FailReply, speakers); err != nil {
				return err
			}
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
