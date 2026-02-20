// Package memory provides an in-memory implementation of the repository.
package memory

import (
	"context"
	"fmt"
	"github.com/strogmv/ang/internal/domain"
	"reflect"
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
func (r *PostRepositoryStub) CountPublished(ctx context.Context, status string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var cnt int64
	for _, item := range r.data {
		if item == nil {
			continue
		}
		match := true
		if !matchesOpPost(item.Status, status, "=") {
			match = false
		}
		if !match {
			continue
		}
		cnt++
	}
	return cnt, nil
}
func (r *PostRepositoryStub) CountPublishedByTag(ctx context.Context, status string, id string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var cnt int64
	for _, item := range r.data {
		if item == nil {
			continue
		}
		match := true
		if !matchesOpPost(item.Status, status, "=") {
			match = false
		}
		if !matchesOpPost(item.ID, id, "IN") {
			match = false
		}
		if !match {
			continue
		}
		cnt++
	}
	return cnt, nil
}
func (r *PostRepositoryStub) FindBySlug(ctx context.Context, slug string) (*domain.Post, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, item := range r.data {
		if item == nil {
			continue
		}
		match := true
		if !matchesOpPost(item.Slug, slug, "=") {
			match = false
		}
		if !match {
			continue
		}
		return item, nil
	}
	return nil, nil
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
		if !matchesOpPost(item.AuthorID, authorID, "=") {
			match = false
		}
		if !match {
			continue
		}
		res = append(res, *item)
	}
	if len(res) > 1 {
		sort.Slice(res, func(i, j int) bool {
			return compareGreaterPost(res[i].CreatedAt, res[j].CreatedAt)
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
		if !matchesOpPost(item.AuthorID, authorID, "=") {
			match = false
		}
		if !matchesOpPost(item.Status, status, "=") {
			match = false
		}
		if !match {
			continue
		}
		res = append(res, *item)
	}
	if len(res) > 1 {
		sort.Slice(res, func(i, j int) bool {
			return compareGreaterPost(res[i].CreatedAt, res[j].CreatedAt)
		})
	}
	return res, nil
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
		if !matchesOpPost(item.Status, status, "=") {
			match = false
		}
		if !match {
			continue
		}
		res = append(res, *item)
	}
	if len(res) > 1 {
		sort.Slice(res, func(i, j int) bool {
			return compareGreaterPost(res[i].PublishedAt, res[j].PublishedAt)
		})
	}
	return res, nil
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
		if !matchesOpPost(item.Status, status, "=") {
			match = false
		}
		if !matchesOpPost(item.ID, id, "IN") {
			match = false
		}
		if !match {
			continue
		}
		res = append(res, *item)
	}
	if len(res) > 1 {
		sort.Slice(res, func(i, j int) bool {
			return compareGreaterPost(res[i].PublishedAt, res[j].PublishedAt)
		})
	}
	return res, nil
}

func matchesOpPost(left, right any, op string) bool {
	switch op {
	case "!=", "<>":
		return !valueEqualsPost(left, right)
	case "<":
		return compareLessPost(left, right)
	case ">":
		return compareGreaterPost(left, right)
	case "<=":
		return compareLessPost(left, right) || valueEqualsPost(left, right)
	case ">=":
		return compareGreaterPost(left, right) || valueEqualsPost(left, right)
	case "IN", "in":
		return valueEqualsPost(left, right)
	default:
		return valueEqualsPost(left, right)
	}
}

func valueEqualsPost(left, right any) bool {
	return reflect.DeepEqual(left, right)
}

func compareLessPost(left, right any) bool {
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

func compareGreaterPost(left, right any) bool {
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
