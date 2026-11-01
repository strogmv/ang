package gamepoc

import (
	"strings"
	"testing"
)

func TestStorefrontActualClockCompositionAndPauseNative(t *testing.T) {
	code := sceneTemplateSection(t, "storefront.cs.tmpl", "  static bool StorefrontOpen(", "  // End pure storefront")
	runPaintedTextureNative(t, "StorefrontClock", `using System;
class PocStorefrontWindow {public long start_second,end_second;}
class PocStorefront {public long open_second,close_second;public int opening_ms,closing_ms;public PocStorefrontWindow[] holiday_windows;}
class Proof {`+code+` static void Need(bool v,string why){if(!v)throw new Exception(why);}static void Near(float a,float b,string why){Need(Math.Abs(a-b)<.0001f,why+" got="+a+" want="+b);}
 public static void Main(){
  var model=new PocStorefront{open_second=25200,close_second=79200,opening_ms=1600,closing_ms=1800,holiday_windows=new[]{new PocStorefrontWindow{start_second=80000,end_second=90000}}};
  Need(!StorefrontOpen(model,25199)&&StorefrontOpen(model,25200)&&StorefrontOpen(model,79199)&&!StorefrontOpen(model,79200),"daily schedule bounds");
  Need(!StorefrontOpen(model,-1)&&StorefrontOpen(model,86400+25200),"day wrap");
  Need(!StorefrontHoliday(model,79999)&&StorefrontHoliday(model,80000)&&StorefrontHoliday(model,89999)&&!StorefrontHoliday(model,90000),"holiday absolute bounds");
  var clock=new StorefrontClock();Need(clock.Sample(model,70000,1e9,false),"first open sample");Near(clock.closed,0,"initial open must snap");
  clock.Sample(model,79200,2e9,false);Near(clock.closed,0,"closing boundary must remain continuous");clock.Sample(model,79200,2.9e9,false);Near(clock.closed,.5f,"half transition");
  Need(!clock.Sample(model,80000,99e9,true),"pause must hold all state including event changes");Near(clock.closed,.5f,"paused pose");Need(!clock.holiday,"paused festival changed");
  clock.Sample(model,79201,3.8e9,false);Near(clock.closed,1,"closed destination");clock.Sample(model,80000,4e9,false);Need(clock.holiday,"real public event not reflected");
  for(int i=0;i<1000;i++)Need(!clock.Sample(model,80000,4e9,false),"stable accepted clock must not allocate phase changes");
  clock.Sample(model,86400+25200,5e9,false);Near(clock.closed,1,"opening boundary must remain continuous");clock.Sample(model,86400+25200,5.8e9,false);Near(clock.closed,.5f,"opening midpoint");clock.Sample(model,86400+25201,6.6e9,false);Near(clock.closed,0,"open destination");
  clock.Sample(model,2*86400+79200,7e9,false);clock.Sample(model,2*86400+79200,7.9e9,false);Near(clock.closed,.5f,"mid closure before reversal");clock.Sample(model,3*86400+25200,7.9e9,false);Near(clock.closed,.5f,"reversal continuous");clock.Sample(model,3*86400+25200,8.3e9,false);Near(clock.closed,.25f,"reversal uses remaining distance duration");clock.Sample(model,3*86400+25200,8.7e9,false);Near(clock.closed,0,"reversed open");
  clock.ForgetTransition();clock.Sample(model,3*86400+80000,9e9,true);Near(clock.closed,1,"new observation/load must snap public appearance without replaying hidden transition");
  var operational=new StorefrontClock();operational.Sample(model,70000,1e9,false,false);Near(operational.closed,1,"absent merchant must remain closed during scheduled hours");
  operational.Sample(model,70000,2e9,false,true);operational.Sample(model,70000,3.6e9,false,true);Near(operational.closed,0,"physical merchant opening drives facade");
  operational.Sample(model,70000,4e9,true,false);Near(operational.closed,0,"paused authoritative closure must not animate");
  var protectedClosed=StorefrontComposition("protected_display",1,true);Near(protectedClosed.goods,1,"protected inventory remains behind shutter");Near(protectedClosed.shutter,1,"protected shutter closed");Near(protectedClosed.festival,1,"closed event composition");
  var packMid=StorefrontComposition("pack_away",.45f,false);Near(packMid.goods,0,"goods pack before shutter starts");Near(packMid.shutter,0,"packing finishes before shutter movement");Need(packMid.goodsDrop>0,"bounded packing slide");var packClosed=StorefrontComposition("pack_away",1,true);Near(packClosed.goods,0,"packed display stays hidden");Near(packClosed.shutter,1,"packed shutter closes");Near(packClosed.festival,1,"holiday independent of stock policy");
  model.open_second=79200;model.close_second=25200;Need(StorefrontOpen(model,86399)&&StorefrontOpen(model,0)&&StorefrontOpen(model,25199)&&!StorefrontOpen(model,25200)&&!StorefrontOpen(model,79199)&&StorefrontOpen(model,79200),"overnight schedule");
  Console.WriteLine("Storefront actual-template clock: daily/overnight/midnight, half-open events, pause, loading/re-observation, opening/closing reversal and independent packing/protected/festival composition PASS");
 }
}`)
	// StorefrontLayers is a C# value-type struct, so its initializer is not a
	// managed allocation. Other new-expressions in the sampling code remain forbidden.
	allocationChecked := strings.ReplaceAll(code, "new StorefrontLayers", "StorefrontLayers")
	for _, forbidden := range []string{"Time.", "DateTime", "new ", "InventoryCommand", "state.Inventory", "LifeCommand"} {
		if strings.Contains(allocationChecked, forbidden) {
			t.Fatalf("presentation clock must not mutate gameplay or allocate each frame: %s", forbidden)
		}
	}
}

