package gamepoc

import (
	"bytes"
	"cuelang.org/go/cue"
	"encoding/json"
	"fmt"
	"math"
)

type CartPose struct {
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	Z          float64 `json:"z"`
	YawDegrees float64 `json:"yaw_degrees"`
}
type CartBay struct {
	ID         string   `json:"id"`
	InstanceID string   `json:"instance_id"`
	PlaceID    string   `json:"place_id"`
	Handle     Point    `json:"handle"`
	PreHandle  Point    `json:"pre_handle"`
	Pose       CartPose `json:"pose"`
}
type CartParkingCatalog struct {
	CartID                 string    `json:"cart_id"`
	OwnerActorID           string    `json:"owner_actor_id"`
	StoreID                string    `json:"store_id"`
	LevelID                string    `json:"level_id"`
	PrefabResource         string    `json:"prefab_resource"`
	CouplingForwardM       float64   `json:"coupling_forward_m"`
	CouplingRightM         float64   `json:"coupling_right_m"`
	GripYM                 float64   `json:"grip_y_m"`
	ReleaseSeconds         float64   `json:"release_seconds"`
	PickupSeconds          float64   `json:"pickup_seconds"`
	TurnSeconds            float64   `json:"turn_seconds"`
	HalfWidthM             float64   `json:"half_width_m"`
	FrontM                 float64   `json:"front_m"`
	BackM                  float64   `json:"back_m"`
	ApproachTimeoutSeconds int64     `json:"approach_timeout_seconds"`
	RetrySeconds           int64     `json:"retry_seconds"`
	Bays                   []CartBay `json:"bays"`
}

func readCartParking(world cue.Value, c Compiled) (*CartParkingCatalog, error) {
	v := world.LookupPath(cue.ParsePath("cart_parking"))
	if !v.Exists() {
		return nil, nil
	}
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil, err
	}
	var out CartParkingCatalog
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err = d.Decode(&out); err != nil {
		return nil, err
	}
	if err = out.validate(c); err != nil {
		return nil, fmt.Errorf("cart_parking: %w", err)
	}
	return &out, nil
}
func (c CartParkingCatalog) validate(w Compiled) error {
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && math.Abs(v) < 1e5 }
	if c.CartID == "" || c.OwnerActorID != "char.market_peddler_akim" || c.StoreID != "peddler/peddler.akim/tray" || c.LevelID != "level.c7_market" || !locationSDKSurfaceResourceKey.MatchString(c.PrefabResource) || len(c.PrefabResource) > 256 || len(c.Bays) != 2 {
		return fmt.Errorf("invalid identity/resource/bays")
	}
	for _, v := range []float64{c.CouplingForwardM, c.CouplingRightM, c.GripYM, c.ReleaseSeconds, c.PickupSeconds, c.TurnSeconds, c.HalfWidthM, c.FrontM, c.BackM} {
		if !finite(v) {
			return fmt.Errorf("non-finite coupling or time")
		}
	}
	if c.CouplingForwardM < .5 || c.CouplingForwardM > 1 || c.CouplingRightM != 0 || c.GripYM < .5 || c.GripYM > 1.2 || c.ReleaseSeconds <= 0 || c.ReleaseSeconds > 10 || c.PickupSeconds <= 0 || c.PickupSeconds > 10 || c.TurnSeconds <= 0 || c.TurnSeconds > 10 || c.HalfWidthM != .39 || c.FrontM != .4 || c.BackM != .50 || c.ApproachTimeoutSeconds < 1 || c.ApproachTimeoutSeconds > 300 || c.RetrySeconds < 1 || c.RetrySeconds > 600 {
		return fmt.Errorf("invalid coupling or bounded duration")
	}
	var life struct {
		Locations map[string]struct {
			Terrain []string `json:"terrain"`
			CellMM  int      `json:"cell_mm"`
		} `json:"locations"`
		Actors []struct {
			ID string `json:"id"`
		} `json:"actors"`
		Brain struct {
			Places []struct {
				ID string `json:"id"`
			} `json:"places"`
		} `json:"brain"`
	}
	if err := json.Unmarshal(w.Life, &life); err != nil {
		return err
	}
	l := life.Locations[c.LevelID]
	if l.CellMM != 500 {
		return fmt.Errorf("missing metric level")
	}
	actor := false
	for _, a := range life.Actors {
		actor = actor || a.ID == c.OwnerActorID
	}
	if !actor {
		return fmt.Errorf("missing owner")
	}
	walk := func(p Point) bool {
		return p.Y >= 0 && p.Y < len(l.Terrain) && p.X >= 0 && p.X < len(l.Terrain[p.Y]) && l.Terrain[p.Y][p.X] == '.'
	}
	ids := map[string]bool{}
	for _, b := range c.Bays {
		place := false
		for _, p := range life.Brain.Places {
			place = place || p.ID == b.PlaceID
		}
		if b.ID == "" || ids[b.ID] || !place || !walk(b.Handle) || !walk(b.PreHandle) || b.PreHandle != (Point{X: b.Handle.X + 1, Y: b.Handle.Y}) {
			return fmt.Errorf("invalid bay %s identity/approach", b.ID)
		}
		ids[b.ID] = true
		found := false
		if w.LocationSDK != nil {
			for _, level := range w.LocationSDK.Levels {
				for _, instance := range level.Instances {
					if instance.ID != b.InstanceID {
						continue
					}
					if instance.ModuleID != "kit.geo.hub.cart_dock" || instance.Width != 2 || instance.Depth != 3 || len(instance.Blocked) != 6 {
						return fmt.Errorf("bay requires a reserved 2x3 SDK footprint")
					}
					found = true
					for _, at := range instance.Blocked {
						if walk(Point{X: at.X, Y: at.Y}) {
							return fmt.Errorf("cart bay must not remain a walk-through route")
						}
					}
				}
			}
		}
		if !found {
			return fmt.Errorf("missing authored dock %s", b.InstanceID)
		}
		for _, v := range []float64{b.Pose.X, b.Pose.Y, b.Pose.Z, b.Pose.YawDegrees} {
			if !finite(v) {
				return fmt.Errorf("non-finite bay pose")
			}
		}
		if b.Pose.Y != 0 || b.Pose.YawDegrees != 270 || math.Abs((float64(b.Handle.X)+.5)*.5-c.CouplingForwardM-b.Pose.X) > 1e-8 || math.Abs((float64(b.Handle.Y)+.5)*.5-b.Pose.Z) > 1e-8 {
			return fmt.Errorf("bay coupling does not align with ordinary handle cell")
		}
	}
	return nil
}
