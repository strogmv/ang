// Package memory provides an in-memory implementation of the repository.
package memory

import (
	"context"
	"fmt"
	"github.com/strogmv/ang/internal/domain"
	"sync"
)

type PostTagRepositoryStub struct {
	mu   sync.RWMutex
	data map[string]*domain.PostTag
}

func NewPostTagRepositoryStub() *PostTagRepositoryStub {
	return &PostTagRepositoryStub{
		data: make(map[string]*domain.PostTag),
	}
}

func (r *PostTagRepositoryStub) Save(ctx context.Context, entity *domain.PostTag) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if entity == nil {
		return fmt.Errorf("entity is required")
	}
	id := fmt.Sprintf("%p", entity)
	r.data[id] = entity
	return nil
}

func (r *PostTagRepositoryStub) FindByID(ctx context.Context, id string) (*domain.PostTag, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entity, ok := r.data[id]
	if !ok {
		return nil, fmt.Errorf("posttag not found: %s", id)
	}
	return entity, nil
}

func (r *PostTagRepositoryStub) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.data, id)
	return nil
}
func (r *PostTagRepositoryStub) ListAll(ctx context.Context, offset, limit int) ([]domain.PostTag, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var items []domain.PostTag
	for _, item := range r.data {
		if item != nil {
			items = append(items, *item)
		}
	}
	// Apply pagination
	if offset >= len(items) {
		return []domain.PostTag{}, nil
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end], nil
}
func (r *PostTagRepositoryStub) DeleteByPost(ctx context.Context, postID string) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var deleted int64
	for id, item := range r.data {
		if item == nil {
			continue
		}
		match := true
		if item.PostID != postID {
			match = false
		}
		if !match {
			continue
		}
		delete(r.data, id)
		deleted++
	}
	return deleted, nil
}
func (r *PostTagRepositoryStub) DeleteByTag(ctx context.Context, tagID string) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var deleted int64
	for id, item := range r.data {
		if item == nil {
			continue
		}
		match := true
		if item.TagID != tagID {
			match = false
		}
		if !match {
			continue
		}
		delete(r.data, id)
		deleted++
	}
	return deleted, nil
}
