package integration

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// InitOptions configures ang pp init scaffolding.
type InitOptions struct {
	ProjectPath   string
	SID           string
	Label         string
	Name          string
	PackageName   string
	Module        string
	TicketSummary string
	KnowledgeID   string // Expert knowledge/data/<id>.json (default: derived from name/sid)
	Force         bool
}

// InitResult lists files created by InitProject.
type InitResult struct {
	Created []string
	Skipped []string
}

// InitProject scaffolds CUE skeleton and ang.yaml (knowledge lives in Expert, not provider).
func InitProject(opts InitOptions) (InitResult, error) {
	opts = normalizeInitOptions(opts)
	if strings.TrimSpace(opts.SID) == "" {
		return InitResult{}, fmt.Errorf("sid is required")
	}
	if strings.TrimSpace(opts.Label) == "" {
		return InitResult{}, fmt.Errorf("label is required")
	}
	if err := os.MkdirAll(opts.ProjectPath, 0o755); err != nil {
		return InitResult{}, err
	}
	var result InitResult

	write := func(rel string, content []byte) error {
		target := filepath.Join(opts.ProjectPath, rel)
		if _, err := os.Stat(target); err == nil && !opts.Force {
			result.Skipped = append(result.Skipped, rel)
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, content, 0o644); err != nil {
			return err
		}
		result.Created = append(result.Created, rel)
		return nil
	}

	if err := write("ang.yaml", []byte(renderAngYAML(opts))); err != nil {
		return InitResult{}, err
	}
	if err := write(filepath.Join(".cue", "cue.mod", "module.cue"), []byte(renderModuleCUE(opts))); err != nil {
		return InitResult{}, err
	}
	providerCUE, err := renderProviderCUE(opts)
	if err != nil {
		return InitResult{}, err
	}
	if err := write(filepath.Join(".cue", "provider.cue"), []byte(providerCUE)); err != nil {
		return InitResult{}, err
	}
	return result, nil
}

func normalizeInitOptions(opts InitOptions) InitOptions {
	opts.ProjectPath = filepath.Clean(strings.TrimSpace(opts.ProjectPath))
	if opts.ProjectPath == "" {
		opts.ProjectPath = "."
	}
	opts.SID = strings.TrimSpace(opts.SID)
	opts.Label = strings.TrimSpace(opts.Label)
	opts.Name = strings.TrimSpace(opts.Name)
	opts.PackageName = strings.TrimSpace(opts.PackageName)
	if opts.PackageName == "" {
		opts.PackageName = filepath.Base(opts.ProjectPath)
	}
	if opts.Label == "" {
		opts.Label = strings.ToUpper(opts.SID)
	}
	if opts.Name == "" {
		opts.Name = opts.Label
	}
	opts.Module = strings.TrimSpace(opts.Module)
	if opts.Module == "" {
		opts.Module = "transferty.local/" + opts.PackageName
	}
	if strings.TrimSpace(opts.KnowledgeID) == "" {
		opts.KnowledgeID = opts.SID
	}
	return opts
}

func renderAngYAML(opts InitOptions) string {
	return fmt.Sprintf(`cue_root: ".cue"
templates_dir: "../.ang/templates/redirect_checkout"
schema_dir: "../.ang/schema"
expert_knowledge_id: %q
`, opts.KnowledgeID)
}

func renderModuleCUE(opts InitOptions) string {
	return fmt.Sprintf(`module: %q
language: {
	version: "v0.14.0"
}
`, opts.Module)
}

