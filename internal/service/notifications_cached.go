package service

import (
	"context"

	"github.com/strogmv/ang/internal/port"
)

type NotificationsCached struct {
	base port.Notifications
}

func NewNotificationsCached(base port.Notifications) *NotificationsCached {
	return &NotificationsCached{base: base}
}
func (c *NotificationsCached) SendInvitationEmail(ctx context.Context, req port.SendInvitationEmailRequest) (port.SendInvitationEmailResponse, error) {
	return c.base.SendInvitationEmail(ctx, req)
}
func (c *NotificationsCached) SendPasswordResetEmail(ctx context.Context, req port.SendPasswordResetEmailRequest) (port.SendPasswordResetEmailResponse, error) {
	return c.base.SendPasswordResetEmail(ctx, req)
}
