package compiler

import (
	"fmt"
	"slices"
	"strings"

	"github.com/strogmv/ang/angir/normalizer"
)

// Capability describes a target generation/runtime ability.
type Capability string

const (
	CapabilityHTTP    Capability = "http"
	CapabilityGRPC    Capability = "grpc"
	CapabilitySQLRepo Capability = "sql_repo"
	CapabilityWS      Capability = "ws"
	CapabilityEvents  Capability = "events"
	CapabilityAuth    Capability = "auth"

	// Usage capabilities are granted per project by what its IR actually uses
	// (see emitter.UsageCapabilities). A step that requires one is skipped as
	// "not used by the project" when the project does not need its output.
	CapabilityUsesRedisClient        Capability = "uses_redis_client"
	CapabilityUsesMongo              Capability = "uses_mongo"
	CapabilityUsesS3                 Capability = "uses_s3"
	CapabilityUsesRefreshStoreMemory Capability = "uses_refresh_store_memory"
	CapabilityUsesRefreshStorePG     Capability = "uses_refresh_store_postgres"
	CapabilityUsesRefreshStoreHybrid Capability = "uses_refresh_store_hybrid"
	// Nothing in the generated runtime consumes these outputs today, so no
	// project is granted them; the steps stay available for when one does.
	CapabilityUsesDTO         Capability = "uses_dto"
	CapabilityUsesRepoStubs   Capability = "uses_repo_stubs"
	CapabilityUsesDIContainer Capability = "uses_di_container"
	CapabilityUsesAIAct       Capability = "uses_ai_act"
	CapabilityUsesNIS2        Capability = "uses_nis2"
	CapabilityUsesSQLC        Capability = "uses_sqlc"

	// Profile capabilities are explicit backend generation profiles.
	CapabilityProfileGoLegacy Capability = "profile_go_legacy"
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

// SupportedTargetValues lists the target stack values ANG generates. An empty
// value takes the default (the first entry). Anything else is rejected instead
// of being generated as Go/chi/Postgres anyway.
var SupportedTargetValues = []struct {
	Field  string
	Values []string
}{
	{"lang", []string{"go"}},
	{"framework", []string{"chi", "grpc"}},
	{"db", []string{"postgres"}},
	{"cache", []string{"redis", "none"}},
	{"queue", []string{"nats", "none"}},
	{"storage", []string{"s3", "none"}},
}

// ValidateTargetDef reports the first target field set to a stack ANG does not
// generate.
func ValidateTargetDef(td normalizer.TargetDef) error {
	got := map[string]string{
		"lang": td.Lang, "framework": td.Framework, "db": td.DB,
		"cache": td.Cache, "queue": td.Queue, "storage": td.Storage,
	}
	for _, field := range SupportedTargetValues {
		value := strings.ToLower(strings.TrimSpace(got[field.Field]))
		if value == "" || slices.Contains(field.Values, value) {
			continue
		}
		return fmt.Errorf("unsupported target %s %q (supported: %s)", field.Field, got[field.Field], strings.Join(field.Values, ", "))
	}
	return nil
}

// ResolveTargetCapabilities derives a stable capability matrix from target settings.
func ResolveTargetCapabilities(td normalizer.TargetDef) (CapabilitySet, error) {
	if err := ValidateTargetDef(td); err != nil {
		return nil, err
	}
	caps := CapabilitySet{}

	lang := strings.ToLower(strings.TrimSpace(td.Lang))
	framework := strings.ToLower(strings.TrimSpace(td.Framework))
	db := strings.ToLower(strings.TrimSpace(td.DB))
	queue := strings.ToLower(strings.TrimSpace(td.Queue))

	switch framework {
	case "chi", "grpc":
		caps[CapabilityHTTP] = true
	}
	if framework == "grpc" {
		caps[CapabilityGRPC] = true
	}

	if db == "postgres" {
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

	if lang != "go" {
		return nil, fmt.Errorf("unsupported target profile: %s/%s/%s (ANG generates Go only)", td.Lang, td.Framework, td.DB)
	}
	caps[CapabilityAuth] = true
	caps[CapabilityProfileGoLegacy] = true

	return caps, nil
}

// IsUsageCapability reports whether cap is granted by project usage rather than
// by the target profile.
func IsUsageCapability(cap Capability) bool {
	return strings.HasPrefix(string(cap), "uses_")
}
