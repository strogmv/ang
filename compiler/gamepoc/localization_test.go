package gamepoc

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func localizationFixture() LocalizationCatalog {
	return LocalizationCatalog{
		DefaultLocale: "ru", FallbackLocale: "en", Locales: []string{"ru", "en"},
		Messages: []LocalizationMessage{
			{Key: "region.node.secret_room.name", Context: "Private story context", Texts: map[string]string{"ru": "SECRET_ROOM_RU", "en": "SECRET_ROOM_EN"}},
			{Key: "ui.region.route_notice", Context: "Private authoring context must not be bundled", Texts: map[string]string{"ru": "RU_ROUTE {name}", "en": "EN_ROUTE {name}"}},
			{Key: "ui.language.label", Context: "Language picker", Texts: map[string]string{"ru": "RU_LANGUAGE", "en": "EN_LANGUAGE"}},
		},
	}
}

func TestLocalizationCatalogRejectsInvalidTranslationsAndPlaceholders(t *testing.T) {
	if err := (LocalizationCatalog{}).Validate(); err != nil {
		t.Fatal("legacy empty catalog rejected", err)
	}
	for _, test := range []struct {
		name string
		bad  bool
		edit func(*LocalizationCatalog)
	}{
		{"valid", false, func(*LocalizationCatalog) {}},
		{"fallback_translation", false, func(c *LocalizationCatalog) { delete(c.Messages[2].Texts, "ru") }},
		{"duplicate_locale", true, func(c *LocalizationCatalog) { c.Locales = append(c.Locales, "ru") }},
		{"invalid_locale", true, func(c *LocalizationCatalog) { c.Locales[0] = "not a locale" }},
		{"missing_default", true, func(c *LocalizationCatalog) { c.DefaultLocale = "de" }},
		{"missing_fallback", true, func(c *LocalizationCatalog) { c.FallbackLocale = "de" }},
		{"duplicate_key", true, func(c *LocalizationCatalog) { c.Messages = append(c.Messages, c.Messages[0]) }},
		{"empty_context", true, func(c *LocalizationCatalog) { c.Messages[0].Context = "  " }},
		{"invalid_key", true, func(c *LocalizationCatalog) { c.Messages[0].Key = "not a stable key" }},
		{"unregistered_text_locale", true, func(c *LocalizationCatalog) { c.Messages[0].Texts["de"] = "Other" }},
		{"empty_translation", true, func(c *LocalizationCatalog) { c.Messages[0].Texts["en"] = " " }},
		{"no_translations", true, func(c *LocalizationCatalog) { c.Messages[0].Texts = nil }},
		{"missing_placeholder", true, func(c *LocalizationCatalog) { c.Messages[1].Texts["en"] = "EN_ROUTE" }},
		{"renamed_placeholder", true, func(c *LocalizationCatalog) { c.Messages[1].Texts["en"] = "EN_ROUTE {destination}" }},
		{"unmatched_brace", true, func(c *LocalizationCatalog) { c.Messages[1].Texts["en"] = "EN_ROUTE {name" }},
		{"escaped_braces", false, func(c *LocalizationCatalog) {
			c.Messages[1].Texts = map[string]string{"ru": "{{literal}} {name}", "en": "{name}: {{literal}}"}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			catalog := localizationFixture()
			test.edit(&catalog)
			if err := catalog.Validate(); (err != nil) != test.bad {
				t.Fatalf("bad=%v: %v", test.bad, err)
			}
		})
	}
}

func TestClientLocalizationIsSortedAndNeverContainsPrivateWorldText(t *testing.T) {
	catalog := localizationFixture()
	first := clientLocalization(catalog)
	raw, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"secret_room", "SECRET_ROOM", "Private authoring context", "Private story context", "region.node", "context"} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatalf("private localization data entered client resource: %s", secret)
		}
	}
	if len(first.Texts) != 4 || first.DefaultLocale != "ru" || first.FallbackLocale != "en" {
		t.Fatal("public interface translations missing")
	}
	for i, row := range first.Texts {
		if !strings.HasPrefix(row.Key, "ui.") {
			t.Fatal("nonpublic key exported")
		}
		if i > 0 && (first.Texts[i-1].Locale > row.Locale || first.Texts[i-1].Locale == row.Locale && first.Texts[i-1].Key > row.Key) {
			t.Fatal("client rows are not ordered by locale/key")
		}
	}
	catalog.Messages[0], catalog.Messages[2] = catalog.Messages[2], catalog.Messages[0]
	second, err := json.Marshal(clientLocalization(catalog))
	if err != nil || !bytes.Equal(raw, second) {
		t.Fatal("authoring order changed public resource bytes")
	}
	first.Locales[0] = "changed"
	first.Texts[0].Text = "changed"
	if catalog.Locales[0] != "ru" || clientLocalization(catalog).Texts[0].Text == "changed" {
		t.Fatal("client projection aliases private authoring")
	}
}

