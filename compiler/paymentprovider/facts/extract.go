package ppfacts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/strogmv/ang/compiler/paymentprovider"
)

// ExtractOptions configures payment-provider fact extraction.
type ExtractOptions struct {
	ProjectPath string
	CueRoot     string
	SchemaDir   string
}

var knownOperations = []string{
	"init_pay",
	"init_payout",
	"check_status",
	"init_refund",
	"init_pay_p2p",
	"cancel_pay",
	"init_subscription",
	"subscription_pay",
	"parse_callback",
	"validate_callback",
	"finish_callback",
}

var knownCapabilities = []struct {
	name    string
	enabled func(*paymentprovider.ProviderSpec) bool
}{
	{"payin", func(spec *paymentprovider.ProviderSpec) bool { return spec.HasPayin }},
	{"payout", func(spec *paymentprovider.ProviderSpec) bool { return spec.HasPayout }},
	{"p2p", func(spec *paymentprovider.ProviderSpec) bool { return spec.HasP2P }},
	{"subscription", func(spec *paymentprovider.ProviderSpec) bool { return spec.HasSubscription }},
	{"refund", func(spec *paymentprovider.ProviderSpec) bool { return spec.HasRefund }},
	{"cancel", func(spec *paymentprovider.ProviderSpec) bool { return spec.HasCancel }},
}

var cueKindToOperation = map[string]string{
	"init_pay":            "init_pay",
	"init_payout":         "init_payout",
	"check_status":        "check_status",
	"check_status_payin":  "check_status",
	"check_status_payout": "check_status",
	"init_refund":         "init_refund",
	"init_pay_p2p":        "init_pay_p2p",
	"cancel_pay":          "cancel_pay",
	"init_subscription":   "init_subscription",
	"subscription_pay":    "subscription_pay",
	"parse_callback":      "parse_callback",
	"validate_callback":   "validate_callback",
	"finish_callback":     "finish_callback",
}

// Extract builds canonical payment-provider facts for a provider project.
func Extract(opts ExtractOptions) (Envelope, error) {
	if strings.TrimSpace(opts.ProjectPath) == "" {
		return Envelope{}, fmt.Errorf("project path is required")
	}
	projectPath := filepath.Clean(opts.ProjectPath)
	pc := paymentprovider.LoadProjectConfig(projectPath)
	cueRoot := opts.CueRoot
	if cueRoot == "" {
		cueRoot = pc.CueRoot
	}
	schemaDir := strings.TrimSpace(opts.SchemaDir)
	if schemaDir == "" && pc.SchemaDir != "" {
		resolved, err := paymentprovider.ResolvePath(projectPath, pc.SchemaDir)
		if err != nil {
			return Envelope{}, err
		}
		schemaDir = resolved
	} else if schemaDir != "" {
		resolved, err := paymentprovider.ResolvePath(projectPath, schemaDir)
		if err != nil {
			return Envelope{}, err
		}
		schemaDir = resolved
	}

	spec, err := paymentprovider.Load(projectPath, cueRoot, schemaDir)
	if err != nil {
		return Envelope{}, err
	}
	scopeID := "payment-provider:" + spec.SID

	builder := &extractBuilder{
		projectPath: projectPath,
		spec:        spec,
		scopeID:     scopeID,
	}

	cueEvidence, err := builder.addCueEvidence(spec)
	if err != nil {
		return Envelope{}, err
	}
	builder.addProviderFact(cueEvidence)
	builder.addCapabilityFacts(cueEvidence)

	declared := declaredOperations(spec)
	methods, err := classifyProviderMethods(projectPath, spec.StructName)
	if err != nil {
		return Envelope{}, fmt.Errorf("parse provider go sources: %w", err)
	}
	goEvidence := builder.addGoEvidence(methods)
	builder.addOperationFacts(declared, methods, goEvidence)

	if drift, err := paymentprovider.CheckSchema(projectPath, cueRoot); err != nil {
		builder.addSchemaSync("unknown", "")
	} else {
		state := "in_sync"
		if len(drift) > 0 {
			state = "drift"
		}
		schemaEvidence := builder.addSchemaEvidence(drift)
		builder.addSchemaSync(state, schemaEvidence)
		builder.addSchemaDriftFacts(drift, schemaEvidence)
	}

	vetIssues := paymentprovider.Vet(spec)
	vetEvidence := builder.addVetEvidence(vetIssues)
	builder.addVetFacts(vetIssues, vetEvidence)
	builder.addSecretFacts(cueEvidence)
	builder.addAuthFacts(cueEvidence)
	builder.addEndpointFacts(spec, cueEvidence)
	builder.addRuntimePolicyFacts(cueEvidence)
	builder.addBehaviorFacts(methods, goEvidence)
	builder.addTestAreaFacts()

	env := Envelope{
		Schema:      SchemaV1,
		ScopeID:     scopeID,
		ProviderID:  spec.SID,
		Facts:       builder.facts,
		Evidence:    builder.evidence,
		Diagnostics: builder.diagnostics,
	}
	canonical := Canonicalize(env)
	if err := Validate(canonical); err != nil {
		return Envelope{}, err
	}
	return canonical, nil
}

