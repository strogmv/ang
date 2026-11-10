package gamepoc

import (
	"cuelang.org/go/cue/cuecontext"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func trainMotionFixture() TrainMotionCatalog {
	return TrainMotionCatalog{BoardingDoorGroup: "passenger_far_", BerthDwellSeconds: 20, PrefabResource: "TrainMotion/MarketTrainV1", ClockDomain: "simulation", TrainID: "train.test", StockID: "stock.test", RouteID: "route.test", RouteVersion: 1, SpeedMPS: 3, AccelerationMPS2: .5, BrakingMPS2: .5, BerthDistanceM: 152.4, BerthLevelID: "a", DoorCloseSeconds: 2.8, DoorTravelM: .94, Timetable: &TrainTimetable{PeriodSeconds: 300, NightPeriodSeconds: 3600, DayStartSecond: 21600, DayEndSecond: 79200, BoardingWindowSeconds: 20, FreightOffsetSeconds: 150},
		Points:   []TrainRoutePoint{{-100, 0}, {300, 0}, {300, 400}, {-100, 400}, {-100, 0}},
		Modules:  []TrainModule{{"passenger", 0, 2.8, 13.6, -.53, []float64{4, -4}}, {"freight", -14.2, 2.8, 13.6, -.53, []float64{4, -4}}, {"freight2", -28.4, 2.8, 13.6, -.53, []float64{4, -4}}, {"freight3", -42.6, 2.8, 13.6, -.53, []float64{4, -4}}},
		Bindings: []TrainBinding{{LevelID: "a", StartM: 100, EndM: 220}, {LevelID: "b", PassageID: "b.lower", StartM: 500, EndM: 620}, {LevelID: "c", PassageID: "c.lower", StartM: 900, EndM: 1020}}}
}
func TestTrainMotionRejectsInvalidRoutes(t *testing.T) {
	life := json.RawMessage(`{"locations":{"a":{},"b":{},"c":{}}}`)
	valid := trainMotionFixture()
	if err := valid.validate(life); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*TrainMotionCatalog){
		"missing boarding side":   func(c *TrainMotionCatalog) { c.BoardingDoorGroup = "" },
		"unsafe boarding side":    func(c *TrainMotionCatalog) { c.BoardingDoorGroup = "../near" },
		"missing physical stand":  func(c *TrainMotionCatalog) { c.BerthDwellSeconds = 0 },
		"missing timetable":       func(c *TrainMotionCatalog) { c.Timetable = nil },
		"invalid acceleration":    func(c *TrainMotionCatalog) { c.AccelerationMPS2 = 0 },
		"invalid braking":         func(c *TrainMotionCatalog) { c.BrakingMPS2 = math.NaN() },
		"berth outside track":     func(c *TrainMotionCatalog) { c.BerthDistanceM = 300 },
		"raw hint":                func(c *TrainMotionCatalog) { c.NameKey = "Passenger train" },
		"bad cue":                 func(c *TrainMotionCatalog) { c.Bindings[0].ApproachSeconds = 31 },
		"unnamed cue":             func(c *TrainMotionCatalog) { c.Bindings[0].ApproachSeconds = 8 },
		"missing third binding":   func(c *TrainMotionCatalog) { c.Bindings = c.Bindings[:2] },
		"unknown level":           func(c *TrainMotionCatalog) { c.Bindings[0].LevelID = "missing" },
		"duplicate level":         func(c *TrainMotionCatalog) { c.Bindings[1].LevelID = "a" },
		"open route":              func(c *TrainMotionCatalog) { c.Points = c.Points[:len(c.Points)-1] },
		"degenerate segment":      func(c *TrainMotionCatalog) { c.Points[1] = c.Points[0] },
		"non-finite route":        func(c *TrainMotionCatalog) { c.Points[1].X = math.NaN() },
		"infinite binding":        func(c *TrainMotionCatalog) { c.Bindings[0].X = math.Inf(1) },
		"non-finite speed":        func(c *TrainMotionCatalog) { c.SpeedMPS = math.NaN() },
		"entry to exterior train": func(c *TrainMotionCatalog) { c.Bindings[0].EntryAllowed = true },
		"sharp visible corner":    func(c *TrainMotionCatalog) { c.Bindings[0].StartM = 350; c.Bindings[0].EndM = 470 },
		"duplicate module":        func(c *TrainMotionCatalog) { c.Modules[1].ID = c.Modules[0].ID },
		"missing body width":      func(c *TrainMotionCatalog) { c.Modules[0].WidthM = 0 },
		"invalid body width":      func(c *TrainMotionCatalog) { c.Modules[0].WidthM = math.NaN() },
		"bogie outside body":      func(c *TrainMotionCatalog) { c.Modules[0].BogieOffsetsM[0] = 10 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			c := trainMotionFixture()
			mutate(&c)
			if err := c.validate(life); err == nil {
				t.Fatal("invalid train route accepted")
			}
		})
	}
}