func TestClientResourceFiltersPrivateLocalizationCatalog(t *testing.T) {
	catalog, err := json.Marshal(localizationFixture())
	if err != nil {
		t.Fatal(err)
	}
	var compiled Compiled
	if err := json.Unmarshal(append(append([]byte(`{"localization":`), catalog...), '}'), &compiled); err != nil {
		t.Fatal(err)
	}
	raw, err := clientResource(compiled)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte("EN_LANGUAGE")) || !bytes.Contains(raw, []byte("RU_LANGUAGE")) {
		t.Fatal("public UI catalog not connected to client resource")
	}
	if bytes.Contains(raw, []byte("secret_room")) || bytes.Contains(raw, []byte("SECRET_ROOM")) || bytes.Contains(raw, []byte("Private authoring context")) {
		t.Fatal("full private localization catalog was serialized into Unity")
	}
}

func TestLocalizationGeneratedGoFallbackAndKeyedErrors(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":          "module localization-check\n\ngo 1.24\n",
		"localization.go": hudTemplate(t, "localization.go.tmpl"),
		"runtime_test.go": `package poc
import("errors";"fmt";"testing")
type Runtime struct{world struct{Localization localizationCatalog}}
func TestActualLocalizedRuntime(t *testing.T){
 r:=&Runtime{};r.world.Localization=localizationCatalog{DefaultLocale:"ru",FallbackLocale:"en",Locales:[]string{"ru","en"},Messages:[]localizationMessage{
  {Key:"region.node.room.name",Texts:map[string]string{"ru":"RU_NAME","en":"EN_NAME"}},
  {Key:"ui.fallback",Texts:map[string]string{"en":"EN_FALLBACK"}},
  {Key:"ui.default",Texts:map[string]string{"ru":"RU_DEFAULT"}},
  {Key:"error.route_unknown",Texts:map[string]string{"ru":"RU_ERROR","en":"EN_ERROR"}},
 }}
 for _,test:=range []struct{locale,key,fallback,want string}{
  {"en","region.node.room.name","","EN_NAME"},{"ru","region.node.room.name","","RU_NAME"},
  {"invalid","region.node.room.name","","RU_NAME"},{"ru","ui.fallback","","EN_FALLBACK"},
  {"en","ui.default","","RU_DEFAULT"},{"en","missing.key","LEGACY","LEGACY"},
  {"en","missing.key","","missing.key"},
 }{if got:=r.localText(test.locale,test.key,test.fallback);got!=test.want{t.Fatalf("%+v got %q",test,got)}}
 for _,locale:=range []string{"","invalid","ru"}{if r.validLocale(locale)!="ru"{t.Fatal("unknown locale did not use authored default")}}
 if r.validLocale("en")!="en"{t.Fatal("registered locale lost")}
 for _,err:=range []error{textError{Key:"error.route_unknown"},&textError{Key:"error.route_unknown"},fmt.Errorf("wrapper: %w",textError{Key:"error.route_unknown"})}{if r.localError("en",err)!="EN_ERROR"{t.Fatal("keyed error did not localize")}}
 if r.localError("en",nil)!=""||r.localError("en",errors.New("legacy diagnostic"))!="legacy diagnostic"{t.Fatal("legacy/nil error fallback changed")}
 r.world.Localization=localizationCatalog{DefaultLocale:"en",FallbackLocale:"ru",Locales:[]string{"ru","en"},Messages:[]localizationMessage{{Key:"ui.label",Texts:map[string]string{"en":"EN_DEFAULT"}}}}
 if r.validLocale("invalid")!="en"||r.localText("invalid","ui.label","")!="EN_DEFAULT"{t.Fatal("default language was hardcoded")}
}`,
	}
	for name, source := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "test", "-count=1", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("actual generated localization runtime: %v\n%s", err, out)
	}
}

