package nats

import (
	"context"
	"encoding/json"
	natspkg "github.com/nats-io/nats.go"
	"github.com/strogmv/ang/internal/domain"
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
func (c *Client) PublishUserLoggedIn(ctx context.Context, event domain.UserLoggedIn) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return c.nc.Publish("UserLoggedIn", data)
}

func (c *Client) SubscribeUserLoggedIn(handler func(context.Context, domain.UserLoggedIn) error) (*natspkg.Subscription, error) {
	return c.nc.Subscribe("UserLoggedIn", func(msg *natspkg.Msg) {
		var event domain.UserLoggedIn
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			return
		}
		_ = handler(context.Background(), event)
	})
}
func (c *Client) PublishUserRegistered(ctx context.Context, event domain.UserRegistered) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return c.nc.Publish("UserRegistered", data)
}

func (c *Client) SubscribeUserRegistered(handler func(context.Context, domain.UserRegistered) error) (*natspkg.Subscription, error) {
	return c.nc.Subscribe("UserRegistered", func(msg *natspkg.Msg) {
		var event domain.UserRegistered
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			return
		}
		_ = handler(context.Background(), event)
	})
}
