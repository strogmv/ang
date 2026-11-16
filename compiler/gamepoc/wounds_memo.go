package gamepoc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/strogmv/ang/compiler/wounds"
	woundsRuntime "github.com/strogmv/ang/compiler/wounds/runtime"
)

// The wound timeline table (design/wounds/timelines.md) replays every scenario row and its risk runs: ~4.7 s of each
// generation, while the rows change only with world.wounds. wounds.Report is a pure function of the model, the RU
// texts it asks for and the generator's code, so its result is remembered under the SHA-256 of this executable and of
// the model's JSON, together with every text it read. An entry is used only when each of those texts is still the same.
// The folder is <ANGELS_PIPELINE_CACHE>/ang-wounds-report (the game's pipeline cache); without that variable, and with
// ANGELS_NO_CACHE=1 or ANGELS_FULL=1, the table is replayed (and the fresh result stored).
type woundsReportEntry struct {
	Schema string            `json:"schema"`
	Texts  map[string]string `json:"texts"`
	Report []byte            `json:"report"`
}

const woundsReportSchema = "ang.wounds-report-memo.v1"

var executableSHA = sync.OnceValue(func() string {
	path, err := os.Executable()
	if err != nil {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
})

func woundsReportPath(m woundsRuntime.Model) string {
	root := os.Getenv("ANGELS_PIPELINE_CACHE")
	binary := executableSHA()
	if root == "" || binary == "" {
		return ""
	}
	model, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	h := sha256.New()
	h.Write([]byte(woundsReportSchema + "\x00" + binary + "\x00"))
	h.Write(model)
	return filepath.Join(root, "ang-wounds-report", hex.EncodeToString(h.Sum(nil))+".json")
}

func woundsMemoReadable() bool {
	return os.Getenv("ANGELS_NO_CACHE") != "1" && os.Getenv("ANGELS_FULL") != "1"
}

// reportWounds is wounds.Report, remembered.
func reportWounds(m woundsRuntime.Model, text func(string) string) ([]byte, error) {
	path := woundsReportPath(m)
	if path != "" && woundsMemoReadable() {
		var entry woundsReportEntry
		if raw, err := os.ReadFile(path); err == nil && json.Unmarshal(raw, &entry) == nil && entry.Schema == woundsReportSchema && len(entry.Report) > 0 {
			same := true
			for key, value := range entry.Texts {
				if text(key) != value {
					same = false
					break
				}
			}
			if same {
				now := time.Now()
				_ = os.Chtimes(path, now, now) // the pipeline cache prunes the least recently used entries first
				return entry.Report, nil
			}
		}
	}
	var mu sync.Mutex
	read := map[string]string{}
	report, err := wounds.Report(m, func(key string) string {
		value := text(key)
		mu.Lock()
		read[key] = value
		mu.Unlock()
		return value
	})
	if err != nil || path == "" {
		return report, err
	}
	raw, err := json.Marshal(woundsReportEntry{Schema: woundsReportSchema, Texts: read, Report: report})
	if err == nil && os.MkdirAll(filepath.Dir(path), 0o755) == nil {
		// Write and rename: a reader sees the whole entry or none.
		temporary := fmt.Sprintf("%s.%d.tmp", path, os.Getpid())
		if os.WriteFile(temporary, raw, 0o644) == nil {
			_ = os.Rename(temporary, path)
		}
	}
	return report, nil
}
