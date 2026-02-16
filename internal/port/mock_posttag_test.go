package port

import (
	"context"

	"github.com/strogmv/ang/internal/domain"
)

type PostTagRepositoryMock struct {
	FindByIDFunc     func(ctx context.Context, id string) (*domain.PostTag, error)
	SaveFunc         func(ctx context.Context, item *domain.PostTag) error
	DeleteFunc       func(ctx context.Context, id string) error
	DeleteByPostFunc func(ctx context.Context) (any, error)
	DeleteByTagFunc  func(ctx context.Context) (any, error)
}

func (m *PostTagRepositoryMock) FindByID(ctx context.Context, id string) (*domain.PostTag, error) {
	if m.FindByIDFunc != nil {
		return m.FindByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *PostTagRepositoryMock) Save(ctx context.Context, item *domain.PostTag) error {
	if m.SaveFunc != nil {
		return m.SaveFunc(ctx, item)
	}
	return nil
}

func (m *PostTagRepositoryMock) Delete(ctx context.Context, id string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id)
	}
	return nil
}
func (m *PostTagRepositoryMock) DeleteByPost(ctx context.Context) (any, error) {
	if m.DeleteByPostFunc != nil {
		return m.DeleteByPostFunc(ctx)
	}
	return nil, nil
}
func (m *PostTagRepositoryMock) DeleteByTag(ctx context.Context) (any, error) {
	if m.DeleteByTagFunc != nil {
		return m.DeleteByTagFunc(ctx)
	}
	return nil, nil
}

func NewPostTagRepositoryMock() *PostTagRepositoryMock {
	return &PostTagRepositoryMock{}
}
