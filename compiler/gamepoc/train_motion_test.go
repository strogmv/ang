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
	return TrainMotionCatalog{PrefabResource: "TrainMotion/MarketTrainV1", ClockDomain: "simulation", TrainID: "train.test", StockID: "stock.test", RouteID: "route.test", RouteVersion: 1, EpochSeconds: 0, InitialDistanceM: 152.4, SpeedMPS: 3, DwellSeconds: 12,
		Points:   []TrainRoutePoint{{-100, 0}, {300, 0}, {300, 400}, {-100, 400}, {-100, 0}},
		Modules:  []TrainModule{{"passenger", 0, 13.6, -.53, []float64{4, -4}}, {"freight", -14.2, 13.6, -.53, []float64{4, -4}}, {"freight2", -28.4, 13.6, -.53, []float64{4, -4}}, {"freight3", -42.6, 13.6, -.53, []float64{4, -4}}},
		Bindings: []TrainBinding{{LevelID: "a", StartM: 100, EndM: 220}, {LevelID: "b", PassageID: "b.lower", StartM: 500, EndM: 620}, {LevelID: "c", PassageID: "c.lower", StartM: 900, EndM: 1020}}}
}
func TestTrainMotionRejectsInvalidRoutes(t *testing.T) {
	life := json.RawMessage(`{"locations":{"a":{},"b":{},"c":{}}}`)
	valid := trainMotionFixture()
	if err := valid.validate(life); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*TrainMotionCatalog){
		"missing third binding":    func(c *TrainMotionCatalog) { c.Bindings = c.Bindings[:2] },
		"unknown level":            func(c *TrainMotionCatalog) { c.Bindings[0].LevelID = "missing" },
		"duplicate level":          func(c *TrainMotionCatalog) { c.Bindings[1].LevelID = "a" },
		"open route":               func(c *TrainMotionCatalog) { c.Points = c.Points[:len(c.Points)-1] },
		"degenerate segment":       func(c *TrainMotionCatalog) { c.Points[1] = c.Points[0] },
		"non-finite route":         func(c *TrainMotionCatalog) { c.Points[1].X = math.NaN() },
		"infinite binding":         func(c *TrainMotionCatalog) { c.Bindings[0].X = math.Inf(1) },
		"non-finite speed":         func(c *TrainMotionCatalog) { c.SpeedMPS = math.NaN() },
		"non-finite dwell":         func(c *TrainMotionCatalog) { c.DwellSeconds = math.Inf(1) },
		"invalid initial distance": func(c *TrainMotionCatalog) { c.InitialDistanceM = 1600 },
		"entry to exterior train":  func(c *TrainMotionCatalog) { c.Bindings[0].EntryAllowed = true },
		"sharp visible corner":     func(c *TrainMotionCatalog) { c.Bindings[0].StartM = 350; c.Bindings[0].EndM = 470 },
		"duplicate module":         func(c *TrainMotionCatalog) { c.Modules[1].ID = c.Modules[0].ID },
		"bogie outside body":       func(c *TrainMotionCatalog) { c.Modules[0].BogieOffsetsM[0] = 10 },
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
	if c.TrainMotion.ClockDomain != "simulation" || c.TrainMotion.SpeedMPS != 3 || len(c.TrainMotion.Modules) != 4 {
		t.Fatal("wrong physical motion contract")
	}
	if got := c.TrainMotion.Bindings; len(got) != 3 || got[0].LevelID != "level.c7_market" || got[1].LevelID != "level.c7_transport_hub" || got[2].LevelID != "level.poc_housing" {
		t.Fatal("wrong physical views")
	}
	if c.TrainMotion.Bindings[1].PassageID != "hub.lower_freight_rail" || c.TrainMotion.Bindings[2].PassageID != "housing.lower_rail" {
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
		for _, passage := range scene.AmbientPassages {
			if scene.LevelID == "level.poc_monorail" && passage.ID == "station.back_rail" {
				if passage.PeriodSeconds != 300 || passage.NightPeriodSeconds != 3600 || passage.DwellSeconds != 20 || len(passage.BerthDoors) != 2 || passage.MotionSource == "train_motion" || passage.MetricRail != nil {
					t.Fatal("legacy passenger berth changed")
				}
			}
			if scene.LevelID == "level.c7_transport_hub" && passage.ID == "hub.lower_freight_rail" {
				if passage.MotionSource != "train_motion" || passage.Image != "" || passage.PeriodSeconds != 0 || passage.ElevationM != 12 {
					t.Fatal("hub exterior gained a second sprite/schedule or wrong elevation")
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
	if metrics["level.c7_transport_hub"] != 29.9 || metrics["level.poc_housing"] != 60 {
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
	files := map[string]string{"go.mod": "module trainmotiontest\n\ngo 1.24\n", "train.go": string(source), "runtime.go": `package poc
 type Runtime struct{ world struct{TrainMotion *trainMotionCatalog};state struct{Epoch string;Tick,PhaseNS int64; Life struct{Seconds int64;Enabled bool}} }
 func(r *Runtime)simNS()int64{return r.state.Tick*100000000+r.state.PhaseNS}
 `, "train_test.go": strings.Replace(trainMotionRuntimeTests, "FIXTURE", string(raw), 1)}
	for name, body := range files {
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
func fixture(t *testing.T)*trainMotionCatalog{t.Helper();var c trainMotionCatalog;if err:=json.Unmarshal([]byte(` + "`FIXTURE`" + `),&c);err!=nil{t.Fatal(err)};return &c}
func TestExplicitPassageIDSurvivesServerSnapshot(t *testing.T){c:=fixture(t);v:=c.snapshot(100);for _,m:=range v.Modules {for i,b:=range m.Bindings {if b.PassageID!=c.Bindings[i].PassageID {t.Fatal("snapshot lost explicit rail aperture identity")}}}}
func TestOneTrainPassesABC(t *testing.T){
 c:=fixture(t);seen:=[]string{};last:="";prior:=0.0
 for seconds:=c.EpochSeconds;seconds<c.EpochSeconds+380;seconds++{
  s:=c.snapshot(float64(seconds));if s.TrainID!=c.TrainID||s.StockID!=c.StockID||s.PrefabResource!=c.PrefabResource||len(s.Modules)!=4{t.Fatal("identity changed")};if s.WheelTravelM<prior{t.Fatal("wheel travel went backward")};prior=s.WheelTravelM
  if s.EntryAllowed||s.DoorsOpen||len(s.PassengerActorIDs)!=0||len(s.CargoContainerIDs)!=0{t.Fatal("exterior train acquired logistics")}
  for _,b:=range s.Modules[0].Bindings{if b.Visible&&b.LevelID!=last{seen=append(seen,b.LevelID);last=b.LevelID}}
 }
 if !reflect.DeepEqual(seen,[]string{"a","b","c"}){t.Fatalf("route order: %v",seen)}
 s:=c.snapshot(float64(c.EpochSeconds));if math.Abs(s.Modules[0].Bindings[0].X-52.4)>1e-8{t.Fatal("initial metric pose")};if math.Abs(s.Modules[3].Bindings[0].X-9.8)>1e-8{t.Fatal("independent rear pose")}
 if math.Abs(s.Modules[0].Bindings[0].Bogies[0].X-56.4)>1e-8{t.Fatal("bogie distance")}
}
func TestUnobservedSaveLoadAndPause(t *testing.T){
 c:=fixture(t);r:=&Runtime{};r.world.TrainMotion=c;r.state.Epoch="a";r.state.Life.Enabled=true;r.state.Life.Seconds=25200;r.state.Tick=(c.EpochSeconds+200)*10
 before:=r.trainMotionJSON();for i:=0;i<100;i++{if r.trainMotionJSON()!=before{t.Fatal("snapshot mutated train")}}
 saved,err:=json.Marshal(r.state);if err!=nil{t.Fatal(err)};loaded:=&Runtime{};loaded.world.TrainMotion=c;if err:=json.Unmarshal(saved,&loaded.state);err!=nil{t.Fatal(err)}
 if loaded.trainMotionJSON()!=before{t.Fatal("load reset trip")}
 // No samples / observer while world time advances between B and C.
 r.state.Tick+=700;loaded.state.Tick+=700
 if loaded.trainMotionJSON()!=r.trainMotionJSON(){t.Fatal("unloaded continuation diverged")}
 now:=c.snapshot(float64(loaded.simNS())/1e9);if !now.Modules[0].Bindings[2].Visible{t.Fatal("train did not arrive at C")}
 loaded.state.Epoch="loaded";now2:=c.snapshot(float64(loaded.simNS())/1e9);if !reflect.DeepEqual(now,now2){t.Fatal("transport epoch altered motion")}
}
func TestCalendarRateDoesNotScalePhysicalMotion(t *testing.T){
 c:=fixture(t);r:=&Runtime{};r.world.TrainMotion=c;r.state.Life.Enabled=true;r.state.Tick=100;r.state.Life.Seconds=25200
 var before,after trainMotionSnapshot;json.Unmarshal([]byte(r.trainMotionJSON()),&before)
 r.state.Life.Seconds+=30000;json.Unmarshal([]byte(r.trainMotionJSON()),&after)
 if !reflect.DeepEqual(before.Trains,after.Trains)||before.ClockSeconds!=after.ClockSeconds{t.Fatal("calendar acceleration moved physical train")}
 r.state.PhaseNS=50000000;json.Unmarshal([]byte(r.trainMotionJSON()),&after)
 if math.Abs(after.Trains[0].WheelTravelM-before.Trains[0].WheelTravelM-.15)>1e-8{t.Fatal("saved sub-tick phase not represented")}
}
func TestIndependentCarAndBogieTangents(t *testing.T){
 c:=fixture(t);s:=c.snapshot(float64(c.EpochSeconds+83))
 front,rear:=s.Modules[0].Bindings[0],s.Modules[1].Bindings[0]
 if front.YawDegrees==rear.YawDegrees{t.Fatal("cars were moved as one rigid root around corner")}
 if front.Bogies[0].YawDegrees==front.Bogies[1].YawDegrees{t.Fatal("bogies did not sample local tangents")}
 if math.Abs(front.Bogies[0].Y+.53)>1e-9{t.Fatal("bogie vertical datum lost")}
}
func TestDwellAndNextRun(t *testing.T){
 c:=fixture(t);arrival:=float64(c.EpochSeconds)+(c.length()-c.InitialDistanceM)/c.SpeedMPS
 stop:=int64(math.Ceil(arrival));s:=c.snapshot(float64(stop));if s.Phase!="dwell"||s.SpeedMPS!=0{t.Fatal("no server stop")}
 for second:=stop;float64(second)<s.NextEventSeconds;second++{d:=c.snapshot(float64(second));if d.DistanceM!=0||d.WheelTravelM!=s.WheelTravelM||d.Run!=0{t.Fatal("train moved during dwell")}}
 after:=c.snapshot(math.Ceil(s.NextEventSeconds));if after.Phase!="moving"||after.Run!=1||after.EventSequence<=s.EventSequence{t.Fatal("departure/run event absent")}
 for _,second:=range []int64{0,c.EpochSeconds,c.EpochSeconds+1000000000}{if _,err:=json.Marshal(c.snapshot(float64(second)));err!=nil{t.Fatal("non-finite output")}}
}
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
