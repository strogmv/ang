package port

import (
	"context"
)

type Assistant interface {
	ChatWithAssistant(ctx context.Context, req ChatWithAssistantRequest) (ChatWithAssistantResponse, error)
	LookupPostForAssistant(ctx context.Context, req LookupPostForAssistantRequest) (LookupPostForAssistantResponse, error)
}

// Request/Response DTOs
type ChatWithAssistantRequest struct {
	UserID   string                                 `json:"userId"`
	Message  string                                 `json:"message"`
	Messages []ChatWithAssistantRequestMessagesItem `json:"messages"`
}

func (d *ChatWithAssistantRequest) Validate() error {
	return nil
}

type ChatWithAssistantResponse struct {
	Content          string `json:"content"`
	FinishReason     string `json:"finishReason"`
	ToolCalls        int    `json:"toolCalls"`
	PromptTokens     int    `json:"promptTokens"`
	CompletionTokens int    `json:"completionTokens"`
	TotalTokens      int    `json:"totalTokens"`
}

func (d *ChatWithAssistantResponse) Validate() error {
	return nil
}

type LookupPostForAssistantRequest struct {
	Slug string `json:"slug"`
}

func (d *LookupPostForAssistantRequest) Validate() error {
	return nil
}

type LookupPostForAssistantResponse struct {
	ID      string `json:"ID"`
	Title   string `json:"title"`
	Status  string `json:"status"`
	Excerpt string `json:"excerpt"`
}

func (d *LookupPostForAssistantResponse) Validate() error {
	return nil
}

type ChatWithAssistantRequestMessagesItem struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (d *ChatWithAssistantRequestMessagesItem) Validate() error {
	return nil
}