type extractBuilder struct {
	projectPath string
	spec        *paymentprovider.ProviderSpec
	scopeID     string
	facts       []Fact
	evidence    []Evidence
	diagnostics []Diagnostic
}

func (b *extractBuilder) providerTerm() Term {
	return Term{Sort: "provider", Value: b.spec.SID}
}

func (b *extractBuilder) addFact(predicate string, terms []Term, evidenceIDs ...string) {
	b.facts = append(b.facts, Fact{
		ID:          factID(predicate, terms...),
		Predicate:   predicate,
		Terms:       terms,
		EvidenceIDs: evidenceIDs,
	})
}

func (b *extractBuilder) addEvidence(extractor string, payload []byte) string {
	hash := contentHash(payload)
	id := evidenceID(extractor, hash)
	for _, existing := range b.evidence {
		if existing.ID == id {
			return id
		}
	}
	evidence := Evidence{
		ID:          id,
		Extractor:   extractor,
		ContentHash: hash,
	}
	b.evidence = append(b.evidence, evidence)
	return id
}

func (b *extractBuilder) addCueEvidence(spec *paymentprovider.ProviderSpec) (string, error) {
	payload, err := json.Marshal(cueEvidencePayload{
		SID:          spec.SID,
		Capabilities: capabilityState(spec),
		Operations:   operationKinds(spec),
		AuthType:     authKind(spec),
		SecretKeys:   secretKeys(spec),
	})
	if err != nil {
		return "", err
	}
	return b.addEvidence("cue_provider_spec", payload), nil
}

func (b *extractBuilder) addGoEvidence(methods methodImplementation) string {
	names := make([]string, 0, len(methods))
	for name, impl := range methods {
		if strings.HasPrefix(name, "behavior:") {
			continue
		}
		names = append(names, name+"="+impl)
	}
	payload, _ := json.Marshal(names)
	return b.addEvidence("go_provider_methods", payload)
}

func (b *extractBuilder) addSchemaEvidence(drift []string) string {
	payload, _ := json.Marshal(drift)
	return b.addEvidence("schema_check", payload)
}

func (b *extractBuilder) addVetEvidence(issues []paymentprovider.VetIssue) string {
	codes := make([]string, 0, len(issues))
	for _, issue := range issues {
		codes = append(codes, issue.Code+":"+issue.Severity)
	}
	payload, _ := json.Marshal(codes)
	return b.addEvidence("pp_vet", payload)
}

func (b *extractBuilder) addProviderFact(evidenceID string) {
	b.addFact("pp_provider", []Term{
		b.providerTerm(),
		{Sort: "provider_id", Value: b.spec.SID},
	}, evidenceID)
}

func (b *extractBuilder) addCapabilityFacts(evidenceID string) {
	for _, capability := range knownCapabilities {
		value := "false"
		if capability.enabled(b.spec) {
			value = "true"
		}
		b.addFact("pp_capability", []Term{
			b.providerTerm(),
			{Sort: "capability", Value: capability.name},
			{Sort: "enabled", Value: value},
		}, evidenceID)
	}
}

func (b *extractBuilder) addOperationFacts(declared map[string]bool, methods methodImplementation, evidenceID string) {
	for _, operation := range knownOperations {
		declaration := "absent"
		if declared[operation] {
			declaration = "declared"
		}
		implementation := methods[operation]
		if implementation == "" {
			implementation = "absent"
		}
		b.addFact("pp_operation", []Term{
			b.providerTerm(),
			{Sort: "operation", Value: operation},
			{Sort: "declaration", Value: declaration},
			{Sort: "implementation", Value: implementation},
		}, evidenceID)
	}
}

func (b *extractBuilder) addSchemaSync(state string, evidenceID string) {
	terms := []Term{b.providerTerm(), {Sort: "state", Value: state}}
	if evidenceID == "" {
		b.addFact("pp_schema_sync", terms)
		return
	}
	b.addFact("pp_schema_sync", terms, evidenceID)
}

