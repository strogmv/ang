package port

import (
	"context"

	"github.com/strogmv/ang/internal/domain"
)

type UserRepositoryMock struct {
	FindByIDFunc    func(ctx context.Context, id string) (*domain.User, error)
	SaveFunc        func(ctx context.Context, item *domain.User) error
	DeleteFunc      func(ctx context.Context, id string) error
	FindByEmailFunc func(ctx context.Context) (any, error)
}

func (m *UserRepositoryMock) FindByID(ctx context.Context, id string) (*domain.User, error) {
	if m.FindByIDFunc != nil {
		return m.FindByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *UserRepositoryMock) Save(ctx context.Context, item *domain.User) error {
	if m.SaveFunc != nil {
		return m.SaveFunc(ctx, item)
	}
	return nil
}

func (m *UserRepositoryMock) Delete(ctx context.Context, id string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id)
	}
	return nil
}
func (m *UserRepositoryMock) FindByEmail(ctx context.Context) (any, error) {
	if m.FindByEmailFunc != nil {
		return m.FindByEmailFunc(ctx)
	}
	return nil, nil
}

func NewUserRepositoryMock() *UserRepositoryMock {
	return &UserRepositoryMock{}
}
