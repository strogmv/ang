package noop

import (
	"context"

	"github.com/example/minimal/internal/port"
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