func TestStorefrontUsesOpaquePanelsAndObservedRetainedResources(t *testing.T) {
	shader := hudTemplate(t, "storefront.shader.tmpl")
	for _, needed := range []string{"lerp(color.a,1,opaque)*valid", "i.uv,1)", "i.uv+float2(0,1-_State.x),1)", "festival.a*=_State.w", "_FogTex"} {
		if !strings.Contains(shader, needed) {
			t.Fatalf("missing facade compositing contract: %s", needed)
		}
	}
	draw := sceneTemplateSection(t, "storefront.cs.tmpl", "  void DrawStorefronts(", "\n }\n}")
	for _, forbidden := range []string{"new Material", "new Mesh", "new RenderTexture", "new Texture2D", "GetPixels", "ReadPixels"} {
		if strings.Contains(draw, forbidden) {
			t.Fatalf("ordinary facade repaint must reuse resources without readback: %s", forbidden)
		}
	}
	for _, needed := range []string{"bool observed=seen=='2'", "item.clock.ForgetTransition()", "if(observed&&authority!=null)", "authority.Open", "authority.Stocked", "if(!stocked)item.layers.goods=0", "state.GameSeconds,renderedSimNS,state.Paused", "item.everObserved", "storefrontFogVersion!=painted.fogVersion", "RenderTexture.active=previous", "GL.sRGBWrite=srgb"} {
		if !strings.Contains(draw, needed) {
			t.Fatalf("missing observed facade lifecycle: %s", needed)
		}
	}
	painted := hudTemplate(t, "painted.cs.tmpl")
	architecture := strings.Index(painted, "foreach(var draw in paintedDraws)if(draw.kind==2)")
	facade := strings.Index(painted, "DrawStorefronts(scene,cache,fit,viewport)")
	actor := strings.Index(painted, "foreach(var draw in paintedDraws)if(draw.kind!=2)")
	if architecture < 0 || facade <= architecture || actor <= facade {
		t.Fatal("facades must compose after original architecture and before clipped actors")
	}
}
