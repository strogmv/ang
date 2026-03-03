package nats

import (
	"context"
	"encoding/json"
	"fmt"
	natspkg "github.com/nats-io/nats.go"
	"github.com/strogmv/ang/internal/domain"
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
func (c *Client) PublishUserLoggedIn(ctx context.Context, event domain.UserLoggedIn) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	delay := time.Duration(100) * time.Millisecond
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if lastErr = c.nc.Publish("UserLoggedIn", data); lastErr == nil {
			return nil
		}
		if attempt < 3-1 {
			time.Sleep(delay)
			delay *= 2
		}
	}
	fmt.Printf("NATS: publish failed after %d attempts [UserLoggedIn]: %v\n", 3, lastErr)
	return lastErr
}

// BroadcastUserLoggedIn delegates to PublishUserLoggedIn — NATS subjects are already broadcast channels.
func (c *Client) BroadcastUserLoggedIn(ctx context.Context, event domain.UserLoggedIn) error {
	return c.PublishUserLoggedIn(ctx, event)
}

// SubscribeUserLoggedIn processes messages concurrently using a worker pool of 20 goroutines.
// The NATS callback returns immediately (non-blocking) — true backpressure via channel semaphore.
// When the pool is full the message is shed (dropped) rather than queued to prevent cascade delays.
func (c *Client) SubscribeUserLoggedIn(handler func(context.Context, domain.UserLoggedIn) error) (*natspkg.Subscription, error) {
	sem := make(chan struct{}, 20)
	return c.nc.Subscribe("UserLoggedIn", func(msg *natspkg.Msg) {
		select {
		case sem <- struct{}{}:
		default:
			fmt.Printf("NATS [UserLoggedIn]: worker pool full (%d), shedding message\n", 20)
			return
		}
		go func() {
			defer func() { <-sem }()
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("NATS handler panic recovered [UserLoggedIn]: %v\n", r)
				}
			}()
			var event domain.UserLoggedIn
			if err := json.Unmarshal(msg.Data, &event); err != nil {
				return
			}
			_ = handler(context.Background(), event)
		}()
	})
}
func (c *Client) PublishUserRegistered(ctx context.Context, event domain.UserRegistered) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	delay := time.Duration(100) * time.Millisecond
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if lastErr = c.nc.Publish("UserRegistered", data); lastErr == nil {
			return nil
		}
		if attempt < 3-1 {
			time.Sleep(delay)
			delay *= 2
		}
	}
	fmt.Printf("NATS: publish failed after %d attempts [UserRegistered]: %v\n", 3, lastErr)
	return lastErr
}

// BroadcastUserRegistered delegates to PublishUserRegistered — NATS subjects are already broadcast channels.
func (c *Client) BroadcastUserRegistered(ctx context.Context, event domain.UserRegistered) error {
	return c.PublishUserRegistered(ctx, event)
}

// SubscribeUserRegistered processes messages concurrently using a worker pool of 20 goroutines.
// The NATS callback returns immediately (non-blocking) — true backpressure via channel semaphore.
// When the pool is full the message is shed (dropped) rather than queued to prevent cascade delays.
func (c *Client) SubscribeUserRegistered(handler func(context.Context, domain.UserRegistered) error) (*natspkg.Subscription, error) {
	sem := make(chan struct{}, 20)
	return c.nc.Subscribe("UserRegistered", func(msg *natspkg.Msg) {
		select {
		case sem <- struct{}{}:
		default:
			fmt.Printf("NATS [UserRegistered]: worker pool full (%d), shedding message\n", 20)
			return
		}
		go func() {
			defer func() { <-sem }()
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("NATS handler panic recovered [UserRegistered]: %v\n", r)
				}
			}()
			var event domain.UserRegistered
			if err := json.Unmarshal(msg.Data, &event); err != nil {
				return
			}
			_ = handler(context.Background(), event)
		}()
	})
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
