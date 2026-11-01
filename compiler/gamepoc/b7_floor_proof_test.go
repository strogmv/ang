package gamepoc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestB7ObservedInputRetainsDistantDestinationAroundVisibleBodies(t *testing.T) {
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
	route := sceneTemplateSection(t, "copper_route_proof.cs.tmpl", "  static bool CopperRouteObservedStep(", "  static bool CopperRouteIdleActivity(")
	segment := sceneTemplateSection(t, "painted_proof.cs.tmpl", "  static bool B7ObservedSegment(", "  bool B7SendObserved(")
	source := `using System;class Proof {` + route + segment + `
 static void Require(bool ok,string reason){if(!ok)throw new Exception(reason);}
 public static void Main(){int x,y;
  var floor=new[]{"#######","#.....#","#.#...#","#.....#","#.....#","#.....#","#######"};
  var seen=new[]{"2222222","2222222","2222222","2222222","2222222","2222222","2222222"};
  Require(B7ObservedSegment(floor,seen,2,1,2,5,out x,out y)&&x==3&&y==1,"occupied adjacent body became the goal instead of the bypass toward the distant destination");
  Require(B7ObservedSegment(floor,seen,3,1,2,5,out x,out y)&&x==3&&y>=3,"long clear aisle was reduced to repeated adjacent requests");
  var unknown=new[]{"2222222","2222222","2220222","2220222","2220222","2222222","2222222"};
  Require(B7ObservedSegment(floor,unknown,2,1,2,5,out x,out y)&&x==1&&y==1,"unobserved shortcut entered instead of the known bypass");
  var sealedFloor=new[]{"#######","##.####","#######","#######","#######","##.####","#######"};
  Require(!B7ObservedSegment(sealedFloor,seen,2,1,2,5,out x,out y),"closed crowd barrier became a fake walkable path");
  Console.WriteLine("observed body bypass, distant destination retained, straight segment, unknown-cell exclusion and blocked wait PASS");
 }};`
	dir := t.TempDir()
	cs := filepath.Join(dir, "B7Input.cs")
	exe := filepath.Join(dir, "B7Input.exe")
	if err := os.WriteFile(cs, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(mono, csc, "/nologo", "/out:"+exe, cs).CombinedOutput(); err != nil {
		t.Fatalf("compile B7 input: %v\n%s", err, out)
	}
	if out, err := exec.Command(mono, exe).CombinedOutput(); err != nil {
		t.Fatalf("B7 input: %v\n%s", err, out)
	} else {
		t.Log(strings.TrimSpace(string(out)))
	}
}
