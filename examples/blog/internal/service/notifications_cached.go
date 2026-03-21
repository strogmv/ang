package service

import (
	"context"

	"github.com/example/blog/internal/port"
)

type NotificationsCached struct {
	base port.Notifications
}

func NewNotificationsCached(base port.Notifications) *NotificationsCached {
	return &NotificationsCached{base: base}
}
func (c *NotificationsCached) SendEmailVerification(ctx context.Context, req port.SendEmailVerificationRequest) (port.SendEmailVerificationResponse, error) {
	return c.base.SendEmailVerification(ctx, req)
}
func (c *NotificationsCached) SendPasswordResetEmail(ctx context.Context, req port.SendPasswordResetEmailRequest) (port.SendPasswordResetEmailResponse, error) {
	return c.base.SendPasswordResetEmail(ctx, req)
}
