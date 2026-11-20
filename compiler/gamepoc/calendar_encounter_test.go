package gamepoc

import "testing"

func TestCalendarEncounterRejectsIncompleteChoicesAndUnboundedDates(t *testing.T) {
	valid := CalendarEncounter{FlexibleRoutine: true, RestActivity: "sleep", RestLabel: "rest", ID: "encounter.test", ReplySeconds: 80, EscortGap: 2, EscortHold: 3, Actor: "a", Guard: "g", FirstDays: 3, MissingDays: 7, Morning: 30600, Evening: 77400, Reach: 6, WalkPose: "walk", IdlePose: "idle", ReturnWalk: "drunk_walk", ReturnIdle: "drunk_idle", Prompt: []DialogueLine{{Speaker: "g", Text: "text.prompt"}}, Greeting: "text.greet", Poses: []EncounterPose{{State: "beckon", Seconds: 4}}}
	valid.Duty.PlayerEligible = true
	valid.Duty.IncludesFriends = true
	valid.Duty.Law = "law.citizen_duty_dome"
	valid.Duty.TimeoutSeconds = 900
	valid.Duty.RatingPenalty = 100
	for _, outcome := range []string{"willing", "forced", "refuse", "escape"} {
		valid.Choices = append(valid.Choices, EncounterChoice{ID: outcome, Text: "text." + outcome, Reply: []DialogueLine{{Speaker: "a", Text: "reply." + outcome}}, Approach: "approach", Hook: "hook", Condition: "condition", Risk: "risk", Consequence: "consequence", Outcome: outcome})
	}
	if err := valid.validate(); err != nil {
		t.Fatal(err)
	}
	checked := valid
	checked.Choices = append([]EncounterChoice(nil), valid.Choices...)
	checked.Choices[0].Check = &DialogueCheck{Skill: "speech", Difficulty: "medium", Kind: "once"}
	checked.Choices[0].Fail = &EncounterFail{Outcome: "forced", Consequence: "c", Reply: []DialogueLine{{Speaker: "a", Text: "r"}}}
	checked.Questions = []EncounterQuestion{{ID: "ask", Text: "q", Reply: []DialogueLine{{Speaker: "g", Text: "r"}}}}
	if err := checked.validate(); err != nil {
		t.Fatalf("a checked answer with its failure and a question: %v", err)
	}
	for _, mutate := range []func(*CalendarEncounter){func(e *CalendarEncounter) { e.MissingDays = 0 }, func(e *CalendarEncounter) { e.Duty.PlayerEligible = false }, func(e *CalendarEncounter) { e.Choices = e.Choices[:3] }, func(e *CalendarEncounter) { e.Poses = nil }, func(e *CalendarEncounter) { e.Morning = e.Evening },
		// a skill check without its failure, a failure without a check, a question that reuses an answer's id
		func(e *CalendarEncounter) {
			e.Choices = append([]EncounterChoice(nil), e.Choices...)
			e.Choices[0].Check = &DialogueCheck{Skill: "speech", Difficulty: "medium", Kind: "once"}
		},
		func(e *CalendarEncounter) {
			e.Choices = append([]EncounterChoice(nil), e.Choices...)
			e.Choices[0].Fail = &EncounterFail{Outcome: "forced", Consequence: "c", Reply: []DialogueLine{{Speaker: "a", Text: "r"}}}
		},
		func(e *CalendarEncounter) {
			e.Questions = []EncounterQuestion{{ID: "willing", Text: "q", Reply: []DialogueLine{{Speaker: "g", Text: "r"}}}}
		},
		func(e *CalendarEncounter) { e.Prompt = nil },
		// a heard failure that would raise trust
		func(e *CalendarEncounter) {
			e.Choices = append([]EncounterChoice(nil), e.Choices...)
			e.Choices[0].Check = &DialogueCheck{Skill: "speech", Difficulty: "medium", Kind: "once"}
			e.Choices[0].Fail = &EncounterFail{Outcome: "forced", Consequence: "c", TrustCost: 0.5, Reply: []DialogueLine{{Speaker: "a", Text: "r"}}}
		}} {
		copy := valid
		mutate(&copy)
		if copy.validate() == nil {
			t.Fatal("invalid finite conversation or calendar accepted")
		}
	}
}