func TestLocalizationCSharpPreferenceAndFallbackNative(t *testing.T) {
	mono, err := exec.LookPath("mono")
	if err != nil {
		matches, _ := filepath.Glob("/Applications/Unity/Hub/Editor/*/Unity.app/Contents/Resources/Scripting/MonoBleedingEdge/bin/mono")
		if len(matches) == 0 {
			t.Skip("Mono unavailable")
		}
		mono = matches[len(matches)-1]
	}
	csc := filepath.Join(filepath.Dir(mono), "../lib/mono/4.5/csc.exe")
	if _, err := os.Stat(csc); err != nil {
		t.Skip("Mono compiler unavailable")
	}
	harness := `namespace UnityEngine {
 public static class PlayerPrefs {static readonly System.Collections.Generic.Dictionary<string,string> data=new System.Collections.Generic.Dictionary<string,string>();public static string GetString(string key,string fallback){string value;return data.TryGetValue(key,out value)?value:fallback;}public static void SetString(string key,string value){data[key]=value;}public static void Save(){}}
}
namespace AngelsDemons.Poc {
 public sealed partial class AngelsDemonsPoc {
  sealed class Resource {public PocLocalization localization;}
  Resource resource;int sent;void SendLocalePreference(){sent++;}
  static void Equal(string a,string b){if(a!=b)throw new System.Exception(a+" != "+b);}
  public static void Main(){
   var catalog=new PocLocalization{default_locale="ru",fallback_locale="en",locales=new[]{"ru","en"},texts=new[]{
    new PocLocaleText{locale="ru",key="ui.label",text="RU_LABEL"},new PocLocaleText{locale="en",key="ui.label",text="EN_LABEL"},
    new PocLocaleText{locale="en",key="ui.fallback",text="EN_FALLBACK"},new PocLocaleText{locale="en",key="ui.number",text="Value {0}"},
    new PocLocaleText{locale="en",key="region.node.secret.name",text="SECRET_NAME"}
   }};
   var app=new AngelsDemonsPoc{resource=new Resource{localization=catalog}};
   Equal(app.CurrentLocale(),"ru");Equal(app.L("ui.label"),"RU_LABEL");Equal(app.L("ui.fallback"),"EN_FALLBACK");
   app.SetLocale("en");if(app.sent!=1)throw new System.Exception("locale preference was not sent");Equal(app.L("ui.label"),"EN_LABEL");Equal(app.L("ui.number",1.5),"Value 1.5");
   Equal(app.L("region.node.secret.name"),"region.node.secret.name");Equal(app.L("missing.key"),"missing.key");
   var restarted=new AngelsDemonsPoc{resource=new Resource{localization=catalog}};Equal(restarted.CurrentLocale(),"en");
   restarted.SetLocale("invalid");Equal(restarted.CurrentLocale(),"ru");Equal(restarted.L("ui.label"),"RU_LABEL");
   restarted.resource.localization=new PocLocalization{default_locale="ru",fallback_locale="en",locales=new[]{"ru","en"},texts=new[]{new PocLocaleText{locale="ru",key="ui.label",text="RELOADED"}}};
   Equal(restarted.L("ui.label"),"RELOADED");System.Console.WriteLine("public localization/preference/fallback/cache PASS");
  }
 }
}`
	dir := t.TempDir()
	source, stub, binary := filepath.Join(dir, "Localization.cs"), filepath.Join(dir, "Harness.cs"), filepath.Join(dir, "Localization.exe")
	for name, contents := range map[string]string{source: hudTemplate(t, "localization.cs.tmpl"), stub: harness} {
		if err := os.WriteFile(name, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := exec.Command(mono, csc, "/nologo", "/out:"+binary, source, stub).CombinedOutput(); err != nil {
		t.Fatalf("compile actual localization C#: %v\n%s", err, out)
	}
	if out, err := exec.Command(mono, binary).CombinedOutput(); err != nil {
		t.Fatalf("localization C#: %v\n%s", err, out)
	} else {
		t.Log(strings.TrimSpace(string(out)))
	}
}
