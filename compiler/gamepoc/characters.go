package gamepoc

import (
	"fmt"
	"image"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func clientCharacters(a CharacterArt) map[string]any {
	ids := make([]string, 0, len(a.Actors))
	for id := range a.Actors {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	actors := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		actors = append(actors, map[string]any{"id": id, "sheet": a.Actors[id]})
	}
	names := make([]string, 0, len(a.Sheets))
	for id := range a.Sheets {
		names = append(names, id)
	}
	sort.Strings(names)
	sheets := make([]map[string]any, 0, len(names))
	for _, id := range names {
		s := a.Sheets[id]
		sheets = append(sheets, map[string]any{"id": id, "frames": s.Frames, "smooth_filter": s.SmoothFilter, "image": s.Image, "idle_image": s.IdleImage, "height_mm": s.HeightMM, "body_height_px": s.BodyHeightPX, "frame_w": s.FrameW, "frame_h": s.FrameH, "draw_size": s.DrawSize, "feet_y": s.FeetY, "idle": s.Idle, "walk": s.Walk, "rows": s.Rows})
	}
	return map[string]any{"default_sheets": a.DefaultSheets, "fps": a.FPS, "rate_variation": a.RateVariation, "actors": actors, "sheets": sheets}
}
func checkCharacterImages(unity string, a CharacterArt) error {
	if len(a.Sheets) == 0 && len(a.Actors) == 0 && len(a.DefaultSheets) == 0 {
		return nil
	}
	if a.FPS < 1 || a.FPS > 24 || a.RateVariation < 0 || a.RateVariation > .15 {
		return fmt.Errorf("characters: invalid animation rate")
	}
	for id, s := range a.Sheets {
		if !strings.HasPrefix(s.Image, "Characters/") || strings.Contains(s.Image, "..") || strings.Contains(s.Image, "\\") {
			return fmt.Errorf("character %s: unsafe image path", id)
		}
		f, err := os.Open(filepath.Join(unity, "Assets/Resources", s.Image+".png"))
		if err != nil {
			return fmt.Errorf("character %s: %w", id, err)
		}
		cfg, _, err := image.DecodeConfig(f)
		_ = f.Close()
		if err != nil {
			return fmt.Errorf("character %s: %w", id, err)
		}
		if s.FrameW < 1 || s.FrameW > 1024 || s.FrameH < 1 || s.FrameH > 1024 || (len(s.Frames) == 0 && (cfg.Width%s.FrameW != 0 || cfg.Height%s.FrameH != 0)) || s.FeetY < 1 || s.FeetY > s.FrameH || s.DrawSize < 16 || s.DrawSize > 128 || len(s.Rows) != 8 || len(s.Walk) < 2 {
			return fmt.Errorf("character %s: invalid sheet geometry", id)
		}
		if s.HeightMM < 0 || s.HeightMM > 10000 || s.BodyHeightPX < 0 || s.BodyHeightPX > s.FeetY || (s.HeightMM == 0) != (s.BodyHeightPX == 0) {
			return fmt.Errorf("character %s: height_mm and body_height_px must be a valid pair", id)
		}
		if len(s.Frames) > 0 {
			if err := checkCharacterFrames(id, s, cfg); err != nil {
				return err
			}
			continue
		}
		idleCfg := cfg
		if s.IdleImage != "" {
			if !strings.HasPrefix(s.IdleImage, "Characters/") || strings.Contains(s.IdleImage, "..") || strings.Contains(s.IdleImage, "\\") {
				return fmt.Errorf("character %s: unsafe idle image path", id)
			}
			f, err := os.Open(filepath.Join(unity, "Assets/Resources", s.IdleImage+".png"))
			if err != nil {
				return fmt.Errorf("character %s idle: %w", id, err)
			}
			idleCfg, _, err = image.DecodeConfig(f)
			_ = f.Close()
			if err != nil {
				return fmt.Errorf("character %s idle: %w", id, err)
			}
			if idleCfg.Width%s.FrameW != 0 || idleCfg.Height%s.FrameH != 0 {
				return fmt.Errorf("character %s: invalid idle geometry", id)
			}
		}
		if s.Idle < 0 || s.Idle >= idleCfg.Width/s.FrameW {
			return fmt.Errorf("character %s: idle frame outside sheet", id)
		}
		for _, n := range s.Walk {
			if n < 0 || n >= cfg.Width/s.FrameW {
				return fmt.Errorf("character %s: frame outside sheet", id)
			}
		}
		for _, n := range s.Rows {
			if n < 0 || n >= cfg.Height/s.FrameH || n >= idleCfg.Height/s.FrameH {
				return fmt.Errorf("character %s: row outside sheet", id)
			}
		}
	}
	seenDefaults := map[string]bool{}
	for _, sheet := range a.DefaultSheets {
		if _, ok := a.Sheets[sheet]; !ok {
			return fmt.Errorf("characters: unknown default sheet %s", sheet)
		}
		if seenDefaults[sheet] {
			return fmt.Errorf("characters: duplicate default sheet %s", sheet)
		}
		seenDefaults[sheet] = true
	}
	for actor, sheet := range a.Actors {
		if _, ok := a.Sheets[sheet]; !ok {
			return fmt.Errorf("character %s: unknown sheet %s", actor, sheet)
		}
	}
	return nil
}

func checkPresentationCamera(c PresentationCamera) error {
	if c == (PresentationCamera{}) {
		return nil
	}
	for _, v := range []float64{c.ZoomDefault, c.ZoomMin, c.ZoomMax} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0.25 || v > 6 {
			return fmt.Errorf("art.camera: zoom must be finite within 0.25..6")
		}
	}
	if c.ZoomMin > c.ZoomDefault || c.ZoomDefault > c.ZoomMax {
		return fmt.Errorf("art.camera: expected min <= default <= max")
	}
	return nil
}

func checkCharacterFrames(id string, s CharacterSheet, cfg image.Config) error {
	if s.IdleImage != "" {
		return fmt.Errorf("character %s: explicit frames require one shared idle/walk atlas", id)
	}
	seen := map[[2]int]bool{}
	for _, f := range s.Frames {
		key := [2]int{f.Row, f.Column}
		if seen[key] {
			return fmt.Errorf("character %s: duplicate frame %v", id, key)
		}
		seen[key] = true
		if f.Row < 0 || f.Column < 0 || f.X < 0 || f.Y < 0 || f.W < 1 || f.H < 1 || f.W > cfg.Width || f.H > cfg.Height || f.X > cfg.Width-f.W || f.Y > cfg.Height-f.H || f.FeetX < 0 || f.FeetX > f.W || f.FeetY < 1 || f.FeetY > f.H || f.BodyHeightPX < 1 || f.BodyHeightPX > f.FeetY {
			return fmt.Errorf("character %s: invalid explicit frame %v", id, key)
		}
	}
	for _, row := range s.Rows {
		if !seen[[2]int{row, s.Idle}] {
			return fmt.Errorf("character %s: missing idle frame %d,%d", id, row, s.Idle)
		}
		for _, column := range s.Walk {
			if !seen[[2]int{row, column}] {
				return fmt.Errorf("character %s: missing walk frame %d,%d", id, row, column)
			}
		}
	}
	return nil
}
