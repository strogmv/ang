package gamepoc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func sceneTemplateSection(t *testing.T, name, start, end string) string {
	t.Helper()
	data, err := sources.ReadFile("templates/" + name)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	a := strings.Index(s, start)
	if a < 0 {
		t.Fatalf("missing %s in %s", start, name)
	}
	b := strings.Index(s[a+len(start):], end)
	if b < 0 {
		t.Fatalf("missing %s after %s", end, start)
	}
	return s[a : a+len(start)+b]
}

func TestSceneDepthTemplateQueuesObservedActorsAndBlocks(t *testing.T) {
	game := sceneTemplateSection(t, "unity.cs.tmpl", "        void DrawGame()", "        void LoadSprites()")
	for _, required := range []string{
		"visibility=='0'", "view.Terrain[y][x]", "if(visibility=='1')texture=RememberedTexture(texture)",
		"Kind=SceneDrawKind.Block", "foreach(Entity e in state.Entities)if(ViewSees(e.X,e.Y))",
		"QueueDemoAgents();", "if(ViewSees(state.PlayerX,state.PlayerY))", "Kind=SceneDrawKind.Hero, Position=shownPosition",
		"DrawSceneItems(drawOrigin,tw,th);",
	} {
		if !strings.Contains(game, required) {
			t.Errorf("missing depth/FOV contract %q", required)
		}
	}
	for _, forbidden := range []string{"DrawCharacter(", "DrawActor(", "DrawDemoAgents(", "for(int pass=0;pass<2;pass++)"} {
		if strings.Contains(game, forbidden) {
			t.Errorf("actors or terrain bypass shared depth pass: %q", forbidden)
		}
	}
	if strings.Index(game, "visibility=='0'") > strings.Index(game, "Kind=SceneDrawKind.Block") {
		t.Fatal("block queued before observation guard")
	}
	if strings.Index(game, "Kind=SceneDrawKind.Hero") > strings.Index(game, "DrawSceneItems(drawOrigin,tw,th);") {
		t.Fatal("hero queued after depth draw")
	}
	queue := sceneTemplateSection(t, "capture.cs.tmpl", "        void QueueDemoAgents()", "        void DrawDemoAgent(")
	if !strings.Contains(queue, "if (!ViewSees(a.X,a.Y)) continue;") || !strings.Contains(queue, "Position=SampleNPC(a), ID=a.Id") {
		t.Fatal("NPC queue must use authoritative visibility and interpolated foot position")
	}
	helpers := sceneTemplateSection(t, "unity.cs.tmpl", "        enum SceneDrawKind", "        void DrawGame()")
	for _, required := range []string{"struct SceneDrawItem", "readonly List<SceneDrawItem>", "static readonly Comparison<SceneDrawItem>", "sceneDrawItems.Sort(sceneDrawComparison)", "sceneDrawItems.Clear();"} {
		if !strings.Contains(helpers, required) {
			t.Errorf("missing reusable buffer contract %q", required)
		}
	}
}

// Execute the actual template comparator, not a Go reimplementation. Unity's Mono
// is optional on compiler-only CI; structural/FOV contracts above always run.
func TestSceneDepthComparatorNative(t *testing.T) {
	mono, err := exec.LookPath("mono")
	if err != nil {
		matches, _ := filepath.Glob("/Applications/Unity/Hub/Editor/*/Unity.app/Contents/Resources/Scripting/MonoBleedingEdge/bin/mono")
		if len(matches) == 0 {
			t.Skip("Mono unavailable; template contracts still tested")
		}
		mono = matches[len(matches)-1]
	}
	csc := filepath.Join(filepath.Dir(mono), "../lib/mono/4.5/csc.exe")
	if _, err := os.Stat(csc); err != nil {
		t.Skip("Unity Mono compiler unavailable")
	}
	comparator := sceneTemplateSection(t, "unity.cs.tmpl", "        enum SceneDrawKind", "        void DrawSceneItems(")
	harness := `using System;
using System.Collections.Generic;
struct Vector2 { public float x,y; public Vector2(float x,float y){this.x=x;this.y=y;} }
class Texture2D {} class Entity {} class PocAgentState {}
class Proof {
` + comparator + `
 static SceneDrawItem Item(SceneDrawKind kind,float x,float y,string id="") { return new SceneDrawItem {Kind=kind,Position=new Vector2(x,y),ID=id}; }
 static void Before(SceneDrawItem a,SceneDrawItem b,string reason) { if(CompareSceneDrawItems(a,b)>=0||CompareSceneDrawItems(b,a)<=0)throw new Exception(reason); }
 public static void Main() {
  var hero=Item(SceneDrawKind.Hero,10,10,"hero");
  Before(hero,Item(SceneDrawKind.Block,10,11),"foreground block must cover hero");
  Before(hero,Item(SceneDrawKind.Block,11,10),"other foreground axis must cover hero");
  Before(Item(SceneDrawKind.Block,10,9),hero,"hero in front of adjacent rear wall");
  Before(Item(SceneDrawKind.Block,9,10),hero,"other rear axis remains behind hero");
  Before(Item(SceneDrawKind.Agent,10.1f,10.1f,"npc"),Item(SceneDrawKind.Block,11,10),"interpolated feet behind foreground block");
  Before(Item(SceneDrawKind.Block,11,10),Item(SceneDrawKind.Agent,11.1f,10.95f,"npc"),"interpolated feet move past footprint edge");
  Before(Item(SceneDrawKind.Agent,10,10,"a"),Item(SceneDrawKind.Agent,10,10,"b"),"stable actor ID tie");
  var original=new List<SceneDrawItem>{hero,Item(SceneDrawKind.Block,11,10),Item(SceneDrawKind.Block,9,10),Item(SceneDrawKind.Agent,10,10,"a"),Item(SceneDrawKind.Agent,10,10,"b")};
  var expected=new List<SceneDrawItem>(original);expected.Sort(sceneDrawComparison);
  var random=new Random(17);
  for(int round=0;round<100;round++) {
   for(int i=original.Count-1;i>0;i--){int j=random.Next(i+1);var temp=original[i];original[i]=original[j];original[j]=temp;}
   original.Sort(sceneDrawComparison);
   for(int i=0;i<original.Count;i++)if(CompareSceneDrawItems(original[i],expected[i])!=0)throw new Exception("unstable input ordering");
  }
  Console.WriteLine("depth ordering PASS");
 }
}
`
	dir := t.TempDir()
	source := filepath.Join(dir, "DepthProof.cs")
	output := filepath.Join(dir, "DepthProof.exe")
	if err := os.WriteFile(source, []byte(harness), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(mono, csc, "/nologo", "/out:"+output, source).CombinedOutput(); err != nil {
		t.Fatalf("compile depth comparator: %v\n%s", err, out)
	}
	if out, err := exec.Command(mono, output).CombinedOutput(); err != nil {
		t.Fatalf("depth comparator: %v\n%s", err, out)
	} else {
		t.Log(strings.TrimSpace(string(out)))
	}
}
