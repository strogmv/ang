package paymentprovider

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// TemplateData is passed to all payment provider Go templates.
type TemplateData struct {
	PackageName string
	SID         string
	Source      string
	Label       string
	MIDPrefix   string

	StructName          string
	ConstructorName     string
	ConstructorParams   string
	ConstructorTestArgs string

	PaymentSource string

	HasPayin        bool
	HasPayout       bool
	HasP2P          bool
	HasCancel       bool
	HasRefund       bool
	HasSubscription bool

	PayinRequest        *ResolvedRequestDef
	PayoutRequest       *ResolvedRequestDef
	PayinStatusRequest  *ResolvedRequestDef
	PayoutStatusRequest *ResolvedRequestDef
	P2PRequest          *ResolvedRequestDef
	RefundRequest       *ResolvedRequestDef

	// Methods is what the provider calls each method the contract supports. It
	// carries no fields of its own — those belong to the per-method objects that
	// use them.
	Methods []ResolvedMethod

	// NeedsOwnerInfoHelper is true when any request constructor reads the
	// ownerInfo(ps) local — the helper is emitted once in the package.
	NeedsOwnerInfoHelper bool

	// AllRequestFields is every leaf field of every request definition, flattened
	// across grouping objects. A template that has to answer "does this provider
	// bind anything to X" reads it once instead of walking each definition and
	// each nesting level itself.
	AllRequestFields []ResolvedField

	PayinRequestType  string
	PayoutRequestType string
	RefundRequestType string

	PayinEndpointConst        string
	PayoutEndpointConst       string
	RefundEndpointConst       string
	PayinStatusEndpointConst  string
	PayoutStatusEndpointConst string
	PayoutStatusSameAsPayout  bool
	PayinStatusMethod         string
	PayoutStatusMethod        string
	PayoutMethod              string
	PayoutPathTxID            bool
	CheckStatusPathFormatTxID bool

	PayinResponseType     string
	PayoutResponseType    string
	RefundResponseType    string
	PayinForeignIDField   string
	PayinRedirectURLField string
	PayoutForeignIDField  string
	RefundForeignIDField  string

	SecretParts                       []SecretPart
	SecretPartsCount                  int
	SecretPartsNeedTransform          bool
	SecretPartsSimple                 bool
	SecretHasJoinRemainder            bool
	SecretSeparator                   string
	SecretFormat                      string
	SecretTestValue                   string
	HasOptionalReturnRecipientDetails bool

	HasOptionalSecretParts bool

	SigningAlgorithm    string
	SigningFormat       string
	SigningSecretField  string
	UseBasicAuth        bool
	UseTransfertyH2H    bool
	UseMacanP2P         bool
	UsePaytechGateway   bool
	UseFluxsgate        bool
	UseRedirectCheckout bool
	UseCentrobillHPP    bool
	SecretUseLabels     bool // CUE: secrets.use_labels
	PubKeyField         string
	SecretKeyField      string

	PayoutRuntime             *PayoutRuntimeTemplate
	InitPayoutPolicy          *InitPayoutPolicyTemplate
	RequestSigning            *RequestSigningTemplate
	CheckStatusForeignIDEmpty string
	ResponseFormat            string
	CallbackFormat            string
	ResponseEnvelope          *ResponseEnvelopeTemplate
	KeysEndpoint              *KeysEndpointTemplate
	CardEncryption            *CardEncryptionTemplate
	PayoutStatusValueField    string
	HasCallbackNestedPaths    bool

	PayinResponsePayloadType  string
	PayoutResponsePayloadType string
	PayinStatusResponseType   string
	PayoutStatusResponseType  string
	PayinStatusField          string
	PayoutStatusField         string

	CheckStatusConfig *CheckStatusConfigTemplate

	HasBalanceFetcher        bool
	HasMobileProcessor       bool
	HasOTPFormRedirector     bool
	HasTDSRedirector         bool
	HasPaymentMethodSelector bool
	HasCustomerRandomization bool

	HasCheckStatusConfig bool
	CheckStatusPeriod    string
	CheckStatusByTxType  bool
	HasStatusEndpoints   bool

	HTTPAuth                  *AuthConfigTemplate
	CallbackSignature         *CallbackSignatureTemplate
	HasCallbackSigFloatFormat bool

	HasOTPConfig         bool
	OTPHandlesExternally bool

	CallbackTxIDField       string
	CallbackForeignIDField  string
	CallbackStatusField     string
	CallbackErrorCodeField  string
	CallbackMessageField    string
	CallbackReturnCodeField string

	P2PMethodCheck string
	CurrencyCode   string

	SupportedMethods    []string
	SupportedCurrencies []string
	HasMultiCurrency    bool

	HasP2PDualEndpoint      bool
	P2PCardEndpointConst    string
	P2PAPMEndpointConst     string
	HasPayoutDualEndpoint   bool
	PayoutCardEndpointConst string
	PayoutAPMEndpointConst  string

	Endpoints []EndpointTemplate

	RequestLiteralConsts []RequestLiteralConst

	PayinStatuses       []StatusTemplate
	PayoutStatuses      []StatusTemplate
	PayoutStatusesExtra []StatusTemplate
	ErrorCodes          []ErrorTemplate
	ErrorCodesNumeric   bool
	StatusDetails       []ErrorTemplate

	PayinStatusTests  []StatusTestTemplate
	PayoutStatusTests []StatusTestTemplate
	StatusInputType   string
	StatusCodeType    string // "string" | "int" for provider status codes

	AllRequestTypes []RequestTypeTemplate
	ResponseTypes   []ResponseType
	CallbackFields  []StructField

	HasAnyRedacted bool
	ExtraImports   []string

	ResponseLoggingMode string

	// Optional callback return-url query channel (dual-channel callbacks).
	CallbackReturnQueryTxIDParam    string
	CallbackReturnQueryStatusValue  string
	CallbackReturnQueryInfoCallback bool

	// Advanced capability knobs (schema-driven).
	CheckStatusThrottleConfig *CheckStatusThrottleConfig
	CrossInstanceStateConfig  *CrossInstanceStateConfig
	StatusConfirmationConfig  *StatusConfirmationConfig
	ExtendedCallbackConfig    *ExtendedCallbackConfig
	RuntimePolicyConfig       *RuntimePolicyConfig
	Async3DSConfig            *Async3DSConfig

	Operations          []OperationTemplate
	UseOperationRuntime bool // operations table + operation-scoped overrides (not for macan_p2p yet)
	UseRuntimePolicy    bool // runtime_policy_config retry/timeout helpers without operations table
	ErrorMappingMatrix  []ErrorMatrixTemplate

	// TnxStatusVars lists model.ParseStatus variables to emit (only kinds used by this provider).
	TnxStatusVars []TnxStatusVar

	MacanLiveEnvPrefix          string
	MacanMethodTests            []MacanMethodTest
	MacanStatusDetailProbeConst string
	ConstructorDepMocks         []ConstructorDepMock
	HasMacanRedirectMethods     bool

	PaymentMethodMap        []PaymentMethodMapTemplate
	MacanPayMethodConsts    []MacanPayMethodConst
	MacanPaymentBrandConsts []MacanPaymentBrandConst
	MacanBrandMatchCases    []MacanBrandMatchCase

	// Paytech Gateway (secrets.go + status.go layout).
	PaytechSecretParts []PaytechSecretPart
	APIKeyField        string
	SigningKeyField    string
}

type PaytechSecretPart struct {
	Name       string
	Key        string
	LabelConst string
	GoField    string
}

type CheckStatusThrottleConfig struct {
	Enabled bool
	Period  string
}

type AuthConfigTemplate struct {
	Type           string
	Header         string
	SecretKeyField string
	ContentType    string
	Prefix         string
	Masked         bool

	// OAuth client_credentials minting (Type == "oauth_client_credentials").
	IsOAuthClientCredentials bool
	TokenURL                 string
	ClientIDKeyField         string
	ClientSecretKeyField     string
	GrantType                string
	Scope                    string
	TokenTTLBuffer           string
	TokenTTLBufferExpr       string
}

type CheckStatusConfigTemplate struct {
	Enabled             bool
	SinceCreatedPeriod  string
	ByTransactionType   bool
	PathSuffixForeignID bool
	PathFormatTxID      bool
}

type PayoutRuntimeTemplate struct {
	ForeignIDOnUnexpectedError bool
	UnexpectedErrorPending     bool
}

type InitPayoutPolicyTemplate struct {
	MapStatusFromResponse bool
	ForeignIDStrategy     string
	ClientUUIDField       string
}

type RequestSigningTemplate struct {
	Algorithm        string
	Format           string
	Header           string
	SecretKeyField   string
	UsernameHeader   string
	UsernameKeyField string
	Encoding         string
	TimestampHeader  string
	NonceHeader      string
	ConcatFields     []string
}

type ResponseEnvelopeTemplate struct {
	Enabled        bool
	WrapperField   string
	WrapperGoField string
	SuccessField   string
	SuccessGoField string
	ErrorField     string
	ErrorGoField   string
	SuccessMode    string
}

type KeysEndpointTemplate struct {
	Enabled        bool
	EndpointConst  string
	BaseURL        string
	CacheTTLExpr   string
	CacheEnabled   bool
	SecretKeyField string
}

type CardEncryptionTemplate struct {
	Enabled           bool
	Algorithm         string
	PEMSecretKeyField string
	PaymentDataType   string

	// AES-GCM symmetric encryption (Algorithm == "aes-256-gcm").
	IsAESGCM          bool
	KeySecretKeyField string
	NonceLength       int
}

type CallbackSignatureFieldTemplate struct {
	MapKey          string
	CallbackGoField string
	OmitIfEmpty     bool
	Format          string
	IsSecretKey     bool
}

type CallbackSignatureTemplate struct {
	Algorithm        string
	SecretKeyField   string
	UsernameKeyField string
	SignatureJSON    string
	Format           string
	CompareEqualFold bool
	SignatureField   string
	Optional         bool
	Header           string
	Fields           []CallbackSignatureFieldTemplate
}

type CrossInstanceStateConfig struct {
	Enabled bool
	Backend string
}

type StatusConfirmationConfig struct {
	Enabled          bool
	Strategy         string
	RetryNotReady    bool
	NotReadyPatterns []string
}

type ExtendedCallbackConfig struct {
	Enabled bool
	Fields  []string
}

