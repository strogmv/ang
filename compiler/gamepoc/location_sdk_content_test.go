package gamepoc

import (
	"os"
	"strings"
	"testing"
)

// demo-hub v4-content: seams of the ring wall and networks of the hub in the generated Unity view.
//   - near cut: a light rim along every outer edge of the cut top (ring wall passport near_cut);
//   - vistas with networks "kit" keep their lights and leave tray, pipes and lamp bodies to the registry batch.
func TestDemoHubContentSeams(t *testing.T) {
	m, err := os.ReadFile("templates/location_sdk_multideck.cs.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	r := string(m)
	if !strings.Contains(r, "LocationSDKMeshObject(\"near cut rim / \"+o.id,rim.Finish()") || !strings.Contains(r, "float rw=.025f") {
		t.Error("near-cut rim missing")
	}
	if !strings.Contains(r, "bool own=v.networks!=\"kit\";") || !strings.Contains(r, "if(own)box(Color.Lerp(lampHot,lampCold,t)") {
		t.Error("vista networks kit mode missing")
	}
	d, err := os.ReadFile("templates/location_sdk.cs.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(d), "public sealed class PocLocationSDKVista {public string id,direction,end,sign_key,networks,reason;") {
		t.Error("vista DTO lacks networks")
	}
}
