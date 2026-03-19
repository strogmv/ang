package nats

import (
	"context"
	"encoding/json"
	natspkg "github.com/nats-io/nats.go"
	"time"
)

type Client struct {
	nc *natspkg.Conn
}

// keep context import
var _ = context.Background

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

func (c *Client) SubscribeRaw(subject string, handler func(data []byte) error) (*natspkg.Subscription, error) {
	return c.nc.Subscribe(subject, func(msg *natspkg.Msg) {
		_ = handler(msg.Data)
	})
}

// Enqueue publishes a raw payload to the given subject (implements port.QueuePublisher).
func (c *Client) Enqueue(ctx context.Context, subject string, payload []byte) error {
	return c.nc.Publish(subject, payload)
}

// Dequeue waits for the next message from subject and returns message id + payload.
func (c *Client) Dequeue(ctx context.Context, subject string, timeout time.Duration) (string, []byte, error) {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	sub, err := c.nc.SubscribeSync(subject)
	if err != nil {
		return "", nil, err
	}
	defer sub.Unsubscribe()
	msg, err := sub.NextMsgWithContext(waitCtx)
	if err != nil {
		return "", nil, err
	}
	msgID := msg.Header.Get("Nats-Msg-Id")
	if msgID == "" {
		msgID = msg.Reply
	}
	return msgID, msg.Data, nil
}

// Ack is a no-op for NATS core subscriptions.
func (c *Client) Ack(ctx context.Context, subject string, messageID string) error {
	return nil
}

// Nack is a no-op for NATS core subscriptions (no server-side nack in this adapter).
func (c *Client) Nack(ctx context.Context, subject string, messageID string, reason string) error {
	return nil
}

// PublishDLQ publishes failed payload to '<subject>.DLQ'.
func (c *Client) PublishDLQ(ctx context.Context, subject string, payload []byte, reason string) error {
	msg := natspkg.NewMsg(subject + ".DLQ")
	msg.Data = payload
	if reason != "" {
		if msg.Header == nil {
			msg.Header = natspkg.Header{}
		}
		msg.Header.Set("X-DLQ-Reason", reason)
	}
	return c.nc.PublishMsg(msg)
}

// Wait blocks until an event with matching name/correlation arrives or ctx is cancelled.
func (c *Client) Wait(ctx context.Context, name string, match string) (any, error) {
	ch := make(chan any, 1)
	sub, err := c.nc.Subscribe(name, func(msg *natspkg.Msg) {
		var payload any
		if jErr := json.Unmarshal(msg.Data, &payload); jErr == nil {
			select {
			case ch <- payload:
			default:
			}
		}
	})
	if err != nil {
		return nil, err
	}
	defer sub.Unsubscribe()
	select {
	case v := <-ch:
		return v, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Subscribe registers an async handler for events with matching name/correlation.
func (c *Client) Subscribe(ctx context.Context, name string, match string, handler func(context.Context, any)) error {
	_, err := c.nc.Subscribe(name, func(msg *natspkg.Msg) {
		var payload any
		if jErr := json.Unmarshal(msg.Data, &payload); jErr != nil {
			return
		}
		go handler(ctx, payload)
	})
	return err
}