type RuntimePolicyConfig struct {
	Timeouts struct {
		RequestTimeout      string
		CheckStatusTimeout  string
		CallbackWaitTimeout string
		RequestTimeoutExpr  string
	}
	Retries struct {
		MaxAttempts        int
		InitialBackoff     string
		MaxBackoff         string
		InitialBackoffExpr string
		MaxBackoffExpr     string
		RetryOnNotFound    bool
		RetryOn5xx         bool
		RetryOnRateLimit   bool
	}
	Limits struct {
		MaxCallbackBodyBytes int
		MaxPendingAge        string
		MaxPendingAgeExpr    string
	}
}

type Async3DSConfig struct {
	Enabled                   bool
	AutoChargeAfter           string
	FinishCallbackWaitTimeout string
	StalePendingGrace         string
}

type OperationTemplate struct {
	Kind      string
	Transport OperationTransportTemplate
}

type OperationTransportTemplate struct {
	Endpoint              string
	EndpointPath          string
	RequestType           string
	ResponseType          string
	ErrorResponseType     string
	ResponseLoggingMode   string
	StatusStrategy        string
	RetryMaxAttempts      int
	RetryInitialBackoff   string
	RetryMaxBackoff       string
	Timeout               string
	PendingCallbackAction string
	StatusField           string
	StatusDetailsField    string
	ErrorCodeField        string
}

type ErrorMatrixTemplate struct {
	Major       string
	Minor       string
	StatusTitle string
	StatusCode  string
	Message     string
}

// TnxStatusVar is one parsed transaction status variable in generated datatypes.go.
type TnxStatusVar struct {
	Kind      string // e.g. "success"
	TitleName string // e.g. "Success" → tnxStatusSuccess
}

// MacanMethodTest is one macanPaymentMethodName table test case.
type MacanMethodTest struct {
	Subtest   string
	BrandExpr string
	Currency  string
	Want      string
	WantErr   bool
}

// ConstructorDepMock maps a constructor dependency to providers.MockObjects field.
type ConstructorDepMock struct {
	Name      string
	MockField string
}

// PaymentMethodMapTemplate drives macanPaymentMethodName generation.
type PaymentMethodMapTemplate struct {
	Brand             string
	Match             string
	MatchKey          string // cardp2p | sbp | click | qrcode | literal
	APIName           string
	APINameConst      string
	KGSAPIConst       string // set when currency_overrides contains KGS
	CurrencyOverrides []CurrencyOverrideTemplate
}

type CurrencyOverrideTemplate struct {
	Currency     string
	APIName      string
	APINameConst string
}

// MacanPayMethodConst is one Macan API payment_method.name value.
type MacanPayMethodConst struct {
	Name  string
	Value string
}

// MacanPaymentBrandConst is a platform payment brand not covered by utils/types.
type MacanPaymentBrandConst struct {
	Name  string
	Value string
}

// MacanBrandMatchCase is one branch in macanBrandMatches.
type MacanBrandMatchCase struct {
	MatchConst string
	MatchValue string
	EqualExpr  string
}

// RequestLiteralConst is a generated package-level const for source "const" request fields.
type RequestLiteralConst struct {
	Name  string
	Value string
}

type ResolvedRequestDef struct {
	Name   string
	Fields []ResolvedField
	// Locals are the variables the field expressions read from, in declaration
	// order. The constructor stands on its own, so it declares them itself.
	Locals []RequestLocal
	// CtorName is the constructor's Go name. Request type names are lower camel,
	// so it cannot be spelled by concatenation in a template.
	CtorName string
	// ObjectTypes is every nested object in this request, depth-first, so a
	// template can declare types without knowing how deep the contract nests.
	ObjectTypes []ResolvedField
	// MaskLeaves is every value-carrying field with a selector from the
	// String() receiver, at any depth.
	MaskLeaves []MaskLeaf
	// UsesSecrets is true when any field expression reads secrets.
	UsesSecrets bool
}

type EndpointTemplate struct {
	ConstName     string
	Path          string
	Method        string
	ContentType   string
	PathSecretKey string
}

type StatusTemplate struct {
	ConstName   string
	Code        string
	StatusTitle string
	StatusCode  string
	Message     string
}

type ErrorTemplate struct {
	ConstName   string
	Code        string
	CodeIsInt   bool
	StatusTitle string
	StatusCode  string
}

type GroupedStatusDetail struct {
	ConstNames  []string
	StatusTitle string
	StatusCode  string
	Message     string
}

func (g GroupedStatusDetail) CaseLabel() string {
	return wrapSwitchCaseLabel(g.ConstNames, switchCaseWrapWidth)
}

// switchCaseWrapWidth accounts for the "\tcase " prefix emitted by datatypes templates.
const switchCaseWrapWidth = 100

func wrapSwitchCaseLabel(names []string, maxLineLen int) string {
	if len(names) == 0 {
		return ""
	}
	if len(names) == 1 {
		return names[0]
	}
	const casePrefixLen = len("\tcase ")
	if maxLineLen <= casePrefixLen+1 {
		maxLineLen = casePrefixLen + 40
	}
	budget := maxLineLen - casePrefixLen

	var lines []string
	var current strings.Builder
	for i, name := range names {
		chunk := name
		if i > 0 {
			chunk = ", " + name
		}
		if current.Len() > 0 && current.Len()+len(chunk) > budget {
			lines = append(lines, current.String())
			current.Reset()
			chunk = name
		}
		current.WriteString(chunk)
	}
	if current.Len() > 0 {
		lines = append(lines, current.String())
	}
	if len(lines) == 1 {
		return lines[0]
	}
	return lines[0] + ",\n\t\t" + strings.Join(lines[1:], ",\n\t\t")
}

func groupStatusTemplates(items []StatusTemplate) []GroupedStatusDetail {
	return groupStatusTemplatesByKey(items, true)
}

func groupStatusTemplatesByOutcome(items []StatusTemplate) []GroupedStatusDetail {
	return groupStatusTemplatesByKey(items, false)
}

func groupStatusTemplatesByKey(items []StatusTemplate, includeMessage bool) []GroupedStatusDetail {
	type key struct {
		title string
		code  string
		msg   string
	}
	var order []key
	groups := map[key][]string{}
	seen := map[key]map[string]struct{}{}
	for _, e := range items {
		k := key{e.StatusTitle, e.StatusCode, ""}
		if includeMessage {
			k.msg = e.Message
		}
		if _, ok := groups[k]; !ok {
			order = append(order, k)
			seen[k] = map[string]struct{}{}
		}
		if _, dup := seen[k][e.ConstName]; dup {
			continue
		}
		seen[k][e.ConstName] = struct{}{}
		groups[k] = append(groups[k], e.ConstName)
	}
	result := make([]GroupedStatusDetail, 0, len(order))
	for _, k := range order {
		result = append(result, GroupedStatusDetail{
			ConstNames:  groups[k],
			StatusTitle: k.title,
			StatusCode:  k.code,
			Message:     k.msg,
		})
	}
	return result
}

func (d *TemplateData) GroupedPayinStatuses() []GroupedStatusDetail {
	return groupStatusTemplates(d.PayinStatuses)
}

func (d *TemplateData) GroupedPayoutStatuses() []GroupedStatusDetail {
	return groupStatusTemplates(d.PayoutStatuses)
}

// GroupedPayinOutcomes collapses codes that map to the same internal status
// and status code. v2 mappers take the provider message as an argument, so
// contract-level Message is not part of the grouping key.
func (d *TemplateData) GroupedPayinOutcomes() []GroupedStatusDetail {
	return groupStatusTemplatesByOutcome(d.PayinStatuses)
}

func (d *TemplateData) GroupedPayoutOutcomes() []GroupedStatusDetail {
	return groupStatusTemplatesByOutcome(d.PayoutStatuses)
}

// ShareStatusMapper is true when payin and payout collapse to the same mapped
// outcomes, so the generated payout mapper can call the payin one.
func (d *TemplateData) ShareStatusMapper() bool {
	return statusMapsEquivalent(d.PayinStatuses, d.PayoutStatuses)
}

func statusMapsEquivalent(a, b []StatusTemplate) bool {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return false
	}
	type outcome struct{ title, code string }
	am := make(map[string]outcome, len(a))
	for _, s := range a {
		am[s.Code] = outcome{s.StatusTitle, s.StatusCode}
	}
	for _, s := range b {
		got, ok := am[s.Code]
		if !ok || got.title != s.StatusTitle || got.code != s.StatusCode {
			return false
		}
	}
	return true
}

func (d *TemplateData) PendingRawStatusCaseLabel() string {
	names := pendingStatusConstNames(d.PayinStatuses, d.PayoutStatuses)
	return wrapSwitchCaseLabel(names, switchCaseWrapWidth)
}

func pendingStatusConstNames(lists ...[]StatusTemplate) []string {
	var names []string
	seen := map[string]struct{}{}
	for _, items := range lists {
		for _, s := range items {
			if s.StatusTitle != "Pending" {
				continue
			}
			if _, ok := seen[s.ConstName]; ok {
				continue
			}
			seen[s.ConstName] = struct{}{}
			names = append(names, s.ConstName)
		}
	}
	return names
}

func (d *TemplateData) GroupedStatusDetails() []GroupedStatusDetail {
	type key struct{ title, code string }
	var order []key
	groups := map[key][]string{}
	for _, e := range d.StatusDetails {
		k := key{e.StatusTitle, e.StatusCode}
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], e.ConstName)
	}
	result := make([]GroupedStatusDetail, 0, len(order))
	for _, k := range order {
		result = append(result, GroupedStatusDetail{
			ConstNames:  groups[k],
			StatusTitle: k.title,
			StatusCode:  k.code,
		})
	}
	return result
}

func (d *TemplateData) GroupedErrorCodes() []GroupedStatusDetail {
	type key struct{ title, code string }
	var order []key
	groups := map[key][]string{}
	for _, e := range d.ErrorCodes {
		k := key{e.StatusTitle, e.StatusCode}
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], e.ConstName)
	}
	result := make([]GroupedStatusDetail, 0, len(order))
	for _, k := range order {
		result = append(result, GroupedStatusDetail{
			ConstNames:  groups[k],
			StatusTitle: k.title,
			StatusCode:  k.code,
		})
	}
	return result
}

type StatusTestTemplate struct {
	Name           string
	Code           string
	ExpectedStatus string
	ExpectedCode   string
}

