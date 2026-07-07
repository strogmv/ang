package testpp

import (
	"encoding/json"
	"fmt"

	"gitlab.q-tech.host/transferty/backend/tnx_processor/model"
	providers "gitlab.q-tech.host/transferty/backend/tnx_processor/payment_providers"
)

const (
	ppSID       = "testpp"
	ppLabel     = "Test"
	ppMIDPrefix = "TST"
)

const (
	endpointPayin = "/payin"
)

const (
	maxCallbackBodyBytes = 1 << 20

	errUnmarshalRespFmt     = "testpp: unmarshal response: %w"
	errUnmarshalCallbackFmt = "testpp: unmarshal callback: %w"
	errReadCallbackBodyFmt  = "testpp: read callback body: %w"
	errCallbackTooLargeFmt  = "testpp: callback body too large (limit %d bytes)"
)

type providerStatus string

const (
	providerStatus1  providerStatus = "1"
	providerStatus10 providerStatus = "10"
)

var (
	tnxStatusSuccess, _  = model.ParseStatus("success")
	tnxStatusPending, _  = model.ParseStatus("pending")
	tnxStatusDeclined, _ = model.ParseStatus("declined")
	tnxStatusError, _    = model.ParseStatus("error")
	tnxStatusBlocked, _  = model.ParseStatus("blocked")
)

func mapPayinStatus(status providerStatus) (model.TnxStatus, model.TnxStatusCode, string) {
	switch status {
	case providerStatus1:
		return tnxStatusPending, model.SCodeOk, model.SCodeOk.Description()
	case providerStatus10:
		return tnxStatusSuccess, model.SCodeOk, model.SCodeOk.Description()
	default:
		return tnxStatusPending, model.SCodeOk, string(status)
	}
}

func mapPayoutStatus(status providerStatus) (model.TnxStatus, model.TnxStatusCode, string) {
	switch status {
	default:
		return tnxStatusPending, model.SCodeOk, string(status)
	}
}
