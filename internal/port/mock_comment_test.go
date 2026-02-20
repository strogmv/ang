package port

import (
	"context"

	"github.com/strogmv/ang/internal/domain"
)

type CommentRepositoryMock struct {
	FindByIDFunc       func(ctx context.Context, id string) (*domain.Comment, error)
	SaveFunc           func(ctx context.Context, item *domain.Comment) error
	DeleteFunc         func(ctx context.Context, id string) error
	CountByPostFunc    func(ctx context.Context) (any, error)
	DeleteByParentFunc func(ctx context.Context) (any, error)
	DeleteByPostFunc   func(ctx context.Context) (any, error)
	ListByPostFunc     func(ctx context.Context) (any, error)
}

func (m *CommentRepositoryMock) FindByID(ctx context.Context, id string) (*domain.Comment, error) {
	if m.FindByIDFunc != nil {
		return m.FindByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *CommentRepositoryMock) Save(ctx context.Context, item *domain.Comment) error {
	if m.SaveFunc != nil {
		return m.SaveFunc(ctx, item)
	}
	return nil
}

func (m *CommentRepositoryMock) Delete(ctx context.Context, id string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id)
	}
	return nil
}
func (m *CommentRepositoryMock) CountByPost(ctx context.Context) (any, error) {
	if m.CountByPostFunc != nil {
		return m.CountByPostFunc(ctx)
	}
	return nil, nil
}
func (m *CommentRepositoryMock) DeleteByParent(ctx context.Context) (any, error) {
	if m.DeleteByParentFunc != nil {
		return m.DeleteByParentFunc(ctx)
	}
	return nil, nil
}
func (m *CommentRepositoryMock) DeleteByPost(ctx context.Context) (any, error) {
	if m.DeleteByPostFunc != nil {
		return m.DeleteByPostFunc(ctx)
	}
	return nil, nil
}
func (m *CommentRepositoryMock) ListByPost(ctx context.Context) (any, error) {
	if m.ListByPostFunc != nil {
		return m.ListByPostFunc(ctx)
	}
	return nil, nil
}

func NewCommentRepositoryMock() *CommentRepositoryMock {
	return &CommentRepositoryMock{}
}
