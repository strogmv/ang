package gamepoc

import (
	"encoding/json"
	"os"
	"time"
)

// phaseClock appends one row per generation phase to ANGELS_STEPS_FILE, the step
// table of the game's tools/location-pipeline/steps.py: wall and CPU seconds of this
// process (all threads). Without the variable it records nothing; outputs never depend on it.
type phaseClock struct {
	path, prefix string
	wall         time.Time
	cpu          time.Duration
}

var phases *phaseClock

func startPhaseClock() *phaseClock {
	path := os.Getenv("ANGELS_STEPS_FILE")
	if path == "" {
		return nil
	}
	return &phaseClock{path: path, prefix: os.Getenv("ANGELS_STEPS_PREFIX") + "ang: ", wall: time.Now(), cpu: processCPU()}
}

// mark records the phase that ends now and starts the next one.
func (c *phaseClock) mark(name string) {
	if c == nil {
		return
	}
	now, cpu := time.Now(), processCPU()
	row, _ := json.Marshal(map[string]any{
		"step":   c.prefix + name,
		"wall_s": float64(now.Sub(c.wall).Milliseconds()) / 1000,
		"cpu_s":  float64((cpu - c.cpu).Milliseconds()) / 1000,
	})
	if f, err := os.OpenFile(c.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
		_, _ = f.Write(append(row, '\n'))
		_ = f.Close()
	}
	c.wall, c.cpu = now, cpu
}