type RequestTypeTemplate struct {
	Name           string
	Fields         []StructField
	HasRedacted    bool
	RedactedFields []string
}

// BuildTemplateData converts a ProviderSpec into template-ready data.
func BuildTemplateData(spec *ProviderSpec) (*TemplateData, error) {
	currencyNum := spec.Currency.ISONum
	statusType := "string"
	if spec.Callback != nil && spec.Callback.StatusType == "int" {
		statusType = "int"
	}

	data := &TemplateData{
		PackageName:               spec.PackageName,
		SID:                       spec.SID,
		Source:                    spec.Source,
		Label:                     spec.Label,
		MIDPrefix:                 spec.MIDPrefix,
		StructName:                spec.StructName,
		ConstructorName:           spec.ConstructorName,
		PaymentSource:             spec.PaymentSource,
		HasPayin:                  spec.HasPayin,
		HasPayout:                 spec.HasPayout,
		HasP2P:                    spec.HasP2P,
		HasCancel:                 spec.HasCancel,
		HasRefund:                 spec.HasRefund,
		HasSubscription:           spec.HasSubscription,
		SecretParts:               spec.Secrets.Parts,
		SecretPartsCount:          len(spec.Secrets.Parts),
		SecretSeparator:           spec.Secrets.Separator,
		SecretFormat:              spec.Secrets.Format,
		SecretTestValue:           buildSecretTestValue(spec),
		SigningAlgorithm:          spec.Signing.Algorithm,
		SigningFormat:             spec.Signing.Format,
		SigningSecretField:        pickSigningSecretField(spec),
		UseBasicAuth:              strings.EqualFold(spec.Signing.Algorithm, "basic") || spec.Signing.Format == "basic_auth",
		UseTransfertyH2H:          strings.EqualFold(spec.APICompat, "transferty_h2h"),
		UseMacanP2P:               strings.EqualFold(spec.APICompat, "macan_p2p"),
		UsePaytechGateway:         strings.EqualFold(spec.APICompat, "paytech_gateway"),
		UseFluxsgate:              strings.EqualFold(spec.APICompat, "fluxsgate"),
		UseRedirectCheckout:       strings.EqualFold(spec.APICompat, "redirect_checkout"),
		UseCentrobillHPP:          strings.EqualFold(spec.CheckoutCompat, "centrobill_hpp"),
		SecretUseLabels:           spec.Secrets.UseLabels,
		CheckStatusForeignIDEmpty: defaultCheckStatusForeignIDEmpty(spec.CheckStatusForeignIDEmpty),
		ResponseFormat:            defaultString(spec.ResponseFormat, "json"),
		CallbackFormat:            defaultString(spec.CallbackFormat, "json"),
		PubKeyField:               pickSecretField(spec, 0, "pubKey"),
		SecretKeyField:            pickSecretField(spec, 1, "secretKey"),
		HasBalanceFetcher:         spec.Interfaces.BalanceFetcher,
		HasMobileProcessor:        spec.Interfaces.MobileProcessor,
		HasOTPFormRedirector:      spec.Interfaces.OTPFormRedirector,
		HasTDSRedirector:          spec.Interfaces.TDSRedirector,
		HasPaymentMethodSelector:  spec.Interfaces.PaymentMethodSelector,
		HasCustomerRandomization:  spec.Interfaces.CustomerRandomization,
		CurrencyCode:              strings.ToUpper(spec.Currency.Code),
		SupportedMethods:          spec.SupportedMethods,
		SupportedCurrencies:       supportedCurrencies(spec),
		HasMultiCurrency:          len(supportedCurrencies(spec)) > 1,
		StatusInputType:           statusType,
		StatusCodeType:            statusType,
		ExtraImports:              buildExtraImports(spec),
		ResponseLoggingMode:       spec.ResponseLoggingMode,
	}

	for _, p := range spec.Secrets.Parts {
		if p.Optional && p.Key == "returnRecipientDetails" {
			data.HasOptionalReturnRecipientDetails = true
			break
		}
	}
	for _, p := range spec.Secrets.Parts {
		if p.Optional {
			data.HasOptionalSecretParts = true
			break
		}
	}
	data.SecretPartsSimple = len(spec.Secrets.Parts) > 0
	for _, p := range spec.Secrets.Parts {
		if p.JoinRemainder {
			data.SecretHasJoinRemainder = true
			data.SecretPartsSimple = false
			break
		}
	}
	for _, p := range spec.Secrets.Parts {
		if p.Optional || strings.TrimSpace(p.Type) == "bool" {
			data.SecretPartsSimple = false
			break
		}
		t := strings.TrimSpace(p.Transform)
		if t != "" && t != "none" {
			data.SecretPartsNeedTransform = true
			data.SecretPartsSimple = false
			break
		}
	}

	if _, ok := spec.Endpoints["p2p_card"]; ok {
		if _, ok2 := spec.Endpoints["p2p_apm"]; ok2 {
			data.HasP2PDualEndpoint = true
			data.P2PCardEndpointConst = endpointKeyToConst("p2p_card")
			data.P2PAPMEndpointConst = endpointKeyToConst("p2p_apm")
		}
	}
	if _, ok := spec.Endpoints["payout_card"]; ok {
		if _, ok2 := spec.Endpoints["payout_apm"]; ok2 {
			data.HasPayoutDualEndpoint = true
			data.PayoutCardEndpointConst = endpointKeyToConst("payout_card")
			data.PayoutAPMEndpointConst = endpointKeyToConst("payout_apm")
		}
	}

	if spec.CheckStatusConfig != nil {
		data.HasCheckStatusConfig = true
		data.CheckStatusPeriod = formatDurationGoExpr(spec.CheckStatusConfig.SinceCreatedPeriod)
		data.CheckStatusByTxType = spec.CheckStatusConfig.ByTransactionType
		data.CheckStatusConfig = &CheckStatusConfigTemplate{
			Enabled:             true,
			SinceCreatedPeriod:  data.CheckStatusPeriod,
			ByTransactionType:   spec.CheckStatusConfig.ByTransactionType,
			PathSuffixForeignID: spec.CheckStatusConfig.PathSuffixForeignID,
			PathFormatTxID:      spec.CheckStatusConfig.PathFormatTxID,
		}
		data.CheckStatusPathFormatTxID = spec.CheckStatusConfig.PathFormatTxID
	}
	if spec.PayoutRuntime != nil {
		data.PayoutRuntime = &PayoutRuntimeTemplate{
			ForeignIDOnUnexpectedError: spec.PayoutRuntime.ForeignIDOnUnexpectedError,
			UnexpectedErrorPending:     spec.PayoutRuntime.UnexpectedErrorPending,
		}
	}
	if spec.InitPayoutPolicy != nil {
		data.InitPayoutPolicy = &InitPayoutPolicyTemplate{
			MapStatusFromResponse: spec.InitPayoutPolicy.MapStatusFromResponse,
			ForeignIDStrategy:     defaultString(spec.InitPayoutPolicy.ForeignIDStrategy, "response"),
			ClientUUIDField:       defaultString(spec.InitPayoutPolicy.ClientUUIDField, "payoutId"),
		}
	}
	if spec.RequestSigning != nil {
		encoding := defaultString(spec.RequestSigning.Encoding, "hex")
		if spec.RequestSigning.Format == "hmac_timestamp_nonce" && strings.TrimSpace(spec.RequestSigning.Encoding) == "" {
			encoding = "base64"
		}
		data.RequestSigning = &RequestSigningTemplate{
			Algorithm:        spec.RequestSigning.Algorithm,
			Format:           spec.RequestSigning.Format,
			Header:           spec.RequestSigning.Header,
			SecretKeyField:   exportGoIdent(spec.RequestSigning.SecretKey),
			UsernameHeader:   spec.RequestSigning.UsernameHeader,
			UsernameKeyField: exportGoIdent(spec.RequestSigning.UsernameKey),
			Encoding:         encoding,
			TimestampHeader:  spec.RequestSigning.TimestampHeader,
			NonceHeader:      spec.RequestSigning.NonceHeader,
			ConcatFields:     append([]string(nil), spec.RequestSigning.ConcatFields...),
		}
	}
	if spec.ResponseEnvelope != nil && spec.ResponseEnvelope.Enabled {
		wrapper := defaultString(spec.ResponseEnvelope.WrapperField, "message")
		success := defaultString(spec.ResponseEnvelope.SuccessField, "status")
		errField := defaultString(spec.ResponseEnvelope.ErrorField, "error")
		data.ResponseEnvelope = &ResponseEnvelopeTemplate{
			Enabled:        true,
			WrapperField:   wrapper,
			WrapperGoField: jsonKeyToGoField(wrapper),
			SuccessField:   success,
			SuccessGoField: jsonKeyToGoField(success),
			ErrorField:     errField,
			ErrorGoField:   jsonKeyToGoField(errField),
			SuccessMode:    defaultString(spec.ResponseEnvelope.SuccessMode, "field_bool"),
		}
	}
	if spec.KeysEndpoint != nil && spec.KeysEndpoint.Enabled {
		epKey := defaultString(spec.KeysEndpoint.EndpointKey, "keys")
		cacheEnabled := true
		if spec.KeysEndpoint.CacheEnabled != nil {
			cacheEnabled = *spec.KeysEndpoint.CacheEnabled
		}
		data.KeysEndpoint = &KeysEndpointTemplate{
			Enabled:        true,
			EndpointConst:  endpointKeyToConst(epKey),
			BaseURL:        strings.TrimRight(strings.TrimSpace(spec.KeysEndpoint.BaseURL), "/"),
			CacheTTLExpr:   formatDurationGoExpr(defaultString(spec.KeysEndpoint.CacheTTL, "12h")),
			CacheEnabled:   cacheEnabled,
			SecretKeyField: exportGoIdent(defaultString(spec.KeysEndpoint.SecretKey, "keysBaseURL")),
		}
	}
	if spec.CardEncryption != nil && spec.CardEncryption.Enabled {
		algorithm := defaultString(spec.CardEncryption.Algorithm, "rsa_oaep_sha256")
		ce := &CardEncryptionTemplate{
			Enabled:           true,
			Algorithm:         algorithm,
			PEMSecretKeyField: exportGoIdent(defaultString(spec.CardEncryption.PEMSecretKey, "callbackSignKey")),
			PaymentDataType:   defaultString(spec.CardEncryption.PaymentDataType, "card"),
		}
		if strings.EqualFold(algorithm, "aes-256-gcm") {
			ce.IsAESGCM = true
			ce.KeySecretKeyField = exportGoIdent(defaultString(spec.CardEncryption.KeySecretKey, "encryptionKey"))
			ce.NonceLength = spec.CardEncryption.NonceLength
			if ce.NonceLength == 0 {
				ce.NonceLength = 12 // GCM standard IV length
			}
		}
		data.CardEncryption = ce
	}
	data.PayoutStatusValueField = strings.TrimSpace(spec.PayoutStatusValueField)
	if spec.OTPConfig != nil {
		data.HasOTPConfig = true
		data.OTPHandlesExternally = spec.OTPConfig.HandlesExternally
	}
	if spec.Callback != nil {
		data.CallbackTxIDField = spec.Callback.TxIDField
		data.CallbackForeignIDField = spec.Callback.ForeignIDField
		data.CallbackStatusField = spec.Callback.StatusField
		data.CallbackErrorCodeField = spec.Callback.ErrorCodeField
		data.CallbackMessageField = spec.Callback.MessageField
		data.CallbackReturnCodeField = spec.Callback.ReturnCodeField
		data.CallbackReturnQueryTxIDParam = spec.Callback.ReturnQueryTxIDParam
		data.CallbackReturnQueryStatusValue = spec.Callback.ReturnQueryStatusValue
		data.CallbackReturnQueryInfoCallback = spec.Callback.ReturnQueryInfoCallback
		data.CallbackFields = normalizeStructFields(spec.Callback.Fields)
		for _, f := range data.CallbackFields {
			if strings.TrimSpace(f.NestedPath) != "" {
				data.HasCallbackNestedPaths = true
				break
			}
		}
	}

	data.HTTPAuth = buildHTTPAuth(spec)
	data.CallbackSignature = buildCallbackSignature(spec)
	if data.CallbackSignature != nil {
		for _, f := range data.CallbackSignature.Fields {
			if f.Format == "float_trailing_zero" {
				data.HasCallbackSigFloatFormat = true
				break
			}
		}
	}

	// Advanced configs (optional).
	if spec.CheckStatusThrottleConfig != nil {
		data.CheckStatusThrottleConfig = &CheckStatusThrottleConfig{
			Enabled: spec.CheckStatusThrottleConfig.Enabled,
			Period:  spec.CheckStatusThrottleConfig.Period,
		}
	}
	if spec.CrossInstanceStateConfig != nil {
		data.CrossInstanceStateConfig = &CrossInstanceStateConfig{
			Enabled: spec.CrossInstanceStateConfig.Enabled,
			Backend: spec.CrossInstanceStateConfig.Backend,
		}
	}
	if spec.StatusConfirmationConfig != nil {
		data.StatusConfirmationConfig = &StatusConfirmationConfig{
			Enabled:          spec.StatusConfirmationConfig.Enabled,
			Strategy:         spec.StatusConfirmationConfig.Strategy,
			RetryNotReady:    spec.StatusConfirmationConfig.RetryNotReady,
			NotReadyPatterns: spec.StatusConfirmationConfig.NotReadyPatterns,
		}
	}
	if spec.ExtendedCallbackConfig != nil {
		data.ExtendedCallbackConfig = &ExtendedCallbackConfig{
			Enabled: spec.ExtendedCallbackConfig.Enabled,
			Fields:  append([]string{}, spec.ExtendedCallbackConfig.Fields...),
		}
	}
	if spec.RuntimePolicyConfig != nil {
		rp := &RuntimePolicyConfig{}
		rp.Timeouts.RequestTimeout = spec.RuntimePolicyConfig.Timeouts.RequestTimeout
		rp.Timeouts.CheckStatusTimeout = spec.RuntimePolicyConfig.Timeouts.CheckStatusTimeout
		rp.Timeouts.CallbackWaitTimeout = spec.RuntimePolicyConfig.Timeouts.CallbackWaitTimeout
		rp.Retries.MaxAttempts = spec.RuntimePolicyConfig.Retries.MaxAttempts
		rp.Retries.InitialBackoff = spec.RuntimePolicyConfig.Retries.InitialBackoff
		rp.Retries.MaxBackoff = spec.RuntimePolicyConfig.Retries.MaxBackoff
		rp.Retries.RetryOnNotFound = spec.RuntimePolicyConfig.Retries.RetryOnNotFound
		rp.Retries.RetryOn5xx = spec.RuntimePolicyConfig.Retries.RetryOn5xx
		rp.Retries.RetryOnRateLimit = spec.RuntimePolicyConfig.Retries.RetryOnRateLimit
		rp.Retries.InitialBackoffExpr = durationGoExpr(rp.Retries.InitialBackoff)
		rp.Retries.MaxBackoffExpr = durationGoExpr(rp.Retries.MaxBackoff)
		rp.Timeouts.RequestTimeoutExpr = durationGoExpr(rp.Timeouts.RequestTimeout)
		rp.Limits.MaxCallbackBodyBytes = spec.RuntimePolicyConfig.Limits.MaxCallbackBodyBytes
		rp.Limits.MaxPendingAge = strings.TrimSpace(spec.RuntimePolicyConfig.Limits.MaxPendingAge)
		if rp.Limits.MaxPendingAge != "" {
			rp.Limits.MaxPendingAgeExpr = durationGoExpr(rp.Limits.MaxPendingAge)
			if rp.Limits.MaxPendingAgeExpr == "0" {
				rp.Limits.MaxPendingAge = ""
				rp.Limits.MaxPendingAgeExpr = ""
			}
		}
		data.RuntimePolicyConfig = rp
	}
	if spec.Async3DSConfig != nil {
		data.Async3DSConfig = &Async3DSConfig{
			Enabled:                   spec.Async3DSConfig.Enabled,
			AutoChargeAfter:           spec.Async3DSConfig.AutoChargeAfter,
			FinishCallbackWaitTimeout: spec.Async3DSConfig.FinishCallbackWaitTimeout,
			StalePendingGrace:         spec.Async3DSConfig.StalePendingGrace,
		}
	}

	if len(spec.Operations) > 0 {
		data.Operations = make([]OperationTemplate, 0, len(spec.Operations))
		for _, op := range spec.Operations {
			data.Operations = append(data.Operations, OperationTemplate{
				Kind: op.Kind,
				Transport: OperationTransportTemplate{
					Endpoint:              op.Transport.Endpoint,
					EndpointPath:          op.Transport.EndpointPath,
					RequestType:           op.Transport.RequestType,
					ResponseType:          op.Transport.ResponseType,
					ErrorResponseType:     op.Transport.ErrorResponseType,
					ResponseLoggingMode:   op.Transport.ResponseLoggingMode,
					StatusStrategy:        op.Transport.StatusStrategy,
					RetryMaxAttempts:      op.Transport.RetryMaxAttempts,
					RetryInitialBackoff:   op.Transport.RetryInitialBackoff,
					RetryMaxBackoff:       op.Transport.RetryMaxBackoff,
					Timeout:               op.Transport.Timeout,
					PendingCallbackAction: op.Transport.PendingCallbackAction,
					StatusField:           op.Transport.StatusField,
					StatusDetailsField:    op.Transport.StatusDetailsField,
					ErrorCodeField:        op.Transport.ErrorCodeField,
				},
			})
		}
	}
	data.UseOperationRuntime = len(data.Operations) > 0 && !data.UseMacanP2P
	data.UseRuntimePolicy = data.RuntimePolicyConfig != nil

	if len(spec.ErrorMappingMatrix) > 0 {
		data.ErrorMappingMatrix = make([]ErrorMatrixTemplate, 0, len(spec.ErrorMappingMatrix))
		for _, e := range spec.ErrorMappingMatrix {
			major := strings.TrimSpace(fmt.Sprint(e.Major))
			minor := strings.TrimSpace(fmt.Sprint(e.Minor))
			if minor == "<nil>" {
				minor = ""
			}
			if major == "<nil>" {
				major = ""
			}
			data.ErrorMappingMatrix = append(data.ErrorMappingMatrix, ErrorMatrixTemplate{
				Major:       major,
				Minor:       minor,
				StatusTitle: titleCase(e.Status),
				StatusCode:  e.StatusCode,
				Message:     e.Message,
			})
		}
	}
	data.ResponseTypes = normalizeResponseTypes(spec.ResponseTypes)

	data.ConstructorParams, data.ConstructorTestArgs = buildConstructorArgs(spec)
	data.Endpoints = buildEndpoints(spec.Endpoints)
	data.PayinEndpointConst = endpointConst(spec.Endpoints, "payin", "endpointPayin")
	data.PayoutEndpointConst = endpointConst(spec.Endpoints, "payout", "endpointPayout")
	data.RefundEndpointConst = endpointConst(spec.Endpoints, "refund", "endpointRefund")
	data.PayinStatusEndpointConst = endpointConst(spec.Endpoints, "payin_status", "endpointPayinStatus")
	data.PayoutStatusEndpointConst = endpointConst(spec.Endpoints, "payout_status", "endpointPayoutStatus")
	if payinEP, ok := spec.Endpoints["payin"]; ok {
		if statusEP, ok2 := spec.Endpoints["payin_status"]; ok2 && payinEP.Path == statusEP.Path {
			data.PayinStatusEndpointConst = data.PayinEndpointConst
		}
	}
	if payoutEP, ok := spec.Endpoints["payout"]; ok {
		if statusEP, ok2 := spec.Endpoints["payout_status"]; ok2 && payoutEP.Path == statusEP.Path {
			data.PayoutStatusSameAsPayout = true
			data.PayoutStatusEndpointConst = data.PayoutEndpointConst
		}
	}
	if !hasEndpoint(spec.Endpoints, "payout_status") && hasEndpoint(spec.Endpoints, "check") {
		data.PayoutStatusEndpointConst = endpointConst(spec.Endpoints, "check", "endpointCheck")
	}
	data.PayinStatusMethod = endpointMethod(spec.Endpoints, "payin_status", "GET")
	data.PayoutStatusMethod = endpointMethod(spec.Endpoints, "payout_status", "GET")
	if data.PayoutStatusMethod == "GET" && hasEndpoint(spec.Endpoints, "check") {
		data.PayoutStatusMethod = endpointMethod(spec.Endpoints, "check", "GET")
	}
	data.PayoutMethod = endpointMethod(spec.Endpoints, "payout", "POST")
	if ep, ok := spec.Endpoints["payout"]; ok {
		data.PayoutPathTxID = strings.Contains(ep.Path, "%s")
	}
	data.HasStatusEndpoints = hasEndpoint(spec.Endpoints, "payin_status") || hasEndpoint(spec.Endpoints, "payout_status") || hasEndpoint(spec.Endpoints, "check")

	if spec.HasP2P && len(spec.SupportedMethods) > 0 {
		data.P2PMethodCheck = strings.ToUpper(spec.SupportedMethods[0])
	}

	var err error
	if spec.PayinRequest != nil {
		data.PayinRequest, err = resolveRequestDef(spec.PayinRequest, spec.Methods, spec.PaymentSource, currencyNum)
		if err != nil {
			return nil, err
		}
		data.PayinRequestType = spec.PayinRequest.Name
	}
	if spec.PayoutRequest != nil {
		data.PayoutRequest, err = resolveRequestDef(spec.PayoutRequest, spec.Methods, spec.PaymentSource, currencyNum)
		if err != nil {
			return nil, err
		}
		data.PayoutRequestType = spec.PayoutRequest.Name
	}
	if spec.PayinStatusRequest != nil {
		data.PayinStatusRequest, err = resolveRequestDef(spec.PayinStatusRequest, spec.Methods, spec.PaymentSource, currencyNum)
		if err != nil {
			return nil, err
		}
	}
	if spec.PayoutStatusRequest != nil {
		data.PayoutStatusRequest, err = resolveRequestDef(spec.PayoutStatusRequest, spec.Methods, spec.PaymentSource, currencyNum)
		if err != nil {
			return nil, err
		}
	}
	if spec.RefundRequest != nil {
		data.RefundRequest, err = resolveRequestDef(spec.RefundRequest, spec.Methods, spec.PaymentSource, currencyNum)
		if err != nil {
			return nil, err
		}
		data.RefundRequestType = spec.RefundRequest.Name
	}
	if spec.P2PRequest != nil {
		data.P2PRequest, err = resolveRequestDef(spec.P2PRequest, spec.Methods, "apm", currencyNum)
		if err != nil {
			return nil, err
		}
	} else if data.HasP2P && spec.PayinRequest != nil {
		data.P2PRequest = data.PayinRequest
	}

	for _, m := range spec.Methods {
		goConst, err := paymentMethodGoConst(m.Sid)
		if err != nil {
			return nil, err
		}
		data.Methods = append(data.Methods, ResolvedMethod{
			Sid:           m.Sid,
			ProviderValue: m.ProviderValue,
			GoConst:       goConst,
		})
	}

	data.AllRequestFields = collectRequestLeaves(
		data.PayinRequest, data.PayoutRequest, data.PayinStatusRequest,
		data.PayoutStatusRequest, data.RefundRequest, data.P2PRequest,
	)

	data.RequestLiteralConsts, err = BuildRequestLiteralConsts(spec)
	if err != nil {
		return nil, err
	}

	data.PayinResponsePayloadType, data.PayinForeignIDField = inferResponse(spec.ResponseTypes, "payin", "payinProfile", "PaymentID", data.ResponseEnvelope)
	data.PayoutResponsePayloadType, data.PayoutForeignIDField = inferResponse(spec.ResponseTypes, "payout", "payoutMessage", "ReferenceID", data.ResponseEnvelope)
	if field := strings.TrimSpace(spec.PayoutForeignIDField); field != "" {
		data.PayoutForeignIDField = field
	}
	data.RefundResponseType, data.RefundForeignIDField = inferResponse(spec.ResponseTypes, "refund", "refundResponse", "PaymentID", nil)

	if data.ResponseEnvelope != nil && data.ResponseEnvelope.Enabled {
		data.PayinResponseType = envelopeWrapperTypeName(data.PayinResponsePayloadType)
		data.PayoutResponseType = envelopeWrapperTypeName(data.PayoutResponsePayloadType)
	} else {
		data.PayinResponseType = data.PayinResponsePayloadType
		data.PayoutResponseType = data.PayoutResponsePayloadType
	}

	data.PayinStatusResponseType = data.PayinResponseType
	data.PayoutStatusResponseType = data.PayoutResponseType
	data.PayinStatusField = inferStatusField(spec.ResponseTypes, "payin", "Status")
	data.PayoutStatusField = inferStatusField(spec.ResponseTypes, "payout", "Status")
	data.PayinRedirectURLField = inferRedirectURLField(spec.ResponseTypes, data.PayinResponsePayloadType)
	data.PayinStatusField = nestUnderEnvelope(data.ResponseEnvelope, data.PayinStatusField)
	data.PayoutStatusField = nestUnderEnvelope(data.ResponseEnvelope, data.PayoutStatusField)
	data.PayinForeignIDField = nestUnderEnvelope(data.ResponseEnvelope, data.PayinForeignIDField)
	data.PayoutForeignIDField = nestUnderEnvelope(data.ResponseEnvelope, data.PayoutForeignIDField)
	data.PayinRedirectURLField = nestUnderEnvelope(data.ResponseEnvelope, data.PayinRedirectURLField)
	data.ResponseTypes = ensureResponseEnvelopeType(data.ResponseTypes, data.PayinResponseType, data.PayinResponsePayloadType, data.ResponseEnvelope)
	data.ResponseTypes = ensureResponseEnvelopeType(data.ResponseTypes, data.PayoutResponseType, data.PayoutResponsePayloadType, data.ResponseEnvelope)

	data.PayinStatuses, data.PayoutStatuses, data.PayoutStatusesExtra = buildStatuses(spec, statusType)
	data.ErrorCodes = buildCodeMappings(spec.ErrorCodes, "errCode")
	data.ErrorCodesNumeric = len(data.ErrorCodes) > 0
	for _, e := range data.ErrorCodes {
		if !e.CodeIsInt {
			data.ErrorCodesNumeric = false
			break
		}
	}
	data.StatusDetails = buildCodeMappings(spec.StatusDetails, "statusDetail")
	data.PayinStatusTests = statusTests(data.PayinStatuses, statusType)
	data.PayoutStatusTests = statusTests(data.PayoutStatuses, statusType)
	data.AllRequestTypes = buildAllRequestTypes(data)
	data.HasAnyRedacted = hasAnyRedacted(data.AllRequestTypes)
	data.TnxStatusVars = buildTnxStatusVars(spec, data)
	if data.UseMacanP2P {
		data.MacanLiveEnvPrefix = strings.ToUpper(spec.SID)
		data.MacanMethodTests = buildMacanMethodTests(spec.SupportedMethods)
		data.HasMacanRedirectMethods = hasMacanRedirectMethods(spec.SupportedMethods)
		data.PaymentMethodMap, data.MacanPayMethodConsts = buildPaymentMethodMap(spec.PaymentMethodMap)
		data.MacanPaymentBrandConsts = buildMacanPaymentBrandConsts(spec.PaymentMethodMap)
		data.MacanBrandMatchCases = buildMacanBrandMatchCases(data.PaymentMethodMap)
		if len(data.PayinStatuses) > 0 {
			data.MacanStatusDetailProbeConst = data.PayinStatuses[0].ConstName
		} else if len(data.PayoutStatuses) > 0 {
			data.MacanStatusDetailProbeConst = data.PayoutStatuses[0].ConstName
		}
	}
	data.ConstructorDepMocks = buildConstructorDepMocks(spec.ConstructorDeps)
	if data.UsePaytechGateway {
		data.PaytechSecretParts = buildPaytechSecretParts(spec.Secrets.Parts)
		for _, p := range data.PaytechSecretParts {
			switch p.Key {
			case "apiKey":
				data.APIKeyField = p.GoField
			case "signingKey":
				data.SigningKeyField = p.GoField
			}
		}
	}

	data.NeedsOwnerInfoHelper = requestDefsNeedLocal(data, "info") ||
		(spec.PaymentSource != "card" && spec.PaymentSource != "apm" && hasRequestSource(data, "external_customer_id"))
	return data, nil
}

