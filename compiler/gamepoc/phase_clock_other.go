//go:build !unix

package gamepoc

import "time"

func processCPU() time.Duration { return 0 }
