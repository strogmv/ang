package paymentprovider

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
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

	PayinRequestType  string
	PayoutRequestType string
	RefundRequestType string

	PayinEndpointConst        string
	PayoutEndpointConst       string
	RefundEndpointConst       string
	PayinStatusEndpointConst  string
	PayoutStatusEndpointConst string
	PayinStatusMethod         string
	PayoutStatusMethod        string

	PayinResponseType    string
	PayoutResponseType   string
	RefundResponseType   string
	PayinForeignIDField  string
	PayoutForeignIDField string
	RefundForeignIDField string

	SecretParts                       []SecretPart
	SecretPartsCount                  int
	SecretPartsNeedTransform          bool
	SecretPartsSimple                 bool
	SecretSeparator                   string
	SecretFormat                      string
	SecretTestValue                   string
	HasOptionalReturnRecipientDetails bool

	SigningAlgorithm   string
	SigningFormat      string
	SigningSecretField string
	UseBasicAuth       bool
	UseTransfertyH2H   bool
	UseMacanP2P        bool
	UsePaytechGateway  bool
	UseFluxsgate       bool
	SecretUseLabels    bool // CUE: secrets.use_labels
	PubKeyField        string
	SecretKeyField     string

	PayoutRuntime             *PayoutRuntimeTemplate
	CallbackRuntime           *CallbackRuntimeTemplate
	InitPayoutPolicy          *InitPayoutPolicyTemplate
	RequestSigning            *RequestSigningTemplate
	CheckStatusForeignIDEmpty string
	ResponseFormat            string
	CallbackFormat            string
	ResponseEnvelope          *ResponseEnvelopeTemplate

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

	CallbackTxIDField      string
	CallbackForeignIDField string
	CallbackStatusField    string
	CallbackErrorCodeField string

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

	PayinStatuses       []StatusTemplate
	PayoutStatuses      []StatusTemplate
	PayoutStatusesExtra []StatusTemplate
	ErrorCodes          []ErrorTemplate
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
	UseOperationRuntime bool // operations table + retry/timeout helpers (not for macan_p2p yet)
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
}

type CheckStatusConfigTemplate struct {
	Enabled             bool
	SinceCreatedPeriod  string
	ByTransactionType   bool
	PathSuffixForeignID bool
}

type PayoutRuntimeTemplate struct {
	ForeignIDOnUnexpectedError bool
}