func renderProviderCUE(opts InitOptions) (string, error) {
	const tmpl = `package provider

import "{{.Module}}/schema"

// Intent scaffold — refine using Expert knowledge/data (see ang.yaml expert_knowledge_id)
// Profile: redirect checkout (wallet / hosted page). Templates: .ang/templates/redirect_checkout
provider: schema.#Provider & schema.ProfileRedirectCheckout & {
	package_name: "{{.PackageName}}"
	sid:          "{{.SID}}"
	source:       "{{.Source}}"
	label:        "{{.Label}}"
	mid_prefix:   "{{.MIDPrefix}}"

	struct_name:      "{{.StructName}}"
	constructor_name: "New{{.StructName}}"

	supported_currencies: ["EUR"]
	supported_methods:    ["applepay", "googlepay"]

	currency: {
		code:    "EUR"
		iso_num: 978
		country: "EU"
	}

	// TODO: endpoints from Expert knowledge/data after PM sign-off
	endpoints: {
		payin:        {path: "/TODO/checkout/sessions", method: "POST"}
		payin_status: {path: "/TODO/checkout/sessions", method: "GET"}
	}

	secrets: {
		format:    "API Key:Signing Key"
		separator: ":"
		parts: [
			{name: "API Key", key: "apiKey"},
			{name: "Signing Key", key: "signingKey"},
		]
	}

	auth: {
		type:         "bearer"
		header:       "Authorization"
		secret_key:   "apiKey"
		content_type: "application/json"
	}

	payin_request: {
		name: "createCheckoutSessionRequest"
		fields: [
			{name: "ReferenceID",   json: "referenceId",   source: "tx_id"},
			{name: "Amount",        json: "amount",        source: "tx_amount_float", type: "float64"},
			{name: "Currency",      json: "currency",      source: "tx_currency"},
			{name: "PaymentMethod", json: "paymentMethod", source: "tx_payment_method"},
			{name: "ReturnUrl",     json: "returnUrl",     source: "tx_result_url"},
			{name: "WebhookUrl",    json: "webhookUrl",    source: "tx_callback_url"},
			{name: "Email",         json: "email",         source: "owner_info", owner_key: "email", owner_from: "apm"},
		]
	}

	response_types: [{
		name: "checkoutSessionResponse"
		fields: [
			{name: "ID",          type: "string", json: "id"},
			{name: "RedirectUrl", type: "string", json: "redirectUrl"},
			{name: "State",       type: "string", json: "state"},
		]
	}]

	payin_statuses: [
		{code: "pending",   status: "pending", status_code: "SCodeOk"},
		{code: "completed", status: "success", status_code: "SCodeOk"},
		{code: "failed",    status: "declined", status_code: "SCodeDeclinedByBank"},
		{code: "expired",   status: "declined", status_code: "SCodeTimeouted"},
	]

	callback: {
		tx_id_field:      "ReferenceID"
		foreign_id_field: "SessionID"
		status_field:     "State"
		status_type:      "string"
		fields: [
			{name: "ReferenceID", type: "string", json: "referenceId"},
			{name: "SessionID",   type: "string", json: "sessionId"},
			{name: "State",       type: "string", json: "state"},
		]
	}

	callback_signature: {
		algorithm:  "hmac-sha256"
		secret_key: "signingKey"
		format:     "hmac_body"
		header:     "X-Signature"
		compare:    "equal"
		fields:     [{json: "referenceId"}]
	}

	constructor_deps: [
		{name: "tdsRedirector", type: "model.TDSRedirector", pkg: "gitlab.q-tech.host/transferty/backend/tnx_processor/model"},
		{name: "txPathLogger", type: "model.TxPathLogger", pkg: "gitlab.q-tech.host/transferty/backend/tnx_processor/model"},
	]
}
`
	t, err := template.New("provider").Parse(tmpl)
	if err != nil {
		return "", err
	}
	mid := strings.ToUpper(opts.SID)
	if mid == "" {
		mid = strings.ToUpper(opts.PackageName)
	}
	data := map[string]string{
		"Module":      opts.Module,
		"PackageName": opts.PackageName,
		"SID":         opts.SID,
		"Source":      ppSourceName(opts.PackageName),
		"Label":       opts.Label,
		"MIDPrefix":   mid,
		"StructName":  ppSourceName(opts.PackageName),
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func toExportIdentifier(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '_' || r == '-' })
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
	}
	return strings.Join(parts, "")
}

func ppSourceName(packageName string) string {
	return "PP" + toExportIdentifier(packageName)
}