func buildPaytechSecretParts(parts []SecretPart) []PaytechSecretPart {
	out := make([]PaytechSecretPart, 0, len(parts))
	for _, p := range parts {
		out = append(out, PaytechSecretPart{
			Name:       p.Name,
			Key:        p.Key,
			LabelConst: p.Key + "Label",
			GoField:    paytechSecretGoField(p.Key),
		})
	}
	return out
}

func paytechSecretGoField(key string) string {
	switch key {
	case "apiKey":
		return "APIKey"
	case "signingKey":
		return "SigningKey"
	default:
		if key == "" {
			return ""
		}
		r := []rune(key)
		r[0] = unicode.ToUpper(r[0])
		return string(r)
	}
}

func buildExtraImports(spec *ProviderSpec) []string {
	out := append([]string(nil), spec.ExtraImports...)
	if len(spec.Methods) > 0 {
		out = appendImportIfMissing(out, "gitlab.q-tech.host/transferty/backend/utils/types")
	}
	if strings.EqualFold(spec.APICompat, "macan_p2p") {
		out = appendImportIfMissing(out, "gitlab.q-tech.host/transferty/backend/utils/helpers")
		if spec.Interfaces.BalanceFetcher {
			out = appendImportIfMissing(out, "gitlab.q-tech.host/transferty/backend/tnx_processor/payment_providers/common")
		}
	}
	if spec.CardEncryption != nil && spec.CardEncryption.Enabled {
		out = appendImportIfMissing(out, "gitlab.q-tech.host/transferty/backend/utils/helpers")
	}
	sort.Strings(out)
	return out
}

