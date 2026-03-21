package port

import (
	"context"
)

type Notifications interface {
	SendEmailVerification(ctx context.Context, req SendEmailVerificationRequest) (SendEmailVerificationResponse, error)
	SendPasswordResetEmail(ctx context.Context, req SendPasswordResetEmailRequest) (SendPasswordResetEmailResponse, error)
}

// Request/Response DTOs
type SendEmailVerificationRequest struct {
	Email     string `json:"email"`
	Name      string `json:"name"`
	VerifyURL string `json:"verifyUrl"`
}

func (d *SendEmailVerificationRequest) Validate() error {
	return nil
}

type SendEmailVerificationResponse struct {
	Ok bool `json:"ok"`
}

func (d *SendEmailVerificationResponse) Validate() error {
	return nil
}

type SendPasswordResetEmailRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	ResetURL string `json:"resetUrl"`
}

func (d *SendPasswordResetEmailRequest) Validate() error {
	return nil
}

type SendPasswordResetEmailResponse struct {
	Ok bool `json:"ok"`
}

func (d *SendPasswordResetEmailResponse) Validate() error {
	return nil
}
