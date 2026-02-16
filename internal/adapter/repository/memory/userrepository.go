// Package memory provides an in-memory implementation of the repository.
package memory

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/strogmv/ang/internal/domain"
)

type UserRepositoryStub struct {
	mu   sync.RWMutex
	data map[string]*domain.User
}

func NewUserRepositoryStub() *UserRepositoryStub {
	return &UserRepositoryStub{
		data: make(map[string]*domain.User),
	}
}

func (r *UserRepositoryStub) Save(ctx context.Context, entity *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if entity == nil {
		return fmt.Errorf("entity is required")
	}
	id := fmt.Sprintf("%p", entity)
	r.data[id] = entity
	return nil
}

func (r *UserRepositoryStub) FindByID(ctx context.Context, id string) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entity, ok := r.data[id]
	if !ok {
		return nil, fmt.Errorf("user not found: %s", id)
	}
	return entity, nil
}

func (r *UserRepositoryStub) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.data, id)
	return nil
}
func (r *UserRepositoryStub) ListAll(ctx context.Context, offset, limit int) ([]domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var items []domain.User
	for _, item := range r.data {
		if item != nil {
			items = append(items, *item)
		}
	}
	// Apply pagination
	if offset >= len(items) {
		return []domain.User{}, nil
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end], nil
}
func (r *UserRepositoryStub) FindByEmail(ctx context.Context, email map[string]any) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, item := range r.data {
		if item == nil {
			continue
		}
		match := true
		if !matchesOpUser(item.Email, email, "=") {
			match = false
		}
		if !match {
			continue
		}
		return item, nil
	}
	return nil, nil
}

func matchesOpUser(left, right any, op string) bool {
	switch op {
	case "!=", "<>":
		return !valueEqualsUser(left, right)
	case "<":
		return compareLessUser(left, right)
	case ">":
		return compareGreaterUser(left, right)
	case "<=":
		return compareLessUser(left, right) || valueEqualsUser(left, right)
	case ">=":
		return compareGreaterUser(left, right) || valueEqualsUser(left, right)
	case "IN", "in":
		return valueEqualsUser(left, right)
	default:
		return valueEqualsUser(left, right)
	}
}

func valueEqualsUser(left, right any) bool {
	return reflect.DeepEqual(left, right)
}

func compareLessUser(left, right any) bool {
	switch l := left.(type) {
	case int:
		if r, ok := right.(int); ok {
			return l < r
		}
	case int64:
		if r, ok := right.(int64); ok {
			return l < r
		}
	case float64:
		if r, ok := right.(float64); ok {
			return l < r
		}
	case string:
		if r, ok := right.(string); ok {
			return l < r
		}
	}
	return fmt.Sprint(left) < fmt.Sprint(right)
}

func compareGreaterUser(left, right any) bool {
	switch l := left.(type) {
	case int:
		if r, ok := right.(int); ok {
			return l > r
		}
	case int64:
		if r, ok := right.(int64); ok {
			return l > r
		}
	case float64:
		if r, ok := right.(float64); ok {
			return l > r
		}
	case string:
		if r, ok := right.(string); ok {
			return l > r
		}
	}
	return fmt.Sprint(left) > fmt.Sprint(right)
}
