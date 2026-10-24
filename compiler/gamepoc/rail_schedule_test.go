package gamepoc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
)

// Like the authored-market contract, this optional integration check connects
// ANG to the book's real CUE without installing game-specific paths in ANG.
func TestAuthoredC7PassengerAndPaintedSchedulesAgree(t *testing.T) {
	book := os.Getenv("ANG_C7_TRANSIT_PROJECT")
	if book == "" {
		t.Skip("set ANG_C7_TRANSIT_PROJECT to the book checkout")
	}
	read := func(relative, path string) cue.Value {
		t.Helper()
		instances := load.Instances([]string{"."}, &load.Config{Dir: filepath.Join(book, relative)})
		if len(instances) != 1 {
			t.Fatal("expected one authored CUE package")
		}
		root := cuecontext.New().BuildInstance(instances[0])
		if err := root.Err(); err != nil {
			t.Fatal(err)
		}
		v := root.LookupPath(cue.ParsePath(path))
		if err := v.Validate(cue.Concrete(true)); err != nil {
			t.Fatal(err)
		}
		return v
	}
	var schedule map[string]int64
	if err := read("content/world/transit", "catalog.c7_loop").Decode(&schedule); err != nil {
		t.Fatal(err)
	}
	want := map[string]int64{"period_seconds": 300, "night_period_seconds": 3600, "day_start_second": 21600, "day_end_second": 79200, "offset_seconds": 0}
	if len(schedule) != len(want) {
		t.Fatal("shared timetable must contain only timing fields")
	}
	for key, value := range want {
		if schedule[key] != value {
			t.Fatalf("owner timetable changed: %s=%d want%d", key, schedule[key], value)
		}
	}
	var world struct {
		Life struct {
			Edges []map[string]any `json:"edges"`
		} `json:"life"`
		Art struct {
			Painted []struct {
				Passages []map[string]any `json:"ambient_passages"`
			} `json:"painted"`
		} `json:"art"`
	}
	data, err := read("content/world", "world").MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &world); err != nil {
		t.Fatal(err)
	}
	check := func(kind string, row map[string]any) {
		t.Helper()
		for key, value := range want {
			if row[key] != float64(value) {
				t.Errorf("%s %s uses %s=%v, shared schedule=%d", kind, row["id"], key, row[key], value)
			}
		}
	}
	found := map[string]bool{}
	for _, edge := range world.Life.Edges {
		id, _ := edge["id"].(string)
		if id == "station_work" || id == "work_station" {
			check("passenger edge", edge)
			if edge["travel_seconds"] != float64(300) || edge["seats"] != float64(4) || edge["mode"] != "rail" {
				t.Fatalf("timetable wiring changed journey/capacity: %+v", edge)
			}
			found[id] = true
		} else if edge["mode"] == "walk" {
			if edge["period_seconds"] != float64(0) || edge["night_period_seconds"] != nil {
				t.Fatalf("walking edge gained a train timetable: %+v", edge)
			}
		}
	}
	for _, scene := range world.Art.Painted {
		for _, passage := range scene.Passages {
			id, _ := passage["id"].(string)
			if id == "market.lower_rail" || id == "housing.lower_rail" || id == "station.back_rail" {
				check("painted passage", passage)
				berthElevation := float64(12)
				if id == "station.back_rail" {
					berthElevation = 18
				}
				if passage["duration_seconds"] != float64(15) || passage["elevation_m"] != berthElevation {
					t.Fatal("shared timetable changed visual duration/elevation")
				}
				found[id] = true
			}
		}
	}
	for _, id := range []string{"station_work", "work_station", "market.lower_rail", "housing.lower_rail", "station.back_rail"} {
		if !found[id] {
			t.Errorf("missing real service/presentation %s", id)
		}
	}
}
