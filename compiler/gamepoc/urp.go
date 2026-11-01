package gamepoc

import (
	"fmt"
	"path"
	"strings"
)

// UnityRendering pins the tested pipeline contract while the actual .asset,
// material and volume objects remain user-owned and survive generation.
type UnityRendering struct {
	Pipeline       string `json:"pipeline"`
	PackageVersion string `json:"package_version"`
	AssetPath      string `json:"asset_path"`
}

func (r UnityRendering) normalized() UnityRendering {
	if r.Pipeline == "" {
		r.Pipeline = "urp"
	}
	if r.PackageVersion == "" {
		r.PackageVersion = "17.6.0"
	}
	if r.AssetPath == "" {
		r.AssetPath = "Assets/Rendering/ANGUniversalRenderPipeline.asset"
	}
	return r
}
func (r UnityRendering) validate() error {
	r = r.normalized()
	if r.Pipeline != "urp" || r.PackageVersion != "17.6.0" {
		return fmt.Errorf("GamePOC rendering requires tested URP 17.6.0 for Unity 6000.6")
	}
	if !strings.HasPrefix(r.AssetPath, "Assets/") || path.Clean(r.AssetPath) != r.AssetPath || strings.Contains(r.AssetPath, "\\") || !strings.HasSuffix(r.AssetPath, ".asset") || strings.HasPrefix(r.AssetPath, "Assets/Generated/") || strings.HasPrefix(r.AssetPath, "Assets/Resources/") {
		return fmt.Errorf("GamePOC rendering asset must be a user-owned Assets path outside generated/resource output")
	}
	return nil
}