func TestTrainMotionFleetValidation(t *testing.T) {
	life := json.RawMessage(`{"locations":{"a":{},"b":{},"c":{}}}`)
	fixture := func() TrainMotionCatalog {
		c := trainMotionFixture()
		c.AdditionalFleet = []TrainMotionStock{{PrefabResource: "TrainMotion/FreightTrainV2", TrainID: "train.freight", StockID: "stock.freight"}}
		return c
	}
	valid := fixture()
	if err := valid.validate(life); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*TrainMotionCatalog){
		"duplicate train": func(c *TrainMotionCatalog) { c.AdditionalFleet[0].TrainID = c.TrainID },
		"duplicate stock": func(c *TrainMotionCatalog) { c.AdditionalFleet[0].StockID = c.StockID },
		"empty stock":     func(c *TrainMotionCatalog) { c.AdditionalFleet[0].StockID = "" },
		"unsafe resource": func(c *TrainMotionCatalog) { c.AdditionalFleet[0].PrefabResource = "../Freight" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			c := fixture()
			mutate(&c)
			if c.validate(life) == nil {
				t.Fatal("unsafe shared fleet accepted")
			}
		})
	}
	raw, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	cue := cuecontext.New().CompileString("train_motion: " + string(raw))
	decoded, err := readTrainMotion(cue, life)
	if err != nil || len(decoded.AdditionalFleet) != 1 {
		t.Fatalf("fleet CUE decode: %v", err)
	}
}

func TestTrainMotionPrefabResourcesPath(t *testing.T) {
	life := json.RawMessage(`{"locations":{"a":{},"b":{},"c":{}}}`)
	for _, path := range []string{"", "/TrainMotion/Model", "../Model", "TrainMotion/../Model", "TrainMotion/Model.prefab", "TrainMotion\\Model", strings.Repeat("x", 257)} {
		c := trainMotionFixture()
		c.PrefabResource = path
		if err := c.validate(life); err == nil {
			t.Fatalf("unsafe prefab accepted: %q", path)
		}
	}
}

func TestTrainMotionCUEStrictDecode(t *testing.T) {
	raw, err := json.Marshal(trainMotionFixture())
	if err != nil {
		t.Fatal(err)
	}
	ctx := cuecontext.New()
	life := json.RawMessage(`{"locations":{"a":{},"b":{},"c":{}}}`)
	valid := ctx.CompileString("train_motion: " + string(raw))
	c, err := readTrainMotion(valid, life)
	if err != nil || c == nil {
		t.Fatalf("valid CUE route: %v", err)
	}
	unknown := ctx.CompileString("train_motion: " + string(raw) + "\ntrain_motion: speed_from_client: true")
	if _, err := readTrainMotion(unknown, life); err == nil {
		t.Fatal("unknown authority field accepted")
	}
}

