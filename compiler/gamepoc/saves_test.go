package gamepoc

import (
	"encoding/json"
	"testing"
)

// The save kinds (owner 19.11): the autosave and the quick save have their own slots, distinct from every other slot,
// sane numbers, and only the quick save's keys and note reach the client (the slots and the rotation stay server-owned).
func TestSaveKindsPolicy(t *testing.T) {
	valid := Config{WorldDir: "../content/world", UnityDir: "../client-unity", LevelID: "level.test", Title: "Game", TickHz: 10, SchedulerHz: 100, GameSecondsPerTick: 6, MoveEveryTicks: 2, InputAckBudgetMS: 30, MinClientFPS: 40, PresentationLeadMS: 15, PresentationHorizonMS: 150, QueueCapacity: 128, MaxCatchupSteps: 4, SaveVersion: 1, MaxSaveBytes: 1048576, SaveSlots: []string{"slot1", "slot2"}, Labels: map[string]string{}}
	for _, name := range []string{"new_game", "load_game", "save_game", "options", "continue_game", "main_menu", "quit"} {
		valid.Labels[name] = name
	}
	auto := func() *AutoSavePolicy {
		return &AutoSavePolicy{Slots: []string{"auto1", "auto2", "auto3"}, EveryPlayMinutes: 60}
	}
	quick := func() *QuickSavePolicy {
		return &QuickSavePolicy{Slots: []string{"quick1", "quick2", "quick3"}, NoteSeconds: 1.5}
	}
	c := valid
	c.AutoSave, c.QuickSave = auto(), quick()
	if err := c.validate(); err != nil {
		t.Fatal(err)
	}
	if err := valid.validate(); err != nil {
		t.Fatalf("a game without autosave and quick save: %v", err)
	}
	for name, change := range map[string]func(*Config){
		"an autosave slot is a manual slot": func(c *Config) { c.AutoSave.Slots[0] = "slot1" },
		"a quick slot is an autosave slot":  func(c *Config) { c.QuickSave.Slots[1] = "auto2" },
		"a quick slot twice":                func(c *Config) { c.QuickSave.Slots[2] = "quick1" },
		"an autosave slot leaves the data":  func(c *Config) { c.AutoSave.Slots[0] = "../auto" },
		"no autosave slots":                 func(c *Config) { c.AutoSave.Slots = nil },
		"no quick slots":                    func(c *Config) { c.QuickSave.Slots = []string{} },
		"an autosave every 0 minutes":       func(c *Config) { c.AutoSave.EveryPlayMinutes = 0 },
		"an autosave over a day apart":      func(c *Config) { c.AutoSave.EveryPlayMinutes = 24*60 + 1 },
		"a note too short to read":          func(c *Config) { c.QuickSave.NoteSeconds = 0.1 },
	} {
		c := valid
		c.AutoSave, c.QuickSave = auto(), quick()
		change(&c)
		if c.validate() == nil {
			t.Fatalf("accepted %s", name)
		}
	}
	c = valid
	c.AutoSave, c.QuickSave = auto(), quick()
	data, err := clientResource(Compiled{Config: c})
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Config map[string]json.RawMessage `json:"config"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	// the keys are the input actions quick_save and quick_load (controls v1): the client gets only how long its note shows
	var keys map[string]any
	if err := json.Unmarshal(got.Config["quick_save"], &keys); err != nil || len(keys) != 1 || keys["note_seconds"] != 1.5 {
		t.Fatalf("client quick save %s", got.Config["quick_save"])
	}
	if _, ok := got.Config["auto_save"]; ok {
		t.Fatal("the autosave policy leaked to the client")
	}
	data, err = clientResource(Compiled{Config: valid})
	if err != nil {
		t.Fatal(err)
	}
	got.Config = nil
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Config["quick_save"]; ok {
		t.Fatal("a game without quick save got its keys")
	}
}
