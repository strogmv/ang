package normalizer

import (
	"encoding/json"
	"fmt"
	"strings"

	"cuelang.org/go/cue"
)

func (n *Normalizer) ExtractProject(val cue.Value) (*ProjectDef, error) {
	projectVal := val.LookupPath(cue.ParsePath("#Project"))
	if !projectVal.Exists() {
		return nil, nil
	}
	name := strings.TrimSpace(getString(projectVal, "name"))
	version := strings.TrimSpace(getString(projectVal, "version"))
	uiProvider := strings.TrimSpace(getString(projectVal, "ui_provider"))
	if uiProvider == "" {
		uiProvider = strings.TrimSpace(getString(projectVal, "ui.provider"))
	}
	rawArchitectureMode := strings.TrimSpace(getString(projectVal, "architecture.mode"))
	if rawArchitectureMode == "" {
		rawArchitectureMode = strings.TrimSpace(getString(projectVal, "architecture_mode"))
	}

	var plugins []string
	if pVal := projectVal.LookupPath(cue.ParsePath("plugins")); pVal.Exists() {
		it, err := pVal.List()
		if err == nil {
			for it.Next() {
				s, err := it.Value().String()
				if err != nil {
					continue
				}
				s = strings.TrimSpace(s)
				if s == "" {
					continue
				}
				plugins = append(plugins, s)
			}
		}
	}

	var allowCross []CrossServiceRule
	if allowVal := projectVal.LookupPath(cue.ParsePath("architecture.allow_cross_service")); allowVal.Exists() {
		it, err := allowVal.List()
		if err == nil {
			for it.Next() {
				v := it.Value()
				service := strings.TrimSpace(getString(v, "service"))
				entity := strings.TrimSpace(getString(v, "entity"))
				if service == "" || entity == "" {
					continue
				}
				allowCross = append(allowCross, CrossServiceRule{
					Service: service,
					Entity:  entity,
				})
			}
		}
	}

	if name == "" && version == "" && len(plugins) == 0 && uiProvider == "" && rawArchitectureMode == "" && len(allowCross) == 0 {
		return nil, nil
	}

	architectureMode := rawArchitectureMode
	if architectureMode == "" {
		architectureMode = "strict"
	}

	return &ProjectDef{
		Name:              name,
		Version:           version,
		Plugins:           plugins,
		UIProvider:        uiProvider,
		ArchitectureMode:  architectureMode,
		AllowCrossService: allowCross,
	}, nil
}

// ExtractTarget parses #Target from project.cue.
func (n *Normalizer) ExtractTarget(val cue.Value) (*TargetDef, error) {
	// Prefer concrete state.target value over the abstract #Target definition.
	targetVal := val.LookupPath(cue.ParsePath("state.target"))
	if !targetVal.Exists() {
		targetVal = val.LookupPath(cue.ParsePath("#Target"))
	}
	if !targetVal.Exists() {
		// Return defaults
		return &TargetDef{
			Name:      "default",
			Lang:      "go",
			Framework: "chi",
			DB:        "postgres",
			Cache:     "redis",
			Queue:     "nats",
			Storage:   "s3",
		}, nil
	}

	td := parseTargetDef(targetVal, "default")
	return &td, nil
}

// ExtractTargets parses multi-target config from project.cue.
// Supported locations:
// - #Targets: [...]
// - state.targets: [...]
// Falls back to a single target from ExtractTarget.
func (n *Normalizer) ExtractTargets(val cue.Value) ([]TargetDef, error) {
	parseList := func(listVal cue.Value) ([]TargetDef, error) {
		it, err := listVal.List()
		if err != nil {
			return nil, err
		}
		var out []TargetDef
		idx := 1
		for it.Next() {
			td := parseTargetDef(it.Value(), fmt.Sprintf("target%d", idx))
			out = append(out, td)
			idx++
		}
		return out, nil
	}

	// Prefer concrete state.targets over the abstract #Targets definition.
	targetsVal := val.LookupPath(cue.ParsePath("state.targets"))
	if !targetsVal.Exists() {
		targetsVal = val.LookupPath(cue.ParsePath("#Targets"))
	}

	if targetsVal.Exists() {
		targets, err := parseList(targetsVal)
		if err != nil {
			return nil, err
		}
		if len(targets) == 0 {
			return targets, nil
		}
		seen := map[string]int{}
		for i := range targets {
			key := strings.ToLower(strings.TrimSpace(targets[i].Name))
			if key == "" {
				key = fmt.Sprintf("target%d", i+1)
				targets[i].Name = key
			}
			seen[key]++
			if seen[key] > 1 {
				targets[i].Name = fmt.Sprintf("%s-%d", targets[i].Name, seen[key])
			}
		}
		return targets, nil
	}

	td, err := n.ExtractTarget(val)
	if err != nil {
		return nil, err
	}
	if td == nil {
		return nil, nil
	}
	return []TargetDef{*td}, nil
}

