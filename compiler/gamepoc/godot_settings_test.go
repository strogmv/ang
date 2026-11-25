package gamepoc

import (
	"context"
	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler/emitter"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func settingsFixture(t *testing.T) (string, Compiled, *normalizer.GRPCDef) {
	root, c, d := screenFixture(t)
	c.GodotSettings = &GodotSettings{Appliers: map[string]SettingApplier{"bus": {Script: "res://appliers.gd", Method: "audio_bus"}}, Profiles: map[string]any{}, Entries: []GodotSetting{{ID: "volume", Section: "audio", Key: "volume", Kind: "stepped_number", Min: 0, Max: 1, Step: .25, Default: float64(0), Label: "ui.test.title", ValueKey: "ui.test.action", DisplayScale: 100, Applier: "bus", Params: map[string]any{"bus": "Master"}}}}
	return root, c, d
}
func TestGodotSettingsNegative(t *testing.T) {
	cases := []struct {
		name, field string
		mutate      func(*Compiled)
	}{
		{"bad default", "default", func(c *Compiled) { c.GodotSettings.Entries[0].Default = 1.1 }},
		{"choice default", "default", func(c *Compiled) {
			s := &c.GodotSettings.Entries[0]
			s.Kind = "choice"
			s.Default = "other"
			s.Options = []SettingOption{{Value: "yes", Key: "ui.test.action"}}
		}},
		{"duplicate storage", "key", func(c *Compiled) {
			s := c.GodotSettings.Entries[0]
			s.ID = "second"
			c.GodotSettings.Entries = append(c.GodotSettings.Entries, s)
		}},
		{"unknown applier", "applier", func(c *Compiled) { c.GodotSettings.Entries[0].Applier = "absent" }},
		{"missing locale", "value_key", func(c *Compiled) { delete(c.Localization.Messages[1].Texts, "en") }},
		{"large text overflow", "value_key", func(c *Compiled) { c.Localization.Messages[1].Texts["en"] = strings.Repeat("Wide ", 1000) }},
		{"unknown notification", "notification.message", func(c *Compiled) {
			c.GodotSettings.Entries[0].Notification = &SettingNotification{Message: "NoSuchMessage", Field: "locale"}
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root, c, d := settingsFixture(t)
			test.mutate(&c)
			_, err := compileGodotSettings(root, c, d)
			if err == nil || !strings.Contains(err.Error(), "setting ") || !strings.Contains(err.Error(), "field "+test.field+":") {
				t.Fatalf("wanted id and field %s: %v", test.field, err)
			}
		})
	}
}
func TestGodotSettingsDataExtensionParsesAndBadConfigFallsBack(t *testing.T) {
	root, c, d := settingsFixture(t)
	// A second bus is only another data entry and two text keys. No new handler.
	extra := c.GodotSettings.Entries[0]
	extra.ID = "music_volume"
	extra.Key = "music_volume"
	extra.Default = float64(1)
	extra.Params = map[string]any{"bus": "Music"}
	extra.Label = "ui.test.music"
	extra.ValueKey = "ui.test.percent"
	c.Localization.Messages = append(c.Localization.Messages, LocalizationMessage{Key: extra.Label, Texts: map[string]string{"ru": "Music", "en": "Music"}}, LocalizationMessage{Key: extra.ValueKey, Texts: map[string]string{"ru": "{value}%", "en": "{value}%"}})
	c.GodotSettings.Entries = append(c.GodotSettings.Entries, extra)
	raw, err := compileGodotSettings(root, c, d)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "var music_volume: float = 1") {
		t.Fatal("data-only entry did not become typed field")
	}
	// the arrows of a settings card (step) stop a number at its ends; Enter (cycle) goes round
	if !strings.Contains(string(raw), "func step(id: String, direction: int) -> void:") || !strings.Contains(string(raw), "clampi(index+direction, 0, count-1)") {
		t.Fatal("the generated settings have no clamped step")
	}
	bin := os.Getenv("ANG_GODOT_BIN")
	if bin == "" {
		t.Log("set ANG_GODOT_BIN for actual Godot parse and config runtime")
		return
	}
	d.Package = "game.v1"
	d.Services = []normalizer.GRPCService{{Name: "API", Methods: []normalizer.GRPCMethod{{Name: "Call", Request: "ClientMessage", Response: "PocGameState", Stream: "none"}}}}
	for i := range d.Messages {
		for j := range d.Messages[i].Fields {
			d.Messages[i].Fields[j].Number = j + 1
		}
		for j := range d.Messages[i].Oneofs {
			d.Messages[i].Oneofs[j].Name = "msg"
			for k := range d.Messages[i].Oneofs[j].Members {
				d.Messages[i].Oneofs[j].Members[k].Number = len(d.Messages[i].Fields) + k + 1
			}
		}
	}
	contract, err := emitter.RenderGDScriptContract(d, "settings.test")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"generated/settings.gd": raw, "generated/game_contract.gd": contract, "appliers.gd": []byte("extends RefCounted\nstatic func audio_bus(_a,_v,_p,_profiles):pass\n"), "project.godot": []byte("config_version=5\n[application]\nconfig/name=\"Settings compiler fixture\"\nconfig/use_custom_user_dir=true\nconfig/custom_user_dir_name=\"godot-link-private\"\n"), "test.gd": []byte(`extends SceneTree
const Settings=preload("res://generated/settings.gd")
func _initialize():
 var path=OS.get_environment("SETTING_FIXTURE_PATH")
 var file=ConfigFile.new()
 file.set_value("audio","volume","loud")
 file.set_value("audio","unknown",123)
 assert(file.save(path)==OK)
 var a=Settings.new()
 a.load_file(path)
 assert(a.volume==0.0 and a.music_volume==1.0 and a.warnings.size()==2)
 a.cycle("music_volume")
 assert(a.music_volume==0.0)
 a.step("music_volume",-1)
 assert(a.music_volume==0.0)
 a.step("music_volume",1)
 assert(a.music_volume>0.0)
 a.step("music_volume",1)
 a.step("music_volume",1)
 a.step("music_volume",1)
 a.step("music_volume",1)
 a.step("music_volume",1)
 assert(a.music_volume==1.0)
 a.cycle("music_volume")
 assert(a.music_volume==0.0)
 a.set_value("volume",0.25)
 assert(a.save_file(path)==OK)
 var b=Settings.new()
 b.load_file(path)
 assert(b.volume==0.25 and b.music_volume==0.0)
 assert(b.file.get_value("audio","unknown")==123)
 quit()
`)}
	for name, data := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	check := exec.CommandContext(ctx, bin, "--headless", "--path", root, "--audio-driver", "Dummy", "--check-only", "--script", "res://generated/settings.gd")
	checked, checkErr := check.CombinedOutput()
	if checkErr != nil || strings.Contains(string(checked), "ERROR:") {
		t.Fatalf("data-only setting did not parse: %v\n%s", checkErr, checked)
	}
	cmd := exec.CommandContext(ctx, bin, "--headless", "--path", root, "--audio-driver", "Dummy", "--script", "res://test.gd")
	cmd.Env = append(os.Environ(), "SETTING_FIXTURE_PATH="+filepath.Join(root, "settings.cfg"))
	output, err := cmd.CombinedOutput()
	if err != nil || strings.Contains(string(output), "ERROR:") {
		t.Fatalf("generated data-only settings failed Godot: %v\n%s", err, output)
	}
	t.Logf("actual Godot parse/config test: %s", output)
}