func (b *extractBuilder) addSchemaDriftFacts(drift []string, evidenceID string) {
	allowed, _ := paymentprovider.BundledSchemaFiles()
	allowedSet := map[string]struct{}{}
	for _, name := range allowed {
		allowedSet[name] = struct{}{}
	}
	for _, entry := range drift {
		base := strings.SplitN(entry, ":", 2)[0]
		base = filepath.Base(strings.TrimSpace(base))
		if _, ok := allowedSet[base]; !ok {
			continue
		}
		b.addFact("pp_schema_drift", []Term{
			b.providerTerm(),
			{Sort: "schema_file", Value: base},
		}, evidenceID)
	}
}

func (b *extractBuilder) addVetFacts(issues []paymentprovider.VetIssue, evidenceID string) {
	for _, issue := range issues {
		b.addFact("pp_vet_issue", []Term{
			b.providerTerm(),
			{Sort: "code", Value: issue.Code},
			{Sort: "severity", Value: issue.Severity},
		}, evidenceID)
	}
}

func (b *extractBuilder) addSecretFacts(evidenceID string) {
	for _, part := range b.spec.Secrets.Parts {
		partType := strings.TrimSpace(part.Type)
		if partType == "" {
			partType = "string"
		}
		optional := "false"
		if part.Optional {
			optional = "true"
		}
		b.addFact("pp_secret_part", []Term{
			b.providerTerm(),
			{Sort: "key", Value: part.Key},
			{Sort: "optional", Value: optional},
			{Sort: "type", Value: partType},
		}, evidenceID)
	}
}

func (b *extractBuilder) addAuthFacts(evidenceID string) {
	kind, header, masked := authDetails(b.spec)
	b.addFact("pp_auth", []Term{
		b.providerTerm(),
		{Sort: "kind", Value: kind},
		{Sort: "header", Value: header},
		{Sort: "masked", Value: masked},
	}, evidenceID)
}

func (b *extractBuilder) addEndpointFacts(spec *paymentprovider.ProviderSpec, evidenceID string) {
	seen := map[string]struct{}{}
	for _, op := range spec.Operations {
		operation := cueKindToOperation[strings.TrimSpace(op.Kind)]
		if operation == "" {
			continue
		}
		key := operation + "\x00" + strings.TrimSpace(op.Transport.Endpoint)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		endpointKey := strings.TrimSpace(op.Transport.Endpoint)
		if endpointKey == "" {
			continue
		}
		endpoint, ok := spec.Endpoints[endpointKey]
		if !ok {
			continue
		}
		method := strings.ToUpper(strings.TrimSpace(endpoint.Method))
		if method == "" {
			method = "POST"
		}
		path := strings.TrimSpace(endpoint.Path)
		if path == "" {
			continue
		}
		b.addFact("pp_endpoint", []Term{
			b.providerTerm(),
			{Sort: "operation", Value: operation},
			{Sort: "method", Value: method},
			{Sort: "path", Value: path},
		}, evidenceID)
	}
}

func (b *extractBuilder) addRuntimePolicyFacts(evidenceID string) {
	if b.spec.RuntimePolicyConfig == nil {
		return
	}
	cfg := b.spec.RuntimePolicyConfig
	policies := map[string]string{
		"request_timeout":       cfg.Timeouts.RequestTimeout,
		"check_status_timeout":  cfg.Timeouts.CheckStatusTimeout,
		"callback_wait_timeout": cfg.Timeouts.CallbackWaitTimeout,
		"max_attempts":          fmt.Sprintf("%d", cfg.Retries.MaxAttempts),
		"initial_backoff":       cfg.Retries.InitialBackoff,
		"max_backoff":           cfg.Retries.MaxBackoff,
		"retry_on_not_found":    fmtBool(cfg.Retries.RetryOnNotFound),
		"retry_on_5xx":          fmtBool(cfg.Retries.RetryOn5xx),
		"retry_on_rate_limit":   fmtBool(cfg.Retries.RetryOnRateLimit),
		"max_callback_body_bytes": fmt.Sprintf("%d", cfg.Limits.MaxCallbackBodyBytes),
		"max_pending_age":         cfg.Limits.MaxPendingAge,
	}
	for name, value := range policies {
		value = strings.TrimSpace(value)
		if value == "" || value == "0" || value == "false" {
			continue
		}
		b.addFact("pp_runtime_policy", []Term{
			b.providerTerm(),
			{Sort: "policy", Value: name},
			{Sort: "value", Value: value},
		}, evidenceID)
	}
}

func (b *extractBuilder) addBehaviorFacts(methods methodImplementation, evidenceID string) {
	for _, behavior := range []string{
		"rsa_oaep_card_encryption",
		"rsa_pkcs1v15_callback_verification",
	} {
		key := "behavior:" + behavior
		if methods[key] != "present" {
			continue
		}
		b.addFact("pp_behavior", []Term{
			b.providerTerm(),
			{Sort: "behavior", Value: behavior},
		}, evidenceID)
	}
}