// ExtractTransformersConfig parses #Transformers from project.cue.
func (n *Normalizer) ExtractTransformersConfig(val cue.Value) (*TransformersConfig, error) {
	trVal := val.LookupPath(cue.ParsePath("#Transformers"))
	if !trVal.Exists() {
		// Return defaults
		return &TransformersConfig{
			Timestamps:  true,
			SoftDelete:  false,
			Image:       true,
			ThumbSuffix: "_thumb",
			Validation:  true,
		}, nil
	}

	cfg := &TransformersConfig{
		Timestamps:  true,
		SoftDelete:  false,
		Image:       true,
		ThumbSuffix: "_thumb",
		Validation:  true,
	}

	// Parse timestamps
	if ts := trVal.LookupPath(cue.ParsePath("timestamps")); ts.Exists() {
		if enabled := ts.LookupPath(cue.ParsePath("enabled")); enabled.Exists() {
			if b, err := enabled.Bool(); err == nil {
				cfg.Timestamps = b
			}
		}
	}

	// Parse soft_delete
	if sd := trVal.LookupPath(cue.ParsePath("soft_delete")); sd.Exists() {
		if enabled := sd.LookupPath(cue.ParsePath("enabled")); enabled.Exists() {
			if b, err := enabled.Bool(); err == nil {
				cfg.SoftDelete = b
			}
		}
	}

	// Parse image
	if img := trVal.LookupPath(cue.ParsePath("image")); img.Exists() {
		if enabled := img.LookupPath(cue.ParsePath("enabled")); enabled.Exists() {
			if b, err := enabled.Bool(); err == nil {
				cfg.Image = b
			}
		}
		if suffix := img.LookupPath(cue.ParsePath("thumb_suffix")); suffix.Exists() {
			if s, err := suffix.String(); err == nil {
				cfg.ThumbSuffix = s
			}
		}
	}

	// Parse validation
	if vl := trVal.LookupPath(cue.ParsePath("validation")); vl.Exists() {
		if enabled := vl.LookupPath(cue.ParsePath("enabled")); enabled.Exists() {
			if b, err := enabled.Bool(); err == nil {
				cfg.Validation = b
			}
		}
	}

	return cfg, nil
}

func getStringWithDefault(v cue.Value, path, def string) string {
	// Regular lookup works for required fields in definitions.
	res := v.LookupPath(cue.ParsePath(path))
	if res.Exists() {
		if s, err := res.String(); err == nil {
			return strings.TrimSpace(s)
		}
	}
	return def
}

// getOptionalStringField reads a string field that may be declared as optional (?) in
// a CUE definition. The CUE Go API's LookupPath skips optional fields in
// definition-constrained values; we use JSON export as fallback.
func getOptionalStringField(v cue.Value, path string) string {
	// First try the regular path (works if the value is not definition-constrained).
	res := v.LookupPath(cue.ParsePath(path))
	if res.Exists() {
		if s, err := res.String(); err == nil {
			return strings.TrimSpace(s)
		}
	}
	// Fallback: export to JSON and unmarshal (handles optional fields in definitions).
	data, err := v.MarshalJSON()
	if err != nil {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	if val, ok := m[path]; ok {
		if s, ok := val.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func parseTargetDef(targetVal cue.Value, defaultName string) TargetDef {
	outputDir := getOptionalStringField(targetVal, "output_dir")
	if strings.TrimSpace(outputDir) == "" {
		outputDir = getStringWithDefault(targetVal, "outputDir", "")
	}
	frontendAppDir := getOptionalStringField(targetVal, "frontend_app_dir")
	if strings.TrimSpace(frontendAppDir) == "" {
		frontendAppDir = getOptionalStringField(targetVal, "frontendAppDir")
	}
	name := getStringWithDefault(targetVal, "name", defaultName)
	if strings.TrimSpace(name) == "" {
		name = defaultName
	}
	var natsWorkers int
	if v, err := targetVal.LookupPath(cue.ParsePath("nats_workers")).Int64(); err == nil && v > 0 {
		natsWorkers = int(v)
	}
	var natsPublishRetryAttempts int
	if v, err := targetVal.LookupPath(cue.ParsePath("nats_publish_retry_attempts")).Int64(); err == nil && v > 0 {
		natsPublishRetryAttempts = int(v)
	}
	var natsPublishRetryDelayMS int
	if v, err := targetVal.LookupPath(cue.ParsePath("nats_publish_retry_delay_ms")).Int64(); err == nil && v > 0 {
		natsPublishRetryDelayMS = int(v)
	}

	return TargetDef{
		Name:                     strings.TrimSpace(name),
		Lang:                     getStringWithDefault(targetVal, "lang", "go"),
		Framework:                getStringWithDefault(targetVal, "framework", "chi"),
		DB:                       getStringWithDefault(targetVal, "db", "postgres"),
		Cache:                    getStringWithDefault(targetVal, "cache", "redis"),
		Queue:                    getStringWithDefault(targetVal, "queue", "nats"),
		Storage:                  getStringWithDefault(targetVal, "storage", "s3"),
		OutputDir:                strings.TrimSpace(outputDir),
		FrontendAppDir:           strings.TrimSpace(frontendAppDir),
		NatsWorkers:              natsWorkers,
		NatsPublishRetryAttempts: natsPublishRetryAttempts,
		NatsPublishRetryDelayMS:  natsPublishRetryDelayMS,
	}
}
