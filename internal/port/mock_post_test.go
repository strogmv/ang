package port

import (
	"context"

	"github.com/strogmv/ang/internal/domain"
)

type PostRepositoryMock struct {
	FindByIDFunc              func(ctx context.Context, id string) (*domain.Post, error)
	SaveFunc                  func(ctx context.Context, item *domain.Post) error
	DeleteFunc                func(ctx context.Context, id string) error
	FindBySlugFunc            func(ctx context.Context) (any, error)
	ListPublishedFunc         func(ctx context.Context) (any, error)
	CountPublishedFunc        func(ctx context.Context) (any, error)
	ListPublishedByTagFunc    func(ctx context.Context) (any, error)
	CountPublishedByTagFunc   func(ctx context.Context) (any, error)
	ListByAuthorFunc          func(ctx context.Context) (any, error)
	ListByAuthorAndStatusFunc func(ctx context.Context) (any, error)
}

func (m *PostRepositoryMock) FindByID(ctx context.Context, id string) (*domain.Post, error) {
	if m.FindByIDFunc != nil {
		return m.FindByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *PostRepositoryMock) Save(ctx context.Context, item *domain.Post) error {
	if m.SaveFunc != nil {
		return m.SaveFunc(ctx, item)
	}
	return nil
}

func (m *PostRepositoryMock) Delete(ctx context.Context, id string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id)
	}
	return nil
}
func (m *PostRepositoryMock) FindBySlug(ctx context.Context) (any, error) {
	if m.FindBySlugFunc != nil {
		return m.FindBySlugFunc(ctx)
	}
	return nil, nil
}
func (m *PostRepositoryMock) ListPublished(ctx context.Context) (any, error) {
	if m.ListPublishedFunc != nil {
		return m.ListPublishedFunc(ctx)
	}
	return nil, nil
}
func (m *PostRepositoryMock) CountPublished(ctx context.Context) (any, error) {
	if m.CountPublishedFunc != nil {
		return m.CountPublishedFunc(ctx)
	}
	return nil, nil
}
func (m *PostRepositoryMock) ListPublishedByTag(ctx context.Context) (any, error) {
	if m.ListPublishedByTagFunc != nil {
		return m.ListPublishedByTagFunc(ctx)
	}
	return nil, nil
}
func (m *PostRepositoryMock) CountPublishedByTag(ctx context.Context) (any, error) {
	if m.CountPublishedByTagFunc != nil {
		return m.CountPublishedByTagFunc(ctx)
	}
	return nil, nil
}
func (m *PostRepositoryMock) ListByAuthor(ctx context.Context) (any, error) {
	if m.ListByAuthorFunc != nil {
		return m.ListByAuthorFunc(ctx)
	}
	return nil, nil
}
func (m *PostRepositoryMock) ListByAuthorAndStatus(ctx context.Context) (any, error) {
	if m.ListByAuthorAndStatusFunc != nil {
		return m.ListByAuthorAndStatusFunc(ctx)
	}
	return nil, nil
}

func NewPostRepositoryMock() *PostRepositoryMock {
	return &PostRepositoryMock{}
}
