package compiler

import (
	"fmt"
	"strings"

	"github.com/strogmv/ang/compiler/normalizer"
)

// Capability describes a target generation/runtime ability.
type Capability string

const (
	CapabilityHTTP    Capability = "http"
	CapabilitySQLRepo Capability = "sql_repo"
	CapabilityWS      Capability = "ws"
	CapabilityEvents  Capability = "events"
	CapabilityAuth    Capability = "auth"

	// Profile capabilities are explicit backend generation profiles.
	CapabilityProfileGoLegacy      Capability = "profile_go_legacy"
	CapabilityProfilePythonFastAPI Capability = "profile_python_fastapi"
)

// CapabilitySet is the resolved capability matrix for a target.
type CapabilitySet map[Capability]bool

func (s CapabilitySet) Has(cap Capability) bool {
	return s[cap]
}

func (s CapabilitySet) HasAll(caps ...Capability) bool {
	for _, cap := range caps {
		if !s.Has(cap) {
			return false
		}
	}
	return true
}

func (s CapabilitySet) Missing(caps ...Capability) []Capability {
	var out []Capability
	for _, cap := range caps {
		if !s.Has(cap) {
			out = append(out, cap)
		}
	}
	return out
}

func (s CapabilitySet) StringSlice() []string {
	var out []string
	for cap, ok := range s {
		if ok {
			out = append(out, string(cap))
		}
	}
	return out
}

// ResolveTargetCapabilities derives a stable capability matrix from target settings.
func ResolveTargetCapabilities(td normalizer.TargetDef) (CapabilitySet, error) {
	caps := CapabilitySet{}

	lang := strings.ToLower(strings.TrimSpace(td.Lang))
	framework := strings.ToLower(strings.TrimSpace(td.Framework))
	db := strings.ToLower(strings.TrimSpace(td.DB))
	queue := strings.ToLower(strings.TrimSpace(td.Queue))

	switch framework {
	case "chi", "echo", "fiber", "gin", "fastapi", "axum", "actix", "express", "fastify":
		caps[CapabilityHTTP] = true
	}

	switch db {
	case "postgres", "mysql", "sqlite":
		caps[CapabilitySQLRepo] = true
	}

	// Current WebSocket generation coverage in ANG is implemented in Go transport.
	if lang == "go" {
		caps[CapabilityWS] = true
	}

	// Events are meaningful when queue is configured; still true by default for known backends.
	if queue != "" && queue != "none" {
		caps[CapabilityEvents] = true
	}

	// Auth is supported in current generated backends for Go and Python profiles.
	if lang == "go" || (lang == "python" && framework == "fastapi") {
		caps[CapabilityAuth] = true
	}

	switch {
	case lang == "go":
		caps[CapabilityProfileGoLegacy] = true
	case lang == "python" && framework == "fastapi" && db == "postgres":
		caps[CapabilityProfilePythonFastAPI] = true
	default:
		return nil, fmt.Errorf("unsupported target profile: %s/%s/%s", td.Lang, td.Framework, td.DB)
	}

	return caps, nil
}
