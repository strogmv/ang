package nats

import (
	natspkg "github.com/nats-io/nats.go"
)

type Client struct {
	nc *natspkg.Conn
}

func NewClient(url string) (*Client, error) {
	nc, err := natspkg.Connect(url)
	if err != nil {
		return nil, err
	}
	return &Client{nc: nc}, nil
}

func (c *Client) Close() {
	c.nc.Close()
}

func (c *Client) IsConnected() bool {
	return c.nc != nil && c.nc.Status() == natspkg.CONNECTED
}

func (c *Client) Subscribe(subject string, handler func(data []byte) error) (*natspkg.Subscription, error) {
	return c.nc.Subscribe(subject, func(msg *natspkg.Msg) {
		_ = handler(msg.Data)
	})
}