func TestTrainMotionAuthoredCUE(t *testing.T) {
	project := os.Getenv("ANG_TRAIN_MOTION_PROJECT")
	if project == "" {
		t.Skip("set ANG_TRAIN_MOTION_PROJECT to staged server project")
	}
	c, _, err := readCompiled(project)
	if err != nil {
		t.Fatal(err)
	}
	if c.TrainMotion == nil {
		t.Fatal("authored train missing from compiled world")
	}
	if c.TrainMotion.ClockDomain != "simulation" || c.TrainMotion.SpeedMPS != 8 || len(c.TrainMotion.Modules) != 4 {
		t.Fatal("wrong physical motion contract")
	}
	if c.TrainMotion.RouteVersion != 5 || len(c.TrainMotion.AdditionalFleet) != 1 || c.TrainMotion.TrainID != "train.c7.passenger.01" || c.TrainMotion.PrefabResource != "TrainMotion/PassengerTrainV2" || c.TrainMotion.Timetable == nil {
		t.Fatal("wrong passenger fleet identity/phase")
	}
	freight := c.TrainMotion.AdditionalFleet[0]
	if freight.TrainID != "train.c7.freight.01" || freight.PrefabResource != "TrainMotion/FreightTrainV2" {
		t.Fatal("wrong freight fleet identity/phase")
	}
	for i, id := range []string{"car.01", "car.02", "car.03", "car.04"} {
		if c.TrainMotion.Modules[i].ID != id {
			t.Fatal("fleet car node contract drift")
		}
	}
	if got := c.TrainMotion.Bindings; len(got) != 3 || got[0].LevelID != "level.c7_transport_hub" || got[1].LevelID != "level.c7_market" || got[2].LevelID != "level.poc_housing" {
		t.Fatal("wrong physical views")
	}
	hubNative := false
	for _, b := range c.ModularLocations {
		if b.LevelID == "level.c7_transport_hub" {
			hubNative = true
		}
	}
	hubBinding := c.TrainMotion.Bindings[0]
	if !hubNative || (hubBinding.PassageID != "" || hubBinding.X != 0 || hubBinding.Y != 0 || hubBinding.Z != 27.5 || hubBinding.YawDegrees != 0 || hubBinding.StartM != 100 || hubBinding.EndM != 164 || hubBinding.FloorElevationM != 18) {
		t.Fatal("native hub must bind the real upper platform railway, not the retired lower loop")
	}
	if c.TrainMotion.Bindings[2].PassageID != "housing.lower_rail" {
		t.Fatal("missing explicit rail identity")
	}

	var life struct {
		Edges []struct {
			ID     string `json:"id"`
			Period int64  `json:"period_seconds"`
			Night  int64  `json:"night_period_seconds"`
			Window int64  `json:"boarding_window_seconds"`
			Seats  int    `json:"seats"`
			Travel int64  `json:"travel_seconds"`
			Fare   *struct {
				Amount   int64  `json:"amount_minor"`
				Currency string `json:"currency"`
			} `json:"fare"`
		} `json:"edges"`
	}
	if err := json.Unmarshal(c.Life, &life); err != nil {
		t.Fatal(err)
	}
	seenEdges := 0
	for _, e := range life.Edges {
		if e.ID == "station_work" || e.ID == "work_station" {
			seenEdges++
			if e.Period != 300 || e.Night != 3600 || e.Window != 20 || e.Seats != 4 || e.Travel != 300 {
				t.Fatal("work commute boarding contract changed")
			}
		}
		if e.ID == "station_hub_rail" || e.ID == "hub_station_rail" {
			seenEdges++
			if e.Period != 300 || e.Night != 3600 || e.Seats != 4 || e.Travel != 120 || e.Fare == nil || e.Fare.Amount != 100 || e.Fare.Currency != "CRD" {
				t.Fatal("upper passenger fare/route changed")
			}
		}
	}
	if seenEdges != 4 {
		t.Fatal("legacy commute/hub passenger edges missing")
	}
	metrics := map[string]float64{}
	for _, scene := range c.Art.Painted {
		if scene.LevelID == "level.c7_transport_hub" {
			t.Fatal("native hub retained incompatible painted geometry")
		}
		for _, passage := range scene.AmbientPassages {
			if scene.LevelID == "level.poc_monorail" && passage.ID == "station.back_rail" {
				if passage.PeriodSeconds != 300 || passage.NightPeriodSeconds != 3600 || passage.DwellSeconds != 20 || len(passage.BerthDoors) != 2 || passage.MotionSource == "train_motion" || passage.MetricRail != nil {
					t.Fatal("legacy passenger berth changed")
				}
			}

			if passage.MetricRail != nil {
				if passage.MetricRail.CellMeters != 1 || passage.MetricRail.PathProgress != .5 {
					t.Fatalf("wrong authored rail metre datum: %+v", passage.MetricRail)
				}
				metrics[scene.LevelID] = passage.MetricRail.TrackM
			}
		}
	}
	if metrics["level.poc_housing"] != 60 {
		t.Fatal("missing fixed authored rail datums in actual CUE")
	}
}

