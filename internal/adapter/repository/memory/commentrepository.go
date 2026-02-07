// Package memory provides an in-memory implementation of the repository.
package memory

import (
	"context"
	"fmt"
	"github.com/strogmv/ang/internal/domain"
	"sort"
	"sync"
)

type CommentRepositoryStub struct {
	mu   sync.RWMutex
	data map[string]*domain.Comment
}

func NewCommentRepositoryStub() *CommentRepositoryStub {
	return &CommentRepositoryStub{
		data: make(map[string]*domain.Comment),
	}
}

func (r *CommentRepositoryStub) Save(ctx context.Context, entity *domain.Comment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if entity == nil {
		return fmt.Errorf("entity is required")
	}
	id := fmt.Sprintf("%p", entity)
	r.data[id] = entity
	return nil
}

func (r *CommentRepositoryStub) FindByID(ctx context.Context, id string) (*domain.Comment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entity, ok := r.data[id]
	if !ok {
		return nil, fmt.Errorf("comment not found: %s", id)
	}
	return entity, nil
}

func (r *CommentRepositoryStub) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.data, id)
	return nil
}
func (r *CommentRepositoryStub) ListAll(ctx context.Context, offset, limit int) ([]domain.Comment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var items []domain.Comment
	for _, item := range r.data {
		if item != nil {
			items = append(items, *item)
		}
	}
	// Apply pagination
	if offset >= len(items) {
		return []domain.Comment{}, nil
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end], nil
}
func (r *CommentRepositoryStub) ListByPost(ctx context.Context, postID string) ([]domain.Comment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var res []domain.Comment
	for _, item := range r.data {
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
		res = append(res, *item)
	}
	if len(res) > 1 {
		sort.Slice(res, func(i, j int) bool {
			return res[i].CreatedAt < res[j].CreatedAt
		})
	}
	return res, nil
}
func (r *CommentRepositoryStub) CountByPost(ctx context.Context, postID string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, item := range r.data {
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
		return item.Count, nil
	}
	return 0, nil
}
func (r *CommentRepositoryStub) DeleteByParent(ctx context.Context, parentID string) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var deleted int64
	for id, item := range r.data {
		if item == nil {
			continue
		}
		match := true
		if item.ParentID != parentID {
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
func (r *CommentRepositoryStub) DeleteByPost(ctx context.Context, postID string) (int64, error) {
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
