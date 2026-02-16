package port

import (
	"context"

	"github.com/strogmv/ang/internal/domain"
)

type TagRepositoryMock struct {
	FindByIDFunc   func(ctx context.Context, id string) (*domain.Tag, error)
	SaveFunc       func(ctx context.Context, item *domain.Tag) error
	DeleteFunc     func(ctx context.Context, id string) error
	FindBySlugFunc func(ctx context.Context) (any, error)
	ListAllFunc    func(ctx context.Context) (any, error)
	ListByPostFunc func(ctx context.Context) (any, error)
}

func (m *TagRepositoryMock) FindByID(ctx context.Context, id string) (*domain.Tag, error) {
	if m.FindByIDFunc != nil {
		return m.FindByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *TagRepositoryMock) Save(ctx context.Context, item *domain.Tag) error {
	if m.SaveFunc != nil {
		return m.SaveFunc(ctx, item)
	}
	return nil
}

func (m *TagRepositoryMock) Delete(ctx context.Context, id string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id)
	}
	return nil
}
func (m *TagRepositoryMock) FindBySlug(ctx context.Context) (any, error) {
	if m.FindBySlugFunc != nil {
		return m.FindBySlugFunc(ctx)
	}
	return nil, nil
}
func (m *TagRepositoryMock) ListAll(ctx context.Context) (any, error) {
	if m.ListAllFunc != nil {
		return m.ListAllFunc(ctx)
	}
	return nil, nil
}
func (m *TagRepositoryMock) ListByPost(ctx context.Context) (any, error) {
	if m.ListByPostFunc != nil {
		return m.ListByPostFunc(ctx)
	}
	return nil, nil
}

func NewTagRepositoryMock() *TagRepositoryMock {
	return &TagRepositoryMock{}
}