func requestDefsNeedLocal(data *TemplateData, name string) bool {
	return defNeedsLocal(data.PayinRequest, name) ||
		defNeedsLocal(data.PayoutRequest, name) ||
		defNeedsLocal(data.PayinStatusRequest, name) ||
		defNeedsLocal(data.PayoutStatusRequest, name) ||
		defNeedsLocal(data.RefundRequest, name) ||
		defNeedsLocal(data.P2PRequest, name)
}

func defNeedsLocal(def *ResolvedRequestDef, name string) bool {
	if def == nil {
		return false
	}
	for _, l := range def.Locals {
		if l.Name == name {
			return true
		}
	}
	for _, obj := range def.ObjectTypes {
		for _, m := range obj.Methods {
			for _, l := range m.Locals {
				if l.Name == name {
					return true
				}
			}
		}
	}
	return false
}

func hasRequestSource(data *TemplateData, source string) bool {
	if data == nil {
		return false
	}
	for _, f := range data.AllRequestFields {
		if f.Source == source {
			return true
		}
	}
	return false
}

func appendImportIfMissing(imports []string, path string) []string {
	for _, imp := range imports {
		if imp == path {
			return imports
		}
	}
	return append(imports, path)
}

func resolveRequestDef(def *RequestDef, methods []Method, paymentSource string, currencyNum int) (*ResolvedRequestDef, error) {
	fields, err := ResolveRequestFieldsIn(RequestTypeScope(def.Name), def.Fields, paymentSource, currencyNum)
	if err != nil {
		return nil, err
	}
	refineOwnerSource(fields, def.Fields, paymentSource)
	if err := FillPerMethodObjects(fields, methods, paymentSource, currencyNum); err != nil {
		return nil, fmt.Errorf("%s: %w", def.Name, err)
	}
	locals := RequestLocals(fields)
	if err := mixedConstructorLocals(locals); err != nil {
		return nil, fmt.Errorf("%s: %w", def.Name, err)
	}
	return &ResolvedRequestDef{
		Name:        def.Name,
		Fields:      fields,
		Locals:      locals,
		CtorName:    constructorName(def.Name),
		ObjectTypes: collectObjectTypes(fields),
		MaskLeaves:  collectMaskLeaves(fields, "r"),
		UsesSecrets: fieldsUseSecret(fields),
	}, nil
}

