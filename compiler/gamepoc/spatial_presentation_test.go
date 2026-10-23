package gamepoc

import (
	"strings"
	"testing"
)

func TestSpatialPresentationCachesAndVisibilityContracts(t *testing.T) {
	data, err := sources.ReadFile("templates/spatial.cs.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		"route_id,symbol", "PocSpatialLight[] lights", "IndexSpatialPresentation(level)",
		"SortedSet<string>(StringComparer.Ordinal)", "links>131072", "list.Count<4", "spatialLightAtlases.Count>=8",
		"void DrawSpatialFloorLight", "if(remembered||Event.current.type!=EventType.Repaint)return;",
		"if(ViewCell(view,x,y)!='2')return", "SpatialLightVisible(view,light.at.x,light.at.y,x,y)",
		"sprite.shape!=\"block\"", "if(nx!=x&&ny!=y&&", "if(hudDebugOpen&&x==light.at.x&&y==light.at.y)",
		"SpatialPaintPoint(at,tw,th,0,.5f+lane)", "SpatialPaintPoint(at,tw,th,1,.5f+lane)",
		"SpatialPaintPoint(at,tw,th,.5f+lane,0)", "SpatialPaintPoint(at,tw,th,.5f+lane,1)",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing presentation contract %s", want)
		}
	}
	markStart := strings.Index(s, "  void DrawSpatialFloorMarks(")
	markEnd := strings.Index(s[markStart:], "  void StartSpatialPreview(")
	marks := s[markStart : markStart+markEnd]
	if strings.Contains(marks, "foreach(var mark in level.decorations)") || !strings.Contains(marks, "index.marks.TryGetValue(y*level.width+x,out list)") {
		t.Fatal("marks must use preindexed current cell")
	}
	lightStart := strings.Index(s, "  void DrawSpatialFloorLight(")
	lightEnd := strings.Index(s[lightStart:], "  static Vector2 SpatialPaintPoint(")
	lights := s[lightStart : lightStart+lightEnd]
	if strings.Contains(lights, "new List<") || strings.Contains(lights, "new Texture2D") || !strings.Contains(lights, "index.lights.TryGetValue(y*level.width+x,out list)") {
		t.Fatal("light draw allocates cache or scans level")
	}
}
