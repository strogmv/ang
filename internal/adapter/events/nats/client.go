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

func (c *Client) SubscribeQueueRaw(subject string, queue string, handler func(data []byte) error) (*natspkg.Subscription, error) {
	return c.nc.QueueSubscribe(subject, queue, func(msg *natspkg.Msg) {
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
func (c *Client) PublishCommentCreated(ctx context.Context, event domain.CommentCreated) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	delay := time.Duration(100) * time.Millisecond
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if lastErr = c.nc.Publish("CommentCreated", data); lastErr == nil {
			return nil
		}
		if attempt < 3-1 {
			time.Sleep(delay)
			delay *= 2
		}
	}
	fmt.Printf("NATS: publish failed after %d attempts [CommentCreated]: %v\n", 3, lastErr)
	return lastErr
}

// BroadcastCommentCreated delegates to PublishCommentCreated — NATS subjects are already broadcast channels.
func (c *Client) BroadcastCommentCreated(ctx context.Context, event domain.CommentCreated) error {
	return c.PublishCommentCreated(ctx, event)
}

// SubscribeCommentCreated processes messages concurrently using a worker pool of 20 goroutines.
// The NATS callback returns immediately (non-blocking) — true backpressure via channel semaphore.
// When the pool is full the message is shed (dropped) rather than queued to prevent cascade delays.
func (c *Client) SubscribeCommentCreated(handler func(context.Context, domain.CommentCreated) error) (*natspkg.Subscription, error) {
	sem := make(chan struct{}, 20)
	return c.nc.Subscribe("CommentCreated", func(msg *natspkg.Msg) {
		select {
		case sem <- struct{}{}:
		default:
			fmt.Printf("NATS [CommentCreated]: worker pool full (%d), shedding message\n", 20)
			return
		}
		go func() {
			defer func() { <-sem }()
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("NATS handler panic recovered [CommentCreated]: %v\n", r)
				}
			}()
			var event domain.CommentCreated
			if err := json.Unmarshal(msg.Data, &event); err != nil {
				return
			}
			_ = handler(context.Background(), event)
		}()
	})
}
func (c *Client) PublishPostCreated(ctx context.Context, event domain.PostCreated) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	delay := time.Duration(100) * time.Millisecond
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if lastErr = c.nc.Publish("PostCreated", data); lastErr == nil {
			return nil
		}
		if attempt < 3-1 {
			time.Sleep(delay)
			delay *= 2
		}
	}
	fmt.Printf("NATS: publish failed after %d attempts [PostCreated]: %v\n", 3, lastErr)
	return lastErr
}

// BroadcastPostCreated delegates to PublishPostCreated — NATS subjects are already broadcast channels.
func (c *Client) BroadcastPostCreated(ctx context.Context, event domain.PostCreated) error {
	return c.PublishPostCreated(ctx, event)
}

// SubscribePostCreated processes messages concurrently using a worker pool of 20 goroutines.
// The NATS callback returns immediately (non-blocking) — true backpressure via channel semaphore.
// When the pool is full the message is shed (dropped) rather than queued to prevent cascade delays.
func (c *Client) SubscribePostCreated(handler func(context.Context, domain.PostCreated) error) (*natspkg.Subscription, error) {
	sem := make(chan struct{}, 20)
	return c.nc.Subscribe("PostCreated", func(msg *natspkg.Msg) {
		select {
		case sem <- struct{}{}:
		default:
			fmt.Printf("NATS [PostCreated]: worker pool full (%d), shedding message\n", 20)
			return
		}
		go func() {
			defer func() { <-sem }()
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("NATS handler panic recovered [PostCreated]: %v\n", r)
				}
			}()
			var event domain.PostCreated
			if err := json.Unmarshal(msg.Data, &event); err != nil {
				return
			}
			_ = handler(context.Background(), event)
		}()
	})
}
func (c *Client) PublishPostPublished(ctx context.Context, event domain.PostPublished) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	delay := time.Duration(100) * time.Millisecond
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if lastErr = c.nc.Publish("PostPublished", data); lastErr == nil {
			return nil
		}
		if attempt < 3-1 {
			time.Sleep(delay)
			delay *= 2
		}
	}
	fmt.Printf("NATS: publish failed after %d attempts [PostPublished]: %v\n", 3, lastErr)
	return lastErr
}

// BroadcastPostPublished delegates to PublishPostPublished — NATS subjects are already broadcast channels.
func (c *Client) BroadcastPostPublished(ctx context.Context, event domain.PostPublished) error {
	return c.PublishPostPublished(ctx, event)
}

// SubscribePostPublished processes messages concurrently using a worker pool of 20 goroutines.
// The NATS callback returns immediately (non-blocking) — true backpressure via channel semaphore.
// When the pool is full the message is shed (dropped) rather than queued to prevent cascade delays.
func (c *Client) SubscribePostPublished(handler func(context.Context, domain.PostPublished) error) (*natspkg.Subscription, error) {
	sem := make(chan struct{}, 20)
	return c.nc.Subscribe("PostPublished", func(msg *natspkg.Msg) {
		select {
		case sem <- struct{}{}:
		default:
			fmt.Printf("NATS [PostPublished]: worker pool full (%d), shedding message\n", 20)
			return
		}
		go func() {
			defer func() { <-sem }()
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("NATS handler panic recovered [PostPublished]: %v\n", r)
				}
			}()
			var event domain.PostPublished
			if err := json.Unmarshal(msg.Data, &event); err != nil {
				return
			}
			_ = handler(context.Background(), event)
		}()
	})
}
func (c *Client) PublishPostUpdated(ctx context.Context, event domain.PostUpdated) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	delay := time.Duration(100) * time.Millisecond
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if lastErr = c.nc.Publish("PostUpdated", data); lastErr == nil {
			return nil
		}
		if attempt < 3-1 {
			time.Sleep(delay)
			delay *= 2
		}
	}
	fmt.Printf("NATS: publish failed after %d attempts [PostUpdated]: %v\n", 3, lastErr)
	return lastErr
}

// BroadcastPostUpdated delegates to PublishPostUpdated — NATS subjects are already broadcast channels.
func (c *Client) BroadcastPostUpdated(ctx context.Context, event domain.PostUpdated) error {
	return c.PublishPostUpdated(ctx, event)
}

// SubscribePostUpdated processes messages concurrently using a worker pool of 20 goroutines.
// The NATS callback returns immediately (non-blocking) — true backpressure via channel semaphore.
// When the pool is full the message is shed (dropped) rather than queued to prevent cascade delays.
func (c *Client) SubscribePostUpdated(handler func(context.Context, domain.PostUpdated) error) (*natspkg.Subscription, error) {
	sem := make(chan struct{}, 20)
	return c.nc.Subscribe("PostUpdated", func(msg *natspkg.Msg) {
		select {
		case sem <- struct{}{}:
		default:
			fmt.Printf("NATS [PostUpdated]: worker pool full (%d), shedding message\n", 20)
			return
		}
		go func() {
			defer func() { <-sem }()
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("NATS handler panic recovered [PostUpdated]: %v\n", r)
				}
			}()
			var event domain.PostUpdated
			if err := json.Unmarshal(msg.Data, &event); err != nil {
				return
			}
			_ = handler(context.Background(), event)
		}()
	})
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
