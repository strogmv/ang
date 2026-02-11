package normalizer

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"cuelang.org/go/cue"
)

const (
	InfraKeyConfig             = "config"
	InfraKeyAuth               = "auth"
	InfraKeyNotificationMuting = "notification_muting"

	infraErrCodeConfigParse = "CUE_INFRA_CONFIG_PARSE_ERROR"
	infraErrCodeAuthParse   = "CUE_INFRA_AUTH_PARSE_ERROR"
)

type InfraLanguage string

const (
	InfraLanguageGo     InfraLanguage = "go"
	InfraLanguagePython InfraLanguage = "python"
)

type InfraExtractor func(*Normalizer, cue.Value) (any, error)
type InfraContextHook func(value any, patch *InfraContextPatch)

type InfraStepSpec struct {
	Name     string
	Requires []string
}

type InfraResolvedStep struct {
	Key      string
	Name     string
	Requires []string
}

type InfraDef struct {
	Key         string
	CUEPath     string
	Type        reflect.Type
	Template    string
	ErrorCode   string
	ErrorOp     string
	Extractor   InfraExtractor
	ContextHook InfraContextHook
	Steps       map[InfraLanguage]InfraStepSpec
}

type InfraContextPatch struct {
	AuthService        string
	AuthRefreshStore   string
	NotificationMuting bool
	ForceHasCache      bool
	ForceHasSQL        bool
}

type InfraExtractError struct {
	Key  string
	Code string
	Op   string
	Err  error
}

func (e *InfraExtractError) Error() string {
	return fmt.Sprintf("%s: %s: %v", e.Code, e.Op, e.Err)
}

func (e *InfraExtractError) Unwrap() error { return e.Err }

func (e *InfraExtractError) FailParams() (code, op string, err error) {
	return e.Code, e.Op, e.Err
}

var registeredInfra = map[string]InfraDef{}

func Register(key string, def InfraDef) {
	k := strings.TrimSpace(key)
	if k == "" {
		panic("normalizer infra: empty key")
	}
	if strings.TrimSpace(def.CUEPath) == "" {
		panic("normalizer infra: empty CUEPath for key " + k)
	}
	if def.Extractor == nil {
		panic("normalizer infra: nil extractor for key " + k)
	}
	if _, exists := registeredInfra[k]; exists {
		panic("normalizer infra: duplicate key " + k)
	}
	def.Key = k
	if def.Steps != nil {
		copied := make(map[InfraLanguage]InfraStepSpec, len(def.Steps))
		for lang, spec := range def.Steps {
			reqs := append([]string(nil), spec.Requires...)
			copied[lang] = InfraStepSpec{Name: spec.Name, Requires: reqs}
		}
		def.Steps = copied
	}
	registeredInfra[k] = def
}

func registeredDefsOrdered() []InfraDef {
	keys := make([]string, 0, len(registeredInfra))
	for k := range registeredInfra {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	defs := make([]InfraDef, 0, len(keys))
	for _, k := range keys {
		defs = append(defs, registeredInfra[k])
	}
	return defs
}

type InfraRegistry struct {
	defs []InfraDef
}

func NewInfraRegistry() *InfraRegistry {
	return &InfraRegistry{defs: registeredDefsOrdered()}
}

func (r *InfraRegistry) ExtractAll(n *Normalizer, val cue.Value) (map[string]any, error) {
	out := make(map[string]any, len(r.defs))
	for _, def := range r.defs {
		parsed, err := def.Extractor(n, val)
		if err != nil {
			return nil, &InfraExtractError{
				Key:  def.Key,
				Code: def.ErrorCode,
				Op:   def.ErrorOp,
				Err:  err,
			}
		}
		out[def.Key] = parsed
	}
	return out, nil
}

func (r *InfraRegistry) BuildContextPatch(values map[string]any) InfraContextPatch {
	var patch InfraContextPatch
	for _, def := range r.defs {
		if def.ContextHook == nil {
			continue
		}
		def.ContextHook(values[def.Key], &patch)
	}
	return patch
}

func (r *InfraRegistry) StepsForValues(lang InfraLanguage, values map[string]any) []InfraResolvedStep {
	steps := make([]InfraResolvedStep, 0, len(r.defs))
	for _, def := range r.defs {
		if values[def.Key] == nil {
			continue
		}
		spec, ok := def.Steps[lang]
		if !ok || strings.TrimSpace(spec.Name) == "" {
			continue
		}
		steps = append(steps, InfraResolvedStep{
			Key:      def.Key,
			Name:     spec.Name,
			Requires: append([]string(nil), spec.Requires...),
		})
	}
	return steps
}

func InfraConfig(values map[string]any) *ConfigDef {
	def, _ := values[InfraKeyConfig].(*ConfigDef)
	return def
}

func InfraAuth(values map[string]any) *AuthDef {
	def, _ := values[InfraKeyAuth].(*AuthDef)
	return def
}

func InfraNotificationMuting(values map[string]any) *NotificationMutingDef {
	def, _ := values[InfraKeyNotificationMuting].(*NotificationMutingDef)
	return def
}

func init() {
	Register(InfraKeyConfig, InfraDef{
		CUEPath:   "#AppConfig",
		Type:      reflect.TypeOf(ConfigDef{}),
		Template:  "config",
		ErrorCode: infraErrCodeConfigParse,
		ErrorOp:   "extract config",
		Extractor: func(n *Normalizer, v cue.Value) (any, error) {
			return n.ExtractConfig(v)
		},
	})

	Register(InfraKeyAuth, InfraDef{
		CUEPath:   "#Auth",
		Type:      reflect.TypeOf(AuthDef{}),
		Template:  "auth",
		ErrorCode: infraErrCodeAuthParse,
		ErrorOp:   "extract auth",
		Extractor: func(n *Normalizer, v cue.Value) (any, error) {
			return n.ExtractAuth(v)
		},
		ContextHook: func(value any, patch *InfraContextPatch) {
			auth, _ := value.(*AuthDef)
			if auth == nil {
				return
			}
			patch.AuthService = auth.Service
			patch.AuthRefreshStore = auth.RefreshStore
			store := strings.TrimSpace(auth.RefreshStore)
			if strings.EqualFold(store, "redis") || strings.EqualFold(store, "hybrid") {
				patch.ForceHasCache = true
			}
			if strings.EqualFold(store, "hybrid") {
				patch.ForceHasSQL = true
			}
		},
		Steps: map[InfraLanguage]InfraStepSpec{
			InfraLanguagePython: {
				Name:     "Python Auth Stores",
				Requires: []string{"profile_python_fastapi", "auth"},
			},
		},
	})

	Register(InfraKeyNotificationMuting, InfraDef{
		CUEPath:   "#NotificationMuting",
		Type:      reflect.TypeOf(NotificationMutingDef{}),
		Template:  "notification_muting.tmpl",
		ErrorCode: infraErrCodeConfigParse,
		ErrorOp:   "extract notification muting",
		Extractor: func(n *Normalizer, v cue.Value) (any, error) {
			return n.ExtractNotificationMuting(v)
		},
		ContextHook: func(value any, patch *InfraContextPatch) {
			def, _ := value.(*NotificationMutingDef)
			if def != nil && def.Enabled {
				patch.NotificationMuting = true
			}
		},
		Steps: map[InfraLanguage]InfraStepSpec{
			InfraLanguageGo: {
				Name:     "Notification Muting",
				Requires: []string{"profile_go_legacy"},
			},
		},
	})
}