// refineOwnerSource decides which of the two owner maps an owner_info field
// reads from. Resolution keeps the declared order, so the two slices stay
// aligned, including inside nested objects.
func refineOwnerSource(resolved []ResolvedField, declared []RequestField, paymentSource string) {
	for i := range resolved {
		f := &declared[i]
		if len(f.Fields) > 0 {
			refineOwnerSource(resolved[i].Nested, f.Fields, paymentSource)
			continue
		}
		if f.Source != "owner_info" {
			continue
		}
		switch from := strings.TrimSpace(f.OwnerFrom); {
		case from == "card":
			resolved[i].IsCard, resolved[i].IsAPM = true, false
		case from == "apm":
			resolved[i].IsCard, resolved[i].IsAPM = false, true
		case paymentSource == "card":
			resolved[i].IsCard, resolved[i].IsAPM = true, false
		default:
			resolved[i].IsCard, resolved[i].IsAPM = false, true
		}
	}
}

// collectRequestLeaves flattens the given definitions into their value-carrying
// fields. The same definition may be reached twice (a p2p body defaulting to the
// payin one), so each is taken once.
func collectRequestLeaves(defs ...*ResolvedRequestDef) []ResolvedField {
	var out []ResolvedField
	seen := map[*ResolvedRequestDef]bool{}
	var walk func(fields []ResolvedField)
	walk = func(fields []ResolvedField) {
		for _, f := range fields {
			if f.IsObject() {
				walk(f.Nested)
				continue
			}
			out = append(out, f)
		}
	}
	for _, def := range defs {
		if def == nil || seen[def] {
			continue
		}
		seen[def] = true
		walk(def.Fields)
	}
	return out
}

func buildAllRequestTypes(data *TemplateData) []RequestTypeTemplate {
	var out []RequestTypeTemplate
	add := func(def *ResolvedRequestDef) {
		if def == nil {
			return
		}
		var fields []StructField
		var redacted []string
		for _, f := range def.Fields {
			fields = append(fields, structFieldFromResolved(f))
			if f.Redacted {
				redacted = append(redacted, f.GoName)
			}
		}
		out = append(out, RequestTypeTemplate{
			Name:           def.Name,
			Fields:         fields,
			HasRedacted:    len(redacted) > 0,
			RedactedFields: redacted,
		})
	}
	add(data.PayinRequest)
	if data.PayoutRequest != nil && (data.PayinRequest == nil || data.PayoutRequest.Name != data.PayinRequest.Name) {
		add(data.PayoutRequest)
	}
	if data.RefundRequest != nil && (data.PayinRequest == nil || data.RefundRequest.Name != data.PayinRequest.Name) && (data.PayoutRequest == nil || data.RefundRequest.Name != data.PayoutRequest.Name) {
		add(data.RefundRequest)
	}
	if data.P2PRequest != nil {
		seen := false
		for _, rt := range out {
			if rt.Name == data.P2PRequest.Name {
				seen = true
				break
			}
		}
		if !seen {
			add(data.P2PRequest)
		}
	}
	if data.PayinStatusRequest != nil {
		seen := false
		for _, rt := range out {
			if rt.Name == data.PayinStatusRequest.Name {
				seen = true
				break
			}
		}
		if !seen {
			add(data.PayinStatusRequest)
		}
	}
	if data.PayoutStatusRequest != nil {
		seen := false
		for _, rt := range out {
			if rt.Name == data.PayoutStatusRequest.Name {
				seen = true
				break
			}
		}
		if !seen {
			add(data.PayoutStatusRequest)
		}
	}
	return out
}

func hasAnyRedacted(types []RequestTypeTemplate) bool {
	for _, t := range types {
		if t.HasRedacted {
			return true
		}
	}
	return false
}

func buildConstructorArgs(spec *ProviderSpec) (params, testArgs string) {
	var paramParts, testParts []string
	for _, dep := range spec.ConstructorDeps {
		paramParts = append(paramParts, dep.Name+" "+dep.Type)
		testParts = append(testParts, "nil")
	}
	return strings.Join(paramParts, ", "), strings.Join(testParts, ", ")
}

func buildEndpoints(endpoints map[string]Endpoint) []EndpointTemplate {
	if len(endpoints) == 0 {
		return nil
	}
	keys := make([]string, 0, len(endpoints))
	for k := range endpoints {
		keys = append(keys, k)
	}
	// stable order
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	out := make([]EndpointTemplate, 0, len(keys))
	seenPaths := map[string]string{}
	for _, k := range keys {
		ep := endpoints[k]
		if prev, ok := seenPaths[ep.Path]; ok && ((k == "payout_status" && prev == "payout") || (k == "payin_status" && prev == "payin")) {
			continue
		}
		seenPaths[ep.Path] = k
		out = append(out, EndpointTemplate{
			ConstName:     endpointKeyToConst(k),
			Path:          ep.Path,
			Method:        ep.Method,
			ContentType:   ep.ContentType,
			PathSecretKey: ep.PathSecretKey,
		})
	}
	return out
}

func endpointKeyToConst(key string) string {
	parts := strings.Split(key, "_")
	var b strings.Builder
	b.WriteString("endpoint")
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	return b.String()
}

func hasEndpoint(endpoints map[string]Endpoint, key string) bool {
	_, ok := endpoints[key]
	return ok
}

func endpointConst(endpoints map[string]Endpoint, key, fallback string) string {
	if _, ok := endpoints[key]; ok {
		return endpointKeyToConst(key)
	}
	return fallback
}

func endpointMethod(endpoints map[string]Endpoint, key, fallback string) string {
	if ep, ok := endpoints[key]; ok && strings.TrimSpace(ep.Method) != "" {
		return strings.ToUpper(strings.TrimSpace(ep.Method))
	}
	return fallback
}

func supportedCurrencies(spec *ProviderSpec) []string {
	if len(spec.SupportedCurrencies) > 0 {
		out := make([]string, len(spec.SupportedCurrencies))
		for i, c := range spec.SupportedCurrencies {
			out[i] = strings.ToUpper(strings.TrimSpace(c))
		}
		return out
	}
	if code := strings.TrimSpace(spec.Currency.Code); code != "" {
		return []string{strings.ToUpper(code)}
	}
	return nil
}

func buildSecretTestValue(spec *ProviderSpec) string {
	parts := make([]string, len(spec.Secrets.Parts))
	for i := range spec.Secrets.Parts {
		parts[i] = "x"
	}
	return strings.Join(parts, spec.Secrets.Separator)
}

func pickSecretField(spec *ProviderSpec, idx int, fallback string) string {
	if idx < len(spec.Secrets.Parts) {
		return exportGoIdent(spec.Secrets.Parts[idx].Key)
	}
	return fallback
}

func pickSigningSecretField(spec *ProviderSpec) string {
	for _, part := range spec.Secrets.Parts {
		k := strings.ToLower(part.Key)
		if strings.Contains(k, "sign") || strings.Contains(k, "secret") {
			return exportGoIdent(part.Key)
		}
	}
	if len(spec.Secrets.Parts) > 1 {
		return exportGoIdent(spec.Secrets.Parts[1].Key)
	}
	if len(spec.Secrets.Parts) > 0 {
		return exportGoIdent(spec.Secrets.Parts[0].Key)
	}
	return "signingKey"
}

func buildHTTPAuth(spec *ProviderSpec) *AuthConfigTemplate {
	if spec.Auth == nil {
		return nil
	}
	a := &AuthConfigTemplate{
		Type:           strings.TrimSpace(spec.Auth.Type),
		Header:         strings.TrimSpace(spec.Auth.Header),
		SecretKeyField: exportGoIdent(spec.Auth.SecretKey),
		ContentType:    strings.TrimSpace(spec.Auth.ContentType),
		Prefix:         spec.Auth.Prefix,
		Masked:         spec.Auth.Masked,
	}
	if a.Type == "" {
		a.Type = "header_token"
	}
	if a.ContentType == "" {
		a.ContentType = "application/json"
	}
	if strings.EqualFold(a.Type, "oauth_client_credentials") {
		a.IsOAuthClientCredentials = true
		a.TokenURL = strings.TrimSpace(spec.Auth.TokenURL)
		a.ClientIDKeyField = exportGoIdent(spec.Auth.ClientIDKey)
		a.ClientSecretKeyField = exportGoIdent(spec.Auth.ClientSecretKey)
		a.GrantType = strings.TrimSpace(spec.Auth.GrantType)
		if a.GrantType == "" {
			a.GrantType = "client_credentials"
		}
		a.Scope = strings.TrimSpace(spec.Auth.Scope)
		a.TokenTTLBuffer = strings.TrimSpace(spec.Auth.TokenTTLBuffer)
		if a.TokenTTLBuffer == "" {
			a.TokenTTLBuffer = "60s"
		}
		a.TokenTTLBufferExpr = formatDurationGoExpr(a.TokenTTLBuffer)
	}
	return a
}

