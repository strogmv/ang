// Package memory provides an in-memory implementation of the repository.
package memory

import (
	"context"
	"fmt"
	"github.com/strogmv/ang/internal/domain"
	"sort"
	"sync"
)

type PostRepositoryStub struct {
	mu   sync.RWMutex
	data map[string]*domain.Post
}

func NewPostRepositoryStub() *PostRepositoryStub {
	return &PostRepositoryStub{
		data: make(map[string]*domain.Post),
	}
}

func (r *PostRepositoryStub) Save(ctx context.Context, entity *domain.Post) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if entity == nil {
		return fmt.Errorf("entity is required")
	}
	id := fmt.Sprintf("%p", entity)
	r.data[id] = entity
	return nil
}

func (r *PostRepositoryStub) FindByID(ctx context.Context, id string) (*domain.Post, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entity, ok := r.data[id]
	if !ok {
		return nil, fmt.Errorf("post not found: %s", id)
	}
	return entity, nil
}

func (r *PostRepositoryStub) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.data, id)
	return nil
}
func (r *PostRepositoryStub) ListAll(ctx context.Context, offset, limit int) ([]domain.Post, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var items []domain.Post
	for _, item := range r.data {
		if item != nil {
			items = append(items, *item)
		}
	}
	// Apply pagination
	if offset >= len(items) {
		return []domain.Post{}, nil
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end], nil
}
func (r *PostRepositoryStub) FindBySlug(ctx context.Context, slug string) (*domain.Post, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, item := range r.data {
		if item == nil {
			continue
		}
		match := true
		if item.Slug != slug {
			match = false
		}
		if !match {
			continue
		}
		return item, nil
	}
	return nil, nil
}
func (r *PostRepositoryStub) ListPublished(ctx context.Context, status string) ([]domain.Post, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var res []domain.Post
	for _, item := range r.data {
		if item == nil {
			continue
		}
		match := true
		if item.Status != status {
			match = false
		}
		if !match {
			continue
		}
		res = append(res, *item)
	}
	if len(res) > 1 {
		sort.Slice(res, func(i, j int) bool {
			return res[i].PublishedAt > res[j].PublishedAt
		})
	}
	return res, nil
}
func (r *PostRepositoryStub) CountPublished(ctx context.Context, status string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, item := range r.data {
		if item == nil {
			continue
		}
		match := true
		if item.Status != status {
			match = false
		}
		if !match {
			continue
		}
		return item.Count, nil
	}
	return 0, nil
}
func (r *PostRepositoryStub) ListPublishedByTag(ctx context.Context, status string, id string) ([]domain.Post, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var res []domain.Post
	for _, item := range r.data {
		if item == nil {
			continue
		}
		match := true
		if item.Status != status {
			match = false
		}
		if item.ID != id {
			match = false
		}
		if !match {
			continue
		}
		res = append(res, *item)
	}
	if len(res) > 1 {
		sort.Slice(res, func(i, j int) bool {
			return res[i].PublishedAt > res[j].PublishedAt
		})
	}
	return res, nil
}
func (r *PostRepositoryStub) CountPublishedByTag(ctx context.Context, status string, id string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, item := range r.data {
		if item == nil {
			continue
		}
		match := true
		if item.Status != status {
			match = false
		}
		if item.ID != id {
			match = false
		}
		if !match {
			continue
		}
		return item.Count, nil
	}
	return 0, nil
}
func (r *PostRepositoryStub) ListByAuthor(ctx context.Context, authorID string) ([]domain.Post, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var res []domain.Post
	for _, item := range r.data {
		if item == nil {
			continue
		}
		match := true
		if item.AuthorID != authorID {
			match = false
		}
		if !match {
			continue
		}
		res = append(res, *item)
	}
	if len(res) > 1 {
		sort.Slice(res, func(i, j int) bool {
			return res[i].CreatedAt > res[j].CreatedAt
		})
	}
	return res, nil
}
func (r *PostRepositoryStub) ListByAuthorAndStatus(ctx context.Context, authorID string, status string) ([]domain.Post, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var res []domain.Post
	for _, item := range r.data {
		if item == nil {
			continue
		}
		match := true
		if item.AuthorID != authorID {
			match = false
		}
		if item.Status != status {
			match = false
		}
		if !match {
			continue
		}
		res = append(res, *item)
	}
	if len(res) > 1 {
		sort.Slice(res, func(i, j int) bool {
			return res[i].CreatedAt > res[j].CreatedAt
		})
	}
	return res, nil
}
