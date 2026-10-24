package gamepoc

import (
	"bytes"
	"fmt"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/strogmv/ang/compiler/locationsdk"
)

const (
	locationSDKSurfaceMaxDimension = 4096
	locationSDKSurfaceMaxFileBytes = 16 << 20
	locationSDKSurfaceMaxBytes     = 64 << 20
	locationSDKSurfaceMaxPixels    = 64 << 20
)

var locationSDKSurfaceResourceKey = regexp.MustCompile(`^[A-Za-z0-9_-]+(/[A-Za-z0-9_-]+)*$`)

// checkLocationSDKSurfaceImages runs before Generate writes any output. Surface
// images are opaque color tiles on already validated geometry, never silhouettes
// or a way to create an apparent doorway through an authoritative solid wall.
func checkLocationSDKSurfaceImages(unity string, bundle locationsdk.RuntimeBundle) error {
	if len(bundle.Surfaces) == 0 {
		return nil
	}
	if len(bundle.Surfaces) > locationsdk.SurfaceBudget {
		return fmt.Errorf("location_sdk.surfaces: exceeds %d definitions", locationsdk.SurfaceBudget)
	}
	for _, surface := range bundle.Surfaces {
		for _, key := range []string{surface.TopImage, surface.SideImage} {
			if len(key) > 256 || !locationSDKSurfaceResourceKey.MatchString(key) {
				return fmt.Errorf("location_sdk surface %q: unsafe extensionless Resources key %q", surface.ID, key)
			}
		}
	}
	// Root.Open prevents both ordinary traversal and symlinks escaping Resources,
	// including replacement races between checking a path and opening its file.
	root, err := os.OpenRoot(filepath.Join(unity, "Assets", "Resources"))
	if err != nil {
		return fmt.Errorf("location_sdk.surfaces Resources: %w", err)
	}
	defer root.Close()
	seen := make(map[string]bool)
	var totalBytes, totalPixels int64
	for _, surface := range bundle.Surfaces {
		for _, ref := range []struct{ field, key string }{{"top_image", surface.TopImage}, {"side_image", surface.SideImage}} {
			if seen[ref.key] {
				continue
			}
			owner := fmt.Sprintf("location_sdk surface %q %s %q", surface.ID, ref.field, ref.key)
			data, err := readLocationSDKSurfacePNG(root, ref.key+".png")
			if err != nil {
				return fmt.Errorf("%s: %w", owner, err)
			}
			totalBytes += int64(len(data))
			if totalBytes > locationSDKSurfaceMaxBytes {
				return fmt.Errorf("location_sdk.surfaces: unique PNG files exceed %d bytes", locationSDKSurfaceMaxBytes)
			}
			cfg, err := png.DecodeConfig(bytes.NewReader(data))
			if err != nil {
				return fmt.Errorf("%s: must be a valid PNG: %w", owner, err)
			}
			if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > locationSDKSurfaceMaxDimension || cfg.Height > locationSDKSurfaceMaxDimension {
				return fmt.Errorf("%s: PNG dimensions must be 1..%d pixels", owner, locationSDKSurfaceMaxDimension)
			}
			totalPixels += int64(cfg.Width) * int64(cfg.Height)
			if totalPixels > locationSDKSurfaceMaxPixels {
				return fmt.Errorf("location_sdk.surfaces: unique PNGs exceed %d total pixels", locationSDKSurfaceMaxPixels)
			}
			image, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				return fmt.Errorf("%s: damaged PNG pixels: %w", owner, err)
			}
			// Check used pixels, not just palette entries or the presence of an alpha
			// channel: an RGBA PNG with alpha=255 everywhere is a valid color tile.
			if opaque, ok := image.(interface{ Opaque() bool }); !ok || !opaque.Opaque() {
				for y := image.Bounds().Min.Y; y < image.Bounds().Max.Y; y++ {
					for x := image.Bounds().Min.X; x < image.Bounds().Max.X; x++ {
						if _, _, _, alpha := image.At(x, y).RGBA(); alpha != 0xffff {
							return fmt.Errorf("%s: surface must be fully opaque (transparent pixel at %d,%d)", owner, x, y)
						}
					}
				}
			}
			seen[ref.key] = true
		}
	}
	return nil
}

func readLocationSDKSurfacePNG(root *os.Root, name string) ([]byte, error) {
	info, err := root.Stat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("PNG must be a regular file")
	}
	if info.Size() > locationSDKSurfaceMaxFileBytes {
		return nil, fmt.Errorf("PNG exceeds %d bytes", locationSDKSurfaceMaxFileBytes)
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, locationSDKSurfaceMaxFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > locationSDKSurfaceMaxFileBytes {
		return nil, fmt.Errorf("PNG exceeds %d bytes", locationSDKSurfaceMaxFileBytes)
	}
	return data, nil
}
