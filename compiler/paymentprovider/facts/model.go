package ppfacts

const SchemaV1 = "ang/payment-provider-facts/v1"

// Envelope is the wire model for payment-provider facts.
type Envelope struct {
	Schema      string       `json:"schema"`
	ScopeID     string       `json:"scope_id"`
	ProviderID  string       `json:"provider_id"`
	Facts       []Fact       `json:"facts"`
	Evidence    []Evidence   `json:"evidence"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

type Term struct {
	Sort  string `json:"sort"`
	Value string `json:"value"`
}

type Fact struct {
	ID          string   `json:"id"`
	Predicate   string   `json:"predicate"`
	Terms       []Term   `json:"terms"`
	EvidenceIDs []string `json:"evidence_ids,omitempty"`
}

type Evidence struct {
	ID          string `json:"id"`
	Extractor   string `json:"extractor"`
	ContentHash string `json:"content_hash"`
}

type Diagnostic struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
}
