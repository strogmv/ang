package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// A build that is killed, or that crashes before Rollback, never removes its
// scratch directories: .ang-build-transaction-* next to each owned path and
// .ang-build-workspace-* next to the project root. .gitignore hides them, so
// they pile up — the consuming project had 17 of them, 657 MB, the oldest a
// month old, several holding a copy of a 91 MB node_modules.
//
// Every scratch directory now records its owner, and a build removes the ones
// whose owner is gone before it creates its own. The rules err on keeping:
// a directory is removed only when its owner is provably dead on this host, or,
// for directories from before owners were recorded, when it is over a day old.
const (
	buildScratchOwnerFile    = "owner.json"
	buildScratchLegacyMaxAge = 24 * time.Hour
	transactionScratchPrefix = ".ang-build-transaction-"
	workspaceScratchPrefix   = ".ang-build-workspace-"
)

type buildScratchOwner struct {
	PID     int       `json:"pid"`
	Host    string    `json:"host"`
	Started time.Time `json:"started"`
	// Keep marks a directory that still holds the project's originals after a
	// rollback failed. It is never removed automatically.
	Keep bool `json:"keep,omitempty"`
}

type buildScratchSweep struct {
	Removed int
	Bytes   int64
	Kept    []string
}

func writeBuildScratchOwner(dir string, keep bool) error {
	host, _ := os.Hostname()
	data, err := json.MarshalIndent(buildScratchOwner{
		PID:     os.Getpid(),
		Host:    host,
		Started: time.Now().UTC(),
		Keep:    keep,
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, buildScratchOwnerFile), append(data, '\n'), 0o644)
}

// sweepAbandonedBuildScratch removes, best effort, the scratch directories with
// prefix directly inside parent whose build is gone. It never fails a build:
// anything it cannot read or remove is left where it is.
func sweepAbandonedBuildScratch(parent, prefix string, now time.Time, result *buildScratchSweep) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return
	}
	host, _ := os.Hostname()
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}
		dir := filepath.Join(parent, entry.Name())
		abandoned, keep := buildScratchAbandoned(dir, host, now)
		if keep {
			result.Kept = append(result.Kept, dir)
			continue
		}
		if !abandoned {
			continue
		}
		size := buildScratchSize(dir)
		if err := os.RemoveAll(dir); err != nil {
			continue
		}
		result.Removed++
		result.Bytes += size
	}
}

func buildScratchAbandoned(dir, host string, now time.Time) (abandoned, keep bool) {
	data, err := os.ReadFile(filepath.Join(dir, buildScratchOwnerFile))
	if err != nil {
		if !os.IsNotExist(err) {
			return false, false
		}
		// Written before owners were recorded: only age can tell, and a build of
		// an older binary may still be running, so give it a day.
		info, statErr := os.Stat(dir)
		if statErr != nil {
			return false, false
		}
		return now.Sub(info.ModTime()) > buildScratchLegacyMaxAge, false
	}
	var owner buildScratchOwner
	if err := json.Unmarshal(data, &owner); err != nil {
		return false, false
	}
	if owner.Keep {
		return false, true
	}
	if owner.PID <= 0 || owner.Host != host || owner.PID == os.Getpid() {
		return false, false
	}
	return !processAlive(owner.PID), false
}

func buildScratchSize(dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, infoErr := d.Info(); infoErr == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

func formatScratchBytes(n int64) string {
	const mb = 1 << 20
	if n >= mb {
		return fmt.Sprintf("%.1f MB", float64(n)/mb)
	}
	return fmt.Sprintf("%d KB", (n+1023)/1024)
}
