package service

import (
	"context"

	"github.com/strogmv/ang/internal/port"
)

type AssistantCached struct {
	base port.Assistant
}

func NewAssistantCached(base port.Assistant) *AssistantCached {
	return &AssistantCached{base: base}
}
func (c *AssistantCached) ChatWithAssistant(ctx context.Context, req port.ChatWithAssistantRequest) (port.ChatWithAssistantResponse, error) {
	return c.base.ChatWithAssistant(ctx, req)
}
func (c *AssistantCached) LookupPostForAssistant(ctx context.Context, req port.LookupPostForAssistantRequest) (port.LookupPostForAssistantResponse, error) {
	return c.base.LookupPostForAssistant(ctx, req)
}
