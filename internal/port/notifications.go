package port

import (
	"context"
)

type Notifications interface {
	SendInvitationEmail(ctx context.Context, req SendInvitationEmailRequest) (SendInvitationEmailResponse, error)
	SendPasswordResetEmail(ctx context.Context, req SendPasswordResetEmailRequest) (SendPasswordResetEmailResponse, error)
}

// Request/Response DTOs
type SendInvitationEmailRequest struct {
	Email       string `json:"email"`
	Name        string `json:"name"`
	InviterName string `json:"inviterName"`
	InviteURL   string `json:"inviteUrl"`
}

func (d *SendInvitationEmailRequest) Validate() error {
	return nil
}

type SendInvitationEmailResponse struct {
	Ok bool `json:"ok"`
}

func (d *SendInvitationEmailResponse) Validate() error {
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