func buildCallbackSignature(spec *ProviderSpec) *CallbackSignatureTemplate {
	if spec.CallbackSignature == nil {
		return nil
	}
	cs := spec.CallbackSignature
	out := &CallbackSignatureTemplate{
		Algorithm:        strings.TrimSpace(cs.Algorithm),
		SecretKeyField:   exportGoIdent(cs.SecretKey),
		UsernameKeyField: exportGoIdent(cs.UsernameKey),
		Format:           strings.TrimSpace(cs.Format),
		CompareEqualFold: cs.Compare == "" || cs.Compare == "equal_fold",
		Optional:         cs.Optional,
		Header:           strings.TrimSpace(cs.Header),
	}
	if out.Format == "" {
		out.Format = "sorted_kv_pipe"
	}
	var callbackFields []StructField
	if spec.Callback != nil {
		callbackFields = normalizeStructFields(spec.Callback.Fields)
	}
	for _, f := range cs.Fields {
		entry := CallbackSignatureFieldTemplate{
			OmitIfEmpty: f.OmitIfEmpty,
			Format:      strings.TrimSpace(f.Format),
		}
		if strings.TrimSpace(f.ConstKey) != "" {
			entry.MapKey = strings.TrimSpace(f.ConstKey)
			entry.IsSecretKey = true
		} else if jsonTag := strings.TrimSpace(f.JSON); jsonTag != "" {
			entry.MapKey = jsonTag
			entry.CallbackGoField = callbackFieldForJSON(callbackFields, jsonTag)
			if entry.Format == "" {
				entry.Format = "plain"
			}
		}
		if entry.MapKey != "" {
			out.Fields = append(out.Fields, entry)
		}
	}
	out.SignatureField = callbackFieldForJSON(callbackFields, "signature")
	sigJSON := strings.TrimSpace(cs.SignatureJSON)
	if sigJSON != "" {
		if field := callbackFieldForJSON(callbackFields, sigJSON); field != "" {
			out.SignatureField = field
		}
		out.SignatureJSON = sigJSON
	}
	if out.SignatureJSON == "" {
		out.SignatureJSON = "x-api-signature"
	}
	if out.SignatureField == "" {
		out.SignatureField = "Signature"
	}
	return out
}

func callbackFieldForJSON(fields []StructField, jsonTag string) string {
	for _, f := range fields {
		if f.JSON == jsonTag {
			return f.Name
		}
	}
	parts := strings.Split(jsonTag, "_")
	for i, p := range parts {
		if p == "" {
			continue
		}
		runes := []rune(p)
		runes[0] = unicode.ToUpper(runes[0])
		parts[i] = string(runes)
	}
	return strings.Join(parts, "")
}

func inferResponse(types []ResponseType, kind, defaultName, defaultForeign string, env *ResponseEnvelopeTemplate) (typeName, foreignField string) {
	for _, rt := range types {
		nameLower := strings.ToLower(rt.Name)
		if strings.Contains(nameLower, kind) && !looksLikeEnvelope(rt, env) {
			return rt.Name, findForeignIDField(rt.Fields, defaultForeign)
		}
	}
	for _, rt := range types {
		if rt.Name == defaultName && !looksLikeEnvelope(rt, env) {
			return rt.Name, findForeignIDField(rt.Fields, defaultForeign)
		}
	}
	for _, rt := range types {
		if !looksLikeEnvelope(rt, env) {
			return rt.Name, findForeignIDField(rt.Fields, defaultForeign)
		}
	}
	if len(types) > 0 {
		return types[0].Name, findForeignIDField(types[0].Fields, defaultForeign)
	}
	return "payinResponse", defaultForeign
}

func findForeignIDField(fields []StructField, fallback string) string {
	for _, f := range fields {
		if f.Name == fallback {
			return f.Name
		}
	}
	for _, f := range fields {
		if strings.Contains(f.Name, "Foreign") || strings.HasSuffix(f.Name, "ID") || f.Name == "ID" {
			return f.Name
		}
	}
	return fallback
}

func inferRedirectURLField(types []ResponseType, payloadType string) string {
	for _, rt := range types {
		if payloadType != "" && rt.Name != payloadType {
			continue
		}
		for _, f := range rt.Fields {
			lower := strings.ToLower(f.Name)
			if lower == "redirecturl" || lower == "paymenturl" || lower == "checkouturl" {
				return f.Name
			}
		}
	}
	return "RedirectUrl"
}

func buildStatuses(spec *ProviderSpec, statusType string) (payin, payout, payoutExtra []StatusTemplate) {
	applySharedStatuses(spec)
	payin = mapStatuses(spec.PayinStatuses, statusType)
	payout = mapStatuses(spec.PayoutStatuses, statusType)
	// Payout-only codes still need their consts declared because mapPayoutStatus
	// references them. The const block emits PayinStatuses + PayoutStatusesExtra
	// for both int and string status types, so compute the extras regardless.
	seen := make(map[string]struct{})
	for _, s := range payin {
		seen[s.ConstName] = struct{}{}
	}
	for _, s := range payout {
		if _, ok := seen[s.ConstName]; !ok {
			payoutExtra = append(payoutExtra, s)
		}
	}
	return payin, payout, payoutExtra
}

func applySharedStatuses(spec *ProviderSpec) {
	if spec == nil || len(spec.Statuses) == 0 {
		return
	}
	if len(spec.PayinStatuses) == 0 {
		spec.PayinStatuses = spec.Statuses
	}
	if len(spec.PayoutStatuses) == 0 {
		spec.PayoutStatuses = spec.Statuses
	}
}

func mapStatuses(items []StatusMapping, statusType string) []StatusTemplate {
	out := make([]StatusTemplate, 0, len(items))
	for _, item := range items {
		codeStr := formatStatusCode(item.Code, statusType)
		constName := statusToConstName(item.Code)
		out = append(out, StatusTemplate{
			ConstName:   constName,
			Code:        codeStr,
			StatusTitle: titleCase(item.Status),
			StatusCode:  item.StatusCode,
			Message:     item.Message,
		})
	}
	return out
}

func formatStatusCode(code any, statusType string) string {
	switch v := code.(type) {
	case int64:
		return strconv.FormatInt(v, 10)
	case int:
		return strconv.Itoa(v)
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case string:
		if statusType == "int" {
			return v
		}
		return v
	default:
		return "0"
	}
}

func statusToConstName(code any) string {
	switch v := code.(type) {
	case int64:
		return fmt.Sprintf("providerStatus%d", v)
	case int:
		return fmt.Sprintf("providerStatus%d", v)
	case float64:
		return fmt.Sprintf("providerStatus%d", int(v))
	case string:
		return stringCodeToConstName(v, "providerStatus")
	default:
		return "providerStatusUnknown"
	}
}

func buildPaymentMethodMap(entries []PaymentMethodMapEntry) ([]PaymentMethodMapTemplate, []MacanPayMethodConst) {
	constNames := make(map[string]string) // api value -> const name
	addConst := func(apiName string) {
		if apiName == "" {
			return
		}
		if _, ok := constNames[apiName]; !ok {
			constNames[apiName] = stringCodeToConstName(apiName, "macanPayMethod")
		}
	}

	out := make([]PaymentMethodMapTemplate, 0, len(entries))
	for _, e := range entries {
		addConst(e.APIName)
		t := PaymentMethodMapTemplate{
			Brand:        e.Brand,
			Match:        e.Match,
			MatchKey:     macanMatchKey(e.Match),
			APIName:      e.APIName,
			APINameConst: constNames[e.APIName],
		}
		for _, o := range e.CurrencyOverrides {
			addConst(o.APIName)
			co := CurrencyOverrideTemplate{
				Currency:     o.Currency,
				APIName:      o.APIName,
				APINameConst: constNames[o.APIName],
			}
			t.CurrencyOverrides = append(t.CurrencyOverrides, co)
			if o.Currency == "KGS" {
				t.KGSAPIConst = constNames[o.APIName]
			}
		}
		out = append(out, t)
	}

	consts := make([]MacanPayMethodConst, 0, len(constNames))
	for value, name := range constNames {
		consts = append(consts, MacanPayMethodConst{Name: name, Value: value})
	}
	sort.Slice(consts, func(i, j int) bool { return consts[i].Name < consts[j].Name })
	return out, consts
}

func macanMatchKey(match string) string {
	switch match {
	case "types_cardp2p":
		return "cardp2p"
	case "types_sbp":
		return "sbp"
	case "types_click":
		return "click"
	case "types_qrcode":
		return "qrcode"
	case "types_trans_card2card":
		return "trans_card2card"
	case "types_trans_sbp":
		return "trans_sbp"
	default:
		return "literal"
	}
}

var macanTransBrandAliases = map[string][]MacanPaymentBrandConst{
	"types_trans_card2card": {
		{Name: "paymentBrandTransCard2Card", Value: "trans_card2card"},
		{Name: "paymentBrandTransCard", Value: "trans_card"},
	},
	"types_trans_sbp": {
		{Name: "paymentBrandTransSbp", Value: "trans_sbp"},
		{Name: "paymentBrandTransPhone", Value: "trans_phone"},
	},
}