// Run the exact server template as a small isolated Go package, not a reimplementation.
func TestTrainMotionRuntimeTrajectory(t *testing.T) {
	dir := t.TempDir()
	source, err := os.ReadFile("templates/train_motion.go.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(trainMotionFixture())
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{"go.mod": "module angelsdemons/server\n\ngo 1.24\n", "train.go": string(source), "runtime.go": `package poc
 type state struct{TrainDispatch trainDispatchState;Epoch string;Tick,PhaseNS int64; Life struct{Seconds int64;Enabled bool}}
 type Runtime struct{world struct{TrainMotion *trainMotionCatalog};state state}
 func(r *Runtime)tickNS()int64{return 100000000}
 func(r *Runtime)simNS()int64{return r.state.Tick*100000000+r.state.PhaseNS}
 `, "train_test.go": strings.Replace(trainMotionRuntimeTests, "FIXTURE", string(raw), 1)}
	dispatch, err := os.ReadFile("templates/train_dispatch.go.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	files["dispatch.go"] = string(dispatch)
	lifeSource, err := os.ReadFile("../behavior/templates/life.go.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	lifeText := string(lifeSource)
	first := strings.Index(lifeText, "type LifeEdge struct")
	last := strings.Index(lifeText, "type LifeStep struct")
	files["internal/behavior/schedule.go"] = "package behavior\ntype Cell struct{X,Y int}\ntype LifeFare struct{}\n" + lifeText[first:last]
	for name, body := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "test", "-count=1", "-v", ".")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("exact runtime template: %v\n%s", err, out)
	}
	t.Log(string(out))
}

const trainMotionRuntimeTests = `package poc
import("encoding/json";"math";"reflect";"testing")
func fixture(t *testing.T)*trainMotionCatalog{t.Helper();var c trainMotionCatalog;if err:=json.Unmarshal([]byte(` + "`FIXTURE`" + `),&c);err!=nil{t.Fatal(err)};c.SpeedMPS=8;c.AccelerationMPS2=2;c.BrakingMPS2=2;c.BerthDistanceM=160;c.DoorCloseSeconds=2.8;c.BerthLevelID="a";c.Timetable=&trainTimetable{PeriodSeconds:300,NightPeriodSeconds:3600,DayStartSecond:21600,DayEndSecond:79200,BoardingWindowSeconds:20,FreightOffsetSeconds:150};c.AdditionalFleet=[]trainMotionStock{{TrainID:"freight",StockID:"freight.stock",PrefabResource:"Freight"}};return &c}
func TestSharedTimetableBoundaries(t *testing.T){c:=fixture(t);for _,pair:=range [][2]int64{{0,0},{1,3600},{21599,21600},{21601,21900},{79199,79200},{79201,82800},{86399,86400}}{if got:=c.Timetable.next(pair[0],false);got!=pair[1]{t.Fatalf("departure %d=%d expected%d",pair[0],got,pair[1])}}}
func TestAllRatesDayNightNoOverlapSavedContinuity(t *testing.T){
 for _,rate:=range []int64{1,10,30}{for _,start:=range []int64{21600,79200}{
  c:=fixture(t);r:=&Runtime{};r.world.TrainMotion=c;r.state.Life.Enabled=true;r.state.Life.Seconds=start
  var previousDistance float64;var previousSpeed float64;var previousRun int64;seen:=map[string]bool{};starts:=0;berths:=0;skipped:=int64(0)
  for tick:=int64(1);tick<=40000;tick++{
   prior:=r.state.Life.Seconds;r.state.Tick=tick;r.state.Life.Seconds+=rate;r.advanceTrainMotion(prior)
   state:=r.state.TrainDispatch;ns:=r.simNS();distance,speed,phase,doors:=c.dispatchPose(state,ns)
   if state.Active!=0{
    seen[phase]=true
    if state.Run!=previousRun{starts++;previousRun=state.Run}else{
     delta:=distance-previousDistance;if delta < -.000001||delta>c.SpeedMPS*.100001{t.Fatalf("position jump rate%d phase%s delta%g",rate,phase,delta)}
     if math.Abs(speed-previousSpeed)>.201{t.Fatalf("speed discontinuity: %g -> %g",previousSpeed,speed)}
    }
    if doors{berths++;if phase!="berthed"||speed!=0||math.Abs(distance-c.BerthDistanceM)>1e-8{t.Fatal("doors outside stationary berth")}}
    previousDistance,previousSpeed=distance,speed
   }
   if tick==1234{raw,_:=json.Marshal(r.state);loaded:=&Runtime{};loaded.world.TrainMotion=c;if err:=json.Unmarshal(raw,&loaded.state);err!=nil{t.Fatal(err)};if !reflect.DeepEqual(r.trainMotionSnapshot(),loaded.trainMotionSnapshot()){t.Fatal("save/load changed pose")}}
   if tick%127==0{before:=r.state;one,two:=r.trainMotionSnapshot(),r.trainMotionSnapshot();if !reflect.DeepEqual(one,two)||!reflect.DeepEqual(before,r.state){t.Fatal("observer advanced train")}}
   if state.Skipped<skipped{t.Fatal("skip count reversed")};skipped=state.Skipped
  }
  if starts<2{t.Fatalf("trains absent rate%d start%d phases%v",rate,start,seen)}
  if rate==1&&(!seen["braking"]||!seen["berthed"]||!seen["accelerating"]||berths==0){t.Fatal("passenger stop incomplete")}
  if rate>1&&skipped==0{t.Fatal("overdue departures not skipped")}
 }}
}
func TestOneSavedTrainPositivelyTraversesAllThreeViews(t *testing.T){
 c:=fixture(t);seen:=[]string{};last:="";prior:=-1.0
 for distance:=0.0;distance<c.length();distance+=.25{
  state:=c.render(distance,c.SpeedMPS,"passing",1,false,true)
  if state.TrainID!=c.TrainID||state.StockID!=c.StockID||state.PrefabResource!=c.PrefabResource||len(state.Modules)!=4||len(state.PassengerActorIDs)!=0||len(state.CargoContainerIDs)!=0{t.Fatal("persistent stock identity or logistics changed")}
  if state.WheelTravelM<prior{t.Fatal("wheel travel reversed within accepted run")};prior=state.WheelTravelM
  for _,binding:=range state.Modules[0].Bindings{if binding.Visible&&binding.LevelID!=last{seen=append(seen,binding.LevelID);last=binding.LevelID}}
 }
 if !reflect.DeepEqual(seen,[]string{"a","b","c"}){t.Fatalf("actual positive three-view coverage/order: %v",seen)}
}
func TestIndependentCarAndBogieTangentsOnTheSavedRoute(t *testing.T){
 c:=fixture(t);s:=c.render(401,c.SpeedMPS,"passing",1,false,true)
 front,rear:=s.Modules[0].Bindings[0],s.Modules[1].Bindings[0]
 if front.YawDegrees==rear.YawDegrees{t.Fatal("cars were moved as one rigid root at a hidden connector")}
 if front.Bogies[0].YawDegrees==front.Bogies[1].YawDegrees{t.Fatal("bogies lost independently sampled tangents")}
 if math.Abs(front.Bogies[0].Y+.53)>1e-9{t.Fatal("authored bogie datum lost")}
 for i,module:=range s.Modules{expected:=trainWrap(s.DistanceM+c.Modules[i].OffsetM,c.length());if math.Abs(module.DistanceM-expected)>1e-9||module.ID!=c.Modules[i].ID{t.Fatal("module offset or identity lost")}}
}
func TestNoAcceptedFreightSlotHasNoInventedArrival(t *testing.T){
 c:=fixture(t);s:=trainDispatchState{Initialized:true,Rate:300};s.Next[0]=c.Timetable.next(21600,false);s.Next[1]=c.Timetable.next(21600,true)
 if c.nextStartSeconds(s,1,0,21600)>=0{t.Fatal("fixture unexpectedly has a freight slot")}
 if eta:=c.arrivalETA(s,1,0,21600,0,93.2);eta!=-1{t.Fatalf("unavailable run acquired phantom ETA %g",eta)}
}
func TestPhysicalStandIsTwentySecondsAtEveryCalendarRate(t *testing.T){
 for _,rate:=range []int64{1,10,30}{for _,calendar:=range []int64{21600,79200}{
  c:=fixture(t);r:=&Runtime{};r.world.TrainMotion=c;r.state.Life.Enabled=true;r.state.Life.Seconds=calendar
  opened:=map[int64]int64{};closed:=map[int64]bool{};stands:=0
  for tick:=int64(1);tick<=18000;tick++{
   previous:=r.state.Life.Seconds;r.state.Tick=tick;r.state.Life.Seconds+=rate;r.advanceTrainMotion(previous)
   s:=r.state.TrainDispatch;if s.Active!=1{continue};_,_,phase,doors:=c.dispatchPose(s,r.simNS())
   if doors{if _,ok:=opened[s.Run];!ok{opened[s.Run]=r.simNS()}}
   if phase=="closing"&&!closed[s.Run]{start,ok:=opened[s.Run];if !ok{t.Fatal("closing without observed physical stand")};duration:=float64(r.simNS()-start)/1e9;if math.Abs(duration-c.BerthDwellSeconds)>.100001{t.Fatalf("rate%d stand=%g",rate,duration)};stands++;closed[s.Run]=true}
  }
  if stands<2{t.Fatalf("rate%d calendar%d only%d stands",rate,calendar,stands)}
 }}
 c:=fixture(t);s:=trainDispatchState{Active:1,Initialized:true,Run:1,StartNS:100000000000,LastNS:100000000000,Rate:10};s.DepartNS=s.StartNS+int64((c.approachSeconds()+c.BerthDwellSeconds)*1e9)
 beforeD,beforeV,beforePhase,beforeDoors:=c.dispatchPose(s,s.DepartNS-1)
 for _,rate:=range []float64{10,100,300}{s.Rate=rate;d,v,phase,doors:=c.dispatchPose(s,s.DepartNS-1);if d!=beforeD||v!=beforeV||phase!=beforePhase||doors!=beforeDoors{t.Fatal("rate changed fixed visible stand")}}
}
func TestSlowerClockKeepsSkippedFutureStartsConsumed(t *testing.T){
 c:=fixture(t);r:=&Runtime{};r.world.TrainMotion=c;r.state.Life.Enabled=true;r.state.Life.Seconds=28800
 step:=func(rate int64){prior:=r.state.Life.Seconds;r.state.Tick++;r.state.Life.Seconds+=rate;r.advanceTrainMotion(prior)}
 for i:=0;i<4000&&r.state.TrainDispatch.Active!=1;i++{step(1)}
 if r.state.TrainDispatch.Active!=1{t.Fatal("ordinary daytime run absent")}
 accepted:=r.state.TrainDispatch;step(30)
 skipped:=r.state.TrainDispatch.Skipped;cursor:=r.state.TrainDispatch.Next[0]
 if skipped<=accepted.Skipped{t.Fatal("fast clock did not consume overlapping future starts")}
 // The owner keeps skipped slots skipped. Slowing down must not put these
 // future departure slots back in a queue or reset the active physical run.
 nextStartGame:=float64(cursor)-c.leadSeconds(false,10)
 if nextStartGame<=float64(r.state.Life.Seconds)+c.length()/c.SpeedMPS*10{t.Fatal("fixture lacks the real slowdown setup gap")}
 found:=false
 for i:=0;i<18000;i++{
  before:=r.state.TrainDispatch;oldD,_,_,_:=c.dispatchPose(before,r.simNS());step(1);after:=r.state.TrainDispatch
  if after.Skipped<skipped{t.Fatal("skipped slots were restored")};skipped=after.Skipped
  if after.Run==accepted.Run&&after.Active!=0{d,_,_,_:=c.dispatchPose(after,r.simNS());if d<oldD-.000001||d-oldD>c.SpeedMPS*.100001{t.Fatal("rate change moved an accepted run discontinuously")}}
  if after.Next[0]<cursor{t.Fatal("passenger cursor moved backwards")}
  if after.Run>accepted.Run&&after.Active!=0&&after.StartNS<c.finishNS(accepted){t.Fatal("slow clock overlapped the accepted run")}
  if after.Run>accepted.Run&&after.Active==1{
   if float64(r.state.Life.Seconds)<nextStartGame-.000001||after.Departure<cursor{t.Fatal("slow clock replayed an already skipped passenger slot")};found=true;break
  }
 }
 if !found{t.Fatal("train never returned after its persisted skipped-slot cursor")}
}
func TestGeometryHasOnlyDeclaredVisibleSlices(t *testing.T){c:=fixture(t);for d:=0.0;d<c.length();d+=.25{state:=c.render(d,8,"moving",1,false,true);for _,m:=range state.Modules{for i,b:=range m.Bindings{if b.PassageID!=c.Bindings[i].PassageID{t.Fatal("binding identity lost")};if b.Visible{p:=c.sample(m.DistanceM,c.length());if p.YawDegrees!=0&&p.YawDegrees!=90&&p.YawDegrees!=180&&p.YawDegrees!=-90{t.Fatal("offtrack yaw")}}}}};idle:=c.render(0,0,"idle",1,false,false);for _,m:=range idle.Modules{for _,b:=range m.Bindings{if b.Visible{t.Fatal("idle train drawn")}}}}
func TestPhysicalBerthCoversWholeLifeWindow(t *testing.T){c:=fixture(t);s:=trainDispatchState{Active:1,StartNS:0,DepartNS:int64((c.approachSeconds()+20)*1e9)};for at:=c.approachSeconds();at<c.approachSeconds()+20;at+=.01{d,v,phase,doors:=c.dispatchPose(s,int64(at*1e9));if math.Abs(d-c.BerthDistanceM)>1e-8||v!=0||phase!="berthed"||!doors{t.Fatalf("boarding window lacks stopped train at%g",at)}}}
`

func TestTrainPaintedBindingIdentityAndBerthProtection(t *testing.T) {
	floor := 18.0
	scene := PaintedScene{LevelID: "b", FloorElevationM: &floor, AmbientPassages: []PaintedAmbientPassage{{ID: "lower", MotionSource: "train_motion", ElevationM: 12, MetricRail: &PaintedRailMetric{CellMeters: 1, TrackM: 30, PathProgress: .5}}}}
	c := trainMotionFixture()
	c.Bindings[2].PassageID = ""
	c.Bindings[1].PassageID = "lower"
	c.Bindings[1].Y = -6
	if err := c.validatePainted([]PaintedScene{scene}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"missing", "wrong", "native", "berth", "elevation", "unowned", "duplicate"} {
		t.Run(name, func(t *testing.T) {
			test := c
			test.Bindings = append([]TrainBinding(nil), c.Bindings...)
			s := scene
			s.AmbientPassages = append([]PaintedAmbientPassage(nil), scene.AmbientPassages...)
			switch name {
			case "missing":
				test.Bindings[1].PassageID = ""
			case "wrong":
				test.Bindings[1].PassageID = "old.berth"
			case "native":
				test.Bindings[0].PassageID = "lower"
			case "berth":
				s.AmbientPassages[0].StopProgress = .5
				s.AmbientPassages[0].DwellSeconds = 20
			case "elevation":
				test.Bindings[1].Y = 12
			case "unowned":
				s.AmbientPassages = append(s.AmbientPassages, PaintedAmbientPassage{ID: "unowned", MotionSource: "train_motion"})
			case "duplicate":
				s.AmbientPassages = append(s.AmbientPassages, s.AmbientPassages[0])
			}
			if err := test.validatePainted([]PaintedScene{s}); err == nil {
				t.Fatal("invalid rail ownership accepted")
			}
		})
	}
}

func TestExternalRailHasNoIndependentCalendarOrSprite(t *testing.T) {
	c := paintedAmbientFixture()
	s := &c.Art.Painted[0]
	old := s.AmbientPassages[0]
	p := PaintedAmbientPassage{ID: old.ID, MotionSource: "train_motion", MetricRail: &PaintedRailMetric{CellMeters: 1, TrackM: 30, PathProgress: .5}, Path: old.Path, VisibleStrip: old.VisibleStrip, ObservationCells: old.ObservationCells, ElevationM: 12}
	s.AmbientPassages = []PaintedAmbientPassage{p}
	if err := checkPaintedAmbient(*s, c.Walkable); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"sprite", "period", "berth", "unknown_source", "no_metric"} {
		test := p
		switch name {
		case "sprite":
			test.Image = "MarketArt/train"
		case "period":
			test.PeriodSeconds = 300
		case "berth":
			test.DwellSeconds = 20
		case "unknown_source":
			test.MotionSource = "client_clock"
		case "no_metric":
			test.MetricRail = nil
		}
		s.AmbientPassages[0] = test
		if err := checkPaintedAmbient(*s, c.Walkable); err == nil {
			t.Fatalf("external rail accepted %s", name)
		}
	}
}
