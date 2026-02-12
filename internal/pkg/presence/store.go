package presence

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Status represents user online/offline status.
type Status struct {
	IsOnline   bool
	LastSeenAt string
}

const presenceKeyPrefix = "presence:"

var (
	mu          sync.RWMutex
	store       = map[string]Status{}
	redisClient *redis.Client
	ttl         = 60 * time.Second
)

// SetRedisClient enables Redis-backed presence storage.
func SetRedisClient(client *redis.Client) {
	redisClient = client
}

// SetTTL configures the TTL for online presence keys.
func SetTTL(d time.Duration) {
	if d > 0 {
		ttl = d
	}
}

// Set updates the presence for a user and returns the stored status.
func Set(userID string, isOnline bool, at time.Time) Status {
	if redisClient != nil {
		ctx := context.Background()
		key := presenceKeyPrefix + userID
		if !isOnline {
			_ = redisClient.Del(ctx, key).Err()
			return Status{IsOnline: false, LastSeenAt: ""}
		}
		ts := at.UTC().Format(time.RFC3339)
		_ = redisClient.Set(ctx, key, ts, ttl).Err()
		return Status{IsOnline: true, LastSeenAt: ts}
	}

	mu.Lock()
	defer mu.Unlock()
	if !isOnline {
		delete(store, userID)
		return Status{IsOnline: false, LastSeenAt: ""}
	}
	status := Status{
		IsOnline:   true,
		LastSeenAt: at.UTC().Format(time.RFC3339),
	}
	store[userID] = status
	return status
}

// Get returns the stored status for a user.
func Get(userID string) (Status, bool) {
	if redisClient != nil {
		ctx := context.Background()
		val, err := redisClient.Get(ctx, presenceKeyPrefix+userID).Result()
		if err != nil {
			return Status{}, false
		}
		return Status{IsOnline: true, LastSeenAt: val}, true
	}

	mu.RLock()
	defer mu.RUnlock()
	status, ok := store[userID]
	return status, ok
}

// GetMany returns a map of statuses for the provided user IDs.
func GetMany(userIDs []string) map[string]Status {
	if redisClient != nil {
		ctx := context.Background()
		keys := make([]string, 0, len(userIDs))
		for _, id := range userIDs {
			keys = append(keys, presenceKeyPrefix+id)
		}
		out := make(map[string]Status, len(userIDs))
		if len(keys) == 0 {
			return out
		}
		values, err := redisClient.MGet(ctx, keys...).Result()
		if err != nil {
			return out
		}
		for i, v := range values {
			if v == nil {
				continue
			}
			str, ok := v.(string)
			if !ok || str == "" {
				continue
			}
			out[userIDs[i]] = Status{IsOnline: true, LastSeenAt: str}
		}
		return out
	}

	mu.RLock()
	defer mu.RUnlock()
	out := make(map[string]Status, len(userIDs))
	for _, id := range userIDs {
		if status, ok := store[id]; ok {
			out[id] = status
		}
	}
	return out
}
