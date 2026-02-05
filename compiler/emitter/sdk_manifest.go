package emitter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

type SDKManifest struct {
	Endpoints    []string `json:"endpoints"`
	QueryKeys    []string `json:"query_keys"`
	QueryOptions []string `json:"query_options"`
}

func (e *Emitter) EmitSDKManifest(endpoints []normalizer.Endpoint, queryResources []QueryResource) error {
	lowerFirst := func(s string) string {
		if len(s) == 0 {
			return ""
		}
		return strings.ToLower(s[:1]) + s[1:]
	}

	manifest := SDKManifest{
		Endpoints:    make([]string, 0),
		QueryKeys:    make([]string, 0),
		QueryOptions: make([]string, 0),
	}

	for _, ep := range endpoints {
		if strings.ToUpper(ep.Method) == "WS" {
			continue
		}
		name := lowerFirst(ep.RPC)
		entry := fmt.Sprintf("%s %s %s", name, strings.ToUpper(ep.Method), ep.Path)
		manifest.Endpoints = append(manifest.Endpoints, entry)
	}

	for _, r := range queryResources {
		base := r.Key
		manifest.QueryKeys = append(manifest.QueryKeys, base+".all")
		if r.HasList {
			manifest.QueryKeys = append(manifest.QueryKeys, base+".lists", base+".list")
		}
		if r.HasDetail {
			manifest.QueryKeys = append(manifest.QueryKeys, base+".details", base+".detail")
		}
		if r.HasMe {
			manifest.QueryKeys = append(manifest.QueryKeys, base+".me")
		}

		optionsBase := base + "Options"
		if r.HasList {
			manifest.QueryOptions = append(manifest.QueryOptions, optionsBase+".list")
		}
		if r.HasDetail {
			manifest.QueryOptions = append(manifest.QueryOptions, optionsBase+".detail")
		}
		if r.HasMe {
			manifest.QueryOptions = append(manifest.QueryOptions, optionsBase+".me")
		}
	}

	sort.Strings(manifest.Endpoints)
	sort.Strings(manifest.QueryKeys)
	sort.Strings(manifest.QueryOptions)

	data, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("failed to marshal SDK manifest: %w", err)
	}

	path := filepath.Join(e.FrontendDir, "sdk-manifest.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write SDK manifest: %w", err)
	}

	fmt.Printf("Generated SDK Manifest: %s\n", path)
	return nil
}