func (b *extractBuilder) addTestAreaFacts() {
	areas := detectTestAreas(b.projectPath)
	payload, _ := json.Marshal(areas)
	evidenceID := b.addEvidence("go_tests", payload)
	for _, area := range []string{"unit", "behavior", "callback", "live"} {
		value := "false"
		if areas[area] {
			value = "true"
		}
		b.addFact("pp_test_area", []Term{
			b.providerTerm(),
			{Sort: "area", Value: area},
			{Sort: "present", Value: value},
		}, evidenceID)
	}
}

type cueEvidencePayload struct {
	SID          string   `json:"sid"`
	Capabilities []string `json:"capabilities"`
	Operations   []string `json:"operations"`
	AuthType     string   `json:"auth_type"`
	SecretKeys   []string `json:"secret_keys"`
}

func declaredOperations(spec *paymentprovider.ProviderSpec) map[string]bool {
	out := map[string]bool{}
	for _, op := range spec.Operations {
		if canonical, ok := cueKindToOperation[strings.TrimSpace(op.Kind)]; ok {
			out[canonical] = true
		}
	}
	if callbackDeclared(spec) {
		for _, operation := range []string{"parse_callback", "validate_callback", "finish_callback"} {
			out[operation] = true
		}
	}
	if checkStatusDeclared(spec) {
		out["check_status"] = true
	}
	return out
}

func callbackDeclared(spec *paymentprovider.ProviderSpec) bool {
	if spec == nil {
		return false
	}
	if spec.CallbackSignature != nil {
		alg := strings.TrimSpace(spec.CallbackSignature.Algorithm)
		if alg != "" && !strings.EqualFold(alg, "none") {
			return true
		}
	}
	if spec.Callback == nil {
		return false
	}
	if strings.TrimSpace(spec.Callback.TxIDField) != "" ||
		strings.TrimSpace(spec.Callback.ForeignIDField) != "" ||
		strings.TrimSpace(spec.Callback.StatusField) != "" ||
		len(spec.Callback.Fields) > 0 {
		return true
	}
	return false
}

func checkStatusDeclared(spec *paymentprovider.ProviderSpec) bool {
	if spec == nil {
		return false
	}
	if spec.CheckStatusConfig != nil {
		return true
	}
	for key := range spec.Endpoints {
		if strings.Contains(strings.ToLower(strings.TrimSpace(key)), "status") {
			return true
		}
	}
	return false
}

func capabilityState(spec *paymentprovider.ProviderSpec) []string {
	out := make([]string, 0, len(knownCapabilities))
	for _, capability := range knownCapabilities {
		if capability.enabled(spec) {
			out = append(out, capability.name)
		}
	}
	return out
}

func operationKinds(spec *paymentprovider.ProviderSpec) []string {
	out := make([]string, 0, len(spec.Operations))
	for _, op := range spec.Operations {
		out = append(out, strings.TrimSpace(op.Kind))
	}
	return out
}

func secretKeys(spec *paymentprovider.ProviderSpec) []string {
	out := make([]string, 0, len(spec.Secrets.Parts))
	for _, part := range spec.Secrets.Parts {
		out = append(out, part.Key)
	}
	return out
}

func authKind(spec *paymentprovider.ProviderSpec) string {
	if spec.Auth == nil {
		return "none"
	}
	return strings.TrimSpace(spec.Auth.Type)
}

func authDetails(spec *paymentprovider.ProviderSpec) (kind, header, masked string) {
	if spec.Auth == nil {
		return "none", "none", "false"
	}
	header = strings.TrimSpace(spec.Auth.Header)
	if header == "" {
		header = "Authorization"
	}
	masked = "false"
	if spec.Auth.Masked {
		masked = "true"
	}
	kind = strings.TrimSpace(spec.Auth.Type)
	if kind == "" {
		kind = "none"
	}
	return kind, header, masked
}

func fmtBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func detectTestAreas(projectPath string) map[string]bool {
	areas := map[string]bool{
		"unit":     false,
		"behavior": false,
		"callback": false,
		"live":     false,
	}
	entries, err := os.ReadDir(projectPath)
	if err != nil {
		return areas
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, "_test.go") {
			continue
		}
		switch {
		case strings.Contains(name, "behavior_test"):
			areas["behavior"] = true
		case strings.Contains(name, "callback_test"):
			areas["callback"] = true
		case strings.Contains(name, "live_test"):
			areas["live"] = true
		default:
			areas["unit"] = true
		}
	}
	return areas
}
