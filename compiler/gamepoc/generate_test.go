package gamepoc

import "testing"

func TestRejectUnsafeOrAmbiguousPolicy(t *testing.T) {
	valid := Config{WorldDir: "../content/world", UnityDir: "../client-unity", LevelID: "level.test", Title: "Game", TickHz: 10, SchedulerHz: 100, GameSecondsPerTick: 6, MoveEveryTicks: 2, InputAckBudgetMS: 30, MinClientFPS: 40, PresentationLeadMS: 15, PresentationHorizonMS: 150, QueueCapacity: 128, MaxCatchupSteps: 4, SaveVersion: 1, MaxSaveBytes: 1048576, SaveSlots: []string{"slot1"}, Labels: map[string]string{}}
	for _, name := range []string{"new_game", "load_game", "save_game", "options", "continue_game", "main_menu", "quit"} {
		valid.Labels[name] = name
	}
	if err := valid.validate(); err != nil {
		t.Fatal(err)
	}
	for _, slots := range [][]string{{"../escape"}, {"slot1", "slot1"}, {"."}, {""}} {
		c := valid
		c.SaveSlots = slots
		if c.validate() == nil {
			t.Fatalf("accepted slots %v", slots)
		}
	}
	for _, version := range []int{0, 2} {
		c := valid
		c.SaveVersion = version
		if c.validate() == nil {
			t.Fatalf("accepted version %d", version)
		}
	}
	c := valid
	c.SchedulerHz = 20
	if c.validate() == nil {
		t.Fatal("accepted scheduler slower than maximum world speed")
	}
	c = valid
	c.PresentationLeadMS = 31
	if c.validate() == nil {
		t.Fatal("accepted presentation lead above input budget")
	}
	c = valid
	c.TickHz = 0
	if c.validate() == nil {
		t.Fatal("accepted zero tick rate")
	}
}
