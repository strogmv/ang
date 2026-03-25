package emitter

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/strogmv/ang-ir/normalizer"
	"github.com/strogmv/ang/compiler/policy"
)

type SDKManifest struct {
	Endpoints    []SDKManifestEndpoint `json:"endpoints"`
	QueryKeys    []string              `json:"query_keys"`
	QueryOptions []string              `json:"query_options"`
}

type SDKManifestEndpoint struct {
	Name       string   `json:"name"`
	Method     string   `json:"method"`
	Path       string   `json:"path"`
	Idempotent bool     `json:"idempotent,omitempty"`
	Timeout    string   `json:"timeout,omitempty"`
	AuthRoles  []string `json:"authRoles,omitempty"`
	AuthInject []string `json:"authInject,omitempty"`
	CacheTTL   string   `json:"cacheTTL,omitempty"`
}

func (e *Emitter) EmitSDKManifest(endpoints []normalizer.Endpoint) error {
	lowerFirst := func(s string) string {
		if len(s) == 0 {
			return ""
		}
		return strings.ToLower(s[:1]) + s[1:]
	}

	manifest := SDKManifest{
		Endpoints:    make([]SDKManifestEndpoint, 0),
		QueryKeys:    make([]string, 0),
		QueryOptions: make([]string, 0),
	}

	for _, ep := range endpoints {
		if strings.ToUpper(ep.Method) == "WS" {
			continue
		}
		name := lowerFirst(ep.RPC)
		p := policy.FromEndpoint(ep)
		entry := SDKManifestEndpoint{
			Name:       name,
			Method:     strings.ToUpper(ep.Method),
			Path:       ep.Path,
			Idempotent: p.Idempotency,
			Timeout:    p.Timeout,
			CacheTTL:   p.CacheTTL,
		}
		if len(p.AuthRoles) > 0 {
			entry.AuthRoles = p.AuthRoles
		}
		if len(ep.AuthInject) > 0 {
			entry.AuthInject = append([]string{}, ep.AuthInject...)
		}
		manifest.Endpoints = append(manifest.Endpoints, entry)
	}

	for _, ep := range endpoints {
		if strings.ToUpper(strings.TrimSpace(ep.Method)) != "GET" {
			continue
		}
		svc := strings.TrimSpace(ep.ServiceName)
		rpc := strings.TrimSpace(ep.RPC)
		if svc == "" || rpc == "" {
			continue
		}
		manifest.QueryKeys = append(manifest.QueryKeys, fmt.Sprintf("endpoint.%s.%s", svc, rpc))
		manifest.QueryOptions = append(manifest.QueryOptions, "endpointQueryOptions."+lowerFirst(rpc))
	}

	sort.Slice(manifest.Endpoints, func(i, j int) bool {
		if manifest.Endpoints[i].Name != manifest.Endpoints[j].Name {
			return manifest.Endpoints[i].Name < manifest.Endpoints[j].Name
		}
		if manifest.Endpoints[i].Method != manifest.Endpoints[j].Method {
			return manifest.Endpoints[i].Method < manifest.Endpoints[j].Method
		}
		return manifest.Endpoints[i].Path < manifest.Endpoints[j].Path
	})
	sort.Strings(manifest.QueryKeys)
	sort.Strings(manifest.QueryOptions)

	data, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("failed to marshal SDK manifest: %w", err)
	}

	path := filepath.Join(e.FrontendDir, "sdk-manifest.json")
	if err := WriteFileIfChanged(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write SDK manifest: %w", err)
	}

	fmt.Printf("Generated SDK Manifest: %s\n", path)
	return nil
}
