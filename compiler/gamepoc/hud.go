package gamepoc

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type HudArt struct {
	HeroPortrait string `json:"hero_portrait"`
}

var hudResourcePath = regexp.MustCompile(`^[A-Za-z0-9_-]+(/[A-Za-z0-9_-]+)*$`)

func checkHudImage(unity string, h *HudArt) error {
	if h == nil || h.HeroPortrait == "" {
		return nil
	}
	if !hudResourcePath.MatchString(h.HeroPortrait) || strings.Contains(h.HeroPortrait, "..") {
		return fmt.Errorf("art.hud: unsafe portrait Resources path")
	}
	f, err := os.Open(filepath.Join(unity, "Assets/Resources", h.HeroPortrait+".png"))
	if err != nil {
		return fmt.Errorf("art.hud: %w", err)
	}
	defer f.Close()
	cfg, format, err := image.DecodeConfig(f)
	if err != nil || format != "png" || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 4096 || cfg.Height > 4096 {
		return fmt.Errorf("art.hud: portrait must be a valid PNG up to4096px")
	}
	return nil
}