type CallbackRuntimeTemplate struct {
	FinishViaCheckStatus bool
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
	}
	Retries struct {
		MaxAttempts      int
		InitialBackoff   string
		MaxBackoff       string
		RetryOnNotFound  bool
		RetryOn5xx       bool
		RetryOnRateLimit bool
	}
	Limits struct {
		MaxCallbackBodyBytes int
		MaxPendingAge        string
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

type ResolvedRequestDef struct {
	Name   string
	Fields []ResolvedField
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
	StatusTitle string
	StatusCode  string
}

type GroupedStatusDetail struct {
	ConstNames  []string
	StatusTitle string
	StatusCode  string
}

func (g GroupedStatusDetail) CaseLabel() string {
	return strings.Join(g.ConstNames, ", ")
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
	data.SecretPartsSimple = len(spec.Secrets.Parts) > 0
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
		}
	}
	if spec.PayoutRuntime != nil {
		data.PayoutRuntime = &PayoutRuntimeTemplate{
			ForeignIDOnUnexpectedError: spec.PayoutRuntime.ForeignIDOnUnexpectedError,
		}
	}
	if spec.CallbackRuntime != nil {
		data.CallbackRuntime = &CallbackRuntimeTemplate{
			FinishViaCheckStatus: spec.CallbackRuntime.FinishViaCheckStatus,
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
		data.RequestSigning = &RequestSigningTemplate{
			Algorithm:        spec.RequestSigning.Algorithm,
			Format:           spec.RequestSigning.Format,
			Header:           spec.RequestSigning.Header,
			SecretKeyField:   exportGoIdent(spec.RequestSigning.SecretKey),
			UsernameHeader:   spec.RequestSigning.UsernameHeader,
			UsernameKeyField: exportGoIdent(spec.RequestSigning.UsernameKey),
			Encoding:         defaultString(spec.RequestSigning.Encoding, "hex"),
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
		}
	}
	if spec.OTPConfig != nil {
		data.HasOTPConfig = true
		data.OTPHandlesExternally = spec.OTPConfig.HandlesExternally
	}
	if spec.Callback != nil {
		data.CallbackTxIDField = spec.Callback.TxIDField
		data.CallbackForeignIDField = spec.Callback.ForeignIDField
		data.CallbackStatusField = spec.Callback.StatusField
		data.CallbackErrorCodeField = spec.Callback.ErrorCodeField
		data.CallbackReturnQueryTxIDParam = spec.Callback.ReturnQueryTxIDParam
		data.CallbackReturnQueryStatusValue = spec.Callback.ReturnQueryStatusValue
		data.CallbackReturnQueryInfoCallback = spec.Callback.ReturnQueryInfoCallback
		data.CallbackFields = normalizeStructFields(spec.Callback.Fields)
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
		rp.Limits.MaxCallbackBodyBytes = spec.RuntimePolicyConfig.Limits.MaxCallbackBodyBytes
		rp.Limits.MaxPendingAge = spec.RuntimePolicyConfig.Limits.MaxPendingAge
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
	if !hasEndpoint(spec.Endpoints, "payout_status") && hasEndpoint(spec.Endpoints, "check") {
		data.PayoutStatusEndpointConst = endpointConst(spec.Endpoints, "check", "endpointCheck")
	}
	data.PayinStatusMethod = endpointMethod(spec.Endpoints, "payin_status", "GET")
	data.PayoutStatusMethod = endpointMethod(spec.Endpoints, "payout_status", "GET")
	if data.PayoutStatusMethod == "GET" && hasEndpoint(spec.Endpoints, "check") {
		data.PayoutStatusMethod = endpointMethod(spec.Endpoints, "check", "GET")
	}
	data.HasStatusEndpoints = hasEndpoint(spec.Endpoints, "payin_status") || hasEndpoint(spec.Endpoints, "payout_status") || hasEndpoint(spec.Endpoints, "check")

	if spec.HasP2P && len(spec.SupportedMethods) > 0 {
		data.P2PMethodCheck = strings.ToUpper(spec.SupportedMethods[0])
	}

	var err error
	if spec.PayinRequest != nil {
		data.PayinRequest, err = resolveRequestDef(spec.PayinRequest, spec.PaymentSource, currencyNum)
		if err != nil {
			return nil, err
		}
		data.PayinRequestType = spec.PayinRequest.Name
	}
	if spec.PayoutRequest != nil {
		data.PayoutRequest, err = resolveRequestDef(spec.PayoutRequest, spec.PaymentSource, currencyNum)
		if err != nil {
			return nil, err
		}
		data.PayoutRequestType = spec.PayoutRequest.Name
	}
	if spec.PayinStatusRequest != nil {
		data.PayinStatusRequest, err = resolveRequestDef(spec.PayinStatusRequest, spec.PaymentSource, currencyNum)
		if err != nil {
			return nil, err
		}
	}
	if spec.PayoutStatusRequest != nil {
		data.PayoutStatusRequest, err = resolveRequestDef(spec.PayoutStatusRequest, spec.PaymentSource, currencyNum)
		if err != nil {
			return nil, err
		}
	}
	if spec.RefundRequest != nil {
		data.RefundRequest, err = resolveRequestDef(spec.RefundRequest, spec.PaymentSource, currencyNum)
		if err != nil {
			return nil, err
		}
		data.RefundRequestType = spec.RefundRequest.Name
	}
	if spec.P2PRequest != nil {
		data.P2PRequest, err = resolveRequestDef(spec.P2PRequest, "apm", currencyNum)
		if err != nil {
			return nil, err
		}
	} else if data.HasP2P && spec.PayinRequest != nil {
		data.P2PRequest = data.PayinRequest
	}

	data.PayinResponsePayloadType, data.PayinForeignIDField = inferResponse(spec.ResponseTypes, "payin", "payinProfile", "PaymentID")
	data.PayoutResponsePayloadType, data.PayoutForeignIDField = inferResponse(spec.ResponseTypes, "payout", "payoutMessage", "ReferenceID")
	data.RefundResponseType, data.RefundForeignIDField = inferResponse(spec.ResponseTypes, "refund", "refundResponse", "PaymentID")

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

	data.PayinStatuses, data.PayoutStatuses, data.PayoutStatusesExtra = buildStatuses(spec, statusType)
	data.ErrorCodes = buildCodeMappings(spec.ErrorCodes, "errCode")
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
	if strings.EqualFold(spec.APICompat, "macan_p2p") {
		out = appendImportIfMissing(out, "gitlab.q-tech.host/transferty/backend/utils/helpers")
		if spec.Interfaces.BalanceFetcher {
			out = appendImportIfMissing(out, "gitlab.q-tech.host/transferty/backend/tnx_processor/payment_providers/common")
		}
	}
	sort.Strings(out)
	return out
}

func appendImportIfMissing(imports []string, path string) []string {
	for _, imp := range imports {
		if imp == path {
			return imports
		}
	}
	return append(imports, path)
}

func resolveRequestDef(def *RequestDef, paymentSource string, currencyNum int) (*ResolvedRequestDef, error) {
	fields, err := ResolveRequestFields(def.Fields, paymentSource, currencyNum)
	if err != nil {
		return nil, err
	}
	// Refine IsCard/IsAPM for owner_info based on OwnerFrom
	for i := range fields {
		f := &def.Fields[i]
		if f.Source == "owner_info" {
			from := strings.TrimSpace(f.OwnerFrom)
			if from == "card" {
				fields[i].IsCard, fields[i].IsAPM = true, false
			} else if from == "apm" {
				fields[i].IsCard, fields[i].IsAPM = false, true
			} else if paymentSource == "card" {
				fields[i].IsCard, fields[i].IsAPM = true, false
			} else {
				fields[i].IsCard, fields[i].IsAPM = false, true
			}
		}
	}
	return &ResolvedRequestDef{Name: def.Name, Fields: fields}, nil
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
	for _, k := range keys {
		ep := endpoints[k]
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

func inferResponse(types []ResponseType, kind, defaultName, defaultForeign string) (typeName, foreignField string) {
	for _, rt := range types {
		nameLower := strings.ToLower(rt.Name)
		if strings.Contains(nameLower, kind) {
			return rt.Name, findForeignIDField(rt.Fields, defaultForeign)
		}
	}
	for _, rt := range types {
		if rt.Name == defaultName {
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

func buildStatuses(spec *ProviderSpec, statusType string) (payin, payout, payoutExtra []StatusTemplate) {
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
		out = append(out, ErrorTemplate{
			ConstName:   stringCodeToConstName(e.Code, constPrefix),
			Code:        e.Code,
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
				if f.Name == fallback || strings.EqualFold(f.Name, "status") {
					return f.Name
				}
			}
		}
	}
	return fallback
}
