package paymentprovider

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// GeneratedFile describes one generator-owned output path and content hash.
type GeneratedFile struct {
	RelativePath string `json:"relative_path"`
	SHA256       string `json:"sha256"`
}

// BuildResult captures a completed payment-provider generation run.
type BuildResult struct {
	OutputDir string          `json:"output_dir"`
	Files     []GeneratedFile `json:"files"`
}

// ManifestHash returns lowercase SHA-256 of the canonical generated-file manifest.
func (result BuildResult) ManifestHash() (string, error) {
	data, err := canonicalManifestJSON(result.Files)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func canonicalManifestJSON(files []GeneratedFile) ([]byte, error) {
	out := append([]GeneratedFile(nil), files...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].RelativePath < out[j].RelativePath
	})
	return json.Marshal(out)
}

func hashFileContents(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