func buildMacanPaymentBrandConsts(entries []PaymentMethodMapEntry) []MacanPaymentBrandConst {
	seen := make(map[string]bool)
	var out []MacanPaymentBrandConst
	add := func(c MacanPaymentBrandConst) {
		if seen[c.Name] {
			return
		}
		seen[c.Name] = true
		out = append(out, c)
	}
	for _, e := range entries {
		for _, c := range macanTransBrandAliases[e.Match] {
			add(c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func buildMacanBrandMatchCases(templates []PaymentMethodMapTemplate) []MacanBrandMatchCase {
	standard := map[string]MacanBrandMatchCase{
		"cardp2p":         {MatchConst: "matchCardP2P", MatchValue: "cardp2p", EqualExpr: "types.CardP2PMethod.Equal(brand)"},
		"sbp":             {MatchConst: "matchSBP", MatchValue: "sbp", EqualExpr: "types.SBPMethod.Equal(brand)"},
		"click":           {MatchConst: "matchClick", MatchValue: "click", EqualExpr: "types.ClickMethod.Equal(brand)"},
		"qrcode":          {MatchConst: "matchQRCode", MatchValue: "qrcode", EqualExpr: "types.QRCodeMethod.Equal(brand)"},
		"trans_card2card": {MatchConst: "matchTransCard2Card", MatchValue: "trans_card2card", EqualExpr: "paymentBrandTransCard2Card.Equal(brand) || paymentBrandTransCard.Equal(brand)"},
		"trans_sbp":       {MatchConst: "matchTransSBP", MatchValue: "trans_sbp", EqualExpr: "paymentBrandTransSbp.Equal(brand) || paymentBrandTransPhone.Equal(brand)"},
	}
	seen := make(map[string]bool)
	var out []MacanBrandMatchCase
	for _, t := range templates {
		key := t.MatchKey
		if key == "literal" || seen[key] {
			continue
		}
		c, ok := standard[key]
		if !ok {
			continue
		}
		seen[key] = true
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MatchConst < out[j].MatchConst })
	return out
}

func titleCase(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func buildConstructorDepMocks(deps []ConstructorDep) []ConstructorDepMock {
	out := make([]ConstructorDepMock, 0, len(deps))
	for _, d := range deps {
		out = append(out, ConstructorDepMock{
			Name:      d.Name,
			MockField: depMockField(d.Name),
		})
	}
	return out
}

func depMockField(name string) string {
	if name == "" {
		return ""
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

func hasMacanRedirectMethods(methods []string) bool {
	for _, m := range methods {
		if m == "click" || m == "qrcode" {
			return true
		}
	}
	return false
}

func buildMacanMethodTests(methods []string) []MacanMethodTest {
	has := func(want string) bool {
		for _, m := range methods {
			if m == want {
				return true
			}
		}
		return false
	}
	var tests []MacanMethodTest
	if has("cardp2p") {
		tests = append(tests, MacanMethodTest{"cardp2p", `types.CardP2PMethod.String()`, "RUB", "card2card", false})
	}
	if has("sbp") {
		tests = append(tests, MacanMethodTest{"sbp", `types.SBPMethod.String()`, "RUB", "sbp", false})
	}
	if has("trans_sbp") {
		tests = append(tests, MacanMethodTest{"trans_sbp", `paymentBrandTransSbp.String()`, "RUB", "trans_sbp", false})
	}
	if has("trans_card2card") {
		tests = append(tests, MacanMethodTest{"trans_card2card", `paymentBrandTransCard2Card.String()`, "RUB", "trans_card2card", false})
	}
	if has("qrcode") {
		tests = append(tests, MacanMethodTest{"qrcode_kgs", `types.QRCodeMethod.String()`, "KGS", "elqr", false})
		tests = append(tests, MacanMethodTest{"qrcode_rub", `types.QRCodeMethod.String()`, "RUB", "nspk", false})
	}
	if has("click") {
		tests = append(tests, MacanMethodTest{"click_rub", `types.ClickMethod.String()`, "RUB", "nspk", false})
	}
	if len(tests) > 0 {
		tests = append(tests, MacanMethodTest{"unknown", `"unknown"`, "RUB", "", true})
	}
	return tests
}

func buildTnxStatusVars(spec *ProviderSpec, data *TemplateData) []TnxStatusVar {
	set := make(map[string]struct{})
	add := func(s string) {
		if s != "" {
			set[s] = struct{}{}
		}
	}
	for _, list := range [][]StatusMapping{spec.PayinStatuses, spec.PayoutStatuses} {
		for _, m := range list {
			add(m.Status)
		}
	}
	for _, list := range [][]ErrorMapping{spec.StatusDetails, spec.ErrorCodes} {
		for _, m := range list {
			add(m.Status)
		}
	}
	add("pending")

	if data.UseTransfertyH2H {
		for _, s := range []string{"success", "pending", "declined", "error", "blocked"} {
			add(s)
		}
	}
	if data.HasP2P || data.HasPayin {
		add("pending")
		add("declined")
		add("error")
	}
	if data.CheckStatusForeignIDEmpty == "error_status" || data.HasPayout {
		add("error")
	}

	order := []string{"success", "pending", "declined", "error", "blocked"}
	out := make([]TnxStatusVar, 0, len(order))
	for _, kind := range order {
		if _, ok := set[kind]; !ok {
			continue
		}
		out = append(out, TnxStatusVar{Kind: kind, TitleName: titleCase(kind)})
	}
	return out
}

func buildCodeMappings(items []ErrorMapping, constPrefix string) []ErrorTemplate {
	out := make([]ErrorTemplate, 0, len(items))
	for _, e := range items {
		code := strings.TrimSpace(e.Code)
		_, err := strconv.Atoi(code)
		out = append(out, ErrorTemplate{
			ConstName:   stringCodeToConstName(code, constPrefix),
			Code:        code,
			CodeIsInt:   err == nil,
			StatusTitle: titleCase(e.Status),
			StatusCode:  e.StatusCode,
		})
	}
	return out
}

func stringCodeToConstName(code, prefix string) string {
	var b strings.Builder
	b.WriteString(prefix)
	upperNext := true
	for _, r := range code {
		if r == '_' || r == '-' {
			upperNext = true
			continue
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			continue
		}
		if upperNext {
			b.WriteRune(unicode.ToUpper(r))
			upperNext = false
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func structFieldFromResolved(f ResolvedField) StructField {
	return StructField{
		Name:      f.GoName,
		Type:      f.Type,
		JSON:      f.JSON,
		OmitEmpty: f.OmitEmpty,
		Omitempty: f.OmitEmpty,
	}
}

func normalizeStructFields(fields []StructField) []StructField {
	out := make([]StructField, len(fields))
	for i, f := range fields {
		f.Omitempty = f.OmitEmpty
		out[i] = f
	}
	return out
}

func normalizeResponseTypes(types []ResponseType) []ResponseType {
	out := make([]ResponseType, len(types))
	for i, rt := range types {
		rt.Fields = normalizeStructFields(rt.Fields)
		out[i] = rt
	}
	return out
}

func statusTests(statuses []StatusTemplate, statusType string) []StatusTestTemplate {
	if len(statuses) == 0 {
		return nil
	}
	out := make([]StatusTestTemplate, 0, len(statuses))
	for _, s := range statuses {
		name := strings.TrimPrefix(s.ConstName, "providerStatus")
		if name == "" {
			name = s.Code
		}
		out = append(out, StatusTestTemplate{
			Name:           name,
			Code:           s.ConstName,
			ExpectedStatus: s.StatusTitle,
			ExpectedCode:   s.StatusCode,
		})
	}
	return out
}

func durationGoExpr(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "0"
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return "0"
	}
	if d%time.Second == 0 {
		n := d / time.Second
		if n == 1 {
			return "time.Second"
		}
		return fmt.Sprintf("%d * time.Second", n)
	}
	if d%time.Millisecond == 0 {
		n := d / time.Millisecond
		if n == 1 {
			return "time.Millisecond"
		}
		return fmt.Sprintf("%d * time.Millisecond", n)
	}
	return fmt.Sprintf("%d", int64(d))
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func defaultCheckStatusForeignIDEmpty(value string) string {
	switch strings.TrimSpace(value) {
	case "declined", "error_status":
		return value
	default:
		return "error"
	}
}

func jsonKeyToGoField(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	runes := []rune(key)
	runes[0] = rune(strings.ToUpper(string(runes[0]))[0])
	return string(runes)
}

func envelopeWrapperTypeName(innerName string) string {
	if strings.HasSuffix(innerName, "Message") {
		return strings.TrimSuffix(innerName, "Message") + "Response"
	}
	return innerName + "Envelope"
}

func looksLikeEnvelope(rt ResponseType, env *ResponseEnvelopeTemplate) bool {
	if env == nil || !env.Enabled {
		return false
	}
	success, wrapper := env.SuccessGoField, env.WrapperGoField
	hasSuccess, hasWrapper, hasStatus := false, false, false
	for _, f := range rt.Fields {
		switch f.Name {
		case success:
			hasSuccess = true
		case wrapper:
			hasWrapper = true
		case "Status":
			if !strings.EqualFold(f.Type, "bool") {
				hasStatus = true
			}
		}
	}
	return hasSuccess && hasWrapper && !hasStatus
}

func nestUnderEnvelope(env *ResponseEnvelopeTemplate, field string) string {
	if env == nil || !env.Enabled || field == "" || strings.Contains(field, ".") {
		return field
	}
	return env.WrapperGoField + "." + field
}

func ensureResponseEnvelopeType(types []ResponseType, envelopeName, payloadName string, env *ResponseEnvelopeTemplate) []ResponseType {
	if env == nil || !env.Enabled || envelopeName == "" || payloadName == "" || envelopeName == payloadName {
		return types
	}
	for _, rt := range types {
		if rt.Name == envelopeName {
			return types
		}
	}
	return append(types, ResponseType{
		Name: envelopeName,
		Fields: []StructField{
			{Name: env.SuccessGoField, Type: "bool", JSON: env.SuccessField},
			{Name: env.WrapperGoField, Type: payloadName, JSON: env.WrapperField},
		},
	})
}

// formatDurationGoExpr converts CUE duration strings into valid Go duration expressions.
func formatDurationGoExpr(period string) string {
	period = strings.TrimSpace(period)
	if period == "" {
		return ""
	}
	if strings.Contains(period, "*") {
		return period
	}
	switch {
	case strings.HasSuffix(period, "ms"):
		return strings.TrimSuffix(period, "ms") + " * time.Millisecond"
	case strings.HasSuffix(period, "s"):
		return strings.TrimSuffix(period, "s") + " * time.Second"
	case strings.HasSuffix(period, "m"):
		return strings.TrimSuffix(period, "m") + " * time.Minute"
	case strings.HasSuffix(period, "h"):
		return strings.TrimSuffix(period, "h") + " * time.Hour"
	default:
		return strconv.Quote(period)
	}
}

func inferStatusField(types []ResponseType, kind, fallback string) string {
	for _, rt := range types {
		nameLower := strings.ToLower(rt.Name)
		if strings.Contains(nameLower, kind) {
			for _, f := range rt.Fields {
				if f.Name == fallback || strings.EqualFold(f.Name, "status") || f.Name == "State" {
					return f.Name
				}
			}
		}
	}
	for _, rt := range types {
		for _, f := range rt.Fields {
			if f.Name == fallback || strings.EqualFold(f.Name, "status") || f.Name == "State" {
				return f.Name
			}
		}
	}
	return fallback
}
