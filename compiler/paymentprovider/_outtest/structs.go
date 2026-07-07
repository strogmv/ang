package testpp

import (
	"encoding/json"
)

type payinRequest struct {
	OrderID  string `json:"orderId"`
	Amount   string `json:"amount"`
	Currency int    `json:"currency"`
	ApiKey   string `json:"apiKey"`
	Salt     string `json:"salt,omitempty"`
}

func (r *payinRequest) Marshal() ([]byte, error) { return json.Marshal(r) }

func (r payinRequest) String() (string, error) {
	b, err := json.Marshal(r)
	return string(b), err
}

type payinResponse struct {
	PaymentID string `json:"paymentId"`
}
