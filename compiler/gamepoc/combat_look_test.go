package gamepoc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func lookOK() json.RawMessage {
	return json.RawMessage(`{"move_mps":1.3,
		"weapons":[{"item":"item.knife_simple","stance":{"state":"combat_hero_wary","source":"mixamo_edit/Hero_Wary_Knife"},
			"strikes":[{"zones":["chest"],"single":{"state":"combat_knife_lunge_mid","source":"m1/Knife_Lunge_Mid"}}]}],
		"hit":{"state":"combat_hit","source":"ual1/Hit_Chest"},
		"body_solver":{"capsules":[{"id":"chest","group":"torso"},{"id":"hand_r","group":"arm_r"},{"id":"hand_l","group":"arm_l"},{"id":"shin_l","group":"leg_l"}]},
		"godot":{"library":"res://actors-assets/combat.res","sound_dir":"res://assets/audio/combat",
			"sources":{"ual1":"ual","m1":"art-source/anim-mixamo/combat-edited-m1","mixamo_edit":"art-source/anim-mixamo/combat-edited"},
			"bodies":{"push_groups":["torso","leg_l"],"push_skip":["hand_l"],"arms":{"Left":"arm_l","Right":"arm_r"},"blade_hand":"hand_r"}}}`)
}

// Every clip family the look uses has a folder for the Godot library; a family without one is refused, as is a look
// without the godot block.
func TestCombatLookNamesAFolderForEveryClipFamily(t *testing.T) {
	if err := validateCombatLook(lookOK()); err != nil {
		t.Fatal(err)
	}
	var look map[string]any
	_ = json.Unmarshal(lookOK(), &look)
	if got := combatLookSources(look); strings.Join(got, ",") != "m1/Knife_Lunge_Mid,mixamo_edit/Hero_Wary_Knife,ual1/Hit_Chest" {
		t.Fatalf("clip sources: %v", got)
	}
	delete(look["godot"].(map[string]any)["sources"].(map[string]any), "m1")
	raw, _ := json.Marshal(look)
	if err := validateCombatLook(raw); err == nil || !strings.Contains(err.Error(), "[m1]") {
		t.Fatalf("a family without a folder must be refused: %v", err)
	}
	delete(look, "godot")
	raw, _ = json.Marshal(look)
	if err := validateCombatLook(raw); err == nil {
		t.Fatal("a look without the godot block must be refused")
	}
	bad := strings.Replace(string(lookOK()), `"m1/Knife_Lunge_Mid"`, `"Knife_Lunge_Mid"`, 1)
	if err := validateCombatLook(json.RawMessage(bad)); err == nil {
		t.Fatal("a source without its family must be refused")
	}
}

// The Godot solver's lists name real capsules and groups of the body solver: a typo is refused.
func TestCombatLookBodiesNameRealCapsules(t *testing.T) {
	for name, change := range map[string][2]string{
		"unknown push group": {`"push_groups":["torso","leg_l"]`, `"push_groups":["torso","legs"]`},
		"unknown skip":       {`"push_skip":["hand_l"]`, `"push_skip":["hand_left"]`},
		"unknown arm":        {`"Right":"arm_r"`, `"Right":"arm_right"`},
		"unknown blade hand": {`"blade_hand":"hand_r"`, `"blade_hand":"blade"`},
		"no push group":      {`"push_groups":["torso","leg_l"]`, `"push_groups":[]`},
	} {
		bad := strings.Replace(string(lookOK()), change[0], change[1], 1)
		if bad == string(lookOK()) {
			t.Fatalf("%s: the fixture moved", name)
		}
		if err := validateCombatLook(json.RawMessage(bad)); err == nil {
			t.Fatalf("%s: accepted", name)
		}
	}
}

// The look goes to the client's presentation and never into the server's world.
func TestCombatLookStaysOutOfTheServerWorld(t *testing.T) {
	c := Compiled{CombatLook: lookOK()}
	world, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(world), "Knife_Lunge_Mid") || strings.Contains(string(world), `"combat_look"`) {
		t.Fatalf("the server's world must not carry the look: %s", world)
	}
	view, err := clientResource(c)
	if err != nil {
		t.Fatal(err)
	}
	var client struct {
		CombatLook struct {
			Godot combatLookGodot `json:"godot"`
		} `json:"combat_look"`
	}
	if err := json.Unmarshal(view, &client); err != nil {
		t.Fatal(err)
	}
	if client.CombatLook.Godot.Library != "res://actors-assets/combat.res" {
		t.Fatalf("the client's presentation lacks the look: %s", view)
	}
}

// Every take of the look must be on disk in its family's folder; a missing folder or take is refused by name.
func TestCombatLookTakesAreOnDisk(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{"art-source/anim-mixamo/combat-edited-m1/Knife_Lunge_Mid.fbx", "art-source/anim-mixamo/combat-edited/Hero_Wary_Knife.fbx"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, f)), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, f), []byte("fbx"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := checkCombatLookTakes(root, lookOK()); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "art-source/anim-mixamo/combat-edited-m1/Knife_Lunge_Mid.fbx")); err != nil {
		t.Fatal(err)
	}
	if err := checkCombatLookTakes(root, lookOK()); err == nil || !strings.Contains(err.Error(), "Knife_Lunge_Mid.fbx") {
		t.Fatalf("a missing take must be named: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(root, "art-source/anim-mixamo/combat-edited")); err != nil {
		t.Fatal(err)
	}
	if err := checkCombatLookTakes(root, lookOK()); err == nil || !strings.Contains(err.Error(), "combat-edited/") {
		t.Fatalf("a missing folder must be named: %v", err)
	}
	if err := checkCombatLookTakes(root, nil); err != nil {
		t.Fatal("a world without the look passes:", err)
	}
}
