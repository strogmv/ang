package noop

import (
	"context"

	"github.com/example/blog/internal/port"
)

type Client struct{}

func New() *Client {
	return &Client{}
}

func (c *Client) Send(ctx context.Context, msg port.EmailMessage) error {
	_ = ctx
	_ = msg
	return nil
}
