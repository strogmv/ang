// Package memory provides an in-memory implementation of the repository.
package memory

import (
	"context"
	"fmt"
	"github.com/strogmv/ang/internal/domain"
	"sort"
	"sync"
)

type TagRepositoryStub struct {
	mu   sync.RWMutex
	data map[string]*domain.Tag
}

func NewTagRepositoryStub() *TagRepositoryStub {
	return &TagRepositoryStub{
		data: make(map[string]*domain.Tag),
	}
}

func (r *TagRepositoryStub) Save(ctx context.Context, entity *domain.Tag) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if entity == nil {
		return fmt.Errorf("entity is required")
	}
	id := fmt.Sprintf("%p", entity)
	r.data[id] = entity
	return nil
}

func (r *TagRepositoryStub) FindByID(ctx context.Context, id string) (*domain.Tag, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entity, ok := r.data[id]
	if !ok {
		return nil, fmt.Errorf("tag not found: %s", id)
	}
	return entity, nil
}

func (r *TagRepositoryStub) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.data, id)
	return nil
}
func (r *TagRepositoryStub) FindBySlug(ctx context.Context, slug map[string]any) (*domain.Tag, error) {
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
func (r *TagRepositoryStub) ListAll(ctx context.Context) ([]domain.Tag, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var res []domain.Tag
	for _, item := range r.data {
		if item == nil {
			continue
		}
		match := true
		if !match {
			continue
		}
		res = append(res, *item)
	}
	if len(res) > 1 {
		sort.Slice(res, func(i, j int) bool {
			return res[i].Name < res[j].Name
		})
	}
	return res, nil
}
func (r *TagRepositoryStub) ListByPost(ctx context.Context, id map[string]any) ([]domain.Tag, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var res []domain.Tag
	for _, item := range r.data {
		if item == nil {
			continue
		}
		match := true
		if item.ID != id {
			match = false
		}
		if !match {
			continue
		}
		res = append(res, *item)
	}
	return res, nil
}
